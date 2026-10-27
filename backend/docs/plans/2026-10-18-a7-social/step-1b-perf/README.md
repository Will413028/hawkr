# A7-1b 封鎖查詢的效能量測

量測日期 2026-10-24（封鎖拆成 `buyer.blocked_buyers`、`buyer.blocked_stores` 兩張表之後重量一次，涵蓋列表、單列歸屬與寫入前置檢查）。結論先講：

- **判斷方式**：不看絕對 p95。每個觀看者變體都帶同一個 query、同一份參數的匿名對照，兩者在同一輪交錯計時（`measure.go` 的 `timePair`），看**同一次量測裡觀看者對匿名的比值**加計畫形狀。這台機器的 load average 常在 30–140，同一個查詢在兩次量測之間差數倍，絕對值不能判斷退步。
- **比值**：22 個觀看者變體，100 個封鎖的觀看者（run 1）p95 比值 0.70–1.36、p50 比值 0.99–1.30；2,000 個封鎖的觀看者（run 2，1,000 家店加 1,000 位買家）p95 比值 0.86–1.60、p50 比值 1.03–2.31（最大是 feed 的 2.7 ms 對 1.2 ms，絕對差 1.5 ms）。沒有因為封鎖而變慢的情境。
- **計畫形狀**：觀看者變體的 `EXPLAIN (ANALYZE, BUFFERS)` 裡，封鎖表沒有 Seq Scan，也沒有 `Function Scan on pub_blocked_stores／pub_blocked_buyers`（兩個函式都被內聯，`measure.go` 對每個情境檢查，有就以 exit 4 結束）。全部是 `Index Only Scan` 在 `blocked_stores_pkey`、`blocked_stores_store`、`blocked_buyers_pkey`、`blocked_buyers_blocked` 上。
- **絕對值**：觀看者變體最慢的是 `TopPosts`（run 1 p50 63.13／p95 104.88 ms，匿名對照 57.67／110.13 ms，比值 1.09／0.95）；它的 `Seq Scan on post_stats` 是 A7-1a 就有的（最新 30 篇取瀏覽最高 5 篇的外層 join），匿名語句不變。其餘全部在 14 ms 以下（run 2 的 hashtag 貼文 p95 13.60 ms，匿名對照 11.70 ms）。
- 完整量測另有一個匿名診斷變體（`hashtag 建議 中文前綴 1 字（可曝光店家先算一份）`，把 `pub_exposable_stores` 實體化成 CTE）p95 超過 300 ms：它是 A7-1a 就有的對照，不含封鎖，不是 query 檔本身。

## 量測條件

- 資料：`step-1a-perf/seed.sql`（`setseed(0.42)`：10,000 家店、50,000 位買家、100,000 篇貼文、700,000 則留言、100,000 則討論區留言），加 `seed-blocks.sql`：觀看者 12345 封鎖 40 家店（2–21 號與 5000–5019 號，占已發布貼文的 3.46%，不含最熱門的 1 號店）與 40 位買家（1–40 號），被 20 位買家封鎖（41–60 號），另有其他買家的隨機封鎖（`blocked_stores` 與 `blocked_buyers` 各約 10,000 列，兩張表共 20,099 列）。run 2 再加 `seed-heavy-viewer.sql`：觀看者 12346 封鎖 1,000 家冷門店與 1,000 位買家。
- 方法同 A7-1a（`step-1a-perf/README.md`）：`perf/measure.go` 呼叫 `reads` 的建構函式、prepared statement、`plan_cache_mode = force_generic_plan`、暖機 10 次、計時 200 次、page_size 50。`-viewer <買家 ID>` 旗標跑有觀看者的變體並配匿名對照；`-ratio`（預設 3）與 `-floor`（預設 10 ms）是退步判斷：觀看者 p95 超過匿名對照 p95 的 `ratio` 倍，而且觀看者 p95 高於 `floor`，以 exit 5 結束；封鎖表出現 Seq／Function Scan 以 exit 4 結束。
- **load average（1 分鐘／5 分鐘／15 分鐘）**：run 1 開始 29.7／44.4／47.1、結束 16.5／28.2／38.6；run 2 開始 17.8／27.9／38.3、結束 17.6／26.4／37.2。機器是 8 核，別的 session 在跑。比值在同一輪交錯計時，兩邊吃到同樣的負載；絕對值仍受影響，不能當門檻。
- 重現（在 `backend/`，其餘步驟同 A7-1a 的「重現」）：

```sh
docker --context desktop-linux exec -i a7-1b-perf psql -U postgres -d perf < docs/plans/2026-10-18-a7-social/step-1a-perf/seed.sql
docker --context desktop-linux exec -i a7-1b-perf psql -U postgres -d perf < docs/plans/2026-10-18-a7-social/step-1b-perf/seed-blocks.sql
docker --context desktop-linux exec -i a7-1b-perf psql -U postgres -d perf < docs/plans/2026-10-18-a7-social/step-1b-perf/seed-heavy-viewer.sql
GOWORK=off go run ./internal/modules/social/perf/measure.go -dsn '…' -reps 200 -viewer 12345 -out /tmp/a7-1b-perf-out   # 或 -viewer 12346 -only '^觀看者'
```

## 結果（ms；page_size 50；比值 = 觀看者 ÷ 同一輪的匿名對照）

| 情境 | 匿名對照 p50 / p95 | 100 個封鎖的觀看者 p50 / p95 | 比值 p50 / p95 | 2,000 個封鎖的觀看者 匿名對照 p50 / p95 | 觀看者 p50 / p95 | 比值 p50 / p95 |
|---|---|---|---|---|---|---|
| feed 第一頁 | 1.57 / 6.84 | 1.81 / 8.43 | 1.15 / 1.23 | 1.20 / 2.72 | 2.71 / 4.35 | 2.25 / 1.60 |
| feed keyset 游標在 50% | 1.27 / 3.85 | 1.34 / 3.16 | 1.06 / 0.82 | 1.17 / 3.50 | 2.70 / 4.56 | 2.31 / 1.30 |
| feed keyset 游標在 90% | 1.45 / 4.17 | 1.56 / 4.27 | 1.08 / 1.02 | 1.20 / 3.05 | 2.77 / 4.84 | 2.31 / 1.59 |
| 店家頁 熱門店 第一頁 | 0.92 / 2.47 | 1.14 / 3.03 | 1.24 / 1.23 | 0.79 / 1.70 | 0.97 / 1.89 | 1.23 / 1.11 |
| 店家頁 熱門店 游標第 200 篇 | 0.90 / 3.04 | 1.08 / 3.53 | 1.20 / 1.16 | 0.95 / 3.95 | 1.13 / 3.58 | 1.19 / 0.91 |
| hashtag 貼文 熱門標籤 第一頁 | 1.34 / 4.07 | 1.59 / 3.62 | 1.18 / 0.89 | 2.00 / 11.70 | 2.27 / 13.60 | 1.13 / 1.16 |
| 熱門貼文 TopPosts | 57.67 / 110.13 | 63.13 / 104.88 | 1.09 / 0.95 | 50.83 / 134.89 | 56.99 / 116.11 | 1.12 / 0.86 |
| 單篇貼文（含 stats）熱門 | 0.60 / 2.69 | 0.64 / 2.72 | 1.07 / 1.01 | 0.54 / 1.43 | 0.62 / 1.24 | 1.16 / 0.87 |
| 寫入前置 PostIsPublic | 0.60 / 3.24 | 0.62 / 2.28 | 1.03 / 0.70 | 0.53 / 1.21 | 0.61 / 1.39 | 1.16 / 1.15 |
| 留言 熱門貼文頂層 第一頁 | 1.57 / 9.28 | 1.62 / 9.76 | 1.03 / 1.05 | 1.03 / 3.23 | 1.39 / 3.37 | 1.36 / 1.05 |
| 留言 熱門貼文頂層 游標第 200 則 | 1.08 / 4.62 | 1.29 / 4.43 | 1.20 / 0.96 | 0.99 / 2.81 | 1.54 / 3.42 | 1.55 / 1.22 |
| 留言 一般貼文頂層 第一頁 | 0.85 / 4.24 | 1.01 / 5.18 | 1.19 / 1.22 | 0.74 / 2.08 | 1.08 / 3.14 | 1.47 / 1.51 |
| 留言 回覆列表 | 0.67 / 2.44 | 0.82 / 2.94 | 1.23 / 1.21 | 0.71 / 2.92 | 0.87 / 2.80 | 1.23 / 0.96 |
| 留言 回覆列表 游標在中段 | 0.67 / 2.96 | 0.78 / 2.82 | 1.16 / 0.95 | 0.65 / 1.36 | 0.81 / 1.67 | 1.24 / 1.22 |
| 討論區 熱門店頂層 第一頁 | 0.86 / 3.23 | 0.99 / 3.00 | 1.15 / 0.93 | 0.82 / 2.43 | 1.06 / 2.60 | 1.28 / 1.07 |
| 討論區 熱門店頂層 游標在中段 | 0.92 / 2.67 | 1.06 / 2.87 | 1.15 / 1.07 | 0.78 / 1.93 | 1.06 / 2.34 | 1.36 / 1.21 |
| 討論區 回覆列表 | 0.61 / 1.95 | 0.74 / 1.96 | 1.21 / 1.00 | 0.56 / 1.45 | 0.89 / 1.85 | 1.59 / 1.27 |
| 寫入前置 ForumIsReadable | 0.57 / 2.02 | 0.56 / 2.00 | 0.99 / 0.99 | 0.52 / 1.65 | 0.54 / 1.61 | 1.03 / 0.98 |
| 歸屬 ReadableCommentPost 頂層留言（留言按讚） | 0.61 / 2.48 | 0.73 / 2.83 | 1.21 / 1.14 | 0.54 / 1.28 | 0.72 / 1.75 | 1.34 / 1.37 |
| 歸屬 ReadableCommentPost 回覆（含 ParentAdmitted） | 0.60 / 2.21 | 0.79 / 2.29 | 1.30 / 1.04 | 0.60 / 1.96 | 1.00 / 2.58 | 1.67 / 1.32 |
| 寫入前置 ReplyTargetIsReadable | 0.55 / 1.89 | 0.68 / 2.05 | 1.24 / 1.08 | 0.47 / 1.14 | 0.69 / 1.41 | 1.46 / 1.24 |
| 寫入前置 ForumReplyTargetIsReadable | 0.48 / 1.32 | 0.59 / 1.79 | 1.23 / 1.36 | 0.47 / 1.12 | 0.69 / 1.38 | 1.48 / 1.22 |

`TopPosts` 的絕對值是 A7-1a 就有的 `Seq Scan on post_stats`，不是封鎖造成：匿名對照與觀看者變體在同一輪都是 50–135 ms。run 2 的 feed p50 比值 2.25–2.31 是 1.2 ms 對 2.7 ms：2,000 個封鎖的 Join Filter 多比對幾千次，p95 比值仍在 1.6 以下。

## 計畫形狀

- **觀看者封鎖店**（`NOT EXISTS (… pub_blocked_stores() … store_id = p.store_id)`）：`Nested Loop Anti Join`，內側是 `Index Only Scan using blocked_stores_pkey`（`Index Cond: blocker_buyer_id = $v`，掃一次，100 個封鎖的觀看者 40 列、2,000 個封鎖的 1,000 列），`store_id` 比對是 Join Filter：成本是（回傳的貼文列數 × 觀看者封鎖的店數）次比較，不隨整張 `blocked_stores` 的大小成長。
- **買家作者**（留言與討論區）：兩個方向各是一個 `hashed SubPlan`（`blocked_buyers_pkey`：觀看者封鎖的買家，掃一次；`blocked_buyers_blocked`：封鎖觀看者的買家，掃一次），每列只做雜湊比對，而且 `actor_kind <> 'BUYER'` 的列（店家作者）不做比對（`OR` 短路）。被排除的列在列表的 `Filter` 裡，所以過濾在 `LIMIT` 之前（SD15）。
- **回覆的父列條件**（`Admits`）：以請求帶的父 ID 做一次 `InitPlan`（`Index Scan using comments_pkey on comments pc`，`loops=1`），外層是 `One-Time Filter`。單列的 `ParentAdmitted`（`ReadableCommentPost` 回覆那一列）是對 `comments_pkey` 的一次探測，比值 1.04（run 1）與 1.32（run 2）。
- 匿名語句沒有任何封鎖節點（golden 檔與 `TestAnonymousReadsCarryNoBlockClause` 擋）。
- 買家封鎖表的內聯與索引另由 `buyer` 的 `TestPubBlocksIsInlinedAndUsesTheIndexes` 在每張約 29,000 列的表上驗證（planner 對 `(blocker_buyer_id, store_id)` 兩個等號條件可能選主鍵或反向索引，兩者都是 Index Only Scan）。

## 已知限制與重新評估的條件

- **退步的判斷與觸發**：觀看者變體的 p95 超過同一輪匿名對照 p95 的 **3 倍**，而且觀看者 p95 高於 10 ms；或封鎖表在計畫裡出現 Seq Scan／Function Scan（`measure.go` 以 exit 5、exit 4 結束）。3 倍的理由：量到的 p95 比值最大 1.60（2,000 個封鎖）、小數字的 p95 雜訊約 ±40%，3 倍留出約 2 倍的餘裕，仍能抓到真正的計畫退步（已知的對照：把固定代碼寫成 bind 參數使 partial index 用不上，量測 59 ms 對 0.7 ms，約 80 倍）；10 ms 的下限是避免對毫秒以下的雜訊比倍數。處置的候選是把 `NOT EXISTS` 的外側改成 planner 能選 Hash Anti Join 的形狀，或加一個觀看者封鎖的批次入口；沒有量到需要，這一步不做。
- 店家封鎖的成本與觀看者封鎖的店數成正比（上面的 Join Filter）：量到 2,000 個封鎖（其中 1,000 家店）p95 比值仍在 1.6 以下。觸發：產品允許單一買家封鎖數千個以上的店，或上面的比值條件成立。
- 這份資料的觀看者沒有封鎖最熱門的店；封鎖了熱門店的觀看者在動態牆上要多掃被排除的貼文才湊得滿一頁（與被排除的貼文占的比例有關），這一步沒有量這個情境。
