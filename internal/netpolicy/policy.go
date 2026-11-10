// Package netpolicy shares the app's proxy policy with HTTP clients and helpers.
package netpolicy

import (
	"errors"
	"golang.org/x/net/http/httpproxy"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Config struct {
	Mode string `json:"mode,omitempty"`
	URL  string `json:"url,omitempty"`
}

var mu sync.RWMutex
var current Config
var proxies map[string]*url.URL
var installed bool
var base = http.DefaultTransport.(*http.Transport).Clone()
var transport = base.Clone()

type sharedTransport struct{}

func (sharedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	mu.RLock()
	t := transport
	mu.RUnlock()
	return t.RoundTrip(r)
}
func local(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
func Validate(c Config) (Config, error) {
	if c.Mode == "" {
		c.Mode = "system"
	}
	if c.Mode != "system" && c.Mode != "direct" && c.Mode != "custom" {
		return c, errors.New("unknown proxy mode")
	}
	if c.Mode != "custom" {
		return Config{Mode: c.Mode}, nil
	}
	c.URL = strings.TrimSpace(c.URL)
	if !strings.Contains(c.URL, "://") {
		c.URL = "http://" + c.URL
	}
	u, err := url.Parse(c.URL)
	if err != nil || u.Hostname() == "" || u.Port() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5") {
		return c, errors.New("use an HTTP, HTTPS or SOCKS5 proxy host and port without credentials")
	}
	port, portErr := strconv.Atoi(u.Port())
	if portErr != nil || port < 1 || port > 65535 {
		return c, errors.New("invalid proxy port")
	}
	_, _, err = net.SplitHostPort(u.Host)
	if err != nil {
		return c, errors.New("invalid proxy address")
	}
	c.URL = strings.TrimSuffix(u.String(), "/")
	return c, nil
}
func Configure(c Config) error {
	c, err := Validate(c)
	if err != nil {
		return err
	}
	ps := systemProxies()
	if c.Mode == "custom" {
		u, _ := url.Parse(c.URL)
		ps = map[string]*url.URL{"http": u, "https": u}
	}
	if c.Mode == "direct" {
		ps = nil
	}
	environment := httpproxy.FromEnvironment()
	if environment.HTTPProxy == "" && ps["http"] != nil {
		environment.HTTPProxy = ps["http"].String()
	}
	if environment.HTTPSProxy == "" && ps["https"] != nil {
		environment.HTTPSProxy = ps["https"].String()
	}
	systemRoute := environment.ProxyFunc()
	t := base.Clone()
	t.Proxy = func(r *http.Request) (*url.URL, error) {
		if local(r.URL.Hostname()) {
			return nil, nil
		}
		if c.Mode == "system" {
			return systemRoute(r.URL)
		}
		return ps[r.URL.Scheme], nil
	}
	mu.Lock()
	old := transport
	transport = t
	current = c
	proxies = ps
	if !installed {
		http.DefaultTransport = sharedTransport{}
		installed = true
	}
	mu.Unlock()
	old.CloseIdleConnections()
	return nil
}
func Current() Config { mu.RLock(); defer mu.RUnlock(); return current }

// ProxyURL uses the same route as fetch, excluding local helper servers.
func ProxyURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || local(u.Hostname()) {
		return ""
	}
	mu.RLock()
	t := transport
	mu.RUnlock()
	if t.Proxy == nil {
		return ""
	}
	p, _ := t.Proxy(&http.Request{URL: u})
	if p == nil {
		return ""
	}
	return p.String()
}
func ProcessEnv(original []string) []string {
	mu.RLock()
	c := current
	ps := proxies
	mu.RUnlock()
	if c.Mode == "" || c.Mode == "system" && len(ps) == 0 {
		return original
	}
	out := make([]string, 0, len(original)+8)
	bypass := "localhost,127.0.0.1,::1"
	for _, v := range original {
		k, _, _ := strings.Cut(v, "=")
		switch strings.ToLower(k) {
		case "no_proxy":
			if c.Mode == "system" {
				_, value, _ := strings.Cut(v, "=")
				if value != "" {
					bypass += "," + value
				}
			}
			continue
		case "http_proxy", "https_proxy", "all_proxy":
			continue
		}
		out = append(out, v)
	}
	for _, scheme := range []string{"http", "https"} {
		v := ""
		if c.Mode == "system" {
			v = ProxyURL(scheme + "://music.youtube.com")
		} else if p := ps[scheme]; p != nil {
			v = p.String()
		}
		out = append(out, scheme+"_proxy="+v, strings.ToUpper(scheme)+"_PROXY="+v)
	}
	return append(out, "NO_PROXY="+bypass, "no_proxy="+bypass)
}
func Env() []string { return ProcessEnv(os.Environ()) }
