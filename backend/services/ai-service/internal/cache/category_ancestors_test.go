package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"
)

func ancestorTestTree() *categorypb.GetCategoryTreeResponse {
	active := codemeta.CommonStatusActive.Value
	return &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		{Id: "4", Name: "肉類", Status: active, Children: []*categorypb.CategoryNode{
			{Id: "198", Name: "豬肉", ParentId: "4", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "2008", Name: "豬肉", ParentId: "198", Status: active},
			}},
		}},
		{Id: "1", Name: "蔬菜", Status: active, Children: []*categorypb.CategoryNode{
			{Id: "10", Name: "包心白", ParentId: "1", Status: active},
		}},
	}}
}

func TestAncestorIDs_L3ExpandsToItsWholeChain(t *testing.T) {
	ci := NewReadyCategoryIndex(nil)
	ci.setTreeForTest(ancestorTestTree())

	require.Equal(t, []string{"2008", "198", "4"}, ci.AncestorIDs(context.Background(), "2008"))
}

func TestAncestorIDs_L2AndL1StopAtTheirOwnLevel(t *testing.T) {
	ci := NewReadyCategoryIndex(nil)
	ci.setTreeForTest(ancestorTestTree())

	require.Equal(t, []string{"198", "4"}, ci.AncestorIDs(context.Background(), "198"))
	require.Equal(t, []string{"4"}, ci.AncestorIDs(context.Background(), "4"))
}

func TestAncestorIDs_UnknownOrEmptyIsNil(t *testing.T) {
	ci := NewReadyCategoryIndex(nil)
	ci.setTreeForTest(ancestorTestTree())

	require.Nil(t, ci.AncestorIDs(context.Background(), "9999"))
	require.Nil(t, ci.AncestorIDs(context.Background(), ""))
}
