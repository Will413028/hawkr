# media 公開圖片：上傳後以 libvips 解碼並重新編碼，不做逐位元組 metadata 剝除

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-11-22（Will 決定） |
| 影響範圍 | `backend/internal/modules/media/internal/image`、`internal/upload/public_process.go`、backend CI 與本機建置（libvips） |
| 執行計畫 | `backend/docs/plans/2026-11-21-b1-media.md`（決定 B1-D13、步驟 B1-2c1、不變量第 1 條） |

## 脈絡

公開圖片寫進 `public/` 後由 image-gateway 提供；`original` preset 與轉換失敗的退回路徑（`backend/workers/image-gateway/src/index.ts`）會直接送出存著的位元組，所以存著的檔本身必須乾淨。

- inherited：Java `ImageMetadataStripper` 逐位元組剝除 metadata、不解碼，保留原編碼。第一版 Go 照搬，兩輪審查分別找到 JPEG 未知標記、JFIF 縮圖、WebP ANMF 子區塊、GIF 擴充子區塊的洩漏，修正後又冒出合法 GIF 被拒與 JPEG-LS 被剝壞。每種容器變體都要手工處理，這個做法沒有收斂點；照搬不是約束（根 `AGENTS.md`「沿用也是決策」）。
- inherited：沒有像素上限，只有位元組上限（10 MiB，Java `application.yml:117`）。
- external：iPhone 預設拍 HEIC、Display P3 色域。

## 考慮過的方案

業界基準：收使用者上傳的平台（Facebook／Instagram、X、Slack、Discord、WordPress）在上傳時解碼並重新編碼：先讀標頭檢查像素上限（防解壓縮炸彈），把方向套進像素，轉成 sRGB，寫出不含 metadata 的新檔；同時讓多格式偽裝檔與解析器攻擊載荷失效。常見實作是 libvips（imgproxy、imagor、sharp）。逐位元組剝除只用在必須保留原始位元組的產品（攝影典藏）。

| 方案 | 代價 | 判定 |
|---|---|---|
| A libvips 解碼後重新編碼 | cgo；CI、本機與日後的部署映像都要裝 libvips；一次有損重新編碼 | ✓ |
| B Go 標準庫解碼後重新編碼 | 無色彩管理（P3 照片偏灰）、無 WebP 編碼器、無動態 WebP、無 HEIC；實測 40 MP JPEG 約 2.5 秒、配置 89 MiB | ✗ |
| C 上傳時交給 Cloudflare Images | 上傳可用性綁外部服務；本機與 CI 只能用替身，真正的清理在 CI 測不到；每張計費 | ✗ |
| D 維持逐位元組剝除 | 每個未處理的容器變體都是洩漏或壞檔；無像素上限；擋不住偽裝檔 | ✗ |

## 決定

- 公開圖片在處理 job 中以 libvips 重新編碼，Go 綁定用 `github.com/cshum/vipsgen` v1.3.11（正式程式的 cgo 依賴，它自己沒有非測試的傳遞依賴；由 libvips introspection 生成，涵蓋動態圖與色彩轉換）。
- 只收 JPEG、PNG、WebP、GIF（含動態 GIF 與動態 WebP）；HEIC 由 client 先轉 JPEG，伺服器拒收；動態 PNG（APNG）拒收，libvips 只解得出第一格，不默默壓成靜態圖。
- 完整解碼前先檢查像素上限與動態圖的「影格數 × 每格像素」上限，與位元組上限定義在同一處。
- 輸出：方向已套進像素、sRGB、不帶任何 metadata；靜態不透明 → JPEG，有透明 → PNG，動態 → 原格式。
- CI 以 apt 裝 `libvips-dev`；本機 `brew install vips`。部署映像在單體有 Dockerfile 時加入 libvips。

## 後果

- `public/` 的不變量變成「只有重新編碼過的像素」，不再需要逐格式維護白名單。
- 有損重新編碼一次；image-gateway 的轉換以這份乾淨原圖為來源。client 拿 `original` 重新上傳會再壓一次，這點在 B1-4 切 client 時檢查。
- 整個 backend 的建置依賴 libvips（cgo）。
- 重新評估的觸發條件：產品需要保留原始位元組的原圖。
