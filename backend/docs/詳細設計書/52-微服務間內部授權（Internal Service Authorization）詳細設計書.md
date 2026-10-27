# 微服務間內部授權（Internal Service Authorization）詳細設計書

## 0. 文件元資訊 (Metadata)

| 項目 | 值 |
|---|---|
| 維護者 | 維護團隊 |
| 對應程式碼 | `services/java/shared/common-core/.../grpc/InternalAuth{Config,Signer,Verifier}.java`、`GrpcAuth{ClientInterceptor,ServerInterceptor,Context,AutoConfiguration}.java`、`services/java/shared/observability-common/.../obs/grpc/GrpcChannelRegistry.java`、`pkg/middleware/grpc/interceptor/internal_auth.go`、各 Go 服務 `internal/grpc/di.go` server 組裝、`k8s/external-services/networkpolicy.yaml`、`k8s/values/*.yaml`、`docker-compose-local.yml` |
| 相關 doc | 44（身份模型，account_id vs 營運鍵）、46（賣家能力權限層，Gateway 端授權）、開發規範《Java 微服務開發規範》§4.2/§5、《API 設計規範》§五、36（MQ 事件不受本層影響）、《後端服務責任邊界規範》 |
| 相關 ADR | [單一共享 HMAC 密鑰](../adr/2026-09-18-internal-auth-single-shared-hmac-secret.md) |

---

## 1. 概述與威脅模型 (Overview & Threat Model)

### 1.1 文件定位

本文件描述**微服務間 gRPC 呼叫的內部身分驗證機制**：以 **HMAC-SHA256 簽章**判定 caller 是不是合法第一方服務，以 **NetworkPolicy per-service ingress 白名單** 收斂網路可達性，並以**明確內部 RPC**（而非歸屬檢查的魔數哨兵）界定內部面。目標讀者為全體 Java 與 Go 微服務、Gateway 的實作者，以及負責密鑰與 NetworkPolicy 落地的維運角色。

本層回答的唯一問題是「**到達某服務 gRPC port 的這通呼叫，是不是合法第一方服務發出的**」。它與 46 號賣家能力層（Gateway 端「賣家能不能做這件事」）、44 號身份模型（「當前賣家/買家是誰」）正交：本層守**服務對服務的傳輸信任**，那兩層守**使用者面授權與身分解析**。

### 1.2 雙軌守門

內部身分偽造與使用者身分偽造是兩個**正交**的面，各由一軌守：

| 面 | 守門 |
|---|---|
| **內部身分**（誰都能帶 `x-internal-service` 宣稱自己是內部） | **HMAC 簽章**：沒有共享密鑰就簽不出有效簽章（§3） |
| **使用者身分**（服務信任 `x-account-id`／`x-account-type`，rogue pod 也帶得出來） | **NetworkPolicy 收斂**：只有 Gateway 能以使用者身分到達服務 port，只有白名單 caller 能到達（§7） |

**Gateway 不得轉發外部 `x-internal-service`**：`gateway/internal/grpc/metadata_interceptor.go` 從零建立 outgoing metadata，外部帶進來的同名 header 不會被傳遞。這是現行契約，任何重構不得繞過——它是「外部使用者無法經 Gateway 取得 internal 身分」的唯一保證。

方案取捨（為什麼不採 mTLS／per-service key／Redis nonce）見
[ADR：單一共享 HMAC 密鑰](../adr/2026-09-18-internal-auth-single-shared-hmac-secret.md)。

### 1.3 設計目標與非目標

| 類型 | 項目 |
|---|---|
| **Goal** | 內部身分不可偽造：無共享密鑰的 pod 無法冒充第一方服務通過內部驗證 |
| **Goal** | 可分級的 mode：`off` / `permissive` / `enforcing` 三態，permissive 供未全數帶簽時不斷鏈（§6） |
| **Goal** | 網路可達性收斂：每服務 gRPC port 只允許白名單 caller，移除「同 ns 全通」 |
| **Goal** | 消滅歸屬檢查魔數哨兵（`storeId=0` 全繞歸屬）→ 明確內部 RPC + 傳輸層守門 |
| **Goal** | 縱深防禦：內部 PII RPC 對入參歸屬鍵做最小校驗，避免合法 caller 的 bug 造成整表外洩 |
| **Non-Goal** | **不**採 mTLS / per-service key（參照 ADR） |
| **Non-Goal** | **不**引入 Redis nonce 唯一性做嚴格 replay 防護；僅時間窗（§3.4） |
| **Non-Goal** | **不**引入 reconciliation / drift 偵測 / 補償任務基礎設施（§11 一致性、[後端服務責任邊界規範](../開發規範/後端服務責任邊界規範.md)） |
| **Non-Goal** | **不**把內部授權 header / mode 納入 CodeMeta（傳輸層概念，不跨業務語意、不對前端暴露） |

---

## 2. 範圍與情境 (Context & Scope)

### 2.1 系統情境圖 (C4 Context)

```mermaid
flowchart LR
  Ext["外部客戶端 / App / Web"] -->|HTTPS + JWT| GW[Gateway]
  Rogue["rogue / 被入侵 pod（同 ns）"] -.->|直連 gRPC port（攻擊面）| SVC
  GW -->|"gRPC：x-account-*（使用者面，不簽 internal）"| SVC["微服務 gRPC server"]
  SVC -->|"gRPC：x-internal-service + ts/nonce/sig（HMAC 簽章）"| SVC2["其他微服務"]
  subgraph Guard["兩道守門"]
    NP["NetworkPolicy：ingress 白名單<br/>只允許 gateway + 白名單 caller"]
    HM["HMAC verifier：<br/>帶 x-internal-service 才驗簽"]
  end
  SVC -.受.-> NP
  SVC -.受.-> HM
  SEC[(k8s secret<br/>hawkr-internal-auth)] -->|INTERNAL_SERVICE_HMAC_SECRET[_PREV]| SVC
```

### 2.2 邊界與責任

| | 涵蓋 (In-scope) | 不涵蓋 (Out-of-scope，由誰負責) |
|---|---|---|
| 責任 | 服務對服務 gRPC 呼叫的內部身分簽章與驗證；網路層可達性收斂；歸屬判定不使用魔數哨兵；內部 PII RPC 入參最小校驗 | 使用者面授權（`requireAccountType`）→ 各服務 RPC 既有規則；賣家能不能做某事 → 46 號；當前賣家/買家是誰 → 44 號 resolver；密鑰值本身的產生與 cluster 建立 → 維運 |

**信任邊界定位**：本層的信任邊界是「**是不是合法第一方服務**」，而非「具體是哪個服務」。任何持共享密鑰的合法服務均可簽發有效簽章；`x-internal-service`（caller 名）**保留作資訊與 log 用途，不再是信任依據**——信任只來自簽章驗證通過。若未來要 caller 級鑑別，再升 per-service key（本設計不預留該複雜度）。

### 2.3 三類 gRPC 呼叫與各自的信任路徑

| 呼叫類型 | 攜帶 header | server 端處置 |
|---|---|---|
| **Gateway → service（使用者面）** | `x-account-id`/`x-account-type`（JWT 後注入），**不帶** `x-internal-service` | 走既有 `requireAccountType` 使用者授權；**不被 HMAC 驗簽**（無 internal header） |
| **service → service（內部面）** | `x-internal-service` + `x-internal-ts`/`nonce`/`sig`（HMAC 簽章），可含傳播的 `x-account-id` | 驗簽通過才 `INTERNAL_CALL=TRUE`；enforcing 下無有效簽章即拒 |
| **service → service（使用者身分傳播，forward-only）** | 只轉發 `x-account-id`/`x-account-type`，**不設** `x-internal-service`（如 buyer/seller/content 的 media client） | 視為使用者面呼叫，走使用者授權，**不簽 internal、不驗簽** |

**關鍵相容規則**：HMAC 驗簽**只對「帶 `x-internal-service` 的呼叫」生效**。不帶 internal header 的呼叫（Gateway 使用者呼叫、forward-only 傳播）維持現行語義，**不被 HMAC 擋**——否則會把 Gateway→service 全部打死。此規則是 Go/Java 兩端 server 一致的兜底。

---

## 3. HMAC 內部身分簽章協議 (Signature Protocol)

### 3.1 密鑰模型

全體服務共用單一密鑰 `INTERNAL_SERVICE_HMAC_SECRET`（高熵 ≥32 bytes）。任何持該密鑰的服務均可簽發有效簽章；簽章不鑑別 caller 身分，只鑑別「是不是合法第一方服務」。

Gateway **不**持有該密鑰、**不**參與內部簽章（見 §3.6）。

選型理由見 [ADR](../adr/2026-09-18-internal-auth-single-shared-hmac-secret.md)。

### 3.2 簽章內容與 header

內部出站呼叫攜帶四個 header（沿用小寫連字號慣例）：

| header | 值 |
|---|---|
| `x-internal-service` | caller 服務名（保留，資訊/log 用，非信任依據） |
| `x-internal-ts` | unix 秒（防重放時間窗基準） |
| `x-internal-nonce` | 隨機 16 bytes 的 hex（必帶；唯一性不校驗，見 §3.4） |
| `x-internal-sig` | `base64( HMAC-SHA256( secret, canonical ) )`（標準 base64 含 padding） |

**canonical string**（跨語言逐位元組一致，UTF-8、`\n`(LF) 分隔）：

```
canonical = serviceName + "\n" + ts + "\n" + nonce + "\n" + accountIdOrEmpty
```

其中 `accountIdOrEmpty` = 當前傳播的 `account_id` 字串，無則空字串。

### 3.3 accountId 綁定

canonical 末段綁入傳播的 `account_id`，使**捕獲到的簽章在時間窗內無法搭配「換一個 account_id」重放**（防窗內身分抽換）。client 簽章時讀當前傳播 accountId、server 驗簽時用收到的 `x-account-id` 重算——兩端都拿得到，成本低。無傳播 accountId 的純系統呼叫則綁空字串。

### 3.4 replay 防護：時間窗

replay 防護**只有時間窗**（stateless、無 Redis 依賴）：

- server 檢查 `|now - ts| <= INTERNAL_AUTH_TS_WINDOW_SECONDS`（預設 300s，用以容忍 pod 之間的時鐘偏移）。超窗即視為驗證失敗。
- `x-internal-nonce` **必帶但不校驗唯一性**。它是為日後接上唯一性校驗預留的欄位，接上時協議不需改。

取捨見 [ADR](../adr/2026-09-18-internal-auth-single-shared-hmac-secret.md)。

### 3.5 server 驗證流程

1. 若呼叫**無** `x-internal-service` → 非內部呼叫，直接放行走使用者面授權（§2.3 相容規則），不驗簽。
2. 有 `x-internal-service` 但**無** `x-internal-sig` → 依 mode：permissive 放行並記 `internal.auth.unsigned` WARN（帶 caller 名）；enforcing 拒絕（`UNAUTHENTICATED`）。
3. 檢查時間窗 `|now - ts| <= ts-window`。
4. 用 current secret（及輪替期的 prev secret）重算 canonical HMAC，**常數時間比對**（Java `MessageDigest.isEqual` / Go `hmac.Equal`）`x-internal-sig`；任一密鑰通過即 internal 可信。
5. 通過 → `INTERNAL_CALL=TRUE`（沿用現有 Context.Key，下游 `requireXxx` 行為不變）；enforcing 下失敗（無效 sig）→ 拒絕並記 `internal.auth.invalid`。

### 3.6 Gateway 不簽內部授權

Gateway → service 呼叫**不是內部呼叫**——它攜帶 end-user 身分（`x-account-id`/`type`，JWT 後注入），在目標服務走 `requireAccountType` 使用者授權路徑。因此：

- Gateway **不簽 internal-auth、不帶 `x-internal-service`**（維持現狀）。
- Gateway **不需**注入 `hawkr-internal-auth` secret。
- **推論**：凡 Gateway 直接呼叫的 RPC，其授權**必須**能靠「使用者身分 + 請求內 Gateway-resolved 歸屬鍵」成立，**不可**依賴 internal bypass。這正好把「哪些 RPC 是真內部、哪些是 Gateway 使用者面」逼出來（見 §8/§9）。

---

## 4. 密鑰注入與雙密鑰輪替 (Key Injection & Rotation)

### 4.1 來源與注入

- **來源**：k8s secret `hawkr-internal-auth`，key `INTERNAL_SERVICE_HMAC_SECRET`（current）＋輪替期 `INTERNAL_SERVICE_HMAC_SECRET_PREV`（prev）。
- **注入**：沿用現有 `secretRef` 慣例。**每一個 Java + Go 微服務（排除 Gateway）** 的 `k8s/values/*.yaml` 加：

  ```yaml
  secretRef:
    - secretName: hawkr-internal-auth
      keys:
        - { name: INTERNAL_SERVICE_HMAC_SECRET, optional: true }
        - { name: INTERNAL_SERVICE_HMAC_SECRET_PREV, optional: true }
  ```

- **兩 key 皆 `optional: true`**：permissive 設計本就容忍缺 secret（不簽/不驗）；若用 required 則 secret 未建時整批 pod `CreateContainerConfigError` 啟動失敗，與 permissive「不斷鏈」精神矛盾並製造 deploy 順序硬依賴。optional 讓「先上 code、後建 secret」與「先建 secret、後上 code」兩序皆安全。
- **本地**：`docker-compose-local.yml` 各微服務（排除 Gateway）environment 直傳 `INTERNAL_AUTH_MODE` + `INTERNAL_SERVICE_HMAC_SECRET[_PREV]`。

### 4.2 fail-closed 條件

缺 secret 時的行為依 mode 分岔：

| mode | 缺 secret 行為 |
|---|---|
| `enforcing` | **啟動即拒**（fail-closed，不做明文 fallback） |
| `permissive` / `off` | 記 WARN 並不簽，不阻擋啟動 |

### 4.3 env var 同步

密鑰相關的環境設定須同步五個位置：cluster 內的 k8s secret 實體、`k8s/values/*.yaml` 的 `secretRef`、`docker-compose-local.yml`、`dev.sh`、本文件。

cluster 側的 secret 由維運動作建立（`kubectl create secret generic hawkr-internal-auth --from-literal=INTERNAL_SERVICE_HMAC_SECRET=<高熵≥32bytes>`），不寫進 codebase。因 values 兩個 key 皆 `optional`，secret 尚未建立時 pod 仍會啟動（不簽），建立後下一次 rollout 自動開始簽章。

### 4.4 雙密鑰輪替（零斷線）

server 同時接受 current 與 prev 兩把密鑰。輪替步驟：

1. 新值寫 `INTERNAL_SERVICE_HMAC_SECRET`、舊值寫 `INTERNAL_SERVICE_HMAC_SECRET_PREV`。
2. 滾動重啟全服務 → client 改用新值簽、server 兩把都收（新舊簽章都通過）。
3. 全數完成後清空 `PREV`。
4. 再滾一次 → 舊值徹底退場。

全程零斷線；輪替不需切 mode。

---

## 5. 跨語言實作 (Cross-Language Implementation)

### 5.1 Java（common-core / observability-common）

Java 側 gRPC channel 有**唯一工廠**：`GrpcChannelRegistry.buildChannel()` 對每個 channel 掛 interceptor，所有出站 stub 一律經 `GrpcStubFactory.blocking()` 取得，**沒有旁路**。內部簽章因此只掛在這條工廠鏈上，全 Java 服務生效；禁止自建 `ManagedChannel` 繞過。

| 元件（`common-core .../grpc/`） | 職責 |
|---|---|
| `InternalAuthConfig` | 程序層級設定靜態持有（mode / secret / secretPrev / tsWindow），client（obs-common）與 server（common-core）共用 |
| `InternalAuthSigner` | 純函式：canonical 組裝、HMAC-SHA256（JDK `javax.crypto.Mac`，原始 HMAC 非 JWT，不引 jjwt）、nonce、ts |
| `InternalAuthVerifier` | 純函式：時間窗檢查 ＋ current／prev 常數時間比對（`MessageDigest.isEqual`） |
| `GrpcAuthContext` | 持有 `HEADER_INTERNAL_TS` / `NONCE` / `SIG` 常數 |
| `GrpcAuthClientInterceptor.start()` | 在 `headers.put(KEY_INTERNAL_SERVICE, ...)` 旁加簽 ts／nonce／sig，綁定傳播的 accountId；缺 secret 時（`permissive`／`off`）不簽 |
| `GrpcAuthServerInterceptor.interceptCall()` | 依 mode 驗簽（§3.5），**只對帶 `x-internal-service` 的呼叫生效** |
| `GrpcAuthAutoConfiguration` | `@Value` 讀 `hawkr.auth.internal.{mode,secret,secret-prev,ts-window-seconds}`（env fallback `INTERNAL_AUTH_MODE` / `INTERNAL_SERVICE_HMAC_SECRET[_PREV]` / `INTERNAL_AUTH_TS_WINDOW_SECONDS`），`@PostConstruct` 注入 `InternalAuthConfig`；`enforcing` ＋ 缺 secret ⇒ fail-closed |

### 5.2 Go（pkg + 各服務組裝）

`pkg/middleware/grpc/interceptor/internal_auth.go` 是 Go 端的唯一實作，與 Java 逐位元組一致：

- client：`InternalAuthClientInterceptor(serviceName, forwardAccountHeaders)`，於各服務 `internal/client/<peer>/` 的 dial option 掛上。**禁止在 method 內 inline `metadata.Pairs` 自行注入 `x-internal-service`**。
- server：`InternalAuthServerInterceptor(logger)`，於各 Go 服務 `internal/grpc/di.go` 的五件套鏈 Logging 之後掛上。
- 設定走 `pkg/config` 讀同一組 env。

### 5.3 `forwardAccountHeaders` 判準

| 對端以什麼身分授權 | 值 | 例 |
|---|---|---|
| **使用者**身分（對端會對 `x-account-id` 做 `requireAccountType`／`requireAccountId`） | `true` | media-service |
| **服務**身分（對端只需知道 caller 是合法第一方） | `false` | coupon、order（crm 出站） |

填錯的症狀不是報錯而是語意漂移：該轉不轉 ⇒ 對端以為沒有使用者（例如 media 回「資產不存在」）；不該轉卻轉 ⇒ 把 caller incoming 的 `x-account-id` 帶進對端的 `requireAccountId`，改變查詢 scope。

**forward-only 呼叫不簽 internal**：buyer／seller／content 的 media client 只帶 `x-account-id`／`x-account-type`、不帶 `x-internal-service`，屬使用者面呼叫。server 端視為非內部呼叫、走使用者面授權，`enforcing` 下也不受 HMAC 影響（§2.3）。

### 5.4 跨語言一致性（黃金向量）

兩端 HMAC 必須逐位元組一致。下列固定向量是跨語言 wire 契約，Go 與 Java 各自斷言同值：

| canonical 輸入 | secret | 期望 sig |
|---|---|---|
| `("order-service","1700000000","00112233445566778899aabbccddeeff","123456789")` | `test-shared-secret-please-change` | `TXm3Uefzoq0Z8ru9n++yFm4DjaEs9XZ2GWMpHsi3BOk=` |
| `("crm-service","1700000000","abcdef00112233445566778899aabbcc","")` | 同上 | `HTg2eOBanXJvJEs67j5dVl+DQXIjU3rvQAmIySTmcZg=` |

驗收方式：Go `TestInternalAuthSign_GoldenCrossLanguage`、Java `InternalAuthSignerTest.signMatchesGoWith{Account,EmptyAccount}`。兩端各須另涵蓋輪替 PREV 通過、換 accountId 失敗、超窗失敗三個案例。

---

## 6. mode 契約 (Auth Mode)

單一 mode 旗標（Java `hawkr.auth.internal.mode` / 全服務 env `INTERNAL_AUTH_MODE`）有三個合法值：

| mode | client 行為 | server 行為（僅對帶 `x-internal-service` 的呼叫） |
|---|---|---|
| `off` | 不簽 | 不驗簽，帶 `x-internal-service` 即視為內部呼叫 |
| `permissive` | 一律簽（帶 ts/nonce/sig） | 有 sig 則驗；**驗過／無 sig／無效 sig 一律放行**。無 sig 記 `internal.auth.unsigned` WARN、無效 sig 記 `internal.auth.invalid` |
| `enforcing` | 一律簽 | 有 `x-internal-service` 但**無有效 sig ⇒ 拒絕**（`UNAUTHENTICATED`）。缺 secret 時啟動即拒（§4.2） |

**生效值**：`k8s/values/*.yaml` base 為 `permissive`（供本機與未覆蓋環境）；`k8s/values/overlays/dev/common.yaml` 與 `overlays/prod/common.yaml` 覆蓋為 `enforcing`。

**切換到 `enforcing` 的前置條件**（不成立即不得切）：所有服務都已部署到具備簽章能力的版本，且該環境的 `internal.auth.unsigned` WARN 已歸零——該 WARN 非零代表仍有 caller 發出無簽呼叫，切換後那條鏈路會直接斷。切換本身是純環境變數翻轉 ＋ 滾動更新，不需改 code。

切換後若 `UNAUTHENTICATED` 突增，代表有漏網 caller，退回 `permissive` 排查。

---

## 7. NetworkPolicy 收斂 (Network Convergence)

### 7.1 收斂原則

每服務 gRPC port 只允許白名單 caller 到達；`deny-cross-namespace` **不得**再放行 `podSelector: {}`（那等於同 namespace 全通）。

🔴 **NetworkPolicy 只能相加（union）不能相減**：只要任何一條 policy 放行了「所有同 ns pod」，其餘 per-service ingress policy 全部形同註解、沒有任何 enforcement 效果。新增放行規則前必須確認它不會擴大成全通。

### 7.2 caller → callee 依賴圖（程式碼真實來源雙軌重建）

⚠️ **caller 白名單的權威來源**：必須由三處交叉重建——Go `services/*/internal/client/<peer>/` 目錄 ＋ `*_SERVICE_ADDR/HOST` env、Java `*GrpcClient.java` ＋ `grpcStubFactory.blocking(...)`、gateway `internal/grpc/*_client.go` ＋ `internal/proxy/reverse_proxy.go`。

🔴 **不得只從 `k8s/values/*.yaml` 反推**：Go 服務的出站對象不寫在 values，這樣反推會系統性漏掉**全部** Go→X 邊，其中包含 `product←order`／`product←seller`／`seller←order` 等結帳 hot path 邊；漏一條，收斂生效當下該鏈路即斷。

下表的實作是 `k8s/external-services/networkpolicy.yaml`；兩者不一致時以本表為規格、以該檔為現況，並在同一個 PR 內對齊。

**Ingress 白名單（每服務 ← 誰呼叫我；gateway 恆含。★=結帳/出貨 hot path 邊，改動後必跑 `infra/smoke-checkout.sh`）**：

| 服務 (callee) | ingress callers（除 gateway 外） | hot path |
|---|---|---|
| account | buyer, coupon, crm, seller, social | |
| aftersales | order | ★ |
| buyer | aftersales, cart, coupon, crm, im, order, product, review, seller, shipping, social | ★ |
| cart | order | ★ |
| content | （僅 gateway） | |
| coupon | aftersales, crm, order | ★ |
| crm | buyer | |
| im | social | |
| media | aftersales, ai, buyer, content, im, order, product, product-import, review, seller, shipping, social | ★ |
| notification | （僅 gateway） | |
| order | aftersales, ai, buyer, coupon, crm, im, payment, review, seller, shipping | ★ |
| payment | aftersales, order | ★ |
| product | ai, buyer, cart, order, product-import, seller, social | ★★ |
| product-import | （僅 gateway） | |
| review | seller, social | |
| seller | aftersales, ai, buyer, cart, coupon, im, order, product, shipping, social | ★★ |
| shipping | aftersales, order, product, product-import,（payuni-webhook＝**僅 dev/local sim**） | ★ |
| social | seller | |

> **payuni-webhook 特例**：其唯一 gRPC 出站（→shipping）僅 `WebhookSimulatorController` 在 `APP_ENV∈{dev,local}` 啟用（prod 403）；**prod 無任何 payuni-webhook gRPC 出站**（真 webhook 走 MQ）。故 shipping ingress 的 payuni-webhook caller **僅 dev 需放行**，prod 可省。

### 7.3 新增一條 caller 邊

服務 A 新增對服務 B 的 gRPC 呼叫時，同一個 PR 內必須：

1. 在 §7.2 表的 B 列補上 A，並在 `k8s/external-services/networkpolicy.yaml` 的 `<B>-ingress` 加 `podSelector: matchLabels: app.kubernetes.io/name: <A>`，對應 B 的 gRPC port。
2. 確認 `k8s/values/<A>.yaml` 有 B 的 `*_SERVICE_HOST` / `*_SERVICE_GRPC_PORT`。
3. 若該 policy 的註解寫著「只有 X 會呼叫」，一併更新。

漏掉第 1 步的症狀取決於 caller 的失敗處置：fail-closed 的呼叫端會直接報錯，**fail-open 的呼叫端則完全無聲**——測試綠、部署綠、smoke 綠，而那條檢查從部署起從未執行過，唯一痕跡是 pod log 一行 `UNAUTHENTICATED: io exception`。因此驗證方式必須是**造一個應該被拒絕的情境，確認它真的被拒絕**，不能只跑一次看有沒有壞。

apply 之後要 `kubectl rollout restart` 呼叫端：gRPC channel 在連線失敗後有退避，TCP 通了但既有 channel 仍在 backoff。

### 7.4 metrics / health probe

🔴 **來源是 node/host IP 的流量，`podSelector` 抓不到，必須用 `ipBlock` 放行**。兩個來源：

- **central Prometheus 抓取 metrics**：Monitoring VM 經 host NodePort 直抓，封包 source 是 node/host IP 而非 pod IP。每個被 central Prometheus 抓的服務都要在 metrics port 放行對應的 `ipBlock`（node 私網段／Monitoring VM CIDR），否則該服務會被判定 ServiceDown 並持續告警。
- **kubelet health probe**：liveness／readiness probe 由 node 上的 kubelet 發起，同屬 host IP 來源，probe port 需一併放行，否則 pod 被誤判不健康而反覆重啟。

---

## 8. 歸屬判定不得使用魔數哨兵 (No Sentinel-Based Ownership)

### 8.1 禁令

🔴 **不得以參數的特殊值（`0`／`null`／空字串）表示「跳過歸屬檢查」。** 這種哨兵讓「有沒有做歸屬校驗」取決於呼叫端傳什麼值，而傳錯值不會報錯、只會靜默放行整表。

內部面需要免歸屬校驗時，正確做法是**定義一支語義明確的內部 RPC**，其授權來自 HMAC internal-auth ＋ NetworkPolicy 白名單，而不是在共用 RPC 內留一條魔數分支。

### 8.2 `GetOrderForShipping` 的現行契約

`OrderService.getOrderForShipping(orderNo)` 為**純內部讀**，只以 `orderNo` 定位單筆：

- 授權來自 HMAC internal-auth ＋ netpol（§7.2 白名單：shipping ← order 面向），**不在方法內做歸屬校驗**。
- request 的 `store_id` 為**保留欄位，不參與授權**，server 端不得依它分岔行為。
- 唯一 caller 是 shipping-service 的 `OrderGrpcClient`。

### 8.3 filter 型可選條件

`OrderRepository` 的買家過濾屬**可選查詢條件**（不帶即不過濾），不是授權判定，因此不適用 §8.1 的禁令。但表達方式一律用 nullable 參數（`null` ⇒ 不過濾）或獨立的 query method，**不得用 `0` 這類魔數**——它與歸屬哨兵同形，會讓讀者無法分辨哪一個是安全相關的。

---

## 9. 內部 PII RPC 縱深校驗 (Defense-in-Depth on PII RPCs)

即使 HMAC + netpol 守住「誰能呼叫」，仍對 PII RPC 做**入參歸屬鍵最小校驗**，避免合法 caller 的 bug 造成整表外洩。分兩類：**內部全域讀**（無單一 scope 鍵，靠 HMAC+netpol 限 caller）與**內部 scoped 讀**（有歸屬鍵，強制 `>0` 校驗）。

| RPC | 回傳 PII | 類別 | 校驗要求 |
|---|---|---|---|
| order `GetOrderForShipping(Internal)` → recipientName/phone/address | 收件人 PII | 內部全域讀（shipping 讀所有賣家訂單） | 僅 orderNo 定位單筆；靠 HMAC+netpol（只 shipping/order 可達），不回整表 |
| order `getBuyerMerchantSummary`（caller = **coupon-service** `getBuyerMemberSellerIds`） | 買家消費彙總 | 內部 scoped 讀 | 要求 `buyerId > 0`，缺/0 直接拒（`INVALID_ARGUMENT`） |
| order `getBuyerOrdersForSeller`（caller = **gateway** seller CRM 使用者面 + **buyer-service** 內部面） | 買家在某商家的訂單 | 雙 caller（見下） | 要求 `storeId>0 && buyerId>0`，查詢恆以 (storeId,buyerId) scope |
| shipping `getShipmentsByOrderNos` / `getShipmentByOrderNo` | 出貨/物流資訊 | 內部全域讀（by orderNo） | 靠 HMAC+netpol（只 order/aftersales/product/product-import 等合法 caller 可達） |
| aftersales `hasActiveCase` / `cancelActiveCaseOnConfirmReceipt`（caller = order 完成/售後鏈） | 售後案件狀態 | 內部全域讀 | 靠 HMAC+netpol（只 order 可達） |

**`getBuyerOrdersForSeller` 雙軌授權**：此 RPC 同時接受「使用者面（gateway，帶 seller 身分 + gateway-resolved storeId、無 internal sig）」與「內部面（buyer-service，有 internal sig）」。server 端本無 `requireAccountType`、純 param-scope 查詢（XML 已 `store_id=#{sellerId} AND buyer_id != #{sellerId}` scope），本層只把 internal 判定從明文改成驗簽、並加 `storeId/buyerId>0` 校驗，**行為不變**。維持單一 RPC 雙軌是預期設計，不拆兩支。gateway seller CRM 路徑的 IDOR 回歸以 `gateway/internal/handler/seller_crm_idor_test.go` 兜底。

---

## 10. 驗收與回歸範圍 (Acceptance & Regression Scope)

動到 `GrpcAuthServerInterceptor` / `InternalAuthServerInterceptor` / `isInternalCall` 語義、mode 值、或 §7.2 白名單時，影響**所有**依賴 internal 判定的 RPC。下列七條鏈路必須逐條實測，**不得以 `go test` / `mvn test` 通過代替行為層驗證**：

| # | 鏈路 | 驗收判準 |
|---|---|---|
| ① | 結帳 hot path | `infra/smoke-checkout.sh` 全綠（order 內部呼叫最密集的一條） |
| ② | 出貨鏈 shipping ↔ order | 建單與查單成功 |
| ③ | 售後鏈 order → aftersales | `hasActiveCase` / 確認收貨取消案件成功 |
| ④ | 買家會員 coupon → order | 消費彙總查得到 |
| ⑤ | 賣家 CRM gateway → order（使用者面） | 不被 HMAC 誤擋，且 `gateway/internal/handler/seller_crm_idor_test.go` 綠 |
| ⑥ | coupon 補償 order → coupon | 發放成功 |
| ⑦ | gateway → 各 Go 服務（無 sig） | 每個 Go 服務至少一條讀路徑實測，確認不被 server HMAC 擋 |

另在 `permissive` 下以 `/trace` 或 Loki 確認 `internal.auth.unsigned` 為零——它非零代表仍有 caller 發無簽呼叫。

---

## 11. 一致性與責任邊界 (Consistency & Responsibility Boundary)

- **單一寫入路徑**：本層不新增任何業務狀態寫入。內部授權為傳輸層的**同步驗證**，不落庫、不發 MQ、不維護任何反正規化欄位。信任判定於每次 RPC 當下同步完成，無非同步 backfill、無 reconciliation 表。
- **不引入補償任務基礎設施（[後端服務責任邊界規範](../開發規範/後端服務責任邊界規範.md)）**：replay 防護明確選擇「僅時間窗、無 Redis nonce 表」正是為避免引入資料完整性監測型基礎設施；密鑰輪替靠雙密鑰並存 + 滾動重啟，不需 drift 偵測。NetworkPolicy 收斂為聲明式 union，非執行期監測。
- **維運 vs application code 分界**：secret 的產生與 cluster `kubectl create secret`、env 同步、netpol flip 的 runtime 流量核對，屬**維運帽子**動作（後端工程師主動做，不寫進 codebase）；signer/verifier/interceptor/values/netpol yaml 屬 application code，隨 PR 交付。
- **權威來源**：內部信任的唯一判定點 = 兩端 server interceptor 的驗簽結果（Java `GrpcAuthServerInterceptor` / Go `InternalAuthServerInterceptor`）；`x-internal-service` 字面值僅資訊用途，不得作為任何服務的授權依據。
- **fail 策略**：enforcing 缺 secret → fail-closed 拒啟動（安全優先）；`permissive` → fail-open 不簽不擋（可用性優先）。兩態邊界由 mode 明確界定，不存在「半驗證」中間態。
