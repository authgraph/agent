# authgraph-agent

The Authgraph Agent — MCP server and AI permissions agent for the [Authgraph](https://authgraph.dev) permission engine.

A single Go binary that runs as:
- **MCP server** (default) — for AI assistants (GitHub Copilot, Claude, Cursor)
- **HTTP server** (`serve` mode) — for GitHub Copilot Extension / custom integrations

## Installation

### Homebrew

```bash
brew tap authgraph/tap
brew install authgraph-agent
```

### Go Install

```bash
go install github.com/authgraph/agent/cmd/agent@latest
```

### Docker

```bash
docker pull ghcr.io/authgraph/agent:latest
docker run -e AUTHGRAPH_API_KEY=ag_... ghcr.io/authgraph/agent:latest
```

### From Source

```bash
git clone https://github.com/authgraph/agent.git
cd agent
go build -o authgraph-agent ./cmd/agent
```

## Usage

### MCP Mode (default)

Configure in your MCP client (VS Code, Claude Desktop, etc.):

```json
{
  "mcpServers": {
    "authgraph": {
      "command": "authgraph-agent",
      "env": {
        "AUTHGRAPH_API_KEY": "ag_your_api_key_here"
      }
    }
  }
}
```

### HTTP Mode (Copilot Extension)

```bash
export AUTHGRAPH_API_KEY=ag_your_key
authgraph-agent serve
```

Deploy to Fly.io:

```bash
fly launch --name authgraph-agent
fly secrets set AUTHGRAPH_API_KEY=ag_your_key
fly deploy
```

## MCP Tools

| Tool | Description |
|------|-------------|
| `check_permission` | Check if a subject has a permission on a resource |
| `grant_permission` | Grant a permission (create a relationship tuple) |
| `revoke_permission` | Revoke a permission (delete a relationship tuple) |
| `list_resources` | List all resources a subject can access |
| `expand_access` | List all subjects that have access to a resource |
| `push_schema` | Push a permission schema |
| `validate_schema` | Validate a schema without applying |
| `get_schema` | Get the current permission schema |
| `test_permissions` | Run permission test assertions |

## Copilot Extension

In GitHub Copilot Chat:

```
@authgraph Can user:alice read document:readme?
@authgraph Grant editor on project:main to user:bob
@authgraph Who has access to document:secret?
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `AUTHGRAPH_API_KEY` | Yes | Your Authgraph API key (starts with `ag_`) |
| `AUTHGRAPH_BASE_URL` | No | API base URL (default: `https://api.authgraph.dev`) |
| `PORT` | No | HTTP port for `serve` mode (default: `3000`) |

## Development

```bash
# Build
go build -o authgraph-agent ./cmd/agent

# Test
go test -race ./...

# Run MCP (stdio)
echo '{}' | AUTHGRAPH_API_KEY=ag_test ./authgraph-agent

# Run HTTP
AUTHGRAPH_API_KEY=ag_test ./authgraph-agent serve
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT — see [LICENSE](LICENSE).
