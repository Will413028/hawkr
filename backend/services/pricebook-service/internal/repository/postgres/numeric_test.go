package postgres

import (
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestNumericFromStringRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"整數", "0", "0"},
		{"價格四位小數", "46.2000", "46.2000"},
		{"零值保留四位小數", "0.0000", "0.0000"},
		{"負值", "-3.5", "-3.5"},
		{"volume_kg 上界", "99999999999.999", "99999999999.999"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := NumericFromString(tt.in)
			if err != nil {
				t.Fatalf("NumericFromString(%q) 回錯誤: %v", tt.in, err)
			}
			if !n.Valid {
				t.Fatalf("NumericFromString(%q) 產出 Valid=false，非 NULL 的值不應變成 NULL", tt.in)
			}
			got := NumericToPtr(n)
			if got == nil {
				t.Fatalf("NumericToPtr 對 %q 回 nil，非 NULL 的值不應變成 NULL", tt.in)
			}
			if *got != tt.want {
				t.Errorf("NumericToPtr = %q, want %q（尾隨零與精度必須逐字保留）", *got, tt.want)
			}
		})
	}
}

// TestNumericToPtrNull 釘住 NULL：pgtype.Numeric 零值必須回 nil，而不是指向 "0" 的指標。
// 0 是合法價格，把缺值渲染成 0 會讓 Task 9 的對帳把休市當成崩盤。
func TestNumericToPtrNull(t *testing.T) {
	var null pgtype.Numeric // Valid == false，即 SQL NULL
	if got := NumericToPtr(null); got != nil {
		t.Fatalf("NumericToPtr(NULL) = %q, want nil", *got)
	}
}

// TestNumericToPtrHandBuiltIsNotNull 釘住的是**既有行為，不是期望行為**：
// 手工組出來的 pgtype.Numeric{Valid: true, Int: nil} 不會回 nil，會回一個字串 ——
// pgx v5.9.2 回 "0"（numeric.go:267 的 nil 短路），v5.8.0 回 "<nil>"。
// 本服務兩種版本都會編到：go.work 的測試走 v5.9.2，Dockerfile 的 per-module build 走 v5.8.0。
//
// 記在這裡的原因是「手工值會靜默變成 NULL」這個猜測是錯的，而且錯得很容易再犯一次。
// 結論不變：不要手工組 pgtype.Numeric，一律用 NumericFromString 或 pgx 掃出來的值。
func TestNumericToPtrHandBuiltIsNotNull(t *testing.T) {
	handBuilt := pgtype.Numeric{Valid: true, Int: nil}
	got := NumericToPtr(handBuilt)
	if got == nil {
		t.Fatal("手工值回了 nil —— 行為變了，請重讀 numeric.go 的註解並更新它")
	}
	if *got != "0" && *got != "<nil>" {
		t.Errorf("手工值渲染成 %q，不是已知的 \"0\"(pgx v5.9.2) 或 \"<nil>\"(v5.8.0)", *got)
	}
}

// TestNumericNullIsNotZero 把 NULL 與 0 擺在一起比對，確保兩者永遠可區分。
func TestNumericNullIsNotZero(t *testing.T) {
	var null pgtype.Numeric
	zero, err := NumericFromString("0")
	if err != nil {
		t.Fatalf("NumericFromString(\"0\") 回錯誤: %v", err)
	}

	nullPtr := NumericToPtr(null)
	zeroPtr := NumericToPtr(zero)

	if nullPtr != nil {
		t.Errorf("NULL 應回 nil，實得 %q", *nullPtr)
	}
	if zeroPtr == nil {
		t.Fatal("0 應回非 nil 指標，實得 nil —— 0 被誤當成 NULL")
	}
	if *zeroPtr != "0" {
		t.Errorf("0 應回 %q，實得 %q", "0", *zeroPtr)
	}
}

// TestNumericPrecisionBeatsFloat64 舉證「不得經過 float64」不是教條：同一個值走 float64
// 會被截斷，走 pgtype.Numeric 不會。
func TestNumericPrecisionBeatsFloat64(t *testing.T) {
	const in = "12345678901234567890.12345678"

	f, err := strconv.ParseFloat(in, 64)
	if err != nil {
		t.Fatalf("ParseFloat(%q) 回錯誤: %v", in, err)
	}
	viaFloat := strconv.FormatFloat(f, 'f', -1, 64)
	if viaFloat == in {
		t.Fatalf("測資選錯了：%q 經過 float64 沒有失真，無法舉證精度問題", in)
	}

	n, err := NumericFromString(in)
	if err != nil {
		t.Fatalf("NumericFromString(%q) 回錯誤: %v", in, err)
	}
	got := NumericToPtr(n)
	if got == nil {
		t.Fatal("NumericToPtr 回 nil")
	}
	if *got != in {
		t.Errorf("NumericToPtr = %q, want %q（float64 路徑會變成 %q）", *got, in, viaFloat)
	}
}

// TestNumericFromStringRejectsNonFinite 是這組測試裡最要緊的一條：
// Postgres 的 numeric 收 NaN / Infinity / -Infinity，pgx 也會把這三個字面值掃成
// Valid=true，於是它們會像一筆正常價格那樣寫進 DB。NaN 在 Postgres 排序中大於一切
// 有限值 —— 放行的話 Task 9 的對帳會讀到一筆歷史新高，而不是一個錯誤。
// numeric.go 是 Task 4/5/6 寫入時共同的收斂點，擋在這裡最便宜。
func TestNumericFromStringRejectsNonFinite(t *testing.T) {
	for _, in := range []string{"NaN", "Infinity", "-Infinity"} {
		n, err := NumericFromString(in)
		if err == nil {
			t.Errorf("NumericFromString(%q) 應回錯誤，實得 nil —— 非有限值會被當成真實價格寫進 DB", in)
		}
		if n.Valid {
			t.Errorf("NumericFromString(%q) 應回 NULL（Valid=false），實得 Valid=true", in)
		}
		if n.NaN {
			t.Errorf("NumericFromString(%q) 回傳值不得帶 NaN 旗標", in)
		}
		if n.InfinityModifier != pgtype.Finite {
			t.Errorf("NumericFromString(%q) 回傳值不得帶 InfinityModifier，實得 %d", in, n.InfinityModifier)
		}
		if got := NumericToPtr(n); got != nil {
			t.Errorf("NumericFromString(%q) 出錯時 NumericToPtr 應回 nil，實得 %q", in, *got)
		}
	}
}

// TestNumericFromStringRejectsScientific 兌現檔頭 🔴 的承諾：
// ScanScientific 對含 e/E 的輸入會走 big.Float → float64 → FormatFloat，超過約 17 位
// 有效數字就失真。擋在入口，這份檔案裡才真的不存在 float64 路徑。
func TestNumericFromStringRejectsScientific(t *testing.T) {
	for _, in := range []string{"1.5e3", "1.5E3", "1e-5", "12345678901234567890.1e1"} {
		n, err := NumericFromString(in)
		if err == nil {
			t.Errorf("NumericFromString(%q) 應回錯誤（科學記號會經過 float64），實得 nil", in)
		}
		if n.Valid {
			t.Errorf("NumericFromString(%q) 應回 NULL（Valid=false），實得 Valid=true", in)
		}
	}
}

// TestNumericFromStringInvalid 確認壞輸入回錯誤，且回傳值是 NULL 而非 0：
// 呼叫端就算漏檢 error，也只會寫進 NULL，不會寫進一個看起來正常的 0。
func TestNumericFromStringInvalid(t *testing.T) {
	for _, in := range []string{"", " ", "abc", "1.2.3"} {
		n, err := NumericFromString(in)
		if err == nil {
			t.Errorf("NumericFromString(%q) 應回錯誤，實得 nil", in)
			continue
		}
		if n.Valid {
			t.Errorf("NumericFromString(%q) 出錯時應回 NULL（Valid=false），實得 Valid=true", in)
		}
		if got := NumericToPtr(n); got != nil {
			t.Errorf("NumericFromString(%q) 出錯時 NumericToPtr 應回 nil，實得 %q", in, *got)
		}
	}
}
