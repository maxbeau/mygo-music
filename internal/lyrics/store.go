package lyrics

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mygo-music/internal/music"
)

// Record keeps the selected lyrics and per-song changes apart from audio playback.
type Record struct {
	Data          music.Lyrics        `json:"data"`
	Manual        bool                `json:"manual,omitempty"`
	OffsetMS      int64               `json:"offsetMs,omitempty"`
	Romanize      *bool               `json:"romanize,omitempty"`
	Romanized     []string            `json:"romanized,omitempty"`
	RomanizedHash string              `json:"romanizedHash,omitempty"`
	Translations  map[string][]string `json:"translations,omitempty"`
	Updated       time.Time           `json:"updated"`
}

// Store uses one atomic JSON file per source/song; no account data is stored here.
type Store struct {
	Dir string
	mu  sync.Mutex
}

func TrackKey(track music.Track) string { return track.Source + "\x00" + track.ID }
func hash(text string) string           { h := sha256.Sum256([]byte(text)); return hex.EncodeToString(h[:]) }
func (s *Store) path(track music.Track) string {
	return filepath.Join(s.Dir, hash(TrackKey(track))+".json")
}

func (s *Store) Read(track music.Track) Record {
	if s == nil || s.Dir == "" {
		return Record{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var r Record
	if b, err := os.ReadFile(s.path(track)); err == nil {
		json.Unmarshal(b, &r)
	}
	return r
}

func (s *Store) Save(track music.Track, r Record) error {
	if s == nil || s.Dir == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.Dir, ".lyrics-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0o600); err == nil {
		_, err = f.Write(b)
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, s.path(track))
}

func Usable(data *music.Lyrics) bool {
	return data != nil && (len(FromSource(data)) > 0 || strings.TrimSpace(data.Text) != "" || data.Instrumental)
}

// TextLines are the stable nonempty original lines used for translation and annotation.
func TextLines(data music.Lyrics) []string {
	var out []string
	if lines := FromSource(&data); len(lines) > 0 {
		for _, l := range lines {
			out = append(out, l.Text)
		}
		return out
	}
	for _, line := range strings.Split(data.Text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func ContentHash(data music.Lyrics) string { b, _ := json.Marshal(data); return hash(string(b)) }
func TranslationKey(data music.Lyrics, c TranslationConfig) string {
	return TranslationHashKey(ContentHash(data), c)
}

func ParseDocument(text string) music.Lyrics {
	text = strings.TrimSpace(strings.TrimPrefix(text, "\ufeff"))
	if len(Build(text, "")) > 0 {
		return music.Lyrics{LRC: text, Provider: "Manual"}
	}
	return music.Lyrics{Text: text, Provider: "Manual"}
}

func Export(data music.Lyrics, lrc bool, offsetMS int64) string {
	if !lrc {
		return strings.Join(TextLines(data), "\n")
	}
	var b strings.Builder
	for _, line := range FromSource(&data) {
		at := max(0, line.At.Milliseconds()+offsetMS)
		fmt.Fprintf(&b, "[%02d:%02d.%03d]%s\n", at/60000, at/1000%60, at%1000, line.Text)
	}
	return b.String()
}

// TranslationHashKey reuses the document hash while rendering frames.
func TranslationHashKey(content string, c TranslationConfig) string {
	return hash(strings.Join([]string{content, c.Target, c.Provider, c.Endpoint, c.Model}, "\x00"))
}
