# A8 單體事件裁決表（D3 草案）

核對日 2026-11-23，dev `bcc906501`。來源：`../2026-11-23-a8-notification.md`（範圍、A8-3、D3／D4／D5／D6／D12／D13）、`inventory.md` §5.4、§8.1、§8.3。publisher 欄是我在目前程式碼用 `rg` 讀到的 `events.Publish` 位置（路徑相對 `backend/internal/modules/`，除非是 `legacy/`、`cmd/`）。**【推】** 表示該格是推論（計畫與程式碼沒寫明），交 Will 確認；沒有【推】的格是讀到的。

## 裁決（Will 2026-11-23）

開放問題 1–7 照本表建議拍板，相關列的【推】視為已確認；問題 8（文末四個範圍外事件）未決，記為計畫 D3b。D6 的呈現分類 10 個值照提案。

## 規則（引自計畫，不是新增）

- D3：範圍只有單體事件；有使用者需要知道、且有發送端才做型別；`product.viewed` 不做；`MeasureRequestSubmitted` 不進通知（管理員不是收件人）。
- D4：交易必要（收件匣、email 不可關）／互動（可關）／行銷（預設開、可關）。
- D5：取貨簡訊刪除。D12：型別可宣告聚合鍵、未讀時合併，不做時間窗摘要。D13：不做 legacy 通知轉接。
- 受眾：BUYER、SELLER（store）。

## 呈現分類 enum 提案（D6，10 個值）

| 值 | 收的內容 |
|---|---|
| `ORDER` | 訂單、出貨（單體尚無事件，隨 B2–B10 加） |
| `REFUND` | 退款與售後（同上，隨 B2、B3） |
| `PAYOUT` | 撥款、收款帳戶 |
| `MESSAGE` | 聊天 |
| `SOCIAL` | 留言、讚、追蹤、追蹤的店發文 |
| `REVIEW` | 評價 |
| `PROMOTION` | 補貨（行銷）；之後的 coupon |
| `PROCUREMENT` | 詢價邀請 |
| `ACCOUNT` | 帳號停權、店家措施與治理、KYC、檢舉結果、代行完成 |
| `SYSTEM` | 後台公告 |

對照舊分類（`inventory.md` §5.2：order_update、refund_update、system、chat、follower、post、coupon、message、shipment_anomaly、restock，另有不在清單的 content、seller_account）：舊 `content`／`follower`／`post`／`message` 併入 `SOCIAL`，`seller_account`／`system` 的治理類併入 `ACCOUNT`。`ORDER`、`REFUND` 目前零個型別，是為了不讓 B2 之後改 enum；若 Will 要「只放現有型別」可先刪這兩個，剩 8 個。【推】

## 裁決表

欄位：# ＝ `§8.1#n`（22 個 route）或 `§8.3#n`（12 項，一項含多個事件時拆 a、b…）。型別名是 snake_case。聚合鍵只有讚與追蹤（D12，A8-3 範圍文字）。

| # | source | event | publisher file:line | type 或「不做」 | audience | D4 | 呈現 | 聚合鍵 | 舊模板（§5.4 列） | reason / evidence |
|---|---|---|---|---|---|---|---|---|---|---|
| §8.1#1 | `hawkr.account` `account.deleted` | `identity.AccountDeleted` | `identity/internal/account/account.go:436`；wire `legacy/accountservice/events.go:26` | 不做 | - | - | - | 不聚合 | 無（靜態訂閱，非模板） | 使用者不需要知道自己刪了帳號。它是 A8-1 個資清除的訂閱來源（計畫 A8-1「訂閱 `identity.AccountDeleted`」），不是通知型別。 |
| §8.1#2 | `hawkr.buyer` `buyer.deleted` | `buyer.BuyerDeleted` | `buyer/internal/deletion/deletion.go:135`；wire `legacy/buyerrelay/relay.go:307` | 不做 | - | - | - | 不聚合 | 無（靜態訂閱） | 同 #1；A8-1 訂閱 `buyer.BuyerDeleted`（`buyer/events.go:25`）清資料。 |
| §8.1#3 | `hawkr.buyer` `buyer.favorite.added` | `buyer.FavoriteAdded` | `buyer/internal/favorite/favorite.go:53`；wire `legacy/buyerrelay/relay.go:234` | 不做【推】 | - | - | - | 不聚合 | #38 `product.favorited` seller | 有發送端，但「有使用者需要知道」未成立：D4 互動清單（聊天、評價、留言、讚、追蹤、追蹤的店發文）與 A8-3 範圍都沒有收藏。收藏是對商品的低訊號行為，比追蹤更弱。【推】若 Will 要保留舊行為，型別 `product_favorited`、SELLER、互動、`SOCIAL`、聚合鍵 `fav:{product_id}`。 |
| §8.1#4 | `hawkr.buyer` `buyer.favorite.removed` | `buyer.FavoriteRemoved` | `buyer/internal/favorite/favorite.go:112`、`deletion.go:108` | 不做 | - | - | - | 不聚合 | 無 | 現行就是刻意不通知（`notifyAllowlist`，`inventory.md` §8.1#4）；取消收藏不需要告知賣家。 |
| §8.1#5 | `hawkr.content` `comment.created` | `social.CommentCreated` | `social/internal/comment/comment.go:212` | `comment_created` | SELLER（貼文所屬店家） | 互動 | `SOCIAL` | 不聚合 | #40 `comment.created` seller；#39 `.buyer` 無對應 | 貼文作者恆為店家（`legacy/socialrelay/relay.go` commentCreated「貼文作者恆為店家」），所以 #39 買家版在單體沒有發送端，不做。店家自己留言自我略過（帳號比對，A8-1）；封鎖雙方不送。 |
| §8.1#6 | `hawkr.content` `comment.like_changed` | `social.CommentLiked` | `social/internal/engage/engage.go:138` | `comment_liked` | 依 `Author` 的種類：BUYER 或 SELLER | 互動 | `SOCIAL` | `comment_like:{comment_id}` | #41 seller（只有 SELLER 版） | 事件帶 `Author`（留言作者）可以是買家或店家（`social/internal/model/events.go:76`）；舊模板只涵蓋店家，買家留言被按讚舊系統不通知。【推】兩種作者都做（計畫 A8-3 寫「讚用聚合鍵」，沒限定作者種類）。只在新增讚時發，取消讚不發（事件註解）。 |
| §8.1#7 | `hawkr.content` `comment.replied` | `social.CommentReplied` | `social/internal/comment/comment.go:216` | `comment_replied` | 依 `ReplyToActor` 的種類：BUYER 或 SELLER | 互動 | `SOCIAL` | 不聚合 | #42 `.buyer`、#43 `.seller` | 舊模板兩張對應同一型別的兩種受眾（一個型別宣告受眾由事件決定，或拆兩個型別，A8-1 註冊表決定）。自我回覆略過；封鎖雙方不送。 |
| §8.1#8 | `hawkr.content` `follow.added` | `buyer.SellerFollowed` | `buyer/internal/follow/follow.go:51`；wire `legacy/buyerrelay/relay.go:272` | `store_followed` | SELLER | 互動 | `SOCIAL` | `follow:{store_id}` | #44 `follow.added` seller | D4 互動清單含追蹤；A8-3 寫「buyer 的追蹤店家用聚合鍵」；`inventory.md` §8.3#11 的「追蹤通知節流／聚合」由此處理。店家已不存在則無收件人（`buyerrelay/relay.go:260` 現行行為）。 |
| §8.1#9 | `hawkr.content` `post.created` | `social.PostPublished` | `social/internal/post/post.go:161`（emit `:175`、`:177`） | `post_published` | BUYER（該店的追蹤者，worker 向 buyer 讀追蹤者） | 互動 | `SOCIAL` | 不聚合 | #45 `post.created` buyer | D4 互動清單含「追蹤的店發文」。觸發點是發布不是草稿；排程到點發布（`Origin=SCHEDULED`）也發同一事件。每個追蹤者一個意圖或一個 fan-out job（現行 `socialrelay` 一頁 100 人，`relay.go:24-26`）。 |
| §8.1#10 | `hawkr.content` `post.hidden_by_platform` | `seller.MeasureCreated`（`Kind=CONTENT_TAKEDOWN`） | `seller/internal/measure/create.go:491`（wire `legacy/socialrelay/relay.go:710-731`） | `store_content_taken_down` | SELLER | 交易必要（治理） | `ACCOUNT` | 不聚合 | #46 `post.hidden_by_platform.seller` | A8-3 列「措施與治理」。事件的 `Contents` 不只貼文，還有聊天訊息與評價回覆（`im/im.go:322`、`review/review.go:262` 的訂閱者），型別以內容種類為參數，不是只做貼文。【推】 |
| §8.1#11 | `hawkr.content` `post.liked` | `social.PostLiked` | `social/internal/engage/engage.go:71` | `post_liked` | SELLER | 互動 | `SOCIAL` | `post_like:{post_id}` | #47 `post.liked` seller | A8-3 明寫「讚用聚合鍵」。只在新增時發（`PostLiked` 註解）。 |
| §8.1#12 | `hawkr.content` `product.viewed` | `buyer.ProductViewed` | `buyer/internal/visit/visit.go:50`；wire `legacy/buyerrelay/relay.go:291` | 不做 | - | - | - | - | #48 `product.viewed` seller | D3 與「範圍：不做」明文：`product.viewed` 不做。同時刪 `buyerrelay` 的 `product.viewed` wire（A8-3）。 |
| §8.1#13 | `hawkr.delegation` `delegation.operation.completed` | `delegation.DelegationOperationCompleted` | `delegation/internal/audit/audit.go:349`、`operation.go:102`、`scheduled.go:67` | `delegation_operation_completed` | SELLER（`TargetStoreID`） | 交易必要（治理） | `ACCOUNT` | 不聚合 | #49 `delegation.operation.completed.seller` | A8-3 列「delegation 代行完成」。事件只在 intent 要求通知賣家時才發（`delegation/internal/model/events.go:10` 註解），所以不需要另判斷。代行者本身不能讀（D9）。分類歸「帳號與治理」是【推】。 |
| §8.1#14 | `hawkr.im` `chat.message.sent` | `im.MessageSent` | `im/internal/write/send.go:145` | `chat_message` | 對方：`SenderKind` 是買家則 SELLER，是店家則 BUYER | 互動 | `MESSAGE` | 不聚合【推】 | #50 buyer、#51 seller | A8-3 寫「im（離線推播）」，所以渠道只有推播、不寫收件匣【推】（舊模板 visible 會寫收件匣，並有「紅線化」清除，`inventory.md` §3.2#1）。即時送達由 im 自己的即時通道負責（`im/internal/model/model.go:277` 註解）。 |
| §8.1#15 | `hawkr.product` `product.restock.target` | `buyer.RestockTargeted` | `buyer/internal/restock/restock.go:89`（呼叫端 `legacy/buyerconsumer/consumer.go:181`） | `product_restocked` | BUYER | 行銷 | `PROMOTION` | 不聚合 | #83 `product.restock.target` buyer | D4 行銷含 restock（預設開、可關）。發送端：Java product 的補貨事件經 `buyerconsumer` 進單體再 fan-out，是單體事件。 |
| 單體 `coupon.VoucherExpiring` | `coupon` | `coupon.VoucherExpiring` | `coupon/internal/expiry/expiry.go:118`（`coupon/events.go`） | `voucher_expiring` | BUYER | 行銷（coupon） | `PROMOTION` | 不聚合（D27） | 無（舊 `coupon.expiring` 無發送端） | D26：推播 22:00–09:00（台北）延到 09:00，收件匣照常即時；D27：每券一則、冪等鍵取 `voucher_id`、送出前重查券仍為 `available`；參數不存活動名稱原文；深連結 `/account/coupons`。A8-3e。 |
| §8.1#16 | `hawkr.review` `review.created` | `review.ReviewCreated` | `review/internal/write/write.go:157` | `review_created` | SELLER（`StoreID`，worker 向 seller 解析收件人） | 互動 | `REVIEW` | 不聚合 | #84 `review.created` seller | D4 互動含評價。 |
| §8.1#17 | `hawkr.review` `review.hidden` | `review.ReviewAggregateChanged` | `review/internal/write/write.go:365`、`review/internal/purge/purge.go:178` | 不做 | - | - | - | - | 無 | 現行就是刻意不通知（`notifyAllowlist`）；事件只表示「聚合要重送」，不指稱單則評價（`review/internal/model/model.go:249`），沒有人可通知。評價被隱藏的作者通知屬 §8.3#7a（moderation）。 |
| §8.1#18 | `hawkr.seller` `seller.deleted` | `seller.StoreRetired` | `seller/internal/purge/purge.go:175`；wire `legacy/sellerrelay/relay.go:228` | 不做 | - | - | - | - | 無（靜態訂閱） | 同 #1；A8-1 訂閱 `seller.StoreRetired`（`seller/events.go:31`）清資料。 |
| §8.1#19 | `hawkr.seller` `seller.exposure.changed` | `seller.StoreExposureChanged` | `seller/internal/exposure/exposure.go:138` | 不做 | - | - | - | - | 無 | 現行刻意不通知（`notifyAllowlist`）；曝光是內部可見度，原因已由措施與停權通知涵蓋。 |
| §8.1#20 | `hawkr.seller` `seller.profile.changed` | `seller.StoreProfileChanged` | `seller/internal/profile/profile.go:549` | 不做 | - | - | - | - | 無 | 現行刻意不通知；是賣家自己的寫入（自我通知無意義）。 |
| §8.1#21 | `hawkr.seller` `seller.suspended` | `seller.StoreSuspended` | `seller/internal/suspension/suspension.go:140` | `store_suspended` | SELLER | 交易必要（治理） | `ACCOUNT` | 不聚合 | #85 `seller.suspended.notice` | A8-3 列「措施與治理」。停權措施的 `StartsAt` 是建立當下（`seller/internal/model/measure.go:1465` 註解），所以 `MeasureCreated(SUSPENSION)` 不另發通知，以免同一刻兩則【推】。店家停權不動 token（A8-3）。 |
| §8.1#22 | `hawkr.seller` `seller.unsuspended` | `seller.StoreUnsuspended` | `seller/internal/suspension/suspension.go:155` | `store_unsuspended` | SELLER | 交易必要（治理） | `ACCOUNT` | 不聚合 | #86 `seller.unsuspended.notice` | 同 #21。 |
| §8.3#1a | accountsuspension | `accountsuspension.Suspended` | `accountsuspension/accountsuspension.go:76` | `account_suspended` | BUYER（只有無店的帳號能被停權，`:63` `requireNoStore`，定義 `:106`） | 交易必要（治理） | `ACCOUNT` | 不聚合 | 無（新型別；舊模板只有店家版） | `accountsuspension/events.go:10` 註解「通知（A8）消費它」；A8-1 端到端範例。同 transaction 會刪 token（A8-3），所以此型別實際渠道是收件匣與 email，沒有推播【推】；受限登入仍可讀收件匣（`accountsuspension/CARRYOVER.md` A7-D17）。 |
| §8.3#1b | accountsuspension | `accountsuspension.Unsuspended` | `accountsuspension/accountsuspension.go:94` | `account_unsuspended` | BUYER | 交易必要（治理） | `ACCOUNT` | 不聚合 | 無 | 同 1a；手動解除才發，沒停權時不發。 |
| §8.3#2a | seller | `seller.ReportReceived` | `seller/internal/governance/report.go:135`（補充 `:205`） | `report_received` | BUYER（`ReporterBuyerID`，檢舉人一律是登入買家） | 交易必要（治理） | `ACCOUNT` | 不聚合 | 無（新型別） | 計畫 A8-3：確認信，意圖參數只放 report 參照，worker 向 seller 讀地址（不進 job 參數）。補充（`SupplementID`）是同一事件，文案可依有無 `SupplementID` 分支。 A8-3c：依 D24 的補充（Will 2026-11-24）型別依受眾各一版——買家檢舉人 `report_received`、店家檢舉人 `store_report_received`，同參數（`ReportSource`、`ReportID`）與文案；兩版都是收件匣、推播加確認信，位址在投遞時依帳號向 identity 讀（不在任何 job 參數）；賣家來源的補充（`SupplementID`）各自一則。 |
| §8.3#2b | seller | `seller.ReportResolved` | `seller/internal/governance/resolve.go:86`、`case.go:116` | `report_resolved` | BUYER（`ReporterBuyerID`） | 交易必要（治理） | `ACCOUNT` | 不聚合 | 無 | 每筆通報一次，帶 `Outcome`（ACTION_TAKEN、NO_VIOLATION、INSUFFICIENT、DUPLICATE、ABUSIVE，`seller/internal/model/report_case.go:16-28`）。主計畫 A8「檢舉結果通知」。 A8-3c：註冊表的 Spawned 型別沒有自己的冪等鍵，所以賣家來源經 fan-out 母意圖 `seller_report_resolved`（只展開那一位買家）建 `report_resolved` 子意圖，與 moderation 來源同一個子型別；買家與店家檢舉人依 D24 的補充各一版：`report_resolved`、`store_report_resolved`（同參數與文案）。 |
| §8.3#3 | seller | `seller.MeasureRequestSubmitted` | `seller/internal/measure/appeal.go:156` | 不做 | - | - | - | - | 無 | D3 明文：管理員不是收件人。賣家自己提出的請求，自我通知無意義。 |
| §8.3#4a | seller 措施 | `seller.MeasureCreated`（`Kind` 為 WARNING、TERMINATION） | `seller/internal/measure/create.go:491` | `store_measure_issued` | SELLER | 交易必要（治理） | `ACCOUNT` | 不聚合 | 無（新型別） | 賣家必須知道被警告或被預告終止合作（預告終止的生效在未來，`measure.go:1465`）。SUSPENSION 見 §8.1#21，CONTENT_TAKEDOWN 見 §8.1#10。 |
| §8.3#4b | seller 措施 | `seller.MeasureRevised` | `seller/internal/measure/update.go:129` | `store_measure_revised` | SELLER | 交易必要（治理） | `ACCOUNT` | 不聚合 | 無 | 只在賣家可見的內容（理由、結束時間）實際改變時發（`measure.go:1486` 註解），賣家需要知道。 |
| §8.3#4c | seller 措施 | `seller.MeasureEnded` | `seller/internal/measure/update.go:231`、`seller/internal/closing/closing.go:287` | `store_measure_ended` | SELLER | 交易必要（治理） | `ACCOUNT` | 不聚合 | 無 | 接受解除請求時同 transaction 也發 `MeasureRequestDecided`（`measure.go:1497` 註解），會兩則重複；依 D3 同一次處置只送 Decided 那則。 A8-3c（D3）：接受解除請求的結束，seller 在 `MeasureEnded` 帶 `RequestID`，intake 不登記 `store_measure_ended`，只送 `store_measure_request_decided`（同一次處置一則）；管理員直接結束與關店照常通知。 |
| §8.3#4d | seller 措施 | `seller.MeasureRevoked` | `seller/internal/measure/revoke.go:349` | `store_measure_revoked` | SELLER | 交易必要（治理） | `ACCOUNT` | 不聚合 | 無 | 撤銷（含接受申覆）；`Contents` 非空代表內容下架解除。效果解除另由 `StoreUnsuspended` 通知（§8.1#22）。 A8-3c（D3）：接受申覆的撤銷（`Reason` 是 `APPEAL_ACCEPTED`）不登記 `store_measure_revoked`，由 `store_measure_request_decided` 通知；其他撤銷原因照常通知。 |
| §8.3#4e | seller 措施 | `seller.MeasureRequestDecided` | `seller/internal/measure/remediation.go:103`、`appeal.go:306` | `store_measure_request_decided` | SELLER | 交易必要（治理） | `ACCOUNT` | 不聚合 | 無 | 申覆或解除請求的裁定結果（`Outcome` ACCEPTED／REJECTED），賣家等這個結果。 A8-3c（D3）：接受與駁回都建立此型別，參數帶 `Outcome`，文案依種類與裁定（四種）。 |
| §8.3#4f | seller 措施 | `seller.MeasureRequestClosed` | `seller/internal/measure/revoke.go:326` | 不做【推】 | - | - | - | - | 無 | 結案原因只有措施被撤銷或已結束（`measure.go:1538`），這兩件事賣家已由 4d、4c 收到通知；再發一則是重複。 |
| §8.3#4g | seller 治理 | `CaseInquiryCreated`（**目前不存在**） | 無；`seller/internal/governance/inquiry.go:130` 註解「不發事件：還沒有消費者」，`seller/governance.go:645` 同 | `store_case_inquiry` | SELLER | 交易必要（治理） | `ACCOUNT` | 不聚合 | 無（新型別） | 計畫 A8-3 要 seller 新增公開事件並在 `CreateCaseInquiry` 同 transaction 發布。`inventory.md` §8.3#4 寫「`CaseInquiryCreated` 已刪（無消費者）」，與計畫 A8-3「新增」的說法一致（事件是被刪過、現在要重加）。賣家需要知道有詢問、有期限（`InquiryDefaultDue`）。 |
| §8.3#5a | seller KYC | `seller.KYCSubmitted` | `seller/internal/kyc/kyc.go:418` | 不做 | - | - | - | - | #71 `merchant.verification.under_review`（無發送端） | 賣家自己送出的申請，自我通知無意義；事件註解「沒有對應的舊 routing key（舊系統不通知送出）」。 |
| §8.3#5b | seller KYC | `seller.KYCApproved` | `seller/internal/kyc/admin.go:372` | `kyc_approved` | SELLER | 交易必要（治理） | `ACCOUNT` | 不聚合 | #67 `merchant.verification.approved` | 計畫 A8-3「KYC 審核結果」；`inventory.md` §8.3#5：賣家目前收不到核准通知。 |
| §8.3#5c | seller KYC | `seller.KYCRejected` | `seller/internal/kyc/admin.go:428` | `kyc_rejected` | SELLER | 交易必要（治理） | `ACCOUNT` | 不聚合 | #70 `merchant.verification.rejected` | 帶退回理由 `Reason`（`kyc.go:404`，事件註解「目前沒有訂閱者」）。 |
| §8.3#5d | seller KYC | `seller.KYCCancelled` | `seller/internal/kyc/kyc.go:474` | 不做 | - | - | - | - | 無 | 賣家自己取消，自我通知無意義。 |
| §8.3#5e | payment | `payment.PayoutAccountChanged` | `payment/internal/account/sync.go:93` | `payout_account_activated` | SELLER（`StoreID`） | 交易必要（撥款） | `PAYOUT` | 不聚合 | 無（#79 `payout.completed` 是撥款完成，不是這個） | 每次同步都發（`payment/internal/model/model.go:112` 註解），所以型別只在 `FirstActivation` 為真時建立。D23（Will 2026-11-24）：事件新增 `FirstActivation`（只在轉帳能力第一次啟用的那次同步為真，型別以它建立，冪等鍵是店家）與 `RequirementsDeadline`（Stripe v2 帳戶 `requirements.summary.minimum_deadline.time`，沒有時是 nil）；payment 同步時存 `requirements_deadline` 與 `first_activated_at`。補件期限提醒另立型別 `payout_account_deadline`（交易必要、`PAYOUT`），每小時掃描的 job 在期限前 7 天、1 天各排一次，冪等鍵＝帳號＋期限＋檔位。`BankFingerprints` 不得進意圖參數。 |
| §8.3#6 | procurement | `procurement.RfqPublished` | `procurement/internal/rfq/publication_execute.go:110` | `rfq_invited` | SELLER（`Invites[].StoreID`） | 互動【推】 | `PROCUREMENT` | 不聚合 | #93 `procurement.rfq_publication_invite.created.seller`（#94 外部廠商版無單體事件，見下節） | 公開池沒有邀請（`rfq.go:80` 註解：收件人由 A9-3c 的 relay 決定），所以此事件只通知有 `Invites` 的店家；公開池通知隨 A9-3c。D4 沒列詢價，歸互動（可關）是【推】。買家（`BuyerID`）是發布者，不是收件人。 A8-3c：偏好分類是 D29 新增的 `PROCUREMENT`（STORE、預設開、可關）；賣家端沒有路由，`deep_link` 空。 |
| §8.3#7a | social、review、im（處分的擁有者，D24） | `social.ContentHidden`、`review.ReviewHidden`、`im.MessageHidden` | `social/internal/sanction/sanction.go` 的 `Hide`、`review/internal/write/write.go` 的 `Hide`、`im/internal/write/write.go` 的 `Hide`（同 transaction，只在新增一筆隱藏時發，冪等重複不發） | `content_hidden` | BUYER（內容作者；social 與 review 的作者一定是買家，im 帶 `AuthorKind`，店家寄的訊息只能走內容下架措施，型別不收 STORE 作者）；停權沿用 §8.3#1a、店家內容下架沿用 §8.1#10 | 交易必要（治理） | `ACCOUNT` | 不聚合 | 無（新型別） | D24（Will 2026-11-24）取代計畫原句「內容被隱藏」由 moderation 發：處分由擁有者在同一個 transaction 發，社群後台直接隱藏也涵蓋。欄位只有參照與列舉：處分 ID、內容參照、作者參照、`policy.Category`、案件參照 `SourceRef`、`AppealPath`（D25：買家一律 `NONE`）、`EventID`、`OccurredAt`，不帶管理員說明文字。 |
| §8.3#7b | social（警告的擁有者，D24） | `social.BuyerWarned` | `social/internal/sanction/sanction.go` 的 `Warn`（同 transaction，每寫一筆警告發一次） | `author_warned` | BUYER（`BuyerID`；店家的警告走 §8.3#4a） | 交易必要（治理） | `ACCOUNT` | 不聚合 | 無 | D24：只有 social 有警告；review、im 沒有警告。欄位同 7a，沒有內容參照。 |
| §8.3#7c | moderation（案件的擁有者，D24） | `moderation.CaseClosedEvent`（型別名不用 `CaseClosed`：根套件已有同名的案件狀態常數） | `moderation/internal/handle/handle.go` 的 `Decide`（結案同 transaction；違規與不違規都發；版本衝突回滾不發） | `report_resolved`（與 §8.3#2b 同型別，參數 `ReportSource`＝MODERATION） | BUYER 或 SELLER：每位檢舉人一則，檢舉人可為買家或店家；worker 以 `moderation.CaseReporters` 分頁讀檢舉人 fan-out | 交易必要（治理） | `ACCOUNT` | 不聚合 | 無 | D24 取代計畫原句「案件結案（帶每位檢舉人帳號）」。欄位：案件 ID、對象種類、結果類別（`OutcomeViolation`、`OutcomeNoViolation`）、政策分類（只在違規時有值）；不帶處置種類（對檢舉人隱私）、不帶檢舉人清單。封鎖雙方不送（A7-3）。與 §8.3#2b（seller 的店家檢舉）是不同模組的檢舉。 A8-3c：買家與店家檢舉人各一個母意圖 `moderation_case_closed_buyers`、`moderation_case_closed_stores`（子型別 `report_resolved`、`store_report_resolved` 的受眾不同）；依 D24 的補充（每個受眾一版）；每個母意圖只讀自己那一邊的檢舉人，Target.ID 與游標是檢舉人號碼（buyer_id 或 store_id；一案一位檢舉人一筆），每頁從游標接著讀。 |
| §8.3#7d | moderation（案件的擁有者，D24） | `moderation.ReportReceived` | `moderation/internal/report/report.go` 的 `Submit`（新增一筆檢舉的 transaction；併入自己既有的檢舉與同一個 `request_id` 的重送不發） | `report_received`（與 §8.3#2a 同型別，參數 `ReportSource`＝MODERATION） | BUYER 或 SELLER（`ReporterBuyerID`、`ReporterStoreID` 恰好一個為正） | 交易必要（治理） | `ACCOUNT` | 不聚合 | 無（新型別） | D24：與 seller 的 `ReportReceived` 對稱。欄位：檢舉 ID、案件 ID、對象（種類加 ID）、檢舉人、檢舉類別；不帶說明文字。 A8-3c：依 D24 的補充，買家檢舉人 `report_received`、店家檢舉人 `store_report_received`（同參數與文案，都寄確認信，見 §8.3#2a）。 |
| §8.3#7e | social、review、im（處分的擁有者，D24） | `social.ContentRestored`、`review.ReviewRestored`、`im.MessageRestored` | 各模組的 `Restore`（同 transaction） | `content_restored` | BUYER（內容作者，同 7a） | 交易必要（治理） | `ACCOUNT` | 不聚合 | 無（新型別） | D24 的「恢復」：管理員撤銷隱藏要告知作者。欄位同 7a，`SanctionID` 是被還原的隱藏紀錄。 |
| §8.3#8 | review／im 改由 notification 直接消費 | - | 見 §8.1#14、#16、#17 | 同 §8.1#14（`chat_message`）、#16（`review_created`）；#17 不做 | 見各列 | 見各列 | 見各列 | 見各列 | #50、#51、#84 | 這是「刪 `imrelay`、`reviewrelay` 的 notification 一半」的工作項，不是新事件；`reviewrelay` 給 Java product 的評分聚合那一半保留（A6 延後項，計畫 A8-3「不能動」）。 |
| §8.3#9 | identity 的 email 來源、Clerk webhook | -（不是事件） | `identity/AGENTS.md` 的寄件地址 API；Clerk 已移除 | 不做 | - | - | - | - | 無 | 不是通知事件。Clerk 已移除，寄件地址改用 `identity.ContactEmail`（計畫「前提」；`identity.go:255`）；屬 A8-3 email adapter，不在本表。 |
| §8.3#10 | `notifications/stream` 改 server streaming | -（不是事件） | 無 | 不做 | - | - | - | - | 無 | 是 A8-2 的 API 範圍（串流只傳失效訊號，D11），不是通知型別。 |
| §8.3#11 | 賣家瀏覽／追蹤通知節流／聚合 | - | 見 §8.1#8、#12 | 追蹤：同 §8.1#8（`store_followed`，`follow:{store_id}`）；瀏覽：不做 | SELLER | 互動 | `SOCIAL` | `follow:{store_id}`；瀏覽不做 | #44、#48 | 追蹤用聚合鍵（D12）；瀏覽依 D3 不做。 |
| §8.3#12 | 食安事件通知近 6 個月購買者 | -（前提 B8，事件不存在） | 無 | 不做（延後） | - | - | - | - | 無 | 計畫「延後」：觸發是 B8。 |

## 沒有單體事件的舊模板（隨發送端搬遷，D13）

下列 §5.4 模板（73 列、55 個 routing key）目前的發送端不在單體，處置一律「隨發送端搬遷（D13）」：發送端搬進單體（B2–B10、A5、A9）時由「模組完成的定義」新增條目定義型別或裁決不做；不寫 `internal/legacy` 轉接。以 routing key 列出，括號是該 key 的模板列數。

- `aftersales.appeal.submitted`（2 列）
- `aftersales.case.applied`（2 列）
- `aftersales.case.auto_completed_confirm_receipt_timeout`（1 列）
- `aftersales.case.auto_completed_return_pending_timeout`（2 列）
- `aftersales.case.auto_completed_store_pickup_timeout`（1 列）
- `aftersales.case.auto_escalated`（2 列）
- `aftersales.case.auto_rejected`（1 列）
- `aftersales.case.cancelled_by_buyer`（2 列）
- `aftersales.case.cancelled_by_confirm_receipt`（1 列）
- `aftersales.case.coupon_compensated`（2 列）
- `aftersales.case.refund_approved`（2 列）
- `aftersales.case.rejected`（2 列）
- `aftersales.case.return_approved`（2 列）
- `aftersales.case.return_at_store`（1 列）
- `aftersales.case.return_at_store_24h`（1 列）
- `aftersales.case.return_delivered`（1 列）
- `aftersales.case.return_delivered_24h`（1 列）
- `aftersales.case.return_failed`（2 列）
- `aftersales.case.return_in_transit`（1 列）
- `aftersales.case.return_pending_24h`（1 列）
- `aftersales.case.return_received`（2 列）
- `aftersales.platform_decision.made`（2 列）
- `aftersales.refund.awaiting_buyer_page`（1 列）
- `aftersales.refund.completed`（2 列）
- `invoice.allowance.issued`（1 列）
- `invoice.complaint.created`（1 列）
- `invoice.complaint.overdue`（1 列）
- `invoice.complaint.responded`（1 列）
- `invoice.issue_failed`（1 列）
- `invoice.issued`（2 列）
- `invoice.operation.review_required`（1 列）
- `invoice.receipt.issued`（1 列）
- `invoice.reissued`（1 列）
- `invoice.resolved_externally`（1 列）
- `invoice.seller_tax_profile.status_changed`（1 列）
- `invoice.voided`（1 列）
- `coupon.expiring`（1 列）
- `coupon.issued`（1 列）
- `merchant.verification.forwarded`（1 列）
- `merchant.verification.pending_info`（1 列）
- `merchant.verification.under_review`（1 列）
- `notification.announcement`（1 列）
- `order.cancelled`（3 列）
- `order.completed`（2 列）
- `order.paid`（1 列）
- `payout.completed`（1 列）
- `product.certification.approved`（1 列）
- `product.certification.rejected`（1 列）
- `product.certification.revoked`（1 列）
- `shipment.anomaly.detected`（1 列）
- `shipment.created`（1 列）
- `shipment.delivered`（2 列）
- `shipment.status.changed`（1 列）
- `payment.rescue.refund_completed`（1 列）
- `procurement.rfq_publication_invite.external_created`（1 列）

說明：
- `notification.announcement`（#72）由 A8-2 的後台公告取代，不是事件型別。
- `merchant.verification.under_review`／`forwarded`／`pending_info`（#71、#68、#69）：單體沒有對應事件（`sellerrelay/relay.go` 明寫不轉送；KYC 只有 Submitted、Approved、Rejected、Cancelled 四個事件，見 §8.3#5）。
- `procurement.rfq_publication_invite.external_created`（#94，外部廠商）：發送端是 ai-service outbox，不是單體。
- `coupon.issued`（#66）發布端是死碼（`inventory.md` §3.4）。

## 不在這 34 列、但單體現在有發送端的事件（未裁決，供 Will 看）

這些不在 §8.1／§8.3，所以不在 D3 範圍。我查到 `events.Publish` 存在、可能有使用者需要知道，列出避免漏看；沒有【推】以外的結論。

- `seller.StoreClosed`／`StoreReopened`（`seller/internal/closing/closing.go:260`、`:391`）：店家關店與恢復營業，賣家可能需要知道。【推】
- `seller.StorePendingDeletion`（`closing.go:105`）、`buyer.BuyerDeletionRequested`（`buyer/internal/deletion/deletion.go:65`）：刪帳申請確認。【推】
- `seller.StoreOpened`（`seller/internal/onboarding/onboarding.go:118`）：賣家自己開店，自我通知無意義。【推】
