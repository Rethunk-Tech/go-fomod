# HUMANS.md

Run and use `go-fomod`. Internals: [AGENTS.md](AGENTS.md).

## Quick start

```bash
go get github.com/Rethunk-Tech/go-fomod
```

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/Rethunk-Tech/go-fomod"
)

func main() {
	cfg, _, ok, err := fomod.Open(os.DirFS("extracted-archive"))
	if err != nil {
		log.Fatal(err)
	}
	if !ok {
		log.Fatal("not a FOMOD archive")
	}
	ctx := fomod.EvalContext{GameVersion: "1.6.15"}
	choices := fomod.Choices{"Options": {"Pack": {"Beta"}}}
	if !fomod.Match(cfg, choices, ctx) {
		log.Fatal("the choices no longer fit this config: ask again")
	}
	for _, op := range fomod.Resolve(cfg, choices, ctx) {
		fmt.Println(op.Source)
	}
}
```

## Contracts

| Behaviour | Contract |
| --- | --- |
| `Open`, `Find` | Take an `fs.FS`; paths are slash paths relative to it. The `fomod` folder sits at the root or one folder down, matched case-insensitively with an exact spelling preferred. `Find` returns `""` and no error when there is none |
| `Open` read | Reads at most a fixed cap and fails with `ErrTooLarge` past it, so a caller can tell an oversized config from a malformed one |
| `Parse` | Takes the raw file bytes and decides the encoding from them: BOM, then NUL bytes for BOM-less UTF-16, then Windows-1252 for bytes that are not UTF-8. The declared encoding is ignored |
| `Parse` order | Steps, groups and plugins come back sorted by their `order` attribute (absent or unknown means `Ascending`); `Explicit` keeps document order. Names compare as UTF-16 code units, case-sensitively |
| `EvalContext.GameVersion` | The installed game version, ordered against each gameDependency minimum by go-semverish (a missing number is 0). `""` makes every game dependency unmet; a version that cannot be ordered counts as met |
| `EvalContext.Files` | The host's view of the installed game. `nil` reports every file `Missing`. The module never touches a disk |
| `Resolve` | Required files, chosen plugins, `Required` plugins under the live flags, unchosen plugins' `alwaysInstall` and usable `installIfUsable` files, then matching conditional installs, stably sorted by priority. It never copies and never checks that a source exists |
| `Match` | False when a choice names a plugin that is gone or `NotUsable` under the flags, or a visible group's type is not satisfied |
| `Unanswered`, `Keep` | `Unanswered` is true when the new config shows a pick-group neither the old config nor the choices knew. `Keep` drops choices naming a vanished plugin and is idempotent |
| Unsafe input | Entities are never expanded, nesting is bounded by `encoding/xml`, and nothing panics on a crafted config (`FuzzParse`) |

## Requirements

Go 1.27.3 or newer, plus `lefthook` and `gitleaks` on PATH if you want the git hooks.

```bash
lefthook install
```

That is per clone; git does not commit hooks. Pre-commit scans staged changes for secrets, and
pre-push runs `gate`.

## Verify

```bash
gate
```
