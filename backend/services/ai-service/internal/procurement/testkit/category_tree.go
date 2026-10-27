package testkit

import (
	"context"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	categorypb "github.com/Will413028/hawkr/go-pkg/proto/category"
)

// FakeCategoryTree 是品項解析用的分類樹替身。Calls 數的是 GetCategoryTree 被呼叫的次數，
// 供「這一輪有沒有去查分類樹」的斷言用；一次解析可能讀樹不只一次，所以只斷言零或非零。
type FakeCategoryTree struct {
	Response *categorypb.GetCategoryTreeResponse
	Err      error
	Calls    int
}

func (f *FakeCategoryTree) GetCategoryTree(_ context.Context, _ string) (*categorypb.GetCategoryTreeResponse, error) {
	f.Calls++
	return f.Response, f.Err
}

// VegetableTree：蔬菜 > 葉菜類 > {高麗菜, 小白菜}，加上 蔬菜 > 根莖類 > 地瓜。
func VegetableTree() *FakeCategoryTree {
	active := codemeta.CommonStatusActive.Value
	return &FakeCategoryTree{Response: &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		{Id: "1", Name: "蔬菜", Status: active, Children: []*categorypb.CategoryNode{
			{Id: "4", Name: "葉菜類", ParentId: "1", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "17", Name: "高麗菜", ParentId: "4", Status: active},
				{Id: "18", Name: "小白菜", ParentId: "4", Status: active},
			}},
			{Id: "5", Name: "根莖類", ParentId: "1", Status: active, Children: []*categorypb.CategoryNode{
				{Id: "20", Name: "地瓜", ParentId: "5", Status: active},
			}},
		}},
	}}}
}
