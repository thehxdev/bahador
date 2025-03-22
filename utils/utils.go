package utils

import (
	"context"
	"io"
	"log"
	"os"
)

func MustBeNil(err error) {
	if err != nil {
		panic(err)
	}
}

func GetNonEmptyEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("empty environment variable: %s", key)
	}
	return v
}

func CopyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (written int64, err error) {
	errChan := make(chan error, 1)
	go func() {
		written, err = io.Copy(dst, src)
		errChan <- err
	}()
	select {
	case <-errChan:
		return written, err
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}
