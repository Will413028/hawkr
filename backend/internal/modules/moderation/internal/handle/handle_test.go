package handle

import (
	"context"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/snapshot"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/text"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func violation(a model.Action) model.DecideInput {
	return model.DecideInput{
		CaseID: uuid.NewV7(), Version: 1, Actor: adminaudit.Actor{AdminID: 3, Action: "/test/Decide"}, Decision: model.DecisionViolation, Category: policy.Spam, Action: a, Text: "理由",
	}
}

func TestValidateViolationShapes(t *testing.T) {
	takedown := violation(model.ActionTakedown)
	takedown.Severity = string(seller.SeverityValues()[0])
	for name, in := range map[string]model.DecideInput{
		"隱藏": violation(model.ActionHide), "警告": violation(model.ActionWarn), "停權": violation(model.ActionBan), "下架": takedown,
	} {
		if _, err := Validate(in); err != nil {
			t.Errorf("%s：合法的輸入被拒絕：%v", name, err)
		}
	}
	bad := map[string]func(*model.DecideInput){
		"沒有案件":    func(in *model.DecideInput) { in.CaseID = uuid.UUID{} },
		"版本為零":    func(in *model.DecideInput) { in.Version = 0 },
		"沒有處置":    func(in *model.DecideInput) { in.Action = "" },
		"處置不認得":   func(in *model.DecideInput) { in.Action = "DISMISS" },
		"沒有類別":    func(in *model.DecideInput) { in.Category = "" },
		"OTHER":   func(in *model.DecideInput) { in.Category = policy.Other },
		"商品類別":    func(in *model.DecideInput) { in.Category = policy.Counterfeit },
		"理由空白":    func(in *model.DecideInput) { in.Text = " 　 " },
		"理由太長":    func(in *model.DecideInput) { in.Text = strings.Repeat("字", model.MaxTextRunes+1) },
		"隱藏帶了嚴重度": func(in *model.DecideInput) { in.Severity = "LOW" },
	}
	for name, mutate := range bad {
		in := violation(model.ActionHide)
		mutate(&in)
		if _, err := Validate(in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v", name, err)
		}
	}
	for name, mutate := range map[string]func(*model.DecideInput){
		"沒有嚴重度":  func(in *model.DecideInput) { in.Severity = "" },
		"嚴重度不認得": func(in *model.DecideInput) { in.Severity = "SEVERE" },
	} {
		in := takedown
		mutate(&in)
		if _, err := Validate(in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("下架 %s：%v", name, err)
		}
	}
}

// 缺管理員或缺 procedure 名稱是 handler 的接線錯誤，不是使用者能修的輸入：回沒有分類的錯誤。
func TestValidateWiringErrorsAreUnclassified(t *testing.T) {
	base := model.DecideInput{CaseID: uuid.NewV7(), Version: 1, Actor: adminaudit.Actor{AdminID: 1, Action: "/test/Decide"}, Decision: model.DecisionNoViolation}
	for name, mutate := range map[string]func(*model.DecideInput){
		"沒有管理員":        func(in *model.DecideInput) { in.Actor.AdminID = 0 },
		"沒有 procedure": func(in *model.DecideInput) { in.Actor.Action = "" },
	} {
		in := base
		mutate(&in)
		_, err := Validate(in)
		if err == nil || apperr.KindOf(err) == apperr.KindInvalidInput {
			t.Errorf("%s：%v", name, err)
		}
	}
}

func TestValidateNoViolationCarriesNothingElse(t *testing.T) {
	ok := model.DecideInput{CaseID: uuid.NewV7(), Version: 2, Actor: adminaudit.Actor{AdminID: 1, Action: "/test/Decide"}, Decision: model.DecisionNoViolation}
	if _, err := Validate(ok); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*model.DecideInput){
		"類別": func(in *model.DecideInput) { in.Category = policy.Spam },
		"處置": func(in *model.DecideInput) { in.Action = model.ActionHide },
		"理由": func(in *model.DecideInput) { in.Text = "x" },
	} {
		in := ok
		mutate(&in)
		if _, err := Validate(in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("不違規帶了%s：%v", name, err)
		}
	}
	in := ok
	in.Decision = ""
	if _, err := Validate(in); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Errorf("空的決定：%v", err)
	}
}

func TestValidateMissingActionIsAWiringErrorNotUserInput(t *testing.T) {
	in := violation(model.ActionHide)
	in.Actor.Action = ""
	if _, err := Validate(in); err == nil || apperr.KindOf(err) != apperr.KindInternal {
		t.Fatalf("沒有 procedure 名稱預期未分類的內部錯誤：%v", err)
	}
}

func TestCheckTakedownStoreUsesTheSnapshotStore(t *testing.T) {
	owner := func(author, store int64) social.ContentOwner {
		return social.ContentOwner{Author: social.Actor{Kind: social.ActorStore, ID: author}, StoreID: store}
	}
	if err := CheckTakedownStore(owner(5, 5), 5); err != nil {
		t.Fatalf("作者與歸屬都是快照店家：%v", err)
	}
	for name, c := range map[string]struct {
		o social.ContentOwner
		s int64
	}{
		"現在的作者不是快照店家": {owner(6, 6), 5},
		"歸屬不是快照店家":    {owner(5, 6), 5},
		"快照沒有店家":      {owner(5, 5), 0},
		"兩邊都沒有店家":     {owner(0, 0), 0},
		"作者是買家":       {social.ContentOwner{Author: social.Actor{Kind: social.ActorBuyer, ID: 5}, StoreID: 5}, 5},
	} {
		if err := CheckTakedownStore(c.o, c.s); apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Errorf("%s：%v", name, err)
		}
	}
}

type fakeSnapshots struct {
	body  string
	err   error
	reads int
}

func (f *fakeSnapshots) LatestSnapshotContent(context.Context, uuid.UUID) (string, error) {
	f.reads++
	return f.body, f.err
}

func TestFrozenContentMissingSnapshotUsesTheCallersSentence(t *testing.T) {
	q := &fakeSnapshots{err: pgx.ErrNoRows}
	for name, missing := range map[string]func() uitext.Message{"decide": text.DecideSnapshotMissing, "takedown": text.DecideTakedownSnapshotMissing} {
		var res applied
		_, err := res.frozenContent(context.Background(), q, uuid.New(), missing)
		want := apperr.FailedPrecondition("%s", missing())
		if apperr.KindOf(err) != apperr.KindFailedPrecondition || err.Error() != want.Error() {
			t.Errorf("%s：%v，預期 %v", name, err, want)
		}
	}
	if text.DecideSnapshotMissing().Text == text.DecideTakedownSnapshotMissing().Text {
		t.Fatal("內容還在時缺快照不能用「內容已不存在」的句子")
	}
}

func TestFrozenContentReadsTheSnapshotOncePerDecide(t *testing.T) {
	body, _ := snapshot.Encode(snapshot.Content{Kind: model.TargetPost, Social: &snapshot.Social{AuthorKind: "STORE", AuthorID: 7, StoreID: 7}})
	q := &fakeSnapshots{body: body}
	var res applied
	for range 3 {
		got, err := res.frozenContent(context.Background(), q, uuid.New(), text.DecideSnapshotMissing)
		if err != nil || got.StoreID() != 7 {
			t.Fatalf("%+v %v", got, err)
		}
	}
	if q.reads != 1 {
		t.Fatalf("讀了 %d 次", q.reads)
	}
}

// 措施的內容快照取自案件凍結的內容：本文全文、發文時間、圖片張數；連結依種類（討論區留言是店家加討論串，其餘是貼文）；缺連結的舊格式不傳半個快照。
func TestMeasureSnapshotFromTheFrozenContent(t *testing.T) {
	at := time.Date(2026, 11, 1, 3, 4, 5, 0, time.UTC)
	post, thread := uuid.NewV7(), uuid.NewV7()
	frozen := snapshot.Social{Body: "全文", AuthoredAt: at, MediaRefs: []string{"a", "b"}, PostID: post.String(), ThreadID: thread.String()}
	got, err := measureSnapshot(model.TargetComment, 12, frozen)
	if err != nil || got.Body != "全文" || !got.AuthoredAt.Equal(at) || got.ImageCount != 2 || got.Link != (seller.ContentLink{PostID: post}) {
		t.Fatalf("留言 %+v err=%v", got, err)
	}
	got, err = measureSnapshot(model.TargetForumEntry, 12, frozen)
	if err != nil || got.Link != (seller.ContentLink{StoreID: 12, ThreadID: thread}) {
		t.Fatalf("討論區留言 %+v err=%v", got, err)
	}
	for _, kind := range []model.TargetKind{model.TargetPost, model.TargetComment, model.TargetForumEntry} {
		if _, err := measureSnapshot(kind, 12, snapshot.Social{Body: "舊格式", AuthoredAt: at}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Errorf("%s 缺連結：%v", kind, err)
		}
	}
}

func TestValidateReverse(t *testing.T) {
	ok := model.ReverseInput{CaseID: uuid.NewV7(), Actor: adminaudit.Actor{AdminID: 3, Action: "/test/ReverseHide"}, Reason: " 誤判 "}
	got, err := ValidateReverse(ok)
	if err != nil || got.Reason != "誤判" {
		t.Fatalf("合法的輸入：%+v %v", got, err)
	}
	for name, mutate := range map[string]func(*model.ReverseInput){
		"沒有案件": func(in *model.ReverseInput) { in.CaseID = uuid.UUID{} },
		"原因空白": func(in *model.ReverseInput) { in.Reason = " 　 " },
		"原因太長": func(in *model.ReverseInput) { in.Reason = strings.Repeat("字", model.MaxTextRunes+1) },
	} {
		in := ok
		mutate(&in)
		if _, err := ValidateReverse(in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v", name, err)
		}
	}
	for name, mutate := range map[string]func(*model.ReverseInput){
		"沒有管理員":        func(in *model.ReverseInput) { in.Actor.AdminID = 0 },
		"沒有 procedure": func(in *model.ReverseInput) { in.Actor.Action = "" },
	} {
		in := ok
		mutate(&in)
		if _, err := ValidateReverse(in); err == nil || apperr.KindOf(err) == apperr.KindInvalidInput {
			t.Errorf("%s 是接線錯誤：%v", name, err)
		}
	}
}
