package usecase

import "testing"

// 遮罩規則的 SSOT 是 58 號 §8.1.3(d) 的兩張表。本檔逐格釘住，因為遮罩錯的兩個方向
// **都不會有任何東西報錯**：遮過頭 ⇒ 功能消失（買家核對不了統編）、露太多 ⇒ 個資外洩。

func TestMaskEmail(t *testing.T) {
	cases := []struct{ name, in, want string }{
		// §8.1.3(d) 表格的兩個逐字範例。
		{"設計書範例：local part ≥ 4", "acmestore@example.com", "acme***@example.com"},
		{"設計書範例：local part < 4 只留 1 碼", "ab@example.com", "a***@example.com"},
		{"剛好 4 碼", "acme@example.com", "acme***@example.com"},
		// 🔴 遮罩字元固定 3 個 `*`，不隨長度變化：跟著長度走會洩漏 local part 長度，
		// 而 local part 本身就是可辨識資訊（§8.1.3(d) 末段）。
		{"很長的 local part 仍是 3 個星號", "averyveryverylongname@example.com", "aver***@example.com"},
		{"空字串維持空字串（沒有值就沒有東西要遮）", "", ""},
		{"不是 email 形狀 ⇒ 全遮，不得原樣回傳", "not-an-email", maskFill},
		{"只有 @ 沒有 domain ⇒ 全遮", "abc@", maskFill},
		{"@ 開頭沒有 local part ⇒ 全遮", "@example.com", maskFill},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MaskEmail(tc.in); got != tc.want {
				t.Fatalf("MaskEmail(%q) = %q，預期 %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestMaskPhone(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"設計書範例：保留前 2 後 4", "0912341234", "09••••1234"},
		{"空字串維持空字串", "", ""},
		{"短於 7 碼全遮（前 2 後 4 會露出 6 碼中的 6 碼＝沒遮）", "091234", maskFill},
		{"剛好 7 碼：中間只剩 1 個遮罩字元", "0912345", "09•2345"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MaskPhone(tc.in); got != tc.want {
				t.Fatalf("MaskPhone(%q) = %q，預期 %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestMaskCarrierID(t *testing.T) {
	cases := []struct{ name, carrierType, in, want string }{
		// §8.1.3(d) 表格的兩個逐字範例。遮罩字元數 = len-4，刻意隨長度變化。
		{"手機條碼 3J0002（8 碼）", amegoCarrierMobileBarcode, "/T7R3U+P", "/T••••+P"},
		{"自然人憑證 CQ0001（16 碼）", amegoCarrierCitizenCert, "AB12345678901234", "AB••••••••••••34"},
		// 🔴 這一格是 §8.6 T-09 的反面：靠長度猜型別的實作會把 16 碼當成手機條碼處理。
		// 兩個 case 的長度不同而規則相同，正是因為型別由第一個參數決定、不是猜的。
		{"未知型別一律全遮，不得回顯任何一碼", "UNKNOWN", "/T7R3U+P", maskFill},
		{"空 carrier_type（打統編／捐贈）也全遮", "", "12345678", maskFill},
		{"已知型別但值過短 ⇒ 全遮", amegoCarrierMobileBarcode, "/T7R", maskFill},
		{"空值維持空值", amegoCarrierMobileBarcode, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MaskCarrierID(tc.carrierType, tc.in); got != tc.want {
				t.Fatalf("MaskCarrierID(%q, %q) = %q，預期 %q", tc.carrierType, tc.in, got, tc.want)
			}
		})
	}
}

// TestMaskCarrierID_NeverLeaksMoreThanFourRunes 是規則本身的不變式，不是逐格對照：
// 任何已知型別的輸出，明碼部分恆為 4 個字元（前 2 ＋ 後 2）。
// 新增載具型別時若寫錯 slice 邊界，本支會抓到，而逐格表格不會（沒有那一列）。
func TestMaskCarrierID_NeverLeaksMoreThanFourRunes(t *testing.T) {
	for _, ct := range []string{amegoCarrierMobileBarcode, amegoCarrierCitizenCert} {
		for _, v := range []string{"/T7R3U+P", "AB12345678901234", "ABCDEF", "ABCDEFGHIJKLMNOPQRST"} {
			got := MaskCarrierID(ct, v)
			if got == maskFill {
				continue
			}
			plain := 0
			for _, r := range got {
				if r != '•' {
					plain++
				}
			}
			if plain != 4 {
				t.Fatalf("MaskCarrierID(%q, %q) = %q 露出 %d 個明碼字元，規則是 4（前 2 ＋ 後 2）", ct, v, got, plain)
			}
		}
	}
}

func TestMaskDisplayName(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		deleted bool
		want    string
	}{
		{"中文暱稱", "陳小明", false, "陳•••"},
		{"英文暱稱", "alexis", false, "a•••"},
		{"兩個字", "阿明", false, "阿•••"},
		{"單一字元原樣", "陳", false, "陳"},
		{"空字串原樣", "", false, ""},
		// 🔴 墓碑不是個資，遮了會變成讀不懂的字串，而它存在的唯一理由就是要讓人讀懂。
		{"墓碑原樣", "已註銷的買家", true, "已註銷的買家"},
		// 長度固定 3 個 •，不隨輸入長度變化：暱稱長度沒有業務用途，
		// 露出長度只是白給一個比對維度。
		{"長暱稱也只有三個點", "abcdefghijklmnop", false, "a•••"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MaskDisplayName(tc.in, tc.deleted); got != tc.want {
				t.Fatalf("MaskDisplayName(%q, %v) = %q, want %q", tc.in, tc.deleted, got, tc.want)
			}
		})
	}
}
