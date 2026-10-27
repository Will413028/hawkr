# `payload_hash` 的 preimage 要涵蓋賣家實際看到的內容

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-18（決策作成於 2026-09-06，落地 2026-09-07） |
| 影響範圍 | ai-service `internal/procurement/commerce/`、confirmed RFQ snapshot 的 wire 與 golden fixtures、web 消費端 |

規格本身見
`docs/詳細設計書/64-Hawkr Agent 商務線（RFQ Publication、Seller Quote、Buyer Acceptance）詳細設計書.md` §3。

## 脈絡

買家接受賣家報價時要 CAS `expectedRfqSnapshotHash`，用來保證
「賣家報價當時看到的那張詢價單」與「買家現在接受的那張」是同一份。

而當時 `payload_hash` 的 preimage 是一個白名單 struct，只含四個欄位：
`qualityKnowledgeState`、`pricingMode`、`referencePriceRange`、`buyerRequestedRange`。
它**不含**品項／crop identity、數量、單位、配送城市、交貨日期、候選名單、報價截止。

⇒ 兩張品項、數量、城市全都不同的詢價單，只要 pricing mode 與價格區間相同，
`payload_hash` 就會一模一樣。那個 CAS 證明不了它宣稱要證明的事。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| A. 維持現行 hash，CAS 用 `(confirmedRfqSnapshotId, payloadHash)` 成對 | 識別由 ID 承擔、hash 只擋「同一列被改過」。而 `procurement_confirmed_rfq_snapshots` 只有 INSERT、沒有 UPDATE 路徑 ⇒ hash 實質上擋不到任何東西——**一個看起來像 CAS、實際上恆為真的檢查**。這是最危險的選項，因為它會讓後續所有人以為內容完整性已經被守住 | 否決 |
| **B. 擴大 preimage，把 seller-facing 的完整內容納入 hash** | 要付「第二次凍結」：golden fixtures ＋ wire ＋ 前端消費端 | **採用** |
| C. 另立 `sellerFacingPayloadHash`，與既有 `payloadHash` 並存 | 成本不比 B 低（一樣要改 producer、fixtures、消費端），卻多留一個同名不同義的欄位。而 `payload_hash` 這個名字在同一支函式裡本來就已經有兩種語意，混淆正是這條線出過的問題 | 否決 |

## 為什麼是 B

- 選 A 等於把「CAS 保護了什麼」這個問題留給未來的人，而問題會在**賣家已經依某張詢價單報了價、
  買家卻接受到另一份內容**時才爆——那時是真實交易。
- B 的代價由「現在沒有真實買家、capability flag 全 OFF」這個窗口吸收。
  **這個窗口會關，而且關掉之後不會再開。**

## 執行約束

B 必須排在 publication 動工**之前**——publication preview 一開始就要編譯 seller-facing payload。

## 後果

- preimage 擴大到 11 欄；`canonical_payload_json` 自帶
  `item`／`itemResolution`／`quantity`／`unit`／`packaging`／`deliveryCity`／`deliveryDate`，
  編譯 seller-facing payload 時**只讀這一份 JSON**，不 join `procurement_requests` 或 revision 表。
- 鍵形狀全 camelCase，由測試釘住。
- 重新評估條件：若 seller-facing 投影的欄位集合再次變動，要同步檢查
  「新欄位加入前建立的 approval 讀回什麼」——本 repo 的慣例是指標 ＋ `omitempty`，
  讓既有 `seller_facing_payload_hash` 逐位元不變。
