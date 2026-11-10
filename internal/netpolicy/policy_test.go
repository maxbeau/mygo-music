package netpolicy

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// A custom proxy carries remote traffic, while local audio helpers remain reachable directly.
func TestProxyRoutingAndLocalBypass(t *testing.T) {
	old := http.DefaultTransport
	defer func() {
		http.DefaultTransport = old
		mu.Lock()
		installed = false
		transport = base.Clone()
		current = Config{}
		mu.Unlock()
	}()
	calls := 0
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Host != "remote.example" {
			t.Error(r.URL)
		}
		io.WriteString(w, "proxied")
	}))
	defer proxy.Close()
	if err := Configure(Config{Mode: "custom", URL: proxy.URL}); err != nil {
		t.Fatal(err)
	}
	response, err := http.Get("http://remote.example/song")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "local") }))
	defer local.Close()
	response, err = http.Get(local.URL)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if calls != 1 || string(b) != "local" {
		t.Fatalf("proxy=%d local=%q", calls, b)
	}
	if err := Configure(Config{Mode: "direct"}); err != nil {
		t.Fatal(err)
	}
	env := strings.Join(ProcessEnv([]string{"HTTPS_PROXY=http://old:9", "ALL_PROXY=socks5://old:8", "HOME=/unchanged"}), "\n")
	if strings.Contains(env, "old:") || !strings.Contains(env, "HTTPS_PROXY=\n") || !strings.Contains(env, "HOME=/unchanged") || !strings.Contains(env, "127.0.0.1") {
		t.Fatal(env)
	}
}

// Invalid or credential-bearing proxy URLs never become saved settings.
func TestProxyValidation(t *testing.T) {
	for _, raw := range []string{"http://user:secret@host:80", "file:///etc/passwd", "http://host", "http://host:80/path", "http://host:80?key=secret"} {
		if _, err := Validate(Config{Mode: "custom", URL: raw}); err == nil {
			t.Fatal(raw)
		}
	}
	got, err := Validate(Config{Mode: "custom", URL: "127.0.0.1:7890"})
	if err != nil || got.URL != "http://127.0.0.1:7890" {
		t.Fatal(got, err)
	}
	mu.RLock()
	tr := transport
	mu.RUnlock()
	req := &http.Request{URL: &url.URL{Scheme: "https", Host: "[::1]:9000"}}
	p, _ := tr.Proxy(req)
	if p != nil {
		t.Fatal("IPv6 localhost proxied")
	}
}

// An HTTP-only helper can tunnel through the configured SOCKS proxy without sending traffic directly.
func TestSOCKSHelperTunnel(t *testing.T) {
	old := Current()
	defer Configure(old)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan error, 1)
	go func() {
		c, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer c.Close()
		greeting := make([]byte, 3)
		_, err = io.ReadFull(c, greeting)
		if err != nil {
			done <- err
			return
		}
		c.Write([]byte{5, 0})
		header := make([]byte, 5)
		_, err = io.ReadFull(c, header)
		if err != nil {
			done <- err
			return
		}
		address := make([]byte, int(header[4])+2)
		_, err = io.ReadFull(c, address)
		if err != nil {
			done <- err
			return
		}
		if string(address[:len(address)-2]) != "media.example" {
			done <- io.ErrUnexpectedEOF
			return
		}
		c.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 80})
		message := make([]byte, 4)
		_, err = io.ReadFull(c, message)
		if err == nil {
			_, err = c.Write(message)
		}
		done <- err
	}()
	if err = Configure(Config{Mode: "custom", URL: "socks5://" + listener.Addr().String()}); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(HTTPHelperProxy("https://media.example"))
	c, err := net.Dial("tcp", u.Host)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(5 * time.Second))
	io.WriteString(c, "CONNECT media.example:443 HTTP/1.1\r\nHost: media.example:443\r\n\r\n")
	reader := bufio.NewReader(c)
	res, err := http.ReadResponse(reader, &http.Request{Method: "CONNECT"})
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("CONNECT %v %v", res, err)
	}
	io.WriteString(c, "ping")
	b := make([]byte, 4)
	_, err = io.ReadFull(reader, b)
	if err != nil || string(b) != "ping" {
		t.Fatalf("tunnel %q %v", b, err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}
