package netpolicy

import (
	socks "golang.org/x/net/proxy"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var bridgeMu sync.Mutex
var bridges = map[string]string{}

// HTTPHelperProxy adapts a SOCKS route for helpers, such as ffmpeg, that only accept HTTP CONNECT.
func HTTPHelperProxy(raw string) string {
	value := ProxyURL(raw)
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "socks5" {
		return value
	}
	bridgeMu.Lock()
	defer bridgeMu.Unlock()
	if known := bridges[value]; known != "" {
		return known
	}
	dialer, err := socks.FromURL(u, &net.Dialer{Timeout: 20 * time.Second})
	if err != nil {
		return value
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return value
	}
	route := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "CONNECT" {
			http.Error(w, "CONNECT required", 405)
			return
		}
		target, err := dialer.Dial("tcp", r.Host)
		if err != nil {
			http.Error(w, "proxy connection failed", 502)
			return
		}
		client, buffer, err := w.(http.Hijacker).Hijack()
		if err != nil {
			target.Close()
			return
		}
		client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		go func() { io.Copy(target, buffer); target.Close(); client.Close() }()
		go func() { io.Copy(client, target); target.Close(); client.Close() }()
	})
	server := &http.Server{Handler: route, ReadHeaderTimeout: 10 * time.Second}
	go server.Serve(listener)
	result := "http://" + listener.Addr().String()
	bridges[value] = result
	return result
}
