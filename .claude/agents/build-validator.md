---
name: build-validator
description: Subagent that validates a Go CLI project before commit or task completion. Runs go build, go vet, staticcheck, and the test suite, then reports all issues grouped by severity. Use this skill at the end of every development session, before any git commit, or when explicitly asked to validate the project. Trigger on "validate", "check the build", "run build-validator", "is the code ready", "pre-commit check", or any request to confirm the project compiles and passes quality gates. Always trigger this after implementing a feature if the user follows the CLAUDE.md principles.
tools: Bash, Read, Grep, Glob
model: haiku
---

# build-validator

Validate a Go CLI project end-to-end before commit.
This subagent is the mandatory last step of every development session.

---

## Execution Steps

Run all steps in sequence. Do not stop on first failure — collect all issues
before reporting. Report a final summary at the end.

### Step 1 — Dependency hygiene

```bash
go mod tidy
git diff --exit-code go.mod go.sum
```

Fail if `go.mod` or `go.sum` changed after tidy (means deps were not tidied).

### Step 2 — Compilation

```bash
go build ./...
```

Report: all package paths that fail to compile.

### Step 3 — Vet

```bash
go vet ./...
```

Report: all vet diagnostics (printf mismatches, unreachable code, etc.).

### Step 4 — Static analysis

Install if absent, then run:

```bash
go install honnef.co/go/tools/cmd/staticcheck@latest
staticcheck ./...
```

Report: all staticcheck findings with their SA/S-code and file location.

### Step 5 — Tests

```bash
go test ./... -race -count=1
```

Report: failed tests with package path and test name.
Flag any data races detected by `-race`.

### Step 6 — Architecture guard (manual check)

Inspect imports to catch layer violations:

```bash
# commands/ must not import service business logic directly (only via interface)
grep -rn "\"service\"" ./internal/commands/ || true

# service/ must not import cobra or OS args
grep -rn "\"github.com/spf13/cobra\"\|\"os\"" ./internal/service/ || true

# output/ must not import service/
grep -rn "\"service\"" ./internal/output/ || true
```

Flag any hits as architecture violations.

### Step 7 — Magic string / constant guard

```bash
# Warn if raw exit codes appear outside constants.go
grep -rn "os\.Exit([0-9])" . --include="*.go" \
  | grep -v "constants.go" \
  | grep -v "_test.go" || true
```

---

## Report Format

After all steps, output a structured report:

```
╔══════════════════════════════════════╗
║        build-validator report        ║
╚══════════════════════════════════════╝

[STEP 1 — go mod tidy]   ✅ clean  |  ❌ <detail>
[STEP 2 — go build]      ✅ clean  |  ❌ <N packages failed>
[STEP 3 — go vet]        ✅ clean  |  ⚠️  <N issues>
[STEP 4 — staticcheck]   ✅ clean  |  ⚠️  <N issues>
[STEP 5 — tests]         ✅ all pass  |  ❌ <N failed>
[STEP 6 — architecture]  ✅ clean  |  ❌ <violation list>
[STEP 7 — magic strings] ✅ clean  |  ⚠️  <N occurrences>

── Issues ────────────────────────────────────────────────
<list each issue: severity | file:line | message>

── Verdict ───────────────────────────────────────────────
✅ READY TO COMMIT   — all checks passed.
❌ NOT READY         — fix the issues above before committing.
```

Severity levels:
- `❌ BLOCKER` — build failure, test failure, architecture violation
- `⚠️  WARNING` — vet / staticcheck / magic string

**Do not mark a task as done if the verdict is `NOT READY`.**

---

## Autofix Guidance

If issues are found, suggest the minimal fix for each blocker before asking
the user to re-run. For warnings, list them but do not block if the user
explicitly accepts them.