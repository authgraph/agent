# Contributing to authgraph-agent

## Development Setup

```bash
git clone https://github.com/authgraph/agent.git
cd agent
go mod tidy
go build ./...
```

## Running Tests

```bash
go test -race ./...
```

## Code Style

- Run `go vet ./...` before committing
- Use `golangci-lint run` for linting
- Follow standard Go project layout

## Commit Messages

Use [conventional commits](https://www.conventionalcommits.org/):

- `feat:` new features
- `fix:` bug fixes
- `docs:` documentation changes
- `refactor:` code restructuring
- `test:` test additions/changes
- `chore:` maintenance

## Releasing

Tags trigger releases automatically via GoReleaser:

```bash
git tag v0.1.0
git push origin v0.1.0
```
