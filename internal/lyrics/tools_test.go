package lyrics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"mygo-music/internal/music"
)

// Native readings cover Japanese kanji, Chinese pinyin and Korean without touching Latin text.
func TestSystemRomanization(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("native macOS readings")
	}
	got, err := Romanize([]string{"こんにちは世界", "世界", "Already Latin"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := Romanize([]string{"你好世界", "안녕하세요"})
	if err != nil || len(other) != 2 || !strings.Contains(other[0], "nǐ") || other[1] == "" {
		t.Fatalf("readings %v %v", other, err)
	}
	if len(got) != 3 || !strings.Contains(strings.ToLower(got[0]), "sekai") || !strings.Contains(strings.ToLower(got[1]), "sekai") || got[2] != "" {
		t.Fatalf("readings %+v", got)
	}
}

// Per-source selections, offsets and derived text survive a new store instance.
func TestStoreKeepsSongOverrides(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	track := music.Track{ID: "same", Source: "one"}
	r := Record{Data: ParseDocument("[00:01.00]Line"), Manual: true, OffsetMS: -500}
	if err := s.Save(track, r); err != nil {
		t.Fatal(err)
	}
	other := &Store{Dir: s.Dir}
	got := other.Read(track)
	if !got.Manual || got.OffsetMS != -500 || got.Data.LRC == "" {
		t.Fatal(got)
	}
	if other.Read(music.Track{ID: "same", Source: "two"}).Manual {
		t.Fatal("cross-source override")
	}
	if Export(got.Data, true, got.OffsetMS) != "[00:00.500]Line\n" {
		t.Fatal("offset export")
	}
}

// Out-of-order translation IDs are aligned; repeated or missing IDs are rejected.
func TestTranslationAlignment(t *testing.T) {
	bad := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-only" {
			t.Error("missing key")
		}
		text := `[{"id":1,"text":"二"},{"id":0,"text":"一"}]`
		if bad {
			text = `[{"id":0,"text":"一"},{"id":0,"text":"二"}]`
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": text}}}})
	}))
	defer srv.Close()
	c := TranslationConfig{Endpoint: srv.URL, Target: "zh-CN", Model: "test"}
	got, err := Translate(context.Background(), nil, c, "test-only", []string{"one", "two"})
	if err != nil || strings.Join(got, "/") != "一/二" {
		t.Fatalf("%+v %v", got, err)
	}
	bad = true
	if _, err := Translate(context.Background(), nil, c, "test-only", []string{"one", "two"}); err == nil {
		t.Fatal("accepted repeated IDs")
	}
}

// DeepL batches preserve repeated lines and reject missing text instead of silently shifting it.
func TestDeepLBatching(t *testing.T) {
	calls := 0
	empty := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "DeepL-Auth-Key test-only" {
			t.Error("missing auth")
		}
		var data struct {
			Text   []string `json:"text"`
			Target string   `json:"target_lang"`
		}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			t.Error(err)
		}
		if len(data.Text) > 40 || data.Target != "JA" {
			t.Error("batch/target", len(data.Text), data.Target)
		}
		out := make([]map[string]string, len(data.Text))
		for i, line := range data.Text {
			out[i] = map[string]string{"text": "訳" + line}
			if empty {
				out[i]["text"] = ""
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"translations": out})
	}))
	defer srv.Close()
	lines := make([]string, 81)
	for i := range lines {
		lines[i] = "same"
	}
	cfg := TranslationConfig{Provider: "deepl", Endpoint: srv.URL, Target: "ja"}
	got, err := Translate(context.Background(), nil, cfg, "test-only", lines)
	if err != nil || len(got) != 81 || calls != 3 || got[80] != "訳same" {
		t.Fatalf("%d %d %v", len(got), calls, err)
	}
	empty = true
	if _, err := Translate(context.Background(), nil, cfg, "test-only", []string{"line"}); err == nil {
		t.Fatal("empty translation accepted")
	}
}

// Redirects cannot forward the API key, and cancellation cannot return partial translations.
func TestTranslationRedirectAndCancellation(t *testing.T) {
	forwarded := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded = true }))
	defer target.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	cfg := TranslationConfig{Endpoint: srv.URL, Target: "en", Model: "test"}
	if _, err := Translate(context.Background(), nil, cfg, "test-only", []string{"一"}); err == nil || forwarded {
		t.Fatal("credential redirect")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := Translate(ctx, nil, cfg, "test-only", []string{"一"}); err == nil || got != nil {
		t.Fatal("cancel ignored")
	}
}
