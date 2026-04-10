# Phase 9: Skill Loader MCP Tool - Context

**Gathered:** 2026-04-10
**Status:** Ready for planning
**Source:** User request for conversational skill loading

<domain>
## Phase Boundary

This phase adds a **conversational skill-trigger system** to DMGN:
- When user says phrases like "hey init dmgn context", AI agent triggers skill loading
- Skill content is loaded from `skill/SKILL.md` or embedded in binary (build-time embed)
- If skill file is not available, inject content directly from embedded fallback
- Goal: AI agents automatically know how to use DMGN without manual setup

</domain>

<decisions>
## Implementation Decisions

### Conversational Trigger
- **D-01:** Trigger on direct match: "init dmgn", "load dmgn", "dmgn context", "init dmgn context"
- **D-02:** Also trigger on fuzzy match: any mention of "dmgn" + skill-related keywords
- **D-03:** Both trigger modes enabled for maximum reliability

### Skill Loading Mechanism
- **D-04:** Primary: Load from `skill/SKILL.md` file at runtime
- **D-05:** Fallback: If file not found, inject directly from embedded skill.md in binary
- **D-06:** Build-time embed skill content into binary (go:embed)
- **D-07:** Both lookup methods supported - named lookup and path-based

### Agent Context Injection
- **D-08:** Optimize for efficiency and effectiveness
- **D-09:** Provide full skill content to agent context window when triggered
- **D-10:** Include: all 7 tools complete reference, behavioral protocol, metadata conventions

### Skill Location
- **D-11:** Default skill file path: `./skill/SKILL.md`
- **D-12:** Embedded fallback path: `skill/SKILL.md` (via go:embed)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Skill Definition
- `skill/SKILL.md` — Full DMGN skill specification with all tools and behavioral protocol

### MCP Server
- `pkg/mcp/server.go` — Current MCP server implementation to extend with skill loader

### CLI
- `cmd/dmgn/main.go` — CLI entry point for any new commands

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- MCP server (`pkg/mcp/server.go`): Already handles stdio transport, JSON-RPC 2.0
- Existing 7 tools: add_memory, query_memory, get_context, link_memories, get_graph, delete_memory, get_status
- CLI structure: Command registration in main.go

### Established Patterns
- Tool definitions follow JSON-RPC 2.0 spec
- Go embed package available for build-time embedding

### Integration Points
- MCP server tool registration
- CLI command additions (if needed)
- Skill file path resolution

</code_context>

<specifics>
## Specific Ideas

### Conversational Trigger Examples
```
User: "hey init dmgn context"
AI → Triggers skill loading → Provides DMGN tools and behavioral protocol

User: "load dmgn"
AI → Triggers skill loading

User: "dmgn context"
AI → Triggers skill loading
```

### Injection Goal
- AI agent should know how to use DMGN after trigger
- All 7 tools available immediately
- Behavioral protocol followed (session start, during conversation, session end)
- Metadata conventions enforced

</specifics>

<deferred>
## Deferred Ideas

- Skill versioning support (future)
- Multiple skills support (future)
- Skill update notifications (future)

</deferred>

---

*Phase: 09-skill-loader*
*Context gathered: 2026-04-10*