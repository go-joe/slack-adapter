<h1 align="center">Joe Bot - Slack Adapter</h1>
<p align="center">Connecting joe with the Slack chat application. https://github.com/go-joe/joe</p>
<p align="center">
	<a href="https://github.com/go-joe/slack-adapter/releases"><img src="https://img.shields.io/github/tag/go-joe/slack-adapter.svg?label=version&color=brightgreen"></a>
	<a href="https://circleci.com/gh/go-joe/slack-adapter/tree/master"><img src="https://circleci.com/gh/go-joe/slack-adapter/tree/master.svg?style=shield"></a>
	<a href="https://godoc.org/github.com/go-joe/slack-adapter"><img src="https://img.shields.io/badge/godoc-reference-blue.svg?color=blue"></a>
	<a href="https://github.com/go-joe/joe/blob/master/LICENSE"><img src="https://img.shields.io/badge/license-BSD--3--Clause-blue.svg"></a>
</p>

---

This repository contains a module for the [Joe Bot library][joe].

**THIS SOFTWARE IS STILL IN ALPHA AND THERE ARE NO GUARANTEES REGARDING API STABILITY YET.**

## Getting Started

Joe is packaged using the new [Go modules][go-modules]. Therefore the recommended
installation is by adding joe and all used modules to your `go.mod` file like this: 

```
module github.com/go-joe/example-bot

require (
	github.com/go-joe/joe v0.2.0
	github.com/go-joe/slack-adapter v0.2.0
	…
)
```

If you want to hack on the adapter you can of course also go get it directly:

```bash
go get github.com/go-joe/slack-adapter
```

### Minimal example

**TODO**

## Built With

* [nlopes/slack](https://github.com/nlopes/slack) - Slack API in Go
* [zap](https://github.com/uber-go/zap) - Blazing fast, structured, leveled logging in Go
* [pkg/errors](https://github.com/pkg/errors) - Simple error handling primitives

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of
conduct and on the process for submitting pull requests to this repository.

## Versioning

We use [SemVer](http://semver.org/) for versioning. For the versions available,
see the [tags on this repository][tags. 

## Authors

- **Friedrich Große** - *Initial work* - [fgrosse](https://github.com/fgrosse)

See also the list of [contributors][contributors] who participated in this project.

## License

This project is licensed under the BSD-3-Clause License - see the [LICENSE](LICENSE) file for details.

[joe]: https://github.com/go-joe/joe
[go-modules]: https://github.com/golang/go/wiki/Modules
[tags]: https://github.com/go-joe/joe/tags
[contributors]: https://github.com/github.com/go-joe/slack-adapter/contributors
