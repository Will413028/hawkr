# 賣家能力權限層（Seller Capability Authorization）的設計裁決

| 項目 | 值 |
|---|---|
| 狀態 | D3、D4、D7 superseded by `2026-09-29-seller-suspension-and-business-identity-in-monolith.md`（A3，2026-09-29）；D1、D2、D9 沿用到單體的逐方法宣告。遷移計畫 A2（2026-09-27）起，account 的 blacklist 已刪除（帳號停權由 identity 在同一個 transaction 刪除 session，每個請求都查 identity），D3、D4 拿它當先例的部分只剩歷史意義；賣家停權（`seller:suspended`）與 gateway 的 capability gating 照舊，A3 搬遷時核對 |
| 日期 | 2026-07-11 ~ 2026-08-02 |
| 影響範圍 | `gateway/internal/authz/*`、`gateway/internal/router/*`、`seller-service`（status 傳播） |
| 規格 | [46-賣家能力權限層](../詳細設計書/46-賣家能力權限層（Seller%20Capability%20Authorization）詳細設計書.md) |

## 脈絡

賣家停權要落地時，Gateway 沒有任何「這個賣家能不能做這件事」的單一問答點：賣家 gating
散在各 `routes_*.go` 的 `requireSeller()` 群組／per-route 掛載，以及 handler 內的
`?view=seller` 分流。逐路由塞 `if suspended` 會產生上百個散點補丁，而「賣家能做什麼」
沒有任何 SSOT 可以驗證有沒有漏掛。

---

## D1：賣家 gating 的收斂點

| 方案 | 代價 | 判定 |
|---|---|---|
| 逐路由散點 `if suspended` | 沒有 SSOT，漏掛無從偵測 | ✗ |
| 獨立中央表 ＋ group-level guard | 表與掛載分離：共用群組會誤評買家流量、per-route guard 不在鏈上、「漏掛 guard」不會被擋（deny-by-default 只對真正經過 guard 的請求成立） | ✗ |
| 每 handler 內 `hasCapability` | 退回散點 | ✗ |
| 宣告＝掛載合一（`requireSeller(cap)`，capability 為必填參數） | 改變既有 `requireSeller()` 簽章，所有掛載點都要動 | ✓ |

**決策**：宣告＝掛載合一。capability 是必填參數 ⇒ 編譯期強制，「guard 不在鏈上」與
「宣告表漏條目」兩類風險在型別層消滅。

## D2：capability 的粒度

| 方案 | 代價 | 判定 |
|---|---|---|
| 每路由一個 token（上百個） | 維護災難，且與「一類事」的授權語意不對應 | ✗ |
| seller-status 布林判斷 | 無法承載未來的會員分級（tier）維度 | ✗ |
| 語意分組的粗粒度 capability | 一個 token 涵蓋多條路由，粒度判斷需要人 | ✓ |

**決策**：語意分組的粗粒度 capability，Go 常數放 `gateway/internal/authz/capability.go`，
不進 CodeMeta——它是 Gateway 內部授權概念，不跨服務、不對前端暴露值。

## D3：停權狀態怎麼傳到 Gateway

| 方案 | 即時性 | 負載 | 判定 |
|---|---|---|---|
| A. JWT claim 帶 status | 差（登入時鑄造，7 天內不更新） | 零 | ✗ 停權可能 7 天不生效 |
| B. 每請求 gRPC 查 seller-service | 佳 | 高（每賣家請求 +1 RPC） | ✗ 熱路徑加壓，seller-service 成單點 |
| C. Redis `seller:suspended:<id>` ＋ seller-service 寫入 | 佳（寫入即生效） | 低（一次 EXISTS，複用既有連線） | ✓ |

**決策**：C，對稱複用既有 `account:blocked:<id>` blacklist 模式。

## D4：Redis 故障時 fail-open 或 fail-closed

| 方案 | 代價 | 判定 |
|---|---|---|
| 全 fail-closed | Redis blip 期間**全體 active 賣家**無法上架／經營（可用性打擊面＝整個賣家母體） | ✗ |
| 全 fail-open | 正確性沒有任何兜底 | ✗ |
| 非對稱：Gateway fail-open ＋ 收斂點 fail-closed | blip 窗口內停權賣家可短暫恢復 BLOCK 能力 | ✓ |

**決策**：非對稱。fail-open 只讓**少數已停權賣家**在故障窗口短暫恢復能力（正確性風險面
小、頻率低），而唯一具「對外不可逆且有交易後果」性質的殘留（產生公開商品）由 product
ACTIVE transition 的 fail-closed backstop 關閉。既有 account blacklist 也選 fail-open，一致。

## D5：兩條 JWTAuth-only wildcard 死代理的處置

| 方案 | 代價 | 判定 |
|---|---|---|
| 補宣告 capability | gin `FullPath()` 對 wildcard 塌縮成單一 pattern，無法對其下 concrete path 做 BLOCK/ALLOW 混合判決 | ✗ |
| 靠收斂點兜底 | `engine.Routes()` 枚舉不等於攻擊面全集 ⇒ boot 斷言失去意義 | ✗ |
| 刪除 | 無（零流量、零風險） | ✓ |

**決策**：刪除。徹底消滅 wildcard 之後 `engine.Routes()` 枚舉＝完整攻擊面，
deny-by-default 才能被機器驗證。

## D6：ai-service 的孤兒 HTTP 端點

Gateway 零顯式入口的 5 條 ai-service HTTP 端點（recommend×3、chat×2）：補 Gateway 入口
或清掉死碼。

**決策**：清 ai-service 死碼，不補入口，且為獨立可 revert 的 commit。

## D7：tier 維度做或不做

**決策**：Phase 1 不實作，但 `GrantedCapabilities(SellerContext)` 的簽章即含 `Tier`。
未來會員分級只需在 `StatusActive` 分支依 tier 收斂 capability set，能力層架構零改動。

## D8：賣家 pricebook 市場行情端點的歸屬

| 方案 | 判定 |
|---|---|
| JWTAuth-only（任何登入帳號可查） | ✗ |
| 收斂進 `CAP_MARKETING`（BLOCK-if-suspended） | ✓ |

**決策**：收斂進 `CAP_MARKETING`。市場行情屬賣家經營情報，比照「純唯讀查看也鎖」的原則
（dashboard／CRM 查詢／報表同款），不應該任何登入帳號都能查。

## D9：seller `Capability` 與 admin `AdminCapability` 分成兩型兩 registry

**決策**：兩個型別、兩份 registry，不合併。合併之後，一條賣家路由誤宣告了 admin token
將不再於 boot panic，deny-by-default 的保證被靜默削弱。ADM 代行營運的 capability 歸
56 號所有。

## 後果

- 新增賣家路由不給 capability 就編譯不過；漏歸類則 boot panic。
- 白名單 capability 的路由完全不查 Redis ⇒ 出貨／售後／救濟管道在 Redis 故障時仍可用。
- 正確性依賴兩個收斂點（CheckoutValidate、product ACTIVE transition）的 fail-closed 檢查，
  任何重構不得移除或繞過。

## 修訂：能力加上前置條件維度，狀態與缺的條件由伺服器回報（2026-10-08，Will）

原決策段保留歷史。D7 預留的第二個維度不是 tier，而是前置條件：能力要不要授予，除了生命週期與停權，還看 KYC 流程等狀態。決策簡報：`docs/plans/2026-09-29-a3-buyer-seller/gating-decision-brief.md`；計畫：`docs/plans/2026-09-29-a3-buyer-seller.md` 10c「賣家功能的前置條件由伺服器判斷」。

**脈絡。** 物流建店要 KYC 核准（PayUni 以已成立的子商店建店），但規則只存在於兩個 client 各自的推導，伺服器沒有擋；而且 client 拿到「沒有這個能力」卻不知道為什麼，只好另讀 `GetMyKYC` 或公開曝光的狀態。

**決策。**

- 業界基準（Stripe Connect 的 capability 加 requirements、Adyen 的 capability 加 problems）：伺服器一個推導點產出每個能力的狀態與缺的條件，client 只照著畫，伺服器逐方法再擋。`model.CapabilitiesOf(store, kycFlow)` 回傳每個能力的 `{active, missing}`，`missing` 是 `SellerRequirement`（營業中、未停權、KYC 核准、店名、logo、鄉鎮市區、未進入刪除），依列舉的數字順序。停權、等待刪除與 KYC 都是「缺的條件」，不是另一套機制。
- 新增 `SHIPPING_SETUP`（物流建店、啟用與關閉配送方式；經營類，停權與等待刪除時一律缺；另缺 `KYC_APPROVED`）與 `PUBLIC_LISTING`（店家狀態：買家看得到、可以結帳；缺的條件就是 `MissingExposure`，原來的 `exposure` 欄位收進它）。`PUBLIC_LISTING` 是狀態不是動作：只出現在 `GetMyStore.capabilities`，Connect 方法不能宣告它，也不進 gateway 的能力 header。`MissingExposure` 仍是探索、目錄、結帳與可曝光公告共用的內部規則。
- 條件的判斷互不重疊：`STORE_ACTIVE` 是店家沒有關閉，`NOT_PENDING_DELETION` 是沒有進入刪除，`NOT_SUSPENDED` 是沒有停權。
- `Authorize` 的拒絕：已關閉 `not_found`；停權 `permission_denied`＋`SELLER_SUSPENDED`（API 錯誤模型 ADR D10 不變）；其他前置條件沒有滿足（等待刪除、KYC 未核准）`failed_precondition`＋`SELLER_REQUIREMENT_MISSING`，ErrorInfo.metadata 的 `missingRequirements` 是缺的條件（`SellerRequirement` 的值名，不加前綴，依列舉的數字由小到大以逗號連接）。流程的讀取只在能力取決於 KYC 時才發生（`DependsOnKYC`），所以沒有 KYC 前置條件的方法不多付兩次查詢；`Store.Can` 不讀流程，取決於 KYC 的能力一律回 false。
- gateway 的 header 只放啟用的動作類能力，`CAP_SHIPPING_SETUP` 是新增的 token；物流建店、啟用與關閉（`shipping_onboardings` 的 POST 與 toggle）改宣告它，列表與查詢仍是 `CAP_PRODUCT_MANAGE`。舊 REST 受 CodeMeta 凍結，前置條件造成的拒絕只回通用 403；client 先看能力，這只是後備，物流搬進單體後 Connect 方法宣告 `SHIPPING_SETUP`、回 `SELLER_REQUIREMENT_MISSING`。

**觸發。** PayUni sandbox 確認 C2C 與宅配可在審核中建店時，`SHIPPING_SETUP` 的條件放寬成「PayUni 已註冊」（只改伺服器）；優惠券、金流若有前置條件同樣加進能力。

## 修訂：pricebook 刪除（2026-10-28，ADR `2026-10-27-general-ecommerce-scope-cut` D1；計畫 `plans/2026-10-27-ecommerce-scope-cut.md` 步驟 6）

D8（賣家 pricebook 市場行情端點的歸屬）作廢：這些端點隨 pricebook 刪除，`CAP_MARKETING` 的其餘宣告不變。
