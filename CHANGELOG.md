# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
Nothing so far

## [v2.1.0] - 2020-07-25
- Add new `EventsAPIAdapter` function to support integrating with Slack via the 
  [Events API](https://api.slack.com/events-api).

## [v2.0.0] - 2020-04-12
- **Breaking change** :fire: : Replace slack library from [nlopes/slack](https://github.com/nlopes/slack)
  to [slack-go/slack](https://github.com/slack-go/slack). This change breaks
  compatibility of the `WithMessageParams(…)` option which used to accept a
  `slack.PostMessageParameters` from `github.com/nlopes/slack` but now requires
  the same type from `github.com/slack-go/slack`. 

## [v1.0.0] - 2020-02-28
- Use error wrapping of standard library instead of github.com/pkg/errors
- Update to Go 1.14
- Release first stable version and start following semantic versioning with regards to backwards compatibility

## [v0.9.0] - 2020-02-25
- Add `ListenPassive` to send all seen messages to the Bot instead of only the ones directed to it 

## [v0.8.0] - 2020-01-22
- Update Slack client library to fix RTM unmarshal errors
- Add new `WithLogUnknownMessageTypes` option to help debug issues with Slack
- Log slack errors when RTM message parsing fails

## [v0.7.0] - 2019-10-22
- Support sending and receiving reactions

## [v0.6.2] - 2019-09-22
- Actually use `Config.Name` field when sending messages
- Fix issue [Bot is handling messages coming from itself #5](https://github.com/go-joe/slack-adapter/issues/5)

## [v0.6.1] - 2019-09-22
*Accidentally tagged on the wrong branch*, use v0.6.2

## [v0.6.0] - 2019-04-19
- Update to joe v0.7.0
- Set the ReceiveMessageEvent.AuthorID field
- Set the ReceiveMessageEvent.Data field to the github.com/nlopes/slack.MessageEvent

## [v0.5.1] - 2019-03-25
- Fix missing avatar and bot name when sending messages

## [v0.5.0] - 2019-03-24
- Automatically parse all sent messages (e.g. allow `@someone` or `#channel`)

## [v0.4.0] - 2019-03-18
- Update to the changed Module interface of joe v0.4.0

## [v0.3.0] - 2019-03-17
- Unit tests :)
- Do not leak received messages as debug messages
- Rename `API` type to `BotAdapter`
- `NewAdapter(…)` now returns a`*BotAdapter` instead of a `joe.Adapter`

## [v0.2.0] - 2019-03-10
- Update to the changed Adapter interface of joe v0.2.0

## [v0.1.0] - 2019-03-03
- Initial alpha release

[Unreleased]: https://github.com/go-joe/slack-adapter/compare/v2.1.0...HEAD
[v2.1.0]: https://github.com/go-joe/slack-adapter/compare/v2.0.0...v2.1.0
[v2.0.0]: https://github.com/go-joe/slack-adapter/compare/v1.0.0...v2.0.0
[v1.0.0]: https://github.com/go-joe/slack-adapter/compare/v0.9.0...v1.0.0
[v0.9.0]: https://github.com/go-joe/slack-adapter/compare/v0.8.0...v0.9.0
[v0.8.0]: https://github.com/go-joe/slack-adapter/compare/v0.7.0...v0.8.0
[v0.7.0]: https://github.com/go-joe/slack-adapter/compare/v0.6.2...v0.7.0
[v0.6.2]: https://github.com/go-joe/slack-adapter/compare/v0.6.0...v0.6.2
[v0.6.1]: https://github.com/go-joe/slack-adapter/compare/v0.6.0...v0.6.1
[v0.6.0]: https://github.com/go-joe/slack-adapter/compare/v0.5.1...v0.6.0
[v0.5.1]: https://github.com/go-joe/slack-adapter/compare/v0.5.0...v0.5.1
[v0.5.0]: https://github.com/go-joe/slack-adapter/compare/v0.4.0...v0.5.0
[v0.4.0]: https://github.com/go-joe/slack-adapter/compare/v0.3.0...v0.4.0
[v0.3.0]: https://github.com/go-joe/slack-adapter/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/go-joe/slack-adapter/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/go-joe/slack-adapter/releases/tag/v0.1.0
