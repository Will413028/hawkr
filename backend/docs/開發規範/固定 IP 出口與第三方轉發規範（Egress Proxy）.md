# 固定 IP 出口與第三方轉發規範（Egress Proxy）

> 制定：2026-07-22（三竹 SMS 導入案定案，#1354／#1728）。本檔為最終權威。
> 適用對象：所有需要對接「來源 IP 白名單制」第三方 API 的後端開發。

## 1. 背景與問題

部分第三方（簡訊商、政府資料源、金流特殊介接等）要求呼叫來源 IP 事先登記白名單。本平台主要基礎設施在 Alibaba Cloud，各環境（local／dev／prod）出口 IP 不同且可能變動，逐環境登記白名單不可維護。

解法：**單一固定出口層 `egress-proxy`**——所有環境對這類第三方的呼叫，一律經由 GCP Cloud Run 上的轉發層出去，第三方只看到一顆固定 IP，白名單只登記一次、永不因環境增減而變動。

## 2. 現行拓撲（SSOT）

| 項 | 值 |
|---|---|
| 服務 | Cloud Run `egress-proxy`（nginx 純轉發） |
| GCP 專案／region | `example-seller-prod`／`asia-east1` |
| 網路 | 掛 `tgos-vpc` + Cloud NAT，`--vpc-egress=all-traffic` |
| 固定出口 IP | `203.0.113.30`（第三方白名單登記這顆） |
| 認證 | IAM-only（`--no-allow-unauthenticated`）；呼叫端以 SA `egress-proxy-caller` 簽 Google ID token |
| SA key 掛載 | k8s secret `egress-proxy-caller-credentials` → `/secrets/egress-proxy-caller/key.json` |
| 代碼位置 | `serverless/egress-proxy/`（Dockerfile／nginx.conf／deploy.sh／README） |
| 已收編路徑 | `/api/mtk/SmSend`、`/api/mtk/SmQuery`（三竹簡訊）；`/addrws/v40/QueryAddr.asmx/QueryAddr`（TGOS 地址驗證） |

> **TGOS 收編已完成（2026-07-28）**：`tgos-proxy`（地址驗證）原為獨立服務，已收編進 egress-proxy 並退役——`serverless/tgos-proxy/` 已從版控移除（源碼存證於 git 歷史，退役 commit `3f47cf908`），buyer-service／seller-service 的 `TGOS_BASE_URL` 皆已指向 egress-proxy。呼叫端 SA 仍沿用 `tgos-proxy-caller`（由 egress-proxy 追授 invoker，故 code 與 secret 皆零改動），secret `tgos-proxy-caller-credentials` 續用。egress-proxy 是 TGOS 的唯一入口。

## 3. 新第三方接入流程（強制）

未來任何需要固定來源 IP 的第三方，**一律收斂到 egress-proxy**，流程：

1. **設計書先行**：對應功能設計書載明第三方、白名單需求、收編路徑前綴。
2. **nginx 白名單**：`serverless/egress-proxy/nginx.conf` 加該第三方的**精確 `location` 路徑**（`location = /<path>`），其餘一律 404——禁止 wildcard／open relay。
3. **部署**：走 CI——`.github/workflows/deploy-egress-proxy.yml`（push `main` 的 path filter ＋ `workflow_dispatch`），該 workflow 重用 `serverless/egress-proxy/deploy.sh` 為單一真相。**禁止本機跑 `deploy.sh` 直上正式環境**（見 [Cloudflare Worker 與 Serverless 部署治理規範](./Cloudflare%20Worker%20與%20Serverless%20部署治理規範.md)）。
4. **呼叫端**：Go 服務仿 `pkg/sms`——`google.golang.org/api/idtoken` 簽 token（audience = proxy URL）＋既有 SA key；**必帶斷路器**（gobreaker，參數對齊 `pkg/sms`：連 5 敗開路／10s 冷卻／半開 3 探測）＋失敗路徑明確業務錯誤碼。
5. **第三方白名單**：登記 `203.0.113.30`（若第三方已有此 IP 則免）。

## 4. 鐵則

- **proxy 只做轉發**：禁止在 proxy 放業務邏輯、重試、憑證——第三方帳密由呼叫端於請求帶入，proxy 不持有任何 secret。
- **憑證不落地**：access log 只記 `$uri` 不含 query args（帳密常在 query）。
- **禁止**為單一第三方另開新 proxy／新 VPC／新固定 IP（成本與白名單碎片化；例外需明確裁決）。
- **禁止**繞過 proxy 從服務直連白名單制第三方。例外：本地開發可用「白名單直連模式」（如 `pkg/sms` direct mode：SA key 留空＋BASE_URL 填第三方域名），前提是開發機 IP 已在該第三方白名單。

## 5. 可用性與成本定位

- **單 region 託管**：Cloud Run 自動健康檢查／實例重啟，`min-instances=1` 防冷啟動；region 級故障時 proxy 不可用——由**呼叫端斷路器**快速熔斷、回明確錯誤，服務本體不受拖累。不為 proxy 上多 region（成本翻倍、OTP 類場景不需要）。
- **成本**：nginx alpine 最小檔位，min-instance 閒置費約 US$3-5/月；VPC／NAT／IP 沿用既有，無增量。

## 6. 相關文件

- `serverless/egress-proxy/README.md`——部署與維運操作面
