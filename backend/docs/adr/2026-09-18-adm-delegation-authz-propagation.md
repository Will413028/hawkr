# 代行營運：授權如何穿過微服務牆（為什麼不冒用賣家身分）

| 項目 | 值 |
|---|---|
| 狀態 | accepted；遷移計畫 A2（2026-09-27）起，代行的 session 與授權資料在單體的 identity 模組，gateway 經 legacy 的 DelegationService 取得；代行身分的傳遞、授權牆與 target store 的取得照舊，A10 代行編排搬進單體時重新核對 |
| 日期 | 2026-09-18（決策內容成形於 2026-08-09） |
| 影響範圍 | `api/proto/**`、Gateway authz、product／seller／social／order 四面的寫入 RPC、hawkr-admin |

## 1. 為什麼不是冒用賣家身分（impersonation）

「換一枚賣家 JWT 給管理員用」是最省事的做法，**明確否決**，理由有二：

- **稽核層從此看不見 admin。** 換發賣家 token 之後 `x-account-id` 變成賣家，
  而「actor 始終是 ADM 管理員」是本案的第一條驗收條件——沒有它，代行與「賣家自己做的」
  在資料上不可區分，事後無法舉證。
- **還要另建一套 token 撤銷模型**與既有 `revoked_tokens` 對齊，而撤銷的即時性正是本案的安全核心。

impersonation 的替代代價（`requireSellerOrDelegated`）是**一個共用 helper 的替換**，
不是十餘份複製，兩者不成比例。

## 2. 代行寫入如何穿過微服務授權牆

**考慮過**：① 換發 `accountType=2` 的賣家 JWT ② 每支寫入 RPC 新增 `*ForDelegation` 變體
③ 既有 RPC 加**可缺席的** `DelegationContext` 欄位

**決策：③ 既有 RPC 加 `DelegationContext`。**

**否決 ①**：見上一節。
**否決 ②**：賣家可觸及的寫入 RPC 有十餘支，複製一份等於把每一個業務規則變成兩份實作，
兩份必然漂移（Java 微服務開發規範的重複實作防護；MetaObjectHandler 曾有八份副本的教訓）。

③ 的代價是每支 RPC 的授權判斷要從 `requireAccountType(SELLER)` 換成
`requireSellerOrDelegated(storeId, capability)`，但那是**一個共用 helper 的替換**。

> ⚠️ 「兩份必然漂移」這把尺在本案被引用了三次，且**不只用在後端**：
> 工作區 repo 的選擇（`2026-09-18-adm-delegation-workspace-host.md`）與
> 代行請求的轉送方式都以同一條理由否決了「在兩個 repo 各維護一份」。

## 3. 代行身分如何跨服務傳遞

**考慮過**：① 新增 `x-delegation-*` gRPC metadata ＋ 把它加進內部授權 HMAC canonical
② 只走 proto 的 `DelegationContext` 欄位

**決策：② 純 proto 欄位，HMAC canonical 不動。**

- **(a) 雙載體**——既然代行語境已在 proto 欄位裡，metadata 就是第二份，
  兩份可能不一致，而「不一致時信哪一份」沒有正確答案。
- **(b) 破壞性遷移**——canonical 是 `serviceName\nts\nnonce\naccountIdOrEmpty`，
  改它要 Java／Go 同波上線並經歷 permissive 雙驗過渡期，為一個可以不改的東西付出全平台遷移成本。

信任模型與既有 `SuspendSellerForAdminRequest.AdminId` 完全相同：52 號 HMAC 已保證呼叫方是內部服務，
delegation 的真實性由 Gateway 保證、微服務信任之。

## 4. admin 側的能力判定放哪

**考慮過**：① 新建獨立的代行授權層 ② 擴充既有 `gateway/internal/authz` 成雙軌

**決策：② 擴充既有 package。**

① 會造出**第二套授權機制**：賣家一套判定、代行一套判定，兩套各有自己的 registry、
boot 斷言與豁免清單，而「這個人能不能做這件事」從此沒有單一問答點——46 號建立能力層
要解決的正是這個問題（當時光盤點 405 條路由就耗掉整輪 session）。
② 讓 `authz` 成為兩種主體的共同問答點，`registry`／「宣告＝掛載合一」／boot 斷言
三個既有機制原地沿用。**附帶收益**：機制對全部 admin 路由開放。

## 5. 代行路徑如何取得 target store

**考慮過**：① 沿用 `gateway/internal/storeid` resolver ② 一律取自 Operation Session 快照

**決策：② 取自 session 快照。**

resolver 的語意是「這個登入者現在是哪一家店」，對 admin 帳號**答案恆為無**——
它會走完整條 gRPC 查詢後在 Redis 留下一個**負快取毒值**。② 另有正確性收益：
session 快照是**建立當下**凍結的目標，即使賣家在代行期間發生 store 換任，
代行的對象也不會漂移。

## 6. capability 的顆粒度：九值而非沿用前端七值

**考慮過**：① 沿用前端既有 7 個，把上架／下架併進 `*.write` ② 9 個，`publish` 從 `write` 分出來

**決策：② 9 個。** ① 是明擺著的技術債：需求書與 UI 都把「上架／下架」定為獨立能力、
風險等級「高」、須二次確認 ＋ 原因必填 ＋ 通知賣家；把它併進 `write` 等於「能編草稿就能上架」，
而 `write` 是中風險、`publish` 是高風險。

曾一度以「加 capability 會讓 admin 前端 fail-closed」為由選 ①——**該理由不成立**：
hawkr-admin 是自家守備範圍，同波修改即可，用「跨 repo 麻煩」換一個永久的權限顆粒度錯誤不划算。

## 7. capability 集合的演進權歸屬（新增第 10 個時誰先動）

**考慮過**：① 前端維持「未知 capability ＝整份契約作廢」，後端每次擴充都必須等 admin 先發布
② 前端改為「忽略未知 capability、保留已知」，兩 repo 同波發布即可

**決策：② 前端忽略未知、保留已知。**

**① 的 fail-closed 是零安全增益**：capability 集合的偽造防護權威**完全在後端**——
Gateway 是 capability 的唯一產生者、session 快照與 `credential_hash` 綁定、每次請求重驗。
前端就算原封不動照收所有值也不可能因此獲得任何多餘能力；它拒絕未知值擋不掉攻擊者，
只擋得掉自己人的正常演進。

**① 的代價卻是實的**：它把「後端加一個 capability」變成跨 repo 破壞性契約變更，
反序即**既有工作階段整批 502**（現存功能壞掉，不是新功能看不到）。
② 之後同一個反序只會讓新能力在 UI 上暫時看不見。

**② 不放寬任何禁區**：Delegated Adapter 不得出現 shipment method、UI 不得渲染履約 CTA
兩項仍是封閉斷言；② 放寬的只有「收到未知 capability 時怎麼處理」這一件事。

**② 的成立前提是後端絕不回空 `capabilities` 陣列**——空陣列仍判契約失敗。

## 8. 非 409 的 HTTP 狀態碼怎麼給

**考慮過**：① 為本功能加一個專用 response helper ② 在 `gateway/internal/response`
建立業務碼 → HTTP status 的統一映射表 ③ 改由微服務回不同的 gRPC status code，Gateway 依 status 分流

**決策：② 統一映射表。**

① 能解決本功能，但下一個需要分流的功能會再加一個 helper，最終變成每個功能一份對照邏輯。

**否決 ③：基數不足**——本功能的二十餘個業務碼要映到 6 種 HTTP status，同一個 status 底下
必須並存多個語意不同的碼（例如 422 同時承載「店家不可代行」與「參數不合法」），
gRPC status 分不出它們；而前端 message 一律不透傳 ⇒ 語意只能靠 code ⇒
**每個失敗情境必須有唯一數字碼**，禁止用 message 區分。③ 能決定 status，
但決定不了那個唯一數字碼，等於仍要另建一張表。

## 後果

- 代行的授權判定只有一個問答點；任何「在 handler 裡自己判一次」的寫法都是回歸。
- `/api/v1/admin/**` 這個前綴不再蘊含 `requireAdmin`（見
  `2026-09-18-adm-delegation-workspace-host.md` 的身分邊界一節），因此需要具名白名單的 boot 斷言。
