# media 切換不保留舊上傳路徑：舊服務的上傳流程暫時失敗到各自改寫

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-11-21（Will 決定） |
| 影響範圍 | website、admin、buyer-app、seller-app 的上傳；gateway 的 media 路由與 `openapi.yaml`；Java product、shipping、order、aftersales；舊 Go review、im、invoice、ai 服務 |
| 執行計畫 | `backend/docs/plans/2026-11-21-b1-media.md`（決定 B1-D7、B1-D11；步驟 B1-4、B1-5） |

## 脈絡

Go media 取代 Java media-service 時，還有服務吃 client 經 Java media 上傳會話拿到的 staged token：

- external（現況）：四個 Java 服務（商品圖、出貨照片、售後佐證、確認收貨照片，21 條流程）與四個舊 Go 服務（評價圖、聊天傳圖、稅籍證明、發票解鎖佐證、AI 風格縮圖，8 條）。每條呼叫點見計畫附件 `upload-routing.md`。它們要到 B3、B4、B8、B9、B10、A6-6 與 ai-service 搬遷才改寫。
- external：Java media-service 在 B1 期間不改程式（ADR `2026-10-03-media-references-owner-side.md` 脈絡段）。
- 上線前不受相容性約束（根 `AGENTS.md`）：系統從未上線，沒有使用者會碰到失敗的流程；本機開發資料可以重建。
- inherited：「不留技術債、不寫之後要拆的東西」（根 `AGENTS.md` 共通規則）。今天仍成立，是 repo 現行規則。

## 考慮過的方案

業界基準：取代一個服務時用 strangler fig，新舊並存、依功能逐步把流量切到新系統，最後退場舊系統（Martin Fowler〈StranglerFigApplication〉，<https://martinfowler.com/bliki/StranglerFigApplication.html>）。這個基準是為「有使用者在線、不能中斷」設計的。

| 方案 | 代價 | 判定 |
|---|---|---|
| A′ 基準：兩套並存，client 依呼叫點分流（Go 擁有者打 Go media，Java 與舊 Go 服務的打舊路徑），舊路徑隨各服務改寫逐一刪 | 過渡期 client 有兩套上傳碼，gateway 的舊路由與 `openapi.yaml` 一路保留到最後一個服務遷出 | ✗ |
| B 單體替 Java 與舊 Go 服務開舊契約（`media.proto` 子集）的 gRPC 轉接面，client 全部打 Go media；好處是每一步都保有可驗證的端到端上傳 | 要在 Go 實作 Finalize／Link／Unlink 的引用語意給 Java 用，與「media 不認識業務」的終點矛盾，而且是之後要拆的轉接層 | ✗ |
| C 把 B1 排在 B3、B4、B8、B9 之後 | 那些改寫本身就需要 Go media，形成循環 | ✗ |
| C′ 只把 B1-4、B1-5 排在 A6-6 之後（評價、聊天先成為單體擁有者） | 少 5 條失敗，但 B1 的 client 切換被 A6 的進度擋住；仍剩發票、AI 縮圖 3 條失敗 | ✗ |
| **D 一次切完：四個 client 全部改打 Go media 的 Connect，刪 gateway 的 media 路由，不保留舊路徑、不寫轉接；收不下新 token 的舊服務流程在本機失敗，直到各自改寫** | 29 條上傳流程在本機暫時不能用，本機整合與 e2e 對這些流程跑不全，可能掩蓋同一流程的其他回歸；每個服務改寫時要順便恢復它的上傳（已列為延後項與觸發條件） | ✓ |

## 決策

採 D。Java media-service 原封不動，等最後一個 Java 呼叫端與 invoice-service（B10）遷出後才退場；B1 期間不為它新增任何程式。失敗的流程清單在 `upload-routing.md` 第三、四節，B1 收尾時搬回主清單，每條附觸發條件（所屬服務的改寫步驟）。

## 理由

取捨是「29 條上傳流程在本機暫時失敗」換「沒有兩套上傳碼、沒有轉接層、gateway 的 media 路由一次刪完」。strangler fig 的前提是有使用者不能中斷，這裡不成立：系統未上線，失敗只影響本機開發，而且每條都有明確的恢復步驟。A′ 與 B 留下的東西注定要在之後的步驟拆掉，違反 repo 的「不留技術債」規則。C′ 能少一些失敗，但讓 B1 依賴另一個計畫的進度，換來的只是本機少幾條暫時失敗的流程。這些流程的端到端驗證本來就在各自的改寫步驟重寫（新契約、新擁有者），B 為了保住舊流程的驗證而寫的轉接層，等價性出錯時比明確失敗更難發現。

## 預期結果

- B1-4 完成後，client 端打舊 media 路徑的搜尋指令結果為 0（計畫 B1-3 的消費端指令）。
- gateway 的 media 路由與 `openapi.yaml` 對應 operation 全部刪除。
- 29 條失敗流程逐條出現在主清單，附各自的恢復步驟。

## Followup

- B1-4、B1-5 連著做（client 一次切完，七個 Go 擁有者在同一單元改吃新 token）。
- B1-8：失敗流程清單搬回主清單。
- 各服務改寫（B3、B4、B8、B9、B10、A6-6、ai-service 搬遷）時改用 Go media 的 `Confirm`。

## 撤銷條件

- 系統在這些服務改寫前就要上線或給外部使用者試用：失敗的流程不能接受，改回 A′ 並保留舊路徑到改寫完成。

## 相關

- 計畫 `backend/docs/plans/2026-11-21-b1-media.md`（`c8870685` 起），附件 `upload-routing.md`、`interface-inventory.md`。
