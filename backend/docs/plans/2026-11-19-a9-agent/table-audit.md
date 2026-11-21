# A9-3a 沿用盤點審查：procurement 31 表

唯讀審查，worktree `.worktrees/a9-1b-proto`（分支 `a9-1b-proto`），2026-11-21。路徑相對 `backend/`，除非另註。

縮寫：`PS`＝`services/ai-service/db/schema/procurement.sql`；`PLAN`＝`backend/docs/plans/2026-11-19-a9-agent.md`；`CO`＝`internal/modules/procurement/CARRYOVER.md`；`DI`＝`backend/docs/plans/2026-11-19-a9-agent/domain-inventory.md`；`BR`＝同目錄 `decision-brief.md`；`PROTO`＝repo 根 `api/hawkr/procurement/v1alpha1/procurement.proto`；`ADR-S`＝`docs/adr/2026-11-19-ai-service-monolith-scope.md`（repo 根）；`ADR-ID`＝`docs/adr/2026-09-25-row-identity-by-data-kind.md`；`ADR-V`＝`docs/adr/2026-09-25-optimistic-concurrency-etag.md`；`ADR-C`＝`docs/adr/2026-09-25-codemeta-retirement.md`；`AI`＝`services/ai-service`。

標記：【量】＝讀到或數到（附 file:line 或指令）；【推】＝推論。

## 0. 結論

- 31 表：keep 3、reshape 21、drop 7（見 §1）。其中 D1／D8／D9／M11／M19／AD4 已決定的處置不在 §4 重列。
- 「SSOT 沒寫」的取捨 11 項（§4）：S1 候選集與 run 的關係、S2 requests／revisions／answer_patches 的模型、S3 JSON 與 typed 欄位的界線、S4 冪等儲存、S5 hash 與快照欄位的重複、S6 沒有寫入者的狀態值與接受單鎖死、S7 採購的 ADK session 存放處、S8 人工同義詞與分類樹的存放、S9 候選的 `display_name` 與可見性、S10 買家關閉後採購資料的去向、S11 次要（selection 折入、conversation 與 request 合併）。
- 計畫字面有兩處與 D1 不一致（§3）：`SearchSellerProspects` 還列在 A9-3 的依賴裡；`A9-3a` 與配額的步驟歸屬沒有人認領。
- 對 `DI` 的更正：`DI` §1.2 寫「DB 沒有 partial unique（未驗）」，實際有：`PS:38-40` 的 `uq_procurement_request_revisions_active ... WHERE status = 'ACTIVE'`【量】。

## 1. 逐表處置

「步驟」欄：計畫字面把 31 表的 schema 全放在 A9-3a（`PLAN:175`「31 表依 A9-D1、AD4、AD12 收斂後的 schema」）；行為歸 A9-3b（run、串流、sweeper，`PLAN:176`）或 A9-3c（賣家報價、建單、relay，`PLAN:177`）。【量】下表「步驟」＝schema 在 3a 建、行為所屬步驟。

通用機械修改（每張表都做，不逐列重複）：主鍵與自己實體的參照改 UUIDv7（應用層 `uuid.NewV7()`、不設 DEFAULT，`CO:11`、`ADR-ID` D1）；`buyer_id`、`store_id` 維持 bigint 不建 FK（`CO:11`；`AGENTS.md` 規則 2）；表名加 `procurement.` schema 前綴、去掉 `procurement_` 名稱前綴（依 assistant 模組 `internal/modules/assistant/db/schema.sql:9-11` 的寫法【量】）；狀態欄 `text` 加具名 CHECK，值＝Go 常數＝proto enum 值名（`ADR-C` D3，`docs/adr/2026-09-25-codemeta-retirement.md:59`【量】）；文字欄寫 `trimtext.Check` 並登記 `trimmedColumns`（`PLAN:47`、`PLAN:175`）。

| # | 表 | 步驟 | 處置 | 怎麼改／為什麼 | 來源 |
|---|---|---|---|---|---|
| 1 | `procurement_requests` | 3a | reshape | `status` CHECK 收成 proto `RequestStatus` 的 3 值（CLARIFYING、REQUIREMENT_READY、MATCHING，`PROTO:173-181`）。`active_revision_id` 與 `version` 是否保留見 S2 | 【量】`PS:8-16` |
| 2 | `procurement_request_revisions` | 3a | reshape | append-only 保留（契約的 `expected_revision_id` 靠它，`PROTO:703` 以下各請求）。`slots_json` 是否改 typed 欄位見 S3（`Slots` 在 proto 是封閉的 9 欄，`PROTO:423-433`）；`status`／`branch_id`／`parent_revision_id` 見 S2 | 【量】`PS:23-43` |
| 3 | `procurement_answer_patches` | 3a | reshape | 它同時是冪等鍵、稽核紀錄、與**對話訊息的唯一來源**（`ListAnswerPatchesByRequestID` 的 `payload_json` 取出 buyer 原文 `Message` 與 assistant 回覆 `Reply`，`AI/internal/procurement/usecase/conversation.go:169-185`【量】）。三個角色混在一張帶 JSON 的表，見 S2、S4 | 【量】`PS:47-64` |
| 4 | `procurement_conversations` | 3a | reshape | `adk_session_id` 的去向見 S7；與 requests 1:1（`uq_procurement_conversations_request`，`PS:82-83`）是否合併見 S11；`deleted_at` 軟刪除保留（買家刪對話後 request 與已發布的詢價要留，`PG 設計規範` §十 允許選用軟刪除，`docs/開發規範/PostgreSQL 設計規範與最佳實踐.md:252-257`【量】）；`ALTER TABLE ... ADD COLUMN`（`PS:85-89`）攤平成一個 `CREATE TABLE` | 【量】`PS:70-98` |
| 5 | `procurement_conversation_commands` | 3a | drop（建議，S4） | 只服務 rename、delete 兩支方法的冪等；rename 是目標狀態、delete 是目標狀態，重送結果相同，但契約仍要求 `client_request_id` 並在內容不同時回 `INVALID_ARGUMENT`（`PROTO:12-14`、`891-910`） | 【量】`PS:103-113` |
| 6 | `procurement_candidate_sets` | 3a（run 行為 3b） | reshape | D1 刪 `external_lane_status`；AD12 無關。`run_id`、`lease_owner`、`lease_expires_at`、`fencing_token`、`failure_reason`、`status`、`platform_lane_status` 這批「run 的生命週期」放哪見 S1。`policy_version` 目前恆為 1（見 S1） | 【量】`PS:123-211`；`ADR-S:34`（刪 lane 欄） |
| 7 | `procurement_candidates` | 3a | reshape | D1：刪 `lane`、`prospect_id`、`brand_logo_key`、`brand_cover_key`，`store_id` 改 NOT NULL，兩條 partial unique 收成 `(candidate_set_id, store_id)`。`seller_name`、`logo_key`、`store_code`：契約的 `Candidate.store` 是共用 `StoreCard`（`PROTO:603-614`），以 ADR 2026-10-03 的「各處同一張卡片」為準，改讀取時組裝，三欄刪（`replicas.csv:50` 的 `disposition` 空白，要填「刪除改查詢」）。`display_name`、`source_id`（product_id 字串）見 S9。`matched_facts_json`、`unverifiable_facts_json` 見 S3。A9-D8：自由文字欄（`research_summary` 等）本來就不在這張表，只在 DTO（`PS:236-265` 無該欄【量】），所以 D8 的屏障是 3a 的 DTO，不是欄位 | 【量】`PS:236-276`；`BR:81-86`；`PLAN:93` |
| 8 | `procurement_candidate_selections` | 3a | reshape | D1：刪 `lane`，`uq` 改 `(request_revision_id, candidate_id)`。冗餘欄 `request_id`、`candidate_set_id`、`buyer_id` 可由 candidate 推出；是否折成 `candidates.selected` 見 S11 | 【量】`PS:280-297` |
| 9 | `procurement_price_previews` | 3a | reshape | `version BIGINT` 沒有更新路徑，恆為 1（`PS:344-350` 註解自承「只 INSERT、沒有更新版本的路徑」）；契約卻帶 `version`、`price_preview_version`（`PROTO:731-738`、`740-752`）。`preview_input_hash`、`payload_hash` 與 `*_range_json` 見 S3、S5 | 【量】`PS:319-342` |
| 10 | `procurement_confirmed_rfq_snapshots` | 3a | reshape | `canonical_payload_json` 與 `payload_hash`：hash 的 preimage 範圍已由 ADR `2026-09-18-rfq-snapshot-hash-preimage.md` 決定（涵蓋賣家看到的完整內容）。`buyer_note`（契約新欄，`PROTO:752`，買家原文、最多 500 字、轉呈賣家）住在 JSON 裡，登記不了 `trimmedColumns`，見 S3。`price_preview_version` 恆為 1，見 S5 | 【量】`PS:354-372` |
| 11 | `procurement_rfq_publication_approvals` | 3c | reshape | `version` 保留（契約有 `approval_version`，`PROTO:804-807`、`826-828`）。WISH_POOL／DIRECT_RECIPIENTS 兩模式保留（`PROTO:277-283` 已定）。6 個 hash 欄、`price_preview_version`、`candidate_set_policy_version` 的重複見 S5、S1 | 【量】`PS:386-438` |
| 12 | `procurement_rfq_publication_approval_recipients` | 3c | reshape | 主鍵 `(approval_id, position)` 已是 `ADR-ID` D1 的「值的集合」寫法，保留；D1：刪 `lane`、`prospect_id`，`store_id` NOT NULL，刪 `chk_prpar_identity_matches_lane` | 【量】`PS:453-464` |
| 13 | `procurement_rfq_publication_commands` | 3c | drop（建議，S4） | 它是 execute 的冪等 claim，`publications.command_id` 唯一把它接到 publication（`PS:526-527`）；publication 一次只建一列，唯一鍵與請求 hash 直接放 publications 即可 | 【量】`PS:474-489` |
| 14 | `procurement_rfq_publications` | 3c | reshape | `version` 保留（`publication_version`，`PROTO:770-784`、`834-846`）。`state` 的 SUPERSEDED、CANCELLED 沒有寫入者（S6）。`approval_version`、`seller_facing_payload_hash`、`audience_*` 是從 approval 複製的（S5） | 【量】`PS:502-530` |
| 15 | `procurement_rfq_publication_invites` | 3c | reshape | D1：刪 `lane`、`prospect_id`，`store_id` NOT NULL，刪 EXTERNAL 的 CHECK 與 `uq_prpi_publication_prospect`。`delivery_state` CHECK 收成 `QUEUED`、`SENT`、`FAILED`：契約的 `InviteDeliveryState` 沒有 NOT_SENT（`PROTO:297-306`），且 invite 與 River job 同 tx 建，一出生就是 QUEUED【推】。`buyer_id` 可由 publication 推出 | 【量】`PS:545-585`；`PROTO:297-306` |
| 16 | `procurement_rfq_publication_outbox` | 3c | drop | M11：改 River job＋`procurement.events` 舊形狀由 relay 維持；`external_created` 事件隨 D1 消失（`CO:21`、`ADR-S:34`）。`attempt_count`、`last_error` 由 River 管 | 【量】`PS:598-621`；`CO:21` |
| 17 | `procurement_seller_responses` | 3c | reshape | D1：刪 `lane`、`prospect_id`，唯一鍵剩 `(publication_id, store_id)`。`write_closed_reason` 的 PUBLICATION_CLOSED、OTHER_WON 沒有寫入者（S6）。`version`（列版本）與 `latest_version`（賣家的回價版次，契約的 `expected_seller_response_version` 指後者）並存，見 S2 的版本欄項 | 【量】`PS:624-665`；`AI/internal/procurement/commerce/seller_response.go:601` 只寫 DECLINED |
| 18 | `procurement_seller_response_versions` | 3c | reshape | D1：刪 `verification_method`、`verification_email`、`verified_at`、`verification_session_id` 四欄與 `chk_prsrv_verification_all_or_none`（Google 驗證是站外業者入口用的，`PS:696-707`）。其餘已是 typed 欄位且 CHECK 完整（金額加總、數量為正），保留。`payload_hash`、`seller_facing_payload_hash` 見 S5 | 【量】`PS:667-730` |
| 19 | `procurement_seller_response_commands` | 3c | drop（建議，S4） | WITHDRAW／DECLINE 的事實同時存在這張表與 `seller_responses` 的 `withdrawn_*`、`write_closed_*`，註解自承「身分列由它推出」（`PS:732-734`）＝同一件事兩處存放；刪 `verification_*`（D1）後只剩冪等用途 | 【量】`PS:735-763` |
| 20 | `procurement_seller_response_drafts` | 3c | keep | 一賣家一草稿、整份覆寫、版本＝契約的 `draft_version`（`PROTO:1173-1179`、`1244`）。`payload` 是部分填寫的 `SellerQuote`，JSONB 合理（加 `jsonb_typeof` CHECK，前例 `payment` 模組）。主鍵由 `seller_response_id` 改為 `(publication_id, store_id)` 或維持，3c 定 | 【量】`PS:765-771` |
| 21 | `procurement_buyer_shortlist_entries` | 3c | keep | 關聯表，`uq (request_id, seller_response_id)`；契約的 Put／Delete 以目標狀態表達、天然冪等（`PROTO:12-14`）。`buyer_id` 可推出 | 【量】`PS:773-787` |
| 22 | `procurement_buyer_quote_acceptances` | 3c | reshape | D1：刪 `lane`、`prospect_id`、`chk_prbqa_identity_matches_lane`。`state` 的 CANCELLED、EXPIRED、PAYMENT_FINALIZED 沒有寫入者，`uq_prbqa_request_live` 因此讓需求永遠鎖死（S6）。`order_provisioning_state`、`order_failure_reason`、`order_no`、`payment_deadline` 保留（契約 `OrderProvisioningState`，`PROTO:349-358`）；重複的 hash 與金額見 S5 | 【量】`PS:789-859`；`AI/internal/procurement/repository/buyer_acceptance.go:334,350` 兩支方法無非測試呼叫者 |
| 23 | `procurement_order_provisioning_outbox` | 3c | drop | M11：改 River job，唯一鍵 `acceptance_id`；狀態由 acceptance 的 `order_provisioning_state` 承載 | 【量】`PS:875-911`；`CO:21` |
| 24 | `procurement_agent_runs` | 3b | reshape | AD12／M19：刪 `controller_kind` 與其 CHECK（`PS:939`、`954-955`）。`run_type` CHECK 刪 `QUOTE_APPROVAL`（`PS:950` 有、`PROTO:184-195` 沒有）。`domain_ref_type`／`domain_ref_id` 的字串多型見 S1。`version` 見 S2 的版本欄項。`last_event_seq`、`min_available_seq` 屬 3b 串流設計 | 【量】`PS:916-962` |
| 25 | `procurement_agent_run_inputs` | 3b | reshape | `retention_class` 只有一個字面值 `"BUYER_INPUT"`（`AI/internal/procurement/usecase/run_dispatch.go:376`【量】），無分支、無清理 job 讀它 → 刪；`input_schema_version` 見 S3；`input_ref` JSONB 保留（買家輸入原文，是否進 `trimmedColumns` 見 S3） | 【量】`PS:971-978` |
| 26 | `procurement_agent_run_events` | 3b | reshape | 主鍵由 `id BIGINT` 改 `(run_id, seq)`（`ADR-ID` D1 的「只在上層底下識別」列；現有 `uq (run_id, seq)` 已存在，`PS:991`）。`schema_version`、`trace_id` 與 payload 形狀見 S3。保留期與清除（契約有 `REPLAY_CURSOR_EXPIRED`，`PROTO:7-9`）屬 3b，目前沒有任何來源寫保留多久 | 【量】`PS:980-991` |
| 27 | `procurement_agent_run_artifacts` | 3b | keep | `(artifact_id, version)` 主鍵、不可變、`checksum`。3b 審 artifact 版本是否還需要多版（只有 `RFQ_PUBLICATION_RESULT` 等少數型別，本審查未逐一驗證【推】） | 【量】`PS:993-1008` |
| 28 | `procurement_agent_run_commands` | 3b | drop（建議，S4） | 建 run 的方法（SubmitTurn、StartCandidateSearch、ResumeRun）讓 run 自帶 `client_request_id` 唯一鍵即可；CancelRun 依語意冪等、請求不帶鍵（`PROTO:1122-1126`），不需要紀錄 | 【量】`PS:1010-1022` |
| 29 | `procurement_agent_run_interrupts` | 3b | reshape | 主鍵 `id TEXT` 改 UUIDv7。`interrupt_kind` 只剩 CLARIFICATION 一值（`PROTO:243-248`），`response_schema_version` 為其附屬；註解「下一個 kind 不該需要動 schema」是為不存在的需求預留（necessity condition 不成立），但 payload 本身是值的集合，JSONB 合理（S3）。`expires_at` 欄註解自承「不再加 TTL」（`PS:1045-1048`），3b 確認是否有寫入者後刪 | 【量】`PS:1049-1062` |
| 30 | `procurement_usage` | 見 §3-b | reshape | D9：改一列計數器加條件 UPDATE（`UPDATE ... WHERE usage_count < limit`），外部呼叫失敗時退回（`CO:19`、`BR:88-94`）。主鍵 `(buyer_id, period_start)` 保留；`usage_count` 改名表意（如 `used`） | 【量】`PS:1068-1074` |
| 31 | `procurement_usage_reservations` | 見 §3-b | drop | D9：不建預約表 | 【量】`PS:1080-1088`；`PLAN:119` |

計數：keep 3（#20、21、27）、reshape 21、drop 7（#5、13、16、19、23、28、31；其中 SSOT 已決定 3：#16、23、31；本審查建議 4：#5、13、19、28）。

## 2. 沿用機制逐項

欄位：必要條件｜今天是否成立｜從零設計的業界做法｜SSOT 是否已決定｜建議。

| 機制 | 必要條件 | 今天是否成立 | 從零設計的業界做法 | SSOT 是否已決定 | 建議 |
|---|---|---|---|---|---|
| ID 欄（TSID BIGINT，`PS:4`、各表 `id BIGINT PRIMARY KEY`） | 舊的 Java 服務需要認得這個 ID | 否：Java 只認 `order_no`、`store_id`、`buyer_id`、`prospect_id`（`CO:11`）；D1 後 `prospect_id` 也消失 | 實體 UUIDv7；值的集合複合鍵；紀錄表 UUIDv7 | 已決：`CO:11`、`ADR-ID` D1（表格 7 列，逐表歸類）、D4 | 照 `ADR-ID` D1 逐表歸類：實體（requests、revisions、conversations、candidate_sets、candidates、snapshots、approvals、publications、invites、seller_responses、acceptances、runs、interrupts）UUIDv7；`approval_recipients (approval_id, position)`、`run_events (run_id, seq)`、`run_artifacts (artifact_id, version)`、`drafts`、`usage (buyer_id, period_start)` 用自然／複合鍵 |
| JSON 欄 vs typed 欄（12 個 JSONB：`slots_json`、`payload_json` ×2、`canonical_payload_json`、`seller_facing_payload_json`、`buyer_requested_range_json`、`quote_total_range_json`、`matched_facts_json`、`unverifiable_facts_json`、`public_payload_json`、`input_ref`、`public_data_json`、`result_json`） | 內容形狀不固定，或不值得為它拆表 | 部分：`Slots`、`SellerFacingRfq`、`BuyerRequestedRange`、`QuoteTotalRange` 在 proto 是封閉的 typed message（`PROTO:423-433`、`703-719`、`761-784`） | 規範 §六：JSONB 限彈性結構，「必須能說明為何不用 table + FK」（`PostgreSQL 設計規範` 第 167-172 行）；封閉形狀用 typed 欄位並加 CHECK | 未決（S3）。只有 `PLAN:47`、`PLAN:175` 要求「採購需求、報價備註」文字欄登記 `trimmedColumns`，那隱含它們是欄位，但沒寫 slots 是不是 | S3 |
| 樂觀鎖 `version` | 呼叫端依讀到的內容修改，要擋過時寫入 | 契約有的：approval、publication、draft、seller response 版次（`PROTO:14-18`、`826-828`、`1244`、`1258-1261`、`1276`） | etag 加條件式 UPDATE（`ADR-V` D4，`docs/adr/2026-09-25-optimistic-concurrency-etag.md:37`）；沒有 client 讀取者的狀態轉移用條件式 `WHERE status = ...`，不另設版本 | 契約欄位已決；內部欄位未決：requests.version、runs.version（契約已拿掉 `run_version`，`PROTO:566`、`631`、`1067`、`1125`、`1133`）、acceptances.version、price_previews.version、seller_responses.version | S2、S5 |
| `fencing_token`、`lease_owner`、`lease_expires_at`（只在 candidate_sets，`PS:168-170`） | 同一個工作可能被第二個 owner 重新領走，舊 owner 的晚到寫入要被擋 | 否：A9-D6 決定 sweeper 標失敗、買家重送，不重驅（`PLAN:116`、`BR:66-72`），沒有第二個 owner。但 `CO:28`（M20）寫「River periodic job 掃 `lease_expires_at`」，與 D6 的「掃逾時的 run」落在不同的表 | 單一 worker 執行加 `WHERE status IN (QUEUED, RUNNING)` 條件式終態寫入，不需要 token；逾時由 `started_at` 加預算推出 | 部分：D6 決定不重驅；M20 沒說 lease 欄去留 | S1 |
| `*_commands` 冪等表（6 張）與各表 `client_request_id` 唯一鍵（answer_patches、price_previews、confirmed_snapshots、approvals、acceptances 共 5 張帶欄位） | 同一 `(呼叫者, client_request_id)` 重送回第一次結果，內容不同回 `INVALID_ARGUMENT` | 成立（契約要求，`PROTO:12-14`） | 一把鍵放在被建立的實體上（與實體同 tx、不過期）；非建立型命令用單一收據表（method、request_hash、result_ref）；Stripe 與 AIP-155 都有 request fingerprint | 語意已決（`PROTO:12-14`）；儲存未決。assistant 的做法（D17(5)：記憶體、10 分鐘）只管 assistant（`PLAN:127`） | S4 |
| append-only revisions 加 SUPERSEDED、`branch_id`、`parent_revision_id` | 回上一題（Rewind）後舊分支不得再被當成現行，且歷史可稽核 | 成立（`Rewind` 在契約，`PROTO:61`） | append-only 歷史加一個「現行指標」或「最新版即現行」；分支只在需要同時保留多條時才建 | 未決 | S2 |
| 賣家資料快照（candidates 的 `seller_name`、`logo_key`、`store_code`、`brand_*`；approval／invite／response 的 `store_id`） | 賣家改名或換圖後，買家看到的候選要維持當時的樣子 | 否：契約已改為共用 `StoreCard`（讀取時組裝，`PROTO:603-614`，ADR 2026-10-03）；`facts_as_of` 記的是比對事實成立時間，保留 | 讀取時組裝；只快照業務判定依據 | 契約已決（`Candidate.store`）；`replicas.csv:50` 的 `disposition` 還空著 | 三欄刪，`store_id` 與 `facts_as_of` 留；填 `replicas.csv:50`。注意 `display_name` 的洩漏面見 S9 |
| outbox 兩張（`PS:598`、`875`） | 要可靠地對外送訊息 | 否：單體 tx 內寫業務列加 `jobs.Enqueue`（規則 4、5） | River job 帶冪等鍵 | 已決：`CO:21`、`PLAN:19` | drop。invite 的 `delivery_state` 由 job 回寫，保留 |
| CHECK 值域 | 把合法值釘在 DB | 成立；但寫法是「registry 的完整值域，不是實際會寫的子集」（`PS:152-158` 註解） | DB 值＝Go 常數＝proto 值名（`ADR-C` D3） | 寫法已決（`ADR-C` D3）；「該不該收未被寫入的值」未決 | S6 |
| 時間欄 | 稽核與排序 | 成立：全部 TIMESTAMPTZ | 同；只在會被修改的列放 `updated_at` | 規範：created_at、updated_at 為標準欄（`PostgreSQL 設計規範` 第 72-76 行） | 不可變列（snapshots、versions、events、inputs）只留 `created_at`（`seller_response_versions` 已如此，`PS:701`）【推】 |
| 軟刪除 | 買家刪對話後，request、publication、order 要留 | 成立：只有 conversations 有 `deleted_at`（`PS:89`） | 同 | 規範允許（§十） | keep，僅 conversations |
| `schema_version`（`PS:418`、`519`、`976`、`985`、`1053`） | 已存的 payload 形狀會演進，舊列要能被新程式讀 | 否：系統從未上線、舊資料不搬（`PLAN:`前提）；hash-bearing 的列例外（hash 只有在 preimage 版本已知時才可重算） | hash-bearing 列留版本；事件改存 proto JSON，向前相容靠 proto 規則 | 未決 | S3（子項） |

## 3. 計畫字面與現況的不一致

a. 計畫仍把 `SearchSellerProspects` 列為 ai-service 對 seller 的 4 支呼叫之一（`PLAN:5`），不變量表也寫「採購候選搜尋（seller prospects）」（`PLAN:21`）。A9-D1 刪除站外業者路線後，prospect 是 EXTERNAL lane 的資料源，PLATFORM lane 的資料源是 catalog 的商品搜尋（`candidates.source_id` 是 product_id，`PS:240`；A9-1c 已補 `legacy.SearchProducts`，`PLAN:229`）。A9-3a 開工時要先改這兩行，否則 3a 的候選搜尋範圍有歧義。【量】

b. 配額的步驟歸屬：D9 的「用在」欄寫 A9-4（`PLAN:119`），A9-4 的驗收只寫修圖配額（`PLAN:187`）；但 `procurement_usage` 在 `PS` 31 表裡（`PLAN:175` 要 3a 建 schema），而 `QuotaSnapshot` 在 `TurnResult`（`PROTO:462-470`），SubmitTurn 要扣。採購配額計數器由哪個步驟寫、驗收是什麼，計畫沒寫。這不是取捨，是排程缺口；建議歸 3b（SubmitTurn 在那裡）並把驗收補進 3b。【推】

c. `PLAN:39` 不變量表有「刪帳訂閱者」，但沒有任何一行說採購訂閱什麼、處置什麼（S10）。

## 4. SSOT 沒寫的取捨

只列「從零設計會不同，且計畫、ADR、CARRYOVER、decision-brief 都沒有決定」的項目。已有來源決定的（D1、D7、D8、D9、AD4、AD12、M1、M11、M19、StoreCard 取代賣家快照）不重列。

### S1 候選集同時是「一批候選」與「一次 run」，以及 lease、fencing、policy_version

- 現況：`candidate_sets` 一列同時是結果容器與搜尋 run 的生命週期（`status`、`platform_lane_status`、`failure_reason`、`lease_*`、`fencing_token`、`run_id`，`PS:115-143`）；`agent_runs` 另有同一個 run 的帳本列，用 `domain_ref_type/id` 這對字串指回（`PS:922-923`）。重試是原地換 `run_id`（`RetryCandidateSet`，`AI/internal/procurement/usecase/candidate_search.go:124-135`【量】），所以 `candidate_sets : agent_runs` 是 1:N（`PS:133-141` 註解）。
- 已決的部分：D1 刪 external lane；D6 不重驅；M20 要 sweeper 掃 `lease_expires_at`（`CO:28`）。沒決的：candidate_sets 還要不要承載 run 的狀態，重試要不要原地改。
- 從零設計：run 狀態只放 `agent_runs`（單一帳本），candidate_set 只是搜尋結果（`id`、`request_revision_id`、`buyer_id`、`search_run_id`、`created_at`）；一次搜尋一個 candidate_set，失敗後買家重送就是新 run、新 candidate_set，不原地改；沒有重驅就不需要 `fencing_token` 與 `lease_owner`，終態寫入用 `WHERE status IN ('QUEUED','RUNNING')`；`agent_runs.domain_ref_*` 改成 typed 可空外鍵（`candidate_set_id`、`publication_id`）。
- 選項：(a) 照搬（一表兩角色，原地重試，保留 lease 欄）；(b) 如上拆開，一次搜尋一個 candidate_set；(c) 折衷：candidate_set 單一，狀態拆出去，但仍允許原地重試。
- 建議 (b)。理由：D6 已經把「重驅」排除，fencing 與 lease 的必要條件（第二個 owner）不成立；一表兩角色的註解自己就說「該分開的一半已撤銷」（`PS:119-121`）。代價：`uq (request_revision_id, policy_version)` 的冪等語意要改成 `(request_revision_id, client_request_id)`；`policy_version` 目前恆為 1（`rg 'PolicyVersion'` 只在 dialogue plan 與 audience 兩處有值，候選側全是預設，`AI/internal/procurement/domain/publication_audience.go:84-103`【量】），建議一併刪掉，`approvals.candidate_set_policy_version` 與 `audience_policy_*` 同理（WISH_POOL 的 `audience_policy_id/version` 是契約欄位，`PROTO:815-816`，保留）。

### S2 requests／revisions／answer_patches 的模型

- 現況：「現行 revision」有三個表示法：`requests.active_revision_id`、`revisions.status = 'ACTIVE'`、partial unique `uq_..._active`（`PS:12`、`31`、`38-40`）。另有 `branch_id`＋`parent_revision_id`。`answer_patches` 與 revision 1:1（`result_revision_id NOT NULL`），`base_revision_id` 與 `parent_revision_id` 同義，卻又是訊息歷史（`conversation.go:169-185`）。`requests.version`（`PS:13`）在契約沒有讀取者（契約的並發守衛是 `expected_revision_id`）。
- 已決：append-only revisions 本身（契約依賴）；沒有任何來源決定其餘。
- 從零設計：revision 表保留，單一「現行」表示法（用 partial unique，DB 擋雙頭）；`requests` 不放指標、不放 `version`（轉移用條件式 UPDATE）；answer_patches 改名 `turns`（或 `messages`）：`id`、`request_id`、`conversation_id`、`source`（LLM_EXTRACT、ANSWER、REWIND）、`buyer_message`（typed 文字欄，登記 `trimmedColumns`）、`assistant_reply`、`result_revision_id`、`client_request_id`（唯一）、`steps`／`recorded_this_turn` 留 JSONB；`base_revision_id` 刪（用 `parent_revision_id`）。
- 選項：(a) 照搬；(b) 如上；(c) 只合併表示法（刪 `active_revision_id`），不動 answer_patches。
- 建議 (b)。若 Will 要降低風險取 (c)。
- 風險：`branch_id` 是否還有讀取者本審查沒有逐一驗證【推】，3a 開工第一步用 `rg branch_id` 確認；沒有讀取者就刪。

### S3 JSON 與 typed 欄位的界線（含 `schema_version`）

- 現況：見 §2 第二列。`Slots`（9 欄）、`SellerFacingRfq`（11 欄）、`BuyerRequestedRange`、`QuoteTotalRange` 在 proto 是封閉形狀。`buyer_note`（買家原文，轉呈賣家，`PROTO:752`）在 `canonical_payload_json` 裡。`trimmedColumns` 的 CHECK 只能掛在文字欄（規則 9，`backend/AGENTS.md` 第 9 條）。
- 已決：hash 的 preimage 要涵蓋賣家看到的完整內容（ADR 2026-09-18）；`PLAN:47` 要求採購需求與報價備註的欄位登記。沒決：哪些是欄位。
- 從零設計：有封閉形狀、要被 CHECK 或被查詢的，用 typed 欄位（slots 九欄、價格區間的整數、數量、單位、送達城市與日期、`buyer_note`、`response_deadline`）；hash 由 Go 從 typed 欄位以固定序列化重算，不另存 canonical JSON（或只存 hash）；JSONB 只留部分填寫的草稿、`steps`、`recorded_this_turn`、interrupt 的選項、事件 payload；事件 payload 存 proto JSON（`WatchRunResponse` 的 oneof），形狀演進靠 proto 規則，刪 `schema_version`／`input_schema_version`／`response_schema_version`；保留 `approvals.schema_version` 與 `publications.schema_version` 一個（hash 要知道 preimage 版本才可重算）。
- 選項：(a) 全沿用 JSONB＋各種 version 欄；(b) 如上；(c) 只把含自由文字的欄位 typed 化（`item`、`packaging`、`delivery_city`、`buyer_note`、`terms_note` 等），其餘維持 JSONB。
- 建議 (b)，最低要求 (c)：自由文字不 typed 化就繞過規則 9 的 `TestTrimmedTextChecksUseTheCanonicalPattern`，`PLAN:175` 的驗收句「文字欄位登記 `trimmedColumns`」就沒有東西可登記。
- 對 3a 的影響：slots 與 snapshot 的 sqlc 型別、hash 重算的 golden 測試（ADR 2026-09-18 的 fixture）要在 3a 一起做。

### S4 冪等的儲存方式

- 現況：6 張 `*_commands`＋5 張實體各帶 `client_request_id` 唯一鍵（§2）。每張的形狀不同（有的存 `result_json`，有的存 `canonical_request_hash`、有的存 `payload_hash`）。
- 已決：語意（`PROTO:12-14`）。assistant 另有一套（記憶體，`PLAN:127`）。採購的儲存沒決。
- 從零設計：呼叫建立一個實體的方法，鍵放在該實體上（與實體同 tx、不過期、重送時直接讀實體）：turns、price_previews、confirmed_snapshots、approvals、publications、acceptances、runs、seller_response_versions；其餘（rename、delete、withdraw、decline）用一張 `command_receipts(principal, method, client_request_id, request_hash, result_ref, created_at)`，River 週期清過期列；刪 `conversation_commands`、`publication_commands`、`seller_response_commands`、`agent_run_commands`。
- 選項：(a) 照搬 11 處；(b) 如上（混合）；(c) 全部集中成一張收據表。
- 建議 (b)。(c) 對建立型方法多一次寫入且與實體分離，需要處理「收據在、實體不在」的半成品；(a) 的代價是 4 張只為冪等存在的表。
- 與 S2 連動：answer_patches 改 turns 後它自己帶鍵。

### S5 hash 與快照欄位的重複

- 現況：`approvals` 帶 6 個 hash 與一份快照參照（`confirmed_rfq_snapshot_id`、`confirmed_rfq_payload_hash`、`price_preview_payload_hash`、`price_preview_input_hash`、`buyer_requested_range_hash`、`seller_facing_payload_hash`、`audience_hash`，`PS:396-410`、`416`）；`publications` 再複製 `seller_facing_payload_hash`、`audience_mode`、`audience_hash`（`PS:512-517`，註解理由是「不必回頭 join approval」）；`acceptances` 再複製 `seller_response_payload_hash`、`confirmed_rfq_payload_hash`、`goods/shipping/tax` 三金額與 `accepted_deal_price`（`PS:797-807`）。`price_previews`、`confirmed_snapshots` 只 INSERT、沒有 UPDATE 路徑（`PS:344-350`），所以 `price_preview_version` 恆為 1。ADR 2026-09-18 已警告「看起來像 CAS、實際恆為真的檢查」。
- 已決：契約的 CAS 對象（`payload_hash`、`expected_rfq_snapshot_hash`，`PROTO:1010`、`830-832`）與 hash 範圍。沒決：資料庫內要不要複製。
- 從零設計：被參照的列不可變、同一個 schema，複製的必要條件（跨服務、被參照列可變）不成立；approval／publication／acceptance 只存參照，hash 以 join 取得；唯一要凍結的是「接受當下的成交價」（三金額加總，保留在 acceptance，因為報價版本不可變也可 join，但成交價是會計事實，留著無害）；`price_preview_version` 與 `price_previews.version` 刪，契約的 `version`／`price_preview_version` 欄位保留為 proto 的不透明字串但由伺服器填 `price_preview_id` 衍生值（需改 proto 才能真正刪，A9-1b proto 還在分支上，成本低）。
- 選項：(a) 照搬所有複製欄；(b) 刪複製、保留參照；(c) 保留參照，另外只在 acceptance 留「接受時的 snapshot hash 與 response payload hash」（那兩個是買家 CAS 的對象，留作稽核）。
- 建議 (c)。若 Will 認為 proto 不再動，`price_preview_version` 就留著填常數 1 並在 CARRYOVER 記一格。

### S6 沒有寫入者的狀態值，與接受單鎖死

- 【量】只有以下值有寫入者：publication `PUBLISHED`（`commerce/publication_execute.go:209`）、acceptance `ACTIVE`、seller response write_closed `DECLINED`（`commerce/seller_response.go:601`）。沒有寫入者：publication `SUPERSEDED`／`CANCELLED`（proto 與 CHECK 都有，`PROTO:286-295`；買家 service 沒有取消或重發 RPC，`PROTO:33-122`）；acceptance `CANCELLED`／`EXPIRED`／`PAYMENT_FINALIZED`（`ReleaseAcceptance`、`MarkAcceptancePaymentFinalized` 只有定義與測試，`rg` 無非測試呼叫者）；write_closed `PUBLICATION_CLOSED`／`OTHER_WON`（常數在 `repository/seller_response.go:33-34`，無呼叫）。
- 後果：`uq_prbqa_request_live`（`PS:850-852`）讓一張需求在接受一份報價後永遠無法再接受別份，包含訂單建立失敗（`ORDER_FAILED`）或未付款到期後；而計畫範圍只有「經 legacy ordering 建單」（`PLAN:177`），沒有任何一步接訂單狀態。
- 已決：值域的寫法（`ADR-C` D3：DB 值＝proto 值名；DB 內部值可列白名單）。沒決：該不該收沒有寫入者的值、接受單怎麼解鎖。
- 從零設計：CHECK 只放有寫入者的值，新值隨它的寫入者那個 migration 加（上線前 migration 無資料成本，`PLAN:` 前提）；接受單的「現行」改由訂單狀態決定（ordering port 查 `order_no` 是否仍有效），不存 PAYMENT_FINALIZED。
- 選項：(a) 照搬完整值域（現況，註解 `PS:152-158` 的理由）；(b) 只留有寫入者的值，並在 CARRYOVER 的「與 ai-service 的差異」記接受單鎖死為已知缺陷，觸發：ordering 搬遷（B8）；(c) A9 內補一個 release 路徑（需要訂單事件，ordering 還在 Java，要走 legacy，超出 3c 範圍）。
- 建議 (b)；但鎖死是行為缺陷，要 Will 明確接受（根目錄 `AGENTS.md`「不留技術債」）。proto 的 `PublicationState`、`InviteDeliveryState`、`QuoteAcceptability` 同樣收斂的話要改 A9-1b 的 proto（分支內，成本低）。

### S7 採購的 ADK session 存放處

- 現況：採購以 ADK session 當 graph 的 checkpoint（`CO:13`）；session 必須跨越掛起與回答兩個 run（`AI/internal/agent/cleaner.go:93`、`usecase/graph_session.go` 註解【量】）；`conversations.adk_session_id` 存這個鍵（`PS:74`）。
- 已決：D5：session 改 pgx 實作 `session.Service`，採購狀態仍在自己的業務表（`CO:13`）。**但**現在唯一的實作在 `internal/modules/assistant/internal/adksession/session.go`（`rg -il adksession`【量】），Go 的 `internal` 規則讓採購不能 import，規則 2 讓採購不能查 assistant 的 schema，依賴表也沒有 procurement→assistant（`BR:47`）。
- 從零設計：session 儲存是無領域知識的基礎設施（四張 ADK 表，鍵是 ADK 的字串），應是 `platform` 套件加每個模組自己的 schema（`assistant.sessions`、`procurement.sessions`），或採購圖形的暫停狀態直接進 `run_interrupts`，不用 ADK session。
- 選項：(a) 把 `adksession` 抽成 `internal/platform/adksession`，兩個模組各建自己的四張表（3a 要多 4 張表，總數 35）；(b) 採購只在 run_interrupts 存暫停狀態、不用 ADK session（要改圖形程式，約 3k 行的 `graph*.go` 部分，`DI:303` 說明這塊未盤點）；(c) assistant 公開 session.Service port、採購依賴 assistant（要改依賴表，等於改架構）。
- 建議 (a)。需 Will 確認，因為它讓 3a 的表數、`platform` 套件結構與 assistant 的 `AGENTS.md`（現在寫「session 是 assistant 的資料」）都要動。
- 本審查沒做：`graph*.go` 的 checkpoint 內容是否真需要持久化（若只需要 interrupt payload，則 (b) 成本比預期低）。

### S8 人工同義詞與分類樹的存放與快取

- 現況：人工同義詞是 `cache/category_index.go:20` 的 Go map（農產名稱，如「地瓜→甘薯」「高麗菜→甘藍」，【量】）；分類樹經 TTL cache（B8，30 秒 timeout、singleflight，`DI:145`）從 catalog 取，現由 `legacy/categoryservice.CategoryTree` 提供（A9-1c 已建）。
- 已決：D7「分類樹加人工同義詞召回」（`PLAN:117`、`ADR-S:28`）。沒決：同義詞放哪、誰維護、分類樹快取留不留；`CO` 沒有 B8 快取的列，是漏的沿用項。一般電商的同義詞目前不存在，要重新建立內容。
- 選項：(a) 採購模組內嵌檔（`go:embed` JSON、有測試守格式與分類對得上）；(b) catalog 擁有的資料表（A5 前不存在）；(c) 採購模組的 DB 表加後台維護（要 admin RPC，超出範圍）。
- 建議 (a)：內容少、變動慢、上線前沒有召回率基準（`PLAN:78`）；分類樹快取用單一 in-process TTL，catalog 搬進單體後以模組 API 取代 legacy 呼叫並重評。對 3a 的 schema 沒有影響，但 resolver 的介面（`TaxonomyReader` port）要在 3a 定。

### S9 候選的 `display_name` 與可見性

- 現況：`candidates.display_name NOT NULL`（`PS:241`）是搜尋當下的店名快照，契約 `Candidate.display_name` 直接給買家（`PROTO:603-606`），另有 `store: StoreCard`。`StoreCard` 的規則是「沒公開過的店家沒有卡片，店名不能被讀出；`store` 不設表示不顯示」（`api/hawkr/common/v1alpha1/store_card.proto:17-20`【量】）。
- 問題：店家從沒公開過（或已 PAUSED），`StoreCard` 不給卡片，但 candidate 的 `display_name` 仍會把店名給買家，繞過共用卡片的可見性規則。搜尋時要不要只收 OPEN 的店、發布前要不要重查可見性，沒有來源寫。
- 已決：D8 只管自由文字；StoreCard ADR 管卡片。兩者沒說候選搜尋的資格。
- 從零設計：`display_name` 刪，名稱只從 `StoreCard` 讀；候選搜尋只收目前可公開曝光的店家（`STORE_CARD_STATUS_OPEN`）；`ExecuteRfqPublication` 在 tx 內對 DIRECT_RECIPIENTS 再檢一次店家未停權（`PLAN` 不變量「跨模組 transaction 由下往上鎖」要求的 seller 鎖在此用到）。
- 選項：(a) 保留 `display_name` 快照；(b) 刪、只讀 StoreCard、搜尋只收 OPEN；(c) 保留快照但只在 StoreCard 存在時給買家。
- 建議 (b)；契約的 `Candidate.display_name` 要同步刪（A9-1b 分支內）。

### S10 買家關閉後採購資料的去向

- 現況：ai-service 沒有任何刪帳處理（消費事件 0，`DI:26`）。單體已有訂閱買家刪除事件的先例：social 訂閱 `buyer.BuyerDeletionRequested`、`BuyerDeleted`，寫複本並排清除 job（`internal/modules/social/social.go:625-637`【量】）。`PLAN:39` 把「刪帳訂閱者」列在 lockorder 依賴，卻沒有一行說採購要做什麼。
- 資料面：採購含買家自由文字（slots、訊息原文、`buyer_note`，且 `buyer_note` 已轉呈給賣家）；`accountdeletion` 有未結訂單時買家保留、完成後關閉（`backend/docs/modules/accountdeletion.md` §3）。
- 選項：(a) 不處理（現況，系統未上線）；(b) 訂閱 `BuyerDeleted`，匿名化或刪除未發布的需求、對話、訊息，已發布的詢價與已成交的 acceptance 保留（賣家與訂單需要）；(c) 同 (b)，另對已發布的 `buyer_note` 也清除。
- 建議 (b)：業界做法是「個資在不再需要時刪除，交易紀錄依法保留」。這同時決定 3a 的表要能被「依 buyer_id 清除」：索引與外鍵關係（`turns` 與 `requests` 的 `buyer_id` 索引）。未決的是保留期與是否匿名化 `buyer_id`，需要 Will 決定。需要補進 `CARRYOVER` 的「與 ai-service 的差異」。

### S11 次要（可不攔 3a）

- selection 折入 candidates：`(request_revision_id, candidate_id)` 與 candidate 1:1（candidate 屬於一個 set、set 屬於一個 revision），`PS:293-294`。從零設計可折成 `candidates.selected`／`selected_at`，少一張表與 3 個冗餘 id；代價是 candidate 列從不可變變可變。建議折入【推】；保留也可。
- conversation 與 request 合併：1:1（`PS:82-83`）。`ADR-ID` 與 `PS:534-538` 自己的註解都批評「垂直切兩半」。但契約有兩個獨立的 ID（`conversation_id`、`request_id`，`PROTO:36-37`、`559-566`），而且刪對話不刪 request。建議維持兩張表【推】。

## 5. 3b／3c 預告（不在 3a 取捨範圍，只記已看到的）

- 3b：run 事件的保留期與清除沒有來源寫（契約有 `REPLAY_CURSOR_EXPIRED`，`PROTO:7-9`；assistant 有 D17 的 10 分鐘，採購沒有）；`run_interrupts.expires_at` 有沒有寫入者；`run_artifacts` 多版是否仍需要。
- 3c：WISH_POOL 沒有 invite 列（`PS:411-412`、`PROTO:795-798`），賣家怎麼列出「公開池」的詢價（`ListSellerPublications`，`PROTO:128`）沒有查詢來源，舊程式的做法需要 3c 開工時讀 `publication_invite_view.sql`；`offered_quality_grade`／`offered_quality_inspection_notes`、`conversion_basis`（`PS:672-678`）是農產時期的「品相」概念，契約仍必填（`PROTO:917-919`），是否屬 scope cut 殘留（ADR 2026-10-27 D2）沒有來源決定。

## 6. 本審查沒做

- 沒有逐一核對 `branch_id`、`run_artifacts` 型別、`interrupts.expires_at` 的讀寫者（S2、§5）。
- 沒有讀 `graph*.go` 的 checkpoint 內容（S7）。
- 沒有跑任何測試、容器或網路呼叫；`rg` 結果只涵蓋 `services/ai-service/internal` 與 `db/queries`。
