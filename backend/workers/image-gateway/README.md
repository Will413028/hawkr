# image-gateway

全平台圖片唯一公開出口（Cloudflare Worker）。對應
[49 號設計書：多媒體儲存與圖片服務（Cloudflare R2 & Image Gateway）](../../docs/詳細設計書/49-多媒體儲存與圖片服務（Cloudflare%20R2%20&%20Image%20Gateway）詳細設計書.md)。

## 職責

- `GET/HEAD /i/{preset}/{objectKey}`：preset 白名單查核 → R2 binding 取原檔 → Images
  binding 轉換 → 回應（`Cache-Control: public, max-age=31536000, immutable`）。
- `GET/HEAD /i/original/{objectKey}`：原檔直出，不轉換。
- 只放行 `objectKey` 前綴為 `public/` 的物件；`secure/`、`tmp/` 一律 404（fail-closed，
  這兩類不可經公開網域讀取）。
- preset 白名單是**全平台唯一定義點**（`src/presets.ts`）——前端只能傳表內 preset 名，
  不接受任意 width/height/quality/format 參數。未知 preset 一律 400。
- Images binding 轉換失敗（來源不可解碼、binding 回空輸出）時 degrade 直出原檔，不破圖。

不做：驗權、R2→OSS fallback、任意尺寸參數。詳見設計書 §2.2 邊界表。

## URL 契約

| 請求 | 回應 |
|---|---|
| `GET /i/{preset}/{objectKey}` | preset 見下表；200 轉換後圖 |
| `GET /i/original/{objectKey}` | 200 原檔（Content-Type 取自 R2 metadata） |
| preset 不在白名單 | 400（純文字，非業務 API，不走 Envelope） |
| objectKey 不存在 / 前綴非 `public/` / 其他路徑 / 非 GET·HEAD | 404 |

## preset 白名單

| preset | 轉換 | 場景 |
|---|---|---|
| `avatar` | 160×160 cover q60 | 買家/賣家/聊天頭像 |
| `thumb` | 240×240 內 cover q70 | 商品縮圖/購物車/訂單項/聊天商品卡 |
| `card` | 720×540 cover q75 | 商品卡/貼文卡/首頁卡 |
| `detail` | 1280 長邊 scale-down q80 | 商品詳情/貼文詳情 |
| `banner` | 1600 長邊 scale-down q85 | 首頁/商家 banner、賣家封面 |
| `chat` | 720 長邊 scale-down q75 | 聊天圖片訊息 |
| `original` | 不轉換 | 全螢幕檢視、畫質敏感圖、PDF 等非圖片檔 |

`format` 一律 `format=auto` 語意：Worker 依請求 `Accept` header 自行協商
avif → webp → jpeg（`ImageOutputOptions.format` 型別不接受 `"auto"` 字面值，故不是直傳
binding，見 `src/index.ts` 的 `negotiateFormat` 註解）。

### 新增/調整 preset

1. 改 `src/presets.ts` 的 `PRESETS` 一處。
2. 同步本檔與 49 號設計書 §4.4 表格。
3. 補 `test/presets.test.ts` 對應斷言。
4. 部署 Worker（見下方部署）——**前端不需改版**，preset 定義只存在這裡。

## 本地開發

```bash
npm install
npm run dev          # wrangler dev，本地跑（無 R2/Images 遠端連線時走 miniflare 模擬）
npm run typecheck    # tsc --noEmit（會先跑 wrangler types 產生 worker-configuration.d.ts）
npm test             # vitest run，跑在 workerd runtime（@cloudflare/vitest-pool-workers）
```

`worker-configuration.d.ts` 是 `wrangler types` 產生的型別檔（依 `wrangler.toml` 的
頂層 binding 生成），已 gitignore，`pretest`/`pretypecheck` 會自動重新產生，不需手動
維護。

## 部署

目前沒有部署環境：公司時期的 dev / production 環境與 custom domain 路由已刪除，
`wrangler.toml` 只留本機開發與測試需要的 binding。選定 demo 部署目標時，再加環境、
bucket 名稱與路由。
