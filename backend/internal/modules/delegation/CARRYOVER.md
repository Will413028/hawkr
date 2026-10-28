# delegation 模組的沿用盤點

從 account-service 沿用的每個機制重新判斷的結論（遷移計畫「模組完成的定義」），以及與 account-service 的差異。模組現行的職責、資料與不變量在同目錄的 `AGENTS.md`，行為規格在 `docs/modules/delegation.md`。

## 沿用盤點

每列重新檢查 account-service 的前提。設計書 56 的業務不變量仍有效；單體歸屬、FK、ID 與事件依較新的 ADR。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 管理員操作稽核與 session 建立時的 IP 欄 | 管理員操作需要逐次 IP 證據 | 不成立：決定不記錄，兩欄沒有寫入值 | 無寫入端的資料欄不建立；登入時 IP 可由 Access 紀錄查詢 | 刪除 `client_ip` 與 `create_client_ip` 欄、舊 proto 欄位及 UI 的 IP 顯示。 |
| 四表 TSID、session ID 為十進位 | 單一發號者，舊端讀 bigint | 前者不成立，後者仍成立 | UUIDv7；排序用時間加 ID | session 暫用 bigint IDENTITY，其他三表用 UUIDv7；數字消費端退場後換 session ID。 |
| `display_code` 從 ID 推導 | ID 不可列舉 | IDENTITY 不成立 | 隨機、不透明且 UNIQUE 的顯示碼 | 保留 `OPS-` 加 13 碼格式，以 CSPRNG 產生、撞號重試。 |
| credential 的核發時機與儲存 | 建立與兌換分屬不同 host | 成立；建立只交付 handoff code | 兌換時產生 256-bit 隨機 credential，僅存 hash | S6 的單句 CAS 同時清 code hash、寫 credential hash；同 code 再兌換回 9282。已兌換 session 的同鍵重放只回摘要，不再發 code 或改寫 hash。 |
| 一次性 handoff code 用七述詞 CAS | 跨 host 只兌換一次 | 成立 | 短效隨機碼、雜湊儲存、原子消耗 | 沿用，時間由模組時鐘傳入；重放與失效共用既定錯誤。 |
| PENDING intent、跨服務寫入、confirm | gateway 仍編排且跨庫 | 成立到 A10 | 同庫時單 transaction；跨庫時明確 intent/confirm | 保留兩階段與批次 50 上限；A10 重新收斂。 |
| 自建 outbox dispatcher、lease 與退避 | account-service 獨立部署 | 不成立 | 同 transaction 發事件，訂閱者排可靠 job | `DelegationOperationCompleted` 同 transaction 發出；legacy 訂閱者排 River/RabbitMQ job。 |
| hard 1800 秒、idle 900 秒，讀取時推導 | 有短效工作階段 | 成立 | 儲存終態、期限以當前時間計算 | 沿用全部進行中述詞；活動只由資源反查順延。 |
| 九個 capability；action 無 CHECK | 能力封閉、稽核動作可能擴充 | 能力封閉成立；無 CHECK 違背設計書 §6.5.3 | Go 枚舉與 DB CHECK 一起管儲存碼 | 九能力與 19 個 action 均 CHECK，reason_code 亦 CHECK。 |
| 店家授權遮蔽全域授權 | 政策可能存在全域列 | 設計書保留語意，營運不建全域列 | 明確優先序、不要過濾無效店家列 | 沿用店家優先的判定與 schema 的 nullable store ID。 |
| `DELEGATION_*` 行為環境變數 | 各部署需不同業務規則 | 不成立 | 規則為常數，憑證由 CSPRNG 核發 | TTL 等沿用預設值作常數；移除 credential 金鑰與輪替環境變數。 |
| 稽核保留 730 天只顯示，維運 SQL 清理 | 本期保留承諾 | 成立，系統未上線無舊資料 | 明確保留政策與執行者 | 沿用設計書 §8.5.1 的常數與維運清理，不清 session。 |
| 模組直接呼叫 seller-service 讀狀態 | 寫入前需驗店家狀態 | 判定需求成立；seller 搬進單體後直接依賴成立（依賴表允許 delegation → seller） | 資料擁有者的公開 API 在請求路徑上回答，不經過傳入的快照 | A3b 步驟 9d：模組以 `Deps.Seller` 讀店家（只有不存在是 `DenialStoreNotDelegable`（legacy 對應 9265），已關閉是 `DenialStoreClosed`（9265 只在 legacy `bizCodes`）；可營運與句子來自 seller 的 `StoreActionDelegate`（第 5b 刀）、seller 的其他錯誤原樣傳出、不當成拒絕：seller 在行程內，沒有跨服務的「暫時讀不到」，所以沒有「讀不到」的狀態；`Deps.Seller` 必填、建構時為 nil 就 panic），公開 API 不再收 `StoreState`。 |
| `revision` 僅在結束或撤銷時增加 | 前端對帳狀態轉移 | 成立 | 版本代表對外可見的狀態變化 | 沿用；授權寫入以互斥 CAS 判定。 |
| 三處冪等鍵、稽核 hash 不比對 | 重送不得重做 | 鍵需求成立，忽略不同 payload 不成立 | 鍵綁定請求語意，衝突回錯 | 三處唯一鍵與 `ON CONFLICT DO NOTHING`；稽核重放比對 action、resource、payload hash。 |
| CodeMeta 下發中文標籤、session 存理由快照 | 舊契約仍讀文字 | 到 A10 成立 | 文字由 client 呈現；事實快照按需保存 | 過渡期標籤由 delegation 的 Go 表提供，理由快照保留；A10 由 client 呈現。 |
| `admin_accounts.delegation_enabled` 人工 SQL，取消不撤銷 | 小型營運名單 | 資格屬代行；跨 schema 兩步無原子性 | 授權 context 擁有資格，撤回與 session 同 transaction | `delegation.admin_eligibilities` 保管資格；`SetAdminEligibility` 撤回時一併撤銷 active session。 |
| gateway 登出呼叫 `RevokeSessionsByActor` | 登出與撤銷分服務 | Access 處理登出，identity 沒有登出寫入 | 上層直接呼叫授權擁有者 | `internal/httpapi/identity` 直接呼叫 delegation 撤銷；legacy RPC 不實作。 |
| 稽核 `resource_id` bigint 與未使用的 image key | 資源 ID 都是數字 | 不成立，訂單以訂單號指稱 | 不透明 ID 用 text | `resource_id` 用 text；移除未寫入的 image key。 |
| `varchar(n)`、status/notify 預設、未使用索引 | DB 防止違反產品長度，省略欄位有預設 | 產品長度成立，其餘不成立 | CHECK 業務長度，必要欄位顯式寫入 | 保留產品長度 CHECK，其餘 text；移除兩個 DEFAULT 及無讀取端索引。 |
| OFFSET 分頁及總數 | 舊契約用頁碼 | 到 A10 成立 | 新契約用 keyset/page token | 過渡期保留，A10 改契約時再換。 |
| 與 identity 管理員表的 FK | 四表與管理員在同 schema | 已不成立 | 跨模組只存 ID，讀取經公開 API | 五表無跨 schema FK，並由 identity 公開 API 查管理員。 |

## 傳輸層的沿用盤點

舊 gateway 的對外路徑與 gRPC adapter 仍為過渡層；A10 的 Connect 契約依〈API 設計規範（Connect）〉。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| gateway 解析管理員身分並傳 actor ID | 舊 gateway 仍提供代行 HTTP | 過渡期成立 | 入口驗證，模組只接受已驗身分 | legacy adapter 維持舊身分映射；新入口由 `internal/httpapi` 驗 Access token。 |
| gRPC 業務碼 9260–9299、中文拒絕訊息 | 舊 client 依現行契約處理 | 到 A10 成立 | 模組回錯誤種類，入口映射協定與文案 | 核心用 `DelegationError`，legacy adapter 保留舊碼；新契約用 Connect `ErrorReason`。 |
| gateway 以 OFFSET 回 page、page_size、total | 舊列表 UI 使用頁碼 | 到 A10 成立 | keyset/page token | 只在 legacy 契約沿用，A10 改 Connect 時換。 |
| gateway/服務間的即時讀取，無代行快取 | 撤銷必須立即生效 | 成立 | 授權事實每請求重查，避免快取失效競態 | 不加可跨撤銷的快取；以 DB 查詢判定。 |
| gateway 的請求大小與限流 | 代行入口需防濫用 | 成立；規則屬入口 | 邊界統一限制大小與頻率 | 過渡期 gateway 原有護欄，新 Connect 由 `internal/httpapi` 的共用護欄管理。 |
| gateway 產生/傳遞 trace、request ID | 跨服務稽核須串聯 | 成立 | 可信入口產生 ID 並往下傳 | adapter 保存既有 correlation/request ID；新入口依共用 tracing 規則。 |
| seller-service 讀取 1200ms deadline，confirm 重試 | 舊跨服務編排 | 店家讀取不再跨服務，不成立；confirm 重試到 A10 成立 | 同程序呼叫沿用請求的 ctx，冪等重試 | 店家讀取是同程序呼叫，沒有獨立 deadline；confirm 重試留 gateway，模組不引入下游重試。 |

## 與 account-service 的差異

資料沒有搬移（系統從未上線）；五張代行表位於獨立 `delegation` schema，管理員 ID 不下跨 schema FK。代行操作事件走同步 transaction bus 與 legacy job，不再有代行 outbox 表。其餘對外與 schema 差異以上方盤點及設計書 56 為準。
