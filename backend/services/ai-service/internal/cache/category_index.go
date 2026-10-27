package cache

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/client/category"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"
	"github.com/Will413028/hawkr/go-pkg/syncutil"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

const categoryIndexTTL = time.Hour

var synonyms = map[string]string{

	"輪霧":  "蓮霧",
	"蕃茄":  "番茄",
	"番荔枝": "釋迦",

	"芋頭":  "芋",
	"地瓜":  "甘薯",
	"地瓜葉": "甘薯葉",
	"高麗菜": "甘藍",
	"空心菜": "蕹菜",
	"四季豆": "敏豆",
	"洋芋":  "馬鈴薯",
	"紅蘿蔔": "胡蘿蔔",
	"香菇":  "濕香菇",
	"木耳":  "濕木耳",
	"秋葵":  "黃秋葵",
	"蔥":   "青蔥",
	"大白菜": "包心白",
	"白花椰": "花椰菜",
	"花菜":  "花椰菜",
	"青花菜": "青花苔",
	"青椒":  "甜椒",
	"玉米筍": "玉米",

	"芭樂": "番石榴",
	"文旦": "柚子",
	"橘子": "椪柑",
	"柳橙": "甜橙",
	"梨子": "梨",
	"李子": "李",

	"豬五花": "豬肉",
	"五花肉": "豬肉",
	"三層肉": "豬肉",
	"梅花肉": "豬肉",
	"胛心肉": "豬肉",
	"里肌肉": "豬肉",
	"大里肌": "豬肉",
	"小里肌": "豬肉",
	"腰內肉": "豬肉",
	"腿庫":  "豬肉",
	"後腿肉": "豬肉",
	"蹄膀":  "豬肉",
	"豬腳":  "豬肉",
	"松阪豬": "豬肉",
	"豬頸肉": "豬肉",
	"豬絞肉": "豬肉",
	"豬小排": "豬肉",
	"排骨":  "豬肉",
	"肋排":  "豬肉",
	"豬骨":  "豬肉",

	"雞胸肉": "雞肉",
	"棒棒腿": "雞肉",
	"清雞肉": "雞肉",
	"雞翅":  "雞肉",
	"三節翅": "雞肉",
	"雞絞肉": "雞肉",

	"白肉雞": "雞肉",
	"毛豬":  "豬肉",
	"肉鵝":  "鵝肉",
	"白羅曼": "鵝肉",
	"土番鴨": "鴨肉",
	"正番鴨": "鴨肉",

	"糯米": "米",
	"稉種": "米",
	"硬秈": "米",
}

type CategoryCandidate struct {
	CatalogID  string
	Path       string
	Confidence string
}

type leafEntry struct {
	catalogID string
	path      string
}

type CategoryIndex struct {
	index map[string][]leafEntry

	pathByID map[string]string

	tree      *categorypb.GetCategoryTreeResponse
	client    *category.Client
	sfg       singleflight.Group
	updatedAt time.Time
	mu        sync.RWMutex
	ready     bool
}

func NewReadyCategoryIndex(_ *category.Client) *CategoryIndex {
	return &CategoryIndex{
		index:     map[string][]leafEntry{},
		updatedAt: time.Now(),
		ready:     true,
	}
}

func NewCategoryIndex(client *category.Client) *CategoryIndex {
	ci := &CategoryIndex{client: client}
	ctx := context.Background()
	const maxAttempts = 3
	for attempt := range maxAttempts {
		if attempt > 0 {
			time.Sleep(2 * time.Second)
		}
		if err := ci.refresh(ctx); err == nil {
			return ci
		} else if attempt == maxAttempts-1 {
			pkglogger.Warn(ctx, "[CategoryIndex] warm-up 失敗，服務標記未就緒", zap.Error(err))
		}
	}
	return ci
}

func (ci *CategoryIndex) Ready() bool {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.ready
}

func (ci *CategoryIndex) Match(keyword string, top int) []CategoryCandidate {
	ci.triggerRefreshIfExpired()

	ci.mu.RLock()
	idx := ci.index
	ci.mu.RUnlock()

	normalized := normalize(keyword)
	if canonical, ok := synonyms[normalized]; ok {
		normalized = normalize(canonical)
	}

	entries := ci.lookup(idx, normalized)
	if top <= 0 {
		top = 3
	}
	if len(entries) > top {
		entries = entries[:top]
	}

	results := make([]CategoryCandidate, 0, len(entries))
	for i, e := range entries {
		confidence := confidenceScore(i, len(entries))
		results = append(results, CategoryCandidate{
			CatalogID:  e.catalogID,
			Path:       e.path,
			Confidence: confidence,
		})
	}
	return results
}

func (ci *CategoryIndex) lookup(idx map[string][]leafEntry, keyword string) []leafEntry {
	if keyword == "" {
		return nil
	}

	if entries, ok := idx[keyword]; ok {
		return entries
	}

	var results []leafEntry
	for k, v := range idx {
		if strings.Contains(k, keyword) || strings.Contains(keyword, k) {
			results = append(results, v...)
		}
	}
	return results
}

func confidenceScore(rank, total int) string {
	if total == 0 {
		return "0.5"
	}

	score := 0.90 - float64(rank)*0.05
	if score < 0.5 {
		score = 0.5
	}
	return fmt.Sprintf("%.2f", score)
}

func normalize(s string) string {
	return strings.TrimSpace(s)
}

func (ci *CategoryIndex) triggerRefreshIfExpired() {
	ci.mu.RLock()
	ready := ci.ready
	expired := time.Since(ci.updatedAt) > categoryIndexTTL
	ci.mu.RUnlock()

	if ready && !expired {
		return
	}

	syncutil.SafeGo(context.Background(), func() {
		_, _, _ = ci.sfg.Do("refresh", func() (interface{}, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := ci.refresh(ctx); err != nil {
				pkglogger.Warn(ctx, "[CategoryIndex] TTL refresh 失敗，繼續使用舊 index", zap.Error(err))
			}
			return nil, nil
		})
	})
}

func (ci *CategoryIndex) refresh(ctx context.Context) error {
	resp, err := ci.client.GetCategoryTree(ctx, "active")
	if err != nil {
		return fmt.Errorf("GetCategoryTree: %w", err)
	}

	newIndex := make(map[string][]leafEntry)
	newPaths := make(map[string]string)
	for _, root := range resp.GetCategories() {
		walkNode(root, "", newIndex)
		walkPaths(root, "", newPaths)
	}

	ci.mu.Lock()
	ci.index = newIndex
	ci.pathByID = newPaths
	ci.tree = resp
	ci.updatedAt = time.Now()
	ci.ready = true
	ci.mu.Unlock()
	return nil
}

func (ci *CategoryIndex) GetCategoryTree(ctx context.Context, status string) (*categorypb.GetCategoryTreeResponse, error) {
	if status != "active" {
		if ci.client == nil {
			return nil, fmt.Errorf("category client unavailable")
		}
		return ci.client.GetCategoryTree(ctx, status)
	}

	ci.mu.RLock()
	tree, ready := ci.tree, ci.ready
	ci.mu.RUnlock()
	if ready && tree != nil {
		ci.triggerRefreshIfExpired()
		return tree, nil
	}

	if ci.client == nil {
		return nil, fmt.Errorf("active category tree unavailable")
	}
	if _, err, _ := ci.sfg.Do("refresh", func() (interface{}, error) {
		return nil, ci.refresh(ctx)
	}); err != nil {
		return nil, err
	}

	ci.mu.RLock()
	defer ci.mu.RUnlock()
	if ci.tree == nil {
		return nil, fmt.Errorf("active category tree unavailable")
	}
	return ci.tree, nil
}

func ManualSynonym(s string) (string, bool) {
	canonical, ok := synonyms[normalize(s)]
	return canonical, ok
}

func CanonicalCategoryName(s string) string {
	key := normalize(s)
	if canonical, ok := synonyms[key]; ok {
		return canonical
	}
	if canonical, ok := generatedCategoryAliases[key]; ok {
		return canonical
	}
	return s
}

func (ci *CategoryIndex) PathOf(categoryID string) string {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.pathByID[categoryID]
}

func walkPaths(node *categorypb.CategoryNode, parentPath string, out map[string]string) {
	if node == nil {
		return
	}
	path := node.GetName()
	if parentPath != "" {
		path = parentPath + " > " + node.GetName()
	}
	if id := node.GetId(); id != "" {
		out[id] = path
	}
	for _, child := range node.GetChildren() {
		walkPaths(child, path, out)
	}
}

func walkNode(node *categorypb.CategoryNode, parentPath string, idx map[string][]leafEntry) {
	if node == nil {
		return
	}
	path := node.GetName()
	if parentPath != "" {
		path = parentPath + " > " + node.GetName()
	}

	children := node.GetChildren()
	if len(children) == 0 {

		key := normalize(node.GetName())
		idx[key] = append(idx[key], leafEntry{
			catalogID: node.GetId(),
			path:      path,
		})
		return
	}

	if parentPath != "" {
		key := normalize(node.GetName())
		for _, child := range children {
			childPath := path + " > " + child.GetName()
			idx[key] = append(idx[key], leafEntry{
				catalogID: child.GetId(),
				path:      childPath,
			})
		}
	}

	for _, child := range children {
		walkNode(child, path, idx)
	}
}
