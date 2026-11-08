package music

import (
	"net/http"
	"net/url"
	"sync/atomic"

	"github.com/egoist/mygo/netproxy"
)

// skipSystemProxy is off until the listener turns off the system's proxy.
// Network requests and playback helpers read it away from the UI thread.
var skipSystemProxy atomic.Bool

var systemProxy = netproxy.Proxy

// SetFollowSystemProxy applies to new requests and playback helpers. An
// explicit proxy in the app's environment is still used when this is off.
func SetFollowSystemProxy(follow bool) {
	disabled := !follow
	if skipSystemProxy.Swap(disabled) != disabled {
		transport.CloseIdleConnections()
	}
}

// ProxyFor finds the route for a music request.
func ProxyFor(req *http.Request) (*url.URL, error) {
	if skipSystemProxy.Load() {
		return http.ProxyFromEnvironment(req)
	}
	return systemProxy(req)
}

var transport = func() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = ProxyFor
	return t
}()

// Transport is shared by clients for catalogs, covers and audio.
func Transport() *http.Transport { return transport }
