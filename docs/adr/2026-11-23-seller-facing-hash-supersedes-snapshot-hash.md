# 買家接受報價比對「賣家實際收到內容的 hash」，不再比對確認快照的 hash

| 項目 | 值 |
|---|---|
| 狀態 | accepted；取代 `2026-09-18-rfq-snapshot-hash-preimage.md`（supersedes） |
| 日期 | 2026-11-23（Will 決定，計畫 `backend/docs/plans/2026-11-19-a9-agent.md` A9-D35） |
| 影響範圍 | `api/hawkr/procurement/v1alpha1` 的 `AcceptSellerQuoteRequest`、`PublishedRfq`；`backend/internal/modules/procurement/internal/rfq`、`internal/rfqhash`；3b-3d 的 `AcceptSellerQuote` |

## 脈絡

`2026-09-18-rfq-snapshot-hash-preimage.md` 決定：買家接受報價時 CAS 的 hash，原像要涵蓋賣家實際看到的全部內容（採用擴大原像，否決「`(snapshotId, payloadHash)` 成對」，因為快照只 INSERT，成對的 hash 恆為真）。

A9-D35 把發布鏈收斂成「可改的草稿（需求現行版本）加一個不可變、會到期、只能執行一次的發布核准」，確認快照與價格預覽兩張表與 `ConfirmRfqSnapshot` 刪除（理由見計畫 A9-D35 與 `backend/internal/modules/procurement/CARRYOVER.md`）。那份 ADR 的**載體**（快照的 hash）因此不存在；它的**原則**（原像涵蓋賣家看到的全部欄位）仍然成立。

約束：

- 外部：無（系統從未上線，沒有線上資料或相容性負擔）。
- 繼承：原像要涵蓋賣家看到的全部欄位。現在仍成立的理由：不涵蓋的話，兩份品項、數量或城市不同的詢價會得到同一個 hash，CAS 證明不了它宣稱的事。
- 繼承：快照作為 hash 的載體。不再成立：快照存在的理由（凍結一份複本）在 S5（被參照的列不可變）之後就沒有了，且 `expires_at` 沒有任何讀取者。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：伺服器對「用戶端確認時看到的內容」算摘要，接受時重算比對（Stripe Quotes 以 finalize 後凍結的 quote 為單一資源、Shopify Draft Orders 以草稿單一物件完成；憑記憶，未查證） | 需要一個「賣家實際收到內容」的確定性函式 | 採用 |
| A. 保留快照，只換掉它的 hash 來源 | 多養一個沒有存在條件的物件與兩張表 | 否決（A9-D35 已刪） |
| B. 比對 `(publicationId, hash)` 成對，或 `(publicationId, revisionId)` | 識別由 ID 承擔，hash 只擋「同一列被改過」；發布列不可變，檢查恆為真（正是 2026-09-18 否決的方案 A） | 否決，不採用「ID 加 hash」 |

## 決策

`AcceptSellerQuote` 的比對值是 `expected_seller_facing_hash`：賣家實際收到的內容（品項、品項標籤、數量、單位、包裝、交貨縣市、交貨日、回價期限、買家補充說明、單價區間與計價基準、`schemaVersion`）的 SHA-256。伺服器從這筆報價所屬發布的核准（凍結的單價區間、補充說明、回價期限）與核准對應的不可變需求版本重算（`rfq.SellerFacingHash`），不讀任何存下的 hash。比對值來自用戶端、重算值來自伺服器，所以檢查不會恆為真；不符回 ABORTED／VERSION_CONFLICT，`conflictTarget` 是 `publication`。

買家畫面上的值取自 `PublishedRfq.seller_facing_hash`（讀取模型）或 `ExecuteRfqPublicationResponse.seller_facing_payload_hash`，三者同一個函式、同一個值。

它防的是：買家畫面上顯示的內容，和賣家報價時依據的那份發布不一致（例如需求重新發布後，買家在舊的分頁按下接受）。

## 後果

- 保留 2026-09-18 的要求：原像涵蓋賣家看到的全部欄位，由 `TestEverySellerFacingFieldChangesTheHash`（原像欄位）與 `TestAssembledSellerHashCoversEveryApprovalAndRevisionField`（核准與需求版本到原像的對照）守住。買家自己的預算、精確比對旗標與核准的 ID、到期時間賣家看不到，不進 hash（`TestAssembledSellerHashIgnoresFieldsTheSellerNeverSees`）。
- 原像隨 `schema_version` 分派；新增或改變欄位要新增版本，已發布詢價的 hash 不得改變（golden 測試）。
- `buyer_quote_acceptances.seller_facing_hash` 記下接受當下比對通過的值，留作稽核。
- 重新評估條件：賣家可見內容的欄位集合改變（要同步檢查舊核准讀回什麼）；或接受報價要綁定「賣家看到的是哪一次發布」以外的更多事實時。
