package grpc

import (
	"testing"
	"time"

	"github.com/Will413028/hawkr/social-service/internal/usecase"

	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

func TestToPostProduct_Valid_WithMap(t *testing.T) {
	productMap := map[string]*usecase.ProductBasicInfo{
		"100": {
			ID:       "100",
			Name:     "小農有機水梨",
			ImageURL: "https://cdn.example.com/pear.jpg",
			Price:    "450",
		},
	}

	prod := toPostProduct(100, true, productMap)
	if prod == nil {
		t.Fatal("expected non-nil product")
	}
	if prod.Id != "100" {
		t.Errorf("Id = %q, want 100", prod.Id)
	}
	if prod.Name != "小農有機水梨" {
		t.Errorf("Name = %q, want %q", prod.Name, "小農有機水梨")
	}
	if prod.ImageKey != "https://cdn.example.com/pear.jpg" {
		t.Errorf("Image = %q, want %q", prod.ImageKey, "https://cdn.example.com/pear.jpg")
	}
	if prod.Price != "450" {
		t.Errorf("Price = %q, want %q", prod.Price, "450")
	}
}

func TestToPostProduct_Valid_NilMap(t *testing.T) {
	prod := toPostProduct(100, true, nil)
	if prod == nil {
		t.Fatal("expected non-nil product even with nil map")
	}
	if prod.Id != "100" {
		t.Errorf("Id = %q, want 100", prod.Id)
	}
	if prod.Name != "" {
		t.Errorf("Name = %q, want empty", prod.Name)
	}
}

func TestToPostProduct_Valid_NotInMap(t *testing.T) {
	productMap := map[string]*usecase.ProductBasicInfo{
		"999": {ID: "999", Name: "other"},
	}
	prod := toPostProduct(100, true, productMap)
	if prod != nil {
		t.Errorf("expected nil for product not found in map, got %+v", prod)
	}
}

func TestToPostProduct_Invalid(t *testing.T) {
	prod := toPostProduct(0, false, nil)
	if prod != nil {
		t.Errorf("expected nil for invalid product_id, got %+v", prod)
	}
}

func TestToPostDetailInfo_WithProduct(t *testing.T) {
	post := &gen.CmsPost{
		ID:        1,
		SellerID:  10,
		Content:   stringPointer("test content"),
		ProductID: int64Pointer(100),
	}
	productMap := map[string]*usecase.ProductBasicInfo{
		"100": {
			ID:       "100",
			Name:     "水梨",
			ImageURL: "https://cdn.example.com/pear.jpg",
			Price:    "450",
		},
	}

	info := toPostDetailInfo(post, 0, false, "賣家A", "logo.jpg", productMap)
	if info == nil {
		t.Fatal("expected non-nil detail info")
	}
	if len(info.Products) != 1 {
		t.Fatalf("Products len = %d, want 1", len(info.Products))
	}
	p := info.Products[0]
	if p.Id != "100" || p.Name != "水梨" || p.Price != "450" {
		t.Errorf("product = {Id:%q, Name:%q, Price:%q}, want {100, 水梨, 450}", p.Id, p.Name, p.Price)
	}
}

func TestToPostDetailInfo_WithoutProduct(t *testing.T) {
	post := &gen.CmsPost{
		ID:       2,
		SellerID: 10,
		Content:  stringPointer("no product"),
	}

	info := toPostDetailInfo(post, 0, false, "", "", nil)
	if info == nil {
		t.Fatal("expected non-nil detail info")
	}
	if len(info.Products) != 0 {
		t.Errorf("Products len = %d, want 0", len(info.Products))
	}
}

func TestToPostDetailInfo_ProductNotInMap(t *testing.T) {
	post := &gen.CmsPost{
		ID:        3,
		SellerID:  10,
		ProductID: int64Pointer(999),
	}
	// Empty map — simulates product deleted/unlisted
	info := toPostDetailInfo(post, 0, false, "", "", map[string]*usecase.ProductBasicInfo{})
	if info == nil {
		t.Fatal("expected non-nil detail info")
	}
	if len(info.Products) != 0 {
		t.Errorf("Products len = %d, want 0 (product not found in map should be excluded)", len(info.Products))
	}
}

func TestToPostDetailInfo_Nil(t *testing.T) {
	info := toPostDetailInfo(nil, 0, false, "", "", nil)
	if info != nil {
		t.Errorf("expected nil for nil post, got %+v", info)
	}
}

func stringPointer(value string) *string { return &value }

func int64Pointer(value int64) *int64 { return &value }

func timePointer(value time.Time) *time.Time { return &value }

// is_mine 比的是 (kind, id)：buyer 7 與 store 7 是兩個不同的人。
func TestIsMine_ComparesKindAndID(t *testing.T) {
	tests := []struct {
		name   string
		role   int16
		viewer usecase.Caller
		want   bool
	}{
		{"買家留言、觀看者是同號買家", 1, usecase.Caller{BuyerID: 7}, true},
		{"買家留言、觀看者只有同號的店", 1, usecase.Caller{StoreID: 7}, false},
		{"店家留言、觀看者是同號的店", 2, usecase.Caller{StoreID: 7}, true},
		{"店家留言、觀看者只有同號的買家", 2, usecase.Caller{BuyerID: 7}, false},
		{"店主同時有兩個身分，買家留言是自己的", 1, usecase.Caller{BuyerID: 7, StoreID: 99}, true},
		{"訪客", 1, usecase.Caller{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comment := toCommentInfo(&gen.CmsComment{ID: 1, UserID: 7, Role: tt.role}, tt.viewer, false, nil)
			if comment.IsMine != tt.want || comment.ActorKind != int32(tt.role) {
				t.Errorf("comment is_mine=%v actor_kind=%d, want %v/%d", comment.IsMine, comment.ActorKind, tt.want, tt.role)
			}
			forum := toForumEntryInfo(&gen.CmsSellerForum{ID: 1, UserID: 7, Role: tt.role}, tt.viewer)
			if forum.IsMine != tt.want || forum.ActorKind != int32(tt.role) {
				t.Errorf("forum is_mine=%v actor_kind=%d, want %v/%d", forum.IsMine, forum.ActorKind, tt.want, tt.role)
			}
		})
	}
}
