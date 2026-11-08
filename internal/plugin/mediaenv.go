package plugin

import (
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// mediaCommandEnv gives a playback helper the same proxy route as the app
// for the URL it is about to fetch. Only these known helpers read the
// settings below; other programs keep their inherited environment.
func mediaCommandEnv(cmd *exec.Cmd, platform string, proxyFor func(*http.Request) (*url.URL, error)) {
	name := strings.ToLower(filepath.Base(cmd.Path))
	name = strings.TrimSuffix(name, ".exe")
	target := mediaTarget(name, cmd.Args[1:])
	if target == nil {
		return
	}

	var extra []string
	switch name {
	case "ffmpeg":
		// FFmpeg uses lowercase http_proxy for both HTTP and HTTPS.
		if os.Getenv("http_proxy") == "" && !hasMediaOption(cmd.Args[1:], "-http_proxy") {
			if proxy, err := proxyFor(&http.Request{URL: target}); err == nil && proxy != nil && proxy.Scheme == "http" {
				extra = append(extra, "http_proxy="+proxy.String())
			}
		}
		// Standalone builds with OpenSSL may not know where macOS keeps
		// its PEM roots. This supplies the roots without turning off TLS verification.
		if platform == "darwin" && target.Scheme == "https" &&
			os.Getenv("SSL_CERT_FILE") == "" && os.Getenv("SSL_CERT_DIR") == "" &&
			!hasMediaOption(cmd.Args[1:], "-ca_file") && !hasMediaOption(cmd.Args[1:], "-cafile") {
			const roots = "/etc/ssl/cert.pem"
			if st, err := os.Stat(roots); err == nil && st.Mode().IsRegular() {
				extra = append(extra, "SSL_CERT_FILE="+roots)
			}
		}
	case "yt-dlp":
		key := target.Scheme + "_proxy"
		if os.Getenv(key) == "" && os.Getenv(strings.ToUpper(key)) == "" &&
			os.Getenv("all_proxy") == "" && os.Getenv("ALL_PROXY") == "" && !hasMediaOption(cmd.Args[1:], "--proxy") {
			if proxy, err := proxyFor(&http.Request{URL: target}); err == nil && proxy != nil {
				extra = append(extra, key+"="+proxy.String())
			}
		}
	}
	if len(extra) > 0 {
		cmd.Env = append(cmd.Environ(), extra...)
	}
}

// The YouTube Music plugin passes a URL as an FFmpeg input or the last URL
// to yt-dlp. Probe invocations and local files don't need network settings.
func mediaTarget(name string, args []string) *url.URL {
	switch name {
	case "ffmpeg":
		for i, a := range args {
			if a == "-i" && i+1 < len(args) {
				if u := remoteMediaURL(args[i+1]); u != nil {
					return u
				}
			}
		}
	case "yt-dlp":
		for i := len(args) - 1; i >= 0; i-- {
			if u := remoteMediaURL(args[i]); u != nil {
				return u
			}
		}
	}
	return nil
}

func remoteMediaURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil
	}
	return u
}

func hasMediaOption(args []string, option string) bool {
	for _, a := range args {
		if a == option || strings.HasPrefix(a, option+"=") {
			return true
		}
	}
	return false
}
