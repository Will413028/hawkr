package middleware

import "testing"

// 58 §8.1.2 層 2：AMEGO AppKey 與買家載具號碼的 access log 遮罩。
//
// 兩個值的判準不同但結論相同（§10.3）：
//   - appKey 是**寫入類憑證**——持有統編＋AppKey 即可以該賣家名義開立、作廢、折讓
//     真實電子發票。問的是「拿到它能做什麼」，不是「它含不含個資」。
//   - carrierId 是**買受人識別資訊**（§8.1.3 B 類），同時是財政部端的查詢鍵：拿到它
//     可以跨平台查該載具下的全部發票與中獎狀態。
//
// ⚠️ **這一支要測的是 forced-redact，不是「有沒有被遮」。** #2506 之後 body 與 query
// 都是 default-deny（`shouldRedact` ＝ 不在 `allowedKeys` 就遮），所以「送一段 JSON 進
// redactBody 看它被遮掉」對本次新增的兩條規則**沒有任何鑑別力**——把 sensitiveKeys 的
// 那兩行整個刪掉，那種測試照樣全綠。真正被 §8.1.2 層 2 買到的性質只有一個：
// forced-redact 勝過 allowedKeys 與 diagnostic escape hatch，而那要用 shouldRedact
// 的第三個參數才驗得出來。
func TestInvoiceCredentials_ForcedRedactBeatsAllowlist(t *testing.T) {
	// 模擬「有人日後把它加進 allowlist」與「診斷模式臨時放行」兩種情形。
	escapeHatch := map[string]struct{}{"appkey": {}, "carrierid": {}}
	for _, k := range []string{"appkey", "carrierid"} {
		t.Run(k, func(t *testing.T) {
			if !isSensitiveKey(k) {
				t.Fatalf("%q 必須在 forced-redact 清單內", k)
			}
			if !shouldRedact(k) {
				t.Fatalf("%q 必須被遮", k)
			}
			if !shouldRedact(k, escapeHatch) {
				t.Fatalf("%q 被 escape hatch 放行了——forced-redact 沒有勝過 allowlist", k)
			}
		})
	}
}

// 反向斷言：`apikey` 這條既有規則涵蓋不到 `appKey`。
//
// 這是「為什麼要新增一條而不是沿用既有那條」的機械證據。normalizeKey 去底線與
// 大小寫之後 appKey → "appkey"，與既有的 "apikey" 是兩個不同的鍵。
func TestAppKeyIsNotCoveredByApiKey(t *testing.T) {
	if normalizeKey("appKey") == normalizeKey("apiKey") {
		t.Fatal("appKey 與 apiKey 若正規化後相等，本測試的前提就不成立了")
	}
	if isSensitiveKey("apikey") && !isSensitiveKey("appkey") {
		t.Fatal(`"apikey" 在清單內但 "appkey" 不在——這正是設計書指出的涵蓋不到`)
	}
}

// 每條進入路徑都要遮（§10.3）：body 與 query 走同一個 shouldRedact。
// 這一支的鑑別力來自「兩條路徑用同一份清單」而不是「值被遮掉了」——後者由
// default-deny 保證，前者才是 §10.3 那次事故（guestId 在 body 有遮、query 沒遮）
// 要防的東西。
func TestInvoiceCredentials_BodyAndQueryUseTheSameDecision(t *testing.T) {
	const R = `"[REDACTED]"`
	bodyCases := []struct{ in, want string }{
		{`{"appKey":"AbCdEfGhIjKlMnOpQrSt"}`, `{"appKey":` + R + `}`},
		{`{"app_key":"AbCdEfGhIjKlMnOpQrSt"}`, `{"app_key":` + R + `}`},
		{`{"carrierId":"/ABC+123"}`, `{"carrierId":` + R + `}`},
		{`{"credential":{"appKey":"AbCdEfGhIjKlMnOpQrSt"}}`, `{"credential":{"appKey":` + R + `}}`},
	}
	for _, c := range bodyCases {
		if got := redactBody([]byte(c.in)); got != c.want {
			t.Fatalf("redactBody(%s)\n  got  %s\n  want %s", c.in, got, c.want)
		}
	}
	queryCases := []struct{ in, want string }{
		{"carrier_id=%2FABC%2B123", "carrier_id=" + R},
		{"carrierId=%2FABC%2B123", "carrierId=" + R},
		{"appKey=AbCdEfGhIjKlMnOpQrSt", "appKey=" + R},
		{"page=1&carrier_id=x&size=20", "page=1&carrier_id=" + R + "&size=20"},
	}
	for _, c := range queryCases {
		if got := redactQuery(c.in); got != c.want {
			t.Fatalf("redactQuery(%q)\n  got  %s\n  want %s", c.in, got, c.want)
		}
	}
}

// 反向斷言：不得過度遮罩。發票線的除錯要追的是發票號碼、單號與 document id，
// 三者都不是憑證也不是 PII。它們今天落在 default-deny 的「遮」那一側是既有行為，
// 本測試只斷言它們**不在 forced-redact 清單**——那才是本次改動的邊界。
func TestInvoiceBusinessKeysNotForcedRedacted(t *testing.T) {
	for _, k := range []string{"invoiceno", "documentid", "orderno", "merchantordernumber"} {
		if isSensitiveKey(k) {
			t.Fatalf("%q 不該進 forced-redact 清單（它不是憑證也不是 PII）", k)
		}
	}
}
