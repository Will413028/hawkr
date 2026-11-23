package admin

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
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

func TestCreateStyleRejectsInvalidInputWithoutFinalizingTheThumbnail(t *testing.T) {
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
		"預設風格不能停用":   {func(in *model.CreateStyleInput) { in.IsDefault, in.Enabled = true, false }, "Enabled"},
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
			if len(h.thumbs.finalized) != 0 || len(h.q.styles) != 0 || len(h.audit.entries) != 0 {
				t.Fatalf("驗證失敗不該轉正縮圖或寫入：%v %v %v", h.thumbs.finalized, h.q.styles, h.audit.entries)
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

func TestCreateStyleStoresTheFinalizedThumbnailAndWritesTheAuditInTheSameTransaction(t *testing.T) {
	h := newHarness(t)
	s, err := h.s.CreateStyle(context.Background(), createInput())
	if err != nil {
		t.Fatal(err)
	}
	if s.ThumbnailKey != "public/t1.jpg" || s.Version != 1 || s.Category != model.CategoryProduct || s.DisplayOrder != 2 {
		t.Fatalf("回傳的風格 = %+v", s)
	}
	row := h.q.styles[s.ID]
	if row.ThumbnailAssetID != "asset-t1" {
		t.Fatalf("asset = %q", row.ThumbnailAssetID)
	}
	if len(h.thumbs.changed) != 1 || len(h.thumbs.changed[0]) != 0 {
		t.Fatalf("Changed 呼叫 = %v，新增時預期一次、沒有退場資產", h.thumbs.changed)
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
	if len(h.audit.entries) != 0 || len(h.thumbs.changed) != 0 {
		t.Fatal("衝突時不該寫稽核或排同步")
	}
	// 同名但不同用途可以。
	in := createInput()
	in.Category = model.CategoryPost
	if _, err := h.s.CreateStyle(context.Background(), in); err != nil {
		t.Fatal(err)
	}
}

func TestCreateStyleAsDefaultClearsTheCategorysPreviousDefault(t *testing.T) {
	h := newHarness(t)
	old := h.seed("old", "PRODUCT", true, "")
	otherCategory := h.seed("post-default", "POST", true, "")
	in := createInput()
	in.IsDefault = true
	s, err := h.s.CreateStyle(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if !s.IsDefault || h.q.styles[old.ID].IsDefault || !h.q.styles[otherCategory.ID].IsDefault {
		t.Fatalf("新風格 default=%v、舊預設 default=%v、其他用途的預設 default=%v", s.IsDefault, h.q.styles[old.ID].IsDefault, h.q.styles[otherCategory.ID].IsDefault)
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
	if !errors.As(err, &ae) || ae.Message != text.AdminStyleEtagRequired(uitext.LocaleFrom(context.Background())).Text || ae.Message == text.AdminStyleStale(uitext.LocaleFrom(context.Background())).Text {
		t.Fatalf("缺 etag 要用自己的文字，得到 %v", err)
	}
}

func TestCreateStyleConcurrentDefaultIsFailedPreconditionWithItsOwnText(t *testing.T) {
	h := newHarness(t)
	h.q.racingDefault = true
	in := createInput()
	in.IsDefault = true
	_, err := h.s.CreateStyle(context.Background(), in)
	wantKind(t, err, apperr.KindFailedPrecondition)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Message != text.AdminStyleDefaultConflict(uitext.LocaleFrom(context.Background())).Text {
		t.Fatalf("得到 %v", err)
	}
	if len(h.audit.entries) != 0 {
		t.Fatal("失敗時不該有稽核")
	}
}

func TestReplacingTheDefaultAuditsTheClearedStyleToo(t *testing.T) {
	ctx := context.Background()
	for name, run := range map[string]func(h *harness, old store.ImageaiStyle) error{
		"create": func(h *harness, _ store.ImageaiStyle) error {
			in := createInput()
			in.IsDefault = true
			_, err := h.s.CreateStyle(ctx, in)
			return err
		},
		"update": func(h *harness, _ store.ImageaiStyle) error {
			other := h.seed("other", "PRODUCT", false, "a")
			in := updateInput(other.ID, other.Version)
			in.IsDefault = true
			_, err := h.s.UpdateStyle(ctx, in)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			old := h.seed("old", "PRODUCT", true, "")
			if err := run(h, old); err != nil {
				t.Fatal(err)
			}
			if len(h.audit.entries) != 2 {
				t.Fatalf("新預設與被取消的舊預設各一筆稽核，得到 %d", len(h.audit.entries))
			}
			if !slices.ContainsFunc(h.audit.entries, func(e adminaudit.Entry) bool { return e.TargetID == old.ID.String() }) {
				t.Fatalf("沒有被取消預設的風格 %s 的稽核：%v", old.ID, h.audit.entries)
			}
		})
	}
}

func TestUpdateStyleStaleVersionIsRejectedBeforeFinalizingTheThumbnail(t *testing.T) {
	h := newHarness(t)
	s := h.seed("a", "PRODUCT", false, "a")
	in := updateInput(s.ID, s.Version-1)
	in.ThumbnailToken = "t2"
	_, err := h.s.UpdateStyle(context.Background(), in)
	wantKind(t, err, apperr.KindVersionConflict)
	if reasonOf(err) != apperr.ReasonVersionConflict {
		t.Fatalf("reason = %v", reasonOf(err))
	}
	if len(h.thumbs.finalized) != 0 || len(h.audit.entries) != 0 {
		t.Fatal("版本不符時不該轉正縮圖或寫稽核")
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
	if len(h.thumbs.finalized) != 0 || len(h.thumbs.changed) != 0 {
		t.Fatalf("沒換圖不該轉正或排同步：%v %v", h.thumbs.finalized, h.thumbs.changed)
	}
	if len(h.audit.entries) != 1 || h.audit.entries[0].TargetType != "ai_style" {
		t.Fatalf("稽核 = %v", h.audit.entries)
	}
}

func TestUpdateStyleReplacingTheThumbnailRetiresTheOldAsset(t *testing.T) {
	h := newHarness(t)
	s := h.seed("a", "PRODUCT", false, "old")
	in := updateInput(s.ID, s.Version)
	in.ThumbnailToken = "new"
	got, err := h.s.UpdateStyle(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if got.ThumbnailKey != "public/new.jpg" || h.q.styles[s.ID].ThumbnailAssetID != "asset-new" {
		t.Fatalf("縮圖沒換：%+v", got)
	}
	if len(h.thumbs.changed) != 1 || !slices.Equal(h.thumbs.changed[0], []string{"old"}) || !slices.Equal(h.thumbs.rejected, []string{"asset-new"}) {
		t.Fatalf("changed=%v rejected=%v", h.thumbs.changed, h.thumbs.rejected)
	}
}

func TestUpdateStyleGivingAThumbnailToAStyleWithoutOneRetiresNothing(t *testing.T) {
	h := newHarness(t)
	s := h.seed("seed", "PRODUCT", true, "")
	in := updateInput(s.ID, s.Version)
	in.IsDefault, in.ThumbnailToken = true, "new"
	if _, err := h.s.UpdateStyle(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if len(h.thumbs.changed) != 1 || len(h.thumbs.changed[0]) != 0 {
		t.Fatalf("changed = %v，預期一次且沒有退場資產（空 asset ID 不是資產）", h.thumbs.changed)
	}
}

func TestUpdateStyleTokenThatCannotBeFinalizedLeavesTheStyleUntouched(t *testing.T) {
	h := newHarness(t)
	h.thumbs.failToken = "bad"
	s := h.seed("a", "PRODUCT", false, "a")
	in := updateInput(s.ID, s.Version)
	in.ThumbnailToken = "bad"
	_, err := h.s.UpdateStyle(context.Background(), in)
	wantKind(t, err, apperr.KindInvalidInput)
	if h.q.styles[s.ID].Version != s.Version || len(h.audit.entries) != 0 {
		t.Fatal("轉正失敗不該改風格或寫稽核")
	}
}

func TestUpdateStyleDefaultRules(t *testing.T) {
	ctx := context.Background()
	t.Run("目前的預設風格設成 false 是 invalid_argument", func(t *testing.T) {
		h := newHarness(t)
		d := h.seed("d", "PRODUCT", true, "")
		in := updateInput(d.ID, d.Version)
		in.IsDefault = false
		_, err := h.s.UpdateStyle(ctx, in)
		wantKind(t, err, apperr.KindInvalidInput)
		if got := fieldsOf(err); !slices.Equal(got, []string{"IsDefault"}) {
			t.Fatalf("欄位 = %v", got)
		}
		if !h.q.styles[d.ID].IsDefault {
			t.Fatal("預設風格不該被取消")
		}
	})
	t.Run("停用目前的預設風格是 failed_precondition", func(t *testing.T) {
		h := newHarness(t)
		d := h.seed("d", "PRODUCT", true, "")
		in := updateInput(d.ID, d.Version)
		in.IsDefault, in.Enabled = true, false
		_, err := h.s.UpdateStyle(ctx, in)
		wantKind(t, err, apperr.KindFailedPrecondition)
	})
	t.Run("把另一個風格設成預設，原本的預設在同一次寫入取消", func(t *testing.T) {
		h := newHarness(t)
		d := h.seed("d", "PRODUCT", true, "")
		other := h.seed("o", "PRODUCT", false, "")
		in := updateInput(other.ID, other.Version)
		in.IsDefault = true
		got, err := h.s.UpdateStyle(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		if !got.IsDefault || h.q.styles[d.ID].IsDefault {
			t.Fatalf("新預設 %v、舊預設 %v", got.IsDefault, h.q.styles[d.ID].IsDefault)
		}
	})
	t.Run("停用中的風格不能設成預設", func(t *testing.T) {
		h := newHarness(t)
		o := h.seed("o", "PRODUCT", false, "")
		in := updateInput(o.ID, o.Version)
		in.IsDefault, in.Enabled = true, false
		_, err := h.s.UpdateStyle(ctx, in)
		wantKind(t, err, apperr.KindInvalidInput)
	})
	t.Run("非預設的風格 default_style=false 不變", func(t *testing.T) {
		h := newHarness(t)
		o := h.seed("o", "PRODUCT", false, "")
		if _, err := h.s.UpdateStyle(ctx, updateInput(o.ID, o.Version)); err != nil {
			t.Fatal(err)
		}
	})
}

func TestDeleteStyleRefusesTheDefaultStyle(t *testing.T) {
	h := newHarness(t)
	d := h.seed("d", "PRODUCT", true, "a")
	err := h.s.DeleteStyle(context.Background(), actor, d.ID, 0)
	wantKind(t, err, apperr.KindFailedPrecondition)
	if _, ok := h.q.styles[d.ID]; !ok || len(h.audit.entries) != 0 || len(h.thumbs.changed) != 0 {
		t.Fatal("預設風格不能刪，也不該寫稽核或釋放縮圖")
	}
}

func TestDeleteStyleReleasesTheThumbnailAndAudits(t *testing.T) {
	h := newHarness(t)
	s := h.seed("a", "PRODUCT", false, "a")
	if err := h.s.DeleteStyle(context.Background(), actor, s.ID, 0); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.q.styles[s.ID]; ok {
		t.Fatal("風格沒被刪")
	}
	if len(h.thumbs.changed) != 1 || !slices.Equal(h.thumbs.changed[0], []string{"a"}) {
		t.Fatalf("changed = %v，預期退場 [a]", h.thumbs.changed)
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

func TestDeleteStyleWithoutAThumbnailRetiresNothing(t *testing.T) {
	h := newHarness(t)
	s := h.seed("a", "PRODUCT", false, "")
	if err := h.s.DeleteStyle(context.Background(), actor, s.ID, 0); err != nil {
		t.Fatal(err)
	}
	if len(h.thumbs.changed) != 1 || len(h.thumbs.changed[0]) != 0 {
		t.Fatalf("changed = %v", h.thumbs.changed)
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
