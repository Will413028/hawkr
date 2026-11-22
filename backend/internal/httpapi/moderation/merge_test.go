package moderation

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/moderation/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/moderation/v1alpha1/moderationv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	sellermod "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// pagedCommunity 與 pagedStores 以真的 keyset 語意（建立時間新到舊、id 新到舊、取 size 筆並以 Next 標示還有）回傳固定的資料，
// 並記錄每次呼叫的游標，讓測試能看「取完的來源是否不再被讀」。
type pagedCommunity struct {
	rows   []moderation.ReportItem
	calls  []*moderation.ReportCursor
	buyers []int64
}

func (p *pagedCommunity) SubmitReport(context.Context, moderation.SubmitInput) (moderation.SubmitResult, error) {
	return moderation.SubmitResult{}, nil
}

func (p *pagedCommunity) MyStoreReports(context.Context, int64, *moderation.ReportCursor, int32) (moderation.ReportPage, error) {
	return moderation.ReportPage{}, nil
}

func (p *pagedCommunity) MyReports(_ context.Context, buyerID int64, after *moderation.ReportCursor, size int32) (moderation.ReportPage, error) {
	p.calls = append(p.calls, after)
	p.buyers = append(p.buyers, buyerID)
	var rest []moderation.ReportItem
	for _, r := range p.rows {
		if after == nil || r.Report.CreatedAt.Before(after.CreatedAt) || (r.Report.CreatedAt.Equal(after.CreatedAt) && string(r.Report.ID[:]) < string(after.ID[:])) {
			rest = append(rest, r)
		}
	}
	page := moderation.ReportPage{}
	if len(rest) > int(size) {
		page.Next = &moderation.ReportCursor{}
		rest = rest[:size]
	}
	page.Items = rest
	if page.Next != nil {
		last := rest[len(rest)-1].Report
		page.Next = &moderation.ReportCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return page, nil
}

type pagedStores struct {
	rows   []sellermod.Report
	calls  []*sellermod.ReportCursor
	buyers []int64
	// current 是呼叫者此刻的店；nil 表示沒有店（NotFound）。
	current *sellermod.Store
}

func (p *pagedStores) CurrentStore(context.Context, int64) (sellermod.Store, error) {
	if p.current == nil {
		return sellermod.Store{}, apperr.NotFound("沒有店")
	}
	return *p.current, nil
}

func (p *pagedStores) BuyerReports(_ context.Context, in sellermod.BuyerReportListInput) (sellermod.ReportPage, error) {
	p.calls = append(p.calls, in.After)
	p.buyers = append(p.buyers, in.BuyerID)
	var rest []sellermod.Report
	for _, r := range p.rows {
		if in.After == nil || r.CreatedAt.Before(in.After.CreatedAt) || (r.CreatedAt.Equal(in.After.CreatedAt) && string(r.ID[:]) < string(in.After.ID[:])) {
			rest = append(rest, r)
		}
	}
	page := sellermod.ReportPage{}
	if len(rest) > int(in.PageSize) {
		rest = rest[:in.PageSize]
		last := rest[len(rest)-1]
		page.Next = &sellermod.ReportCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	page.Items = rest
	return page, nil
}

type fakeStores struct{ pagedStores }

// shortStores 每頁比要求的少回一筆（至少一筆）、但仍以 Next 標示還有：合併不能假設來源剛好回 size 筆。
type shortStores struct{ pagedStores }

func (p *shortStores) BuyerReports(ctx context.Context, in sellermod.BuyerReportListInput) (sellermod.ReportPage, error) {
	in.PageSize = max(1, in.PageSize-1)
	return p.pagedStores.BuyerReports(ctx, in)
}

func idOf(n byte) uuid.UUID { return uuid.UUID{15: n} }

var base = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

// community 與 store 是以「第幾分鐘前」與序號建立的資料；序號大的 id 大（同時間 id 新到舊 = 序號大的在前）。
func community(minutesAgo int, n byte) moderation.ReportItem {
	return moderation.ReportItem{
		Report: moderation.Report{ID: idOf(n), Category: policy.Spam, CreatedAt: base.Add(-time.Duration(minutesAgo) * time.Minute), Outcome: moderation.OutcomePending},
		Target: moderation.Target{Kind: moderation.TargetPost, ContentID: idOf(n)},
	}
}

func storeReport(minutesAgo int, n byte) sellermod.Report {
	return sellermod.Report{ID: idOf(n), StoreID: 1, Category: policy.Fraud, Description: "s", CreatedAt: base.Add(-time.Duration(minutesAgo) * time.Minute)}
}

func sortedCommunity(rows ...moderation.ReportItem) []moderation.ReportItem {
	slices.SortFunc(rows, func(a, b moderation.ReportItem) int {
		if c := b.Report.CreatedAt.Compare(a.Report.CreatedAt); c != 0 {
			return c
		}
		return slices.Compare(b.Report.ID[:], a.Report.ID[:])
	})
	return rows
}

func sortedStores(rows ...sellermod.Report) []sellermod.Report {
	slices.SortFunc(rows, func(a, b sellermod.Report) int {
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c
		}
		return slices.Compare(b.ID[:], a.ID[:])
	})
	return rows
}

// pageAll 以 size 讀完所有頁，回傳依序的標籤（c<n> 或 s<n>）與每頁的筆數。
func pageAll(t *testing.T, e *env, size int32) (labels []string, sizes []int) {
	t.Helper()
	token := ""
	for range 50 {
		resp, err := e.client.ListMyReports(context.Background(), signedIn(&pb.ListMyReportsRequest{PageSize: size, PageToken: token}))
		if err != nil {
			t.Fatal(err)
		}
		sizes = append(sizes, len(resp.Msg.GetReports()))
		for _, it := range resp.Msg.GetReports() {
			prefix := "c"
			if it.GetStore() != nil {
				prefix = "s"
			}
			n, err := strconv.ParseUint(it.GetId()[len(it.GetId())-2:], 16, 8)
			if err != nil {
				t.Fatal(err)
			}
			labels = append(labels, fmt.Sprintf("%s%d", prefix, n))
		}
		if token = resp.Msg.GetNextPageToken(); token == "" {
			return labels, sizes
		}
	}
	t.Fatal("分頁沒有結束")
	return nil, nil
}

func TestListMyReportsInterleavesBothSourcesByTimeAcrossPages(t *testing.T) {
	// 時間（分鐘前）：c1=1, s2=2, c3=3, s4=4, c5=5, s6=6, c7=7, s8=8
	c := &pagedCommunity{rows: sortedCommunity(community(1, 1), community(3, 3), community(5, 5), community(7, 7))}
	s := &pagedStores{rows: sortedStores(storeReport(2, 2), storeReport(4, 4), storeReport(6, 6), storeReport(8, 8))}
	e := newEnvWith(t, c, s)
	want := []string{"c1", "s2", "c3", "s4", "c5", "s6", "c7", "s8"}
	for _, size := range []int32{1, 2, 3, 5, 8, 20} {
		got, _ := pageAll(t, e, size)
		if !slices.Equal(got, want) {
			t.Fatalf("size=%d：%v，預期 %v（不重複、不遺漏、時間序）", size, got, want)
		}
	}
}

func TestListMyReportsOrdersTiesCommunityFirstThenIDDescending(t *testing.T) {
	c := &pagedCommunity{rows: sortedCommunity(community(5, 1), community(5, 3))}
	s := &pagedStores{rows: sortedStores(storeReport(5, 2), storeReport(5, 4))}
	e := newEnvWith(t, c, s)
	want := []string{"c3", "c1", "s4", "s2"}
	for _, size := range []int32{1, 2, 3, 4} {
		if got, _ := pageAll(t, e, size); !slices.Equal(got, want) {
			t.Fatalf("size=%d：%v，預期 %v", size, got, want)
		}
	}
}

func TestListMyReportsOneSourceExhaustedStopsBeingRead(t *testing.T) {
	c := &pagedCommunity{rows: sortedCommunity(community(1, 1))}
	s := &pagedStores{rows: sortedStores(storeReport(2, 2), storeReport(3, 3), storeReport(4, 4), storeReport(5, 5), storeReport(6, 6))}
	e := newEnvWith(t, c, s)
	got, _ := pageAll(t, e, 2)
	if want := []string{"c1", "s2", "s3", "s4", "s5", "s6"}; !slices.Equal(got, want) {
		t.Fatalf("%v，預期 %v", got, want)
	}
	if len(c.calls) != 1 {
		t.Fatalf("社群來源在第一頁就取完，之後不該再讀：%d 次", len(c.calls))
	}
}

// 一方的尾端剛好跨過頁界：本頁取到但沒輸出的資料，下一頁要從原游標之後（不是取到的最後一筆）讀。
func TestListMyReportsTailCrossingThePageBoundaryKeepsTheUnemittedSideCursor(t *testing.T) {
	c := &pagedCommunity{rows: sortedCommunity(community(1, 1), community(2, 2), community(3, 3))}
	s := &pagedStores{rows: sortedStores(storeReport(10, 4))}
	e := newEnvWith(t, c, s)
	got, sizes := pageAll(t, e, 2)
	if want := []string{"c1", "c2", "c3", "s4"}; !slices.Equal(got, want) {
		t.Fatalf("%v，預期 %v，頁 %v", got, want, sizes)
	}
	// 第一頁：社群取到 2 筆（Next 非 nil，都輸出），店家取到 1 筆（沒輸出到，Next 為 nil 但不算取完）。第二頁店家必須仍從頭讀。
	if len(s.calls) < 2 || s.calls[1] != nil {
		t.Fatalf("沒輸出到的店家來源要保留原游標（nil）：%v", s.calls)
	}
}

func TestListMyReportsExhaustedWhenBothSourcesAreEmptyOrFit(t *testing.T) {
	e := newEnvWith(t, &pagedCommunity{}, &pagedStores{})
	resp, err := e.client.ListMyReports(context.Background(), signedIn(&pb.ListMyReportsRequest{}))
	if err != nil || len(resp.Msg.GetReports()) != 0 || resp.Msg.GetNextPageToken() != "" {
		t.Fatalf("空：%v %v", resp.Msg, err)
	}
	c := &pagedCommunity{rows: sortedCommunity(community(1, 1))}
	s := &pagedStores{rows: sortedStores(storeReport(2, 2))}
	e = newEnvWith(t, c, s)
	resp, err = e.client.ListMyReports(context.Background(), signedIn(&pb.ListMyReportsRequest{PageSize: 2}))
	if err != nil || len(resp.Msg.GetReports()) != 2 || resp.Msg.GetNextPageToken() != "" {
		t.Fatalf("剛好一頁：%v %v", resp.Msg, err)
	}
}

func TestListMyReportsRejectsForgedCursors(t *testing.T) {
	e := newEnvWith(t, &pagedCommunity{}, &pagedStores{})
	scope := rpc.PageScope(moderationv1alpha1connect.ModerationAppServiceListMyReportsProcedure, 7)
	for name, cur := range map[string]myReportsCursor{
		"零值的社群游標":   {Community: &moderation.ReportCursor{}},
		"零值的店家游標":   {Store: &sellermod.ReportCursor{}},
		"取完的來源還帶游標": {CommunityDone: true, Community: &moderation.ReportCursor{CreatedAt: base, ID: idOf(1)}},
	} {
		_, err := e.client.ListMyReports(context.Background(), signedIn(&pb.ListMyReportsRequest{PageToken: rpc.TokenFor(scope, cur)}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("%s：%v", name, err)
		}
	}
	other := rpc.PageScope(moderationv1alpha1connect.ModerationAppServiceListMyReportsProcedure, 8)
	_, err := e.client.ListMyReports(context.Background(), signedIn(&pb.ListMyReportsRequest{PageToken: rpc.TokenFor(other, myReportsCursor{CommunityDone: true})}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("別的買家的 token：%v", err)
	}
}

func TestListMyReportsStoreItemCarriesTheStoreReportAndSharedFields(t *testing.T) {
	outcome := sellermod.OutcomeNoViolation
	resolved := base.Add(time.Hour)
	note := "查過了"
	r := storeReport(1, 9)
	r.Outcome, r.ResolvedAt, r.ResolutionNote = &outcome, &resolved, &note
	e := newEnvWith(t, &pagedCommunity{}, &pagedStores{rows: []sellermod.Report{r}})
	resp, err := e.client.ListMyReports(context.Background(), signedIn(&pb.ListMyReportsRequest{}))
	if err != nil || len(resp.Msg.GetReports()) != 1 {
		t.Fatalf("%v %v", resp.Msg, err)
	}
	it := resp.Msg.GetReports()[0]
	if it.GetCommunity() != nil || it.GetStore().GetId() != it.GetId() || it.GetStore().GetResolutionNote() != "查過了" || it.GetDescription() != "s" ||
		!it.GetResolveTime().AsTime().Equal(resolved) || !it.GetCreateTime().AsTime().Equal(r.CreatedAt) {
		t.Fatalf("%v", it)
	}
}

func TestListMyReportsUnresolvedStoreItemHasNoOutcome(t *testing.T) {
	e := newEnvWith(t, &pagedCommunity{}, &pagedStores{rows: []sellermod.Report{storeReport(1, 9)}})
	resp, err := e.client.ListMyReports(context.Background(), signedIn(&pb.ListMyReportsRequest{}))
	if err != nil || len(resp.Msg.GetReports()) != 1 {
		t.Fatalf("%v %v", resp.Msg, err)
	}
	it := resp.Msg.GetReports()[0]
	if it.GetStore().Outcome != nil || it.ResolveTime != nil {
		t.Fatalf("outcome=%v 解決時間=%v", it.GetStore().Outcome, it.GetResolveTime())
	}
}

func TestListMyReportsSourceReturningFewerThanSizeWithNextLosesNothing(t *testing.T) {
	c := &pagedCommunity{rows: sortedCommunity(community(1, 1), community(3, 3), community(5, 5), community(7, 7))}
	s := &shortStores{pagedStores{rows: sortedStores(storeReport(2, 2), storeReport(4, 4), storeReport(6, 6), storeReport(8, 8))}}
	e := newEnvWith(t, c, s)
	want := []string{"c1", "s2", "c3", "s4", "c5", "s6", "c7", "s8"}
	for _, size := range []int32{2, 3, 5} {
		if got, _ := pageAll(t, e, size); !slices.Equal(got, want) {
			t.Fatalf("size=%d：%v，預期 %v", size, got, want)
		}
	}
}

func TestListMyReportsPassesTheCallerBuyerToBothSources(t *testing.T) {
	c := &pagedCommunity{rows: sortedCommunity(community(1, 1))}
	s := &pagedStores{rows: sortedStores(storeReport(2, 2))}
	e := newEnvWith(t, c, s)
	pageAll(t, e, 5)
	if len(c.buyers) == 0 || len(s.buyers) == 0 {
		t.Fatalf("兩個來源都該被讀：%v %v", c.buyers, s.buyers)
	}
	for _, b := range append(slices.Clone(c.buyers), s.buyers...) {
		if b != 7 {
			t.Fatalf("來源收到的買家 %d，預期呼叫者 7：%v %v", b, c.buyers, s.buyers)
		}
	}
}

func TestMergedPageSizeLimitFitsBothSources(t *testing.T) {
	if maxPageSize > moderation.MyReportsMaxPageSize || maxPageSize > sellermod.ReportMaxPageSize {
		t.Fatalf("合併的上限 %d 超過來源的上限（%d、%d）", maxPageSize, moderation.MyReportsMaxPageSize, sellermod.ReportMaxPageSize)
	}
}

func TestOutcomeAndTargetEnumsMatchTheModules(t *testing.T) {
	if m := rpc.EnumMismatches[pb.ReportTargetKind]([]moderation.TargetKind{moderation.TargetPost, moderation.TargetComment, moderation.TargetForumEntry, moderation.TargetBuyer, moderation.TargetReview, moderation.TargetChatMessage, moderation.TargetReviewReply}); len(m) != 0 {
		t.Fatalf("ReportTargetKind：%v", m)
	}
	if m := rpc.EnumMismatches[pb.ReportOutcome]([]moderation.Outcome{moderation.OutcomePending, moderation.OutcomeViolation, moderation.OutcomeNoViolation}); len(m) != 0 {
		t.Fatalf("ReportOutcome：%v", m)
	}
}
