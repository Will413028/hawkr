package categoryservice

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	categorypb "github.com/Will413028/hawkr/backend/internal/legacy/gen/category"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/category/categoryconnect"
)

type fakeCategories struct {
	categoryconnect.CategoryServiceClient
	resp *categorypb.GetCategoryTreeResponse
	err  error
	seen *[]string
}

func (f fakeCategories) GetCategoryTree(_ context.Context, req *connect.Request[categorypb.GetCategoryTreeRequest]) (*connect.Response[categorypb.GetCategoryTreeResponse], error) {
	*f.seen = append(*f.seen, req.Msg.GetStatus())
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.resp), nil
}

func TestCategoryTree(t *testing.T) {
	var seen []string
	resp := &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{
		{Id: "1", Name: "水果", Status: "ACTIVE", Children: []*categorypb.CategoryNode{
			{Id: "11", Name: "柑橘", ParentId: "1", ParentName: "水果", Status: "ACTIVE", DeletedAt: "ignored"},
		}},
	}}
	c := &Client{rpc: fakeCategories{resp: resp, seen: &seen}}
	tree, err := c.CategoryTree(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 1 || tree[0].ID != 1 || tree[0].ParentID != 0 || len(tree[0].Children) != 1 {
		t.Fatalf("tree = %+v", tree)
	}
	child := tree[0].Children[0]
	if child.ID != 11 || child.ParentID != 1 || child.ParentName != "水果" || child.Name != "柑橘" || child.Status != "ACTIVE" {
		t.Fatalf("child = %+v", child)
	}
	if _, err := c.CategoryTree(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[0] != "active" || seen[1] != "" {
		t.Fatalf("status filter = %q", seen)
	}
}

func TestCategoryTreeRejectsBadData(t *testing.T) {
	var seen []string
	for name, f := range map[string]fakeCategories{
		"transport":  {err: errors.New("offline")},
		"code":       {resp: &categorypb.GetCategoryTreeResponse{Code: 500}},
		"bad id":     {resp: &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{{Id: "x"}}}},
		"zero id":    {resp: &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{{Id: "0"}}}},
		"bad parent": {resp: &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{{Id: "1", ParentId: "p"}}}},
	} {
		f.seen = &seen
		if _, err := (&Client{rpc: f}).CategoryTree(context.Background(), true); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	deep := &categorypb.CategoryNode{Id: "1"}
	cur := deep
	for i := 0; i < maxDepth+1; i++ {
		next := &categorypb.CategoryNode{Id: "1"}
		cur.Children = []*categorypb.CategoryNode{next}
		cur = next
	}
	f := fakeCategories{resp: &categorypb.GetCategoryTreeResponse{Categories: []*categorypb.CategoryNode{deep}}, seen: &seen}
	if _, err := (&Client{rpc: f}).CategoryTree(context.Background(), true); err == nil {
		t.Error("over-deep tree accepted")
	}
	if _, err := New("", ""); err == nil {
		t.Error("empty address accepted")
	}
}
