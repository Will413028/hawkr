# review 模組的沿用盤點

<!-- 決策紀錄：不放進 AGENTS.md，因為 AGENTS.md 每次在這個目錄工作都會載入。模組現行的規則寫在 AGENTS.md。 -->

來源：`backend/docs/plans/2026-11-19-a6-review-im-crm/domain-inventory.md` §2 的 M 列（只列與 review 有關的）與 §6.1 的 RD 列；實作設計 `step-1-design.md` v2。本檔對應 A6-1b、A6-1c 的程式；A6-1c 以後才做的項目在「決定」欄寫明。

## 沿用盤點

review-service 的每個機制，搬進來時都重新判斷過（遷移計畫「模組完成的定義」）：ID 方案、extension、套件、共用 helper、舊的檢查邏輯、schema 慣例、CodeMeta 的值域與錯誤碼。從零設計會不同的，完成前提出來決定。對外 API（舊 gateway 的 handler）的機制在下一節。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| M1 評價 ID 是 TSID bigint（`usecase/review_usecase.go:190`） | Java 或別的服務拿評價 ID 當鍵 | 不成立：Java 不呼叫評價實體，只有 `order_item_id`、`order_no` 是外部鍵；`review.created` 的 `review_id` 只進推播 data，四端都沒有讀 | 實體主鍵用應用層產生的 UUIDv7（row-identity ADR） | 改：`reviews.id` 是 UUIDv7；`buyer_id`、`store_id`、`product_id`、`order_item_id` 維持 bigint（外部業務鍵）。`legacy/reviewrelay` 的兩個事件 `review_id` 送 0（設計書 §9 第 2 項，A6-1c 實作並由 `wire_test`、`relay_test` 守住） |
| M4 Redis（封鎖的 `block:out:`／`block:in:` 讀取；啟動連不上只警告、封鎖 fail-open） | review 需要即時的封鎖資料 | 不成立：封鎖在 buyer 的 DB，單體同一個資料庫 | 同庫用擁有者發布的唯讀函式，在 SQL 述詞內過濾，失敗就整個查詢失敗（fail-closed） | 改：`buyer.pub_blocked_buyers()`、`buyer.pub_blocked_stores()` 內聯進公開查詢的述詞；刪除 Redis 依賴。review 沒有其他 Redis key |
| M5 `product_id DEFAULT 0`、`store_id DEFAULT 0` 哨兵 | 舊資料沒有商品 ID 的評價 | 不成立：系統從未上線，訂單項目一定有商品 | NOT NULL ＋ CHECK > 0，不用哨兵 | 改：`product_id`、`store_id` 是 `NOT NULL CHECK > 0`；訂單項目沒有商品 ID 時 legacy 轉接回錯誤而不是 0（`orderservice.OrderForReview`）。`item_image_key` 沒有圖是 NULL＋CHECK 非空，不用空字串哨兵 |
| M6 媒體引用：提交時 `FinalizeAndLink`（先 link 後插入，插入失敗不回滾）、評價從不釋放 | media 呼叫與業務列寫入要一致 | 不成立：媒體呼叫不得在 DB transaction 內（單體規則 5）；失敗時留下孤兒 link | 與 social 同：transaction 外 `Finalize`，transaction 內寫列＋同步版本＋排同步 job，同步 job 依業務列現況 Link／Unlink | 改：`mediaref`（照 social）與 `media_ref_sync`、`retired_assets`；提交失敗時沒有任何 link。刪帳 purge 的釋放（RD4）在每批的 transaction 內記退場資產並排同步；同步 worker 與保底掃描已接進 `cmd/hawkr` 的 worker（`newReview`） |
| M8 MQ 消費者（review 沒有消費者） | 刪帳等外部事件要驅動 review | 成立（新增需求 A6-D6：買家刪帳要刪評價） | 同一資料庫內下層通知上層用同 transaction 的模組事件（單體規則 4） | 改：訂閱 `buyer.BuyerDeletionRequested`／`BuyerDeleted`，只排 purge job（不寫複本；A6-3c 起）；訂閱者（`OnBuyerDeletionRequested`／`OnBuyerDeleted`，`cmd/hawkr` 的 `newReview` 在 serve 與 worker 訂閱）與 `review.purge_author` job（`internal/purge`，依 id 升序每批 200、可重跑）已在 A6-1c。刪帳者的評價在買家進入刪除中的 transaction 提交時就不公開、不計分（讀取 join `buyer.pub_inactive_buyers()`）；`review.purge_sweep` 週期 job（`purge.PeriodicJobs`，啟動即跑、每 15 分鐘、每次最多 100 位）找出在 `buyer.pub_inactive_buyers()` 但仍有評價列的買家補排 `purge_author`，補 MaxAttempts 用完被丟棄的 job（A6-D13） |
| M9 買家名稱與頭像讀時由 gateway 補，註銷旗標 `BuyerDeleted` | 評價列表要顯示買家 | 成立（顯示需要），但不能存快照（個資） | 讀時批次補，已刪帳回旗標不回名稱 | 沿用：模組不存買家快照；`buyer.BatchDisplay` 由 httpapi 在 A6-5 補 |
| M10 快取失效：commit 之後 best-effort 的 `BustAsync`，店家 tag 另用一次 RPC 解 `store_code` | 評分與評價數變動後 website 快取要失效 | 成立，但 commit 後 best-effort 會在崩潰時漏失效 | 寫入同 transaction 排 job（transactional outbox），tag 由 seller API 取 | 改：提交、隱藏、還原在同 transaction 排 `revalidate` job（`seller.Store.StoreCode` 取代 `ResolveStoreCode`）。商品 tag（`revalidate.ProductTag`，與舊 `pkg/revalidate/tags.go` 同字串）與店家的兩個 tag 在同一個 job（`write.bust` 組 tag；刪帳清除的每批在 `purge.bust` 組） |
| M11 事件：fat event、無 outbox（`review.created`、`review.hidden` 帶統計與暱稱） | Java order、product、notification 依賴舊 wire | 成立（A6-1c 前 Java 仍消費） | 模組事件只帶領域鍵，relay 在 transaction 外組 wire payload | 改：模組事件 `ReviewCreated`、`ReviewAggregateChanged` 只帶領域鍵、同 transaction 發布；wire 形狀由 `legacy/reviewrelay`（A6-1c）維持：`wire_test` 對舊 `pkg/mq/events.go` 的 struct 逐欄比對；`review.hidden` 沒有放進 `event_payload_keys.json` golden（notification-service 的 `TestTemplateContract` 要求 golden 每一項都有模板，`review.hidden` 沒有模板），它的消費者契約（Java `ReviewStatsConsumer` 的 `product_id`、`product_avg_rating`、`product_review_count`）由 `reviewrelay/golden_test.go` 對 Java 原始碼驗。relay 只在測試啟用，A6-6e 才接進 `cmd/hawkr`。重複隱藏不再重發（只有狀態真的改變才發） |
| M12 封鎖過濾在 handler 層、分頁之後，只有商品評論列表（RD1） | 過濾要在分頁前、所有公開入口一致 | 不成立 | 過濾進 SQL 述詞，keyset 之前 | 改：匿名與有觀看者各一支查詢，列表、單筆都帶；統計不過濾；店家封鎖買家不影響（A6-D4） |
| M13 詞典過濾 `pkg/moderation.Check` | 髒字詞典只有一份 | 不成立：單體有 `platform/textfilter` | 唯一詞典 | 改：提交與每次回覆都跑 `platform/textfilter`（回覆覆蓋也跑，沿用舊行為）；提交在查訂單之前（同舊碼） |
| M14 principal 解析（解析賣家 principal 才發 notification，查不到是 0） | notification 要收件人 account | 成立（A6-1c relay） | 在 relay job 內由 seller API 取，查不到不推 | 沿用（`reviewrelay` 在 job 內取）：`notify_seller_account_id` 由 `seller.Store.AccountID` 取，0 表示查不到 |
| M18 分頁：page／pageSize＋`COUNT`（上限 500） | 畫面需要總數與跳頁 | 不成立：client 是無限捲動 | AIP-158 keyset，不回總數，上限 100 | 改：`(created_at DESC, id DESC)` keyset，頁大小預設 20、上限 100，多取一筆判斷下一頁 |
| M19 一次性回填與 ops backfill（只剩註解） | 有指令可跑 | 不成立：倉庫沒有對應命令 | 不留沒有實作的承諾 | 不搬：統計在讀時重算（relay job 執行時讀現況），沒有存下來的聚合要回填 |
| 評價狀態欄位 `hidden_at`、`hidden_reason`、`deleted_at`（RD3） | 隱藏要能還原與稽核；有人寫 `deleted_at` | 不成立：`deleted_at` 沒有寫入者；`hidden_at` 無法還原 | 隱藏紀錄表，歷史留列、部分唯一索引保證最多一筆未還原 | 改：`review.hides`（照 `social.hides`）；沒有 `deleted_at`（刪帳是實刪，A6-D6）。類別用 `policy.CommunityAssessable` |
| `image_keys TEXT[]` 與 usecase 的截斷死碼（RD7） | 附圖張數有上限、同步要 asset ID | 不成立：舊碼只存 object key；截斷是死碼 | 子表＋位置範圍 CHECK | 改：`review_images`（PK `(review_id, position)`，`position BETWEEN 0 AND 3`）；超過 4 個 token 回 InvalidInput，不截斷 |
| 長度以位元組計 `len()`（RD2） | 上限的單位要和訊息一致 | 不成立 | 字元（rune）計 | 改：寫入端 `utf8.RuneCountInString`，DB `char_length`；上限 1000，proto 註明在 A6-5 |
| 快照採請求值（RD6：price、sku_name 取 client） | 賣家頁快照要可信 | 不成立 | 一切取權威來源 | 改：`SubmitInput` 沒有這些欄位，全取 `OrderForReview` 的項目；建立後不變 |
| `GetOrderForReview` 任何錯誤都是 `ErrOrderNotFound`（RD9）；訂單圖用組好的 CDN URL 剝殼還原 | 暫時性故障與不存在要分流；Java 已直接回 object key | 不成立：Java 一律回 `image_object_key`（URL 是同一個 key 組的） | NotFound 與 Unavailable 分流；用權威欄位，不反解 URL | 改：`orderservice.OrderForReview` 只有回應內 code 404 是 `ErrOrderNotFound`，其餘（傳輸、其他 code、不合法欄位）是一般錯誤，review 轉 Unavailable；刪除 URL 剝殼 |
| 單筆讀取不過濾隱藏（RD10） | 單筆與列表的可見性一致 | 不成立 | 一個述詞給所有公開讀取 | 改：`public.sql` 每支查詢帶同一段述詞（`testdata/public_predicate.sql`），`TestEveryPublicQueryCarriesThePublicPredicate` 守門；不可讀與不存在同為 NotFound |
| 統計重算＋事件的競態（RD8） | 全量覆寫的消費者要收斂到最後狀態 | 成立（接受到 catalog 搬遷） | 版本或序號；或消費者讀 `pub_*` | 接受（計畫 RD8）：`reviewrelay` 的 job 執行時讀現況重算；不帶版本（Java 不讀）。刪帳的 `review.hidden` 只走 `review.purge_author` 每批刪除後在同 transaction 對該批每個不同的 (店, 商品) 發的 `ReviewAggregateChanged`（A6-D12、A6-D13；沒有刪帳事件），做完之後重跑不再發 |
| 退款後評價仍計分（RD5） | 退款後的評價要排除 | 成立 | 排除全額退款的評價 | 接受（A6-D7 (a)，Will 2026-11-19）：只放行訂單狀態 50，先評後退的延後到 ordering 搬遷 |

## 傳輸層的沿用盤點

舊 gateway 的 handler 與 gRPC 的機制，以機制為單位。本步沒有對外入口（Connect 與 handler 在 A6-5），這裡記下模組層已定的決定，A6-5 照做。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 認證與身分（gateway 驗 JWT、傳 buyer／store ID） | 評價的寫入要知道買家或店家 | 成立 | session 在入口解析，模組只收已解析的身分 | 沿用語意：`Submit` 收 `BuyerID`，`Reply` 收已解析的 `seller.Store` 並以 `seller.Authorize(CapMarketing)` 檢查（停權 fail-closed）；入口在 A6-5 |
| 錯誤碼（9210 `ModerationRejected`、404／409／428 等舊碼） | client 要能分辨可處理的情境 | 成立 | 錯誤分類＋ reason | 改：`apperr` 分類加模組端的 `Reason` 常數（`REVIEW_ALREADY_EXISTS`、`ORDER_NOT_FOUND`、`ORDER_NOT_REVIEWABLE`、`REVIEW_WINDOW_EXPIRED`、`REVIEW_NOT_REPLYABLE`、`MODERATION_REJECTED`）；proto `ErrorReason` 已定案（`REVIEW_ALREADY_EXISTS` 等；9210 沒有對應值）；CodeMeta 的 9210 與 `ReportTargetType` 的 `REVIEW` 已標擁有者（9210 為 im 與 review 的模組內 reason，`ReportTargetType` 為 moderation，ADR `2026-09-25-codemeta-retirement` 過渡第 3 步），舊碼只服務舊契約。句子在 `internal/text` 目錄 |
| 分頁 page／size＋總數 | 見 M18 | 不成立 | keyset | 改：見 M18 |
| 下游呼叫的逾時與重試（order-service 無明確逾時） | 訂單查詢不能無限等 | 成立 | 明確 deadline、fail-closed | 改：`orderservice` 的 5 秒逾時；任何失敗不當成「不存在」 |
| 請求大小與限流 | 防濫用 | 成立 | 入口統一 | 不在模組：沿用 `httpapi` 既有限流（M4）；A6-5 |

## 與 review-service 的差異

資料沒有搬移（系統從未上線），schema 是新的 baseline（`review.inactive_authors` 已由 A6-3c 的 migration 刪除）。

- 表：`order_item_reviews` → `review.reviews`（UUIDv7 主鍵、`order_item_id` UNIQUE、無 `hidden_at`／`hidden_reason`／`deleted_at`／`image_keys`）、`review.review_images`、`review.hides`、`review.media_ref_sync`、`review.retired_assets`。
- 隱藏可以還原；重複隱藏不重發事件。
- 內容、回覆長度以字元計；`price`、`sku_name`、`product_id`、商品圖全取訂單項目；附圖最多 4 張且超過回錯誤。
- 公開讀取（列表、單筆、統計）共用同一個「公開可讀」述詞；觀看者封鎖在 SQL 內、keyset 之前；單筆讀取不再洩漏隱藏的評價。
- 訂單暫時性故障是 Unavailable，不是 NotFound。
- 分頁是 keyset，沒有總數；店家評價有各星等的分佈（`StoreStats.Distribution`）。
- 快取失效在寫入同 transaction 排 job。

`legacy/reviewrelay` 的 `typedWorker`、`baseEvent` 與其他舊 relay 的同名機制刻意重複（relay 是暫時的，隨 A8、catalog、ordering 搬遷刪除，見計畫「延後」），不抽共用。

## A6-3c 停用身分改由擁有者守門的沿用盤點

設計：`backend/docs/plans/2026-11-19-a6-review-im-crm/step-3c-design.md` §9.1、§9.5。

| 照搬的機制 | 先問「從零設計會怎麼做」 | 結論 |
|---|---|---|
| `inactive_authors` 複本表（A6-1c） | 複本的必要條件是「資料庫端存旗標」且「函式只能參照自己的表」；review 只在查詢過濾與寫入把關，沒有存旗標 | 不成立；公開述詞、`own`、`purge` 的查詢 join `buyer.pub_inactive_buyers()`，刪表（`review_drop_inactive_authors`） |
| `LockAuthor`（`Submit` 沒有可鎖的列，所以用 advisory lock） | buyer 已有 `LockActive`，鎖的是買家列（`FOR KEY SHARE`），lockorder 看得到 | 不成立；`Submit` 的 transaction 第一句 `Buyers.LockActive`，FailedPrecondition 與 NotFound 都翻成 `ReviewAuthorInactive` |
| 刪帳訂閱者只排 job、抹除 job、保底掃描 | 抹除的形狀在 review 自己 | 成立，保留；`Deactivate` 改名 `EnqueuePurge`（只排 job，不要求 transaction），保底掃描讀 `pub_inactive_buyers` |
| 公開述詞只寫一份（`testdata/public_predicate.sql`） | 一條規則一個定義 | 成立，述詞內容改 join；`TestInactiveBuyersPredicateInlinesThePubFunction` 以 `EXPLAIN` 鎖住內聯 |
| `AuthorInactive`：`Submit` tx 外的提早檢查 | 必要條件是「被擋的請求不做 Finalize 轉正圖片」 | 成立，保留；tx 內的 `LockActive` 才是正確性保證，提早檢查只省成本 |
| `ReviewForReport` 是證據讀取，不給顯示用 | 檢舉的快照要證據欄位（作者、評分、店家回覆、圖片 asset ID），顯示投影有 `ItemImageKey` 等，兩者各自演進 | 成立 | 同 | 新增（A6-4b）：可讀規則與 `GetReview` 共用同一個公開可讀述詞與觀看者封鎖（`ReviewForReport` 呼叫同一條讀取路徑，不另寫述詞）；不鎖列、加入呼叫方的 transaction |

觸發條件（改為 buyer 加 `WHERE status <> 'ACTIVE'` 的部分索引；門檻是推測，沒有量測）：`review.purge_sweep` 單輪耗時超過 30 秒，或 `SELECT count(*) FROM buyer.buyers WHERE status <> 'ACTIVE'` 超過 10 萬列。保底掃描從 `pub_inactive_buyers()` 出發逐一 `EXISTS` 評價列（`reviews_buyer` 索引），不加索引（設計 W5 (a)）。

## 公司時期未完成項

A6 對照現行程式碼的結果（舊公司 repo 的 issue 只有一行標題，內文取不到；#3497 的標題以外無內容）。

| 項目 | 結論 | 依據 |
|---|---|---|
| review `SubmitItemReview` 沒有 transaction | 已修：提交的寫入集中在同一個 transaction。舊碼是 `FinalizeAndLink` 逐張轉正後單句 `repo.Insert`，事件以 best-effort 發布（`94796248:backend/services/review-service/internal/usecase/review_usecase.go:133,200-232`）；現在 `buyer.LockActive`、`InsertReview`、`InsertReviewImage`、媒體同步 job、快取失效 job 與 `ReviewCreated` 事件都在同一個 `DB.InTx` 內，任何一步失敗整體回滾（讀程式碼的推論，沒有找到專門驗回滾的測試）；transaction 外只剩 media 轉正（單體規則 5） | `internal/write/write.go:122`（`InTx`）、`:124`（`LockActive`）、`:132`（`InsertReview`）、`:144`（`InsertReviewImage`）、`:150`（`mediaref.Changed`）、`:154`（`bust`）、`:157`（`events.Publish`）；沿用盤點 M6、M10、M11 |
| #3494：`GetReview` 不過濾 hidden／deleted，已隱藏的評論任何登入者都讀得到 | 已修（RD3）：單筆讀取與列表共用公開可讀述詞，有未還原的隱藏或作者不在營運中都回 NotFound，不洩漏存在性；沒有 `deleted_at`，刪帳是實刪。未讀到專門的「隱藏後單筆讀取回 NotFound」測試名稱，結論依 SQL 與程式碼 | `db/queries/public.sql:65-69`（`GetPublicReviewAnonymous`，`NOT EXISTS review.hides ... restored_at IS NULL` 與 `pub_inactive_buyers()`）、`:71` 起（`GetPublicReviewForViewer` 另加觀看者封鎖）；`internal/read/read.go:156-170`（`GetReview` 只走這兩支，`ErrNoRows` 回 `ReviewNotFound`）；`api/hawkr/review/v1alpha1/review.proto:46` |
| #3495：`ReviewStats` 對外用 double／int64，違反數字一律 string 的規範 | 不成立：現行規範是〈API 設計規範（Connect）〉§四，`int64` 在 proto3 JSON 是字串、有界的計數用 `int32`（JSON 數字）、金額禁浮點；「數量、計數用字串」只管舊 REST，不管 Connect。現行統計 `count` 是 `int32`（有界筆數，符合）；`average` 是 `double`，是評分平均而非金額，§四沒有禁止；`distribution` 的各星等數量見 `RatingCount`。商品統計與店家統計同形。對外沒有 `int64` | `docs/共通規範/API 設計規範（Connect）.md:100`；`backend/AGENTS.md:65`；`api/hawkr/review/v1alpha1/review.proto:157,162,164,175,177,181`；規範沒寫「平均值」的型別，這裡採 `double` 是現況，不是規範明文 |
| #3497：review-service 技術債七項 | 標題以外無內容，七項無法逐項對照；舊服務已整體重寫並刪除。舊碼自己盤出的缺陷與機制處置見上方「沿用盤點」，那不等於這七項 | `domain-inventory.md` §6；本檔「沿用盤點」 |
| #3499：`ReplyReview` 要不要 revalidate product tag | 不需要：`Reply` 只改回覆欄位，不排快取失效，也不改評分與評價數；`bust`（商品與店面 tag）只在提交（`write.go:154`）與隱藏、還原的 `aggregateChanged`（`:362`）呼叫。回覆文字不在被快取的公開頁：website 只有賣家中心顯示回覆（量：`rg -l -i reply website --glob '!**/node_modules/**' --glob '!**/tests/**' | rg -i review` 只得 `features/seller-center/products/ProductReviewsPanel.tsx`、`lib/reviews/repo.ts`），App 不用伺服器快取 tag。觸發：公開商品頁或店面開始顯示店家回覆時，`Reply` 改排 `bust` | `internal/write/write.go:232`（`Reply`，區間內沒有 `bust`）、`:154`、`:362` |
