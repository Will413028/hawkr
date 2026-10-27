# buyer 模組的沿用盤點

從 buyer-service 沿用的每個機制重新判斷的結論（遷移計畫「模組完成的定義」），以及與 buyer-service 的差異。模組現行的職責、資料與不變量在同目錄的 `AGENTS.md`。

## 沿用盤點

依 buyer-inventory §2、§2.1 與遷移計畫的設計決定。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| TSID／serial／固定方案 ID | 多服務發號 | 不成立，Java BIGINT 仍成立 | row-identity ADR：實體 UUIDv7、關聯自然鍵 | buyer/address IDENTITY；membership UUIDv7；刪帳申請 UUIDv7 由 accountdeletion 擁有；restock 複合自然鍵；plan_code 與關聯複合鍵 |
| stint 與建立衝突重讀 | 同一帳號關閉後可重新成為買家，需隔離歷史 | 不成立：建帳就是買家（W1），後台只刪賣家或整個帳號（W3），帳號活著 ⇔ 買家活著；刪除的帳號留墓碑，同一 Clerk 使用者不會再建帳號 | 一個聚合一列、以唯一約束保證 | 不再有重新建立：`UNIQUE (account_id)`，建帳事件內單純 INSERT（不授予角色，W5），衝突即錯誤回滾；刪掉登入時的衝突重讀與 CLOSED 重試。status 仍區分 ACTIVE／PENDING_DELETION／CLOSED（等待訂單與 PII 清理需要） |
| account_id 寫入／PII | 一帳號只有一列 | 成立（W3：一個帳號一生一個買家），但 Java 與事件仍以 buyer_id 為業務鍵 | aggregate ID 定位 | 所有業務寫入與清理改用 buyer_id |
| buyer_id 與 account_id 各自發號 | 買家是 buyer 這個 bounded context 的聚合 | 成立 | 每個 bounded context 擁有自己的 ID；對外看得到的業務 ID 不暴露登入用的 principal ID | 維持各自發號（Will 2026-10-01 W6）：buyer_id 會出現在賣家看得到的地方（粉絲、CRM、訂單），登入用的 account_id 不外流；本機 seed 刻意讓兩者不同（`buyer_id = account_id + 50000`），互換的缺陷才藏不住 |
| 收藏 Redis set | 跨服務熱點快取 | 未有量測，單體同 DB | 索引直接查 | 移除，複合 PK 查 membership |
| 地址 Redis 快取 | TGOS 查詢昂貴 | TGOS 已決定移除 | 移除無消費端快取 | 不搬 |
| TGOS 與 GCP proxy | 地址查詢 provider | 舊公司不可連，決定 9a | 不建立無需求整合 | 不搬，步驟 3／5 移除入口 |
| gateway 按角色分派地址查詢 | 兩服務各一實作 | 不成立 | 單一擁有者 | 隨 TGOS 刪除，步驟 3／5 |
| outbox／fx scheduler／監控 | 跨 DB 與 MQ | 單體不成立 | events.Publish 加 River | 移除 outbox；legacy relay 步驟 4。舊 outbox 的「卡住」「終態失敗」告警隨 buyer-service 刪除：`buyer.deleted` 遺失會讓五個舊服務不清買家 PII，所以要以 River 的 `discarded` job 告警取代（`river_job.state = discarded` 或 `river.Config.ErrorHandler`）。觸發條件：第一次部署之前（wiki 的 CD 待辦） |
| goroutine 與 best effort 事件 | 容許丟事件 | 收藏計數不允許 | 同 transaction publish | 收藏、追蹤、瀏覽皆原子發布 |
| AMQP confirm／tx 內 fan-out | 跨服務通知 | 模組內不成立 | tx 事件與持久 job | FanOut 有界 DELETE RETURNING 加事件；relay 步驟 4 |
| restock dedup 表 | 重送 MQ | 狀態與 job 已可去重 | River unique job 加業務狀態 claim | `product.restock` 以 event_id 排 River unique job；River v0.47 預設含 completed 狀態、completed job 保留 24 小時，因此此去重窗是完成後 24 小時，長期冪等由 FanOut 的訂閱刪除 claim 保證。relay 排入的送出 job（`legacy.rabbitmq_publish`）以 event_id 做 River unique：relay job 排入後才失敗而重跑時不重複送出（同樣的 24 小時窗；排入是 relay 的最後一步，重跑緊接在失敗之後） |
| 每日一次 100 筆 PII cron | 過期資料清理 | 成立，單批不足且重啟不能重置等待 | 啟動補跑、時間窗去重、有序分批至耗盡 | River 每小時加 RunOnStart、ByPeriod=time.Hour；CLOSED 嚴格超過 30 天、buyer_id、注入 clock |
| 地址筆數不設上限，client 讀到底 | 舊 REST 回整份清單 | 不成立：Connect 清單分頁，client 讀到底要靠「地址很少」的假設 | 設明確上限，清單一頁完整 | 每個買家最多 20 筆（W4），新增時在買家列鎖內檢查、超過回 failed_precondition（要先刪掉一筆，〈API 設計規範（Connect）〉§八）；上限寫在 proto 的 CreateAddress／ListAddresses 說明；清單預設頁大小 20，client 照 next_page_token 讀到底，上限調整時不會被截斷 |
| 地址平鋪中文行政區字串（city、district、detail_address） | 儲存使用者自由輸入之縣市與行政區字串 | 不成立：中文行政區字串存在異體字（台／臺）、同名與改制問題，且無法精確關聯統計與地理資訊 | 結構化行政區代碼（如內政部代碼）加明細字串 | 改存內政部 8 碼鄉鎮市區代碼（town_code，CHECK 8 碼數字）與 1–255 字元明細（address_detail），中文縣市行政區與郵遞區號由 platform/twaddr 統一推導；對外 API 共用 PostalAddress 訊息；legacy 邊界轉為中文回傳（ADR 2026-10-04-address-town-code） |
| 結帳的郵遞區號由 client 送出（Java order 依它算運費區域） | 舊地址存中文名稱、郵遞區號由 client 手抄表查 | 成立到 B 階段：order 在 Java，結帳的請求由 client 組，單體不在路徑上 | 伺服器依地址 ID 推導，不信任 client 宣稱的值 | 8j 起 client 由 `town_code` 經生成的對照表推導（website、buyer-app 同一份資料）；B 階段改寫 order 時改由伺服器依 `address_id` 推導，client 不再送郵遞區號 |
| RoleState、source_id 與 BUYER 角色 | 跨服務遲到撤銷；買家是可單獨退出的角色 | 不成立：帳號活著 ⇔ 買家活著，買家不是角色（W5） | 由不變量推導的屬性不另外儲存 | 刪除：建立與關閉買家都不碰 identity 的角色，也不自己鎖帳號；關閉只由 accountdeletion 在已鎖帳號列的 transaction 內呼叫（鎖順序帳號先於買家），不訂閱 `AccountDeleted` |
| EnsureProfile fail-open grant | 分散式補角色 | 不成立 | 原子 transaction | 買家在建帳的 transaction 內建立，失敗時建帳整筆回滾；沒有登入時的補建 |
| GetAccountInfo email/phone 副本 | 買家需要聯絡資料 | phone 是自行維護，email 不成立 | 歸屬清楚、避免副本 | 移除 email；phone 由買家編輯 |
| 預設暱稱 math/rand | 無 token 名稱 | 成立 | 注入隨機來源供測試 | 保留格式；NicknameNumber 注入，僅新建時使用；預設用 crypto/rand，符合現有 gosec gate |
| offset／id cursor | 舊頁碼契約 | 新契約不成立 | AIP-158 keyset | 時間加 ID cursor，platform.PageSize |
| PayUni 姓名與缺欄位驗證 | 物流字寬規則 | 姓名約束成立 | 邊界驗證加 DB CHECK | 保留 ASCII 1／其他 2、4–30；補 gender/phone/date 驗證 |
| 電話採本地格式 | 台灣市場、物流商與 legacy Java 讀者使用本地號碼 | 成立；CRM（決定 10a、A6）須依單一正規格式精確查詢 | 台灣限定採 09xxxxxxxx／區碼格式；跨國系統採 E.164 並於邊界格式化 | platform/phone 統一本地格式；需要接受非台灣號碼時改存 E.164、邊界轉換 |
| bizcode／DomainError／gRPC status | legacy wire | 只在過渡邊界成立 | apperr 分類，傳輸層轉換 | 模組用 apperr，ErrorReason 步驟 3 |
| 頭像 URL、account ref 與獨立 UpdateAvatar RPC | 媒體顯示與引用；頭像與 profile 分開更新 | 不成立：URL 非資料、ref 應屬 stint；頭像與個人資料屬同一 aggregate，獨立 RPC 造成併發競態與預檢無效 token 消耗 | object key 與業務 ID 引用；併入 Profile 更新 | 只存 key，BUYER_AVATAR ref_id=buyer_id；移除獨立 UpdateAvatar RPC，併入 UpdateProfile（接受可選 avatar_staged_token），轉正前先預檢驗證與版本，單一 CAS transaction 寫入文字與頭像，舊圖由 buyer.media_sync 退場機制釋放（ADR 2026-10-03-media-references-owner-side） |
| HMAC interceptor | 跨服務認證 | 單體不成立 | 入口認證 | 模組不搬；legacy 步驟 4 |
| fx／OTel／go-pkg server／health | 獨立程序 | 不成立 | 單體組裝與平台 | 刪除服務框架，main 組裝 |
| sqlc 加手寫 principals SQL | 型別安全查詢 | 成立 | sqlc 管所有查詢 | 全查詢 sqlc，db.Conn 跟隨 transaction |
| Atlas 公司 IP | 舊公司資料庫 | 不成立 | 本機 Atlas desired state | 使用本機開發資料庫產生 migration，部署以版本化 migration 套用 |
| BuyerEventType | 跨語言事件 | legacy 才成立 | 具名事件帶 UUIDv7 | FavoriteAdded/Removed；步驟 4 轉 legacy |
| ProductEventType | restock MQ | legacy 才成立 | 狀態轉移事件 | RestockTargeted；consumer 步驟 4 |
| MembershipEventType／BillingErrorCode | 會員計費 | 未接上 | 不搬無消費端功能 | 刪除，沒有 payment 依賴 |
| AssetReferenceType BUYER_AVATAR | media 契約 | 成立 | media 公開常數 | 保留 media.RefTypeBuyerAvatar |
| OrderStatus 字串未結集合 | 跨服務訂單讀取 | buyer 不擁有 | caller 查詢後輸入 | accountdeletion 的 Orders port 以 buyer_id 查詢，buyer 不依賴 ordering |
| BuyerStatus／MemberLevel／source/status／gender | 封閉值域 | 除 MemberLevel/outbox 以外成立 | 寫入規則與 CHECK | 移除 MemberLevel/outbox；其餘 CHECK 與測試 |
| 9203／9204 | 刪除中狀態分支 | 成立 | FailedPrecondition | 模組分類，步驟 3 ErrorReason |
| 9400 | 無效方案 | 成立 | InvalidInput | 指派回分類錯誤，步驟 3 reason |
| 9700 | 停用地址查詢 | 功能刪除 | 刪無用錯誤 | 步驟 3／5 移除 |
| 9233 | 補店家資料 | buyer 不再補資料 | httpapi 查 seller | 步驟 3 處理 |
| NotFound／Forbidden／InvalidInput／Conflict | 錯誤分類 | 成立，地址越權不曝露 | apperr 與 NotFound ownership | NotFound 隱藏他人地址；create conflict 內部重讀 |
| 會員月週期與人工天數指派 | ADR 09-18 | 成立 | 錨點月 clamp；人工指派明確天數 | 保留不回補 clamp，付費過期回 free，注入 clock |
| 地址 default false 覆寫 | 可取消預設 | 有地址就須有預設 | 鎖 aggregate 加唯一約束 | 原預設不因 false 清空，改選別筆才切換 |
| 換圖後 best-effort 釋放（以 keep 名單「其餘全刪」） | media 在 B1 前遠端、引用在另一個系統，需要在「刪引用」與「並發加引用」之間建立先後 | 成立到 B1；「其餘全刪」不成立：keep 是快照，會刪掉已登記、還沒 commit 的新圖 | 引用與業務列同一個 transaction（Active Storage）；引用在別的系統時，以單一寫入者序列化並只精確移除被取代的資產 | 換圖在 transaction 內記退場、排入 `buyer.media_sync`；同步 job 是頭像引用唯一的寫入者，Link 目前的、精確 Unlink 退場的（`docs/adr/2026-10-03-media-references-owner-side.md`）；B1 同一個 transaction 後消除 |
| 同步 job 以 unique 加 seq 版本（沿用 payuni 的 `intent_seq`／`synced_seq`）去重，只搬了版本、沒有搬保底掃描 | 執行期間的改變不能靠重新排入（唯一鍵含執行中） | 版本成立；只有版本不成立：結算 commit 之後、River 標記完成之前排入的新 job 被略過會遺失更新，job 被丟棄也沒有人補排 | transactional outbox 的 sweeper／polling publisher（payuni 的 `payuni_sweep` 是同 repo 的先例） | 結算與 `river.JobCompleteTx` 同一個 transaction（鎖序：版本列、再 river_job），加每 15 分鐘的 `*.media_sweep` 補排 `synced_seq < seq` 且閒置的 ref；監控是擁有者端的落後時間，不依賴 River 的 job 列（`docs/adr/2026-10-03-media-references-owner-side.md`）。seq 協定、拒絕寫回與結算只有一份，在 `media.RefSyncer` |
| media client 內層重試 Link（3 次，沿用 go-pkg/media），並複製給 Unlink | 舊的 Link 在請求路徑裡，呼叫方沒有持久重試 | 不成立：唯一的呼叫者是 River job，job 有持久重試；內層重試只讓逾時之後才在遠端生效的請求變多 | 只在一層重試，背景工作交給 queue | 移除，`Link`、`Unlink` 都是單次呼叫加 5 秒逾時（`internal/legacy/mediaservice`） |
| `FinalizeAndLink(…, ref)` 的 ref 在轉正時就要給 | Java 的 `FinalizeImages` 在建立資產時寫上 ref_type | 部分成立：ref_type 還是要給（Java 不變），ref_id 不需要 | media 不認識業務（B1 的終點） | `Finalize(ctx, uploader, token, refType)`；B1 之後 ref_type 也消失 |
| 解除時間常數：第二次解除前等 30 秒、job 逾時 45 秒（以 Link 含重試約 16 秒推算） | 收掉逾時之後才在遠端生效的 Link | 前提不成立（內層重試已移除） | 以一次呼叫的逾時加餘裕推算，並說明遠端遲到沒有上界 | `HealDelay` 15 秒（一次呼叫 5 秒加排隊餘裕 10 秒）、`JobTimeout` 30 秒（Link 與 Unlink 各 5 秒加資料庫與餘裕）、退場保留 2 小時（staged token 最長有效期的兩倍）；常數與推導在 `refsync.go` 與 ADR |
| 儲存代碼的表示法 | 模組擁有封閉值域，須與 DB 約束一致 | 成立，ADR CodeMeta D3 | Go 具名字串型別與 UPPER_SNAKE 常數，加 DB CHECK 與集合比對測試 | 全部儲存列舉採 UPPER_SNAKE；步驟 3 驗證 proto 雙向 mapping，legacy 在邊界轉換 |
| request_id 冪等與 advisory lock | 跨買家同 request 併發 | 不成立：後台指派入口依計畫不搬，`AssignMembership` 沒有呼叫方，`memberships` 不會被寫入，entitlement 一律是預設方案 | 沒有指派入口就不保留寫入路徑（D8）；計費或指派入口要做時重新設計 | Will 2026-10-01 決定刪除寫入路徑（`AssignMembership`、`memberships` 表與測試），entitlement 固定回預設方案，方案表與 admin 的方案管理保留。觸發條件：A3b 收尾（步驟 11）之前或計費設計開工，先到者 |
| display_name 空值與 FillName | 舊資料有空暱稱待補 | 不成立，新建一律提供合法名字 | DB 約束未清 PII 時 1–100 字元，清理後 NULL | 移除 DEFAULT 與 FillName；墓碑字串由步驟 3 httpapi／步驟 4 legacy adapter 呈現 |
| 粉絲週統計 | 台灣賣家按日曆週比較新增數 | 原始計數成立；週日起的週與「本週到目前比上週整週」不成立（比較期不等長，週初必然大跌） | 等長、緊鄰的比較期，以店家時區切日（ShopifyQL `COMPARE TO`、vocus 的創作者統計），回原始值 | `SubscriberStats(storeID, windows)`：總數與各時間窗的新增數，界線由呼叫者給；賣家儀表板（8d）用最近 7 個完整台北日對前 7 日（Will 2026-10-03）。只為 gateway 舊儀表板存在的週日週由 legacy buyerserver 自己算（`TestSubscriberWeeksFollowTheTaipeiSunday`），gateway 的 `/app/sellers/me/dashboard` 路由已在 A3b 第 4B 刀刪除，但成長計算屬 `GetSubscriberDashboardStats` 的回應，該 server 因 seller-service 還讀 Total 而保留（K23），週日週與測試隨它在步驟 11 一起刪；`TestSubscriberStatsCountsEachWindowHalfOpenForTheStore` |
| 補貨終態歷史 | 有已通知或取消歷史讀者 | 不成立；舊 ListActiveRestockSubscriptionsByBuyerID 與 ListRestock／BatchCheck 只讀待通知 | 純關聯複合主鍵，取消與通知後刪列 | PK (buyer_id, product_id)，無 UUID／status／notified_at；有界 FanOut 事件與刪除同 tx |
| 刪除稽核來源與原因 | 需辨識來源並保留操作者文字 | 成立；account_id 可由 buyer_id 推得 | trigger 封閉列舉、reason 自由文字、同模組 FK | 申請與稽核改由 accountdeletion.deletion_requests 擁有；buyer 只維護 stint |
| 每次瀏覽發 ProductViewed | 商品瀏覽原始計數與通知來源 | 成立；設計書 39 的 visit 事件、13 的「瀏覽您的商品」、45 的 engagement，以及 seller A3b 計數均需要 | buyer 發原始事件，下游各自聚合 | 每次 RecordVisit 都發，事件不帶店家（商品所屬的店家不在請求路徑上解析：RecordVisit 因此不依賴商品服務；seller 7c 的重做，取代階段 4 把 `StoreID` 經入口帶進事件的做法）；seller 以事件追加原始瀏覽事件，店家由它的背景 job 解析（見 seller 的 CARRYOVER）；seller 通知節流／聚合屬 notification A8，依下列設計書 |
| visit_history 不設筆數或期限 | 每個 buyer×product 僅需最後瀏覽時間 | 成立；upsert 使每 stint 筆數受 catalog 大小限制 | 複合鍵保留最後時間；以量測決定 cap／TTL | 維持無 cap／retention。觸發條件：單一買家超過 1,000 列，或 `ListVisits` 的 p95 超過 200 ms；量測要等部署後的監控（見第 19 列），第一次部署時一併加上這兩個指標 |
| 地址標籤（label：HOME／COMPANY／OTHER，REQUIRED＋CHECK） | 有人設定（使用者選）或有人讀（畫面或下游） | 不成立：沒有畫面設定也沒有讀者，buyer-app 的選擇器 2026-07-16 品牌改版刪除（`d09f9fa61`），值全由 client 編造；legacy gRPC 轉送但沒有讀者 | ［外部，未查證］零售平台的地址標籤是使用者選的選填欄位；沒有功能就不存欄位 | 刪除（5c F1 α）；產品要讓買家替地址分類時以選填加回，同時做選擇器與顯示 |
| buyer-app 地址 ID 以 `int` 保存（`address_repository.dart` 的 `int.parse`） | 契約的地址 `id` 是整數 | 不成立：契約 `id` 是不透明字串（AIP-122），`int` 是舊 REST 時期的形狀 | ［外部，AIP-122，本次未重新查證］業界把資源 ID 當不透明字串，client 不解析 | 保留；buyer-app 結帳改走 Connect（ordering 搬進單體）時改成 `String` |
| 性別的「沒有填」（`GENDER_UNSPECIFIED` 當清除） | 契約要能表達沒有填 | 成立，但零值承載意義，與 Q4 相反 | AIP-126 零值不使用、AIP-149 只建議數值用 `optional`［外部］；本庫依 Q4 以欄位有沒有設值表達（`API 設計規範（Connect）.md` 的 `optional` 條） | 改 `optional`，`UNSPECIFIED` 回 `invalid_argument`（5d G1 α） |
| 封鎖主鍵 tsid（舊 `social-service` 的 `user_blocks`） | 多服務各自發號 | 不成立；封鎖沒有 Java 消費端的 BIGINT 主鍵 | row-identity ADR：關聯用自然鍵的複合主鍵 | 複合主鍵：`blocked_buyers` (blocker_buyer_id, blocked_buyer_id)、`blocked_stores` (blocker_buyer_id, store_id)，加反向索引；沒有代理主鍵 |
| 封鎖名單 OFFSET 分頁（`ListBlocks(limit, offset)`） | 名單可能很長，翻頁不重不漏 | 不成立：OFFSET 隨頁數變慢，翻頁中新增或解除會重複或漏列 | AIP-158 的 page token（keyset） | `ListBlockedBuyers`、`ListBlockedStores` 以 (created_at, 對象 ID) 降冪 keyset 分頁，cursor 驗證合法性 |
| 混合號碼空間：單表的 `blocked_id` 同時放現任 buyer_id、store_id 或舊 account_id，寫入時用 `resolvePrincipal`（舊 `usecase/block.go`）逐軸反解 | 舊 social 不知道號碼屬於買家還是店 | 不成立：新系統的買家 ID 與店家 ID 各自發號、呼叫端知道對象的種類 | 對象種類不同就分表，各表的對象欄位帶自己的外鍵或約束（多型關聯建不了 FK） | 兩張表：買家封鎖買家（`blocked_buyer_id REFERENCES buyers`、CHECK 不能封鎖自己）、買家封鎖店（`store_id` 跨模組不建 FK、CHECK > 0）；反解不搬。「同一人以買家與賣家兩個身分」的跨身分封鎖由 A7-2 的 handler 經 identity 判斷後拒絕（買家封鎖自己的店、店家封鎖店主本人），buyer 只擋同表的自我封鎖 |
| 封鎖名單的 Redis 集合（熱路徑讀） | 跨服務熱點快取 | 不成立：單體同 DB，讀取是主鍵或反向索引的 EXISTS | 索引直接查（social 的 M4、M11 同結論） | 移除；social 的查詢以 `pub_blocked_stores()`、`pub_blocked_buyers()` 在 SQL 述詞內檢查（`TestPubBlocksIsInlinedAndUsesTheIndexes`），in-process 呼叫走 `IsBlockingStore`、`IsBlockedBetween`（social 的 M4 同一結論） |
| 買家刪帳後封鎖列的去向 | 對象退場後名單與過濾是否保留他 | 不成立：墓碑對象的列沒有讀者（名單顯示不出內容，過濾對一個已不公開的對象沒有效果） | 帳號刪除時一併清除與該帳號有關的關聯（雙向） | 買家刪帳時 buyer 在自己的刪帳 transaction 刪他發起的兩種封鎖與以他為對象的 `blocked_buyers` 列；各店對他的封鎖由 accountdeletion 在同一個 transaction 刪（seller 擁有）。店退場後以店為對象的列保留 |
| 封鎖與並發寫入的鎖 | 封鎖生效後不應再有新內容寫入 | 不成立為強保證：封鎖與他人的寫入各自提交，READ COMMITTED 下封鎖提交前開始的寫入可以成功 | Mastodon 的封鎖與寫入不互鎖，寫入前檢查 | 封鎖與他人的寫入不互鎖：寫入端在 transaction 內檢查封鎖（social），封鎖提交之後開始的寫入被擋。封鎖買家則在同一個 transaction 依 buyer_id 升冪鎖發起者（排他）與對象（共享），鎖到後才檢查對象狀態，所以不會留下指向已關閉買家的列，兩人同時互相封鎖也不互等（`TestMutualBlocksDoNotDeadlock`、`TestBlockingAClosedBuyerFails`）。重新評估條件：營運出現「封鎖之後仍寫入成功」的檢舉 |

瀏覽事件依據：[設計書 39](../../../docs/詳細設計書/39-買家服務（Buyer%20Service）詳細設計書.md) 的 visit／product.viewed、[設計書 13](../../../docs/詳細設計書/13-通知模板引擎與收件箱（Notification%20Template）詳細設計書.md) 的賣家通知模板，以及 [設計書 45](../../../docs/詳細設計書/45-首頁區塊動態排序與營運干預（Homepage%20Dynamic%20Layout%20&%20Merchandising）詳細設計書.md) 的商品 engagement 聚合。

## 傳輸層的沿用盤點

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| gateway buyer handlers 與補資料 | REST 舊入口 | 切換前成立 | Connect handler 組裝讀取 | 步驟 3 建立 Connect、5 刪 REST；步驟 4 legacy 不承接 gateway buyer REST |
| applyRestockSubscribed | catalog 尚在 Java | 成立 | 批次讀取關聯 | BatchCheckRestock，步驟 4 legacy server |
| BatchCheckFavorites／CheckFavorite overlay | catalog 尚在 gateway | 成立 | 批次 membership 查詢 | 公開 API 保留，步驟 4 接線 |
| auth 與 business ID resolver | 入口驗證 | 成立，Redis 不成立 | 入口解一次身分 | 共用入口解析身分；步驟 4 將業務 ID 來源切至模組 |
| envelope bizcode | 舊 client | 僅過渡成立 | Connect code/ErrorReason | 步驟 3 新契約、5 client |
| offset page | REST 頁碼 | 新契約不成立 | AIP-158 | 模組關聯與地址 keyset，Connect 封裝 token；訂單過渡期讀完舊 RPC 的 offset 頁、排序後以雙欄 keyset 對外分頁 |
| 未結訂單的狀態集合 | ordering 擁有訂單狀態與刪帳阻擋規則 | 過渡期成立 | 由 ordering 公開 API 判斷 | 暫讀 CodeMeta 的 OrderStatus 投影；ordering 搬進單體時重新評估，由其公開 API 提供規則，檢查移入刪帳 transaction |
| 會員卡消費排序分頁 | 訂單仍在舊服務 | 過渡期成立 | 由擁有者提供穩定快照或 cursor | 每次請求讀取快照，依淨消費額降序、store_id 降序分頁；跨頁消費額變動可能造成重複或漏項。店家集合有界且 client 下拉刷新，接受此限制；ordering 搬進單體時重新評估 |
| 資源版本與 etag | 前台並行編輯 profile、地址；後台編輯方案 | 成立 | 各 aggregate root 獨立版本，SQL CAS | buyer／address／plan 分別比對；預設地址切換遞增兩側版本；頭像轉正前預檢且寫入時 CAS |
| Redis caching | 跨服務延遲 | 未量測，單體不成立 | 索引查詢 | buyer 模組不加快取 |
| request size／rate limit／trace | 外部入口 | 成立 | 共用 rpc middleware | 步驟 3 使用單體入口 |
| timeout／retry | 遠端呼叫 | media/Java 過渡成立 | client deadline、job 重試 | media 釋放 job 10 秒；步驟 3／4 legacy client 明確期限 |
| social-service 粉絲頁碼 | 舊 gRPC 僅有 page／page_size | 在 social-service 搬遷前成立 | 新契約用 keyset；舊契約用有界單次 offset 查詢 | legacy `GetSubscribersList` 以最多 100 筆、offset 最多 100000 的單次 SQL 查詢轉接；`SubscriberInfo.id` 原是追蹤列 ID，模組是複合鍵、沒有列 ID，所以不填（唯一呼叫方 social-service 只讀 `buyer_id`）；social-service 搬進單體時刪除 `SubscribersOffset` 與 SQL |
| RabbitMQ consumer 的毒訊息處理 | 舊 `pkg/mq` 具 DLQ 與指標；遷移中的 consumer 原本永久錯誤直接 ACK | 非法上游 payload 仍會發生 | 每個 durable queue 綁 dead-letter exchange，永久錯誤記結構化 log 並 NACK 不重排，讓 broker 保留待查；暫時錯誤重排 | 單體 queue 使用 `hawkr.monolith.dead`；order/product consumer 搬進單體後隨 RabbitMQ bridge 刪除；指標由 broker queue depth 提供。單體的 queue、`hawkr.monolith.dead` 與 binding 由 consumer 在執行期宣告，不進 `infra/mq/topology/topology.yaml`（topology 因此有兩個擁有者，重試也不同：yaml 的 queue 延遲重試 3 次再進 DLQ）：yaml 只服務舊服務，過渡期接受；退場條件同上，最後一個 RabbitMQ consumer 搬進單體時兩者一起刪 |
| gateway 以 `GetBuyerProfile` 解析他人的 buyer_id | gateway 的 seller CRM、seller、resolve handler 尚需由 account_id 解析「現任」買家：帳號刪除後要得到 `ErrNoCurrentBuyer` | 僅在這些 gateway caller 搬遷前成立 | 由 buyer 模組的 `CurrentBuyerID` 提供只回 ID 的查詢 | 暫留舊 RPC（Q5 評估過）：一個帳號一生一個買家之後，legacy `ListBuyerIDsByAccount` 最多回一個 ID，但它含已關閉的買家（帳號刪除後仍回；im-service 清 PII 需要這個語意，不能改），gateway 改用它會把已刪帳號的買家當成現任。legacy 契約沒有只回現任 ID 的 RPC，新增要改 go-pkg 的舊 proto 與 gateway，違反「舊服務只做維持運作的修改」。剩下的差別只有 gateway 多讀到一份 profile 個資（不回給 client）；gateway 的 seller CRM、seller 與 resolve caller 搬進單體後改用模組的 `CurrentBuyerID` 並刪除這支 PII 查詢 |
| relay 時讀取店家、商品補 legacy payload | 舊 consumer 需要賣家帳號、商品名稱等欄位 | 過渡期成立（計畫步驟 2 的決定：payload 的店家、商品欄位由 legacy 讀，不是事件當下的快照） | 事件帶齊消費端要的快照 | relay job 執行時讀；店家不存在是永久狀態：收藏、瀏覽缺店家欄位照送（計數依 product_id），追蹤沒有收件人就不送，都不重試；帳號欄位缺值送空字串（舊 publisher 的形狀）。對應 consumer 搬進單體、改訂閱模組事件時刪除這段補資料 |
| buyer 事件轉送舊 RabbitMQ consumer | 舊服務尚未搬進單體 | 過渡期成立 | 每種事件有獨立、可演進的 job args；出口共用發布 job 與重試設定 | 六種事件各有 River kind，補資料後排入 `legacy.rabbitmq_publish`，共用最多 20 次的發布重試。對應 consumer 搬入單體時刪其 job 與轉送；最後一個搬遷後刪 relay |

## 與 buyer-service 的差異

系統從未上線，schema 不受舊資料相容性約束。
- 刪除硬清地址、逐筆發 FavoriteRemoved；pending 期間買家仍可用（收藏、追蹤除外）；一個帳號一生一個買家，關閉後不再新建。
- 買家不是 identity 的角色（W5）：建立與關閉都不授予、移除角色，舊契約的買家位元由 `internal/legacy/accounttype` 合成。
- 新增（A7-1a，buyer-service 沒有）：`pub_inactive_buyers()` 給 social 每日對帳核對它自己的複本（`inactive_authors`；social 在刪帳的同 transaction 訂閱者寫複本並讓內容重新求值，讀取不 join 它）；`BuyerDeletionRequested` 在進入刪除中時發一次，供 social 排清除 job；`LockActive` 讓其他模組的買家寫入與 `BeginDeletion` 序列化。必要條件：舊的 `buyer.deleted` 只在關閉時發，申請刪除到關閉之間內容仍公開、清除 job 與新寫入會競態；單體同 DB 才做得到共享鎖。
- PII 僅清 CLOSED 的單一 stint、排序並分批處理完積壓。
- 事件原子發布且帶 UUIDv7 與注入 clock 的 OccurredAt；會員 request_id 比對內容。
- avatar NULL 表示預設頭像，client 決定呈現，不再儲存 public/defaults/buyer_avatar.png；頭像更新併入 UpdateProfile（avatar_staged_token），移除獨立 UpdateAvatar RPC。
- suggested nickname 超過 100 runes 截斷，空建議名用預設格式；既有暱稱不覆寫；長名稱不阻斷登入。
- 身分、地址與關聯鍵依 row-identity ADR；收貨地址以內政部 8 碼鄉鎮市區代碼儲存（ADR 2026-10-04-address-town-code）；gender、buyer/membership 狀態有 CHECK；地址沒有標籤欄位；restock 無終態歷史。
- 列表不補商品／店家；不保留 Redis、TGOS、通知表、MemberLevel、email 與計費。

## 公司時期未完成項

legacy-backlog 的 A3 段裡屬於 buyer 的項目（A3a 步驟 6 對照現行程式碼）。

| 項目 | 結論 | 依據 |
|---|---|---|
| buyer #91：個人資料的「行政區」「個人簡介」顯示儲存成功，但後端沒有欄位 | 仍成立，未修：後端照設計沒有這兩欄（`Profile` 只有暱稱、電話、性別、生日、城市、職業），buyer-app 的編輯頁仍顯示兩欄、儲存時丟棄並提示「已儲存」。已修：Will 2026-10-01 決定拿掉 buyer-app 的兩欄（profile 契約不擴充）；同類的信箱欄（profile 沒有 email，必填卻被丟棄）改成唯讀顯示 Clerk 的登入信箱 | `api/hawkr/buyer/v1alpha1/buyer.proto` 的 `Profile`；`buyer-app/lib/features/settings/presentation/profile_edit_screen.dart`（district 恆為空、sign 不送）；website 的個人資料頁沒有這兩欄 |
| #2345：仍用 `account_id` 的位置（buyer 範圍） | 已解決：業務表一律以 `buyer_id` 為鍵，`account_id` 只在 `buyers` 的對應欄（UNIQUE）與「由登入者找自己的買家」的查詢；legacy 事件裡的 `account_id`、`seller_id`、`merchant_id`、`follower_id` 是舊契約定義在帳號空間的欄位，照舊送 | `db/schema.sql`；`db/queries/buyers.sql`；golden `tests/contract/golden/event_payload_keys.json` 的 `identity_spaces`；`internal/httpapi/buyer/buyer_integration_test.go`（讓 buyer_id 與 account_id 不同值）、`internal/legacy/buyerserver/server_integration_test.go` 的 `ResolvePrincipals` |
| buyer-app L52：封鎖名單頁永遠無法顯示姓名與頭像 | 仍成立，不屬於 buyer：封鎖名單是 social 的資料，`ListBlocks` 只回 `blocked_id`，而它的命名空間是混合的（現任 buyer_id 或 store_id，查無時退回 account_id），補顯示要先在 social 定下被封鎖者的身分，才能用 `buyer.BatchDisplay` 或 seller 補。隨 A7 social（seller-app L54 同一件事） | `services/social-service/db/schema/schema.sql` 的 `user_blocks`；`buyer-app/lib/features/moderation/data/block_model.dart` |
