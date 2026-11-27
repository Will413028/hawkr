//go:build gateway || integration

package characterization

import (
	"context"
	"sync"
	"time"

	ch "github.com/Will413028/hawkr/backend/internal/httpapi/characterize"
)

var scenarios = []ch.Scenario[Driver]{
	{Name: "anonymous callers are rejected", Run: func(ctx context.Context, t ch.T, d Driver) {
		_, err := d.ListMyCoupons(ctx, ch.Anonymous, ListQuery{})
		ch.WantKind(t, err, ch.ErrUnauthenticated)
		_, err = d.Claim(ctx, ch.Anonymous, d.Unknown())
		ch.WantKind(t, err, ch.ErrUnauthenticated)
		_, err = d.SellerList(ctx, ch.Anonymous, "", Page{})
		ch.WantKind(t, err, ch.ErrUnauthenticated)
		if d.AdminRPC() {
			err = d.AdminPublish(ctx, ch.Anonymous, d.Unknown())
			ch.WantKind(t, err, ch.ErrUnauthenticated)
		}
	}},
	{Name: "my coupons list decodes used records", Run: func(ctx context.Context, t ch.T, d Driver) {
		for _, status := range []string{"", "USED"} {
			got := ch.Must(t, ch.Ret(d.ListMyCoupons(ctx, ch.Buyer, ListQuery{Status: status})))
			if got.Used == nil {
				t.Errorf("status %q：Used 應是空清單而不是 nil", status)
			}
			for _, u := range got.Used {
				if u.ID == "" || u.TemplateID == "" {
					t.Errorf("status %q：已使用的券缺 ID 或模板 ID：%+v", status, u)
				}
			}
		}
	}},
	{Name: "buyers cannot use seller routes", Run: func(ctx context.Context, t ch.T, d Driver) {
		want := ch.ErrPermission
		if !d.Legacy() {
			want = ch.ErrNotFound
		}
		_, err := d.SellerCreate(ctx, ch.Buyer, cashInput("coupon-"+ch.Nonce(), TargetAll))
		ch.WantKind(t, err, want)
		_, err = d.SellerList(ctx, ch.Buyer, "", Page{})
		ch.WantKind(t, err, want)
	}},
	{Name: "non admins cannot use admin routes", Run: func(ctx context.Context, t ch.T, d Driver) {
		if skipWithoutAdminRPC(t, d) {
			return
		}
		_, err := d.AdminCreate(ctx, ch.Buyer, cashInput("coupon-"+ch.Nonce(), TargetAll))
		ch.WantKind(t, err, ch.ErrPermission)
		err = d.AdminSetQuota(ctx, ch.Seller, d.StoreOf(ch.Seller), 10)
		ch.WantKind(t, err, ch.ErrPermission)
	}},
	{Name: "seller creates an active cash template visible in list and detail", Run: func(ctx context.Context, t ch.T, d Driver) {
		sellerReady(ctx, t, d)
		in := cashInput("coupon-"+ch.Nonce(), TargetAll)
		created := ch.Must(t, ch.Ret(d.SellerCreate(ctx, ch.Seller, in)))
		if created.ID == "" {
			t.Fatalf("建立的模板沒有 ID")
		}
		for name, got := range map[string]Template{"建立回應": created, "列表": mustFind(ctx, t, d, created.ID), "詳情": mustDetail(ctx, t, d, ch.Seller, created.ID).Template} {
			if got.Status != StatusActive || got.Type != TypeCash || got.Target != TargetAll || got.Issuer != IssuerSeller {
				t.Errorf("%s：status/type/target/issuer = %s/%s/%s/%s，預期 ACTIVE/CASH/ALL/SELLER", name, got.Status, got.Type, got.Target, got.Issuer)
			}
			if got.StoreID != d.StoreOf(ch.Seller) || got.IssuerStoreID != d.StoreOf(ch.Seller) {
				t.Errorf("%s：適用店家 %q、發行店家 %q，預期都是 %s", name, got.StoreID, got.IssuerStoreID, d.StoreOf(ch.Seller))
			}
			if got.Threshold != 200 || got.DiscountAmount != 50 || (d.Legacy() && got.ClaimRequired) {
				t.Errorf("%s：門檻 %d、折抵 %d、需領取 %v，預期 200、50、false", name, got.Threshold, got.DiscountAmount, got.ClaimRequired)
			}
		}
		stored := mustFind(ctx, t, d, created.ID)
		if stored.TotalIssueLimit != -1 || stored.IssuedCount != 0 {
			t.Errorf("列表的發行上限 %d、已發行 %d，預期 -1、0", stored.TotalIssueLimit, stored.IssuedCount)
		}
		detail := mustDetail(ctx, t, d, ch.Seller, created.ID)
		if detail.Used != 0 || detail.Reserved != 0 {
			t.Errorf("新模板的 used/reserved = %d/%d，預期 0/0", detail.Used, detail.Reserved)
		}
		other := sellerTemplates(ctx, t, d, ch.Seller2, "")
		if _, ok := find(other, created.ID); ok {
			t.Errorf("別家賣家的列表看得到這張模板")
		}
	}},
	{Name: "seller creates a percent template", Run: func(ctx context.Context, t ch.T, d Driver) {
		sellerReady(ctx, t, d)
		start, end := window()
		created := ch.Must(t, ch.Ret(d.SellerCreate(ctx, ch.Seller, TemplateInput{
			Title: "coupon-" + ch.Nonce(), Type: TypePercent, Target: TargetAll,
			DiscountPercent: 10, MaxDiscount: 30, StartAt: start, EndAt: end,
		})))
		got := mustFind(ctx, t, d, created.ID)
		if got.Type != TypePercent || got.DiscountPercent != 10 || got.MaxDiscount != 30 || got.Threshold != 0 {
			t.Errorf("percent/max/threshold = %d/%d/%d，type %s，預期 10/30/0、PERCENT", got.DiscountPercent, got.MaxDiscount, got.Threshold, got.Type)
		}
	}},
	{Name: "template validation rejects bad input", Run: func(ctx context.Context, t ch.T, d Driver) {
		sellerReady(ctx, t, d)
		start, end := window()
		base := func() TemplateInput {
			return TemplateInput{Title: "coupon-" + ch.Nonce(), Type: TypeCash, Target: TargetAll, DiscountAmount: 50, StartAt: start, EndAt: end}
		}
		cases := map[string]func(*TemplateInput){
			"現金券沒有折抵金額": func(in *TemplateInput) { in.DiscountAmount = 0 },
			"百分比為 0":    func(in *TemplateInput) { in.Type, in.DiscountAmount, in.MaxDiscount = TypePercent, 0, 30 },
			"百分比超過 99": func(in *TemplateInput) {
				in.Type, in.DiscountAmount, in.DiscountPercent, in.MaxDiscount = TypePercent, 0, 100, 30
			},
			"百分比沒有折抵上限": func(in *TemplateInput) { in.Type, in.DiscountAmount, in.DiscountPercent = TypePercent, 0, 10 },
			"開始晚於結束":    func(in *TemplateInput) { in.StartAt, in.EndAt = end, start },
			"門檻是負數":     func(in *TemplateInput) { in.Threshold = -5 },
			"不認得的類型":    func(in *TemplateInput) { in.Type = "9" },
			"賣家不能發補償券":  func(in *TemplateInput) { in.Target = TargetCompensation },
			"不認得的發放目標":  func(in *TemplateInput) { in.Target = "9" },
			"指定買家沒有名單":  func(in *TemplateInput) { in.Target = TargetDesignated },
		}
		for name, mutate := range cases {
			in := base()
			mutate(&in)
			_, err := d.SellerCreate(ctx, ch.Seller, in)
			if err == nil {
				t.Errorf("%s：預期被拒絕，卻建立成功", name)
				continue
			}
			ch.WantKind(t, err, ch.ErrInvalid)
		}
	}},
	{Name: "seller member template is accepted", Run: func(ctx context.Context, t ch.T, d Driver) {
		sellerReady(ctx, t, d)
		created := ch.Must(t, ch.Ret(d.SellerCreate(ctx, ch.Seller, cashInput("coupon-"+ch.Nonce(), TargetMember))))
		if got := mustFind(ctx, t, d, created.ID); got.Target != TargetMember || got.Status != StatusActive {
			t.Errorf("target/status = %s/%s，預期 MEMBER/ACTIVE", got.Target, got.Status)
		}
	}},
	{Name: "seller quota limits active templates and revoking frees a slot", Run: func(ctx context.Context, t ch.T, d Driver) {
		if err := d.AdminSetQuota(ctx, ch.Admin, d.StoreOf(ch.Seller2), 2); err != nil {
			t.Fatalf("設定配額：%v", err)
		}
		first := ch.Must(t, ch.Ret(d.SellerCreate(ctx, ch.Seller2, cashInput("coupon-"+ch.Nonce(), TargetAll))))
		ch.Must(t, ch.Ret(d.SellerCreate(ctx, ch.Seller2, cashInput("coupon-"+ch.Nonce(), TargetAll))))
		_, err := d.SellerCreate(ctx, ch.Seller2, cashInput("coupon-"+ch.Nonce(), TargetAll))
		ch.WantKind(t, err, ErrQuotaExceeded)
		if err := d.SellerRevoke(ctx, ch.Seller2, first.ID, "quota"); err != nil {
			t.Fatalf("收回：%v", err)
		}
		ch.Must(t, ch.Ret(d.SellerCreate(ctx, ch.Seller2, cashInput("coupon-"+ch.Nonce(), TargetAll))))
	}},
	{Name: "seller revoke ends a template", Run: func(ctx context.Context, t ch.T, d Driver) {
		sellerReady(ctx, t, d)
		created := ch.Must(t, ch.Ret(d.SellerCreate(ctx, ch.Seller, cashInput("coupon-"+ch.Nonce(), TargetAll))))
		if _, ok := find(myCoupons(ctx, t, d, ch.Buyer), created.ID); !ok {
			t.Fatalf("收回前買家的列表看不到模板")
		}
		if err := d.SellerRevoke(ctx, ch.Seller, created.ID, "test"); err != nil {
			t.Fatalf("收回：%v", err)
		}
		if got := mustDetail(ctx, t, d, ch.Seller, created.ID).Template; got.Status != StatusEnded {
			t.Errorf("收回後 status = %s，預期 ENDED", got.Status)
		}
		revoked := sellerTemplates(ctx, t, d, ch.Seller, StatusEnded)
		if _, ok := find(revoked, created.ID); !ok {
			t.Errorf("以 REVOKED 篩選的列表看不到模板")
		}
		active := sellerTemplates(ctx, t, d, ch.Seller, StatusActive)
		if _, ok := find(active, created.ID); ok {
			t.Errorf("以 ACTIVE 篩選的列表還看得到已收回的模板")
		}
		if _, ok := find(myCoupons(ctx, t, d, ch.Buyer), created.ID); ok {
			t.Errorf("收回後買家的列表還看得到模板")
		}
		err := d.SellerRevoke(ctx, ch.Seller, created.ID, "again")
		if d.Legacy() {
			ch.WantKind(t, err, ErrWrongStatus)
		} else if err != nil {
			t.Errorf("D5：結束已經結束的活動是冪等成功：%v", err)
		}
	}},
	{Name: "seller revoke of someone else's or unknown template reports wrong status or not found", Run: func(ctx context.Context, t ch.T, d Driver) {
		sellerReady(ctx, t, d)
		created := ch.Must(t, ch.Ret(d.SellerCreate(ctx, ch.Seller, cashInput("coupon-"+ch.Nonce(), TargetAll))))
		want := ErrWrongStatus
		if !d.Legacy() {
			want = ch.ErrNotFound
		}
		ch.WantKind(t, d.SellerRevoke(ctx, ch.Seller2, created.ID, "x"), want)
		ch.WantKind(t, d.SellerRevoke(ctx, ch.Seller, d.Unknown(), "x"), want)
		if got := mustDetail(ctx, t, d, ch.Seller, created.ID).Template; got.Status != StatusActive {
			t.Errorf("別家賣家收回後 status = %s，預期仍是 ACTIVE", got.Status)
		}
	}},
	{Name: "seller detail only shows own templates", Run: func(ctx context.Context, t ch.T, d Driver) {
		sellerReady(ctx, t, d)
		own := ch.Must(t, ch.Ret(d.SellerCreate(ctx, ch.Seller, cashInput("coupon-"+ch.Nonce(), TargetAll))))
		_, err := d.SellerDetail(ctx, ch.Seller2, own.ID)
		ch.WantKind(t, err, ch.ErrNotFound)
		platform := adminActive(ctx, t, d, cashInput("coupon-"+ch.Nonce(), TargetAll))
		_, err = d.SellerDetail(ctx, ch.Seller, platform.ID)
		ch.WantKind(t, err, ch.ErrNotFound)
		_, err = d.SellerDetail(ctx, ch.Seller, d.Unknown())
		ch.WantKind(t, err, ch.ErrNotFound)
		_, err = d.SellerDetail(ctx, ch.Seller, d.Malformed())
		ch.WantKind(t, err, ch.ErrInvalid)
	}},
	{Name: "seller designated template reaches only the named buyers", Run: func(ctx context.Context, t ch.T, d Driver) {
		sellerReady(ctx, t, d)
		in := cashInput("coupon-"+ch.Nonce(), TargetDesignated)
		in.BuyerAccounts = []string{d.BuyerAccount(ch.Buyer2)}
		created := ch.Must(t, ch.Ret(d.SellerCreate(ctx, ch.Seller, in)))
		detail := mustDetail(ctx, t, d, ch.Seller, created.ID)
		if len(detail.Template.TargetBuyers) != 1 || detail.Template.TargetBuyers[0] != d.BuyerID(ch.Buyer2) {
			t.Errorf("指定買家 = %v，預期 [%s]", detail.Template.TargetBuyers, d.BuyerID(ch.Buyer2))
		}
		if _, ok := find(myCoupons(ctx, t, d, ch.Buyer2), created.ID); !ok {
			t.Errorf("被指定的買家看不到模板")
		}
		if _, ok := find(myCoupons(ctx, t, d, ch.Buyer), created.ID); ok {
			t.Errorf("沒被指定的買家看得到模板")
		}
	}},
	{Name: "buyer list rejects an unknown status", Run: func(ctx context.Context, t ch.T, d Driver) {
		_, err := d.ListMyCoupons(ctx, ch.Buyer, ListQuery{Status: "bogus"})
		ch.WantKind(t, err, ch.ErrInvalid)
		ch.Must(t, ch.Ret(d.ListMyCoupons(ctx, ch.Buyer, ListQuery{Status: "UNUSED"})))
	}},
	{Name: "ending a campaign blocks new claims", Run: func(ctx context.Context, t ch.T, d Driver) {
		draft := ch.Must(t, ch.Ret(d.AdminCreate(ctx, ch.Admin, cashInput("coupon-"+ch.Nonce(), TargetAll))))
		if draft.Status != StatusDraft || draft.Issuer != IssuerAdmin || (d.Legacy() && !draft.ClaimRequired) || draft.StoreID != "" {
			t.Errorf("admin 建立的 status/issuer/需領取/店家 = %s/%s/%v/%q，預期 DRAFT/ADMIN/true（只有舊的有這個旗標）/空", draft.Status, draft.Issuer, draft.ClaimRequired, draft.StoreID)
		}
		if _, ok := find(myCoupons(ctx, t, d, ch.Buyer), draft.ID); ok {
			t.Errorf("草稿出現在買家的列表")
		}
		_, err := d.Claim(ctx, ch.Buyer, draft.ID)
		ch.WantKind(t, err, ErrNotActive)

		if err := d.AdminPublish(ctx, ch.Admin, draft.ID); err != nil {
			t.Fatalf("上架：%v", err)
		}
		if _, ok := find(myCoupons(ctx, t, d, ch.Buyer), draft.ID); !ok {
			t.Errorf("上架後買家的列表看不到模板")
		}
		ch.WantKind(t, d.AdminPublish(ctx, ch.Admin, draft.ID), ErrWrongStatus)

		if err := d.AdminDisable(ctx, ch.Admin, draft.ID); err != nil {
			t.Fatalf("停用：%v", err)
		}
		if d.Legacy() {
			ch.WantKind(t, d.AdminDisable(ctx, ch.Admin, draft.ID), ErrWrongStatus)
		} else if err := d.AdminDisable(ctx, ch.Admin, draft.ID); err != nil {
			t.Errorf("D5：結束已經結束的活動是冪等成功：%v", err)
		}
		_, err = d.Claim(ctx, ch.Buyer, draft.ID)
		ch.WantKind(t, err, ErrNotActive)
		if _, ok := find(myCoupons(ctx, t, d, ch.Buyer), draft.ID); ok {
			t.Errorf("停用後買家的列表還看得到模板")
		}
		ch.WantKind(t, d.AdminPublish(ctx, ch.Admin, d.Unknown()), ch.ErrNotFound)
		ch.WantKind(t, d.AdminPublish(ctx, ch.Admin, d.Malformed()), ch.ErrInvalid)
	}},
	{Name: "claim is idempotent and counts once", Run: func(ctx context.Context, t ch.T, d Driver) {
		tpl := adminActive(ctx, t, d, cashInput("coupon-"+ch.Nonce(), TargetAll))
		first := ch.Must(t, ch.Ret(d.Claim(ctx, ch.Buyer, tpl.ID)))
		if first.ClaimedAt == "" {
			t.Errorf("領券沒有回 claimedAt")
		}
		second := ch.Must(t, ch.Ret(d.Claim(ctx, ch.Buyer, tpl.ID)))
		if second.ClaimedAt != first.ClaimedAt {
			t.Errorf("重複領券的 claimedAt = %q，預期同一個 %q", second.ClaimedAt, first.ClaimedAt)
		}
		got := mustFindIn(t, myCoupons(ctx, t, d, ch.Buyer), tpl.ID)
		if !got.Claimed || got.IssuedCount != 1 {
			t.Errorf("claimed/issuedCount = %v/%d，預期 true/1", got.Claimed, got.IssuedCount)
		}
		if other := mustFindIn(t, myCoupons(ctx, t, d, ch.Buyer2), tpl.ID); other.Claimed {
			t.Errorf("別的買家的列表顯示已領取")
		}
	}},
	{Name: "claim creates a voucher", Run: func(ctx context.Context, t ch.T, d Driver) {
		if skipOnLegacy(t, d, "D2：舊的領券只寫 coupon_claims，不建 voucher，也不回 voucher ID") {
			return
		}
		tpl := adminActive(ctx, t, d, cashInput("coupon-"+ch.Nonce(), TargetAll))
		first := ch.Must(t, ch.Ret(d.Claim(ctx, ch.Buyer, tpl.ID)))
		if first.VoucherID == "" {
			t.Fatalf("領券沒有回 voucher ID")
		}
		second := ch.Must(t, ch.Ret(d.Claim(ctx, ch.Buyer, tpl.ID)))
		if second.VoucherID != first.VoucherID {
			t.Errorf("重複領券的 voucher = %q，預期同一張 %q", second.VoucherID, first.VoucherID)
		}
	}},
	{Name: "claim fails for unknown, malformed and not claimable templates", Run: func(ctx context.Context, t ch.T, d Driver) {
		sellerReady(ctx, t, d)
		_, err := d.Claim(ctx, ch.Buyer, d.Unknown())
		ch.WantKind(t, err, ch.ErrNotFound)
		_, err = d.Claim(ctx, ch.Buyer, d.Malformed())
		ch.WantKind(t, err, ch.ErrInvalid)
		sellerTpl := ch.Must(t, ch.Ret(d.SellerCreate(ctx, ch.Seller, cashInput("coupon-"+ch.Nonce(), TargetAll))))
		_, err = d.Claim(ctx, ch.Buyer, sellerTpl.ID)
		if d.Legacy() {
			ch.WantKind(t, err, ErrNotClaimable)
		} else if err != nil {
			t.Errorf("D2：沒有 claim_required，買家領得到賣家的 ALL 活動：%v", err)
		}
	}},
	{Name: "claim outside the validity window fails", Run: func(ctx context.Context, t ch.T, d Driver) {
		now := time.Now()
		for name, w := range map[string][2]time.Time{
			"還沒開始": {now.Add(time.Hour), now.Add(2 * time.Hour)},
			"已經結束": {now.Add(-2 * time.Hour), now.Add(-time.Hour)},
		} {
			if !d.Legacy() && w[1].Before(now) {
				continue
			}
			in := cashInput("coupon-"+ch.Nonce(), TargetAll)
			in.StartAt, in.EndAt = w[0], w[1]
			tpl := adminActive(ctx, t, d, in)
			_, err := d.Claim(ctx, ch.Buyer, tpl.ID)
			if err == nil {
				t.Errorf("%s：預期領券失敗", name)
				continue
			}
			ch.WantKind(t, err, ErrOutOfWindow)
		}
	}},
	{Name: "claim of a designated template is limited to the named buyers", Run: func(ctx context.Context, t ch.T, d Driver) {
		in := cashInput("coupon-"+ch.Nonce(), TargetDesignated)
		in.BuyerAccounts = []string{d.BuyerAccount(ch.Buyer2)}
		tpl := adminActive(ctx, t, d, in)
		_, err := d.Claim(ctx, ch.Buyer, tpl.ID)
		if d.Legacy() {
			ch.WantKind(t, err, ch.ErrPermission)
		} else {
			ch.WantKind(t, err, ErrNotEligible)
		}
		ch.Must(t, ch.Ret(d.Claim(ctx, ch.Buyer2, tpl.ID)))
		if _, ok := find(myCoupons(ctx, t, d, ch.Buyer), tpl.ID); ok {
			t.Errorf("沒被指定的買家看得到模板")
		}
	}},
	{Name: "claim stops at the total issue limit", Run: func(ctx context.Context, t ch.T, d Driver) {
		in := cashInput("coupon-"+ch.Nonce(), TargetAll)
		in.TotalIssueLimit = 1
		tpl := adminActive(ctx, t, d, in)
		ch.Must(t, ch.Ret(d.Claim(ctx, ch.Buyer, tpl.ID)))
		_, err := d.Claim(ctx, ch.Buyer2, tpl.ID)
		ch.WantKind(t, err, ErrSoldOut)
		if got := mustFindIn(t, myCoupons(ctx, t, d, ch.Buyer), tpl.ID); got.IssuedCount != 1 || got.TotalIssueLimit != 1 {
			t.Errorf("issuedCount/limit = %d/%d，預期 1/1", got.IssuedCount, got.TotalIssueLimit)
		}
	}},
	{Name: "concurrent claims never exceed the total issue limit", Run: func(ctx context.Context, t ch.T, d Driver) {
		if skipOnLegacy(t, d, "BD4：舊的先檢查再寫 claim，遞增失敗被忽略，並發領券可以超過上限") {
			return
		}
		in := cashInput("coupon-"+ch.Nonce(), TargetAll)
		in.TotalIssueLimit = 1
		tpl := adminActive(ctx, t, d, in)
		callers := []ch.Caller{ch.Buyer, ch.Buyer2}
		errs := make([]error, len(callers))
		var wg sync.WaitGroup
		for i, c := range callers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, errs[i] = d.Claim(ctx, c, tpl.ID)
			}()
		}
		wg.Wait()
		ok := 0
		for _, err := range errs {
			if err == nil {
				ok++
				continue
			}
			ch.WantKind(t, err, ErrSoldOut)
		}
		if ok != 1 {
			t.Errorf("成功領券 %d 位，預期剛好 1 位", ok)
		}
	}},
	{Name: "admin issue to designated buyers", Run: func(ctx context.Context, t ch.T, d Driver) {
		if skipWithoutAdminRPC(t, d) {
			return
		}
		tpl := adminActive(ctx, t, d, cashInput("coupon-"+ch.Nonce(), TargetDesignated))
		res := ch.Must(t, ch.Ret(d.AdminIssue(ctx, ch.Admin, tpl.ID, []string{d.BuyerAccount(ch.Buyer2)}, false)))
		if res.Issued != 1 || res.Distributed != 0 {
			t.Errorf("issued/distributed = %d/%d，預期 1/0", res.Issued, res.Distributed)
		}
		got := mustFindIn(t, myCoupons(ctx, t, d, ch.Buyer2), tpl.ID)
		if got.Claimed == d.Legacy() {
			t.Errorf("claimed = %v，預期 %v（舊的只指定不代領；單體發的是買家持有的券）", got.Claimed, !d.Legacy())
		}
		if _, ok := find(myCoupons(ctx, t, d, ch.Buyer), tpl.ID); ok {
			t.Errorf("沒被指定的買家看得到模板")
		}
	}},
	{Name: "admin issue with immediate distribution claims for the buyers", Run: func(ctx context.Context, t ch.T, d Driver) {
		if skipWithoutAdminRPC(t, d) {
			return
		}
		if skipOnMonolith(t, d, "D6：不保留代領（Distribute）；指定發放本身就建立買家的券") {
			return
		}
		tpl := adminActive(ctx, t, d, cashInput("coupon-"+ch.Nonce(), TargetDesignated))
		res := ch.Must(t, ch.Ret(d.AdminIssue(ctx, ch.Admin, tpl.ID, []string{d.BuyerAccount(ch.Buyer2)}, true)))
		if res.Issued != 1 || res.Distributed != 1 {
			t.Errorf("issued/distributed = %d/%d，預期 1/1", res.Issued, res.Distributed)
		}
		got := mustFindIn(t, myCoupons(ctx, t, d, ch.Buyer2), tpl.ID)
		if !got.Claimed || got.IssuedCount != 1 {
			t.Errorf("claimed/issuedCount = %v/%d，預期 true/1", got.Claimed, got.IssuedCount)
		}
	}},
	{Name: "admin issue counts only newly designated buyers", Run: func(ctx context.Context, t ch.T, d Driver) {
		if skipWithoutAdminRPC(t, d) {
			return
		}
		if skipOnLegacy(t, d, "BD21：舊的 issuedCount 是請求的買家數，不是實際新增的名單數") {
			return
		}
		tpl := adminActive(ctx, t, d, cashInput("coupon-"+ch.Nonce(), TargetDesignated))
		first := ch.Must(t, ch.Ret(d.AdminIssue(ctx, ch.Admin, tpl.ID, []string{d.BuyerAccount(ch.Buyer2)}, false)))
		again := ch.Must(t, ch.Ret(d.AdminIssue(ctx, ch.Admin, tpl.ID, []string{d.BuyerAccount(ch.Buyer2)}, false)))
		if first.Issued != 1 || again.Issued != 0 {
			t.Errorf("第一次 %d、重複 %d，預期 1、0", first.Issued, again.Issued)
		}
	}},
	{Name: "admin issue needs an active template", Run: func(ctx context.Context, t ch.T, d Driver) {
		if skipWithoutAdminRPC(t, d) {
			return
		}
		draft := ch.Must(t, ch.Ret(d.AdminCreate(ctx, ch.Admin, cashInput("coupon-"+ch.Nonce(), TargetDesignated))))
		_, err := d.AdminIssue(ctx, ch.Admin, draft.ID, []string{d.BuyerAccount(ch.Buyer2)}, false)
		want := ErrWrongStatus
		if !d.Legacy() {
			want = ErrNotActive
		}
		ch.WantKind(t, err, want)
	}},
	{Name: "admin void rejects unknown vouchers and a missing reason", Run: func(ctx context.Context, t ch.T, d Driver) {
		if skipWithoutAdminRPC(t, d) {
			return
		}
		want := ErrWrongStatus
		if !d.Legacy() {
			want = ch.ErrNotFound
		}
		ch.WantKind(t, d.AdminVoid(ctx, ch.Admin, d.Unknown(), "test"), want)
		ch.WantKind(t, d.AdminVoid(ctx, ch.Admin, d.Unknown(), ""), ch.ErrInvalid)
	}},
	{Name: "available coupons apply the threshold and estimate the discount", Run: func(ctx context.Context, t ch.T, d Driver) {
		sellerReady(ctx, t, d)
		start, end := window()
		cash := ch.Must(t, ch.Ret(d.SellerCreate(ctx, ch.Seller, cashInput("coupon-"+ch.Nonce(), TargetAll))))
		percent := ch.Must(t, ch.Ret(d.SellerCreate(ctx, ch.Seller, TemplateInput{
			Title: "coupon-" + ch.Nonce(), Type: TypePercent, Target: TargetAll,
			DiscountPercent: 10, MaxDiscount: 30, StartAt: start, EndAt: end,
		})))
		claimIfWallet(ctx, t, d, ch.Buyer, cash.ID, percent.ID)
		for _, tc := range []struct {
			amount       int64
			wantCash     bool
			cashDiscount int64
			percent      int64
		}{
			{amount: 100, wantCash: false, percent: 10},
			{amount: 300, wantCash: true, cashDiscount: 50, percent: 30},
			{amount: 1000, wantCash: true, cashDiscount: 50, percent: 30},
		} {
			list := ch.Must(t, ch.Ret(d.ListAvailable(ctx, ch.Buyer, AvailableQuery{StoreID: d.StoreOf(ch.Seller), OrderAmount: tc.amount, Page: Page{Size: 100}})))
			gotCash, hasCash := findAvailable(list.Records, cash.ID)
			if hasCash != tc.wantCash {
				t.Errorf("訂單 %d：現金券在列表 = %v，預期 %v", tc.amount, hasCash, tc.wantCash)
			} else if hasCash && gotCash.EstimatedDiscount != tc.cashDiscount {
				t.Errorf("訂單 %d：現金券預估折抵 %d，預期 %d", tc.amount, gotCash.EstimatedDiscount, tc.cashDiscount)
			}
			gotPercent, hasPercent := findAvailable(list.Records, percent.ID)
			if !hasPercent {
				t.Errorf("訂單 %d：百分比券不在列表", tc.amount)
			} else if gotPercent.EstimatedDiscount != min(tc.amount/10, 30) {
				t.Errorf("訂單 %d：百分比券預估折抵 %d，預期 %d", tc.amount, gotPercent.EstimatedDiscount, min(tc.amount/10, 30))
			}
		}
	}},
	{Name: "available coupons need a store and an order amount", Run: func(ctx context.Context, t ch.T, d Driver) {
		_, err := d.ListAvailable(ctx, ch.Buyer, AvailableQuery{})
		ch.WantKind(t, err, ch.ErrInvalid)
		_, err = d.ListAvailable(ctx, ch.Buyer, AvailableQuery{StoreID: d.StoreOf(ch.Seller)})
		ch.WantKind(t, err, ch.ErrInvalid)
	}},
	{Name: "available coupons hide unclaimed claim-required templates until claimed", Run: func(ctx context.Context, t ch.T, d Driver) {
		tpl := adminActive(ctx, t, d, cashInput("coupon-"+ch.Nonce(), TargetAll))
		q := AvailableQuery{StoreID: d.StoreOf(ch.Seller), OrderAmount: 1000, Page: Page{Size: 100}}
		before := ch.Must(t, ch.Ret(d.ListAvailable(ctx, ch.Buyer, q)))
		if _, ok := findAvailable(before.Records, tpl.ID); ok {
			t.Errorf("領券前結帳頁的可用券已經有這張")
		}
		ch.Must(t, ch.Ret(d.Claim(ctx, ch.Buyer, tpl.ID)))
		after := ch.Must(t, ch.Ret(d.ListAvailable(ctx, ch.Buyer, q)))
		got, ok := findAvailable(after.Records, tpl.ID)
		if !ok {
			t.Fatalf("領券後結帳頁的可用券看不到這張")
		}
		if got.EstimatedDiscount != 50 {
			t.Errorf("預估折抵 %d，預期 50", got.EstimatedDiscount)
		}
	}},
	{Name: "available coupons report the total across pages", Run: func(ctx context.Context, t ch.T, d Driver) {
		if skipOnLegacy(t, d, "BD12：舊的 total 是記憶體過濾後該頁的筆數，不是總數") {
			return
		}
		sellerReady(ctx, t, d)
		for range 3 {
			created := ch.Must(t, ch.Ret(d.SellerCreate(ctx, ch.Seller, cashInput("coupon-"+ch.Nonce(), TargetAll))))
			claimIfWallet(ctx, t, d, ch.Buyer, created.ID)
		}
		list := ch.Must(t, ch.Ret(d.ListAvailable(ctx, ch.Buyer, AvailableQuery{StoreID: d.StoreOf(ch.Seller), OrderAmount: 1000, Page: Page{Number: 1, Size: 1}})))
		if len(list.Records) != 1 || list.Total < 3 {
			t.Errorf("第一頁 %d 筆、total %d，預期 1 筆、total 至少 3", len(list.Records), list.Total)
		}
	}},
}

func window() (start, end time.Time) {
	now := time.Now()
	return now.Add(-time.Hour), now.Add(30 * 24 * time.Hour)
}

func cashInput(title string, target Target) TemplateInput {
	start, end := window()
	return TemplateInput{Title: title, Type: TypeCash, Target: target, Threshold: 200, DiscountAmount: 50, StartAt: start, EndAt: end}
}

func skipOnLegacy(t ch.T, d Driver, why string) bool {
	if !d.Legacy() {
		return false
	}
	s, ok := t.(interface{ Skipf(string, ...any) })
	if !ok {
		panic("skipOnLegacy：這個 T 不能跳過，情境不能在舊的一端靜默通過")
	}
	s.Skipf("舊的 coupon-service 的行為不同：%s", why)
	return true
}

func sellerReady(ctx context.Context, t ch.T, d Driver) {
	t.Helper()
	if err := d.AdminSetQuota(ctx, ch.Admin, d.StoreOf(ch.Seller), 100); err != nil {
		t.Fatalf("設定賣家配額：%v", err)
	}
}

func adminActive(ctx context.Context, t ch.T, d Driver, in TemplateInput) Template {
	t.Helper()
	tpl := ch.Must(t, ch.Ret(d.AdminCreate(ctx, ch.Admin, in)))
	if err := d.AdminPublish(ctx, ch.Admin, tpl.ID); err != nil {
		t.Fatalf("上架：%v", err)
	}
	return tpl
}

func find(list []Template, id string) (Template, bool) {
	for _, tpl := range list {
		if tpl.ID == id {
			return tpl, true
		}
	}
	return Template{}, false
}

func findAvailable(list []AvailableCoupon, id string) (AvailableCoupon, bool) {
	for _, c := range list {
		if c.Template.ID == id {
			return c, true
		}
	}
	return AvailableCoupon{}, false
}

func mustFindIn(t ch.T, list []Template, id string) Template {
	t.Helper()
	tpl, ok := find(list, id)
	if !ok {
		t.Fatalf("列表裡找不到模板 %s", id)
	}
	return tpl
}

func myCoupons(ctx context.Context, t ch.T, d Driver, c ch.Caller) []Template {
	t.Helper()
	var all []Template
	for n := 1; ; n++ {
		got := ch.Must(t, ch.Ret(d.ListMyCoupons(ctx, c, ListQuery{Page: Page{Number: n, Size: 100}})))
		all = append(all, got.Available...)
		if len(got.Available) == 0 || len(all) >= got.Total {
			return all
		}
	}
}

func sellerTemplates(ctx context.Context, t ch.T, d Driver, c ch.Caller, status Status) []Template {
	t.Helper()
	var all []Template
	for n := 1; ; n++ {
		got := ch.Must(t, ch.Ret(d.SellerList(ctx, c, status, Page{Number: n, Size: 100})))
		all = append(all, got.Records...)
		if len(got.Records) == 0 || len(all) >= got.Total {
			return all
		}
	}
}

func mustFind(ctx context.Context, t ch.T, d Driver, id string) Template {
	t.Helper()
	return mustFindIn(t, sellerTemplates(ctx, t, d, ch.Seller, ""), id)
}

func mustDetail(ctx context.Context, t ch.T, d Driver, c ch.Caller, id string) SellerDetail {
	t.Helper()
	return ch.Must(t, ch.Ret(d.SellerDetail(ctx, c, id)))
}

func skipOnMonolith(t ch.T, d Driver, why string) bool {
	if d.Legacy() {
		return false
	}
	s, ok := t.(interface{ Skipf(string, ...any) })
	if !ok {
		panic("skipOnMonolith：這個 T 不能跳過，情境不能在單體的一端靜默通過")
	}
	s.Skipf("單體不保留這個能力：%s", why)
	return true
}

func skipWithoutAdminRPC(t ch.T, d Driver) bool {
	if d.AdminRPC() {
		return false
	}
	s, ok := t.(interface{ Skipf(string, ...any) })
	if !ok {
		panic("skipWithoutAdminRPC：這個 T 不能跳過，情境不能靜默通過")
	}
	s.Skipf("這個 driver 沒有 admin 對外 API：這個情境驗的是 admin RPC 本身")
	return true
}

func claimIfWallet(ctx context.Context, t ch.T, d Driver, c ch.Caller, ids ...string) {
	t.Helper()
	if d.Legacy() {
		return
	}
	for _, id := range ids {
		ch.Must(t, ch.Ret(d.Claim(ctx, c, id)))
	}
}
