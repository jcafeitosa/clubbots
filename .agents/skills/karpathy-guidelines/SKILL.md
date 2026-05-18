---
name: karpathy-guidelines
description: Andrej Karpathy coding principles — think before coding, simplicity first, surgical changes, goal-driven execution. Use always.
---

# Karpathy Coding Guidelines

Four principles for all code changes. Apply on every task. No exceptions.

## 1. Think Before Coding
- State assumptions explicitly before writing code
- Surface ambiguity — ask if unsure
- Push back on bad requirements
- Stop when confused — don't guess

## 2. Simplicity First
- Minimum code that solves the problem
- No speculative features, abstractions, or flexibility
- If 200 lines could be 50, rewrite
- One function, not a strategy pattern

## 3. Surgical Changes
- Touch ONLY what the user asked for
- Match existing style exactly
- Do NOT refactor adjacent code
- Clean up only your own orphans
- Every changed line MUST trace to the request

## 4. Goal-Driven Execution
- Convert "do X" into "write test that reproduces X, then make it pass"
- Use verifiable checkpoints: `[Step] --> verify: [check]`
- Strong criteria let the agent loop independently
- Reproduce bug in test first, then fix

## Verification Checkpoint Pattern
```
[Build] --> verify: go build ./...
[Test] --> verify: go test ./pkg/... -run TestFoo
[Lint] --> verify: go vet ./...
```

Before claiming completion: run all three checks. Evidence before assertions.
