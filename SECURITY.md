# Security

## Supported versions

The latest tagged release. There are no long-lived release branches, so a fix ships in the next tag
rather than being backported.

## Reporting a vulnerability

Use GitHub's **private vulnerability reporting** on this repository (Security → Report a
vulnerability). It stays private until a fix ships.

Please do not open a public issue for a suspected vulnerability first.

## What is in scope

- `Parse` and `Open` read a `ModuleConfig.xml` from an archive anyone can publish. A crafted file must not panic,
  expand an entity (a billion-laughs or external entity fails to parse), recurse past the standard library's nesting
  limit, or make `Match` and `Resolve` slower than linear in a group's plugin count. `FuzzParse` checks the first and
  the order and idempotence invariants.
- `Open` reads at most a fixed cap and returns `ErrTooLarge` past it, so the file cannot exhaust memory.
- `Find` walks the caller's `fs.FS` to one folder of depth and reads nothing but directory listings, with a bounded
  case-folded lookup.
- The `source` and `destination` strings in `Op` are untrusted text from the config. Nothing in this module opens or
  copies them.

## What is out of scope

- Resolving an `Op.Source` or `Op.Destination` against a disk. The caller must refuse `..`, absolute paths and links
  that leave the install root, and check that a source exists.
- The `fs.FS` itself: the caller confines it to the extracted archive.
- A config that is valid XML but asks for odd installs (an empty `Required` list, a repeated plugin name, a priority
  at the integer limits) is handled as the spec describes; it is the caller's policy whether to install it.
