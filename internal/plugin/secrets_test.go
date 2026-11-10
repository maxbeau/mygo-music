package plugin

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/egoist/mygo"
	"mygo-music/internal/music"
	"os"
	"path/filepath"
	"testing"
)

// Large credentials are encrypted, scoped by plugin, and survive a new runtime without plaintext on disk.
func TestCredentialEncryptionAndIsolation(t *testing.T) {
	oldGet, oldSet := secretKeyGet, secretKeySet
	defer func() { secretKeyGet, secretKeySet = oldGet, oldSet }()
	keys := map[string][]byte{}
	secretKeyGet = func(k string) ([]byte, error) {
		if v := keys[k]; v != nil {
			return v, nil
		}
		return nil, mygo.ErrSecretNotFound
	}
	secretKeySet = func(k string, v []byte) error { keys[k] = bytes.Clone(v); return nil }
	p := &Plugin{manifest: Manifest{Info: music.Info{ID: "one"}}, data: t.TempDir()}
	value := bytes.Repeat([]byte("sensitive-cookie"), 500)
	raw, _ := json.Marshal(string(value))
	if err := p.writeCredential("cookie", raw); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(p.credentialPath("cookie"))
	if err != nil || bytes.Contains(saved, value[:16]) {
		t.Fatal("plaintext stored", err)
	}
	fresh := &Plugin{manifest: p.manifest, data: p.data}
	got, err := fresh.readCredential("cookie")
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatal("restart lost credentials", err)
	}
	other := &Plugin{manifest: Manifest{Info: music.Info{ID: "two"}}, data: p.data}
	if _, err := other.readCredential("cookie"); err == nil {
		t.Fatal("another plugin unlocked credentials")
	}
	saved[len(saved)-1] ^= 1
	os.WriteFile(p.credentialPath("cookie"), saved, 0600)
	if _, err := p.readCredential("cookie"); err == nil {
		t.Fatal("tampering was accepted")
	}
	secretKeyGet = func(string) ([]byte, error) { return nil, errors.New("locked") }
	if err := p.writeCredential("other", raw); err == nil {
		t.Fatal("saved while credential store locked")
	}
	if _, err := os.Stat(filepath.Join(p.data, "storage.json")); !os.IsNotExist(err) {
		t.Fatal("plaintext fallback created")
	}
}

// Retry metadata survives the real JavaScript bridge without exposing provider-specific errors to queue logic.
func TestTemporaryErrorBridge(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"id":"test"}`), 0600)
	os.WriteFile(filepath.Join(dir, "index.js"), []byte(`export function stream(){const e=new Error("busy");e.code="rateLimited";e.retryAfter=45000;throw e}`), 0600)
	p, err := Load(dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	_, err = p.Stream("song", "high")
	delay, limited, ok := music.RetryDelay(err)
	if !ok || !limited || delay.Milliseconds() != 45000 {
		t.Fatal(delay, limited, ok, err)
	}
}
