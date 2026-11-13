# 賣家收款開通：經 provider 中立的 payment port，作品集接 Stripe Connect 測試模式

| 項目 | 值 |
|---|---|
| 狀態 | accepted（Will，2026-10-29 至 2026-11-13 分次決定；各決定的日期在決策段） |
| 日期 | 2026-11-13 |
| 影響範圍 | `internal/modules/payment`（port、adapter、`payment.accounts`）、`internal/platform/stripe`、`internal/platform/inbox`、`internal/modules/seller`（開通閘門、`GetMyOnboarding`）、`internal/modules/reviewqueue`、`api/hawkr/seller`、`api/hawkr/admin/payment`、`api/hawkr/identity`、website、admin、seller-app |
| 相關 | 收款帳戶歸 payment、webhook inbox 由 platform 提供，已寫在 `2026-09-24-go-modular-monolith.md` 文末 2026-11-06 修訂，本檔只指過去；平台審核、防詐身分比對、入駐同意與個資保留見 `2026-11-13-seller-platform-review-and-identity-matching.md`；文案分界見 `2026-10-17-multi-locale-architecture.md` |

## 脈絡

賣家要先通過平台審核，再讓金流商開通收款帳戶，店家才能公開曝光。公司時期的做法是平台自己收身分證號、銀行帳號與證件，審核後以 PayUni 代理商 API 建子商店。單體搬遷時要決定收款開通怎麼放；作品集既拿不到 PayUni 測試帳號，也不能在台灣示範任何在地金流。

約束：
- 外部：沒有 PayUni 測試帳號，代理商 API 要簽約才開通（Will 2026-10-29 決定不簽）；PayUni 程式因此無法對真實 sandbox 驗證。
- 外部：Stripe 不接受台灣的平台帳號，Connect 跨境撥款只開給平台在 US、UK、EEA、CA、CH；沒有台灣的超商代碼、ATM、LINE Pay、街口（https://stripe.com/global、https://docs.stripe.com/connect/cross-border-payouts、https://docs.stripe.com/payments/payment-methods/payment-method-support，2026-10-29 核對）。Stripe adapter 因此只示範機制。
- 外部：平台自己收款再轉給賣家可能落入《電子支付機構管理條例》的代理收付（第 3、5 條；第三方轉載，未在全國法規資料庫核對，未確認）。只影響真實上線。
- 外部：Stripe v2 有 `configuration.recipient` 的帳戶必須帶 `contact_email`（sandbox 實測，兩種 dashboard 都回 400）；`2026-09-30.preview` 在一般測試模式不能建 v2 Account Link，要用 sandbox。
- 外部：webhook 會重送、不保證順序（https://docs.stripe.com/webhooks）。
- 繼承：系統從未上線，沒有相容性與資料搬移約束（根 `AGENTS.md`）；現在仍成立，上線後失效，之後的改動要走 expand-contract。
- 繼承：外部呼叫不在 transaction 內、webhook 先寫 inbox（`backend/AGENTS.md` 單體規則 5）；仍成立：transaction 內打外部 API，失敗時會留下半成品。
- 繼承：可曝光公告與 social 的可見性在 SQL 端直接讀 view `seller.store_requirement_flags`，不能呼叫 Go；仍成立：它們是資料庫函式。
- 繼承：使用者面文字依目錄分界（`2026-10-17-multi-locale-architecture.md`）；仍成立。
- 不再成立而拿掉：平台先收身分證號、銀行帳號、證件再送金流商，以及「平台與 PayUni 兩關都過才能收單」。理由：平台持有最多個資，沒有業界基準支持（D2）。

## 考慮過的方案

「基準」是同樣條件下、假設還沒有程式碼時的業界做法；標「推測」的是沒有核到官方出處。未採方案的理由在決策段。

| 決定 | 基準（業界做法與來源） | 採用 | 未採 |
|---|---|---|---|
| D1 adapter | 平台把開通交給 connected account 的 provider（Stripe Connect recipient 帳戶，https://docs.stripe.com/connect/accounts-v2） | Stripe Connect 測試模式，PayUni adapter 刪除 | PayUni adapter 留在 port 後面；簽 PayUni |
| D2 port 的兩軸 | Stripe 分開「誰收資料」與「用什麼介面」（https://docs.stripe.com/connect/onboarding、https://docs.stripe.com/connect/hosted-onboarding） | 收件方式與介面兩軸，預設金流商收件加託管頁面 | 預設平台收件；平台自建表單再送出 |
| D3 內嵌元件的使用者驗證 | 平台負責補件的帳戶可關掉 Stripe 登入視窗，代價是平台承擔負餘額（https://docs.stripe.com/connect/get-started-connect-embedded-components?platform=web） | 關掉 | 維持預設 |
| D4 測試平台國別 | 平台與 connected account 在支援的國別組合內（脈絡的外部約束） | US | SG、JP、US 平台跨境撥到台灣 |
| D5 Stripe API | 新整合用 Accounts v2，事件用 thin events 並重讀物件（https://docs.stripe.com/connect/accounts-v2、https://docs.stripe.com/webhooks） | v2、釘 `2026-09-30.preview`、收到後一律重讀 | `2026-04-22.preview`；v1 Accounts；信任事件內容 |
| D6 狀態模型 | 完成看 `stripe_transfers.status`，行動方看 `status_details[].resolution`（https://docs.stripe.com/connect/marketplace/tasks/onboard） | 5 值，由 `stripe_transfers` 推導 | 加 `ONBOARDING`；自創「已送出」；只存原始 JSON |
| D7 閘門與收款模式 | capability 加 requirements 當授權依據（`2026-09-18-seller-capability-authorization.md` 2026-10-08 修訂）；市集用 destination charges 是推論，未核出處（推測） | 平台審核核准且轉帳能力 active；destination charges | 只看平台審核；看 `charges_enabled` |
| D8 建立時點與冪等 | 隨機 v4 UUID 當冪等鍵，v2 鍵期限 30 天（https://docs.stripe.com/api/idempotent_requests、https://docs.stripe.com/api-v2-overview#idempotency） | 核准後建立，保存的 UUID 當鍵，先寫本地列 | 送出即建；首次收款才建；每次新鍵；管理員補件 |
| D9 聯絡信箱 | 用平台已有的使用者信箱建帳戶，持有人可在開通頁面改（https://docs.stripe.com/connect/marketplace/quickstart、https://docs.stripe.com/connect/hosted-onboarding） | 帳號層級，核准時快照 | 直接用登入信箱；KYC 申請欄位；客服信箱；電話 |
| D10 快取不倒退 | 事件不保證順序，收到後重讀（https://docs.stripe.com/webhooks）；寫入條件沒有統一名稱（推測） | 讀取開始時間不早於 `fetched_at` 才寫 | 排 job 並等結果；整列覆寫不比時間 |
| D11 開通狀態 | 伺服器整理 requirements，平台只呈現（https://docs.stripe.com/connect/onboarding）；讀取方法無副作用（https://google.aip.dev/131） | 伺服器推導，唯讀 `GetMyOnboarding` 加寫入類 `RefreshMyPaymentAccount` | client 各自推導；共用 client 函式庫；讀取帶 `refresh` |
| D12 文案歸屬 | 同一個 enum 的對照表只寫一份（`2026-10-17-multi-locale-architecture.md`） | 標題與狀態句由伺服器，說明與標籤進目錄 | 伺服器送說明；兩端各寫 |
| D13 模組歸屬 | 資源的 API 在擁有者的 service，擁有者驗證留在上層入口（`internal/modules/payment/CARRYOVER.md`） | 賣家端 RPC 留 seller，後台 RPC 在 payment | 全搬 payment；全留 seller |
| D14 App 內開通 | Stripe 有 iOS、Android 的 account onboarding 元件（https://docs.stripe.com/connect/supported-embedded-components） | 外部瀏覽器加 website 靜態頁，回前景重讀 | App 內嵌 SDK；deep link |

## 決策

賣家收款開通只經 provider 中立的 payment port。作品集的 adapter 是 Stripe Connect 測試模式，預設由 Stripe 收件加託管頁面，平台收件用內嵌元件；平台不收身分證號、銀行帳號與證件；PayUni adapter 刪除。店家公開曝光要平台審核核准且轉帳能力 active，狀態、任務清單與閘門由伺服器從同一組來源欄位推導，client 只呈現。

**D1 adapter**（Will 2026-10-29、2026-11-02）：Stripe Connect 測試模式，單體的 PayUni adapter 刪除，PayUni 的付款、退款、撥款與物流留在 Java 舊服務，由單體計畫的 B2、B3、B8–B10 搬。
- 未採：PayUni adapter 留在 port 後面當第二個 adapter：沒有測試帳號無法驗證，會隨 port 演進失真；git 找得回。
- 未採：簽 PayUni：測試帳號要簽約，作品集要展示國際標準。

**D2 port 的兩軸**（Will 2026-10-29、2026-11-02）：port 的能力描述有兩個獨立的軸，`requirement_collection`（`provider` 金流商收件、`platform` 平台收件，帳戶列上也存）與 `onboarding_surface`（`hosted` 託管頁面、`embedded` 內嵌元件）。預設是金流商收件加託管頁面；平台收件用 Stripe 內嵌元件。port 有 `EnsureAccount`、`CreateOnboardingSession`、`FetchAccount`、`ParseEvent`，沒有 `SubmitKYC`，平台不自建 KYC 表單，也不再收身分證號、銀行帳號與證件；統編是市集登記資料，仍由平台收。
- 未採：預設平台收件：沿用舊畫面最多，但平台持有的個資最多。
- 未採：兩種收件都由平台自建表單再送出：兩邊重複收個資。
- 防詐訊號改用銀行帳號 fingerprint 與統編的雜湊，細節在平台審核 ADR。觸發重建：決定接台灣金流商。

**D3 內嵌元件關掉 Stripe 使用者驗證**（Will 2026-11-03）：平台收件的 Account Session 帶 `disable_stripe_user_authentication=true`。賣家已用 Google 或 LINE 登入，平台收件的目的就是賣家留在平台頁面完成。代價：平台承擔負餘額；本平台的帳戶本來就由平台承擔損失（`losses_collector=application`），所以代價已在。託管頁面不受影響。
- 未採：維持預設：賣家每次開通都要另外登入或註冊 Stripe。

**D4 測試平台帳號是 US**（Will 2026-11-02）：US 平台，沿用不改。
- 未採：SG：平台收件的帳戶卡在 `proof_of_liveness`，沒有 API 參數可繞。
- 未採：JP：KYC 欄位與文件以日文為主。
- 未採：US 平台跨境撥到台灣：台灣只能開 transfers，離上線情境更遠。

**D5 Stripe API**（Will 2026-11-02、2026-11-03）：帳戶用 Accounts v2（`/v2/core/accounts`，configuration `recipient`，capability `stripe_transfers`），釘 `Stripe-Version: 2026-09-30.preview`（`internal/platform/stripe` 的 `APIVersion`）。Account Session 是 v1 呼叫，用同一 release 的 GA 版 `2026-09-30.endive`（`APIVersionV1`）。webhook 只收平台自己的 `v2.core.account*` thin events，收到後一律重讀帳戶，不信事件內容；驗過簽章但不處理的事件回 2xx，不寫 inbox。
- 未採：`2026-04-22.preview`：沿用一般測試模式，但被拒狀態要從 `status_details` 推斷、升級時重做（`rejected` 狀態見 https://docs.stripe.com/changelog/endive/2026-09-30/rejected-accounts-v2-capabilities）。
- 未採：v1 Accounts：只有台灣只收轉帳的帳號需要，D4 的 US 碰不到。
- 未採：信任事件內容：事件不保證順序，重讀才是最新。

**D6 收款帳戶的狀態模型**（Will 2026-11-02，2026-11-03 改依 v2）：5 值 `NOT_CREATED`、`UNDER_REVIEW`、`ACTION_REQUIRED`、`ENABLED`、`DISABLED`，由 `account.StatusOf` 從 `stripe_transfers` 的快照推導：無外部 ID 是 `NOT_CREATED`；`active` 是 `ENABLED`；任何一筆細節要賣家行動（含 `pending`）是 `ACTION_REQUIRED`；其餘 `pending` 是 `UNDER_REVIEW`；`rejected`、`unsupported`，以及 `restricted` 加平台處理或無解法，是 `DISABLED` 並存原因碼；不認得的值與 `restricted` 卻沒有細節是 `UNDER_REVIEW` 並記警示。細節多筆時賣家能行動的優先，但 `rejected`、`unsupported` 是終態，優先於補件。建立外部帳戶後、第一次同步前的列是 `ACTION_REQUIRED`。快取（`payment.accounts`）存推導後的 `status`、`status_reason`（provider 原因碼，只給後台）、`transfers_active`、`payouts_active`、`requirements` 原始 JSON、`fetched_at`；不另存 capability 的 status 與 resolution。`payouts` 受限不進閘門，只顯示。
- 沒有 `ONBOARDING`：v2 沒有「已送出」訊號，新帳戶一開始就是 `restricted`＋`requirements_past_due`＋`provide_info`、原因全是 `routine_onboarding`（`internal/platform/stripe/testdata/retrieve_provider_new.json`），賣家動作與補件相同；賣家端文案不得把這時的 past_due 寫成逾期停用。
- 未採：以 requirements 計數造「已送出」、以 `routine_onboarding` 推 `ONBOARDING`、以賣家有沒有開過連結判斷：都是自創語意。
- 未採：只存原始 JSON 讓 client 判讀：補漏輪詢要以狀態篩選（索引 `accounts_poll`），後台要列表。
- 未採：不認得的值猜成拒絕：Stripe 可以解除限制，掉了 webhook 時只有輪詢補得到。

**D7 開通閘門與收款模式**（Will 2026-10-29、2026-11-02）：店家通過閘門要平台審核核准且轉帳能力 active。view `seller.store_requirement_flags` 輸出 `kyc_approved` 與 `transfers_active`，`onboarding_complete` 是兩者的 AND；`transfers_active` 讀 seller 自存的副本（`seller.payout_capabilities`，由 payment 發的 `PayoutAccountChanged` 經 seller 的同 transaction 訂閱者更新）。收款模式是 destination charges（平台收款再轉給賣家）。代理收付的法規前提未確認，只影響上線。
- 未採：只看平台審核：店家可能在金流商未開通時曝光卻收不了款。
- 未採：看 `charges_enabled`：v2 的 recipient 帳戶沒有這個欄位，轉帳能力才是 destination charges 的前提。

**D8 建立外部帳戶的時點與冪等**（Will 2026-11-02）：平台審核核准後才建立。核准的 transaction 內只 INSERT「建立中」的列（`external_account_id` 為空，`create_idempotency_key` 是寫列時產生並保存的 UUID，`ON CONFLICT DO NOTHING`）並排以 `store_id` 為唯一鍵的建立 job；外部呼叫在 transaction 外，同一家店的每次重試都用列上的鍵。一店一個外部帳戶，關店再開沿用。建立失敗由 River 退避重試，用完記 `create_failed_at`，進後台待辦 `PAYMENT_ACCOUNT_CREATE_FAILED`；管理員只有「重新整理」與「重試建立」，缺件由賣家經新的開通連結補。
- 未採：申請送出就建：被退回的申請也會留下外部帳戶。
- 未採：賣家第一次要收款才建：要改建立 job、輪詢與後台重試。
- 未採：每次呼叫產生新鍵：重試會建第二個帳戶；關店再開另建帳戶：賣家要重新開通，外部帳戶變兩個。
- 未採：管理員重新綁定或補件：管理員沒有可補的資料。

**D9 聯絡信箱**（Will 2026-11-03）：信箱在帳號層級：provider 驗證過的 `email` 優先，沒有才用自填的聯絡信箱（`api/hawkr/identity` 的 `GetContactEmail`、`SetContactEmail`；自填的未驗證，只在沒有可用的已驗證信箱時可設）。賣家送審要求帳號有其中之一；核准的 transaction 快照到 `payment.accounts.contact_email`，之後不自動同步；建立帳戶時送給 Stripe。
- 未採：直接用登入信箱：LINE 可能沒有信箱，賣家也改不了。
- 未採：KYC 申請上的獨立欄位：client 契約拿不到登入信箱，同一人有兩個來源。
- 未採：客服信箱（公開地址）、電話（Stripe 仍要求信箱）。

**D10 快取不倒退**（Will 2026-11-11）：同步寫快取時，只有這次讀取的開始時間不早於已存的 `fetched_at`（NULL 一律寫）才寫；被擋下的那次不標記 inbox 事件已處理、不發 `PayoutAccountChanged`（較晚開始的讀取已涵蓋它）。條件的存在理由是有 job 以外的同步呼叫（`RefreshMyPaymentAccount`），「一個帳戶同一時間一個同步」只剩 River 唯一鍵保證；那個呼叫拿掉時，條件一起拿掉。
- 未採：RPC 排 job 並等結果：handler 要多一套等待與逾時機制。
- 未採：整列覆寫不比時間：較早開始的讀取可能蓋掉較新的快取與 seller 的副本。

**D11 開通狀態由伺服器推導**（Will 2026-11-05、2026-11-10、2026-11-11）：`GetMyOnboarding` 唯讀（NO_SIDE_EFFECTS），回依序的必要任務（`PROFILE`、`PLATFORM_REVIEW`、`PAYMENT_ACCOUNT`）與 `complete`；每項有任務 id、只供分支的狀態、下一步 `action` 與伺服器產生的句子。任務清單與閘門讀同一組來源欄位（view 的 `kyc_approved`、`transfers_active`），不是同一個 Go 函式，因為 SQL 端不能呼叫 Go；兩邊一致由 integration 測試鎖住（兩項任務都完成，等於 view 的 `onboarding_complete`，等於能力沒缺 `ONBOARDING_COMPLETE`）。單一來源讀取失敗只讓該項 `UNKNOWN`；停權與關店不列為任務，由公告頁處理。重讀是獨立的寫入類 `RefreshMyPaymentAccount`：同一帳戶 10 秒內不重打 Stripe；名額在呼叫 Stripe 前占用（失敗與並行的都算）；間隔內若名額之後快取已更新就回成功，否則回 `resource_exhausted` 加 `RetryInfo`，不以舊快取回成功。後台 `GetStoreOnboarding` 回同一個 message。
- 未採：client 各自組 `GetMyKYC`、`GetMyPaymentAccount`、`GetMyStore`：實測兩端對「平台審核完成」的定義已不一致。
- 未採：共用 client 函式庫：TS 與 Dart 兩份，閘門仍是第三份。
- 未採：Go 算完寫回欄位：寫入時機要與 KYC、副本的更新同步，機制較重。
- 未採：讀取方法帶 `refresh`：讀取帶寫入，停權帳號連讀都被拒；保留 `OTHER_REQUIREMENT` 當保險：七個公開條件都已有任務或公告頁，改用單元測試守住對應。
- 未採：不限頻率或只算成功的讀取：重讀由賣家觸發外部呼叫，連點與 Stripe 故障時的重試都會打到 Stripe；以舊快取回成功：client 對 `unavailable` 自動重試，回成功會蓋掉錯誤。

**D12 文案歸屬**（Will 2026-11-05、2026-11-11）：任務標題與狀態句由 seller 模組的 `internal/text` 產生，隨回應以 `LocalizedMessage` 送出，client 與共用文案目錄都不寫；任務說明、面板文字與欄位標籤進共用文案目錄（`seller.OnboardingTaskId.*`、`seller.PaymentAccountStatus`）；純畫面文字留在各 client。分界見 `2026-10-17-multi-locale-architecture.md`。
- 未採：`OnboardingTask` 加 `description` 由伺服器送出：要改契約與三端生成物。
- 未採：兩端各寫說明表：同一個 enum 的兩份對照表，`scripts/check-enum-text-cross.sh` 擋下；拿掉說明文字：畫面文案變動。

**D13 模組歸屬**（Will 2026-11-06）：收款帳戶歸 payment，inbox 由 platform 提供（見 2026-11-06 修訂）。賣家端的 `GetMyPaymentAccount`、`CreatePaymentOnboardingSession`、`RefreshMyPaymentAccount` 留在 seller 的 service：seller 確認店家擁有者後呼叫 payment 的公開 API，因為 payment 是葉節點，不能向上讀 seller 驗證擁有者。後台 3 支（`GetPaymentAccount`、`RefreshPaymentAccount`、`RetryPaymentAccountCreate`）在 `PaymentAdminService`，權限 `payment.accounts.*`。
- 未採：賣家端 RPC 全搬 payment：要讓 payment 收已驗證的擁有者參照，website 與 App 都要改。
- 未採：後台 RPC 全留 seller：後台讀收款帳戶仍掛在 seller。

**D14 App 內不做 Stripe 內嵌開通**（Will 2026-11-03、2026-11-05）：seller-app 的金流商收件用外部瀏覽器開託管頁面，`return_url` 與 `refresh_url` 指向 website 的免登入靜態頁（`website/app/[locale]/(public)/seller-app/payment-return/`），App 回到前景時重讀；平台收件的內嵌元件只在 website，App 顯示「請到網頁版完成」。`OnboardingAction` 能表達「到網頁完成」，之後改動不動契約。這是不新增依賴的選擇，不是 Stripe 的限制。
- 未採：App 內嵌 SDK：要新增依賴與原生設定；deep link：要新增 App 端的路由與平台設定。
- 觸發：App 的開通完成率偏低，或平台收件成為主要模式。

## 後果

- 本機的付款與物流端到端在付款、物流模組搬進單體前不通：`internal/legacy/sellerserver` 回給 Java 的 `payuni_mer_id` 與物流的 `MerId` 是空值（Stripe 的 `acct_` 不是 PayUni 商店代號）。付款改走 port 時以外部帳戶 ID 取代這個接縫；物流改綠界後恢復建店與出貨（Will 2026-10-29、2026-11-12 接受）。
- Stripe adapter 不示範台灣情境（沒有 TWD、台灣 KYC、超商與 ATM）。決定在台灣上線時，先確認法規（代理收付）與合約，再寫台灣金流商 adapter，並重新評估 D2（平台收件是否要重建自建 KYC）與 D7（收款模式）。
- live mode 的銀行帳號 fingerprint 是否跨 connected account 一致，只在測試模式驗過；上線前核對範圍，防詐比對靠它。
- 撥款模型與手續費（代理商費反推、`payuni-channel-fee-bearer`）未決定：付款與撥款搬進 payment 時重審（B2、B10 開工）。
- 賣家端沒有「管理收款帳戶」入口，也沒有 `account_update` 連結；觸發：賣家要求改資料，或上線前的設定頁需求。
- 沒有存收款帳戶的首次啟用時間與補件期限（`requirements.summary.minimum_deadline`），也沒有期限前提醒；觸發：開通結果通知（A8）開工。
- 自填聯絡信箱未驗證；觸發：A8 的 email 管道上線時加驗證碼，並把帳號信箱變更同步到 Stripe。A8 只寄已驗證的信箱。
- 同一家店再次核准時，不更新尚未建立外部帳戶的列的聯絡信箱（核准不能寫 payment 的既有列，`docs/modules/payment.md` 已知限制）。
- D3 讓平台承擔負餘額；要上線且改變損失歸屬時重新評估。
