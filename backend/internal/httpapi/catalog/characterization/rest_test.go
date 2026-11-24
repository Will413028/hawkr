//go:build gateway

package characterization

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
)

// restDriver 以舊 gateway 的 REST API（統一 envelope）操作舊的 Java product-service：路徑與 wire 型別照
// gateway/internal/handler 的 product.go、seller_product.go、search.go、hashtag.go、category.go、admin_product.go，
// r.rest.Do 送出請求並以 ch.RESTKind 分類錯誤。Java 的業務錯誤是 HTTP 200 加 envelope code，先在
// restBusinessKind 對應。product-service 刪除時一起刪除。
type restDriver struct {
	rest ch.REST
}

var _ Driver = (*restDriver)(nil)

func newRESTDriver(base string, tokens map[ch.Caller]string, client *http.Client) *restDriver {
	return &restDriver{rest: ch.REST{Base: base, Tokens: tokens, HTTP: client, Business: restBusinessKind}}
}

// restBusinessKind 對應 GrpcResponseCode 裡 product-service 用到的業務碼。
func restBusinessKind(code int32) error {
	switch code {
	case 4002:
		return ErrWrongStatus
	case 4012:
		return ch.ErrNotFound
	case 9240:
		return ch.ErrPermission
	}
	return nil
}

func (d *restDriver) Legacy() bool { return true }

func (d *restDriver) StoreOf(c ch.Caller) string {
	if c == ch.Seller2 {
		return StoreSeller2
	}
	return StoreSeller
}

func (d *restDriver) Unknown() string { return "1" }

const (
	publicProducts = "/api/v1/public/products"
	appProducts    = "/api/v1/app/products"
	adminProducts  = "/api/v1/admin/products"
	listPageSize   = "100"
)

// statusWire 是 CodeMeta 的商品狀態（4001 草稿、4003 未啟用、4004 啟用）；4002 審核中不在新的生命週期裡（D16）。
var statusWire = map[Status]string{StatusDraft: "4001", StatusArchived: "4003", StatusActive: "4004"}

func statusFromWire(wire string) Status {
	for k, v := range statusWire {
		if v == wire {
			return k
		}
	}
	return Status("?" + wire)
}

type wireHashtag struct {
	Name         string `json:"name"`
	ProductCount string `json:"productCount"`
}

// wireProduct 是詳情（ProductDetailItem）與列表項（SellerProductItem、ProductItem、ProductHitData）共同用到的欄位。
type wireProduct struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	StoreID     string        `json:"storeId"`
	CategoryID  string        `json:"categoryId"`
	Status      string        `json:"status"`
	PriceMin    string        `json:"priceMin"`
	PriceMax    string        `json:"priceMax"`
	Stock       string        `json:"stock"`
	Purchasable bool          `json:"purchasable"`
	Hashtags    []wireHashtag `json:"hashtags"`
	Skus        []wireSku     `json:"skus"`
}

type wireSku struct {
	ID string `json:"id"`
}

// numReader 讀數字字串；空字串是 0，其餘解析失敗記下第一個錯誤。
type numReader struct{ err error }

func (r *numReader) int(s string) int64 {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil && r.err == nil {
		r.err = fmt.Errorf("wire 的數字欄位不是整數：%q：%w", s, err)
	}
	return n
}

func (w wireProduct) product() (Product, error) {
	var r numReader
	p := Product{
		ID: w.ID, Name: w.Name, StoreID: w.StoreID, CategoryID: w.CategoryID,
		Status:   statusFromWire(w.Status),
		PriceMin: r.int(w.PriceMin), PriceMax: r.int(w.PriceMax), Stock: r.int(w.Stock),
		Purchasable: w.Purchasable,
	}
	for _, h := range w.Hashtags {
		p.Hashtags = append(p.Hashtags, h.Name)
	}
	return p, r.err
}

func products(ws []wireProduct) ([]Product, error) {
	out := make([]Product, 0, len(ws))
	for _, w := range ws {
		p, err := w.product()
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

type wirePage struct {
	Records []wireProduct `json:"records"`
}

func (d *restDriver) list(ctx context.Context, c ch.Caller, path string, q url.Values) ([]Product, error) {
	q.Set("page_size", listPageSize)
	var page wirePage
	if err := d.rest.Do(ctx, c, http.MethodGet, path+"?"+q.Encode(), nil, &page); err != nil {
		return nil, err
	}
	return products(page.Records)
}

func (d *restDriver) get(ctx context.Context, c ch.Caller, path string) (Product, error) {
	var w wireProduct
	if err := d.rest.Do(ctx, c, http.MethodGet, path, nil, &w); err != nil {
		return Product{}, err
	}
	return w.product()
}

type wireCategory struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	ParentID string          `json:"parentId"`
	Children []*wireCategory `json:"children"`
}

func (d *restDriver) Categories(ctx context.Context, c ch.Caller) ([]Category, error) {
	var tree []*wireCategory
	if err := d.rest.Do(ctx, c, http.MethodGet, "/api/v1/public/categories/tree", nil, &tree); err != nil {
		return nil, err
	}
	var out []Category
	var walk func(nodes []*wireCategory)
	walk = func(nodes []*wireCategory) {
		for _, n := range nodes {
			parent := n.ParentID
			if parent == "0" {
				parent = ""
			}
			out = append(out, Category{ID: n.ID, Name: n.Name, ParentID: parent, Leaf: len(n.Children) == 0})
			walk(n.Children)
		}
	}
	walk(tree)
	return out, nil
}

// wireSkuInput 是 CreateSkuInput／UpdateSkuInput：重量、單位與尺寸是 gateway 的必填，情境不看，送固定值。price 是原價、
// salePrice 是特價：ListPrice 有值時 price 送 ListPrice、salePrice 送 Price。
type wireSkuInput struct {
	ID        string `json:"id,omitempty"`
	SkuName   string `json:"skuName"`
	Weight    string `json:"weight"`
	Unit      string `json:"unit"`
	Price     string `json:"price"`
	SalePrice string `json:"salePrice,omitempty"`
	StockQty  string `json:"stockQty"`
	LengthCm  string `json:"lengthCm"`
	WidthCm   string `json:"widthCm"`
	HeightCm  string `json:"heightCm"`
}

func skuInput(id string, in ProductInput) wireSkuInput {
	s := wireSkuInput{
		ID: id, SkuName: "預設", Weight: "500", Unit: "g",
		Price: strconv.FormatInt(in.Price, 10), StockQty: strconv.FormatInt(in.Stock, 10),
		LengthCm: "10", WidthCm: "10", HeightCm: "10",
	}
	if in.ListPrice != 0 {
		s.Price, s.SalePrice = strconv.FormatInt(in.ListPrice, 10), strconv.FormatInt(in.Price, 10)
	}
	return s
}

type wireProductInput struct {
	IdempotencyKey string         `json:"idempotencyKey"`
	Name           string         `json:"name,omitempty"`
	CategoryID     string         `json:"categoryId,omitempty"`
	Skus           []wireSkuInput `json:"skus"`
	Hashtags       []string       `json:"hashtags,omitempty"`
	UpdateHashtags bool           `json:"updateHashtags,omitempty"`
}

func (d *restDriver) SellerCreate(ctx context.Context, c ch.Caller, in ProductInput) (Product, error) {
	body := wireProductInput{
		IdempotencyKey: ch.Nonce() + ch.Nonce(), Name: in.Name, CategoryID: in.CategoryID,
		Skus: []wireSkuInput{skuInput("", in)}, Hashtags: in.Hashtags,
	}
	var w wireProduct
	if err := d.rest.Do(ctx, c, http.MethodPost, appProducts, body, &w); err != nil {
		return Product{}, err
	}
	return w.product()
}

func (d *restDriver) SellerGet(ctx context.Context, c ch.Caller, id string) (Product, error) {
	return d.get(ctx, c, appProducts+"/"+url.PathEscape(id))
}

func (d *restDriver) SellerList(ctx context.Context, c ch.Caller, keyword string) ([]Product, error) {
	q := url.Values{}
	if keyword != "" {
		q.Set("keyword", keyword)
	}
	return d.list(ctx, c, appProducts, q)
}

// SellerUpdate 以呼叫者讀到的唯一 SKU 的 ID 編輯（gateway 要既有 SKU 帶 id；讀不到時以不存在的 ID 送出，讓
// product-service 自己判斷擁有者）。
func (d *restDriver) SellerUpdate(ctx context.Context, c ch.Caller, id string, in ProductInput) (Product, error) {
	skuID := d.Unknown()
	var cur wireProduct
	if err := d.rest.Do(ctx, c, http.MethodGet, appProducts+"/"+url.PathEscape(id), nil, &cur); err == nil && len(cur.Skus) == 1 {
		skuID = cur.Skus[0].ID
	}
	body := wireProductInput{
		IdempotencyKey: ch.Nonce() + ch.Nonce(), Name: in.Name, CategoryID: in.CategoryID,
		Skus: []wireSkuInput{skuInput(skuID, in)}, Hashtags: in.Hashtags, UpdateHashtags: true,
	}
	var w wireProduct
	if err := d.rest.Do(ctx, c, http.MethodPut, appProducts+"/"+url.PathEscape(id), body, &w); err != nil {
		return Product{}, err
	}
	return w.product()
}

func (d *restDriver) SellerSetListed(ctx context.Context, c ch.Caller, id string, listed bool) error {
	status := statusWire[StatusArchived]
	if listed {
		status = statusWire[StatusActive]
	}
	return d.rest.Do(ctx, c, http.MethodPut, appProducts+"/"+url.PathEscape(id)+"/status", map[string]string{"status": status}, nil)
}

func (d *restDriver) SellerDelete(ctx context.Context, c ch.Caller, id string) error {
	return d.rest.Do(ctx, c, http.MethodDelete, appProducts+"/"+url.PathEscape(id), nil, nil)
}

func (d *restDriver) PublicGet(ctx context.Context, c ch.Caller, id string) (Product, error) {
	return d.get(ctx, c, publicProducts+"/"+url.PathEscape(id))
}

func (d *restDriver) PublicList(ctx context.Context, c ch.Caller, pq PublicQuery) ([]Product, error) {
	q := url.Values{}
	set := func(k, v string) {
		if v != "" && v != "0" {
			q.Set(k, v)
		}
	}
	set("keyword", pq.Keyword)
	set("category_id", pq.CategoryID)
	set("store_ids", pq.StoreID)
	set("price_min", strconv.FormatInt(pq.PriceMin, 10))
	set("price_max", strconv.FormatInt(pq.PriceMax, 10))
	return d.list(ctx, c, publicProducts, q)
}

func (d *restDriver) Search(ctx context.Context, c ch.Caller, keyword string) ([]Product, error) {
	return d.list(ctx, c, "/api/v1/public/search/products", url.Values{"keyword": {keyword}})
}

func (d *restDriver) Suggest(ctx context.Context, c ch.Caller, prefix string) ([]string, error) {
	var out struct {
		Suggestions []string `json:"suggestions"`
	}
	err := d.rest.Do(ctx, c, http.MethodGet, "/api/v1/public/search/suggest?"+url.Values{"q": {prefix}}.Encode(), nil, &out)
	return out.Suggestions, err
}

func (d *restDriver) HashtagAutocomplete(ctx context.Context, c ch.Caller, prefix string) ([]Hashtag, error) {
	var out struct {
		Items []wireHashtag `json:"items"`
	}
	if err := d.rest.Do(ctx, c, http.MethodGet, "/api/v1/public/hashtags?"+url.Values{"prefix": {prefix}}.Encode(), nil, &out); err != nil {
		return nil, err
	}
	var r numReader
	tags := make([]Hashtag, 0, len(out.Items))
	for _, h := range out.Items {
		tags = append(tags, Hashtag{Name: h.Name, Count: r.int(h.ProductCount)})
	}
	return tags, r.err
}

func (d *restDriver) AdminList(ctx context.Context, c ch.Caller, keyword string) ([]Product, error) {
	q := url.Values{}
	if keyword != "" {
		q.Set("keyword", keyword)
	}
	return d.list(ctx, c, adminProducts, q)
}

func (d *restDriver) AdminGet(ctx context.Context, c ch.Caller, id string) (Product, error) {
	return d.get(ctx, c, adminProducts+"/"+url.PathEscape(id))
}
