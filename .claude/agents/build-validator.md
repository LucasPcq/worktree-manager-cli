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

### Step 6 — Architecture guard

#### 6a — `internal/flow/` import rule (BLOCKER, mechanical)

`internal/flow/` carries the flow of each command independently of the surface that
runs it. It may import only `internal/service/`, `internal/rules/`, `internal/domain/`
and the stdlib. Anything else — a UI library, a rendering package, the config loader,
the command layer — puts a surface back inside the flow and makes the layer
un-replayable by the dashboard.

```bash
go list -f '{{$p := .ImportPath}}{{range .Imports}}{{$p}} -> {{.}}
{{end}}' ./internal/flow/... \
  | grep -E ' -> (github.com/spf13|github.com/charmbracelet|github.com/LucasPcq/wtm/internal/(output|tui|config|commands|infra))' \
  || echo "flow imports OK"
```

Any line other than `flow imports OK` is a **BLOCKER**: report it as
`<flow package> -> <forbidden import>`.

This checks **direct** imports on purpose. `go list -deps` would be wrong here: `flow/`
legitimately imports `service/`, which itself reaches `infra/` and `config/`, so the
transitive graph always contains them. What the rule forbids is `flow/` reaching them
itself — the fix is a thin wrapper in `service/` (`worktree.FindByBranch`,
`worktree.ListAll`), never an exception.

Cross-check, which also covers a file that does not compile yet (`go list` skips those)
and test files:

```bash
grep -rnE '^[[:space:]]*([a-z_][a-zA-Z0-9]* )?"(github\.com/(spf13|charmbracelet)|github\.com/LucasPcq/wtm/internal/(output|tui|config|commands|infra))' \
  ./internal/flow/ --include='*.go' || echo "flow imports OK"
```

It matches import lines only — a doc comment naming `internal/tui/newwt` is not a
violation.

#### 6b — the other layers

```bash
# service/ must not import cobra, bubbletea or lipgloss
grep -rn "spf13/cobra\|charmbracelet/bubbletea\|charmbracelet/lipgloss" ./internal/service/ --include='*.go' || true

# output/ must not import service/
grep -rn "internal/service" ./internal/output/ --include='*.go' || true

# lipgloss.Style may only be instantiated in styles/
grep -rn "lipgloss.NewStyle" ./internal/ --include='*.go' | grep -v "/internal/styles/" || true
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
[STEP 6 — architecture]  ✅ clean  |  ❌ <violation list>   (6a flow imports / 6b layers)
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