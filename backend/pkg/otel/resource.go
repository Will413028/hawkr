package otel

import (
	"context"
	"os"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func newServiceResource(serviceName string) *resource.Resource {
	env := os.Getenv("ENV")
	if env == "" {
		env = "development"
	}

	attrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String(serviceName),
		attribute.String("deployment.environment", env),
	}

	// service.instance.id ⇒ Prometheus 的 `instance` label。
	//
	// 為什麼需要：`prometheusremotewrite` 對 OTLP 的 resource 只特別處理兩個屬性——
	// service.name → job、service.instance.id → instance（其餘由 target_info 承載，
	// 而本叢集的 collector 設定是 target_info.enabled=false）。少了 instance，**同一個
	// 服務的每個 pod 在 Prometheus 裡是同一條 series**：兩個各自單調的 counter 寫進同
	// 一個 identity，Prometheus 把下降讀成 counter reset，rate()/increase() 靜默灌水，
	// 而且不會有任何錯誤訊號。scrape 路徑沒有這個問題（Prometheus 自己補 instance/pod）。
	//
	// 目前 13 個 Go 服務在 dev/prod 都是 1 replica，所以這是潛伏而非現行故障——但
	// 擴容那天通常是被流量逼的，沒有餘裕先修 observability 再擴。
	//
	// 取值順序：POD_NAME（若 chart 有注入）→ os.Hostname()。k8s 的 pod hostname 預設
	// 等於 pod name，所以第二條在本叢集已經足夠，不必為此改 chart 讓 23 個服務全部重啟。
	// POD_NAME 留作覆寫：萬一未來 hostname 語意改變（hostNetwork、自訂 hostname/subdomain）
	// 可以不動碼修正。
	//
	// 取不到就**不設**這個屬性，而不是填一個假值——一個所有 pod 共用的常數比沒有更糟，
	// 它會讓 series 看起來已經分開了。
	if id := instanceID(); id != "" {
		attrs = append(attrs, semconv.ServiceInstanceIDKey.String(id))
	}

	res, _ := resource.New(context.Background(), resource.WithAttributes(attrs...))
	return res
}

// instanceID 回傳本 process 在同一個 service 內的唯一識別，取不到時回空字串。
func instanceID() string {
	if v := os.Getenv("POD_NAME"); v != "" {
		return v
	}
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return ""
}
