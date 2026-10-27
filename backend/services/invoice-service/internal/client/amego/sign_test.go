package amego

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

// officialAppKey 是光貿公開文件的測試 App Key。
const officialAppKey = "sHeq7t8G1wiQvhAuIM27" // gitleaks:allow（光貿公開文件的測試 App Key）

// TestSignatureOfficialGoldenFixtures 用光貿官方文件的三組 fixture 當 golden。
// 其中兩組的統編已換成合成統編，期望簽章以文件規定的 MD5(data+time+AppKey) 另行重算；
// 含 `+` 的第二組維持文件原值。
//
// 這三組是「data 原樣字串 + time + AppKey」的完整輸入，我方算出的 sign 必須逐字相同。
// 第二組 `/TRM+O+P` 特別重要：它含 `+`，證明簽章吃的是**未編碼**的原字串，而 form
// 必須把它編成 %2B（自己拼字串會讓 `+` 被解成空白）。
func TestSignatureOfficialGoldenFixtures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		data string
		unix int64
		want string
	}{
		{"ban_query 陣列", `[{"ban":"45444444"}]`, 1787577174, "418571bfda957d168bffeb2a728f591a"},
		{"barcode 含加號", `{"barCode":"/TRM+O+P"}`, 1787577174, "5a928f1c56c093d3bf1a19544baeb6e5"},
		{"invoice_query 物件", `{"OrderId":"A20200817105934","BuyerIdentifier":"45444444"}`, 1628136135, "e69e42249735e2e07d76a952c33ccea4"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := signature(tc.data, tc.unix, officialAppKey); got != tc.want {
				t.Fatalf("signature = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestSignatureIsLowercaseHex：大寫 hex 會被 AMEGO 拒（code 16）。
func TestSignatureIsLowercaseHex(t *testing.T) {
	t.Parallel()
	got := signature(`{"a":"B"}`, 1, officialAppKey)
	if got != strings.ToLower(got) {
		t.Fatalf("signature 必須是小寫 hex，得到 %q", got)
	}
	if len(got) != 32 {
		t.Fatalf("signature 必須 32 碼，得到 %d", len(got))
	}
}

// TestEncodeOnceMatchesOfficialGoldenPayload 證明我方的 struct 序列化結果，
// 逐字等於官方 fixture 的 data 字串 ⇒ golden sign 可以直接套在真實 payload 上。
func TestEncodeOnceMatchesOfficialGoldenPayload(t *testing.T) {
	t.Parallel()
	payload := struct {
		OrderID         string `json:"OrderId"`
		BuyerIdentifier string `json:"BuyerIdentifier"`
	}{OrderID: "A20200817105934", BuyerIdentifier: "45444444"}
	got, err := encodeOnce(payload)
	if err != nil {
		t.Fatalf("encodeOnce: %v", err)
	}
	const want = `{"OrderId":"A20200817105934","BuyerIdentifier":"45444444"}`
	if got != want {
		t.Fatalf("encodeOnce = %q, want %q", got, want)
	}
	if s := signature(got, 1628136135, officialAppKey); s != "e69e42249735e2e07d76a952c33ccea4" {
		t.Fatalf("golden sign 對不上: %q", s)
	}
}

// TestEncodeOnceNoTrailingNewline：json.Encoder 會補一個換行，那個換行不屬於 data。
// 沒 trim 的話 sign 與 form 都會多一個位元組（AMEGO 回 code 16，訊息不提示原因）。
func TestEncodeOnceNoTrailingNewline(t *testing.T) {
	t.Parallel()
	got, err := encodeOnce(map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("encodeOnce: %v", err)
	}
	if strings.HasSuffix(got, "\n") {
		t.Fatalf("encodeOnce 不得留下尾端換行: %q", got)
	}
}

// TestEncodeOnceDiffersFromStdMarshalOnHTMLChars 是「序列化一次」這條鐵則的鑑別力來源。
//
// 它證明「再 marshal 一次」**真的會產出不同的位元組**：encoding/json 的 Marshal
// 預設轉義 < > &，而 encodeOnce 關掉了轉義。若沒有這個差異，任何「分兩次序列化」
// 的變異都不會被任何測試抓到。
func TestEncodeOnceDiffersFromStdMarshalOnHTMLChars(t *testing.T) {
	t.Parallel()
	payload := map[string]string{"OrderId": "A&B<C>D"}
	once, err := encodeOnce(payload)
	if err != nil {
		t.Fatalf("encodeOnce: %v", err)
	}
	twice, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if once == string(twice) {
		t.Fatal("encodeOnce 與 json.Marshal 在含 & < > 的 payload 上必須不同，" +
			"否則『序列化一次』的變異測試沒有鑑別力")
	}
	if signature(once, 1, officialAppKey) == signature(string(twice), 1, officialAppKey) {
		t.Fatal("兩種序列化的 sign 必須不同")
	}
}

// TestBuildFormStandardSingleEncoding：`+` 必須編成 %2B。
// 自己拼字串把 `+` 直接塞進 body 會被伺服器解成空白 ⇒ sign 必然對不上。
func TestBuildFormStandardSingleEncoding(t *testing.T) {
	t.Parallel()
	data := `{"barCode":"/TRM+O+P"}`
	form := buildForm("12345678", data, 1787577174, "deadbeef")
	encoded := form.Encode()
	if !strings.Contains(encoded, "%2B") {
		t.Fatalf("form 編碼必須把 + 編成 %%2B: %s", encoded)
	}
	parsed, err := url.ParseQuery(encoded)
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}
	if parsed.Get("data") != data {
		t.Fatalf("解碼後的 data = %q, want %q", parsed.Get("data"), data)
	}
	if parsed.Get("time") != "1787577174" {
		t.Fatalf("time = %q", parsed.Get("time"))
	}
	if parsed.Get("invoice") != "12345678" {
		t.Fatalf("invoice = %q", parsed.Get("invoice"))
	}
}

// TestMaskSignKeepsEightChars：全長 sign ＋ 已落庫的 data／time ＝ 離線暴力破解
// AppKey 的 oracle ⇒ 稽核只留前 8 碼。
func TestMaskSignKeepsEightChars(t *testing.T) {
	t.Parallel()
	full := "418571bfda957d168bffeb2a728f591a"
	if got := maskSign(full); got != "418571bf" {
		t.Fatalf("maskSign = %q, want 418571bf", got)
	}
	if got := maskSign("abc"); got != "abc" {
		t.Fatalf("短字串應原樣回傳, 得到 %q", got)
	}
}
