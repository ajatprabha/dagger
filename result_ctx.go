package dagger

import (
	"context"
)

type resultCtxKey int

const resultErrKey resultCtxKey = iota

func resultErrToContext(ctx context.Context, err error) context.Context {
	return context.WithValue(ctx, resultErrKey, err)
}

func resultErrFromContext(ctx context.Context) error {
	if err, ok := ctx.Value(resultErrKey).(error); ok {
		return err
	}

	return nil
}
