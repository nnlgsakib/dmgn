# Phase 9: Skill Loader MCP Tool - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-10
**Phase:** 09-skill-loader
**Areas discussed:** Tool Interface, Fallback Strategy, Skill Discovery, Conversational Trigger

---

## Phase Selection

| Option | Description | Selected |
|--------|-------------|----------|
| Extend Phase 7 | Add to daemon MCP integration | |
| Extend Phase 6 | Add to existing MCP tools | |
| **New phase** | Create separate phase for skill management | ✓ |

**User's choice:** New phase (Phase 9)
**Notes:** User wanted skill-loading as a distinct feature with granular control

---

## Tool Interface

| Option | Description | Selected |
|--------|-------------|----------|
| load_skill | Loads skill content - most direct naming | |
| get_skill | Fetches skill - implies retrieval | |
| inject_skill | Injects into session - describes the action | ✓ (via conversational trigger) |

**User's choice:** Conversational trigger (not direct tool call)
**Notes:** User wants trigger like "hey init dmgn context" that automatically loads skill

---

## Fallback Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Error on missing | Return clear error - user must ensure skill exists | |
| **Inject inline if missing** | Accept skill content directly as fallback - more flexible | ✓ |

**User's choice:** Inject inline if missing
**Notes:** More resilient - always works even if file is missing

---

## Skill Discovery

| Option | Description | Selected |
|--------|-------------|----------|
| Named lookup | User provides name + tool loads from known location | |
| Path-based | User provides file path to skill file | |
| **Both** | Support name lookup AND path-based - most flexible | ✓ |

**User's choice:** Both
**Notes:** Maximum flexibility

---

## Conversational Trigger

| Option | Description | Selected |
|--------|-------------|----------|
| Direct match | Exact phrases like 'init dmgn', 'load dmgn', 'dmgn context' | |
| Fuzzy match | Any mention of 'dmgn' + skill-related words | |
| **Both** | Both direct and fuzzy triggers | ✓ |

**User's choice:** Both
**Notes:** Both trigger modes for maximum reliability

---

## What Gets Injected

| Option | Description | Selected |
|--------|-------------|----------|
| Full skill content | Inject complete SKILL.md into agent context window | ✓ |
| Summary + ref | Brief summary + reference to full skill file | |
| Tool definitions | Inject as tool definitions the agent can call | |

**User's choice:** do the best with best efficiency and effectiveness so that ai agent does and follows the correct instruct

**Notes:** Optimized injection - provide full skill content including all 7 tools, behavioral protocol, metadata conventions

---

## Skill Location

| Option | Description | Selected |
|--------|-------------|----------|
| **skill/SKILL.md** | Default location: ./skill/SKILL.md | ✓ |
| Custom path | Specify a custom skill directory path | |

**User's choice:** skill/SKILL.md
**Notes:** Default location confirmed

---

## Deferred Ideas

- Skill versioning support (future)
- Multiple skills support (future)
- Skill update notifications (future)