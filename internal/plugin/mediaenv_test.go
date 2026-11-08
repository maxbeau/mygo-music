package plugin

import (
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"testing"
)

// The catalog can use a proxy while a subprocess starts without it; feed
// the route into each media helper without changing the parent's settings.
func TestMediaCommandUsesProxyForRemoteInput(t *testing.T) {
	t.Setenv("http_proxy", "")
	t.Setenv("https_proxy", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("all_proxy", "")
	t.Setenv("ALL_PROXY", "")
	t.Setenv("SSL_CERT_FILE", "")
	t.Setenv("SSL_CERT_DIR", "")

	const proxyAddress = "127.0.0.1:7890"
	cases := []struct {
		name, command, target, variable string
		args                            []string
	}{
		{"ffmpeg", "/opt/homebrew/bin/ffmpeg", "https://audio.example.test/song", "http_proxy",
			[]string{"-i", "cover.jpg", "-i", "https://audio.example.test/song", "http://127.0.0.1:20000/"}},
		{"yt-dlp", "yt-dlp.exe", "https://www.example.test/watch?v=123", "https_proxy",
			[]string{"--referer", "https://example.test/", "https://www.example.test/watch?v=123"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := exec.Command(c.command, c.args...)
			mediaCommandEnv(cmd, "linux", func(r *http.Request) (*url.URL, error) {
				if r.URL.String() != c.target {
					t.Fatalf("proxy looked up for %s, want %s", r.URL, c.target)
				}
				return &url.URL{Scheme: "http", Host: proxyAddress}, nil
			})
			if !slices.Contains(cmd.Env, c.variable+"=http://"+proxyAddress) {
				t.Fatalf("the child didn't get %s", c.variable)
			}
			if os.Getenv(c.variable) != "" {
				t.Fatal("the app's environment changed")
			}
		})
	}
}

// FFmpeg's local work and other commands must not gain a proxy, and the
// app should not replace a proxy supplied explicitly to a media helper.
func TestMediaCommandKeepsExplicitAndLocalBehavior(t *testing.T) {
	t.Setenv("http_proxy", "")
	t.Setenv("https_proxy", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("all_proxy", "")
	t.Setenv("ALL_PROXY", "")
	for _, cmd := range []*exec.Cmd{
		exec.Command("ffmpeg", "-version"),
		exec.Command("ffmpeg", "-i", "local.mp3", "http://127.0.0.1:20000/"),
		exec.Command("ffmpeg", "-http_proxy", "http://proxy.example.test", "-i", "https://audio.example.test/song"),
		exec.Command("yt-dlp", "--proxy=", "https://example.test/video"),
		exec.Command("other", "https://example.test/video"),
	} {
		mediaCommandEnv(cmd, "linux", func(*http.Request) (*url.URL, error) {
			t.Fatalf("%v unexpectedly asked for a proxy", cmd.Args)
			return nil, nil
		})
		if cmd.Env != nil {
			t.Errorf("%v got a new environment", cmd.Args)
		}
	}
}

// On macOS the standalone OpenSSL build needs a trust bundle for HTTPS,
// but an explicitly configured certificate bundle must be left alone.
func TestMediaCommandMacCertificates(t *testing.T) {
	const roots = "/etc/ssl/cert.pem"
	if st, err := os.Stat(roots); err != nil || !st.Mode().IsRegular() {
		t.Skip("macOS PEM bundle is not available")
	}
	t.Setenv("http_proxy", "")
	t.Setenv("SSL_CERT_FILE", "")
	t.Setenv("SSL_CERT_DIR", "")
	noProxy := func(*http.Request) (*url.URL, error) { return nil, nil }
	cmd := exec.Command("ffmpeg", "-i", "https://audio.example.test/song")
	mediaCommandEnv(cmd, "darwin", noProxy)
	if !slices.Contains(cmd.Env, "SSL_CERT_FILE="+roots) {
		t.Fatal("FFmpeg didn't get the macOS PEM bundle")
	}

	t.Setenv("SSL_CERT_FILE", "/custom/ca.pem")
	cmd = exec.Command("ffmpeg", "-i", "https://audio.example.test/song")
	mediaCommandEnv(cmd, "darwin", noProxy)
	if cmd.Env != nil {
		t.Fatal("FFmpeg's certificate settings were changed")
	}
}
