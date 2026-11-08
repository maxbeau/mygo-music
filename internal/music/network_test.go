package music

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

// Turning off the system proxy changes the route even for clients that
// already hold the transport, while keeping an explicit environment proxy.
func TestFollowSystemProxyChangesNewRequests(t *testing.T) {
	was := systemProxy
	t.Cleanup(func() {
		SetFollowSystemProxy(true)
		systemProxy = was
	})
	req := must(http.NewRequest(http.MethodGet, "https://music.invalid/cover", nil))
	want := must(url.Parse("http://127.0.0.1:7890"))
	calls := 0
	systemProxy = func(r *http.Request) (*url.URL, error) {
		calls++
		return want, nil
	}
	proxy := Transport().Proxy
	SetFollowSystemProxy(true)
	if got, err := proxy(req); got != want || err != nil {
		t.Fatalf("following the system: %v, %v", got, err)
	}
	SetFollowSystemProxy(false)
	explicit, explicitErr := http.ProxyFromEnvironment(req)
	got, err := proxy(req)
	if calls != 1 || err != explicitErr || !reflect.DeepEqual(got, explicit) {
		t.Fatalf("with system proxy off: %v, %v; wanted environment route %v, %v (%d system lookups)", got, err, explicit, explicitErr, calls)
	}
	SetFollowSystemProxy(true)
	if got, err := proxy(req); got != want || err != nil || calls != 2 {
		t.Fatalf("following again: %v, %v (%d system lookups)", got, err, calls)
	}
}
