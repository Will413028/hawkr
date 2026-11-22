# media 上傳：公開圖片預簽直傳暫存區後非同步處理，機敏檔經單體串流加密，暫存憑證是不透明 token

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-11-21（Will 決定方向）；2026-11-22（Will 決定機敏檔改經單體串流） |
| 影響範圍 | `backend/internal/modules/media`、`api/` 的 media 契約、website、admin、buyer-app、seller-app 的上傳 |
| 執行計畫 | `backend/docs/plans/2026-11-21-b1-media.md`（決定 B1-D2、B1-D5；B1-2 共用段的狀態機） |

## 脈絡

media 從 Java 改寫成單體內的 Go 模組時，上傳通道與暫存憑證要重新決定。約束：

- external：物件儲存是 Cloudflare R2（ADR `docs/adr/2026-09-18-media-storage-r2-image-gateway.md`）。R2 不支援 HTML 表單 POST，所以沒有 POST policy 的 `content-length-range`；預簽 PUT 可以把確切的 `Content-Length`、`Content-Type`、`Content-MD5`、`If-None-Match: *` 簽進網址，不符就拒絕而且不落檔（官方文件 <https://developers.cloudflare.com/r2/api/s3/presigned-urls/> 與 sandbox bucket 實測，計畫附件 `spike.md` (e)）。
- external：機敏檔（KYC 證件、稅籍證明、治理佐證）在 R2 上只能是 media 應用層加密的密文（ADR `2026-07-20-seller-kyc-pii-encryption-scope.md`）。
- external：client 有兩個 Next.js 網站與兩支 Flutter App。website 經 Vercel 的同源反代打單體，反代路徑的請求大小上限未量測。
- inherited：外部 provider 不在 DB transaction 內呼叫（`backend/AGENTS.md` 單體規則 5）。今天仍成立：R2 的延遲不受控，transaction 內等它會佔住連線池。
- inherited：Java 的暫存憑證是 HS256 JWT（`StagedTokenService.java`）。理由是 Java 服務之間沒有共用的表；media 搬進單體、有自己的表之後，這個理由不成立，不當約束。
- 上線前不受相容性約束（根 `AGENTS.md`）：client 的上傳碼可以整個換掉。

## 考慮過的方案

業界基準：client 以短效預簽網址直傳物件儲存的私有暫存區，伺服器在 client 回報完成後非同步驗證與處理，處理完成前不可使用。例：Rails Active Storage direct upload（<https://guides.rubyonrails.org/active_storage_overview.html#direct-uploads>）、Shopify staged uploads（<https://shopify.dev/docs/api/admin-graphql/latest/mutations/stagedUploadsCreate>）、AWS presigned URL 上傳（<https://docs.aws.amazon.com/AmazonS3/latest/userguide/PresignedUrlUploadObject.html>）。

| 方案 | 代價 | 判定 |
|---|---|---|
| A 基準，兩種檔案都預簽直傳 `tmp/`，`CompleteUpload` 後由 River job 處理（public 剝 metadata，secure 加密） | 機敏檔的明文會停在 R2 的 `tmp/` 直到處理完成；刪失敗時靠 GC 與 bucket lifecycle（約 1 到 2 天） | ✗（機敏檔） |
| **A′：public 照 A；secure 由 client 經單體上傳，單體邊收邊加密後寫入 `secure/`** | 兩條上傳路徑、兩套測試；機敏檔的上傳流量經單體；website 反代路徑的大小上限要先量 | ✓ |
| B 全部經單體串流 | public 圖片的上傳流量與長連線也壓在應用伺服器；有物件儲存時業界不這樣做 | ✗ |
| C 直傳後在 `CompleteUpload` 請求內同步處理 | 處理時間算進請求期限；R2 失敗時沒有重試 | ✗ |
| 憑證 D：沿用 HS JWT | 簽章是和資料表並存的第二份真相；撤銷要另建黑名單 | ✗ |
| **憑證 E：不透明隨機 token，表只存雜湊，一對一對應暫存列** | 每次驗證查一次表（同一個 Postgres，成本可忽略） | ✓ |

## 決策

上傳走 A′，憑證用 E。

- public：`BeginUpload` 檢查宣告的大小與型別後建暫存列，回預簽 PUT（綁上述四個 header，key 是伺服器產生的 `tmp/<uuid v4>`，不含檔名）與 token → client 直傳 → `CompleteUpload` 驗上傳者、列標 `uploaded` 並在同一個 transaction 排處理 job → job 在 transaction 外依 magic bytes 辨認格式、剝除 metadata，寫正式 key 後以條件式 UPDATE 建資產列並標 `ready` 或 `rejected`。
- secure：client 把檔案送到單體的上傳端點，單體在請求內依 magic bytes 辨認格式（PDF 或圖片）、邊讀邊加密（格式見 ADR `2026-11-22-media-secure-stream-aead.md`）寫入 `secure/`，成功後建資產列與 `ready` 的暫存列並回 token。明文不落 R2。
- 兩種都由擁有者在自己的 transaction 內 `Confirm`（ADR `2026-10-03-media-references-owner-side.md` 2026-11-22 修訂）。過期未確認的暫存由 GC 連同物件與資產列刪除；`tmp/` 另有 bucket lifecycle 兜底。

## 理由

public 圖片：取捨是「多一個非同步狀態與 client 輪詢」換「上傳流量不經應用伺服器、大小與型別在儲存端就被擋、處理失敗可重試」。R2 只能簽確切大小不是缺點：client 本來就知道檔案大小，伺服器在發網址前檢查上限即可（缺陷 5.8 的單一上限）。確認前的 public 物件已在 `public/<uuid>`，image gateway 會提供它；uuid 是 v4 隨機值、只回給上傳者，等同不可猜的網址，接受。

機敏檔：取捨是「多一條上傳路徑、機敏檔流量經單體」換「明文永遠不落在第三方儲存」。A 的明文窗口平常只有數秒，但刪除失敗時會停到 bucket lifecycle 執行（最長約兩天），而且 Cloudflare 持有 R2 靜態加密的金鑰，擋不住拿到 bucket 讀取權限的人；機敏檔的量小（KYC、稅籍、佐證），經單體的頻寬代價低。

## 預期結果

- public 圖片的上傳流量不經單體；超過上限或型別不符的上傳在 R2 被拒、不落檔（B1-3 integration 驗收）。
- 辨認不出格式的檔案不會進 `public/`；機敏檔的明文不出現在任何 R2 key（B1-2c、B1-3 驗收）。
- 暫存憑證可在表上撤銷與過期，不需要黑名單。

## Followup

- B1-3：量 website 反代路徑對機敏檔上限大小的實際行為，超過就停下回報；Connect 契約；client 輪詢 public 處理結果；R2 CORS 與 `tmp/` lifecycle（各環境由 Will 設定，B1-8 列出）。
- B1-2c：移除 B1-2b 為 secure 實作的預簽路徑，改成串流寫入。

## 撤銷條件

- 量到 website 反代撐不住機敏檔大小，而且沒有不經反代的安全做法：重新評估機敏檔的通道。
- R2 支援 POST policy 的大小範圍：public 可改綁範圍，省掉 client 先報大小。

## 相關

- 計畫 `backend/docs/plans/2026-11-21-b1-media.md`（`c8870685` 起），附件 `spike.md` (e)。
- `backend/internal/modules/media/CARRYOVER.md`：逐項沿用盤點。
