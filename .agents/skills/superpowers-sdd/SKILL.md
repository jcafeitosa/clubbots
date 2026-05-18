---
name: subagent-driven-development
description: Use when executing implementation plans with independent tasks — dispatches subagents per task with spec + code review gates
---

# Subagent-Driven Development

Execute implementation plans by dispatching independent tasks to subagents.

## Controller Pattern
1. Read the plan file
2. Create TaskCreate todo for each item
3. For each task: dispatch a fresh subagent with full context
4. Never ask subagent to read plan files — provide task text inline
5. Review each subagent output before marking task done

## Subagent Templates

### Implementer
```
You are implementing task X from plan Y.
Context: [what was built so far]
Task: [exact task text]
Files to modify: [list]
Expected output: [description]
Verification: go build ./... && go test ./...
```

### Spec Reviewer
```
Review this implementation against the spec:
- Does every requirement have code?
- Are there any extra changes?
- Report: PASS or NEEDS_CHANGE with specific items.
```

### Code Quality Reviewer
```
Review this diff for:
- Bugs / logic errors
- Security issues
- Code clarity
- Project conventions
Report: PASS or NEEDS_CHANGE with specific items.
```

## Review Gates
1. Spec compliance review → implementer fixes
2. Code quality review → implementer fixes
3. Both pass → mark task done

## Completion
After all tasks done → run full verification → invoke `finishing-a-development-branch`.
