# 賣家收款開通：經 provider 中立的 payment port，作品集接 Stripe Connect 測試模式

| 項目 | 值 |
|---|---|
| 狀態 | accepted（Will，2026-10-29 至 2026-11-14 分次決定；各決定的日期在決策段） |
| 日期 | 2026-11-13 |
| 影響範圍 | `internal/modules/payment`（port、adapter、`payment.accounts`）、`internal/platform/stripe`、`internal/platform/inbox`、`internal/modules/seller`（開通閘門、`GetMyOnboarding`）、`internal/modules/reviewqueue`、`api/hawkr/payment`、`api/hawkr/seller`、`api/hawkr/admin/payment`、`api/hawkr/identity`、website、admin、seller-app |
| 相關 | 收款帳戶歸 payment、webhook inbox 由 platform 提供，已寫在 `2026-09-24-go-modular-monolith.md` 文末 2026-11-06 修訂，本檔只指過去；平台審核、防詐身分比對、入駐同意與個資保留見 `2026-11-13-seller-platform-review-and-identity-matching.md`；文案分界見 `2026-10-17-multi-locale-architecture.md` |

## 脈絡

賣家要先通過平台審核，再讓金流商開通收款帳戶，店家才能公開曝光。公司時期的做法是平台自己收身分證號、銀行帳號與證件，審核後以 PayUni 代理商 API 建子商店。單體搬遷時要決定收款開通怎麼放；作品集既拿不到 PayUni 測試帳號，也不能在台灣示範任何在地金流。

約束：
- 外部：沒有 PayUni 測試帳號，代理商 API 要簽約才開通（Will 2026-10-29 決定不簽）；PayUni 程式因此無法對真實 sandbox 驗證。
- 外部：Stripe 不接受台灣的平台帳號，Connect 跨境撥款只開給平台在 US、UK、EEA、CA、CH；沒有台灣的超商代碼、ATM、LINE Pay、街口（https://stripe.com/global、https://docs.stripe.com/connect/cross-border-payouts、https://docs.stripe.com/payments/payment-methods/payment-method-support，2026-10-29 核對）。Stripe adapter 因此只示範機制。
- 外部：平台自己收款再轉給賣家可能落入《電子支付機構管理條例》的代理收付（第 3、5 條；第三方轉載，未在全國法規資料庫核對，未確認）。只影響真實上線。
- 外部：Stripe v2 有 `configuration.recipient` 的帳戶必須帶 `contact_email`（sandbox 實測，兩種 dashboard 都回 400）；v2 的 Account Link 在一般測試模式不能建，要用 sandbox。
- 外部：webhook 會重送、不保證順序（https://docs.stripe.com/webhooks）。
- 外部：Stripe v2 的冪等鍵保存 30 天；v2 失敗的請求不重播，同一把鍵改對內容後可以成功（sandbox 實測）。
- 繼承：系統從未上線，沒有相容性與資料搬移約束（根 `AGENTS.md`）；現在仍成立，上線後失效，之後的改動要走 expand-contract。
- 繼承：外部呼叫不在 transaction 內、webhook 先寫 inbox（`backend/AGENTS.md` 單體規則 5）；仍成立：transaction 內打外部 API，失敗時會留下半成品。
- 繼承：可曝光公告與 social 的可見性在 SQL 端直接讀 view `seller.store_requirement_flags`，不能呼叫 Go；仍成立：它們是資料庫函式。
- 繼承：使用者面文字依目錄分界（`2026-10-17-multi-locale-architecture.md`）；仍成立。
- 繼承：已套用的 migration 不得修改（`backend/AGENTS.md` 單體規則 6）；仍成立，但只在分支新增、尚未併入 `dev` 的 migration 沒有已套用的對象，可在合併前壓平。
- 不再成立而拿掉：平台先收身分證號、銀行帳號、證件再送金流商，以及「平台與 PayUni 兩關都過才能收單」。理由：平台持有最多個資，沒有業界基準支持。

## 考慮過的方案

「基準」是同樣條件下、假設還沒有程式碼時的業界做法；標「推測」的是沒有核到官方出處。未採方案的理由在決策段。

| 決定 | 基準（業界做法與來源） | 採用 | 未採 |
|---|---|---|---|
| 收款 adapter | 平台把開通交給 connected account 的 provider（Stripe Connect recipient 帳戶，https://docs.stripe.com/connect/accounts-v2） | Stripe Connect 測試模式，PayUni adapter 刪除 | PayUni adapter 留在 port 後面；簽 PayUni |
| port 的兩軸 | Stripe 分開「誰收資料」與「用什麼介面」（https://docs.stripe.com/connect/onboarding、https://docs.stripe.com/connect/hosted-onboarding） | 收件方式是 port 的軸（預設金流商收件）；session 的形狀由帳戶列的收件方式與呼叫端宣告的內嵌能力決定，不是第二個軸 | 預設平台收件；平台自建表單再送出 |
| 內嵌元件的使用者驗證 | 平台負責補件的帳戶可關掉 Stripe 登入視窗，代價是平台承擔負餘額（https://docs.stripe.com/connect/get-started-connect-embedded-components?platform=web） | 關掉 | 維持預設 |
| 測試平台國別 | 平台與 connected account 在支援的國別組合內（脈絡的外部約束） | US | SG、JP、US 平台跨境撥到台灣 |
| Stripe API 與 client | 新整合用 Accounts v2，事件用 thin events 並重讀物件，client 用官方 SDK（https://docs.stripe.com/connect/accounts-v2、https://docs.stripe.com/webhooks） | v2、官方 stripe-go v87.1.0 釘的 API 版本、收到後一律重讀 | 手寫 HTTP client 加釘 preview 版本；舊 preview；v1 Accounts；信任事件內容 |
| 狀態模型 | 完成看 `stripe_transfers.status`，行動方看 `status_details` 的 `resolution`（https://docs.stripe.com/connect/marketplace/tasks/onboard） | 5 值，由 `stripe_transfers` 推導 | 加 `ONBOARDING`；自創「已送出」；只存原始 JSON |
| 閘門與收款模式 | capability 加 requirements 當授權依據（`2026-09-18-seller-capability-authorization.md` 2026-10-08 修訂）；市集用 destination charges 是推論，未核出處（推測） | 平台審核核准且轉帳能力 active；destination charges | 只看平台審核；看 `charges_enabled` |
| 建立時點、冪等與分步 | 隨機 v4 UUID 當冪等鍵，v2 鍵期限 30 天（https://docs.stripe.com/api/idempotent_requests、https://docs.stripe.com/api-v2-overview#idempotency） | 核准後建立，保存的 UUID 當鍵，先寫本地列；建立請求內容固定，信箱與設定另以更新寫入 | 送出即建；首次收款才建；每次新鍵；管理員補件；冪等鍵與信箱一起存、原樣重送 |
| 聯絡信箱 | 用平台已有的使用者信箱建帳戶，持有人可在開通頁面改（https://docs.stripe.com/connect/marketplace/quickstart、https://docs.stripe.com/connect/hosted-onboarding） | 帳號層級，建立外部帳戶的當下讀取，不存副本 | 直接用登入信箱；KYC 申請欄位；客服信箱；電話；核准時快照到收款帳戶列 |
| 快取不倒退 | 事件不保證順序，收到後重讀（https://docs.stripe.com/webhooks）；寫入條件沒有統一名稱（推測） | 讀取開始時間不早於 `fetched_at` 才寫 | 排 job 並等結果；整列覆寫不比時間 |
| 開通狀態與下一步 | 伺服器整理 requirements，平台只呈現（https://docs.stripe.com/connect/onboarding）；讀取方法無副作用（https://google.aip.dev/131）；下一步由伺服器決定、狀態靠 webhook 與自動重讀、動作只在一個設定頁執行（Will 2026-11-14 與業界做法比較時的描述，未逐項核出處） | 伺服器推導，下一步只讀任務的 `action`；唯讀 `GetMyOnboarding` 加寫入類 `RefreshMyPaymentAccount`，client 自動呼叫 | client 各自推導；共用 client 函式庫；讀取帶 `refresh`；賣家端手動重新整理按鈕 |
| 文案歸屬 | 同一個 enum 的對照表只寫一份（`2026-10-17-multi-locale-architecture.md`） | 標題與狀態句由伺服器，說明與標籤進目錄 | 伺服器送說明；兩端各寫 |
| 模組歸屬 | 資源的 API 在擁有者的 service，擁有者驗證留在上層入口（`internal/modules/payment/CARRYOVER.md`） | 賣家端 RPC 留 seller，後台 RPC 在 payment | 全搬 payment；全留 seller |
| App 內開通 | Stripe 有 iOS、Android 的 account onboarding 元件（https://docs.stripe.com/connect/supported-embedded-components） | 外部瀏覽器加 website 靜態頁，回前景重讀 | App 內嵌 SDK；deep link |
| 個資清理與外部帳戶 | 計畫沒有記業界基準 | 清理先關閉外部帳戶、成功才刪本地列 | 只刪本地對應、關閉延後到上線前 |
| 開通 session 的內容 | 計畫沒有記業界基準 | session 帶 provider 與公開金鑰，金鑰由後端一處設定 | website 的 build 設定放金鑰，延後到接第二家金流商 |
| 回跳網址 | 計畫沒有記業界基準 | client 傳目的地 enum 與語系，伺服器用 `PUBLIC_WEB_ORIGIN` 組網址 | client 傳完整網址 |
| 契約刪欄位與 `reserved` | `reserved` 防的是已發佈的 client 與已存的資料 | `v1alpha1` 不留，改名 `v1` 之後才留 | 維持現況，上線前也留 |
| fake provider 的環境 | 計畫沒有記業界基準 | 只能在 dev 環境選到，不成立時啟動失敗 | （計畫沒有列未採方案） |
| 分支內 migration | 計畫沒有記業界基準 | 分支新增、尚未併入 `dev` 的 migration 在合併前壓平，對 `dev` 的任何合法資料都能套用 | 照現況合併；只刪「種子資料會違反」的列；一句都不刪 |

## 決策

賣家收款開通只經 provider 中立的 payment port。作品集的 adapter 是 Stripe Connect 測試模式，預設由 Stripe 收件加託管頁面，平台收件的帳戶在能承載內嵌元件的 client（website）用內嵌元件、其餘用託管頁面；平台不收身分證號、銀行帳號與證件；PayUni adapter 刪除。店家公開曝光要平台審核核准且轉帳能力 active，狀態、任務清單、下一步與閘門由伺服器從同一組來源欄位推導，client 只呈現。

**收款 adapter**（Will 2026-10-29、2026-11-02）：Stripe Connect 測試模式，單體的 PayUni adapter 刪除，PayUni 的付款、退款、撥款與物流留在 Java 舊服務，由單體計畫的 B2、B3、B8–B10 搬。
- 未採：PayUni adapter 留在 port 後面當第二個 adapter：沒有測試帳號無法驗證，會隨 port 演進失真；git 找得回。
- 未採：簽 PayUni：測試帳號要簽約，作品集要展示國際標準。

**port 的能力描述與 session 的形狀**（Will 2026-10-29、2026-11-02；2026-11-14 改為 D58）：port 的能力描述有 `requirement_collection`（`provider` 金流商收件、`platform` 平台收件，帳戶列上也存；行程設定只決定新列寫入的值）與 `embedded_runtimes`（provider 能開內嵌 session 的執行環境，Stripe 是 `STRIPE_CONNECT_JS`，fake 沒有）。開通 session 的形狀不是 port 的第二個收件軸：由 `account.ChooseSession` 唯一決定，條件是列保存的收件方式是平台收件，而且 provider 支援的與呼叫端宣告的內嵌執行環境有交集才回內嵌（Account Session），其餘一律回託管連結（Account Link；平台收件的帳戶走託管連結可行，`spike/README.md` Q23）。預設是金流商收件加託管頁面。port（`Provider`）的方法是 `Name`、`Capabilities`、`EnsureAccount`、`ResolveAccount`、`CreateOnboardingSession`、`FetchAccount`、`CloseAccount`、`ParseEvent`，沒有 `SubmitKYC`，平台不自建 KYC 表單，也不再收身分證號、銀行帳號與證件；統編是市集登記資料，仍由平台收。這修訂了先前的「介面由收件方式決定」（D2）。
- 未採：預設平台收件：沿用舊畫面最多，但平台持有的個資最多。
- 未採：兩種收件都由平台自建表單再送出：兩邊重複收個資。
- 防詐訊號改用銀行帳號 fingerprint 與統編的雜湊，細節在平台審核 ADR。觸發重建：決定接台灣金流商。

**內嵌元件關掉 Stripe 使用者驗證**（Will 2026-11-03）：平台收件的 Account Session 帶 `disable_stripe_user_authentication`。賣家已用 Google 或 LINE 登入，平台收件的目的就是賣家留在平台頁面完成。代價：平台承擔負餘額；本平台的帳戶本來就由平台承擔損失（`losses_collector=application`），所以代價已在。託管頁面不受影響。
- 未採：維持預設：賣家每次開通都要另外登入或註冊 Stripe。

**測試平台帳號是 US**（Will 2026-11-02）：US 平台，沿用不改。
- 未採：SG：平台收件的帳戶卡在 `proof_of_liveness`，沒有 API 參數可繞。
- 未採：JP：KYC 欄位與文件以日文為主。
- 未採：US 平台跨境撥到台灣：台灣只能開 transfers，離上線情境更遠。

**Stripe API 與官方 SDK**（Will 2026-11-02、2026-11-03；改用 SDK 2026-11-13）：帳戶用 Accounts v2（configuration `recipient`，capability `stripe_transfers`）。client 用官方 `github.com/stripe/stripe-go/v87` v87.1.0（`internal/platform/stripe` 在 SDK 之上只加錯誤分類、429 退避與總逾時），所有呼叫（v2 帳戶、Account Link，以及 v1 的 Account Session 與外部銀行帳號列表）都用 SDK 釘的 API 版本 `2026-10-28.endive`，不逐次覆寫 `Stripe-Version`。當初為了 capability 的 `rejected` 狀態與 Account Link 不帶 `configurations` 而釘 preview，這兩項都已進 GA；sandbox 實測此版本下建立、Account Link、讀取、兩支 v1 呼叫與關閉都成功。webhook 只收平台自己的 `v2.core.account` 開頭的 thin events：簽章用 SDK 驗，之後只讀事件 id、type 與相關物件，收到後一律重讀帳戶，不信事件內容；驗過簽章但不處理的事件回 2xx，不寫 inbox。
- 未採：手寫 HTTP client 並把理由（釘 preview、不新增依賴）記成沿用：手寫的必要條件是沒有官方 SDK，對 Stripe 不成立。
- 未採：v87.0.0 加 `2026-09-30.endive`：拿不到 87.1.0 對 webhook secret 與重試的修正。
- 未採：v87.1.0 逐次覆寫回舊的兩個版本：每個呼叫都與 SDK 型別的版本不一致。
- 未採：舊 preview：被拒狀態要從 `status_details` 推斷、升級時重做。
- 未採：v1 Accounts：只有台灣只收轉帳的帳號需要，US 碰不到。
- 未採：信任事件內容：事件不保證順序，重讀才是最新。

**收款帳戶的狀態模型**（Will 2026-11-02，2026-11-03 改依 v2）：5 值 `NOT_CREATED`、`UNDER_REVIEW`、`ACTION_REQUIRED`、`ENABLED`、`DISABLED`，由 `account.StatusOf` 從轉帳能力的快照推導：無外部 ID 是 `NOT_CREATED`；`active` 是 `ENABLED`；任何一筆細節要賣家行動（含 `pending`）是 `ACTION_REQUIRED`；其餘 `pending` 是 `UNDER_REVIEW`；`rejected`、`unsupported`，以及 `restricted` 加平台處理或無解法，是 `DISABLED` 並存原因碼；不認得的值與 `restricted` 卻沒有細節是 `UNDER_REVIEW` 並記警示。細節多筆時賣家能行動的優先，但 `rejected`、`unsupported` 是終態，優先於補件。建立外部帳戶後、第一次同步前的列是 `ACTION_REQUIRED`。快取（`payment.accounts`）存推導後的 `status`、`status_reason`（provider 原因碼，只給後台）、`transfers_active`、`payouts_active`、`requirements` 原始 JSON、`fetched_at`；不另存 capability 的 status 與 resolution。`payouts` 受限不進閘門，只顯示。
- 沒有 `ONBOARDING`：v2 沒有「已送出」訊號，新帳戶一開始就是 `restricted`＋`requirements_past_due`＋`provide_info`、原因全是 `routine_onboarding`（`internal/platform/stripe/testdata/retrieve_provider_new.json`），賣家動作與補件相同；賣家端文案不得把這時的 past_due 寫成逾期停用。
- 未採：以 requirements 計數造「已送出」、以 `routine_onboarding` 推 `ONBOARDING`、以賣家有沒有開過連結判斷：都是自創語意。
- 未採：只存原始 JSON 讓 client 判讀：補漏輪詢要以狀態篩選（索引 `accounts_poll`），後台要列表。
- 未採：不認得的值猜成拒絕：Stripe 可以解除限制，掉了 webhook 時只有輪詢補得到。

**開通閘門與收款模式**（Will 2026-10-29、2026-11-02）：店家通過閘門要平台審核核准且轉帳能力 active。view `seller.store_requirement_flags` 輸出 `kyc_approved` 與 `transfers_active`，`onboarding_complete` 是兩者的 AND；`transfers_active` 讀 seller 自存的副本（`seller.payout_capabilities`，由 payment 發的 `PayoutAccountChanged` 經 seller 的同 transaction 訂閱者更新）。收款模式是 destination charges（平台收款再轉給賣家）。代理收付的法規前提未確認，只影響上線。
- 未採：只看平台審核：店家可能在金流商未開通時曝光卻收不了款。
- 未採：看 `charges_enabled`：v2 的 recipient 帳戶沒有這個欄位，轉帳能力才是 destination charges 的前提。

**建立外部帳戶的時點、冪等與分兩步建立**（時點與冪等 Will 2026-11-02；分兩步 2026-11-14）：平台審核核准後才建立。核准的 transaction 內只 INSERT「建立中」的列（`external_account_id` 為空，`create_idempotency_key` 是寫列時產生並保存的 UUID，`ON CONFLICT DO NOTHING`）並排以 `store_id` 為唯一鍵的建立 job；外部呼叫在 transaction 外，同一家店的每次重試都用列上的鍵。一店一個外部帳戶，關店再開沿用。建立失敗由 River 退避重試，用完記 `create_failed_at`，進後台待辦 `PAYMENT_ACCOUNT_CREATE_FAILED`；管理員只有「重新整理」與「重試建立」，缺件由賣家經新的開通連結補。
建立分兩步，都在 Stripe adapter 內：第一步只帶不會變的內容（`metadata.store_id` 與 `metadata.hawkr_namespace`，後者來自每個環境不同的必填設定 `STRIPE_ACCOUNT_NAMESPACE`，D61），用列上的冪等鍵，本文固定，同一把鍵永遠得到同一個帳戶；第二步用更新寫入當下的聯絡信箱、recipient 設定、責任歸屬、identity 與 dashboard，可重複送出。第一步拿到外部 ID 就寫進帳戶列（不等第二步），第二步成功才記 `create_completed_at`，「已建立」「可開通」「輪詢」「同步」都看它，不看有沒有外部 ID（D59，2026-11-14 修訂「兩步都成功才寫外部 ID」）；port 因此拆成「取得 ID」（`AcquireAccount`）、「完成設定」（`ConfigureAccount`）與只找不建的 `ResolveAccount`（列出沒有關閉的帳戶、以 `metadata.store_id` 與命名空間比對，回傳全部符合的，新的在前）。原因：第 2 輪設計審查指出，兩步都成功才寫 ID、列上又分不出「從沒呼叫」與「結果不明」，且會過期（30 天）的冪等鍵被當成查詢鍵，沒有外部 ID 又問不到 provider 的列被直接刪、丟掉唯一能找回外部帳戶的鍵；現在呼叫前先記 `create_called_at`，從沒呼叫過的列直接刪，呼叫過的 30 天內重送同一把鍵、超過 30 天以 `metadata.store_id` 列出對帳。原因（兩步本身）：聯絡信箱改成每次嘗試讀當下的值之後，冪等鍵與請求內容不再是一個單位：第一次請求結果未知而信箱又變了，Stripe 回 `idempotency_error`，後台重試沿用同一把鍵，永遠建不出來。sandbox 實測：只帶 `metadata` 建立成功；更新寫入信箱與設定成功、重送成功；更新之後才建得出 Account Link；失敗的請求不重播，同一把鍵改對內容後成功。
- 未採：申請送出就建：被退回的申請也會留下外部帳戶。
- 未採：賣家第一次要收款才建：要改建立 job、輪詢與後台重試。
- 未採：每次呼叫產生新鍵：重試會建第二個帳戶；關店再開另建帳戶：賣家要重新開通，外部帳戶變兩個。
- 未採：管理員重新綁定或補件：管理員沒有可補的資料。
- 未採：把冪等鍵與這次用的信箱一起存在列上、結果未知時原樣重送：信箱副本會留到管理員處理，還要定義哪些錯誤算明確拒絕。
- 未採：後台重試時換鍵：第一次已建成時留下沒人知道的帳戶。
- 對帳（D59）：結果未知的請求放超過 Stripe v2 冪等鍵的保存期（30 天）不再重送，建立與清理都先列出沒有關閉的帳戶以 `metadata.store_id` 與 `metadata.hawkr_namespace` 對帳（sandbox 實測 2026-11-14，spike Q24：列表含只做完第一步的空帳戶並帶 `metadata`，`limit` 上限 20 以 `next_page_url` 翻頁，預設不列已關閉的帳戶、已關閉的帳戶 `metadata` 被清成 null，所以同一家店先前關閉的帳戶不會被比對到）；比對要兩者都相符（D61：兩個資料庫可能共用同一個 Stripe 帳號，只比 `store_id` 會關到另一個環境的帳戶；未採：規定每個環境用自己的 Stripe 帳號，那是靠人守）；符合的帳戶有多個時，清理全部關閉才刪列，建立採用最新的一個並關閉其餘；整份列完才算「沒有」，列出失敗不是「沒有」。推翻條件：列出不含空帳戶或不帶 `metadata`（則超過 30 天的那一列直接進終局交人處理）。未採：只把直接刪改成留列（列永遠留著、沒有終局）；維持直接刪（丟掉唯一的鍵）。

**聯絡信箱在建立外部帳戶的當下讀取**（Will 2026-11-03；改為建立當下讀取 2026-11-13）：信箱在帳號層級：provider 驗證過的 `email` 優先，沒有才用自填的聯絡信箱（`api/hawkr/identity` 的 `GetContactEmail`、`SetContactEmail`；自填的未驗證，只在沒有可用的已驗證信箱時可設）。賣家送審要求帳號有其中之一（`CONTACT_EMAIL_REQUIRED`）；核准不再檢查信箱。payment 不存信箱：建立 job 在呼叫金流商的當下經 `Owners` 的 `OwnerContactEmail` 讀取（由組裝點以 seller 與 identity 的公開 API 實作），讀不到時不呼叫金流商、記成建立失敗。核准之後賣家改了信箱，建立時用新的。快照到收款帳戶列是帳戶還在 seller 時的形狀；payment 成為獨立模組之後建立 job 已能向上讀，下層不存上層個資的副本。
- 未採：直接用登入信箱：LINE 可能沒有信箱，賣家也改不了。
- 未採：KYC 申請上的獨立欄位：client 契約拿不到登入信箱，同一人有兩個來源。
- 未採：客服信箱（公開地址）、電話（Stripe 仍要求信箱）。
- 未採：維持核准時快照、把「再次核准不更新信箱」記成延後項。

**快取不倒退**（Will 2026-11-11）：同步寫快取時，只有這次讀取的開始時間不早於已存的 `fetched_at`（NULL 一律寫）才寫；被擋下的那次不標記 inbox 事件已處理、不發 `PayoutAccountChanged`（較晚開始的讀取已涵蓋它）。條件的存在理由是有 job 以外的同步呼叫（賣家與後台的重新整理），「一個帳戶同一時間一個同步」只剩 River 唯一鍵保證；那個呼叫拿掉時，條件一起拿掉。
- 未採：RPC 排 job 並等結果：handler 要多一套等待與逾時機制。
- 未採：整列覆寫不比時間：較早開始的讀取可能蓋掉較新的快取與 seller 的副本。

**開通狀態與下一步由伺服器推導**（Will 2026-11-05、2026-11-10、2026-11-11、2026-11-13）：`GetMyOnboarding` 唯讀（NO_SIDE_EFFECTS），回依序的必要任務（`PROFILE`、`PLATFORM_REVIEW`、`PAYMENT_ACCOUNT`）與 `complete`；每項有任務 id、只供分支的狀態、下一步 `action` 與伺服器產生的句子。任務清單與閘門讀同一組來源欄位（view 的 `kyc_approved`、`transfers_active`），不是同一個 Go 函式，因為 SQL 端不能呼叫 Go；兩邊一致由 integration 測試鎖住（兩項任務都完成，等於 view 的 `onboarding_complete`，等於能力沒缺 `ONBOARDING_COMPLETE`）。單一來源讀取失敗只讓該項 `UNKNOWN`；停權與關店不列為任務，由公告頁處理。
- 下一步只有一個來源：website 與 seller-app 的認證頁、錢包面板與閘門橫幅都只讀 `PAYMENT_ACCOUNT` 任務的 `action`（2026-11-13）；按鈕文字只在共用文案目錄的 `seller.OnboardingAction.button.*`。
- 重讀是獨立的寫入類 `RefreshMyPaymentAccount`：同一帳戶 10 秒內不重打 Stripe；名額在呼叫 Stripe 前占用（失敗與並行的都算）；間隔內若名額之後快取已更新就回成功，否則回 `resource_exhausted` 加 `RetryInfo`，不以舊快取回成功。後台的 `RefreshPaymentAccount` 走同一條同步重讀（同一個時間條件與名額），回應帶最新帳戶，admin 沒有輪詢（2026-11-13）。後台 `GetStoreOnboarding` 回與賣家端同一個 message。
- 未採：client 各自組 `GetMyKYC`、`GetMyPaymentAccount`、`GetMyStore`：實測兩端對「平台審核完成」的定義已不一致。
- 未採：共用 client 函式庫：TS 與 Dart 兩份，閘門仍是第三份。
- 未採：Go 算完寫回欄位：寫入時機要與 KYC、副本的更新同步，機制較重。
- 未採：讀取方法帶 `refresh`：讀取帶寫入，停權帳號連讀都被拒；保留 `OTHER_REQUIREMENT` 當保險：七個公開條件都已有任務或公告頁，改用單元測試守住對應。
- 未採：不限頻率或只算成功的讀取：重讀由賣家觸發外部呼叫，連點與 Stripe 故障時的重試都會打到 Stripe；以舊快取回成功：client 對 `unavailable` 自動重試，回成功會蓋掉錯誤。

**收款帳戶的下一步**（Will 2026-11-14）：進行中的收款任務 `action` 是 `NONE`；`OnboardingAction` 沒有「重新整理」，賣家端不再有「重新整理」按鈕，按鈕與說明句只讀任務，徽章讀帳戶狀態。三項：
- `RefreshMyPaymentAccount` 保留，由 client 在進入認證頁、從開通流程返回、App 回前景時自動呼叫（必要條件：返回時 webhook 可能還沒到）；自動呼叫失敗（含 `resource_exhausted`、停權的 `permission_denied`）不顯示，照常讀 `GetMyOnboarding`。未採「刪掉 RPC、由 `GetMyOnboarding` 重讀」：它不重讀，而且停權的店家可以讀它但不能觸發外部呼叫。
- 「開始設定」在每個 client 都能執行：App 送空的內嵌能力清單，一律拿託管連結並開外部瀏覽器；「到網頁完成」的提示與路徑刪除（修訂 D56 第 2 項：原本回網站認證頁、賣家要在網頁再登入一次，現在平台收件的賣家在 App 看到 Stripe 品牌的託管頁，Will 2026-11-14 接受）。未採：App 自己判斷介面只顯示提示（沒有出口）、讀摘要時帶 client 能力。
- 入口頁（網站的閘門橫幅、錢包面板，以及 seller-app 的錢包面板）不讀 `action`，未完成時固定顯示「前往商家認證」連到認證頁。伺服器句子「收款帳戶的狀態正在更新，請稍後重新整理。」因按鈕刪除改成「請稍後再查看。」（英文 `Please check back later.`）；共用文案目錄與後台的重新整理成功訊息跟著改。
- 做法依據：開工前逐分支比對伺服器的 `paymentAccountTask` 與兩個 client 的分支，再與業界做法（下一步由伺服器決定、狀態靠 webhook 與自動重讀、動作只在一個設定頁執行）比較後決定。

**開通 session 帶金流商與公開金鑰**（Will 2026-11-13；形狀改為 oneof 2026-11-14）：內嵌是中立的介面，client 要知道載入哪一家的元件；內嵌 session 以 `embedded` 的 `oneof provider` 分支表示金流商（目前只有 `stripe`，帶 `client_secret` 與 `publishable_key`，初始化元件的公開金鑰，不是 secret），不再有字串型的 `provider` 欄位；金鑰由後端設定 `STRIPE_PUBLISHABLE_KEY` 一處維護：設了 `STRIPE_SECRET_KEY` 卻沒有它，啟動失敗，與收件方式無關（行程設定改成金流商收件後，既有的平台收件帳戶仍會開內嵌 session）。website 依分支選元件，不認得的分支顯示既有的錯誤狀態；website 的 build 設定不再放金鑰。
- 未採：延後到接第二家金流商時再做，維持 website 的 build 設定放金鑰。

**回跳網址由伺服器組**（Will 2026-11-13；語系改走 `Accept-Language` 2026-11-14）：`CreatePaymentOnboardingSessionRequest` 的 client 不傳網址，只傳回跳目的地的 enum（`OnboardingReturnDestination`：網站的認證頁、App 的回跳頁）；語系不再是請求欄位，伺服器取 rpc interceptor 協商 `Accept-Language` 的結果（`uitext.LocaleFrom(ctx)`），修訂了先前的 `locale` 欄位（D49-2：它是語系 ADR 之外的第二條通道）。伺服器以 `PUBLIC_WEB_ORIGIN` 組成完成與過期兩個網址，網站路徑與語系前綴只在 `internal/httpapi/rpc/web_origin.go` 一處，測試確認每個目的地的路徑在 website 的路由存在（`TestOnboardingReturnPagesExistInTheWebsiteRoutes`）。seller-app 與 website 刪掉各自的網站來源設定。
- 未採：維持 client 傳完整網址：網站路徑與語系前綴要在 website 與 seller-app 各寫一份；搬到伺服器後只在一處。

**開通 session 的契約是兩支 `oneof`**（Will 2026-11-14，D58）：第 2 輪設計審查指出回應是型態欄位加攤平欄位，會出現「內嵌介面但網址是網站認證頁」的第三種組合，賣家端帳戶上的開通介面欄位與 session 的介面來源不同，`locale` 是語系 ADR 之外的第二條通道。與業界做法比較（Stripe、Adyen 都是託管回網址、內嵌回 token 兩種資源；Stripe 的內嵌元件不能放 App 的 web view，沒有原生元件時連到瀏覽器）後決定：請求分開帶回跳目的地與呼叫端能承載的內嵌執行環境清單（`embedded_runtimes`，空表示不能內嵌；enum `EmbeddedOnboardingRuntime` 目前只有 `STRIPE_CONNECT_JS`），回應是 `oneof session`（`hosted` 託管連結、`embedded` 內嵌，內嵌再以 `oneof provider` 分金流商），賣家端帳戶上的開通介面欄位與那個介面列舉刪除。伺服器的決定表只在 `account.ChooseSession`（只看帳戶列保存的收件方式、provider 經 port 宣告支援的內嵌執行環境與呼叫端宣告，不比 provider 名稱），fake provider 不支援任何內嵌執行環境，一律回託管連結。語系走 `Accept-Language`（rpc interceptor 協商）。修訂：D2「介面由收件方式決定」、D53 的回應形狀、D56 第 2 項「到網站完成」、D49-2 的 `locale`。推翻條件：平台收件的帳戶在託管頁面走不完開通（2026-11-14 在 sandbox 實測走得完，`spike/README.md` Q23）。
- 未採：三支 `oneof`（保留 App 到網站再登入；是 spike 不成立時的退路）；記錄並延後（留著違反 API 設計規範的型態欄位加攤平欄位）。

**賣家端移除沒有消費端的欄位，列舉搬到擁有者**（Will 2026-11-13）：賣家端 `PaymentAccount` 刪除 `requirement_collection`、`transfers_active`、`requirements`，`CreatePaymentOnboardingSessionResponse` 刪除 `expire_time`（沒有 client 讀；後台的 message 不動），`PaymentAccount` 也刪除開通介面欄位（D58）。`PaymentAccountStatus`、`RequirementCollection` 與呼叫端宣告用的 `EmbeddedOnboardingRuntime` 定義在 payment 擁有的 proto package `hawkr.payment.v1alpha1`，seller 與 admin payment 的 message 都 import 它；共用文案目錄的鍵跟著搬到 `payment.PaymentAccountStatus.*`、`payment.RequirementCollection.*`。
- 未採：只刪欄位、列舉搬家留到付款模組搬進來時；全部延後（計畫沒有寫理由）。

**`v1alpha1` 不留 `reserved`**（Will 2026-11-13）：`api/hawkr/**/v1alpha1` 刪掉欄位或 enum 值時不留 `reserved`，欄位編號與 enum 值連續重編；`reserved` 防的是已發佈的 client 與已存的資料，系統未上線時沒有保護對象，改名 `v1` 之後才留，刪除的編號與名稱都要留。規則寫在 `API 設計規範（Connect）.md`。
- 未採：維持現況，在 API 設計規範寫明上線前也留。

**文案歸屬**（Will 2026-11-05、2026-11-11）：任務標題與狀態句由 seller 模組的 `internal/text` 產生，隨回應以 `LocalizedMessage` 送出，client 與共用文案目錄都不寫；任務說明、面板文字與欄位標籤進共用文案目錄（`seller.OnboardingTaskId.*`、`payment.PaymentAccountStatus.*`）；純畫面文字留在各 client。分界見 `2026-10-17-multi-locale-architecture.md`。
- 未採：任務加 `description` 由伺服器送出：要改契約與三端生成物。
- 未採：兩端各寫說明表：同一個 enum 的兩份對照表，`scripts/check-enum-text-cross.sh` 擋下；拿掉說明文字：畫面文案變動。

**模組歸屬**（Will 2026-11-06）：收款帳戶歸 payment，inbox 由 platform 提供（見 2026-11-06 修訂）。賣家端的 `GetMyPaymentAccount`、`CreatePaymentOnboardingSession`、`RefreshMyPaymentAccount` 留在 seller 的 service：seller 確認店家擁有者後呼叫 payment 的公開 API，因為 payment 是葉節點，不能向上讀 seller 驗證擁有者。後台 3 支（`GetPaymentAccount`、`RefreshPaymentAccount`、`RetryPaymentAccountCreate`）在 `PaymentAdminService`，權限 `payment.accounts.*`。
- 未採：賣家端 RPC 全搬 payment：要讓 payment 收已驗證的擁有者參照，website 與 App 都要改。
- 未採：後台 RPC 全留 seller：後台讀收款帳戶仍掛在 seller。

**App 內不做 Stripe 內嵌開通**（Will 2026-11-03、2026-11-05）：seller-app 開通一律開外部瀏覽器：託管頁面的 `return_url` 與 `refresh_url` 指向 website 的免登入靜態頁（`website/app/[locale]/(public)/seller-app/payment-return/`），App 回到前景時重讀；平台收件的內嵌元件只在 website；App 送空的內嵌能力清單，對平台收件的帳戶也拿到託管連結。這是不新增依賴的選擇，不是 Stripe 的限制。
- 未採：App 內嵌 SDK：要新增依賴與原生設定；deep link：要新增 App 端的路由與平台設定。
- 觸發：App 的開通完成率偏低，或平台收件成為主要模式。

**個資清理時關閉外部帳戶**（Will 2026-11-13）：身分與銀行資料在金流商的帳戶上，只刪本地對應等於清理只做一半。擁有者個資清理時，seller 的清理 transaction 呼叫 payment 的 `ForgetOwners`，只排「關閉後刪除」的 job（唯一鍵 `store_id`，外部呼叫在 transaction 外）：先標 `purge_pending_at`（呼叫金流商之前，之後任何錯誤都有週期掃描接手），關閉成功才刪本地列。關閉冪等：Stripe 對已關閉的帳戶再關回 403，adapter 讀取帳戶，已關閉才視為成功；關閉要帶帳戶實際的 `applied_configurations`（先讀回：已完成設定的是 `recipient`，只做完第一步的空帳戶是 `[]`，傳錯是 400 `configs_must_match_to_close`，spike Q25，D60）；不存在的帳戶是永久錯誤。從沒呼叫過 provider 的列直接刪；呼叫過、沒有外部 ID 的列（建立請求可能其實已成功）30 天內以同一把冪等鍵取得外部帳戶、超過 30 天列出對帳，先把 ID 存回列、再關閉刪除。關閉失敗用完重試時本地列留著（保住 `store_id` 與外部 ID 的對應），排除在建立與同步輪詢之外，由週期掃描重排。列的 provider 與目前設定不同、或沒有設定 provider 時不關也不刪，記警示 log，等設定回來再處理（刪掉就永遠找不到外部帳戶）。**清理有終局（D59、D60，2026-11-14 修訂「只記警示 log」）**：金流商明確拒絕關閉，或標成等待清理 7 天仍關不掉、問不到 provider，列標 `purge_blocked_at` 與原因進終局，掃描不再重排，進後台待辦 `PAYMENT_ACCOUNT_PURGE_STUCK`（標題「處理無法關閉的收款帳戶」）；管理員「重試關閉」（清除終局、期限重算）或「已在金流商處理，刪除對應」（需要權限 `payment.accounts.delete_mapping`，同一個 transaction 寫只增不改的 `payment.account_mapping_deletions`——店家、provider、外部 ID、管理員、時間——並刪列，不呼叫金流商）。7 天：既有重試是單一 job 8 次約 80 分鐘、掃描每 15 分鐘，7 天涵蓋金流商故障與金鑰設定修正。紀錄用新表而不沿用 seller 的 `decided_by_admin_id`：那個欄位寫在被審的列上，列刪了就不見。未採：列不刪、改標「已人工處理」（推翻 D52、列永遠留著）。Q26（有餘額時關閉的錯誤形狀）沒有量到（未完成開通的帳戶收不到轉帳）：照文件處理，非預期的拒絕進終局。
- 未採：延後到上線前：現在只有測試模式的帳戶。

**fake provider 只在 dev 環境**（Will 2026-11-13）：`PAYMENT_PROVIDER=fake` 只在 `requireDevEnvironment` 成立時選得到（有 `DEV_AUTH_KEY_FILE` 而且沒有 Cloudflare Access 設定），不成立就啟動失敗，`readPayment` 的每個呼叫點都適用。fake 回報所有帳戶轉帳能力已啟用，在非 dev 環境選到等於不經金流商就通過開通閘門。

**分支內的 migration 在合併前壓平**（Will 2026-11-13、2026-11-14）：本分支新增的 migration 只在本分支、診斷分支、本機與 CI 的拋棄式資料庫套用過，「已套用的 migration 不得修改」保護的對象還不存在，所以在併入 `dev` 之前壓成各模組的淨差異；併入之後就不能改，`dev` 已有的 migration 一律不動。壓平後的 migration 對 `dev` 的任何合法資料都能套用：舊 schema 下合法、新約束會拒絕的列全部先刪（手寫的 seller `seller_payment_onboarding_drop_objects`），並保留一個在 `dev` 的 schema 狀態插入資料再套用的 migration 測試（`TestPaymentOnboardingSquashedMigrationDeletesRetiredRowsAndKeepsTheRest`）。`atlas migrate diff` 只起一次性的生成容器，可以在本機執行。規則寫進 `backend/AGENTS.md` 單體規則 6。
- 未採：照現況合併：建了又刪的表永久留在歷史裡。
- 未採：只留「種子資料會違反」的刪除：操作過 KYC 或停權過種子店家的 `dev` 資料庫會在收緊約束時失敗，留下的刪除也沒有測試。
- 未採：一句刪除都不留（計畫沒有寫理由）。

## 後果

- 本機的付款與物流端到端在付款、物流模組搬進單體前不通：`internal/legacy/sellerserver` 回給 Java 的 `payuni_mer_id` 與物流的 `MerId` 是空值（Stripe 的 `acct_` 不是 PayUni 商店代號）。付款改走 port 時以外部帳戶 ID 取代這個接縫；物流改綠界後恢復建店與出貨（Will 2026-10-29、2026-11-12 接受）。
- Stripe adapter 不示範台灣情境（沒有 TWD、台灣 KYC、超商與 ATM）。決定在台灣上線時，先確認法規（代理收付）與合約，再寫台灣金流商 adapter，並重新評估兩軸 port 的設計（平台收件是否要重建自建 KYC）與收款模式。
- live mode 的銀行帳號 fingerprint 是否跨 connected account 一致，只在測試模式驗過；上線前核對範圍，防詐比對靠它。
- 撥款模型與手續費（代理商費反推）未決定：付款與撥款搬進 payment 時重審（B2、B10 開工）。
- 賣家端沒有「管理收款帳戶」入口，也沒有 `account_update` 連結；觸發：賣家要求改資料，或上線前的設定頁需求。
- 沒有存收款帳戶的首次啟用時間與補件期限（Stripe 回報的 requirements 摘要裡最早的期限），也沒有期限前提醒；觸發：開通結果通知（A8）開工。
- 自填聯絡信箱未驗證；觸發：A8 的 email 管道上線時加驗證碼，並把帳號信箱變更同步到 Stripe。A8 只寄已驗證的信箱。
- 平台承擔負餘額；要上線且改變損失歸屬時重新評估內嵌元件關掉使用者驗證的決定。
- 沿用機制與延後項的觸發條件（輪詢沒有上限、inbox 沒有終局狀態（同步讀到已關閉的帳戶已在 9g 處理：列標 `DISABLED`、原因碼 `closed`，不再輪詢，D61）、`provider` 欄位在換 provider 時舊列的去向；清理的終局已在 9g 處理）寫在 `internal/modules/payment/CARRYOVER.md` 與 `docs/modules/payment.md` §8。
