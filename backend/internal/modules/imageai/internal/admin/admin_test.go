package admin

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func createInput() model.CreateStyleInput {
	return model.CreateStyleInput{
		Actor: actor, Name: "clean", Label: "乾淨", Prompt: "白底", Category: model.CategoryProduct, ThumbnailToken: "t1", DisplayOrder: 2, Enabled: true,
	}
}

func fieldsOf(err error) []string {
	var out []string
	var ae *apperr.Error
	if errors.As(err, &ae) {
		for _, v := range ae.FieldViolations() {
			out = append(out, v.Field)
		}
	}
	return out
}

func TestCreateStyleRejectsInvalidInputWithoutConfirmingTheThumbnail(t *testing.T) {
	for name, mut := range map[string]struct {
		f     func(*model.CreateStyleInput)
		field string
	}{
		"沒有縮圖":       {func(in *model.CreateStyleInput) { in.ThumbnailToken = "" }, "ThumbnailToken"},
		"名稱空白":       {func(in *model.CreateStyleInput) { in.Name = " 　" }, "Name"},
		"顯示名稱空白":     {func(in *model.CreateStyleInput) { in.Label = "" }, "Label"},
		"提示詞空白":      {func(in *model.CreateStyleInput) { in.Prompt = "  " }, "Prompt"},
		"提示詞 501 字元": {func(in *model.CreateStyleInput) { in.Prompt = strings.Repeat("字", 501) }, "Prompt"},
		"用途未指定":      {func(in *model.CreateStyleInput) { in.Category = "" }, "Category"},
		"用途不認得":      {func(in *model.CreateStyleInput) { in.Category = "OTHER" }, "Category"},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			in := createInput()
			mut.f(&in)
			_, err := h.s.CreateStyle(context.Background(), in)
			wantKind(t, err, apperr.KindInvalidInput)
			if got := fieldsOf(err); !slices.Equal(got, []string{mut.field}) {
				t.Fatalf("欄位 = %v，預期 [%s]", got, mut.field)
			}
			if len(h.media.confirmed) != 0 || len(h.q.styles) != 0 || len(h.audit.entries) != 0 {
				t.Fatalf("驗證失敗不該確認縮圖或寫入：%v %v %v", h.media.confirmed, h.q.styles, h.audit.entries)
			}
		})
	}
}

func TestCreateStylePromptAcceptsExactly500CharactersAfterTrimming(t *testing.T) {
	h := newHarness(t)
	in := createInput()
	in.Prompt = "  " + strings.Repeat("字", 500) + "\n"
	s, err := h.s.CreateStyle(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if s.Prompt != strings.Repeat("字", 500) {
		t.Fatalf("提示詞沒有去掉頭尾空白：%q", s.Prompt)
	}
}

func TestCreateStyleStoresTheConfirmedThumbnailAndWritesTheAuditInTheSameTransaction(t *testing.T) {
	h := newHarness(t)
	s, err := h.s.CreateStyle(context.Background(), createInput())
	if err != nil {
		t.Fatal(err)
	}
	if s.ThumbnailKey != "public/t1.jpg" || s.Version != 1 || s.Category != model.CategoryProduct || s.DisplayOrder != 2 {
		t.Fatalf("回傳的風格 = %+v", s)
	}
	row := h.q.styles[s.ID]
	if row.ThumbnailAssetID == nil || *row.ThumbnailAssetID != assetOf("t1") {
		t.Fatalf("asset = %v", row.ThumbnailAssetID)
	}
	if len(h.media.deleted) != 0 {
		t.Fatalf("新增不得標記刪除：%v", h.media.deleted)
	}
	if len(h.audit.entries) != 1 {
		t.Fatalf("稽核 = %v，預期一筆（fakeAudit 只在 transaction 內接受）", h.audit.entries)
	}
	e := h.audit.entries[0]
	if e.TargetType != "ai_style" || e.TargetID != s.ID.String() || e.Actor != (actorOf()) {
		t.Fatalf("稽核內容 = %+v", e)
	}
	if _, leaked := e.Detail["prompt"]; leaked {
		t.Fatal("稽核 detail 不放提示詞內容")
	}
}

func TestCreateStyleFailsWhenTheAuditCannotBeWritten(t *testing.T) {
	h := newHarness(t)
	h.s.d.Audit = failingAudit{}
	if _, err := h.s.CreateStyle(context.Background(), createInput()); err == nil {
		t.Fatal("稽核寫入失敗時，業務寫入不能成立")
	}
}

func TestCreateStyleDuplicateNameIsAlreadyExists(t *testing.T) {
	h := newHarness(t)
	h.seed("clean", "PRODUCT", false, "a")
	_, err := h.s.CreateStyle(context.Background(), createInput())
	wantKind(t, err, apperr.KindAlreadyExists)
	if len(h.audit.entries) != 0 || len(h.media.deleted) != 0 {
		t.Fatal("衝突時不該寫稽核或標記刪除")
	}
	in := createInput()
	in.Category = model.CategoryPost
	if _, err := h.s.CreateStyle(context.Background(), in); err != nil {
		t.Fatal(err)
	}
}

func TestCreateStyleIsNeverTheDefaultAndLeavesTheCurrentDefaultAlone(t *testing.T) {
	h := newHarness(t)
	old := h.seed("old", "PRODUCT", true, "")
	s, err := h.s.CreateStyle(context.Background(), createInput())
	if err != nil {
		t.Fatal(err)
	}
	if s.IsDefault || !h.q.styles[old.ID].IsDefault || h.q.styles[old.ID].Version != old.Version {
		t.Fatalf("新風格 default=%v、舊預設 default=%v version=%d", s.IsDefault, h.q.styles[old.ID].IsDefault, h.q.styles[old.ID].Version)
	}
}

func TestListStylesRejectsAnUnknownCategoryAndFiltersByCategory(t *testing.T) {
	h := newHarness(t)
	h.seed("a", "PRODUCT", false, "")
	h.seed("b", "POST", false, "")
	bad := model.StyleCategory("")
	_, err := h.s.ListStyles(context.Background(), &bad)
	wantKind(t, err, apperr.KindInvalidInput)
	post := model.CategoryPost
	got, err := h.s.ListStyles(context.Background(), &post)
	if err != nil || len(got) != 1 || got[0].Name != "b" {
		t.Fatalf("got=%v err=%v", got, err)
	}
	all, err := h.s.ListStyles(context.Background(), nil)
	if err != nil || len(all) != 2 {
		t.Fatalf("全部用途 = %v err=%v", all, err)
	}
}

func updateInput(id [16]byte, version int64) model.UpdateStyleInput {
	return model.UpdateStyleInput{Actor: actor, ID: id, Version: version, Label: "新名", Prompt: "新提示", DisplayOrder: 5, Enabled: true}
}

func TestUpdateStyleRequiresTheVersion(t *testing.T) {
	h := newHarness(t)
	s := h.seed("a", "PRODUCT", false, "a")
	_, err := h.s.UpdateStyle(context.Background(), updateInput(s.ID, 0))
	wantKind(t, err, apperr.KindInvalidInput)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Message != "必須提供讀取時的版本" || ae.Message == text.AdminStyleStale(uitext.LocaleFrom(context.Background())).Text {
		t.Fatalf("缺 etag 要用自己的文字，得到 %v", err)
	}
}

func TestUpdateStyleStaleVersionIsRejectedBeforeConfirmingTheThumbnail(t *testing.T) {
	h := newHarness(t)
	s := h.seed("a", "PRODUCT", false, "a")
	in := updateInput(s.ID, s.Version+1)
	in.ThumbnailToken = "t2"
	_, err := h.s.UpdateStyle(context.Background(), in)
	wantKind(t, err, apperr.KindVersionConflict)
	if reasonOf(err) != apperr.ReasonVersionConflict {
		t.Fatalf("reason = %v", reasonOf(err))
	}
	if len(h.media.confirmed) != 0 || len(h.audit.entries) != 0 {
		t.Fatal("版本不符時不該確認縮圖或寫稽核")
	}
}

func TestUpdateStyleMissingStyleIsNotFound(t *testing.T) {
	h := newHarness(t)
	s := h.seed("a", "PRODUCT", false, "")
	delete(h.q.styles, s.ID)
	_, err := h.s.UpdateStyle(context.Background(), updateInput(s.ID, 1))
	wantKind(t, err, apperr.KindNotFound)
}

func TestUpdateStyleKeepsTheThumbnailWithoutATokenAndBumpsTheVersion(t *testing.T) {
	h := newHarness(t)
	s := h.seed("a", "PRODUCT", false, "a")
	got, err := h.s.UpdateStyle(context.Background(), updateInput(s.ID, s.Version))
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != s.Version+1 || got.ThumbnailKey != s.ThumbnailKey || got.Label != "新名" || got.DisplayOrder != 5 || got.Name != "a" || got.Category != model.CategoryProduct {
		t.Fatalf("got = %+v", got)
	}
	if len(h.media.confirmed) != 0 || len(h.media.deleted) != 0 {
		t.Fatalf("沒換圖不該確認或標記刪除：%v %v", h.media.confirmed, h.media.deleted)
	}
	if len(h.audit.entries) != 1 || h.audit.entries[0].TargetType != "ai_style" {
		t.Fatalf("稽核 = %v", h.audit.entries)
	}
}

func TestUpdateStyleReplacingTheThumbnailMarksTheOldAssetDeleted(t *testing.T) {
	h := newHarness(t)
	s := h.seed("a", "PRODUCT", false, "old")
	in := updateInput(s.ID, s.Version)
	in.ThumbnailToken = "new"
	got, err := h.s.UpdateStyle(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if got.ThumbnailKey != "public/new.jpg" || *h.q.styles[s.ID].ThumbnailAssetID != assetOf("new") {
		t.Fatalf("縮圖沒換：%+v", got)
	}
	if !slices.Equal(h.media.confirmed, []string{"new"}) || !slices.Equal(h.media.deleted, []uuid.UUID{assetOf("old")}) {
		t.Fatalf("confirmed=%v deleted=%v", h.media.confirmed, h.media.deleted)
	}
}

func TestUpdateStyleGivingAThumbnailToAStyleWithoutOneMarksNothingDeleted(t *testing.T) {
	h := newHarness(t)
	s := h.seed("seed", "PRODUCT", true, "")
	in := updateInput(s.ID, s.Version)
	in.ThumbnailToken = "new"
	if _, err := h.s.UpdateStyle(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if len(h.media.deleted) != 0 {
		t.Fatalf("deleted = %v，預期沒有（原本沒有縮圖資產）", h.media.deleted)
	}
}

func TestUpdateStyleTokenThatCannotBeConfirmedLeavesTheStyleUntouched(t *testing.T) {
	h := newHarness(t)
	h.media.failToken = "bad"
	s := h.seed("a", "PRODUCT", false, "a")
	in := updateInput(s.ID, s.Version)
	in.ThumbnailToken = "bad"
	_, err := h.s.UpdateStyle(context.Background(), in)
	wantKind(t, err, apperr.KindInvalidInput)
	if h.q.styles[s.ID].Version != s.Version || len(h.audit.entries) != 0 {
		t.Fatal("確認失敗不該改風格或寫稽核")
	}
}

func TestUpdateStyleVersionIsCheckedInsideTheTxBeforeTheDefaultRule(t *testing.T) {
	h := newHarness(t)
	s := h.seed("a", "PRODUCT", false, "")
	h.q.onLock = func(id uuid.UUID) {
		cur := h.q.styles[id]
		cur.Version++
		cur.IsDefault = true
		h.q.styles[id] = cur
	}
	in := updateInput(s.ID, s.Version)
	in.Enabled = false
	_, err := h.s.UpdateStyle(context.Background(), in)
	wantKind(t, err, apperr.KindVersionConflict)
	if len(h.audit.entries) != 0 {
		t.Fatal("版本不符不該寫稽核")
	}
}

func TestUpdateStyleDefaultRules(t *testing.T) {
	ctx := context.Background()
	t.Run("停用目前的預設風格是 failed_precondition", func(t *testing.T) {
		h := newHarness(t)
		d := h.seed("d", "PRODUCT", true, "")
		in := updateInput(d.ID, d.Version)
		in.Enabled = false
		_, err := h.s.UpdateStyle(ctx, in)
		wantKind(t, err, apperr.KindFailedPrecondition)
		if !h.q.styles[d.ID].Enabled || len(h.audit.entries) != 0 {
			t.Fatal("被拒絕時不該改風格或寫稽核")
		}
	})
	t.Run("修改預設風格的其他欄位不動預設", func(t *testing.T) {
		h := newHarness(t)
		d := h.seed("d", "PRODUCT", true, "")
		got, err := h.s.UpdateStyle(ctx, updateInput(d.ID, d.Version))
		if err != nil || !got.IsDefault {
			t.Fatalf("got=%v err=%v", got, err)
		}
	})
	t.Run("停用非預設的風格可以", func(t *testing.T) {
		h := newHarness(t)
		o := h.seed("o", "PRODUCT", false, "")
		in := updateInput(o.ID, o.Version)
		in.Enabled = false
		got, err := h.s.UpdateStyle(ctx, in)
		if err != nil || got.Enabled || got.IsDefault {
			t.Fatalf("got=%v err=%v", got, err)
		}
	})
}

func TestSetDefaultMovesTheDefaultAndWritesOneAuditRowNamingThePreviousOne(t *testing.T) {
	h := newHarness(t)
	d := h.seed("d", "PRODUCT", true, "")
	other := h.seed("o", "PRODUCT", false, "")
	post := h.seed("p", "POST", true, "")
	got, err := h.s.SetDefault(context.Background(), actor, other.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsDefault || got.Version != other.Version+1 || h.q.styles[d.ID].IsDefault || !h.q.styles[post.ID].IsDefault {
		t.Fatalf("新預設 %+v、舊預設 default=%v、其他用途 default=%v", got, h.q.styles[d.ID].IsDefault, h.q.styles[post.ID].IsDefault)
	}
	if h.q.styles[d.ID].Version != d.Version+1 {
		t.Fatalf("被取消的舊預設版本 = %d", h.q.styles[d.ID].Version)
	}
	if len(h.audit.entries) != 1 {
		t.Fatalf("只該有目標的一筆稽核，得到 %v", h.audit.entries)
	}
	e := h.audit.entries[0]
	if e.TargetType != styleTarget || e.TargetID != other.ID.String() || e.Detail["previous_default"] != d.ID.String() {
		t.Fatalf("稽核 = %+v", e)
	}
}

func TestSetDefaultRules(t *testing.T) {
	ctx := context.Background()
	t.Run("停用的目標是 failed_precondition", func(t *testing.T) {
		h := newHarness(t)
		d := h.seed("d", "PRODUCT", true, "")
		o := h.seed("o", "PRODUCT", false, "")
		s := h.q.styles[o.ID]
		s.Enabled = false
		h.q.styles[o.ID] = s
		_, err := h.s.SetDefault(ctx, actor, o.ID, 0)
		wantKind(t, err, apperr.KindFailedPrecondition)
		if !h.q.styles[d.ID].IsDefault || h.q.styles[o.ID].IsDefault || len(h.audit.entries) != 0 {
			t.Fatal("被拒絕時不該改預設或寫稽核")
		}
	})
	t.Run("etag 不符是 aborted，不寫入", func(t *testing.T) {
		h := newHarness(t)
		d := h.seed("d", "PRODUCT", true, "")
		o := h.seed("o", "PRODUCT", false, "")
		_, err := h.s.SetDefault(ctx, actor, o.ID, o.Version+1)
		wantKind(t, err, apperr.KindVersionConflict)
		if !h.q.styles[d.ID].IsDefault || len(h.audit.entries) != 0 {
			t.Fatal("版本不符不該改預設或寫稽核")
		}
	})
	t.Run("etag 相符成功", func(t *testing.T) {
		h := newHarness(t)
		h.seed("d", "PRODUCT", true, "")
		o := h.seed("o", "PRODUCT", false, "")
		if _, err := h.s.SetDefault(ctx, actor, o.ID, o.Version); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("不存在是 not_found", func(t *testing.T) {
		h := newHarness(t)
		_, err := h.s.SetDefault(ctx, actor, uuid.MustParse("20000000-0000-7000-8000-000000000001"), 0)
		wantKind(t, err, apperr.KindNotFound)
	})
	t.Run("已經是預設：成功、不改任何東西、不寫稽核", func(t *testing.T) {
		h := newHarness(t)
		d := h.seed("d", "PRODUCT", true, "")
		got, err := h.s.SetDefault(ctx, actor, d.ID, 0)
		if err != nil || !got.IsDefault || got.Version != d.Version || h.q.styles[d.ID].Version != d.Version || len(h.audit.entries) != 0 {
			t.Fatalf("got=%+v err=%v audit=%v", got, err, h.audit.entries)
		}
	})
	t.Run("稽核寫不進去整筆失敗", func(t *testing.T) {
		h := newHarness(t)
		h.s.d.Audit = failingAudit{}
		h.seed("d", "PRODUCT", true, "")
		o := h.seed("o", "PRODUCT", false, "")
		if _, err := h.s.SetDefault(ctx, actor, o.ID, 0); err == nil {
			t.Fatal("稽核失敗時該回錯")
		}
	})
}

func TestDeleteStyleRefusesTheDefaultStyle(t *testing.T) {
	h := newHarness(t)
	d := h.seed("d", "PRODUCT", true, "a")
	err := h.s.DeleteStyle(context.Background(), actor, d.ID, 0)
	wantKind(t, err, apperr.KindFailedPrecondition)
	if _, ok := h.q.styles[d.ID]; !ok || len(h.audit.entries) != 0 || len(h.media.deleted) != 0 {
		t.Fatal("預設風格不能刪，也不該寫稽核或標記刪除縮圖")
	}
}

func TestDeleteStyleMarksTheThumbnailDeletedAndAudits(t *testing.T) {
	h := newHarness(t)
	s := h.seed("a", "PRODUCT", false, "a")
	if err := h.s.DeleteStyle(context.Background(), actor, s.ID, 0); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.q.styles[s.ID]; ok {
		t.Fatal("風格沒被刪")
	}
	if !slices.Equal(h.media.deleted, []uuid.UUID{assetOf("a")}) {
		t.Fatalf("deleted = %v，預期 [a]", h.media.deleted)
	}
	if len(h.audit.entries) != 1 || h.audit.entries[0].TargetID != s.ID.String() {
		t.Fatalf("稽核 = %v", h.audit.entries)
	}
}

func TestDeleteStyleChecksTheVersionOnlyWhenGiven(t *testing.T) {
	h := newHarness(t)
	s := h.seed("a", "PRODUCT", false, "a")
	err := h.s.DeleteStyle(context.Background(), actor, s.ID, s.Version+1)
	wantKind(t, err, apperr.KindVersionConflict)
	if _, ok := h.q.styles[s.ID]; !ok {
		t.Fatal("版本不符不該刪除")
	}
	if err := h.s.DeleteStyle(context.Background(), actor, s.ID, s.Version); err != nil {
		t.Fatal(err)
	}
	wantKind(t, h.s.DeleteStyle(context.Background(), actor, s.ID, 0), apperr.KindNotFound)
}

func TestDeleteStyleWithoutAThumbnailMarksNothingDeleted(t *testing.T) {
	h := newHarness(t)
	s := h.seed("a", "PRODUCT", false, "")
	if err := h.s.DeleteStyle(context.Background(), actor, s.ID, 0); err != nil {
		t.Fatal(err)
	}
	if len(h.media.deleted) != 0 {
		t.Fatalf("deleted = %v", h.media.deleted)
	}
}

func TestStyleWritesRecordTheAuditInsideTheTransaction(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	s := h.seed("a", "PRODUCT", false, "a")
	if _, err := h.s.UpdateStyle(ctx, updateInput(s.ID, s.Version)); err != nil {
		t.Fatal(err)
	}
	if err := h.s.DeleteStyle(ctx, actor, s.ID, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.CreateStyle(ctx, createInput()); err != nil {
		t.Fatal(err)
	}
	if len(h.audit.entries) != 3 {
		t.Fatalf("三個寫入要三筆稽核（transaction 外的 Record 會被 fakeAudit 拒絕），得到 %d", len(h.audit.entries))
	}
}

func reasonOf(err error) apperr.Reason {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Reason
	}
	return apperr.Reason{}
}
