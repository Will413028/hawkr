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

## 要拍板的題目（D35–D45）

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
| 地址 | 物流的店家資料（`internal/legacy/sellerserver/server.go:163,435`） |
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

- 現況：識別欄位原本只存密文，要比對就得另存用金鑰算的雜湊（HMAC）。每個識別的雜湊存三處：申請列的欄位、申請的雜湊表 `kyc_identity_hashes`、防詐保存表 `account_identity_hashes`。步驟 8 之後平台自己收的識別只剩統編，而統編在申請列是明文（`seller/db/schema.sql:946-948`），後台也不遮蔽。銀行 fingerprint 的雜湊只存在保存表，仍然需要金鑰。這一段的 schema 是審查者讀的，比對程式我只逐行讀過 `LinkedAccountHits`。
- 業界基線：盲索引（blind index）用在加密欄位的查詢（CipherSweet 文件，https://ciphersweet.paragonie.com/internals/blind-index）；明文欄位直接等值比對。清理個資後仍要比對的，才需要另存雜湊。
- 選項：
  - A（建議）：統編的雜湊只留保存表一份（清理申請之後仍要比對 5 年）。刪掉申請列的統編雜湊欄位與 `kyc_identity_hashes` 整張表；還沒清理的申請之間用明文統編比對。金鑰環、換鑰重算與啟動檢查留著，服務保存表。失敗條件：比對查詢改寫後，處分快照與保存表之間有一段對不上；8c 開工時要先逐行讀完比對路徑再定案。
  - B：三處都留，只是種類剩一個。失敗條件：同一個公開識別碼的雜湊存三處，一張表只剩一種值。
- 推翻建議的事實：有查詢需要在申請還沒清理時以雜湊比對（例如換鑰期間）。
- 可逆性：加回一張表與回填，未上線時成本小。
- 限制：無。

### D43 平台審核退回的通知事件

- 現況：管理員退回申請時，單體經 `internal/legacy/sellerrelay` 發舊事件 `merchant.verification.rejected`（`relay.go:106,193,388`）給 Java 的通知服務；PayUni 的審核事件用同一組事件名。D18（舊事件停發，A8 恢復通知）的內文講的是 PayUni 審核通知，沒有單獨點名平台退回。seller-app 有一組對應的推播類型（`seller-app/lib/features/seller/data/seller_status_push.dart:9-17`）。
- 選項：
  - A（建議）：一起停發，seller-app 對應的推播類型也刪；步驟 9 給 A8 的需求寫成兩條：收款開通結果的通知、平台審核結果（核准、退回）的通知。失敗條件：A8 之前需要在本機示範退回通知。
  - B：退回事件留到 A8，只停 PayUni 的。失敗條件：為沒有部署的 Java 通知服務留一條舊事件與它的接縫。
- 業界基線：通知由通知模組訂閱領域事件產生，不由各模組轉發舊格式（單體計畫 A8 的方向）。
- 推翻建議的事實：A8 排在上線之後。
- 可逆性：A8 重做，不沿用舊事件。
- 限制：無（保留到 A8 是 SELF-IMPOSED 的排程理由）。

### D44 物流用的商店代號（確認後果）

- 現況：`GetSellerShippingProfile` 回給 Java 物流服務的 `MerId` 與付款用的 `payuni_mer_id` 都由店家代號組出來（`internal/legacy/sellerserver/server.go:187,437`），不讀 PayUni 的表。計畫前提段只記了你接受「Java 以 PayUni 下單的本機端到端不通，B2 恢復」。
- 事實：新開通的店家沒有 PayUni 商店，PayUni 物流以這個代號建店與出貨，所以不管回什麼值，本機的物流端到端都不通；這是 D6（刪掉單體的 PayUni）的後果。
- 建議：兩處都回空值，並在計畫前提段補一句「本機的物流建店與出貨也不通，B3（物流搬遷）恢復」。未採：留著格式化函式回一個 PayUni 那邊不存在的代號（Java 會拿它去呼叫 PayUni 而失敗在更晚的地方）。
- 需要你確認的只有這個後果。

### D45 申請上的「與受處分帳號共用銀行帳號」旗標

- 現況：後台審核申請時，後端用申請的銀行帳號重算雜湊，比對受處分帳號，回 `shares_bank_account_with_sanctioned_account`。申請不再有銀行帳號後它恆為 false；fingerprint 要到核准後建立收款帳戶才有（D16：核准後才建立外部帳戶）。現成的 fingerprint 比對方向是「處分 → 關聯帳號」（`kyc.sql:282-289`），不是「帳號 → 處分」。
- 後果：審核當下不再有共用銀行帳號的警示；新核准的店家之後才出現的 fingerprint 若與受處分帳號相同，只有在看那筆處分的關聯帳號時才看得到，沒有主動提醒。
- 選項：
  - A（建議）：刪欄位；主動提醒併入計畫已列的延後項「裝置與聯絡方式重疊、風險評分等防人頭措施」（觸發：上線前）。失敗條件：上線前沒有補上，人頭帳號換店重開不會被提醒。
  - B：本步驟新增「fingerprint 同步時比對受處分帳號並產生審核待辦」。失敗條件：在刪除步驟裡新增一個機制，範圍擴大。
- 業界基線：金流商自己做重複帳戶與詐欺偵測（Stripe Radar 與 Connect 的風險審查）；平台另做的是加值。出處未核（推測）。
- 推翻建議的事實：作品集要示範主動的防人頭提醒。
- 可逆性：之後加審核待辦是新增功能，不受這次刪除影響。
- 限制：無。

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

## 子步驟

步驟 8 改到約 190 個檔、五個區域，拆成各有驗收的子步驟；每個子步驟結束時 CI 要綠（D21：步驟 4–8 在分支完成才進 `main`），契約的每一刀與它的所有消費端同一個子步驟。順序由消費端往來源刪：流程狀態是從 PayUni 註冊推導的，所以先刪流程狀態，再刪 PayUni 本體。

| 子步驟 | 範圍 | 被擋於 |
|---|---|---|
| 8a 流程狀態與鎖 | F 類：`KYCFlow`、`KYCFlowStatus`、`identity_locked`、`CheckNameUnlocked`、重送規則；文案目錄；seller-app `MyKyc.fromApplication` 與 `kyc_flow.dart`、admin 徽章與清單欄位、website 的鎖與相關文案 | D35、D40 |
| 8b PayUni 本體與接縫 | P 類：proto 的 PayUni RPC、message、enum；`platform/payuni`、`seller/internal/payuni`（送出前必填的檢查先搬到 model）、`payuni_*` 表與審核待辦函式（新 migration）；`PAYUNI_TERMS`（proto、CHECK、manifest、`generate-legal.ts` 同一個 commit）；`internal/legacy` 的事件與商店代號；`cmd/hawkr` 的設定；admin 的 PayUni 卡、重新綁定、待辦種類、清單分頁；聯絡欄位規則 | 8a、D39、D40、D43、D44 |
| 8c KYC 個資管線 | I 類：proto 欄位與退回欄位的值域、密文與雜湊欄位（新 migration）、顯示全文與讀取紀錄、舊雜湊、雜湊比對的形狀、加密金鑰與 `platform/seal`、種子、admin 的身分卡與文件檢視、`TestKYCNeverReturnsOrStoresPlaintext` 改寫與 mutation | 8b、D36、D37、D38、D42、D45 |
| 8d 去向與驗收 | `payuni-inventory.md` 的去向、`check_inventory.py` 與 mutation、允許清單逐筆核對、`backend/docs/第三方整合/` 的檔頭、CARRYOVER、賣家指南等事實修正、與 CI 基線比對；開 `payment-onboarding` → `main` 的 PR | 8c、D41 |

backend 內的 commit 順序見 `step8-inventory/backend.md` 第 4 節。步驟 9 要修訂的 ADR 因本步驟增加：`2026-10-19-seller-kyc-payuni-consolidated.md` 的證件影像授權（D36）、身分比對與保存（D38、D42）、聯絡欄位（D39）。

## 審查帳本（開工時，2026-11-12）

兩輪獨立審查（第 1 輪：設計審查與決定簡報審查；第 2 輪只審修正）。第 2 輪找到第 1 輪修正造成的 blocking，依審查規則停止修正，以下未處理的項目等 Will 回覆後再改。

| # | 嚴重度 | 項目 | 狀態 |
|---|---|---|---|
| L1 | blocking | D35 沒有申請時 admin 徽章的文字來源 | 已修 |
| L2 | should | 子步驟順序（流程狀態由 PayUni 註冊推導） | 未解，見 N1 |
| L3–L14、L16–L18 | should、nit | 前提題、D36–D39、D42–D45、預設表各列 | 已修 |
| L15 | should | D40、D41 缺基線、失敗條件、推翻事實 | 部分未解 |
| L19 | nit | 7b、7c、7e1、B2 等編號未在同句說明 | 未解 |
| N1 | blocking | 8a 刪不掉 `KYCFlowStatus`：PayUni 本體拿它當階段型別（`model/payuni.go:506`、`payuni_state.go:106`、`payuni/intent.go:250,326-346`、`store_action.go:223`、admin proto 的 PayUni 註冊 `flow_status`、`store-row.ts:62`）；`CheckResubmit` 拿掉後送出檢查由誰做沒寫 | 未解：8a、8b 在 backend 與 proto 要併成一刀，或把這些符號列進 8a |
| N2 | should | D45 現況寫錯：`MeasureIdentityMatches`（`kyc.sql:254-261`）已有「帳號 → 處分」方向，選項 B 只缺 fingerprint 同步時的呼叫點與待辦 | 未解 |
| N3 | should | D43 不是新決定：D18 已涵蓋同一個路由鍵；缺口只有給 A8 的需求少了平台審核結果 | 未解 |
| N4 | should | D42 選項 A「申請之間用明文統編比對」做不到：處分快照只有雜湊，要改連保存表的統編列，與 D38 選項 C 不採的理由相同 | 未解 |
| N5 | nit | D39 讀者表：`server.go:163` 是一般店家資訊不是物流；買家是否看得到地址未查 | 未解 |
| N6 | nit | D43 的 seller-app 路徑應為 `lib/features/notification/data/push/seller_status_push.dart`，同組類型另在 `notification_banner.dart`、`router.dart` | 未解 |
| N7 | nit | 送出前必填的檢查應在 8a 先搬（`payuni/storedata.go:57`） | 未解 |

第 1 輪 findings 25 (blocking 1)：修 24、記錄 1（L19）。第 2 輪 findings 10 (blocking 1)：全部未處理，交 Will。
