package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// fakeIdem 是冪等表的記憶體替身，**忠實重現 (scope, actor_id, idem_key) 這個主鍵**。
//
// 🔴 少了任何一段主鍵，測試就驗不到它存在的理由：
//   - 少 scope  ⇒ 同一把鍵在不同端點之間互撞（前端常對整個頁面用同一把鍵）
//   - 少 actorID ⇒ A 買家的鍵擋住 B 買家，而鍵是前端產生的、不保證全域唯一
type fakeIdem struct {
	rows map[[3]string]*domain.IdempotencyRecord
}

func newFakeIdem() *fakeIdem { return &fakeIdem{rows: map[[3]string]*domain.IdempotencyRecord{}} }

func (f *fakeIdem) key(scope string, actorID int64, k string) [3]string {
	return [3]string{scope, string(rune(actorID)), k}
}

func (f *fakeIdem) Claim(_ context.Context, in domain.ClaimIdempotency) (domain.IdempotencyRecord, bool, error) {
	k := f.key(in.Scope, in.ActorID, in.Key)
	if rec, ok := f.rows[k]; ok {
		return *rec, false, nil
	}
	rec := &domain.IdempotencyRecord{
		Scope: in.Scope, ActorID: in.ActorID, Key: in.Key, RequestHash: in.RequestHash,
	}
	f.rows[k] = rec
	return *rec, true, nil
}

func (f *fakeIdem) SetResult(_ context.Context, scope string, actorID int64, k string, ref int64) error {
	rec, ok := f.rows[f.key(scope, actorID, k)]
	if !ok {
		return errors.New("no such idempotency row")
	}
	rec.ResultRef = &ref
	return nil
}

// #3015：**同一把鍵 ＋ 不同 body** 是唯一有鑑別力的組合。
//
// 票上逐字記載：只做「同 key ＋ 同 body」會看到第二次被業務去重（9306）擋下，
// **看起來像冪等生效** —— 那是巧合。原報告者第一次就是這樣差點判成 PASS。
func TestClaimIdempotency(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	const scope, actor = IdemScopeBuyerProfileCreate, int64(2623900520)

	mk := func(key, fp string) domain.ClaimIdempotency {
		return domain.ClaimIdempotency{Scope: scope, ActorID: actor, Key: key, RequestHash: fp, Now: now}
	}

	t.Run("第一次 ⇒ FirstTime", func(t *testing.T) {
		f := newFakeIdem()
		got, err := ClaimIdempotency(ctx, f, mk("K1", "hashA"))
		if err != nil || !got.FirstTime {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})

	t.Run("🔴 同鍵 ＋ 不同 body ⇒ 9350，且不得執行（本票的核心）", func(t *testing.T) {
		f := newFakeIdem()
		if _, err := ClaimIdempotency(ctx, f, mk("K2", "hashA")); err != nil {
			t.Fatal(err)
		}
		_ = f.SetResult(ctx, scope, actor, "K2", 111)
		_, err := ClaimIdempotency(ctx, f, mk("K2", "hashB"))
		if !errors.Is(err, ErrIdempotencyKeyReused) {
			t.Fatalf("err = %v, want ErrIdempotencyKeyReused —— 舊行為是「body 不同就當成全新請求」⇒ 建立第二筆", err)
		}
	})

	t.Run("同鍵 ＋ 同 body ⇒ 回放第一次的結果，不重跑", func(t *testing.T) {
		f := newFakeIdem()
		if _, err := ClaimIdempotency(ctx, f, mk("K3", "hashA")); err != nil {
			t.Fatal(err)
		}
		_ = f.SetResult(ctx, scope, actor, "K3", 222)
		got, err := ClaimIdempotency(ctx, f, mk("K3", "hashA"))
		if err != nil {
			t.Fatal(err)
		}
		if got.FirstTime || got.ResultRef != 222 {
			t.Fatalf("got=%+v, want 回放 ResultRef=222", got)
		}
	})

	t.Run("不同鍵 ⇒ 各自獨立", func(t *testing.T) {
		f := newFakeIdem()
		for _, k := range []string{"K4", "K5"} {
			got, err := ClaimIdempotency(ctx, f, mk(k, "hashA"))
			if err != nil || !got.FirstTime {
				t.Fatalf("key=%s got=%+v err=%v", k, got, err)
			}
		}
	})

	t.Run("🔴 不同買家用同一把鍵 ⇒ 互不影響（鍵由前端產生，不保證全域唯一）", func(t *testing.T) {
		f := newFakeIdem()
		a := domain.ClaimIdempotency{Scope: scope, ActorID: 1, Key: "SAME", RequestHash: "hashA", Now: now}
		b := domain.ClaimIdempotency{Scope: scope, ActorID: 2, Key: "SAME", RequestHash: "hashB", Now: now}
		if got, _ := ClaimIdempotency(ctx, f, a); !got.FirstTime {
			t.Fatal("買家 A 的第一次不是 FirstTime")
		}
		got, err := ClaimIdempotency(ctx, f, b)
		if err != nil || !got.FirstTime {
			t.Fatalf("買家 B 被買家 A 的鍵擋住了：got=%+v err=%v", got, err)
		}
	})

	t.Run("空鍵 ⇒ 放行（內部呼叫端不帶鍵）", func(t *testing.T) {
		f := newFakeIdem()
		got, err := ClaimIdempotency(ctx, f, mk("", "hashA"))
		if err != nil || !got.FirstTime {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})

	t.Run("nil repo ⇒ 放行，不 panic", func(t *testing.T) {
		got, err := ClaimIdempotency(ctx, nil, mk("K6", "hashA"))
		if err != nil || !got.FirstTime {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	})
}

// 指紋必須涵蓋使用者改得動的每一欄；漏一欄，那一種改法就繞得過 9350。
func TestRequestFingerprint(t *testing.T) {
	base := RequestFingerprint("MOBILE", "/ABC1234", "", "", "label", "false")
	for _, m := range [][]string{
		{"EMAIL", "/ABC1234", "", "", "label", "false"},          // carrierType
		{"MOBILE", "/XYZ9999", "", "", "label", "false"},         // value
		{"MOBILE", "/ABC1234", "77777177", "", "label", "false"}, // buyerTaxID
		{"MOBILE", "/ABC1234", "", "抬頭", "label", "false"},       // buyerName
		{"MOBILE", "/ABC1234", "", "", "別的", "false"},            // label
		{"MOBILE", "/ABC1234", "", "", "label", "true"},          // setDefault
	} {
		if RequestFingerprint(m...) == base {
			t.Errorf("改了 %v 指紋卻沒變 —— 那一種改法會繞過「同鍵不同內容」的判定", m)
		}
	}
	// 分隔符必須擋得住「欄位值本身含分隔字元」造成的碰撞。
	if RequestFingerprint("a", "b") == RequestFingerprint("ab", "") {
		t.Error("欄位邊界沒守住：不同的輸入折成了同一個字串")
	}
}
