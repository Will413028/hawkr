# 商家名單：聯絡方式是可編輯的清單，撞到聯絡鍵是比對訊號而不是唯一鍵

| 項目 | 值 |
|---|---|
| 狀態 | accepted（Will 2026-10-04） |
| 日期 | 2026-10-04 |
| 影響範圍 | `internal/modules/seller`（prospect）、`internal/modules/agent`（vendor portal 取 email）、`internal/legacy`（ai-service 的 prospect 搜尋與網站）、admin 的商家名單（步驟 10） |
| 計畫 | `docs/plans/2026-09-29-a3-buyer-seller.md` 步驟 8h-1；取代 seller `CARRYOVER.md`「比對鍵存成 4 個單值欄位」與「`status <> 'ARCHIVED'` 的 T2 述詞」兩列的 7c 決定（Will 2026-10-02） |

## 脈絡

7c 把商家名單的比對鍵從 4 個單值欄位改成識別子子表 `prospect_identifiers`，`(kind, value_normalized)` 在 live 的業者之間由 partial UNIQUE 保證唯一：「同一家」由資料庫保證，撞到就拒絕，沒有覆寫。8h 開工盤點（快照 f6901018d）時，這個形狀在管理員實際操作的路徑上有四個問題：

- **誤判沒有出路**：解析器 `BuildProspectMatchKeys` 對網站欄位取 host、丟掉路徑；`prospectSocialHosts` 沒涵蓋的共用平台（`linktr.ee/<x>`、`pinkoi.com/store/<x>`）變成網域鍵 `linktr.ee`，第二家用同一平台的業者建不起來。合作社、家族農場共用電話或 email 同理。舊實作只提示重複。
- **鍵看不到**：比對鍵由 `public_email` 等單值欄位推導，合併帶來的鍵沒有來源欄位，管理員看不到也無法直接移除（只能先把 `public_*` 設成那個值再改掉）。
- **還原卡死**：封存期間鍵被別的業者佔走時，Restore 一定失敗，沒有出路。
- **單值欄位**：一家業者有多個 email、電話時只能存一個。

約束：

- 外部：沒有。系統未上線，名單只有 admin 寫入。
- 消費端：vendor portal 寄信要一個 email（`seller.ProspectContact`，ADR `2026-10-03-vendor-portal-invite-scoped-access.md`）；ai-service 讀業者網站，而且依資料類別只放行網站、擋下 email、電話、社群（`ai-service/internal/procurement/domain/artifact.go`），所以聯絡方式要保留類型。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：MDM 的比對與資料管理（Informatica MDM、Reltio）：比對規則產生候選配對，由資料管理者合併或標記「不是同一家」，標記留存、不再出現；資料庫只對權威識別碼強制唯一；golden record 保有多個聯絡方式。Salesforce 的 duplicate rule 同樣分「擋下」與「提示後仍可儲存」；HubSpot 以網域去重公司並排除共用網域 | 多一張「不是同一家」的表；同時建立同一家業者時兩筆都會成功，之後出現在疑似重複清單 | 採用 |
| 維持 7c：聯絡鍵由資料庫唯一，只擴充解析器的共用平台表 | 平台表永遠不完整，下一個沒列到的平台或共用電話照樣擋死，沒有覆寫；鍵仍然看不到 | 否決 |
| 維持唯一鍵，另開移除識別子的方法 | 解決看不到與還原卡死，不解決真的是兩家卻共用聯絡方式的情況 | 否決 |
| 只提示、不擋（舊實作） | 管理員沒看到提示就建出重複，疑似重複清單變成主要的去重手段 | 否決 |

## 決策

- **D1 權威識別碼**：統編維持 live 列之間的資料庫唯一（`prospects_tax_id`）。還原或修改撞到統編回 `FAILED_PRECONDITION`，該做的是合併。
- **D2 聯絡方式是清單**：`public_email`、`public_phone`、`public_website`、`public_social` 與 `prospect_identifiers` 刪除，改成 `prospect_contacts`：每列是類型（`EMAIL`、`PHONE`、`WEBSITE`、`SOCIAL`）、管理員輸入的值、由解析器推導的比對鍵（可為 NULL：解析器判定不具識別力時），有某種類型的聯絡方式時，該類型恰好一個主要的。對外是資源上可寫的 repeated 欄位，整批替換（同 API 規範 §七 多張圖片的「狀態而不是指令」）。vendor portal 用主要 email，ai-service 用主要網站。
- **D3 撞到聯絡鍵是比對訊號**：比對鍵不設唯一。Create、Update（改到聯絡方式時）、Restore 時，與其他 live 業者有相同比對鍵、而且沒有標記為「不是同一家」的，回 `FAILED_PRECONDITION`，`ErrorInfo.reason` 是 `PROSPECT_MATCH_FOUND`、列出相符的業者；client 依它顯示相符的業者，讓管理員選擇合併，或在請求帶上確認不是同一家的業者 ID 再送一次。確認寫入 `prospect_distinct_pairs`（較小的 ID 在前、記管理員與時間），之後這一對不再擋、不再出現在疑似重複清單。
- **D4 疑似重複清單**：尚未處理的候選配對，來源是相同比對鍵（並發建立漏網的）與名稱相同（忽略大小寫與前後空白），排除已標記不是同一家的；keyset 分頁。
- **D5 合併**：輸家的聯絡方式併入贏家（同一個比對鍵或同一個值只留一列，贏家的主要聯絡方式不變），輸家的「不是同一家」配對改掛贏家（變成自己對自己或重複的刪除）；管理員明確合併一對曾標記不是同一家的業者時，刪除那筆標記。其餘照 7c（統編、分類、圖片、事件）。
- **D6 解析器**：網站欄位加共用平台的排除清單（照 HubSpot 的共用網域排除）：host 在清單裡時比對鍵為 NULL。社群平台表與既有規則不變。

## 後果

**接受的代價**

- 兩個管理員同時建立同一家業者時兩筆都會成功，由疑似重複清單收斂。
- 共用 email 的兩家業者各自收得到 vendor portal 的邀請；邀請以 RFQ 收件人為範圍，權限不外溢。
- admin 的建立、修改、還原流程多一個「確認不是同一家」的分支（步驟 10，UI 改動先問 Will）。

**應該重新評估的條件**

- 疑似重複清單長期累積、沒有人處理：評估自動合併的規則（只對統編或完全相同的強識別子）。
- 「不是同一家」的標記被濫用來繞過比對：加理由欄或改成需要第二位管理員確認。

## 修訂：直接標記配對（2026-10-04，設計審查依 D4 的基準補上）

原決策段保留歷史。D3 的「不是同一家」標記只能在建立、修改、還原撞到比對鍵時確認；D4 的疑似重複清單因此有兩種配對處理不掉：名稱相同的配對，以及並行建立而漏網的配對（沒有任何一次寫入可以順帶確認），唯一的出路是合併，而合併不可逆。D4 的基準（MDM 的資料管理者對候選配對標記「不是同一家」）本來就包含直接標記。

- **決定**：D4 清單裡的配對由合併，或由管理員直接標記不是同一家處理，與 D3 在建立、修改、還原時的確認並列。直接標記寫入同一張 `prospect_distinct_pairs`（較小的 ID 在前、管理員、時間），效果與確認相同：這一對不再出現在清單，也不再擋建立、修改與還原。
- **規則**：標記是身分的決定，不是內容的決定，所以不帶 etag（etag ADR 2026-10-04 修訂的判準）；兩個業者都必須存在而且是進行中的（D4 的候選只在進行中的業者之間）；已標記的重送回傳成功、不改寫第一次的管理員與時間；沒有清除標記的方法，明確合併一對標記過的業者就是撤銷它（D5）。
- **API**：`SellerProspectAdminService.MarkProspectsDistinct`，權限 `seller.prospect_distinct_pairs.create`（A3b 步驟 8h-2）。

## 修訂：統編衝突帶 reason，live 的定義（2026-10-21，A3b 步驟 10e 第 4b 刀）

- **live 是未封存**：D1、D3 的 live 指進行中與已入駐（`status <> 'ARCHIVED'`），統編唯一與比對鍵相符都以它為準；合併與直接標記仍只限進行中。契約與模組註解寫成「進行中」的地方改正。
- **D1 的統編衝突帶 reason**：建立（`ALREADY_EXISTS`）、修改與還原（`FAILED_PRECONDITION`）撞到統編都帶 `ErrorInfo.reason` `PROSPECT_TAX_ID_HELD`，metadata `prospectIds` 是持有者。理由：admin 修改時要能直接與持有者合併（D1「該做的是合併」），而同一個錯誤條件只有一個形狀（AIP-193）。持有者已入駐、或還原時撞到，不能合併，伺服器句子改說封存這筆或不需要還原。並行寫入同一個統編時，交易在檢查持有者之前以統編取交易級 advisory lock（`pg_advisory_xact_lock`），同一個統編的寫入排隊，應用層的檢查就是權威、錯誤形狀相同；資料庫唯一仍保留，撞到代表程式有錯，當內部錯誤。
- **標記之後的出路**：admin 沒有一般的「合併到其他業者」，所以標記過不是同一家的一對在畫面上無法再合併；確認框照實說明。重新評估的條件：BD 要合併兩筆從不相撞的業者，或第一次有人標錯。

