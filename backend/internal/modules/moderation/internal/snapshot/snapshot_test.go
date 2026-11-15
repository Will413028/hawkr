package snapshot

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/model"
)

func comment(body string, refs ...string) Content {
	return Content{Kind: model.TargetComment, AuthorKind: "BUYER", AuthorID: 7, StoreID: 3, Body: body, MediaRefs: refs}
}

func TestEquivalentRepresentationsHashTheSame(t *testing.T) {
	_, want := Encode(comment("第一行\n第二行", "b", "a"))
	for name, c := range map[string]Content{
		"CRLF":         comment("第一行\r\n第二行", "a", "b"),
		"頭尾空白":         comment("　第一行\n第二行 \t", "a", "b"),
		"媒體參照重複":       comment("第一行\n第二行", "a", "b", "a"),
		"舊式 CR 換行":     comment("第一行\r第二行", "b", "a"),
		"NBSP 與全形空白包住": comment(" 第一行\n第二行　", "a", "b"),
	} {
		if _, got := Encode(c); got != want {
			t.Errorf("%s：雜湊應與正規形式相同", name)
		}
	}
}

func TestAnyChangeOfContentChangesTheHash(t *testing.T) {
	_, base := Encode(comment("本文", "a"))
	for name, c := range map[string]Content{
		"本文":   comment("本文改了", "a"),
		"媒體":   comment("本文", "b"),
		"沒有媒體": comment("本文"),
		"作者":   func() Content { x := comment("本文", "a"); x.AuthorID = 8; return x }(),
		"種類":   func() Content { x := comment("本文", "a"); x.Kind = model.TargetForumEntry; return x }(),
		"店家":   func() Content { x := comment("本文", "a"); x.StoreID = 4; return x }(),
		"中間換行": comment("本\n文", "a"),
	} {
		if _, got := Encode(c); got == base {
			t.Errorf("%s 不同，雜湊不能相同", name)
		}
	}
}

func TestBuyerProfileFieldsAreHashed(t *testing.T) {
	a := Content{Kind: model.TargetBuyer, DisplayName: "小明", Avatar: "k1"}
	b := a
	b.Avatar = "k2"
	c := a
	c.DisplayName = "小華"
	d := a
	d.Deleted = true
	_, ha := Encode(a)
	for name, x := range map[string]Content{"頭像": b, "名稱": c, "刪除": d} {
		if _, h := Encode(x); h == ha {
			t.Errorf("%s 不同，雜湊不能相同", name)
		}
	}
	if text, _ := Encode(Content{Kind: model.TargetBuyer}); len(text) < 2 {
		t.Fatal("空內容也要是合法的 JSON 物件")
	}
}

func TestNilAndEmptyMediaAreTheSame(t *testing.T) {
	x, hx := Encode(comment("本文"))
	y, hy := Encode(comment("本文", []string{}...))
	if x != y || hx != hy {
		t.Fatalf("nil 與空切片要相同：%s %s", x, y)
	}
}
