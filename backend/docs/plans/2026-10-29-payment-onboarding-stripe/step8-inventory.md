# 步驟 8 刪除盤點（唯讀，快照 commit `499c5ae8`）

步驟 8 原本引用的 `step4-inventory.md`（A–E 類）不在分支的歷史裡（`git log --all -- '**/step4-inventory.md'` 0 筆），本檔取代它。細表在同名目錄：

| 檔 | 範圍 |
|---|---|
| `step8-inventory/backend.md` | `backend/internal/**`、`backend/cmd/**`、seller 的 SQL 與 migration |
| `step8-inventory/proto.md` | `api/hawkr/**/*.proto` 的每個符號與各端引用數 |
| `step8-inventory/website.md` | website、`packages/hawkr-ui-text-ts`、`packages/hawkr-api-ts` |
| `step8-inventory/apps-admin.md` | seller-app、buyer-app、admin、Dart 套件 |

四份都是讀程式與 grep 得到的，沒有 build 或跑測試；「會壞」由呼叫點推得，未經編譯器確認。各檔第一節有重現用的指令，最後一節列沒涵蓋的部分。

## 分類（取代 A–E 類）

| 類 | 內容 | 去向 |
|---|---|---|
| P | PayUni 收款開通：`platform/payuni`、`seller/internal/payuni`、`payuni_*` 表、admin 的 4 支 RPC、`PAYUNI_BIND_RETRY`、`PAYUNI_TERMS`、`internal/legacy` 的接縫 | 刪（D6、D11、D14、D18） |
| F | 流程狀態與鎖：`KYCFlow`、`KYCFlowStatus`、`identity_locked`、`CheckNameUnlocked` | 刪（D15、D17；D35） |
| I | 平台自建的 KYC 個資管線：身分證號、銀行帳號、證件與相關欄位、密文、hash、顯示全文 | 刪（D22；D36–D38、D42、D45） |
| D | 買家付款、訂單、退款、撥款、物流（含 7-ELEVEN 門市地圖、`PayUniSubmitStatus`）、Java 推播 `PAYUNI_STATUS`、CodeMeta 生成物 | 保留，B 階段處理 |
| N | 只是名稱：舊 repo 名 `backend-payuni#NNN`、ADR 檔名、已套用的 migration、CARRYOVER 的沿用紀錄 | 保留，列入允許清單（D41） |

## 規模

| 區域 | 命中的檔 | 刪 | 改 | 保留（D、N） |
|---|---|---|---|---|
| backend（非 `gen/`） | 160（測試 63） | 29 | 約 120 | 約 11，另有 15 個已套用的 migration |
| `api/hawkr` | 5 個檔 119 行 | RPC 4、message 13、整個 enum 3 | 見 `proto.md` | — |
| website 與 TS 套件 | 109 | 2 | 16 | 81（D 60、N 21） |
| seller-app | 93 | lib 1、test 2 | lib 9、test 11 | D 17、N 36 |
| buyer-app | 35 | 0 | 0 | 全部 |
| admin | 27 | 1（另 2 個依 D36） | 11、test 7 | — |

刪與改的數字是盤點者手算的分類，不是指令輸出。

## 盤點發現的既有問題

- **個人賣家的證書比對已經沒有基準**：`MatchCertHolder` 以申請的 `real_name` 比對政府證書上的業者名稱（`backend/internal/modules/seller/internal/kyc/admin.go:647`），個人賣家沒有 `company_name`，`real_name` 是唯一基準。7b、7c 之後兩端表單都不送 `real_name`（`git grep -n -i -E 'real_?name' -- website/features website/lib seller-app/lib` 只剩訂單的收件人欄位），所以分支上新送件的個人賣家一律走「沒有基準、轉人工」。處置見 D37。
- 7c 審查帳本的 R13 在 repo 裡找不到內容；推測是進度表同一格寫的「刪 flow 時要改 `MyKyc.fromApplication`」。

## 決定題（D35–D45）

Will 2026-11-12：D35–D41、D43、D44 與子步驟拆法照建議。D42、D45 在第 2 輪審查後重寫，未決。

名詞：PayUni 註冊是核准後平台替店家向 PayUni 建立的商店紀錄（`payuni_registrations` 表）；fingerprint 是金流商回報的銀行帳號指紋，同一個銀行帳號在不同帳戶得到同一個值；CHECK 是資料表上限制欄位值域的約束；CARRYOVER 是模組的沿用盤點表（每個從舊服務照搬的機制一列）。

### 前提題：這些程式之後會不會被重寫

本步驟刪掉的程式沒有排定的重寫步驟。計畫寫了三個「條件成立才重建」的觸發，都不是排定的步驟：

| 觸發 | 會重建什麼 | 重建量 | 分類 |
|---|---|---|---|
| 決定接台灣金流商（D22：平台自建的個資管線退場的重建條件） | 平台收身分與銀行資料的表單、密文、顯示全文 | 依當時金流商的欄位重新設計，舊程式只能參考（git 可找回） | REAL：取決於還沒有的合約與金流商規格 |
| 同上（D11：金流商條款的同意種類） | 平台代收金流商條款的同意種類 | 一個 enum 值、一個 CHECK、manifest 一筆 | REAL，同上 |
| A8 搬通知模組（D18：舊事件停發） | 賣家收款開通結果與平台審核結果的通知 | 由 A8 依新的通知設計重做，不沿用舊事件 | SELF-IMPOSED：我們自己的排程；現在保留舊事件是為沒有部署的 Java 通知服務留接縫 |

所以選錯的重工只有本步驟自己。系統未上線，沒有正式資料；下面各題的「限制」欄只列把建議推離業界基線的限制，沒有的寫「無」。

### D35 流程狀態 `KYCFlow`／`KYCFlowStatus` 的去留

- 現況：`KYCFlow` 是後端把平台審核與 PayUni 註冊合成的一個物件，帶合成狀態 `KYCFlowStatus`（11 個值）、可否重送 `can_resubmit`、身分欄位鎖 `identity_locked`、PayUni 明細。PayUni 拿掉後剩 5 個值（`NOT_STARTED`、`DRAFT`、`SUBMITTED`、`REJECTED`、`APPROVED`），與申請自己的狀態 `KYCStatus` 只差「沒有申請」。
- 業界基線：一條軸一個狀態來源；子資源不存在用「欄位不出現」表示，不另造一個狀態值（Google AIP-149 的 `optional`，https://google.aip.dev/149）。Stripe 的帳戶狀態也只有一個來源（https://docs.stripe.com/connect/accounts-v2）。
- 與已決事項的關係：D15（賣家端狀態拆成平台審核與收款帳戶兩條）把平台審核寫成五個值，含 `NOT_STARTED`；7e1 之後賣家端的「尚未開始」已由伺服器的開通摘要 `GetMyOnboarding` 表達。唯一還直接讀 `NOT_STARTED` 文字的是 admin 商家清單的徽章（`admin/src/app/admin/merchants/store-row.ts:62-90`，顯示「尚未申請」）。
- 選項：
  - A（建議）：整個刪。admin 清單與店家摘要的欄位改成選填的 `KYCStatus`，沒有申請就不帶值；admin 對「不帶值」顯示自己的文字「尚未申請」（這是「沒有資料」的畫面文字，不是 enum 成員的文字）。可否編輯與重送由狀態決定（草稿、已退回可以；審核中只能取消），website 已經這樣做（`website/lib/seller/kyc.ts:498`），seller-app 跟上。文案目錄只把還有畫面在用的那組徽章文字留在 `seller.KYCStatus.*`（7a 已建），沒有畫面用的 banner、gate、action 各組刪掉。這等於把 D15 的 `NOT_STARTED` 改成「沒有申請」。失敗條件：之後有第二個畫面要從伺服器拿「尚未開始」的文字，而它不是開通摘要涵蓋的畫面。
  - B：留一個五值的 enum（改名成平台審核狀態）給清單與摘要用，`KYCStatus` 留給申請本身。失敗條件：兩個 enum 表達幾乎同一件事，三端各自選一個讀，回到 D26（開通狀態由伺服器推導）要解決的分歧。
- 連帶要定：已核准的申請現在可以重送（`kyc/kyc.go:409`；實際只在 PayUni 註冊可修正時放行）。建議改成不能重送；核准後要改登記資料是另一個功能，延後，觸發條件：賣家要求改登記資料。
- 推翻建議的事實：admin 之外還有畫面需要「尚未申請」的伺服器文字。
- 可逆性：之後要加一個狀態值不破壞契約；成本是三端生成與一組文字。
- 限制：無。

### D36 後台怎麼看保留下來的公司補充文件

- 現況：顯示全文 `RevealKYCSensitive` 做兩件事：回三個明文欄位（身分證號、負責人身分證號、銀行帳號），並發 15 分鐘授權讓同一位管理員讀證件圖片。公司文件的檢視網址與 `asset_id`（退回時用來指出哪一張）只在它的回應裡。每次讀圖寫一列讀取紀錄 `kyc_document_accesses`，這張表的每一列都掛在授權列上（`seller/db/schema.sql:1264-1273`），後台沒有畫面讀它。
- 補充文件實際收什麼（讀過兩端的表單文字）：工廠登記證、保險、檢驗報告、授權書、農產驗證等（`seller-app/lib/features/seller/presentation/kyc_wizard_steps.dart:377`、`website/features/seller-center/profile/certifications/CertificationsClient.tsx:1757`）。沒有規格要求附自然人證件；伺服器不限制上傳內容。
- 業界基線：理由必填加限時授權用在遮蔽的個資欄位；一般審核附件用後台角色權限控管。出處未核（推測）。同模組已有「有權限就能讀、每讀一張寫一列」的做法：`seller.evidence_accesses`（`schema.sql:856` 起）。
- 與已決事項的關係：ADR `2026-10-19-seller-kyc-payuni-consolidated.md` 的 2026-10-04 修訂規定證件影像要限時授權與理由。B、C 都要在步驟 9 修訂這一段，寫明「公司補充文件不是證件」。
- 選項：
  - A：保留整套，只服務公司文件（改名、拿掉三個全文欄位）。失敗條件：每次審核都要填理由才看得到審核必看的文件。
  - B（建議）：有後台的 KYC 讀取權限就能讀，`GetKYCApplication` 直接回文件的檢視網址與 `asset_id`；讀取紀錄改成 `evidence_accesses` 的形狀（誰、哪一張、何時），不再掛授權。刪 `RevealKYCSensitive`、`ListKYCReveals`、`kyc_reveals` 表、限時授權與 admin 的理由對話框。保留紀錄的理由：上傳內容不受限制，授權書等可能帶個人姓名與簽名。失敗條件：這份紀錄永遠沒有人查，變成只寫不讀的表。
  - C：連讀取紀錄也不留。失敗條件：有人質疑誰看過某份文件時沒有紀錄可查。
- 推翻建議的事實：決定補充文件要收自然人證件（那就用 A）。
- 可逆性：之後加回授權等於重做 A，成本一個子步驟。終態：B；觸發重評：補充文件的規格改成收個人證件。
- 限制：無。

### D37 D22 沒點名的七個個人欄位（確認後果）

- 欄位：本名 `real_name`、英文姓名、生日、身分證發證日期、類型、地點、銀行名稱。全部是 PayUni 註冊要送的欄位。
- 與已決事項的關係：D9 的修訂（平台不收身分與銀行資料，平台審核只管市集政策與統編）已經決定方向；兩端表單在 7b、7c 已不送這些欄位。這題要確認的是刪掉之後的三個後果。
- 後果：
  1. 後台審核佇列以本名當列名與搜尋條件（`seller/db/queries/kyc.sql:204,223`），個人申請改顯示店名並以店名搜尋。
  2. 證書持有人比對 `MatchCertHolder`（政府證書上的業者名稱是否屬於這個賣家）先比本名、再比公司名稱（`seller/internal/model/certholder.go:99`）。刪掉本名後，個人賣家沒有比對基準，公司賣家只剩公司名稱。這個函式目前在 seller 模組外沒有呼叫者（`git grep -l 'MatchCertHolder' -- backend/internal backend/cmd` 排除 seller 後 0 個非生成檔）；它要等商品模組搬進單體（B4）才接上，「沒有基準時怎麼辦」的路徑也還不存在。
  3. 後台的身分資訊卡整張消失。
- 選項：
  - A（建議）：七個全刪。在 B4 的需求加一條「個人賣家的證書持有人沒有比對基準，由 B4 決定轉人工或改用統編比對」。失敗條件：B4 需要個人賣家的證書自動比對，而政府查驗不回統編。
  - B：留本名當市集登記資料，把欄位加回兩端表單。失敗條件：平台收自然人姓名卻不驗證，比對基準是賣家自己填的字。
- 業界基線：身分由金流商驗證的市集不保存自然人身分資料（Stripe Connect 的託管開通，https://docs.stripe.com/connect/hosted-onboarding）。
- 推翻建議的事實：個人賣家的證書自動比對是上線必要功能。
- 可逆性：加回一個欄位與表單項目，成本小。
- 限制：無。

### D38 已不可能命中的舊身分雜湊

- 現況（讀過 `seller/db/queries/kyc.sql:262-292` 的關聯帳號查詢 `LinkedAccountHits`）：第一段比的是申請的雜湊表 `kyc_identity_hashes`，第二段只比銀行 fingerprint。步驟 8 刪掉申請的密文與這張表的三個種類後，防詐保存表 `account_identity_hashes` 裡身分證號、負責人身分證號、平台自收銀行帳號三個種類的列再也比不中，只會被複製進處分快照。
- 與已決事項的關係：計畫步驟 8 寫這些列「保留到 5 年清理」；D3（防詐身分雜湊與個資分開保存 5 年）與 ADR `2026-10-19` 的 2026-10-30 修訂是它的來源。選 A 要修訂這三處，寫明 5 年保存只剩統編與銀行 fingerprint。
- 業界基線：不保存不會再被使用的個資衍生資料（資料最小化，GDPR 第 5 條 1(c)；台灣個資法第 5 條的比例原則）。
- 選項：
  - A（建議）：刪這三個種類的列與所有對應的列舉值（保存表的 CHECK、處分快照的身分證種類、關聯類別的身分證與負責人兩個值、admin 標籤）。統編與銀行 fingerprint 不變。失敗條件：已有真實的身分證雜湊要保存；未上線，只有本機種子，所以不成立。
  - B：照字面保留列與列舉值。失敗條件：留下永遠不會命中的值，後台還會顯示對應的類別標籤。
  - C：保留列，並把比對改連保存表讓舊列仍能命中。失敗條件：為只存在於種子的資料新增機制。
- 推翻建議的事實：某個環境裡有真實申請留下的身分證雜湊。
- 可逆性：刪掉的雜湊無法重算（平台不再收身分證號）；沒有真實資料，所以不構成損失。
- 限制：無。

### D39 店家聯絡欄位的規則

- 現況：三條規則都是為 PayUni 註冊訂的：送出申請前必填（地址、聯絡人姓名、聯絡手機、客服信箱）、有 PayUni 註冊後不能清掉（`seller/internal/profile/profile.go:396`）、長度上限是 PayUni 的欄寬（聯絡人姓名 20 字、客服信箱 50 字，`seller/internal/model/validation.go:118`）。ADR `2026-10-19` 把這些寫成 PayUni 的要求；改規則要在步驟 9 修訂它。
- 刪掉 PayUni 之後誰讀這些欄位（`git grep -n -E 'ContactName|ContactMobile|SupportEmail|AddressDetail' -- backend/internal/legacy backend/internal/modules`，排除測試、生成物與 seller 自己的驗證）：

| 欄位 | 讀者 |
|---|---|
| 地址 | 回給 Java 的一般店家資訊與物流的店家資料（`internal/legacy/sellerserver/server.go:163,435`）；對外契約只有賣家自己與後台看得到（`git grep -n -i address_detail -- api/hawkr`） |
| 客服信箱 | 物流的店家資料（`server.go:438`） |
| 聯絡人姓名 | 只有後台顯示 |
| 聯絡手機 | 只有後台顯示 |

- 業界基線：必填欄位由實際使用它的流程決定；信箱長度上限 254（RFC 5321 第 4.5.3.1 節）。
- 選項：
  - A（建議）：地址與客服信箱維持送出前必填，申請在審核中或已核准時不能清掉（物流要用）。聯絡人姓名與聯絡手機改成選填、可以清掉，上限改成平台自己的（姓名 50 字；手機格式不變）。客服信箱上限改 254。兩端表單的必填標記與驗證常數跟著改。失敗條件：營運需要每個店家都有可撥打的手機；那就維持必填，理由改寫成營運聯絡。
  - B：四個欄位規則全部不變，只把理由從 PayUni 改成平台。失敗條件：兩個沒有流程在讀的欄位擋住送出，上限是別家公司的欄寬。
  - C：聯絡人姓名與手機整個刪掉。失敗條件：後台聯絡賣家只剩信箱。
- 推翻建議的事實：平台審核或客服流程規定要電話聯絡。
- 可逆性：必填放寬後要收緊，已送出的申請會有空值；未上線時沒有這個問題。
- 限制：無。

### D40 畫面文案與版面（依 repo 規則要先同意）

用語照術語表與現行文案：「金流商」（不用「金流服務商」）。下表的文字只有 zh-TW：website 的兩個副標目前是寫死在元件裡的中文，admin 只有 zh-TW。

| 位置 | 現在 | 建議 |
|---|---|---|
| website 與 seller-app 的客服信箱說明 | 「PayUni 會寄金流相關通知到這個信箱」 | 刪掉說明（金流通知寄到帳號的聯絡信箱） |
| website 認證類型選擇的副標 | 「需身分證 + 存摺」「需公司資訊 + 負責人身分證 + 公司帳戶存摺」 | 個人：刪副標；公司：「需公司資訊與統一編號」 |
| admin 核准鈕上方的小字與核准成功提示 | 「核准後系統會向 PayUni 註冊」 | 「核准後系統會建立收款帳戶，賣家再到金流商完成收款設定」 |
| admin 審核備註 | 「重點核對營業登記證與負責人身分證、統一編號是否一致，並確認所有證件皆在有效期內」 | 「重點核對營業登記資料與統一編號是否一致」 |
| admin「店家目前的聯絡資料」卡 | 屬於 PayUni 卡，刪卡後聯絡人姓名與手機沒有別處顯示 | 併進基本資料卡 |
| admin 商家清單的「待重新綁定」分頁 | 篩 PayUni 綁定失敗 | 刪；建立收款帳戶失敗的店家從審核待辦進來（既有） |
| admin 清單與詳情頁首的徽章 | 讀流程狀態 | 依 D35：讀申請狀態，沒有申請顯示「尚未申請」 |
| 賣家指南、FAQ、landing 的敘述 | 描述上傳身分證、銀行帳號、姓名鎖、PayUni | 只做事實修正：PayUni 改成「金流商」，刪掉已不存在的步驟；不重寫 |
| `seller-terms.md` 的示範聲明 | 「不收集…身分證件與銀行帳號；請勿輸入真實資料」 | 不改（仍然成立，改了要升條款版本） |

因 D17（店名與身分欄位鎖刪除）而消失的句子與恢復可編輯的欄位屬於跟隨決定，不另列。可逆性：文案隨時可改。限制：無。

### D41 驗收的允許清單補列

驗收指令 `git grep -n -i payuni -- backend/internal backend/cmd api/hawkr` 在現行允許清單之外還會剩下列幾類。建議列入允許清單：已套用的 15 個 migration 與 `atlas.sum`（不得修改）、新的 drop migration 本身、註解引用的 ADR 檔名 `2026-10-19-seller-kyc-payuni-consolidated`（23 檔 35 處）、seller 與 discovery 的 `CARRYOVER.md` 裡的沿用紀錄、凍結的 `platform/codemeta` 生成檔（gateway 與 seller-service 還在用）。三個測試資料檔裡的字樣改掉。未採：改 ADR 檔名（已接受的 ADR 不改名）。失敗條件：允許清單寬到遮住真的殘留；所以每一類都寫成路徑樣式，由 `check_inventory.py` 逐筆比對。可逆性：隨時可收緊。限制：已套用的 migration 不得修改（REAL，`backend/AGENTS.md` 單體規則 6）。

### D42 只剩統編之後的雜湊比對

- 現況（讀過 `seller/db/queries/kyc.sql:241-292`、`seller/internal/kyc/identity.go:15-80`、`seller/db/schema.sql:1040-1075`）：統編的雜湊（用金鑰算的 HMAC）寫進三處：申請列的欄位（只給草稿判斷「欄位有沒有改」）、申請的雜湊表 `kyc_identity_hashes`（隨 184 天個資清理刪除）、防詐保存表 `account_identity_hashes`（保存 5 年）。用到它們的地方：
  - 送出與核准時的比對 `match`：從申請的明文統編用每一把金鑰重算，再查處分快照。不讀這三處。
  - 建立處分時的快照：從保存表複製。
  - 處分的關聯帳號查詢 `LinkedAccountHits`：第一段連申請的雜湊表，第二段連保存表（只比銀行 fingerprint）。
- 步驟 8 之後申請的雜湊表只剩統編一種，而且同一批列在保存表都有。
- 業界基線：盲索引（blind index）用在加密欄位的查詢（CipherSweet，https://ciphersweet.paragonie.com/internals/blind-index）；同一份索引只存一處。
- 選項：
  - A（建議）：刪申請的雜湊表與申請列的統編雜湊欄位。關聯帳號查詢兩段併成一段，都連保存表；草稿的「有沒有改」直接比明文統編。金鑰環、換鑰重算與啟動檢查留著，服務保存表。行為差異：關聯帳號的統編命中從「申請清理前（184 天）」變成「保存期內（5 年）」，與 D3（防詐雜湊保存 5 年）的目的一致。失敗條件：換鑰重算目前從申請解密補算，改成只服務保存表後，統編的來源要改成申請的明文欄位；清理後的列無法補新金鑰（現況也是如此）。
  - B：兩張表都留，只是種類剩一個。失敗條件：同一批雜湊存兩處、查詢分兩段，各有自己的清理時機。
- 與 D38 選項 C 的差別：那裡是為了不會再產生的舊種類加一條比對路徑；這裡是把仍在用的統編比對從兩張表收成一張，少一張表與一段查詢。
- 推翻建議的事實：有流程需要「申請清理後統編就不再列為關聯」。
- 可逆性：加回一張表並從保存表回填，成本小。
- 限制：無。
- 狀態：未決（第 2 輪審查後重寫，等 Will）。

### D43 平台審核退回的通知事件（確認後果，不是新決定）

- 管理員退回申請時，單體經 `internal/legacy/sellerrelay` 發舊事件 `merchant.verification.rejected`（`relay.go:106,193,388`）給 Java 的通知服務；PayUni 的審核事件用同一組事件名。D18（舊事件停發，A8 搬通知模組時恢復）寫的是停發 `merchant.verification.*`，已經涵蓋它。
- 後果：A8 之前，賣家被退回時不會收到通知（Java 通知服務本來就沒有部署）。
- 處理：一起停發；seller-app 只刪這組事件專用的推播類型（`seller-app/lib/features/notification/data/push/seller_status_push.dart:9-17`，同組類型在 `notification_banner.dart:101-129` 與 `router.dart:35` 的對照一起刪），Java 仍會發的 `PAYUNI_STATUS` 留著。步驟 9 給 A8 的需求寫成兩條：收款開通結果的通知、平台審核結果（核准、退回）的通知。

### D44 物流用的商店代號（確認後果）

- 現況：`GetSellerShippingProfile` 回給 Java 物流服務的 `MerId` 與付款用的 `payuni_mer_id` 都由店家代號組出來（`internal/legacy/sellerserver/server.go:187,437`），不讀 PayUni 的表。計畫前提段只記了你接受「Java 以 PayUni 下單的本機端到端不通，B2 恢復」。
- 事實：新開通的店家沒有 PayUni 商店，PayUni 物流以這個代號建店與出貨，所以不管回什麼值，本機的物流端到端都不通；這是 D6（刪掉單體的 PayUni）的後果。
- 建議：兩處都回空值，並在計畫前提段補一句「本機的物流建店與出貨也不通，B3（物流搬遷）恢復」。未採：留著格式化函式回一個 PayUni 那邊不存在的代號（Java 會拿它去呼叫 PayUni 而失敗在更晚的地方）。
- Will 2026-11-12 確認。

### D45 「與受處分帳號共用銀行帳號」的提醒

- 現況：後台審核申請時，後端用申請的銀行帳號重算雜湊，查處分快照（`MeasureIdentityMatches`，`seller/db/queries/kyc.sql:254-261`，吃任意雜湊、方向是「帳號 → 處分」），回 `shares_bank_account_with_sanctioned_account`。申請不再有銀行帳號後它恆為 false；fingerprint 要到核准後建立收款帳戶、同步回來才有（D16：核准後才建立外部帳戶）。處分快照已經把 fingerprint 存成銀行帳號種類（`kyc.sql:246`）。
- 後果：審核當下不再有共用銀行帳號的提醒。新核准的店家之後出現的 fingerprint 若與受處分帳號相同，目前只有打開那筆處分的關聯帳號才看得到。D4（共用銀行帳號列入關聯帳號，只標記、不自動終止）的「標記」在這個方向沒有人會看到。
- 業界基線：金流商自己做重複帳戶與詐欺偵測；平台另做的比對是加值。出處未核（推測）。
- 選項：
  - A：刪欄位；主動提醒併入計畫已列的延後項「防人頭措施」（觸發：上線前）。失敗條件：上線前沒有補上，受處分的人換店重開、用同一個銀行帳號收款時沒有人被提醒。
  - B（建議）：刪申請上的欄位，並在 fingerprint 寫入的同一個 transaction（`seller/internal/kyc/fingerprints.go` 的 `RecordBankFingerprints`）用現成的查詢比對處分快照；命中生效中的處分就產生一筆審核待辦（新的待辦種類，只提醒、不擋）。新機制的必要條件：D4 要求共用銀行帳號被標記，而訊號的來源已從審核當下移到核准之後；查詢與快照都現成，缺的是呼叫點、待辦種類與 admin 的標籤。失敗條件：測試模式的銀行帳號只有少數幾個，本機示範時幾乎每個店家都命中。
- 推翻建議的事實：作品集不需要示範這個方向的提醒（那就選 A）。
- 可逆性：待辦種類之後可以刪；不影響其他刪除。
- 限制：無。
- 狀態：未決（第 2 輪審查指出現況寫錯，重寫後等 Will）。選 B 時另開子步驟 8d，在 8b 之後。

## 不交拍板、照預設處理的項目

每項都跟隨已決的決定，或有寫出的理由；有異議再提。

| 項目 | 處理 | 理由 |
|---|---|---|
| 刪掉的 proto 欄位與 enum 值 | 留 `reserved`（保留編號不再使用） | 三端生成物在分支內分批更新時，重用編號會讓舊的生成碼把新欄位解成舊欄位而不報錯；成本是一行 |
| 縮值域的 CHECK 遇到既有列 | 新 migration 先刪舊值的列再改 CHECK | 未上線；D11（不轉換舊的同意紀錄） |
| migration 刪掉的身分證與存摺文件列對應的媒體資產 | 不處理，重跑本機種子 | 只有本機種子資料；正常的退場路徑（`documents.go:254`）在 migration 裡叫不到 |
| 文件表 `kyc_documents` 的 `slot` 欄位 | 刪，主鍵改成 `(application_id, position)`；單張位置的合併邏輯一起刪 | 只剩公司文件一種，`slot` 是常數 |
| 個人申請的 `IndividualKYC` 變成空 message | 留著當型態分支，並在 seller 的 CARRYOVER 補一列理由 | 型態由設定哪一支表示，公司那一支仍有欄位；改成型態 enum 要動三端表單，沒有對應的需求 |
| 加密金鑰 `KYC_ENCRYPTION_KEYS` 與 `platform/seal` 的加密、解密、舊格式相容路徑 | 刪；只留算雜湊的那一半與 `KYC_HASH_KEYS` | 三個密文欄位是唯一用途（`rg 'seal\.[A-Z]\w+'` 的使用者只有 seller） |
| 後台動作「重新綁定」、PayUni 相關的不可用原因、可否重送的原因 enum | 刪 | D14（刪除管理員重新綁定）；最後一項依 D35 |
| 退回原因碼「無法辨識」「已過期」 | 留 | 公司文件仍是文件 |
| CodeMeta 的 `PayuniMerchantVerificationStatus` | 不動 | CodeMeta 凍結，gateway 與 seller-service 還在用 |
| 鎖順序檢查表 | 不用改 | `git grep -n -i payuni -- backend/internal/platform/lockorder` 0 筆 |
| seller-app `tool/coverage_baseline.json` 裡 6 個不存在的檔 | 重算基準時一併清掉 | 守門工具的輸入 |
| `seller-center.css` 沒有引用的 class | 只刪本步驟刪掉的元件用的 class | 其餘不是本步驟造成的 |
| website、seller-app 的內部設計文件 | 只修寫錯事實的句子 | 驗收不涵蓋 client 文件 |

## 子步驟（Will 2026-11-12 同意）

步驟 8 改到約 190 個檔、五個區域，拆成各有驗收的子步驟；每個子步驟結束時 CI 要綠（D21：步驟 4–8 在分支完成才進 `main`），契約的每一刀與它的所有消費端同一個子步驟。流程狀態與 PayUni 本體互相引用（流程狀態由 PayUni 註冊推導；PayUni 本體拿 `KYCFlowStatus` 當階段型別：`model/payuni.go:506`、`payuni_state.go:106`、`payuni/intent.go:250,326-346`、`store_action.go:223`、admin proto 的 PayUni 註冊 `flow_status`），兩個方向都切不開，所以併成一個子步驟。

| 子步驟 | 範圍 | 被擋於 |
|---|---|---|
| 8a 流程狀態、鎖與 PayUni 本體 | F 類與 P 類。F：`KYCFlow`、`KYCFlowStatus`、`identity_locked`、`CheckNameUnlocked`、重送規則（`CheckResubmit` 刪除後，送出只看申請狀態：草稿、已退回可送，已核准不可）；文案目錄；seller-app `MyKyc.fromApplication` 與 `kyc_flow.dart`、admin 徽章與清單欄位、website 的鎖。P：proto 的 PayUni RPC、message、enum；`platform/payuni`、`seller/internal/payuni`（送出前必填的檢查先搬到 model）、`payuni_*` 表與審核待辦函式（新 migration）；`PAYUNI_TERMS`（proto、CHECK、manifest、`generate-legal.ts` 同一個 commit）；`internal/legacy` 的事件與商店代號；`cmd/hawkr` 的設定；admin 的 PayUni 卡、重新綁定、待辦種類、清單分頁；聯絡欄位規則；D40 的文案 | D35、D39、D40、D43、D44（皆已決） |
| 8b KYC 個資管線 | I 類：proto 欄位與退回欄位的值域、密文與雜湊欄位（新 migration）、顯示全文與讀取紀錄、舊雜湊、雜湊比對的形狀、加密金鑰與 `platform/seal`、文件表的 `slot`、種子、admin 的身分卡與文件檢視、`TestKYCNeverReturnsOrStoresPlaintext` 改寫與 mutation | 8a、D36、D37、D38（已決）、D42、D45（未決） |
| 8c 去向與驗收 | `payuni-inventory.md` 的去向、`check_inventory.py` 與 mutation、允許清單逐筆核對、`backend/docs/第三方整合/` 的檔頭、CARRYOVER、賣家指南等事實修正、與 CI 基線比對；開 `payment-onboarding` → `main` 的 PR | 8b（D45 選 B 時另加 8d，排在 8c 之前）、D41（已決） |

8a 內部的 commit 順序照 `step8-inventory/backend.md` 第 4 節的第 1–7 步（先搬共用件、切斷呼叫、對外入口、legacy 與組裝、刪本體、SQL、`PAYUNI_TERMS`），8b 是第 8 步，文案與生成物跟著各自的子步驟。步驟 9 要修訂的 ADR 因本步驟增加：`2026-10-19-seller-kyc-payuni-consolidated.md` 的證件影像授權（D36）、身分比對與保存（D38、D42）、聯絡欄位（D39）。

## 審查帳本（開工時，2026-11-12）

兩輪獨立審查（第 1 輪：設計審查與決定簡報審查；第 2 輪只審修正）。第 2 輪找到第 1 輪修正造成的 blocking，依審查規則停下交 Will；Will 回覆後依下表處理，沒有再派第 3 輪。

| # | 嚴重度 | 項目 | 狀態（佐證） |
|---|---|---|---|
| L1 | blocking | D35 沒有申請時 admin 徽章的文字來源 | 已修（D35 選項 A） |
| L2、N1 | should、blocking | 子步驟順序：流程狀態與 PayUni 本體互相引用 | 已修：併成 8a（「子步驟」段列出引用處） |
| L3–L14、L16–L18 | should、nit | 前提題、D36–D39、預設表各列 | 已修 |
| L15 | should | D40、D41 缺基線與推翻事實 | 記錄：D40 是文案對照、D41 是驗收清單，沒有可比的業界做法；Will 已同意 |
| L19 | nit | 7b、7c、7e1、B2 等編號未在同句說明 | 記錄，不修 |
| N2 | should | D45 現況寫錯（已有「帳號 → 處分」的查詢） | 已修：D45 重寫（`kyc.sql:254-261`），未決 |
| N3 | should | D43 已被 D18 涵蓋 | 已修：改成確認後果 |
| N4 | should | D42 選項 A 的做法不可行 | 已修：D42 重寫（讀過 `identity.go:15-80`），未決 |
| N5 | nit | D39 讀者表的地址一格 | 已修 |
| N6 | nit | D43 的 seller-app 路徑 | 已修 |
| N7 | nit | 送出前必填的檢查先搬 | 已修：在 8a |

第 1 輪 findings 25 (blocking 1)：修 24、記錄 1。第 2 輪 findings 10 (blocking 1)：修 8、記錄 2。
