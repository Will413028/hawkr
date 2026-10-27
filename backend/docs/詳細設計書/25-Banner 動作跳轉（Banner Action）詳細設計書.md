# Banner 動作跳轉（Banner Action）詳細設計書

> **單體（遷移計畫 A1，2026-09-26 起是現行實作）**：content-service 已刪除，現行實作在單體的 `internal/modules/content`。點擊行為不再是原樣透傳的 JSONB：對外契約是 proto 的 `oneof`（根目錄 `api/`），DB 是 `action_type`、`action_target` 兩欄加 `banners_action_check`，值域是封閉的四種，新增動作類型要同時改 proto、模組與 CHECK。url 只接受 http、https。本文件的「各層契約」是舊設計，以 content 模組的 `AGENTS.md`（不變量）與 `CARRYOVER.md`（沿用盤點）為準；動作的語意與 App 的分派規則仍以本文件為準。

## 概述

Buyer App 首頁 Banner 點擊後的跳轉行為，由 Admin 後台設定 `action` JSONB 欄位控制。支援開啟外部網頁、跳轉 App 內商品頁、跳轉 App 內貼文頁、顯示彈窗四種行為。

## Action JSON 格式規範

### 欄位定義

| 欄位 | 型別 | 說明 |
|------|------|------|
| `type` | string | 動作類型：`url` / `product` / `post` / `popup` |
| `url` | string | 當 `type=url` 時，外部網頁 URL |
| `id` | string | 當 `type=product` 或 `type=post` 時，對應資源 ID |

彈窗上的 CTA 按鈕文案由 banner 的**同層** `buttonText` 欄位提供（不在 `action` 物件內）：

| 欄位 | 型別 | 說明 |
|------|------|------|
| `buttonText` | string | CTA 按鈕自訂文案。空字串或未提供時，依 `action.type` 取預設值：`url`→「查看詳情」、`product`→「立即購買」、`post`→「查看貼文」 |

### 範例

```jsonc
// 開啟外部網頁
{ "type": "url", "url": "https://example.com/promo" }

// 跳轉 App 內商品頁
{ "type": "product", "id": "12345" }

// 跳轉 App 內貼文頁
{ "type": "post", "id": "67890" }

// 顯示彈窗（或 action 為 null / {}）
{ "type": "popup" }
```

### 預設行為

`action` 為 `null`、`{}`、或 `{"type": "popup"}` 時皆走彈窗邏輯，且不顯示 CTA 按鈕。
`type` 為未知值時同樣退回彈窗——App 端不得因為收到不認識的 `type` 而報錯或不顯示 banner。

🔴 **`action` 是語義化結構，不是路由字串**：`type` ＋ 對應欄位（`url` / `id`）由各端自行對應到平台路由。
**不得**讓 Admin 直接填 App 內部 route 或 deep link URI——那會把路由結構變成對外契約，之後每次改路由都會讓既有 banner 失效。

## 各層契約

### Backend（content-service）

- DB `banners.action`（JSONB）儲存動作定義。
- Proto `BannerItem.action` 為 string，內容是上述 JSON。
- Gateway **原樣透傳** `action`，不解析、不驗證語義。

⇒ 新增動作類型時 backend 不需改動；值域的實際約束在 Admin 的下拉選單與 App 的分派邏輯兩端。

### Admin Dashboard

`src/components/banner/BannerEditorClient.tsx` 提供：

- **動作類型**下拉選單：`無動作（彈窗）`、`外部連結`、`商品`、`貼文`
- 選「外部連結」→ 顯示 URL 輸入框
- 選「商品」→ 顯示商品 ID 輸入框
- 選「貼文」→ 顯示貼文 ID 輸入框
- 儲存時把選擇結果序列化為 `action` JSON

### Buyer App

`_showBannerDetail()` 讀 banner 的 `action` 與 `buttonText`，交給 `BannerAction.fromMap(actionMap, buttonText: buttonText)`，再由 `showBannerPopupDialog` 呈現。CTA 的分派：

| `action.type` | 行為 | CTA 出現條件 |
|---|---|---|
| `url` | 以外部瀏覽器開啟（`LaunchMode.externalApplication`） | `url` 非空 |
| `product` | 進商品頁 `/merchant-product`（帶 `skuId`） | `id` 非空 |
| `post` | 進貼文頁 `/social/post/{id}` | `id` 非空 |
| 其他／`popup`／`null` | 只顯示彈窗，不出現 CTA 按鈕 | — |

🔴 **`url` 類型必須檢查 scheme**：只有 `http` / `https` 可開啟。`action.url` 是 Admin 可填的任意字串，不檢查等於讓後台能對 App 觸發任意 scheme。

## 資料流

```
Admin 設定 action JSON
  → POST/PUT /api/v1/admin/banners  (action 欄位)
  → content-service 存入 DB (banners.action JSONB)
  → Buyer App GET /api/v1/public/banners
  → 解析 action.type → 對應跳轉行為
```
