# CLAUDE.md — local-ci configuration guidelines

## Build & Test Commands

- Build the CLI: `go build -o local-ci main.go`
- Run all tests: `go test ./...`
- Format Go code: `go fmt ./...`
- Check Go code (vet): `go vet ./...`

## Development Workflow

- We follow **TDD (Test-Driven Development)** for new features.
- Keep the CLI clean and minimal.
- Use ` BurntSushi/toml` for config parsing.
