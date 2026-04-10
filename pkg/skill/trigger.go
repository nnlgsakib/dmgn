package skill

import "strings"

// DirectPatterns are exact phrases that trigger skill loading.
var DirectPatterns = []string{
	"init dmgn",
	"load dmgn",
	"dmgn context",
	"init dmgn context",
}

// FuzzyKeywords combined with "dmgn" trigger skill loading.
var FuzzyKeywords = []string{
	"context", "memory", "remember", "skill", "load", "init", "setup", "enable",
}

// Detect checks if input triggers skill loading.
// Returns true for direct pattern matches or fuzzy "dmgn" + keyword matches.
func Detect(input string) bool {
	lower := strings.ToLower(input)

	// Direct match
	for _, p := range DirectPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}

	// Fuzzy match: "dmgn" + keyword
	if strings.Contains(lower, "dmgn") {
		for _, kw := range FuzzyKeywords {
			if strings.Contains(lower, kw) {
				return true
			}
		}
	}

	return false
}
