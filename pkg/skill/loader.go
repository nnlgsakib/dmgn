package skill

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed SKILL.md
var embeddedSkill []byte

const maxSkillSize = 50 * 1024 // 50KB limit per threat model T-09-03

// Load returns skill content from file or embedded fallback.
// It first tries to read ./skill/SKILL.md from the working directory.
// If the file is missing or unreadable, it returns the build-time embedded content.
func Load() ([]byte, error) {
	path := filepath.Clean(filepath.Join(".", "skill", "SKILL.md"))

	// Validate path stays within ./skill/ directory (T-09-01)
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean("skill")) {
		return nil, fmt.Errorf("invalid skill path: %s", path)
	}

	if data, err := os.ReadFile(path); err == nil {
		if len(data) > maxSkillSize {
			return data[:maxSkillSize], nil
		}
		return data, nil
	}

	// Fallback to embedded
	return embeddedSkill, nil
}
