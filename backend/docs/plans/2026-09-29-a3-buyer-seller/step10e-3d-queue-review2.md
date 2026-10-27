# 第二輪獨立審查：step10e-3d-queue-brief.md v2（HEAD 4cc2ec079，唯讀）

路徑縮寫：W=<10e worktree>；S=W/backend/internal/modules/seller；I=S/internal；P=W/backend/docs/plans/2026-09-29-a3-buyer-seller；A7=W/backend/docs/plans/2026-10-18-a7-social。
標記：[量測]＝我讀到該行或跑了指令；[推論]＝由讀到的內容推得；[未查證]＝一般知識，沒有查。

## 一、問題表

| 嚴重度 | 位置 | 問題 | 證據 | 建議修法 |
|---|---|---|---|---|
| BLOCKER | §1 選項表、§1.5 | **漏了本專案現行、已核准的機制：`pub_*` 組合。** ADR 2026-09-24 的 D3 修訂第 4 點：要用下層模組的屬性篩選或排序，就 join 下層擁有者發布的 `<owner>.pub_*`（帶時間參數的 SQL 函式，時間由請求傳入）；D4 (a) 收窄又明寫「能在讀取時推導的可見性，改為 join `pub_*`」。套到待辦：seller 發布 `seller.pub_review_items(at)`，佇列模組（依賴 seller，A7-1e 時再依賴 moderation）用一支 SQL `UNION ALL` 各來源的 `pub_*`，以 `(due_at, …, id)` 做 keyset。這樣**沒有合併層、哨兵、`SeekFor`**（(e) 對 (a) 的主要優點），寫入路徑一行都不用改，A7-1e 只是多加一個 `UNION ALL` 分支，沒有要丟掉的程式。機制已經存在（`seller.pub_exposable_stores(at)`）。少了這個選項，Q1 是在 (a) 的合併層和 (e) 的 35 個呼叫點之間二選一，最便宜而且合乎現行規則的選項沒有被列出來 | [量測] W/backend/docs/adr/2026-09-24-go-modular-monolith.md:312-316（D3 修訂第 3–5 點）、D4 (a)「能在讀取時推導的可見性，改為 join `pub_*`」；S/db/pub.sql:19 `seller.pub_exposable_stores(at TIMESTAMPTZ)`；A7/2026-10-18-a7-social.md:545（原 A 方案：`reviewqueue`「依賴 seller 與 moderation」） | 加選項 (g)「`pub_*` 組合」，寫出它的業界對照（單一系統裡佇列就是對實體做查詢）、約束帳和終局（ADR 第 5 點：p95 超過門檻、有第一份達成率報表，或有第二位營運人員時才實體化）。重新比較 (e) 和 (g) |
| MAJOR | §1.5「對 (a′)：…這正是『之後再做』的債」、§0 | 把「延後決定」說成技術債，不成立。依必要條件原則：中央表的三個必要條件是 (i) 同一個分頁清單裡有兩個以上的來源、(ii) 有人讀歷史、(iii) 有認領或指派。三個現在都不成立：(i) 只有 seller；(ii) 簡報自己用「沒有讀者」拒絕存關閉原因（§2.1），可是又把「留下當時的期限」當成 (e) 的好處；(iii) Q2 建議不做。(a′) 或 (g) 都不會留下臨時寫法：每一種待辦的判定式和 SQL，在之後實體化時就是 `desired` 的計算和重建查詢 | [量測] brief :88、:106、:160-161；3cd D′1 的觸發條件（brief :11 自己承認都沒發生） | 改寫 §1.5 對 (a′) 的那一列：說清楚 (e) 現在多出來的好處只剩「讀取時只查一張表」，再拿它和 35 個呼叫點、核對 job、兩份 ADR 例外的代價比較 |
| MAJOR | §2.1 `Sync`、附錄 A「`Sync` 先鎖再算（同 `Announce`）」 | **自相矛盾，也做不到「先鎖再算」。**（1）`Sync(…, desired, now)` 收的是呼叫方**已經算好**的 `desired`，所以是先算再鎖，正是 seller `AGENTS.md:73` 點名的錯誤順序。（2）`Announce` 鎖得到，是因為公告列在開店時就預先建好；待辦的列在要開新週期時還不存在，「鎖住開著的列」什麼都鎖不到。兩個沒有被同一把鎖串行的 `Sync` 同時開同一個 `(kind, subject)`，就會撞部分唯一索引（23505），依單體規則 3，整筆指令只能失敗。（3）在指令裡真正讓兩筆寫入串行的，是 seller 的店家列或帳號列鎖：照 READ COMMITTED，拿到鎖之後的讀取看得到對方已經 commit 的事實。這一點簡報沒有寫 | [量測] I/exposure/exposure.go:106-133（`LockStoreExposures` 先鎖，之後才 `StoresByIDs`、`Classify`）；S/AGENTS.md:73「先算再鎖…後鎖到的會以舊的輸入覆蓋」；I/suspension/suspension.go:80（`Recompute` 自己先 `LockAccountStores` 再讀）；I/measure/jobs.go:44（job 逐帳號先鎖再重讀） | 把不變式寫成：`syncReview` 一定在這個範圍的領域鎖之下執行（店家相關的待辦用帳號全部的 stint，也就是 `LockAccountStores`；來函相關的用 `LockAuthorityOrder`），鎖住之後才算 `desired`。`reviewqueue` 不負責串行。核對 job 照 `closeEndedWorker` 的模式逐帳號 `InTx` → 鎖 → 重算 |
| MAJOR | §2.2「對象的根是店家以外的領域鍵」、§2.3 | **以領域鍵當重算單位，等於把「哪個事實影響哪種待辦」的知識又寫回每個呼叫點。** 關店要重算這家店的食品業者登錄、措施（`endNotices` 改 `ends_at`，影響複審和解除請求的 `auto_close_at`），之後如果商家認證的成員條件加上店家狀態，也要重算商家認證；`kyc.Approve` 和 `SaveDraft` 要重算登錄；帳號刪除要重算帳號下的每一家店。v1 會漏寫，就是因為這份知識散在各處；資料集測試要事先放進這些跨根的依附資料才抓得到，而測試清單是從同一份盤點來的。兩個先例的單位都比較粗：`Announce` 以店家為單位，`suspension.Recompute` 以帳號為單位 | [量測] I/closing/closing.go:259-280（關店時結束告知）；I/foodreg/foodreg.go:304-307（核准要求店家營業中、KYC 已核准）；I/suspension/suspension.go:80；I/governance/authority.go:323,420（來函跨多個帳號，自己有 `LockAuthorityOrder`） | 重算範圍改成「帳號」（重算這個帳號全部店家相關的待辦）加上「來函」（來函和被指名者告知）。呼叫方本來就知道自己鎖了哪個帳號，不用再列出依附的根。`reviewqueue` 的列多存 `scope`（帳號或來函），`Sync` 依 `(source, scope)` 比對、關掉不在 `desired` 裡的列。可以再考慮一個結構性的保護：seller 的 transaction helper 記下這次鎖了哪些帳號，commit 前自動 `syncReview`，就不用逐一加 35 個呼叫，也不會漏 |
| MAJOR | §2.1 週期判斷 `(kind, subject, start_at)` | 對大多數種類是正確的語意，但有三個洞：（1）**恢復營業**：E11 的登錄開始時間 = max(送出, KYC 核准)，不含恢復營業的時間。關店時那一列關掉，恢復營業後 `desired` 又出現**同一個 start_at**；`Sync` 只拿開著的列比對，所以會再開一列，歷史裡出現兩列相同的週期，而且新的一列一開就已經逾期（期限 = 原本的開始 + 7 天）。要不要重新起算是產品語意，簡報沒有定。（2）**`ScheduleReview` 和告知改日期**：處理完就關掉、開新週期，語意正確；但在 `visible_from` 之前改期，會留下一列從來沒出現在待辦頁的週期，弄亂歷史和達成率。（3）重新起算的語意完全繫在開始時間的公式上，「修正資料」和「真的重新起算」分不出來（目前沒有可以改收文時間的指令，所以只是潛在風險） | [量測] P/step10e-3d-design.md:67（E11）、:190（複審 `review_by − 24h`）；S/db/queries/stores.sql:132-143（`ReopenStore`；`EndStoreClosure` 有記 `reopened_at`，事實存在）；I/measure/update.go:238（`ScheduleReview`「重排複審期限」）；I/governance/authority.go（只有 Create、Complete、ExtendHold、RecordNotice、Revoke，沒有改收文時間的指令） | 定義：每種待辦的開始時間只在「真的重新起算」時改變。登錄的開始改成 max(送出, KYC 核准, 最近一次恢復營業)，或明寫恢復營業不重新起算；`(kind, subject, start_at)` 的唯一性涵蓋全部的列（不是只看開著的）；在 `visible_from` 之前就關掉的列直接刪除，不留在歷史 |
| MAJOR | §2.2 核對「有任何寫入就以 error 記 log」 | **會把合理的時間變化當成漏寫。**（1）RISK 措施自然結束、而且沒有待處理的解除請求時，沒有任何 job 會碰它（`closeEndedWorker` 只掃有待處理解除請求的措施），複審的列只能等核對 job 關掉，每一次都會記成 error。（2）如果 `desired` 直接沿用 3d v2 和時間有關的判定式（`CaseInquiry.Overdue(now)`、`ReviewDue(now)`、`NoticeFollowUpDue(now)`），列要等到核對時才開，最多晚一小時，而且每一筆都記 error；§2.2 卻寫「就是 3d v2 的判定式」。先例的對帳 job（`reconcile_exposure`）把補上時間造成的變化當成本來就該做的事，不記錯誤 | [量測] I/measure/jobs.go:28-33,64-81（`MeasuresWithEndedRemediation`）；P/step10e-3d-design.md:183,186,189；S/AGENTS.md:73（「時間造成的變化…由 reconcile_exposure 補發」）；`rg -n "slog" I/exposure I/suspension` 0 命中 | 把 `desired` 定義成和時間無關（義務存在，從 `visible_from` 起看得到，到 `auto_close_at` 為止）。核對時，只有「關掉 `auto_close_at <= now` 的列」算正常的時間收斂，其他寫入才算漏寫，才記 error。測試裡漏寫直接讓測試失敗 |
| MAJOR | 結論「推翻三份已核准的東西」、§1.4、Q1 | **漏掉兩份被推翻的已核准決定。**（1）A7 決策簡報 A7-D8 (c) 和 A7 計畫 A7-1e：「3d 做 seller 來源、合併層、`reviewqueue` 契約；A7-1e 只加 moderation 來源，接進 10e 3d 的合併層」，原 A 方案的 `reviewqueue` 還是「依賴 seller 與 moderation」，(e) 把依賴方向倒過來。A7-D8 第 4 點的否決理由（「投影是同 transaction 的同步訂閱者，它的 bug 會 rollback `AcknowledgeReport`、`CreateMeasure`」）正是 (e) 接受的代價。（2）ADR 2026-09-24 的 D4 (a) 收窄（能在讀取時推導的可見性改用 `pub_*`）和 D3 修訂第 3 點（d3 簡報 :294 把待辦列為「composition，不變」）。另外，rb R3 是刻意選了「可以推導的重新起算」、否決了選項 (b)（另存期限會觸發實體化），簡報沒有回應 | [量測] A7/decision-brief.md:242-260；A7/2026-10-18-a7-social.md:321-322,541,545；A7/d3-read-model-brief.md:294；ADR 2026-09-24 D4 (a)；P/step10e-reports-brief.md:186-201（第一輪審查 MAJOR 6，v2 沒有處理 rb R3 選項 (b)） | 列進結論、約束帳和 Q1，並寫出 A7 計畫要改的段落 |
| MAJOR | §1.3「ADR D4」那一列、Q1 | 選同步的理由很弱，還有一項代價沒有寫進 Q1。主要理由「(f) 的數秒延遲會讓剛處理的事還在」：River 在 insert 時用 LISTEN/NOTIFY 通知，延遲通常在一秒以內［未查證］，而且只要 admin 在操作成功後從本地清單移除那一筆就能處理。同步真正的理由是「同一個 database，不需要版本、唯一 job、sweep 這些投影機器」。至於代價「`reviewqueue` 或判定式有 bug 時，seller 的每個操作都會失敗」，Q1 沒有寫 | [量測] brief :68、:91、:157 | 理由改成「省掉投影的機器」；Q1 的代價補一句：「待辦的計算有 bug 時，賣家和後台的相關操作會整筆失敗（同店家公告）」 |
| MAJOR | §2.1 鎖順序、§4 | 「佇列的鎖排在最後，同 `Announce` 的最後一個鎖」：兩個不可能都是最後。而且 ADR D4 (c) 的全域鎖順序是**由下往上**；`reviewqueue` 是 seller 依賴的下層模組，照條文應該先鎖。它之所以安全，只因為「每一列只有一個來源模組會鎖到」，而這一點沒有寫成不變式。核對 job 如果不拿帳號鎖，它批次的 `Sync` 和指令會在 `reviewqueue` 的列上交錯（第一輪審查 MAJOR，v2 只留在 §4「沒有逐條推演」） | [量測] S/AGENTS.md:73「公告永遠是 transaction 裡最後一個鎖」；I/exposure/exposure.go:106；I/suspension/suspension.go:87；ADR 2026-09-24 D4 (c) 第 3、4 點 | 寫成：帳號／店家 → …… → 公告列 → `reviewqueue` 的列，並明列 D4 (c) 的例外與它成立的條件（每一列只有一個來源寫入）。改用上面第三列的修法之後，`reviewqueue` 的鎖不再負責串行，這個風險會小很多 |
| MAJOR | 結論、Q1「關掉的待辦留下當時的期限，之後算達成率」 | 歷史的價值說過頭。沒有存關閉原因，所以「處理完」、「轉成等賣家」、「店家關閉」、「措施結束」在表裡都只是一個 `closed_at`。達成率要分得出「準時處理」，就得回頭從來源的事實猜是哪一個事實關掉它的。`opened_at` 是建列的時間，可能是回填、核對或比可見時間早好幾個月（複審在建立措施時就開列）。所以目前存下的歷史算不出達成率 | [量測] brief :104-106 | 二選一：在 `Sync` 存關閉原因（判定式本來就知道是哪個條件不成立），或者把「歷史、達成率」從 (e) 的好處裡拿掉，改成終局的觸發條件 |
| MINOR | §2.1「`closed_at = auto_close_at`」 | 和先例不一致：`suspension.Recompute` 以重算的那一刻 `now` 結束期間。用 `auto_close_at` 比較精確，但要說明理由。同時要寫明 `closed_at` 不會早於 `opened_at`（指令在 `auto_close_at` 之後才第一次開列時） | [量測] I/suspension/suspension.go:144-153（`EndSuspensionPeriod` 用 `now`） | 寫明理由與不等式 |
| MINOR | §2.3 呼叫清單 | 清單混了指令和內部 helper（`close`、`Reopen` 是在 `measure.Revoke` 的 transaction 裡被呼叫的），只列名稱，沒有附 file:line，也沒有寫每個呼叫要重算的範圍。`foodreg.Revoke`、`ExtendAuthorityOrderHold`、`ReconcileTerminations`、`AdvanceStore` 沒有逐一判定（大多會經過 `closeTerminated`，或不影響待辦） | [量測] I/measure/revoke.go:217,262；I/closing/termination.go:58-70,159-200；I/foodreg/foodreg.go:386 | 改成一個 transaction 邊界一列（`InTx` 的位置），寫出鎖的範圍；或直接用上面說的 helper 自動重算 |
| MINOR | Q1、Q2 的白話 | Q2 改好了。Q1 還有幾個術語沒有解釋：「合併層」、「跨來源翻頁的特殊處理」、「工作項目」、「週期」。其他選項擠在一行裡，沒有各自的優點和代價（第一輪審查 minor，只改了一部分）；Q1 也沒提 A7 計畫要跟著改 | brief :154-161 | 每個選項一行優點、一行代價（加上 (g)），第一次出現的術語用一句白話解釋 |
| MINOR | §1.1「我們的情況：第二層」 | 我們是一個 database 的單體，11 種待辦都在 seller 的 schema 裡，形狀比較接近第一層（ServiceNow：各種工作延伸同一個 `task` 表，佇列是查詢、SLA 由引擎在每次更新時重算後存下）。Salesforce Omni 另存工作項目是為了路由。把我們歸到第二層，是 (e) 最主要的業界依據，但這個歸類本身有爭議 | ir A1（task 繼承、SLA 條件「每次建立或更新時評估」VERIFIED）、ir A4 | 寫出兩種歸類各自的結論；比較誠實的說法是：(e) 是「Omni 的中央項目」加上「ServiceNow 的 SLA 引擎」組合出來的混合做法 |

## 二、第一輪審查的處理狀況

| # | 第一輪 | 狀態 | 證據 |
|---|---|---|---|
| 1 | BLOCKER 寫入點低估 | 部分處理 | Q1 改成約 35 個，§2.3 列出名稱；但沒有附 file:line，也沒有寫跨根的依附範圍（見問題表第四列） |
| 2 | MAJOR 店家關閉是雙向的 | 部分處理 | 重算會自然重開，`Reopen` 也在清單上；但「`Reopen` 要重算哪些根」沒有定義，而且恢復營業會產生重複的週期、一開就逾期（第五列） |
| 3 | MAJOR 複審的 `auto_close_at`、`Update`／`End` | 已處理 | 改成重算 `auto_close_at`，`Update`／`End`／`ScheduleReview` 都在清單上；但衍生出新問題：時間造成的關閉會被記成漏寫（第六列） |
| 4 | MAJOR 核對和「不是投影」自相矛盾 | 部分處理 | §1.3 承認成員和期限是副本，開著的週期跟著目前的規則走；但沒有定義什麼算漂移（第六列） |
| 5 | MAJOR D3 修訂第 5 點 | 已處理 | 寫成要加 ADR 例外；但 D4 (a) 和第 3、4 點沒有提（第一、七列） |
| 6 | MAJOR gov ADR :29、:113 與 rb R3 選項 (b) | 部分處理 | gov ADR 已經列出；rb R3 刻意否決選項 (b) 這一點沒有回應 |
| 7 | MAJOR「前提已不成立」、「第一步」是推論 | 已處理 | 結論第一點、§1.1 標成 INFERENCE |
| 8 | MAJOR 業界基準說過頭 | 部分處理 | 分成兩層；但沒有回應 3b:357，把我們歸到第二層也有爭議（最後一列） |
| 9 | MAJOR 漏了 (d)、(e) | 已處理 | 但漏了 `pub_*` 選項（第一列） |
| 10 | MAJOR 發票不是 REAL | 已處理 | 約束帳 :82 |
| 11 | MAJOR 核對修復沒有規定鎖 | **沒有處理**（附錄寫「已處理」，實際不成立） | §2.1 的 `Sync` 收的是算好的 `desired`，而且鎖不到還不存在的列（第三列） |
| 12 | minor 冪等語意 | 大致處理 | 重算本身是冪等的；同時開列撞 23505 的情況見第三列 |
| 13 | minor 只限管理員撤回 | 已處理 | 沿用 3cb B11 |
| 14 | minor 詳情頁的期限只有一個來源 | 已處理 | §2.2 |
| 15 | minor 只有 2/11 種不能用索引 | 已處理 | |
| 16 | minor 引用漂移 | 已處理 | |
| 17 | minor 沒交代的待決事項（含鎖順序） | 部分處理 | 鎖順序只留在 §4，但 (e) 能不能成立就靠它（第九列） |
| 18 | minor Q1、Q2 的白話 | 部分處理 | Q2 改好了，Q1 還沒有（倒數第二列） |

退步：附錄 A 第 5 列宣稱鎖的問題已處理，和 §2.1 矛盾。(e) 把佇列的依賴方向倒過來，和 A7 計畫衝突，卻沒有揭露。

## 三、審查者自己的建議

**現在採 (g)「`pub_*` 組合」；終局是修正過的 (e)，有明確的觸發條件。**

1. **(g) 合乎現行的、已核准的規則**：ADR D3 修訂第 4 點、D4 (a)；也合乎第一層的業界做法（一個 database 裡，佇列就是對實體做的查詢）。seller 發布 `seller.pub_review_items(at)`，裡面就是 3d v2 的判定式和 `ReviewPolicy` 參數，Go 和 SQL 共用一份，資料集測試照 E10 鎖住一致。待辦模組依賴 seller，用一支 SQL 做 keyset；A7-1e 只加 `moderation.pub_review_items(at)` 和一個 `UNION ALL` 分支。沒有合併層、哨兵和 `SeekFor`，寫入路徑零改動，seller 的指令也不會被佇列的 bug 拖垮。
2. **中央表現在的必要條件都不成立**：來源只有一個；歷史沒有讀者，而且沒存關閉原因也算不出達成率；只有一位營運人員（REAL）。(g) 不會留下要丟掉的程式：實體化的那天，`pub_*` 就是 `desired` 的計算和重建查詢。
3. **觸發條件（同 ADR 第 5 點）**：第一份達成率報表、出現第二位營運人員（要認領）、正式資料量下的 p95 超過 300 ms，其中任一個成立就改成 (e′)。
4. **(e′) 的形狀**（觸發時，或 Will 現在就要歷史的話）：
   - 重算範圍是「帳號」和「來函」，在已經拿到的領域鎖之下先鎖再算；
   - 用 seller 的 transaction helper 自動重算，不手動加 35 個呼叫；
   - 列上存關閉原因；
   - 開始時間只在真的重新起算時改變，`(kind, subject, start_at)` 在全部的列上唯一，在 `visible_from` 之前就關掉的列刪除；
   - 漂移的定義排除 `auto_close_at` 造成的關閉；
   - 鎖順序：公告列 → `reviewqueue` 的列，並在 D4 (c) 寫下例外。
5. 如果 Will 認為「歷史現在就要」是硬需求，那 (e′) 比 v2 的 (e) 好，但 Q1 要誠實寫出代價：同步耦合，再加上 A7 計畫要改。

## 四、我驗證了什麼、沒驗證什麼

- 讀了：brief 全文、rv 全文、ir（A1、A2 和結論）、3d v2 :19-72,175-192；exposure.go:95-185；suspension.go:60-170；measure/jobs.go:1-130；foodreg.go:300-410；authority.go 的鎖（:323、:410-428、:691-712）；termination.go:55-200；stores.sql `ReopenStore`；S/AGENTS.md:52,73,94,105,113；ADR 2026-09-24 :300-352；modules.go；pub.sql；A7 decision-brief :242-260、A7 計畫 :321-322、:541-554；d3 簡報 :294。
- 沒有做：沒有跑測試和 SQL；沒有查 River 的通知延遲［未查證］；沒有逐條核對 35 個函式是否完整（只抽查 foodreg、closing、kyc、authority 的匯出函式）；沒有用 `EXPLAIN` 量 (g) 的效能。
