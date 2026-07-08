package app

import (
	"context"
)

func Execute() int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	di := newContainer()

	err := di.CommandRoot().ExecuteContext(ctx)
	if err != nil {
		reportSystemError(err)
		return 1
	}

	return 0
}
