package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// node 是攤平後的分類節點。L1Name 帶著是為了讓 prompt 能告訴模型「甘藍在蔬菜
// 之下，不是花卉」，以及讓生成檔的行內註解可讀。
type node struct {
	ID       string
	Name     string
	Level    int
	L1Name   string
	Children []node
}

// tree 分成兩份刻意不同的集合：
//   - All：所有層級、**所有 L1**（含被排除的），供「別名不得撞既有節點名」的
//     比對與樹快照使用。排除花卉只影響產別名的對象，不影響撞名範圍。
//   - L2：要產別名的目標，已排除指定 L1 與非 ACTIVE 節點。
type tree struct {
	All []node
	L2  []node
}

type rawNode struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	ParentID string    `json:"parentId"`
	Status   string    `json:"status"`
	Children []rawNode `json:"children"`
}

func active(n rawNode) bool {
	return n.ID != "" && n.Name != "" && n.Status == codemeta.CommonStatusActive.Value
}

func parseTree(raw []byte, excludeL1 []string) (tree, error) {
	var envelope struct {
		Data []rawNode `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return tree{}, fmt.Errorf("decoding category tree: %w", err)
	}
	if len(envelope.Data) == 0 {
		return tree{}, fmt.Errorf("category tree has no L1 nodes")
	}

	var out tree
	for _, l1 := range envelope.Data {
		if !active(l1) {
			continue
		}
		out.All = append(out.All, node{ID: l1.ID, Name: l1.Name, Level: 1, L1Name: l1.Name})
		skip := slices.Contains(excludeL1, l1.Name)
		for _, l2 := range l1.Children {
			if !active(l2) {
				continue
			}
			out.All = append(out.All, node{ID: l2.ID, Name: l2.Name, Level: 2, L1Name: l1.Name})
			group := node{ID: l2.ID, Name: l2.Name, Level: 2, L1Name: l1.Name}
			for _, l3 := range l2.Children {
				if !active(l3) {
					continue
				}
				child := node{ID: l3.ID, Name: l3.Name, Level: 3, L1Name: l1.Name}
				out.All = append(out.All, child)
				group.Children = append(group.Children, child)
			}
			if !skip {
				out.L2 = append(out.L2, group)
			}
		}
	}
	if len(out.L2) == 0 {
		return tree{}, fmt.Errorf("category tree has no eligible L2 nodes after exclusions")
	}
	return out, nil
}

func fetchTree(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("category tree endpoint returned %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
