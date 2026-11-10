package plugin

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MoeForge/moejs"
	"github.com/egoist/mygo"
	"mygo-music/internal/diagnostics"
)

// A per-plugin system key encrypts arbitrary-size credentials; cookies can exceed the keychain entry limit.
var secretKeyGet = func(key string) ([]byte, error) { return mygo.Secrets.Get(key) }
var secretKeySet = func(key string, value []byte) error { return mygo.Secrets.Set(key, value) }

func (p *Plugin) credentialPath(key string) string {
	h := sha256.Sum256([]byte(key))
	return filepath.Join(p.data, "credentials", hex.EncodeToString(h[:])+".enc")
}
func (p *Plugin) credentialCipher(create bool) (cipher.AEAD, error) {
	name := "plugin-credentials-" + p.manifest.ID
	key, err := secretKeyGet(name)
	if errors.Is(err, mygo.ErrSecretNotFound) && create {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err == nil {
			err = secretKeySet(name, key)
		}
	}
	if err != nil {
		return nil, errors.New("system credential store is unavailable")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("invalid credential encryption key")
	}
	return cipher.NewGCM(block)
}
func (p *Plugin) readCredential(key string) (json.RawMessage, error) {
	// The runtime is serial, and p.mu also covers migration or a future second caller.
	p.mu.Lock()
	defer p.mu.Unlock()
	b, err := os.ReadFile(p.credentialPath(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("could not read saved credentials")
	}
	gcm, err := p.credentialCipher(false)
	if err != nil {
		return nil, err
	}
	if len(b) < gcm.NonceSize() {
		return nil, errors.New("invalid saved credentials")
	}
	value, err := gcm.Open(nil, b[:gcm.NonceSize()], b[gcm.NonceSize():], []byte(p.manifest.ID+"\x00"+key))
	if err != nil {
		return nil, errors.New("could not unlock saved credentials")
	}
	rememberCredential(value)
	return value, nil
}
func rememberCredential(raw []byte) {
	var value string
	if json.Unmarshal(raw, &value) == nil {
		diagnostics.Remember(value)
	}
}
func (p *Plugin) writeCredential(key string, raw []byte) error {
	if len(raw) > 128<<10 {
		return errors.New("credential is too large")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	gcm, err := p.credentialCipher(true)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	b := gcm.Seal(nonce, nonce, raw, []byte(p.manifest.ID+"\x00"+key))
	path := p.credentialPath(key)
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return errors.New("could not save credentials")
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".credential-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	err = f.Chmod(0600)
	if err == nil {
		_, err = f.Write(b)
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		return errors.New("could not save credentials")
	}
	rememberCredential(raw)
	return nil
}
func (p *Plugin) secretHost(v *vm) map[string]any {
	key := func(args []moejs.Value) (string, error) {
		name := moejs.Arg(args, 0).String()
		if name == "" || len(name) > 256 {
			return "", fmt.Errorf("invalid credential name")
		}
		return name, nil
	}
	return map[string]any{
		"get": moejs.NativeFunc(func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			name, err := key(args)
			if err != nil {
				return moejs.Undefined(), err
			}
			raw, err := p.readCredential(name)
			if err != nil || raw == nil {
				return moejs.Undefined(), err
			}
			return v.rt.ParseJSON(raw)
		}),
		"set": moejs.NativeFunc(func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			name, err := key(args)
			if err != nil {
				return moejs.Undefined(), err
			}
			raw, err := v.rt.AppendJSON(nil, moejs.Arg(args, 1))
			if err == nil {
				err = p.writeCredential(name, raw)
			}
			return moejs.Undefined(), err
		}),
		"remove": moejs.NativeFunc(func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			name, err := key(args)
			if err != nil {
				return moejs.Undefined(), err
			}
			err = os.Remove(p.credentialPath(name))
			if errors.Is(err, os.ErrNotExist) {
				err = nil
			}
			return moejs.Undefined(), err
		}),
	}
}
