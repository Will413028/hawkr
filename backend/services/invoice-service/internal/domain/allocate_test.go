package domain

import (
	"errors"
	"testing"
)

func TestAllocateLargestRemainder_PostConditions(t *testing.T) {
	cases := []struct {
		name    string
		weights []int64
		total   int64
		want    []int64
	}{
		{"尾差一單位", []int64{333, 333, 334}, 100, []int64{33, 33, 34}},
		{"餘數平手由 weights DESC 決勝", []int64{1250, 750}, 300, []int64{188, 112}},
		{"桶內二次分攤", []int64{360, 890}, 188, []int64{54, 134}},
		{"例 4 行層", []int64{760, 450, 290}, 100, []int64{51, 30, 19}},
		{"total == 0", []int64{10, 20}, 0, []int64{0, 0}},
		{"Σweights == 0", []int64{0, 0}, 0, []int64{0, 0}},
		{"total == Σweights 走快捷", []int64{200, 50}, 250, []int64{200, 50}},
		{"零權重的行不會被加到", []int64{0, 5}, 1, []int64{0, 1}},
		{"空輸入", nil, 0, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := AllocateLargestRemainder(tc.weights, tc.total)
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if len(got) != len(tc.weights) {
				t.Fatalf("len = %d, want %d", len(got), len(tc.weights))
			}
			var sum int64
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
				if got[i] < 0 || got[i] > tc.weights[i] {
					t.Fatalf("out[%d]=%d violates 0 <= out <= weights[%d]=%d", i, got[i], i, tc.weights[i])
				}
				sum += got[i]
			}
			if sum != tc.total {
				t.Fatalf("Σout = %d, want %d", sum, tc.total)
			}
		})
	}
}

// 決定性：同輸入必得同輸出。排序鍵三段缺任何一段都會讓這條在平手輸入上不穩定。
func TestAllocateLargestRemainder_Deterministic(t *testing.T) {
	w := []int64{100, 100, 100, 100, 100, 100, 100}
	first, err := AllocateLargestRemainder(w, 3)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		got, err := AllocateLargestRemainder(w, 3)
		if err != nil {
			t.Fatal(err)
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("run %d: got %v, want %v", i, got, first)
			}
		}
	}
	// 全部權重相同 ⇒ index ASC 決勝 ⇒ 前 3 個各拿 1。
	want := []int64{1, 1, 1, 0, 0, 0, 0}
	for j := range want {
		if first[j] != want[j] {
			t.Fatalf("got %v, want %v", first, want)
		}
	}
}

// 輸入不會被就地排序破壞（必須排索引陣列而不是 weights 本身）。
func TestAllocateLargestRemainder_DoesNotMutateInput(t *testing.T) {
	w := []int64{5, 900, 30}
	_, err := AllocateLargestRemainder(w, 7)
	if err != nil {
		t.Fatal(err)
	}
	if w[0] != 5 || w[1] != 900 || w[2] != 30 {
		t.Fatalf("input mutated: %v", w)
	}
}

func TestAllocateLargestRemainder_Guards(t *testing.T) {
	cases := []struct {
		name    string
		weights []int64
		total   int64
		want    error
	}{
		{"總額為負", []int64{10}, -1, ErrAllocateNegative},
		{"權重為負", []int64{10, -1}, 5, ErrAllocateNegative},
		{"總額超過溢位護欄", []int64{1 << 31}, 1 << 31, ErrAllocateOperandTooLarge},
		{"權重合計超過溢位護欄", []int64{1 << 30, 1 << 30, 1 << 30}, 10, ErrAllocateOperandTooLarge},
		{"總額大於權重合計", []int64{10, 20}, 31, ErrAllocateTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := AllocateLargestRemainder(tc.weights, tc.total)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// 護欄刻意不用業務常數 MaxAmount：小計超過 199,999 但實付未超過的訂單完全合法。
func TestAllocateLargestRemainder_BusinessLimitIsNotTheGuard(t *testing.T) {
	if _, err := AllocateLargestRemainder([]int64{300_000}, 150_000); err != nil {
		t.Fatalf("subtotal above MaxAmount must not be rejected by the overflow guard: %v", err)
	}
}

// 隨機性質測試：後置條件在大量組合上恆成立。
func TestAllocateLargestRemainder_PropertySweep(t *testing.T) {
	seed := int64(1)
	next := func(n int64) int64 {
		seed = (seed*1103515245 + 12345) & 0x7fffffff
		return seed % n
	}
	for iter := 0; iter < 3000; iter++ {
		n := int(next(6)) + 1
		w := make([]int64, n)
		var s int64
		for i := range w {
			w[i] = next(2000)
			s += w[i]
		}
		total := int64(0)
		if s > 0 {
			total = next(s + 1)
		}
		got, err := AllocateLargestRemainder(w, total)
		if err != nil {
			t.Fatalf("iter %d: %v", iter, err)
		}
		var sum int64
		for i := range got {
			if got[i] < 0 || got[i] > w[i] {
				t.Fatalf("iter %d: out[%d]=%d weights=%v total=%d", iter, i, got[i], w, total)
			}
			sum += got[i]
		}
		if sum != total {
			t.Fatalf("iter %d: Σout=%d total=%d weights=%v", iter, sum, total, w)
		}
	}
}

// 排序鍵第二段（weights DESC）的鑑別力測試。
//
// ⚠️ 設計書 §5A.7 例 2 被標為「排序鍵第二段非要不可的證據」，實測不成立：該例的桶依稅別碼
// 升冪排列後恰好也是金額降冪（1250 在前、750 在後），index ASC 會得到同一個答案 ⇒ 拿掉
// weights DESC 例 2 照樣綠。真正有鑑別力的是「小額在前」的順序，本測試補上。
func TestAllocateLargestRemainder_WeightDescTiebreakIsLoadBearing(t *testing.T) {
	// 餘數同為 1000，金額 750 / 1250。weights DESC ⇒ 大的先拿 ⇒ [112, 188]。
	got, err := AllocateLargestRemainder([]int64{750, 1250}, 300)
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != 112 || got[1] != 188 {
		t.Fatalf("got %v, want [112 188] —— 若得到 [113 187] 表示排序鍵第二段（weights DESC）已失效", got)
	}
	// 對照組：同一組權重換成升冪輸入，兩者的「大額拿到 +1」必須一致。
	rev, err := AllocateLargestRemainder([]int64{1250, 750}, 300)
	if err != nil {
		t.Fatal(err)
	}
	if rev[0] != 188 || rev[1] != 112 {
		t.Fatalf("got %v, want [188 112]", rev)
	}
}
