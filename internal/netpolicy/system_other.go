//go:build !darwin && !windows

package netpolicy

import "net/url"

func systemProxies() map[string]*url.URL { return nil }
