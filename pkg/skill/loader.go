package skill

import (
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed SKILL.md
var embeddedSkill []byte

const maxSkillSize = 50 * 1024 // 50KB limit per threat model T-09-03

// SkillSearchPaths lists conventional locations where AI coding agents
// store skill files, searched in priority order. The first file found wins.
var SkillSearchPaths = []string{
	filepath.Join(".opencode", "skills", "dmgn-skill", "SKILL.md"),
	filepath.Join(".windsurf", "skills", "dmgn-skill", "SKILL.md"),
	filepath.Join(".cursor", "skills", "dmgn-skill", "SKILL.md"),
	filepath.Join(".cline", "skills", "dmgn-skill", "SKILL.md"),
	filepath.Join("skill", "SKILL.md"),
}

// Load searches conventional skill system paths for the DMGN skill file.
// AI agents call this via the load_skill MCP tool to inject DMGN capabilities
// into their context. If no skill file is found on disk, the build-time
// embedded copy is returned directly so the agent always gets the skill content.
func Load() ([]byte, error) {
	// Search skill system paths in priority order
	for _, p := range SkillSearchPaths {
		clean := filepath.Clean(p)
		if data, err := os.ReadFile(clean); err == nil {
			if len(data) > maxSkillSize {
				return data[:maxSkillSize], nil
			}
			return data, nil
		}
	}

	// No skill file found on disk — inject embedded skill directly
	return embeddedSkill, nil
}
