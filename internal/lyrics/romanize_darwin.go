//go:build darwin

package lyrics

import (
	"errors"
	"strings"
	"sync"
	"unicode"
	"unicode/utf16"

	"github.com/ebitengine/purego"
)

type cfRange struct{ Location, Length int64 }

var romanCF struct {
	once        sync.Once
	err         error
	create      func(uintptr, string, uint32) uintptr
	length      func(uintptr) int64
	text        func(uintptr, *byte, int64, uint32) bool
	release     func(uintptr)
	copyMutable func(uintptr, int64, uintptr) uintptr
	transform   func(uintptr, *cfRange, uintptr, bool) bool
	locale      func(uintptr, uintptr) uintptr
	tokenizer   func(uintptr, uintptr, cfRange, uint64, uintptr) uintptr
	advance     func(uintptr) uint64
	rangeOf     func(uintptr) cfRange
	attribute   func(uintptr, uint64) uintptr
}

const cfUTF8 = 0x08000100

func initRomanCF() {
	lib, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_LAZY|purego.RTLD_LOCAL)
	if err != nil {
		romanCF.err = err
		return
	}
	purego.RegisterLibFunc(&romanCF.create, lib, "CFStringCreateWithCString")
	purego.RegisterLibFunc(&romanCF.length, lib, "CFStringGetLength")
	purego.RegisterLibFunc(&romanCF.text, lib, "CFStringGetCString")
	purego.RegisterLibFunc(&romanCF.release, lib, "CFRelease")
	purego.RegisterLibFunc(&romanCF.copyMutable, lib, "CFStringCreateMutableCopy")
	purego.RegisterLibFunc(&romanCF.transform, lib, "CFStringTransform")
	purego.RegisterLibFunc(&romanCF.locale, lib, "CFLocaleCreate")
	purego.RegisterLibFunc(&romanCF.tokenizer, lib, "CFStringTokenizerCreate")
	purego.RegisterLibFunc(&romanCF.advance, lib, "CFStringTokenizerAdvanceToNextToken")
	purego.RegisterLibFunc(&romanCF.rangeOf, lib, "CFStringTokenizerGetCurrentTokenRange")
	purego.RegisterLibFunc(&romanCF.attribute, lib, "CFStringTokenizerCopyCurrentTokenAttribute")
}

func romanString(ref uintptr) string {
	if ref == 0 {
		return ""
	}
	b := make([]byte, romanCF.length(ref)*4+1)
	if !romanCF.text(ref, &b[0], int64(len(b)), cfUTF8) {
		return ""
	}
	return strings.TrimRight(string(b), "\x00")
}

// Romanize uses the system tokenizer's readings for Japanese, and ICU Latin
// transcription for Chinese/Korean, with no network or bundled dictionaries.
func Romanize(lines []string) ([]string, error) {
	romanCF.once.Do(initRomanCF)
	if romanCF.err != nil {
		return nil, errors.New("system romanization is unavailable")
	}
	localeID := romanCF.create(0, "ja", cfUTF8)
	defer romanCF.release(localeID)
	locale := romanCF.locale(0, localeID)
	defer romanCF.release(locale)
	transformID := romanCF.create(0, "Any-Latin", cfUTF8)
	defer romanCF.release(transformID)
	japanese := false
	for _, line := range lines {
		for _, r := range line {
			japanese = japanese || unicode.In(r, unicode.Hiragana, unicode.Katakana)
		}
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		var kana, cjk bool
		for _, r := range line {
			kana = kana || unicode.In(r, unicode.Hiragana, unicode.Katakana)
			cjk = cjk || unicode.In(r, unicode.Han, unicode.Hangul)
		}
		if !kana && !cjk {
			continue
		}
		ref := romanCF.create(0, line, cfUTF8)
		if japanese {
			tok := romanCF.tokenizer(0, ref, cfRange{0, romanCF.length(ref)}, 1<<16, locale)
			units := utf16.Encode([]rune(line))
			var parts []string
			for tok != 0 && romanCF.advance(tok) != 0 {
				r := romanCF.rangeOf(tok)
				attr := romanCF.attribute(tok, 1<<16)
				text := romanString(attr)
				if attr != 0 {
					romanCF.release(attr)
				}
				if text == "" && r.Location >= 0 && r.Location+r.Length <= int64(len(units)) {
					text = string(utf16.Decode(units[r.Location : r.Location+r.Length]))
				}
				if text != "" {
					parts = append(parts, text)
				}
			}
			if tok != 0 {
				romanCF.release(tok)
			}
			out[i] = strings.Join(parts, " ")
		} else {
			mutable := romanCF.copyMutable(0, 0, ref)
			if romanCF.transform(mutable, nil, transformID, false) {
				out[i] = romanString(mutable)
			}
			romanCF.release(mutable)
		}
		romanCF.release(ref)
	}
	return out, nil
}
