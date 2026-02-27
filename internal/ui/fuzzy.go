// internal/ui/fuzzy.go
package ui

import "strings"

// fuzzyMatch reports whether all runes of query appear in target as a
// subsequence (in order). Case-insensitive. Supports CJK/Chinese.
func fuzzyMatch(query, target string) bool {
	if query == "" {
		return true
	}
	qRunes := []rune(strings.ToLower(query))
	qi := 0
	for _, r := range strings.ToLower(target) {
		if r == qRunes[qi] {
			qi++
			if qi == len(qRunes) {
				return true
			}
		}
	}
	return false
}
