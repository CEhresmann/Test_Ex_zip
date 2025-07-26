package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sync/errgroup"
)

type DownloadResult struct {
	Index    int
	FilePath string
	Error    error
}

func DownloadFiles(ctx context.Context, urls []string, tempDir string, timeout time.Duration) ([]string, []error) {
	results := make(chan DownloadResult, len(urls))
	g, ctx := errgroup.WithContext(ctx)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for i, url := range urls {
		i, url := i, url
		g.Go(func() error {
			filePath, err := downloadSingleFile(ctx, url, tempDir)
			results <- DownloadResult{
				Index:    i,
				FilePath: filePath,
				Error:    err,
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, []error{err}
	}
	close(results)

	downloaded := make([]string, len(urls))
	errors := make([]error, len(urls))

	for res := range results {
		if res.Error != nil {
			errors[res.Index] = fmt.Errorf("URL %s: %w", urls[res.Index], res.Error)
		} else {
			downloaded[res.Index] = res.FilePath
		}
	}

	return downloaded, errors
}

func downloadSingleFile(ctx context.Context, url, dir string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	ext := filepath.Ext(url)
	if ext == "" {
		ext = ".bin"
	}

	file, err := os.CreateTemp(dir, "dl-*"+ext)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		os.Remove(file.Name())
		return "", err
	}

	return file.Name(), nil
}
