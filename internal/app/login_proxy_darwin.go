//go:build darwin

package app

import (
	"errors"
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
	"mygo-music/internal/netpolicy"
	"net/url"
	"sync"
)

var loginNetwork struct {
	once     sync.Once
	err      error
	endpoint func(string, string) uintptr
	http     func(uintptr, uintptr) uintptr
	socks    func(uintptr) uintptr
	tls      func() uintptr
	release  func(uintptr)
	exclude  func(uintptr, string)
}

func configureLoginProxy(win *mygo.Window) error {
	c := netpolicy.Current()
	if c.Mode == "" || c.Mode == "system" {
		return nil
	}
	loginNetwork.once.Do(func() {
		lib, err := purego.Dlopen("/System/Library/Frameworks/Network.framework/Network", purego.RTLD_LAZY|purego.RTLD_LOCAL)
		if err != nil {
			loginNetwork.err = err
			return
		}
		for _, name := range []string{"nw_proxy_config_create_http_connect", "nw_proxy_config_create_socksv5", "nw_proxy_config_add_excluded_domain"} {
			if _, err := purego.Dlsym(lib, name); err != nil {
				loginNetwork.err = err
				return
			}
		}
		purego.RegisterLibFunc(&loginNetwork.endpoint, lib, "nw_endpoint_create_host")
		purego.RegisterLibFunc(&loginNetwork.http, lib, "nw_proxy_config_create_http_connect")
		purego.RegisterLibFunc(&loginNetwork.socks, lib, "nw_proxy_config_create_socksv5")
		purego.RegisterLibFunc(&loginNetwork.tls, lib, "nw_tls_create_options")
		purego.RegisterLibFunc(&loginNetwork.release, lib, "nw_release")
		purego.RegisterLibFunc(&loginNetwork.exclude, lib, "nw_proxy_config_add_excluded_domain")
	})
	if loginNetwork.err != nil {
		return errors.New(T("Custom login proxy requires macOS 14 or later"))
	}
	raw := c.URL
	if c.Mode == "direct" {
		raw = "http://127.0.0.1:9"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	var result error
	mygo.RunOnMain(func() {
		var find func(objc.ID) objc.ID
		find = func(view objc.ID) objc.ID {
			if view == 0 {
				return 0
			}
			if objc.Send[bool](view, objc.RegisterName("isKindOfClass:"), objc.GetClass("WKWebView")) {
				return view
			}
			children := view.Send(objc.RegisterName("subviews"))
			for i, n := uint(0), objc.Send[uint](children, objc.RegisterName("count")); i < n; i++ {
				if found := find(children.Send(objc.RegisterName("objectAtIndex:"), i)); found != 0 {
					return found
				}
			}
			return 0
		}
		view := find(objc.ID(win.NativeHandle()).Send(objc.RegisterName("contentView")))
		if view == 0 {
			result = errors.New(T("Couldn't configure the login proxy"))
			return
		}
		store := view.Send(objc.RegisterName("configuration")).Send(objc.RegisterName("websiteDataStore"))
		if !objc.Send[bool](store, objc.RegisterName("respondsToSelector:"), objc.RegisterName("setProxyConfigurations:")) {
			result = errors.New(T("Custom login proxy requires macOS 14 or later"))
			return
		}
		endpoint := loginNetwork.endpoint(u.Hostname(), u.Port())
		defer loginNetwork.release(endpoint)
		var config uintptr
		if u.Scheme == "socks5" {
			config = loginNetwork.socks(endpoint)
		} else {
			var tls uintptr
			if u.Scheme == "https" {
				tls = loginNetwork.tls()
				defer loginNetwork.release(tls)
			}
			config = loginNetwork.http(endpoint, tls)
		}
		if config == 0 {
			result = errors.New(T("Couldn't configure the login proxy"))
			return
		}
		defer loginNetwork.release(config)
		for _, local := range []string{"localhost", "127.0.0.1", "::1"} {
			loginNetwork.exclude(config, local)
		}
		if c.Mode == "direct" {
			loginNetwork.exclude(config, "*")
		}
		array := objc.ID(objc.GetClass("NSArray")).Send(objc.RegisterName("arrayWithObject:"), config)
		store.Send(objc.RegisterName("setProxyConfigurations:"), array)
	})
	return result
}
