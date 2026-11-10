package diagnostics

import (
	"strings"
	"testing"
)

// Reports remove saved secrets, headers, signed URL parameters and the user's home directory.
func TestDiagnosticRedaction(t *testing.T) {
	Remember("unique-secret-value")
	got := Redact("unique-secret-value\nCookie: SID=private\nAuthorization: Bearer hidden\nhttps://example.com/audio?signature=hidden\n")
	for _, s := range []string{"unique-secret-value", "SID=private", "Bearer hidden", "signature=hidden"} {
		if strings.Contains(got, s) {
			t.Fatal("leaked", s)
		}
	}
	if !strings.Contains(got, "https://example.com/audio") {
		t.Fatal("unhelpful report", got)
	}
}
