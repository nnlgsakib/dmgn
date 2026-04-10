package skill

import "testing"

func TestDetect(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// Direct pattern matches
		{"direct init dmgn", "hey init dmgn", true},
		{"direct load dmgn", "load dmgn now", true},
		{"direct dmgn context", "dmgn context please", true},
		{"direct init dmgn context", "init dmgn context", true},

		// Fuzzy matches: "dmgn" + keyword
		{"fuzzy setup", "dmgn setup", true},
		{"fuzzy memory", "dmgn memory", true},
		{"fuzzy remember", "can dmgn remember this", true},
		{"fuzzy skill", "dmgn skill", true},
		{"fuzzy enable", "enable dmgn", true},

		// Case insensitive
		{"uppercase DMGN", "DMGN is great", false},
		{"uppercase with keyword", "DMGN context", true},
		{"mixed case", "Init DMGN", true},

		// No match
		{"no match random", "hello world", false},
		{"no match partial", "dog mn", false},
		{"no match similar", "my dog named spot", false},
		{"dmgn alone no keyword", "what is dmgn", false},
		{"empty input", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Detect(tt.input)
			if got != tt.want {
				t.Errorf("Detect(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
