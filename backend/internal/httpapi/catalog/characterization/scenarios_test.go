//go:build gateway

package characterization

import (
	"context"
	"errors"
	"slices"
	"strings"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
)

// scenarios 是 Java product-service 要通過的情境，Driver 的實作各自跑同一份。資料庫可能已經有別的情境建的
// 商品，所以每個情境只看名稱帶自己 nonce 的那幾筆。期望值來自 ProductService、ProductGrpcService 與 gateway 的
// handler/product.go、seller_product.go、search.go、hashtag.go，鎖定的是現況，沒有驗證現況是否正確。
//
// 標 skipOnLegacy 的是計畫決定要改的行為（D 編號）：舊的一端跳過並附編號，單體的一端照新行為斷言。
// 編輯與刪除都先下架再做：Java 只允許編輯、刪除草稿或已下架的商品（ProductService.java:527、:2759），新的
// 生命週期（D16）同樣允許，情境不鎖定上架中能不能直接編輯。
var scenarios = []ch.Scenario[Driver]{
	{Name: "seller API rejects anonymous callers and non-sellers", Run: func(ctx context.Context, t ch.T, d Driver) {
		_, err := d.SellerList(ctx, ch.Anonymous, "")
		ch.WantKind(t, err, ch.ErrUnauthenticated)
		_, err = d.SellerList(ctx, ch.Buyer, "")
		ch.WantKind(t, err, ch.ErrPermission)
		_, err = d.SellerCreate(ctx, ch.Buyer, ProductInput{Name: "x", Price: 100, Stock: 1})
		ch.WantKind(t, err, ch.ErrPermission)
	}},
	{Name: "admin API rejects non-admins", Run: func(ctx context.Context, t ch.T, d Driver) {
		_, err := d.AdminList(ctx, ch.Anonymous, "")
		ch.WantKind(t, err, ch.ErrUnauthenticated)
		_, err = d.AdminList(ctx, ch.Seller, "")
		ch.WantKind(t, err, ch.ErrPermission)
	}},
	{Name: "category tree has leaves to attach products to", Run: func(ctx context.Context, t ch.T, d Driver) {
		cats := ch.Must(t, ch.Ret(d.Categories(ctx, ch.Anonymous)))
		ids := map[string]bool{}
		for _, c := range cats {
			if c.ID == "" || c.Name == "" {
				t.Fatalf("分類缺 ID 或名稱：%+v", c)
			}
			ids[c.ID] = true
		}
		for _, c := range cats {
			if c.ParentID != "" && !ids[c.ParentID] {
				t.Errorf("分類 %s 的上層 %s 不在樹裡", c.ID, c.ParentID)
			}
		}
		second := false
		for _, c := range cats {
			second = second || (c.Leaf && c.ParentID != "")
		}
		if !second {
			t.Fatalf("分類樹沒有第二層的葉節點（商品掛第二層，D8）：%+v", cats)
		}
	}},
	{Name: "seller reads back what was created", Run: func(ctx context.Context, t ch.T, d Driver) {
		n := ch.Nonce()
		leaf := leafOf(t, ch.Must(t, ch.Ret(d.Categories(ctx, ch.Seller))))
		in := ProductInput{Name: n + " 茶葉", CategoryID: leaf, Price: 300, ListPrice: 450, Stock: 7, Hashtags: []string{"t" + n}}
		created := ch.Must(t, ch.Ret(d.SellerCreate(ctx, ch.Seller, in)))
		if created.ID == "" {
			t.Fatalf("建立的商品沒有 ID")
		}
		got := ch.Must(t, ch.Ret(d.SellerGet(ctx, ch.Seller, created.ID)))
		if got.ID != created.ID || got.Name != in.Name || got.CategoryID != leaf {
			t.Errorf("讀回 %+v，預期 ID %s、名稱 %q、分類 %s", got, created.ID, in.Name, leaf)
		}
		if got.PriceMin != 300 || got.PriceMax != 300 || got.Stock != 7 {
			t.Errorf("讀回價格 %d–%d、庫存 %d，預期 300–300、7", got.PriceMin, got.PriceMax, got.Stock)
		}
		if !slices.Equal(got.Hashtags, []string{"t" + n}) {
			t.Errorf("讀回 hashtag %v，預期 [t%s]", got.Hashtags, n)
		}
		list := ch.Must(t, ch.Ret(d.SellerList(ctx, ch.Seller, n)))
		if ids(list) != created.ID {
			t.Errorf("賣家列表 %v，預期只有 %s", ids(list), created.ID)
		}
	}},
	{Name: "new product starts as a draft", Run: func(ctx context.Context, t ch.T, d Driver) {
		if skipOnLegacy(t, d, "D16：Java 建立即上架（ProductService.java:383）") {
			return
		}
		p := createIn(ctx, t, d, ch.Seller, ch.Nonce())
		if p.Status != StatusDraft {
			t.Errorf("新建商品的狀態 %s，預期 %s", p.Status, StatusDraft)
		}
		_, err := d.PublicGet(ctx, ch.Anonymous, p.ID)
		ch.WantKind(t, err, ch.ErrNotFound)
	}},
	{Name: "listed product is public and unlisting hides it from lists", Run: func(ctx context.Context, t ch.T, d Driver) {
		n := ch.Nonce()
		p := listed(ctx, t, d, ch.Seller, n)
		if got := ch.Must(t, ch.Ret(d.SellerGet(ctx, ch.Seller, p.ID))); got.Status != StatusActive {
			t.Errorf("上架後狀態 %s，預期 %s", got.Status, StatusActive)
		}
		pub := ch.Must(t, ch.Ret(d.PublicGet(ctx, ch.Anonymous, p.ID)))
		if pub.Name != p.Name || !pub.Purchasable {
			t.Errorf("公開商品頁 %+v，預期名稱 %q 而且可購買", pub, p.Name)
		}
		if got := ids(ch.Must(t, ch.Ret(d.PublicList(ctx, ch.Anonymous, PublicQuery{Keyword: n})))); got != p.ID {
			t.Errorf("公開列表 %v，預期只有 %s", got, p.ID)
		}

		ch.Must(t, ch.Ret(struct{}{}, d.SellerSetListed(ctx, ch.Seller, p.ID, false)))
		if got := ch.Must(t, ch.Ret(d.SellerGet(ctx, ch.Seller, p.ID))); got.Status != StatusArchived {
			t.Errorf("下架後狀態 %s，預期 %s", got.Status, StatusArchived)
		}
		if got := ids(ch.Must(t, ch.Ret(d.PublicList(ctx, ch.Anonymous, PublicQuery{Keyword: n})))); got != "" {
			t.Errorf("下架後公開列表 %v，預期沒有", got)
		}
		pub = ch.Must(t, ch.Ret(d.PublicGet(ctx, ch.Anonymous, p.ID)))
		if pub.Purchasable {
			t.Errorf("下架後商品頁仍可購買：%+v", pub)
		}
		if got := ids(ch.Must(t, ch.Ret(d.SellerList(ctx, ch.Seller, n)))); got != p.ID {
			t.Errorf("下架後賣家列表 %v，預期仍有 %s", got, p.ID)
		}

		ch.Must(t, ch.Ret(struct{}{}, d.SellerSetListed(ctx, ch.Seller, p.ID, true)))
		if got := ids(ch.Must(t, ch.Ret(d.PublicList(ctx, ch.Anonymous, PublicQuery{Keyword: n})))); got != p.ID {
			t.Errorf("重新上架後公開列表 %v，預期 %s", got, p.ID)
		}
	}},
	{Name: "seller edits an unlisted product", Run: func(ctx context.Context, t ch.T, d Driver) {
		n := ch.Nonce()
		p := listed(ctx, t, d, ch.Seller, n)
		ch.Must(t, ch.Ret(struct{}{}, d.SellerSetListed(ctx, ch.Seller, p.ID, false)))
		in := ProductInput{Name: n + " 改名", CategoryID: p.CategoryID, Price: 520, Stock: 3, Hashtags: []string{"u" + n}}
		ch.Must(t, ch.Ret(d.SellerUpdate(ctx, ch.Seller, p.ID, in)))
		got := ch.Must(t, ch.Ret(d.SellerGet(ctx, ch.Seller, p.ID)))
		if got.Name != in.Name || got.PriceMin != 520 || got.PriceMax != 520 || got.Stock != 3 {
			t.Errorf("編輯後讀回 %+v，預期名稱 %q、價格 520、庫存 3", got, in.Name)
		}
		if !slices.Equal(got.Hashtags, []string{"u" + n}) {
			t.Errorf("編輯後 hashtag %v，預期 [u%s]", got.Hashtags, n)
		}
	}},
	{Name: "deleted product disappears everywhere", Run: func(ctx context.Context, t ch.T, d Driver) {
		n := ch.Nonce()
		p := listed(ctx, t, d, ch.Seller, n)
		ch.Must(t, ch.Ret(struct{}{}, d.SellerSetListed(ctx, ch.Seller, p.ID, false)))
		ch.Must(t, ch.Ret(struct{}{}, d.SellerDelete(ctx, ch.Seller, p.ID)))
		_, err := d.SellerGet(ctx, ch.Seller, p.ID)
		ch.WantKind(t, err, ch.ErrNotFound)
		if got := ids(ch.Must(t, ch.Ret(d.SellerList(ctx, ch.Seller, n)))); got != "" {
			t.Errorf("刪除後賣家列表 %v，預期沒有", got)
		}
		_, err = d.PublicGet(ctx, ch.Anonymous, p.ID)
		ch.WantKind(t, err, ch.ErrNotFound)
	}},
	{Name: "unknown product is not found", Run: func(ctx context.Context, t ch.T, d Driver) {
		_, err := d.SellerGet(ctx, ch.Seller, d.Unknown())
		ch.WantKind(t, err, ch.ErrNotFound)
		_, err = d.PublicGet(ctx, ch.Anonymous, d.Unknown())
		ch.WantKind(t, err, ch.ErrNotFound)
	}},
	{Name: "another store cannot read or change the product", Run: func(ctx context.Context, t ch.T, d Driver) {
		n := ch.Nonce()
		p := listed(ctx, t, d, ch.Seller, n)
		ch.Must(t, ch.Ret(struct{}{}, d.SellerSetListed(ctx, ch.Seller, p.ID, false)))
		_, err := d.SellerGet(ctx, ch.Seller2, p.ID)
		ch.WantKind(t, err, ch.ErrNotFound)
		_, err = d.SellerUpdate(ctx, ch.Seller2, p.ID, ProductInput{Name: n + " 偷改", CategoryID: p.CategoryID, Price: 1, Stock: 1})
		wantDenied(t, d, err)
		wantDenied(t, d, d.SellerSetListed(ctx, ch.Seller2, p.ID, true))
		wantDenied(t, d, d.SellerDelete(ctx, ch.Seller2, p.ID))
		if got := ids(ch.Must(t, ch.Ret(d.SellerList(ctx, ch.Seller2, n)))); got != "" {
			t.Errorf("別家店的賣家列表 %v，預期沒有", got)
		}
		got := ch.Must(t, ch.Ret(d.SellerGet(ctx, ch.Seller, p.ID)))
		if got.Name != p.Name || got.Status != StatusArchived {
			t.Errorf("別家店操作後商品變成 %+v", got)
		}
	}},
	{Name: "public list filters by store and price", Run: func(ctx context.Context, t ch.T, d Driver) {
		n := ch.Nonce()
		cheap := listedWith(ctx, t, d, ch.Seller, ProductInput{Name: n + " 便宜", Price: 100, Stock: 5})
		dear := listedWith(ctx, t, d, ch.Seller2, ProductInput{Name: n + " 昂貴", Price: 900, Stock: 5})
		if got := ids(ch.Must(t, ch.Ret(d.PublicList(ctx, ch.Anonymous, PublicQuery{Keyword: n})))); got != join(cheap.ID, dear.ID) {
			t.Errorf("公開列表 %v，預期 %s", got, join(cheap.ID, dear.ID))
		}
		byStore := ch.Must(t, ch.Ret(d.PublicList(ctx, ch.Anonymous, PublicQuery{Keyword: n, StoreID: d.StoreOf(ch.Seller2)})))
		if got := ids(byStore); got != dear.ID {
			t.Errorf("以店家篩選 %v，預期只有 %s", got, dear.ID)
		}
		for _, p := range byStore {
			if p.StoreID != d.StoreOf(ch.Seller2) {
				t.Errorf("以店家篩選的商品 %s 的店家 %s，預期 %s", p.ID, p.StoreID, d.StoreOf(ch.Seller2))
			}
		}
		if got := ids(ch.Must(t, ch.Ret(d.PublicList(ctx, ch.Anonymous, PublicQuery{Keyword: n, PriceMin: 500})))); got != dear.ID {
			t.Errorf("最低價 500 篩選 %v，預期只有 %s", got, dear.ID)
		}
		if got := ids(ch.Must(t, ch.Ret(d.PublicList(ctx, ch.Anonymous, PublicQuery{Keyword: n, PriceMax: 500})))); got != cheap.ID {
			t.Errorf("最高價 500 篩選 %v，預期只有 %s", got, cheap.ID)
		}
	}},
	{Name: "public list filters by category", Run: func(ctx context.Context, t ch.T, d Driver) {
		n := ch.Nonce()
		p := listed(ctx, t, d, ch.Seller, n)
		if got := ids(ch.Must(t, ch.Ret(d.PublicList(ctx, ch.Anonymous, PublicQuery{Keyword: n, CategoryID: p.CategoryID})))); got != p.ID {
			t.Errorf("以分類篩選 %v，預期 %s", got, p.ID)
		}
		other := ""
		for _, c := range ch.Must(t, ch.Ret(d.Categories(ctx, ch.Anonymous))) {
			if c.Leaf && c.ID != p.CategoryID {
				other = c.ID
				break
			}
		}
		if other == "" {
			return
		}
		if got := ids(ch.Must(t, ch.Ret(d.PublicList(ctx, ch.Anonymous, PublicQuery{Keyword: n, CategoryID: other})))); got != "" {
			t.Errorf("以別的分類篩選 %v，預期沒有", got)
		}
	}},
	{Name: "search and suggestions find listed products by name", Run: func(ctx context.Context, t ch.T, d Driver) {
		n := ch.Nonce()
		p := listed(ctx, t, d, ch.Seller, n)
		if got := ids(ch.Must(t, ch.Ret(d.Search(ctx, ch.Anonymous, n)))); got != p.ID {
			t.Errorf("搜尋 %q 得到 %v，預期 %s", n, got, p.ID)
		}
		if got := ch.Must(t, ch.Ret(d.Suggest(ctx, ch.Anonymous, n))); !slices.Contains(got, p.Name) {
			t.Errorf("建議 %q 得到 %v，預期含 %q", n, got, p.Name)
		}
		ch.Must(t, ch.Ret(struct{}{}, d.SellerSetListed(ctx, ch.Seller, p.ID, false)))
		if got := ids(ch.Must(t, ch.Ret(d.Search(ctx, ch.Anonymous, n)))); got != "" {
			t.Errorf("下架後搜尋 %q 得到 %v，預期沒有", n, got)
		}
	}},
	{Name: "hashtag autocomplete counts listed products", Run: func(ctx context.Context, t ch.T, d Driver) {
		n := ch.Nonce()
		tag := "h" + n
		listedWith(ctx, t, d, ch.Seller, ProductInput{Name: n + " 一", Price: 100, Stock: 1, Hashtags: []string{tag}})
		listedWith(ctx, t, d, ch.Seller2, ProductInput{Name: n + " 二", Price: 100, Stock: 1, Hashtags: []string{tag}})
		got := ch.Must(t, ch.Ret(d.HashtagAutocomplete(ctx, ch.Anonymous, tag[:5])))
		var found *Hashtag
		for i := range got {
			if got[i].Name == tag {
				found = &got[i]
			}
		}
		if found == nil || found.Count != 2 {
			t.Errorf("自動完成 %q 得到 %+v，預期 %s 有 2 件", tag[:5], got, tag)
		}
	}},
	{Name: "hashtags are normalized to lower case", Run: func(ctx context.Context, t ch.T, d Driver) {
		n := ch.Nonce()
		p := createWith(ctx, t, d, ch.Seller, ProductInput{Name: n + " 大寫", Price: 100, Stock: 1, Hashtags: []string{"Mix" + n}})
		got := ch.Must(t, ch.Ret(d.SellerGet(ctx, ch.Seller, p.ID)))
		if !slices.Equal(got.Hashtags, []string{"mix" + n}) {
			t.Errorf("hashtag %v，預期 [mix%s]", got.Hashtags, n)
		}
	}},
	{Name: "invalid product input is rejected", Run: func(ctx context.Context, t ch.T, d Driver) {
		n := ch.Nonce()
		leaf := leafOf(t, ch.Must(t, ch.Ret(d.Categories(ctx, ch.Seller))))
		for name, in := range map[string]ProductInput{
			"沒有名稱":    {Name: "", CategoryID: leaf, Price: 100, Stock: 1},
			"沒有分類":    {Name: n + " 無分類", Price: 100, Stock: 1},
			"價格為 0":   {Name: n + " 零元", CategoryID: leaf, Price: 0, Stock: 1},
			"原價不高於售價": {Name: n + " 原價低", CategoryID: leaf, Price: 300, ListPrice: 300, Stock: 1},
			"庫存為負":    {Name: n + " 負庫存", CategoryID: leaf, Price: 100, Stock: -1},
		} {
			_, err := d.SellerCreate(ctx, ch.Seller, in)
			if !errors.Is(err, ch.ErrInvalid) {
				t.Errorf("%s：err = %v，預期 %v", name, err, ch.ErrInvalid)
			}
		}
		if got := ids(ch.Must(t, ch.Ret(d.SellerList(ctx, ch.Seller, n)))); got != "" {
			t.Errorf("被拒絕的輸入建出了商品：%v", got)
		}
	}},
	{Name: "admin sees products of every store", Run: func(ctx context.Context, t ch.T, d Driver) {
		n := ch.Nonce()
		a := createIn(ctx, t, d, ch.Seller, n)
		b := listed(ctx, t, d, ch.Seller2, n)
		if got := ids(ch.Must(t, ch.Ret(d.AdminList(ctx, ch.Admin, n)))); got != join(a.ID, b.ID) {
			t.Errorf("後台列表 %v，預期 %s", got, join(a.ID, b.ID))
		}
		got := ch.Must(t, ch.Ret(d.AdminGet(ctx, ch.Admin, b.ID)))
		if got.ID != b.ID || got.Name != b.Name {
			t.Errorf("後台詳情 %+v，預期 %s %q", got, b.ID, b.Name)
		}
		_, err := d.AdminGet(ctx, ch.Admin, d.Unknown())
		ch.WantKind(t, err, ch.ErrNotFound)
	}},
}

// wantDenied 確認別家店的寫入被拒：單體回 not_found（越權存取比照不存在，API 設計規範 §二）。
func wantDenied(t ch.T, d Driver, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("別家店的寫入成功了")
	}
	ch.WantKind(t, err, ch.ErrNotFound)
}

// skipOnLegacy 在舊的一端略過計畫決定要改的行為。
func skipOnLegacy(t ch.T, d Driver, why string) bool {
	if !d.Legacy() {
		return false
	}
	s, ok := t.(interface{ Skipf(string, ...any) })
	if !ok {
		panic("skipOnLegacy：這個 T 不能跳過，情境不能在舊的一端靜默通過")
	}
	s.Skipf("舊的 product-service 的行為不同：%s", why)
	return true
}

// leafOf 回傳一個掛得了商品的葉節點：優先有上層的（D8 的兩層分類，商品掛第二層；Java 同樣要第二層）。
func leafOf(t ch.T, cats []Category) string {
	t.Helper()
	top := ""
	for _, c := range cats {
		if !c.Leaf {
			continue
		}
		if c.ParentID != "" {
			return c.ID
		}
		if top == "" {
			top = c.ID
		}
	}
	return top
}

// createWith 建立商品；沒給分類時掛在第一個葉節點。
func createWith(ctx context.Context, t ch.T, d Driver, c ch.Caller, in ProductInput) Product {
	t.Helper()
	if in.CategoryID == "" {
		in.CategoryID = leafOf(t, ch.Must(t, ch.Ret(d.Categories(ctx, c))))
	}
	return ch.Must(t, ch.Ret(d.SellerCreate(ctx, c, in)))
}

func createIn(ctx context.Context, t ch.T, d Driver, c ch.Caller, nonce string) Product {
	t.Helper()
	return createWith(ctx, t, d, c, ProductInput{Name: nonce + " 商品", Price: 200, Stock: 10})
}

// listedWith 建立並上架；建立後已經上架（Java）就不再上架一次。
func listedWith(ctx context.Context, t ch.T, d Driver, c ch.Caller, in ProductInput) Product {
	t.Helper()
	p := createWith(ctx, t, d, c, in)
	if p.Status != StatusActive {
		ch.Must(t, ch.Ret(struct{}{}, d.SellerSetListed(ctx, c, p.ID, true)))
		p.Status = StatusActive
	}
	return p
}

func listed(ctx context.Context, t ch.T, d Driver, c ch.Caller, nonce string) Product {
	t.Helper()
	return listedWith(ctx, t, d, c, ProductInput{Name: nonce + " 商品", Price: 200, Stock: 10})
}

// ids 把商品 ID 排序後以逗號串起，方便比對集合。
func ids(ps []Product) string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.ID)
	}
	return join(out...)
}

func join(ids ...string) string {
	s := slices.Clone(ids)
	slices.Sort(s)
	return strings.Join(s, ",")
}
