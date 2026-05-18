---
name: systematic-debugging
description: Use when encountering any bug, test failure, or unexpected behavior, before proposing fixes
---

# Systematic Debugging

Iron Law: **No fixes without root cause investigation first.**

## Phase 1: Root Cause Investigation
1. Reproduce the bug reliably
2. Isolate: what's the smallest input that triggers it?
3. Trace the code path from entry to failure
4. Find the exact line where behavior diverges from expectation

## Phase 2: Pattern Analysis
1. Is this a one-off or a pattern?
2. Are there similar bugs in adjacent code?
3. What assumption was violated?

## Phase 3: Hypothesis
1. State: "I believe X causes Y because Z"
2. Test the hypothesis with a minimal reproduction
3. If hypothesis fails, return to Phase 1

## Phase 4: Implementation
1. Write a test that fails (reproduces the bug)
2. Fix the root cause, not the symptom
3. Verify the test passes
4. Check for regressions: `go test ./...`

## Red Flags
- "I'll just add a nil check" → symptom masking
- "It works on my machine" → environment assumption
- "This is probably the issue" → guessing instead of tracing
