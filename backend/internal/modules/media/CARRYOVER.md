# media 模組的沿用盤點

從 Java media-service 沿用的每個機制，搬進 Go 時重新判斷的結論（遷移計畫「模組完成的定義」）。計畫與範圍在 `backend/docs/plans/2026-11-21-b1-media.md`；R2 實測事實在 `backend/docs/plans/2026-11-21-b1-media/spike.md` (e)。本表由 B1-2a 建立，B1-2b、B1-2c 落地的機制（暫存 TTL 長度、狀態機、處理 job 等）隨步驟補列。

## 沿用盤點（B1-2a：儲存層）

Java `StorageService` 的操作與 Go port 的對應：`generatePresignedUploadUrl` → `PresignPut`；`getObjectStream` → `Get`；`statObject` → `Head`；`uploadStream` → `Put`；`deleteObject` → `Delete`；`generateObjectKey`、`generateFinalObjectKey` → `NewKey`；`copyObject`、`generatePresignedDownloadUrl`、`isEnabled` 不搬（見下表）。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 預簽 PUT 讓 client 直傳（`R2StorageService.java:126-148`） | 檔案不經應用伺服器 | 成立：圖片與機敏檔都由 client 直傳 | [R2 預簽網址](https://developers.cloudflare.com/r2/api/s3/presigned-urls/)是官方的直傳做法 | 沿用；簽章改綁 `Content-Length`、`Content-Type`、`Content-MD5`、`If-None-Match: *`（Java 只綁 `Content-Type`，`R2StorageService.java:41`，大小要靠事後 HEAD 兜底）。R2 不支援表單 POST，無法綁大小範圍，只能綁確切大小（spike (e)） |
| 暫存物件放 `tmp/` 前綴，確認前不進正式前綴 | 確認前的上傳不能被公開讀取，且過期要能整批清除 | 成立 | [R2 lifecycle](https://developers.cloudflare.com/r2/buckets/object-lifecycles/) 可依前綴過期 | 沿用，前綴固定為 `tmp/`、`public/`、`secure/`；Java 的暫存 key 是 `{visibility}/{yyyyMMdd}/{uuid}{ext}`（`AbstractStorageService`），暫存與正式混在同一前綴，lifecycle 無法只清暫存，不沿用 |
| 完整性用 `Content-MD5`，不用 `x-amz-checksum-sha256` | R2 PutObject 認得的完整性 header | R2 文件列 `Content-MD5`，`x-amz-checksum-*` 在 PutObject 是否支援沒寫（spike (e)） | [R2 S3 API](https://developers.cloudflare.com/r2/api/s3/api/) | 用 `Content-MD5`；SDK 的 checksum 計算設為 `WhenRequired`，預簽網址不多綁 `x-amz-checksum-*` |
| object key 含檔名副檔名與日期（`{uuid}{ext}`） | 讀取端靠副檔名判斷型別 | 不成立：辨認改依 magic bytes（B1-2c），型別存在資產列 | 以不可猜的隨機 key 加伺服器端記錄型別 | `NewKey` 只產 `<prefix>/<uuid v4>`，不含呼叫端檔名（缺陷 5.16）、不含 `.attempt-`（缺陷 5.15）；v4 隨機讓確認前的暫存 key 不可猜 |
| 未配置時走 mock／skip（`R2StorageService.java:121-131`，`isEnabled`） | 本機沒有 R2 時還能跑 | 不成立：本機與 CI 用 MinIO 容器（Will 2026-11-22） | 缺設定就啟動失敗（fail-fast） | 不搬 `isEnabled` 與假網址；`MEDIA_STORAGE=r2\|s3compat` 明確選擇，缺選擇或缺任一設定，建構時回錯誤、啟動失敗（缺陷 5.12），絕不退回 fake；`storage.Fake` 只由 Go 測試接線 |
| `copyObject`：確認時把暫存複製到正式路徑（`R2StorageService.java:241`） | 暫存與正式物件內容相同 | 不成立：處理 job 讀暫存、剝除 metadata 或加密後寫正式 key | 處理後的位元組不同於原檔，不能複製 | 不搬；port 沒有 Copy |
| `generatePresignedDownloadUrl`（`R2StorageService.java:151`） | 公開圖片由 client 直接下載 | 不成立：公開出口是 image gateway 與 CDN；機敏下載由擁有者端點代理 `Get`（ADR `2026-10-03` 修訂） | — | 不搬 |
| Java 用 AWS SDK v2、R2 region 固定 `auto`、endpoint `https://<account>.r2.cloudflarestorage.com`（`R2StorageService.java:90-101`） | 與 R2 的 SigV4 相容 | 成立 | [R2 S3 相容](https://developers.cloudflare.com/r2/api/s3/api/) 官方用 AWS SDK | 沿用到 aws-sdk-go-v2（`aws` 與 `service/s3` 兩個直接依賴，Will 2026-11-22 同意）；憑證用 `aws.CredentialsProviderFunc`，不加 `credentials`、`config`。`s3compat` 帶自訂 endpoint 與 path-style，同一個 adapter |
| `Put` 同 key 重寫 | 處理 job 重試要冪等 | 成立 | S3 PutObject 預設覆寫；預簽 PUT 才帶 `If-None-Match: *` | `Put` 覆寫；`If-None-Match` 只綁在預簽 PUT，避免 client 覆蓋已存在的暫存物件 |
