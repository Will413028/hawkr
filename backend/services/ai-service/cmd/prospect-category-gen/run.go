package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"go.uber.org/zap"
)

type options struct {
	treeURL, treeFile, dsn string
	limit, samples         int
	minAgree               int
	out, report            string
	fromReport             string
}

type annotation struct {
	ProspectID  int64    `json:"prospect_id"`
	DisplayName string   `json:"display_name"`
	Category    string   `json:"category"`
	Summary     string   `json:"product_summary"`
	CategoryIDs []string `json:"category_ids"`
	Labels      []string `json:"labels"`
	Dropped     []string `json:"dropped,omitempty"`
}

func run(ctx context.Context, o options) error {
	if o.fromReport != "" {
		return renderFromReport(o)
	}
	nodes, err := loadTree(ctx, o.treeURL, o.treeFile)
	if err != nil {
		return err
	}
	byID := make(map[string]node, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}

	prospects, err := loadProspects(ctx, o.dsn, o.limit)
	if err != nil {
		return err
	}
	if len(prospects) == 0 {
		return fmt.Errorf("no ACTIVE prospects found")
	}

	provider, err := newProvider()
	if err != nil {
		return err
	}
	src := newModelTagSource(provider)

	annotations := make([]annotation, 0, len(prospects))
	var noTag int
	for i, p := range prospects {
		rounds := make([][]string, 0, o.samples)
		for range o.samples {
			got, err := src.Tags(ctx, p, nodes)
			if err != nil {
				return fmt.Errorf("tagging %s(%d): %w", p.DisplayName, p.ID, err)
			}
			rounds = append(rounds, got)
		}
		agreed, dropped := consensus(rounds, o.minAgree, byID)
		if len(agreed) == 0 {
			noTag++
		}
		labels := make([]string, 0, len(agreed))
		for _, id := range agreed {
			labels = append(labels, byID[id].Name)
		}
		annotations = append(annotations, annotation{
			ProspectID: p.ID, DisplayName: p.DisplayName, Category: p.Category,
			Summary: p.ProductSummary, CategoryIDs: agreed, Labels: labels, Dropped: dropped,
		})
		fmt.Fprintf(os.Stderr, "\r標註中 %d/%d", i+1, len(prospects))
	}
	fmt.Fprintf(os.Stderr, "\n")

	fmt.Fprintf(os.Stderr, "標註完成：%d 筆，其中 %d 筆沒有任何分類\n", len(annotations), noTag)
	printDistribution(annotations, byID)

	if o.report != "" {
		blob, err := json.MarshalIndent(annotations, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(o.report, blob, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "逐筆報告：%s\n", o.report)
	}

	sql := renderMigration(annotations)
	if o.out == "" {
		fmt.Print(sql)
		return nil
	}
	return os.WriteFile(o.out, []byte(sql), 0o644)
}

func newProvider() (llmprovider.Procurement, error) {
	var zero llmprovider.Procurement
	cfg, err := config.Load()
	if err != nil {
		return zero, err
	}
	logger, _ := zap.NewDevelopment()
	p := llmprovider.NewProcurement(cfg, logger)
	if !p.Enabled() {
		return p, fmt.Errorf("procurement LLM 未設定：檢查 EXTRACTOR_MODEL 與對應的 API key")
	}
	return p, nil
}

func consensus(rounds [][]string, min int, byID map[string]node) (agreed, dropped []string) {
	counts := map[string]int{}
	var order []string
	for _, round := range rounds {
		seen := map[string]bool{}
		for _, raw := range round {
			id := strings.TrimSpace(raw)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			if counts[id] == 0 {
				order = append(order, id)
			}
			counts[id]++
		}
	}
	for _, id := range order {
		switch {
		case byID[id].ID == "":
			dropped = append(dropped, id+"（不在分類樹）")
		case counts[id] < min:
			dropped = append(dropped, fmt.Sprintf("%s（只出現 %d 次）", byID[id].Name, counts[id]))
		default:
			agreed = append(agreed, id)
		}
	}
	sort.Strings(agreed)
	return agreed, dropped
}

func printDistribution(annotations []annotation, byID map[string]node) {
	perL1 := map[string]int{}
	perLevel := map[int]int{}
	droppedReasons := map[string]int{}
	for _, a := range annotations {
		for _, id := range a.CategoryIDs {
			perL1[byID[id].L1Name]++
			perLevel[byID[id].Level]++
		}
		for _, d := range a.Dropped {
			if i := strings.Index(d, "（"); i >= 0 {
				droppedReasons[d[i:]]++
			}
		}
	}
	fmt.Fprintf(os.Stderr, "層級分布：L1=%d L2=%d\n", perLevel[1], perLevel[2])
	fmt.Fprintf(os.Stderr, "L1 分布：")
	keys := make([]string, 0, len(perL1))
	for k := range perL1 {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, " %s=%d", k, perL1[k])
	}
	fmt.Fprintf(os.Stderr, "\n丟棄理由分布：")
	for reason, n := range droppedReasons {
		fmt.Fprintf(os.Stderr, " %s=%d", reason, n)
	}
	fmt.Fprintf(os.Stderr, "\n")
}

func renderFromReport(o options) error {
	blob, err := os.ReadFile(o.fromReport)
	if err != nil {
		return err
	}
	var annotations []annotation
	if err := json.Unmarshal(blob, &annotations); err != nil {
		return fmt.Errorf("decoding report: %w", err)
	}
	sql := renderMigration(annotations)
	if o.out == "" {
		fmt.Print(sql)
		return nil
	}
	return os.WriteFile(o.out, []byte(sql), 0o644)
}
