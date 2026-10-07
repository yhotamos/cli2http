# Changelog

## [0.1.2] - 2026-10-07

### Added

- `--token` option to reuse a token across restarts

### Changed

- Reject `cli2http` itself as a target command
- Show `URL` in startup output and remove the extra `POST /exec` line

## [0.1.1] - 2026-10-05

### Added

- `--port` option to choose a listening port
- Prebuilt binaries for Windows, macOS, and Linux

### Changed

- Reject common shells as target commands
- Minimum Go version raised to 1.26

## [0.1.0] - 2026-10-04

### Added

- HTTP API for running an existing CLI with `cli2http <command>`
- Token authentication for command execution and server information
- Exit code, stdout, and stderr in execution responses
- Installation with `go install` and version display with `--version`

[0.1.2]: https://github.com/yhotamos/cli2http/releases/tag/v0.1.2
[0.1.1]: https://github.com/yhotamos/cli2http/releases/tag/v0.1.1
[0.1.0]: https://github.com/yhotamos/cli2http/releases/tag/v0.1.0
