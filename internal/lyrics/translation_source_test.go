package lyrics

import (
	"mygo-music/internal/music"
	"runtime"
	"testing"
)

// A source translation keeps word timings and cannot attach a different recording's text.
func TestSourceTranslationKeepsTiming(t *testing.T) {
	base := music.Lyrics{Lines: []music.LyricLine{{Text: "Hello world", Start: 1000, End: 3000, Words: []music.LyricWord{{Text: "Hello ", Start: 1000, End: 2000}, {Text: "world", Start: 2000, End: 3000}}}, {Text: "Good night", Start: 4000, End: 6000}}}
	other := music.Lyrics{LRC: "[00:01.10]Hello world\n[00:04.10]Good night", Translation: "[00:01.10]Translation one\n[00:04.10]Translation two"}
	merged, ok := MergeSourceTranslation(base, other)
	if !ok || merged.Lines[0].Words[1].Start != 2000 || merged.Lines[0].Translation != "Translation one" || !HasSourceTranslation(merged) {
		t.Fatal(merged)
	}
	other.LRC = "[00:01.10]Different recording\n[00:04.10]Other text"
	if _, ok := MergeSourceTranslation(base, other); ok {
		t.Fatal("different lyric text aligned")
	}
}

// Display script conversion changes only the shown text, leaving its source untouched.
func TestChineseDisplayScript(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("native Chinese conversion")
	}
	if got := DisplayScript("音樂與愛", "simplified"); got != "音乐与爱" {
		t.Fatal(got)
	}
	if got := DisplayScript("音乐与爱", "traditional"); got != "音樂與愛" {
		t.Fatal(got)
	}
}
