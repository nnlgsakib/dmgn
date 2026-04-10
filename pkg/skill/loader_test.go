package skill

import "testing"

func TestLoadEmbeddedFallback(t *testing.T) {
	// Load() will try ./skill/SKILL.md first (likely missing in test dir),
	// then fall back to embedded content.
	data, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Load() returned empty content")
	}
	// Verify it contains expected skill content markers
	content := string(data)
	if !contains(content, "DMGN") {
		t.Error("expected skill content to mention DMGN")
	}
	if !contains(content, "add_memory") {
		t.Error("expected skill content to reference add_memory tool")
	}
}

func TestEmbeddedSkillNotEmpty(t *testing.T) {
	if len(embeddedSkill) == 0 {
		t.Fatal("embeddedSkill is empty — go:embed failed")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
