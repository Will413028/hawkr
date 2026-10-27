# 可申訴的措施（appealable measure）決策簡報 v2

快照（2026-10-12）：
- `<main>` = main checkout，已 commit 的是 `main` = `40e2aa5c7`；**[4A-3]** 標記的事實只在 4A-3 尚未 commit 時的工作區 diff。
- `<wt>` = `local/10e` 的 worktree，已 commit 的是 `fd4969b17`（2a = `6f6a23ee4`），merge-base `422349b06`。之後 `401cb424b` 合入 main（`40e2aa5c7`）時，2a 的 migration 改名為 `20261011180000_suspension_account_id.sql`；本文引用合併前的路徑。
- 業界基準（DSA、P2B、Google、YouTube、Meta）是**參考基準**，台灣沒有對應法規，不是我們的法遵義務。條款是 Will 的草稿（`seller-terms.md:3`「未經律師審閱」）：它是**現行 SSOT、Will 可以修訂**；下文需要時列出「修訂條款」的選項。

---

## 0. 一頁摘要

| 決策 | 業界基準 | 建議 | REAL 限制（只有這些讓建議偏離基準） |
|---|---|---|---|
| D1 可申訴的實體 | 「認定」（finding、strike、issue）與「措施」（decision：限制什麼、多久、理由、救濟）分開，N:M；申訴對象是措施（P2B Art.4、11；DSA Art.17、20；YouTube strike→終止） | **E**：新增 `Measure` aggregate（理由與版本、期間、申訴、凍結、`can_appeal`、註記、效果）；`violations` 退回只做認定（類別、嚴重度、升級、來源通報）；措施以依據表指向 0..N 筆認定，或帶風險依據 | 無（形狀不受限）。措施種類只做 WARNING、SUSPENSION：商品下架的執行者在 Java product（B4 前不在邊界內）、撥款保全沒有執行者（B10） |
| D2 管理員直接停權 | 不分依據，凡限制都有理由、可申訴；屢犯由多筆認定導出一個措施（YouTube、Meta） | 管理員停權＝措施：依據 RISK（條款 17.1 的風險類型＋17.5 外部要求）或 VIOLATION（新認定，或「拒不改善」時**引用先前的認定**）；刪除 `source = ADMIN`、`reason`、`detail` | 舊契約的 9601–9603 到步驟 11 前仍有出口：只做機械對照（Java 無人依代碼分支，已驗證） |
| D3 風險措施的期間與申覆次數 | Google MC：停權無固定結束日，「不同意」一次＋「已修正」可重複請求審查（第二次失敗後冷卻一週）；P2B Art.4(3)：可釐清事實、撤回後儘速恢復；DSA Art.23：「合理期間」 | **(ii′)**：RISK 措施可不設結束時間，但**必填複審期限**；賣家申覆一次（17.4），另有「補正後請求解除」入口（4.3 的補正方式，不算申覆，同時最多一筆待處理）；並修訂條款 17.4 寫明補正請求不計入申覆次數 | 17.5 外部要求（金流商、主管機關）的期間不在平台手上；17.4「申覆一次」是現行 SSOT（可修訂） |
| D4 admin 能做哪些動作 | Merchant API `Action.is_available`；AIP-216 | 每個動作一個 OUTPUT_ONLY 布林，含 `can_decide_appeal`；與前置檢查共用 model 函式 | 無 |
| D5 帳號範圍、冪等 | 多個措施可並存；建立以 AIP-155 `request_id` 冪等 | 拿掉 2a 的「每帳號最多一筆生效中 ADMIN」唯一索引；`CreateMeasure` 加 `request_id`（同時補上 `CreateViolation` 現在連點會建兩筆的缺口） | 無 |
| D6 給賣家的理由與讀取 | DSA 17(3)：種類、範圍、期間、事實、依據、救濟；P2B 4(5)：依法不得揭露時可不給 | 措施輸出種類、依據、理由（版本）、期間、`can_appeal`、申訴、補正請求；停權公告讀「生效中的措施」清單；接受申訴也必附理由（依 18.4，非問題） | 無 |
| D7 其他 admin 動作 | 下架、終止都是要附理由、可申訴的措施（P2B 4(1)(2)；DSA 17(1)） | 商品強制下架：B4 時成為措施種類，現在只記錄牴觸；撥款保全：B10；**admin 關店：見 Q3** | Java product、finance 不在邊界內 |
| 排序 | — | 4A-3 commit → `local/10e` 合入含 4A-3 的 main → **10e 第 2c 刀（措施模型）** → 合回 main → 10e 第 3 刀、4B、4C | 4A-3 屬另一個 session |

**要 Will 決定的**（§9）：Q1 模型 E 或 C；Q2 風險措施的期間 (i)/(ii)/(ii＋修訂條款)；Q3 admin 關店是否為措施（與條款 21.2 的 14 日預告）；Q4 UI 文案與新入口的同意。

---

## 1. 現況（只列和這題有關的）

| 事實 | 出處 |
|---|---|
| 條款 17.1：違規**或**「有具體事證顯示存在…風險」時，可以要求改善、限制曝光、暫停商品、停止接單、暫緩撥款、暫停帳號 | `<main>/website/docs/legal/seller-terms.md:173` |
| 17.2：情節重大、反覆違規、**拒不改善**時可不經事前通知先處理，儘速告知理由 | 同上 :174 |
| 17.3：措施應與風險相當，限於必要範圍及**期間** | 同上 :175 |
| 17.4：可就前述措施申覆，**不設期限；同一措施以申覆一次為限** | 同上 :176 |
| 17.5：不限制配合主管機關、法院、金融機構或第三方支付依法採取的措施 | 同上 :177 |
| 18.4：對下架、**停權**、長期撥款限制或其他重大措施有異議可申覆；由未直接作成原決定的人員**或具相當權限者**複核；以可保存方式回覆**結果及主要理由** | 同上 :184 |
| 4.3：限制功能或暫緩上架要說明理由及**補正方式**；依法或重大風險不得揭露者不在此限 | 同上 :50 |
| 5.5：補件、暫停販售或下架，依第十八條提供申覆管道 | 同上 :61 |
| 21.2：甲方得於**十四日前**通知終止；21.3：重大違約七日未改善、重大食安或詐欺等，得立即終止 | 同上 :205-206 |
| 處分表兩種來源：`ADMIN` 必有 reason、`ends_at` 必為 NULL；`VIOLATION` 必有 `violation_id` 與 `ends_at` | `main:backend/internal/modules/seller/db/schema.sql:392`（CHECK :406） |
| 管理員停權只由管理員解除；違規停權只能撤銷違規或接受申訴解除 | `suspension.go` 的 `Lift`（`local/10e` :141-164） |
| 申訴內嵌在違規列，每筆一次；接受即撤銷 | `schema.sql:270-305`；`CARRYOVER.md:73` |
| [4A-3] `model.CanAppeal`、`Frozen`（提出申訴起只能縮短） | `model/governance.go:438-451`；ADR 文末修訂 :122-128 |
| 違規的停權必有晚於現在的到期時間 | `governance/violation.go:49-50` |
| 升級規則只把第 3 筆 MEDIUM 的**嚴重度**標成 HIGH（`escalated_from`）；處分種類由 admin 逐筆選（`penalty` REQUIRED），嚴重度不機械決定處分 | `schema.sql:248,285`；`governance.proto` `CreateViolationRequest` |
| 賣家看到的停權只有 `source` 與 `reason` enum：沒有理由文字、ID、期間 | `seller.proto:326-331`、`:375` |
| 管理員停權原因三值：POLICY_VIOLATION、REPEATED_WARNING_IGNORED、OTHER | `model/model.go:66-81` |
| [10e] 處分加 `account_id`；「每帳號最多一筆生效中 ADMIN」partial UNIQUE；`Suspend` 在帳號鎖內遇到既有的就回傳那一筆（冪等） | `local/10e:…/migrations/20261011034226_suspension_account_id.sql`；`local/10e:…/suspension/suspension.go:86-129`；ADR 修訂 `local/10e` :109-111 |
| `CreateViolation` 沒有冪等鍵（repo 內沒有任何方法實作 AIP-155 `request_id`；錯誤模型 ADR 已預留它作為可重送的條件） | `rg -n request_id api/` 只命中 `api/testdata/error-dispatch.json`；ADR `2026-09-25-api-error-model.md:77` |
| website 停權公告對管理員停權顯示「提出申訴」，卡片導到沒有東西可申訴的違規頁；:105 的「停權僅能由平台管理員解除」對**違規停權**也顯示 | `SellerSuspendedNotice.tsx:105,147-151`；測試 `.unit.test.ts:132-145` |
| seller-app：「停權僅能由平台管理員解除」「提出申訴…聯繫平台客服」 | `suspension_notice_screen.dart:30` |
| plan：缺口記錄 :272；4A-3 :275；4B 暫定「管理員停權改指向客服」:279；4C 反轉申訴卡斷言 :283；10e 的「能否延長、能否改理由」:285 | `<main>/backend/docs/plans/2026-09-29-a3-buyer-seller.md`（工作區） |
| 商品強制下架：admin 已有 `ProductForceUnpublishDialog.tsx`（main 與 10e 都有），註解 :12-16 寫明「刻意沒有理由欄位」，打舊 REST `PUT /admin/products/{id}/status` | `admin/src/app/admin/merchants/[id]/ProductForceUnpublishDialog.tsx`；`openapi.yaml:8406` |
| 撥款保全：`rg -il "payout.?hold\|撥款.?(限制\|暫緩\|保全)" backend/internal` **零命中**；Java `aftersales-service/.../AfterSalePayoutHoldService.java` 是訂單層級的撥款資格閘門，不是治理措施 | — |
| admin 關店 `CloseStore`：理由必填，記在 `stores.close_reason`，賣家看不到；關店後帳號可開新 stint（帳號停權中不行） | `local/10e:api/…/admin/seller/v1alpha1/seller.proto:53,315-319`；`CARRYOVER.md:217,219`；`onboarding.go:66-69` |
| 裁定申訴不檢查裁定者；權限獨立；只有 `RoleOwner` 一個角色 | `violation.go` `DecideAppeal`（main :440，[4A-3] :475）；`adminperm.go:17,20,78` |

**現有機制為什麼長這樣**：公司時期的設計書 32 已有 §13「不服勸導」停權（私人封存的設計書 32 :560-566）；7b（`5208b940b`，2026-10-02）改寫成現在的措辭（`main` 設計書 32 :405-418）。ADR `2026-10-02` 把違規與處分拆開，管理員停權只當成處分表的另一種**來源**（ADR :69），ADR :40 也選了「人工停權是有理由的處分（DSA Art.17）」，但沒有處理申訴。條款草稿 2026-10-07 晚於 7b，第 17、18 條讓停權可申覆。所以缺口的直接起因是**條款改版之後模型沒跟上**。

---

## 2. 業界基準（已查官方或官方文字的出處）

| 來源 | 要點 | 驗證 |
|---|---|---|
| P2B Reg. (EU) 2019/1150 Art.4 | (1) 限制或暫停時（或之前）以可保存方式給理由；(2) 終止全部服務要 **30 日前**通知並附理由；(3) 給商家經 Art.11 內部申訴「釐清事實」的機會，撤回時「不遲延地」恢復；(4) 依法、重大理由或**反覆違反條款**時免 30 日；(5) 理由要指出具體事實與依據，依法不得揭露時可不給 | 已查，[legislation.gov.uk（EU 通過原文）](https://www.legislation.gov.uk/eur/2019/1150/article/4/adopted) |
| P2B Art.11 | 內部申訴涵蓋影響商家的措施；合理期間內處理；個別化告知結果 | 已查，[Art.11](https://www.legislation.gov.uk/eur/2019/1150/article/11/adopted) |
| DSA Reg. (EU) 2022/2065 Art.17 | 限制可見度、金流、服務、帳號都要附理由；(3) 種類、地域範圍、**期間**、事實、是否自動化、依據（法律或條款條號）、救濟 | 條文經轉載 OJ L 277 的 [springlex](https://www.springlex.eu/en/packages/dsa/dsa-regulation/article-17/) 核對；官方 [EUR-Lex](https://eur-lex.europa.eu/eli/reg/2022/2065/oj/eng) 本次無法讀取（回 202 challenge），**原文未直接核對** |
| DSA Art.20 | 申訴對象是「決定」（含暫停或終止帳號、限制金流）；至少 6 個月；(4) 理由充分時「不遲延地」撤回；(5) 附理由告知；(6) 不得只由自動化作成。**沒有**「只能申訴一次」 | 同上，[Art.20](https://www.springlex.eu/en/packages/dsa/dsa-regulation/article-20/) |
| DSA Art.23 | 針對濫用的暫停：事前警告、「合理期間」、逐案評估；條款要寫明政策與期間 | 同上，[Art.23](https://www.springlex.eu/en/packages/dsa/dsa-regulation/article-23/) |
| Google Merchant Center | 停權沒有固定結束日；請求審查時選「已修正」或「不同意」；**「不同意」只有一次**；第二次審查仍未解決時可能進入一週冷卻，之後可能更長；審查涵蓋該國全部帳號問題 | 已查，[Request a review](https://support.google.com/merchants/answer/13585221?hl=en) |
| Merchant API | 每個動作帶 `is_available`，不可用時告知去哪裡處理 | 已查，[issueresolution v1](https://developers.google.com/merchant/api/reference/rpc/google.shopping.merchant.issueresolution.v1) |
| YouTube | strike 是認定，90 天失效，**3 個 strike 導致終止**（N:1）；**每個 strike 只能申訴一次**、6 個月內；終止另外申訴（一年內最多兩次） | 已查，[strikes](https://support.google.com/youtube/answer/2802032?hl=en)、[appeal](https://support.google.com/youtube/answer/185111?hl=en)、[termination](https://support.google.com/youtube/answer/2802168?hl=en) |
| Meta | strike 累計導出限制（7 次封 1 天…10+ 次 30 天）；申訴成立時恢復內容並**移除 strike 與限制** | 已查，[Restricting accounts](https://transparency.meta.com/enforcement/taking-action/restricting-accounts/) |
| Amazon Seller Central | Account Health、Plan of Action、重送次數 | **未驗證**：官方說明在登入牆後，搜尋只得到第三方指南，不採用 |
| 蝦皮 | 計分制 | **未驗證**，刪除不用 |

**共通形狀**：認定（strike、issue、違規）與措施（對商家作成的決定，帶理由、期間、救濟）是兩個概念；一個措施可以由多筆認定導出（YouTube、Meta、P2B 4(4)「反覆違反」），也可以沒有認定（風險、法令、外部要求）。申訴對象是措施；措施因認定錯誤被撤回時，認定也一起撤回（Meta、YouTube）。申覆次數上，P2B、DSA 沒有限制，YouTube 與 Google 都把「不同意」限一次，但 Google 另外允許「已修正」重複請求。

---

## 3. D1：可申訴的實體

### 選項

| 選項 | 內容 | 對照基準 | 代價 |
|---|---|---|---|
| **A** 管理員停權一律改用 `CreateViolation` | 刪掉直接停權 | 偏離：風險措施沒有認定 | 風險暫停要硬塞 `OTHER` 類別；違規停權必有結束時間（`violation.go:49`） |
| **B** 申訴改成多型子表 | `appeals` 指向違規或管理員停權 | 偏離：申訴、理由、凍結分散兩處 | `CanAppeal`、`Frozen`、版本各兩份；賣家兩種清單 |
| **C** 把 `violations` 推廣成 `measures`（一張表，`basis` = VIOLATION／RISK，違規欄位以條件式 CHECK 限定） | 認定與措施**同一列**，1:1 | 部分：能表達無認定的措施；不能表達「一個措施由多筆認定導出」或「一筆認定先後導出兩個措施」 | 改名：表、sqlc、兩份 proto、website 4A 頁、事件、設計書 32 |
| **D** 另開 `risk_holds` aggregate，照抄申訴機制 | 違規不動 | 偏離：兩份申訴機制 | 與 B 同 |
| **E（建議）** 認定與措施分表 | `measures`：種類（WARNING、SUSPENSION）、依據（VIOLATION／RISK＋`risk_ground`）、理由與版本表、期間、申訴、註記、`can_*`、帳號；`measure_findings`（measure_id, violation_id）表達 0..N 筆認定；`violations` 只留類別、嚴重度、`escalated_from`、來源通報、狀態；`suspensions.measure_id` 是效果 | **符合** | 與 C 相近（同樣要搬移申訴、版本、註記與兩份 proto），多一張依據表與一條跨表規則（VIOLATION 依據至少一筆認定：model 層檢查＋測試，或延遲到 commit 的 constraint trigger） |

### E 比 C 多解決什麼（都是現在就有的流程）
1. **「拒不改善」（9602）**：條款 17.2 的反覆違規、拒不改善是「由先前的認定導出的措施」。E 直接引用那幾筆認定；C 只能新造一個 `NON_COMPLIANCE` 類別的認定（v1 的 Q2）。
2. **申訴成立的語意**：E 可以區分「認定錯誤」（措施與它一起建立的認定都撤銷，不再計入升級）與「認定成立、措施過重」（只撤銷措施）。C 只有一種：接受即撤銷違規。
3. **終局**：一筆認定先後導出警告與停權、或一個措施同時有停權與（B4 後）下架的效果，E 不必再改形狀。

### 申訴與認定的關係（E 之下的規則，由條款決定，不是問題）
- 申訴對象是措施（17.4、18.4）。一筆認定的申覆權附在**與它一起建立的那個措施**上；之後引用它的措施（例如拒不改善）是新的措施，可以再申覆一次（17.3、17.4，與 [4A-3]「加重是新的措施」一致）。
- 接受申訴：撤銷措施；裁定時選擇是否同時撤銷與它一起建立的認定（認定錯誤）。被引用的舊認定不因新措施的申訴被撤銷。理由必填（18.4）。

### 限制清單

| 讓設計偏離基準的限制 | 標記 | 來源 |
|---|---|---|
| 4A-1 到 4A-3、website 4A 已經以 `Violation` 為申訴單位實作 | SELF-IMPOSED（邊界內、未上線） | AGENTS.md「上線前不受相容性約束」 |
| 變更大小（E 比 C 多一張表） | SELF-IMPOSED | — |
| 和設計書 32 §13「不經違規認定」一致 | SELF-IMPOSED（和既有文件一致） | 設計書 32 :405-418 |
| 商品下架（Java product，B4）、撥款保全（無執行者，B10）沒有單體的執行者 | REAL（依賴不在邊界內，現在做是拋棄式工作）：只限制**這次做的措施種類**，不限制形狀 | `openapi.yaml:8406`；ADR :80；`go-modular-monolith.md` B4、B10 |
| Java 與 gateway 讀 9601–9603 | REAL（步驟 11 前不在邊界內），只影響舊契約的對照 | `legacy/sellerreason/reason.go:1-3` |

**建議 E。** 沒有 REAL 限制推離基準；C 的唯一優勢是少一張表（SELF-IMPOSED）。名稱問題因此消失：`Violation` 保留原意（認定），新 aggregate 叫 `Measure`。
- 終局：措施種類加上商品限制（觸發：B4 catalog）、撥款保全（觸發：B10 finance）、帳號終止（見 Q3）。

---

## 4. D2：管理員直接停權變成什麼

- 基準：P2B、DSA 不區分依據，凡限制都要理由與申訴；屢犯由多筆認定導出措施。
- 現有三個原因的去處（E 之下）：
  - POLICY_VIOLATION → VIOLATION 依據，與新認定一起建立（等於現在的 `CreateViolation` 加停權）。
  - REPEATED_WARNING_IGNORED → VIOLATION 依據，**引用先前的認定**（17.2）；升級規則照舊只看認定。
  - OTHER → RISK 依據，`risk_ground` 取 17.1 的交易、食安、詐欺、個資、侵權、資安、重大履約，加 17.5 的 `EXTERNAL_ORDER`。理由必填（取代選填、賣家看不到的 `detail`）。
- 17.5 外部要求的措施能不能申覆：**條款已回答**——18.4 涵蓋所有停權，17.5 只表示平台可以配合外部要求。申覆審查的是「外部要求是否存在、是否適用、是否仍有效」；要求仍有效時駁回並說明。理由可依 4.3 但書、P2B 4(5) 在依法不得揭露時只寫「依法配合」。**要律師確認**的部分只有措辭（未驗證，不是 Will 的產品決定）。
- 限制清單：舊契約對照是 REAL 但機械性（見 §8 legacy）；沒有其他 REAL 限制。

---

## 5. D3：風險措施的期間、凍結與申覆次數

### 選項
- **(i)** RISK 措施必須有結束時間，凍結前可延長。和 VIOLATION 依據同一套。代價：admin 要填猜的日期；到期時風險還在就得在到期前另立一筆重疊措施，否則有停權空窗；17.5 外部要求的期間平台根本不知道。
- **(ii)（建議）** RISK 措施可以不設結束時間，但**必填 `review_by`**（複審期限，到期進 admin 收件匣；複審只能結束或縮短措施、或延後下一次複審並記錄理由）。賣家：申覆一次（17.4，「不同意」）；另有**補正後請求解除**（4.3 的補正方式，「已修正」），不算申覆，同一措施同時最多一筆待處理，處理結果附理由。這就是 Google MC 的「不同意一次＋已修正可重複」。
- **(ii′)** 同 (ii)，另外修訂條款 17.4，明文寫出「依補正方式完成補正後，得請求解除，不計入申覆次數」，消除「補正請求是不是第二次申覆」的解讀風險。
- **(iii)** 不設結束時間、不複審、只能申覆一次。與 17.3「必要期間」、4.3「補正方式」牴觸。不建議。

### 凍結
- [4A-3] 的規則（提出申訴起只能縮短）對無結束時間的措施自然成立：結束＝把結束時間設成現在。延後 `review_by` 不是加重（措施內容不變），凍結後仍可做。
- 不受依據影響、一律沿用：待裁定時不能撤銷；撤銷的措施與認定不計入升級。
- VIOLATION 依據的措施：照舊必有結束時間（設計書 32、`violation.go:49`），不加補正請求（到期即恢復）。

### 限制清單

| 限制 | 標記 | 來源 |
|---|---|---|
| 17.5 外部要求的期間不由平台決定 | **REAL**（第三方） | 條款 :177 |
| 17.3 限於必要期間 | REAL（現行 SSOT，可修訂）；指向「有期限**或**複審點」，不指向寫死結束時間 | :175 |
| 17.4 同一措施申覆一次 | REAL（現行 SSOT，可修訂；DSA、P2B 沒有這個要求） | :176 |
| 「和違規依據同一套規則、零特例」 | SELF-IMPOSED（和既有程式一致） | — |

**建議 (ii′)**（(ii) 的機制＋條款明文，避免補正請求被解讀成第二次申覆）；v1 建議的 (i) 理由是 SELF-IMPOSED，撤回。見 Q2。
- 終局：複審期限的提醒在 A8 之前只出現在 admin 收件匣（觸發：A8 notification 時加通知）。

---

## 6. D4、D5、D6

### D4：admin 能做哪些動作
- 基準：Merchant API `is_available`；本 repo 的 `can_resubmit`、[4A-3] `can_appeal`。
- **建議**：措施輸出 `can_update_statement`、`can_extend`、`can_shorten`、`can_end`（RISK）、`can_revoke`、`can_decide_appeal`、`can_decide_remediation`，全部由 model 的同一組函式產生，與 `UpdateMeasure`、`RevokeMeasure`、`DecideAppeal` 的前置檢查共用。`can_decide_appeal` 現在就輸出（v1 省略它的前提在第二個角色出現時就失效）。REAL 限制：無。

### D5：帳號範圍與冪等
- 措施帶 `account_id`（違規本來就以帳號讀取，`CARRYOVER.md:73,252`）；效果表 `suspensions` 保留 2a 的 `account_id` 與帳號範圍讀取。
- **拿掉** 2a 的 `suspensions_admin_active`：基準是多個措施可以並存（例如食安風險與金流商要求同時存在），各自理由與申訴。
- **冪等（這一刀內一起做，不留給之後）**：2a 的唯一索引是 `SuspendStore` 現在的冪等手段（帳號已有生效中的就回傳那一筆，`suspension.go:107-110`）。選項：
  - (a)（建議）`CreateMeasure` 加 AIP-155 `request_id`（client 產生 UUID，`measures.request_id` UNIQUE；同一 `request_id` 重送回傳同一筆，內容不同回 `invalid_argument`）。錯誤模型 ADR 已把「帶 `request_id`」列為可自動重試的條件（ADR `2026-09-25-api-error-model.md:77`；`api/testdata/error-dispatch.json:43`），這會是第一個實作。順帶補上 `CreateViolation` 現在沒有冪等、連點會建兩筆的缺口（它併入 `CreateMeasure`）。
  - (b) 自然鍵「同帳號、同依據、同 `risk_ground` 最多一筆生效中」：擋掉合法的並存（同類風險的兩個獨立事件），不建議。
  - (c) 不做：比 2a 退步。
- admin UI 建立前顯示帳號已生效中的措施（UI 層，不是約束）。

### D6：給賣家的理由與讀取
- 措施對賣家輸出：種類、依據（認定的類別，或風險依據；條款條號由 client 呈現層對照）、理由（必填、有版本）、期間（`SanctionPeriod`；RISK 無結束時間時為空）、`can_appeal`、申訴、`can_request_remediation`、補正請求。「是否自動化」不輸出：沒有自動化措施，觸發是第一個自動化措施。
- 停權公告：讀 `ListMyMeasures` 的「生效中」篩選（可能多筆，各自連到詳情），`GetMyStore` 只保留能力判斷需要的停權狀態，`SuspensionSource`、`SuspensionReason` 刪除。理由：措施可並存，單一 ID 不夠，清單是唯一來源（Google 的 Needs attention、Amazon 的 Account Health 都是清單）。
- **依 SSOT 執行，不交給 Will**：
  - 接受申訴也必附理由（18.4「結果及主要理由」）：改 `schema.sql:301` 的 `(appeal_status = 'ACCEPTED' OR appeal_decision_reason IS NOT NULL)` 與 `seller.proto:902`、`governance.proto:212` 的註解。
  - 複核者：18.4 是「未直接作成原決定的人員**或**具相當權限者」，現在只有 `RoleOwner`（`adminperm.go:17,20`），OWNER 就是具相當權限者，現在不擋符合條款。觸發：第二個角色出現時，加「裁定者不得是作成者，除非具 OWNER」的檢查與 `can_decide_appeal` 的對應。

---

## 7. D7：其他 admin 動作是不是措施

| 動作 | 現況 | 條款與基準 | 建議 |
|---|---|---|---|
| 商品強制下架 | `ProductForceUnpublishDialog.tsx` 打 Java product 的舊 REST，沒有理由、不能申訴 | 5.5、18.4 要求可申覆；P2B 4(1)、DSA 17(1) | REAL：執行者在 Java product，B4 前不在邊界內，現在包一層措施是拋棄式工作。**現在只記錄牴觸**（§10 #9），文案不承諾申訴；終局：B4 catalog 時成為措施種類「商品限制」，效果在 catalog。系統未上線、沒有實際營運，過渡期沒有賣家受影響 |
| 撥款保全 | 沒有執行者 | 17.1、18.4 | REAL：B10 finance 時加措施種類 |
| admin 關店 `CloseStore` | 理由必填但賣家看不到；立即關閉；帳號之後可開新 stint；關店後失去什麼（店號、評價、追蹤者是否延續）**未查證** | 條款 21.2：平台終止要 **14 日前通知**；21.3：重大違約等可立即終止；18.4「其他重大措施」可申覆；P2B 4(2)：終止要 30 日前通知並附理由，4(3) 可申訴 | **Q3**（條款「其他重大措施」是否涵蓋關店沒有寫明，且現行實作與 21.2 的預告牴觸） |

---

## 8. 影響範圍與重工

### 10e 第 2a 刀（`6f6a23ee4`）的重工
| 部分 | 規模（2a 的 diff） | 去向 |
|---|---|---|
| admin proto：`SuspendStore`、`LiftSuspension`、`StoreSuspension.reason/detail`、`GetStore.suspension_source/active_suspensions`、`ListSuspensions` | `seller.proto` +16 行及生成碼 | `SuspendStore`／`LiftSuspension` 併入 `CreateMeasure`（RISK）／`EndMeasure`；`active_suspensions` 改成生效中的措施；`ListSuspensions` 改成帳號的措施清單 |
| admin UI | `SuspendStoreDialog.tsx` 163、`LiftSuspensionDialog.tsx` 111、`SuspensionHistoryCard.tsx` 153、`SuspensionStatusCard.tsx` 115 行；`store-detail.ts` 321 行中的停權部分；測試 `merchant-suspension-status-card.test.ts` 172、`merchant-store-detail.test.ts` 550 中的停權部分、`admin-governance-modal-a11y.test.ts` 部分 | 四個元件改寫成「建立風險措施」「結束措施」「措施歷史」「生效中措施」；版面沿用 |
| 後端 | `suspension.go` 的 `Suspend`／`Lift`（+89/-）、`suspensions.sql`（+28）、`httpapi/seller/admin.go`（+14）、`admin_integration_test.go` 的停權部分、`sellerreason`（+7） | `Suspend`／`Lift` 刪除，由措施服務寫效果列 |
| schema | migration `20261011034226_…`（27 行） | `account_id` 與帳號索引保留；`suspensions_admin_active` 刪除 |
| ADR 修訂（`local/10e` :109-111） | 1 段 | 「帳號為鍵」保留；「每帳號最多一筆 ADMIN」由新修訂取代 |
| **不受影響** | `CloseStoreDialog`、`SellerAccountDeleteDialog`、`GovernanceNotice`、帳號範圍讀取、gateway 停權 4 條路由的刪除 | — |

### 其他
- **資料與 migration**（未上線，只有本機與測試資料）：新增 `measures`、`measure_findings`、`measure_revisions`（由 `violation_revisions` 搬）、`measure_notes`（由 `violation_notes` 搬；ADR :120 的「註記只掛在違規」改成掛在措施）、`remediation_requests`（若 (ii)）；`appeal_evidence`、`evidence_accesses` 改指向措施；`violations` 刪除申訴、版本、處分相關欄位；`suspensions.violation_id` → `measure_id NOT NULL`，刪 `source`、`reason`、`detail`；「VIOLATION 依據的停權必有到期」改為措施層的規則。回填：每筆違規 1:1 產生措施（沿用 4A-2「版本序列第一個缺號是申訴」的精確重建）；既有 ADMIN 列轉 RISK 措施，理由取 `detail`，沒有時取原因標籤。
- **契約**：賣家 `ListMyViolations`／`GetMyViolation` → `ListMyMeasures`／`GetMyMeasure`（認定內嵌為依據），`SubmitAppeal` 改指措施，新增 `RequestRemediation`（若 (ii)）；admin `CreateViolation`／`UpdateViolation`／`RevokeViolation`／`DecideAppeal`／註記 → 措施版本，另加 `EndMeasure`、`DecideRemediation`。
- **事件**：`ViolationCreated` 等改成措施事件。治理事件**不 relay 到 Java**（`legacy/sellerrelay/relay.go:9-11`；`rg "Violation|Appeal" relay.go` 無命中），只影響單體與 A8 的訂閱。
- **10e 第 3 刀**（plan :285）：消費措施清單（預設篩選「申訴待裁定」與「補正待處理」、「複審到期」）、兩種建立表單（認定＋措施、風險措施；拒不改善時選先前的認定）、詳情（版本、註記、佐證、效果、依據）、依 `can_*` 的操作；gateway 路由刪除清單不變。
- **4B**（plan :277-283）：:279「管理員停權改指向客服」作廢，所有停權公告連到生效中的措施；App 的「違規與申訴」改讀措施；申訴表單指向措施；補正請求入口（若 (ii)，UI 要同意）。**4B 必須等新契約**，否則重做。
- **4C**：:283 的斷言改成「申訴卡連到那一筆措施」；同時修 :105 對違規停權也錯的「只能由管理員解除」。
- **legacy**（已驗證 Java 不依 9601–9603 分支）：`rg "reasonCode\(\)|SellerSuspensionReason\." -g '*.java' -g '!*Test*' backend/services` 唯一命中是 product-import 的無關欄位；gateway 只驗證代碼是否合法（`gateway/internal/middleware/auth.go:89,124`）；notification 模板只讀 `reasonLabel`（`notification-service/db/migrations/20260714010000_1514_…sql:10-11`）；im 只記 log。所以 `sellerreason.Of` 改成：RISK 依據 → 9603「其他」，VIOLATION 依據 → 照舊 `SUSPENDED_GOVERNANCE`（無原因碼）；`reasonLabel` 取依據名稱。舊 notification 模板「如有疑問請至申訴頁提出申訴」對所有停權送出，改完之後這句話才成真（申訴頁要能找到該措施，website 4C 一起做）。admin 的 `codemeta/generated.ts:274-284` 是凍結的生成物，不改。

---

## 9. 要 Will 決定的問題（SSOT 回答不了、且有真取捨）

**Q1 可申訴實體的模型。**
- E：認定與措施分表（N:M）。符合基準；可直接表達拒不改善、可區分「認定錯誤」與「措施過重」；多一張依據表與一條跨表規則。
- C：把違規推廣成措施（1:1，`basis` 加條件式 CHECK）。少一張表；拒不改善要新增 `NON_COMPLIANCE` 類別；之後出現多對一或一筆認定多個措施時要再拆表。
- 建議：E。

**Q2 風險措施的期間與補正。**
- (i) 必有結束時間；(ii) 可不設結束時間＋必填複審期限＋補正後請求解除（不算申覆）；(ii′) = (ii) 並修訂條款 17.4 明文寫出補正請求不計入申覆次數；(iii) 無期限、不複審。
- 附帶：複審期限的上限天數（例如 30 天）。
- 建議：(ii′)，上限 30 天（無業界數字可依，屬產品判斷）。

**Q3 admin 關店是否為措施。**
- 現況：立即關閉、理由賣家看不到、不能申覆；與條款 21.2「十四日前通知」牴觸（21.3 的立即終止情形除外）。
- (a) 成為措施種類「終止」：給賣家理由、可申覆一次；非 21.3 情形時排定 14 日後關閉。(b) 維持 admin 工具，修訂條款（例如把 admin 關店限定為 21.3 情形並寫明申覆管道）。(c) 維持現狀，只把理由給賣家看。
- 建議：(a)，可在 2c 之後另一刀做（不影響措施模型的形狀）。

**Q4 UI 改動的同意**（AGENTS.md：版面、文案、流程要先同意）：賣家端「違規與申訴」的名稱與內容改成措施（候選「處置與申訴」）；停權公告改連到措施；(ii) 的「補正後請求解除」新入口；admin 的停權對話框改成建立風險措施。

**已由 SSOT 回答、不再提問**：接受申訴附理由（18.4）；複核者現在不擋（18.4「或具相當權限者」）；外部要求措施可申覆（18.4、17.5）；申覆不設期限（17.4）；每措施申覆一次（17.4，Q2 提供修訂選項）；B4、B10 前不做商品與撥款措施（REAL 依賴）。

---

## 10. SSOT 互相牴觸之處

1. 條款 17.4、18.4（停權可申覆） vs 設計書 32 §13 與 §3.3、`suspension.go` `Lift`、`schema.sql` 處分註解（管理員停權只由管理員解除）。本題。
2. 條款 17.3（必要期間） vs 管理員停權 `ends_at` 必為 NULL（`schema.sql:406`；設計書 32 :418）。
3. 條款 18.4（回覆結果及主要理由） vs 接受申訴可不附理由（`schema.sql:301`）。依 SSOT 修。
4. 條款 18.4（未直接作成者或具相當權限者複核） vs `DecideAppeal` 不檢查。現在只有 OWNER，不構成違反；觸發是第二個角色。
5. 設計書 32 :135「每個店家最多一筆生效中的管理員停權」與 ADR :69 同句 vs [10e] ADR 修訂（`local/10e` :111）以帳號為鍵；設計書 :422 寫的是「以 store_id 記來源、效果以帳號為單位」，不牴觸。本案通過後兩者都被取代。
6. 設計書 32 §13.4、§13.5（:428-456）仍寫 gateway REST、Redis、outbox，已被 ADR `2026-10-02` 取代。
7. ADR :40 引用 DSA Art.17「人工停權也是有理由的處分」 vs 賣家看不到管理員停權理由（`seller.proto:326-331`）。
8. website 對管理員停權承諾可申訴卻無處可申訴（`SellerSuspendedNotice.tsx:105,147-151`），:105 對違規停權也寫錯；plan :279 的暫定方案（指向客服）也與 18.4 不一致；舊 notification 模板對所有停權寫「至申訴頁提出申訴」（`20260714010000_…sql:10`）。
9. 條款 5.5、18.4（下架可申覆） vs `ProductForceUnpublishDialog.tsx:12-16` 沒有理由、不能申訴。REAL 依賴，終局 B4。
10. 條款 21.2（平台終止要 14 日前通知） vs `CloseStore` 立即關閉（Q3）。

---

## 11. 排序

1. **4A-3 commit**（另一個 session；REAL 依賴，不在我們控制內）。
2. `<wt>` 進行中的 merge（main `40e2aa5c7`，不含 4A-3）完成後，**再合一次含 4A-3 的 main**；預期衝突：`schema.sql`、ADR、`CARRYOVER.md`、`governance/violation.go`、`governance_integration_test.go`、`seller.proto`（merge-report 已處理過前一輪的同類衝突）。
3. **10e 第 2c 刀（措施模型）**，在 `local/10e` 做：它同時改 4A-3 的違規與 2a 的停權，只有這條分支兩者都有。名稱用 2c，避免與 10c 已有的「3-0 刀」（plan :243）混淆。
4. `local/10e` 合回 main，之後才開 4B、4C（消費賣家契約）與 10e 第 3 刀（消費 admin 契約）。A8 訂閱的是措施事件。
5. Q3 若選 (a)：在 2c 之後另一刀。

替代排序：先把 `local/10e` 合回 main，2c 在 main 上做。差別只在合併次數；選哪個取決於 10e 的 session 安排，不影響設計。

---

## 12. 對 critique 的處理

| # | 處理 |
|---|---|
| 事實表「migration 檔名 WRONG」 | **駁回**：已 commit 的 `local/10e` 是 `20261011034226_…`（`git -C <wt> ls-tree -r --name-only local/10e \| rg suspension_account_id`）；`180000` 是進行中的 merge 改的名（scratchpad `merge-report.md:20`）。快照段已說明 |
| 1 HIGH D3 建議用 SELF-IMPOSED 理由 | 採納：改建議 (ii)，ledger 加 17.5 REAL，17.4 標為可修訂，加 (ii′) |
| 2 HIGH 缺認定／措施分表選項 | 採納：加 E 並建議 E |
| 3 MED Q5、Q6 可由 SSOT 回答 | 採納：§6 D6 |
| 4 MED 條款標 REAL | 採納：全文標「現行 SSOT，可修訂」，加 (ii′) |
| 5 MED 冪等被推成另一題 | 採納：§6 D5 |
| 6 MED Q7 框定偏誤 | 採納：§7 中立陳述並查到 21.2 的 14 日預告（新的牴觸 #10） |
| 7 MED 下架沒列進牴觸 | 採納：§7、§10 #9 |
| 8 MED 兩個未驗證可結案 | 採納並重新驗證：§8 legacy、事件 |
| 9 LOW 未說明 EU 法規只是基準；P2B 4(2)、4(5) | 採納：快照段、§2 |
| 10 LOW 公告連哪筆 | 採納：§6 D6（讀生效中清單） |
| 11 LOW `can_decide_appeal` | 採納：現在就輸出 |
| 12 LOW 行號 | 採納：ADR OWNER → `adminperm.go:17,20`；設計書 :422 更正；4A-3 在 plan :275；`DecideAppeal` 行號依版本分列 |
| 13 LOW notification 模板 | 採納：§8 legacy、§10 #8 |
| 排程：量化 2a、4A-3 是 REAL | 採納：§8、§11 |

## 13. 沒有驗證的部分

- DSA 條文經 OJ 轉載網站核對，EUR-Lex 原文本次無法讀取。
- Amazon 的申訴流程（登入牆）與蝦皮：未驗證，未作為依據。
- admin 關店後帳號重新開店時失去或延續什麼（店號、評價、追蹤者）。
- 17.5 外部要求措施的申覆措辭是否需要律師調整。
- E 的工作量只按 2a 與 4A 的檔案估計，沒有逐檔盤點 website 4A 頁面與 seller 契約的消費端。
