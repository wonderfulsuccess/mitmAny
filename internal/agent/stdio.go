package agent

import (
	"context"
	"flag"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"io"
	"os"
	"os/signal"
)

// RunStdio is shared by the packaged desktop --mcp mode and standalone bridge.
func RunStdio(args []string, stderr io.Writer) error {
	defaultDir, err := DefaultDir()
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("mitmany-mcp", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("data-dir", defaultDir, "mitmAny application data directory")
	if err = flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	server := NewMCP(func(ctx context.Context, name string, a Args) (any, error) { return RemoteCall(ctx, *dir, name, a) })
	return server.Run(ctx, &mcp.StdioTransport{})
}
