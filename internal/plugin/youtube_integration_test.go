package plugin

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type youtubeTransport func(*http.Request) (*http.Response, error)

func (f youtubeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// The actual YouTube plugin executes in MyGO's JS engine, including persisted
// station pages. HTTP is intercepted: this test never changes a live account.
func TestYouTubePluginRuntime(t *testing.T) {
	dir := os.Getenv("YOUTUBE_PLUGIN_DIR")
	if dir == "" {
		t.Skip("set YOUTUBE_PLUGIN_DIR to the companion plugin checkout")
	}
	dir, _ = filepath.Abs(dir)
	previous := http.DefaultTransport
	defer func() { http.DefaultTransport = previous }()
	var continuations []string
	http.DefaultTransport = youtubeTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "music.youtube.com" {
			t.Errorf("unexpected host %s", r.URL.Host)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		token, _ := body["continuation"].(string)
		continuations = append(continuations, token)
		video, next := "next", "page-two"
		if token != "" {
			video, next = "last", ""
		}
		answer := map[string]any{"playlistPanelRenderer": map[string]any{
			"contents": []any{
				map[string]any{"playlistPanelVideoRenderer": map[string]any{"videoId": "seed", "title": map[string]string{"simpleText": "Seed"}, "lengthText": map[string]string{"simpleText": "3:00"}}},
				map[string]any{"playlistPanelVideoRenderer": map[string]any{"videoId": video, "title": map[string]string{"simpleText": video}, "lengthText": map[string]string{"simpleText": "3:00"}}},
			},
			"continuations": []any{map[string]any{"nextRadioContinuationData": map[string]string{"continuation": next}}},
		}}
		b, _ := json.Marshal(answer)
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(b))), Request: r}, nil
	})
	data := t.TempDir()
	p, err := Load(dir, data)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	stations, err := p.Stations()
	if err != nil || len(stations) != 1 || !stations[0].Public {
		t.Fatalf("stations %+v %v", stations, err)
	}
	st, err := p.Radio("track", "seed")
	if err != nil {
		t.Fatal(err)
	}
	tracks, err := p.StationNext(st.ID)
	if err != nil || len(tracks) != 1 || tracks[0].ID != "next" || tracks[0].Source != "youtube-music" {
		t.Fatalf("tracks %+v %v", tracks, err)
	}
	p.Close()
	restored, err := Load(dir, data)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restored.Close)
	tracks, err = restored.StationNext(st.ID)
	if err != nil || len(tracks) != 1 || tracks[0].ID != "last" {
		t.Fatalf("restored %+v %v", tracks, err)
	}
	if len(continuations) != 2 || continuations[1] != "page-two" {
		t.Fatalf("requests %+v", continuations)
	}
}
