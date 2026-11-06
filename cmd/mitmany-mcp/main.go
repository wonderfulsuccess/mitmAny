// mitmany-mcp bridges Codex's stdio MCP connection to the running desktop app.
package main

import (
	"fmt"
	"mitmAny/internal/agent"
	"os"
)

func main() {
	if err := agent.RunStdio(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
