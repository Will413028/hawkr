# 單體裡的商家治理：統一通報、違規與處分分離、通知走事件

| 項目 | 值 |
|---|---|
| 狀態 | superseded by 2026-10-18-seller-governance-consolidated.md（修訂超過 3 段，整合成一份；本檔保留歷史，現況讀新檔） |
| 日期 | 2026-10-02 |
| 影響範圍 | `internal/modules/seller`（治理子套件、停權推導）、`internal/httpapi`（步驟 8 的 Connect）、admin、website、seller-app |
| 取代 | `2026-09-18-seller-governance-schema-and-enforcement.md` 的決策二、三、七、八；決策四改成以處分列計算（見下） |
| 計畫 | `docs/plans/2026-09-29-a3-buyer-seller.md` 步驟 7b |

## 脈絡

治理（檢舉、食安通報、違規、申訴）從 seller-service 搬進 seller 模組。舊實作的盤點找到的問題多半來自資料形狀：

- 處分嵌在違規列裡（`penalty_type`、`penalty_amount_cents`、`expires_at`）：改到期日不重算停權、還原排程每 10 分鐘重跑全部歷史、六種處分只有兩種有作用，另外四種沒有任何地方執行。
- 檢舉與食安通報分兩張表：admin 收件匣把兩張表各自 limit／offset 再合併，翻頁漏筆或重複；兩份狀態機重複。
- 通知送到從未宣告的 exchange，`seller_notification_log` 記下的是沒有送出的通知。
- `sellers.rating` 是 review 事件覆寫的副本，只用來顯示。

約束：
- 外部：沒有台灣的平台治理法規可循；參照歐盟 Digital Services Act 的 Art.16（通報與處理：不需登入也能通報、處理結果告知通報人）、Art.17（每個限制都附理由通知被處分者）、Art.20（申訴）、Art.23（防濫用）。
- 繼承：依賴表 `seller → identity、media`；review 依賴 seller，seller 不得依賴 review。仍成立（單體 ADR D2）。
- 繼承：停權以帳號為單位、只有 `suspension.Recompute` 一個推導點、停權期間是停權事件的身分（計畫 7a 的決定）。仍成立。
- 不再成立：舊決策二「兩種通報的 SLA 與嚴重度不同，共用一張表要寫條件式 CHECK」——條件式 CHECK 的成本遠小於合併分頁與重複的狀態機；舊決策七的 advisory lock 是因為沒有可鎖的 aggregate 列，單體裡店家列就是。系統從未上線，沒有資料要搬。

## 考慮過的方案

**通報**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：一個通報入口（DSA Art.16），通報人是登入的買家或留下聯絡方式的匿名者，類別決定優先序與 SLA；嚴重度由平台分案時決定 | 一張表要以 CHECK 表達「通報人二擇一」「FDA 轉報只在食安類別」 | ✓ |
| 沿用兩張表 | admin 收件匣要跨表分頁；兩份狀態機與欄位 | ✗ |
| 依 D8 刪掉沒有 client 的建立入口 | 通報是 admin 收件匣唯一的來源，刪掉後治理只剩空殼；需求（設計書 32）存在，只是 client 沒做 | ✗ |

**處分**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：違規（認定、申訴）與處分（限制什麼、從何時到何時、誰以什麼理由解除）分開；人工停權也是有理由的處分（DSA Art.17） | 撤銷違規、接受申訴時要在同一個 transaction 解除它產生的處分 | ✓ |
| 從違規列推導治理停權，`stores.suspension` 加治理的值 | 處分仍嵌在違規裡，改到期日、撤銷都要另外記得重算；兩種停權兩條推導路徑 | ✗ |
| 保留六種處分 | 四種沒有執行者，欄位與 CHECK 只是佔位 | ✗ |

**停權狀態：存起來還是讀取時推導**（Will 2026-10-02，7b 審查後）

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：有時效的限制在檢查時比對到期時間，衍生值不存（GCP IAM Conditions 的 `request.time`、AWS IAM 的 `DateLessThan`、SpiceDB 的 expiring relationships、JWT 的 `exp`）；排程只負責副作用 | 每次能力檢查多 join 處分表（索引讀取，與 7a 的 fail-closed 讀取同一級） | ✓ |
| 沿用 7a 的 `stores.suspension` 存推導值，到期由 job 推回 | 兩份事實會隨時間分歧：跨 stint 的競態把生效中的停權寫成 NONE 且對帳修不回、worker 時鐘偏慢時到期 job 白跑、開店前要先重算繞過 | ✗ |

**通知**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：領域在寫入的 transaction 內發事件，通知系統負責送達與送達紀錄；決策的稽核是領域列本身 | A8 的 notification 之前沒有推播，賣家與 admin 以拉取的方式看到 | ✓ |
| 保留 `seller_notification_log` | 在通知系統上線前它記的都是沒送出的通知，是假的稽核 | ✗ |

**店家評分**

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：評價的擁有者維護彙總；只顯示的消費端在呈現層組裝，需要排序或篩選的消費端自己維護讀取模型 | 店家頁多一次對 review 的讀取（步驟 8，經 legacy 到 A6） | ✓ |
| 單體接舊的 AMQP 事件寫 seller 擁有的副本 | 暫時寫法；亂序送達會讓評分倒退；依賴方向相反 | ✗ |

## 決策

- **通報**：seller 擁有一張通報表。通報人是登入的買家（`buyer_id`）或匿名的聯絡方式（電話或 email），CHECK 保證剛好一種；類別含 `FOOD_SAFETY`，FDA 轉報的欄位只在食安類別。嚴重度不由通報人填，admin 分案時決定。去重沿用 partial UNIQUE（舊決策五），日與週的邊界改成台北時間。匿名通報人的聯絡欄位在保留期限後清成 NULL，不做 hash。Connect 要開放哪些方法在步驟 8 決定；client 的入口是 UI 改動，另外取得同意。
- **公開的違規列表刪除**：主流電商不公開賣家的處分，停權的店家直接下架；公布食安違規是主管機關的職責；平台自行公布在誤判時有商譽與名譽的風險。`public_summary` 改成「給賣家的處分理由」。
- **處分**：7a 的 `seller.suspensions` 擴充成處分表，來源是 `ADMIN` 或 `VIOLATION`（帶 `violation_id`），有 `ends_at`（NULL 是無期限）。`AccountSuspended` 只查這張表：沒有解除且沒有到期。每個店家最多一筆生效中的 ADMIN 處分，每個違規最多一筆處分。撤銷違規與接受申訴在同一個 transaction 解除它的處分。到期時以排在 `ends_at` 的 job 重算（發出 `StoreUnsuspended`），另有只掃描停權中店家的對帳 job。違規只保留認定與申訴；處分種類只有警告（不產生處分）與停權，其他種類等有執行者時再加。
- **停權狀態讀取時推導**：處分表是唯一來源，能力（`CapabilitiesOf`）以「帳號名下沒有解除、也沒有到期的處分」在每次檢查時推導；`stores.suspension` 不存。停權期間（`suspension_periods`）與 `StoreSuspended`／`StoreUnsuspended` 仍只由 `Recompute` 產生：處分寫入的 transaction 內呼叫，到期由排在 `ends_at` 的 job 以 `max(ends_at, now)` 判斷，對帳 job 修補「有進行中的停權期間卻沒有生效處分」（或反之）的帳號。job 晚到只讓事件晚到，不影響權限。
- **升級規則**（90 天內第 3 次 medium 升為 high，設計書 32）以店家列的鎖串行，不用 advisory lock。
- **通知**：處分、撤銷、申訴的送出與裁定、通報的確認與結案、admin 要求賣家配合，都在寫入的 transaction 內發模組事件，A8 的 notification 訂閱並保存送達紀錄。`seller_notification_log` 不搬。
- **評分**：seller 不存評分，也不處理 `review.created`／`review.hidden`；店家頁的評分由 `internal/httpapi/seller` 組裝，推薦排行由 `discovery`（可依賴 review）自行決定是否維護讀取模型。

## 後果

- 舊 ADR 的決策一（治理與內容檢舉分開）、五（partial UNIQUE 去重）、六（公開端點的 IP 限流，步驟 8 在入口實作）仍有效。
- 設計書 32 的資料表、狀態與公開列表段落依本 ADR 改寫。
- A8 之前沒有推播：賣家看到處分要打開違規頁，通報人不會收到結果。A8 的 notification 要訂閱本模組的治理事件。
- 重新評估：出現罰款、部分停權等需要執行者的處分時，加新的處分種類與它的執行點；出現依評分排序的需求時，由那個消費端建讀取模型。

## 修訂：佐證是機敏檔、違規修改留紀錄、通報的對外入口（2026-10-04，Will）

原決策段保留歷史。步驟 8g 開放 Connect 時決定三件事。

**佐證的分級**：通報與申訴的佐證（收據與訂單截圖上的姓名、電話、地址，申訴附的發票與進貨單上的第三方個資）原本是公開圖片（公司時期的上傳慣例，沒有評估過個資）。業界的通報與檢舉附件（Amazon 的侵權通報、Meta 與 YouTube 的檢舉）只給審查人員與提交者，放在私有儲存，不給被檢舉方原件。

| 方案 | 代價 | 判定 |
|---|---|---|
| 公開圖片（API 回 CDN 網址） | 拿到網址就能讀、沒有讀取紀錄、無法個別撤回 | ✗ |
| 通報佐證機敏、申訴佐證公開 | 發票與進貨單常含第三方個資；兩套規則 | ✗ |
| 機敏檔，只經後台的擁有者端點讀，每次讀取先記錄再讀（比照 8f 的 KYC 證件，Will 2026-10-04 的機敏檔規則） | 新表與兩條端點；client 改用機敏上傳（步驟 10）；B1 之前 media 整份讀進記憶體 | ✓ |
| 再加 KYC 式的理由與限時授權 | 審案時每件都要做；目前只有一位 OWNER | ✗（觸發條件見下） |
| 機敏儲存加短效簽章網址 | 記不到讀取，R2 存的是 media 加密過的密文（8f 已否決） | ✗ |

決策：轉正時要求 media 回報 `is_secure`，公開的佐證在寫入之前拒絕。後台以 (通報或違規, 位置) 定址的兩條 `/api/files/admin/seller/…` 端點讀佐證，各宣告一個權限（ADR `2026-10-04-admin-roles.md`）；通報與申訴共用一張 append-only 的讀取紀錄表，先寫紀錄、commit 之後才讀 media。API 只回位置與檢視網址，不回 asset ID 或 object key；同樣的理由，通報清單不帶匿名通報人的聯絡方式（只在讀單筆時出現）。提交者自己看自己的佐證、賣家看通報佐證，8g 都不提供。

**違規的修改**：建立、撤銷、裁定申訴都記下管理員，只有修改（內部說明、給賣家的理由、停權的到期時間）不記；改到期時間直接改變賣家的停權。決策：每次實際寫入在同一個 transaction 寫一列 append-only 的修改紀錄（誰、何時、改完的版本、被改掉的值，到期時間記前後）。沒有改變的修改不記。

**通報的對外入口**：登入的買家以 `SellerReportAppService.CreateReport` 通報任何類別、可附佐證，去重期間內的重複回 `already_exists`（那一筆一定是自己的）；匿名的入口只收食安（`SellerReportPublicService.SubmitFoodSafetyReport`，舊系統的公開端點也只收食安：它是公共衛生的緊急管道，其他類別要能追蹤與附圖，需要登入），回應沒有內容，新的與重複的送出無法區分。兩者只能通報曾經公開過的店（與店家卡片同一個規則），公開入口不能拿來探測從沒公開過的店家 ID；送出的額度沿用舊決策六的兩個時間窗（10 分鐘 5 次、1 小時 10 次），匿名以來源 IP、登入以帳號計數。

後果與重新評估：
- 佐證的終點：B1 之後只換 `media.ReadSecure` 背後的 adapter，並由 media 回報型別與大小，擁有者在轉正時限制佐證的型別（圖片與 PDF）與大小。B1 之前若出現大於 10 MB 的佐證，或後台讀取變慢，先在 client 限制大小。
- 改成理由加限時授權：佐證開始收身分證件類的文件時，或第一個非 OWNER 的角色出現、要區分辦案與只讀時。
- 提交者自看：client 要顯示自己交的佐證時，加以「本人或本店」授權的擁有者端點，同一張讀取紀錄表照記。賣家看通報材料：要求配合的流程需要給賣家看材料時，另外決定遮蔽或轉述（避免認出通報人）。
- 佐證含個資卻沒有保留期限：與其他稽核文字一起，在隱私政策定稿或 A8 時評估。
- 讀取紀錄與修改紀錄目前沒有對外的列表方法：第一個非 OWNER 的角色出現、需要在後台稽核時加。

## 修訂：管理員停權以帳號為鍵（2026-10-11，Will 決定）

原決策段保留歷史。「每個店家最多一筆生效中的 ADMIN 處分」改為以帳號為鍵：`seller.suspensions` 加 `account_id`（寫入時由店家的 `account_id` 帶入，migration 回填既有列），`suspensions_admin_active` 改為對 `account_id` 的 partial UNIQUE（`source = 'ADMIN' AND lifted_at IS NULL`）。`Suspend` 在帳號鎖內以生效中的處分判斷，撞到約束時回 AlreadyExists。理由：處分以帳號為範圍（效果歸屬於帳號），新 stint 會繼承舊 stint 的管理員停權；以店家為鍵時，新店再停權會多出第二筆生效中的處分。實作與測試見計畫 10e 段（`backend/docs/plans/2026-09-29-a3-buyer-seller.md`，「第 2a 刀的 review 後」那一列）。

## 修訂：內部說明改成註記時間軸、修改紀錄改成版本表（2026-10-11，Will 決定）

原決策段與 2026-10-04 的修訂保留歷史。步驟 10c 第 4 刀把申訴的並行控制定在「違規的 `version` 只在賣家看得到的內容改變時才加一」，兩個實作者在同一個取捨停下：內部說明（`description`）可以編輯，改它就要讓版本加一，賣家依舊 etag 的申訴因此被擋下，可是賣家看不到內部說明。2026-10-04 修訂的 `violation_changes` 只記被改掉的值，也不足以讓裁定者對照賣家申訴時看到的內容。

**內部說明**（業界基準：Zendesk 的 internal note；Vernon 的 aggregate 邊界；子集合：Zendesk 工單留言是獨立的清單端點，AIP-132 List 加 AIP-158 分頁）：內部說明不是賣家可見內容，也不是違規這個 aggregate 需要用版本保護的不變量。決策：違規列刪除 `description`，內部說明改成只新增的註記時間軸 `violation_notes`，每則記作者、時間與內容，不能改也不能刪，更正就再寫一則。建立違規時第一則註記必填；新權限 `seller.violation_notes.create` 與 `seller.violation_notes.list`（ADR `2026-10-04-admin-roles.md` 的命名，目錄一個方法一個權限）。寫註記不帶 etag、不動 `violations`（`version` 不變），etag ADR D4 不修改：D4 規範 aggregate 內的子表（食材、步驟）經過根並讓版本加一，註記是 aggregate 邊界之外只新增的紀錄，先例是 `evidence_accesses`、`kyc_reviews`。因為註記不受根的並行控制，管理員的 `Violation` 不內嵌它（內嵌會讓 `Violation` 的 etag 看起來涵蓋了它，卻在寫註記時不變）：`ListViolationNotes` 以違規為父、舊到新 keyset 分頁讀取。

**版本**（業界基準：AIP-162 的 revision、Wikipedia 的 `page_latest`）：決策：`violation_revisions` 取代 `violation_changes`，一列是賣家看得到的完整內容（給賣家的處分理由、停權的到期時間）的一個版本，記誰、何時，建立違規時寫第 1 版，之後每次實際改了就寫下一版。內容只存在版本表：`violations` 刪除 `statement_to_seller`，只有 `current_revision` 以複合外鍵（可延遲到 commit 檢查）指向目前那一版，所有讀取（賣家與管理員、單筆與列表、申訴的路徑）以 `(id, current_revision)` 連接版本取得目前的理由；`UpdateViolation` 在同一個 transaction 寫新的一版並讓 `current_revision` 前進。提出申訴時 `current_revision` 複製到 `appeal_revision`（同樣是外鍵），一致性由結構保證而不是 `max() + 1`。`suspensions.ends_at` 保留：它是運作中的狀態（`stint.Effective` 判斷處分是否生效用它），版本的 `sanction_ends_at` 是賣家當時看到的歷史快照，兩者用途不同、不是複本。版本屬於 aggregate（每一版都由一次 `UpdateViolation` 產生並讓 `version` 加一），所以 `GetViolation` 內嵌全部版本與申訴的版本，裁定者比對申訴時與目前的內容；`ListViolations` 不帶。回填不比對時鐘：`violation_changes.version` 序列的第一個缺號就是提出申訴（申訴之前只有修改），只改內部說明的修改不產生版本。回填的前提是 `version` 的每一次加一都有紀錄解釋（建立 1、每筆修改紀錄 1、提出申訴 1、裁定 1、駁回之後的直接撤銷或沒申訴的撤銷 1）；`version` 比 `violation_changes` 早出現，兩者之間的修改沒有紀錄，會被誤當成申訴，所以 migration 先逐筆核對，對不上就 `RAISE EXCEPTION` 列出違規 ID，整個 migration 失敗，不寫出錯誤的歷史（`TestViolationNotesRevisionsMigrationRefusesVersionsItCannotExplain`）。

**與 2026-10-04 修訂的關係**：「讀取紀錄與修改紀錄目前沒有對外的列表方法」只剩 `evidence_accesses` 成立；違規的版本改由 `GetViolation` 對管理員讀取，沒有獨立的列表方法。

後果與重新評估：
- 註記只掛在違規；通報與調查是否也要註記，觸發是 10e 做後台的通報處理介面時。

## 修訂：賣家的 `can_appeal` 與提出申訴起凍結違規（2026-10-12，Will 決定）

原決策段與前兩次修訂保留歷史。步驟 10c 第 4B 刀盤點時，client 各自從 `status` 與 `appeal` 推導「能不能申訴」，規則散在 website 與 App，且 `SubmitAppeal` 的前置檢查是伺服器端第三份。另外，賣家提出申訴後管理員仍能改理由、延長處分，等於在賣家申覆的同時加重措施。

**`can_appeal`**（業界基準：Merchant API 的 `isAvailable`、本 repo 的 `KYCFlow.can_resubmit`）：決策：賣家的 `Violation` 加 OUTPUT_ONLY 的 `can_appeal`，由伺服器判斷，規則是「違規生效中且還沒申訴過」，與 `SubmitAppeal` 的前置檢查共用同一個函式（`model.CanAppeal`），client 不再自己推導（website 現在就讀 `can_appeal`；seller-app 在 4B 搬到 Connect 之前，舊 REST 路徑仍在 `seller_violation_model.dart` 自己推導，4B 改讀 `can_appeal` 後才全部成立）。不加 `revoke_time`、`update_time`（已有 `lift_time`、`decide_time`，沒有讀者）；條款目前不設申覆期限（第十七條第 4 項），契約不加期限或不能申訴的原因，觸發是條款加上期限。

**提出申訴起凍結**（條款草稿第十七條第 3、4 項：措施限於必要範圍、同一措施申覆一次為限，加重是新的措施，可再申覆）：決策：違規有申訴（待裁定或已駁回；接受的申訴已經撤銷，在更早的 ENFORCED 檢查就被擋下）後，`UpdateViolation` 只能縮短到期時間，改 `statement`（往輕改也一樣）或延長到期時間回 `failed_precondition`（沒有改變的重送仍是不寫入）；`RevokeViolation` 的規則不變：申訴待裁定時回 `failed_precondition`（違規只能經 `DecideAppeal` 接受結束），駁回後可以縮短到期時間或撤銷；沒申訴過的違規照舊可改。判斷是 governance 內的未匯出 `frozen(row)`（`AppealStatus != nil`），admin 輸出要顯示「是否凍結」時（觸發是 10e 做 admin 編輯介面）再抽到 model 與 `CanAppeal` 並列。這推翻第 4 刀的「待裁定時不鎖修改」：那條的前提是不加治理規則，但凍結本身就是治理規則；只在駁回後才凍結，待裁定期間的加重會在駁回後變成賣家不能再申覆的措施。有新事實就另立一筆違規，它是新的措施、可再申覆；原理由有誤就依實情裁定申訴，接受即撤銷。撤銷的違規不計入升級規則（`CountRecentMediumViolations` 只計 ENFORCED），另立的那筆以自己的立案時間計入窗口。`failed_precondition` 不帶欄位：`BadRequest` 只屬於 `invalid_argument`（API 設計規範），`apperr` 的欄位只能由 `InvalidInputOn`、`InvalidInputFields` 在建立時給（`*Error` 沒有事後加欄位的方法，分類 `kind` 也未匯出、建立後不能改，只能經 `Kind()` 讀取，所以其他分類不可能帶欄位），也不另加 `ErrorReason`（client 不依原因分流，是管理員介面的文字提示）。

## 修訂：措施與認定分開、效果由措施推導（2026-10-12，Will 決定）

取代：決策段的 `suspensions` 處分表、2026-10-11 修訂的「每個帳號最多一筆生效中的 ADMIN 處分」與 `suspensions.ends_at`、以及註記與版本掛在違規。其餘（通報、佐證是機敏檔、`can_appeal`、凍結）保留，改掛在措施上。

**問題**：效果（停權、警告）散在兩個地方——`suspensions` 管理員停權與違規列內嵌的 `penalty`／申訴，兩者各有一套解除、版本與申訴規則，管理員停權沒有理由、沒有申訴、沒有版本，與 DSA Art.17、Art.20 對每一個限制都附理由、可申訴的要求不一致。

**業界基準**（Meta Account Quality、Stripe Radar 風險處置、DSA Art.17 statement of reasons）：認定（finding）與限制（restriction）是兩個物件，限制引用認定，限制本身有理由、期間、申覆與覆核。

**決策**：
- `violations` 只是認定。新增 `measures`（警告或停權，依據是一到多筆認定或風險類型；給賣家的理由與結束時間的目前值也存在這裡，見下一節修訂）、`measure_revisions`（版本歷史）、`measure_findings`、`measure_notes`、`measure_requests`（申訴）、`measure_request_evidence`、`measure_identities`。刪除 `suspensions`、`suspension_identities`、`violation_revisions`、`violation_notes`、`appeal_evidence`。
- 效果只由措施推導：「生效中」的定義是未撤銷、已開始、`ends_at` 為空或在未來的停權（`ends_at` 是 `measures` 的目前值）；指定帳號的讀取是 `EffectiveMeasures`，問全部帳號的讀取是 `EffectiveMeasuresAll`（兩段都在 `measures` 的部分索引上有 Index Cond，讀取量只隨目前仍有效力的措施成長，不隨歷史成長，見下一節修訂），`model.StateOf` 是同一個定義的 Go 寫法；`TestEffectiveMeasuresAndStateAgreeAtTheBoundaries` 鎖住逐帳號的讀取與 `StateOf` 在邊界，`TestEffectiveMeasuresPerAccountAllAccountsAndStateOfAgreeOnADataset` 以含已撤銷、警告、邊界、多版本、已結束與開放式 RISK 的資料集鎖住三者的生效中集合相等。不建效果表。
- 管理員停權就是 RISK 措施（沒有固定結束時間、必須有複審期限、風險排除時結束）；`SuspendStore`／`LiftSuspension`／`ListSuspensions` 刪除。同一帳號可以有多個生效中的措施，不再有「每帳號一筆」的約束。
- 兩個 etag：後台是 `measures.version`，賣家是 `current_revision`；註記不改任何 etag。
- 升級規則以帳號計（`violations.account_id`），與效果以帳號為單位一致。
- 撤銷帶 `revoke_findings` 時連帶撤銷引用同一批認定的其他措施（`FINDING_REVOKED`），待處理的申訴以 CLOSED 結案。
- 建立措施可帶 `request_id`（AIP-155）。
- 沒有新增 `RiskGround` 的值（W2=(a)）。

**遷移（W2）**：違規 1:1 回填成措施，ID 沿用；申訴搬成請求（接受時沒附理由的補「（裁定時未記錄理由）」）；任何 `source = 'ADMIN'` 的處分列（含已解除）沒有誠實可用的風險類型，migration 整個失敗並列出 ID，清掉之後重跑。

**後果與重新評估**：
- 第二種賣家請求已定案：補正請求（賣家修正風險後請求解除，RISK 措施、可重複、同時最多一筆待處理）是 2c-2 刀，在計畫 10e（`2026-09-29-a3-buyer-seller/step10e-2c-design.md` G14、2c-2 列）：`measure_requests.kind` 加 `REMEDIATION`（只增值的 migration）、`RequestRemediation`、`DecideRemediation`、`can_request_remediation`、`can_decide_remediation`。再新增請求種類時同樣加 `RequestKind` 值與對應的 `Can*`。
- 通報與調查的註記仍未做，觸發同 2026-10-11 修訂。

## 修訂：措施的目前值放在主表、版本表只存歷史（2026-10-14，Will 決定）

**問題**：2c-1 把賣家看得到的內容（給賣家的理由、`ends_at`）只放在 `measure_revisions`，`measures.current_revision` 指向目前那一版，所以「此刻生效中」的讀取都要 join 到目前版本。沒有固定期間的 RISK 那一段因此沒有辦法用索引條件排除已結束（`ends_at` 已寫入）與已撤銷的措施，只能掃 `measures_review` 並用 Filter，讀取量隨未撤銷的 RISK 筆數成長（審查 r3 的 R3-1）。

**業界基準**：SQL:2011 的系統版本表（system-versioned table）：主表存每筆資料的目前值，歷史表存每次被改掉的版本，兩者由同一個寫入維持一致；目前狀態的查詢只讀主表，歷史查詢才讀歷史表（PostgreSQL 沒有內建，慣用做法是同一個 transaction 內雙寫，或 trigger；SQL Server、MariaDB、Oracle 內建）。

**決策**：`measures` 存每個會版本化欄位的目前值（`statement_to_seller`、`ends_at`），是目前值的唯一來源；`measure_revisions` 是不可變的快照歷史，每一版一列、含目前那一版，所以版本歷史、申訴引用的版本與賣家 etag（`current_revision`）的語意不變。寫入新版本只有一條程式路徑：`InsertMeasure`（第 1 版）與 `AdvanceMeasureRevision`（之後每一版）各以一個 SQL 語句（CTE）同時寫 `measures` 並追加歷史列，`current_revision` 經可延遲的複合外鍵 `(id, current_revision)` 指向歷史。所有「此刻生效中」的讀取（逐帳號、全部帳號、對帳、`ListStores` 的 suspended 篩選、`ListMeasures` 的 state 篩選、`StateOf` 的輸入）只讀 `measures`。`measures_period_check` 讓警告一定沒有 `ends_at`、依據認定的停權一定有，所以沒有固定期間的只可能是 RISK，不需要跨表約束。索引（皆為部分索引）：`measures_active_timed (ends_at, created_at)` 涵蓋未撤銷、有結束時間的停權，以 `ends_at > now` 與 `created_at <= now` 為 Index Cond；`measures_active_open (created_at)` 只含未撤銷也未結束的開放式停權（結束寫 `ends_at`、撤銷改 `status`，列都會離開索引），以 `created_at <= now` 為 Index Cond。`measure_revisions_ends_at` 刪除。

**取捨與約束**：
- 目前值與歷史是兩份資料。目前值＝歷史裡 `current_revision` 那一版的一致性由單一寫入路徑（單一 SQL 語句）與 `assertCurrentValuesMatchHistory`（`TestEffectiveMeasuresPerAccountAllAccountsAndStateOfAgreeOnADataset`、`TestMeasureRevisionsRecordEachSellerVisibleVersion`）維持；`ends_at` 可為 NULL，複合外鍵（預設 MATCH SIMPLE）對含 NULL 的列不檢查，所以資料庫不能用外鍵保證值相等，只保證 `current_revision` 指到存在的版本。從零設計也不會用 trigger 去補（隱藏的寫入路徑）。
- 不改的：`MeasureState` 語意、後台與賣家的 etag 規則、註記不改 etag、申訴凍結規則、`request_id` 流程、所有 API 行為。
- migration `20261012214337_seller_measures.sql`（尚未發佈，直接折進）從目前版本的 `sanction_ends_at` 與理由回填 `measures`；警告的目前版本若有結束時間，回填前檢查失敗並列出 ID。

## 修訂：補正後請求解除（2026-10-15，Will 決定「全照建議」）

原決策段與前幾次修訂保留歷史。2c-2 依 `2026-09-29-a3-buyer-seller/step10e-2c2-brief.md` 實作補正請求，與前文有四處不同：

- **模型**：RISK 是條件型的限制（風險消除就該解除），補正開放給 RISK（含條款第 17.5 條的外部要求，修訂 2c 設計書 G21），可重複，同時最多一筆待處理的請求（申訴與補正合計，`measure_requests_one_pending`）；VIOLATION 是懲罰型，期間本身就是處分，只有申訴。接受補正是措施結束（寫一版 `ends_at` 是現在、發 `MeasureEnded`），不是撤銷，認定不動；待處理的補正凍結加重（改理由、延長、結束），縮短、撤銷與接受補正不受限。
- **冷卻**（條款第 17.4 條）：同一措施上被駁回的請求（申訴與補正合計，`CLOSED` 不計）累計超過兩次後，每再被駁回一次就起算冷卻 7、14、28 天，上限 28 天；由請求推導、不存，`Measure.next_remediation_time` 與 `rejection_cooldown` 由伺服器給。
- **修訂「只增值的 migration」**：`measure_requests` 加 `basis`（由 `measures` 回填）、複合外鍵 `(measure_id, basis)` 與 CHECK `kind <> 'REMEDIATION' OR basis = 'RISK'`，由 DB 保證補正只掛風險措施；加 `close_cause`（`REVOKED` 必有 admin，`ENDED` 可以是系統結案）。「只增值」是切分時的估計，不是限制。
- **修訂「`DecideRemediation(measure_id, request_id, …)`」**：不帶 `request_id`。同一措施至多一筆待處理請求，etag 保證裁定的是管理員看到的那一筆；兩個 client 的執行期會把名為 `request_id` 的欄位當冪等鍵自動重試。

補正只在措施生效中才有標的，這個不變量由每個需要請求名額的寫入路徑強制（`CanAppeal` 不算已結束措施上待處理的補正、`SubmitAppeal` 在同一個 transaction 先以 `CLOSED`（`ENDED`）結案、`DecideRemediation` 要求生效中），請求對外的狀態在讀取時由 `model.Measure.EffectiveRequest` 推導（已結束措施上儲存為 `PENDING` 的補正輸出 `CLOSED`／`ENDED`、結案時間是措施的結束時間），所以讀取不依賴任何 job；儲存值與 `MeasureRequestClosed` 由 `measure` 套件自己的週期 job（`seller.measure_close_ended_requests`，只掃已結束而仍有待處理補正的措施，寫回的 `decided_at` 同為 `ends_at`）收斂，停權的到期 job 與對帳不管請求（2026-10-15 設計審查，Will 決定：取代原先經 `suspension.Deps.AfterRecompute` 的接法）。條款第 17.4 條改版（版本 2026-10-15）；已開店的賣家不要求重新同意。

## 修訂：終止合作（2026-10-15，Will 決定「全照這版做」）

原決策段與前幾次修訂保留歷史。2c-3 依 `2026-09-29-a3-buyer-seller/step10e-2c3-brief.md`（定案版）把賣家合作條款第 21 條的終止做進措施模型，與前文有下列不同；與 `step10e-2c-design.md` 衝突處以 2c-3 簡報為準。

- **種類與依據**：`MeasureKind` 加 `TERMINATION`，`MeasureBasis` 加 `CONTRACT`（只配終止：預告終止、關聯帳號與沒有認定的第 21.3 條事由）。終止一定有事由 `termination_ground`（`NOTICE`、`MATERIAL_BREACH`、`CEASED_BUSINESS`、`DISSOLUTION`、`BANKRUPTCY`、`LICENSE_REVOKED`、`FOOD_SAFETY_INCIDENT`、`FRAUD`、`PLATFORM_SECURITY`、`OTHER_SERIOUS`、`LINKED_ACCOUNT`），永遠不是 `RISK`，所以沒有複審期限、補正與 `EndMeasure`（`RemediationUnavailableReasons` 的 `NOT_RISK`）。`NOTICE` 與 `LINKED_ACCOUNT` 一定是 `CONTRACT`，`MATERIAL_BREACH` 一定是 `VIOLATION`，其他事由有認定是 `VIOLATION`、沒有是 `CONTRACT`；這些組合與 `(source_measure_id IS NOT NULL) = (termination_ground = 'LINKED_ACCOUNT')` 都由 CHECK 保證。終止建立時沒有 `ends_at`，API 不接受終止的 `end_time`；只有系統在店家關閉時為預告終止寫入。
- **開始時間**：所有措施加不可變的 `starts_at`（修訂原決策「`created_at` 是開始」）：停權與立即生效的終止等於 `created_at`，預告終止（第 21.2 條）是 `created_at` 加 720 小時（固定時數，不用 `interval '30 days'`：timestamptz 加日數依 session 時區計算）。「生效中」只有一個式子：`ENFORCED ∧ kind IN (SUSPENSION, TERMINATION) ∧ starts_at <= now ∧ (ends_at IS NULL ∨ ends_at > now)`，`EffectiveMeasures`、`EffectiveMeasuresAll`、`ScheduledMeasureIDs` 與 `model.StateOf` 四處用同一個定義，由資料集測試鎖住。契約沿用 `MeasurePeriod.start_time`，新增狀態 `SCHEDULED`（預定生效；`StateOf` 的順序是已撤銷、警告、`ENDED`、`SCHEDULED`、`ACTIVE`，預告期間內因店家關閉而結束的措施 `ends_at` 早於 `starts_at`，是 `ENDED`；不認得的種類 panic，不落進任何時間分支）。devseed 自己的措施 SQL 刪除，改問 `EffectiveMeasures`。
- **申覆期間**：終止的申覆期限是生效後 4416 小時（184 天，任何連續六個曆月的最大長度，不短於條款的「六個月」與 DSA 20(1) 的 at least six months），推導、不儲存，輸出 `Measure.appeal_deadline_time`；期限內（含預告期間，措施還是 `SCHEDULED`）可申訴，期滿後 `can_appeal` 是 false。申訴的可用性由伺服器推導：`AppealUnavailableReason`（依序 `REVOKED`、`ALREADY_APPEALED`、`APPEAL_WINDOW_ENDED`、`REQUEST_PENDING`），`CanAppeal` 與 `SubmitAppeal` 的前置檢查共用同一個函式（比照 `remediation_unavailable_reasons`）。已在期限內提出的申訴照常裁定。
- **關店可逆**：生效中的終止在沒有未結訂單時由 job `seller.termination_effect`（以 measure_id 唯一、排在 `starts_at`）關店（`closed_by = TERMINATION`、`closed_by_measure_id`）；步驟是 transaction 內鎖帳號並以 `max(starts_at, now)` 重算停權期間、transaction 外問未結訂單（fail-closed）、關店 transaction 內先鎖 identity 帳號與帳號全部的 stint、重讀措施並重新判斷（未撤銷、生效中、stint 還是查過的那一個而且不在等待刪除）才關。「待關閉」不存（生效中的終止加 stint 未關閉）。`order.completed` 事件加速（經 `buyerconsumer`），每小時 `seller.reconcile_terminations` 保底；River 去重的狀態集合必含 running，執行中被去重的加速由掃描補，方向是「晚關」。關店不再釋放 logo、封面也不再讓 relay 發 `seller.deleted`：不可逆的點移到保留期滿的個資清理（`StoreRetired`）。撤銷終止（`RevokeMeasure`、接受申訴、連帶撤銷）時，店家在申覆期間內於同一個 transaction 恢復——條件是造成關店的那一筆（`store_closures.closed_by_measure_id`）已撤銷，而且撤銷之後帳號沒有生效中的終止（`closing.Reopen`：同一個 `store_id`，結束目前的關店紀錄、重新授予 SELLER、重新公告可曝光、發 `StoreReopened`、補排被放棄的 PayUni 綁定）；帳號已開新店、帳號在等待刪除或保留期已滿時只撤銷。恢復與否看撤銷之前的資料（`ReopenPlan` 在改動任何請求之前決定），所以期滿前提出、期滿後裁定的申覆仍能恢復。
- **保留期與 legal hold**（修訂 ADR `2026-10-02-seller-kyc-payuni-in-monolith`「個資保留」）：`TERMINATION` 關閉的 stint 保留到 `greatest(關閉後 30 天, 帳號上未撤銷的終止中最晚的申覆期限)`（已撤銷的終止不延長），其他關閉 30 天；帳號上有終止的待處理申覆（或帳號上的關聯終止其來源措施有待處理申覆）時，帳號全部已關閉的 stint 都不清理。判斷式只有 `RetireDueStores` 一支查詢，清理與恢復共用，時數由 Go 常數給。
- **第 21.2 條在單一關店路徑結束**：`closing.close()`（賣家自己、終止、刪帳）把帳號上尚未結束的預告終止寫一版 `ends_at = 關店時間`（作者是系統：`measure_revisions.admin_id` 可為 NULL，`system_cause = STORE_CLOSED`，比照 `close_cause`）、發 `MeasureEnded` 並重算停權，否則 `stint.Derive` 會永遠擋住 21.2 的重新申請。第 21.3 條措施永不結束（只能撤銷），以既有的 `stint.Derive` 擋同帳號開店、以措施的身分快照擋其他帳號以同一身分證號或統編送 KYC；快照逐措施在建立時取（停權與非預告的終止，不靠 `Recompute` 的轉換），保留到措施撤銷。
- **關聯帳號（W9）**：措施詳情的 `linked_accounts` 反查身分快照命中的其他帳號；管理員對每個帳號另建一筆 `LINKED_ACCOUNT` 終止（`source_measure_id` 指向來源，來源必須是另一個帳號、未撤銷、事由不是預告或關聯帳號的終止，伺服器再驗證 KYC 身分確實命中來源的快照，同一來源對同一帳號只能一筆）；來源撤銷（三條路徑：`RevokeMeasure`、接受申訴、`FINDING_REVOKED`）時同一個 transaction 連帶撤銷（`SOURCE_REVOKED`），待處理請求以 `CLOSED` 結案、申覆期間內的關聯店家恢復；關聯措施自己的申訴成立只撤銷它。
- **鎖順序**：identity 帳號鎖先於店家列（恢復營業要授予 SELLER），跨帳號一律依 account_id 升序；關店改成鎖帳號全部的 stint（取代 `LockStore(current)`），與措施寫入的升序鎖不再互鎖；撤銷涉及終止時先預讀要碰的帳號、依序鎖住、鎖後重讀，範圍變了就整個 transaction 重來。
- **刪除**：管理員直接關店（`CloseStore`、`CloseByAdmin`、權限 `seller.stores.close`、`stores.closed_by = ADMIN` 與 `closed_by_admin_id`、`close_reason`）整組刪除，平台關店一律經終止措施（理由與作者在措施裡）。既有的 `ADMIN` 列只在本機：migration 遇到就失敗並列出 `store_id`（先例：`suspensions.source = 'ADMIN'`）。
- **事件**：`MeasureCreated` 加 `StartsAt` 與事由，不新增 `TerminationScheduled`（沒有訂閱者，開始時間是所有措施的屬性）；新增 `StoreReopened`、`StoreRetired`；`StoreClosed` 在單體內只有 relay 訂閱，relay 改成 `StoreRetired` → `seller.deleted`。
- **legacy**：`EffectiveMeasure` 帶 `Kind` 與事由，終止（不論依據）在舊契約是管理員停權（9603），標籤優先用終止事由。
- 條款第 21.2–21.4 條與賣家身分認證告知事項（第 4 項）照簡報定案文字改版，版本行改成改版當日、不要求重新同意；條款與告知事項未經律師審閱。

## 修訂：政策分類取代違規類別（2026-10-18，Will 決定「照建議」）

原決策段與前幾次修訂保留歷史。10e 第 3c-a 刀的第一個 commit（`docs/plans/2026-09-29-a3-buyer-seller/step10e-3ca-design.md` §1.1、§2.1，分類清單依 `step10e-reports-brief.md` R10）把 seller 自己的七值違規類別換成平台共用的政策分類，與前文有下列不同：

- **一份政策分類**：`hawkr.common.v1alpha1.PolicyCategory`（13 個值，契約 `api/hawkr/common/v1alpha1/policy_category.proto`）與 Go 的 `internal/platform/policy`，取代 `seller.v1alpha1.ViolationCategory` 與 seller 的 `model.Category`。檢舉人選的類別、平台判定類別（之後的案件類別）與認定的類別共用它；放在 common 與 platform，是因為 A7 的 moderation 也共用（A7-1c 再加社群的三個值，同一段放契約與消費者），platform 只依賴 platform，沒有依賴環。每個值對應賣家合作條款第十一條、第十六條的依據，給賣家的理由要能指到條款（DSA 第 17(3)(e) 條）；名稱先進〈使用者面用語術語表〉的「治理」，再進共用文案目錄 `common.PolicyCategory.*`。
- **各入口只接受自己的子集，伺服器驗證**：`ReporterSelectable`（檢舉人可選，12 個，沒有 `PLATFORM_ABUSE`）、`Assessable`（平台判定類別與認定，12 個，沒有 `OTHER`）。「其他」是檢舉人無法歸類時選的，認定的理由指不到條款，所以分案時要改判成有依據的類別或直接設定結果；`PLATFORM_ABUSE`（干擾或濫用平台）只有平台自己看得出來，檢舉人不能選。`reports_category_check`、`violations_category_check` 各綁自己的子集（`TestChecksMatchGo`）；`CreateReport` 的類別、`ListReports` 的類別篩選與 `CreateMeasure` 的新認定（`NewFinding.category`）在伺服器驗證各自的子集。client 的子集目前是 admin 的常數（`admin/src/lib/policy-category.ts`），因為目前只有 admin 用子集；第二個 client 要用子集（10f 的檢舉表單）時，改成 proto 的 custom option，由 `uitext-gen` 生成三端常數。
- **改名與值的搬遷**：`UNAUTHORIZED_SALE` 改名 `UNLICENSED_SALE`（條款 11.2 說的是登錄、許可或資格沒有完成，不是「未經授權」），`reports`、`violations` 的值由 migration 改寫；認定的類別是 `OTHER` 的列沒有誠實的對應（改成別的類別會寫出錯誤的歷史），migration 整個失敗並列出認定的 ID（先例：`closed_by = ADMIN`），本機重建資料庫後重跑——系統從未上線，只有開發資料，不受相容性約束。`measures.request_hash` 不重算，只影響本機資料以舊類別的同一個 `request_id` 重送。
- **賣家端的顯示跟著改字**：措施依據的類別名稱取自目錄，食安→食品安全、詐騙→詐欺或操縱交易、仿冒→仿冒商品、未授權販售→未經許可販售、誤導資訊→不實或誤導資訊；「個人資料」寫成「不當使用個人資料」，因為個人資料本身不是違規的名稱。
- 不變的：通報的狀態機、嚴重度、去重、FDA 轉報與認定的升級規則；案件模型（通報與案件分開）在後續的修訂。

## 修訂：通報與案件分開（2026-10-18，Will 決定「照建議」）

原決策段與前幾次修訂保留歷史。10e 第 3c-a 刀的第三個 commit（`docs/plans/2026-09-29-a3-buyer-seller/step10e-3ca-design.md` §2–§4，決策來源 `step10e-reports-brief.md` R2、R3、R7）把「一筆通報就是一個處理流程」改成通報與案件兩個資源，與前文有下列不同：

- **兩個資源**：通報（`reports`）是一位檢舉人的一次送出，有自己的結果；案件（`cases`）是平台對「這家店、這個平台判定類別」的處理單位，同時最多一個進行中（`cases_one_open`），決定即結案，結案是終態（同店同類別的新通報開新案件）。嚴重度、決定、內部註記、FDA 轉報（`case_referrals`）與導出的措施都掛在案件上：一次事件對主管機關只轉報一次、對賣家只處理一次。通報不再有狀態欄與嚴重度，確認收件由分案取代，「調查中」拿掉。統一入口（一張 `reports`、類別決定優先序）的決策不變，狀態機的部分由本段取代。
- **結果與決定**：通報的結果是已採取措施、未違規、內容不足、重複、濫報，設定後不能改；「已採取措施」只能設在案件裡的通報，而且案件要已有作為（有導出的措施，不論狀態，或已有成員的結果是已採取措施）。分案把待分案的通報放進案件（這家店、這個類別沒有進行中的案件時開新的並決定嚴重度）或直接設定結果；移動等於改這筆的平台判定類別，已是認定來源的通報不能移，通報都移走的案件以 `NO_REPORTS` 自動關閉，不算一次決定；決定（採取措施或不採取措施）即結案，把還沒有結果的成員帶入「已採取措施」或「未違規」。「不採取措施」只在案件沒有未撤銷的導出措施、也沒有已採取措施的成員時可選，所以每個進行中的案件至少有一個可選的決定。
- **推導，不存**：處理狀態（通報：待分案、在案件中、已有結果；案件：進行中、已結案）與此刻做得到什麼動作，都在讀取時由事實推導，只有 `ReportFacts.Availability`、`CaseFacts.Availability` 兩個函式回答，API 的 `can_*`、`available_outcomes`、`decision_options` 與指令的前置檢查看同一份結果（比照措施的 `*UnavailableReasons`）。案件導出的措施是 `measures.source_case_id` 指向它，或措施依據的認定以它的成員為來源。
- **補充**：同一位檢舉人在去重期間內再次送出不是失敗：內容併成原通報的補充（`report_supplements`，只新增，一筆通報最多 10 則，匿名超過的內容不存、只計次數），原通報的版本前進，案件不鎖也不改版本，只顯示「有新的補充」提醒結案前先看。去重只比對還沒有結果的通報；登入的檢舉人不能檢舉自己的店。
- **保留規則與事件**：匿名檢舉人的聯絡欄位在 `greatest(建立 + 90 天, 結果 + 30 天)` 之後清除，沒有結果的不清。每次送出發 `ReportReceived`（新的、補充或丟棄；匿名帶這次送出的 email 給 A8 寄確認信，不落表），每筆通報有了結果發一次 `ReportResolved`；`ReportAcknowledged` 與 `SellerCooperationRequested` 刪除（要求賣家配合改成案件的詢問，3c-b 做；`report_cooperation_requests` 留給它）。
- **來源規則**：認定的來源通報（3b）從「不是不成立」改成「同一家店，結果不是未違規、內容不足或濫報」（`Report.Supporting`），API 的 `can_cite_as_source` 是同一個規則加上店家沒有關閉；建立措施可以連到案件（`source_case_id`，同店、進行中，任何依據都能連）。
- **鎖順序**：identity 帳號、店家（帳號全部的 stint）、通報（id 升序）、案件（id 升序）、措施。分案、移動、決定與建立措施先鎖店家，同店同類別的開案由它串行；補充只鎖原通報。
- **開發資料**：migration 把舊狀態轉成新模型（進行中的通報依店家與類別合併成一個進行中的案件；查證屬實每筆一個已結案的案件；不成立只留結果；FDA 轉報搬到案件）；沒有誠實對應的列（查證屬實而類別是 `OTHER`、有 FDA 轉報卻不會進任何案件、合併後有兩筆以上轉報、配合要求掛在不會進案件的通報上）整個失敗並列出通報的 ID，本機重建資料庫後重跑，步驟見 seller 的 `CARRYOVER.md`。系統從未上線，不受相容性約束。
- 不變的：去重的時區（台北日與台北 ISO 週）、匿名不收佐證、佐證是機敏檔、認定的升級規則與措施模型。

## 修訂：匿名聯絡方式加 365 天絕對上限（2026-10-18，Will 決定）

原決策段與前幾次修訂保留歷史。「通報與案件分開」修訂的保留規則寫「沒有結果的不清」，匿名檢舉人的電話、email、姓名因此可能無限期保留（GDPR 5(1)(e) 儲存限制）。Will 決定加絕對上限，取代那一句：

- 清除條件有兩條，任一成立就清：一是有結果的，`greatest(建立 + 90 天, 結果 + 30 天)` 之後（原規則不變）；二是不論有沒有結果、案件有沒有決定，建立滿 365 天（`created_at < now − 365 天`，剛好 365 天不清）。
- 清理 job 回報每筆清掉的通報是否只靠上限清的，大於 0 時記一筆 warn log：沒有結果卻拖滿一年，代表處理流程出了問題，值得被看到。
- 實作與測試：設計書 `step10e-3ca-design.md` §3.4、附錄 D #11；索引 `reports_contact_cap`（依建立時間；清理查詢兩條路共用它）。
