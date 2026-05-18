---
name: test-driven-development
description: Use when implementing any feature or bugfix, before writing implementation code
---

# Test-Driven Development

Iron Law: **No production code without a failing test first.**

## Red-Green-Refactor Cycle

### Red — Write Failing Test
1. Write the smallest possible test that fails
2. Run it — confirm it fails for the expected reason
3. If it passes without code changes, the test is wrong

### Green — Make It Pass
1. Write minimum code to make the test pass
2. No refactoring, no cleanup, no "while I'm here"
3. Run all tests — confirm only the new test went from red to green

### Refactor — Clean Up
1. Remove duplication
2. Improve names
3. Simplify logic
4. Run tests after every change

## Rationalization Detection
If you think "this is too simple to need a test":
- You're rationalizing
- Simple code breaks too
- The test is the specification

## Verification
```bash
go test ./pkg/... -v -run TestNewFeature
go test ./... -count=1  # full suite, no cache
```
