# A7-1a 效能量測與 pg_trgm spike（設計書 §9.3、§9.4）

量測日期 2026-10-23（初量），同日 fix round 2 重量（keyset 改寫與熱門標籤窗口上限 30 天之後），同日 A7-1a DR-B1／B2（帶可見性規則的 26 支 query 改用 go-jet、計數改成旗標加 reevaluate）之後再重量全部情境，下面的表是最後這一次。結論先講：

- **R3b（P1b、DR-A1）之後的結論**：(1) `ListHashtagPosts` 的排序鍵複製進 `post_hashtags.published_at`（索引 `post_hashtags_feed`），游標條件由 join 上的 Filter 變成 `Index Cond: ((name = $2) AND (ROW(published_at, post_id) < ROW($3, $4)))`，熱門標籤第一頁 p50 78.41 → 1.18 ms、p95 110.08 → 5.06 ms，游標第 100 篇 p95 109.08 → 2.56 ms（前一輪的資料是另一個隨機種子，標籤列數 199,561 對 199,546、最熱門標籤 11,342 對 11,396 篇，差距遠小於這個改善）。(2) 全量對帳改成每 500 列一批的集合語句：這份 10⁵ 貼文／7×10⁵ 留言的資料上 `social-recount` 整體 307 秒（第二次，沒有漂移；第一次 428 秒，修正了 2,066 個漂移，同時機器 load average 25–28），舊的一個對象一個 transaction 的做法在同一份資料上跑了 22 分鐘還沒結束（load average 20–32，被中止，沒有完成的數字）。兩次的時間都包含旗標修正（`reevaluateAll`，每 200 則留言呼叫一次 plpgsql 函式，沒有改），以及留言與討論區留言的批次；一批 500 列的貼文重算 p50 21.70 ms（熱門貼文所在的一批 72.76 ms）。
- **其餘情境沒有退步**：沒有 Function Scan，計畫形狀沒變（`pub_exposable_stores` 照樣被內聯成索引探測；R4 起讀取不再 join `buyer.pub_inactive_buyers`，見下一節）；keyset 的列比較都是 `Index Cond`，頂層留言與討論區的容器規則在計畫裡是 `InitPlan` 加 `One-Time Filter`，固定代碼 `status = 'PUBLISHED'` 是常數所以 partial index 在 generic plan 下可用（`TestRuleCodesAreInlineConstants` 擋）。
- **這一輪機器很吵**：量測期間 1 分鐘 load average 在 6–13（別的 session 的行程，沒有獨佔）。沒有改過的 `SuggestHashtags` 情境 p50 45–92 ms，p95 在兩次量測之間 95–423 ms 跳動（4 個情境在完整量測超過 300 ms，單獨重跑 2 個），**超過門檻的是雜訊造成的長尾，不是 query 的改變**：它們的計畫與 p50 和前一輪相同。判斷這些情境要看 p50 與計畫形狀，不要看單次的 p95。

- **keyset 的游標條件改成單一列比較**（`(published_at, id) < ($, $)`，第一頁由 Go 給方向的極值游標，沒有 `NOT has_cursor OR`）：在 generic plan 下，動態牆、店家頁、店主清單、留言與回覆、討論區、處分紀錄**都是 `Index Cond: (ROW(…, id) < ROW($, $))`**；動態牆游標在 90% 與第一頁同一量級（fix round 2 的量測：改前 54.78 ms 對 2.70 ms；改後 4.29 ms 對 10.28 ms，同一次的雜訊比差距大；DR-B1／B2 之後 2.87 ms 對 5.01 ms）。唯一的例外是計畫由集合驅動的 `ListHiddenContentOf`（該買家自己的內容）：游標是 join 上的 Filter，成本由集合大小決定，第一頁與深頁相同（見「keyset 的形狀」）；`ListHashtagPosts` 在 R3b 改成 Index Cond。
- **`TrendingHashtags` 窗口上限改成 30 天**（預設也是 30 天，超過是 InvalidInput）：近 30 天 p95 在三次完整量測是 45.38／69.02／91.80 ms（初量 43.53 ms），遠低於 300 ms；原本超標的 365 天窗口已不是合法請求，不再量。
- 初量的結論仍成立：每個計畫都沒有 `Function Scan`（`seller.pub_exposable_stores` 被內聯；初量時的 `buyer.pub_inactive_buyers` 在 R4 起不在讀取裡）；包含搜尋不用 pg_trgm（見「spike」）。
- **量測雜訊與可比性**：fix round 2 重量時這台機器的 load average 在 11–54，吃 CPU 的情境 p95 在三次完整量測之間差到 3–4 倍。DR-B1／B2 這一輪量測前後的 1 分鐘 load average 是 9.1 與 5.2（機器上仍有別的 session 的行程，沒有獨佔），比 fix round 2 安靜得多，而且播種的隨機資料不同（那一輪的 `seed.sql` 沒有固定亂數種子，R3b 起固定：hashtag 列數這次 199,561，前次 397,437；留言 800,000，前次 700,000），所以**這一輪的絕對值（尤其 hashtag、重算、TopPosts）低於前一輪，不能拿來說 query 改寫變快**；沒有在同樣的負載與同一份資料上做前後對照。能說的是：計畫形狀沒變、全部情境 ≤ 118 ms。

## 環境

| 項目 | 值 |
|---|---|
| 機器 | MacBook Pro（Apple M1 Pro、8 核、16 GB），Docker Desktop VM 分到 8 CPU、約 6 GB 記憶體；DR-B1／B2 這一輪量測前後的 1 分鐘 load average 是 9.1 與 5.2，機器上有別的 session 的行程（沒有獨佔） |
| Postgres | 一次性的 container `postgres:18-alpine`（和 `docker-compose-infra.yml` 與 `platform/testdb` 同一個 image），PostgreSQL 18.6、aarch64-musl；`shared_buffers` 128 MB、`work_mem` 4 MB、`effective_cache_size` 4 GB、`max_parallel_workers_per_gather` 2、`jit` on，全是 image 的預設值，沒有調過 |
| Locale | `en_US.utf8`（libc provider），和單體開發用的 `hawkr-postgres` 一樣 |
| Schema | 單體的 `hawkr migrate` 套用全部 migration 的結果（seller、buyer、social 都是真的 schema） |
| 資料量 | 店家 10,000（可曝光 9,406）、買家 50,000（2% 刪除中）、貼文 100,000（已發布且未刪 80,532）、hashtag 列 199,546（5,000 個不同標籤；`published_at` 複本與索引 `post_hashtags_feed` 13 MB，表加索引共 84 MB）、留言 700,000（含回覆；計入的旗標為真的列數見種子結尾，R4 起作者在 `inactive_authors`（1,000 位）的買家內容旗標為假）、隱藏紀錄 7,051、貼文讚 999,653、留言讚 279,710、討論區留言 100,000、處分 20,000；database 850 MB |
| 分佈 | 店家與貼文偏斜（前 1% 的店家各有上千篇貼文）、熱門貼文最多 3,949 則頂層留言（含回覆共 7,877 則）、最熱門的標籤出現在 11,396 篇貼文；5% 的貼文軟刪、8% 下架、2% 草稿；3% 的店家沒有核准的 KYC、2% 有生效中的停權、1% 正在隨帳號刪除 |
| 量法 | 單一連線、同機（不含網路）；prepared statement，`plan_cache_mode = force_generic_plan`；暖機 10 次，計時 200 次，回報 p50／p95／max；page_size 50；go-jet 的 query 直接呼叫 `internal/modules/social/internal/reads` 的建構函式，sqlc 的 query 讀 `internal/modules/social/db/queries/*.sql`，不手抄 |
| 沒量的 | 並發寫入負載下的讀取（單一連線、沒有別的寫入）；10⁵ 以外的規模（沒有跑 5× 的資料）；正式環境的 Postgres 設定與 Docker 以外的主機；`social_views`／`post_views` 的寫入路徑 |

資料是合成的，分佈是我設的（見 `seed.sql` 的註解），不是正式流量的樣子；`seed.sql` 一開始 `setseed(0.42)`，重跑得到相同的列數與分佈（uuid 不受 `setseed` 影響，每次不同，不影響分佈；本檔以後的前後對照用同一份種子）。種子直接寫 `counted` 旗標（與 stats 同一個定義）並關掉 trigger。

## 重現

```sh
# 在 backend/。DOCKER_CONTEXT 與 Homebrew postgres 佔 5432 的處理見 AGENTS.local.md；這裡用一個非 5432 的 port。
docker --context desktop-linux run -d --name a7-1a-perf -e POSTGRES_PASSWORD=perf -e POSTGRES_DB=perf -p 127.0.0.1:55477:5432 postgres:18-alpine
GOWORK=off go build -o /tmp/hawkr-perf ./cmd/hawkr
env -i PATH="$PATH" HOME="$HOME" DATABASE_URL='postgres://postgres:perf@127.0.0.1:55477/perf?sslmode=disable' /tmp/hawkr-perf migrate
docker --context desktop-linux exec -i a7-1a-perf psql -U postgres -d perf < docs/plans/2026-10-18-a7-social/step-1a-perf/seed.sql   # 約 5 分鐘，結尾 VACUUM (ANALYZE)
GOWORK=off go run ./internal/modules/social/perf/measure.go \
  -dsn 'postgres://postgres:perf@127.0.0.1:55477/perf?sslmode=disable' -reps 200 -out /tmp/a7-1a-perf-out
# -only '建議|熱門標籤' 只跑標籤符合的情境；超過 300 ms 的情境讓指令以 2 結束
docker --context desktop-linux exec -i a7-1a-perf psql -U postgres -d perf -v ON_ERROR_STOP=1 < docs/plans/2026-10-18-a7-social/step-1a-perf/spike-trgm.sql
```

`seed.sql` 的參數：`-v stores=… -v buyers=… -v posts=… -v comments_per_post=… -v likes_per_post=…`。用完 `docker rm -f a7-1a-perf`。

## A7-1a R4（D-sync）：讀取改看旗標之後重量（2026-10-23）

改動：買家刪帳的事實在 social 有複本 `inactive_authors`，「計入」只依 social 自己的表，讀取直接用 `comments.counted`／`forum_entries.counted`（不再 join `buyer.pub_inactive_buyers`，也不再每列推導隱藏與刪除）；`seed.sql` 補 `inactive_authors`（與 `buyer.buyers` 不營運中的 1,000 位一致）、旗標與 stats 同為「旗標之和」；列數與前一輪相同（貼文 100,000、hashtag 列 199,546、留言 700,000、討論區 100,000），`setseed(0.42)`。

**重量抓到一個退步，已修**：第一次重量（load average 12.4 → 11.3）頂層留言與回覆的 p50 從 0.6–1.7 ms 變成 58–196 ms（熱門貼文頂層第一頁 195.62 ms、店主讀熱門貼文頂層 p95 323.18 ms），計畫是 `Seq Scan on comments`（`Filter: (counted AND (parent_id = $1) AND …)`，`Rows Removed by Filter: 699997`）。原因：`comments_replies`、`forum_entries_replies` 是 `WHERE deleted_at IS NULL` 的 partial index，讀取改看 `counted` 之後 planner 無法從 `counted` 證明 `deleted_at IS NULL`，索引用不上。修法：兩個索引的述詞改成 `WHERE counted`（`schema.sql`，migration 重新壓平），其餘不動。修後留言與討論區的 12 個情境都回到 Index Cond、沒有 Seq Scan：單獨重跑（load average 17 → 22）熱門貼文頂層第一頁 p50 4.70／p95 15.62 ms、回覆列表 p50 1.14／p95 5.74 ms、店主讀回覆 p50 0.77 ms。沒有加別的 partial index：EXPLAIN 沒有顯示需要（動態牆、店家頁、hashtag 的述詞沒有碰旗標，計畫沒變）。

**計畫形狀的改變**：只有留言與討論區的列表：`c.counted`（單一布林）取代「沒有刪除、沒有未還原的隱藏、買家作者未停用」三個子條件，頂層留言的 `visible` 旗標在 `LATERAL` 裡直接是欄位，「底下有沒有可見回覆」的 `EXISTS` 變成對 `comments_replies`（`WHERE counted`）的索引探測，不再每列各查一次 `hides` 與買家。佔位項目的 state 才讀 `inactive_authors`（只對輸出的頁內列求值）。重算語句不再 join 買家，批次重算 p50 熱門 72.76 → 79.82 ms（雜訊範圍；p95 261 → 148 ms）、一般 21.70 → 37.99 ms（最後一輪 load 78，第一輪 load 12 時是 43.86／13.07 ms，比前一輪快）。

**環境與可比性**：完整量測共三次：第一次（索引未修，load average 12.4 → 11.3）、第二次（load average 42 → 15，另有一個量測的 `DROP INDEX` 因為和 `VACUUM` 同一個 `-c` 而整批回滾，索引其實沒改成，所以第二次仍是 Seq Scan）、第三次（索引改好後的完整量測，下面的表；load average 20.7 → **78.1**，別的 session 在跑，8 核）。第三次的絕對值被 load 拉高：沒有碰旗標的情境（動態牆、店家頁）p95 也比第一次高 2–3 倍，所以判斷以 p50 與計畫形狀為準。四個超過 300 ms 的列都是診斷用的「可曝光店家先算一份」變體（不是 query 檔本身，前一輪也有超標），它們在 load 20 以下的第一次量測是 p95 77–239 ms。

## 結果（p95 ≤ 300 ms @ page_size 50；A7-1a R4（D-sync）之後重量，索引改為 `WHERE counted` 之後）

| 情境 | p50 ms | p95 ms | max ms | Function Scan | 計畫的 Index Cond 之外的 Seq Scan | 列比較是 Index Cond |
|---|---|---|---|---|---|---|
| feed 第一頁 | 1.89 | 6.59 | 22.00 | false | 無 | 是 |
| feed keyset 游標在 50% | 2.44 | 7.29 | 27.90 | false | 無 | 是 |
| feed keyset 游標在 90% | 1.88 | 9.42 | 19.73 | false | 無 | 是 |
| 店家頁 熱門店 第一頁 | 1.53 | 5.83 | 18.54 | false | 無 | 是 |
| 店家頁 熱門店 游標第 200 篇 | 1.13 | 4.17 | 13.69 | false | 無 | 是 |
| 店家頁 冷門店 第一頁 | 0.83 | 2.12 | 4.54 | false | 無 | 是 |
| hashtag 貼文 熱門標籤 第一頁 | 2.00 | 5.10 | 16.19 | false | 無 | 是 |
| hashtag 貼文 熱門標籤 游標第 100 篇 | 3.14 | 10.94 | 18.00 | false | 無 | 是 |
| hashtag 貼文 冷門標籤 | 1.27 | 16.84 | 79.63 | false | 無 | 是 |
| hashtag 建議 中文前綴 1 字 | 81.50 | 209.48 | 501.14 | false | post_hashtags | — |
| hashtag 建議 英數前綴 1 字母 | 58.74 | 137.30 | 247.03 | false | post_hashtags | — |
| hashtag 建議 包含 中文 1 字 | 89.39 | 193.26 | 348.25 | false | post_hashtags | — |
| hashtag 建議 包含 中文 2 字 | 109.12 | 190.87 | 286.67 | false | post_hashtags | — |
| hashtag 建議 包含 英數 2 字 | 89.51 | 289.20 | 665.85 | false | post_hashtags | — |
| hashtag 熱門 近 30 天 | 44.26 | 209.21 | 590.50 | false | 無 | — |
| 計數 單篇貼文（含 stats）熱門 | 1.20 | 7.33 | 43.29 | false | 無 | — |
| 計數 熱門貼文所在的一批重算（RecountPostStatsBatch，500 列） | 79.82 | 148.56 | 358.63 | false | post_stats、post_shares | — |
| 計數 一般的一批重算（RecountPostStatsBatch，500 列） | 37.99 | 84.68 | 147.72 | false | post_stats、post_shares | — |
| 計數 每日對帳的分頁（PostStatsPage） | 3.25 | 10.42 | 18.40 | false | 無 | — |
| 店主的貼文 熱門店第一頁 | 4.91 | 30.57 | 59.66 | false | 無 | 是 |
| 留言 熱門貼文頂層 第一頁（含佔位 EXISTS） | 3.30 | 9.12 | 36.44 | false | 無 | 是 |
| 留言 熱門貼文頂層 游標第 200 則 | 7.47 | 19.97 | 41.35 | false | 無 | 是 |
| 留言 一般貼文頂層 第一頁 | 5.28 | 19.64 | 49.92 | false | 無 | 是 |
| 留言 店主讀熱門貼文頂層 | 2.41 | 9.44 | 84.56 | false | 無 | 是 |
| 留言 回覆列表 | 3.51 | 11.22 | 25.93 | false | 無 | 是 |
| 留言 回覆列表 游標在中段 | 3.19 | 10.64 | 36.56 | false | 無 | 是 |
| 留言 店主讀回覆 第一頁 | 4.25 | 13.43 | 19.23 | false | 無 | 是 |
| 留言 店主讀回覆 游標在中段 | 3.85 | 18.90 | 200.89 | false | 無 | 是 |
| 店主的貼文 熱門店游標第 200 篇 | 4.75 | 20.14 | 45.82 | false | 無 | 是 |
| 討論區 熱門店頂層 游標在中段 | 3.87 | 10.89 | 19.87 | false | 無 | 是 |
| 討論區 回覆列表 游標在中段 | 1.59 | 12.07 | 45.31 | false | 無 | 是 |
| 處分紀錄 第一頁 | 1.74 | 7.04 | 37.86 | false | 無 | 是 |
| 處分紀錄 游標第 5 筆 | 1.62 | 8.13 | 31.54 | false | 無 | 是 |
| 被隱藏的內容 第一頁 | 3.38 | 9.87 | 48.92 | false | 無 | 集合驅動（Filter） |
| 被隱藏的內容 游標第 2 則 | 2.77 | 5.81 | 10.65 | false | 無 | 集合驅動（Filter） |
| 討論區 熱門店頂層 第一頁（含佔位 EXISTS） | 1.61 | 5.74 | 16.29 | false | 無 | 是 |
| 討論區 回覆列表 | 0.80 | 2.97 | 11.42 | false | 無 | 是 |
| 熱門貼文 TopPosts（最新 30 篇取瀏覽最高 5 篇） | 55.71 | 112.89 | 231.17 | false | post_stats | — |
| 歸屬 GetPostOwner（moderation 驗證） | 1.76 | 12.74 | 46.14 | false | 無 | — |
| 歸屬 ReadableCommentPost（留言按讚） | 0.85 | 2.73 | 4.91 | false | 無 | — |
| hashtag 貼文 熱門標籤 第一頁（可曝光店家先算一份） | 353.01 | 1537.45 **超過門檻** | 3092.02 | false | stores、kyc_applications、payuni_registrations | 是 |
| hashtag 建議 中文前綴 1 字（可曝光店家先算一份） | 196.71 | 935.90 **超過門檻** | 1711.62 | false | stores、kyc_applications、payuni_registrations、post_hashtags | — |
| hashtag 建議 包含 中文 1 字（可曝光店家先算一份） | 143.65 | 1818.36 **超過門檻** | 3485.51 | false | stores、kyc_applications、payuni_registrations、post_hashtags | — |
| hashtag 熱門 近 30 天（可曝光店家先算一份） | 79.89 | 475.02 **超過門檻** | 1793.56 | false | stores、kyc_applications、payuni_registrations | — |

- 計畫看的是 `explain-selected.txt`（13 個代表性的 `EXPLAIN (ANALYZE, BUFFERS)`；全部情境的計畫是 `perf/measure.go -out` 的輸出，這裡沒有收，192 KB）。每個 join 的內側是 Index Scan／Index Only Scan，`pub_exposable_stores` 展開成 stores、kyc_applications、payuni_registrations 的索引探測，`measures` 的 `NOT EXISTS` 是 Anti Join。
- 建議 query 的 Seq Scan 是 `LIKE` 對 `post_hashtags.name` 的掃描：預設的 btree 在 libc locale 下服務不了 `LIKE 'x%'`，包含搜尋本來就不用索引。掃描本身只要約 16 ms（見 spike），其餘是對每個比對到的列探測店家可曝光。
- 計數的重算是批次語句（`RecountPostStatsBatch`、`RecountCommentStatsBatch`、`RecountForumEntryStatsBatch`，500 列一批，分組 COUNT 加 `LEFT JOIN`）：計畫對範圍內的 `post_stats`、`post_shares`、`comments`（旗標為真）做範圍掃描，這是全量對帳本來就要讀的資料，不是請求路徑上的成本；舊的單一貼文版本（熱門貼文 p95 38.66 ms）已刪除。`hides` 是 Index Only Scan。
- 留言與討論區的「佔位的 EXISTS 分支」（頂層不可見而底下有可見回覆）在這些資料量下沒有成為成本：熱門貼文頂層第一頁 p95 1.86 ms、討論區 2.33 ms。

## `TrendingHashtags` 的窗口（P2，主 session 2026-10-23 決定）

初量時取近 365 天 p95 392 ms（近 30 天 44 ms），成本在每個貼文各探測一次店家可曝光，加索引不會降（試過 `text_pattern_ops`、`gin_trgm_ops`、`posts (published_at) INCLUDE …`：整組更慢，沒有留 migration）。舊 social 允許到 1 年，但沒有 client 用超過 30 天（website 頁尾用預設 1 個月、搜尋用 7 天；`rg -n 'tags/trending' website`），所以窗口上限降到 30 天、預設 30 天，超過的請求是 InvalidInput（`model.MaxTrendingWindow`；`TestTrendingHashtagsRejectsWindowsAboveTheMaximum`、`TestTrendingHashtagsWindowIsCappedAtThirtyDays`）。重新評估的觸發：產品需要更長的窗口時改成預先算好的週期 rollup（讀取模型，需要先寫必要條件）。

## keyset 的形狀（P1）

初量發現 `NOT has_cursor OR (row) < (...)` 在 generic plan 下是 `Filter`（`Rows Removed by Filter: 43111`，游標在 50%），成本與游標深度成正比（動態牆 90% 深度 p95 54.78 ms、第一頁 2.70 ms）。fix round 2 把 13 支 keyset query 全部改成單一列比較，第一頁由 `model.DescStart()`／`AscStart()` 給方向的極值游標（`9999-12-31 23:59:59.999999` 與全 f 的 UUID；由小到大則是 `0001-01-01` 與全 0 的 UUID），所有頁走同一個 prepared statement。

| 情境（本次量測） | 改前 p95 | 改後 p95 |
|---|---|---|
| feed 第一頁 | 2.70 ms | 10.28 ms（雜訊，同次 p50 2.23 ms） |
| feed 游標在 50% | 32.65 ms | 5.75 ms |
| feed 游標在 90% | 54.78 ms | 4.29 ms |

計畫（`EXPLAIN` 在 `force_generic_plan` 下）：`measure.go` 對每個有游標參數的 query 檢查計畫有沒有 `Index Cond: … ROW(a, id) < ROW($n, $m)`，沒有而且不在 `setDriven` 的例外清單就以 exit 3 結束；R3b 之後的量測 44 個情境（含 4 個『可曝光店家先算一份』變體）中，有游標參數的都是 Index Cond，例外只有：

| query | 為什麼游標是 Filter | 成本 |
|---|---|---|
| `ListHiddenContentOf` | 計畫由該買家自己的留言（`comments_actor`）驅動，再到 `hides`；排序鍵在 `hides` | 成本由「該買家的內容數」決定（有界），與游標深度無關：第一頁 3.42 ms、第 2 則 1.59 ms |

`ListHashtagPosts` 原本也是例外（計畫由該標籤的貼文集合驅動，排序鍵不在驅動的索引上，p50 約 78–235 ms）；R3b（P1b）把貼文的 `published_at` 複製進 `post_hashtags`（trigger 維護，貼文第一次發布時寫入、發布後不變），索引 `post_hashtags_feed (name, published_at DESC, post_id DESC) WHERE published_at IS NOT NULL`，計畫變成 `Index Only Scan using post_hashtags_feed` 加 `posts_pkey` 的探測，`Index Cond: ((name = $2) AND (ROW(published_at, post_id) < ROW($3, $4)))`（`explain-hashtag-posts.txt`）。`measure.go` 的 `setDriven` 只剩 `ListHiddenContentOf`，所以 `ListHashtagPosts` 再退回 Filter 時量測會以 exit 3 結束。

`ListSanctions` 在空表時計畫是 Seq Scan（初量的 seed 沒有處分紀錄），所以 `seed.sql` 補了 20,000 筆處分（前 2,000 位買家各 10 筆）；有資料時走 `sanctions_buyer`，游標是 Index Cond。

## 全量對帳的耗時（DR-A1）

指令：`DATABASE_URL=… hawkr social-recount`（和每日對帳 job 同一條路：`stats.RecountAll`），在上面那份播種的 database 上量：

| 次 | 做的事 | 時間 | 機器 |
|---|---|---|---|
| 舊做法（一個對象一個 transaction，A7-1a 初版） | 旗標修正＋逐個貼文、留言、討論區留言重算 | 22 分鐘還沒結束，被中止（沒有完成的數字） | load average 20–32 |
| 第 1 次（新做法） | 旗標修正＋批次重算；修正 2,066 個漂移（種子直接寫旗標與 stats，與述詞不盡相同） | 428 秒 | load average 25–28 |
| 第 2 次（新做法，沒有漂移） | 同上 | 307 秒 | load average 7–12 |

時間花在哪裡：留言 700,000 則，所以留言的旗標修正（`reevaluate_comments`，每 200 則一次）與 `comment_stats` 的 1,400 批各佔大宗，貼文只有 200 批；一批 500 列的貼文重算 p50 21.70 ms（含最熱門貼文的一批 72.76 ms）。每日對帳是背景 job（`Timeout` 一小時），寫入者最多等一批（500 列）的時間。這個數字是這份資料、這台吵雜的機器上的量測，不是正式環境的預測；資料增長 5 倍時預期線性增長（每批成本固定、批數成正比）——這是推論，沒有量過。

## spike：hashtag 的「包含」搜尋與 pg_trgm（§9.4）

指令：`spike-trgm.sql`（結果原文在 `spike-results.txt`）。locale 是 `en_US.utf8`、libc provider。

1. **正確性**：1–4 個漢字、前綴／中段／結尾共 332 個查詢字串，`name LIKE '%q%'` 與 `strpos(name, q) > 0` 的列數全部一致。
2. **pg_trgm 對漢字**：這個 locale 下漢字被當成字詞字元切 trigram（`show_trgm('烘焙茶')` 有 4 個 trigram），所以不是「不可用」。限制是結構性的：trigram 要 3 個字元，**1–2 個漢字的 `LIKE '%q%'` 抽不出 trigram，即使有 GIN 索引也是 Seq Scan**（16 ms）；3 個漢字 5.1 ms、4 個漢字 0.7 ms 走 Bitmap Index Scan。
3. **不用索引的耗時**（397,437 列）：謂詞本身 14–17 ms（Parallel Seq Scan）；完整的建議 query（含對每個比對到的列探測店家可曝光）p95 85–146 ms。

結論：**現在不需要 pg_trgm**（計畫的條件是「不用索引太慢」，沒有）。停止條件（CJK trigram 不可用而包含搜尋必要）不成立。觸發條件：包含搜尋的完整 query p95 超過 300 ms（成本與比對到的列數成正比；現在最壞 146 ms 的情境比對 21,000 列），這時 3 個字元以上的查詢加 GIN 索引會有幫助，1–2 個漢字的查詢不會（這也是中文最常見的長度），要靠限制最短長度或預先算好。加 extension 的做法照主計畫 `2026-09-24-go-modular-monolith.md` 第 79–82 行。

限制：這是 Alpine（musl）的 Postgres；pg_trgm 對漢字的切分看 libc 的 `iswalnum`，換成 glibc 的正式環境要重跑 `spike-trgm.sql` 的第 1 節。

## 檔案

| 檔案 | 用途 |
|---|---|
| `seed.sql` | 播種（參數化、偏斜分佈、結尾 `VACUUM (ANALYZE)`） |
| `backend/internal/modules/social/perf/measure.go` | 量測：go-jet 的 query 呼叫 `reads` 的建構函式、sqlc 的 query 讀原文、prepared statement、generic plan、200 次 p95、`EXPLAIN (ANALYZE, BUFFERS)`；`build ignore`，用 `go run`；放在 social 底下才能 import 模組的 internal 套件 |
| `spike-trgm.sql` | §9.4 的 spike |
| `explain-selected.txt`、`spike-results.txt` | 這次執行的輸出 |
| `explain-hashtag-posts.txt` | R3b 之後熱門標籤第一頁的 `EXPLAIN (ANALYZE, BUFFERS)`（generic plan，`Index Cond` 含游標的列比較） |

## R3c：SuggestHashtags 的 4 個情境重跑（A7-1a DR-A5 附帶）

R3b 的完整量測裡 `SuggestHashtags` 有 p95 超過 300 ms 的情境。R3c 在同一份種子（`setseed(0.42)`）、全新建的 container 上只重跑 `-only '建議'`（reps 200，generic plan），共 4 次；表列 4 個情境（`-only '建議'` 另外跑了英數 2 字與 2 個「可曝光店家先算一份」變體，趨勢相同，沒有列出），單位 ms，格式 p50 / p95。每次的 1 分鐘 load average 取量測前→後。

| 次 | load average（前→後） | 中文前綴 1 字 | 英數前綴 1 字母 | 包含 中文 1 字 | 包含 中文 2 字 |
|---|---|---|---|---|---|
| 1 | 72.6 → 32.2 | 111 / 580 | 96 / 365 | 106 / 477 | 129 / 647 |
| 2 | 32.2 → 41.1 | 94 / 263 | 70 / 258 | 96 / 379 | 100 / 353 |
| 3 | 41.1 → 17.9 | 65 / 141 | 80 / 487 | 76 / 192 | 70 / 112 |
| 4（`VACUUM FULL post_hashtags` 之後） | 17.8 → 18.3 | 75 / 227 | 53 / 115 | 102 / 615 | 90 / 326 |

結論：**是雜訊，不是退步**。(1) 同一個情境的 p95 在 4 次之間差 4–6 倍（中文前綴 141–580 ms），p50 也跟 load 同向（65–111 ms）；這台機器 load average 17–72（8 核，別的 session 在跑），超過門檻的次數跟 load 一致。(2) 計畫形狀沒變：與 R3a 的 `explain-selected.txt`「中文前綴 1 字」逐節點相同（Limit → Sort → GroupAggregate → Sort → Nested Loop Semi Join → Hash Join〔posts_store_feed Index Only Scan ＋ post_hashtags Seq Scan〕→ Nested Loop Anti Join），query 本身 R3a 到 R3b 沒有改。(3) 唯一的實質差異是 `post_hashtags` 的 Seq Scan 讀的頁數：R3a 是 1,620 頁，R3b 的種子是 3,307 頁，`VACUUM FULL` 後 1,803 頁。差異有兩個來源：P1b 加了 `published_at`（每列 +8 bytes，約 +11% 頁數，已量到）；R3b 的 `seed.sql` 先 INSERT 再 `UPDATE … SET published_at`，每列更新一次讓堆積膨脹成兩倍（`n_tup_upd` 185,414），這是種子的做法造成的，正式流量的標籤列是 INSERT 時 trigger 帶入值，不會有這個膨脹。估計成本 22,015 → 22,122（+0.5%）。(4) 這個 query 的掃描是 `LIKE` 對 `post_hashtags.name` 的 Seq Scan（見上面的「建議 query 的 Seq Scan」），p50 在 50–110 ms 與 R3a 的 45–92 ms 同一個量級；要讓它穩定在 300 ms 以下，需要的是索引或查詢的改法（pg_trgm 的 spike 結論是 10⁵ 規模下不需要），不是 R3b 引入的問題，這一輪沒有改 query。
