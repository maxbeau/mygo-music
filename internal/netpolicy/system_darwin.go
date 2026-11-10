//go:build darwin

package netpolicy

import (
	"net"
	"net/url"
	"os/exec"
	"strings"
)

func systemProxies() map[string]*url.URL {
	b, err := exec.Command("/usr/sbin/scutil", "--proxy").Output()
	if err != nil {
		return nil
	}
	fields := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(line, ":")
		if ok {
			fields[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	out := map[string]*url.URL{}
	for _, s := range []struct{ protocol, prefix string }{{"http", "HTTP"}, {"https", "HTTPS"}} {
		prefix := s.prefix
		scheme := "http"
		if fields[prefix+"Enable"] != "1" {
			prefix = "SOCKS"
			scheme = "socks5"
		}
		if fields[prefix+"Enable"] == "1" && fields[prefix+"Proxy"] != "" && fields[prefix+"Port"] != "" {
			out[s.protocol] = &url.URL{Scheme: scheme, Host: net.JoinHostPort(fields[prefix+"Proxy"], fields[prefix+"Port"])}
		}
	}
	return out
}
