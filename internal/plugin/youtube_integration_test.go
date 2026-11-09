package plugin

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mygo-music/internal/music"
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

// The real JS runtime passes track metadata, AbortController and UTF-8 lyrics across providers.
func TestYouTubeLyricsRuntime(t *testing.T) {
	dir := os.Getenv("YOUTUBE_PLUGIN_DIR")
	if dir == "" {
		t.Skip("set YOUTUBE_PLUGIN_DIR to the companion plugin checkout")
	}
	dir, _ = filepath.Abs(dir)
	for _, provider := range []string{"lrclib", "kugou", "plain"} {
		t.Run(provider, func(t *testing.T) {
			previous := http.DefaultTransport
			http.DefaultTransport = youtubeTransport(func(r *http.Request) (*http.Response, error) {
				var answer any
				status := 200
				switch r.URL.Host {
				case "music.youtube.com":
					if strings.Contains(r.URL.Path, "/next") {
						answer = map[string]any{"tabRenderer": map[string]any{"endpoint": map[string]any{"browseEndpoint": map[string]string{"browseId": "MPLYwords", "params": "lyrics-tab"}}}}
					} else {
						answer = map[string]any{"musicDescriptionShelfRenderer": map[string]any{"description": map[string]string{"simpleText": "Plain words"}}}
					}
				case "api.betterlyrics.org":
					status, answer = 401, map[string]any{}
				case "lrclib.net":
					if provider == "lrclib" {
						answer = map[string]any{"trackName": "Song", "artistName": "Singer", "duration": 180, "syncedLyrics": "[00:01.00]Test words"}
					} else {
						status, answer = 404, map[string]any{}
					}
				case "lyrics.kugou.com":
					if strings.Contains(r.URL.Path, "/search") {
						candidates := []any{}
						if provider == "kugou" {
							candidates = append(candidates, map[string]any{"id": "candidate", "accesskey": "test-only", "song": "Song", "singer": "Singer", "duration": 180000})
						}
						answer = map[string]any{"candidates": candidates}
					} else {
						answer = map[string]string{"content": base64.StdEncoding.EncodeToString([]byte("[00:01.00]测试歌词"))}
					}
				default:
					t.Errorf("unexpected lyrics host %s", r.URL.Host)
					status, answer = 404, map[string]any{}
				}
				b, _ := json.Marshal(answer)
				return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(b))), Request: r}, nil
			})
			defer func() { http.DefaultTransport = previous }()
			p, err := Load(dir, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			l, err := p.LyricsForTrack(music.Track{ID: "seed", Name: "Song", Artists: []music.Artist{{Name: "Singer"}}, Duration: 180000})
			if err != nil {
				t.Fatal(err)
			}
			switch provider {
			case "lrclib":
				if l.LRC != "[00:01.00]Test words" {
					t.Fatalf("%+v", l)
				}
			case "kugou":
				if l.LRC != "[00:01.00]测试歌词" {
					t.Fatalf("%+v", l)
				}
			case "plain":
				if l.Text != "Plain words" {
					t.Fatalf("%+v", l)
				}
			}
		})
	}
}

// Manual search and candidate downloads cross the actual JS bridge, including UTF-8 preview data.
func TestYouTubeLyricSearchRuntime(t *testing.T) {
	dir := os.Getenv("YOUTUBE_PLUGIN_DIR")
	if dir == "" {
		t.Skip("set YOUTUBE_PLUGIN_DIR")
	}
	dir, _ = filepath.Abs(dir)
	previous := http.DefaultTransport
	http.DefaultTransport = youtubeTransport(func(r *http.Request) (*http.Response, error) {
		var answer any
		if r.URL.Host == "lrclib.net" {
			answer = []any{map[string]any{"id": 1, "trackName": "Song", "artistName": "Singer", "duration": 180, "syncedLyrics": "[00:01.00]预览"}}
		} else if r.URL.Path == "/search" {
			answer = map[string]any{"candidates": []any{map[string]any{"id": "pick", "accesskey": "test-only", "song": "Song", "singer": "Singer", "duration": 180000}}}
		} else {
			answer = map[string]string{"content": base64.StdEncoding.EncodeToString([]byte("[00:01.00]选定歌词"))}
		}
		b, _ := json.Marshal(answer)
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(b))), Request: r}, nil
	})
	defer func() { http.DefaultTransport = previous }()
	p, err := Load(dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	track := music.Track{ID: "seed", Name: "Song", Artists: []music.Artist{{Name: "Singer"}}, Duration: 180000}
	rows, err := p.SearchLyrics(track, "Song", "Singer", "LRCLIB")
	if err != nil || len(rows) != 1 || rows[0].Data == nil || rows[0].Data.LRC != "[00:01.00]预览" {
		t.Fatalf("%+v %v", rows, err)
	}
	rows, err = p.SearchLyrics(track, "Song", "Singer", "KuGou")
	if err != nil || len(rows) != 1 {
		t.Fatalf("%+v %v", rows, err)
	}
	data, err := p.LyricCandidate(rows[0].ID)
	if err != nil || data.LRC != "[00:01.00]选定歌词" {
		t.Fatalf("%+v %v", data, err)
	}
}
