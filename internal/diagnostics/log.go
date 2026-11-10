// Package diagnostics keeps a bounded, redacted log for user-requested reports.
package diagnostics

import (
	"bytes"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var mu sync.Mutex
var secrets []string
var path string
var urls = regexp.MustCompile(`https?://[^\s"'<>]+`)
var credentials = regexp.MustCompile(`(?i)(authorization|cookie|set-cookie|api[-_]?key|access[-_]?token|refresh[-_]?token|password|SAPISIDHASH)\s*[:=]\s*[^\r\n]+`)

func Remember(value string) {
	mu.Lock()
	defer mu.Unlock()
	if len(value) >= 8 {
		secrets = append(secrets, value)
	}
	if len(secrets) > 256 {
		secrets = secrets[len(secrets)-256:]
	}
}
func Redact(text string) string {
	mu.Lock()
	ss := append([]string(nil), secrets...)
	mu.Unlock()
	for _, secret := range ss {
		text = strings.ReplaceAll(text, secret, "[redacted]")
	}
	text = credentials.ReplaceAllString(text, "$1: [redacted]")
	text = urls.ReplaceAllStringFunc(text, func(s string) string {
		u, err := url.Parse(s)
		if err != nil {
			return "[redacted URL]"
		}
		u.User = nil
		if u.RawQuery != "" || u.Fragment != "" {
			u.RawQuery = "redacted"
			u.Fragment = ""
		}
		return u.String()
	})
	if home, err := os.UserHomeDir(); err == nil {
		text = strings.ReplaceAll(text, home, "~")
	}
	return text
}

type writer struct {
	mu      sync.Mutex
	file    *os.File
	size    int64
	partial []byte
}

func (w *writer) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.partial = append(w.partial, b...)
	for {
		at := bytes.IndexByte(w.partial, '\n')
		if at < 0 {
			break
		}
		line := []byte(Redact(string(w.partial[:at+1])))
		w.partial = w.partial[at+1:]
		if w.size+int64(len(line)) > 1<<20 {
			if w.file != nil {
				w.file.Close()
			}
			os.Remove(path + ".1")
			os.Rename(path, path+".1")
			w.file, _ = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
			w.size = 0
		}
		os.Stderr.Write(line)
		if w.file != nil {
			n, _ := w.file.Write(line)
			w.size += int64(n)
		}
	}
	if len(w.partial) > 64<<10 {
		w.partial = nil
	}
	return len(b), nil
}
func Start(dir string) (io.Closer, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path = filepath.Join(dir, "mygo.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	st, _ := f.Stat()
	w := &writer{file: f, size: st.Size()}
	log.SetOutput(w)
	return w, nil
}
func Tail() string {
	if path == "" {
		return ""
	}
	b, _ := os.ReadFile(path)
	if len(b) > 64<<10 {
		b = b[len(b)-(64<<10):]
	}
	return Redact(string(b))
}

func (w *writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}
