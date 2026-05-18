---
name: brainstorming
description: Use before any creative work — features, components, behavior changes. Explores intent, requirements, and design before implementation.
---

# Brainstorming Into Design

Hard gate: **Do NOT write code until design is approved.**

## Checklist (in order)
1. Explore project context — check files, docs, recent commits
2. Ask clarifying questions — one at a time, understand constraints
3. Propose 2-3 approaches — with trade-offs, recommend one
4. Present design — architecture, components, data flow, error handling
5. Write spec — save to `docs/superpowers/specs/YYYY-MM-DD-topic-design.md`
6. Self-review — check for placeholders, contradictions, ambiguity
7. User reviews spec — get approval before implementation

## Approach Selection
- List 2-3 viable approaches
- For each: effort, risk, trade-offs
- Recommend one with clear reasoning
- Let user choose

## Spec Format
```markdown
## Summary
## Motivation
## Architecture
## Components
## Data Flow
## Error Handling
## Migration
## Tasks
## Verification
```

## Transition
After spec approved → invoke `writing-plans` skill.
After plan written → invoke `subagent-driven-development` skill.
