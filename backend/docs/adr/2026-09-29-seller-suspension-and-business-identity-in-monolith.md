# 單體裡的賣家停權檢查與業務身分的傳遞

| 項目 | 值 |
|---|---|
| 狀態 | accepted（Will 2026-09-29） |
| 日期 | 2026-09-29 |
| 影響範圍 | `internal/httpapi`（入口、`rpc` 的授權）、`internal/modules/seller`、`internal/modules/buyer`、`internal/legacy/gatewayproxy`、gateway 的 `authz`、`storeid`、`buyerid` |
| 取代 | `2026-09-18-seller-capability-authorization.md` 的 D3、D4；`2026-08-19-identity-current-store-id-rpc.md` |
| 計畫 | `docs/plans/2026-09-29-a3-buyer-seller.md`（決定第 2 項，步驟 1、7） |

## 脈絡

公司時期，賣家停權檢查放在 gateway：seller-service 把停權寫成 Redis 標記，gateway 每個賣家請求查一次，沒命中時回頭呼叫 seller-service，任何錯誤都放行（fail-open）；真正不可逆的兩個動作（商品上架、結帳）另有 fail-closed 的保底。`store_id` 與 `buyer_id` 也由 gateway 各自的 resolver 呼叫 seller-service、buyer-service 解析，快取在 Redis 60 秒。

A3 把 buyer、seller 搬進單體，入口在單體（A2），gateway 只剩還沒搬的模組的 REST，直到階段 C 刪除。

約束：
- 繼承：還沒搬的模組（Java 的 catalog、ordering、cart、coupon 等，以及晚搬的 Go 服務）的 REST 仍在 gateway，約 150 個 handler 呼叫點需要 `store_id`、`buyer_id` 與賣家停權狀態；仍成立到各模組搬遷。
- 繼承：gateway 已經信任單體帶的身分 header（A2，〈API 設計規範（Connect）〉§二）；gateway 不部署，只存在於本機與 CI。
- 不再成立：舊 D3 否決「每請求查 seller」的理由是多一次跨服務 RPC、seller-service 成為單點；舊 D4 的 fail-open 是為了 Redis 故障。單體裡是同一個 DB 的索引讀取，和 identity 每個請求解析 session 的成本相同。

## 考慮過的方案

**停權檢查**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：授權在處理請求的程序內，以權威資料判斷，錯誤時拒絕（fail-closed）；需要快取時由擁有者在寫入時失效（OWASP ASVS V4「fail securely」；Google 對停權專案回 `PERMISSION_DENIED`） | 每個賣家請求一次索引讀取；DB 故障時賣家請求失敗，但這時其他請求也一樣失敗 | ✓ |
| 沿用 Redis 標記＋fail-open | 單體多一個 Redis 依賴只為這件事；故障窗口內停權的賣家恢復能力；要保留兩個 fail-closed 保底 | ✗ |
| 停權狀態放進 token | Clerk 的 token 在登入時簽發，停權不會即時生效 | ✗ |

**還沒搬的模組怎麼拿到業務身分**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：入口解析一次身分，以受信任的 header 傳給後端（API gateway 的 identity propagation；A2 的做法） | 單體每個轉發的請求多查一次 buyer 與 seller（同上，索引讀取）；header 契約多三個欄位 | ✓ |
| gateway 經 legacy gRPC 呼叫單體解析，保留 Redis 快取 | gateway 不用改；保留 60 秒過期（stint 換新後最多 60 秒拿到舊的 ID）與兩份快取、兩支只為 gateway 存在的 RPC | ✗ |

## 決策

- 賣家停權檢查在單體：每個 seller 區的 Connect 方法宣告它需要的能力（保留舊 ADR 的 D1「宣告＝掛載」、D2 粗粒度能力、D9 賣家與管理員分開），啟動時檢查每個方法都有宣告；以 seller 模組的公開 API 讀取停權狀態，錯誤時拒絕。停權的回應依 ADR `2026-09-25-api-error-model.md` D10。
- 入口解析呼叫者自己的 `buyer_id`、`store_id` 與賣家停權狀態，轉給 gateway 時以身分 header 帶上；查詢失敗時不轉發（fail-closed）。gateway 的 `buyerid`、`storeid` resolver 回答呼叫者自己時讀 header，查別的帳號（賣家 CRM 的顧客、優惠券的對象等）仍直接呼叫 RPC；兩份 Redis 快取、失效機制與停權的 Redis 標記讀取刪除，`requireSeller` 改讀 header。
- 舊 D7 的會員分級維度不保留：賣家會員方案依 D8 刪除（計畫決定第 7 項），真的要分級時重新設計。

## 後果

- 單體每個賣家請求與每個轉發到 gateway 的登入請求，多一到兩次索引讀取。
- 商品上架、結帳的 fail-closed 保底不再是停權正確性的唯一依靠，但在 Java 的 catalog、ordering 改寫之前仍保留；B 階段改寫時改成呼叫 seller 的公開 API。
- gateway 的 handler 仍信任 header，只有單體連得到 gateway（A2 的 CD 待辦）的前提不變。
- 重新評估：出現多店、店員（`2026-06-29-identity-defer-store-entity.md` 的前提改變），一個帳號就不再只有一個 `store_id`，header 要改成由 client 指定並驗證。

## 8a 實作時的核對（2026-10-03）

- D1 在 Connect 的實現：宣告表（key 是生成的 procedure 常數）放在 `internal/httpapi/seller`，由只掛在 `SellerAppService`、位在 `rpc.NewInterceptor` 之內的 interceptor 讀取並執行（呼叫 `seller.ActingStore`、把店家放進 ctx，handler 不自己呼叫）。表就是執行，所以表與掛載不會分開漂移；「宣告＝掛載」的強制從舊 gateway 的路由編譯與開機斷言，移到 service 啟動時雙向對照 descriptor（缺宣告或指向不存在的方法，`NewHandler` 回錯誤）。
- 舊能力檢查 ADR D2 的「能力名稱不對前端暴露值」不再成立：`GetMyStore` 回傳呼叫者目前的能力（`SellerCapability`），client 事前就知道哪些操作可用（Stripe、GitHub 回應帶 permissions 的做法）；伺服器仍逐方法檢查，client 的清單只影響畫面。停權拒絕是 `permission_denied`＋`ErrorInfo.reason` `SELLER_SUSPENDED`（取代 9240）。
- 這是實作時的核對，沒有宣稱 Will 已核准。

## 修訂：gateway 改讀單體推導的能力集合（2026-10-06，Will）

原決策段保留歷史。

**脈絡。** 9d 把 gateway 前的店家解析切到 seller 模組後，店家多了舊 seller-service 沒有的狀態：帳號等待刪除時，店家拒絕營業類能力、保留履約類（計畫 7a 的能力集合，seller 是唯一推導能力的地方）。gateway 的 `requireSeller` 原本以 `X-Seller-Suspended` 加自己的停權白名單（`authz/policy.go`）推導能力，等待刪除的店只帶 `X-Store-Id`，會通過所有營業類宣告；而且這是能力的第二份推導。gateway 上帶營業類宣告的路由橫跨 ai、coupon、crm、invoice、post、pricebook、procurement、product、shipping，要到階段 C 刪除 gateway 才消失。

**考慮過的方案。**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：擁有規則的一方推導權限，邊緣只檢查「需要的權限在集合內」（OAuth scopes、Shopify access scopes 由簽發方算好；Envoy `ext_authz` 是每請求問權威方的另一種形式） | header 契約多一欄；gateway 的推導刪除 | ✓ |
| `X-Seller-Suspended` 加新值表示等待刪除 | 「停權」的語意被改（im、social 依它細分；9240 的文案是停權）；gateway 仍是第二份推導 | ✗ |
| 只帶 `X-Store-Id` | 等待刪除的店在 gateway 可做全部營業類操作 | ✗ |
| 當成沒有店家 | 履約類（出貨、售後）一起被擋，違反等待刪除保留履約類 | ✗ |

**決策。** 入口帶 `X-Store-Id` 時一定同時帶 `X-Seller-Capabilities`（seller 推導的能力集合，契約在〈API 設計規範（Connect）〉§二）；gateway 缺少、空值或含不認得的能力時拒絕（fail-closed），`requireSeller` 只檢查成員資格，`authz/policy.go` 的推導刪除。拒絕時停權仍回 9240，其他回通用 403（CodeMeta 凍結，不新增碼）。`X-Seller-Suspended` 不變，留給 9240 與下游的停權細分。

**終點。** 這個 header 只存在於單體→gateway，隨 `internal/legacy/gatewayproxy` 在 gateway 刪除時一起刪除；在那之前每搬走一個模組，需要它的路由就少一些。模組的 Connect handler 一律直接呼叫 seller 的授權（8a 的做法）。

## 修訂：能力帶狀態與缺的條件，前置條件造成的拒絕（2026-10-08，Will）

原決策段保留歷史；脈絡與完整決策見 `2026-09-18-seller-capability-authorization.md` 的同日修訂。對本 ADR 的影響：

- 8a 核對所寫「`GetMyStore` 回傳呼叫者目前的能力」改為回傳 `repeated CapabilityStatus { capability, active, missing }`，每個能力一項（含狀態 `PUBLIC_LISTING`），`exposure` 欄位刪除。停權拒絕仍是 `permission_denied`＋`SELLER_SUSPENDED`。
- 非停權的前置條件拒絕（等待刪除、KYC 未核准）在 Connect 是 `failed_precondition`＋`SELLER_REQUIREMENT_MISSING`（metadata `missingRequirements`），取代等待刪除原本沒有 reason 的 `failed_precondition`。
- 2026-10-06 修訂的 header 只放「啟用的動作類能力」：多了 `CAP_SHIPPING_SETUP`，不含 `PUBLIC_LISTING`；單體每個賣家請求補讀 KYC 流程（兩次查詢，與 `GetMyStore` 讀公開曝光同一種讀取）。gateway 拒絕仍是停權 9240、其餘通用 403。

## 修訂：能力宣告可以掛在其他模組的 AppService（2026-10-17，Will）

原決策段保留歷史；決策簡報 `docs/plans/2026-09-29-a3-buyer-seller/step10d-a7-social-brief.md` D1 與 D3（Will 全照建議）。

- 「宣告＝掛載」的原則不變，但不再限於 `SellerAppService`：資源被買家與店家共用的模組（A7 的 `SocialAppService`）在自己的方法宣告「以店家身分動作時需要的賣家能力」（`seller.Cap*`），或明列豁免與理由。由擁有資料的模組先依資源歸屬推導這次是不是店家身分，是才向 seller 公開 API 檢查能力（`seller.ActingStore` 或 `CapabilitiesOf`），不是則不查；handler 仍不自己檢查。啟動時的雙向對照照舊，各 service 各一張宣告表。
- 停權時的分類（D3）：新增類能力（`CapMarketing`）被停權擋下，收縮類能力（`CapContentContract`）與收縮需要的讀取放行；被拒的回應照舊（`permission_denied`＋`SELLER_SUSPENDED`）。

## 修訂：pricebook 刪除（2026-10-28，ADR `2026-10-27-general-ecommerce-scope-cut` D1；計畫 `plans/2026-10-27-ecommerce-scope-cut.md` 步驟 6）

脈絡裡列的「帶營業類宣告的 gateway 路由」不再包含 pricebook：那些路由已刪除。
