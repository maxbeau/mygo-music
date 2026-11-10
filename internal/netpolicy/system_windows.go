//go:build windows

package netpolicy

import (
	"golang.org/x/sys/windows/registry"
	"net/url"
	"strings"
)

func systemProxies() map[string]*url.URL {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	on, _, _ := k.GetIntegerValue("ProxyEnable")
	if on == 0 {
		return nil
	}
	raw, _, _ := k.GetStringValue("ProxyServer")
	out := map[string]*url.URL{}
	if !strings.Contains(raw, "=") {
		u, _ := url.Parse("http://" + raw)
		out["http"], out["https"] = u, u
		return out
	}
	for _, part := range strings.Split(raw, ";") {
		scheme, host, ok := strings.Cut(part, "=")
		if ok && (scheme == "http" || scheme == "https") {
			out[scheme], _ = url.Parse("http://" + host)
		}
	}
	return out
}
