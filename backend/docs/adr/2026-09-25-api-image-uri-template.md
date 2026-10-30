# 對外 API 的圖片：伺服器組好 URI 樣板，client 只選 preset

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-25 |
| 影響範圍 | `api/`（新增 `hawkr.common.v1alpha1.Image`；content 前後台的圖片欄位）、單體 `internal/httpapi`、四個 client 的圖片 helper、image gateway 的 preset 清單；〈API 設計規範（Connect）〉§四、〈多媒體與圖片 URL 規範〉§2、§4 |
| 上層決策 | `2026-09-18-media-storage-r2-image-gateway.md`（本檔取代其 ADR-08）；`2026-09-24-go-modular-monolith.md` D9 |

## 脈絡

Connect 草稿 §四 把 ADR-08 的「扁平成對 `xxxKey`＋`xxxUrl`、在 gateway 組」改成「只回 key，client 以 `buildImageUrl(key, preset)` 組」，沒有寫 ADR，依據只有「四個 client 都這樣組」。結果是媒體網域與 `/i/{preset}/{key}` 的路徑形狀成了沒有工具檢查的隱性契約：

- 兩支 App 的 `lib/config/flavor.dart` 把 `https://media.example.com`（舊公司的網域）寫死在程式碼，`media_image.dart` 另外寫死一組網域，用來從 URL 反推 key；website 有 `PROD_MEDIA_ORIGIN` 與建置時的斷言。
- 四份 helper 已經不一致：admin 與 buyer-app 的 preset 清單缺 `motion`。
- 這個專案付過一次同樣的代價：OSS 換 R2 時，媒體的 host 與 URL 形狀一起換，破圖分散在 8 套實作（media ADR 的脈絡）。

約束：

- 外部：舊公司的網域不能再用（D9，`scripts/check-legacy-endpoints.py`），媒體網域一定要換；新網域在 CD 第 4、5 步決定。
- 外部（上線後）：已安裝的 App 不會同步更新，寫進 App 的東西要改就得發版、再強制更新。
- 外部：要顯示多大的圖只有 client 知道（版面、DPR）。
- 繼承：DB 與事件只存 object key（規範 §1）。仍成立：換網域、換 URL 形狀不必遷移資料。
- 繼承：image gateway 是公開圖片唯一的出口，preset 白名單在 `backend/workers/image-gateway/src/presets.ts`，未知的 preset 回 400（ADR-01）。仍成立：計費與防濫用都需要尺寸的集合有界。
- 繼承：寫入只收 staged token，不收 URL 或 key（ADR-08 第 3 點）。仍成立：這是授權問題，與協定無關。
- 拿掉：ADR-08 否決巢狀訊息的兩個理由（與已上線的欄位一致、regen 成本）。系統沒有上線，proto 由 buf 生成。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| **基準**：伺服器回可用的 URL 或 URL 樣板，client 不知道圖片的 host，只決定尺寸。Twitch Helix 的 `thumbnail_url` 帶寬高的佔位符；Spotify 的 `images` 是多個 `{url, width, height}`；Shopify Storefront 的 `Image.url(transform:)`；Cloudflare Images 的具名 variant URL | 見下列各種形式 | 採用，形式是 B |
| A. 只回 key，client 用建置時的 host 組（現行草稿） | host 與路徑形狀進了 App 的 binary，換網域要四端改版、上線後還要強制更新；外部圖片要另開例外；組裝邏輯四份、兩種語言各自手寫 | 否決 |
| **B. `Image { uri_template }`**：伺服器組 `https://<媒體網域>/i/{preset}/<key>`，client 只把 `{preset}` 換成 preset 名稱 | 每張圖的回應多一百多 bytes；回應內容隨部署設定而不同；樣板的格式要用測試固定 | **採用** |
| C. 回原圖 URL，client 在後面附加 `preset`（與有上限的寬度）參數 | image gateway 要改成以 query 參數選 preset；client 仍要懂參數的語法；原圖成為預設的 URL | 否決，見理由 |
| D. 每張圖回所有 preset 的 URL（Spotify 的做法） | 8 個 preset 就是每張圖 8 個 URL；多一個 preset 所有回應都變 | 否決 |
| E. 請求帶 preset，伺服器回那個尺寸的 URL（Shopify 以 GraphQL 的欄位參數做到） | RPC 沒有欄位層級的參數；同一個畫面常要同一張圖的兩個尺寸 | 否決 |
| F. 只回 key，媒體網域由一支設定 RPC 下發 | 畫面與 SSR 都要先拿到設定才能畫圖；外部圖片仍要例外 | 否決 |
| G. ADR-08 的扁平 key＋原圖 URL | 原圖不能直接拿來顯示，client 還是要用 key 自己組 | 否決 |

## 決策

- **D1 共用訊息**

  ```proto
  // hawkr/common/v1alpha1/image.proto
  message Image {
    // 取圖的 URI 樣板（RFC 6570 Level 1）：恰好一個變數 {preset}，或沒有變數（外部圖片）。
    // client 把 {preset} 換成 preset 名稱後直接使用，不解析、不改寫其他部分。
    string uri_template = 1;
  }
  ```

  - 欄位用語意名（`preview_image`、`image`），沒有圖時不設（message presence）；多張圖用 `repeated Image`。欄位名依 AIP-140 用 `uri`。
  - `Image` 只出現在回應，object key 不在對外契約裡。之後有「多張圖要指定沿用哪幾張」的資源搬進單體時，再加一個不透明的識別欄位。
- **D2 組裝點**：單體的 `internal/httpapi` 是唯一組 URI 的地方，以伺服器設定 `MEDIA_ORIGIN` 組成（啟動時檢查：https、不帶路徑）。模組照舊只處理 key。外部圖片（第三方頭像）組成沒有變數的樣板。
- **D3 client**：helper 只剩一件事，把 `{preset}` 換成 preset 名稱。四端拿掉媒體網域的設定、從 URL 反推 key、host 白名單，以及「http 開頭就是外部圖片」的判斷。preset 名稱都是小寫字母（RFC 6570 的 unreserved 字元），直接做字串替換即可；client 不在 helper 以外碰 `uri_template`。
- **D4 preset 清單**：`presets.ts` 是唯一來源，四端的 preset 常數由它生成，CI 檢查 drift，取代現在四份手寫、已經不一致的清單。新增 preset 的順序照規範 §3.4（worker 先上）；上線後 preset 只增不刪，舊版 App 還會送舊的名稱。
- **D5 寫入不變**：新圖用 `*_staged_token`；沿用目前的圖時，不把那個欄位列進 `update_mask`。
- **D6 其他通道**：DB 與事件只存 key；回給 client 的串流也用 `Image`，由傳輸層組；推播由組推播的 worker 組完整的 URL；機敏檔（`secure/`）不用 `Image`，照舊走代理下載。

## 理由

- **核心取捨是 host 與路徑形狀放在伺服器還是 client。** ADR-01 已經把轉換語法收回 gateway；本檔把剩下的 host 與路徑也收回伺服器，client 只保留它唯一該決定的事：這個畫面要哪個 preset。代價是每張圖多一段文字、回應隨部署設定而不同。
- **選樣板（B）而不是原圖加參數（C）**：
  - gateway 的 preset 本來就在路徑上（`/i/{preset}/`），B 不必改 gateway；C 要改路由與快取鍵。
  - C 讓原圖成為預設的 URL，而原圖直出儲存的位元組，暴露面比 preset 大（寫入 `public/` 時已剝除 EXIF，見 49 號設計書 §8.1，2026-10-31）。
  - C 的論點「樣板本身不是可用的 URL，忘了替換就破圖」，由 D3 的單一 helper 與測試擋住。
  - C 另外提議開放有上限的寬度階梯，讓 web 恢復 srcset。這是 image gateway 的決定（修訂 ADR-01、影響 Cloudflare Images 的計費），與 API 怎麼表示圖片無關，另案處理。
- **不回 key**：現在沒有讀取端（content 的圖都是單張，沿用靠 `update_mask`）。依「不做沒有讀取端的功能」，等需要時再加不透明的識別欄位。
- **不帶寬高**：目前沒有讀取端，media 也沒有記錄圖片尺寸；需要時在 `Image` 加欄位，不影響既有欄位。
- **用訊息，而不是 `string` 欄位**：全平台一個型別、一支 helper，之後加欄位不必在每個資源旁加一排 `*_width`。

## 後果

- **接受的代價**
  - `MEDIA_ORIGIN` 設錯時，四端同時全部破圖（A 是各自設錯各自壞）。backend CD 要對設定的 origin 探測：未知的 preset 回 400、已知的 preset 配不存在的 key 回 404。
  - App 連 API 的 host 仍寫在 binary（`gatewayHost`）：整個網域失去時，App 一樣要改版。本檔保護的是「媒體網域或路徑形狀單獨改變」這種情況。
  - 換媒體網域時，舊網域要保留到 website 的資料快取與 App 保存的舊回應都過期。
- **要改的地方**
  - proto：`common/v1alpha1/image.proto`；content 前後台的 `preview_image_key`、`popup_image_key`、`image_key` 改成 `Image`。
  - `internal/httpapi/content` 的轉換；`cmd/hawkr` 的 `MEDIA_ORIGIN` 設定。
  - 四端的 helper（試點步驟 6）；preset 常數的生成與 drift CI。
  - 〈API 設計規範（Connect）〉§四、〈多媒體與圖片 URL 規範〉§2、§4、49 號設計書 §6；content 模組 `CARRYOVER.md` 傳輸層沿用盤點的圖片列。
  - D8 特性測試的圖片比對：REST driver 回 key、Connect driver 回樣板，在 driver 內正規化成同一種值。
  - media ADR 的 ADR-08 標 `superseded by 2026-09-25-api-image-uri-template.md`。
- **不在本檔，另列待辦**：website 的 `next/image` loader 忽略寬度、一律回 `original`，90 處 `AppImage` 有 61 處沒傳 preset 而下載原圖。
- **重新評估的條件**：圖片服務需要每個尺寸各自簽章或短效 token；web 需要 srcset 而 preset 的固定寬度不夠；出現 monorepo 以外的 client。
- 試點不通過、回到 OpenAPI 時，同樣形狀的 JSON schema 照樣適用。

## 來源

- 本 ADR 是與 Will 討論後的決定（2026-09-25），另以兩份獨立草稿對照；無其他外部來源文件。
- Twitch Helix（`thumbnail_url` 的尺寸佔位符）：https://dev.twitch.tv/docs/api/videos/
- Spotify Web API（`images` 的多個尺寸）：https://developer.spotify.com/documentation/web-api/reference/get-an-album
- Shopify Storefront `Image.url(transform:)`：https://shopify.dev/docs/api/storefront/latest/objects/Image
- Cloudflare Images 的 variant URL：https://developers.cloudflare.com/images/manage-images/serve-images/serve-uploaded-images/
- AIP-140（URI 欄位的命名）：https://google.aip.dev/140 ；RFC 6570：https://www.rfc-editor.org/rfc/rfc6570
