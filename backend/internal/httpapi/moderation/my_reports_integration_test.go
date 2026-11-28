//go:build integration

package moderation_test

import (
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/moderation/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// 真的社群與店家兩個模組、真的資料庫：兩邊的建立時間交錯，分頁讀完不重複、不遺漏、時間序，而且只看得到呼叫者自己的檢舉。
func TestListMyReportsMergesTheRealModulesInterleavedOwnerOnly(t *testing.T) {
	s := startAdminStack(t)
	ctx := t.Context()
	w := s.world(t)
	storeID := w.storeID
	profile, err := s.buyers.Profile(ctx, s.customerAccount)
	if err != nil {
		t.Fatal(err)
	}
	me, other := profile.BuyerID, s.newBuyer(t)

	base := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Hour)
	categories := []policy.Category{policy.Fraud, policy.FoodSafety, policy.MisleadingInfo, policy.Other}
	var want []string
	for i := range 4 {
		*s.now = base.Add(-time.Duration(2*i+1) * time.Minute)
		c, err := s.moderation.SubmitReport(ctx, moderation.SubmitInput{ReporterBuyerID: me, Target: moderation.Target{Kind: moderation.TargetBuyer, BuyerID: s.newBuyer(t)}, Category: policy.Spam})
		if err != nil {
			t.Fatal(err)
		}
		*s.now = base.Add(-time.Duration(2*i+2) * time.Minute)
		st, err := s.sellerMod.SubmitReport(ctx, seller.SubmitReportInput{StoreID: storeID, ReporterBuyerID: me, Category: categories[i], Description: "店家檢舉"})
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, c.Report.ID.String(), st.ReportID.String())
	}
	if _, err := s.moderation.SubmitReport(ctx, moderation.SubmitInput{ReporterBuyerID: other, Target: moderation.Target{Kind: moderation.TargetBuyer, BuyerID: me}, Category: policy.Spam}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.sellerMod.SubmitReport(ctx, seller.SubmitReportInput{StoreID: storeID, ReporterBuyerID: other, Category: policy.Fraud, Description: "別人的"}); err != nil {
		t.Fatal(err)
	}

	for _, size := range []int32{1, 3, 5, 8, 20} {
		var got []string
		token := ""
		for range 20 {
			resp, err := s.app.ListMyReports(ctx, connect.NewRequest(&pb.ListMyReportsRequest{PageSize: size, PageToken: token}))
			if err != nil {
				t.Fatal(err)
			}
			for _, it := range resp.Msg.GetReports() {
				got = append(got, it.GetId())
			}
			if token = resp.Msg.GetNextPageToken(); token == "" {
				break
			}
		}
		if !slices.Equal(got, want) {
			t.Fatalf("size=%d：%v，預期 %v（交錯、不重複、不遺漏、只有自己的）", size, got, want)
		}
	}
}
