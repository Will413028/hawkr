# A7-1d 決定簡報（Will 2026-10-29 已決）

證據等級：〔量測〕＝以指令或讀檔確認；〔推論〕＝由程式與文件推得；〔推測〕＝業界做法憑公開資料記憶，未逐條查證原文。路徑相對 `backend/`。本檔只留結論與約束帳；選項比較的原稿與設計審查在 session scratchpad，不入庫。結論寫進 `../2026-10-18-a7-social.md` 的 A7-D14 到 A7-D17、A7-D13 修訂與 A7-1d／1d′／1d″。

## 0. 前提題：要改的程式之後會不會被重寫、搬走或刪掉

| 會被改到的程式 | 之後的命運 | REAL / SELF-IMPOSED |
|---|---|---|
| `internal/legacy/accountservice/server.go:142`（舊 gRPC `SuspendAccount` 轉呼叫 identity） | A7-5 刪除；單體內 `SuspendAccount` 的非測試呼叫者只有這 1 處〔量測：`rg -n '\.SuspendAccount\(' backend -g '*.go'`〕 | SELF-IMPOSED（搬遷排程） |
| `identity.SuspendAccount`（`internal/modules/identity/identity.go:217-219`、`internal/account/account.go:195-221`） | 不刪；A7-1d′ 改它 | SELF-IMPOSED |
| `identity/internal/auth/auth.go:198-206` `accountStatus`（停權即 `PermissionDenied`） | 無排程改寫；A7-1d″ 改它 | SELF-IMPOSED |
| `seller.cases`／`seller.reports`（`internal/modules/seller/db/schema.sql:160-240`） | 不動；moderation 是新模組 | SELF-IMPOSED |

沒有「因為之後要刪所以現在將就」的理由。系統未上線，凡「現有程式長這樣」「計畫這樣寫」都是 SELF-IMPOSED。

## 決定 1（A7-D14）：案件粒度與處置冪等

- 結論：每個對象 `(target_kind, target_id)` 至多一個進行中案件（partial UNIQUE `WHERE closed_at IS NULL`）；同對象的檢舉掛進該案；結案把決定寫進各檢舉的 outcome；不依類別分案。處置的冪等＝案件 open → closed 的狀態轉移加 `version`。A7-1e 待辦一列一案。
- 業界基準〔推測〕：Meta、YouTube、Reddit 依內容聚合成一個審查單位；DSA Art.16/17〔推論〕：決定針對內容，回覆針對每份檢舉。
- 來由：A7-D7 第 9、10 項以「檢舉」為單位，是照搬舊 social-service 每筆 report 自帶 `status`（舊 `schema.sql:95-101`）；seller 已有同形（`seller/db/schema.sql:160-215`，`cases_one_open`）；`decision-brief.md:218` 已決與 seller 對齊。
- 不分類別的理由〔推論〕：單則內容只會有一個處置決定，分案會出現互相矛盾的結案；seller 分類別是因為店家層級的類別是不同調查。
- 約束帳：計畫第 9、10 項字句、舊 report 形狀皆 SELF-IMPOSED；無 REAL。
- 延後：依檢舉數或觸及量提高優先度。觸發：上線後單日檢舉量讓 72h 期限逾期，或出現病毒式散播的違規內容。

## 決定 2（A7-D15）：停權的單一入口與「有店不能停權」

- 結論：新編排模組 `accountsuspension`（與 `accountdeletion` 同層）是停權與解除停權的唯一入口；「擁有商店的帳號不能被停權」只在這裡，錯誤原因 `ACCOUNT_HAS_STORE`；identity 的停權入口只被它呼叫。
- 業界基準〔推測〕：Etsy、eBay、Shopee 的帳號停權連帶店鋪；不變式放在擁有狀態的服務，跨聚合前置條件放在編排層〔推論〕。
- 來由：`identity` 是葉節點（`internal/arch/modules.go:15`），不能 import seller 查有沒有店〔量測〕；前例是 `accountdeletion`（`modules.go:36`）。動機〔推論〕：店家帳號被停權會讓店無人可登入，而 seller 的措施、曝光與未結訂單都不知道。
- 約束帳：identity 不依賴 seller＝SELF-IMPOSED（正確分層，編排者滿足它）；帳號停權對店鋪、未結訂單、撥款的連帶規則未定＝**REAL（缺資料）**，來源是賣家條款與營運政策；計畫把規則寫在 moderation＝SELF-IMPOSED。
- 終態與觸發：賣家條款定出帳號停權對店／訂單／撥款的效果後，`accountsuspension` 連帶對店建立措施，取代「有店就拒絕」。

## 決定 3（A7-D16）：只有永久停權與手動解除

- 結論：A7 的停權紀錄與 social 的處分紀錄都沒有 `ends_at`；停權型別只有永久。
- 業界基準〔推測〕：X、Reddit、Discord 有暫時與永久兩種；到期以讀時判斷（`ends_at IS NULL OR ends_at > now()`）加排程 job 處理副作用〔推論〕。
- 來由：計畫原本加「可為空的 `ends_at`」是為階梯式停權預留，A7 內沒有任何入口建立定期停權，欄位沒有到期機制就語意不成立。
- 約束帳：階梯參數未定＝**REAL（缺資料，買家條款）**；「先預留欄位」＝SELF-IMPOSED。
- 終態與觸發：買家條款定出處罰階梯時，停權紀錄加 `ends_at`，認證讀時判斷到期，排程 job 寫到期解除紀錄並發事件。因為 A7-D17 不停用 Clerk 使用者，不需要重新啟用 Clerk。

## 決定 4（A7-D17）：受限登入

- 結論：停權不再停用 Clerk 使用者；被停權帳號可登入，伺服器只放行允許清單（讀自己的停權、讀自己的處分、登出、刪除帳號），其餘回 `ACCOUNT_SUSPENDED`；identity 的認證結果帶停權狀態，不再回錯誤；允許清單在 `rpc` 層。停權時發通知（A8 消費）。
- 業界基準〔推測〕：X、Reddit 停權帳號可登入、可瀏覽、不可發文；Google 停用帳號顯示通知與申訴入口。
- 來由：現況停權即 `PermissionDenied` 且 Clerk ban（`auth.go:198-206`、`account.go:195-217`），被停權者讀不到理由，與 A7-D9「作者讀得到理由」矛盾〔推論〕。
- 約束帳：現有 auth 與 Clerk ban＝SELF-IMPOSED；DSA Art.17／20 對台灣市場非 REAL，僅作參考（進入 EU 市場時變 REAL，需對照停權紀錄欄位）；申訴流程未設計＝SELF-IMPOSED，允許清單先不含申訴。
- 終態與觸發：申訴加進允許清單，觸發是申訴流程的產品決定。

## A7-D13 修訂：檢舉原因對映

| 舊值 | 對映 |
|---|---|
| SPAM、HARASSMENT、FRAUD、OTHER | 直接對映 |
| SEXUAL | 新增 `PolicyCategory.SEXUAL_CONTENT`：社群類別，檢舉人可選、可認定 |
| POLITICAL | `OTHER`（不設專屬類別） |
| FAKE_NEWS | `MISLEADING_INFO`（對社群檢舉入口開放） |
| FOOD_SAFETY | 保留在社群檢舉人可選子集 |

## 設計審查的修正（寫進 A7-1d）

- 去重的唯一鍵只比對還沒有 outcome 的檢舉（所屬案件仍開著）；每日上限舊值 20（`reportRateLimitPerDay`，舊 `usecase/report.go:35`）記進盤點表，並發安全靠同 transaction 內的計數列原子 upsert，不用先 count 再比較。
- 檢舉人與買家對象一律存 `buyer_id`，不沿用舊 reporter 的 buyer／store／account fallback；轉成 `account_id` 只在 moderation 的 BAN 分支呼叫 `accountsuspension` 之前做一次。
- 買家分支（social `Hide`／`Warn`、`accountsuspension`）與店家分支一樣在 moderation 的同一個 transaction 內，驗收有回滾的 integration 測試。

## 連帶影響

A7-1e 一案一列；A7-2 新增 `ErrorReason` `ACCOUNT_SUSPENDED`、`ACCOUNT_HAS_STORE`，停權讀取 API 在允許清單內；A8 通知：每筆檢舉的 outcome 各自通知檢舉人，帳號停權通知帶理由；seller 現在不動；legacy accountservice 改呼叫 `accountsuspension`（A7-5 刪）。
