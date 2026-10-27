# 3d 設計審查（開工前）：step10e-3d-design.md v3

審查者沒有參與實作；只看設計。讀了：3d-design v3 全文、3d-queue-brief v3 全文、ADR 2026-09-24 文末「pub_*、D4 收窄」修訂段、`S/db/objects.sql`（effective_measures、pub_exposable_stores 的註解）、`S/db/pub.sql`、`S/db/queries/inquiries.sql:45-53`、`measures.sql:421-459`、`food_registrations.sql:102-115`、`model/report_case.go:340-356`、常數的使用點、`internal/arch/modules.go`、`H/adminauth` 的匯出函式。

---

[A] 3d-design E10、§3.1（`ReviewPolicy` 與 `pub_review_items` 的字面值兩份）；對照 queue brief §3 的 E2、E10 列與 ADR 2026-09-24 修訂段第 4 點
- 沿用或將就的是什麼：期限規則寫兩份——Go 的 `model.ReviewPolicy`（資源的 `*_deadline` 用）與 SQL 函式裡的時長字面值（待辦用），靠資料集測試鎖住一致。成員條件也是兩份（Go 判定式＋SQL 分支）。先例引的是 `effective_measures` 與 `model.StateOf`。
- 必要條件，以及今天是否成立：ADR 修訂段第 4 點寫「`pub_*` 必須算現在的狀態……規則只有一個家，擁有者的 Go 程式讀同一份 SQL」［量測：ADR 修訂段原文］；Will 核准的 queue brief v3 §3 寫「E2：資源的期限由 seller 讀同一個 `pub_review_items`（以對象篩選），不另寫一份規則」、「E10：期限的時長只寫在 `pub_review_items` 裡（規則一個家）」，§4 的退路也是「改成每種一個 `pub_*` 函式」，仍是一個家［量測］。設計 v3 改成兩份，理由只有「`pub_*` 只收時間參數」，但 ADR 原文是「帶時間參數的 `BEGIN ATOMIC` 函式」，沒有寫只收時間參數，而且這個理由也擋不了「Go 讀 SQL」［量測：ADR 文字；推論：理由不成立］。引的先例不對應：`objects.sql` 的註解說 `StateOf` 是「預定生效」的另一個寫法，才用資料集測試鎖住；曝光的寫法是「Go 只讀這裡、不重算」（`store_requirement_flags`、`exposure.Classify` 讀 `pub_exposable_stores`）［量測］。期限跟曝光一樣是純讀取的推導，對應的先例應該是曝光。
- 從零設計的做法：規則放在一個地方，其他地方都讀它（DRY/single source of truth）。這個 repo 自己的例子：曝光規則放在 SQL，Go 讀出來用。具體做法是 seller 內部的每種期限一支 SQL 函式或 view（不是 `pub_`），`pub_review_items` 跟資源的清單／詳情查詢都 join 它，Go 只把查出來的 start/due 轉成 proto。Go 判定式要留的只有指令守門那些（`AwaitingDecision` 等，判斷記憶體裡已載入的實體），留下來就沿用 StateOf 的「兩種寫法＋資料集測試」，並在 CARRYOVER 記一列。這是推論，沒有外部來源；依據是本 repo 的 ADR。
- 建議：改（或交給使用者決定）。v3 違反了 Will 已核准的 brief，卻沒有列成決定題，所以至少要列出來給 Will。照 brief 改的話，`ReviewPolicy` 的 Go 型別、`Service` 持有的政策、§6.2 的「期限 ⇔ 政策」與「資源輸出 ⇔ 待辦」都可以刪掉。
- 信心：高

[B] 3d-design §3.1 第 2 點（Service 持有政策、測試注入另一份）、§3.1 型別註解、§6.2「期限 ⇔ 政策」與它的 mutation
- 沿用或將就的是什麼：v2 的「政策是可注入的值型別，測試用互不相同的時長」（附錄 A 的 MAJOR 4）照搬到 v3。但 v3 的 SQL 是字面值，不收參數。
- 必要條件，以及今天是否成立：這個機制只在 SQL 也讀同一份可注入政策時才有意義。v3 E10 寫 SQL 是「字面值」；§3.1 的型別註解卻寫「SQL 的待辦查詢以參數取得同一份政策的值」；§6.2 寫「注入一份每個時長都不同的政策……SQL 的任兩個時長參數對調」［量測：三處原文互相矛盾］。SQL 是字面值時，注入一份不同的 Go 政策，比對必然失敗；「時長參數對調」這個 mutation 也不存在［推論］。所以 `Service` 持有政策＋測試注入這組機制在 v3 沒有可用的地方，是 v2 合併層設計留下來的。
- 從零設計的做法：照上一條改成一個家，這組機制整個不需要；如果 Will 選擇保留兩份，可注入的政策也沒有用，因為測試只能拿 `DefaultReviewPolicy()` 跟 SQL 字面值逐種比對。
- 建議：改。刪掉注入、把三處文字統一；mutation 改寫成「改 SQL 某一個分支的字面值，或改 Go 的某一個時長，測試要失敗」（E10 已經這樣寫）。
- 信心：高

[B] 3d-design §1.3 第 1 點（`AdminMeasuresReviewDue` 跟著改）、§4.2 #124（食品業者登錄清單）、§6.2「複審到期分頁與待辦一致」
- 沿用或將就的是什麼：既有的收件匣查詢各自保留一份成員條件，跟 `pub_review_items` 平行存在。複審到期的規則會有三份：Go `Measure.ReviewDue`、`AdminMeasuresReviewDue` 的 SQL（用 Go 算好的 `active_ids`）、`pub_review_items` 的分支（用 `effective_measures`）。食品業者登錄的「可以裁定」也有三份：`AdminFoodRegistrationInbox` 的 `awaiting_kyc`、`CanApprove()`、函式分支。兩份 SQL 之間靠新加的資料集測試鎖住。
- 必要條件，以及今天是否成立：收件匣要的是另外的篩選（店家、依據……）跟各自的排序，這點成立；但成員條件不一定要再寫一份。`AdminMeasuresReviewDue` 用 `review_by <= now` 排序，提前 24h 之後，它還要跟待辦對齊同一個開始時間［量測：`measures.sql:421-459`］。食品業者登錄清單依 `submitted_at` 排序，但新加的「期限」欄是從 `max(送出, KYC 核准)` 算的，所以清單順序跟它顯示的期限不一致［量測：`food_registrations.sql:102-115`；推論：順序不一致］。
- 從零設計的做法：收件匣以 `kind` 為條件讀同一個定義（seller 內部函式），再套自己的篩選，不另寫成員條件（同上一條的單一來源）。推論。
- 建議：記進盤點表，或併進第一條一起改。至少要寫明為什麼收件匣保留自己的一份；食品業者登錄清單要不要改成依期限排序，交給使用者決定（這是 UI 排序的改動）。
- 信心：中

[A] 3d-design E3、§2.1 `has_due`、§2.2 的 `coalesce(due_at, start_at)` 與 `NOT has_due`、§1.3 選填的 `deadline`、§6.3「沒有期限的列」用臨時 SQL 分支測
- 沿用或將就的是什麼：v2 為了「加來源不改合併層」而保留的「有無期限」旗標，照搬到 (g)。
- 必要條件，以及今天是否成立：v2 附錄 A 的 MAJOR 1 說，保留旗標是因為它跟「加來源不改合併層」衝突。queue brief v3 §1.3 自己把這個約束判成 SELF-IMPOSED，並寫明「(g) 下加來源是加一個分支，沒有合併層」［量測］。所以在 (g) 下，A7-1e 本來就要改 `reviewqueue` 的 SQL 跟 pub 型別，到時候加一欄（或讓 `due_at` 可以是 NULL）的成本跟現在一樣。現在的 11 種都有期限，這一欄是常數 true；為了它，排序鍵、游標、proto 的選填、測試替身都要多一層，A7 的種類有沒有期限也還沒有定義［量測：E3 原文］。
- 從零設計的做法：YAGNI，等第一個沒有期限的種類出現時（A7-1e），在同一次變更裡加上。推論。
- 建議：交給使用者決定（D′1 的不變式是已核准的）。建議刪掉，觸發條件寫成「A7-1e 有沒有期限的種類」。
- 信心：中

[C] 3d-design §9 第 3 點、§3.2（`adminauth` 有沒有讓 handler 讀呼叫者權限的 API）
- 沿用或將就的是什麼：權限交集（E13）依賴一支還沒確認存在的 API。
- 必要條件，以及今天是否成立：`H/adminauth` 匯出的只有 `GuardFile` 跟宣告表的 `requires`，沒有可以在 handler 裡查權限的函式［量測：`rg -n "func " adminauth/*.go`，只查了 Allowed／Perm 相關的名字］。所以「加一支 `adminauth.Allowed`」幾乎一定會發生，而且它是一個新機制（手寫的種類 → 權限表，跟 interceptor 共用角色表）。
- 從零設計的做法：現在就在設計裡定下 `Allowed(ctx, perm)` 的形狀跟資料來源，不要留到實作時。
- 建議：改（把「實作時查」換成定案）。
- 信心：中

---

## 檢查過的機制

- 讀法 (g)：`pub_review_items` 加 `reviewqueue` 一支 SQL。符合 ADR 第 4 點跟 D4 (a)，判定合理。
- 不另存，另存 SLA 紀錄是終局（E12）：觸發條件寫明了（報表、第二位營運人員、p95），判定合理。
- 新模組 `reviewqueue`（沒有表）：A7-1e 已經排定會加 moderation 的分支，而且 httpapi 不能放 SQL，判定合理。
- 11 個 `UNION ALL` 分支寫在一個函式裡：待辦本來就要掃全部開著的項目；常數的 `kind = ANY` 可以推進各分支（推論），§6.5 會量，判定合理。如果採第一條的建議，詳情頁不用再走這個函式，「以對象篩選推不進去」的疑慮也就沒了。
- 全序 `(NOT has_due, due, start, kind C, item_id C)` 加 row-value keyset：跟業界的 keyset 相同。`has_due` 這一欄見第四條。
- `item_id TEXT`（UUID 的 text；來函對象是 `order_id/lpad(position)`）：為了位元組順序穩定的 tiebreak，避開 NULL 的 row-value 比較，判定合理。
- 型別化的 subject 欄位（稀疏的複合型別）加 proto `oneof`：判定合理。
- 一支語句就是一個快照，不用 `InReadSnapshot`（E8）：判定合理。
- 每個請求一個 `now`、時間截到 µs：判定合理。
- page token 用 `rpc.FilterScope` 綁請求的 kinds（排序、去重）；page_size 預設 50、上限 100；用 N＋1 判斷有沒有下一頁：沿用 API 規範，判定合理。
- 第一頁的游標用最小值，不寫 `IS NULL OR`：判定合理。
- E9 翻頁時 key 會變：接受，有終局跟觸發條件，判定合理。
- 權限細到種類加上交集（E13）：照已核准的 D′1，判定合理。API 沒有定案的部分見第五條。
- 錯誤模型：不新增 `ErrorReason`、用 `INVALID_ARGUMENT`；沒有任何種類的權限時回空清單，不回錯誤。判定合理。
- 共用的 `Deadline{start,due}` 訊息、放在 common、改名去掉 `_time`（E2）：上線前沒有相容性約束，判定合理。JSM 的 `startTime` 沒有重新查證，但不影響這個選擇。
- `Deadline` 只在義務還在時才設，告知跟詢問逾期不輸出在資源上：有寫出必要條件，判定合理。
- `ReviewPolicy` 用值型別跟欄位、不用 map，`Decide` 遇到未知嚴重度就 panic：如果保留兩份，判定合理；但整個型別是否必要，見第一條。
- `InquiryDefaultDue`、`DefaultAuthorityDue` 不搬：它們不是營運的審閱政策，判定合理。
- 刪掉 `FoodSafetyTriageSLA` 等常數：判定合理，不過要留意 `seller/governance.go:266`、`:293` 的公開重匯出也要一起刪。
- 成員條件的 Go 判定式跟 SQL 分支兩份加資料集測試：指令守門需要 Go 判定式，可以接受，但要記進盤點表（併在第一條）。
- 開始時間 `CASE_DECISION` 跟既有的 `CaseDecisionClockStarts` 加 `DecisionClockStart` 是同一個規則的第三份，併在第一條。
- E11 食品業者登錄只收「可以裁定」的、開始時間是 `max(送出, KYC 核准)`：照 3b D8，判定合理。排序見第三條。
- E16 詢問逾期是營運的期限，不是賣家的：判定合理。
- 索引：只幫篩選、先量再加（§6.5）：判定合理。
- 手寫 migration、`objects.sql` 加 `pub.sql`、契約測試：照 ADR 修訂段，判定合理。
- 種類標籤用 admin 單端的文字表：照 3cd §2 第 4 項，判定合理。
- 30 秒自動更新、`keyset-pages`、`DeadlineText`：重用既有的，判定合理。
- 發票申訴不在範圍：invoice-service 是 REAL 的邊界，判定合理。
- 範圍外但有觸發條件的：通知、指派、校準，判定合理。
