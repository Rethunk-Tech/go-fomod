# Security

## Supported versions

The latest tagged release. There are no long-lived release branches, so a fix ships in the next tag
rather than being backported.

## Reporting a vulnerability

Use GitHub's private vulnerability reporting on this repository: <https://github.com/Rethunk-Tech/go-fomod/security/advisories/new>, or the **Security** tab, then **Report a vulnerability**. The report stays private between you and us until a fix ships.

If private reporting is unavailable, email [oss@rethunk.tech](mailto:oss@rethunk.tech) with **[SECURITY]** in the subject.

Please do not open a public issue, pull request or discussion for a suspected vulnerability.

A useful report has the affected repository and version (or commit), steps to reproduce, what an attacker gets and under which conditions, and whether anyone else knows.

## What counts as a vulnerability

A vulnerability is a flaw that lets someone cross a boundary they should not: run code, read or change data they have no right to, escape a sandbox or path restriction, or leak a credential. A bug is anything else that misbehaves, such as a crash on your own input, wrong output or a slow path; open an ordinary issue for those. If you are unsure which one you have, report it privately and we will say.

## Coordinated disclosure

We handle the report inside the private advisory, so you see each status change and can comment on the fix. The advisory goes public when the fix ships, or 90 days after your report, whichever comes first. If a fix is close, either of us can ask for more time in the thread. You are credited in the advisory unless you would rather not be.

## Response timelines

- We acknowledge your report within 3 business days.
- We give you an assessment (accepted, needs more information or declined, with reasons) within 10 business days.
- We aim to ship a fix within 90 days of the report.

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
- A config that is valid XML but asks for odd installs (an empty `Required` list, a priority
  at the integer limits) is handled as the spec describes; it is the caller's policy whether to install it. A choice that names a
  plugin twice is not such a config: `Match` refuses it and `Resolve` installs the plugin once.

## Safe harbour

We will not pursue legal action against good-faith research that avoids privacy violations, data destruction and disruption of services, stops at a proof of concept, and keeps the details private until the advisory is published. Rethunk.Tech, LLC maintains this repository.
