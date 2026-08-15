package mcp

import (
	"context"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// RunStdio serves the same registry over stdio. This is a developer adapter:
// logs go to stderr (never stdout), and it is not required in the production
// image (ADR 0006 / first-GA default).
func (s *Server) RunStdio(ctx context.Context) error {
	return s.run(ctx, &sdk.StdioTransport{})
}

func (s *Server) run(ctx context.Context, t sdk.Transport) error {
	return s.sdk.Run(ctx, t)
}
