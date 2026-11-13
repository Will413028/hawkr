package stripe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"
)

// clock 是測試開始時的真實時間：SDK 以系統時間驗時間戳，所以簽章用它起算，邊界測試離容許值留 10 秒餘裕。
var clock = time.Now()

func webhookClient(t *testing.T) *Client {
	t.Helper()
	c, err := New(Config{SecretKey: "sk_test_x", WebhookSecret: testSecret})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func sign(secret string, at time.Time, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(at.Unix(), 10) + "." + body))
	return hex.EncodeToString(mac.Sum(nil))
}

func header(at time.Time, sigs ...string) http.Header {
	v := fmt.Sprintf("t=%d", at.Unix())
	for _, s := range sigs {
		v += ",v1=" + s
	}
	return http.Header{"Stripe-Signature": {v + ",v0=legacy"}}
}

func eventBody(id, typ, relatedID, relatedType string) string {
	return fmt.Sprintf(`{"id":%q,"object":"v2.core.event","type":%q,"created":"2026-11-03T12:00:00.000Z","livemode":false,"context":null,"reason":null,"related_object":{"id":%q,"type":%q,"url":"/v2/core/accounts/%s"}}`, id, typ, relatedID, relatedType, relatedID)
}

func TestParseEventAcceptsPlatformAccountEvents(t *testing.T) {
	for _, typ := range []string{
		"v2.core.account.created",
		"v2.core.account[defaults].updated",
		"v2.core.account[identity].updated",
		"v2.core.account[configuration.recipient].updated",
		"v2.core.account[configuration.recipient].capability_status_updated",
	} {
		body := eventBody("evt_1", typ, "acct_1", "v2.core.account")
		ev, err := webhookClient(t).ParseEvent(header(clock, sign(testSecret, clock, body)), []byte(body))
		if err != nil || ev.ID != "evt_1" || ev.AccountID != "acct_1" {
			t.Errorf("%s：%+v %v", typ, ev, err)
		}
	}
}

func TestParseEventRejectsBadSignatures(t *testing.T) {
	body := eventBody("evt_1", "v2.core.account.created", "acct_1", "v2.core.account")
	good := sign(testSecret, clock, body)
	for name, h := range map[string]http.Header{
		"沒有標頭":      {},
		"簽章是別的密鑰":   header(clock, sign("whsec_other", clock, body)),
		"簽章對別的本文":   header(clock, sign(testSecret, clock, body+" ")),
		"沒有 v1":     {"Stripe-Signature": {fmt.Sprintf("t=%d,v0=%s", clock.Unix(), good)}},
		"沒有 t":      {"Stripe-Signature": {"v1=" + good}},
		"t 不是數字":    {"Stripe-Signature": {"t=abc,v1=" + good}},
		"v1 不是 hex": header(clock, "zz"),
		"本文空白":      header(clock, sign(testSecret, clock, "")),
		"時間戳太舊":     header(clock.Add(-SignatureTolerance-10*time.Second), sign(testSecret, clock.Add(-SignatureTolerance-10*time.Second), body)),
	} {
		if _, err := webhookClient(t).ParseEvent(h, []byte(body)); !errors.Is(err, ErrSignature) {
			t.Errorf("%s：err=%v", name, err)
		}
	}
}

func TestParseEventToleranceBoundaryAndMultipleV1(t *testing.T) {
	body := eventBody("evt_1", "v2.core.account.created", "acct_1", "v2.core.account")
	inside := clock.Add(-SignatureTolerance + 10*time.Second)
	if _, err := webhookClient(t).ParseEvent(header(inside, sign(testSecret, inside, body)), []byte(body)); err != nil {
		t.Errorf("容許範圍內：%v", err)
	}
	h := header(clock, sign("whsec_rotated_out", clock, body), sign(testSecret, clock, body))
	if _, err := webhookClient(t).ParseEvent(h, []byte(body)); err != nil {
		t.Errorf("多個 v1 其中一個相符：%v", err)
	}
}

func TestParseEventIgnoresOtherTypesOnlyAfterTheSignatureChecks(t *testing.T) {
	for name, body := range map[string]string{
		"連線帳戶的 v1 事件":         eventBody("evt_1", "v1.account.updated", "acct_1", "v1.account"),
		"v1 capability":       eventBody("evt_1", "v1.capability.updated", "acct_1", "v1.account"),
		"person 事件":           eventBody("evt_1", "v2.core.account_person.created", "person_1", "v2.core.account_person"),
		"person 事件但相關物件宣稱是帳戶": eventBody("evt_1", "v2.core.account_person.created", "acct_1", "v2.core.account"),
		"帳戶型別的事件但相關物件不是帳戶":    eventBody("evt_1", "v2.core.account.created", "x_1", "v2.core.other"),
		"其他 v2 事件":            eventBody("evt_1", "v2.core.event_destination.ping", "acct_1", "v2.core.account"),
		"型別只是前綴相同":            eventBody("evt_1", "v2.core.accounts.created", "acct_1", "v2.core.account"),
	} {
		_, err := webhookClient(t).ParseEvent(header(clock, sign(testSecret, clock, body)), []byte(body))
		if !errors.Is(err, ErrNotHandled) {
			t.Errorf("%s：err=%v", name, err)
		}
		if _, err := webhookClient(t).ParseEvent(header(clock, sign("whsec_other", clock, body)), []byte(body)); !errors.Is(err, ErrSignature) {
			t.Errorf("%s：簽章不符的不處理事件應回 ErrSignature：%v", name, err)
		}
	}
}

func TestParseEventRejectsSignedButMalformedBodies(t *testing.T) {
	for name, body := range map[string]string{
		"不是 JSON":              `not json`,
		"沒有 id":                `{"type":"v2.core.account.created","related_object":{"id":"acct_1","type":"v2.core.account"}}`,
		"沒有 type":              `{"id":"evt_1","related_object":{"id":"acct_1","type":"v2.core.account"}}`,
		"沒有 related_object.id": `{"id":"evt_1","type":"v2.core.account.created","related_object":{"type":"v2.core.account"}}`,
	} {
		_, err := webhookClient(t).ParseEvent(header(clock, sign(testSecret, clock, body)), []byte(body))
		if !errors.Is(err, ErrMalformed) {
			t.Errorf("%s：err=%v", name, err)
		}
	}
}

func TestParseEventSnapshotEventsAreNotHandled(t *testing.T) {
	body := `{"id":"evt_1","object":"event","type":"account.updated","data":{"object":{"id":"acct_1"}}}`
	_, err := webhookClient(t).ParseEvent(header(clock, sign(testSecret, clock, body)), []byte(body))
	if !errors.Is(err, ErrNotHandled) {
		t.Errorf("err=%v", err)
	}
	if _, err := webhookClient(t).ParseEvent(header(clock, sign("whsec_other", clock, body)), []byte(body)); !errors.Is(err, ErrSignature) {
		t.Errorf("簽章不符：%v", err)
	}
}

// 簽章正確但 SDK 的型別化結構解不了的事件，照舊分類：不是帳戶事件的 → ErrNotHandled；帳戶事件多了未知形狀的欄位 → 照常處理。只有簽章走 SDK。
// mutation：改回以 SDK 的 ParseEventNotification 解析 → 兩個案例回 ErrMalformed，失敗。
func TestParseEventLenientParseAfterSignatureForShapesTheSDKStructCannotDecode(t *testing.T) {
	other := `{"id":"evt_1","object":"v2.core.event","type":"v2.billing.meter.error_report_triggered","created":123,"related_object":{"id":"mtr_1","type":"v2.billing.meter","url":7},"data":{"x":[1,2]}}`
	_, err := webhookClient(t).ParseEvent(header(clock, sign(testSecret, clock, other)), []byte(other))
	if !errors.Is(err, ErrNotHandled) {
		t.Errorf("其他物件的事件：err=%v", err)
	}
	extra := `{"id":"evt_2","object":"v2.core.event","type":"v2.core.account.updated","created":{"unexpected":true},"livemode":"maybe","context":{"a":1},"reason":[1],"data":42,"related_object":{"id":"acct_1","type":"v2.core.account","url":9,"extra":{"k":1}}}`
	ev, err := webhookClient(t).ParseEvent(header(clock, sign(testSecret, clock, extra)), []byte(extra))
	if err != nil || ev.ID != "evt_2" || ev.AccountID != "acct_1" {
		t.Errorf("帳戶事件多了未知欄位：%+v %v", ev, err)
	}
}
