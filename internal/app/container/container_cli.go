package container

import (
	"context"
)

// RunCLI executes the full cobra command tree (check, mapping, graph,
// self-inspect, version) against an in-process spec decoder.
//
// This is the machinery behind the scaffolded `.go-arch-lint/main.go`:
// the launcher delegates EVERY command name to the scaffold, so the
// scaffold must route them — previously it always ran `check`, and
// `mapping`/`graph`/`selfInspect` silently degraded to a check run
// while looking successful.
//
// args is the argument list without the binary name, WITH the command
// as the first non-flag token (already preprocessed by the app layer:
// launcher-dialect flags translated, default command injected). The
// returned error follows the archlint.Run contract: nil on success,
// models.UserSpaceError when check finds violations, anything else is
// a config/system error.
func (c *Container) RunCLI(ctx context.Context, spec SpecDecoder, args []string) error {
	c.externalDecoder = spec

	//nolint:contextcheck // ExecuteContext propagates ctx into every
	// command's RunE (cobra stores it on the command); the checker cannot
	// see through the cobra indirection and flags the call as detached.
	root := c.CommandRoot()
	root.SetArgs(args)

	return root.ExecuteContext(ctx)
}
