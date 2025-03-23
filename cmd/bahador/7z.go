package main

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// maxPartSize ::= <number>[b|k|m|g]
func SplitFileToParts(ctx context.Context, filePath, outPath, maxPartSize string) ([]string, error) {
	if !strings.HasSuffix(outPath, ".7z") {
		return nil, errors.New("output path must be a file path with .7z extention")
	}

	cmdCtx, cmdCancel := context.WithTimeout(ctx, time.Minute*15)
	defer cmdCancel()

	args := []string{"a", "-t7z", "-m0=lzma2", "-mx=1", "-v"+maxPartSize, "-sdel", outPath, filePath}
	errChan := make(chan error, 1)
	go func() {
		cmd := exec.Command("7zz", args...)
		err := cmd.Start()
		if err != nil {
			errChan <- err
			return
		}
		err = cmd.Wait()
		errChan <- err
	}()

	select {
	case err := <-errChan:
		if err != nil {
			return nil, err
		}
	case <-cmdCtx.Done():
		return nil, cmdCtx.Err()
	}

	outDir := filepath.Dir(outPath)
	files, err := filepath.Glob(outDir+"/*.7z.*")
	if err != nil {
		return nil, err
	}

	return files, nil
}
