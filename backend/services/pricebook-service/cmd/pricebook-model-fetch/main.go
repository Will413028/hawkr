package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Will413028/hawkr/pricebook-service/internal/modelregistry"
)

var errFlagParseFailed = errors.New("flag parse failed")

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		if !errors.Is(err, errFlagParseFailed) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("pricebook-model-fetch", flag.ContinueOnError)
	registryPath := fs.String("registry", "model/registry.json", "path to registry.json")
	cacheDir := fs.String("cache-dir", "", "persistent directory to cache the verified model in between runs; required")
	out := fs.String("out", "", "path to write the verified model.json to; required")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return errFlagParseFailed
	}
	if *cacheDir == "" {
		fs.Usage()
		return fmt.Errorf("--cache-dir 為必填")
	}
	if *out == "" {
		fs.Usage()
		return fmt.Errorf("--out 為必填")
	}

	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKeyID := os.Getenv("R2_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	bucket := os.Getenv("R2_BUCKET_NAME")

	reg, err := modelregistry.LoadRegistry(*registryPath)
	if err != nil {
		return fmt.Errorf("讀取 registry 失敗: %w", err)
	}

	dl, err := modelregistry.NewR2Downloader(ctx, accountID, accessKeyID, secretAccessKey, bucket)
	if err != nil {
		return fmt.Errorf("建立 R2 client 失敗: %w", err)
	}

	cachedPath, err := modelregistry.FetchModel(ctx, reg, *cacheDir, dl)
	if err != nil {
		return fmt.Errorf("抓取 model.json 失敗: %w", err)
	}

	if err := copyFile(cachedPath, *out); err != nil {
		return fmt.Errorf("複製到 %s 失敗: %w", *out, err)
	}

	fmt.Printf("model.json 就緒：%s（model_version=%s, series_count=%d）\n", *out, reg.ModelVersion, reg.SeriesCount)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = in.Close()
	}()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
