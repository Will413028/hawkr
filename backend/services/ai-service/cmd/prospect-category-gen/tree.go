package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

type node struct {
	ID     string
	Name   string
	Level  int
	L1Name string
	L2Name string
}

type rawNode struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Status   string    `json:"status"`
	Children []rawNode `json:"children"`
}

func activeNode(n rawNode) bool {
	return n.ID != "" && n.Name != "" && n.Status == codemeta.CommonStatusActive.Value
}

const maxL2PerL1 = 15

func loadTree(ctx context.Context, url, file string) ([]node, error) {
	var raw []byte
	var err error
	if file != "" {
		raw, err = os.ReadFile(file)
	} else {
		raw, err = fetchTree(ctx, url)
	}
	if err != nil {
		return nil, fmt.Errorf("loading category tree: %w", err)
	}
	var envelope struct {
		Data []rawNode `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decoding category tree: %w", err)
	}
	if len(envelope.Data) == 0 {
		return nil, fmt.Errorf("category tree has no L1 nodes")
	}
	var out []node
	for _, l1 := range envelope.Data {
		if !activeNode(l1) {
			continue
		}
		out = append(out, node{ID: l1.ID, Name: l1.Name, Level: 1, L1Name: l1.Name})
		var l2s []node
		for _, l2 := range l1.Children {
			if !activeNode(l2) {
				continue
			}
			l2s = append(l2s, node{ID: l2.ID, Name: l2.Name, Level: 2, L1Name: l1.Name, L2Name: l2.Name})
		}
		if len(l2s) <= maxL2PerL1 {
			out = append(out, l2s...)
		}
	}
	return out, nil
}

func fetchTree(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "prospect-category-gen")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("category tree returned %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
