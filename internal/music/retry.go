package music

import (
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

// TemporaryError is a source's recoverable outage, never a reason to skip its entire queue.
type TemporaryError struct {
	Message string
	After   time.Duration
	Limited bool
}

func (e *TemporaryError) Error() string { return e.Message }
func RetryDelay(err error) (time.Duration, bool, bool) {
	var temporary *TemporaryError
	if errors.As(err, &temporary) {
		return temporary.After, temporary.Limited, true
	}
	var network net.Error
	if errors.As(err, &network) || errors.Is(err, io.ErrUnexpectedEOF) {
		return 0, false, true
	}
	return 0, false, false
}
func HTTPError(res *http.Response) error {
	if res.StatusCode == 429 || res.StatusCode == 503 || res.StatusCode == 502 || res.StatusCode == 504 {
		delay := time.Duration(0)
		if n, err := strconv.ParseFloat(res.Header.Get("Retry-After"), 64); err == nil {
			delay = time.Duration(n * float64(time.Second))
		} else if at, err := http.ParseTime(res.Header.Get("Retry-After")); err == nil {
			delay = time.Until(at)
		}
		return &TemporaryError{Message: "temporary service failure", After: max(0, min(delay, 15*time.Minute)), Limited: res.StatusCode == 429 || delay > 0}
	}
	return errors.New(res.Status)
}
