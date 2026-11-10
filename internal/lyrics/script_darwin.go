//go:build darwin

package lyrics

import (
	"sync"
	"unicode"
)

var scriptMu sync.Mutex
var scriptCache = map[string]string{}

func DisplayScript(text, script string) string {
	if script != "simplified" && script != "traditional" {
		return text
	}
	han := false
	for _, r := range text {
		if unicode.In(r, unicode.Han) {
			han = true
			break
		}
	}
	if !han {
		return text
	}
	key := script + "\x00" + text
	scriptMu.Lock()
	defer scriptMu.Unlock()
	if known, ok := scriptCache[key]; ok {
		return known
	}
	romanCF.once.Do(initRomanCF)
	if romanCF.err != nil {
		return text
	}
	transform := "Traditional-Simplified"
	if script == "traditional" {
		transform = "Simplified-Traditional"
	}
	name := romanCF.create(0, transform, cfUTF8)
	defer romanCF.release(name)
	ref := romanCF.create(0, text, cfUTF8)
	defer romanCF.release(ref)
	mutable := romanCF.copyMutable(0, 0, ref)
	defer romanCF.release(mutable)
	result := text
	if romanCF.transform(mutable, nil, name, false) {
		result = romanString(mutable)
	}
	if len(scriptCache) > 2048 {
		scriptCache = map[string]string{}
	}
	scriptCache[key] = result
	return result
}
