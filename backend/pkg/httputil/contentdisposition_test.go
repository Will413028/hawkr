package httputil

import "testing"

// 期望值一律硬寫字面值——不呼叫 rfc5987Encode／asciiFallbackName 求期望，
// 否則那兩支寫錯時測試會跟著錯。
func TestContentDisposition(t *testing.T) {
	cases := []struct {
		name        string
		disposition string
		fileName    string
		fallback    string
		want        string
	}{
		{
			// 5 個中文字 = 15 bytes ⇒ 15 個底線（按「字數」算會寫成 9 個，那是錯的）
			name:        "純中文：filename 退化，真正的名字在 filename*",
			disposition: "attachment", fileName: "作廢申請書.pdf", fallback: "invoice.pdf",
			want: `attachment; filename="_______________.pdf"; filename*=UTF-8''%E4%BD%9C%E5%BB%A2%E7%94%B3%E8%AB%8B%E6%9B%B8.pdf`,
		},
		{
			name:        "純 ASCII：兩個參數同值",
			disposition: "attachment", fileName: "invoice-AA10003101.pdf", fallback: "invoice.pdf",
			want: `attachment; filename="invoice-AA10003101.pdf"; filename*=UTF-8''invoice-AA10003101.pdf`,
		},
		{
			name:        "中英混合",
			disposition: "attachment", fileName: "INV58R6-證明.pdf", fallback: "invoice.pdf",
			want: `attachment; filename="INV58R6-______.pdf"; filename*=UTF-8''INV58R6-%E8%AD%89%E6%98%8E.pdf`,
		},
		{
			// 空格是 ASCII 0x20 ⇒ 不換底線；quoted-string 內的空格本來就合法
			name:        "空格保留（quoted-string 內合法）",
			disposition: "attachment", fileName: "my file.csv", fallback: "download.csv",
			want: `attachment; filename="my file.csv"; filename*=UTF-8''my%20file.csv`,
		},
		{
			// 有副檔名時**不**走 fallback：`______.pdf` 雖然名字沒了，至少還告訴舊 client
			// 這是個 PDF，比換成一個與內容無關的預設名有用。
			name:        "純中文但有副檔名：保留副檔名，不走 fallback",
			disposition: "inline", fileName: "報表.pdf", fallback: "report.pdf",
			want: `inline; filename="______.pdf"; filename*=UTF-8''%E5%A0%B1%E8%A1%A8.pdf`,
		},
		{
			// 連副檔名都沒有 ⇒ 退化名整串都是底線，一點資訊都不剩 ⇒ 這時才用 fallback
			name:        "純中文且無副檔名：退化名毫無資訊，改用 fallback",
			disposition: "attachment", fileName: "報表", fallback: "report.pdf",
			want: `attachment; filename="report.pdf"; filename*=UTF-8''%E5%A0%B1%E8%A1%A8`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ContentDisposition(tc.disposition, tc.fileName, tc.fallback); got != tc.want {
				t.Errorf("ContentDisposition()\n got = %s\nwant = %s", got, tc.want)
			}
		})
	}
}

// 釘住「不能用 url.PathEscape 代替」那句註解：PathEscape 放行 , : ; = ? @，
// 而它們在 RFC 5987 的 attr-char 之外。
func TestRFC5987EncodeRejectsNonAttrChar(t *testing.T) {
	if got := rfc5987Encode("aZ0!#$&+-.^_`|~"); got != "aZ0!#$&+-.^_`|~" {
		t.Errorf("attr-char 不該被編碼, got = %s", got)
	}
	for in, want := range map[string]string{
		",": "%2C", ":": "%3A", ";": "%3B", "=": "%3D", "?": "%3F", "@": "%40",
		" ": "%20", "*": "%2A", "'": "%27", "(": "%28",
	} {
		if got := rfc5987Encode(in); got != want {
			t.Errorf("rfc5987Encode(%q) = %s, want %s", in, got, want)
		}
	}
}
