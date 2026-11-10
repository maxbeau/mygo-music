package lyrics

import (
	"mygo-music/internal/music"
	"strings"
	"unicode"
)

func HasSourceTranslation(data music.Lyrics) bool {
	for _, line := range FromSource(&data) {
		if strings.TrimSpace(line.Translation) != "" {
			return true
		}
	}
	return false
}
func textKey(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, text)
}

// MergeSourceTranslation retains the original document's word timings and accepts only aligned original text.
func MergeSourceTranslation(base, other music.Lyrics) (music.Lyrics, bool) {
	current := FromSource(&base)
	translated := FromSource(&other)
	if len(current) == 0 || len(translated) == 0 {
		return base, false
	}
	matched := 0
	for i := range current {
		best := -1
		for j, line := range translated {
			if line.Translation == "" || textKey(line.Text) != textKey(current[i].Text) {
				continue
			}
			delta := line.At - current[i].At
			if delta < 0 {
				delta = -delta
			}
			if delta.Milliseconds() > 1500 {
				continue
			}
			if best < 0 {
				best = j
			} else {
				a := translated[best].At - current[i].At
				if a < 0 {
					a = -a
				}
				if delta < a {
					best = j
				}
			}
		}
		if best >= 0 {
			current[i].Translation = translated[best].Translation
			matched++
		}
	}
	if matched == 0 || matched*10 < len(current)*7 {
		return base, false
	}
	base.Lines = make([]music.LyricLine, len(current))
	for i, l := range current {
		words := make([]music.LyricWord, len(l.Words))
		for j, w := range l.Words {
			words[j] = music.LyricWord{Text: w.Text, Start: w.Start.Milliseconds(), End: w.End.Milliseconds()}
		}
		base.Lines[i] = music.LyricLine{Text: l.Text, Start: l.At.Milliseconds(), End: l.End.Milliseconds(), Words: words, Translation: l.Translation}
	}
	return base, true
}
