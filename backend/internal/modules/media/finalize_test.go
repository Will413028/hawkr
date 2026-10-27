package media_test

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// rejecting 對 bad 裡的 token 回 err，其餘交給 Fake。
type rejecting struct {
	media.Media
	bad map[string]error
}

func (r rejecting) Finalize(ctx context.Context, u media.Uploader, token, refType string) (media.Asset, error) {
	if err, ok := r.bad[token]; ok {
		return media.Asset{}, err
	}
	return r.Media.Finalize(ctx, u, token, refType)
}

func rejected(msg string) error {
	return fmt.Errorf("mediaservice：轉正（code=404）：%w：%w", media.ErrStagedTokenRejected, apperr.InvalidInput("%s", msg))
}

func violationsOf(t *testing.T, err error) (*apperr.Error, []string) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput {
		t.Fatalf("err = %v，預期 InvalidInput", err)
	}
	var fields []string
	for _, v := range ae.FieldViolations() {
		fields = append(fields, v.Field)
	}
	return ae, fields
}

func TestFinalizeListPointsAtTheRejectedTokenAndKeepsTheCauseInTheText(t *testing.T) {
	m := rejecting{Media: media.NewFake(), bad: map[string]error{"t2": rejected("圖片已過期")}}
	_, err := media.FinalizeList(context.Background(), m, media.Uploader{ID: 1}, "Docs", []string{"t1", "t2"}, media.RefTypeCertification, nil)
	ae, fields := violationsOf(t, err)
	if ae.Message != "圖片已過期" || !slices.Equal(fields, []string{"Docs[1]"}) {
		t.Fatalf("message = %q、欄位 = %v", ae.Message, fields)
	}
	if !strings.Contains(err.Error(), "code=404") {
		t.Fatalf("錯誤的文字缺少 media 的原因：%v", err)
	}
}

// 每個 token 都轉正，所有被拒絕的與 check 判定不合格的一次回報，各自帶索引；訊息不同時整體用通用的一句。
func TestFinalizeListReportsEveryRejectedAndFailedElementAtOnce(t *testing.T) {
	f := media.NewFake()
	m := rejecting{Media: f, bad: map[string]error{"t0": rejected("過期"), "t3": rejected("過期")}}
	_, err := media.FinalizeList(context.Background(), m, media.Uploader{ID: 1}, "Docs", []string{"t0", "t1", "t2", "t3", "t4"}, media.RefTypeCertification, func(i int, _ media.Asset) string {
		if i == 2 {
			return "不是機敏"
		}
		return ""
	})
	ae, fields := violationsOf(t, err)
	if !slices.Equal(fields, []string{"Docs[0]", "Docs[2]", "Docs[3]"}) {
		t.Fatalf("欄位 = %v", fields)
	}
	if ae.Message != "圖片無法使用，請重新上傳" {
		t.Fatalf("訊息不同時要用通用的一句：%q", ae.Message)
	}
	if v := ae.FieldViolations(); v[0].Message != "過期" || v[1].Message != "不是機敏" {
		t.Fatalf("每筆要有自己的訊息：%v", v)
	}
	// 沒有在第一個失敗就停：五個 token 都打了 media 或被拒絕的 wrapper（Fake 只記到沒被攔下的三個）。
	if calls := f.Calls(); len(calls) != 3 {
		t.Fatalf("media 呼叫 = %v，預期未被拒絕的三個都轉正", calls)
	}
}

func TestFinalizeListStopsAtNonTokenErrors(t *testing.T) {
	f := media.NewFake()
	boom := errors.New("boom")
	m := rejecting{Media: f, bad: map[string]error{"t1": boom}}
	_, err := media.FinalizeList(context.Background(), m, media.Uploader{ID: 1}, "Docs", []string{"t0", "t1", "t2"}, media.RefTypeCertification, nil)
	if !errors.Is(err, boom) || apperr.KindOf(err) == apperr.KindInvalidInput {
		t.Fatalf("內部錯誤要原樣回傳：%v", err)
	}
	if calls := f.Calls(); len(calls) != 1 {
		t.Fatalf("media 呼叫 = %v，內部錯誤之後不再轉正", calls)
	}
}

func TestFinalizeFieldUsesTheModuleMessageAndPassesThroughOtherErrors(t *testing.T) {
	m := rejecting{Media: media.NewFake(), bad: map[string]error{"t": rejected("x")}}
	_, err := media.FinalizeField(context.Background(), m, media.Uploader{ID: 1}, media.Field{Name: "Avatar", Message: "頭像無法使用"}, "t", media.RefTypeCertification)
	ae, fields := violationsOf(t, err)
	if ae.Message != "頭像無法使用" || !slices.Equal(fields, []string{"Avatar"}) {
		t.Fatalf("err = %v", err)
	}
	boom := errors.New("boom")
	m.bad["t"] = boom
	_, err = media.FinalizeField(context.Background(), m, media.Uploader{ID: 1}, media.Field{Name: "Avatar"}, "t", media.RefTypeCertification)
	if !errors.Is(err, boom) || apperr.KindOf(err) == apperr.KindInvalidInput {
		t.Fatalf("內部錯誤要原樣回傳：%v", err)
	}
}

// Fake 與真的 client 一樣在本地擋下呼叫方的參數錯誤：缺上傳者、無效的 ref_type、空白的 token 是未分類的程式錯誤，不是 token 被拒絕。
func TestFakeRejectsCallerBugsLikeTheRealClient(t *testing.T) {
	for name, c := range map[string]struct {
		u        media.Uploader
		token    string
		refType  string
		wantText string
	}{
		"缺上傳者":         {media.Uploader{}, "t", media.RefTypeBanner, "缺少上傳者"},
		"無效的 ref_type": {media.Uploader{ID: 1}, "t", "x", "無效的 ref_type"},
		"空白的 token":    {media.Uploader{ID: 1}, "  ", media.RefTypeBanner, "空白"},
	} {
		f := media.NewFake()
		_, err := f.Finalize(context.Background(), c.u, c.token, c.refType)
		if err == nil || errors.Is(err, media.ErrStagedTokenRejected) || apperr.KindOf(err) != apperr.KindInternal || !strings.Contains(err.Error(), c.wantText) || len(f.Calls()) != 0 {
			t.Errorf("%s：err = %v、呼叫 %v", name, err, f.Calls())
		}
	}
}

// refTypes 是 ref_type.go（生成檔）常數的全集：生成檔增減常數時，CheckFinalizeArgs 的封閉集合要跟著改。
func TestRefTypesMatchTheGeneratedConstants(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "ref_type.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for _, v := range spec.Values {
			lit, ok := v.(*ast.BasicLit)
			if !ok {
				continue
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatal(err)
			}
			n++
			if err := media.CheckFinalizeArgs(media.Uploader{ID: 1}, "t", value); err != nil {
				t.Errorf("ref_type %s 不在封閉集合：%v", value, err)
			}
		}
		return true
	})
	if n < 20 {
		t.Fatalf("只解析到 %d 個常數", n)
	}
	if err := media.CheckFinalizeArgs(media.Uploader{ID: 1}, "t", "0000"); err == nil {
		t.Fatal("不在集合的 ref_type 應被擋下")
	}
}
