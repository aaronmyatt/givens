# Constraints

Givens is built to the safety rules of [Tiger Style](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md)
and NASA/JPL's [Power of Ten](https://en.wikipedia.org/wiki/The_Power_of_10:_Rules_for_Developing_Safety-Critical_Code),
adapted to Go. Every rule a tool can check is checked, on every edit and every commit.

## Enforced rules

| Rule | Source | Checked by |
| --- | --- | --- |
| Function bodies are at most 70 lines | Tiger Style; Power of Ten 4 | tigerlint `functionlength` |
| Lines are at most 100 columns (a tab counts as 4) | Tiger Style | tigerlint `linelength` |
| Control flow nests at most 4 levels in a function | Tiger Style ("centralize control flow") | tigerlint `nestingdepth` |
| No recursion | Power of Ten 1; Tiger Style | tigerlint `norecursion` |
| No `goto`, no `init()`, no `recover()`, no `panic()` outside `internal/assert` | Power of Ten 1 | tigerlint `simpleflow` |
| Every loop has a fixed bound: `range`, three clauses, or a `// bound:` comment saying why it ends | Power of Ten 2; Tiger Style ("put a limit on everything") | tigerlint `boundedloops` |
| `make` of a slice or map states a size | Power of Ten 3; Tiger Style (static allocation), adapted | tigerlint `sizedmake` |
| Each package averages 2 or more assertions per function | Power of Ten 5; Tiger Style | tigerlint `assertdensity` |
| No package-level variables, except `Err*` sentinels | Power of Ten 6 | tigerlint `noglobals` |
| No call drops an error result | Power of Ten 7; Tiger Style | tigerlint `checkederrors` |
| No `unsafe`, `reflect` or cgo | Power of Ten 8 and 9, adapted | tigerlint `forbiddenimports` |
| No pointer-to-pointer | Power of Ten 9 | tigerlint `nodoublepointers` |
| No debt markers in comments; work goes in `todo.txt` | Tiger Style ("zero technical debt") | tigerlint `debtmarkers` |
| Code is `gofmt`-formatted | Tiger Style ("run the formatter") | `scripts/check.sh fmt` |
| Zero warnings from the compiler's checks | Power of Ten 10 | `go vet` |
| Direct dependencies are on an allowlist | Tiger Style ("zero dependencies") | `scripts/check.sh deps` |
| Shell scripts pass shellcheck | Power of Ten 10 | `scripts/check.sh shell` |
| The kit's bats tests pass under bash 3.2 | this project | `scripts/check.sh kit` |
| Feature files follow `capabilities/GIVENS.md` | this project | `go test ./acceptance` |
| Tests pass, scenarios without `@wip` included | this project | `go test ./...` |

## Allowed exceptions

Each one is written where it applies, so a reviewer sees it.

- `//tiger:allow-global <why>` on a package-level variable, such as an `//go:embed` target or
  a compiled regexp.
- `// bound: <why it ends>` on a loop that isn't a `range` or three-clause loop.
- `_ = call()` drops an error on purpose. Printing to stdout and writing to a
  `strings.Builder` or `bytes.Buffer` are exempt.
- Test files don't count towards assertion density.

## Rules a tool can't check

Followed in review. From Tiger Style unless noted.

- Assertions: assert arguments, return values, pre- and postconditions. Assert the positive
  space and the negative space. Split compound assertions: `assert.That(a)`, `assert.That(b)`.
  A failed assertion crashes: it's a bug. An operating error (a missing file, a failed command)
  is a returned `error`.
- Put a limit on everything: queue sizes, file sizes, timeouts, retries. Name the limit as a
  constant with its unit last: `setupTimeoutSecondsMax`, not `maxTimeout`.
- Names say what a thing is; don't abbreviate. Units and qualifiers go last.
- Declare variables in the smallest scope, close to where they're used. (Power of Ten 6)
- Push `if`s up and `for`s down: a parent decides, helpers do one thing.
- Comments say why. Each exported name has a doc comment; each use of a library, API or
  external format links to its docs.
- Prefer the standard library. A new dependency needs a reason and a line in
  `scripts/check.sh`'s allowlist.
- Back-of-the-envelope the cost of anything that touches the disk, the network or a process.

## How it's enforced

| When | What runs | Blocks |
| --- | --- | --- |
| Claude edits a file | `.claude/hooks/after-edit.sh`: gofmt + tigerlint on the package, the spec test for `.feature`, the allowlist for `go.mod`, shellcheck for scripts | the edit's result is sent back to Claude to fix |
| Claude finishes a turn | `.claude/hooks/before-stop.sh`: `scripts/check.sh` | Claude keeps working until it passes |
| `git commit` | `.githooks/pre-commit`: `scripts/check.sh` | the commit |
| Any time | `scripts/check.sh` (or `scripts/check.sh lint` for one check) | nothing; it reports |

The git hook is on once per clone: `git config core.hooksPath .githooks`.
tigerlint's own rules are tested in `tools/tigerlint/rules/testdata/`: one package per rule,
each with the violations it must catch.
