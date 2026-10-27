package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/cache"
	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"go.uber.org/zap"
)

func main() {
	treeURL := flag.String("tree-url", "http://localhost:3000/api/v1/public/categories/tree?status=active", "分類樹來源（免認證的 public endpoint）")
	treeFile := flag.String("tree-file", "", "改讀本機 JSON dump，讓跑動可重現；設了就忽略 -tree-url")
	excludeL1 := flag.String("exclude-l1", "花卉", "不產別名的 L1，逗號分隔")
	samples := flag.Int("samples", 3, "每個節點的取樣次數")
	minAgree := flag.Int("min-agree", 2, "保留出現至少這麼多次的別名")
	out := flag.String("out", "internal/cache/category_alias_gen.go", "生成的 Go 檔輸出路徑")
	snapshotOut := flag.String("snapshot-out", "internal/cache/testdata/category_tree_snapshot.json", "分類樹快照輸出路徑")
	flag.Parse()

	if err := run(*treeURL, *treeFile, *excludeL1, *samples, *minAgree, *out, *snapshotOut); err != nil {
		fmt.Fprintf(os.Stderr, "category-alias-gen: %v\n", err)
		os.Exit(1)
	}
}

func run(treeURL, treeFile, excludeL1 string, samples, minAgree int, out, snapshotOut string) error {
	ctx := context.Background()

	var raw []byte
	var err error
	if treeFile != "" {
		raw, err = os.ReadFile(treeFile)
	} else {
		raw, err = fetchTree(ctx, treeURL)
	}
	if err != nil {
		return fmt.Errorf("loading category tree: %w", err)
	}

	var excluded []string
	for _, name := range strings.Split(excludeL1, ",") {
		if name = strings.TrimSpace(name); name != "" {
			excluded = append(excluded, name)
		}
	}
	t, err := parseTree(raw, excluded)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "分類樹：%d 個節點，其中 %d 個 L2 要產別名（排除 %s）\n",
		len(t.All), len(t.L2), strings.Join(excluded, "、"))

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	provider := llmprovider.NewProcurement(cfg, zap.NewNop())
	if !provider.Enabled() {
		return fmt.Errorf("procurement LLM 未開通（檢查 EXTRACTOR_MODEL 與對應金鑰）")
	}

	cands, err := generateAll(ctx, newModelAliasSource(provider), t, samples, minAgree)
	if err != nil {
		return err
	}

	accepted, rejected := validateCandidates(cands, t, cache.ManualAliasKeys())
	for _, r := range rejected {
		fmt.Fprintf(os.Stderr, "丟棄 %q → %q：%s\n", r.Alias, r.Target, r.Reason)
	}
	fmt.Fprintf(os.Stderr, "共識後 %d 條，通過檢查 %d 條，丟棄 %d 條\n", len(cands), len(accepted), len(rejected))

	aliasSrc, err := renderAliasFile(accepted, renderMeta{
		Model: provider.ModelName(), Samples: samples, MinAgree: minAgree,
		L2Count: len(t.L2), NodeCount: len(t.All),
		GeneratedAt: time.Now().UTC().Format("2006-01-02"), ExcludedL1: excluded,
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, aliasSrc, 0o644); err != nil {
		return err
	}

	snap, err := renderSnapshot(t)
	if err != nil {
		return err
	}
	return os.WriteFile(snapshotOut, snap, 0o644)
}
