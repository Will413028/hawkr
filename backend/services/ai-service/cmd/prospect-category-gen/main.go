package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	treeURL := flag.String("tree-url", "http://localhost:3000/api/v1/public/categories/tree?status=active", "分類樹來源（免認證的 public endpoint）")
	treeFile := flag.String("tree-file", "", "改讀本機 JSON dump，讓跑動可重現；設了就忽略 -tree-url")
	dsn := flag.String("dsn", "", "seller DB 連線字串（省略時由 DB_HOST/DB_USER/DB_PASSWORD 組）")
	limit := flag.Int("limit", 0, "只處理前 N 筆（0 = 全部），供小樣本試跑")
	samples := flag.Int("samples", 2, "每個業者的取樣次數")
	minAgree := flag.Int("min-agree", 2, "保留出現至少這麼多次的分類 id")
	out := flag.String("out", "", "輸出的 migration SQL 路徑（空字串＝印到 stdout）")
	report := flag.String("report", "", "逐筆標註報告的輸出路徑（供人工抽樣審核）")
	fromReport := flag.String("from-report", "", "改讀既有的標註報告重新輸出 migration，不呼叫 LLM")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	err := run(ctx, options{
		treeURL: *treeURL, treeFile: *treeFile, dsn: *dsn,
		limit: *limit, samples: *samples, minAgree: *minAgree,
		out: *out, report: *report, fromReport: *fromReport,
	})
	cancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "prospect-category-gen: %v\n", err)
		os.Exit(1)
	}
}
