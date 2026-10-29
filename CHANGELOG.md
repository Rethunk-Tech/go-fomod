# Changelog

Notable changes to `go-fomod`. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `Parse` reads a `ModuleConfig.xml` in UTF-8, UTF-16 or Windows-1252, whatever it declares, and applies the `order`
  attribute to steps, groups and plugins.
- `Find` and `Open` locate and read the config in any `fs.FS`, with a read cap and `ErrTooLarge`.
- `Composite.Eval`, `PluginType`, `VisibleSteps` and `FlagsFrom` evaluate flag, file and game-version conditions.
- `Resolve` lists the file and folder operations for a set of `Choices`; `Match`, `Unanswered` and `Keep` replay
  recorded choices against a changed config.

[Unreleased]: https://github.com/Rethunk-Tech/go-fomod/commits/main
