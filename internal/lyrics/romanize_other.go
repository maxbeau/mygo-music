//go:build !darwin

package lyrics

import "errors"

func Romanize([]string) ([]string, error) {
	return nil, errors.New("system romanization is available on macOS")
}
