# 社群互動計數：業界做法與從零設計

證據標記：【量】＝本次讀過原始碼或官方文件／論文原文；【二手】＝只找到第三方轉述；「未查證」＝憑記憶。
原始碼是 2026-10-23 各專案 main 分支的快照（研究時下載在 session 暫存目錄，未保存）。

## 先講結論

1. 七種計數**全部存起來**。讚、留言、回覆、討論區回覆在**寫入來源列的同一個 transaction** 內維護，做法是 PostgreSQL trigger 依「是否計入」的舊值與新值差異來加減（Lemmy 的模式）。瀏覽與分享先寫一筆去重的事件列，事件列真的新增時才在同一個 transaction 內加一（Discourse 的模式）。另外每天跑一次 River 對帳 job 重算並記錄漂移，再加一個手動全量重算指令。
2. 語意是「一般公開觀看者在父項可見時看得到幾則」。平台隱藏、下架、作者刪除、刪帳者的內容都不計入；草稿上不會有留言。「是否計入」只能由該列自己的欄位決定。可撤銷的上層狀態（店家停權或關閉）不改計數。不可撤銷的上層狀態（買家刪帳）在刪帳流程中寫到每一列，觸發計數變動。
3. 貼文的留言數含回覆（等於公開可見的總則數），每則頂層留言另外顯示自己的回覆數。
4. 所有觀看者看到同一個數字，封鎖、作者、店主都不調整。

---

## 前提：PostgreSQL 的 count(*) 為什麼不是免費的

| 事實 | 來源 |
|---|---|
| MVCC 讓不同 transaction 看到不同版本，「PostgreSQL must walk through all rows to determine visibility」，加了 WHERE 才能用 index 縮小範圍 | 【量】[PG wiki: Slow Counting](https://wiki.postgresql.org/wiki/Slow_Counting) |
| index-only scan 要看 visibility map：page 的 all-visible bit 有設才不回 heap，沒設就要回 heap，「no performance advantage is gained over a standard index scan」；最近改過的列會讓 bit 清掉 | 【量】[PG docs 11.9 Index-Only Scans](https://www.postgresql.org/docs/current/indexes-index-only-scans.html) |
| all-visible bit「only set by vacuum, but are cleared by any data-modifying operations on a page」 | 【量】[PG docs 66.4 Visibility Map](https://www.postgresql.org/docs/current/storage-vm.html)、[24.1 Routine Vacuuming](https://www.postgresql.org/docs/current/routine-vacuuming.html) |

對我們的意義（推論）：留言表與讚表是寫入熱的表，新留言所在的 page 正好是最常被計數的 page，visibility map 幫不上忙。更關鍵的是，我們的可見性要 join 店家狀態、作者帳號狀態、封鎖表，計數本來就做不成 index-only，每算一次都要走 heap 加 join。以 10⁵ 貼文的規模，一頁 20 則逐則 count 仍在毫秒級（推論，沒有量），所以「讀時算」**在現在的規模可行**。不選它的理由在第 1 題。

---

## 第 1 題：存起來還是讀時算

### 業界做法

| 系統 | 做法 | 來源 | 為什麼（逼出它的條件） |
|---|---|---|---|
| Lemmy | `post.comments`、`comment.child_count`、`score/upvotes` 等存在列上。用**statement-level trigger 加 transition table**，在同一個 transaction 內依 `count_diff`（舊列 −1、新列 +1）加總後更新；`r.is_counted(item) = NOT (deleted OR removed)`。註解寫明多表更新要固定順序（person → comment → post → community → site），避免死結並縮短熱列持鎖時間 | 【量】`crates/diesel_utils/replaceable_schema/triggers.sql`、`utils.sql` | 同樣是 PostgreSQL 單庫、feed 每列都要顯示計數。用 trigger 的原因是隱藏、刪除、批次 ban 等任何寫入路徑都會經過，不會漏掉；2023 年他們也刪過「昂貴又壞掉」的 row-level trigger（migration `2023-07-27-134652_remove-expensive-broken-trigger`），後來改成 statement-level |
| Discourse | `topics.posts_count/reply_count/like_count/views`、`posts.reply_count/like_count` 都存。寫入時用 `UPDATE … SET x = x + 1`（app 程式），`EnsureDbConsistency` **每 12 小時**跑 `reset_all_highest!` 從 posts 重算，只更新有差異的列；刪文時 `reset_highest(topic_id)` 對單一 topic 重算 | 【量】`app/models/topic.rb`、`post.rb`、`app/jobs/scheduled/ensure_db_consistency.rb` | 計數散落在多條 app 路徑，靠定期對帳收斂漂移 |
| Mastodon | `status_stats` 是**獨立的表**（`replies_count/reblogs_count/favourites_count/quotes_count`），在 `after_create_commit` 與 `after_destroy_commit` 增減。讀時 `[值, 0].max` 夾住負數。遠端來的數字另存 `untrusted_*` | 【量】`app/models/status.rb`、`status_stat.rb` | 計數放在 commit 之後、不同 transaction，而且是 read-modify-write，所以會漂移，才需要 clamp。分表是為了不讓熱計數改寫 statuses 主列（推論） |
| Facebook TAO | `assoc_count(id1, atype)` 回傳整條 association list 的大小；「To avoid potentially expensive SELECT COUNT queries, association counts are stored in a separate table」；快取層把 count 當成獨立項目快取。另一方面「we filter every item with privacy checks that take into account the current viewer … each time the content is viewed」 | 【量】[TAO, USENIX ATC'13](https://www.usenix.org/system/files/conference/atc13/atc13-bronson.pdf) §3、§4、§5.2 | 讀遠多於寫（論文說讀占 99.8%），count 要 O(1)。隱私過濾套在**項目**上；count 是原始邊數（count 不依觀看者過濾是推論） |
| GitLab CounterAttribute | 增量先寫 Redis，約 10 分鐘由 `FlushCounterIncrementsWorker` 批次寫回 DB；可選 `returns_current` 讀時加上緩衝值；有 `initiate_refresh!/finalize_refresh` 做重算 | 【量】`app/models/concerns/counter_attribute.rb`；動機見 [issue 24469](https://gitlab.com/gitlab-org/gitlab/-/issues/24469)、[MR 35878](https://gitlab.com/gitlab-org/gitlab/-/merge_requests/35878)【二手摘要】 | `project_statistics` 這種**單一熱列**被大量並行更新，造成鎖競爭與 query timeout。這是規模條件，不是預設做法 |
| Reddit | 存計數；票數有 fuzzing | 未查證 | 防操票、規模 |
| YouTube 瀏覽數 | 先計、再驗證；可能「temporarily slow down, freeze, or change your metric count」，低品質播放會被丟掉 | 【量】[YouTube Help 2991785](https://support.google.com/youtube/answer/2991785) | 防灌量，需要非同步驗證 |
| Discourse 瀏覽數 | 每個 user（或匿名 IP）每天每 topic 算一次：先用 Redis `setnx` 擋，再 `INSERT … WHERE NOT EXISTS` 進 `topic_views`，同一個 transaction 內 `views = views + 1` | 【量】`app/models/topic_view_item.rb` | 去重讓數字有意義；每天一次讓去重表能用日期分區 |

### 在我們條件下的推薦

全部存，理由依權重排：

1. **業界基線一致**：上面每個有原始碼可看的系統都存計數。逼出這個做法的條件是「每個 feed 項目都要顯示計數，讀遠多於寫」，我們的條件一樣，而且是 REAL 的（需求寫明畫面要顯示）。
2. **讀時算會把可見性 join 帶進每一列 feed 的熱路徑**，而且 count 不能建 index，之後若要「依讚數排序／熱門」就做不到。熱門排序目前不在需求內，這是推論出的潛在需求。
3. 只要把「是否計入」限定在列自己的欄位上（見第 2 題），用 trigger 維護就有結構性保證，漂移風險可控。

各計數的維護方式：

| 計數 | 存在哪 | 維護 | 防漂移 |
|---|---|---|---|
| 貼文讚 | `post_stats.like_count` | `post_likes`（PK `(post_id, buyer_id)`）的 INSERT/DELETE trigger，同一個 transaction | trigger 加 `CHECK (like_count >= 0)` 加每日對帳 |
| 貼文留言 | `post_stats.comment_count` | `post_comments` 的 statement-level trigger：依 `is_counted(OLD)`、`is_counted(NEW)` 的差加減（見第 2、3 題） | 同上 |
| 留言讚 | `comment_stats.like_count`，或留言列上的欄位 | `comment_likes` 的 trigger | 同上 |
| 留言回覆 | 頂層留言的 `reply_count` | 回覆列的 trigger | 同上 |
| 討論區回覆 | 討論頂層的 `reply_count` | 同上 | 同上 |
| 瀏覽 | `post_stats.view_count` | 開啟貼文詳情時 `INSERT INTO post_view_marks(post_id, viewer_key, day) ON CONFLICT DO NOTHING`，真的新增時才在同一個 transaction 內 +1；作者本人不計；去重表只保留最近 1–2 天，用 River periodic job 刪舊資料 | 去重表本身就是事件來源，對帳只核對保留期內的資料；之前的累計值不重算（業界的瀏覽數本來就是近似值） |
| 分享 | `post_stats.share_count` | 同瀏覽：client 回報分享動作，每人每貼文每天算一次 | 同瀏覽 |

設計細節：

- **計數放在獨立的 `post_stats` 表，跟 Mastodon 一樣**。必要條件：計數的更新頻率遠高於貼文內容，而且店家編輯貼文（持有 row lock）時不該跟按讚互相等待。如果最後確認貼文列沒有 `FOR UPDATE` 的編輯路徑，也可以把欄位直接放在 post 上，這屬於實作取捨。
- **用 trigger，不放在 app 的各條路徑裡**。理由是批次路徑（刪帳一次改數百則留言、平台批次下架）與未來新增的路徑都自動被涵蓋，Lemmy 就是這樣選的。代價是邏輯在 SQL 裡、測試要打真的 DB。Discourse 和 Mastodon 走 app 路徑，結果需要每 12 小時對帳或 clamp 來補漂移，這正好說明 app 路徑的弱點。
- **鎖順序**：一個 trigger 同時更新多張表時，固定 comment_stats → post_stats 的順序（Lemmy 的註解就是講這件事）。
- **對帳**：River periodic job 每天依來源重算，只更新有差異的列（Discourse `reset_all_highest!` 的寫法），漂移筆數記成 metric 或 log。trigger 正確時漂移應該是 0，所以不是 0 就代表有 bug，不是常態。另外提供一個手動全量重算的指令。
- **測試**：整合測試在每個情境之後斷言「存的值等於重算的值」，把對帳 SQL 直接當 oracle（判定標準）。

### 換做法的觸發條件

- 單一貼文或單一店家的計數列出現持續的鎖等待（`pg_stat_activity` 的 Lock wait 或 p99 寫入延遲上升）：讚改成分片計數列（`post_like_slots(post_id, slot)`，讀時 sum），或改用 GitLab 式的緩衝批次寫入（River 累積之後 flush）。
- 瀏覽寫入占總寫入大宗或造成 bloat：瀏覽改成只寫事件，River 每 N 分鐘彙總；或接受近似值，把增量放在程序內記憶體批次寫入。
- 「是否計入」必須依賴可撤銷又會頻繁變動的上層狀態（例如「被停權買家的留言暫時不計」）：那一種計數改成讀時算，或改成 materialized 的條件欄位。不要硬做 fan-out 更新。

---

## 第 2 題：被隱藏、下架、刪帳、草稿算不算

### 業界做法

| 系統 | 做法 | 來源 | 為什麼 |
|---|---|---|---|
| Lemmy | 貼文留言數與使用者計數**排除** `deleted` 或 `removed`（moderator 移除）；`child_count` 的 trigger **沒有**套 `is_counted`，作者刪除與被移除的子留言仍算在子孫數內 | 【量】`triggers.sql`、`utils.sql` | 貼文計數要反映可讀的內容；子孫數拿來決定要不要顯示「展開」（被刪的留言仍以佔位符留在樹上，推論） |
| Discourse | `posts_count` 只排除 `deleted_at IS NOT NULL` 與非公開的 post type（例如 whisper）；**flag 造成的 hidden 仍算在內**（`hide!` 只調整 UserStat，不動 topic 的計數） | 【量】`topic.rb` `reset_highest`、`post.rb` `hide!` | 被隱藏的文章在串裡仍以可點開的佔位符出現 |
| Mastodon | `replies_count` 只計 `distributable?`（公開或不列出）的回覆，direct 不計；刪除（含管理員刪除）走 destroy 會減 | 【量】`status.rb`（`distributable?` 的確切定義沒讀到，依命名推論） | 不讓私訊回覆從計數洩漏 |
| Reddit | 被移除的留言仍算在留言數內 | 【二手】[reagle.org](https://reagle.org/joseph/2020/reddit-deletions.html) 等轉述 Help Center | 被移除的留言以 [removed] 佔位符留在樹上 |
| YouTube | 待審（held）的留言不公開；計數是否包含不明 | 【量】只確認不公開：[YouTube Help 9483359](https://support.google.com/youtube/answer/9483359)；計數部分未查證 | — |

規則是：**計數跟「一般觀看者在串裡看得到的東西（含佔位符）」一致**。Reddit 和 Discourse 會計入被移除或隱藏的內容，是因為那些內容在樹上仍佔一格；Lemmy 的貼文計數與 Mastodon 排除，是因為那些內容對公眾不存在。

### 在我們條件下的推薦

我們的需求是隱藏與下架後「不公開」，只有作者看得到，不是佔位符，所以採 Lemmy 貼文計數與 Mastodon 的語意：

`is_counted(row) = status = 'published' AND deleted_at IS NULL AND moderation_state = 'visible'`，只看該列自己的欄位。

| 狀態 | 留言數 | 機制 |
|---|---|---|
| 平台隱藏買家留言、下架店家內容 | 不計；撤銷後恢復計入 | 列上的 moderation 欄位變動，trigger 依差異加減 |
| 作者刪除 | 不計 | `deleted_at`，trigger |
| 買家刪帳 | 不計 | **刪帳流程把他的每一則內容寫成刪除**（例如 `deleted_at` 加 `deleted_reason='account_deleted'`），trigger 照常運作。這條路是不可撤銷的，fan-out 只發生一次。他的讚直接刪列（讚不是內容，Mastodon 刪帳時也刪 favourites，未查證） |
| 草稿 | 草稿不能留言，計數自然是 0；草稿貼文本身不出現在公開列表 | 不需要額外處理 |
| 店家停權或關閉 | **計數不變** | 整店不公開，一般人看不到計數。可撤銷，所以不 fan-out；恢復後數字自然正確 |
| 貼文下架或回到草稿 | 底下留言的計數**不變** | 父項不可見時公眾看不到這個數字；店主看到的是凍結的公開語意值 |

**頂層被隱藏或刪除時，底下的回覆怎麼辦**：這是產品決定，需求沒寫。如果頂層不可見時整串回覆都不公開，貼文留言數必須連回覆一起扣：頂層狀態翻轉時 `post.comment_count ±= 1 + 該頂層的 reply_count`，回覆翻轉時只有在頂層計入的情況下才動 post。`reply_count` 本身永遠只計可見的回覆，不管頂層的狀態。如果改成 Reddit 或 Lemmy 那樣保留佔位符、回覆照常顯示，就只扣頂層那 1 則。兩種都能用 trigger 做，**需要決策者決定**。

### 換做法的觸發條件

- 產品改成被隱藏或刪除的內容以佔位符留在樹上：回覆數改成 Lemmy `child_count` 的語意（含佔位符），貼文留言數仍只計可讀的內容。
- 新增可撤銷的作者層狀態（例如買家停權時暫時隱藏他的內容）：不要納入計數（照店家停權處理）；如果一定要納入，就把狀態寫到每一列，並接受 fan-out 的成本。

---

## 第 3 題：留言數含不含回覆

### 業界做法

| 系統 | 做法 | 來源 | 為什麼 |
|---|---|---|---|
| Lemmy | `post.comments` 計整棵樹的所有留言；`comment.child_count` 計所有子孫 | 【量】`triggers.sql` | 「這篇有多少討論」的直覺 |
| Discourse | 串是扁平的，`posts_count` 就是全部；`posts.reply_count` 是直接回覆數 | 【量】`topic.rb`、`post.rb` | 扁平模型 |
| Mastodon | `replies_count` 只計**直接回覆** | 【量】`status.rb`（`thread` 是 `in_reply_to_id`） | 聯邦架構下拿不到完整的樹，只能計自己知道的直接子節點（推論） |
| Reddit、YouTube、Instagram、Facebook | 貼文或影片的總數含回覆，頂層另外顯示「N 則回覆」 | 未查證 | — |

### 在我們條件下的推薦

- 貼文的留言數等於可見的頂層加上可見的回覆（且回覆的頂層也計入）。只有兩層，所以「子孫」就是「直接回覆」，不需要 Lemmy 的 ltree 展開。
- 每則頂層留言顯示自己的 `reply_count`。討論區每則頂層顯示 `reply_count`；需求沒提討論區的總數，不做。
- 理由：主流消費型產品的使用者直覺；Mastodon 只計直接回覆是聯邦架構逼出來的，我們沒有那個條件。

### 換做法的觸發條件

- 層數不再限於兩層：回覆數要決定是「直接」還是「子孫」，子孫就照 Lemmy 的 path 展開。
- 產品要區分「留言」與「回覆」兩種數字：在 `post_stats` 加 `top_level_count`，同一個 trigger 維護。

---

## 第 4 題：不同觀看者看到的數字要不要不同

### 業界做法

| 系統 | 做法 | 來源 | 為什麼 |
|---|---|---|---|
| Facebook TAO | count 是 association list 的大小；隱私檢查在每次觀看時套用到**項目** | 【量】TAO §2、§3.3（count 不依觀看者過濾是推論） | 依觀看者過濾的 count 不能快取，O(1) 的保證就沒了 |
| Instagram／Facebook 隱藏讚數 | 是**顯示層的開關**（作者對單篇關閉，或觀看者對所有人關閉），後端照常計數 | 【二手】[Tubefilter 2021](https://www.tubefilter.com/?p=158649)、[Sprout Social](https://help.influencer.sproutsocial.com/en/articles/6480646-instagram-hidden-likes) | 心理健康考量，不是準確性問題 |
| Discourse ignore、Reddit 封鎖 | 被忽略或封鎖者的內容會收合或隱藏，計數不變 | 未查證 | — |
| Lemmy、Mastodon | 計數存在列上，是全域單一值，沒有任何依觀看者調整的路徑 | 【量】（同上原始碼：計數欄位沒有 viewer 維度） | 存起來的計數本來就不分觀看者 |

### 在我們條件下的推薦

所有人看到同一個公開語意的值：

- **被封鎖者、封鎖者**：不調整。封鎖者可能看到「5 則留言」但列表只有 4 則，業界普遍接受這種落差，換到的是計數可以存。
- **作者看自己被隱藏的留言**：計數不含它（少 1），可以接受。
- **店主看自己不公開的貼文**：看到凍結的公開語意值。如果店主需要「含不公開」的數字，是另一個管理面的需求，在店家後台讀時算即可（只有店主自己在看，量很小）。

### 換做法的觸發條件

- 產品或法遵要求封鎖者看到的計數必須和列表一致：改成「存的值減去觀看者封鎖對象的貢獻」。封鎖名單很小，用 `(post_id, author_id)` index 做一次小查詢就夠，不必整個改回讀時算。
- 如果出現私密或限定對象的內容（例如只給追蹤者看的留言），那類內容一開始就不計入公開計數（Mastodon 對 direct 的做法），不要改成依觀看者計數。

---

## 整體設計

| 計數 | 存不存 | 計入語意 | 維護 |
|---|---|---|---|
| 貼文讚 | 存（`post_stats`） | 現存的買家讚；刪帳時刪讚列 | 讚表 trigger，同一個 transaction |
| 貼文留言 | 存 | 已發布、未刪、未被隱藏或下架的頂層加回覆（回覆的頂層也要計入，待決定，見第 2 題） | 留言表 statement-level trigger，用 `is_counted` 舊值與新值的差 |
| 留言讚 | 存 | 現存的讚（被讚的留言不可見時，值保留但不顯示） | 讚表 trigger |
| 留言回覆 | 存（頂層） | 可見的直接回覆 | 回覆的 trigger |
| 討論區回覆 | 存（頂層） | 同上 | 同上 |
| 瀏覽 | 存 | 每個觀看者（登入的用 user id，否則用裝置或 IP 的 key）每貼文每天一次；作者不計 | 去重表 `ON CONFLICT DO NOTHING`，新增時才 +1；只保留 1–2 天 |
| 分享 | 存 | 每人每貼文每天一次分享動作（client 回報，無法驗證真的分享出去） | 同瀏覽 |

共通規則：`is_counted` 只看列自己的欄位；可撤銷的上層狀態不改計數；不可撤銷的上層狀態（刪帳）寫到每一列；`CHECK (>= 0)`；trigger 固定鎖順序；每日 River 對帳只修有差異的列並記錄漂移數；手動全量重算指令；整合測試以重算 SQL 當 oracle。

### 不確定的地方

1. **頂層被隱藏或刪除時回覆是否公開**（佔位符與否）會改變貼文留言數的維護公式，屬於產品決定。
2. **trigger 和這個 monolith 既有的資料存取慣例是否衝突**：我沒讀既有程式與計畫，只依業界基線推薦。如果團隊選 app 路徑維護，對帳 job 就從「偵測 bug」變成「必要的修正機制」（Discourse 模式），漂移在對帳週期內可見。
3. **「讀時算」在 10⁵ 規模的實際延遲沒有量過**；我推論它可行，所以選「存」的理由主要是業界基線與排序、熱門的潛在需求，不是效能壓力。如果決策者認為熱門排序永遠不會出現，讀時算也是可辯護的選項，代價是 feed 熱路徑多一組帶可見性 join 的聚合。
4. **瀏覽與分享的定義**（feed 曝光算不算瀏覽、匿名觀看者的 key 怎麼取、要不要排除 bot）是產品與反濫用的決定，這裡採 Discourse 的「詳情頁、每人每天」。
5. Reddit、YouTube、Instagram、Facebook 關於計數語意與封鎖的部分大多是【二手】或未查證，結論主要靠 Lemmy、Discourse、Mastodon、TAO、GitLab 的原文。
