//go:build !darwin

package app

import "github.com/egoist/mygo"

// The login web engine retains its operating system proxy. The network settings page exposes this limitation.
func configureLoginProxy(win *mygo.Window) error { return nil }
