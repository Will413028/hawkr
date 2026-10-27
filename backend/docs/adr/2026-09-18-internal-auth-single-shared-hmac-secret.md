# 微服務間內部授權採單一共享 HMAC 密鑰

| 項目 | 值 |
|---|---|
| 狀態 | accepted；遷移計畫 A2 起：單體呼叫舊服務時照這份簽章（`internal/legacy/grpcclient`）；單體替舊服務提供的 legacy gRPC（account-service 的 AccountService、DelegationService）不驗簽章，因為它只存在於本機與 CI（gateway 與舊服務不會部署）。隨舊服務一起退場 |
| 日期 | 2026-09-18 |
| 影響範圍 | 全體 Java / Go 微服務的 gRPC 傳輸層信任；`k8s/external-services/networkpolicy.yaml` |

## 脈絡

微服務間 gRPC 的「內部身分」原先以明文 header `x-internal-service` 判定，**任意非空字串即被信任**，構成兩個裸面：

- **Java server**：`GrpcAuthServerInterceptor` 讀到非空 `x-internal-service` 即設 `INTERNAL_CALL=TRUE`，下游 `requireAccountType()` / `requireAccountId()` 全部跳過。
- **Go server**：完全沒有 auth interceptor，對到達其 gRPC port 的呼叫無條件信任。

同時 `deny-cross-namespace` 以 `podSelector: {}` 放行所有同 namespace pod 互通，微服務 gRPC port 未限定合法 caller。

爆炸半徑涵蓋全站訂單收件人姓名／電話／地址、買家跨商家消費彙總、出貨物流資訊、售後案件狀態——單一 rogue pod 或被入侵的第三方 sidecar 在同 namespace 內即可拉取全站訂單 PII。

Gateway 側不構成入口：Gateway 從零建 metadata、不轉發外部 `x-internal-service`，故外部使用者無法經 Gateway 注入 internal 身分；實際攻擊面是**繞過 Gateway、直連微服務 gRPC port 的同 namespace pod**。

這產生兩個**正交**的偽造面，因而需要兩軌解法：

| 偽造面 | 內容 |
|---|---|
| 內部身分偽造 | 任何連到目標 port 的 pod 帶 `x-internal-service: <任意值>` 即拿到 internal bypass |
| 使用者身分偽造 | 服務信任 Gateway 注入的 `x-account-id`／`x-account-type`；rogue pod 直接帶 `x-account-id: <victim>` 也被信任 |

不可動的約束：團隊沒有獨立 SRE／ops 角色，所有密鑰與網路策略的產生、分發與輪替由後端工程師兼任。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| **mTLS**（每服務憑證，傳輸層互相驗證） | 需要憑證簽發、分發、到期輪替與撤銷的完整生命週期管理；無 SRE 團隊時憑證過期即全站斷線 | 否決 |
| **per-service HMAC key**（每服務一把，可鑑別 caller 身分） | N 把密鑰的分發與輪替；每新增一個服務就要新增一把並更新所有 verifier | 否決 |
| **單一共享 HMAC 密鑰**（全體服務共用一把） | 無法鑑別「具體是哪個服務」，只能鑑別「是不是合法第一方服務」 | **採用** |
| **Redis SETNX nonce 唯一性**（嚴格 replay 防護） | 引入 Redis 為傳輸層授權的硬依賴；每次 RPC 多一次網路往返 | 否決（僅取時間窗） |

## 決策

1. 內部身分以**全體服務共用的單一密鑰** `INTERNAL_SERVICE_HMAC_SECRET` 簽發 HMAC-SHA256 簽章。信任邊界定為「**是不是合法第一方服務**」，不鑑別 caller 身分；`x-internal-service` 僅保留作資訊與 log 用途。
2. 網路可達性另以 **NetworkPolicy per-service ingress 白名單**收斂。HMAC 與 netpol 互補：誰都連得到也簽不出（HMAC），且連不到就不用談簽章（netpol）——任一單獨都不足以封閉上述兩個偽造面。
3. replay 防護**僅用時間窗**（stateless、無 Redis 依賴、零延遲），`x-internal-nonce` 欄位先帶但不校驗唯一性。

## 後果

- **接受**：無法從簽章鑑別 caller 身分。任何持共享密鑰的合法服務都能簽發有效簽章。要做 caller 級鑑別必須升級為 per-service key，屆時協議本身不需改（canonical 已含 serviceName）。
- **接受**：時間窗內捕獲到的簽章可被重放。簽章綁死 canonical（含傳播的 account_id），重放者無法改參數或換身分。要嚴格 replay 防護時接 Redis SETNX 即可，**無需改協議**（nonce 欄位已預留）。
- **接受**：密鑰一旦外洩，全體服務同時失守。緩解是雙密鑰並存的零斷線輪替。
- **重新評估條件**：團隊出現獨立 SRE 角色、或需要以 caller 身分做差異化授權時，重評 per-service key／mTLS。

現行規格（協議、header、輪替步驟、NetworkPolicy 白名單）見
`docs/詳細設計書/52-微服務間內部授權（Internal Service Authorization）詳細設計書.md`。
