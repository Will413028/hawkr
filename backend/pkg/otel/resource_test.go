package otel

import (
	"os"
	"testing"
)

// TestInstanceID_PrefersPodNameOverHostname 釘住取值順序。
//
// POD_NAME 的存在理由是「hostname 語意改變時可以不動碼修正」，那個理由只有在它**優先於**
// hostname 時才成立。順序寫反的話兩者在 k8s 裡碰巧會回同一個值（pod hostname 預設等於 pod
// name），於是這個缺陷在本叢集完全無聲——正是需要被釘住的形狀。
func TestInstanceID_PrefersPodNameOverHostname(t *testing.T) {
	host, err := os.Hostname()
	if err != nil {
		t.Skipf("os.Hostname unavailable: %v", err)
	}
	const override = "account-service-7bdf7b65ff-fw45w"
	if override == host {
		t.Skip("本機 hostname 恰好等於測試用的覆寫值，此案無鑑別力")
	}

	t.Setenv("POD_NAME", override)
	if got := instanceID(); got != override {
		t.Errorf("instanceID() = %q, want %q（POD_NAME 必須優先於 hostname）", got, override)
	}
}

// TestInstanceID_FallsBackToHostname 釘住沒有 POD_NAME 時仍取得到值。
// 本叢集刻意不改 chart 注入 POD_NAME（那會讓 23 個服務全部重啟），所以 production 實際
// 走的就是這條 fallback——它壞掉等於 instance label 整批消失。
func TestInstanceID_FallsBackToHostname(t *testing.T) {
	host, err := os.Hostname()
	if err != nil {
		t.Skipf("os.Hostname unavailable: %v", err)
	}
	t.Setenv("POD_NAME", "")
	if got := instanceID(); got != host {
		t.Errorf("instanceID() = %q, want hostname %q", got, host)
	}
}

// TestNewServiceResource_CarriesInstanceID 釘住屬性真的被掛進 resource。
//
// 用 semconv 的正式 key（"service.instance.id"）而不是字面字串比對——這個 key 是
// prometheusremotewrite 唯一會轉成 `instance` label 的來源，拼錯的話屬性仍在、
// label 卻永遠不會出現，而那是一個測試很容易放過的形狀。
func TestNewServiceResource_CarriesInstanceID(t *testing.T) {
	t.Setenv("POD_NAME", "seller-service-abc123")
	res := newServiceResource("seller-service")

	var got string
	for _, kv := range res.Attributes() {
		if string(kv.Key) == "service.instance.id" {
			got = kv.Value.AsString()
		}
	}
	if got != "seller-service-abc123" {
		t.Errorf("resource 的 service.instance.id = %q, want %q；"+
			"少了它，同一個服務的每個 pod 在 Prometheus 裡會塌成同一條 series", got, "seller-service-abc123")
	}
}
