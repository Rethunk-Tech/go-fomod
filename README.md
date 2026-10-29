<h1 align="center">go-fomod</h1>

<div align="center">

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.27.3+-00ADD8?logo=go&logoColor=white)](go.mod)

</div>

---

A reader and evaluator for FOMOD installers: parses ModuleConfig.xml, evaluates its conditions and turns choices into the file copy list. Package `fomod`, module `github.com/Rethunk-Tech/go-fomod`.

It copies no files, lays nothing out on disk and never looks at the game folder: the caller hands it an `fs.FS` and
the installed game's file states. Nexus archives ship configs in UTF-8, UTF-16 and Windows-1252 whatever they declare,
so the parser decides the encoding from the bytes and ignores the declaration.

## Quick start

```bash
go get github.com/Rethunk-Tech/go-fomod
```

A worked example and the contracts: [HUMANS.md](HUMANS.md).

## Features

- `Open` and `Find` locate and parse `fomod/ModuleConfig.xml` in any `fs.FS`, at the root or one folder down.
- `Parse` reads UTF-8, UTF-16 and Windows-1252 configs, with or without a BOM or a correct declaration.
- `Resolve` turns choices into the copy list, sorted by priority; `Match`, `Unanswered` and `Keep` replay recorded choices.
- Steps, groups and plugins come back in their declared `order`, already sorted.
- `VisibleSteps`, `FlagsFrom` and `PluginType` give a wizard what to show under the current answers.
- The config read is capped, entities are never expanded and nesting is bounded.

## Documentation

| Doc | Audience |
| --- | --- |
| [HUMANS.md](HUMANS.md) | Running and using the package |
| [AGENTS.md](AGENTS.md) | Internals, invariants, and why |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Process — commits, tests, docs |
| [SECURITY.md](SECURITY.md) | Vulnerability disclosure |
| [CHANGELOG.md](CHANGELOG.md) | Release notes |

## License

MIT — see [LICENSE](LICENSE). © 2026 Rethunk.Tech, LLC.
