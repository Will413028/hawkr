# 修改以 etag 做樂觀鎖，取代「最後寫入者獲勝」

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-25 |
| 影響範圍 | 單體所有模組中，client 讀取後修改的資源；對外契約的 Update 與 Delete |
| 上層決策 | `2026-09-24-go-modular-monolith.md` D7；Connect 試點計畫步驟 3 引用本 ADR |
| 修訂 | 2026-10-04：改變狀態的自訂方法的判準、帶 etag 的指令不推斷重送（文末）；2026-09-25：D2 的業務原因改由 `2026-09-25-api-error-model.md` 規定（proto 的 `ErrorReason` 放進 `google.rpc.ErrorInfo.reason`，名稱仍是 `VERSION_CONFLICT`） |

## 脈絡

後台修改 banner、食譜這類資源是整筆覆寫。兩個人同時修改同一筆時，後存檔的一方會靜默蓋掉前一方；換圖時，在 media 搬進單體（遷移計畫 B1）之前，先 commit 的一方釋放舊引用，還可能讓 GC 刪掉正在顯示的圖（`docs/modules/content.md` §6）。

約束：

- 外部：沒有。系統從未上線，會寫入這些資源的只有 admin，與後端一起部署。
- 繼承：設計書 50 §7.4、54 §5.3 以「低頻、不會同時改」明訂不加樂觀鎖、最後寫入者獲勝。現在不成立：頻率低只讓衝突少見，不讓它無害；當時也沒考慮到換圖會讓 GC 刪圖。
- 繼承：seller、invoice、social、delegation 已經各自以 `expectedVersion` 加 `version = version + 1` 實作樂觀鎖，CodeMeta 有 6 個各領域的 `*_VERSION_CONFLICT`（3026、9234、9266、9308、9322、9343），其中 9234 刻意回 412、與 409 區分。到那些模組搬遷前仍在運作。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：資源帶 etag，修改時比對（Google AIP-154；HTTP 的 `ETag`／`If-Match`，RFC 9110 §13.1.1）；Contentful、Kubernetes 的 `resourceVersion` 同一做法，repo 裡 social-service #2772 也是 | 每張表多一個版本欄；client 要處理衝突 | 採用 |
| 維持最後寫入者獲勝 | 靜默遺失修改；換圖時可能刪掉正在顯示的圖 | 否決 |
| 以 `updated_at` 當版本 | 它是給人看的修改時間，取的是 transaction 開始的時間，精度與語意都不適合比對 | 否決 |
| 以 Postgres 的 `xmin` 當版本 | 任何 UPDATE 都會改它，包括 `view_count` 這類與內容無關的計數，會誤判衝突 | 否決 |
| 只用 field mask 部分更新，不比對版本 | 兩個人改同一個欄位照樣靜默蓋掉；server 以讀取、合併、寫回實作時，改不同欄位也會蓋掉 | 否決 |

## 決策

- **D1 適用範圍**：client 以讀到的內容為基礎送出的修改（Update 與 Delete），不論整筆覆寫還是 field mask。這類資源在回應裡帶 `etag`，由伺服器產生，client 當成不透明字串。
- **D2 比對**：Update 必須帶讀取時拿到的 etag；沒帶回 `INVALID_ARGUMENT`（client 的錯），不符回 `ABORTED`，不寫入。Delete 可以帶，帶了就同樣檢查。版本衝突只有一個業務碼（CodeMeta 新增一個通用的版本衝突碼 `VERSION_CONFLICT`，放在錯誤的 detail：Connect 是 AIP-193 形狀的 `ErrorInfo.reason`），前端依它判斷，不靠 status code。
- **D3 以 key 建立或修改的資源**（例如站點文案、版本門檻的 upsert）：沒帶 etag 表示只建立，已經存在就拒絕，業務碼與 D2 的版本衝突相同；帶了表示只修改，且要相符。Connect 以 `ALREADY_EXISTS` 回應（AIP-133；2026-09-25 修訂，原本寫 `ABORTED`，Will 決定）；HTTP 是 `If-None-Match: *` 不成立的 412。SQL 照 PostgreSQL 設計規範 §十二：建立用 `INSERT … ON CONFLICT DO NOTHING`，修改用下面的條件式 UPDATE，0 列就是條件不成立。
- **D4 版本**：
  - aggregate root 的表加 `version bigint NOT NULL DEFAULT 1`，etag 由它產生；子表（例如食材、步驟）的寫入一律經過 root，並讓 root 的版本加一。
  - 修改是 `UPDATE … SET version = version + 1 WHERE id = $1 AND version = $2`；0 列時再分辨是不存在（`NOT_FOUND`）還是版本不符（`ABORTED`）。
  - Update 只寫 client 可以編輯的欄位。會改到這些欄位的其他寫入路徑（狀態轉移、worker）也要讓版本加一；計數這類 client 不能編輯的欄位不加（例如 `view_count`）。
  - 以自然鍵識別、而且可以刪除再重建的資源，etag 要能區分不同的建立（例如併入 `created_at`），否則重建後版本從 1 重來會誤判相符。
- **D5 wire 對應**：Connect 是訊息裡的 `etag` 欄位，不符回 `ABORTED`（Connect 對應 HTTP 409）；HTTP／OpenAPI 是回應的 `ETag` header 與請求的 `If-Match`，不符回 412（RFC 9110 §15.5.13）。資源的 etag 與公開讀取的 HTTP 快取 `ETag`（條件請求、304）是兩件事，各自處理。
- **D6 既有的樂觀鎖**：seller、invoice、social、delegation 的 `expectedVersion` 與 6 個各領域的碼，在那些模組搬遷時改成本 ADR 的做法（etag 與 D2 的單一衝突碼），列進各模組的沿用盤點表。9234 用 412 區分「要不要清掉草稿」的需求，改由前端依 D2 的業務碼決定。

## 後果

**接受的代價**

- 每個可修改的資源多一個版本欄，每支 Update 多一個必填欄位。
- 前端要處理衝突：保留使用者的輸入，並顯示最新版本讓使用者對照後再送出，不直接重新載入（Will 2026-09-25 決定，admin 在 Connect 試點步驟 6 實作）。
- 被拒的一方可能已經轉正了新圖；實作時在轉正之前先比對一次版本（最後仍以條件式 UPDATE 為準），減少注定被拒的轉正。
- B1 之前仍有一個很小的時間窗：勝方 commit 之後、釋放舊引用完成之前，若另一個人讀到新版本並完成換圖，勝方的釋放仍可能刪掉對方剛登記的引用。B1 把引用移進業務列的 transaction 後消失。

**應該重新評估的條件**

- 出現多人需要同時編輯同一筆、而且衝突頻繁到拒絕會妨礙工作的資源：改評估欄位層級的合併。

## 修訂：改變狀態的自訂方法（2026-10-04，Will 決定）

原決策段保留歷史。D1 只涵蓋 Update 與 Delete，自訂方法各自決定：ApproveKYC、RejectKYC 必帶 etag，RebindPayUni、治理的通報指令不帶，8h 商家名單的合併、轉換、封存沒有依據可循。基準（AIP-154：請求依據讀到的內容時帶 etag；Kubernetes 的 `resourceVersion` 前置條件）不以方法種類區分，而以「狀態檢查擋不擋得住過時的決定」區分。

- **判準**：改變狀態的自訂方法，若決定依據的內容在狀態不變時仍可能被改（管理員依某個版本做了決定，期間別人改了內容但狀態沒變），請求的 etag 必填，沒帶回 `INVALID_ARGUMENT`、不符回 `ABORTED`（同 D2）。決定只依據狀態機已經檢查的狀態、或依據不可變的內容的，不帶 etag，由狀態檢查保護。
- **套用**：新方法照判準設計；既有的自訂方法在 A3b 步驟 8h-1 逐支以判準檢查，需要的補上 etag（資源沒有版本欄的先補，照 D4），結果記在各模組的 `CARRYOVER.md`。

## 修訂：帶 etag 的指令不推斷重送（2026-10-04，Will 決定）

原決策段保留歷史。ApproveKYC 起，帶 etag 的自訂方法以「帶的版本正好是生效前一版、而且資源已經是這個指令的結果」推斷為重送並回傳成功；8h 複製到另外 6 支後，每支各有一套推斷規則，並出現兩次誤判（別人的寫入讓版本也正好多一）。規範 §八 規定 client 只自動重試 `NO_SIDE_EFFECTS`、`IDEMPOTENT` 或帶 `request_id` 的方法，這些指令都不是，推斷只在處理人為的重按。基準（Kubernetes 的 `resourceVersion`、AIP-154）是重送照樣回衝突，client 重讀後看到已經生效；需要安全的自動重試時用 AIP-155 的 `request_id`。

- **決定**：帶 etag 的指令不推斷重送：版本不符一律 `ABORTED`（`VERSION_CONFLICT`），不寫入；client 重讀後依目前的狀態呈現。
- **重新評估的條件**：有 client 需要自動重試這類指令（例如離線佇列、網路逾時後自動重送）時，該方法加 AIP-155 的 `request_id`，不恢復版本推斷。
