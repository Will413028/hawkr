package snapshot

import (
	"testing"
	"time"

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
	_, ha := Encode(a)
	for name, x := range map[string]Content{"頭像": b, "名稱": c} {
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

func TestDecodeReadsBackWhatEncodeWrote(t *testing.T) {
	c := comment("本文", "asset-b", "asset-a")
	c.AuthorKind, c.AuthorID = "BUYER", 7
	text, _ := Encode(c)
	got, err := Decode(text)
	if err != nil {
		t.Fatal(err)
	}
	if got.AuthorKind != "BUYER" || got.AuthorID != 7 || got.Body != "本文" || len(got.MediaRefs) != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestStoreOfReadsTheFrozenStoreAuthorOnly(t *testing.T) {
	store, _ := Encode(Content{Kind: model.TargetPost, AuthorKind: "STORE", AuthorID: 7, StoreID: 7})
	buyer, _ := Encode(Content{Kind: model.TargetComment, AuthorKind: "BUYER", AuthorID: 7, StoreID: 3})
	if got := StoreOf(store); got == nil || *got != 7 {
		t.Fatalf("店家作者 = %v", got)
	}
	if StoreOf(buyer) != nil || StoreOf("not json") != nil {
		t.Fatal("買家作者與壞掉的快照不回店家")
	}
}

func TestStoreOfRequiresTheStoreToBeTheAuthor(t *testing.T) {
	other, _ := Encode(Content{Kind: model.TargetComment, AuthorKind: "STORE", AuthorID: 8, StoreID: 7})
	if StoreOf(other) != nil {
		t.Fatal("別家店的留言下的店家作者不算快照店家")
	}
}

// 處分依據的發文時間與連結目標一併凍結：寫出再讀回得到同一份（時間以 UTC 微秒為準），改發文時間或連結雜湊就不同。
func TestEncodeFreezesAuthoredTimeAndLinkTarget(t *testing.T) {
	at := time.Date(2026, 11, 1, 3, 4, 5, 123456789, time.FixedZone("TST", 8*3600))
	c := comment("本文")
	c.AuthoredAt, c.PostID = at, "0190a000-0000-7000-8000-000000000001"
	text, sum := Encode(c)
	got, err := Decode(text)
	if err != nil {
		t.Fatal(err)
	}
	if !got.AuthoredAt.Equal(at.Truncate(time.Microsecond)) || got.PostID != c.PostID || got.ThreadID != "" {
		t.Fatalf("讀回 %+v", got)
	}
	for name, mutate := range map[string]func(*Content){
		"發文時間": func(c *Content) { c.AuthoredAt = c.AuthoredAt.Add(time.Second) },
		"貼文連結": func(c *Content) { c.PostID = "0190a000-0000-7000-8000-000000000002" },
		"討論串":  func(c *Content) { c.ThreadID = "0190a000-0000-7000-8000-000000000003" },
	} {
		d := c
		mutate(&d)
		if _, other := Encode(d); other == sum {
			t.Errorf("%s 不同時雜湊要不同", name)
		}
	}
}
