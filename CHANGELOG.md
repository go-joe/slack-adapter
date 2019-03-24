# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
- Automatically parse all sent messages (e.g. allow `@someone` or `#channel`)

## [v0.4.0] - 2019-03-18
### Changed
- Update to the changed Module interface of joe v0.4.0

## [v0.3.0] - 2019-03-17
### Added
- Unit tests :)

### Changed
- Do not leak received messages as debug messages
- Rename `API` type to `BotAdapter`
- `NewAdapter(…)` now returns a`*BotAdapter` instead of a `joe.Adapter`

## [v0.2.0] - 2019-03-10

### Changed
- Update to the changed Adapter interface of joe v0.2.0

## [v0.1.0] - 2019-03-03

Initial alpha release

[Unreleased]: https://github.com/go-joe/slack-adapter/compare/v0.4.0...HEAD
[v0.3.0]: https://github.com/go-joe/slack-adapter/compare/v0.3.0...v0.4.0
[v0.3.0]: https://github.com/go-joe/slack-adapter/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/go-joe/slack-adapter/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/go-joe/slack-adapter/releases/tag/v0.1.0
