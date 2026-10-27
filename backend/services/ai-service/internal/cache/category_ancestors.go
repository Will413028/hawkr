package cache

import (
	"context"

	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"
)

func (ci *CategoryIndex) AncestorIDs(ctx context.Context, categoryID string) []string {
	if categoryID == "" {
		return nil
	}
	tree, err := ci.GetCategoryTree(ctx, "active")
	if err != nil || tree == nil {
		return nil
	}
	for _, l1 := range tree.GetCategories() {
		if l1.GetId() == categoryID {
			return []string{l1.GetId()}
		}
		for _, l2 := range l1.GetChildren() {
			if l2.GetId() == categoryID {
				return []string{l2.GetId(), l1.GetId()}
			}
			for _, l3 := range l2.GetChildren() {
				if l3.GetId() == categoryID {
					return []string{l3.GetId(), l2.GetId(), l1.GetId()}
				}
			}
		}
	}
	return nil
}

func (ci *CategoryIndex) setTreeForTest(tree *categorypb.GetCategoryTreeResponse) {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	ci.tree = tree
	ci.ready = true
}
