package main

import (
	"fmt"
	"os"

	"github.com/authgraph/agent/internal/agent"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			fmt.Printf("authgraph-agent %s\n", version)
			return
		case "serve":
			if err := agent.ServeHTTP(); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			return
		case "--help", "-h", "help":
			printUsage()
			return
		}
	}

	// Default: run as MCP server (stdio)
	if err := agent.ServeMCP(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`authgraph-agent %s

The Authgraph Agent — MCP server and HTTP agent for AI-native permission management.

Usage:
  authgraph-agent           Run as MCP server (stdio transport, for AI assistants)
  authgraph-agent serve     Run as HTTP server (for GitHub Copilot Extension)
  authgraph-agent version   Print version information

Environment Variables:
  AUTHGRAPH_API_KEY       Your Authgraph API key (required)
  AUTHGRAPH_BASE_URL      API base URL (default: https://api.authgraph.dev)
  PORT                    HTTP server port for 'serve' mode (default: 3000)
  GITHUB_TOKEN            GitHub token for Copilot signature verification (serve mode)

MCP Mode (default):
  Connects via stdio. Configure in your MCP client:
  {
    "mcpServers": {
      "authgraph": {
        "command": "authgraph-agent",
        "env": { "AUTHGRAPH_API_KEY": "ag_..." }
      }
    }
  }

HTTP Mode (serve):
  Runs an HTTP server for GitHub Copilot Extension integration.
  Deploy to Fly.io, Railway, or any container platform.
`, version)
}
