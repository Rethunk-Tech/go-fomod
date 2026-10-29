# AGENTS.md

Package `fomod` parses a FOMOD installer's `fomod/ModuleConfig.xml`, evaluates its conditions (flags, file states,
game version) and turns a set of choices into the file and folder copy operations it declares. Mod managers, wizards
and linters import it. It imports no consumer package, and no `os` or `path/filepath` in production code (depguard
enforces it): every path is a slash path inside the caller's `fs.FS`, which is why CI has one Linux test job.

## Gate

`gate` is the command. Each override in `.gate.toml` is there because a runner
cannot resolve that check from PATH.

## Shape

- `Parse` is the only place the encoding is decided (`decodeXML`, then `decodeLegacy`), and its `CharsetReader`
  accepts any label. The bytes are UTF-8 by then, and declarations on real archives are often wrong. Never reject on
  the declaration.
- Windows-1252 is a table, not `golang.org/x/text`: one 32-entry table spares every consumer a third-party
  requirement. The five bytes Windows leaves undefined map to U+FFFD.
- `Decoder.Entity` and `Strict = false` stay unset. `encoding/xml` then never expands an entity, and its own nesting
  limit bounds `Composite.UnmarshalXML`'s recursion. `TestParseRefusesEntities` and `TestParseBoundsNesting` pin both.
- The read cap (`maxConfigBytes`, 16 MiB) is unexported and belongs to `Open`, because every consumer reads an
  untrusted archive. It is far above any real config, and a UTF-16 file decodes to at most 1.5 times its size, so the
  parser's input is bounded as well.
- `Parse` applies `order` to steps, groups and plugins, comparing names as UTF-16 code units, case-sensitively, as
  Qt's `QString` does in Mod Organizer 2. Consumers never sort.
- `selected` judges `Required` under the flags in force when the group is reached, as the spec does. A plugin that is
  `Required` by default but `NotUsable` under the flags is not forced in, so the choices still `Match`.
- `pluginIndex` keeps `Match` and `Resolve` linear in a group's plugin count; `TestLargeGroupIsLinear` compares a
  ratio, not a wall-clock time.
- `gameDepOK` orders versions with `semverish.Compare`, and a version it cannot order counts as met. That policy is
  FOMOD semantics owned here, not the comparator's.
- `Find` resolves names with `safefs.ResolveFold` on a `fs.Sub` of each folder it visits, so a case-variant sibling
  cannot redirect the lookup, and joins with `path.Join`: `"./fomod"` fails `fs.ValidPath` at the root.
- `Keep` visits each step and group name once, so a config that repeats a name still gets an idempotent result.
- `testdata/behaviour.golden` pins every evaluator function over fixtures, choice sets and contexts. Regenerate it
  with `FOMOD_RECORD=1` only for an intended change.

## Files

| File | Holds |
| --- | --- |
| `doc.go` | Package comment |
| `types.go` | Exported constants, `Config` and its node types, `Choices`, `Op`, `EvalContext` |
| `parse.go` | `Parse`, the `order` sort, and the unexported `xml*` structs with the dependency unmarshalers |
| `decode.go` | `decodeXML` (BOM and NUL detection, UTF-16), the Windows-1252 table |
| `resolve.go` | `Composite.Eval`, `PluginType`, `VisibleSteps`, `FlagsFrom`, `Match`, `Resolve`, `Unanswered`, `Keep` |
| `find.go` | `Find`, `Open`, `ErrTooLarge` and the read cap |
