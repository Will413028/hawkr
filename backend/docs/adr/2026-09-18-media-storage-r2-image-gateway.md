# 多媒體儲存與圖片出口：R2 ＋ Image Gateway Worker

| 項目 | 值 |
|---|---|
| 狀態 | accepted（ADR-08 superseded by `2026-09-25-api-image-uri-template.md`） |
| 日期 | 2026-07-13（D8 於 2026-07-20 追加） |
| 影響範圍 | `services/java/media-service`、`services/java/product-import-service`、`workers/image-gateway`、`pkg/mediaurl`、全平台圖片欄位與四個前端 repo |
| 對應規格 | `docs/詳細設計書/49-多媒體儲存與圖片服務（Cloudflare R2 & Image Gateway）詳細設計書.md` |

> 本檔保存 `WHY THIS DESIGN` / `WHY NOT THE ALTERNATIVES`。
> preset 白名單、objectKey 規則、逐服務欄位落地清單、特例處置一律在 49 號詳細設計書，本檔不複製。

---

## 脈絡

儲存層原本深度耦合阿里雲 OSS，而耦合不只在 SDK 那一層：

- 圖片壓縮 100% 依賴 `x-oss-process` 專屬語法，且散佈在 web、買家 App、賣家 App 各自的產生點。
- URL 組裝分裂成後端數套機制 ＋ 四個前端 repo 各一套。
- 多個 DB 欄位在寫入時把**完整 URL 烤進庫**，同一個欄位混存 CDN URL、objectKey、
  過期 presigned URL、JWT 字串等多種型態。

其後果是：任何一次域名或轉換語法的變動都會全平台破圖，而破圖點分散在十幾個地方。
根治的方向因此不是「換一家儲存」，而是**讓 objectKey 成為唯一真實來源**，
URL 只在對外那一刻由單點組出來。

---

## ADR-01 圖片出口形態：Worker Gateway `/i/{preset}/{key}`

**不採用**：① 前端直組 `/cdn-cgi/image/` ② 延後不做 ③ 不做任何轉換。

**決策**：`media.example.com` 上一支 Cloudflare Worker 作為唯一公開圖片出口。

- preset 成為單一 SSOT，四個前端不再各持一份副本。
- 調整轉換參數不需要 App 發版。
- 白名單在服務端強制（deny-by-default），Cloudflare 的轉換語法零外露。

方案 ① 省掉一個部署單元，但上面三個病根會換一種形式全部回歸。
方案 ③ 的行動端流量不可接受。

## ADR-02 媒體域名：新開 `media.example.com`

**不採用**：沿用 `cdn.example.com`。

`media.` 語意正確（這是圖片服務而不是純 CDN），CF 配置可以從零建立而不必繼承舊設定，
也與 media-service 呼應。`cdn.` 隨 OSS 一起退役。

## ADR-03 Worker 內的轉換機制：Images binding

**不採用**：`fetch(cf: {image})` —— 它需要一個**公開的 origin**。

Images binding 直接 transform R2 串流，bucket 因此可以維持全私有。
若 binding 的參數面不足以覆蓋 preset 需求，備援方案是把 R2 綁到內部 origin hostname
再走 `cf.image` fetch，bucket 仍然不對外宣傳。

⚠️ 這個機制有兩個 runtime 行為直接影響 Worker 實作，規格寫在 49 號 §4.4 與 §7.3：
輸出格式沒有 `auto` 字面值（要自己協商），以及轉換失敗可能靜默回 200 ＋ 0 bytes。

## ADR-04 BD 可配置的 TransformSet：留擴充位，現在不做

preset 定義集中在 `presets.ts` 單點，未來要做成 BD 可配置，只需改成讀 KV／DB 白名單。
現在不預建資料表與後台。

## ADR-05 存量死圖：清理，不保留

**不採用**：保留。

那些連結已經死了（過期 presigned、JWT 字串、已被 GC 的 `tmp/` 物件），
保留只是把破圖原封不動帶進新架構。清理時產出 undo SQL 留檔。

## ADR-06 訂單評論的凍結快照：一次性改寫成 objectKey

**不採用**：維持 URL。

「評論快照凍結」約束的是**業務寫入**，不是 schema 遷移。

## ADR-07 遷移工具：一次性 Go CLI

**不採用**：rclone／Node／Java batch。

manifest、referenced-only 對帳、DB 值修正都需要與 DB 整合，rclone 涵蓋不了。
OSS 支援 S3 相容 API，所以單一 aws-sdk-go-v2 可以同時打兩端。
工具放 `tools/`，一次性，不進服務 codebase。

## ADR-08 對外圖片契約：扁平成對 `xxxKey` ＋ `xxxUrl`，全平台一次收斂，組裝點在 Gateway

> **superseded by `2026-09-25-api-image-uri-template.md`**（2026-09-25）：對外 API 改回伺服器組好的 `Image.uri_template`，client 只選 preset，key 不對外。本節保留為當時的紀錄；ADR-01～07 不受影響。

**不採用**：① 巢狀 `MediaRef{object_key, cdn_url}` ② 只收斂 seller ③ 在各業務服務內組 URL
④ 留相容雙軌並存欄位。

1. **扁平成對**：與已落地的 order／content 扁平雙欄一致（改巢狀等於重做已上線欄位），
   前端四端的 `buildImageUrl(key)` helper 本來就對扁平友善，也省掉新增巢狀 message 的 regen 成本。
2. **全平台一次收斂**：同一個 root cause 全修，
   以 proto 分批切開破壞面。
3. **組裝點收斂到 Gateway**：業務服務側一律不呼叫 `BuildURL`、只回 `xxx_key`，
   objectKey 成為單一 SSOT。這同時滿足安全契約——host 由後端保證，key 不可由 client 指定。

---

## 後果

- `x-oss-process` 這條路徹底封死：轉換語法只准出現在 `workers/image-gateway/`。
- 前端失去傳 width／height／quality／format 的能力，新尺寸需求一律走 preset 新增流程。
- Worker 是所有公開圖片的單點：它壞掉等於全平台圖片壞掉（degrade 行為見 49 號 §7.3）。
- 組裝點收斂到 Gateway 之後，**不經 Gateway 的通道**（MQ、WS/SSE、FCM）各自需要明定組裝點，
  那些例外逐條列在 49 號 §6.7。
