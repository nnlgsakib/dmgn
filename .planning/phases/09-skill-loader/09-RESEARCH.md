# Phase 9: Skill Loader MCP Tool - Research

**Researched:** 2026-04-10  
**Phase:** 09-skill-loader  
**Goal:** Add conversational skill-trigger system for DMGN

---

## Domain Analysis

### What This Feature Does

The skill loader enables AI agents to automatically discover and load DMGN's capabilities when users mention "dmgn" in conversation. Instead of requiring manual configuration, the AI agent detects trigger phrases and injects the full skill content (all 7 tools reference, behavioral protocol) into its context window.

### Trigger Detection Strategies

**Direct Match Patterns (D-01):**
- `"init dmgn"` — explicit initialization request
- `"load dmgn"` — load the skill
- `"dmgn context"` — request context/initialization
- `"init dmgn context"` — full form

**Fuzzy Match Patterns (D-02):**
- Any message containing `"dmgn"` + skill-related keywords:
  - `context`, `memory`, `remember`, `skill`, `load`, `init`, `setup`, `enable`
- Case-insensitive matching
- Word boundary awareness (avoid false positives in other words containing "dmgn")

### Skill Loading Mechanisms

**Primary: Runtime File Load (D-04)**
- Path: `./skill/SKILL.md` relative to working directory
- Use: `os.ReadFile()` or similar
- Behavior: On trigger, read file contents and provide to agent

**Fallback: Embedded Content (D-05, D-06)**
- Use Go's `embed` package to embed `skill/SKILL.md` at compile time
- If runtime file not found, use embedded content
- File path in embed: `skill/SKILL.md` (matches file structure)

---

## Technical Implementation

### 1. Trigger Detection Package

Create `pkg/skill/trigger.go`:

```go
package skill

// TriggerPatterns defines what activates skill loading
var DirectPatterns = []string{
    "init dmgn",
    "load dmgn", 
    "dmgn context",
    "init dmgn context",
}

// Fuzzy keywords combined with "dmgn"
var FuzzyKeywords = []string{
    "context", "memory", "remember", "skill", "load", "init", "setup", "enable",
}

// Detect checks if input triggers skill loading
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
```

### 2. Skill Loader Package

Create `pkg/skill/loader.go`:

```go
package skill

import (
    _ "embed"
    "os"
    "path/filepath"
)

//go:embed SKILL.md
var embeddedSkill []byte

// Load returns skill content from file or embedded fallback
func Load() ([]byte, error) {
    // Try file first
    path := filepath.Join(".", "skill", "SKILL.md")
    if data, err := os.ReadFile(path); err == nil {
        return data, nil
    }
    
    // Fallback to embedded
    return embeddedSkill, nil
}
```

### 3. Integration with MCP Server

Extend `pkg/mcp/server.go` to add skill loader tool:

```go
// Add this tool registration in newServer()
mcp.AddTool(server, &mcp.Tool{
    Name:        "load_skill",
    Description: "Load DMGN skill content for agent context. Call this when user mentions DMGN.",
}, s.handleLoadSkill)

func (s *MCPServer) handleLoadSkill(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, string, error) {
    content, err := skill.Load()
    if err != nil {
        return nil, "", err
    }
    // Return as formatted text for agent consumption
    return nil, string(content), nil
}
```

### 4. Tool Definitions for Trigger Detection

Option A: Add a dedicated `load_skill` tool (simpler, explicit)
Option B: Have AI agent detect triggers and call tool automatically (more automatic)

**Recommendation:** Option A with documentation. The skill content includes instructions telling the agent to call `load_skill` when triggers are detected.

---

## Validation Architecture

### Dimension 1: Trigger Detection

| Test Case | Input | Expected |
|-----------|-------|----------|
| Direct match | "hey init dmgn" | true |
| Direct match | "load dmgn now" | true |
| Direct match | "dmgn context please" | true |
| Fuzzy match | "dmgn setup" | true |
| Fuzzy match | "dmgn memory" | true |
| No match | "dog mn" | false |
| No match | "DMGN is great" | true (uppercase handled) |
| No match | "my dog named" | false |

### Dimension 2: Skill Loading

| Test Case | Scenario | Expected |
|-----------|----------|----------|
| File exists | skill/SKILL.md present | File content returned |
| File missing | skill/SKILL.md absent | Embedded content returned |
| Empty file | skill/SKILL.md empty | Empty (not error) |

### Dimension 3: Tool Integration

| Test Case | Expected |
|-----------|----------|
| MCP server starts | load_skill tool registered |
| Call load_skill | Returns skill content |
| Invalid call | Error returned |

---

## Security Considerations

- **Path traversal:** Validate skill path doesn't escape directory
- **File size limit:** Enforce max skill content size (prevent DoS)
- **No execution:** Skill content is data, never executed

---

## Dependencies

| Dependency | Purpose | Version |
|------------|---------|---------|
| `github.com/modelcontextprotocol/go-sdk/mcp` | MCP server | from go.mod |
| Standard library | strings, os, embed | Go 1.21+ |

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| False positives (wrong triggers) | Medium | Low | Refine keyword list, add negative patterns |
| Embed file not found at build | Low | High | Test embed in CI, fallback path validation |
| Skill content too large | Low | Medium | Truncate at 50KB, warn if truncated |

---

## Alternatives Considered

### Alternative 1: MCP Prompts
Use MCP's prompt feature instead of tool. Rejected: More complex, less compatible with all agents.

### Alternative 2: Automatic Context Injection
Instead of tool, have MCP server push skill on trigger. Rejected: MCP stdio is request-response, push not reliable.

### Alternative 3: Separate Skill Server
Run skill loader as separate service. Rejected: Overkill for single-file lookup.

**Chosen Approach:** Tool-based with embedded fallback. Simple, reliable, works with all MCP clients.

---

## Next Steps for Planning

1. Create `pkg/skill/` package with trigger detection and loader
2. Add `load_skill` tool to MCP server
3. Create embedded skill file with go:embed
4. Update SKILL.md with trigger detection instructions
5. Test trigger detection and loading independently

---

*Research complete: 2026-04-10*
