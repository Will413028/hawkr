//go:build integration

package seller_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// prospectAdmin 是管理員的 ID（沒有 FK，只記是誰做的）。
const prospectAdmin = int64(9001)

// newProspect 建立一個業者，mutate 改輸入。名稱預設唯一（T3 的名稱相同要刻意製造）。
func (f *fixture) newProspect(t *testing.T, name string, mutate func(*CreateProspectInput)) Prospect {
	t.Helper()
	in := CreateProspectInput{AdminID: prospectAdmin, Fields: ProspectFields{DisplayName: name, VendorType: VendorBusiness}}
	if mutate != nil {
		mutate(&in)
	}
	p, err := f.module.CreateProspect(context.Background(), in)
	must(t, err)
	return p
}

// contactList 把每個有值的欄位變成該類型唯一的（也是主要的）聯絡方式。
func contactList(email, phone, website, social string) []ContactPoint {
	var out []ContactPoint
	for _, c := range []ContactPoint{{Kind: ContactEmail, Value: email}, {Kind: ContactPhone, Value: phone}, {Kind: ContactWebsite, Value: website}, {Kind: ContactSocial, Value: social}} {
		if c.Value != "" {
			c.Primary = true
			out = append(out, c)
		}
	}
	return out
}

func withContact(email, phone, website, social string) func(*CreateProspectInput) {
	return func(in *CreateProspectInput) { in.Contacts = contactList(email, phone, website, social) }
}

func text(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// contactsOf 把聯絡方式寫成「類型:值」，主要的加 *，排序後比較。
func contactsOf(p Prospect) []string {
	out := make([]string, 0, len(p.Contacts))
	for _, c := range p.Contacts {
		s := string(c.Kind) + ":" + c.Value
		if c.Primary {
			s += "*"
		}
		out = append(out, s)
	}
	slices.Sort(out)
	return out
}

// matchKeys 是業者的聯絡方式在資料庫裡的比對鍵（解析器的輸出，不對外）。
func (f *fixture) matchKeys(t *testing.T, id int64) []string {
	t.Helper()
	ctx := context.Background()
	rows, err := f.db.Conn(ctx).Query(ctx, "SELECT match_key FROM seller.prospect_contacts WHERE prospect_id = $1 AND match_key IS NOT NULL ORDER BY match_key", id)
	must(t, err)
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		must(t, rows.Scan(&k))
		keys = append(keys, k)
	}
	must(t, rows.Err())
	return keys
}

func (f *fixture) prospectRow(t *testing.T, id int64) Prospect {
	t.Helper()
	p, err := f.module.Prospect(context.Background(), id)
	must(t, err)
	return p
}

// archive 以目前的版本封存。
func (f *fixture) archive(t *testing.T, id int64) Prospect {
	t.Helper()
	p, err := f.module.ArchiveProspect(context.Background(), ArchiveProspectInput{ProspectID: id, AdminID: prospectAdmin, ExpectedVersion: f.prospectRow(t, id).Version})
	must(t, err)
	return p
}

// merge 以兩邊目前的版本合併。
func (f *fixture) merge(t *testing.T, winner, loser int64) Prospect {
	t.Helper()
	p, err := f.module.MergeProspects(context.Background(), f.mergeInput(t, winner, loser))
	must(t, err)
	return p
}

func (f *fixture) mergeInput(t *testing.T, winner, loser int64) MergeProspectsInput {
	t.Helper()
	return MergeProspectsInput{WinnerID: winner, LoserID: loser, AdminID: prospectAdmin, WinnerVersion: f.prospectRow(t, winner).Version, LoserVersion: f.prospectRow(t, loser).Version}
}

func (f *fixture) restore(t *testing.T, id int64, confirmed ...int64) (Prospect, error) {
	t.Helper()
	return f.module.RestoreProspect(context.Background(), RestoreProspectInput{ProspectID: id, AdminID: prospectAdmin, ConfirmedDistinct: confirmed})
}

// distinctPairs 是資料庫裡全部「不是同一家」的配對（小的在前，依序）。
func (f *fixture) distinctPairs(t *testing.T) [][2]int64 {
	t.Helper()
	ctx := context.Background()
	rows, err := f.db.Conn(ctx).Query(ctx, "SELECT low_prospect_id, high_prospect_id FROM seller.prospect_distinct_pairs ORDER BY 1, 2")
	must(t, err)
	defer rows.Close()
	var out [][2]int64
	for rows.Next() {
		var p [2]int64
		must(t, rows.Scan(&p[0], &p[1]))
		out = append(out, p)
	}
	must(t, rows.Err())
	return out
}

// duplicatePairs 走完疑似重複清單的每一頁（每頁 1 筆，驗證分頁不漏不重）。
func (f *fixture) duplicatePairs(t *testing.T) [][2]int64 {
	t.Helper()
	var pairs [][2]int64
	var after *DuplicateCursor
	for pages := 0; ; pages++ {
		page, err := f.module.DuplicateProspects(context.Background(), 1, after)
		must(t, err)
		if len(page.Items) > 1 {
			t.Fatalf("page size 1 returned %d", len(page.Items))
		}
		for _, p := range page.Items {
			pairs = append(pairs, [2]int64{p.Left.ID, p.Right.ID})
		}
		if page.Next == nil {
			return pairs
		}
		if pages > 20 {
			t.Fatal("pagination does not terminate")
		}
		after = page.Next
	}
}

// insertSharedKey 繞過模組，把一個聯絡方式（比對鍵相同）直接寫給另一個業者：模擬並行建立漏網的重複。
func (f *fixture) insertContact(t *testing.T, prospectID int64, kind, value, matchKey string, primary bool) {
	t.Helper()
	ctx := context.Background()
	_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO seller.prospect_contacts (prospect_id, kind, value, match_key, is_primary) VALUES ($1, $2, $3, NULLIF($4, ''), $5)`, prospectID, kind, value, matchKey, primary)
	must(t, err)
}

// wantUniqueViolation 確認錯誤是資料庫的唯一性違反（繞過應用層直接寫入時的保底）。
func wantUniqueViolation(t *testing.T, err error, constraint string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != constraint {
		t.Fatalf("direct insert: %v, want unique violation of %s", err, constraint)
	}
}

func wantKind(t *testing.T, err error, kind apperr.Kind) {
	t.Helper()
	if apperr.KindOf(err) != kind {
		t.Fatalf("err=%v, want kind %v", err, kind)
	}
}

// wantMatchFound 確認錯誤是 PROSPECT_MATCH_FOUND（FailedPrecondition），而且 metadata 列出的相符業者剛好是 ids（由小到大）。
func wantMatchFound(t *testing.T, err error, ids ...int64) {
	t.Helper()
	labels := make([]string, len(ids))
	for i, id := range ids {
		labels[i] = strconv.FormatInt(id, 10)
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindFailedPrecondition || ae.Reason != apperr.ReasonProspectMatchFound ||
		ae.Metadata["prospectIds"] != strings.Join(labels, ",") {
		t.Fatalf("err=%v, want a match with prospects %v", err, ids)
	}
}

func (f *fixture) prospectCount(t *testing.T) int64 {
	return f.count(t, "SELECT count(*) FROM seller.prospects")
}

func TestCreateProspectNormalizesKeysAndStoresDisplayValues(t *testing.T) {
	t.Parallel()
	f := setup(t)
	p := f.newProspect(t, " 範例小農 ", func(in *CreateProspectInput) {
		withContact(" Hello@Example.COM ", "+886-2-1234-5678", "https://www.example.com/shop?a=1", "https://www.threads.com/@dailyfarm")(in)
		in.Fields.TaxID = text("12345678")
		in.CategoryIDs = []string{"fruit", " veg ", "fruit"}
	})
	if p.DisplayName != "範例小農" || p.Status != ProspectStatusActive || p.OutreachStatus != OutreachNotContacted || p.Version != 1 || *p.CreatedByAdminID != prospectAdmin || *p.UpdatedByAdminID != prospectAdmin {
		t.Fatalf("prospect=%+v", p)
	}
	wantContacts := []string{"EMAIL:Hello@Example.COM*", "PHONE:+886-2-1234-5678*", "SOCIAL:https://www.threads.com/@dailyfarm*", "WEBSITE:https://www.example.com/shop?a=1*"}
	if got := contactsOf(p); !slices.Equal(got, wantContacts) {
		t.Fatalf("contacts=%v, want %v", got, wantContacts)
	}
	wantKeys := []string{"DOMAIN:example.com", "EMAIL:hello@example.com", "PHONE:0212345678", "SOCIAL:threads:dailyfarm"}
	if got := f.matchKeys(t, p.ID); !slices.Equal(got, wantKeys) {
		t.Fatalf("keys=%v, want %v", got, wantKeys)
	}
	if !slices.Equal(p.CategoryIDs, []string{"fruit", "veg"}) {
		t.Fatalf("categories=%v", p.CategoryIDs)
	}
}

// P1 的解析器沿用：佔位值不是比對鍵（聯絡方式照原值存），所以幾百筆「待確認」不會被比對成同一家。
func TestPlaceholderContactsNeverMatch(t *testing.T) {
	t.Parallel()
	f := setup(t)
	for i := range 3 {
		p := f.newProspect(t, "業者"+strconv.Itoa(i), withContact("待確認", "待確認", "N/A", "-"))
		if len(p.Contacts) != 4 || len(f.matchKeys(t, p.ID)) != 0 {
			t.Fatalf("contacts=%v keys=%v", contactsOf(p), f.matchKeys(t, p.ID))
		}
	}
	if pairs := f.duplicatePairs(t); len(pairs) != 0 {
		t.Fatalf("placeholders produced duplicate pairs: %v", pairs)
	}
}

// D6：共用平台的網址（連結聚合頁、電商店面）不是識別力：兩家各自的 linktr.ee 或 pinkoi 頁面都建得起來，不必確認，也不是疑似重複。
func TestSharedPlatformWebsitesAreNotMatchKeys(t *testing.T) {
	t.Parallel()
	f := setup(t)
	a := f.newProspect(t, "甲", withContact("", "", "https://linktr.ee/a", ""))
	b := f.newProspect(t, "乙", withContact("", "", "https://linktr.ee/b", ""))
	c := f.newProspect(t, "丙", withContact("", "", "https://www.pinkoi.com/store/c", ""))
	d := f.newProspect(t, "丁", withContact("", "", "https://www.pinkoi.com/store/d", ""))
	for _, p := range []Prospect{a, b, c, d} {
		if len(p.Contacts) != 1 || len(f.matchKeys(t, p.ID)) != 0 {
			t.Fatalf("prospect %d contacts=%v keys=%v", p.ID, contactsOf(p), f.matchKeys(t, p.ID))
		}
	}
	if pairs := f.duplicatePairs(t); len(pairs) != 0 {
		t.Fatalf("pairs=%v", pairs)
	}
	matches, err := f.module.PreviewProspect(context.Background(), ProspectPreviewInput{DisplayName: "新的", Contacts: contactList("", "", "https://linktr.ee/a", "")})
	must(t, err)
	if len(matches) != 0 {
		t.Fatalf("preview=%+v", matches)
	}
}

// T1：統編在 live 的業者之間唯一，由應用層擋下（說明是誰），資料庫是保底。
func TestTaxIDIsUniqueAmongLiveProspects(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	first := f.newProspect(t, "甲", func(in *CreateProspectInput) { in.Fields.TaxID = text("12345678") })
	_, err := f.module.CreateProspect(ctx, CreateProspectInput{AdminID: prospectAdmin, Fields: ProspectFields{DisplayName: "乙", VendorType: VendorBusiness, TaxID: text("12345678")}})
	wantKind(t, err, apperr.KindAlreadyExists)
	// 資料庫本身也拒絕（繞過應用層）。
	_, err = f.db.Conn(ctx).Exec(ctx, `INSERT INTO seller.prospects (display_name, vendor_type, tax_id, created_at, updated_at) VALUES ('丙', 'BUSINESS', '12345678', now(), now())`)
	wantUniqueViolation(t, err, "prospects_tax_id")
	if _, err := f.module.CreateProspect(ctx, CreateProspectInput{AdminID: prospectAdmin, Fields: ProspectFields{DisplayName: "丁", VendorType: VendorBusiness, TaxID: text("1234567")}}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("bad format: %v", err)
	}
	// 修改撞到統編是 FailedPrecondition（該做的是合併），確認不是同一家也繞不過它。
	other := f.newProspect(t, "戊", nil)
	_, err = f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: other.ID, ExpectedVersion: other.Version, AdminID: prospectAdmin, TaxID: ptr("12345678"), ConfirmedDistinct: []int64{first.ID}})
	wantKind(t, err, apperr.KindFailedPrecondition)
	// 封存之後統編讓出來。
	f.archive(t, first.ID)
	f.newProspect(t, "乙", func(in *CreateProspectInput) { in.Fields.TaxID = text("12345678") })
}

// D3：聯絡方式的比對鍵相同是比對訊號、不是唯一鍵：大小寫、電話格式、網址的差異都是同一個鍵；Create 回 PROSPECT_MATCH_FOUND
// 列出相符的業者，確認不是同一家才建立，並記下配對；之後這一對不再擋、不再出現在疑似重複清單。
func TestSharedContactKeyNeedsConfirmationThenIsRemembered(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	holder := f.newProspect(t, "甲", withContact("Sales@Example.com", "02-1234-5678", "https://www.example.com", ""))
	before := f.prospectCount(t)
	for name, contacts := range map[string][]ContactPoint{
		"email 大小寫": contactList(" SALES@example.COM", "", "", ""),
		"電話 886 國碼": contactList("", "+886 2 1234 5678", "", ""),
		"同網域":       contactList("", "", "http://example.com/about", ""),
	} {
		_, err := f.module.CreateProspect(ctx, CreateProspectInput{AdminID: prospectAdmin, Fields: ProspectFields{DisplayName: "丙 " + name, VendorType: VendorBusiness}, Contacts: contacts})
		wantMatchFound(t, err, holder.ID)
		matches, err := f.module.PreviewProspect(ctx, ProspectPreviewInput{DisplayName: "完全不同的名稱", Contacts: contacts})
		must(t, err)
		if len(matches) != 1 || matches[0].Prospect.ID != holder.ID || matches[0].Tier != MatchContact {
			t.Fatalf("%s: preview=%+v", name, matches)
		}
	}
	if n := f.prospectCount(t); n != before || len(f.distinctPairs(t)) != 0 {
		t.Fatalf("a rejected create wrote something: prospects %d -> %d pairs=%v", before, n, f.distinctPairs(t))
	}
	// 確認一個不相干的 ID 不算確認相符的那一個，也不會被記下。
	stranger := f.newProspect(t, "路人", nil)
	_, err := f.module.CreateProspect(ctx, CreateProspectInput{AdminID: prospectAdmin, Fields: ProspectFields{DisplayName: "乙", VendorType: VendorBusiness}, Contacts: contactList("sales@example.com", "", "", ""), ConfirmedDistinct: []int64{stranger.ID}})
	wantMatchFound(t, err, holder.ID)
	// 確認之後建立成功，配對在同一個 transaction 記下（較小的 ID 在前），而且不記沒有相符的 ID。
	second, err := f.module.CreateProspect(ctx, CreateProspectInput{AdminID: prospectAdmin, Fields: ProspectFields{DisplayName: "乙", VendorType: VendorBusiness}, Contacts: contactList("sales@example.com", "", "", ""), ConfirmedDistinct: []int64{holder.ID, stranger.ID}})
	must(t, err)
	if got := f.distinctPairs(t); !slices.Equal(got, [][2]int64{{holder.ID, second.ID}}) {
		t.Fatalf("pairs=%v", got)
	}
	var admin int64
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT confirmed_by_admin_id FROM seller.prospect_distinct_pairs").Scan(&admin))
	if admin != prospectAdmin {
		t.Fatalf("confirmed by %d", admin)
	}
	if pairs := f.duplicatePairs(t); len(pairs) != 0 {
		t.Fatalf("a confirmed pair is still listed: %v", pairs)
	}
	// 第三家撞到前兩家：兩家都要確認；只確認一家仍然被擋。
	_, err = f.module.CreateProspect(ctx, CreateProspectInput{AdminID: prospectAdmin, Fields: ProspectFields{DisplayName: "丁", VendorType: VendorBusiness}, Contacts: contactList("SALES@example.com", "", "", ""), ConfirmedDistinct: []int64{holder.ID}})
	wantMatchFound(t, err, holder.ID, second.ID)
	f.newProspect(t, "丁", func(in *CreateProspectInput) {
		withContact("SALES@example.com", "", "", "")(in)
		in.ConfirmedDistinct = []int64{second.ID, holder.ID}
	})
	// 資料庫不強制比對鍵唯一（並行建立的兩筆都會成功）。
	loose := f.newProspect(t, "戊", nil)
	f.insertContact(t, loose.ID, "EMAIL", "sales@example.com", "EMAIL:sales@example.com", true)
}

// D4：疑似重複是尚未處理的候選配對：名稱相同、或比對鍵相同（並行建立漏網的），排除已標記不是同一家的；可以分頁，不是截斷在 500 對。
func TestDuplicatesAreUnresolvedPairsBySameNameOrSharedKey(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	a := f.newProspect(t, "好農場", nil)
	b := f.newProspect(t, " 好農場 ", nil)
	c := f.newProspect(t, "好農場", nil)
	f.newProspect(t, "別的名字", nil)
	matches, err := f.module.PreviewProspect(ctx, ProspectPreviewInput{DisplayName: "好農場"})
	must(t, err)
	if len(matches) != 3 || matches[0].Tier != MatchName {
		t.Fatalf("preview=%+v", matches)
	}
	e := f.newProspect(t, "東", withContact("shared@example.com", "", "", ""))
	g := f.newProspect(t, "西", nil)
	f.insertContact(t, g.ID, "EMAIL", "Shared@Example.com", "EMAIL:shared@example.com", true)
	want := [][2]int64{{a.ID, b.ID}, {a.ID, c.ID}, {b.ID, c.ID}, {e.ID, g.ID}}
	if got := f.duplicatePairs(t); !slices.Equal(got, want) {
		t.Fatalf("pairs=%v, want %v", got, want)
	}
	// 每一對都說明命中的原因：名稱相同是 name，比對鍵相同是鍵的種類；名稱與鍵都相同時兩個都列。
	h := f.newProspect(t, "雙重", withContact("both@example.com", "", "", ""))
	i := f.newProspect(t, "雙重", nil)
	f.insertContact(t, i.ID, "EMAIL", "both@example.com", "EMAIL:both@example.com", true)
	reasons := map[[2]int64][]MatchReason{}
	for after := (*DuplicateCursor)(nil); ; {
		page, err := f.module.DuplicateProspects(ctx, 2, after)
		must(t, err)
		for _, p := range page.Items {
			reasons[[2]int64{p.Left.ID, p.Right.ID}] = p.MatchedOn
		}
		if page.Next == nil {
			break
		}
		after = page.Next
	}
	for pair, on := range map[[2]int64][]MatchReason{{a.ID, b.ID}: {ReasonName}, {e.ID, g.ID}: {ReasonEmail}, {h.ID, i.ID}: {ReasonEmail, ReasonName}} {
		if !slices.Equal(reasons[pair], on) {
			t.Fatalf("pair %v matched_on=%v, want %v", pair, reasons[pair], on)
		}
	}
	// 這兩筆只為了上面的斷言，封存之後不影響後面的清單。
	f.archive(t, h.ID)
	f.archive(t, i.ID)
	// 標記不是同一家之後，那一對不再出現：名稱相同的一對（直接標記）與比對鍵相同的一對（經修改聯絡方式的確認流程）。
	_, err = f.db.Conn(ctx).Exec(ctx, "INSERT INTO seller.prospect_distinct_pairs VALUES ($1, $2, $3, now())", a.ID, b.ID, prospectAdmin)
	must(t, err)
	_, err = f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: e.ID, ExpectedVersion: e.Version, AdminID: prospectAdmin, Contacts: &[]ContactPoint{{Kind: ContactEmail, Value: "shared@example.com", Primary: true}, {Kind: ContactPhone, Value: "02-1111-2222", Primary: true}}})
	wantMatchFound(t, err, g.ID)
	_, err = f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: e.ID, ExpectedVersion: e.Version, AdminID: prospectAdmin, Contacts: &[]ContactPoint{{Kind: ContactEmail, Value: "shared@example.com", Primary: true}, {Kind: ContactPhone, Value: "02-1111-2222", Primary: true}}, ConfirmedDistinct: []int64{g.ID}})
	must(t, err)
	if got, want := f.duplicatePairs(t), [][2]int64{{a.ID, c.ID}, {b.ID, c.ID}}; !slices.Equal(got, want) {
		t.Fatalf("pairs=%v, want %v", got, want)
	}
	// 封存的不在清單裡。
	f.archive(t, c.ID)
	if got := f.duplicatePairs(t); len(got) != 0 {
		t.Fatalf("pairs=%v", got)
	}
}

func TestUpdateProspectReplacesTheContactListAndKeepsOmittedFields(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := f.newProspect(t, "甲", func(in *CreateProspectInput) {
		withContact("old@example.com", "02-1111-2222", "", "")(in)
		in.Fields.Category, in.Fields.ProductSummary = text("水果"), text("芒果")
	})
	// 沒有帶 Contacts：聯絡方式原樣保留。
	same, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: p.Version, AdminID: prospectAdmin + 1, Region: ptr("台南")})
	must(t, err)
	if got := contactsOf(same); !slices.Equal(got, []string{"EMAIL:old@example.com*", "PHONE:02-1111-2222*"}) || *same.Category != "水果" || *same.Region != "台南" || *same.UpdatedByAdminID != prospectAdmin+1 || *same.CreatedByAdminID != prospectAdmin {
		t.Fatalf("same=%+v", same)
	}
	// 整批替換：新的主要 email、舊的留成非主要；電話原樣。
	updated, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: same.Version, AdminID: prospectAdmin, Contacts: &[]ContactPoint{
		{Kind: ContactEmail, Value: "New@Example.com", Primary: true}, {Kind: ContactEmail, Value: "old@example.com", Primary: false}, {Kind: ContactPhone, Value: "02-1111-2222", Primary: false},
	}})
	must(t, err)
	if updated.Version != same.Version+1 || !slices.Equal(contactsOf(updated), []string{"EMAIL:New@Example.com*", "EMAIL:old@example.com", "PHONE:02-1111-2222*"}) {
		t.Fatalf("updated=%+v contacts=%v", updated, contactsOf(updated))
	}
	if got, want := f.matchKeys(t, p.ID), []string{"EMAIL:new@example.com", "EMAIL:old@example.com", "PHONE:0211112222"}; !slices.Equal(got, want) {
		t.Fatalf("keys=%v, want %v", got, want)
	}
	// 同一批再送一次：沒有改變，不寫入（版本不變）。
	again, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: updated.Version, AdminID: prospectAdmin, Contacts: &[]ContactPoint{
		{Kind: ContactPhone, Value: "02-1111-2222", Primary: true}, {Kind: ContactEmail, Value: "old@example.com", Primary: false}, {Kind: ContactEmail, Value: " New@Example.com ", Primary: true},
	}})
	must(t, err)
	if again.Version != updated.Version || !again.UpdatedAt.Equal(updated.UpdatedAt) {
		t.Fatalf("an unchanged contact list wrote the row: %+v", again)
	}
	// 同類型多個而沒有（或有兩個）主要的：拒絕，什麼都不寫。
	for name, bad := range map[string][]ContactPoint{
		"沒有主要的": {{Kind: ContactEmail, Value: "a@example.com", Primary: false}, {Kind: ContactEmail, Value: "b@example.com", Primary: false}},
		"兩個主要的": {{Kind: ContactEmail, Value: "a@example.com", Primary: true}, {Kind: ContactEmail, Value: "b@example.com", Primary: true}},
		"空白的值":  {{Kind: ContactPhone, Value: "  ", Primary: true}},
	} {
		_, err = f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: updated.Version, AdminID: prospectAdmin, Contacts: &bad})
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%s: err=%v", name, err)
		}
	}
	// 空清單清掉全部聯絡方式與它們的比對鍵；舊的 email 讓出來，別人可以用而不需要確認。
	cleared, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: updated.Version, AdminID: prospectAdmin, Contacts: &[]ContactPoint{}})
	must(t, err)
	if len(cleared.Contacts) != 0 || len(f.matchKeys(t, p.ID)) != 0 || cleared.Version != updated.Version+1 {
		t.Fatalf("cleared=%+v keys=%v", cleared, f.matchKeys(t, p.ID))
	}
	f.newProspect(t, "乙", withContact("old@example.com", "", "", ""))
}

// 改到聯絡方式而撞到別人的比對鍵：回 PROSPECT_MATCH_FOUND，什麼都不改（連同一次請求裡的其他欄位）；確認之後寫入並記下配對，
// 之後同一對不再擋；沒有改到聯絡方式的修改不受未處理的重複影響。
func TestUpdateProspectContactMatchNeedsConfirmation(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	taken := f.newProspect(t, "甲", withContact("taken@example.com", "", "", ""))
	p := f.newProspect(t, "乙", withContact("mine@example.com", "", "", ""))
	in := UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: p.Version, AdminID: prospectAdmin, DisplayName: ptr("改名"), Contacts: &[]ContactPoint{{Kind: ContactEmail, Value: "Taken@example.com", Primary: true}}}
	_, err := f.module.UpdateProspect(ctx, in)
	wantMatchFound(t, err, taken.ID)
	after := f.prospectRow(t, p.ID)
	if after.Version != p.Version || after.DisplayName != "乙" || !slices.Equal(contactsOf(after), []string{"EMAIL:mine@example.com*"}) || len(f.distinctPairs(t)) != 0 {
		t.Fatalf("after=%+v pairs=%v", after, f.distinctPairs(t))
	}
	in.ConfirmedDistinct = []int64{taken.ID}
	moved, err := f.module.UpdateProspect(ctx, in)
	must(t, err)
	if moved.Version != p.Version+1 || moved.DisplayName != "改名" || !slices.Equal(contactsOf(moved), []string{"EMAIL:Taken@example.com*"}) {
		t.Fatalf("moved=%+v", moved)
	}
	if got := f.distinctPairs(t); !slices.Equal(got, [][2]int64{{taken.ID, p.ID}}) {
		t.Fatalf("pairs=%v", got)
	}
	// 同一對已經確認過：再改聯絡方式（仍含同一個鍵）不再被擋，也不必再帶確認。
	next, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: moved.Version, AdminID: prospectAdmin, Contacts: &[]ContactPoint{{Kind: ContactEmail, Value: "Taken@example.com", Primary: true}, {Kind: ContactPhone, Value: "02-1111-2222", Primary: true}}})
	must(t, err)
	// 沒有改到聯絡方式的修改，不查比對。
	third := f.newProspect(t, "丙", nil)
	f.insertContact(t, third.ID, "EMAIL", "taken@example.com", "EMAIL:taken@example.com", true)
	_, err = f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: next.Version, AdminID: prospectAdmin, ProductSummary: ptr("摘要")})
	must(t, err)
}

// 樂觀鎖（P4）：舊實作的文字欄位是 last-write-wins；現在整列有版本，過期的版本衝突，沒有改變的更新不動版本。
func TestUpdateProspectOptimisticLock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := f.newProspect(t, "甲", nil)
	_, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, AdminID: prospectAdmin, DisplayName: ptr("新")})
	wantKind(t, err, apperr.KindInvalidInput) // 沒帶版本
	same, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: p.Version, AdminID: prospectAdmin, DisplayName: ptr("甲")})
	must(t, err)
	if same.Version != p.Version || !same.UpdatedAt.Equal(p.UpdatedAt) {
		t.Fatalf("no-op update changed the row: %+v", same)
	}
	_, err = f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: p.Version, AdminID: prospectAdmin, DisplayName: ptr("乙")})
	must(t, err)
	_, err = f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: p.Version, AdminID: prospectAdmin, DisplayName: ptr("丙")})
	wantKind(t, err, apperr.KindVersionConflict)
	// 過期的版本即使沒有改變也衝突。
	_, err = f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: p.Version, AdminID: prospectAdmin, DisplayName: ptr("乙")})
	wantKind(t, err, apperr.KindVersionConflict)
	// 招商狀態不能手動設成已入駐。
	_, err = f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: p.Version + 1, AdminID: prospectAdmin, OutreachStatus: ptr(OutreachConverted)})
	wantKind(t, err, apperr.KindInvalidInput)
}

func TestArchiveNeedsTheCurrentVersionAndYieldsTaxIDAndMatching(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	a := f.newProspect(t, "甲", func(in *CreateProspectInput) {
		withContact("shared@example.com", "", "", "")(in)
		in.Fields.TaxID = text("11111111")
	})
	// 過期或沒帶的版本：衝突或 InvalidInput，什麼都不寫。
	_, err := f.module.ArchiveProspect(ctx, ArchiveProspectInput{ProspectID: a.ID, AdminID: prospectAdmin})
	wantKind(t, err, apperr.KindInvalidInput)
	edited, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: a.ID, ExpectedVersion: a.Version, AdminID: prospectAdmin, Region: ptr("台北")})
	must(t, err)
	_, err = f.module.ArchiveProspect(ctx, ArchiveProspectInput{ProspectID: a.ID, AdminID: prospectAdmin, ExpectedVersion: a.Version})
	wantKind(t, err, apperr.KindVersionConflict)
	if got := f.prospectRow(t, a.ID); got.Status != ProspectStatusActive || got.Version != edited.Version {
		t.Fatalf("stale archive wrote: %+v", got)
	}
	archived, err := f.module.ArchiveProspect(ctx, ArchiveProspectInput{ProspectID: a.ID, AdminID: prospectAdmin, ExpectedVersion: edited.Version})
	must(t, err)
	if archived.Status != ProspectStatusArchived || archived.Version != edited.Version+1 || !slices.Equal(contactsOf(archived), []string{"EMAIL:shared@example.com*"}) {
		t.Fatalf("archived=%+v", archived)
	}
	// 重新讀到封存之後的版本再送一次是 no-op。
	again, err := f.module.ArchiveProspect(ctx, ArchiveProspectInput{ProspectID: a.ID, AdminID: prospectAdmin, ExpectedVersion: archived.Version})
	must(t, err)
	if again.Version != archived.Version {
		t.Fatalf("archive is not idempotent: %+v", again)
	}
	// 封存的業者讓出統編，也不再被比對：別人用同一個 email 與統編建立不需要確認。
	b := f.newProspect(t, "乙", func(in *CreateProspectInput) {
		withContact("shared@example.com", "", "", "")(in)
		in.Fields.TaxID = text("11111111")
	})
	// 統編被拿走：還原回 FailedPrecondition，確認不是同一家也一樣；沒有寫任何東西。
	_, err = f.restore(t, a.ID)
	wantKind(t, err, apperr.KindFailedPrecondition)
	if _, err = f.restore(t, a.ID, b.ID); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("confirming cannot override a tax id clash: %v", err)
	}
	var held *apperr.Error
	if errors.As(err, &held) && held.Reason == apperr.ReasonProspectMatchFound {
		t.Fatalf("a tax id clash is not a match: %v", err)
	}
	if got := f.prospectRow(t, a.ID); got.Status != ProspectStatusArchived || len(f.distinctPairs(t)) != 0 {
		t.Fatalf("failed restore changed the row: %+v pairs=%v", got, f.distinctPairs(t))
	}
	// 統編讓出之後，剩下聯絡方式的比對鍵被 b 佔著：回 PROSPECT_MATCH_FOUND，確認不是同一家才還原並記下配對。
	_, err = f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: b.ID, ExpectedVersion: b.Version, AdminID: prospectAdmin, TaxID: ptr("")})
	must(t, err)
	_, err = f.restore(t, a.ID)
	wantMatchFound(t, err, b.ID)
	if got := f.prospectRow(t, a.ID); got.Status != ProspectStatusArchived {
		t.Fatalf("failed restore changed the row: %+v", got)
	}
	restored, err := f.restore(t, a.ID, b.ID)
	must(t, err)
	if restored.Status != ProspectStatusActive || restored.Version != archived.Version+1 || !slices.Equal(f.distinctPairs(t)[0][:], []int64{a.ID, b.ID}) {
		t.Fatalf("restored=%+v pairs=%v", restored, f.distinctPairs(t))
	}
	// 已經是進行中的重送回傳目前的業者。
	resent, err := f.restore(t, a.ID)
	must(t, err)
	if resent.Version != restored.Version {
		t.Fatalf("restore is not idempotent: %+v", resent)
	}
}

func TestWritesRequireAnActiveProspect(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := f.newProspect(t, "甲", nil)
	archived := f.archive(t, p.ID)
	_, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: archived.Version, AdminID: prospectAdmin, DisplayName: ptr("新")})
	wantKind(t, err, apperr.KindFailedPrecondition)
	_, err = f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: archived.Version, AdminID: prospectAdmin, Logo: &ProspectImageUpload{StagedToken: "t"}})
	wantKind(t, err, apperr.KindFailedPrecondition)
	if len(f.media.finalized) != 0 {
		t.Fatalf("finalized for an archived prospect: %v", f.media.finalized)
	}
	_, err = f.module.ConvertProspect(ctx, ConvertProspectInput{ProspectID: p.ID, StoreID: 1, AdminID: prospectAdmin, ExpectedVersion: archived.Version})
	wantKind(t, err, apperr.KindFailedPrecondition)
	_, err = f.module.Prospect(ctx, p.ID+999)
	wantKind(t, err, apperr.KindNotFound)
}

// 合併（D5）：輸家的聯絡方式併入贏家（同一個比對鍵只留一列、贏家的主要聯絡方式不變）、「不是同一家」的配對改掛贏家，輸家
// 的聯絡方式與配對移走；合併帶來的聯絡方式看得見，也能經 UpdateProspect 移除。
func TestMergeUnionsContactsAndRepointsDistinctPairs(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	winner := f.newProspect(t, "贏家", func(in *CreateProspectInput) {
		withContact("w@example.com", "02-1111-1111", "", "")(in)
		in.CategoryIDs = []string{"fruit"}
	})
	loser := f.newProspect(t, "輸家", func(in *CreateProspectInput) {
		in.Contacts = []ContactPoint{
			{Kind: ContactEmail, Value: "W@example.com", Primary: false}, {Kind: ContactEmail, Value: "l@example.com", Primary: true}, {Kind: ContactEmail, Value: "l2@example.com", Primary: false},
			{Kind: ContactPhone, Value: "+886-2-3333-4444", Primary: true}, {Kind: ContactWebsite, Value: "https://www.loser.tw", Primary: true},
		}
		in.CategoryIDs = []string{"fruit", "veg"}
		in.ConfirmedDistinct = []int64{winner.ID} // 與贏家共用 w@example.com：確認不是同一家，記下 (贏家, 輸家) 這一對
	})
	other := f.newProspect(t, "旁人", nil)
	third := f.newProspect(t, "第三人", nil)
	for _, pair := range [][2]int64{{winner.ID, other.ID}, {loser.ID, other.ID}, {loser.ID, third.ID}} {
		_, err := f.db.Conn(ctx).Exec(ctx, "INSERT INTO seller.prospect_distinct_pairs VALUES (LEAST($1::bigint, $2), GREATEST($1::bigint, $2), $3, now())", pair[0], pair[1], prospectAdmin)
		must(t, err)
	}
	merged := f.merge(t, winner.ID, loser.ID)
	want := []string{
		"EMAIL:l2@example.com", "EMAIL:l@example.com", "EMAIL:w@example.com*", "PHONE:+886-2-3333-4444", "PHONE:02-1111-1111*", "WEBSITE:https://www.loser.tw*",
	}
	if merged.ID != winner.ID || merged.Version != winner.Version+1 || !slices.Equal(contactsOf(merged), want) || !slices.Equal(merged.CategoryIDs, []string{"fruit", "veg"}) {
		t.Fatalf("merged=%+v contacts=%v", merged, contactsOf(merged))
	}
	gone := f.prospectRow(t, loser.ID)
	if gone.Status != ProspectStatusArchived || gone.MergedInto == nil || *gone.MergedInto != winner.ID || len(gone.Contacts) != 0 || len(gone.CategoryIDs) != 0 {
		t.Fatalf("loser=%+v", gone)
	}
	// 配對：贏家的（含已有的重複，只留一列）加上輸家改掛過來的；輸家的與贏家對輸家的標記都消失（明確合併就是撤銷那個標記）。
	if got, want := f.distinctPairs(t), [][2]int64{{winner.ID, other.ID}, {winner.ID, third.ID}}; !slices.Equal(got, want) {
		t.Fatalf("pairs=%v, want %v", got, want)
	}
	// 輸家的 email 之後再建立：撞到贏家，預覽也指向贏家。
	_, err := f.module.CreateProspect(ctx, CreateProspectInput{AdminID: prospectAdmin, Fields: ProspectFields{DisplayName: "再匯入", VendorType: VendorBusiness}, Contacts: contactList("L@example.com", "", "", "")})
	wantMatchFound(t, err, winner.ID)
	matches, err := f.module.PreviewProspect(ctx, ProspectPreviewInput{Contacts: contactList("l@example.com", "+886-2-3333-4444", "", "")})
	must(t, err)
	if len(matches) != 1 || matches[0].Prospect.ID != winner.ID || !slices.Equal(matches[0].MatchedOn, []MatchReason{ReasonEmail, ReasonPhone}) {
		t.Fatalf("preview=%+v", matches)
	}
	// 被合併的列不可還原，也不能再被合併或轉換。
	_, err = f.restore(t, loser.ID)
	wantKind(t, err, apperr.KindFailedPrecondition)
	_, err = f.module.MergeProspects(ctx, f.mergeInput(t, winner.ID, loser.ID))
	wantKind(t, err, apperr.KindFailedPrecondition)
	_, err = f.module.MergeProspects(ctx, MergeProspectsInput{WinnerID: winner.ID, LoserID: winner.ID, AdminID: prospectAdmin, WinnerVersion: 1, LoserVersion: 1})
	wantKind(t, err, apperr.KindInvalidInput)
	// 合併帶來的聯絡方式可以移除；沒移除的不會弄丟。
	updated, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: winner.ID, ExpectedVersion: merged.Version, AdminID: prospectAdmin, Contacts: &[]ContactPoint{
		{Kind: ContactEmail, Value: "w@example.com", Primary: true}, {Kind: ContactEmail, Value: "l@example.com", Primary: false}, {Kind: ContactPhone, Value: "02-1111-1111", Primary: true}, {Kind: ContactWebsite, Value: "https://www.loser.tw", Primary: true},
	}})
	must(t, err)
	if got, want := contactsOf(updated), []string{"EMAIL:l@example.com", "EMAIL:w@example.com*", "PHONE:02-1111-1111*", "WEBSITE:https://www.loser.tw*"}; !slices.Equal(got, want) {
		t.Fatalf("after removing the inherited ones: %v", got)
	}
	if got, want := f.matchKeys(t, winner.ID), []string{"DOMAIN:loser.tw", "EMAIL:l@example.com", "EMAIL:w@example.com", "PHONE:0211111111"}; !slices.Equal(got, want) {
		t.Fatalf("keys=%v, want %v", got, want)
	}
}

// 贏家在某個類型沒有聯絡方式時，輸家那個類型的主要的成為主要的；贏家沒有主要的類型不會留下沒有主要的聯絡方式。
func TestMergeKeepsExactlyOnePrimaryPerKind(t *testing.T) {
	t.Parallel()
	f := setup(t)
	winner := f.newProspect(t, "贏家", withContact("w@example.com", "", "", ""))
	loser := f.newProspect(t, "輸家", func(in *CreateProspectInput) {
		in.Contacts = []ContactPoint{{Kind: ContactPhone, Value: "02-1", Primary: false}, {Kind: ContactPhone, Value: "02-2", Primary: true}, {Kind: ContactPhone, Value: "02-3", Primary: false}, {Kind: ContactSocial, Value: "https://www.facebook.com/lp", Primary: true}}
	})
	merged := f.merge(t, winner.ID, loser.ID)
	want := []string{"EMAIL:w@example.com*", "PHONE:02-1", "PHONE:02-2*", "PHONE:02-3", "SOCIAL:https://www.facebook.com/lp*"}
	if got := contactsOf(merged); !slices.Equal(got, want) {
		t.Fatalf("contacts=%v, want %v", got, want)
	}
	if n := f.count(t, `SELECT count(*) FROM (SELECT prospect_id, kind FROM seller.prospect_contacts GROUP BY 1, 2 HAVING count(*) FILTER (WHERE is_primary) <> 1) x`); n != 0 {
		t.Fatalf("%d (prospect, kind) groups without exactly one primary", n)
	}
}

// 兩邊都要帶讀到的版本：任何一邊過期都是 VersionConflict，什麼都不寫（也不發事件）。
func TestStaleVersionsRejectMergeConvertAndArchiveWithoutWriting(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	merged := record[ProspectMerged](f)
	archived := record[ProspectArchived](f)
	winner := f.newProspect(t, "贏家", withContact("w@example.com", "", "", ""))
	loser := f.newProspect(t, "輸家", withContact("l@example.com", "", "", ""))
	_, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: loser.ID, ExpectedVersion: loser.Version, AdminID: prospectAdmin, Region: ptr("花蓮")})
	must(t, err)
	in := MergeProspectsInput{WinnerID: winner.ID, LoserID: loser.ID, AdminID: prospectAdmin, WinnerVersion: winner.Version, LoserVersion: loser.Version}
	_, err = f.module.MergeProspects(ctx, in) // 輸家過期
	wantKind(t, err, apperr.KindVersionConflict)
	fresh := in
	fresh.LoserVersion = f.prospectRow(t, loser.ID).Version
	fresh.WinnerVersion = winner.Version + 1 // 贏家過期
	_, err = f.module.MergeProspects(ctx, fresh)
	wantKind(t, err, apperr.KindVersionConflict)
	for _, bad := range []MergeProspectsInput{{WinnerID: winner.ID, LoserID: loser.ID, AdminID: prospectAdmin, WinnerVersion: winner.Version}, {WinnerID: winner.ID, LoserID: loser.ID, AdminID: prospectAdmin, LoserVersion: loser.Version}} {
		_, err = f.module.MergeProspects(ctx, bad)
		wantKind(t, err, apperr.KindInvalidInput)
	}
	_, s := f.open(t)
	_, err = f.module.ConvertProspect(ctx, ConvertProspectInput{ProspectID: loser.ID, StoreID: s.StoreID, AdminID: prospectAdmin, ExpectedVersion: loser.Version})
	wantKind(t, err, apperr.KindVersionConflict)
	_, err = f.module.ConvertProspect(ctx, ConvertProspectInput{ProspectID: loser.ID, StoreID: s.StoreID, AdminID: prospectAdmin})
	wantKind(t, err, apperr.KindInvalidInput)
	_, err = f.module.ArchiveProspect(ctx, ArchiveProspectInput{ProspectID: loser.ID, AdminID: prospectAdmin, ExpectedVersion: loser.Version})
	wantKind(t, err, apperr.KindVersionConflict)
	w, l := f.prospectRow(t, winner.ID), f.prospectRow(t, loser.ID)
	if w.Version != winner.Version || w.Status != ProspectStatusActive || l.Version != loser.Version+1 || l.Status != ProspectStatusActive || l.MergedInto != nil || l.ConvertedStore != nil ||
		!slices.Equal(contactsOf(w), []string{"EMAIL:w@example.com*"}) || !slices.Equal(contactsOf(l), []string{"EMAIL:l@example.com*"}) {
		t.Fatalf("a stale request wrote something: winner=%+v loser=%+v", w, l)
	}
	if len(merged.all()) != 0 || len(archived.all()) != 0 {
		t.Fatalf("events: merged=%+v archived=%+v", merged.all(), archived.all())
	}
	// 帶目前的版本就成功。
	f.merge(t, winner.ID, loser.ID)
	if len(merged.all()) != 1 {
		t.Fatalf("merged=%+v", merged.all())
	}
}

// 兩個管理員拿著同一個版本同時改：一個成功、一個版本衝突，不會互相覆蓋。
func TestConcurrentProspectUpdatesConflict(t *testing.T) {
	t.Parallel()
	f := setup(t)
	p := f.newProspect(t, "甲", nil)
	var wg sync.WaitGroup
	results := make([]error, 2)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, results[i] = f.module.UpdateProspect(context.Background(), UpdateProspectInput{
				ProspectID: p.ID, ExpectedVersion: p.Version, AdminID: prospectAdmin, ProductSummary: ptr("摘要 " + strconv.Itoa(i)),
			})
		}()
	}
	wg.Wait()
	ok, conflicts := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			ok++
		case apperr.KindOf(err) == apperr.KindVersionConflict:
			conflicts++
		}
	}
	if ok != 1 || conflicts != 1 {
		t.Fatalf("results=%v", results)
	}
	if got := f.prospectRow(t, p.ID); got.Version != p.Version+1 {
		t.Fatalf("version=%d", got.Version)
	}
}

// 統編只在贏家沒有時搬過去；輸家的統編一律釋放。已經被輸家吸收的列在路徑壓縮後直接指向最後的贏家。
func TestMergeTaxIDAndPathCompression(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	empty := f.newProspect(t, "沒統編的贏家", nil)
	withTax := f.newProspect(t, "有統編", func(in *CreateProspectInput) { in.Fields.TaxID = text("22222222") })
	merged, err := f.module.MergeProspects(ctx, f.mergeInput(t, empty.ID, withTax.ID))
	must(t, err)
	if merged.TaxID == nil || *merged.TaxID != "22222222" || merged.Version != empty.Version+1 {
		t.Fatalf("tax id was not carried: %+v", merged)
	}
	if got := f.prospectRow(t, withTax.ID); got.TaxID != nil {
		t.Fatalf("loser kept its tax id: %+v", got)
	}
	// 贏家已經有統編：輸家的不搬，也釋放（之後別人可以用）。
	has := f.newProspect(t, "贏家有統編", func(in *CreateProspectInput) { in.Fields.TaxID = text("33333333") })
	loserTax := f.newProspect(t, "輸家有統編", func(in *CreateProspectInput) { in.Fields.TaxID = text("44444444") })
	merged, err = f.module.MergeProspects(ctx, f.mergeInput(t, has.ID, loserTax.ID))
	must(t, err)
	if *merged.TaxID != "33333333" {
		t.Fatalf("winner's tax id was overwritten: %+v", merged)
	}
	f.newProspect(t, "拿走 4444", func(in *CreateProspectInput) { in.Fields.TaxID = text("44444444") })
	// 路徑壓縮：absorbed 併進 mid，mid 再併進 top，absorbed 直接指向 top。
	top := f.newProspect(t, "top", nil)
	mid := f.newProspect(t, "mid", nil)
	absorbed := f.newProspect(t, "absorbed", nil)
	_, err = f.module.MergeProspects(ctx, f.mergeInput(t, mid.ID, absorbed.ID))
	must(t, err)
	_, err = f.module.MergeProspects(ctx, f.mergeInput(t, top.ID, mid.ID))
	must(t, err)
	for _, id := range []int64{mid.ID, absorbed.ID} {
		if got := f.prospectRow(t, id); got.MergedInto == nil || *got.MergedInto != top.ID {
			t.Fatalf("prospect %d merged_into=%v, want %d", id, got.MergedInto, top.ID)
		}
	}
}

func (f *fixture) prospectRef(refType string, prospectID int64) media.Ref {
	return f.storeRef(refType, prospectID)
}

func (f *fixture) syncProspectImages(t *testing.T, id int64) {
	t.Helper()
	f.syncMedia(t, f.prospectRef(media.RefTypeSellerProspectLogo, id))
	f.syncMedia(t, f.prospectRef(media.RefTypeSellerProspectCover, id))
}

// 圖片併進 UpdateProspect：版本條件寫入、以 prospect_id 為 ref；換圖只有被換掉的那一張退場；文字、聯絡方式與兩張圖在同一個版本
// 條件的 transaction 內寫入，版本只加一。
func TestProspectImagesLinkByProspectIDAndUseVersion(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := f.newProspect(t, "甲", nil)
	updated, err := f.module.UpdateProspect(ctx, UpdateProspectInput{
		ProspectID: p.ID, ExpectedVersion: p.Version, AdminID: prospectAdmin, ProductSummary: ptr("芒果"),
		Contacts: &[]ContactPoint{{Kind: ContactEmail, Value: "a@example.com", Primary: true}},
		Logo:     &ProspectImageUpload{StagedToken: "one", SourceURL: "https://source.example/a"},
		Cover:    &ProspectImageUpload{StagedToken: "cover"},
	})
	must(t, err)
	if updated.Version != p.Version+1 || updated.LogoKey == nil || *updated.LogoKey != media.RefTypeSellerProspectLogo+"/one" || *updated.LogoSourceURL != "https://source.example/a" ||
		updated.CoverKey == nil || *updated.CoverKey != media.RefTypeSellerProspectCover+"/cover" || updated.CoverSourceURL != nil || *updated.ProductSummary != "芒果" || len(updated.Contacts) != 1 {
		t.Fatalf("updated=%+v", updated)
	}
	if !slices.Equal(f.media.finalized, []string{media.RefTypeSellerProspectLogo, media.RefTypeSellerProspectCover}) {
		t.Fatalf("finalized=%+v", f.media.finalized)
	}
	logoRef := f.prospectRef(media.RefTypeSellerProspectLogo, p.ID)
	if got := f.media.Referenced(logoRef); len(got) != 0 {
		t.Fatalf("寫入不得建立引用：%v", got)
	}
	f.syncMedia(t, logoRef)
	if got := f.media.Referenced(logoRef); fmt.Sprint(got) != "[asset-one]" {
		t.Fatalf("references=%v", got)
	}
	// 只換 logo：封面原樣保留；同步 job 登記新的、精確解除被換掉的。
	again, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: updated.Version, AdminID: prospectAdmin, Logo: &ProspectImageUpload{StagedToken: "two"}})
	must(t, err)
	f.syncMedia(t, logoRef)
	if got := f.media.Referenced(logoRef); fmt.Sprint(got) != "[asset-two]" || again.LogoSourceURL != nil || again.CoverKey == nil || *again.CoverKey != *updated.CoverKey {
		t.Fatalf("references=%v again=%+v", got, again)
	}
	// 被換掉的圖不能寫回：同一個 staged token 重新轉正會回到同一個資產。
	_, err = f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: again.Version, AdminID: prospectAdmin, Logo: &ProspectImageUpload{StagedToken: "one"}})
	wantKind(t, err, apperr.KindInvalidInput)
}

// 來源網址選填、只收 https；空白的 token 不是圖片。這些都在轉正之前拒絕。
func TestProspectImageUploadValidation(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := f.newProspect(t, "甲", nil)
	for name, up := range map[string]ProspectImageUpload{
		"http":       {StagedToken: "t", SourceURL: "http://source.example/a"},
		"javascript": {StagedToken: "t", SourceURL: "javascript:alert(1)"},
		"沒有主機":       {StagedToken: "t", SourceURL: "https://"},
		"空白的 token":  {StagedToken: "  "},
	} {
		_, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: p.Version, AdminID: prospectAdmin, Cover: &up})
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%s: err=%v", name, err)
		}
	}
	if len(f.media.finalized) != 0 {
		t.Fatalf("finalized=%v", f.media.finalized)
	}
	got, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: p.Version, AdminID: prospectAdmin, Cover: &ProspectImageUpload{StagedToken: "ok", SourceURL: " https://source.example/b "}})
	must(t, err)
	if got.CoverSourceURL == nil || *got.CoverSourceURL != "https://source.example/b" {
		t.Fatalf("got=%+v", got)
	}
}

// 被拒絕的請求不消耗 staged token（比照店家的 TestRejectedProfileUpdateDoesNotConsumeTheStagedToken）：過期的版本、不合法的欄位、
// 撞到比對鍵、撞到統編都在轉正之前拒絕；修正之後用同一個 token 重送成功。
func TestRejectedProspectUpdateDoesNotConsumeTheStagedToken(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	taken := f.newProspect(t, "甲", func(in *CreateProspectInput) {
		withContact("taken@example.com", "", "", "")(in)
		in.Fields.TaxID = text("12345678")
	})
	p := f.newProspect(t, "乙", nil)
	logo := &ProspectImageUpload{StagedToken: "logo1"}
	for name, c := range map[string]struct {
		in   UpdateProspectInput
		want apperr.Kind
	}{
		"過期的版本":  {UpdateProspectInput{ExpectedVersion: p.Version + 5}, apperr.KindVersionConflict},
		"不合法的欄位": {UpdateProspectInput{ExpectedVersion: p.Version, DisplayName: ptr("  ")}, apperr.KindInvalidInput},
		"撞到比對鍵":  {UpdateProspectInput{ExpectedVersion: p.Version, Contacts: &[]ContactPoint{{Kind: ContactEmail, Value: "Taken@example.com", Primary: true}}}, apperr.KindFailedPrecondition},
		"撞到統編":   {UpdateProspectInput{ExpectedVersion: p.Version, TaxID: ptr("12345678")}, apperr.KindFailedPrecondition},
	} {
		c.in.ProspectID, c.in.AdminID, c.in.Logo = p.ID, prospectAdmin, logo
		_, err := f.module.UpdateProspect(ctx, c.in)
		wantKind(t, err, c.want)
		if len(f.media.finalized) != 0 {
			t.Fatalf("%s: token finalized by a rejected request: %v", name, f.media.finalized)
		}
	}
	if got := f.prospectRow(t, p.ID); got.Version != p.Version || got.LogoKey != nil {
		t.Fatalf("a rejected request wrote: %+v", got)
	}
	updated, err := f.module.UpdateProspect(ctx, UpdateProspectInput{
		ProspectID: p.ID, ExpectedVersion: p.Version, AdminID: prospectAdmin, Logo: logo,
		Contacts: &[]ContactPoint{{Kind: ContactEmail, Value: "Taken@example.com", Primary: true}}, ConfirmedDistinct: []int64{taken.ID},
	})
	must(t, err)
	if len(f.media.finalized) != 1 || updated.LogoKey == nil || updated.Version != p.Version+1 || len(updated.Contacts) != 1 {
		t.Fatalf("retry: finalized=%v prospect=%+v", f.media.finalized, updated)
	}
}

// 換圖的並行：轉正之後、寫入之前業者被別人改了，版本條件讓這次寫入衝突，什麼都不寫。
func TestProspectImageWriteLosesToAConcurrentEdit(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := f.newProspect(t, "甲", nil)
	f.media.onFinalize = func() {
		_, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: p.Version, AdminID: prospectAdmin + 1, Region: ptr("宜蘭")})
		must(t, err)
	}
	_, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: p.Version, AdminID: prospectAdmin, ProductSummary: ptr("摘要"), Logo: &ProspectImageUpload{StagedToken: "late"}})
	wantKind(t, err, apperr.KindVersionConflict)
	got := f.prospectRow(t, p.ID)
	if got.Version != p.Version+1 || got.LogoKey != nil || got.ProductSummary != nil || *got.Region != "宜蘭" {
		t.Fatalf("got=%+v", got)
	}
}

// D16：封存與合併把圖片欄位清掉、同一個 transaction 把清掉的圖記成退場並排入同步；同步 job 精確解除它們。封存之後的還原
// 不會把圖帶回來，所以被清掉的圖不會再被寫回。
func TestArchiveAndMergeRetireImagesAndSyncUnlinksThem(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	withImages := func(name string) Prospect {
		p := f.newProspect(t, name, nil)
		p, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: p.Version, AdminID: prospectAdmin, Logo: &ProspectImageUpload{StagedToken: name + "-logo"}, Cover: &ProspectImageUpload{StagedToken: name + "-cover"}})
		must(t, err)
		return p
	}
	archivedSource := withImages("封存")
	f.syncProspectImages(t, archivedSource.ID)
	jobsBefore := f.jobs.count("seller.media_sync")
	archived, err := f.module.ArchiveProspect(ctx, ArchiveProspectInput{ProspectID: archivedSource.ID, AdminID: prospectAdmin, ExpectedVersion: archivedSource.Version})
	must(t, err)
	if archived.LogoKey != nil || archived.CoverKey != nil {
		t.Fatalf("archived prospect still points at images: %+v", archived)
	}
	if got := f.jobs.count("seller.media_sync"); got != jobsBefore+2 {
		t.Fatalf("archive enqueued %d sync jobs, want one per image kind", got-jobsBefore)
	}
	f.syncProspectImages(t, archived.ID)
	for _, refType := range []string{media.RefTypeSellerProspectLogo, media.RefTypeSellerProspectCover} {
		if got := f.media.Referenced(f.prospectRef(refType, archived.ID)); len(got) != 0 {
			t.Fatalf("%s: references=%v, want everything unlinked", refType, got)
		}
	}
	// 封存、還原、換圖：目前的圖照樣被登記，舊圖不會回來。
	newToken, oldToken := fmt.Sprintf("%s-new", "restored"), fmt.Sprintf("%s-logo", "封存")
	restored, err := f.restore(t, archived.ID)
	must(t, err)
	changed, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: archived.ID, ExpectedVersion: restored.Version, AdminID: prospectAdmin, Logo: &ProspectImageUpload{StagedToken: newToken}})
	must(t, err)
	f.syncProspectImages(t, archived.ID)
	if got := f.media.Referenced(f.prospectRef(media.RefTypeSellerProspectLogo, archived.ID)); fmt.Sprint(got) != "[asset-restored-new]" {
		t.Fatalf("restore then change image: references=%v", got)
	}
	_, err = f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: archived.ID, ExpectedVersion: changed.Version, AdminID: prospectAdmin, Logo: &ProspectImageUpload{StagedToken: oldToken}})
	wantKind(t, err, apperr.KindInvalidInput)
	// 合併的輸家一樣。
	winner := f.newProspect(t, "贏家", nil)
	loser := withImages("輸家")
	f.syncProspectImages(t, loser.ID)
	jobsBefore = f.jobs.count("seller.media_sync")
	_, err = f.module.MergeProspects(ctx, f.mergeInput(t, winner.ID, loser.ID))
	must(t, err)
	if got := f.jobs.count("seller.media_sync"); got != jobsBefore+2 {
		t.Fatalf("merge enqueued %d sync jobs, want one per image kind", got-jobsBefore)
	}
	f.syncProspectImages(t, loser.ID)
	for _, refType := range []string{media.RefTypeSellerProspectLogo, media.RefTypeSellerProspectCover} {
		if got := f.media.Referenced(f.prospectRef(refType, loser.ID)); len(got) != 0 {
			t.Fatalf("%s: references=%v, want everything unlinked", refType, got)
		}
	}
	if got := f.prospectRow(t, loser.ID); got.LogoKey != nil || got.CoverKey != nil {
		t.Fatalf("merged loser still points at images: %+v", got)
	}
}

// 轉成店家：converted_store_id 指向 store_id（不是帳號）；轉過一次之後再轉被擋；同一個店家只能被一個業者認領。
func TestConvertProspectToStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	p := f.newProspect(t, "甲", withContact("conv@example.com", "", "", ""))
	converted, err := f.module.ConvertProspect(ctx, ConvertProspectInput{ProspectID: p.ID, StoreID: s.StoreID, AdminID: prospectAdmin, ExpectedVersion: p.Version})
	must(t, err)
	if converted.Status != ProspectStatusConverted || converted.OutreachStatus != OutreachConverted || converted.ConvertedStore == nil || *converted.ConvertedStore != s.StoreID ||
		converted.ConvertedAt == nil || !converted.ConvertedAt.Equal(f.now) || converted.Version != p.Version+1 {
		t.Fatalf("converted=%+v", converted)
	}
	if s.StoreID == accountID {
		t.Fatal("fixture must keep store_id and account_id apart")
	}
	// 二次轉換：同一個業者轉到別的店家，也被擋（不會覆蓋 converted_store_id）。
	_, other := f.open(t)
	_, err = f.module.ConvertProspect(ctx, ConvertProspectInput{ProspectID: p.ID, StoreID: other.StoreID, AdminID: prospectAdmin, ExpectedVersion: converted.Version})
	wantKind(t, err, apperr.KindFailedPrecondition)
	if got := f.prospectRow(t, p.ID); *got.ConvertedStore != s.StoreID {
		t.Fatalf("second conversion overwrote converted_store_id: %+v", got)
	}
	// 同一個店家被另一個業者認領：AlreadyExists。
	q := f.newProspect(t, "乙", nil)
	_, err = f.module.ConvertProspect(ctx, ConvertProspectInput{ProspectID: q.ID, StoreID: s.StoreID, AdminID: prospectAdmin, ExpectedVersion: q.Version})
	wantKind(t, err, apperr.KindAlreadyExists)
	// 店家不存在、已關閉都是 NotFound；錯誤的轉換不改業者。
	_, err = f.module.ConvertProspect(ctx, ConvertProspectInput{ProspectID: q.ID, StoreID: s.StoreID + 9999, AdminID: prospectAdmin, ExpectedVersion: q.Version})
	wantKind(t, err, apperr.KindNotFound)
	_, err = f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	_, err = f.module.ConvertProspect(ctx, ConvertProspectInput{ProspectID: q.ID, StoreID: s.StoreID, AdminID: prospectAdmin, ExpectedVersion: q.Version})
	wantKind(t, err, apperr.KindNotFound)
	if got := f.prospectRow(t, q.ID); got.Status != ProspectStatusActive {
		t.Fatalf("failed conversion changed the prospect: %+v", got)
	}
	// 已入駐的業者不能封存或修改，但仍佔著比對鍵。
	_, err = f.module.ArchiveProspect(ctx, ArchiveProspectInput{ProspectID: p.ID, AdminID: prospectAdmin, ExpectedVersion: converted.Version})
	wantKind(t, err, apperr.KindFailedPrecondition)
	_, err = f.module.CreateProspect(ctx, CreateProspectInput{AdminID: prospectAdmin, Fields: ProspectFields{DisplayName: "丙", VendorType: VendorBusiness}, Contacts: contactList("conv@example.com", "", "", "")})
	wantMatchFound(t, err, p.ID)
}

func TestConcurrentConversionOfOneProspectSucceedsOnce(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, a := f.open(t)
	_, b := f.open(t)
	p := f.newProspect(t, "甲", nil)
	var wg sync.WaitGroup
	results := make([]error, 2)
	for i, store := range []int64{a.StoreID, b.StoreID} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, results[i] = f.module.ConvertProspect(context.Background(), ConvertProspectInput{ProspectID: p.ID, StoreID: store, AdminID: prospectAdmin, ExpectedVersion: p.Version})
		}()
	}
	wg.Wait()
	if (results[0] == nil) == (results[1] == nil) {
		t.Fatalf("results=%v", results)
	}
}

// 統編對帳以 KYC 申請的 store_id 為鍵（舊實作以 account_id 取店家，不分 stint）：只對得上目前的 stint。
func TestTaxIDMatchesAreKeyedByStoreID(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	insertApplication := func(storeID int64, taxID string, status string) {
		_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO seller.kyc_applications (application_id, store_id, status, submission_seq, seller_type, company_name, tax_id, tax_id_hash, tax_id_hash_key, created_at, updated_at)
			VALUES (gen_random_uuid(), $1, $2::text, CASE WHEN $2::text IN ('DRAFT', 'CANCELLED') THEN 0 ELSE 1 END, 'COMPANY', '測試公司', $3, decode(repeat('ab', 32), 'hex'), 'test', $4, $4)`, storeID, status, taxID, f.now)
		must(t, err)
	}
	accountID, oldStint := f.open(t)
	insertApplication(oldStint.StoreID, "55555555", "DRAFT")
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	newStint, err := f.module.Apply(ctx, validApply(accountID))
	must(t, err)
	insertApplication(newStint.StoreID, "66666666", "DRAFT")
	_, third := f.open(t)
	insertApplication(third.StoreID, "77777777", "CANCELLED")
	oldOne := f.newProspect(t, "舊 stint 的統編", func(in *CreateProspectInput) { in.Fields.TaxID = text("55555555") })
	newOne := f.newProspect(t, "新 stint 的統編", func(in *CreateProspectInput) { in.Fields.TaxID = text("66666666") })
	thirdOne := f.newProspect(t, "第三家", func(in *CreateProspectInput) { in.Fields.TaxID = text("77777777") })
	f.newProspect(t, "沒有申請", func(in *CreateProspectInput) { in.Fields.TaxID = text("88888888") })
	page, err := f.module.ProspectTaxIDMatches(ctx, 1, nil)
	must(t, err)
	if len(page.Items) != 1 || page.Next == nil {
		t.Fatalf("first page=%+v", page)
	}
	rest, err := f.module.ProspectTaxIDMatches(ctx, 10, page.Next)
	must(t, err)
	all := slices.Concat(page.Items, rest.Items)
	if len(all) != 2 || rest.Next != nil {
		t.Fatalf("matches=%+v", all)
	}
	for i, want := range []struct {
		prospect Prospect
		store    int64
		status   KYCStatus
	}{{newOne, newStint.StoreID, KYCStatusDraft}, {thirdOne, third.StoreID, KYCStatusCancelled}} {
		if all[i].Prospect.ID != want.prospect.ID || all[i].StoreID != want.store || all[i].KYCStatus != want.status || *all[i].CompanyName != "測試公司" {
			t.Fatalf("match %d=%+v, want prospect %d store %d", i, all[i], want.prospect.ID, want.store)
		}
	}
	for _, m := range all {
		if m.Prospect.ID == oldOne.ID || m.StoreID == oldStint.StoreID {
			t.Fatalf("matched the closed stint: %+v", m)
		}
	}
	// 轉成店家之後不再出現在對帳。
	_, err = f.module.ConvertProspect(ctx, ConvertProspectInput{ProspectID: newOne.ID, StoreID: newStint.StoreID, AdminID: prospectAdmin, ExpectedVersion: newOne.Version})
	must(t, err)
	page, err = f.module.ProspectTaxIDMatches(ctx, 10, nil)
	must(t, err)
	if len(page.Items) != 1 || page.Items[0].Prospect.ID != thirdOne.ID {
		t.Fatalf("after conversion=%+v", page.Items)
	}
}

// 統編對帳不列已被其他業者認領的店家：店家已經連到另一個業者，這個業者再對得上也沒有東西可連。
func TestTaxIDMatchesExcludeStoresClaimedByAnotherProspect(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	_, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO seller.kyc_applications (application_id, store_id, status, seller_type, company_name, tax_id, tax_id_hash, tax_id_hash_key, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, 'DRAFT', 'COMPANY', '測試公司', '55555555', decode(repeat('ab', 32), 'hex'), 'test', $2, $2)`, s.StoreID, f.now)
	must(t, err)
	byTaxID := f.newProspect(t, "統編對得上", func(in *CreateProspectInput) { in.Fields.TaxID = text("55555555") })
	claimer := f.newProspect(t, "手動認領的", nil)
	page, err := f.module.ProspectTaxIDMatches(ctx, 10, nil)
	must(t, err)
	if len(page.Items) != 1 || page.Items[0].Prospect.ID != byTaxID.ID || page.Items[0].StoreID != s.StoreID {
		t.Fatalf("before the claim: %+v", page.Items)
	}
	_, err = f.module.ConvertProspect(ctx, ConvertProspectInput{ProspectID: claimer.ID, StoreID: s.StoreID, AdminID: prospectAdmin, ExpectedVersion: claimer.Version})
	must(t, err)
	page, err = f.module.ProspectTaxIDMatches(ctx, 10, nil)
	must(t, err)
	if len(page.Items) != 0 {
		t.Fatalf("a store claimed by another prospect is still listed: %+v", page.Items)
	}
}

func searchIDs(t *testing.T, f *fixture, in ProspectSearchInput) []int64 {
	t.Helper()
	hits, err := f.module.SearchProspects(context.Background(), in)
	must(t, err)
	ids := make([]int64, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.Prospect.ID)
	}
	slices.Sort(ids)
	return ids
}

// P5：採購候選的加權 OR、部分比對、分類樹走子表、只看 ACTIVE。
func TestSearchProspectsSemantics(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	mango := f.newProspect(t, "芒果園", func(in *CreateProspectInput) {
		in.Fields.Category, in.Fields.Region, in.Fields.ProductSummary = text("水果"), text("台南東山"), text("愛文芒果")
		in.CategoryIDs = []string{"fruit.mango"}
	})
	tea := f.newProspect(t, "茶行", func(in *CreateProspectInput) {
		in.Fields.Category, in.Fields.Region, in.Fields.Positioning = text("茶葉"), text("嘉義竹崎"), text("高山茶")
	})
	egg := f.newProspect(t, "蛋場", func(in *CreateProspectInput) {
		in.Fields.Category, in.Fields.ResearchSummary = text("雞肉／蛋品"), text("100% 放牧")
		in.CategoryIDs = []string{"egg"}
	})
	archived := f.newProspect(t, "封存的芒果", func(in *CreateProspectInput) { in.Fields.Category = text("水果") })
	_, err := f.module.ArchiveProspect(ctx, ArchiveProspectInput{ProspectID: archived.ID, AdminID: prospectAdmin, ExpectedVersion: archived.Version})
	must(t, err)
	_, s := f.open(t)
	conv := f.newProspect(t, "已入駐的芒果", func(in *CreateProspectInput) { in.Fields.Category = text("水果") })
	_, err = f.module.ConvertProspect(ctx, ConvertProspectInput{ProspectID: conv.ID, StoreID: s.StoreID, AdminID: prospectAdmin, ExpectedVersion: conv.Version})
	must(t, err)

	for name, tt := range map[string]struct {
		in   ProspectSearchInput
		want []int64
	}{
		"沒有條件是全部 ACTIVE":     {ProspectSearchInput{}, []int64{mango.ID, tea.ID, egg.ID}},
		"分類部分比對":             {ProspectSearchInput{Category: "水"}, []int64{mango.ID}},
		"地區泛稱比對細粒度的值":        {ProspectSearchInput{Region: "台南"}, []int64{mango.ID}},
		"加權 OR：分類對、地區不對仍然入選": {ProspectSearchInput{Category: "水果", Region: "嘉義"}, []int64{mango.ID, tea.ID}},
		"關鍵字比對三段文字":          {ProspectSearchInput{Keywords: "高山"}, []int64{tea.ID}},
		"分類樹 ID 走子表":         {ProspectSearchInput{CategoryIDs: []string{"egg", "nothing"}}, []int64{egg.ID}},
		"分類、關鍵字、分類樹是 OR":     {ProspectSearchInput{Keywords: "芒果", CategoryIDs: []string{"egg"}}, []int64{mango.ID, egg.ID}},
		"% 不是萬用字元":           {ProspectSearchInput{Keywords: "%"}, []int64{egg.ID}},
		"_ 不是萬用字元":           {ProspectSearchInput{Keywords: "_"}, nil},
		"空白的條件等於沒填":          {ProspectSearchInput{Category: "  ", CategoryIDs: []string{" "}}, []int64{mango.ID, tea.ID, egg.ID}},
		"沒有命中":               {ProspectSearchInput{Category: "不存在"}, nil},
	} {
		got := searchIDs(t, f, tt.in)
		if !slices.Equal(got, tt.want) && (len(got) != 0 || len(tt.want) != 0) {
			t.Errorf("%s: got %v, want %v", name, got, tt.want)
		}
	}
	hits, err := f.module.SearchProspects(ctx, ProspectSearchInput{Category: "水果", Region: "嘉義", Keywords: "芒果", CategoryIDs: []string{"fruit.mango"}})
	must(t, err)
	for _, h := range hits {
		switch h.Prospect.ID {
		case mango.ID:
			if !h.HitCategory || h.HitRegion || !h.HitKeywords || !h.HitCategoryIDs {
				t.Errorf("mango hits=%+v", h)
			}
		case tea.ID:
			if h.HitCategory || !h.HitRegion || h.HitKeywords || h.HitCategoryIDs {
				t.Errorf("tea hits=%+v", h)
			}
		}
	}
	// Limit 是安全上限。
	hits, err = f.module.SearchProspects(ctx, ProspectSearchInput{Limit: 2})
	must(t, err)
	if len(hits) != 2 {
		t.Fatalf("limit 2 returned %d", len(hits))
	}
	if _, err := f.module.SearchProspects(ctx, ProspectSearchInput{Limit: -1}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("negative limit: %v", err)
	}
}

// ai-service 只拿主要的網站（email、電話、社群由資料類別擋下，不在這條路上）。
func TestSearchProspectsReturnsThePrimaryWebsite(t *testing.T) {
	t.Parallel()
	f := setup(t)
	with := f.newProspect(t, "有網站", func(in *CreateProspectInput) {
		in.Contacts = []ContactPoint{{Kind: ContactWebsite, Value: "https://a.example.com", Primary: false}, {Kind: ContactWebsite, Value: "https://main.example.com", Primary: true}, {Kind: ContactEmail, Value: "x@example.com", Primary: true}}
		in.Fields.Category = text("水果")
	})
	without := f.newProspect(t, "沒網站", withContact("y@example.com", "", "", "https://www.facebook.com/y"))
	hits, err := f.module.SearchProspects(context.Background(), ProspectSearchInput{})
	must(t, err)
	got := map[int64]*string{}
	for _, h := range hits {
		got[h.Prospect.ID] = h.Website
	}
	if len(got) != 2 || got[with.ID] == nil || *got[with.ID] != "https://main.example.com" || got[without.ID] != nil {
		t.Fatalf("websites=%v", got)
	}
}

func TestProspectContactForVendorPortal(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	with := f.newProspect(t, "有 email", withContact("Vendor@Example.com", "", "", ""))
	without := f.newProspect(t, "沒有 email", withContact("", "02-1234-5678", "", ""))
	c, err := f.module.ProspectContact(ctx, with.ID)
	must(t, err)
	if c.ProspectID != with.ID || c.DisplayName != "有 email" || c.Email == nil || *c.Email != "Vendor@Example.com" || c.Status != ProspectStatusActive {
		t.Fatalf("contact=%+v", c)
	}
	c, err = f.module.ProspectContact(ctx, without.ID)
	must(t, err)
	if c.Email != nil {
		t.Fatalf("contact=%+v", c)
	}
	_, err = f.module.ProspectContact(ctx, without.ID+999)
	wantKind(t, err, apperr.KindNotFound)
	// 封存的業者仍讀得到（邀請流程不因名單整理而斷）。
	_, err = f.module.ArchiveProspect(ctx, ArchiveProspectInput{ProspectID: with.ID, AdminID: prospectAdmin, ExpectedVersion: with.Version})
	must(t, err)
	c, err = f.module.ProspectContact(ctx, with.ID)
	must(t, err)
	if c.Status != ProspectStatusArchived {
		t.Fatalf("封存的業者 status = %s", c.Status)
	}
}

// vendor portal 取主要的 email（不是第一個、也不是最新的）；主要的換了就取新的。
func TestProspectContactReturnsThePrimaryEmail(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := f.newProspect(t, "多個 email", func(in *CreateProspectInput) {
		in.Contacts = []ContactPoint{{Kind: ContactEmail, Value: "a@example.com", Primary: false}, {Kind: ContactEmail, Value: "Main@Example.com", Primary: true}, {Kind: ContactEmail, Value: "z@example.com", Primary: false}}
	})
	c, err := f.module.ProspectContact(ctx, p.ID)
	must(t, err)
	if c.Email == nil || *c.Email != "Main@Example.com" {
		t.Fatalf("contact=%+v", c)
	}
	_, err = f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: p.Version, AdminID: prospectAdmin, Contacts: &[]ContactPoint{
		{Kind: ContactEmail, Value: "a@example.com", Primary: false}, {Kind: ContactEmail, Value: "Main@Example.com", Primary: false}, {Kind: ContactEmail, Value: "z@example.com", Primary: true},
	}})
	must(t, err)
	if c, err = f.module.ProspectContact(ctx, p.ID); err != nil || c.Email == nil || *c.Email != "z@example.com" {
		t.Fatalf("contact=%+v err=%v", c, err)
	}
}

// ProspectContact 在呼叫方的 transaction 內讀，那一列帶共享鎖：與封存、合併序列化（vendor portal 的 Ensure 靠這點，讀到進行中之後
// 封存不能插進來）。讀完之前的封存要等呼叫方提交；提交之後封存才進得去，下一次讀到的就是封存。
func TestProspectContactInTransactionBlocksArchiveUntilCommit(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := f.newProspect(t, "被鎖住", withContact("a@example.com", "", "", ""))
	other := f.newProspect(t, "合併的對象", nil)
	readActive := make(chan struct{})
	release := make(chan struct{})
	committed := make(chan struct{})
	go func() {
		defer close(committed)
		err := f.db.InTx(ctx, func(ctx context.Context) error {
			c, err := f.module.ProspectContact(ctx, p.ID)
			if err != nil || c.Status != ProspectStatusActive {
				t.Errorf("contact=%+v err=%v", c, err)
			}
			close(readActive)
			<-release
			return nil
		})
		if err != nil {
			t.Error(err)
		}
	}()
	<-readActive
	archived := make(chan error, 2)
	go func() {
		_, err := f.module.ArchiveProspect(ctx, ArchiveProspectInput{ProspectID: p.ID, AdminID: prospectAdmin, ExpectedVersion: p.Version})
		archived <- err
	}()
	go func() {
		_, err := f.module.MergeProspects(ctx, MergeProspectsInput{WinnerID: other.ID, LoserID: p.ID, AdminID: prospectAdmin, WinnerVersion: other.Version, LoserVersion: p.Version})
		archived <- err
	}()
	select {
	case err := <-archived:
		t.Fatalf("共享鎖之下封存或合併不該完成：%v", err)
	case <-time.After(500 * time.Millisecond):
	}
	close(release)
	<-committed
	for range 2 {
		// 一個成功、另一個依狀態成功或回 FailedPrecondition，這裡只確認都放行了。
		if err := <-archived; err != nil {
			t.Logf("放行後的結果：%v", err)
		}
	}
	c, err := f.module.ProspectContact(ctx, p.ID)
	must(t, err)
	if c.Status != ProspectStatusArchived {
		t.Fatalf("status = %s", c.Status)
	}
}

func TestListProspectsFiltersAndPaginates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	var ids []int64
	for i := range 5 {
		f.now = f.now.Add(time.Minute)
		p := f.newProspect(t, "業者"+strconv.Itoa(i), func(in *CreateProspectInput) {
			in.Fields.Category = text("水果")
			if i%2 == 0 {
				in.Fields.VendorType = VendorIndividual
				in.Fields.ProductSummary = text("含 100% 與 under_score")
			}
		})
		ids = append(ids, p.ID)
	}
	f.now = f.now.Add(time.Minute)
	_, err := f.module.ArchiveProspect(ctx, ArchiveProspectInput{ProspectID: ids[4], AdminID: prospectAdmin, ExpectedVersion: 1})
	must(t, err)
	var got []int64
	var after *ProspectCursor
	for range 5 {
		page, err := f.module.Prospects(ctx, ProspectListInput{PageSize: 2, After: after})
		must(t, err)
		for _, p := range page.Items {
			got = append(got, p.ID)
		}
		if page.Next == nil {
			break
		}
		after = page.Next
	}
	if want := []int64{ids[3], ids[2], ids[1], ids[0]}; !slices.Equal(got, want) { // 新到舊，預設不含封存
		t.Fatalf("got %v, want %v", got, want)
	}
	archived := ProspectStatusArchived
	page, err := f.module.Prospects(ctx, ProspectListInput{Status: &archived})
	must(t, err)
	if len(page.Items) != 1 || page.Items[0].ID != ids[4] {
		t.Fatalf("archived=%+v", page.Items)
	}
	individual := VendorIndividual
	page, err = f.module.Prospects(ctx, ProspectListInput{VendorType: &individual, Category: "水果"})
	must(t, err)
	if len(page.Items) != 2 { // 業者 0 與 2（4 已封存）
		t.Fatalf("individuals=%d", len(page.Items))
	}
	// % 與 _ 只當字面比對：「%%」不是萬用字元，「_」不會比對任意一個字。
	for keyword, want := range map[string]int{"100%": 2, "under_score": 2, "業者1": 1, "%%": 0, "under_scor_": 0, "不存在": 0} {
		page, err = f.module.Prospects(ctx, ProspectListInput{Keyword: keyword})
		must(t, err)
		if len(page.Items) != want {
			t.Errorf("keyword %q: %d items, want %d", keyword, len(page.Items), want)
		}
	}
	bad := ProspectStatus("DELETED")
	if _, err := f.module.Prospects(ctx, ProspectListInput{Status: &bad}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("bad status: %v", err)
	}
}

func TestArchiveProspectPublishesInTheTransactionOnlyOnTheTransition(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	archived, merged := record[ProspectArchived](f), record[ProspectMerged](f)
	a := f.newProspect(t, "甲", nil)
	got, err := f.module.ArchiveProspect(ctx, ArchiveProspectInput{ProspectID: a.ID, AdminID: prospectAdmin, ExpectedVersion: a.Version})
	must(t, err)
	if evs := archived.all(); len(evs) != 1 || evs[0].ProspectID != a.ID || !evs[0].OccurredAt.Equal(f.now) || evs[0].EventID == (uuid.UUID{}) {
		t.Fatalf("archived=%+v", evs)
	}
	// 重送是 no-op，不發事件；還原不發事件。
	_, err = f.module.ArchiveProspect(ctx, ArchiveProspectInput{ProspectID: a.ID, AdminID: prospectAdmin, ExpectedVersion: got.Version})
	must(t, err)
	restored, err := f.restore(t, a.ID)
	must(t, err)
	if len(archived.all()) != 1 || len(merged.all()) != 0 || got.Status != ProspectStatusArchived {
		t.Fatalf("archived=%+v merged=%+v", archived.all(), merged.all())
	}
	// 還原之後再封存是新的轉移，再發一次。
	_, err = f.module.ArchiveProspect(ctx, ArchiveProspectInput{ProspectID: a.ID, AdminID: prospectAdmin, ExpectedVersion: restored.Version})
	must(t, err)
	if len(archived.all()) != 2 {
		t.Fatalf("archived=%+v", archived.all())
	}
}

func TestArchiveProspectEventFailureRollsBackTheArchive(t *testing.T) {
	t.Parallel()
	f := setup(t)
	events.Subscribe(f.bus, func(context.Context, ProspectArchived) error { return errors.New("reject") })
	a := f.newProspect(t, "甲", nil)
	if _, err := f.module.ArchiveProspect(context.Background(), ArchiveProspectInput{ProspectID: a.ID, AdminID: prospectAdmin, ExpectedVersion: a.Version}); err == nil {
		t.Fatal("event failure lost")
	}
	if got := f.prospectRow(t, a.ID); got.Status != ProspectStatusActive || got.Version != a.Version {
		t.Fatalf("archive survived the rollback: %+v", got)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.prospects WHERE prospect_id = $1 AND status = 'ARCHIVED'", a.ID); n != 0 {
		t.Fatalf("archived rows=%d", n)
	}
}

func TestMergeProspectsPublishesOnlyProspectMerged(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	archived, merged := record[ProspectArchived](f), record[ProspectMerged](f)
	winner, loser := f.newProspect(t, "贏家", nil), f.newProspect(t, "輸家", nil)
	_, err := f.module.MergeProspects(ctx, f.mergeInput(t, winner.ID, loser.ID))
	must(t, err)
	evs := merged.all()
	if len(evs) != 1 || evs[0].SourceProspectID != loser.ID || evs[0].TargetProspectID != winner.ID || !evs[0].OccurredAt.Equal(f.now) || evs[0].EventID == (uuid.UUID{}) {
		t.Fatalf("merged=%+v", evs)
	}
	// 輸家雖然被封存，封存只由 ProspectMerged 表達。
	if len(archived.all()) != 0 || f.prospectRow(t, loser.ID).Status != ProspectStatusArchived {
		t.Fatalf("archived=%+v", archived.all())
	}
}

func TestMergeProspectsEventFailureRollsBackTheMerge(t *testing.T) {
	t.Parallel()
	f := setup(t)
	events.Subscribe(f.bus, func(context.Context, ProspectMerged) error { return errors.New("reject") })
	winner, loser := f.newProspect(t, "贏家", nil), f.newProspect(t, "輸家", nil)
	if _, err := f.module.MergeProspects(context.Background(), f.mergeInput(t, winner.ID, loser.ID)); err == nil {
		t.Fatal("event failure lost")
	}
	if got := f.prospectRow(t, loser.ID); got.Status != ProspectStatusActive || got.MergedInto != nil {
		t.Fatalf("merge survived the rollback: %+v", got)
	}
	if got := f.prospectRow(t, winner.ID); got.Version != winner.Version {
		t.Fatalf("winner changed: %+v", got)
	}
}

// 每個類型恰好一個主要的：同類型兩個主要的由唯一索引擋下，有聯絡方式卻沒有主要的、刪掉主要的而留下其他的，由 commit 時檢查的
// 外鍵擋下；整批替換在同一個 transaction 內刪了再寫，檢查只看最後的狀態。
func TestExactlyOnePrimaryContactPerKindIsEnforcedByTheDatabase(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	p := f.newProspect(t, "甲", withContact("a@example.com", "", "", ""))
	exec := func(query string, args ...any) error {
		_, err := f.db.Conn(ctx).Exec(ctx, query, args...)
		return err
	}
	err := exec(`INSERT INTO seller.prospect_contacts (prospect_id, kind, value, is_primary) VALUES ($1, 'EMAIL', 'b@example.com', true)`, p.ID)
	wantUniqueViolation(t, err, "prospect_contacts_primary")
	var pgErr *pgconn.PgError
	err = exec(`INSERT INTO seller.prospect_contacts (prospect_id, kind, value, is_primary) VALUES ($1, 'PHONE', '02-1', false)`, p.ID)
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != "prospect_contacts_has_primary" {
		t.Fatalf("a kind with contacts but no primary: %v", err)
	}
	must(t, exec(`INSERT INTO seller.prospect_contacts (prospect_id, kind, value, is_primary) VALUES ($1, 'EMAIL', 'b@example.com', false)`, p.ID))
	err = exec(`DELETE FROM seller.prospect_contacts WHERE prospect_id = $1 AND value = 'a@example.com'`, p.ID)
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != "prospect_contacts_has_primary" {
		t.Fatalf("deleting the primary but keeping the others: %v", err)
	}
	// 同一個 transaction 內換主要的是合法的。
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		if _, err := f.db.Conn(ctx).Exec(ctx, `UPDATE seller.prospect_contacts SET is_primary = false WHERE prospect_id = $1 AND value = 'a@example.com'`, p.ID); err != nil {
			return err
		}
		_, err := f.db.Conn(ctx).Exec(ctx, `UPDATE seller.prospect_contacts SET is_primary = true WHERE prospect_id = $1 AND value = 'b@example.com'`, p.ID)
		return err
	}))
	if got := contactsOf(f.prospectRow(t, p.ID)); !slices.Equal(got, []string{"EMAIL:a@example.com", "EMAIL:b@example.com*"}) {
		t.Fatalf("contacts=%v", got)
	}
	// 配對的格式：較小的 ID 在前。
	other := f.newProspect(t, "乙", nil)
	err = exec(`INSERT INTO seller.prospect_distinct_pairs VALUES ($1, $2, $3, now())`, other.ID, p.ID, prospectAdmin)
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != "prospect_distinct_pairs_order_check" {
		t.Fatalf("a pair with the larger ID first: %v", err)
	}
}

// migration 套用在已有業者資料的 DB 上：public_* 成為各類型的主要聯絡方式，識別子裡沒有被它們認領的鍵（合併帶來的）成為非主要的
// 聯絡方式（某個類型沒有主要的時補一個），共用平台的網域鍵不帶入，稽核欄改名；每一列的比對鍵與解析器對它的值推導的一致，
// 所以整批回送（Update 的 Contacts）不會弄丟鍵。
func TestProspectContactsMigrationBackfillsExistingProspects(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t)
	files, err := fs.Glob(Migrations(), "*.sql")
	must(t, err)
	last := len(files) - 1
	for last >= 0 && !strings.HasSuffix(files[last], "_prospect_contacts.sql") {
		last--
	}
	if last < 1 {
		t.Fatalf("migrations=%v", files)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := d.Pool().Exec(ctx, sql, args...)
		must(t, err)
	}
	for _, name := range files[:last] {
		raw, err := fs.ReadFile(Migrations(), name)
		must(t, err)
		exec(string(raw))
	}
	seed := func(name, status string, email, phone, website, social *string, createdBy int64) int64 {
		t.Helper()
		var id int64
		must(t, d.Pool().QueryRow(ctx, `INSERT INTO seller.prospects (display_name, vendor_type, status, public_email, public_phone, public_website, public_social, created_by_account_id, updated_by_account_id, created_at, updated_at)
			VALUES ($1, 'BUSINESS', $2, $3, $4, $5, $6, $7, $7, now(), now()) RETURNING prospect_id`, name, status, email, phone, website, social, createdBy).Scan(&id))
		return id
	}
	identifier := func(id int64, live bool, kind, value string) {
		t.Helper()
		exec(`INSERT INTO seller.prospect_identifiers (prospect_id, live, kind, value_normalized) VALUES ($1, $2, $3, $4)`, id, live, kind, value)
	}
	merged := seed("合併過的", "ACTIVE", ptr("Hello@Example.COM"), ptr("+886-2-1234-5678"), ptr("https://www.example.com/shop"), ptr("https://www.threads.com/@dailyfarm"), 7)
	for _, k := range [][2]string{
		{"EMAIL", "hello@example.com"}, {"PHONE", "0212345678"}, {"DOMAIN", "example.com"}, {"SOCIAL", "threads:dailyfarm"},
		{"EMAIL", "l@example.com"}, {"PHONE", "0233334444"}, {"DOMAIN", "loser.tw"}, {"SOCIAL", "facebook:loserpage"}, {"SOCIAL", "line:loserline"},
	} {
		identifier(merged, true, k[0], k[1])
	}
	archived := seed("封存的", "ARCHIVED", ptr("待確認"), nil, ptr("https://linktr.ee/a"), nil, 8)
	identifier(archived, false, "DOMAIN", "linktr.ee")
	onlyExtras := seed("只有合併帶來的", "ACTIVE", nil, nil, nil, nil, 9)
	identifier(onlyExtras, true, "EMAIL", "z@y.tw")
	identifier(onlyExtras, true, "EMAIL", "x@y.tw")
	social := seed("社群網站", "ACTIVE", nil, nil, ptr("https://www.instagram.com/i_honey.tw"), nil, 10)
	identifier(social, true, "SOCIAL", "instagram:i_honey.tw")
	empty := seed("沒有聯絡方式", "ACTIVE", nil, nil, nil, nil, 11)

	raw, err := fs.ReadFile(Migrations(), files[last])
	must(t, err)
	exec(string(raw))

	rows, err := d.Pool().Query(ctx, `SELECT prospect_id, kind, value, COALESCE(match_key, ''), is_primary FROM seller.prospect_contacts ORDER BY prospect_id, kind, value`)
	must(t, err)
	got := map[int64][]string{}
	for rows.Next() {
		var id int64
		var kind, value, key string
		var primary bool
		must(t, rows.Scan(&id, &kind, &value, &key, &primary))
		if want := model.ContactMatchKey(model.ContactKind(kind), value); want != key {
			t.Errorf("prospect %d %s %q: stored key %q, the parser says %q", id, kind, value, key, want)
		}
		entry := kind + ":" + value
		if primary {
			entry += "*"
		}
		got[id] = append(got[id], entry)
	}
	must(t, rows.Err())
	want := map[int64][]string{
		merged: {
			"EMAIL:Hello@Example.COM*", "EMAIL:l@example.com", "PHONE:+886-2-1234-5678*", "PHONE:0233334444",
			"SOCIAL:https://line.me/@loserline", "SOCIAL:https://www.facebook.com/loserpage", "SOCIAL:https://www.threads.com/@dailyfarm*",
			"WEBSITE:https://www.example.com/shop*", "WEBSITE:loser.tw",
		},
		archived:   {"EMAIL:待確認*", "WEBSITE:https://linktr.ee/a*"},
		onlyExtras: {"EMAIL:x@y.tw*", "EMAIL:z@y.tw"},
		social:     {"WEBSITE:https://www.instagram.com/i_honey.tw*"},
	}
	for id, w := range want {
		slices.Sort(w)
		slices.Sort(got[id])
		if !slices.Equal(got[id], w) {
			t.Errorf("prospect %d contacts=%v, want %v", id, got[id], w)
		}
	}
	if len(got[empty]) != 0 {
		t.Errorf("a prospect without contacts got %v", got[empty])
	}
	var keysOfMerged []string
	must(t, d.Pool().QueryRow(ctx, `SELECT array_agg(match_key ORDER BY match_key) FROM seller.prospect_contacts WHERE prospect_id = $1 AND match_key IS NOT NULL`, merged).Scan(&keysOfMerged))
	wantKeys := []string{"DOMAIN:example.com", "DOMAIN:loser.tw", "EMAIL:hello@example.com", "EMAIL:l@example.com", "PHONE:0212345678", "PHONE:0233334444", "SOCIAL:facebook:loserpage", "SOCIAL:line:loserline", "SOCIAL:threads:dailyfarm"}
	if !slices.Equal(keysOfMerged, wantKeys) {
		t.Errorf("the keys of the merged prospect changed: %v, want %v", keysOfMerged, wantKeys)
	}
	var noPrimary int64
	must(t, d.Pool().QueryRow(ctx, `SELECT count(*) FROM (SELECT prospect_id, kind FROM seller.prospect_contacts GROUP BY 1, 2 HAVING count(*) FILTER (WHERE is_primary) <> 1) x`).Scan(&noPrimary))
	if noPrimary != 0 {
		t.Errorf("%d (prospect, kind) groups without exactly one primary", noPrimary)
	}
	var createdBy, updatedBy int64
	must(t, d.Pool().QueryRow(ctx, `SELECT created_by_admin_id, updated_by_admin_id FROM seller.prospects WHERE prospect_id = $1`, merged).Scan(&createdBy, &updatedBy))
	if createdBy != 7 || updatedBy != 7 {
		t.Errorf("audit columns: created=%d updated=%d", createdBy, updatedBy)
	}
	var oldColumns int64
	must(t, d.Pool().QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema = 'seller' AND table_name = 'prospects' AND column_name IN ('public_email', 'public_phone', 'public_website', 'public_social', 'created_by_account_id', 'updated_by_account_id')`).Scan(&oldColumns))
	var identifiersTable *string
	must(t, d.Pool().QueryRow(ctx, `SELECT to_regclass('seller.prospect_identifiers')::text`).Scan(&identifiersTable))
	if oldColumns != 0 || identifiersTable != nil {
		t.Errorf("old columns=%d identifiers table=%v", oldColumns, identifiersTable)
	}
}

// 不推斷重送：命令生效後帶舊版本再送一次一律 VersionConflict（先於狀態檢查），不寫入、不發事件；
// 帶目前版本的封存對已經封存的業者回傳目前的列（既有的狀態規則）。
func TestStaleEtagAfterArchiveMergeAndConvertConflicts(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	archivedEvents, mergedEvents := record[ProspectArchived](f), record[ProspectMerged](f)
	version := func(id int64) int64 { return f.prospectRow(t, id).Version }

	a := f.newProspect(t, "封存", nil)
	archiveIn := ArchiveProspectInput{ProspectID: a.ID, AdminID: prospectAdmin, ExpectedVersion: a.Version}
	first, err := f.module.ArchiveProspect(ctx, archiveIn)
	must(t, err)
	_, err = f.module.ArchiveProspect(ctx, archiveIn)
	wantKind(t, err, apperr.KindVersionConflict)
	if len(archivedEvents.all()) != 1 || version(a.ID) != first.Version {
		t.Fatalf("stale archive wrote: events=%d version=%d", len(archivedEvents.all()), version(a.ID))
	}
	// 帶目前的版本封存已封存的業者：回傳目前的列，不寫入、不發事件。
	same, err := f.module.ArchiveProspect(ctx, ArchiveProspectInput{ProspectID: a.ID, AdminID: prospectAdmin, ExpectedVersion: first.Version})
	must(t, err)
	if same.Version != first.Version || len(archivedEvents.all()) != 1 {
		t.Fatalf("archive at the current version: %+v events=%d", same, len(archivedEvents.all()))
	}
	// 還原之後（別的寫入）同樣的舊版本一樣是衝突。
	_, err = f.restore(t, a.ID)
	must(t, err)
	_, err = f.module.ArchiveProspect(ctx, archiveIn)
	wantKind(t, err, apperr.KindVersionConflict)
	if got := f.prospectRow(t, a.ID); got.Status != ProspectStatusActive {
		t.Fatalf("stale archive wrote: %+v", got)
	}

	winner, loser := f.newProspect(t, "贏家", nil), f.newProspect(t, "輸家", nil)
	mergeIn := f.mergeInput(t, winner.ID, loser.ID)
	merged, err := f.module.MergeProspects(ctx, mergeIn)
	must(t, err)
	_, err = f.module.MergeProspects(ctx, mergeIn)
	wantKind(t, err, apperr.KindVersionConflict)
	if len(mergedEvents.all()) != 1 || version(winner.ID) != merged.Version {
		t.Fatalf("stale merge wrote: events=%d version=%d", len(mergedEvents.all()), version(winner.ID))
	}
	// 贏家之後又被改過：同樣的舊版本一樣是衝突。
	_, err = f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: winner.ID, ExpectedVersion: merged.Version, AdminID: prospectAdmin, Region: ptr("花蓮")})
	must(t, err)
	_, err = f.module.MergeProspects(ctx, mergeIn)
	wantKind(t, err, apperr.KindVersionConflict)
	// 輸家已經併進別人：用輸家的舊版本併進另一個贏家，也是衝突（版本先於狀態）。
	other := f.newProspect(t, "別的贏家", nil)
	_, err = f.module.MergeProspects(ctx, MergeProspectsInput{WinnerID: other.ID, LoserID: loser.ID, AdminID: prospectAdmin, WinnerVersion: other.Version, LoserVersion: mergeIn.LoserVersion})
	wantKind(t, err, apperr.KindVersionConflict)
	if len(mergedEvents.all()) != 1 || version(other.ID) != other.Version {
		t.Fatalf("stale merge wrote: events=%d", len(mergedEvents.all()))
	}

	_, s := f.open(t)
	_, other2 := f.open(t)
	c := f.newProspect(t, "轉換", nil)
	convertIn := ConvertProspectInput{ProspectID: c.ID, StoreID: s.StoreID, AdminID: prospectAdmin, ExpectedVersion: c.Version}
	converted, err := f.module.ConvertProspect(ctx, convertIn)
	must(t, err)
	_, err = f.module.ConvertProspect(ctx, convertIn)
	wantKind(t, err, apperr.KindVersionConflict)
	if got := f.prospectRow(t, c.ID); got.Version != converted.Version || got.ConvertedStore == nil || *got.ConvertedStore != s.StoreID {
		t.Fatalf("stale convert wrote: %+v", got)
	}
	// 帶目前的版本再轉：轉過的回 FailedPrecondition（既有的狀態規則）。
	convertIn.ExpectedVersion = converted.Version
	_, err = f.module.ConvertProspect(ctx, convertIn)
	wantKind(t, err, apperr.KindFailedPrecondition)
	// 轉換之前有別的寫入：舊版本是衝突。
	d := f.newProspect(t, "有人先改", nil)
	_, err = f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: d.ID, ExpectedVersion: d.Version, AdminID: prospectAdmin, Region: ptr("宜蘭")})
	must(t, err)
	_, err = f.module.ConvertProspect(ctx, ConvertProspectInput{ProspectID: d.ID, StoreID: other2.StoreID, AdminID: prospectAdmin, ExpectedVersion: d.Version})
	wantKind(t, err, apperr.KindVersionConflict)
}

// 兩個管理員互相確認對方不是同一家（A 確認 B ∥ B 確認 A）：外鍵檢查對對方的列取 KEY SHARE，業者列的鎖不能與它衝突，否則兩邊互鎖
// （40P01）。兩邊都必須完成；重複的回合讓交錯的機會出現，不看時間。
func TestCrossedDistinctConfirmationsDoNotDeadlock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	for round := range 15 {
		a := f.newProspect(t, "甲"+strconv.Itoa(round), nil)
		b := f.newProspect(t, "乙"+strconv.Itoa(round), nil)
		key := "EMAIL:x" + strconv.Itoa(round) + "@example.com"
		f.insertContact(t, a.ID, "EMAIL", "x"+strconv.Itoa(round)+"@example.com", key, true)
		f.insertContact(t, b.ID, "EMAIL", "X"+strconv.Itoa(round)+"@example.com", key, true)
		var wg sync.WaitGroup
		results := make([]error, 2)
		for i, pair := range [][2]Prospect{{a, b}, {b, a}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				self, other := f.prospectRow(t, pair[0].ID), pair[1]
				contacts := append(slices.Clone(self.Contacts), ContactPoint{Kind: ContactPhone, Value: "02-" + strconv.Itoa(round+1000) + "-" + map[bool]string{true: "1", false: "2"}[pair[0].ID == a.ID], Primary: true})
				_, results[i] = f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: self.ID, ExpectedVersion: self.Version, AdminID: prospectAdmin, Contacts: &contacts, ConfirmedDistinct: []int64{other.ID}})
			}()
		}
		wg.Wait()
		for i, err := range results {
			var pgErr *pgconn.PgError
			if err != nil || errors.As(err, &pgErr) {
				t.Fatalf("round %d side %d: %v", round, i, err)
			}
		}
		if got := f.distinctPairs(t); len(got) != round+1 {
			t.Fatalf("round %d pairs=%v", round, got)
		}
	}
}

// 封存與合併清掉圖片時一併清掉來源網址（沒有圖片就沒有來源）；還原之後沒有來源網址，資料庫也拒絕沒有圖片卻有來源網址的列。
func TestArchiveAndMergeClearImageSourceURLs(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	withSources := func(name string) Prospect {
		p := f.newProspect(t, name, nil)
		p, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: p.Version, AdminID: prospectAdmin,
			Logo: &ProspectImageUpload{StagedToken: name + "-l", SourceURL: "https://source.example/l"}, Cover: &ProspectImageUpload{StagedToken: name + "-c", SourceURL: "https://source.example/c"}})
		must(t, err)
		if p.LogoSourceURL == nil || p.CoverSourceURL == nil {
			t.Fatalf("setup: %+v", p)
		}
		return p
	}
	a := withSources("封存")
	archived := f.archive(t, a.ID)
	if archived.LogoSourceURL != nil || archived.CoverSourceURL != nil || archived.LogoKey != nil {
		t.Fatalf("archived=%+v", archived)
	}
	restored, err := f.restore(t, a.ID)
	must(t, err)
	if restored.LogoSourceURL != nil || restored.CoverSourceURL != nil {
		t.Fatalf("restored=%+v", restored)
	}
	loser := withSources("輸家")
	f.merge(t, f.newProspect(t, "贏家", nil).ID, loser.ID)
	if got := f.prospectRow(t, loser.ID); got.LogoSourceURL != nil || got.CoverSourceURL != nil {
		t.Fatalf("merged loser=%+v", got)
	}
	p := f.newProspect(t, "直接寫", nil)
	_, err = f.db.Conn(ctx).Exec(ctx, "UPDATE seller.prospects SET cover_source_url = 'https://x.example' WHERE prospect_id = $1", p.ID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != "prospects_source_urls_check" {
		t.Fatalf("a source URL without an image: %v", err)
	}
}

// 直接標記配對：名稱相同的一對與並行建立漏網的一對（比對鍵相同）都能不經合併處理；標記後不再出現在清單，也不再擋後續的比對；
// 重送成功、不改寫第一次的管理員；同一個 ID、不存在、不是進行中的業者被拒絕，什麼都不寫。
func TestMarkProspectsDistinctDismissesACandidatePair(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	a := f.newProspect(t, "好農場", nil)
	b := f.newProspect(t, "好農場", nil)
	e := f.newProspect(t, "東", withContact("shared@example.com", "", "", ""))
	g := f.newProspect(t, "西", nil)
	f.insertContact(t, g.ID, "EMAIL", "Shared@Example.com", "EMAIL:shared@example.com", true)
	if got, want := f.duplicatePairs(t), [][2]int64{{a.ID, b.ID}, {e.ID, g.ID}}; !slices.Equal(got, want) {
		t.Fatalf("pairs=%v, want %v", got, want)
	}
	mark := func(admin, x, y int64) error {
		return f.module.MarkProspectsDistinct(ctx, MarkProspectsDistinctInput{FirstID: x, SecondID: y, AdminID: admin})
	}
	// ID 的順序不影響：較大的在前也寫成較小的在前。
	must(t, mark(prospectAdmin, b.ID, a.ID))
	must(t, mark(prospectAdmin, e.ID, g.ID))
	if got := f.duplicatePairs(t); len(got) != 0 {
		t.Fatalf("標記之後清單應該是空的：%v", got)
	}
	if got, want := f.distinctPairs(t), [][2]int64{{a.ID, b.ID}, {e.ID, g.ID}}; !slices.Equal(got, want) {
		t.Fatalf("pairs=%v, want %v", got, want)
	}
	// 重送（連同別的管理員）：成功，第一次的管理員與時間不變。
	var firstAdmin int64
	var firstAt time.Time
	row := f.db.Conn(ctx).QueryRow(ctx, "SELECT confirmed_by_admin_id, confirmed_at FROM seller.prospect_distinct_pairs WHERE low_prospect_id = $1", a.ID)
	must(t, row.Scan(&firstAdmin, &firstAt))
	must(t, mark(prospectAdmin+1, a.ID, b.ID))
	var admin2 int64
	var at2 time.Time
	row = f.db.Conn(ctx).QueryRow(ctx, "SELECT confirmed_by_admin_id, confirmed_at FROM seller.prospect_distinct_pairs WHERE low_prospect_id = $1", a.ID)
	must(t, row.Scan(&admin2, &at2))
	if firstAdmin != prospectAdmin || admin2 != firstAdmin || !at2.Equal(firstAt) || len(f.distinctPairs(t)) != 2 {
		t.Fatalf("重送改寫了第一次的紀錄：%d %v → %d %v", firstAdmin, firstAt, admin2, at2)
	}
	// 標記之後，聯絡方式相同也不再擋（比對只看沒有標記的）。
	if _, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: e.ID, ExpectedVersion: e.Version, AdminID: prospectAdmin,
		Contacts: &[]ContactPoint{{Kind: ContactEmail, Value: "shared@example.com", Primary: true}, {Kind: ContactPhone, Value: "02-1111-2222", Primary: true}}}); err != nil {
		t.Fatalf("標記過的配對不該再擋：%v", err)
	}
	before := len(f.distinctPairs(t))
	wantKind(t, mark(prospectAdmin, a.ID, a.ID), apperr.KindInvalidInput)
	wantKind(t, mark(prospectAdmin, a.ID, 999999), apperr.KindNotFound)
	wantKind(t, mark(prospectAdmin, 0, a.ID), apperr.KindInvalidInput)
	wantKind(t, mark(0, a.ID, e.ID), apperr.KindInvalidInput)
	archived := f.archive(t, g.ID)
	c := f.newProspect(t, "丙", nil)
	wantKind(t, mark(prospectAdmin, c.ID, archived.ID), apperr.KindFailedPrecondition)
	if got := len(f.distinctPairs(t)); got != before {
		t.Fatalf("被拒絕的標記不能寫入：%d → %d", before, got)
	}
}

// wantTaxIDHeld 確認錯誤是 PROSPECT_TAX_ID_HELD：錯誤碼、metadata 的持有者 ID 與句子都符合。
func wantTaxIDHeld(t *testing.T, err error, kind apperr.Kind, holder int64, message string) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != kind || ae.Reason != apperr.ReasonProspectTaxIDHeld ||
		ae.Metadata["prospectIds"] != strconv.FormatInt(holder, 10) || ae.Message != message {
		t.Fatalf("err=%v (kind=%v reason=%v metadata=%v), want %v PROSPECT_TAX_ID_HELD holder %d %q", err, apperr.KindOf(err), reasonOf(err), metadataOf(err), kind, holder, message)
	}
}

func reasonOf(err error) apperr.Reason {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Reason
	}
	return apperr.Reason{}
}

func metadataOf(err error) map[string]string {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Metadata
	}
	return nil
}

// taxIDHolders 建立兩個持有者：進行中的與已入駐的，各有自己的統編。
func (f *fixture) taxIDHolders(t *testing.T) (active, converted Prospect) {
	t.Helper()
	active = f.newProspect(t, "進行中的持有者", func(in *CreateProspectInput) { in.Fields.TaxID = text("12345678") })
	holder := f.newProspect(t, "已入駐的持有者", func(in *CreateProspectInput) { in.Fields.TaxID = text("87654321") })
	_, s := f.open(t)
	converted, err := f.module.ConvertProspect(context.Background(), ConvertProspectInput{ProspectID: holder.ID, StoreID: s.StoreID, AdminID: prospectAdmin, ExpectedVersion: holder.Version})
	must(t, err)
	return active, converted
}

func TestTaxIDHeldCarriesTheReasonHolderAndASentencePerOperationAndHolderStatus(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	active, converted := f.taxIDHolders(t)
	cases := []struct {
		name   string
		holder Prospect
		create string
		update string
		revive string
	}{
		{"進行中的持有者", active,
			"統一編號已由業者 %d 使用", "統一編號已由業者 %d 使用，該做的是合併", "統一編號已由業者 %d 使用，是同一家的話不需要還原"},
		{"已入駐的持有者", converted,
			"統一編號已由業者 %d 使用", "統一編號已由已入駐的業者 %d 使用；是同一家的話請封存這筆", "統一編號已由業者 %d 使用，是同一家的話不需要還原"},
	}
	for i, c := range cases {
		taxID := *c.holder.TaxID
		id := c.holder.ID
		_, err := f.module.CreateProspect(ctx, CreateProspectInput{AdminID: prospectAdmin, Fields: ProspectFields{DisplayName: "新的 " + c.name, VendorType: VendorBusiness, TaxID: &taxID}})
		wantTaxIDHeld(t, err, apperr.KindAlreadyExists, id, fmt.Sprintf(c.create, id))

		editing := f.newProspect(t, "修改 "+c.name, nil)
		_, err = f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: editing.ID, ExpectedVersion: editing.Version, AdminID: prospectAdmin, TaxID: &taxID})
		wantTaxIDHeld(t, err, apperr.KindFailedPrecondition, id, fmt.Sprintf(c.update, id))
		if got := f.prospectRow(t, editing.ID); got.Version != editing.Version || got.TaxID != nil {
			t.Fatalf("%s: rejected update wrote: %+v", c.name, got)
		}

		reviving := f.newProspect(t, "還原 "+c.name, func(in *CreateProspectInput) { in.Fields.TaxID = text(fmt.Sprintf("0000000%d", i+1)) })
		f.archive(t, reviving.ID)
		_, err = f.db.Conn(ctx).Exec(ctx, `UPDATE seller.prospects SET tax_id = $2 WHERE prospect_id = $1`, reviving.ID, taxID)
		must(t, err)
		_, err = f.restore(t, reviving.ID, id)
		wantTaxIDHeld(t, err, apperr.KindFailedPrecondition, id, fmt.Sprintf(c.revive, id))
		if got := f.prospectRow(t, reviving.ID); got.Status != ProspectStatusArchived {
			t.Fatalf("%s: rejected restore wrote: %+v", c.name, got)
		}
	}
}

// raceOnTaxID 讓另一個 transaction 先取得 taxID 的 advisory lock、寫入持有 taxID 的業者但不 commit；op 的檢查必須等在這把鎖上
// （pg_stat_activity 的 wait_event 是 advisory），等到持有者 commit 才往下，所以看得到持有者。
func raceOnTaxID(t *testing.T, f *fixture, taxID string, op func() error) (holder int64, err error) {
	t.Helper()
	ctx := context.Background()
	inserted := make(chan int64)
	release := make(chan struct{})
	holderDone := make(chan error, 1)
	var releaseOnce sync.Once
	releaseHolder := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseHolder)
	go func() {
		holderDone <- f.db.InTx(ctx, func(ctx context.Context) error {
			if err := store.New(f.db.Conn(ctx)).LockProspectTaxID(ctx, taxID); err != nil {
				return err
			}
			var id int64
			if err := f.db.Conn(ctx).QueryRow(ctx, `INSERT INTO seller.prospects (display_name, vendor_type, tax_id, created_at, updated_at) VALUES ('並行的持有者', 'BUSINESS', $1, now(), now()) RETURNING prospect_id`, taxID).Scan(&id); err != nil {
				return err
			}
			inserted <- id
			<-release
			return nil
		})
	}()
	select {
	case holder = <-inserted:
	case err := <-holderDone:
		t.Fatalf("holder transaction: %v", err)
	}
	opDone := make(chan error, 1)
	go func() { opDone <- op() }()
	for f.count(t, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND datname = current_database()`) == 0 {
		select {
		case err := <-opDone:
			t.Fatalf("op finished before it blocked on the holder: %v", err)
		case <-time.After(5 * time.Millisecond):
		}
	}
	if n := f.count(t, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND wait_event = 'advisory' AND datname = current_database()`); n != 1 {
		t.Fatalf("op is waiting on something other than the tax ID advisory lock (advisory waiters: %d)", n)
	}
	releaseHolder()
	must(t, <-holderDone)
	return holder, <-opDone
}

func TestConcurrentTaxIDWritesQueueOnTheTaxIDLockAndTheSecondSeesTheHolder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t.Run("建立", func(t *testing.T) {
		t.Parallel()
		f := setup(t)
		holder, err := raceOnTaxID(t, f, "11112222", func() error {
			_, err := f.module.CreateProspect(ctx, CreateProspectInput{AdminID: prospectAdmin, Fields: ProspectFields{DisplayName: "建立", VendorType: VendorBusiness, TaxID: text("11112222")}})
			return err
		})
		wantTaxIDHeld(t, err, apperr.KindAlreadyExists, holder, fmt.Sprintf("統一編號已由業者 %d 使用", holder))
		if n := f.count(t, `SELECT count(*) FROM seller.prospects WHERE display_name = '建立'`); n != 0 {
			t.Fatalf("rejected create left %d rows", n)
		}
	})
	t.Run("修改", func(t *testing.T) {
		t.Parallel()
		f := setup(t)
		p := f.newProspect(t, "修改", nil)
		holder, err := raceOnTaxID(t, f, "11112222", func() error {
			_, err := f.module.UpdateProspect(ctx, UpdateProspectInput{ProspectID: p.ID, ExpectedVersion: p.Version, AdminID: prospectAdmin, TaxID: ptr("11112222")})
			return err
		})
		wantTaxIDHeld(t, err, apperr.KindFailedPrecondition, holder, fmt.Sprintf("統一編號已由業者 %d 使用，該做的是合併", holder))
		if got := f.prospectRow(t, p.ID); got.Version != p.Version || got.TaxID != nil {
			t.Fatalf("rejected update wrote: %+v", got)
		}
	})
	t.Run("還原", func(t *testing.T) {
		t.Parallel()
		f := setup(t)
		p := f.newProspect(t, "還原", func(in *CreateProspectInput) { in.Fields.TaxID = text("11112222") })
		archived := f.archive(t, p.ID)
		holder, err := raceOnTaxID(t, f, "11112222", func() error {
			_, err := f.restore(t, p.ID)
			return err
		})
		wantTaxIDHeld(t, err, apperr.KindFailedPrecondition, holder, fmt.Sprintf("統一編號已由業者 %d 使用，是同一家的話不需要還原", holder))
		if got := f.prospectRow(t, p.ID); got.Status != ProspectStatusArchived || got.Version != archived.Version {
			t.Fatalf("rejected restore wrote: %+v", got)
		}
	})
}
