# Changelog

Notable changes to `go-fomod`. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- A symlinked `ModuleConfig.xml` is ignored.

## [0.2.0] - 2026-11-12

### Added

- `Op.Escapes` marks a path operand that leaves the root, and `ResolveSafe` returns the operations that escape separately.

### Fixed

- `Parse` refuses documents with more markup than any real config, and keeps equal-priority file and folder operations in document order.
- `Resolve` installs every plugin of a `SelectAll` group that repeats a name.
- `Unanswered` judges a step's visibility where the installer reaches it.

## [0.1.2] - 2026-11-07

### Fixed

- The same code as 0.1.1, released again: 0.1.1's tag moved after the Go module proxy had cached it, so `go get` from the proxy and from GitHub disagreed on its checksum. Use 0.1.2.

## [0.1.1] - 2026-11-07

### Fixed

- A plugin named twice in a group is refused and installs once.
- `Parse` computes each sort key once, so a config with many plugins sorts in linear time.

## [0.1.0] - 2026-10-29

### Added

- `Parse` reads a `ModuleConfig.xml` in UTF-8, UTF-16 or Windows-1252, whatever it declares, and applies the `order`
  attribute to steps, groups and plugins.
- `Find` and `Open` locate and read the config in any `fs.FS`, with a read cap and `ErrTooLarge`.
- `Composite.Eval`, `PluginType`, `VisibleSteps` and `FlagsFrom` evaluate flag, file and game-version conditions.
- `Resolve` lists the file and folder operations for a set of `Choices`; `Match`, `Unanswered` and `Keep` replay
  recorded choices against a changed config.

[Unreleased]: https://github.com/Rethunk-Tech/go-fomod/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/Rethunk-Tech/go-fomod/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/Rethunk-Tech/go-fomod/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/Rethunk-Tech/go-fomod/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/Rethunk-Tech/go-fomod/releases/tag/v0.1.0
