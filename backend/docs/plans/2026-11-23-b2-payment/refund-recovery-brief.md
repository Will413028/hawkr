# B2 3c：退款與轉帳的先後順序，決策簡報

## 結論（先講）

**前提錯了，應該改。** D1 的「先撤回轉帳，再退款」，以及「轉帳金額在寫列時就定死」，正是 W1 和 W2 的來源。業界做法（Stripe 對 SCT 的官方說明，加上 destination charge 的 `reverse_transfer`）是：**買家退款一律由平台餘額先付；向賣家收回的錢是另一件事，等退款成功之後再做。** 收回的方式可以是撤回轉帳（transfer reversal），也可以是在之後的轉帳裡少付。

建議改成「**先退款，後收回**」，並把轉帳金額改在 **job 第一次呼叫 provider 時**（鎖住付款列）才算定：

1. 轉帳還沒呼叫 provider 時發生的退款，直接讓轉帳少付，不需要撤回。
2. 轉帳已呼叫（或已成功）後發生的退款，照常退給買家。**provider 一接受退款**（回應不是失敗，`pending` 也算）就開始向賣家收回（撤回轉帳）。如果轉帳還沒有結果，就等轉帳 `SUCCEEDED` 再撤回。不等退款 `SUCCEEDED` 的原因：平台餘額不足時，退款會停在 `pending`，要等撤回把錢補回平台才會成功。如果撤回反過來等退款成功，兩邊會互相等，卡死。
3. W1 消失：`CreateRefund` 不必再因為轉帳是 `PENDING` 或 `ANOMALY` 而回 FailedPrecondition。
4. W2 **從常見變成罕見，而且一定會告警**，但不是不可能：被 provider 直接拒絕或查無的退款，根本不會觸發撤回；只有「provider 接受了、後來才變成 `FAILED`」的退款，才會出現賣家已被扣、買家沒拿到錢的情況。這種情況照 Will 2026-11-24 的決定處理：收回的錢留在平台，發 `MoneyAnomaly`，由後台決定。反過來的情況（退款成功、撤回永久失敗）由平台吸收並發 `MoneyAnomaly`。依 spike #9，撤回不會因為賣家餘額不足而失敗，所以只剩轉帳已撤光、或 provider 異常兩種。

需要修訂 D1（計畫）與 ADR D20 的退款句子。D68 不必改。

## 前提問題：要改的程式是否本來就排定要重寫？

- 要改的是 3b 的撤回路徑、3c 的退款路徑，以及 schema。計畫裡沒有任何一步排定要重寫或刪除它們：步驟 4 只把 legacy `CreateRefund` 和 `GrantPayment` 接到這裡；B8 只把呼叫端改成 in-process；B10 已退場（計畫「不做」那一行）。
- 所以這不是「之後反正會重寫」的情況，而是**現在不改，就會變成永久設計**。系統還沒上線，root `AGENTS.md`「上線前不受相容性約束」「不留技術債」：migration 可以直接改，不必做相容層。
- 重工量（實測檔案大小，用 `wc -l`；改動比例是推論）：
  - 主要改動：`internal/charge/refund.go`（532 行，約一半要重寫：拿掉 REVERSING 分支和轉帳 PENDING 的拒絕）、`reversal.go`（270 行，觸發點從「退款 REVERSING」改成「退款 SUCCEEDED 且轉帳 SUCCEEDED」，呼叫與收斂的骨架大多保留）、`transfer.go`（340 行，金額改在 claim 時計算，轉帳成功時要補排待收回）、`anomaly.go`（撤回階段的人工結案要改掛到新的收回列）、`jobs.go` 的補掃。
  - schema：`20261124010742_payment_refunds.sql` 和 `db/schema.sql` 的退款狀態拿掉 `REVERSING`、`REVERSED`。撤回仍用**退款列上的欄位**記錄（`reversal_amount`、撤回狀態、撤回鍵、`reversal_external_id`），不另開表：一筆退款最多一次撤回，是一對一的關係。只有「一筆退款要撤回多次」或「沒有退款也要撤回」（例如爭議款）時，才需要獨立的表；目前這兩個條件都不成立。撤回改成退款列上的一條**獨立狀態欄**，跟退款狀態並行，各自收斂；`queries/transfers.sql`、sqlc 生成檔重新生成。
  - 測試：`refund_integration_test.go`（849 行）裡撤回相關的案例、`transfer_integration_test.go` 的撤回段、`converge_*` 的撤回階段、mutation 清單（把「拿掉先撤回」換成「退款成功前就撤回」「claim 之後仍改金額」）。
  - 不動：3a′ 的三分收斂與查回骨架（`attempt.go`、`convergence.go`）、付款建立、inbox 的 handler、port 的方法簽章（`ReverseTransfer`、`FindReversal` 照用）。
  - 還要加上：預留額度的記帳（待收回額的計算、什麼時候釋出，見 Q4）、「provider 接受退款就觸發撤回」的觸發點（退款 job 的成功分支加上 sync 的 `REFUNDING` 分支）、轉帳換鍵時重算金額，以及人工結案帶出 provider 核實過的金額。修正後的估計是一個中型子步驟。這是推論，比原本估的多出約三分之一。
- 判定：這次重工是 REAL 需要的工作，不是被計畫排程擋住的。延後只會讓步驟 4 和 B8 的呼叫端學會處理「轉帳中請重試」的錯誤，等於把技術債傳出去。

## 業界基準

| 系統 | 退款與向賣家收回的順序 | 來源與證據等級 |
|---|---|---|
| Stripe SCT（我們的模式） | 「Regardless of the charge type, refunds come from your platform's balance.」「refunding a charge doesn't affect any associated transfers.」平台自己負責收回：「by reducing subsequent transfer amounts or by reversing transfers.」平台餘額不足時，退款不會失敗，而是「the refund status is set to pending」 | 文件，本次讀過：docs.stripe.com/connect/marketplace/tasks/refunds-disputes |
| Stripe destination charge | 退款帶 `reverse_transfer=true`，Stripe 在退款時一併撤回；主體是退款，撤回是附帶動作 | 文件（同頁，以及 spikes.md #7 引用的參數）；「主體是退款」這個解讀是推論 |
| Stripe 撤回的限制 | 「Can only reverse up to the unreversed amount remaining」「Once entirely reversed, a transfer can't be reversed again.」 | 文件，本次讀過：docs.stripe.com/api/transfer_reversals/create |
| Stripe 爭議款 | 接受爭議就立刻撤回；抗辯就等結果再撤回。「Delaying the reversal … puts your platform at risk」。也就是說，收回是**跟在退錢事件後面**的 | 文件，本次讀過（refunds-disputes） |
| Etsy Payments、Uber Eats、Airbnb、蝦皮 | 買家退款由平台（或託管）先付，再從賣家的餘額或下一次撥款扣回；賣家餘額可以是負的。託管型（蝦皮）在撥款前退款，撥款金額就是已收減已退 | **推論**（憑一般認知；Etsy、Adyen 的說明頁本次抓取 404，沒有讀到原文） |
| Adyen for Platforms | 退款從 balance account 依 split 指示扣款，賣家餘額可為負，由平台承擔 | **推論**（文件頁 404，未讀到） |

共同點：**買家先拿到錢，收回賣家的錢跟在退款之後；撥款前就退款的，直接讓撥款少付。** 沒有任何一家是「先扣賣家，再退買家」。

## 現行機制為什麼存在

- spikes.md #6、#8：退款如果不撤回轉帳，錢會從平台餘額出，平台餘額可能變負，讓後續帶 `source_transaction` 的轉帳以 insufficient funds 失敗。D1 為了「平台餘額永遠不變負」而把撤回排在退款前面。
- 「轉帳金額在寫列時定死」：3b 寫列時就算好 `付款 − 非失敗退款`，job 只負責送出。所以寫列之後才進來的退款會造成多付，W1 因此只能拒絕這種退款。
- 這兩點都是為了守住平台餘額。但 #8 的風險其實是**暫時性**的：撤回在退款之後幾秒到幾分鐘內就會補回；轉帳 job 已經把餘額不足當成可重試（3b）；Stripe 退款遇到餘額不足也只是 pending（文件）。所以這兩點不值得拿 W1、W2 去換。

## 子題分析

### Q2：轉帳金額要在呼叫時才算嗎？
要。轉帳 job 在送出前，用一個 transaction：鎖付款列 → 重算 `付款 − 非失敗退款` → 把金額連同 `called_at` 一起寫入（稱為 claim，意思是「這筆金額已承諾給 provider」）→ commit 之後才呼叫 provider。
- **金額綁著冪等鍵**：同一把鍵的金額永遠不變。原因是 Stripe 的冪等機制會比對參數：同一把鍵如果送出不同參數，Stripe 會回錯誤（文件 docs.stripe.com/api/idempotent_requests，本次沒有重讀，憑先前的認知）。D10 的「同鍵重送」也是建立在這一點上。
- 只有在**換新鍵之後**才重算金額。換鍵只發生在可重試拒絕、而且是這把鍵的第一次呼叫（`transfer.go:181` 的 `rotateTransferKey`；`transfer.go:117` 的說明寫「清掉 called_at」）。重算放在換鍵的同一個 transaction 裡，鎖住付款列。結果不明時一律同鍵、同金額。
- 算出來是 0（全退了）：轉帳直接進終局，不呼叫 provider（細節留給實作決定）。

### Q3：轉帳成功之後的退款，三種做法比較

| 做法 | 平台餘額（#8） | W2（賣家被扣但買家沒退到） | 失敗條件 |
|---|---|---|---|
| A 先撤回再退款（現行） | 永遠不變負 | **會發生**，要靠告警加人工處理 | 退款被拒或查無；而且轉帳還沒結果時退款只能被拒（W1） |
| **B 先退款再撤回（建議）** | 從 provider 接受退款到撤回落地的這段時間，平台餘額會變低（見下方「平台餘額變低多久」） | 罕見、一定告警：只有 provider 先接受、後來才 `FAILED` 的退款會發生；錢留在平台，發 `MoneyAnomaly`（Will 2026-11-24 的決定） | 撤回永久失敗（轉帳已撤光或 provider 異常）→ 平台吸收，發 `MoneyAnomaly`。依 #9 不會因為賣家沒錢而失敗 |
| C 賣家帳本（ledger）淨額，從下一次撥款扣 | 同 B，而且負擔拉得更長 | 不可能 | 要做跨付款的賣家餘額帳本和對帳，等於重做 B10；賣家之後沒有訂單就收不回來 |

**平台餘額變低多久（spike #8：別的賣家的轉帳會因此失敗）**
- 撤回是同步的：API 回 200，就代表撤回已經建立。spike #9 實測，撤回後賣家的 available 立刻變負數；「錢同時回到平台餘額」是推論，3d 可以實測確認。
- 所以餘額變低的時間 = provider 接受退款，到撤回 job 跑完的這段時間。正常情況是 job 排隊加一次 API 呼叫，幾秒到幾分鐘（推論）。最壞情況是撤回的結果不明，要走 D10 的重送與查回，最長到補掃週期乘重試次數（推論，沒有量過）。
- 這段時間裡被擋下的轉帳，3b 已經把它當可重試拒絕：換新鍵重試、不計期限，所以不會掉錢，只會晚一點撥款。
- 要不要另外做平台保留餘額或最低餘額檢查？只有在一個條件成立時才需要：平台平常的餘額，小於「同一時間正在收回的金額總和」，而且撥款延遲讓賣家實際受影響。上線前沒有任何流量，這個條件無法成立，所以**現在不做**。觸發條件：上線後，轉帳的可重試拒絕次數出現在告警或指標上。

選 B。C 是 Stripe 文件列出的另一個選項，但 SCT 是一筆付款對一筆轉帳，撤回已經足夠，而且永遠不會因為賣家餘額不足而失敗。C 需要的帳本機制，目前沒有任何必要條件成立。

### Q4：退款和轉帳同時在跑時，誰保證不多付？
兩邊都鎖**付款列**，所以對同一筆付款來說，兩者會排成先後順序：
- 退款先 commit，轉帳 claim 在後：claim 的金額已經扣掉這筆退款，不需要撤回。
- 轉帳 claim 先 commit，退款在後：退款看到轉帳已 claim（不論 `PENDING`、`SUCCEEDED` 或 `ANOMALY`），照常排退款 job，並記下「需收回額 = min(退款額, 已 claim 金額 − 已撤回 − 其他待收回)」。
  - 待收回的撤回在「provider 已接受退款（`REFUNDING` 或 `SUCCEEDED`）且轉帳 `SUCCEEDED`」時排 job。兩個事件誰後到，誰就負責排，同樣鎖付款列。
  - 轉帳最後 `FAILED`：不撤回，錢本來就在平台手上。
  - 轉帳 `ANOMALY`：撤回維持等待。如果人工結案判定轉帳成功，撤回的上限用 **provider 核實過的轉帳金額**（結案時查回的轉帳物件金額，減掉 provider 上已撤回的金額），不用列上的金額，然後重算每筆待收回額。人工結案判定失敗，就不撤回。
- **預留額度什麼時候釋出**（預留 = 待收回額，計入「其他待收回」）：
  - 退款在撤回開始前就 `FAILED`（provider 直接拒絕或查無）：在同一個 transaction 釋出預留，不撤回。
  - 退款在撤回已成立後才 `FAILED`：預留變成「已撤回」，轉帳的 `reversed_amount` 不減，錢留在平台，發 `MoneyAnomaly`（照 Will 2026-11-24 的決定）。
  - 退款在撤回正在進行時 `FAILED`：撤回照樣收斂到底（不能取消已經送出的呼叫），落地後走上一條。
  - 撤回永久失敗（`ErrPermanent` 或查無滿期限）：釋出預留，退款保持原狀，發 `MoneyAnomaly`，由平台吸收。
  - 撤回結果不明或進 `ANOMALY`：預留持續佔住，直到人工結案。
- 不變量：`已轉 − 已撤回 ≤ 付款 − 已成功退款`。只有在撤回還沒完成的那段時間會暫時不成立，由收回列追蹤。累計上限（D4）照舊：鎖付款列並加總非失敗退款。

### Q5：限制條件總表

| 限制 | 判定 | 來源 |
|---|---|---|
| SCT 退款不會自動撤回、要另外撤回 | REAL（Stripe 的行為） | spikes.md #7、Stripe 文件 |
| 撤回不能超過未撤回的餘額；撤光就不能再撤 | REAL | Stripe API 文件、spikes.md #4 |
| 撤回不會因為賣家餘額不足而失敗（賣家變負，由平台承擔） | REAL | spikes.md #9、SCT 文件「responsible for negative balances」 |
| 平台餘額不足時轉帳會失敗 | REAL，但 3b 已把它當可重試，所以不構成「必須先撤回」的理由；餘額變低多久見 Q3 | spikes.md #8 |
| Stripe 冪等鍵：同一把鍵送不同參數會回錯誤 | REAL（Stripe 的行為；這才是同一把鍵金額不能改的根據） | 文件 docs.stripe.com/api/idempotent_requests（本次沒重讀） |
| D10/D68 的「先查回、再換鍵」規則 | SELF-IMPOSED（我們自己定的，但建立在上一列的 REAL 行為上，而且本案不需要推翻它） | ADR D68 |
| 「先撤回再退款」的順序 | SELF-IMPOSED（D1 的設計選擇，Stripe 沒有規定順序） | 計畫 D1、spikes.md「對計畫的影響」 |
| 轉帳金額在寫列時定死 | SELF-IMPOSED（3b 的實作選擇） | `payment.md` §10 |
| 3b、3c 已經寫好、已經 commit | SELF-IMPOSED（屬於工作量與一致性） | `72ee68147` |
| 步驟 4、B8 的呼叫端 | SELF-IMPOSED，而且改了反而更簡單（少一種要重試的錯誤） | 計畫步驟 4 表、D5 修訂 |
| B10 手續費與對帳 | 不適用（已退場）；C 案才需要 | 計畫「不做」 |
| Stripe 對平台持有資金的時間上限（延後項） | 不擋本題，各案都一樣受影響。文件本次仍沒找到上限（推論：沒有寫）；觸發條件照舊：上線前核對 | 計畫「延後」、SCT 文件 |

所有把設計推離業界基準的限制都是 SELF-IMPOSED，所以建議就是業界基準（B 案加呼叫時才算金額），不做任何調整。

### Q6：ADR 和計畫要修什麼
- 計畫 D1 的「完成後的退款先撤回轉帳」改成「退款由平台先付；轉帳金額在第一次呼叫時算定；已承諾的轉帳在退款成功後撤回。撤回不會因賣家餘額不足而失敗，永久失敗時由平台吸收並告警」。
- ADR `docs/adr/2026-11-13-payment-onboarding-provider-port-stripe.md` 的 D20 列（第 234 行）與本文「開通閘門與收款模式」一節加一筆修訂。是否開新的 D 編號，依 adr-gate 決定。
- D68 不必改：收斂規則照舊適用於退款與撤回。
- `payment.md` §8 刪掉兩條已知限制（W1、W2）；§10、§12 重寫。

## 會推翻這個建議的事實
- Stripe 對 SCT 退款遇到平台餘額不足時是「直接失敗」而不是 pending（目前文件寫 pending，3d 可以在 sandbox 實測確認）。如果會失敗，B 案的退款也要有可重試分支；這只會加程式，不會推翻順序。
- 撤回在某些情況下**會**因為賣家餘額不足而被拒（例如沒有開 connected reserves 的帳戶類型；本次只看到摘要提到這點，沒讀到原文）。如果是這樣，B 案的「平台吸收」就會成為常見情況，這時要評估 C 案（從後續撥款扣），或改用 destination charge 搭配 `reverse_transfer`。這件事 spike #9 實測過是 200，所以目前不成立。
- 法規或合約規定賣家要先同意才能退款，這種情況目前沒有。

## 可逆性與成本
- 現在改：重工約五個 Go 檔、一份 migration、四個 integration 測試檔，外加 mutation 清單，估計一個子步驟（推論）。
- 上線前想再改回 A 案：成本差不多，可以逆轉。上線後要改，就得做資料遷移，加上處理進行中的撤回，成本高很多。所以現在是改的最便宜時機。

## 終局與觸發
- 終局：B 案加 claim 時算金額，作為 3c 的修正子步驟（建議叫 3c′）。W1、W2 從 §8 刪除；mutation 新增五條：「provider 拒絕退款仍撤回」「同一把鍵重算金額」「轉帳後到時不補排待收回」「退款在撤回前 FAILED 時不釋出預留」「人工結案用列上金額而非核實金額」，各自要有測試失敗。
- 再評估 C 案（賣家帳本）的觸發條件：要做跨訂單的手續費或扣款，或撤回的永久失敗在營運上不再罕見。
