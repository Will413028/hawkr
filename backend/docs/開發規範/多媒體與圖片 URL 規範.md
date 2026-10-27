# 多媒體與圖片 URL 規範

> 依據：49 號詳細設計書（多媒體儲存與圖片服務）。本檔為全平台強制規範，適用所有後端服務（Java/Go）、Gateway、前端（web/admin/buyer App/seller App）。
> 動機：2026-07 盤點實錘——9+ 個 DB 欄位寫入時把完整 URL 烤進庫、同欄位混存多達 5 種值型態（CDN URL／objectKey／過期 presigned／JWT／外部 URL）、URL 組裝分裂為 8 套實作。本規範防止修復後偏差重生。

## 1. 儲存層（DB / MQ / 快取）

1. **圖片與檔案欄位一律只存 objectKey**（格式見 08 號 §2.2）。**禁止**將完整 URL、presigned URL、JWT/staged token 寫入任何 DB 欄位、MQ payload、快取值。
2. **欄位命名必須誠實**：存 objectKey 的欄位名用 `*_key`／`*_keys`（DB）、`*Key`／`*Keys`（JSON/proto）。禁止 `*_url` 名存 key 值（歷史教訓：`banners.preview_image_url`）。語意中性名（如 `image`、`content`）允許，但註解必須標明存 key。
3. **例外（唯二）**：
   - 第三方外部圖（Google OAuth 頭像等）——本來就不是我方資產，存外部 URL；讀取端以「http 開頭=外部 URL 直用、否則=objectKey 組 URL」判別。
   - 機敏檔（`secure/` 前綴）——DB 不存任何 URL（08 號機制：app 端讀取走 `secure_download`，只有上傳者本人；後台沒有通用的機敏檔下載，由擁有者端點各自授權）。
4. MQ event 圖片欄位帶 objectKey，欄位名 `*Key` 語意；消費端自行組 URL，禁止透傳上游組好的 URL 落庫。

## 2. URL 組裝（唯一許可位置）

| 端 | 唯一入口 | 禁止 |
|---|---|---|
| Java | `common-core` `MediaUrlProperties.buildUrl(objectKey)` | 自行拼 `MEDIA_BASE_URL`／硬編碼域名 |
| Go（單體） | `internal/httpapi` 組 `Image.uri_template`（`{MEDIA_ORIGIN}/i/{preset}/{key}`） | 模組或其他地方組 URL |
| Go（舊服務、gateway） | `pkg/mediaurl.BuildURL(baseUrl, objectKey)`，只到該模組搬進單體為止 | ad-hoc 字串拼接（歷史違規：review/social 各自拼、social 用獨立 `OSS_CUSTOM_DOMAIN`） |
| 前端（四端） | 把 `Image.uri_template` 的 `{preset}` 換成 preset 名稱（web/admin 共用 TS 模組；兩 App 共用 Dart） | 持有媒體網域；從 URL 反推 key；在樣板以外拼 URL 或加參數 |

- 對外 API 的圖片一律由**單體的傳輸層**組成 `Image.uri_template`，client 只選 preset（ADR `2026-09-25-api-image-uri-template.md`）。模組只處理 key。
- 媒體網域只在單體的設定 `MEDIA_ORIGIN`（與 image gateway 的 route）；client 不持有媒體網域，**禁止**寫死於程式碼常數。
- 還沒搬進單體的模組，其 REST 回應在搬遷前維持 Gateway 組的 `xxxUrl`（49 號 §6.1／§6.6／§6.7）。其他通道：MQ payload 帶 key、消費端組；回給 client 的串流用 `Image`，由傳輸層組；FCM 由 notification worker 組。

## 3. 圖片處理參數（preset 制）

1. 前端**禁止**傳 width／height／quality／format 等自由參數——顯示尺寸需求一律以 preset 名表達（白名單見 49 號 §4.4；SSOT=`workers/image-gateway/src/presets.ts`）。
2. Cloudflare 轉換語法（Images binding 參數、`cf.image` options）**只准出現在 `workers/image-gateway/`**；後端服務與前端 repo 出現即違規。
3. 新增／調整 preset 流程：改 `presets.ts` ＋ 49 號 §4.4 表同步 → Worker CI 部署。前端不需改版。Image Gateway 對未知 preset 回 400（deny-by-default），繞不過去。
4. 🔴 **新增 preset 的部署順序固定：backend 先上 prod worker → 實測 → 前端才 promote。** preset 白名單在 backend、送出名字的是前端，兩者**各自部署**；前端先上而 worker 沒跟上，該 preset 的每一張圖回 **400**，使用者看到整片破圖，而前端 code 完全正確（現象看起來像前端壞了）。2026-08-28 新增 `motion` 時實測過這個窗口：`test-media` 已認得（404＝檔案不存在）、`media` 尚未（400）。
5. **判斷某個環境的 gateway 認不認得某個 preset，用「不存在的 key」探測**，不要綁真實物件（物件會被刪，綁了必腐化）：

   ```
   未知 preset             → 400
   已知 preset + 檔案不存在 → 404
   ```

   `curl -o /dev/null -w '%{http_code}' "$MEDIA_BASE_URL/i/<preset>/public/__probe-does-not-exist__.gif"`
   hawkr-website 的部署前閘門即以此實作（`deploy.yml` 的 `Validate deploy configuration`：把 `ImagePreset` union 解出來逐一探測部署目標），新增 preset 自動納入。
6. **`animated: true` 的 preset（動圖）本機驗不到，必須對已部署的 worker 實測。** Miniflare Images 模擬器對 `image/gif` 回 `IMAGES_TRANSFORM_ERROR 9520: GIF output is not supported in local mode`（於是 degrade 直出原檔，看起來「是動的」其實沒轉換）、對 `image/webp` 成功但靜默吐單幀。又因轉換失敗時 Worker 一律 degrade 直出原檔且 **HTTP 仍是 200**，只看狀態碼會把完全沒生效當成成功 ⇒ 用 `workers/image-gateway/scripts/verify-motion-preset.mjs` 驗**位元組數與幀數**，並以「位元組數與原檔相同」當作沒生效的指紋。

## 4. API 契約

1. 對外回應的圖片用共用訊息 `hawkr.common.<版本>.Image { uri_template }`（ADR `2026-09-25-api-image-uri-template.md`，取代 49 號 ADR D8 的扁平成對）。欄位用語意名（`preview_image`），沒有圖時不設，多張用 `repeated Image`。還沒搬進單體的模組在搬遷前維持扁平成對的 `xxxKey`＋`xxxUrl`。
2. object key 不在對外契約。前端可控輸入**只收 staged token**：新圖以 `<語意>_staged_token` 傳入，沿用目前的圖時不把該欄位列進 `update_mask`；禁止收完整 URL 或 key 回存（歷史違規：review `item_image_url` 前端直送落庫）。
3. 特例：第三方外部圖（Google 頭像）組成沒有變數的樣板，client 照樣使用；機敏檔（`secure/`）不用 `Image`，走擁有者授權的下載端點（app 端是 secure_download）。
4. preset 清單的唯一來源是 `workers/image-gateway/src/presets.ts`，四端的 preset 常數由它生成，CI 檢查 drift。

## 5. 上傳與儲存後端

1. 儲存操作只准經 media-service／product-import-service 的 `StorageService` 層（R2 S3 API）；其他服務禁止直接依賴 storage SDK。
2. 上傳驗證最低要求：MIME 白名單、size 上限、finalize HEAD 回查實際 size/Content-Type、檔名消毒（拒 `/`、`..`）——見 49 號 §8.1。
3. R2 憑證一律 secret 管理（k8s secret `hawkr-r2`／`.env` gitignored）；**禁止**寫入 application.yml fallback 預設值（歷史違規：OSS AK/SK 明文入 git）。
4. R2 bucket browser CORS policy 是切換必要的 out-of-band 設定（不隨部署走），SSOT＝`infra/r2-cors/`；bucket 重建／遷移後必套用，遺漏＝presigned PUT 瀏覽器 preflight 403（見 49 號 §8.5）。

## 6. Code Review 檢查點（速查）

- [ ] 新欄位存圖？→ 名稱 `*_key` 語意＋值為 objectKey
- [ ] 對外回應有圖？→ 單體的傳輸層組 `Image.uri_template`；模組只回 key
- [ ] 回應圖片欄位？→ `Image`（語意名，沒有圖不設）；舊服務的 REST 在搬遷前維持 `xxxKey`＋`xxxUrl`
- [ ] 有新顯示尺寸需求？→ preset 新增流程，不是加參數
- [ ] MQ payload 有圖？→ objectKey ＋ `*Key` 欄位名（消費端組 URL）
- [ ] 串流推圖給 client？→ 用 `Image`，由傳輸層組
- [ ] 前端送出圖片？→ 只送 staged token；沿用舊圖不帶
