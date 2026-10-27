# AMEGO（光貿）電子發票 API 官方文檔（統整版）

> **整合來源**：光貿 AMEGO 開發專區官方 API 文件（`https://invoice.amego.tw/api_doc/` 的 apiDoc bundle）＋ 15 份官方補充說明頁
> **最後更新**：2026-08-24
> **涵蓋範圍**：開立／作廢／註銷發票、折讓與作廢折讓、查詢與狀態、載具與捐贈碼、字軌配號、錯誤碼全表、時間窗與限制
> **我方整合設計**：見 `../詳細設計書/58-電子發票服務（Invoice Service）詳細設計書.md` §4.5（本檔只寫光貿的行為，不寫我方的處置決策）

> **標註規則（全文一致）**
> - 【文件】＝官方原文逐字抄錄，未改寫。
> - 【實測】＝對 `https://invoice-api.amego.tw` 實際打過，附回應原文。
> - 【推定】＝文件未載明、由證據推論，**實作前必須確認**。
>
> **先讀**：§1.10 一頁速查 → §5（文件未載明清單）→ §6（文件互相矛盾清單）。
> 這兩節決定了哪些東西不能照文件寫、哪些東西必須先問光貿。

---

## 0. 總覽

### 0.1 這份 API 文件的形態（決定你怎麼重抓）

- `https://invoice.amego.tw/api_doc/` 是 **apiDoc 單頁 SPA**，HTML 只有樣板殼與 `Loading...`，**沒有任何 API 內容**。
- 真正的 API 定義（24 筆 `@api` 區塊 ＋ 專案 header）內嵌在
  `https://invoice-static.amego.tw/www/api_doc/assets/main.bundle.js?2026061001`（562,079 bytes，md5 `195b40448cc2ddc19b96454a8e2e5f70`）。
- 從 bundle 抽出的 `api_data.json`（234 KB）即為完整定義，**本檔所有參數表以它為準**。
- **不存在** `api_data.js` / `api_project.js`（apiDoc 慣例檔）——實測回 200 但內容是網站首頁（soft-404，23,879 bytes）。
  ⇒ **判斷是否抓到真頁**：size ≠ 23879 且 HTML 含 `api_doc/assets/main.bundle`。
- 側欄的每一條都是**同頁錨點** `#api-{group}-{name}`，**不是獨立 URL**，curl 無法個別驗證。
- 重抓方法：
  ```bash
  curl -sSL https://invoice-static.amego.tw/www/api_doc/assets/main.bundle.js -o main.bundle.js
  # 用錨點字串 {type:"post",url:"/json/  與 sampleUrl:"https://invoice-api.amego.tw" 定位陣列，
  # 做括號配對切出來，包成 module 用 node 轉 JSON -> api_data.json
  ```
  ⚠️ bundle 內的變數名（`let Ht` / `const ut`）是壓縮後的隨機名，改版會變，**不要拿它當錨點**。

### 0.2 端點總表（23 個唯一端點）

原始資料 24 筆，其中 `/json/g0401` 出現兩次（來源檔 `d0401.php` 與 `d0401_custom.php`，兩份定義有差異 → §6-2），
渲染後只顯示一條 ⇒ **唯一端點數 23**。另 `project.order` 陣列列了一個 `Invoice_Void`，但**資料中沒有對應端點定義**（排序殘留）。

| 群組 | 名稱 | 方法與路徑 | 舊路徑 | `data` 形狀 | 一句話 |
|---|---|---|---|---|---|
| 發票 | 開立發票(自動配號) | `POST /json/f0401` | `/json/c0401` | **物件** | 我方送明細，AMEGO 回發票號碼／時間／隨機碼 |
| 發票 | 作廢發票 | `POST /json/f0501` | `/json/c0501` | **陣列** | 作廢已開立的發票 |
| 發票 | 發票狀態 | `POST /json/invoice_status` | — | **陣列** | 發票的上傳狀態（有隱藏時間窗，見 §4.6） |
| 發票 | 發票查詢 | `POST /json/invoice_query` | — | 物件 | 查詢發票內容（180 天內） |
| 發票 | 發票列表 | `POST /json/invoice_list` | — | 物件 | 發票主檔分頁列表（無日期限制） |
| 發票 | 發票檔案 | `POST /json/invoice_file` | — | 物件 | 取 PDF 連結（10 分鐘有效） |
| 發票 | 發票列印 | `POST /json/invoice_print` | — | 物件 | 產出熱感應機列印字串 |
| 折讓 | 開立折讓 | `POST /json/g0401` | `/json/d0401` | **陣列** | 折讓已開立的發票 |
| 折讓 | 作廢折讓 | `POST /json/g0501` | `/json/d0501` | **陣列** | 作廢已開立的折讓單 |
| 折讓 | 折讓狀態 | `POST /json/allowance_status` | — | **陣列** | 折讓單上傳狀態（同樣有隱藏時間窗） |
| 折讓 | 折讓查詢 | `POST /json/allowance_query` | — | 物件 | 查詢折讓內容（實測無期限） |
| 折讓 | 折讓列表 | `POST /json/allowance_list` | — | 物件 | 折讓主檔分頁列表 |
| 折讓 | 折讓檔案 | `POST /json/allowance_file` | — | 物件 | 取折讓 PDF（可無限次下載） |
| 折讓 | 折讓列印 | `POST /json/allowance_print` | — | 物件 | 折讓熱感應機列印字串 |
| 中獎 | 獎項定義 | `POST /json/lottery_type` | — | **空字串** | 獎項代碼對照（簽章要用空字串，見 §1.3.5） |
| 中獎 | 中獎發票 | `POST /json/lottery_status` | — | 物件 | 該期中獎發票；「建議雙月1號才查詢」 |
| 其他 | 手機條碼查詢 | `POST /json/barcode` | — | 物件 | 驗手機條碼（唯一的載具驗證端點） |
| 其他 | 公司名稱查詢 | `POST /json/ban_query` | — | **陣列** | 統編 → 公司名（可批次） |
| 其他 | 所有字軌資料 | `POST /json/track_all` | — | 物件 | 三層巢狀字軌資料 |
| 其他 | 伺服器時間 | `GET /json/time` | — | — | **唯一免簽章端點** |
| 自行配號 | 字軌取號 | `POST /json/track_get` | — | 物件 | 有副作用、無冪等鍵 |
| 自行配號 | 字軌狀態 | `POST /json/track_status` | — | 物件 | 只回「API 配號」類型的字軌 |
| 自行配號 | 開立發票(API 配號) | `POST /json/f0401_custom` | `/json/c0401_custom` | **陣列** | 我方自行指定發票號碼／日期／隨機碼 |

**未文件化但實際存在**：`POST /json/f0701`（與 `/json/c0701`）＝發票註銷 → §2.24。

### 0.3 官方文件 URL 清單（全部可 curl）

| # | 內容 | URL |
|---|---|---|
| S1 | API 文件首頁（SPA 殼） | https://invoice.amego.tw/api_doc/ |
| S2 | **API 定義本體（bundle）** | https://invoice-static.amego.tw/www/api_doc/assets/main.bundle.js?2026061001 |
| S3 | API 範例程式（PHP / Python / Java / C#） | https://invoice.amego.tw/api_doc/example |
| S4 | **API 錯誤代碼**（通用＋17 個分節） | https://invoice.amego.tw/info_detail?mid=71 |
| S5 | 自動配號 vs API 配號說明 | https://invoice.amego.tw/info_detail?mid=70 |
| S6 | 光貿會員載具規則 | https://invoice.amego.tw/info_detail?mid=73 |
| S7 | 統一編號、載具、捐贈碼檢查 | https://invoice.amego.tw/info_detail?mid=74 |
| S8 | 機型支援功能表 | https://invoice.amego.tw/info_detail?mid=77 |
| S9 | **API 處理時限說明（關帳日／法定上傳期限）** | https://invoice.amego.tw/info_detail?mid=95 |
| S10 | API 文件異動紀錄 | https://invoice.amego.tw/info_detail?mid=76 |
| S11 | 發票查詢連結產生規則 | https://invoice.amego.tw/info_detail?mid=101 |
| S12 | **API 補充說明目錄（重抓時以它為準，不要憑記憶列 mid）** | https://invoice.amego.tw/info_list?cid=24 |
| S13 | 🧩開立發票▸作廢發票（作廢期限） | https://invoice.amego.tw/info_detail?mid=24 |
| S14 | 🧩開立發票▸折讓單（折讓日期窗口／部分 vs 全額） | https://invoice.amego.tw/info_detail?mid=25 |
| S15 | 電子發票相關問題（作廢 vs 折讓判準） | https://invoice.amego.tw/info_detail?mid=47 |
| S16 | 📎補充▸變更買方統編／補打買方統編（註銷唯一情境） | https://invoice.amego.tw/info_detail?mid=60 |
| S17 | 重複開立作業規範（收費） | https://invoice.amego.tw/info_detail?mid=105 |
| S18 | 電子發票熱門問題（會不會自動 Mail） | https://invoice.amego.tw/info_detail?mid=45 |
| S19 | 關於光貿加值中心相關問題（費用方案／客服時間） | https://invoice.amego.tw/info_detail?mid=46 |
| S20 | 申請流程：**無**電子發票營業人 | https://invoice.amego.tw/info_list?cid=2 （mid=78 / 39 / 36 / 40） |
| S21 | 申請流程：**有**電子發票營業人 | https://invoice.amego.tw/info_list?cid=3 （mid=41 / 42 / 43） |
| S22 | 消費者發票查詢頁 | https://invoice.amego.tw/user/order_query |
| S23 | 官網／測試後台登入入口 | https://invoice.amego.tw/ |
| E1 | 財政部 統一編號檢查碼邏輯 PDF | https://www.fia.gov.tw/singlehtml/3?cntId=c4d9cff38c8642ef8872774ee9987283 |
| E2 | 財政部 公司名稱查詢 swagger（ban_query 的上游） | https://eip.fia.gov.tw/OAI/swagger-ui.html |
| E3 | 財政部 捐贈碼清單（open data） | https://www.einvoice.nat.gov.tw/portal/ods/ODS318E/einvoice_open_data |
| E4 | 財政部 捐贈碼 CSV 下載（**需圖形驗證碼**） | https://www.einvoice.nat.gov.tw/portal/btc/btc603w/search |

### 0.4 本次合併時的即時覆核（2026-08-24）

| 覆核項 | 命令 | 結果 |
|---|---|---|
| API 可達性 | `curl -sS https://invoice-api.amego.tw/json/time` | `{"timestamp":1787578776,"text":"2026\/08\/24 21:39:36",...}` ✅ |
| 錯誤碼表是否含 `3040113` | `curl "…mid=71" \| grep -c 3040113` | **0**（確認官方表**沒有**這個碼，但實測存在 → §6-20） |
| 錯誤碼表是否含 `3050113`（CancelReason） | 同上 grep | **命中** ✅（確認 `CancelReason` 確實存在但參數表沒列 → §6-19） |
| 機型支援功能表原始內容 | `curl "…mid=77"` 解析 `<table>` | **推翻兩份二手抄錄** → §6-8（以本檔 §3.17 為準） |

---

## 1. 環境與認證

### 1.1【文件】API 網址

> - API 網址：https://invoice-api.amego.tw
> - 目前測試或正式都請用同一個 API 網址
> - 測試請用測試公司的統編與 App Key，進行測試。正式上線時再切換為貴公司的統編與 App Key。

**⇒ 沒有獨立的 sandbox 網域。測試／正式只靠「統編 + App Key」區分。**

| | 測試公司 | 正式公司 |
|---|---|---|
| 統編 | `12345678` | 公司統編 |
| App Key | `sHeq7t8G1wiQvhAuIM27` | 請與客服聯絡 |

**測試帳號後台**【文件】：網址 `https://invoice.amego.tw/`、帳號 `test@example.com`、密碼 `12345678`、統編 `12345678`、公司「測試環境有限公司」。

**這是本整合最大的環境風險**：base URL 相同、回應中**沒有任何環境標記**，config 填錯就會對正式公司開出真發票，而且**無法從回應辨識**。
⇒ 建議在 config `Validate()` 加 fail-fast：非 prod 環境時 `AMEGO_INVOICE` 必須等於 `12345678`。

### 1.2 測試環境 vs 正式環境：已知差異

| # | 面向 | 測試 | 正式 | 來源 |
|---|---|---|---|---|
| D1 | API base URL | `https://invoice-api.amego.tw` | **同左** | S2【文件】 |
| D2 | 統編 | `12345678` | 貴公司統編 | S2【文件】 |
| D3 | App Key | `sHeq7t8G1wiQvhAuIM27` | 「請與客服聯絡」 | S2【文件】 |
| D4 | **Email 通知** | **不會主動發送信件** | 會自動發 Email 通知 | S2 + S18【文件】 |
| D5 | 補發信件 | 登入測試後台 → 查到該筆發票 → 按「補發發票開立通知」 | 自動寄出（亦可後台補寄） | S2【文件】 |
| D6 | 資料隔離 | **無**，公用測試池（實測有 7 萬+ 筆他人資料） | 貴公司獨立 | 【實測】 |
| D7 | 簽章演算法 | 相同 | 相同 | S2 |
| D8 | 端點路徑 | 相同 | 相同 | S2 |
| D9 | 發票字軌 | 測試公司既有字軌 | 需自行向財政部取號並匯入 | S20/S21 |

**D4 的實作影響**：「開票後買家收到通知信」這條 happy path **在 dev 無法自動驗證**。
dev 只能驗到「AMEGO 回 `code:0` 且 `BuyerEmailAddress` 已送達」。驗收條件必須拆兩段，**不可在 dev 宣稱「通知信已驗證」**。

### 1.3 認證機制與簽章

#### 1.3.1【文件】基本傳入參數說明

> - 每支 API 必須帶入這些參數，B2C 及 B2B API 的欄位內容則需編碼成 JSON 格式字串，放置於 data 欄位內傳送
> - 每支 API 接收到的 POST 資料會先自動進行一次 url decode，所以 <span style="color:red">POST 的 data 欄位內容請進行 url encode 動作</span>
> - POST API 的 Header Content-Type 應為 <span style="color:blue">application/x-www-form-urlencoded</span>，請勿使用 Content-Type: application/json

#### 1.3.2【文件】共通參數表

| 欄位 | 類型 | 描述 |
|---|:---:|---|
| `invoice` | String | 統一編號 |
| `data` | String | API 的 JSON 格式字串 |
| `time` | Number | 時間戳記，僅接受與伺服器時間誤差+-60秒 |
| `sign` | String | 簽名，md5加密，加密規則：md5(data 轉 json 格式字串 + time + APP Key) |

#### 1.3.3 補上文件沒寫、但實作必須知道的欄位細節

| 欄位名 | 型別 | 長度 | 必填 | 說明 | 範例 |
|---|---|---|---|---|---|
| `invoice` | String（數字字串） | 8 碼【推定，文件只寫「統一編號」】 | ✅ | 我方（賣方／開票方）統編，同時是 API 帳號識別。空值→`code 11`；非本帳號／不存在→`code 12`【實測】 | `12345678` |
| `data` | String | 未載明上限【推定受 PHP `post_max_size` 限制；官方僅載明「商品陣列最多 9999 筆」】 | ✅ | 業務參數，先序列化成 JSON 字串再放入本欄。**必須是 JSON 物件或陣列**；純量在多筆型 API 回 `code 23`、`null` 回 `code 17`【實測】 | `[{"ban":"45444444"}]` |
| `time` | Number（Unix 秒） | 10 碼整數 | ✅ | 「Unix Timesatmp 10位數，不含毫秒」（S3 逐字）。誤差 **±60 秒（含）**；毫秒值直接判 `code 15`【實測】 | `1787577174` |
| `sign` | String | 32 碼**小寫** hex | ✅ | `md5(data + time + AppKey)`。大寫 hex 被拒（`code 16`）【實測】 | `418571bfda957d168bffeb2a728f591a` |
| （App Key） | String | 範例值 20 碼英數大小寫混合；長度規則未載明 | — | **不出現在請求中**，只參與 md5 | `sHeq7t8G1wiQvhAuIM27` |

**沒有的東西（不要憑印象加）**：沒有 Authorization header、沒有 OAuth／token、沒有 API version header、沒有 nonce、沒有 request id。
**認證資訊全部在 form body 的四個欄位裡。** 另有來源 IP 檢查（錯誤碼 `14 IP 錯誤`，機制完全未載明 → §5-B1）。

#### 1.3.4 簽章計算

【文件】「簽名，md5加密，加密規則：md5(data 轉 json 格式字串 + time + APP Key)」
即 `sign = md5( <data 欄位的 JSON 字串> + <time 的十進位字串> + <App Key> )`，
三段**直接串接、無分隔符、無排序、無 key=value 組裝**，UTF-8 位元組，輸出小寫 hex。

【文件】官方各語言片段（S3）：

```php
// PHP
$sData_String = json_encode($aData, JSON_UNESCAPED_UNICODE);
$sSign = md5($sData_String . ((string) $nTime) . $sApp_Key);
```
```python
# Python（原文含官方自檢值）
sApi_Data = json.dumps(invoice_data, indent = 0)
# 此範例 md5 結果為 f53a336934b2af0589b845638d1495cc，請自檢測是否相符
sHash_Text = sApi_Data + str(nCurrent_Now_Time) + APP_KEY
sSign = hashlib.md5(sHash_Text.encode("utf-8")).hexdigest()
```
```java
// Java（原文含官方自檢值）
// 此範例 md5 結果為 efe84e2b95153a09df64a36e04e8ae1c，請自檢測是否相符
String sHash_Text = sApi_Data + nCurrent_Now_Time + APP_KEY;
byte[] bytesOfMessage = sHash_Text.getBytes("UTF-8");
```
```csharp
// C#（注意原文最後有 .ToLower()）
string signBefore = data + time + "sHeq7t8G1wiQvhAuIM27";
sign = ToHexString(md5.ComputeHash(Encoding.UTF8.GetBytes(signBefore))).ToLower();
```

**【實測】官方兩個自檢值都能重現，而它們證明了一件關鍵的事**（`time=1628136135`、AppKey 同上）：

| 官方範例 | data 字串長相 | 官方宣稱 md5 | 重算 | 結果 |
|---|---|---|---|---|
| Python | `json.dumps(..., indent=0)` ⇒ 每個 key 一行、含換行、中文 `\uXXXX` 轉義 | `f53a336934b2af0589b845638d1495cc` | 相同 | ✅ |
| Java | 單行 compact、中文為 UTF-8 原字 | `efe84e2b95153a09df64a36e04e8ae1c` | 相同 | ✅ |

⇒ **同一份業務資料、兩種完全不同的 JSON 排版，兩個 sign 都是官方認可的正確值。**
⇒ **AMEGO 不做任何 canonicalization**，伺服器拿「你實際送來的 data 字串」原樣算 md5 比對。
⇒ **實作鐵則：序列化一次，把那個字串同時用於 (a) 算 sign 與 (b) 放進 form 的 `data`。**
   任何「先 marshal 算 sign、再 marshal 一次送出」的寫法（中間經過 map 重排／struct 換序／`SetEscapeHTML` 設定不同／多一次 pretty print）
   都會 100% 回 `code 16`，而且錯誤訊息完全不提示原因。

**可直接寫進單元測試的固定 fixture**（`AppKey = sHeq7t8G1wiQvhAuIM27`）：

| data（原樣，含所有引號） | time | 期望 sign |
|---|---|---|
| `[{"ban":"45444444"}]` | `1787577174` | `418571bfda957d168bffeb2a728f591a` |
| `{"barCode":"/TRM+O+P"}` | `1787577174` | `5a928f1c56c093d3bf1a19544baeb6e5` |
| `{"OrderId":"A20200817105934","BuyerIdentifier":"45444444"}` | `1628136135` | `e69e42249735e2e07d76a952c33ccea4` |

#### 1.3.5 一個特例：`/json/lottery_type` 的簽章基準【實測】

文件說「data 欄位不需要傳入資料」。實測：
- `data=`（空字串）＋ `sign=md5("" + time + key)` → **成功** `{"code":0,...}`
- `data={}` ＋ `sign=md5("{}" + time + key)` → **失敗** `{"code":16,"msg":"sign(簽名)驗證錯誤","data":[]}`

⇒ 這支 API 的伺服器端**固定以空字串驗簽**，忽略實際傳入的 data。呼叫它時 `data` 必須送空字串。

### 1.4 共通 request 格式

- Method：POST（唯一例外 `GET /json/time`）
- `Content-Type: application/x-www-form-urlencoded`（官方明令「請勿使用 Content-Type: application/json」）
- Body：四個欄位 `invoice` / `data` / `time` / `sign`，標準 form 編碼

【實測】用 `Content-Type: application/json` 送 JSON body → 回 `{"code":11,"msg":"invoice(統編)不可為空"}`（**HTTP 仍為 200**，且訊息完全誤導）。`text/plain` 同樣回 11。
【實測】`multipart/form-data` 意外可用（回 `code 0`），但**文件未載明、不要依賴**。

#### 1.4.1 「請進行 url encode 動作」的正確解讀（最容易做錯的一句話）

字面上會讀成「要比標準 form 編碼**多**編一次」。**【實測】結論：不是，就是標準的一次 form 編碼。**

| 送法 | 結果【實測】 |
|---|---|
| `data` 做**標準一次** form 編碼，sign 取原 JSON | `code 0` ✅ |
| `data` 先自行 urlencode 一次、再 form 編碼（雙重編碼），sign 取**原 JSON** | `code 16` 簽名驗證錯誤 |
| `data` 雙重編碼，sign 取**編碼後**字串 | `code 17`（簽名過了，但 JSON 解析失敗） |

**決定性佐證**：送 `{"barCode":"A%20B"}`（值裡有字面上的 `%20`）、sign 取原字串 → 回 `9000112 手機條碼格式錯誤` 而**不是** `code 16`。
若伺服器真的多做一次 decode，`A%20B` 會變成 `A B`、sign 必然對不上 ⇒ **伺服器只解碼一次**（就是 PHP 讀 `$_POST` 的那一次）。

含 `+` 的值也安全：`{"barCode":"/TRM+O+P"}` 以標準編碼（`+` → `%2B`）送出 → `{"code":0,"msg":""}`。
**前提是用真正的 form encoder**（產生 `%2B`），不要自己拼字串把 `+` 直接塞進 body——那才會被解成空白。

#### 1.4.2 `data` 是單筆或多筆由每支 API 各自規定

- **多筆型**（`ban_query`、`invoice_status`、`f0501`、`g0401`、`g0501`、`f0401_custom`、`allowance_status`）：`data` 必須是 **JSON 陣列字串**，否則 `code 23`（通用）或各 API 的 `…112` 變體。
  - 【實測】`ban_query` 傳物件 → `{"code":23,"msg":"此 API 支援傳輸多筆資料，data 欄位資料應為陣列字串"}`
  - 【實測】傳空陣列 `[]` → `{"code":17,"msg":"data 不可為空"}`
- **單筆型**（`f0401`、各查詢／列表／檔案／列印類）：`data` 必須是**物件字串**，否則 `f0401` 回 `3040112`「此 API 只支援傳輸單張發票，data 欄位資料應為物件字串」。
- **無參數型**（`lottery_type`）：`data` 送**空字串**，且 sign 用空字串算（§1.3.5）。

### 1.5 共通 response 格式

#### 1.5.1 Envelope（三種形狀，不能只做一個泛型 DTO）

**（a）標準型**（絕大多數 API）：

| 欄位 | 型別 | 必回 | 說明【文件，兩種措辭並存於原文】 |
|---|---|:--:|---|
| `code` | Number | ✅ | 「0 代表正確 其他代碼請參考錯誤代碼」／「0 代表正確，其他代碼代表錯誤。」 |
| `msg` | String | ✅ | 「錯誤訊息」（`code=0` 時為空字串 `""`）【實測】 |
| `data` | Object 或 Object[] | ❌ | 「資料陣列，正確才會回傳」／「正確才會回傳」 |

**（b）分頁型**（`invoice_list` / `allowance_list`）額外三欄，**與 `data` 平級、不在 data 之內**：

| 欄位 | 型別 | 說明【文件】 |
|---|---|---|
| `page_total` | Number | 總共頁數，正確才會回傳 |
| `page_now` | Number | 目前頁數，正確才會回傳 |
| `data_total` | Number | 總資料數，正確才會回傳 |

**（c）`f0401` 例外**：業務結果**攤在頂層、不放 `data`**（`invoice_number` / `invoice_time` / `random_number` / `barcode` / `qrcode_left` / `qrcode_right` / `base64_data`）。
**（d）`GET /json/time` 例外**：**沒有 `code` / `msg`**，形狀與其他 API 完全不同。

⇒ **DTO 設計含意**：不能只做一個 `AmegoResponse<T>{code,msg,data}` 就收工。用泛型 envelope 硬套 `f0401` 會拿不到發票號碼。

#### 1.5.2 HTTP 層的硬事實【實測】

| 觀察 | 值 | 實作含意 |
|---|---|---|
| HTTP status | **一律 200**，包含簽章錯誤、統編錯誤、業務錯誤 | **絕對不能用 HTTP status 判成敗**，只能看 body 的 `code` |
| 不存在的路徑 `POST /json/not_exist` | HTTP **200**、`Content-Type: text/html`、body 就是字串 `error` | 打錯路徑時 JSON 解析失敗而非 404 ⇒ 反序列化失敗要有獨立分支，不能吞掉 |
| POST 類回應 Content-Type | `application/json; charset=utf-8` | — |
| `GET /json/time` 回應 Content-Type | **`text/plain; charset=utf-8`**（不是 json） | HTTP client 若用 Content-Type 決定是否反序列化，這支會失敗 |
| JSON 轉義風格 | `"2026\/08\/24 21:11:44"`（PHP `json_encode` 預設轉義 `/`） | 標準 parser 皆可處理，但正則比對字串時要注意 |
| 其他 header | `Access-Control-Allow-Origin: *`、`Strict-Transport-Security`、`Cache-Control: no-store` | **無任何 rate-limit header**（無 `X-RateLimit-*`、無 `Retry-After`） |
| `code` 的 JSON 型別 | **不一致**：`f0501`／查詢類回 **Number**；`g0401`／`g0501`／`f0701` 回 **String** | DTO 的 `code` 必須能同時吃 number 與 string（Go `json.Number` 或自訂 `UnmarshalJSON`）。用 `int` 直接綁會在**折讓的錯誤路徑**上 unmarshal 失敗，而那正是最需要讀到錯誤碼的路徑 |

#### 1.5.3 `msg` 文字與文件表格不一致【實測】

| code | 文件表格寫的（S4） | 實際回傳的 msg |
|---|---|---|
| 15 | `Time 錯誤` | `time(時間戳記)錯誤` |
| 16 | `簽名驗證錯誤` | `sign(簽名)驗證錯誤` |
| 17 | `資料不可為空` | `data 不可為空` |
| 11 / 12 / 23 | （一致） | （一致） |

⇒ **所有分支判斷只准用 `code`，`msg` 只能原樣落庫／原樣顯示給人看。** 任何 `if msg == "..."` 都會壞。
⇒ 但**有兩處不得不比對 msg**（因為 code 撞在一起）：`3040171` 的兩種語意（§2.1.6）、`f0401_custom`／`track_get` 全部回 `99`（§3.20）。這是文件層面的既成事實，設計時要正面承認。
⇒ DB 欄位：`msg` 為中文，且部分錯誤會內插「第n筆／第n品項」與欄位名，長度不可預期 ⇒ 用 `TEXT`【推定，文件未載明長度】。

### 1.6 驗證順序（決定錯誤碼分支的優先序）【實測】

同時弄壞多個欄位，回傳的是**最先失敗的那一個**，順序固定為：

```
invoice（11 空 / 12 錯） → time（15） → sign（16） → data（17 / 20 / 23 / 各 API 自有碼）
```

| 送出內容 | 回應（全部 HTTP 200） |
|---|---|
| invoice=99999999 + time 錯 + sign 錯 + data 錯 | `{"code":12,"msg":"invoice(統編)錯誤"}` |
| invoice 正確 + time 錯 + sign 錯 + data 錯 | `{"code":15,"msg":"time(時間戳記)錯誤"}` |
| invoice/time 正確 + sign 錯 + data 錯 | `{"code":16,"msg":"sign(簽名)驗證錯誤"}` |
| invoice/time/sign 正確 + 省略 data | `{"code":17,"msg":"data 不可為空"}` |

⇒ **不能用「回 16 就是金鑰錯」當唯一判斷**——16 也可能是 data 在傳輸中被改（例如自己重新 marshal 過一次，§1.3.4）。
💡 反向利用：`ban_query` 故意傳物件會回 `code 23`（而非 16），這證明**簽章已通過**——是驗證憑證有效性最省成本的方法。

### 1.7 `time` 窗口的精確邊界【實測】

| 與伺服器時間的偏移 | 回應 |
|---|---|
| `-60` / `-59` 秒 | `code 0` ✅ |
| `-61` / `-600` 秒 | `code 15` ❌ |
| `+60` 秒 | `code 0` ✅ |
| `+61` / `+120` 秒 | `code 15` ❌ |
| 缺 `time` 欄位 | `code 15` ❌ |
| `time` 帶毫秒（13 位） | `code 15` ❌ |
| `time` 帶小數 `1787577208.0` | `code 16`（伺服器轉整數後算 sign，導致簽章不符） |

⇒ **pod 時鐘偏移超過 1 分鐘就全線失敗**，錯誤碼是 15。監控上要能把「大量 code 15」對應到 NTP 問題。
⇒ **每次重試都要重算 `time` 與 `sign`**，沿用舊值超過 60 秒必定失敗。
⇒ `time` 一律用整數秒送（Go `strconv.FormatInt(t.Unix(),10)`），不要讓浮點數混進來。

### 1.8 重放（replay）行為【實測】

完全相同的 `invoice/data/time/sign` 連送 3 次，3 次都回 `code 0`：
```
第1次 -> {"code":0,"msg":"","data":[{"ban":"45444444","name":"光貿科技股份有限公司"}]}
第2次 -> （相同）
第3次 -> （相同）
```
⇒ **沒有 nonce、沒有 request-id、沒有伺服器端去重。** 60 秒窗口內同一份簽章可無限重放。
⇒ **冪等性完全由業務欄位承擔**：發票靠 `OrderId`（重複回 `3040171`）；折讓靠 `AllowanceNumber`（重複回 `4040161`）。

### 1.9 冪等對帳決策樹（本檔的建議，官方完全沒談冪等）

```
需要判斷「這張訂單是否已開立發票」：

1. 先查我方 DB。有 invoice_number → 結束。

2. 沒有 → POST /json/invoice_query {"type":"order","order_id":"<我方訂單號>"}
   ├ code 0            → 已開立。回填 invoice_number / invoice_date / random_number / invoice_status
   ├ code 71 查無資料   → 「180 天內確定沒開過」→ 可安全開立
   └ code 51 超過查詢期限 → **不確定**（發票日期已超過 180 天）
        → 改用 /json/invoice_list（date_select=1，涵蓋該訂單可能的日期區間）逐頁比對 order_id
        → 仍找不到才視為未開立

3. 開立 f0401
   ├ code 0        → 成功，落庫
   └ code 3040171  → 已被開過 → 回到步驟 2 取回發票號碼
                     ⚠️ msg 若為「OrderId 重複，該訂單編號正在註銷重開中」→ 退避重試，不可當永久失敗
```

**明確不可用於冪等判斷的做法**：

| 做法 | 為什麼不行 |
|---|---|
| 用 `invoice_status` 回 `NOT_FOUND` 判定「沒開過」 | 【實測】對超過窗口（推測「當期+前兩期」）的發票**靜默回 NOT_FOUND + code 0**，造成重複開立。§2.4.4 有 9 筆實證 |
| 用 `allowance_status` 回 `NOT_FOUND` 判定折讓沒開過 | 同上，2025-01 的折讓單回 NOT_FOUND，但 `allowance_query` 查得到 |
| 只靠 `code != 0` 判失敗 | 查詢類一律 HTTP 200，且 `71`（查無資料）是正常業務結果不是錯誤；`51` 與 `71` 語意完全不同 |
| 依賴 `data` 一定是陣列 | `invoice_status` / `allowance_status` 文件同時列新版陣列與舊版物件形狀，未說明切換方式 |
| 依賴分頁錯誤碼終止迴圈 | 【實測】page 超界回 `code 0` + 空陣列，不報錯 |
| 用 `file_url` 當永久連結 | 【文件】僅 10 分鐘有效 |

### 1.10 一頁式速查

```
Base URL     : https://invoice-api.amego.tw    （測試/正式同一個，無 sandbox 網域）
Method       : POST（唯一例外 GET /json/time，免簽章）
Content-Type : application/x-www-form-urlencoded   ← 禁用 application/json
Body         : invoice=<我方統編8碼>
               data=<業務 JSON 字串，標準一次 form 編碼>
               time=<Unix 秒，10 位整數，±60 秒內（含）>
               sign=<md5(data + time + AppKey)，32 碼小寫 hex>
序列化鐵則   : 序列化一次，同一個字串同時拿去算 sign 與送出（伺服器不做 canonicalization）
回應         : HTTP 恆 200；{"code":0,"msg":"","data":...}；code≠0 即失敗
                f0401 例外：業務欄位攤在頂層（invoice_number / random_number / ...）
                /json/time 例外：無 code/msg
                code 型別：查詢類 Number，g0401/g0501/f0701 為 String
判成敗       : 只看 code，永不看 HTTP status，永不比對 msg（唯二例外：3040171 與全 99 的 API）
冪等         : 伺服器無 nonce/去重；靠 OrderId（發票）/ AllowanceNumber（折讓）
非同步       : 無 webhook；上傳結果輪詢 /json/invoice_status（有隱藏時間窗，見 §4.6）
                中獎輪詢 /json/lottery_status（官方建議雙月 1 號後）
法定時限     : 個人發票開立後 2 天內、統編發票 7 天內必須上傳完成（逾時國稅局記點/罰款）
關帳日       : 單月 5 號 23:59:59；6 日 00:00 起鎖定上期「開立/註銷/作廢」上傳
查詢窗口     : invoice_query/file/print = 180 天；invoice_list/allowance_* list/query = 無限制
                invoice_status/allowance_status = 約當期+前兩期，超過靜默 NOT_FOUND
```

---

## 2. API 逐支參考

### 2.1 開立發票（自動配號）— `POST /json/f0401`

- 舊路徑 `/json/c0401`（【實測】仍可用，回應一致）；來源檔 `c0401.php`；group `發票` / name `Invoice`；version `1.0.0`
- 【文件】說明：開立發票後，回傳發票號碼、發票時間及隨機碼，若有傳入熱感應機型號代碼，則會多輸出列印格式字串。
- `data` 形狀：**單一物件**（送陣列回 `3040112`）

#### 2.1.1 Request 參數（`data` 內的 JSON）

| 欄位名 | 型別 | 必填 | 說明【文件逐字】 | 長度（文件／【實測】） |
|---|---|---|---|---|
| `OrderId` | String | **是** | 訂單編號，不可重複，不可超過40字 | ≤40／【實測】41 字未被擋 |
| `TrackApiCode` | String | 否 | 指定字軌開立，需要在後台 發票作業 > 發票字軌列表 設定 API指定代碼 若不指定字軌，則依照後台發票字軌列表的排序去開立 | 未載明（實測值 `FSM` 為 3 碼） |
| `BuyerIdentifier` | String | **是** | 買方統一編號，買方統一編號，沒有則填入 0000000000 | 8（統編）或 10（`0000000000`） |
| `BuyerName` | String | **是** | 買方名稱 1. 不打統編：可以填寫客人、消費者 2. 打統編：如不能填入買方公司名稱，請填買方統一編號 3. 不可填0、00、 000及0000 | 未載明／**【實測】60 字元**（61 → 3040123） |
| `BuyerAddress` | String | 否 | 買方地址 | 未載明／【實測】300 字 → 3040124 |
| `BuyerTelephoneNumber` | String | 否 | 買方電話 | 未載明／【實測】100 字 → 3040125 |
| `BuyerEmailAddress` | String | 否 | 買方電子信箱，寄送通知信用，若不希望寄送，留空即可。 測試環境不會主動發送信件，若想要收測試信件，請登入測試後台，查詢到該筆發票內容，按下「補發發票開立通知」按鈕 | 未載明／【實測】312 字 → 3040126 |
| `MainRemark` | String | 否 | 總備註，不可超過200字 | ≤200（【實測】200 PASS／201 → 3040127） |
| `CarrierType` | String | 否 | 載具類別，手機條碼 3J0002，自然人憑證條碼 CQ0001 光貿會員載具 amego，載具顯碼及載具隱碼需為 a+手機號碼 或 電子信箱 光貿會員載具的載具顯碼或隱碼帶入電子信箱，並不會發送信件，若想發送信件請填寫 BuyerEmailAddress 欄位 載具類別亦可帶入貴公司自行申請的會員載具 | 6 碼（`amego` 例外為 5 碼）／【實測】7 碼 → 3040128 |
| `CarrierId1` | String | 否 | 載具顯碼 | 未載明／**【實測】64 字元**（65 → 3040129） |
| `CarrierId2` | String | 否 | 載具隱碼 | 同上（3040130） |
| `NPOBAN` | String | 否 | 捐贈碼 | 未載明／**【實測】3–7 碼純數字** |
| `ProductItem` | Object[] | **是** | 商品陣列，最多 9999 筆 | ≤9999 筆 |
| `ProductItem.Description` | String | **是** | 品名，不可超過256字 | ≤256（【實測】256 PASS／257 → 3040139） |
| `ProductItem.Quantity` | Number | **是** | 數量，小數精準度到7位數 | 小數 7 位 |
| `ProductItem.Unit` | String | 否 | 單位，不可超過6字 | ≤6 |
| `ProductItem.UnitPrice` | Number | **是** | 單價，預設含稅，小數精準度到7位數。 若發票打統編需用未稅價，DetailVat 欄位可設定為 0 | 小數 7 位 |
| `ProductItem.Amount` | Number | **是** | 小計，小數精準度到7位數。 小計可透過 DetailAmountRound 欄位可設定為1:一律四捨五入到整數 | 小數 7 位 |
| `ProductItem.Remark` | String | 否 | 備註，不可超過120字 | ≤120（【實測】120 PASS／121 → 3040143）與查詢回應的 40 字矛盾 → §6-1 |
| `ProductItem.TaxType` | Number | **是** | 課稅別　1：應稅　2：零稅率　3：免稅 | 1 碼 |
| `SalesAmount` | Number | **是** | 應稅銷售額合計 | 未載明 |
| `FreeTaxSalesAmount` | Number | **是** | 免稅銷售額合計 | 未載明 |
| `ZeroTaxSalesAmount` | Number | **是** | 零稅率銷售額合計 | 未載明 |
| `TaxType` | Number | **是** | 課稅別　1：應稅　2：零稅率　3：免稅　4：應稅(特種稅率)　9：混合應稅與免稅或零稅率(限訊息C0401使用) | 1 碼 |
| `TaxRate` | String | **是** | 稅率，為5%時本欄位值為0.05 | **String 型別** |
| `TaxAmount` | Number | **是** | 營業稅額。有打統編才需計算5%稅額，沒打統編發票一律帶0。 | 未載明 |
| `TotalAmount` | Number | **是** | 總計 | 未載明 |
| `CustomsClearanceMark` | Number | 否（零稅率時必填） | 通關方式註記，若為零稅率發票，則此欄位為必填。 1:非經海關出口 2:經海關出口 | 1 碼 |
| `ZeroTaxRateReason` | Number | 否（零稅率時必填） | 零稅率原因，若為零稅率發票，則此欄位為必填。（71–79 全文見 §3.10） | 2 碼 |
| `BrandName` | String | 否 | 品牌名稱 | 未載明（錯誤碼 3040160 說「不可為空或過長」⇒ 帶了就不能是空字串） |
| `DetailVat` | Number | 否 | 明細的單價及小計 為 含稅價 或 未稅價，**預設為含稅價**〔原文紅字〕，0:未稅價 1：含稅價 **只有**打統編**的發票可以設定 0:未稅價**〔原文紅字〕 | 1 碼 |
| `DetailAmountRound` | Number | 否 | 明細的小計處理方式，預設為小數精準度到7位數，0:小數精準度到7位數 1:一律四捨五入到整數 | 1 碼 |
| `TaxAdjustment` | Number | 否 | 稅額調整，預設 0 0:不調整，1:營業稅額減 1 僅限打統編且 DetailVat=0（未稅）時，且 SalesAmount 應稅銷售額合計尾數為 10/30/50/70/90 才可使用 | 1 碼 |
| `PrinterType` | Number | 否 | 熱感應機型號代碼 可支援功能請參看機型支援功能表 | 1–2 碼 |
| `PrinterLang` | Number | 否 | 熱感應機編碼 1：BIG5 2：GBK 3：UTF-8 若不帶入此參數，會使用預設編碼 | 1 碼 |
| `PrintDetail` | Number | 否 | 熱感應機是否列印明細 0:不列印 1:印出明細(裁切) (預設) 2:印出明細(不裁切) 打統編一律列印明細 目前僅支援 PrinterType >= 2 可以設定此參數 | 1 碼 |

#### 2.1.2 Response 欄位（Success 200）

| 欄位名 | 型別 | 必回 | 說明【文件逐字】 |
|---|---|:--:|---|
| `code` | Number | ✅ | 0 代表正確 其他代碼請參考錯誤代碼 |
| `msg` | String | ✅ | 錯誤訊息 |
| `invoice_number` | String | ❌ | 發票號碼，正確才會回傳 |
| `invoice_time` | Number | ❌ | 發票開立時間，Unix 時間戳記，正確才會回傳 |
| `random_number` | String | ❌ | 隨機碼，正確才會回傳 |
| `barcode` | String | ❌ | 電子發票的條碼內容，正確才會回傳 |
| `qrcode_left` | String | ❌ | 電子發票的左側 QRCODE 內容，正確才會回傳 **0元發票會回傳空字串** |
| `qrcode_right` | String | ❌ | 電子發票的右側 QRCODE 內容，正確才會回傳 **0元發票會回傳空字串** |
| `base64_data` | String | ❌ | PrinterType = 1，base64編碼的 XML 列印格式字串(mC-Print3 熱感應機專用)，正確且需要列印才會回傳。 PrinterType >= 2，base64編碼的 ESC/POS 列印格式字串，正確且需要列印才會回傳。 如何設定熱感應機及印出發票，請洽客服。 **0元發票不會回傳此欄位** |

#### 2.1.3 官方 Request 範例（原樣抄錄）

**一般開立（需列印）**
```json
{
	"OrderId": "A20200817101021",
	"BuyerIdentifier": "0000000000",
	"BuyerName": "客人",
	"BuyerAddress": "",
	"BuyerTelephoneNumber": "",
	"BuyerEmailAddress": "",
	"MainRemark": "",
	"CarrierType": "",
	"CarrierId1": "",
	"CarrierId2": "",
	"NPOBAN": "",
	"ProductItem": [
		{ "Description": "測試商品1", "Quantity": "1", "UnitPrice": "170", "Amount": "170", "Remark": "", "TaxType": "1" },
		{ "Description": "會員折抵", "Quantity": "1", "UnitPrice": "-2", "Amount": "-2", "Remark": "", "TaxType": "1" }
	],
	"SalesAmount": "168",
	"FreeTaxSalesAmount": "0",
	"ZeroTaxSalesAmount": "0",
	"TaxType": "1",
	"TaxRate": "0.05",
	"TaxAmount": "0",
	"TotalAmount": "168"
}
```

**光貿會員載具**（其餘同上，差異部分）
```json
{ "CarrierType": "amego", "CarrierId1": "a0911222333", "CarrierId2": "a0911222333" }
```

**手機載具**
```json
{ "CarrierType": "3J0002", "CarrierId1": "\/TRM+O+P", "CarrierId2": "\/TRM+O+P" }
```

**捐贈**
```json
{ "NPOBAN": "275" }
```

**打統編（含稅價）**
```json
{
	"OrderId": "A20200817101322",
	"BuyerIdentifier": "45444444",
	"BuyerName": "光貿科技有限公司",
	"ProductItem": [
		{ "Description": "測試商品1", "Quantity": "1", "UnitPrice": "170", "Amount": "170", "Remark": "", "TaxType": "1" },
		{ "Description": "會員折抵", "Quantity": "1", "UnitPrice": "-2", "Amount": "-2", "Remark": "", "TaxType": "1" }
	],
	"SalesAmount": "160", "FreeTaxSalesAmount": "0", "ZeroTaxSalesAmount": "0",
	"TaxType": "1", "TaxRate": "0.05", "TaxAmount": "8", "TotalAmount": "168",
	"DetailVat": 1
}
```

**打統編（未稅價）**
```json
{
	"BuyerIdentifier": "45444444",
	"ProductItem": [
		{ "Description": "測試商品1", "Quantity": "1", "UnitPrice": "161.9", "Amount": "161.9", "Remark": "", "TaxType": "1" },
		{ "Description": "會員折抵", "Quantity": "1", "UnitPrice": "-1.9", "Amount": "-1.9", "Remark": "", "TaxType": "1" }
	],
	"SalesAmount": "160", "TaxAmount": "8", "TotalAmount": "168",
	"DetailVat": 0
}
```

#### 2.1.4 Response 實測【官方文件缺此範例，本節為實測補上】

⚠️ 官方對 `f0401` 只給了 Success 欄位表，**沒有任何 response JSON 範例**（只有 `f0401_custom` 有）。
**`f0401` 的成功欄位平鋪在頂層，不是包在 `data` 裡**——與 `f0401_custom` 的 `data[]` 結構**不同**，兩支的 response DTO **不能共用**。

B2C（不打統編）：
```json
{"code":0,"msg":"","invoice_number":"ZA10063775","invoice_time":1787577229,"random_number":"9690",
 "barcode":"11508ZA100637759690",
 "qrcode_left":"ZA1006377511508249690000000a0000000a8000000001234567867y6BkUlQxCwz4v/rc5j6w==:**********:1:2:0:",
 "qrcode_right":"**測試商品1:1:170"}
```
B2B（打統編 `45444444`，`DetailVat=1`，160/8/168）：
```json
{"code":0,"msg":"","invoice_number":"ZA10063776","invoice_time":1787577243,"random_number":"2769",
 "barcode":"11508ZA100637762769",
 "qrcode_left":"ZA1006377611508242769000000a0000000a82808062312345678+h6yqxjgh/lLYt7bFJs+ZA==:**********:1:2:0:",
 "qrcode_right":"**測試商品1:1:170"}
```

**可直接拿來定 DB 欄位的觀察**：

| 回傳欄位 | 實測值 | DB 型別建議 |
|---|---|---|
| `invoice_number` | `ZA10063775` | 固定 **10 碼**（2 英文字軌 + 8 數字）→ `CHAR(10)` |
| `random_number` | `9690` | 固定 **4 碼數字字串** → `CHAR(4)`；**必須存字串**，前導 0 會被吃掉 |
| `invoice_time` | `1787577229` | **Unix 秒級時間戳（Number）**，非字串、非 ISO8601 |
| `barcode` | `11508ZA100637759690` | **19 碼** ＝ 民國年期別 5 碼(`11508`) + 發票號碼 10 碼 + 隨機碼 4 碼 → `VARCHAR(19)` |
| `qrcode_left` / `qrcode_right` | 長度隨品名與品項數變動 | **`TEXT`**，不要設固定長度 |
| `base64_data` | 未帶 `PrinterType` 故未回 | `TEXT` |

⚠️ `barcode` 前 5 碼是**民國年 + 期別**（`11508` = 民國 115 年 07-08 月期），是財政部 MIG 格式不是 AMEGO 自訂。
⚠️ `qrcode_left` 內含 base64 加密驗證資訊（例 `67y6BkUlQxCwz4v/rc5j6w==`），含 `/`、`+`、`=`，**存 DB 與轉 JSON 都不要做任何轉義，原樣存原樣出**。

#### 2.1.5 錯誤分支實測

| 情境 | 實際回應 |
|---|---|
| `OrderId` 重複 | `{"code":3040171,"msg":"OrderId 重複"}` |
| `SalesAmount` 與明細加總不符 | `{"code":3040174,"msg":"SalesAmount 計算錯誤"}` |
| `BuyerName` 空字串 | `{"code":3040123,"msg":"BuyerName 不可為空或過長"}` |
| **不打統編卻帶 `TaxAmount=5`** | `{"code":3040174,"msg":"SalesAmount 計算錯誤"}` ← 訊息**指向錯誤的欄位** |
| `data` 傳陣列 | `{"code":3040112,"msg":"此 API 只支援傳輸單張發票，data 欄位資料應為物件字串"}` |
| `TaxType=3`（免稅）卻把金額放在 `SalesAmount` | `{"code":3040174,"msg":"SalesAmount 計算錯誤"}` |
| `ProductItem: []` | `{"code":3040113,"msg":"ProductItem 資料應為陣列"}` **此碼不在官方錯誤碼表內**（本次 curl 重驗 grep 命中 0） |

**`3040174` 是「什麼都可能」的萬用錯誤**——金額不平、稅額算錯、稅別放錯欄位都回同一碼，訊息還會誤導。
⇒ **必須在送出前自己做完整的金額不變式檢核**，不能靠 AMEGO 的錯誤訊息定位問題。

#### 2.1.6 `3040171` 一碼兩義（重試邏輯的地雷）

官方錯誤碼表有兩列同碼：
- `3040171 OrderId 重複` → **永久衝突**（該訂單已有發票，應改走 `invoice_query` 取回號碼）
- `3040171 OrderId 重複，該訂單編號正在註銷重開中` → **暫時態，應退避重試**

⇒ **無法用 code 區分，只能字串比對 `msg`**，而 msg 是中文且隨時可能改字。**這是脆弱點，要寫測試釘住。**

#### 2.1.7 金額計算邏輯【文件逐字，官方權威算式】

```
如果商品皆為含稅價
DetailVat = 1

SalesAmount = Round(所有 ProductItem TaxType=1 的 Amount 加總)
FreeTaxSalesAmount = Round(所有 ProductItem TaxType=3 的 Amount 加總)
ZeroTaxSalesAmount = Round(所有 ProductItem TaxType=2 的 Amount 加總)

不打統編(不須分拆稅額)
TaxAmount = 0

打統編(須要分拆稅額)
TaxAmount = SalesAmount - Round(SalesAmount / 1.05)
SalesAmount = SalesAmount - TaxAmount

總計
TotalAmount = SalesAmount + FreeTaxSalesAmount + ZeroTaxSalesAmount + TaxAmount
```

```
如果商品皆為未稅價，僅用於打統編發票
DetailVat = 0

SalesAmount = Round(所有 ProductItem TaxType=1 的 Amount 加總)
FreeTaxSalesAmount = Round(所有 ProductItem TaxType=3 的 Amount 加總)
ZeroTaxSalesAmount = Round(所有 ProductItem TaxType=2 的 Amount 加總)

打統編(須要分拆稅額)
TaxAmount = Round(SalesAmount * 0.05)

總計
TotalAmount = SalesAmount + FreeTaxSalesAmount + ZeroTaxSalesAmount + TaxAmount
```

**落地說明（金額一律整數元、禁浮點）**：
- `TaxAmount = SalesAmount - Round(SalesAmount / 1.05)` 以整數運算等價於 `SalesAmount - round(SalesAmount * 20 / 21)`，
  用 `Long` 做 `(SalesAmount * 20 + 10) / 21` 即可，**不要引入 BigDecimal / double**。
- 官方沒有明說 `Round` 的捨入模式（四捨五入 vs banker's rounding）→ §5-C1。
  實測 B2B 例 `168 - round(168/1.05) = 8`，兩種模式結果相同、**沒有鑑別力**，需另用 .5 邊界值驗。
- 明細層 `Amount` 可到小數 7 位，但合計欄位在實測中都是整數。要強制明細整數用 `DetailAmountRound=1`。
- **金額不可為負**（3040152 / 3040181~3040184），但**單一品項的 `UnitPrice`/`Amount` 可以是負數**——官方範例即用 `-2` 表示「會員折抵」。⇒ **折扣要做成負數品項列，不是扣總額。**
- **金額欄位有「格式錯誤」（3040145~3040151）與「計算錯誤」（3040174~3040178）兩組獨立錯誤碼** ⇒ AMEGO 會重算並驗證加總，我方算式必須與上面公式**逐位元一致**（特別是 Round 的時機）。

#### 2.1.8 B2C 與 B2B 的判別（重要：沒有兩支不同的 API）

同一支 `/json/f0401`，靠 `BuyerIdentifier` 決定：

| | B2C（不打統編） | B2B（打統編） |
|---|---|---|
| `BuyerIdentifier` | 固定填 `0000000000`（**10 個 0**） | 買方 8 碼統一編號 |
| `BuyerName` | 可填「客人」「消費者」 | 買方公司名稱；「如不能填入買方公司名稱，請填買方統一編號」 |
| `TaxAmount` | **一律帶 0** | 需計算 5% 稅額 |
| `SalesAmount` | ＝含稅總額（不分拆） | ＝未稅銷售額（已扣除 TaxAmount） |
| `DetailVat` | **只能是 1（含稅）** | 可設 0（未稅）或 1（含稅） |
| 載具／捐贈碼 | 可用 | API 配號的錯誤碼列了「列印發票不可設定載具／捐贈碼」，但 `f0401` **實測不擋** → §2.1.9 |
| 查詢回的 `invoice_type` | `C0401` | `A0401`（B2B 存證）／`A0101`（B2B 交換） |

⚠️ `DetailVat=0` 的限制是硬性的：違反時回 `3040162`「只有打統編發票，才可以用未稅單價及小計」。
⚠️ **這個「打統編／不打統編」的因果連動四個欄位，但文件沒有一段文字把它講清楚**（→ §5-F1），容易漏改其中一處。

#### 2.1.9 載具／捐贈／統編的互斥：AMEGO **不擋**，必須我方強制【實測】

| 情境 | `f0401` 結果 |
|---|---|
| `CarrierType=3J0002` + 有效手機條碼 + `NPOBAN=275`（同時給） | **通過** ⇒ AMEGO 不擋 |
| 打統編 `45444444` + `NPOBAN=275` | **通過** ⇒ AMEGO 不擋 |
| 打統編 `45444444` + 手機條碼載具 | **通過** ⇒ AMEGO 不擋 |

而 `f0401_custom`（API 配號）的錯誤清單**明列**三條互斥規則：
> 第n筆 列印發票不可設定載具 ／ 第n筆 列印發票不可設定捐贈碼 ／ 第n筆 捐贈發票不可設定載具

⇒ **「打統編 / 載具 / 捐贈」三選一的互斥必須由我方在後端強制**，不能指望 AMEGO 回錯誤碼。**這是最容易踩的坑。**

#### 2.1.10 買方欄位的實測值域

**`BuyerIdentifier`**【實測】：

| 傳入值 | 結果 |
|---|---|
| `0000000000`（10 個零） | 通過 ← 「不打統編」的唯一寫法 |
| `45444444`（合法統編） | 通過 |
| `00000000`（8 個零） | `3040122 BuyerIdentifier 格式錯誤` |
| `12345678`（**測試統編本身，檢查碼不合法**） | `3040122 BuyerIdentifier 格式錯誤` ⚠️ |
| `0000000001` / `1234567890` | `3040122` |
| `1234567`（7 碼）／`""`／`00000000000`（11 碼） | `3040121 BuyerIdentifier 字數錯誤` |

⇒ **只接受兩種：`0000000000` 或通過檢查碼的 8 碼統編。AMEGO 有做統編檢查碼驗證。**
⇒ **不可對 8 碼統編做左補零**——那會變成 10 碼而被 3040122 拒絕。

**`BuyerName`**【實測】：`""` → 3040123；`"0000"` → **通過**（文件明寫「不可填0、00、000及0000」但**實測不擋**，要我方自己擋）；60 中文字通過、61 字 → 3040123。

### 2.2 開立發票（API 配號）— `POST /json/f0401_custom`

- 舊路徑 `/json/c0401_custom`；來源檔 `c0401_custom.php`；group `自行配號專用` / name `Invoice_Custom`
- 【文件】說明：需自行指定發票號碼、發票日期時間及隨機碼。 上傳發票資訊後，會回傳是否成功，若有傳入熱感應機型號代碼，則會回傳的 base64_data 列印格式字串數值。 **若有傳入熱感應機型號代碼，一次只限制上傳一張。**
- `data` 形狀：**陣列**

> **選型建議：除非有「必須自己控發票號碼」的硬需求，一律選 `f0401` 自動配號。** 兩個理由：
> ① `f0401_custom` **全部 50+ 種錯誤都回 code `99`**，只能解析中文 msg 分流（§3.20）。
> ② 「順時順號」原則由 AMEGO 保證 vs 我方自己保證（§4.5.5）。

#### 2.2.1 與 `f0401` 的差異

`f0401_custom` 的欄位集合是 `f0401` 的**超集**，多出：
`InvoiceNumber` / `InvoiceDate` / `InvoiceTime` / `RandomNumber` / `PrintMark` / `SellerPersonInCharge` /
`ExchangeRate` / `Currency` / `GroupMark` / `BondedAreaConfirm` / `ProductItem.RelateNumber`。

**且有四處不相容，兩支的 request DTO 不可共用**：
1. `OrderId`（大寫、**必填**）→ 在這支叫 **`order_id`（小寫底線、optional）**
2. `data` 收**陣列**（`f0401` 收單一物件）
3. `ProductItem.TaxType` 在這支是 **optional（預設應稅）**，`f0401` 是**必填**
4. `PrinterType` / `PrinterLang` 的值域說明不同（→ §6-9、§6-10）

#### 2.2.2 專屬 Request 參數（僅列與 `f0401` 不同者）

| 欄位名 | 型別 | 必填 | 說明【文件逐字】 |
|---|---|---|---|
| `InvoiceNumber` | String | **是** | 發票號碼 |
| `InvoiceDate` | String | **是** | 發票日期，格式：YYYYMMDD，例如：20161028 |
| `InvoiceTime` | String | **是** | 發票時間，格式：hh:mm:ss，例如：16:40:42 |
| `SellerPersonInCharge` | String | 否 | 賣方負責人，不可超過30字 |
| `PrintMark` | String | **是** | 列印註記，Y:列印 N:不列印 |
| `RandomNumber` | String | **是** | 隨機碼 |
| `ProductItem.RelateNumber` | String | 否 | 相關號碼，不可超過50字 |
| `ProductItem.TaxType` | Number | **否** | 課稅別，預設為應稅，1：應稅　2：零稅率　3：免稅 |
| `ExchangeRate` | Number | 否 | 匯率，小數精準度到3位數 |
| `Currency` | String | 否 | 幣別，有若值應為3字，請參看 電子發票資料交換標準訊息建置指引 v4.1 的 CurrencyCodeEnum 資料元規格 |
| `GroupMark` | String | 否 | 彙開註記，*字或空值 |
| `BondedAreaConfirm` | Number | 否 | 買受人簽署適用零稅率註記，若為零稅率發票，此欄位為可填。（1–4，全文見 §3.11） |
| `order_id` | String | 否 | 訂單編號，不可重複，不可超過40字 |
| `PrinterType` | Number | 否 | 熱感應機型號代碼 1：Star mC-Print3 2：芯燁XP-Q90EC |
| `PrinterLang` | Number | 否 | 熱感應機編碼 1：BIG5 2：GBK Star mC-Print3 僅支援 BIG5(預設) 芯燁XP-Q90EC 支援 BIG5、GBK(預設) |

#### 2.2.3 Response 欄位（Success 200）— 結構與 `f0401` 不同

| 欄位名 | 型別 | 必回 | 說明【文件逐字】 |
|---|---|:--:|---|
| `code` | Number | ✅ | 0 代表正確，其他代碼代表錯誤 |
| `msg` | String | ✅ | 錯誤訊息 |
| `data` | Object[] | ❌ | 發票號碼的資料 |
| `data.invoice_number` | String | ❌ | 發票號碼 |
| `data.barcode` | String | ❌ | 電子發票的條碼內容 |
| `data.qrcode_left` | String | ❌ | 電子發票的左側 QRCODE 內容 0元發票會回傳空字串 |
| `data.qrcode_right` | String | ❌ | 電子發票的右側 QRCODE 內容 0元發票會回傳空字串 |
| `data.base64_data` | String | ❌ | 列印字串。預設空字串，若需列印且有指定熱感應機型號代碼才會有數值 PrinterType = 1，base64編碼的 XML 列印格式字串(mC-Print3 熱感應機專用)…PrinterType = 2，base64編碼的 ESC/POS 列印格式字串(芯燁XP-Q90EC 熱感應機專用)…0元發票不會回傳此欄位 |

#### 2.2.4 官方範例（原樣抄錄）

Request（打統編、需列印）：
```json
[
  {
    "InvoiceNumber": "MQ00000004",
    "InvoiceDate": "20220426",
    "InvoiceTime": "21:35:01",
    "BuyerIdentifier": "45444444",
    "BuyerName": "光貿科技有限公司",
    "BuyerAddress": "", "BuyerTelephoneNumber": "", "BuyerEmailAddress": "",
    "MainRemark": "", "CarrierType": "", "CarrierId1": "", "CarrierId2": "",
    "PrintMark": "Y", "NPOBAN": "", "RandomNumber": "6713", "PrinterType": "2",
    "ProductItem": [
      { "Description": "測試商品", "Quantity": "1", "UnitPrice": "699", "Amount": "699", "SequenceNumber": "1", "Remark": "" }
    ],
    "SalesAmount": "666", "FreeTaxSalesAmount": "0", "ZeroTaxSalesAmount": "0",
    "TaxType": "1", "TaxRate": "0.05", "TaxAmount": "33", "TotalAmount": "699",
    "order_id": "1122042600005"
  }
]
```
> ⚠️ 注意範例出現了參數表**沒有**的 `SequenceNumber` 欄位。

Response（回應成功）：
```json
{
	"code": 0, "msg": "",
	"data": [{
		"invoice_number": "MQ00000004",
		"barcode": "11104MQ000000046713",
		"qrcode_left": "MQ00000004111042667130000029a000002bb2808062312345678HgZwSAdJ5EbTjUOiKSmBDg==:**********:1:1:0:測試商品:1:699",
		"qrcode_right": "**",
		"base64_data": "G0AbQ............."
	}]
}
```

#### 2.2.5 兩種配號模式的差異【文件逐字，S5】

> 自動配號：API 傳送發票明細給本系統，系統回傳 發票號碼、發票日期、隨機碼...等資訊
> API 配號：API 傳送發票明細、發票號碼、發票日期、隨機碼...等資訊給本系統
> 主要差異在 發票號碼 跟 發票日期 由哪方指定

> 自動配號 開立流程：首先要在後台的 發票作業 > 發票字軌列表，將字軌類型設定為[自動配號]。
> API 配號 開立流程：首先要在後台的 發票作業 > 發票字軌列表，將字軌類型設定為[API 配號]。
> 流程：API 字軌取號 → 取得字軌起訖 → API 開立發票含發票號碼 → 回傳列印資訊 → 列印發票給消費者

⇒ **兩種模式互斥，且由 AMEGO 後台的「字軌類型」設定決定，不是 API 參數。**

💡 API 配號模式下，「這個號碼已經開過了」的訊號是 **code 99 + msg「指定發票號碼 已存在開立」**，
這其實**天然具備冪等語意**（同一號碼重送不會產生第二張），但因 code 撞在一起，**必須做 msg 字串比對**。

### 2.3 作廢發票 — `POST /json/f0501`

- 舊路徑 `/json/c0501`；來源檔 `c0501.php`；name `Invoice_Cancel`
- 【文件】說明：作廢已開立的發票
- `data` 形狀：**陣列**

#### 2.3.1 Request 參數

| 欄位名 | 型別 | 必填 | 長度 | 說明 |
|---|---|---|---|---|
| （根） | Object[] | **是** | — | 物件陣列 |
| `array.CancelInvoiceNumber` | String | **是** | 10（發票號碼） | 發票號碼 |
| `CancelReason` | String | 否 | **≤20 字** | **文件參數表未列**，但錯誤碼 `3050113` 與【實測】皆證實存在 |

**【實測】`CancelReason` 確實被驗證**：
```
送 21 字 → {"code":3050113,"msg":"第1筆 CancelReason 超過 20 字"}
送 20 字 → {"code":3050125,"msg":"ZZ00000000 發票不存在"}   ← 通過長度檢查，往下走到存在性檢查
```
⇒ **我方 DTO 應該把它加進去**（作廢原因對帳很有用）。

官方 Request 範例（原樣）：
```json
[{ "CancelInvoiceNumber": "AB00001111" }]
```
```json
[{"CancelInvoiceNumber":"AB00001111"},{"CancelInvoiceNumber":"AB00001112"},{"CancelInvoiceNumber":"AB00001113"}]
```

#### 2.3.2 Response

只有 `code` / `msg`。**官方沒有給作廢成功的回應範例**，也**沒有逐筆結果欄位**。
【實測】作廢成功回 `{"code":0,"msg":""}`——**不回傳被作廢的號碼**。

**一次送多張時，錯誤是「整批第 n 筆」形式的單一錯誤**（msg 前綴「第1筆」或直接帶單號），
**無法從回應得知部分成功／部分失敗** → §5-A8。⇒ **建議一次只送一張**，讓失敗歸屬明確。

#### 2.3.3 錯誤代碼【文件逐字，S4】

| code | 描述 |
|---|---|
| 3050111 | 第n筆 CancelInvoiceNumber 錯誤 |
| 3050112 | 此 API 支援傳輸多張發票，data 欄位資料應為陣列字串 |
| 3050113 | 第n筆 CancelReason 超過 20 字 |
| 3050121 | 指定發票號碼 開立中 |
| 3050122 | 指定發票號碼 已存在開立作廢 |
| 3050123 | 指定發票號碼 已存在註銷 |
| 3050124 | 指定發票號碼 發票類型錯誤 |
| 3050125 | 指定發票號碼 發票不存在 |
| 3050126 | 指定發票號碼 **已超過修改期限** |
| 3050131 | 指定發票號碼 等待 開立/作廢/註銷 |
| 3050141 | 指定發票號碼 **已存在折讓單** |

【實測】驗證順序：陣列型別(3050112) → `CancelReason` 長度(3050113) → 發票存在性(3050125)。

**關鍵業務約束**：
- `3050141` — **發票只要已開過折讓單，就不能再作廢**（要先作廢折讓單）。折讓與作廢互斥。
- `3050121` / `3050131` — **發票尚在上傳中就不能作廢**。S13 亦寫明：狀態必須是「完成」才有作廢按鈕 ⇒ 實作必須先查 `invoice_status` 到 99。
- `3050122`「已存在開立作廢」⇒ **這個錯誤碼其實代表「已達成目標」**，重試邏輯應視同成功。

#### 2.3.4 作廢是非同步的【實測】

`f0501` 回 `code:0` 之後：
- `invoice_status` 仍然回 `{"type":"C0401","status":2}`（原本的開立狀態）
- 作廢會排進 `invoice_query` 的 `wait[]` 陣列：`{"invoice_type":"C0501","create_date":...}`
- 立刻重複作廢 → `{"code":3050131,"msg":"ZA10063775 等待作廢"}`
- 作廢不存在的發票 → `{"code":3050125,"msg":"ZZ99999999 發票不存在"}`

⇒ **「作廢 API 回 0」≠「已作廢」。** 要確認作廢完成，得輪詢 `invoice_query` 看 `invoice_type` 變成 `C0501`、或 `wait[]` 清空。
**輪詢多久算超時，文件未載明** → §5-E5。

### 2.4 發票狀態 — `POST /json/invoice_status`

- 來源檔 `invoice_status.php`；name `Invoice_Status`
- 【文件】說明：發票的上傳狀態
- `data` 形狀：**陣列**（送物件回 `code 23`）

#### 2.4.1 Request / Response 欄位

Request：`[{"InvoiceNumber":"AB00001111"}, ...]`

| 欄位名 | 型別 | 必回 | 說明【文件逐字】 |
|---|---|:--:|---|
| `code` / `msg` | Number / String | ✅ | 同通用 |
| `data` | Object | ❌ | 發票號碼的對應上傳狀態 |
| `data.invoice_number` | String | ❌ | 發票號碼 |
| `data.type` | String | ❌ | 發票類型 NOT_FOUND：查無發票 C0401：發票開立 C0501：發票作廢 C0701：發票註銷 TYPE_ERROR：類型錯誤 |
| `data.status` | Number | ❌ | 發票狀態 1：待處理 2：上傳中 3：已上傳 31：處理中 32：處理完成／待確認 91：錯誤 99：完成 |
| `data.total_amount` | Number | ❌ | 發票金額 |

錯誤代碼【文件】：`99` 第n筆 InvoiceNumber 錯誤。

#### 2.4.2 兩種 response 形狀（新版／舊版），文件沒說怎麼切換

**回應成功(新版)** — `data` 是**陣列**【官方原樣】：
```json
{
	"code": 0, "msg": "",
	"data": [
		{ "invoice_number": "AB00001111", "type": "NOT_FOUND", "status": -1, "total_amount": 0 },
		{ "invoice_number": "AB00001112", "type": "C0401", "status": 99, "total_amount": 1580 },
		{ "invoice_number": "AB00001113", "type": "C0401", "status": 99, "total_amount": 199 }
	]
}
```
**回應成功(舊版)** — `data` 是**以發票號碼為 key 的物件**【官方原樣】：
```json
{
	"code": 0, "msg": "",
	"data": {
		"AB00001111": { "type": "NOT_FOUND", "status": -1, "total_amount": 0 },
		"AB00001112": { "type": "C0401", "status": 99, "total_amount": 1580 },
		"AB00001113": { "type": "C0401", "status": 99, "total_amount": 199 }
	}
}
```
【實測 2026-08-24】實際回的是**新版陣列**。
⇒ 反序列化必須容忍兩種（先判 `data` 是 array 還是 object），否則 AMEGO 任何時候切回舊版就 parse 失敗。
⇒ **`status: -1` 未列在欄位表的列舉值裡**（只列 1/2/3/31/32/91/99），但官方範例與實測都出現。enum 必須容納 `-1`。

#### 2.4.3 批次筆數【實測】

- 送 **50 / 300 / 1000 / 3000 筆相異**發票號碼，皆 `code 0` 且回傳筆數 ＝ 送出筆數（3000 筆時 `data` 約 93KB）。
- 但**送重複鍵值會去重**：送 600 筆**重複**號碼只回 **1** 筆；`ban_query` 送 500 筆重複統編也只回 1 筆。
- ⇒ **絕對不可用「陣列索引對位」把回應塞回請求**，必須用 `invoice_number` / `ban` 這類鍵值 join。
- 文件未載明上限，建議自訂 ≤200 筆分批。

#### 2.4.4 最危險的坑：`invoice_status` 對舊發票**靜默回 NOT_FOUND**

用**同一批確實存在**的發票（都能在 `invoice_list` 撈到）去打 `invoice_status`【實測】：

| 發票日期 | 發票號碼 | `invoice_status` 回傳 | `invoice_query` 回傳 | 該發票真的存在？ |
|---|---|---|---|---|
| 20260701 | AB10000000 | `C0401 / 99 / 10` | code 0 | ✅ |
| 20260501 | ML21000050 | `C0501 / 99 / 1980` | code 0 | ✅ |
| 20260401 | AA01205336 | `C0501 / 99 / 500` | code 0 | ✅ |
| 20260301 | XX12000000 | `C0501 / 99 / 69` | code 0 | ✅ |
| **20260226** | AC00000027 | **`NOT_FOUND / -1 / 0`** ❌ | **code 0（查得到！）** | ✅ **存在** |
| **20260220** | VV62013268 | **`NOT_FOUND / -1 / 0`** ❌ | code 51 | ✅ 存在 |
| **20260201** | IT62008006 | **`NOT_FOUND / -1 / 0`** ❌ | code 51 | ✅ 存在 |
| **20251201** | EX10005989 | **`NOT_FOUND / -1 / 0`** ❌ | code 51 | ✅ 存在 |
| **20250101** | QK12345600 | **`NOT_FOUND / -1 / 0`** ❌ | code 51 | ✅ 存在（`invoice_list` 查得到：C0401/99/150 元） |

**這是 false negative：發票明明存在，API 卻回「查無發票」，而且 `code` 是 0（成功）。**

【推定】窗口切點落在 2026-02-26 與 2026-03-01 之間，正好是**發票期別**邊界：
2026-08-24 屬期別 3（07-08月）；`20260301` 屬期別 1（03-04月）✅、`20260226` 屬期別 0（01-02月）❌
⇒ 疑為「**當期 + 前兩期**」的滾動窗口，而非固定 180 天。
**但官方文件對 `invoice_status` 完全沒有提到任何時間窗口**（`invoice_query` 有寫 180 天，`invoice_status` 一個字都沒有）。

⇒ **絕對不可以用 `invoice_status` 回 `NOT_FOUND` 當作「這筆訂單還沒開過發票」的判準**——會導致**重複開立**。
⇒ 冪等對帳一律走 `invoice_query`（超限有明確 code 51，可辨識），或走 `invoice_list` 按日期區間比對。

### 2.5 發票查詢 — `POST /json/invoice_query`

- 來源檔 `invoice_query.php`；name `Invoice_Query`
- 【文件】說明：查詢發票內容

#### 2.5.1 Request 參數

| 欄位名 | 型別 | 必填 | 說明【文件逐字】 |
|---|---|---|---|
| `type` | String | **是** | 查詢類型 order：訂單編號 invoice：發票號碼，擇一查詢 |
| `order_id` | String | **是**※ | 訂單編號，不可超過40字。以發票日期為主，只能查詢180天內的發票 |
| `invoice_number` | String | **是**※ | 發票號碼，不可超過10字。以發票日期為主，只能查詢180天內的發票 |

※ 文件把兩者都標 REQUIRED 但說明寫「**擇一查詢**」，由 `type` 決定用哪一個。
【實測】`type:"order"` 只帶 `order_id`、`type:"invoice"` 只帶 `invoice_number`，兩者皆回 `code:0`。
⇒ **DTO 不可把兩個都設 NOT NULL**；也代表**文件的 optional 旗標不可信** → §6-23。

```json
{ "type": "order", "order_id": "P202212010001" }
{ "type": "invoice", "invoice_number": "AA12301393" }
```

#### 2.5.2 Response 欄位

| 欄位名 | 型別 | 說明【文件逐字】 |
|---|---|---|
| `data.invoice_number` | String | 發票號碼 |
| `data.invoice_type` | String | 發票類型 C0401：B2C 存證發票開立 C0501：B2C 存證發票作廢 C0701：B2C 存證發票註銷 A0401：B2B 存證發票開立 A0501：B2B 存證發票作廢 A0101：B2B 交換發票開立 A0102：B2B 交換發票開立(接收確認) A0201：B2B 交換發票作廢 A0202：B2B 交換發票作廢(接收確認) A0301：B2B 交換發票退回 A0302：B2B 交換發票退回(接收確認) 詳情請參考財政部的電子發票資料交換標準訊息建置指引(MIG)文件 |
| `data.invoice_status` | Number | 上傳至財政部的狀態 1：待處理 2：上傳中 3：已上傳 31：處理中 32：處理完成／待確認 91：錯誤 99：完成 |
| `data.invoice_date` | String | 發票日期 格式：YYYYMMDD，例如：20161028 【實測回 Number】 |
| `data.invoice_time` | String | 發票時間 HH:mm:ss |
| `data.buyer_identifier` | String | 買方統一編號，沒有統一編號會回傳 0000000000 |
| `data.buyer_name` | String | 買方名稱 |
| `data.buyer_zip` | Number | 買方郵遞區號 |
| `data.buyer_address` | String | 買方地址 |
| `data.buyer_telephone_number` | String | 買方電話 |
| `data.buyer_email_address` | String | 買方信箱 |
| `data.sales_amount` | Number | 應稅銷售額合計 |
| `data.free_tax_sales_amount` | Number | 免稅銷售額合計 |
| `data.zero_tax_sales_amount` | Number | 零稅率銷售額合計 |
| `data.tax_type` | Number | 課稅別　1：應稅　2：零稅率　3：免稅　4：應稅(特種稅率)　9：混合應稅與免稅或零稅率(限訊息C0401使用) |
| `data.tax_rate` | String | 稅率，為5%時本欄位值為0.05 |
| `data.tax_amount` | Number | 營業稅額 |
| `data.total_amount` | Number | 總計 |
| `data.print_mark` | String | 列印註記 |
| `data.random_number` | String | 隨機碼 |
| `data.main_remark` | String | 總備註 |
| `data.customs_clearance_mark` | String | 通關方式註記，若為非零稅率發票，則此欄位為 0 。 1:非經海關出口 2:經海關出口 |
| `data.carrier_type` | String | 載具類別，手機條碼 3J0002，自然人憑證條碼 CQ0001 光貿會員載具 amego |
| `data.carrier_id1` | String | 載具**明碼**（⚠️ 開立時叫「顯碼」） |
| `data.carrier_id2` | String | 載具隱碼 |
| `data.npoban` | String | 捐贈碼 |
| `data.cancel_date` | Number | 作廢時間，Unix 時間戳記 |
| `data.invoice_lottery` | Number | 獎項代碼，請參考 [獎項定義] API （實際值域對不上，見 §3.25） |
| `data.order_id` | String | 訂單編號 |
| `data.detail_vat` | Number | 明細的單價及小計為 0:未稅價 1:含稅價 |
| `data.detail_amount_round` | Number | 明細的小計 0:直接加總 1:先四捨五入再加總 |
| `data.create_date` | Number | 建立時間，Unix 時間戳記 |
| `data.product_item` | Object[] | 商品陣列，最多 9999 筆 |
| `data.product_item.tax_type` | Number | 課稅別　1：應稅　2：零稅率　3：免稅 |
| `data.product_item.description` | String | 品名，不可超過256字 |
| `data.product_item.unit_price` | Number | 單價，可到小數7位數 |
| `data.product_item.quantity` | Number | 數量，可到小數7位數 |
| `data.product_item.unit` | String | 單位 |
| `data.product_item.amount` | Number | 小計，可到小數7位數 |
| `data.product_item.remark` | String | 備註，**不可超過40字** 與開立端的 120 字矛盾 → §6-1 |
| `data.wait` | Object[] | 未處理的排程陣列，例如：等待改成「發票作廢」 |
| `data.wait.invoice_type` | String | 改成發票類型（同上 C0401… 全表） |
| `data.wait.create_date` | Number | 建立時間，Unix 時間戳記 |
| `data.allowance` | Object[] | 折讓單陣列(2024.10.24 新增) |
| `data.allowance.invoice_type` | **Number** | 折讓單類型 D0401：B2C 存證折讓單開立 D0501：B2C 存證折讓單作廢 B0401：B2B 存證折讓單開立 B0501：B2B 存證折讓單作廢 B0101：B2B 交換折讓單開立 B0102：B2B 交換折讓單開立(接收確認) B0201：B2B 交換折讓單作廢 B0202：B2B 交換折讓單作廢(接收確認) **型別標 Number 但值是字串** → §6-4 |
| `data.allowance.invoice_status` | Number | 上傳至財政部的狀態（同上） |
| `data.allowance.allowance_type` | String | 折讓單種類 1. 買方開立(作廢)折讓證明單 2. 賣方開立(作廢)折讓證明通知單 |
| `data.allowance.allowance_number` | String | 折讓單編號 |
| `data.allowance.allowance_date` | String | 折讓單日期 格式：YYYYMMDD |
| `data.allowance.tax_amount` | Number | 營業稅額 |
| `data.allowance.total_amount` | Number | **未稅總計** |

#### 2.5.3 官方 Response 範例（原樣）

```json
{
	"code": 0, "msg": "",
	"data": {
		"invoice_number": "AA12301393", "invoice_type": "C0401", "invoice_status": 31,
		"invoice_date": "20221201", "invoice_time": "09:56:38",
		"buyer_identifier": "0000000000", "buyer_name": "客人", "buyer_zip": 0,
		"buyer_address": "", "buyer_telephone_number": "", "buyer_email_address": "",
		"sales_amount": 100, "free_tax_sales_amount": 0, "zero_tax_sales_amount": 0,
		"tax_type": 1, "tax_rate": "0.05", "tax_amount": 0, "total_amount": 100,
		"print_mark": "Y", "random_number": "2729", "main_remark": "",
		"customs_clearance_mark": 0,
		"carrier_type": "", "carrier_id1": "", "carrier_id2": "", "npoban": "",
		"cancel_date": 0, "invoice_lottery": 0, "order_id": "P202212010001",
		"detail_vat": 1, "detail_amount_round": 0, "create_date": 1669859798,
		"product_item": [
			{ "tax_type": 1, "description": "紅利儲值", "unit_price": 100, "quantity": 1, "unit": "", "amount": 100, "remark": "" }
		],
		"wait": [ { "invoice_type": "C0501", "create_date": 1670486981 } ]
	}
}
```
```json
{ "code": 51, "msg": "該發票超過查詢期限" }
```
```json
{ "code": 71, "msg": "查無資料" }
```

⚠️ **空載具／空捐贈是空字串不是 null。**

#### 2.5.4 錯誤代碼【文件逐字，S4】

| code | 描述 |
|---|---|
| 31 | type 查詢類型不存在 |
| 32 | order_id 不可為空或過長 |
| 33 | invoice_number 不可為空或格式錯誤 |
| 34 | type 查詢類型錯誤 |
| 51 | 該發票超過查詢期限 |
| 71 | 查無資料 |

#### 2.5.5 180 天窗口的邊界【實測】

實測日 2026-08-24：

| 發票日期 | 距今 | 結果 |
|---|---|---|
| 20260701 / 20260501 / 20260401 / 20260301 | — | `code 0` ✅ |
| **20260226** | 179 天 | **`code 0` ✅**（剛好在窗內） |
| **20260225** | **180 天** | **`code 0` ✅**（另一輪實測，含第 180 天） |
| **20260224** | **181 天** | **`code 51` ❌** |
| 20260220 / 20260201 / 20251201 | 185 天以上 | `code 51` ❌ |

⇒ **邊界為「發票日 ≥ 今日−180 天」，含第 180 天**；超限時是**明確可辨識的 `code 51`，不是靜默查無**。
⇒ 這點使 `invoice_query` 成為冪等對帳的**唯一安全查詢**。
⇒ **我方 DB 必須保存完整發票內容，不能靠 AMEGO 當長期存證來源。**

#### 2.5.6 型別陷阱【實測】

- `data.invoice_date` 文件標 **String**（「格式：YYYYMMDD」），**實測回 Number** `20260824`。`invoice_list` 同樣。
  ⇒ Java/Go 用強型別 DTO 直接對映 String 會**反序列化失敗**。建議收 Number 再自行轉字串。
- `data.allowance[].invoice_type` 文件標 Number 但值是 `D0401` 字串 ⇒ 用 String 反序列化。

### 2.6 發票列表 — `POST /json/invoice_list`

- 來源檔 `invoice_list.php`；name `Invoice_List`；2024.11.13 新增
- 【文件】說明：發票的主檔資料

#### 2.6.1 Request 參數

| 欄位名 | 型別 | 必填 | 說明【文件逐字】 |
|---|---|---|---|
| `date_select` | Number | **是** | 日期條件 1:發票日期 2:建立日期 |
| `date_start` | String | **是** | 開始日期，格式：YYYYMMDD，例如：20240901 |
| `date_end` | String | **是** | 結束日期，格式：YYYYMMDD，例如：20241031 |
| `limit` | Number | 否 | 每頁顯示資料筆數 20~500，預設 20 筆 |
| `page` | Number | 否 | 目前頁數，預設第1頁 |

```json
{ "date_select": 1, "date_start": 20230101, "date_end": 20230228, "limit": 20, "page": 1 }
```
> ⚠️ 文件說 `date_start`/`date_end` 是 String，但**官方自己的範例送的是 Number（無引號）**，實測兩者皆可。

#### 2.6.2 Response

頂層：`code` / `msg` / `page_total` / `page_now` / `data_total` / `data`（Object[]）。

`data[]` 欄位與 `invoice_query` 幾乎相同，**差異如下（不能共用同一個 DTO）**：

| 差異 | `invoice_query` | `invoice_list` |
|---|---|---|
| `product_item` 明細 | **有** | **無**（只有主檔） |
| `wait` 未處理排程 | **有** | **無** |
| `allowance` 折讓陣列 | **有** | **無** |
| `zero_tax_rate_reason` 零稅率原因 | **無** | **有** |
| `detail_vat` / `detail_amount_round` | **有** | **無** |

官方 Response 範例（節錄，原樣）：
```json
{
	"code": 0, "msg": "", "page_total": 187, "page_now": 1, "data_total": 3738,
	"data": [
		{
			"invoice_number": "AB85223400", "invoice_type": "C0401", "invoice_status": 99,
			"invoice_date": 20230101, "invoice_time": "17:35:47",
			"buyer_identifier": "0000000000", "buyer_name": "客人", "buyer_zip": 0,
			"buyer_address": "", "buyer_telephone_number": "", "buyer_email_address": "",
			"sales_amount": 0, "free_tax_sales_amount": 130, "zero_tax_sales_amount": 0,
			"tax_type": 3, "tax_rate": "0.05", "tax_amount": 0, "total_amount": 130,
			"print_mark": "Y", "random_number": "1060", "main_remark": "",
			"customs_clearance_mark": 0, "zero_tax_rate_reason": 0,
			"carrier_type": "", "carrier_id1": "", "carrier_id2": "", "npoban": "",
			"cancel_date": 0, "invoice_lottery": 87,
			"order_id": "28-sam2023010101002-B", "create_date": 1672565747
		},
		......以下省略
	]
}
```
> ⚠️ 注意範例中 `invoice_date` 是 **Number**（`20230101`），與文件的 String 標示不符。

#### 2.6.3 錯誤代碼【文件逐字，S4】

| code | 描述 |
|---|---|
| 31 | date_select 參數錯誤 |
| 32 | date_start 不是 YYYYMMDD 格式 |
| 33 | date_end 不是 YYYYMMDD 格式 |
| 34 | limit 只能帶入 20 ~ 500 |
| 35 | page 必須 >= 1 |
| 36 | date_start 不可大於 date_end |

#### 2.6.4 實測的邊界與陷阱

| 項目 | 實測結果 |
|---|---|
| `limit` 邊界 | `limit=19` → `code:34`；`limit=501` → `code:34`。**上下界都嚴格檢查** |
| `date_start > date_end` | `code:36` ✅ 符合文件 |
| **page 超界** | `page=99999`（超出 `page_total=3431`）→ **`code:0`** + `page_now:99999` + **空 `data` 陣列**，**不報錯**。⇒ **迴圈終止條件必須用 `data` 為空 或 `page > page_total`，不可依賴錯誤碼** |
| **日期格式錯誤** | `date_start="2026-08-01"`（帶連字號）→ **`code:0`**，回 `data_total:330377`（＝**全部歷史**）；`date_start=""` 同樣回全部歷史。**文件宣稱的 `code:32` 從未觸發**；只有純字母 `"abcdefgh"` 才回 `code:36`（而且是 36 不是 32）。⇒ **日期格式必須由我方嚴格驗證**，否則一次拉回三十萬筆 |
| **180 天限制** | **不受限**。查 `20200101~20260824`（6.6 年）回 `data_total: 330376`、`code:0`；查 `20250101~20250105`（約 600 天前）回 `data_total: 863` |

⇒ **`invoice_list` 是唯一能查到超過 180 天發票主檔的管道**，是長期對帳／稽核的必要工具（但沒有明細與折讓資訊）。

### 2.7 發票檔案（PDF）— `POST /json/invoice_file`

- 來源檔 `invoice_file.php`；name `Invoice_File`
- 【文件】說明：下載發票檔案(PDF格式)。**載具發票中獎後才可下載，非載具發票可無限次下載。**

#### 2.7.1 Request 參數

| 欄位名 | 型別 | 必填 | 說明【文件逐字】 |
|---|---|---|---|
| `type` | String | **是** | 查詢類型 order：訂單編號 invoice：發票號碼，擇一查詢 |
| `order_id` | String | **是**※ | 訂單編號，不可超過40字。以發票日期為主，只能查詢180天內的發票 |
| `invoice_number` | String | **是**※ | 發票號碼，不可超過10字。以發票日期為主，只能查詢180天內的發票 |
| `download_style` | Number | 否 | 下載樣式，預設為 0（值域見 §3.21） |

【文件逐字】`download_style`：
> 有打統編發票，四種樣式
> 0：A4整張 1：A4(地址+A5) 2：A4(A5x2) 3：A5 5：QRcode_A4
> 沒有打統編的發票，一種樣式
> 0：A4整張(背面兌獎聯，需雙面列印)

⚠️ 原文寫「四種」但實列**五項**，且**沒有 `4`** → §6-17。
【實測】傳 `download_style: 9`（未定義值）→ **API 仍回 `code:0`**，產生 `...&type=9` 的 URL，GET 之仍回 PDF。
**AMEGO 對 `download_style` 沒有做值域驗證**（對照：`allowance_file` **有** 錯誤碼 34「download_style 參數錯誤」，`invoice_file` **沒有**）。
⇒ **我方必須自行以白名單 `{0,1,2,3,5}` 驗證**，否則錯誤值會靜默產生非預期版面的 PDF。

#### 2.7.2 Response

| 欄位名 | 型別 | 說明【文件逐字】 |
|---|---|---|
| `data.file_url` | String | 檔案連結，**連結僅10分鐘有效** |

```json
{ "code": 0, "msg": "", "data": { "file_url": "https://invoice.amego.tw/user/invoice_print_type?token=1679038020_d22dd1ac994c3cdf4fab428b5cc95fa6&type=0" } }
```
```json
{ "code": 51, "msg": "該發票超過查詢期限" }
{ "code": 71, "msg": "查無資料" }
```

#### 2.7.3 錯誤代碼【文件逐字，S4】

| code | 描述 |
|---|---|
| 31 | type 查詢類型不存在 |
| 32 | order_id 不可為空或過長 |
| 33 | invoice_number 不可為空或格式錯誤 |
| 34 | type 查詢類型錯誤 |
| 51 | 該發票超過查詢期限 |
| 52 | 該發票有等待異動的排程 |
| **53** | **載具發票，中獎後才可以下載** |
| 55 | 該發票不符合下載條件 |
| **53** | **該發票類型不可下載** |
| 71 | 查無資料 |

⚠️ **`53` 在官方表中出現兩次、語意不同**（`55` 在列印表甚至出現三次）。**不可只靠 code 分支，必須同時記錄／解析 `msg`。**

#### 2.7.4 端到端實測

```text
# 非載具發票 AB10000316（invoice_date 20260730, carrier_type ""）
POST /json/invoice_file  {"type":"invoice","invoice_number":"AB10000316","download_style":0}
-> {"code":0,"msg":"","data":{"file_url":"https://invoice.amego.tw/user/invoice_print_type?token=1787578007_76548906549bcf443e288b3e7a18a8eb&type=0"}}
GET file_url -> HTTP 200  Content-Type: application/pdf  323,646 bytes  magic=b'%PDF-'

# 載具發票 AB10000312（carrier_type "3J0002"，invoice_lottery 0 未中獎）
-> {"code":53,"msg":"載具發票，中獎後才可以下載"}

# 超過 180 天（invoice_date 20250101）
-> {"code":51,"msg":"該發票超過查詢期限"}
```

⇒ **開了載具發票之後，我方拿不到 PDF。** 「下載發票 PDF」只能對**非載具發票**開放，
UI 必須依 `carrier_type` 是否為空決定按鈕可用性；`code:53` 是**正常業務分支不是錯誤**，要有對應文案。

#### 2.7.5 `file_url` token 的實測行為（文件完全未載明）

| 觀察 | 結果 |
|---|---|
| 是否需登入 / cookie | **不需要**。無 auth header、無 cookie 即可下載 ⇒ **token 是 bearer 憑證，等同機敏資料** |
| token 格式 | `{unix_timestamp}_{32位hex}`，例 `1787578007_76548906549bcf443e288b3e7a18a8eb` |
| 有效期 | 文件寫「僅10分鐘有效」；**本次未等待 10 分鐘驗證，屬未坐實** |
| 可否重複使用 | **可以**。同一 token 連續 GET 兩次皆回 200 PDF |
| 竄改 token | 回 **HTTP 200** 但 body 為純文字 `此查詢已過期，請重新查詢！`（39 bytes，`Content-Type: text/html`） |
| URL 的 `type` 參數 | **可由客戶端任意改** —— 同一 token 改 `type=0/1/2/3/5/9` 皆回不同大小的 PDF |

⇒ **三條實作鐵則**：
1. **失敗時不是 4xx/5xx，而是 HTTP 200 + 中文純文字。** 下載端必須檢查 `Content-Type == application/pdf` 或 magic bytes `%PDF-`，
   只看 HTTP status 會把錯誤頁當 PDF 存進 storage。
2. token 無需認證即可下載他人發票 PDF ⇒ **絕對不可寫入 log、不可回傳前端當長期連結、不可存進 DB**。
   正確做法：後端即時取得後轉存我方 storage，或以我方簽章 URL 代理。
3. 10 分鐘有效期 ⇒ **不能把 `file_url` 存成 DB 欄位**。DB 只存 `invoice_number` / `order_id`，每次要下載時即時呼叫。

### 2.8 發票列印（熱感應機）— `POST /json/invoice_print`

- 來源檔 `invoice_print.php`；name `Invoice_Print`
- 【文件】說明：產出列印格式字串。 **0元發票無法產生，熱感應紙上的 QRcode，所以0元發票無法列印發票正本及補印。** 機型支援功能表

| 欄位名 | 型別 | 必填 | 說明【文件逐字】 |
|---|---|---|---|
| `type` | String | **是** | 查詢類型 order：訂單編號 invoice：發票號碼，擇一查詢 |
| `order_id` | String | **是**※ | 訂單編號，不可超過40字。以發票日期為主，只能查詢180天內的發票 |
| `invoice_number` | String | **是**※ | 發票號碼，不可超過10字。以發票日期為主，只能查詢180天內的發票 |
| `printer_type` | Number | **是** | 熱感應機型號代碼 可支援功能請參看機型支援功能表 |
| `printer_lang` | Number | 否 | 熱感應機編碼 1：BIG5 2：GBK 3：UTF-8 若不帶入此參數，會使用預設編碼 |
| `print_invoice_type` | Number | **是** | 列印格式 1：發票正本 2：發票補印 3：單印明細(僅限制Xprinter 芯燁通用以後的機型B2C發票可使用) |
| `print_invoice_detail` | Number | 否 | 列印 1.發票正本 或 2.發票補印 這兩種格式，可以設定是否列印明細 0:不列印 1:印出明細(裁切) (預設) 2:印出明細(不裁切) 打統編一律列印明細 此參數僅限制Xprinter 芯燁通用以後的機型B2C發票可使用 |

Response：`data.base64_data`（String）。

錯誤代碼【文件逐字，S4】：

| code | 描述 |
|---|---|
| 31 | type 查詢類型不存在 |
| 32 | order_id 不可為空或過長 |
| 33 | invoice_number 不可為空或格式錯誤 |
| 34 | type 查詢類型錯誤 |
| 36 | Star mC-Print3 不支援單印明細 |
| 35 | printer_type 熱感應機型號代碼錯誤 |
| 51 | 該發票超過查詢期限 |
| 56 | 0元發票，無法列印發票正本及發票補印 |
| 52 | 該發票有等待異動的排程 |
| 53 | 載具發票，中獎後才可以列印 |
| 55 | 該發票不符合列印條件 |
| 53 | 該發票類型不可列印 |
| 55 | 該發票不符合列印條件 |
| 55 | 該發票不符合列印條件 |
| 53 | 該發票類型不可列印 |
| 71 | 查無資料 |
| 72 | 取得發票列印格式錯誤 |

**`53` 出現 2 次、`55` 出現 3 次，官方表格就是這樣重複列出。只有 `msg` 能區分。**

### 2.9 開立折讓 — `POST /json/g0401`

- 舊路徑 `/json/d0401`；name `Allowance` / group `折讓`
- 【文件】說明：折讓已開立的發票
- 正式名稱：**銷貨退回進貨退出或折讓證明單**（賣方開立者為「賣方折讓證明通知單」）
- `data` 形狀：**陣列**（一次可送多張折讓單）

> **文件內部矛盾**：`g0401` 在 bundle 中出現**兩次**（來源檔 `d0401.php` 與 `d0401_custom.php`），
> group／name／version 完全相同，渲染後只顯示一條。兩者差異有二 → §6-2：
> ① `AllowanceNumber` 長度：`d0401.php` 版「不可超過**16**字」 vs `d0401_custom.php` 版「不可超過**40**字」
> ② `AllowanceType`：`d0401.php` 版把「1:買方開立折讓證明單」加了**刪除線**並註明 114 年新制，`d0401_custom.php` 版沒有
> ⇒ **DB 欄位長度取保守值 16**（錯誤碼 4040121 與實測皆為 16）；`AllowanceType` **一律用 2**。

#### 2.9.1 Request 參數（逐欄抄錄，含巢狀）

| 欄位名 | 型別 | 必填 | 長度 | 說明【文件逐字】 |
|---|---|---|---|---|
| `AllowanceNumber` | String | **是** | **≤16**（實測 17 字被拒） | 折讓單編號，不可重複，不可超過16字 |
| `AllowanceDate` | String | **是** | 8（`Ymd`） | 折讓單日期，Ymd |
| `AllowanceType` | Number | **是** | 1 | 折讓單種類，~~1:買方開立折讓證明單~~（原文刪除線） 2:賣方折讓證明通知單。**自114年1月1日起，經雙方合意之退回或折讓，賣方營業人應開立電子發票折讓單依限據實上傳** |
| `BuyerIdentifier` | String | **是** | 8 或 10 | 買方統一編號，買方統一編號，沒有則填入 `0000000000` |
| `BuyerName` | String | **是** | 未載明 | 買方名稱 |
| `BuyerAddress` | String | 否 | 未載明 | 買方地址 |
| `BuyerTelephoneNumber` | String | 否 | 未載明 | 買方電話 |
| `BuyerEmailAddress` | String | 否 | 未載明 | 買方電子信箱（**是否觸發寄信，文件未說** → §5-F8） |
| `ProductItem` | Object[] | **是** | ≤9999 筆 | 商品陣列，最多 9999 筆 |
| `ProductItem.OriginalInvoiceNumber` | String | **是** | 10 | 原發票號碼 |
| `ProductItem.OriginalInvoiceDate` | Number | **是** | 8（`Ymd`） | 原發票日期，Ymd |
| `ProductItem.OriginalDescription` | String | **是** | **≤256** | 原品名，不可超過256字 |
| `ProductItem.Quantity` | Number | **是** | 未載明（查詢端標小數 7 位） | 數量 |
| `ProductItem.UnitPrice` | Number | **是** | 同上 | **單價(不含稅)** |
| `ProductItem.Amount` | Number | **是** | 同上 | **小計(不含稅)** |
| `ProductItem.Tax` | Number | **是** | **整數**（4040139） | 稅金 |
| `ProductItem.TaxType` | Number | **是** | 1 | 課稅別　1：應稅　2：零稅率　3：免稅 |
| `TaxAmount` | Number | **是** | 整數 | 營業稅額 |
| `TotalAmount` | Number | **是** | — | **金額合計(不含稅)** |

**與開立發票的最大落差：折讓的單價／小計固定是「未稅」**，而 `f0401` 的 `UnitPrice`/`Amount` 預設含稅（`DetailVat` 預設 1）。
S14 逐字佐證：「注意折讓單單價固定以未稅輸入，若您是要折讓100(含稅)，則單價需填入未稅的 $95」。
⇒ 從訂單金額推導折讓明細時**必須做含稅→未稅換算**，不能直接沿用開立時的數字。**兩套 DTO 不可共用金額欄位語意。**

**`AllowanceType` 目前只能是 `2`**【實測】：
```
AllowanceType=1 → {"code":"4040145","msg":"第1筆 自114年1月1日起，AllowanceType 不可為 1 買方開立折讓證明單"}
AllowanceType=3 → {"code":"4040123","msg":"第1筆 AllowanceType 錯誤"}
```
（`4040145` **不在官方錯誤碼表內** —— 本次 curl 重驗 mid=71 確認 → §6-20。）

**折讓沒有載具欄位** —— `g0401` 的參數表**不含** `CarrierType` / `CarrierId1` / `CarrierId2` / `NPOBAN`。
折讓單靠 `OriginalInvoiceNumber` 綁回原發票，載具資訊沿用原發票。

#### 2.9.2 官方 Request 範例（原樣抄錄，含註解）

```json
// 原開立兩台超聲波清洗機，未稅單價是 2180元
[
	{
		"AllowanceNumber": "3821061800001",
		"AllowanceDate": "20210618",
		"AllowanceType": "2",
		"BuyerIdentifier": "0000000000",
		"BuyerName": "蕭XX",
		"BuyerAddress": "",
		"BuyerTelephoneNumber": "",
		"BuyerEmailAddress": "",
		"ProductItem": [{
			"OriginalInvoiceDate": 20210520,
			"OriginalInvoiceNumber": "NW93016392",
			"OriginalDescription": "超聲波清洗機",
			"Quantity": 2,
			"UnitPrice": "2180",
			"Amount": "4360",
			"Tax": 218,
			"TaxType": 1
		}],
		"TaxAmount": "218",
		"TotalAmount": "4360"
	}
]
```
> ⚠️ 官方範例本身**混用字串與數字**（`Quantity` 數字、`UnitPrice` 字串、`OriginalInvoiceDate` 數字、`AllowanceType` 字串 `"2"`）。
> 伺服器對數值型別寬鬆，但我方 DTO 應統一並實測。

#### 2.9.3 Response

只有 `code` / `msg`（`code` 【實測】為 **String**）。

**開立折讓成功後不回傳任何識別碼**（沒有 allowance_number 回拋、沒有建立時間）。
⇒ 折讓單編號完全由我方指定，**我方必須自行保證 `AllowanceNumber` 唯一且可回查，且在送出前就先落庫**——
「送出成功但回應遺失」的情境只能靠 `allowance_query` 回查我方自己產的單號來判定。

#### 2.9.4 部分折讓 vs 全額折讓【文件逐字，S14】

> 4. 根據您折讓的需求會有2種情況
> (1) 部分品項或金額折讓
> (2) 全額折讓
>
> 🫧【部分品項或金額折讓】
> 下方系統帶入原發票明細與折讓明細，需要操作的為折讓單明細的部分。
> 假設原發票明細有3項商品，退貨其中一項，折讓單明細則留下要折讓的品項，其餘品項刪除。
> 或是品項只有一項，要折退部分金額，則修改單價部分。
> 注意折讓單單價固定以未稅輸入，若您是要折讓100(含稅)，則單價需填入未稅的$95
> 最後確認總計欄位的 【營業稅額+未稅總計】 為您需要折讓的金額即可點擊送出開立折讓單。
>
> 🫧【全額折讓】
> 若送出後沒反應顯示開立成功，上方會出現 紅色提示訊息「折讓單的營業稅額已大於原發票的營業稅額」
> 此時在折讓單明細中，使用彙整明細功能可調整稅額誤差。彙整後折讓單與原發票稅額就會呈現一致。

**API 層的對應規則**：
- 部分折讓＝`ProductItem` 只放要折的品項／改小 `UnitPrice`；**沒有獨立的「部分/全額」旗標欄位**。
- 全額折讓＝把原發票所有品項照抄進 `ProductItem`。
- `4040171`「此折讓單折讓金額**已大於原發票的開立金額**」
- `4040173`「此折讓單**加上其他同發票折讓單**的折讓金額已大於原發票的開立金額」
  ⇒ **AMEGO 會累計同一張原發票的所有折讓單金額做上限檢查** ⇒ 可對同一張發票分多次部分折讓，直到累計等於原發票金額。
- 全額折讓時「營業稅額大於原發票營業稅額」是實務常見坑：逐品項 `Tax` 四捨五入後加總 ≠ 原發票 `TaxAmount`。
  **我方必須把尾差吸收到某一品項，讓 `TaxAmount` 與原發票一致**（後台稱「彙整明細」）。

#### 2.9.5 一張折讓單只能對應一張原發票【實測】

```
ProductItem 放兩張不同的 OriginalInvoiceNumber
→ {"code":"4040132","msg":"第1筆 第2品項 目前僅支援一張折讓單折讓同一張發票"}
```
⇒ **一筆訂單若開了 N 張發票，退款要開 N 張折讓單。**
⇒ 我方資料模型必須是「**折讓單 : 發票 = 1 : 1**」而非「折讓單 : 訂單」。

#### 2.9.6 錯誤代碼【文件逐字，S4】

| code | 描述 |
|---|---|
| 4040112 | 此 API 支援傳輸多張發票，data 欄位資料應為陣列字串 |
| 4040121 | 第n筆 AllowanceNumber 不可為空值或超過16字 |
| 4040122 | 第n筆 AllowanceDate 錯誤 |
| 4040123 | 第n筆 AllowanceType 錯誤 |
| 4040124 | 第n筆 OriginalInvoiceNumber 錯誤 |
| 4040125 | 第n筆 OriginalInvoiceDate 錯誤 |
| 4040126 | 第n筆 BuyerIdentifier 錯誤 |
| 4040127 | 第n筆 BuyerName 錯誤 |
| 4040128 | 第n筆 BuyerAddress 錯誤 |
| 4040129 | 第n筆 BuyerTelephoneNumber 錯誤 |
| 4040130 | 第n筆 BuyerEmailAddress 錯誤 |
| 4040131 | 第n筆 ProductItem 錯誤 |
| 4040132 | 第n筆 第n品項 目前僅支援一張折讓單折讓同一張發票 |
| 4040133 | 第n筆 第n品項 目前僅支援一張折讓單折讓同一張發票 |
| 4040134 | 第n筆 第n品項 OriginalDescription 錯誤 |
| 4040135 | 第n筆 第n品項 Quantity 錯誤 |
| 4040136 | 第n筆 第n品項 UnitPrice 錯誤 |
| 4040137 | 第n筆 第n品項 Amount 錯誤 |
| 4040138 | 第n筆 第n品項 Tax 錯誤 |
| 4040139 | 第n筆 第n品項 Tax 必須為整數 |
| 4040140 | 第n筆 TaxType 錯誤 |
| 4040141 | 第n筆 TaxAmount 錯誤 |
| 4040142 | 第n筆 TotalAmount 錯誤 |
| 4040151 | 指定發票號碼 原發票日期錯誤 |
| 4040152 | 指定發票號碼 開立中 |
| 4040153 | 指定發票號碼 已存在開立作廢 |
| 4040154 | 指定發票號碼 已註銷 |
| 4040155 | 指定發票號碼 原發票類型錯誤 |
| 4040156 | 指定發票號碼 原發票不存在 |
| 4040161 | 指定折讓單號 已存在折讓開立 |
| 4040162 | 指定折讓單號 已存在作廢折讓 |
| 4040163 | 指定折讓單號 折讓類型錯誤 |
| 4040171 | 第n筆 此折讓單折讓金額已大於原發票的開立金額 |
| 4040173 | 第n筆 此折讓單加上其他同發票折讓單的折讓金額已大於原發票的開立金額 |
| **4040145** | **不在官方表內**（【實測】）：第n筆 自114年1月1日起，AllowanceType 不可為 1 買方開立折讓證明單 |

**【實測】驗證順序（遇到第一個錯誤就回傳，不會列出全部）**：
`AllowanceNumber` 長度(4040121) → `AllowanceDate` 格式(4040122) → `AllowanceType`(4040145/4040123)
→ `ProductItem[0].OriginalInvoiceNumber`(4040124) → 同單多發票(4040132) → 原發票存在性(4040156)

**4040161/4040162 ＝ 折讓單號重複偵測**：同一個 `AllowanceNumber` 若已開立（或已作廢）就不能再用。
⇒ **作廢折讓後「重開」必須換一個新的折讓單編號**，不可沿用舊號
（S14：「折讓單開立後是無法進行編輯的，只能作廢重開」＋「折讓單編號 不可與過去單號重複」）。

### 2.10 作廢折讓 — `POST /json/g0501`

- 舊路徑 `/json/d0501`；來源檔 `d0501.php`；name `Allowance_Cancel`
- 【文件】說明：作廢已開立的折讓單
- `data` 形狀：**陣列**

| 欄位名 | 型別 | 必填 | 長度 | 說明 |
|---|---|---|---|---|
| （根） | Object[] | **是** | — | 物件陣列 |
| `array.CancelAllowanceNumber` | String | **是** | ≤16 | 折讓單編號 |

【實測】`CancelReason` 對 `g0501` **無效**（送了不會報長度錯，直接走到存在性檢查）→ **作廢折讓沒有原因欄位**。

```json
[{ "CancelAllowanceNumber": "3821061800001" }]
```
```json
[{"CancelAllowanceNumber":"3821061800001"},{"CancelAllowanceNumber":"3821061800002"},{"CancelAllowanceNumber":"3821061800003"}]
```

Response：只有 `code`（【實測】String）/ `msg`。

錯誤代碼【文件逐字，S4】：

| code | 描述 |
|---|---|
| 4050112 | 此 API 支援傳輸多張發票，data 欄位資料應為陣列字串 |
| 4050121 | 第n筆 CancelAllowanceNumber 錯誤 |
| 4050131 | 指定折讓單號 折讓開立中 |
| 4050132 | 指定折讓單號 已存在已存在作廢折讓（原文如此，重複字） |
| 4050133 | 指定折讓單號 折讓類型錯誤 |
| 4050134 | 指定折讓單號 折讓單不存在 |
| 4050135 | 指定折讓單號 **已超過修改期限** |
| 4050141 | 指定折讓單號 **等待 折讓開立/作廢折讓** |

`4050131` +`4050141` ⇒ **剛開立的折讓單不能立刻作廢**，必須等 `allowance_status` 的 `status` 走到 `99：完成`
（S14：「當發票狀態顯示完成時(上傳完成)，右上角會有 作廢 的按鈕。」）。
`4050132` ⇒ **作廢折讓是冪等失敗**（重送回錯誤而非成功）。重試邏輯必須把 `4050132` 當成「**已達成目標**」而非失敗。

### 2.11 折讓狀態 — `POST /json/allowance_status`

- 來源檔 `allowance_status.php`；`data` 形狀：**陣列**

Request：`[{"AllowanceNumber":"3821061800001"}, ...]`

| 欄位名 | 型別 | 說明【文件逐字】 |
|---|---|---|
| `data.type` | String | 折讓單類型 NOT_FOUND：查無折讓單 D0401：折讓單開立 D0501：折讓單作廢 TYPE_ERROR：類型錯誤 |
| `data.status` | Number | 折讓單狀態 1：待處理 2：上傳中 3：已上傳 31：處理中 32：處理完成／待確認 91：錯誤 99：完成 |
| `data.tax_amount` | Number | 營業稅額 |
| `data.total_amount` | Number | **未稅總計** |

官方 Response 範例「回應成功(新版)」（原樣）：
```json
{
	"code": 0, "msg": "",
	"data": [
		{ "allowance_number": "3821061800001", "type": "NOT_FOUND", "status": -1, "tax_amount": 0, "total_amount": 0 },
		{ "allowance_number": "3821061800002", "type": "D0401", "status": 99, "tax_amount": 117, "total_amount": 2343 },
		{ "allowance_number": "3821061800003", "type": "D0401", "status": 99, "tax_amount": 29, "total_amount": 578 }
	]
}
```
官方 Response 範例「回應成功(舊版)」（原樣，`data` 是以單號為 key 的 map）：
```json
{
	"code": 0, "msg": "",
	"data": {
		"3821061800001": { "type": "NOT_FOUND", "status": -1, "tax_amount": 0, "total_amount": 0 },
		"3821061800002": { "type": "D0401", "status": 99, "tax_amount": 117, "total_amount": 2343 },
		"3821061800003": { "type": "D0401", "status": 99, "tax_amount": 29, "total_amount": 578 }
	}
}
```
【實測 2026-08-24】實際回**新版陣列**：
```
POST /json/allowance_status  data=[{"AllowanceNumber":"ZZZNOTEXIST0001"}]
→ {"code":0,"msg":"","data":[{"allowance_number":"ZZZNOTEXIST0001","type":"NOT_FOUND","status":-1,"tax_amount":0,"total_amount":0}]}
```
> ⚠️ 注意：Response 欄位表**沒有列 `allowance_number`**，但兩個官方範例的新版形狀與實測都有回。

錯誤代碼【文件】：`99` 第n筆 AllowanceNumber 錯誤。

**與 `invoice_status` 同樣的靜默 false negative**【實測】：2025-01-01 的舊折讓單 `A20241231-001`（591 天前）
→ `allowance_status` 回 `NOT_FOUND / -1 / 0`，但 `allowance_query` **查得到完整資料**。
⇒ **折讓的冪等對帳必須走 `allowance_query`，絕不可用 `allowance_status`。**

### 2.12 折讓查詢 — `POST /json/allowance_query`

- 來源檔 `allowance_query.php`

| 欄位名 | 型別 | 必填 | 說明【文件逐字】 |
|---|---|---|---|
| `allowance_number` | String | **是** | 折讓單編號，**不可超過16字**。 |

> **折讓查詢沒有「用訂單編號查」的選項**，只能用折讓單號，而折讓單號是**我方開立時自己指定的**。
> ⇒ **我方 DB 必須自己保存 `allowance_number`，否則折讓永遠查不回來。** 這是與發票（可用 order_id 反查）最大的差異。

Response 欄位【文件逐字】：

| 欄位名 | 型別 | 說明 |
|---|---|---|
| `data.allowance_number` | String | 折讓單編號 |
| `data.invoice_type` | String | 折讓類型 D0401：B2C 存證折讓單開立 D0501：B2C 存證折讓單作廢 B0401：B2B 存證折讓單開立 B0501：B2B 存證折讓單作廢 B0101：B2B 交換折讓單開立 B0102：B2B 交換折讓單開立(接收確認) B0201：B2B 交換折讓單作廢 B0202：B2B 交換折讓單作廢(接收確認) |
| `data.invoice_status` | Number | 上傳至財政部的狀態（1/2/3/31/32/91/99） |
| `data.allowance_date` | Number | 折讓單日期 格式：YYYYMMDD，例如：20161028 |
| `data.allowance_type` | **String** | 折讓單種類，1:買方開立折讓證明單 2:賣方折讓證明通知單 （`allowance_list` 標 Number → §6-4） |
| `data.buyer_identifier` | String | 買方統一編號，沒有統一編號會回傳 0000000000 |
| `data.buyer_name` | String | 買方名稱 |
| `data.buyer_zip` | Number | 買方郵遞區號 |
| `data.buyer_address` | String | 買方地址 |
| `data.buyer_telephone_number` | String | 買方電話 |
| `data.buyer_email_address` | String | 買方信箱 |
| `data.tax_amount` | Number | 營業稅額 |
| `data.total_amount` | Number | 金額合計(不含稅) |
| `data.cancel_date` | Number | 作廢時間，Unix 時間戳記 |
| `data.detail_vat` | Number | 明細的單價及小計為 0:未稅價 1:含稅價 |
| `data.create_date` | Number | 建立時間，Unix 時間戳記 |
| `data.product_item` | Object[] | 商品陣列，最多 9999 筆 |
| `data.product_item.original_invoice_number` | String | 原發票號碼 |
| `data.product_item.original_invoice_date` | Number | 原發票日期 格式：YYYYMMDD |
| `data.product_item.tax_type` | Number | 課稅別　1：應稅　2：零稅率　3：免稅 |
| `data.product_item.description` | String | 品名，**不可超過256字** （`allowance_list` 寫 500 → §6-3） |
| `data.product_item.unit_price` | Number | 單價(不含稅)，可到小數7位數 |
| `data.product_item.quantity` | Number | 數量，可到小數7位數 |
| `data.product_item.unit` | String | 單位 |
| `data.product_item.amount` | Number | 小計(不含稅)，可到小數7位數 |
| `data.product_item.tax` | Number | 稅金，整數 |
| `data.wait` | Object[] | 未處理的排程陣列 |
| `data.wait.invoice_type` | String | 改成發票類型（D0401/D0501/B0401…） |
| `data.wait.create_date` | Number | 建立時間，Unix 時間戳記 |

官方 Response 範例（原樣）：
```json
{
    "code": 0, "msg": "",
    "data": {
        "allowance_number": "835b42ca", "invoice_type": "D0401", "invoice_status": 99,
        "allowance_date": 20250901, "allowance_type": 2,
        "buyer_identifier": "0000000000", "buyer_name": "消費者", "buyer_zip": 0,
        "buyer_address": "", "buyer_telephone_number": "", "buyer_email_address": "",
        "tax_amount": 7, "total_amount": 143, "cancel_date": 0,
        "detail_vat": 0, "create_date": 1756664308,
        "product_item": [
            { "original_invoice_number": "AA11110053", "original_invoice_date": 20250901,
              "tax_type": 1, "description": "南非砂糖橘小可愛A_#12",
              "unit_price": 143, "quantity": 1, "unit": "", "amount": 143, "tax": 7 }
        ],
        "wait": [{ "invoice_type": "D0501", "create_date": 1756664358 }]
    }
}
```
```json
{ "code": 71, "msg": "查無資料" }
```

**【實測】`allowance_query` 沒有 180 天限制**：查 591 天前的折讓單仍 `code 0` 完整回傳。
文件也確實沒寫 180 天（與 `invoice_query` 的參數說明形成對照）。
⚠️ 但這是**行為觀察不是官方承諾** → §5-E9。

錯誤代碼：S4 **沒有 `[折讓 - 折讓查詢]` 分節** → §5-F13。實測查無資料回 `code 71`。

### 2.13 折讓列表 — `POST /json/allowance_list`

Request 欄位與 `invoice_list` 完全相同，僅 `date_select` 語意不同（`1:折讓單日期 2:建立日期`）。

```json
{ "date_select": 1, "date_start": 20250501, "date_end": 20250531, "limit": 20, "page": 1 }
```

Response 頂層同分頁型。`data[]` 與 `allowance_query` 的差異：
- **有 `product_item`**（`invoice_list` 反而沒有）
- **沒有 `wait`、沒有 `detail_vat`**
- `data.allowance_type` 在 list 是 **Number**，在 query 是 **String** → §6-4
- `data.product_item.description` 標「品名，**不可超過500字**」→ §6-3

`allowance_list` 的 `product_item` 欄位【文件】：`original_invoice_date`（原發票日期 Ymd）／`original_invoice_number`／
`tax_type`／`description`（≤500 字）／`unit_price`／`quantity`／`unit`／`amount`（金額(不含稅之進貨額)）／`tax`（營業稅額，整數）。

官方 Response 範例（節錄，原樣）：
```json
{
    "code": 0, "msg": "", "page_total": 5, "page_now": 1, "data_total": 91,
    "data": [
        {
            "allowance_number": "QI36593012-0", "invoice_type": "D0401", "invoice_status": 99,
            "allowance_date": 20250501, "allowance_type": 2,
            "buyer_identifier": "0000000000", "buyer_name": "曾XX", "buyer_zip": 0,
            "buyer_address": "臺中市XX", "buyer_telephone_number": "", "buyer_email_address": "",
            "tax_amount": 200, "total_amount": 4010, "cancel_date": 0, "create_date": 1746069472,
            "product_item": [
                { "original_invoice_date": 20250430, "original_invoice_number": "AC12340179",
                  "tax_type": 1, "description": "東坡肉", "unit_price": 4010, "quantity": 1,
                  "unit": "", "amount": 4010, "tax": 200 }
            ]
        },
        {
            "allowance_number": "CC92632356-0", "invoice_type": "D0401", "invoice_status": 99,
            "allowance_date": 20250501, "allowance_type": 2,
            "buyer_identifier": "45444444", "buyer_name": "光貿科技股份有限公司",
            "tax_amount": 19, "total_amount": 389, "cancel_date": 0, "create_date": 1746070037,
            "product_item": [
                { "original_invoice_date": 20250430, "original_invoice_number": "AC12340180",
                  "tax_type": 1, "description": "紅燒牛肉麵", "unit_price": 122.8571, "quantity": 2,
                  "unit": "", "amount": 246, "tax": 12 },
                { "original_invoice_date": 20250430, "original_invoice_number": "AC12340180",
                  "tax_type": 1, "description": "運費", "unit_price": 142.8571, "quantity": 1,
                  "unit": "", "amount": 143, "tax": 7 }
            ]
        },
        ......以下省略
    ]
}
```
> 💡 範例中 `allowance_number` 是 `QI36593012-0` ⇒ **折讓單編號允許含 `-`**；S14 逐字：「英文數字皆可」。
> 💡 範例中 `unit_price` 是 `122.8571`（未稅單價含小數），但 `amount`／`tax` 是整數。

**【實測】`allowance_list` 沒有 180 天限制**（查 `20250101~20250131` 正常回傳）。
【實測】測試公司 `20260601~20260824`、limit 20 → `code 0`、`data_total 24967`、`page_total 1249`。

錯誤代碼：S4 **沒有 `[折讓 - 折讓列表]` 分節**。可**推定**沿用 `[發票 - 發票列表]` 的 31~36，**但非文件明載**。

### 2.14 折讓檔案（PDF）— `POST /json/allowance_file`

- 【文件】說明：下載折讓單檔案(PDF格式)。**可無限次下載。**（對比發票檔案：載具發票中獎後才可下載）

| 欄位名 | 型別 | 必填 | 說明【文件逐字】 |
|---|---|---|---|
| `allowance_number` | String | **是** | 折讓單編號，不可超過16字。 |
| `download_style` | Number | 否 | 下載樣式，預設為 0 三種樣式 **0：A4整張 1：A4(地址+A5) 3：A5** |

樣式代碼是 **0 / 1 / 3**（**跳過 2**），與 `invoice_file` 的 `{0,1,2,3,5}` **值域不同，兩者不可共用同一個 enum**。

Response：`data.file_url`（10 分鐘有效）。

官方 Response 範例（原樣）：
```json
{ "code": 0, "msg": "", "data": { "file_url": "https://invoice.amego.tw/user/invoice_print_type?token=1679038020_d22dd1ac994c3cdf4fab428b5cc95fa6&type=0" } }
```
```json
{ "code": 71, "msg": "查無資料" }
```

錯誤代碼【文件逐字，S4】：

| code | 描述 |
|---|---|
| 33 | allowance_number 不可為空 |
| 34 | download_style 參數錯誤 |
| 52 | 該折讓單有等待異動的排程 |
| 53 | 該折讓單類型不可下載 |
| 71 | 查無資料 |

⚠️ **折讓的錯誤碼表沒有 `51`（超過查詢期限）**，呼應「可無限次下載」。

【實測】驗證「折讓無 180 天限制」：
```text
# 近期折讓 D20260701092933（allowance_date 20260701）
-> {"code":0,...,"file_url":"https://invoice.amego.tw/user/invoice_d0401_print_type?token=1787578118_e12b995b55067dd3a7255cf6ed666f9d&type=0"}
GET -> HTTP 200 application/pdf 320,850 bytes

# 舊折讓 A20241231-001（allowance_date 20250101，距今 >590 天）
-> {"code":0,...}   ← 仍可下載
```
> 💡 折讓的 token 路徑是 `/user/invoice_d0401_print_type`，**與發票的 `/user/invoice_print_type` 不同**
> （官方 response 範例誤植為發票路徑）⇒ **不要對 `file_url` 的路徑做硬編碼比對，直接使用回傳值。**

### 2.15 折讓列印 — `POST /json/allowance_print`

| 欄位名 | 型別 | 必填 | 說明【文件逐字】 |
|---|---|---|---|
| `allowance_number` | String | **是** | 折讓單編號，不可超過16字。 |
| `printer_type` | Number | **是** | 熱感應機型號代碼 可支援功能請參看機型支援功能表 |
| `printer_lang` | Number | 否 | 熱感應機編碼 1：BIG5 2：GBK 3：UTF-8 若不帶入此參數，會使用預設編碼 |

Response：`data.base64_data`【文件逐字】「printer_type >= 2，base64編碼的 ESC/POS 列印格式字串，正確且需要列印才會回傳。 如何設定熱感應機及印出折讓單，請洽客服。」

⚠️ 支援折讓列印的機型見 §3.14 的「折讓單」欄（**代碼 1 Star mC-Print3 不支援**）。
錯誤碼：S4 **沒有 `[折讓 - 折讓列印]` 分節**。

### 2.16 獎項定義 — `POST /json/lottery_type`

- 【文件】說明：中獎的發票類型定義 **data 欄位不需要傳入資料 ***
- **【實測】`data` 必須送空字串 `""`，且 sign 要用 `md5("" + time + AppKey)`**；送 `{}` 或 `[]` 回 `code 16` → §1.3.5

Response：`data[].type`（Number，獎項代碼）／`data[].name`（String，獎項名稱）。

官方範例：
```json
{ "code": 0, "msg": "", "data": [ { "type": 11, "name": "特別獎(1,000萬)" }, { "type": 12, "name": "特獎(200萬元)" } ] }
```
【實測】完整回傳 13 筆 → §3.25。⚠️ **這張表是動態的（每期可能增開獎項），不要硬編，開發時直接呼叫這支取。**

### 2.17 中獎發票 — `POST /json/lottery_status`

- 【文件】說明：中獎的發票，**建議雙月1號才查詢**。 9-10月的發票，11/25 開獎，**建議 12/1 再透過 API 查詢**。

| 欄位名 | 型別 | 必填 | 說明【文件逐字】 |
|---|---|---|---|
| `Year` | Number | **是** | 西元年 |
| `Period` | Number | **是** | 期別 0:01-02月 1:03-04月 2:05-06月 3:07-08月 4:09-10月 5:11-12月 |

Response：`data[].invoice_date`（YYYYMMDD）／`data[].invoice_number`／`data[].type`（獎項代碼）。

```json
{
	"code": 0, "msg": "",
	"data": [
		{ "invoice_date": 20220819, "invoice_number": "HK73530001", "type": 22 },
		{ "invoice_date": 20220819, "invoice_number": "HK73530002", "type": 18 }
	]
}
```
> ⚠️ 文件把 `data[].type` 標 String，範例卻是 Number。
> ⚠️ 錯誤碼：S4 **沒有 `[中獎 - *]` 分節** → §5-F13。

### 2.18 手機條碼查詢 — `POST /json/barcode`

- 【文件】說明：查詢手機條碼是否正確。**這是唯一的載具驗證端點，且只驗手機條碼。**

| 欄位名 | 型別 | 必填 | 說明 |
|---|---|---|---|
| `barCode` | String | **是** | 手機條碼 |

```json
{ "barCode": "/TRM+O+P" }
```
Response：只有 `code` / `msg`（**成功時沒有任何載具資訊**）。官方**沒有提供 response 範例**。

錯誤碼【文件逐字】：

| code | 描述 |
|---|---|
| 9000111 | 手機條碼不可為空 |
| 9000112 | 手機條碼格式錯誤 |
| 9000113 | 手機條碼不存在 |

#### 2.18.1 行為矩陣【實測 2026-08-24】

| 傳入 `barCode` | 實際回應 |
|---|---|
| `/TRM+O+P` | `{"code":0,"msg":""}` ← 官方範例中真實存在的條碼 |
| `  /TRM+O+P  `（前後空白） | `{"code":0,"msg":""}` ⇒ **會 trim** |
| `/TRM+O+p`（末碼小寫）／`/abcdefg` | `{"code":9000112,"msg":"手機條碼格式錯誤"}` |
| `/ABCDEF`（6 碼）／`/ABCDEFGH`（8 碼）／`ABCDEFG`（無前導 `/`）／`/ABCDEF*` | `9000112` |
| `/AB-C.D+`、`/1234567`（格式合法但不存在） | `{"code":9000113,"msg":"手機條碼不存在"}` |
| `""` | `{"code":9000111,"msg":"手機條碼不可為空"}` |
| `[{"barCode":"/TRM+O+P"}]`（陣列） | `{"code":9000111,...}` ⇒ **不支援批次，只吃單一物件** |

⇒ **`9000112`（格式）與 `9000113`（不存在）可明確區分**，前端可分別給不同提示。

#### 2.18.2 但 `f0401` 開立時把兩者壓成同一碼【實測】

| 情境 | `f0401` 回應 |
|---|---|
| 顯碼＝隱碼，且條碼**真實存在** | 通過 |
| 顯碼 ≠ 隱碼／只給顯碼、隱碼留空 | `3040131 載具顯碼、隱碼不相符` |
| 顯碼隱碼**皆空** | `3040132 載具號碼不存在` |
| 顯碼＝隱碼但**格式錯**（小寫／少一碼／多一碼） | `3040132 載具號碼不存在` ← **不是格式碼** |
| 顯碼＝隱碼、格式對但**不存在** | `3040132 載具號碼不存在` |

⇒ 要給使用者精確錯誤訊息，**必須在送出前自行做 regex 檢查 ＋ 呼叫 `/json/barcode`**，不能只靠 `f0401` 的錯誤碼分流。

### 2.19 公司名稱查詢 — `POST /json/ban_query`

- 【文件】說明：查詢統一編號對應的公司名稱 資料來源：財政部財政資訊中心（E2）
- `data` 形狀：**陣列**

| 欄位名 | 型別 | 必填 | 說明【文件逐字】 |
|---|---|---|---|
| `ban` | String | **是** | 統一編號，數字8碼 統編基本檢查邏輯請參考 營利事業統一編號檢查碼邏輯修正說明（E1） |

```json
[ {"ban": "45444444"}, {"ban": "96999966"} ]
```

Response：

| 欄位名 | 型別 | 說明【文件逐字】 |
|---|---|---|
| `data` | Object | 發票號碼的對應上傳狀態（**原文如此，明顯是複製貼上錯誤，實為公司名稱陣列**） |
| `data.ban` | String | 傳入的統一編號 |
| `data.name` | String | 查詢到的公司名稱 **若查詢不到公司名稱，此欄位會回應空字串，並不代表此統編錯誤或不存在** |

官方範例：
```json
{ "code": 0, "msg": "", "data":[ {"ban":"45444444","name":"光貿科技股份有限公司"}, {"ban":"96999966","name":"紅磚數位有限公司"} ] }
```
【實測】：
```
data = [{"ban":"45444444"},{"ban":"96999966"},{"ban":"77777177"},{"ban":"11111101"}]
→ {"code":0,"msg":"","data":[
     {"ban":"45444444","name":"光貿科技股份有限公司"},
     {"ban":"96999966","name":"紅磚數位股份有限公司"},   ← 官方範例寫「有限公司」，實際是「股份有限公司」→ §6-21
     {"ban":"77777177","name":"高思數位網路有限公司"},
     {"ban":"11111101","name":""}]}                      ← 查不到，name 空字串，code 仍 0
```

錯誤碼【文件逐字】：

| code | 描述 |
|---|---|
| 99 | 第 n 筆 統一編號長度錯誤 |
| 99 | 第 n 筆 統一編號格式錯誤 |

⚠️ 兩種錯誤**共用 code 99**，只能靠 `msg` 區分。

**【實測】批次是 all-or-nothing**：
```
[{"ban":"45444444"},{"ban":"96999966"},{"ban":"12345678"},{"ban":"00000000"}]
→ {"code":99,"msg":"第3筆 統一編號格式錯誤"}   ← 整批沒有任何結果回來
```
⇒ **必須先自行做檢查碼驗證再送批次**，否則一顆壞蘋果會讓整批失敗。
**官方公告的測試統編 `12345678` 本身通不過檢查碼**（回「第1筆 統一編號格式錯誤」）。
送 500 筆**重複**統編只回 **1 筆**（去重）→ 必須用 `ban` 鍵值 join，不可索引對位。

⚠️ **不可以用 `ban_query` 回空字串當作「統編無效」的判準**，那會擋掉合法的買方。
S7 逐字：「本公司提供的 公司名稱查詢 API，資料來源為這些資料集，並非百分之百都查詢的到名稱。**管委會、會計事務所..等統編，都不在資料集內。**」
⇒ 統編有效性要用**財政部檢查碼演算法**自己算（E1）。

### 2.20 所有字軌資料 — `POST /json/track_all`

- 【文件逐字】說明：該公司在加值中心的所有字軌資料。 一共三層 1:財政部配給的字軌 2:給光貿用的字軌 3:發票字軌列表的內容

| 欄位名 | 型別 | 必填 | 說明 |
|---|---|---|---|
| `Year` | Number | **是** | 西元年 |
| `Period` | Number | **是**（size `0-5`） | 期別 0:01-02月 1:03-04月 2:05-06月 3:07-08月 4:09-10月 5:11-12月 |

Response 欄位【文件】：

| 欄位名 | 型別 | 說明 |
|---|---|---|
| `data.layer` | Number | 一共三層 1:財政部配給的字軌 2:給光貿用的字軌 3:發票字軌列表的內容 |
| `data.category` | Number | 1:自動配號 2:API配號 |
| `data.code` | String | 字軌名稱，**英文兩碼** |
| `data.start` | String | 字軌起號（8 碼） |
| `data.end` | String | 字軌訖號（8 碼） |
| `data.now` | String | 目前配號 |
| `data.total_booklet` | Number | 總本數 |
| `data.remark` | String | 字軌別名 |
| `data.TrackApiCode` | String | API指定代碼 |
| `data.source` | Number | **字軌來源 1:系統匯入 2:人工輸入**〔原文紅字〕 |
| `data.status` | Number | 字軌狀態 1:使用 2:停用 3:過期 9:用畢 |

**`data` 是巢狀三層**：每層物件自己再帶一個 `data` 陣列（layer 1 → 2 → 3），
**文件的平坦欄位表沒有表達這個結構**，只有範例看得出來。`category` / `now` / `TrackApiCode` / `source` / `status` **只在 layer 3 出現**。

官方範例（原樣）：
```json
{
	"code": 0, "msg": "",
	"data": [
		{ "layer": 1, "code": "ZB", "start": "80354450", "end": "80354749", "total_booklet": 6,
		  "data": [
			{ "layer": 2, "code": "ZB", "start": "80354450", "end": "80354749", "total_booklet": 6,
			  "data": [
				{ "layer": 3, "category": 1, "code": "ZB", "start": "80354450", "end": "80354749",
				  "now": "80354584", "total_booklet": 6, "remark": "", "TrackApiCode": "",
				  "source": 1, "status": 3 }
			  ] }
		  ] }
	]
}
```
【實測】測試公司 2026 年期別 4（09-10月），可見 `category` 1 與 2 並存：
```json
{"code":0,"msg":"","data":[
 {"layer":1,"code":"AC","start":"20000000","end":"20009999","total_booklet":200,"data":[
   {"layer":2,"code":"AC","start":"20000000","end":"20009999","total_booklet":200,"data":[
     {"layer":3,"category":1,"code":"AC","start":"20000000","end":"20009999","now":"",
      "total_booklet":200,"remark":"","TrackApiCode":"FSM","source":2,"status":1}]}]},
 {"layer":1,"code":"ET","start":"20260800","end":"20262899","total_booklet":42,"data":[
   {"layer":2,"code":"ET","start":"20260800","end":"20262899","total_booklet":42,"data":[
     {"layer":3,"category":2,"code":"ET","start":"20260800","end":"20262899","now":"20262899",
      "total_booklet":42,"remark":"","TrackApiCode":"","source":2,"status":9}]}]}]}
```
> 注意：`now: ""`（空字串，尚未配過號）與 `status: 9`（用畢）是真實會出現的值。

錯誤碼【文件】：`99` Period 錯誤。
⚠️ **`Period` 是 0–5 的期別代碼，不是民國年月**。送 `{"Period":"11508"}` 回 `{"code":99,"msg":"Period 錯誤"}`【實測】。

### 2.21 字軌取號 — `POST /json/track_get`

- 【文件】說明：取發票字軌，**只能取用字軌類型為「API 配號」的字軌**

| 欄位名 | 型別 | 必填 | 說明【文件逐字】 |
|---|---|---|---|
| `Year` | Number | **是** | 西元年 |
| `Period` | Number | **是** | 期別（0–5） |
| `Book` | Number | **是** | **本數，1本為50張發票。** |
| `TrackApiCode` | String | 否 | 指定字軌取號，需要在後台 發票作業 > 發票字軌列表 設定 API指定代碼 若不指定字軌，則依照後台發票字軌列表的排序去取號 |

Response：`data.code`（字軌名稱，英文兩碼）／`data.start`（起號）／`data.end`（訖號）。
```json
{ "code": 0, "msg": "", "data": { "code": "MQ", "start": "00000000", "end": "00000099" } }
```
> 範例 `Book: 2` → `00000000`~`00000099` 共 100 號，印證「1本 = 50張」。

錯誤碼【文件逐字】—— ⚠️ **全部都是 `99`，只能靠 `msg` 區分**：

| code | 描述 |
|---|---|
| 99 | Period 錯誤 |
| 99 | Book 錯誤 |
| 99 | 無字軌可取 |
| 99 | 剩餘字軌本數不足 |
| 99 | 字軌取號錯誤，請聯絡客服人員 |
| 99 | 字軌取號的訖號錯誤，請聯絡客服人員 |
| 99 | 資料庫更新資料錯誤，請稍後重試 |

**`track_get` 是有副作用的寫入操作**（它會把號段配走），卻**沒有任何冪等鍵**。
超時／連線中斷時**無法判斷號段是否已被配走**，重試會拿到**不同的號段**、前一段就此漏掉
（號段作廢會造成財政部端字軌不連續）。補救只能靠 `track_status` 的 `now` 比對。
**這是 API 配號模式最大的工程風險。** 本次調查**刻意未實打此端點**，避免消耗測試公司字軌。

### 2.22 字軌狀態 — `POST /json/track_status`

- 【文件】說明：發票字軌配號狀態，**只會回傳字軌類型為「API 配號」的字軌**

Request：`Year`（必填）／`Period`（必填 0–5）／`TrackApiCode`（optional）。

Response：`data[].code` / `start` / `end` / `now` / `total_booklet` / `used_booklet` / `status`。

```json
{
	"code": 0, "msg": "",
	"data": [
		{ "code": "MQ", "start": "00000000", "end": "00000149", "now": "00000099", "total_booklet": 3, "used_booklet": 2, "status": 1 },
		{ "code": "NY", "start": "00000200", "end": "00000249", "now": "", "total_booklet": 1, "used_booklet": 0, "status": 1 }
	]
}
```
【實測】`{"Year":2026,"Period":3}`（民國 115 年 07-08 月期）：
```json
{"code":0,"msg":"","data":[
 {"code":"AQ","start":"00000000","end":"99999999","now":"00039899","total_booklet":2000000,"used_booklet":798,"status":1},
 {"code":"DS","start":"11000000","end":"11004999","now":"","total_booklet":100,"used_booklet":0,"status":1}]}
```
錯誤碼【文件】：`99` Period 錯誤。

**`TrackApiCode`** 在 `track_get` / `track_status` / `f0401` 三支都可選用，語意一致：
「指定字軌取號／查詢／開立，需要在後台 發票作業 > 發票字軌列表 設定 API指定代碼；若不指定字軌，則依照後台發票字軌列表的排序去取號／查詢／開立」。
⇒ **多店／多通路要分流字軌就靠這個代碼。它是後台設定的字串，不是 API 產生的**，我方 config 需有對應欄位。

### 2.23 伺服器時間 — `GET /json/time`

- 【文件逐字】說明：查詢目前伺服器時間 **不需要帶入 invoice、data、time 也不須要計算 sign**，直接進入網址就可以看到伺服器時間

| 欄位 | 型別 | 必回 | 說明 |
|---|---|:--:|---|
| `timestamp` | Number | ✅ | Unix Timestamp 時間戳記 |
| `text` | String | ✅ | 目前時間文字 年/月/日 時:分:秒 |
| `year` / `month` / `day` / `hour` / `minute` / `second` | Number | ✅ | 年 / 月 / 日 / 時 / 分 / 秒 |

官方範例：
```json
{ "timestamp": 1683776130, "text": "2023/05/11 11:35:30", "year": 2023, "month": 5, "day": 11, "hour": 11, "minute": 35, "second": 30 }
```
【實測 2026-08-24】：
```json
{"timestamp":1787578776,"text":"2026\/08\/24 21:39:36","year":2026,"month":8,"day":24,"hour":21,"minute":39,"second":36}
```
**這支不回 `code`/`msg`**，形狀與其他 API 完全不同（**不能共用 envelope DTO**），且 Content-Type 是 `text/plain`。
【推定】`text` 與拆解欄位是 **Asia/Taipei（UTC+8）** 本地時間、`timestamp` 是 UTC epoch（由 21:39:36 對應 13:39:36Z 推得；**文件未載明時區**）。
⇒ **程式一律只用 `timestamp`，不要解析 `text`。**
⇒ 用途：憑證有效性自檢、時鐘偏移監控、健康檢查（§1.7）。

### 2.24 未文件化端點：發票註銷 `POST /json/f0701`

**文件面：沒有公開的註銷 API。**
- `invoice_status.data.type` 列舉含 `C0701：發票註銷`；`invoice_query.data.invoice_type` 含 `C0701：B2C 存證發票註銷`
- `f0501` 錯誤碼有 `3050123 已存在註銷`、`3050131 等待 開立/作廢/註銷`
- apiDoc 的 `project.order` 列了 `Invoice_Void`，但**資料中沒有對應 API 定義**（曾存在、後被移除）
- **官方唯一說明註銷用途的頁面是 S16**（變更買方統編／補打買方統編），逐字：
  > 儲存後，系統會**先將原發票註銷，再重新上傳變更後的發票資訊**。（原始發票號碼不會改變）
  > 當您儲存後，系統會顯示該筆發票狀態為「等待 發票開立」
  > [ 請注意 ] 該發票須為**沒有被列印的狀態**下，才可進行編輯。

⇒ **註銷（C0701）在官方文件裡是「後台編輯發票時系統自動做的內部動作」，不是給外部呼叫的動作。**

**【實測】端點實際存在**：
```
POST /json/f0701  data=[{"CancelInvoiceNumber":"ZZ00000000"}]
→ {"code":"3070111","msg":"第1筆 VoidInvoiceNumber 錯誤"}
POST /json/c0701  （同上）→ 同樣回應
POST /json/invoice_void / /json/g0701 → 連線層錯誤（端點不存在）
```
可推得：端點存在、錯誤碼族為 `307xxxx`、參數名是 **`VoidInvoiceNumber`**（不是 `CancelInvoiceNumber`）、`code` 回 **String**。
其餘（是否需陣列、有無 reason、成功回應形狀、可註銷條件）**完全未經驗證**。

⇒ **設計結論：不要把「註銷」納入本期範圍。** 要用必須先向光貿客服索取正式文件。
⇒ **API 串接方要改統編，只能「作廢 `f0501` + 重開 `f0401`」，發票號碼會變**（後台操作則號碼不變，兩條路徑行為不同）。

**「註銷折讓」不存在**：折讓的類型列舉只有 `D0401 開立` 與 `D0501 作廢`（B2B 對應 `B0401/B0501/B0101/B0102/B0201/B0202`），
**沒有任何 D0701 / 折讓註銷代碼**。⇒ 若需求寫「註銷折讓」，對應到的實作就是 `g0501 作廢折讓`。

---

## 3. 代碼對照總表

> 全部逐字抄錄自官方（S2 / S4 / S6 / S8），標【實測】者為官方未給、由實打補上。
> **建議在我方 CodeMeta 建立對應的內部技術碼，不要在程式碼硬編。**

### 3.1 課稅別 `TaxType`（主檔層）

| 值 | 意義 |
|---|---|
| `1` | 應稅 |
| `2` | 零稅率 |
| `3` | 免稅 |
| `4` | 應稅(特種稅率) |
| `9` | 混合應稅與免稅或零稅率(限訊息C0401使用) |

### 3.2 課稅別 `ProductItem.TaxType`（明細層）—— **只有 3 個值，比主檔少**

| 值 | 意義 |
|---|---|
| `1` | 應稅 |
| `2` | 零稅率 |
| `3` | 免稅 |

⚠️ `f0401` 的明細 `TaxType` 標**必填**；`f0401_custom` 的明細 `TaxType` 標 **optional（預設為應稅）**。兩支不一致 → §6-11。
⚠️ **折讓沒有表頭層 `TaxType` 欄位**，只有明細層。

### 3.3 稅率 `TaxRate`

| 值 | 說明 |
|---|---|
| `"0.05"` | 「稅率，為5%時本欄位值為0.05」 —— **String 型別，不是 Number** |

⚠️ `TaxType=4 應稅(特種稅率)` 存在，但**特種稅率的稅率值清單文件完全沒給** → §5-C5。

### 3.4 含稅／未稅 `DetailVat`

| 值 | 意義 |
|---|---|
| `0` | 未稅價 —— **只有打統編的發票可以設定**（違反回 `3040162`） |
| `1` | 含稅價（**預設**） |

### 3.5 明細小計處理 `DetailAmountRound`

| 值 | 意義 |
|---|---|
| `0` | 小數精準度到 7 位數（**預設**） |
| `1` | 一律四捨五入到整數 |

（查詢回應的 `detail_amount_round` 說明略異：`0:直接加總 1:先四捨五入再加總`）

### 3.6 稅額調整 `TaxAdjustment`（僅 `f0401` 有）

| 值 | 意義 |
|---|---|
| `0` | 不調整（**預設**） |
| `1` | 營業稅額減 1 |

限制【文件逐字】：「僅限打統編且 DetailVat=0（未稅）時，且 SalesAmount 應稅銷售額合計尾數為 **10/30/50/70/90** 才可使用」。

### 3.7 載具類別 `CarrierType`

| 值 | 意義 | 顯碼／隱碼格式 |
|---|---|---|
| `3J0002` | 手機條碼 | 正規式 `^\/[0-9A-Z\+\-\.]{7}$`（S7）；可用 `/json/barcode` 驗真實存在 |
| `CQ0001` | 自然人憑證條碼 | 正規式 `^TP[0-9]{14}$`（S7）；**無任何 API 可驗存在性** |
| `amego` | 光貿會員載具 | 「載具顯碼及載具隱碼需為 a+手機號碼 或 電子信箱」；細部 regex 見 §3.8 |
| （自訂 6 碼） | 「載具類別亦可帶入貴公司自行申請的會員載具」 | 需另向光貿申請（申請流程未載明 → §5-F6） |
| `""` | 不使用載具 | 三個載具欄位都留空 |

【文件逐字】「光貿會員載具的載具顯碼或隱碼帶入電子信箱，**並不會發送信件**，若想發送信件請填寫 BuyerEmailAddress 欄位」。

**【實測】AMEGO 不驗證載具類別白名單**：

| 傳入值 | 結果 |
|---|---|
| `3J0002` | 走手機條碼專屬檢查（格式＋**存在性**） |
| `CQ0001` | 走自然人憑證專屬檢查（**只驗格式**） |
| `amego` | 走光貿會員載具專屬檢查 |
| `AMEGO` / `Amego` | `3040155 載具類別應為6碼` ⇒ **大小寫敏感**，只有全小寫 `amego` 有效 |
| `3J000`（5 碼非 amego） | `3040155 載具類別應為6碼` |
| `3J00022`（7 碼） | `3040128 CarrierType 過長`（上限 6 碼） |
| `ZZ0001` / `999999` / `abcdef` / `shopee` / `EG0138`（任意 6 碼） | **通過**（只要顯碼隱碼皆非空且相符） |

⇒ **我方必須自己維護允許的 `CarrierType` 白名單。**
錯誤碼 `3040155`「載具類別應為6碼」與官方指定的 `amego`（5 碼）矛盾 → §6-7。
**驗證邏輯不可一律套 6 碼，要依載具種類分流。**

### 3.8 光貿會員載具（`CarrierType=amego`）的顯碼／隱碼規則【文件逐字，S6】

| 名稱 | 載具明碼/隱碼 | 正規表示式檢查規則 |
|---|---|---|
| 手機 | a手機號碼 | `^a09[0-9]{8}$` |
| 信箱 | 電子信箱 | 電子信箱檢查規則 |
| 蝦皮 | shopee_蝦皮帳號 | `^shopee_[A-Za-z0-9_\.]{5,30}$` |
| 露天 | ruten_露天帳號 | `^ruten_[a-z0-9_\-]{4,24}$` |
| Yahoo 拍賣 | yahoo_Y拍代號 | `^yahoo_[Yy][0-9]{10}$` |
| 8591 | 8591_買家編號 | `^8591_\d{5,7}$` |

⚠️ 這張表**只有 6 列**，但 S11 的查詢連結規則另外列出 **賣貨便(type=4)、好賣+(type=7)、mo店+(type=10)** ——
**這三個平台的載具正規表示式官方沒給** → §5-F7。

**【實測】AMEGO 的實作比文件寬鬆**：

| 顯碼 / 隱碼 | 結果 |
|---|---|
| `a0911222333` / 同 | 通過 |
| `A0911222333` / 同（**大寫 A**） | **通過** ⇒ 與文件 regex `^a09...` 不符 |
| `0911222333` / 同（**缺 `a` 前綴**） | **通過** ⇒ 純數字手機號也被接受 |
| `a091122233`（10 碼）／`a09112223334`（12 碼） | `3040134 CarrierId1 載具顯碼錯誤` |
| `probe@example.com` / 同 | 通過（信箱形式） |
| `shopee_abcdef` / `ruten_abcd` / `yahoo_Y1234567890` / `8591_12345` | 通過 |
| `zzz` / 同（不符任何規則） | `3040134` |
| `a0911222333` / `a0911222444`（顯≠隱） | `3040133 載具顯碼、隱碼不相符` |
| 顯碼隱碼**皆空** | `3040134` |
| 只給顯碼或只給隱碼 | `3040133` |

⚠️ **`3040131` 與 `3040133` 官方描述完全相同（「載具顯碼、隱碼不相符」），但觸發條件不同**：
- `3040131` ← `3J0002` / `CQ0001`（共通性載具）顯隱不符
- `3040133` ← `amego`（會員載具）顯隱不符
- `3040135`「載具顯碼或隱碼不可為空」**只在「非 3J0002/CQ0001/amego 的自訂 6 碼載具」情境出現**

### 3.9 通關方式註記 `CustomsClearanceMark`

| 值 | 意義 |
|---|---|
| `1` | 非經海關出口 |
| `2` | 經海關出口 |
| `0` | 查詢回傳時：「若為非零稅率發票，則此欄位為 0」 |

⚠️ **零稅率發票時此欄位為必填**（漏了回 `3040179`）。

### 3.10 零稅率原因 `ZeroTaxRateReason` / `zero_tax_rate_reason`【文件逐字】

| 值 | 意義 |
|---|---|
| `71` | 第一款 外銷貨物 |
| `72` | 第二款 與外銷有關之勞務，或在國內提供而在國外使用之勞務 |
| `73` | 第三款 依法設立之免稅商店銷售與過境或出境旅客之貨物 |
| `74` | 第四款 銷售與保稅區營業人供營運之貨物或勞務 |
| `75` | 第五款 國際間之運輸。但外國運輸事業在中華民國境內經營國際運輸業務者，應以各該國對中華民國國際運輸事業予以相等待遇或免徵類似稅捐者為限 |
| `76` | 第六款 國際運輸用之船舶、航空器及遠洋漁船 |
| `77` | 第七款 銷售與國際運輸用之船舶、航空器及遠洋漁船所使用之貨物或修繕勞務 |
| `78` | 第八款 保稅區營業人銷售與課稅區營業人未輸往課稅區而直接出口之貨物 |
| `79` | 第九款 保稅區營業人銷售與課稅區營業人存入自由港區事業或海關管理之保稅倉庫、物流中心以供外銷之貨物 |

（`invoice_list` 回傳時未設定為 `0`。2024.12.13 新增此欄位。）

### 3.11 買受人簽署適用零稅率註記 `BondedAreaConfirm`（僅 `f0401_custom`）

| 值 | 意義 |
|---|---|
| `1` | 符合加值型及非加值型營業稅法第 7 條第 4 款規定(買受人為保稅區營業人) |
| `2` | 符合加值型及非加值型營業稅法第 7 條第 7 款規定(買受人為遠洋漁業營業人) |
| `3` | 符合自由貿易港區設置管理條例第 28 條第 1項第 1 款及第 4款規定(買受人為自由貿易港區營業人) |
| `4` | 其他 |

### 3.12 發票類型 `invoice_type` / `type`（MIG 訊息代號）

| 值 | 意義 |
|---|---|
| `C0401` | B2C 存證發票開立 |
| `C0501` | B2C 存證發票作廢 |
| `C0701` | B2C 存證發票**註銷** |
| `A0401` | B2B 存證發票開立 |
| `A0501` | B2B 存證發票作廢 |
| `A0101` | B2B 交換發票開立 |
| `A0102` | B2B 交換發票開立(接收確認) |
| `A0201` | B2B 交換發票作廢 |
| `A0202` | B2B 交換發票作廢(接收確認) |
| `A0301` | B2B 交換發票退回 |
| `A0302` | B2B 交換發票退回(接收確認) |
| `NOT_FOUND` | 查無發票（**僅 `invoice_status`**） |
| `TYPE_ERROR` | 類型錯誤（**僅 `invoice_status`**） |

**「API 路徑用 F/G，資料欄位值用 C/D」是兩套代號，DB 存的是後者。**
MIG 4.0 之後路徑改成 `f0401`/`g0401`，但 `invoice_query` 回的 `invoice_type` 仍是 `C0401`
（【實測確認】本次開立的發票查回來 `"invoice_type":"C0401"`）。**mapping 要分開。**

### 3.13 折讓類型 `invoice_type` / `type`

`allowance_status` 的簡表：

| 值 | 說明 |
|---|---|
| `NOT_FOUND` | 查無折讓單 |
| `D0401` | 折讓單開立 |
| `D0501` | 折讓單作廢 |
| `TYPE_ERROR` | 類型錯誤 |

`allowance_query` / `allowance_list` / `invoice_query.allowance[]` 的完整表：

| 值 | 意義 |
|---|---|
| `D0401` | B2C 存證折讓單開立 |
| `D0501` | B2C 存證折讓單作廢 |
| `B0401` | B2B 存證折讓單開立 |
| `B0501` | B2B 存證折讓單作廢 |
| `B0101` | B2B 交換折讓單開立 |
| `B0102` | B2B 交換折讓單開立(接收確認) |
| `B0201` | B2B 交換折讓單作廢 |
| `B0202` | B2B 交換折讓單作廢(接收確認) |

### 3.14 上傳財政部狀態 `invoice_status` / `status`（發票與折讓共用同一組）

| 值 | 意義 |
|---|---|
| `-1` | **文件欄位表未列**，僅出現在官方範例與【實測】；`NOT_FOUND` 時搭配回傳 |
| `1` | 待處理 |
| `2` | 上傳中 |
| `3` | 已上傳 |
| `31` | 處理中 |
| `32` | 處理完成／待確認 |
| `91` | 錯誤 |
| `99` | 完成 |

**只有 `99` 才是「財政部已收訖」。** `code:0` 只代表 AMEGO 收單成功。
- UI 顯示「發票已開立」→ 以 `invoice_number` 有值為準
- UI 顯示「已上傳財政部」→ 必須輪詢到 `status=99`
- **`99` 也是可作廢／可折讓的前提**（S13 逐字：「若發票狀態為『待處理』、『上傳中』、『處理完成/待確認』，表上尚未完成上傳作業，還無法操作作廢」）
- `91`（錯誤）是需要人工介入的終態，**必須有告警**（後續處置路徑文件未載明 → §5-E4）
- **enum 必須容納 `-1`，且要有 default 分支**（是否還有其他未列值未知）

### 3.15 折讓單種類 `AllowanceType` / `allowance_type`

| 值 | 意義 | 可用性 |
|---|---|---|
| ~~`1`~~ | ~~買方開立折讓證明單~~ | **自 114/01/01 起不可用**（原文刪除線；【實測】送 1 回 `4040145`） |
| `2` | 賣方折讓證明通知單 | **唯一可用值** |

【文件逐字】：「自114年1月1日起，經雙方合意之退回或折讓，**賣方**營業人應**開立**電子發票**折讓單**依限據實上傳」。
⚠️ 但**查詢類 API 的回應說明仍平鋪列出 1 與 2**（因為歷史資料可能有 1）。

### 3.16 字軌相關

| 欄位 | 值 | 意義 |
|---|---|---|
| `Period` | `0` / `1` / `2` / `3` / `4` / `5` | 01-02月／03-04月／05-06月／07-08月／09-10月／11-12月 |
| `layer` | `1` / `2` / `3` | 1:財政部配給的字軌　2:給光貿用的字軌　3:發票字軌列表的內容 |
| `category` | `1` / `2` | 1:自動配號　2:API配號 |
| `source` | `1` / `2` | 1:系統匯入　2:人工輸入 |
| `status` | `1` / `2` / `3` / `9` | 1:使用　2:停用　3:過期　9:用畢 |
| `Book` | Number | 「本數，**1本為50張發票**」 |

### 3.17 熱感應機型號代碼 `PrinterType` / `printer_type`

**以下為 2026-08-24 直接從官方 https://invoice.amego.tw/info_detail?mid=77 解析出的原始表格**
（二手抄錄與官方原表不符 → §6-8，**以本表為準**）：

| 代碼 | 機型 | 正本發票 | 補印發票 | 設定是否列印明細(僅B2C發票) | 單印明細(僅B2C發票) | 折讓單 | 編碼 |
|---|---|:--:|:--:|:--:|:--:|:--:|---|
| 1 | Star mC-Print3 | V | V | | | | BIG5(預設) |
| 2 | Xprinter 芯燁通用 | V | V | **V** | V | V | BIG5 / GBK(預設) / UTF-8 |
| 3 | EPSON TM-T82III | V | V | | V | V | BIG5(預設) |
| 4 | HPRT TP805L | V | V | | V | V | BIG5(預設) |
| 5 | 芯燁XP-P201A | V | V | | V | V | BIG5 / GBK(預設) |
| 7 | GM-Q99K | V | V | | V | V | BIG5 / GBK(預設) |
| 8 | Star TSP650II | V | V | | V | V | BIG5(預設) |
| 9 | RONGTA ACE G1-UE | V | V | | V | V | BIG5 / GBK / UTF-8(預設) |
| 10 | BIRCH QM3 | V | V | | V | V | BIG5(預設) / GBK |
| 11 | WP-T810 | V | V | | V | V | BIG5(預設) / GBK |
| 12 | HPRT TP801 | V | V | | V | V | BIG5(預設) / GBK |

**要點**：
- **代碼 6 不存在**（官方表格就是從 5 跳到 7）
- **只有代碼 2 支援「設定是否列印明細」**（對應 `PrintDetail` / `print_invoice_detail` 參數，且參數說明寫「僅支援 PrinterType >= 2」）
- **代碼 1（Star mC-Print3）不支援折讓單列印**，其餘皆支援
- `f0401_custom` 的 `PrinterType` 說明**只列 1 和 2**，且把代碼 2 稱作「**芯燁XP-Q90EC**」，
  而 mid=77 稱代碼 2 為「**Xprinter 芯燁通用**」→ §6-9

### 3.18 熱感應機編碼 `PrinterLang` / `printer_lang`

| 值 | 意義 |
|---|---|
| `1` | BIG5 |
| `2` | GBK |
| `3` | UTF-8 |

⚠️ `f0401_custom` 的說明只列 `1：BIG5 2：GBK`（**沒有 3**）→ §6-10。以 `f0401`／`invoice_print`／`allowance_print` 的三值為準。

### 3.19 是否列印明細 `PrintDetail` / `print_invoice_detail`

| 值 | 意義 |
|---|---|
| `0` | 不列印 |
| `1` | 印出明細(裁切)（**預設**） |
| `2` | 印出明細(不裁切) |

限制【文件逐字】：「**打統編一律列印明細**」「目前僅支援 **PrinterType >= 2** 可以設定此參數」
（`invoice_print` 版另加「此參數僅限制 Xprinter 芯燁通用以後的機型 B2C 發票可使用」）。

### 3.20 列印格式 `print_invoice_type`（`invoice_print` 專用）

| 值 | 意義 |
|---|---|
| `1` | 發票正本 |
| `2` | 發票補印 |
| `3` | 單印明細(僅限制Xprinter 芯燁通用以後的機型B2C發票可使用) |

### 3.21 PDF 下載樣式 `download_style`

**`invoice_file`**（原文寫「有打統編發票，四種樣式」但實列五項，**沒有 4**）：

| 值 | 意義 |
|---|---|
| `0` | A4整張（**預設**） |
| `1` | A4(地址+A5) |
| `2` | A4(A5x2) |
| `3` | A5 |
| `5` | QRcode_A4 |

沒有打統編的發票，一種樣式：`0` A4整張(背面兌獎聯，需雙面列印)

**`allowance_file`**（三種樣式，**沒有 2、沒有 5**）：

| 值 | 意義 |
|---|---|
| `0` | A4整張（**預設**） |
| `1` | A4(地址+A5) |
| `3` | A5 |

**兩者值域不同，不可共用 enum。** 且 `invoice_file` **實測不驗值域**（傳 9 仍回 code 0），必須我方白名單擋。

### 3.22 列印註記 `PrintMark`（`f0401_custom` 必填）／查詢回應 `print_mark`

| 值 | 意義 |
|---|---|
| `Y` | 列印 |
| `N` | 不列印 |

⚠️ `f0401` **沒有** `PrintMark` 欄位。「帶了 `PrinterType` 之後是否就算『列印』而不能用載具」**文件未載明** → §5-F3。

### 3.23 查詢類型 `type`（`invoice_query` / `invoice_file` / `invoice_print`）

| 值 | 意義 |
|---|---|
| `order` | 訂單編號 |
| `invoice` | 發票號碼 |

### 3.24 日期條件 `date_select`

| 值 | `invoice_list` | `allowance_list` |
|---|---|---|
| `1` | 發票日期 | 折讓單日期 |
| `2` | 建立日期 | 建立日期 |

### 3.25 中獎獎項 `type`（`/json/lottery_type` 實測全表）【實測】

| type | name |
|---|---|
| `11` | 特別獎(1,000萬) |
| `12` | 特獎(200萬元) |
| `13` | 頭獎(20萬元) |
| `14` | 二獎(4萬元) |
| `15` | 三獎(1萬元) |
| `16` | 四獎(4,000元) |
| `17` | 五獎(1,000元) |
| `18` | 六獎(200元) |
| `19` | 增開六獎 |
| `20` | 雲端發票專屬(2,000元) |
| `21` | 雲端發票專屬(100萬元) |
| `22` | 雲端發票專屬(500元) |
| `23` | 雲端發票專屬(800元) |

⚠️ **這張表是動態的（每期可能增開獎項），不要硬編**，開發時直接呼叫 `/json/lottery_type` 取。

**但 `invoice_lottery` 欄位的實際值域與這張表零重疊**：
抽樣測試公司 2026-06 的 **1,500 筆**發票，`invoice_lottery` 分布為

| 值 | 筆數 |
|---|---|
| 87 | 915 |
| 80 | 277 |
| 81 | 192 |
| 84 | 85 |
| 86 | 25 |
| 85 | 3 |
| 88 | 2 |
| 82 | 1 |

**沒有任何一筆落在 11~23**。官方 `invoice_list` 範例中也出現 `"invoice_lottery": 87`（佐證非測試資料異常）。
另一輪實測中新開立的發票 `invoice_lottery` 為 `0`（也不在 11~23 內）。
文件對 `invoice_lottery` 的說明只有「獎項代碼，請參考 [獎項定義] API」——**該指引在實際資料上不成立**。
⇒ **`invoice_lottery` 必須以原始整數存 DB，禁止用 enum 嚴格解析**；顯示層一律 fallback；80~88 的定義**必須向客服索取** → §5-F10。

### 3.26 錯誤碼全表【文件逐字，S4 https://invoice.amego.tw/info_detail?mid=71】

#### 3.26.1 通用錯誤代碼（所有 API 共用）

| code | 描述 | 性質【推定】 | 建議處置【推定，官方無處置建議】 |
|---|---|---|---|
| 0 | 成功 | — | — |
| 10 | 系統停機維護中 | **暫時性** | 可重試（拉長間隔，維護窗可達 4 小時）＋告警 |
| 11 | invoice(統編)不可為空 | 請求錯 | 不可重試。⚠️ **用錯 Content-Type 也會回這個碼**（誤導性，寫進 runbook） |
| 12 | invoice(統編)錯誤 | 帳號問題 | 不可重試，直接告警 |
| 13 | status 未啟用 | 帳號／開通狀態 | 不可重試，人工處理（無自助恢復途徑） |
| 14 | IP 錯誤 | **IP 未在白名單** | 不可重試；出口 IP 變動會整批失敗 |
| 15 | Time 錯誤 | 時鐘偏移／重試沿用舊 time | 重算 time+sign 後可重試一次；連續發生＝NTP 問題 |
| 16 | 簽名驗證錯誤 | 簽章不符 | 不可重試；多半是序列化做了兩次或 App Key 錯 |
| 17 | 資料不可為空 | 請求錯 | 不可重試（我方 bug） |
| 18 | 無法建立資料庫連線 | **暫時性** | 指數退避重試 |
| 19 | 公司停權，無法使用 API 開立發票 | 帳號問題 | 不可重試，直接告警到人 |
| 20 | data 非 JSON 字串格式 | 請求錯 | 不可重試（⚠️ 實測非 JSON 時實際回 17 → §6-13） |
| 21 | 人數過多，請稍後 | **限流** | 指數退避重試（**無門檻／無 Retry-After** → §5-A6） |
| 22 | 尚未申請 API 串接 | 帳號問題 | 不可重試，人工處理 |
| 23 | 此 API 支援傳輸多筆資料，data 欄位資料應為陣列字串 | 請求錯 | 不可重試（我方 bug） |

#### 3.26.2 編碼規律（供 DB 欄位型別決策）

| 前綴／範圍 | 歸屬 | 例 |
|---|---|---|
| `0`–`23` | 通用 | — |
| `31`–`36`、`51`–`56`、`71`、`72`、`99` | 查詢／列表／檔案／列印類的**區域碼**（**同一數字在不同 API 意義不同**） | `71` 在 `invoice_file` 是「查無資料」；`99` 在 `ban_query` 是「統一編號長度錯誤」 |
| `3040111`–`3040193` | 發票 - 開立發票(自動配號) | `3040111 字軌不足`、`3040171 OrderId 重複` |
| `3050111`–`3050141` | 發票 - 作廢發票 | `3050125 指定發票號碼 發票不存在` |
| `307xxxx` | **未文件化**：發票註銷 `f0701` | `3070111 第1筆 VoidInvoiceNumber 錯誤`【實測】 |
| `4040112`–`4040173` | 折讓 - 開立折讓 | `4040171 此折讓單折讓金額已大於原發票的開立金額` |
| `4050112`–`4050141` | 折讓 - 作廢折讓 | `4050135 指定折讓單號 已超過修改期限` |
| `9000111`–`9000113` | 其他 - 手機條碼查詢 | `9000113 手機條碼不存在` |

⇒ **DB 欄位型別**：目前最大值 `9000113` < 2^31，`INTEGER` 足夠。
   但 code 的語意是「同數字跨 API 不同意義」⇒ **落庫必須同時記 `api_path` + `code` + `msg`**，只存 code 無法還原問題。
   且 `code` 的 JSON 型別在折讓寫入類是 **String**（§1.5.2）⇒ 建議 DB 存 `VARCHAR(16)` 較安全。

#### 3.26.3 開立發票(自動配號) `f0401` 錯誤碼【文件逐字】

| code | 描述 |
|---|---|
| 3040111 | 字軌不足 |
| 3040112 | 此 API 只支援傳輸單張發票，data 欄位資料應為物件字串 |
| 3040121 | BuyerIdentifier 字數錯誤 |
| 3040122 | BuyerIdentifier 格式錯誤 |
| 3040123 | BuyerName 不可為空或過長 |
| 3040124 | BuyerAddress 過長 |
| 3040125 | BuyerTelephoneNumber 過長 |
| 3040126 | BuyerEmailAddress 過長 |
| 3040127 | MainRemark 過長 |
| 3040128 | CarrierType 過長 |
| 3040129 | CarrierId1 過長 |
| 3040130 | CarrierId2 過長 |
| 3040131 | 載具顯碼、隱碼不相符 |
| 3040132 | 載具號碼不存在 |
| 3040133 | 載具顯碼、隱碼不相符 |
| 3040134 | CarrierId1 載具顯碼錯誤 |
| 3040135 | 載具顯碼或隱碼不可為空 |
| 3040136 | NPOBAN 格式錯誤 |
| 3040137 | NPOBAN 不存在 |
| 3040138 | ProductItem 不可為空或過多 |
| 3040139 | 第n品項 Description 不可為空或過長 |
| 3040140 | 第n品項 Quantity 格式錯誤 |
| 3040141 | 第n品項 UnitPrice 格式錯誤 |
| 3040142 | 第n品項 Amount 格式錯誤 |
| 3040143 | 第n品項 Remark 過長 |
| 3040144 | 第n品項 TaxType 錯誤 |
| 3040145 | SalesAmount 格式錯誤 |
| 3040146 | FreeTaxSalesAmount 格式錯誤 |
| 3040147 | ZeroTaxSalesAmount 格式錯誤 |
| 3040148 | TaxType 格式錯誤 |
| 3040149 | TaxRate 格式錯誤 |
| 3040150 | TaxAmount 格式錯誤 |
| 3040151 | TotalAmount 格式錯誤 |
| 3040152 | TotalAmount 不可為負數 |
| 3040153 | OrderId 不可為空或過長 |
| 3040154 | 載具類別不可為空 |
| 3040155 | 載具類別應為6碼 |
| 3040160 | BrandName 不可為空或過長 |
| 3040161 | DetailVat 格式錯誤 |
| 3040162 | 只有打統編發票， 才可以用未稅單價及小計 |
| 3040163 | DetailAmountRound 格式錯誤 |
| 3040171 | OrderId 重複 |
| 3040171 | OrderId 重複，該訂單編號正在註銷重開中 |
| 3040172 | 第n品項 Amount 金額錯誤 |
| 3040173 | TaxType 設定錯誤 |
| 3040174 | SalesAmount 計算錯誤 |
| 3040175 | ZeroTaxSalesAmount 計算錯誤 |
| 3040176 | FreeTaxSalesAmount 計算錯誤 |
| 3040177 | TaxAmount 計算錯誤 |
| 3040178 | TotalAmount 計算錯誤 |
| 3040179 | 若為零稅率發票，通關方式註記必填 |
| 3040180 | 通關方式註記錯誤 |
| 3040181 | SalesAmount 不應該為負數 |
| 3040182 | ZeroTaxSalesAmount 不應該為負數 |
| 3040183 | FreeTaxSalesAmount 不應該為負數 |
| 3040184 | TotalAmount 不應該為負數 |
| 3040191 | 無法取得下一張發票 |
| 3040192 | 取得發票列印格式錯誤 |
| 3040193 | TaxAdjustment 格式錯誤 |
| **3040113** | **不在官方表內**（【實測】）：ProductItem 資料應為陣列 |

**【實測】確認的觸發條件補充**：

| code | 實測觸發條件 |
|---|---|
| 3040121 | `BuyerIdentifier` 長度非 8 也非 10（含空字串） |
| 3040122 | 長度對但**檢查碼不合法**（`12345678`、`00000000`、`0000000001`） |
| 3040123 | `BuyerName` 空字串或 **> 60 字元** |
| 3040124 / 3040125 / 3040126 | 300 字 / 100 字 / 312 字觸發（精確上限未二分） |
| 3040127 | `MainRemark` > 200 字元 |
| 3040128 / 3040129 / 3040130 | `CarrierType` > 6；`CarrierId1` > 64；`CarrierId2` 推定同 64 |
| 3040131 | `3J0002`/`CQ0001` 顯≠隱（含只給其一） |
| 3040132 | `3J0002`：格式錯／不存在／顯隱皆空；`CQ0001`：格式錯／顯隱皆空 |
| 3040133 | `amego` 顯≠隱（含只給其一） |
| 3040134 | `amego` 顯碼不符任何會員載具格式（含顯隱皆空） |
| 3040135 | **自訂 6 碼載具**（非 3J0002/CQ0001/amego）顯碼或隱碼為空 |
| 3040136 | `NPOBAN` 為 1–2 碼數字、或負數 |
| 3040137 | `NPOBAN` ≥8 碼、或含非數字字元 |
| 3040139 / 3040143 | Description > 256／Remark > 120 |
| 3040154 | `CarrierType` 空但 `CarrierId1`/`CarrierId2` 有值 |
| 3040155 | `CarrierType` 長度 5 且不等於 `amego`（`AMEGO`、`Amego` 也中） |
| 3040174 | **萬用錯誤**：金額不平、稅額算錯、稅別放錯欄位都回它，訊息還會誤導 |
| 3040153 | 【實測】`OrderId` 41 字**未**觸發（與文件的 ≤40 不符） |

#### 3.26.4 其餘分節

`[發票 - 作廢發票]` → §2.3.3；`[發票 - 發票狀態]` → `99` 第n筆 InvoiceNumber 錯誤；
`[發票 - 發票查詢]` → §2.5.4；`[發票 - 發票列表]` → §2.6.3；`[發票 - 發票檔案]` → §2.7.3；`[發票 - 發票列印]` → §2.8；
`[折讓 - 開立折讓]` → §2.9.6；`[折讓 - 作廢折讓]` → §2.10；
`[折讓 - 折讓狀態]` → `99` 第n筆 AllowanceNumber 錯誤；`[折讓 - 折讓檔案]` → §2.14；
`[其他 - 手機條碼查詢]` → §2.18；`[其他 - 公司名稱查詢]` → §2.19；`[其他 - 所有字軌資料]` → `99` Period 錯誤；
`[自行配號專用 - 字軌取號]` → §2.21；`[自行配號專用 - 字軌狀態]` → `99` Period 錯誤。

**S4 目錄共 17 個分節。⚠️ 沒有** `[折讓 - 折讓查詢]`、`[折讓 - 折讓列表]`、`[折讓 - 折讓列印]`、`[中獎 - *]` 的分節
⇒ 這幾支的非通用錯誤碼**文件完全未載明** → §5-F13。

#### 3.26.5 開立發票(API配號) `f0401_custom` —— **全部 50+ 種錯誤共用 code `99`**【文件逐字】

| code | 描述 |
|---|---|
| 99 | 此 API 支援傳輸多張發票，data 欄位資料應為陣列字串 |
| 99 | 有指定熱感應機型號代碼，只能一次傳輸一張 |
| 99 | 第n筆 InvoiceNumber 錯誤 |
| 99 | 第n筆 InvoiceDate 錯誤 |
| 99 | 第n筆 InvoiceTime 錯誤 |
| 99 | 第n筆 InvoiceNumber 非當期字軌 |
| 99 | 第n筆 InvoiceNumber 超過開立期限 |
| 99 | 第n筆 InvoiceNumber 號碼不在字軌起訖內 |
| 99 | 第n筆 InvoiceNumber 字軌類別設定錯誤 |
| 99 | 第n筆 InvoiceNumber 字軌類型設定錯誤 |
| 99 | 第n筆 BuyerIdentifier 錯誤 |
| 99 | 第n筆 BuyerName 錯誤 |
| 99 | 第n筆 BuyerAddress 錯誤 |
| 99 | 第n筆 BuyerTelephoneNumber 錯誤 |
| 99 | 第n筆 BuyerEmailAddress 錯誤 |
| 99 | 第n筆 MainRemark 錯誤 |
| 99 | 第n筆 CarrierType 錯誤 |
| 99 | 第n筆 CarrierId1 錯誤 |
| 99 | 第n筆 CarrierId2 錯誤 |
| 99 | 第n筆 PrintMark 錯誤 |
| 99 | 第n筆 載具顯碼、隱碼不相符 |
| 99 | 第n筆 載具號碼不存在 |
| 99 | 第n筆 CarrierId1 載具顯碼錯誤 |
| 99 | 第n筆 NPOBAN 錯誤 |
| 99 | 第n筆 RandomNumber 錯誤 |
| 99 | 第n筆 ProductItem 錯誤 |
| 99 | 第n筆 第n品項 Description 錯誤 |
| 99 | 第n筆 第n品項 Quantity 錯誤 |
| 99 | 第n筆 第n品項 UnitPrice 錯誤 |
| 99 | 第n筆 第n品項 Amount 錯誤 |
| 99 | 第n筆 第n品項 Remark 錯誤 |
| 99 | 第n筆 第n品項 TaxType 錯誤 |
| 99 | 第n筆 若為零稅率發票，通關方式註記必填 |
| 99 | 第n筆 通關方式註記錯誤 |
| 99 | 第n筆 SalesAmount 錯誤 |
| 99 | 第n筆 FreeTaxSalesAmount 錯誤 |
| 99 | 第n筆 ZeroTaxSalesAmount 錯誤 |
| 99 | 第n筆 TaxType 錯誤 |
| 99 | 第n筆 TaxRate 錯誤 |
| 99 | 第n筆 TaxAmount 錯誤 |
| 99 | 第n筆 TotalAmount 錯誤 |
| 99 | 第n筆 order_id 錯誤 |
| 99 | **第n筆 列印發票不可設定載具** |
| 99 | **第n筆 列印發票不可設定捐贈碼** |
| 99 | **第n筆 捐贈發票不可設定載具** |
| 99 | 第n筆 載具類型錯誤 |
| 99 | 第n筆 載具顯碼錯誤 |
| 99 | 第n筆 載具隱碼錯誤 |
| 99 | 指定發票號碼 已存在開立 |
| 99 | 指定發票號碼 已存在開立作廢 |
| 99 | 指定發票號碼 註銷中 |
| 99 | 指定發票號碼 發票類型錯誤 |

**這是選型的關鍵差異**：API 配號模式**無法用 code 分辨錯因，只能解析中文 `msg`**；
而自動配號 `f0401` 有結構化的 `304xxxx` 代碼。⇒ **一律優先選 `f0401`。**

#### 3.26.6 只出現在錯誤碼、參數表完全沒提的業務規則

| 規則 | 出處 |
|---|---|
| **列印發票不可設定載具** | `f0401_custom` 錯誤碼（`f0401` 實測不擋 → §2.1.9） |
| **列印發票不可設定捐贈碼** | 同上 |
| **捐贈發票不可設定載具** | 同上 |
| 「已存在折讓單」的發票**不能作廢** | `3050141` |
| 發票有「修改期限」，逾期不能作廢 | `3050126` |
| 折讓單有「修改期限」，逾期不能作廢 | `4050135` |
| 折讓總額不能大於原發票金額（**含累計多張折讓單**） | `4040171` / `4040173` |
| **一張折讓單只能折同一張發票** | `4040132` / `4040133` |
| `CancelReason` 存在且上限 20 字 | `3050113` |
| `BrandName` 帶了就不能是空字串 | `3040160` |
| `ProductItem.Tax` 必須為整數 | `4040139` |
| `AllowanceType` 不可為 1（114 年新制） | `4040145`（**不在官方表內**） |
| `ProductItem` 必須是陣列 | `3040113`（**不在官方表內**） |

---

## 4. 限制與時間窗

### 4.1 欄位長度總表（直接對應 DB DDL）

> **「文件未載明」欄位的數字全部來自【實測】，不是官方承諾**，改版可能變動。

| 欄位 | 文件明載 | 【實測】邊界 | 建議 DB 型別 | 備註 |
|---|---|---|---|---|
| `OrderId` / `order_id` | **≤40**，「不可重複」 | 41 字未被擋 | `VARCHAR(40)` + UNIQUE | 唯一的冪等鍵 |
| `BuyerIdentifier` | 8 碼統編或 `0000000000` | 只接受這兩種 | `VARCHAR(10)` | **無統編是 10 個 0 不是 8 個**；不可對 8 碼統編左補零 |
| `BuyerName` | 未載明（3040123「不可為空或過長」） | **60 字元**（61 → 錯） | `VARCHAR(60)` | 字元數不是 byte 數 |
| `BuyerAddress` | 未載明（3040124） | 300 字被拒，精確上限未測 | `VARCHAR(100)`（保守） | 待確認 |
| `BuyerTelephoneNumber` | 未載明（3040125） | 100 字被拒 | `VARCHAR(26)`（保守） | 待確認 |
| `BuyerEmailAddress` | 未載明（3040126） | 312 字被拒 | `VARCHAR(80)`（保守） | 待確認 |
| `MainRemark` | **≤200** | 200 PASS / 201 錯 | `VARCHAR(200)` | — |
| `CarrierType` | 6 碼（3040155） | >6 → 3040128 | `VARCHAR(6)` | `amego` 是 5 碼（矛盾 → §6-7） |
| `CarrierId1` / `CarrierId2` | 未載明（3040129/3040130） | **64 字元**（65 → 錯） | `VARCHAR(64)` | — |
| `NPOBAN` | 未載明 | **3–7 碼純數字** | `VARCHAR(7)` | ≥8 碼直接拒 |
| `BrandName` | 未載明（3040160） | 未測 | 未載明 | 帶了就不能空字串 |
| `TrackApiCode` | 未載明 | 實測值 `FSM`（3 碼） | 未載明 | 後台設定值 |
| `ProductItem.Description` | **≤256** | 256 PASS / 257 錯 | `VARCHAR(256)` | — |
| `ProductItem.Unit` | **≤6** | — | `VARCHAR(6)` | — |
| `ProductItem.Remark` | **≤120**（開立端）**≤40**（`invoice_query` 回應端） | 120 PASS / 121 錯 | **取 40 保守值** | 矛盾 → §6-1 |
| `ProductItem.RelateNumber`（custom） | **≤50** | — | `VARCHAR(50)` | — |
| `ProductItem.Quantity` / `UnitPrice` / `Amount` | 小數 **7 位** | — | `NUMERIC(x,7)` | 要整數就用 `DetailAmountRound=1` |
| `ProductItem`（陣列） | **最多 9999 筆**（2025.01.14 起） | — | — | body 大小上限未載明 |
| `SellerPersonInCharge`（custom） | **≤30** | — | `VARCHAR(30)` | — |
| `Currency`（custom） | **3 字** | — | `CHAR(3)` | MIG v4.1 CurrencyCodeEnum |
| `ExchangeRate`（custom） | 小數 **3 位** | — | `NUMERIC(x,3)` | — |
| `InvoiceNumber` / `invoice_number` | ≤10 | 實測固定 10 碼 | `CHAR(10)` | 2 英文 + 8 數字 |
| `RandomNumber` / `random_number` | — | 實測 4 碼 | `CHAR(4)` | **必須存字串**（前導 0） |
| `InvoiceDate`（custom） | `YYYYMMDD` | — | `CHAR(8)` 或 `DATE` | — |
| `InvoiceTime`（custom） | `hh:mm:ss` | — | `CHAR(8)` 或 `TIME` | — |
| `CancelInvoiceNumber` | 發票號碼 10 碼 | — | `CHAR(10)` | — |
| `CancelReason` | **參數表未列**（3050113） | **20 字** | `VARCHAR(20)` | 只有 `f0501` 有；`g0501` 無效 |
| `AllowanceNumber` | **16**（一份）vs **40**（另一份） | 17 字被拒 | **`VARCHAR(16)`** + UNIQUE | 矛盾 → §6-2；允許含 `-` |
| `OriginalDescription` / 折讓品名 | **256**（g0401、allowance_query）**500**（allowance_list） | — | **取 256** | 矛盾 → §6-3 |
| `barcode`（回傳） | — | 19 碼 | `VARCHAR(19)` | 民國年期別5 + 號碼10 + 隨機碼4 |
| `qrcode_left` / `qrcode_right` | 未載明 | 隨品項變動 | **`TEXT`** | **原樣存原樣出，不做轉義** |
| `base64_data` | 未載明 | — | `TEXT` | — |
| `msg` | 未載明 | 含「第n筆／第n品項」內插 | `TEXT` | **永不用來做分支** |
| `code` | — | Number 或 String | `VARCHAR(16)`（保守） | 型別不一致 → §6-6 |
| `file_url` | 「連結僅10分鐘有效」 | — | **不要存 DB** | bearer token，等同機敏資料 |

### 4.2 金額限制

| 項目 | 規則 | 來源 |
|---|---|---|
| 幣別單位 | 元；合計欄位實測皆為整數 | 【實測】 |
| **合計不可為負** | `3040152` TotalAmount 不可為負數；`3040181`~`3040184` SalesAmount/ZeroTax/FreeTax/TotalAmount 不應該為負數 | 【文件】 |
| **單一品項可以為負** | 官方範例即用 `"UnitPrice":"-2"` 表示「會員折抵」 | 【文件】 |
| 折扣表示方式 | **做成負數品項列，不是扣總額** | 由上兩條推得 |
| 兩組獨立錯誤碼 | 「格式錯誤」3040145~3040151 vs 「計算錯誤」3040174~3040178 ⇒ AMEGO 會重算並驗證加總 | 【文件】 |
| 折讓金額語意 | **一律未稅**（`UnitPrice`/`Amount`/`TotalAmount`），稅額另放 `Tax`/`TaxAmount` 且**必須整數** | 【文件】g0401 + S14 |
| 折讓金額上限 | ≤ 原發票開立金額，且**同發票所有折讓單累計**亦不得超過 | `4040171` / `4040173` |
| 0 元發票 | **可以開**，但：`qrcode_left`/`qrcode_right` 回空字串、不回 `base64_data`、`invoice_print` 回 `56`「0元發票，無法列印發票正本及發票補印」 | 【文件】（但「TotalAmount 是否允許 0」沒有正面敘述 → §5-C3） |
| 合計欄位上限與小數位 | **未載明** | → §5-C6 |
| `Round()` 捨入模式 | **未載明**（四捨五入 vs banker's） | → §5-C1 |

### 4.3 字數與筆數限制

| 項目 | 限制 | 來源 |
|---|---|---|
| `ProductItem` 陣列 | 最多 **9999 筆**（`f0401` / `invoice_query` / `g0401` / `f0401_custom`，2025.01.14 起） | 【文件】 |
| `invoice_list` / `allowance_list` `limit` | **20~500**，預設 20（上下界都嚴格檢查，越界回 `34`） | 【文件】+【實測】 |
| `invoice_status` 批次 | 文件未載明；【實測】50/300/1000/3000 筆相異號碼皆可 | 建議自訂 ≤200 分批 |
| `ban_query` 批次 | 文件未載明（範例僅 2 筆）；**all-or-nothing** | 先自驗檢查碼再送 |
| 批次回應去重 | 送重複鍵值只回 1 筆 ⇒ **必須用鍵值 join，不可索引對位** | 【實測】 |
| `f0401` | **只收單張**（送陣列回 3040112） | 【文件】+【實測】 |
| `f0401_custom` | 收陣列，但「**有指定熱感應機型號代碼，只能一次傳輸一張**」 | 【文件】 |
| POST body / `data` 大小 | **未載明**（實測 93KB 可通過） | → §5-A10 |

### 4.4 查詢時間窗總表（本檔最重要的一張表）

| API | 官方文件寫的窗口 | 【實測】實際行為 | 超限時的訊號 | 可否用於冪等判斷 |
|---|---|---|---|---|
| `invoice_query` | 「以發票日期為主，只能查詢180天內的發票」 | **邊界精確成立**：發票日 ≥ 今日−180 天（含第 180 天）；−181 天起失敗 | **`code 51`**（可辨識） | ✅ **建議主用** |
| `invoice_status` | **未載明任何窗口** | **約當期+前兩期**，更早者靜默 `NOT_FOUND` | **無訊號**（`code 0` + NOT_FOUND + status −1） | ❌ **禁用** |
| `invoice_list` | 未載明 | **無限制**（實測 6.6 年 / 600 天前皆正常） | — | ✅ 補救管道（只有主檔，無明細） |
| `invoice_file` / `invoice_print` | 「只能查詢180天內的發票」 | 實測 `invoice_file` 一致（20250101 → 51） | `code 51` | —（非查詢用途） |
| `allowance_query` | **未提 180 天** | **無限制**（實測 591 天前正常） | — | ✅ **折讓建議主用** |
| `allowance_status` | 未載明 | 同 `invoice_status`，靜默 `NOT_FOUND` | **無訊號** | ❌ **禁用** |
| `allowance_list` | 未載明 | 無限制 | — | ✅ 補救管道 |
| `allowance_file` | 「**可無限次下載**」，錯誤碼表**無 51** | 實測 590 天前仍可下載 | — | — |
| `file_url`（兩者） | **僅 10 分鐘有效** | 未實測過期情況 | 過期／竄改回 **HTTP 200 + 中文純文字** | — |

**設計含意：發票 PDF 與明細只在開立後 180 天內可取。**
若業務需要長期保存（客服調閱一年前的發票），**必須在我方 DB 落地完整明細，並在 180 天內把 PDF 抓下來自存**。

### 4.5 法定與營運時間窗

#### 4.5.1 開立後上傳時限【文件逐字，S9】

> ❗注意根據財政部的規定，發票的「開立」上傳必須遵守以下時限：
> 1. **個人發票 ：需於開立發票後2天內完成上傳。**
> 2. **統一編號發票 ：需於開立發票後7天內完成上傳。**
> 例如 ：發票建檔日期為7月1日，則必須在7月3日（含）前將發票上傳至光貿。
> 如果延遲至7月4日才傳至光貿進行上傳，則會被視為發票逾時上傳，國稅局後續會對營業人處以記點或罰款。
> ✨請確保在規定的時限內完成發票上傳，避免不必要的處罰或風險。

FAQ 的另一種表述【文件逐字，S15】：
> 您好，營業人交付與取得電子發票的時間限制如下：
> B2C：48小時內上船發票及載具識別資訊
> B2B：7日內完成發票開立、接收、上傳
> 因考量到企業在營運過程，或有不可抗力、不可歸責的事由，因此無法在時限內交付與取得，統一發票使用辦法-第7條第5項另訂定如下：
> 「符合前項規定者，視為已將統一發票交付買受人，買受人視為已取得統一發票。但有其他不可歸責於營業人之事由，致無法依前項規定辦理者，應於事由消滅之翌日起算三日內完成傳輸並向所在地主管稽徵機關申請，經該管稽徵機關核准者，視同已依規定交付。」

**設計含意（法遵，不是技術問題）**：我方一旦在系統內「產生發票資料」，就開始跑 **48 小時（B2C）/ 7 日（B2B）** 的法定時鐘。
**任何把開立排入非同步佇列的設計，都必須有「距離逾期剩幾天」的告警與人工介入路徑**，
**不能只有重試次數**——逾時的代價是國稅局記點或罰款。

#### 4.5.2 關帳日與跨期上傳鎖定【文件逐字，S9】

> 上期的發票，例如7-8月的發票，在9月份處理的時限
> ⭐預設關帳日單月5號 23:59:59
> 跨期後單月1號~5號23:59:59 都可以自由上傳(開立、註銷、作廢)
> 單月6日 00:00:00 起 鎖定「開立」、「註銷」和「作廢」上傳
> 若需要上傳「開立」、「註銷」和「作廢」需要聯繫光貿協助開啟權限(關帳日修改)
> ⭐空白發票上傳為單月5號以及9號(各1次)凌晨00:00:00執行
> 9號 凌晨00:00:00 系統會將前期字軌轉為過期狀態。
> 若需要上傳「開立」需要聯繫光貿協助調整關帳日以及字軌轉為使用中的狀態。
> !注意! 除了發票逾期上傳問題外，空白發票還需要自行人工至財政部電子發票整合服務平台更新處理

⇒ **每單月 6 日 00:00:00 起，上期發票的開立／作廢／註銷全部被鎖，要解鎖得聯繫光貿人工開權限。**
⇒ **跨期退款／跨期作廢有硬牆**，退款流程必須有「原發票已跨期且過了 5 號」這條分支，正解通常是**開折讓單而不是作廢發票**。

#### 4.5.3 發票作廢期限【文件逐字，S13】

> 🪄 發票作廢期限說明
> 當期至跨期的關帳日前(一般預設為單月5日)，ex. 5-6月的發票可以在 **7/5(含)前**自行操作作廢。
> ※提醒您，只要是有跨期操作，ex. 7/5作廢5/20的發票，建議還是與公司的會計說一聲，避免會計已經提前完成申報作業。
> **單月6日~15日為申報期限的最後10天，關帳日後，系統會鎖定您上一期發票的作廢功能。**
> 若您還有確定要作廢的發票並且向會計告知確認後，可以**聯繫光貿客服**，提供欲作廢發票號碼，由我們協助您將發票轉作廢上傳。
> 🧩**如果會計已完成申報，無法再作廢發票，可以改開立折讓單的方式將發票全額折讓**
>
> 2. 當您開立發票後，系統會上傳到國稅局，**待國稅局系統回傳上傳成功，才可以做作廢或折讓等操作。**
> 若發票狀態為「待處理」、「上傳中」、「處理完成/待確認」，表上尚未完成上傳作業，**還無法操作作廢**。
> 3. ​當發票狀態為「完成」，表示財政部已接收完畢，這時候右上角就會有按鈕，點擊即可作廢此張發票。
> ※提醒您，如果已有列印該張發票，請記得要回收並撕掉銷毀。

對應 API 錯誤碼：逾期 → `3050126`；尚未上傳完成 → `3050121` / `3050131`。
⚠️ 「修改期限」的**具體天數官方沒有正面給** → §5-E6。

#### 4.5.4 折讓單日期的回溯窗口【文件逐字，S14】

> ❗**折讓單編號**　不可與過去單號重複，英文數字皆可
> ❗**折讓單日期**　預設為操作的當下
> 假設原發票日期為5/1，現在為5/22
> 【一般發票(無統編)】折讓單日期可以選擇**包含今天的過去 2 天內**的時間：2025/5/21、5/22
> 【打統編發票】折讓單日期可以選擇**包含今天的過去 7 天內**的時間：2025/5/16、17、18、19、20、21、22

⇒ `AllowanceDate` **不可任意回填**：無統編 2 天內、有統編 7 天內（與上傳時限 2 天/7 天同源）。逾期回 `4040122 AllowanceDate 錯誤`。
⚠️ 這是**後台 UI 的規則描述**；API 端是否套用同一窗口**文件未明說** → §5-E12。**實作上直接送「今天」最安全。**
⚠️ **作廢折讓的期限（`4050135`）官方沒有任何頁面說明** → §5-E7。

#### 4.5.5 打統編發票指定開立日期與「順時順號」【文件逐字】

> 打統編發票與B2B交換發票可往前開6天內的發票，往後可開6天內的發票。
> 若您有需要指定日期功能，請先聯繫 客服中心 開啟此功能。
> **開立電子發票須遵守「順時順號」的原則。**
> 舉例 ：一整段字軌已開立日期3/2發票，則無法再選擇日期3/1日開立 (不符合順時順號原則)。
> 遇到這個情況可以將字軌切割分段，作為獨立未使用的本數

**「順時順號」是硬約束**：同一段字軌的發票號碼順序必須與開立時間順序一致。
⇒ 併發開立時若我方自行配號（`f0401_custom`），**必須保證號碼分配與時間單調一致**；用 `f0401` 則由 AMEGO 保證。
**這是選 `f0401` 而非 `f0401_custom` 的第二個理由。**

#### 4.5.6 作廢 vs 折讓 的選用判準

【文件逐字，S15】：
> **問：請問我當期的第一個月的發票，結果客人第二個月來退貨(費)，那這張發票是要作廢還是折讓？甚麼狀況使用作廢或折讓？**
> 您好，當期的二個月中，其實作廢跟折讓都是可以的，但通常會計都是這樣分類：
> **當【月】的用作廢**
> **跨【月】的用折讓**
> **所以如果跨期已申報後，一定是折讓喔。**

【文件逐字，S15】開錯發票的處理：
> B2B 發票, 只能作廢, 再重新開立
> B2C 發票, 基本上不會給紙本發票, 若沒有給紙本發票, 可以直接作廢再重開，若會員有索取紙本發票，那發票就一定要收回

【文件逐字，S14】：
> 適合用於「**部分商品退貨**」或「**跨期發票全額折讓**」的情況。
> 舉例2. 原發票日期為3/15，現在5/22已過5/15申報3-4期發票期限，無法直接作廢該發票，而是開立折讓單在當下日期，金額全折讓，後續7/15前申報5-6期發票就會有這筆折讓資料。

**可機械執行的決策樹（由上述規則整理，非官方原文）**：
```
退款發生時：
├─ 原發票尚未上傳完成（invoice_status.status != 99）
│     → 兩者都不能做，必須先等 status=99（否則 3050121 / 3050131）
├─ 原發票已有折讓單（3050141）
│     → 只能再開折讓（作廢已被鎖死）
├─ 全額退款 且 原發票在「當期且未過關帳日（單月 6 日 00:00 前）」
│     → 作廢 f0501（會計慣例：當月用作廢）
├─ 全額退款 但 已跨期／已過關帳日
│     → 全額折讓 g0401（S13 明文指定的替代路徑）
└─ 部分退款（任何時點）
      → 部分折讓 g0401（作廢無法表達部分金額）
```

### 4.6 速率限制與重試

**【文件未載明】官方完全沒有任何速率限制數字**：沒有 QPS、沒有每分鐘/每日上限、沒有併發連線數、沒有配額 header。

唯一線索：
- 通用錯誤 `21`「人數過多，請稍後」——暗示有節流／過載保護，但**觸發門檻與冷卻時間皆未載明**
- 【實測】連續打約 40 次唯讀請求（含 1000 筆批次）**未觸發 code 21**，回應 header 中**無任何 rate-limit 欄位**
- `track_get` 的「資料庫更新資料錯誤，**請稍後重試**」（但 code 是 99）

**可從錯誤碼語意機械推導的重試分支（本檔建議，非官方）**：

| 分類 | code | 建議處置 |
|---|---|---|
| **可安全重試**（暫時性、無副作用） | `10` 停機維護、`18` DB 連線、`21` 人數過多 | 指數退避（起始 1s、上限 60s、最多 5 次）；`10` 應拉長間隔（維護窗可達 4 小時）。**每次重算 time+sign** |
| **重試前必須先對帳** | 網路逾時／連線中斷／無回應 | **必須先 `invoice_query(type=order)` 反查**，確認是否已開立，再決定補送 |
| **視為已成功的重複** | `3040171` OrderId 重複、`3050122` 已存在開立作廢、`4050132` 已存在作廢折讓 | 改走查詢取回既有資料，不要重送。⚠️ **這三個「錯誤碼其實代表成功」的分支必須明確寫進設計書** |
| **等待後重試** | `3050131` / `4050141`「等待 開立/作廢/註銷」、`52`「有等待異動的排程」、`3040171` 的「正在註銷重開中」 | 上一個異動還在排隊，延遲數秒~數分後再試 |
| **重試無用，資源耗盡** | `3040111` 字軌不足、`3040191` 無法取得下一張發票 | 告警＋停止開立 |
| **不可重試**（請求本身錯） | `11`~`17`、`20`、`22`、`23`、所有 `304xxxx` 驗證類、`4040xxx` | 修正請求或人工介入 |
| **不可重試**（帳號狀態） | `12`、`13`、`14`、`19`、`22` | 落庫並直接告警到人；**不要放進自動重試佇列**（無自助恢復途徑） |
| **不可重試**（狀態機不允許） | `3050121`~`3050141`、`4040151`~`4040163`、`4050131`~`4050141` | 先查狀態再決定 |

### 4.7 停機維護

錯誤碼 `10「系統停機維護中」`。實際公告樣本（官網彈窗，逐字）：
> 本網站將於113年12月21日(六) 02:00 - 06:00進行停機升級，暫停對外服務。
> 停機期間，將暫時停止下述服務：(一) 光貿系統網站無法進入 (二) 無法透過光貿系統網站進行發票之相關作業，如：開立發票、發票作廢、發票折讓...等

⇒ 維護窗**以 4 小時級距**發生，且**只在官網公告、沒有 API 或訂閱通知** → §5-B7。
⇒ 重試策略必須能吸收 4 小時級的中斷，或設計成「排入佇列 + 告警」。
⚠️ 與 §4.5.1 的法定 48 小時／7 日時鐘要一起看：**維護窗吃掉的時間也算在法定時限內。**

### 4.8 財政部規定造成的其他限制

| # | 限制 | 影響 | 來源 |
|---|---|---|---|
| 1 | **B2C 48 小時 / B2B 7 日內完成上傳**，逾時記點或罰款 | 非同步開立必須有逾時告警 | S9 / S15 |
| 2 | **必須使用電子發票專用（丁種）字軌**，未使用者無法上傳財政部存證 | 字軌是先決條件，不足即 `3040111` | S15 / S20 |
| 3 | 字軌配號分「**期配**」（每 2 個月一期，一般公司）與「**年配**」（需另向國稅局申請） | 期別 `Period` 是 0~5 的雙月制；跨期要重新取號 | S20 |
| 4 | 期配：**僅可取用本期及下期**；財政部於**雙月18號**開始下期配號，依本期最後已取用本數決定下期；要調整需**本期雙月18號前**申請 | 字軌容量規劃的時點 | S20 |
| 5 | **順時順號原則** | 自行配號時的併發控制；補開過去日期需切割字軌 | S20 |
| 6 | **MIG 4.0**：MIG 3.2.1 於民國 115 年（2026）1/1 停止使用；光貿已於 2025-01-01 升級 | 路徑改名 c0401→f0401 等 | S2 / S10 |
| 7 | **自 114 年（2025）1/1 起，經雙方合意之退回或折讓，賣方營業人應開立電子發票折讓單依限據實上傳** | 退款流程必須開折讓單，不能只做內部帳；`AllowanceType` 只能是 2 | g0401 說明 |
| 8 | **統一發票使用辦法第 7 條第 5 項**：不可歸責事由致無法依限傳輸者，應於事由消滅之翌日起算 **3 日內**完成傳輸並向稽徵機關申請核准 | 系統長時間中斷時的法遵補救路徑 | S15 |
| 9 | **申報期**：單月 1~15 日為前期申報期，6~15 日是最後 10 天，關帳日後系統鎖定上期發票的開立/註銷/作廢 | 跨期退款只能走折讓 | S9 / S13 |
| 10 | **統一編號有檢查碼邏輯** | 送出前本地驗證，減少 3040121/3040122 | E1 |
| 11 | **捐贈碼須為財政部公告之受捐贈機關代碼** | 需自行同步財政部清單（AMEGO 不驗，見 §4.9） | E3 / E4 |
| 12 | **載具中獎才可下載/列印**（`53`）；0 元發票無 QRCode 故不可列印正本（`56`） | PDF 下載功能要分「有無載具」兩條路徑 | §2.7 / §2.8 |
| 13 | **空白發票**（未使用字軌）由光貿於單月 5 號、9 號各上傳一次；9 號凌晨前期字軌轉「過期」 | 每期末的字軌盤點時點 | S9 |
| 14 | **重複開立收費**：第 1~3 次免收處理費，**自第 4 次起酌收**（依實際重複張數另行報價）。重複開立＝**同一組字軌號碼同時提供兩套以上系統使用** | **一組字軌只能由一套系統使用**。與既有 POS／電商平台並存時必須切割字軌分段 | S17 |
| 15 | 費用：首次申請**免費 3 個月**（每推薦一家再加 3 個月，可累加）；免費期後**終身月費 199 元(含稅)**，無張數或設定費限制 | 成本評估 | S19 |
| 16 | 客服：LINE `@amego`、電話 `0800-777-399`、市話 `04-2302-9119`；**回覆時間平日 am 10:00-12:00、pm 02:00-06:00** | App Key 只能由客服人工發放 ⇒ 事故應變 RTO 受限於客服上班時間 | S15 / S19 |

### 4.9 AMEGO 不驗、必須由我方驗的清單

| 項目 | 我方必須做的驗證 | 理由（AMEGO 的實際行為） |
|---|---|---|
| **手機條碼** | `^/[0-9A-Z+\-.]{7}$` **＋** 呼叫 `/json/barcode` | `f0401` 把「格式錯」與「不存在」都壓成 `3040132`，給不出好訊息 |
| **自然人憑證** | `^TP[0-9]{14}$` | AMEGO **完全不驗存在性**——隨手編的 `TP12345678901234` 照樣開票成功，消費者事後在財政部平台查不到 |
| **會員載具（amego）** | 依 §3.8 六種 regex 逐一比對 | AMEGO 實作比文件寬鬆（大寫 A、無 `a` 前綴都放行），會存進不一致的載具號 |
| **載具類別** | 自建白名單（`3J0002`/`CQ0001`/`amego`/自家申請的 6 碼） | AMEGO 接受**任意 6 碼字串** |
| **捐贈碼** | `^[0-9]{3,7}$` **＋** 比對財政部捐贈碼清單（E3/E4） | `3040137「NPOBAN 不存在」`其實只是「不是 3–7 碼純數字」的意思——`"999"`、`"000"` 這種顯然不存在的碼**一律通過**。打錯的捐贈碼會成功開票，捐贈失效且**不可逆**（只能作廢重開） |
| **統編** | 財政部檢查碼演算法（E1）**＋** `ban_query` 取抬頭 | AMEGO 有驗檢查碼（`3040122`），但 `ban_query` **批次 all-or-nothing**，壞值讓整批失敗；且查不到名稱 ≠ 統編無效 |
| **抬頭** | 非空、≤60 字元、不可為 `0`/`00`/`000`/`0000` | 「不可填 0000」文件有寫但 AMEGO **實測不擋** |
| **三選一互斥** | 「打統編」/「載具」/「捐贈」擇一 | `f0401` **完全不擋**三者並存（§2.1.9） |
| **`OrderId` 唯一性** | 自己保證不重複，且**送出前先落庫** | `3040171` 有兩種訊息要分別處理；重複時不回既有發票號碼 |
| **`download_style`** | 白名單：發票 `{0,1,2,3,5}`、折讓 `{0,1,3}` | `invoice_file` **實測不驗值域**（傳 9 仍回 code 0，靜默產生非預期版面） |
| **`date_start` / `date_end` 格式** | 嚴格 `YYYYMMDD` | `"2026-08-01"` 或 `""` 會**靜默回全部歷史**（33 萬筆），文件宣稱的 `code 32` 從未觸發 |
| **PDF 下載結果** | 檢查 `Content-Type == application/pdf` 或 magic bytes `%PDF-` | `file_url` 失敗時回 **HTTP 200 + 中文純文字**，只看 status 會把錯誤頁存成 PDF |
| **環境防呆** | 非 prod 時斷言 `AMEGO_INVOICE == 12345678` | base URL 相同、回應無環境標記，設定錯就對正式公司開真發票 |

### 4.10 捐贈碼 `NPOBAN` 的完整實測行為

【文件】能找到的全部原文只有三處：參數表「捐贈碼」三個字、範例 `"NPOBAN": "275"`、以及 S7 的「捐贈碼可串接 財政部提供的 API（E3/E4）」。

| 傳入值 | 結果【實測】 |
|---|---|
| `""`（空） | 通過（＝不捐贈） |
| `"0"` | **通過** ⇒ 被當成「未填」（PHP 弱比較 `'0' == 0`） |
| `"1"` / `"12"` | `3040136 NPOBAN 格式錯誤` |
| `"123"` / `"1234"` / `"12345"` / `"123456"` / `"1234567"` | **全部通過** |
| `"000"` / `"999"` / `"001"` / `"8585"` / `"275"` | **全部通過** |
| `" 275"`（前導空白） | **通過**（PHP `is_numeric` 容許前導空白） |
| `"12345678"`（8 碼）／`"123456789"`／`"abc"`／`"0.5"`／`"12a"` | `3040137 NPOBAN 不存在` |
| `"-1"` | `3040136 NPOBAN 格式錯誤` |

**結論：`3040137「NPOBAN 不存在」`根本不是在比對捐贈碼清單，而是「不是 3–7 碼純數字」的意思。**
⇒ **AMEGO 完全不驗證捐贈碼真實存在**，我方**必須自建捐贈碼清單**（來源 E3/E4，且 CSV 下載需圖形驗證碼 → §5-F5）。

### 4.11 消費者發票查詢連結（不是 API，是 URL 組裝規則）

【文件逐字，S11】：
> 1. 用訂單編號及信箱、電話查詢
> `https://invoice.amego.tw/user/order_query?v={賣方統編}&order_id={訂單編號}&data={信箱或電話}`
> 2. 發票號碼、發票日期、隨機碼
> `https://invoice.amego.tw/user/order_query?type=2&number={發票號碼}&date=YYYY-MM-DD&random={隨機碼}`
> 3. 會員載具 - 蝦皮
> `https://invoice.amego.tw/user/order_query?type=3&v={賣方統編}&order_id={訂單編號}&data={蝦皮帳號}`
> 4. 會員載具 - 賣貨便
> `https://invoice.amego.tw/user/order_query?type=4&v={賣方統編}&order_id={訂單編號}&data={發票金額}`
> 5. 會員載具 - 露天
> `https://invoice.amego.tw/user/order_query?type=5&v={賣方統編}&order_id={訂單編號}&data={露天帳號}`
> 6. 會員載具 - Yahoo
> `https://invoice.amego.tw/user/order_query?type=6&v={賣方統編}&order_id={訂單編號}&data={Y拍帳號}`
> 7. 會員載具 - 好賣+
> `https://invoice.amego.tw/user/order_query?type=7&v={賣方統編}&order_id={訂單編號}&data={發票金額}`
> 10. 會員載具 - mo店+
> `https://invoice.amego.tw/user/order_query?type=10&v={賣方統編}&order_id={訂單編號}&data={發票金額}`
> 12. 會員載具 - 8591
> `https://invoice.amego.tw/user/order_query?type=12&v={賣方統編}&order_id={訂單編號}&data={買家編號}`

⚠️ `v` = 賣方統編（等同我方的 `invoice` 參數值）。**type 值不連續**（無 1、8、9、11）。
**【實測】頁面下拉選單另有 `103`（蝦皮宅配訂單），官方文件完全沒記載** → §5-F11。

**【實測】重大限制：該頁有 Cloudflare Turnstile 人機驗證。**
實測 `GET .../order_query?type=2&number=AB10000316&date=2026-07-30&random=3942`：
- HTTP 200、`text/html`、23,208 bytes
- 頁面**只是把參數預填進表單**（`<input name="number" value="AB10000316">` 等）
- 表單內含 `<div class="cf-turnstile" data-sitekey="0x4AAAAAAAgC7F8o97O52VEj"></div>` + `<button type="submit">送出</button>`
- 對照組：把 `random` 改成錯誤值 `0000`，回應**大小完全相同**（23,208 bytes）⇒ **GET 階段不做任何驗證，不會回傳發票內容**

⇒ **實作影響**：
1. 這**不是**「點了就看到發票」的深連結，消費者必須**再過一次人機驗證並按送出**。
2. **不能 iframe 內嵌、不能自動跳轉取內容、不能爬取。**
3. 若產品需求是「買家在我方 App／網站內直接看到發票」，**正解是走 `invoice_file` 取 PDF 後由我方轉存並自行呈現**。
4. 若仍要提供此連結（例如放通知信），type=1 需要 `v`+`order_id`+`data`（信箱或電話），
   其中 `data` 必須與開票時填的 `BuyerEmailAddress` / `BuyerTelephoneNumber` **完全一致**
   —— 這使「開票時 email/電話留空」與「提供查詢連結」互相衝突。
5. **安全面向**：樣板 1／2 是**沒有簽章的公開查詢**，`order_id` + 信箱/電話 就能看到發票內容。
   ⇒ 若我方的 `OrderId` 是可猜測的序號（日期+流水號），等於把發票內容曝露給任何知道訂單號與買家 email 的人。
   **`OrderId` 應帶不可猜測的成分**（例如 TSID 或加鹽雜湊）。

### 4.12 Email 通知

**沒有任何寄送 email 的 API 端點。** 對全部 24 筆 API 定義窮舉，沒有 `send_mail` / `resend` / `notify` 之類的端點。

- Email 是「開立發票」的**副作用**，由 `BuyerEmailAddress` 觸發【文件】：「寄送通知信用，若不希望寄送，留空即可。」
- 【文件】「**測試環境不會主動發送信件**，若想要收測試信件，請登入測試後台，查詢到該筆發票內容，按下『補發發票開立通知』按鈕」
- 【文件】「光貿會員載具的載具顯碼或隱碼帶入電子信箱，**並不會發送信件**，若想發送信件請填寫 `BuyerEmailAddress` 欄位」
  ⇒ **「載具帶了 email」不等於「會寄信」。`BuyerEmailAddress` 是唯一開關。**
- 【文件，S18】正式環境確實會自動寄：
  > **請問開立完成的發票，會自動Mail給我的客戶嗎?**
  > 是的，您開立發票後，系統會自動發Email通知，但如果您首次開立發票，我們會建議你第一次開發票先填入自己信箱，收到發票通知後，您看過格式與下載檔案後，之後開發票都可以直接填入客戶的信箱喔。
- **補寄／重寄的唯一記載方式**：登入後台 → 查詢到該筆發票 → 按「補發發票開立通知」。**這是後台 UI 操作，沒有對應 API。**
  （另有「只有當期的可以做補發發票開立通知」的限制。）

⇒ **「重寄發票通知信」後端無法自動化。** 若產品需要此功能，必須改為我方自建寄信
（用 `invoice_file` 取 PDF，由我方 notification 服務寄出）。
⇒ 折讓 `g0401` 也有 `BuyerEmailAddress`，但**說明只有「買方電子信箱」五個字，是否觸發寄信未載明** → §5-F8。

### 4.13 正式環境申請流程摘要

**路徑 A：無電子發票資格的營業人**（4 個順序，S20）
1. 註冊光貿會員（LINE / Facebook / Apple ID / 電子信箱）→ 公司列表 → +新增公司 → 填公司資料
2. 聯繫光貿客服（LINE `@amego` / 0800-777-399 / 04-2302-9119）→ 收到送審文件（承諾書、自行檢測表、感熱紙切結書、委任書、字軌申請書等）→ **5 處蓋公司大小章**（有事務所協助則 7 處）→ 送審 → **國稅局審核約 7 個工作天**
3. 收到核准公文 → 登入財政部大平台 → **密碼種子設定**（人員帳號及權限管理 → 密碼種子管理）→ **電子發票專用字軌號碼取號**（先建立取號通知 E-mail）→ **等待約 15 分鐘**收到配號通知信
4. 聯繫光貿客服開通 → 公司列表狀態變更為「啟用」

**路徑 B：已有電子發票資格的營業人**（3 個順序，S21）
1. 註冊 + 新增公司資料（「匯入字軌模式」**建議選「全權自己處理」**，以免造成兩邊平台發票字軌重複開立）
2. 用**工商憑證／自然人憑證 + 讀卡機**在大平台設定：加密種子密碼設定（截圖給客服核對，**密碼請勿馬賽克**）＋**授權加值中心（統編 `45444444`）**＋（有 B2B 交換需求才需）營業人接收方式設定
3. 聯繫光貿客服開通系統

**取得正式 App Key**：【文件】「請與客服聯絡」。**沒有自助產生 App Key 的 API 或後台按鈕的記載** → §5-B5。

---

## 5. 文件未載明／需向 AMEGO 書面確認 —— 總清單

> **本節是整份手冊最重要的一節。**
> 以下每一項都經過實查（`grep` 全部 24 支 API 定義 ＋ 15 份補充說明頁純文字），確認官方**真的沒寫**。
> 每條都標了「**不知道會導致什麼**」——請把標 ⚑ 的直接列成**向光貿客服書面確認清單**。
> 「不確定」和「已確認」一樣需要證據：本節的每一條都不是「我沒找到」，是「已窮舉搜尋且零命中」。

### 5.A 可靠性、重試、冪等（整類皆無）

| # | 未載明的事 | 實查證據 | 不知道會導致什麼 | 建議處置 |
|---|---|---|---|---|
| **A1** ⚑ | **完全沒有 webhook／callback／回調機制** | 對 `api_data.json`（全 24 支 API 定義）＋ **15 個補充說明頁純文字**（ref-70/71/73/74/77/example ＋ mid=9/32/49/60/76/95/99/101/105）以 `webhook\|callback\|回調\|回呼\|推播\|通知網址\|notify_url\|notifyUrl` 掃描 → **兩邊皆 0 命中**（2026-08-24 實跑）。站上唯一叫「通知」的是「蝦皮平台通知消費者發票中獎」，那是通知**消費者**不是回呼商家 | 發票是否真的上傳財政部（`status=99`）、是否轉成 `91 錯誤`，**只能自己輪詢**；中獎也只能自己查。若誤以為有 webhook 而不做輪詢，發票會停在「已送出但不知結果」的狀態，直到有人客訴才發現 | 自建輪詢排程（`invoice_status` / `allowance_status` / `lottery_status`）。多副本部署時該 worker 需 lease claim，不可用永久旗標 |
| **A2** ⚑ | **沒有任何重試策略說明**（重試幾次、退避多久、哪些碼可重試） | 全文 `retry\|重試` 只在錯誤訊息「資料庫更新資料錯誤，請稍後重試」出現一次 | **`f0401` 在網路超時時，我方無法分辨「AMEGO 沒收到」還是「收到了但回應遺失」——盲目重試可能開出第二張發票** | 逾時 → 一律先 `invoice_query(type=order)` 對帳再決定；重試必重算 time/sign；退避參數自訂（建議起始 1s／上限 60s／最多 5 次）並向客服確認 |
| **A3** | **沒有冪等鍵（idempotency key）機制** | `grep -i 'idempot\|冪等'` **0 命中** | 唯一保護是 `OrderId` / `AllowanceNumber` 的唯一性。若我方在呼叫前沒先落庫，重試時無從沿用同一個鍵 | **送出前先把 `OrderId` / `AllowanceNumber` 寫進我方 DB（同一 transaction）**，失敗重試沿用同鍵 |
| **A4** | **`3040171` 一碼兩義** | S4 錯誤碼表兩列同碼：「OrderId 重複」／「OrderId 重複，該訂單編號正在註銷重開中」 | 前者是**永久衝突**（應改查詢）、後者是**暫時態**（應退避重試）。無法用 code 區分 ⇒ **必須字串比對中文 msg**，而 msg 不是契約、隨時可能改字 | 寫測試釘住這兩個字串；msg 一旦不符預期就走保守路徑（當成永久衝突並告警） |
| **A5** ⚑ | **沒有 HTTP timeout / SLA / 回應時間承諾** | 全文無 | client deadline 只能自己拍。太短會製造 A2 的不確定狀態 | 【推定】開票類 30s、查詢類 10s；上線後依實測 p99 調整。**必須設 deadline** |
| **A6** ⚑ | **限流門檻未載明**：`code 21` 的觸發條件、QPS 上限、有無 `Retry-After` | 有錯誤碼 21，但無任何數字；【實測】回應 header **無 `X-RateLimit-*`、無 `Retry-After`**；連打 40 次未觸發 | 大促／日結批次開票可能整批被擋，且**沒有 header 可判斷該等多久** | 我方自己限速（保守 ≤5 QPS），監控 code 21 出現率；申請時向客服問 QPS 上限與冷卻時間 |
| **A7** | **`f0401` 單次請求的發票筆數上限**（`f0401_custom` 收陣列但沒說上限） | `f0401_custom` 說明有一句被截斷的「若有傳入熱感應機型號代碼，一次只⋯」 | 批次開票的分批大小只能保守取值 | 自訂上限並實測邊界 |
| **A8** ⚑ | **批次寫入的部分成功語意未載明**（`f0501` / `g0501` / `f0401_custom` 送多張時） | 回應只有單一 `code`/`msg`，**沒有逐筆結果陣列**；【實測】錯誤訊息是「第1筆 …」或直接帶單號 | **無法判斷「前 3 張成功、第 4 張失敗」** | **一律一次一張**，用我方的批次層做並行；或事後用 `invoice_status` 逐張覆核 |
| **A9** | **批次查詢的單次最大筆數未載明** | 【實測】`invoice_status` 3000 筆仍正常（93KB），但無官方承諾 | 不知道 batch size 設多少才安全；AMEGO 隨時可加限制 | 自訂 ≤200 筆分批；**且回應會依鍵值去重，必須用鍵值 join 不可索引對位** |
| **A10** | **`data` 欄位／POST body 的大小上限未載明** | 商品最多 9999 筆（S10），但沒說 body 上限；【實測】93KB 可通過 | 9999 筆的 JSON 可能達數 MB，是否會被拒不明；大批次對帳可能在某個規模突然失敗 | 自訂單張 ≤200 品項，超過改拆單，並實測邊界 |
| **A11** | **`track_get` 沒有冪等鍵，逾時後號段是否已配走未知** | 錯誤碼全部是 99，無任何說明 | 網路中斷時無法安全重試，**可能永久漏掉一個號段**（號段作廢會造成財政部端字軌不連續） | 避開 API 配號模式；若必須用，逾時後靠 `track_status` 的 `now` 比對 |

### 5.B 網路與環境

| # | 未載明的事 | 實查證據 | 不知道會導致什麼 | 建議處置 |
|---|---|---|---|---|
| **B1** ⚑ | **`code 14 IP 錯誤` 的白名單機制**：如何登記、是否預設啟用、能登幾組、支不支援 CIDR、以哪個 IP 為準 | 通用錯誤碼表有這一行，但對 S3–S23 全文與 `api_data.json` grep `IP` **只有錯誤碼表那一行命中** | **若正式帳號啟用 IP 綁定，k8s pod 的浮動出口 IP 會整批失敗**，而且會在「切正式的當下」才炸。這會直接決定要不要走固定出口 IP 方案 | **申請正式 App Key 時必須同時問清楚。** 若需要白名單，走固定出口 IP（本 repo 有 egress-proxy 機制）。【實測】測試帳號從一般家用／辦公網路打全部成功 ⇒ 測試帳號至少沒啟用 |
| **B2** ⚑ | **測試環境的發票是否真的上傳財政部** | 文件只說「測試環境不會主動發送信件」，沒說會不會上傳 | 【實測】測試公司的發票 `invoice_status` 會跑到 `2 上傳中`，但沒有證據說明最終會不會進財政部。**若會，測試資料就是真發票，QA 時不能亂開** | 向客服確認；在確認前，QA 開的每一張測試發票都要作廢 |
| **B3** | **測試／正式共用 base URL，且回應中沒有任何環境標記** | 【文件逐字】「目前測試或正式都請用同一個 API 網址」；【實測】回應無環境欄位 | config 打錯 → **在 prod 開出測試發票，或在測試環境用 prod 統編開出真發票**，而且**無法從回應辨識** | 我方自建斷言：非 prod 環境時 `AMEGO_INVOICE` 必須 == `12345678`，在 config `Validate()` fail-fast |
| **B4** | **是否支援 keep-alive / HTTP/2 / 連線池上限** | 全文無 | 連線池參數只能靠實測調 | 實測後定案 |
| **B5** ⚑ | **正式 App Key 的申請／輪替／撤銷流程** | 只寫「請與客服聯絡」。沒說是自助後台產生還是客服人工發放、能否自行 rotate、能否多組並存、外洩時如何撤銷 | **secret 輪替方案無法設計**；若 key 只能由客服人工更換，**事故應變 RTO 取決於客服上班時間**（平日 10-12、14-18） | 向客服確認；secret 進 secret store；規劃輪替時的雙 Key 過渡（若不支援雙 Key，要規劃停機窗口） |
| **B6** | **App Key 的長度、字元集** | 範例值 20 碼英數大小寫混合，無規則說明 | DB／secret 欄位長度無依據 | 【推定】`VARCHAR(128)`，存 k8s Secret |
| **B7** | **停機公告的取得方式** | `code 10` 只在事發時才知道；官網彈窗曾公告 2024-12-21 02:00–06:00 停機，**無 RSS／API／訂閱** | 無法預先避開維護窗口；維護窗吃掉的時間仍算在法定 48h／7 日內 | `10` 視為可重試但需告警；人工訂閱官網公告 |
| **B8** | **測試公司的資料保留／清理政策** | 測試統編 `12345678` 是**所有串接者共用**的公開帳號（【實測】有 7 萬+ 筆他人資料），文件沒說測試資料保留多久、會不會被清 | 測試資料可能隨時消失，或被別人的測試干擾；也代表**我方的測試資料對其他串接者可見** | 測試資料不得含真實個資；QA fixture 要能重建 |

### 5.C 金額與捨入

| # | 未載明的事 | 實查證據 | 不知道會導致什麼 | 建議處置 |
|---|---|---|---|---|
| **C1** ⚑ | **`Round()` 的捨入模式** | 官方算式只寫 `Round(...)`，沒說四捨五入／banker's rounding／無條件捨去 | `TaxAmount = SalesAmount - Round(SalesAmount / 1.05)` 在 `.5` 邊界會差 1 元 ⇒ 回 `3040177 TaxAmount 計算錯誤`，整張發票開不出來 | **必須用 .5 邊界值實測定案**（找出使 `/1.05` 落在 .5 的 SalesAmount）。實測的 168 案例兩種模式結果相同、**沒有鑑別力** |
| **C2** | **明細 7 位小數如何在合計時處理** | 只說「小數精準度到7位數」，沒說合計時的 Round 位置 | 我方金額是整數元、禁浮點。若明細出現非整數，`SalesAmount` 的 Round 位置會決定差額。用 `DetailAmountRound=1` 可迴避，但**會改變送給 AMEGO 的明細金額**，可能與我方訂單金額對不起來 | 定案前先實測；設計書要明訂「明細金額以何者為準」 |
| **C3** ⚑ | **`TotalAmount` 是否允許 0（0 元發票）** | 有 `3040152 TotalAmount 不可為負數`、`3040184 不應該為負數`，但**沒說 0 是否合法**；另一方面文件大量提到「0元發票」的行為（qrcode 空字串、不回 base64_data、無法列印正本／補印、錯誤碼 56） | 從「0元發票」被反覆提到可**推測**可以開，但**沒有正面敘述**。全額折扣／全額優惠券的訂單要不要開票、能不能開，會直接影響結帳流程分支 | 上線前實開一張 0 元測試發票確認 |
| **C4** | **負數明細的規則** | 官方範例有 `"UnitPrice":"-2"`（會員折抵），但**無任何說明負數的限制** | 多行負數、負數大於正數合計、負數在免稅／零稅率品項上會怎樣，全部未知 | 實測邊界；折扣一律做成單一負數品項列 |
| **C5** | **`TaxRate` 是否只能 `"0.05"`；`TaxType=4 應稅(特種稅率)` 的稅率清單** | 只說「為5%時本欄位值為0.05」；特種稅率有 enum 值但**沒有稅率清單** | 若未來有特種稅率商品（如特種飲食業），稅率值無從得知 | 目前不支援 `TaxType=4`；需要時向客服索取 |
| **C6** | **合計欄位的數值上限與小數位** | 明細講了小數 7 位，`SalesAmount`/`TaxAmount`/`TotalAmount` 沒講 | 超大金額（例如企業大單）可能踩到未知上限 | 以 `BIGINT`（元）保守設計，超大金額前先實測 |

### 5.D 欄位長度與型別

| # | 未載明的事 | 實查證據 | 不知道會導致什麼 | 建議處置 |
|---|---|---|---|---|
| **D1** ⚑ | **`BuyerName` / `BuyerAddress` / `BuyerTelephoneNumber` / `BuyerEmailAddress` / `CarrierId1` / `CarrierId2` / `NPOBAN` / `BrandName` / `TrackApiCode` 的長度上限** | 錯誤碼只說「過長」（3040123–3040130、3040136、3040160），參數表**無數字** | **DB 欄位長度無法定案**。訂太短會截斷、訂太長會在送出時才被 AMEGO 拒（而且是在使用者按下結帳的當下） | 本檔 §4.1 的數字全是**實測**不是承諾。向客服索取 MIG 4.0 的欄位長度規格；設計書標為待確認 |
| **D2** | **`ProductItem.Remark` 到底 120 還 40** | `f0401`/`f0401_custom` 參數表寫「不可超過120字」；`invoice_query` 回傳欄位說明寫「不可超過40字」（本次以 `api_data.json` 覆核確認兩處並存） | **同一份文件自相矛盾**。取 40 保守值代表超過 40 字的備註會被截斷或報錯，**而行為未知** | 取 40；向客服確認；→ §6-1 |
| **D3** | **`AllowanceNumber` 到底 16 還 40** | bundle 內兩份 `g0401` 定義分別寫 16 與 40；錯誤碼 `4040121` 寫「超過16字」；【實測】17 字被拒 | 取 16 是安全的，但若真是 40，我方就浪費了編碼空間（折讓單號通常要塞訂單資訊） | 取 16；向客服確認；→ §6-2 |
| **D4** | **`CarrierType` 6 碼 vs `amego` 5 碼的分流規則** | 錯誤碼 `3040155 載具類別應為6碼`，但官方指定的光貿會員載具值是 `amego`（5 碼） | 檢查規則的適用範圍未知。**可能是只對 `3J0002`/`CQ0001` 這類財政部標準載具檢查**，但沒有文字說明 | 依載具種類分流驗證，**勿一律套 6 碼**；→ §6-7 |
| **D5** | **`invoice_date` 型別文件標 String、實際回 Number** | 文件：「發票日期 格式：YYYYMMDD，例如：20161028」型別 String；【實測】回 `"invoice_date":20260824`；官方 `invoice_list` 範例也是 Number | **強型別語言直接反序列化會失敗**。而且這代表**所有標 String 的欄位都需要重新確認實際型別**，不能照文件建 DTO | 收 Number 再自行轉字串；所有 DTO 欄位型別以實測為準 |
| **D6** | **`allowance[].invoice_type` 型別標 Number 但值是字串**；`allowance_type` 在 query 是 String、在 list 是 Number | `api_data.json` 覆核確認 | 同 D5 | 反序列化用 String + 寬鬆處理；→ §6-4 |
| **D7** | **`qrcode_left` / `qrcode_right` / `barcode` 的最大長度** | 全文無 | DB 只能用 `TEXT`；固定寬度 UI 也無從估算 | `TEXT`；**原樣存原樣出，不做任何轉義** |
| **D8** | **`code` 欄位的 JSON 型別不一致、數值範圍未載明** | 【實測】`f0501`／查詢類回 Number，`g0401`／`g0501`／`f0701` 回 **String**；有 7 位數碼（`3040171`）也有 2 位數（`99`） | 用 `int` 直接綁 DTO，會在**折讓的錯誤路徑**上 unmarshal 失敗——而那正是最需要讀到錯誤碼的路徑 | `code` 用能同時吃 number 與 string 的型別；DB 存 `VARCHAR(16)`；→ §6-6 |
| **D9** | **`msg` 的長度上限與字元集** | 全文無 | DB 欄位長度無依據 | 用 `TEXT`；**永遠不要用 msg 做分支** |

### 5.E 狀態與生命週期

| # | 未載明的事 | 實查證據 | 不知道會導致什麼 | 建議處置 |
|---|---|---|---|---|
| **E1** | **`invoice_status` / `allowance_status` 新舊兩種 response 形狀的切換條件** | 官方並列「回應成功(新版)」`data` 為陣列、「回應成功(舊版)」`data` 為以號碼為 key 的物件，**沒有版本參數、沒有 header、沒有生效日期**；【實測】目前回新版 | 若 AMEGO 依帳號或依時間切換，我方解析器會**突然爆掉**，且是在輪詢排程裡靜默失敗 | 解析器同時容忍兩種形狀（先判 `data` 是 array 還是 object），或至少對非預期形狀發告警 |
| **E2** | **`status = -1` 未列在欄位表的列舉值** | 欄位表只列 1/2/3/31/32/91/99；官方範例與【實測】都出現 `-1` | enum 若用嚴格解析會炸。**是否還有其他未列值未知** | enum 必須容納 `-1` **且要有 default 分支** |
| **E3** ⚑ | **從 `code:0` 到 `status:99` 要多久** | 全文無 SLA。官方 FAQ 逐字：「有時候快的話可能幾分鐘就上傳確認，但也有幾小時才被確認的，所以都不一定」 | 輪詢間隔與超時無從設計；「多久沒到 99 算異常」的閾值只能猜 | 輪詢退避（分鐘級起跳）；逾 N 小時未達 99 則告警。N 值向客服確認 |
| **E4** ⚑ | **`status:91 錯誤` 之後怎麼辦** | 只列了這個值，**沒說發生後怎麼辦、能不能重送、要不要作廢重開、發票號碼會不會被回收** | 這是需要人工介入的終態，但**修復路徑完全未知**——真的發生時沒有 runbook 可寫 | 落庫並直接告警到人；向客服確認處置流程 |
| **E5** | **作廢完成的判定與超時** | `f0501` 回 `{"code":0,"msg":""}`，【實測】之後 `invoice_status` 仍回原本的 `C0401`，作廢排進 `invoice_query` 的 `wait[]` | 「作廢 API 成功」≠「已作廢」。要輪詢到 `invoice_type` 變 `C0501`，但**輪詢多久算超時未知** | 自訂超時並告警 |
| **E6** ⚑ | **「發票修改期限」是多久**（`3050126`）與關帳日是否同一件事 | 錯誤碼有這個碼但無天數；S9 講關帳日、S13 講作廢期限，**沒有明說兩者是否同一個「修改期限」** | **退款流程的設計依賴這個數字**——不知道就無法在 UI 上告訴賣家「還剩幾天可作廢」 | 向客服確認；在確認前一律先查狀態再決定，接受 `3050126` 當終態 |
| **E7** ⚑ | **折讓的「修改期限」完全無說明**（`4050135`） | 發票作廢有明確的「跨期關帳日單月 5 日」規則（S13），**折讓沒有任何對應頁面** | 不能在 UI 準確告訴賣家「還剩幾天可作廢折讓」，只能事後接錯誤碼 | 向客服確認；【推定】與發票同為關帳日前，但**未經證實** |
| **E8** ⚑ | **`invoice_status` / `allowance_status` 的查詢時間窗完全未載明** | `invoice_query` 明寫「只能查詢180天內」，`invoice_status` **一個字都沒有**；【實測】對更早的發票回 `code 0` + `NOT_FOUND` + `status -1`，是**靜默 false negative**（§2.4.4 有 9 筆實證） | **這是本次調查發現的最危險的坑**：拿它當「沒開過」的判準會造成**重複開立發票**（而重複開立還有收費規範） | **設計書必須明令禁用它做存在性判斷**；冪等一律走 `invoice_query`／`allowance_query`。窗口規則（推測「當期+前兩期」）向客服確認 |
| **E9** | **`allowance_query` 是否真的無期限** | `invoice_query` 明寫 180 天且有錯誤碼 51；`allowance_query` **沒有對應敘述、也沒有 51 分節**；【實測】591 天前可查 | 實測可查**可能只是尚未清理**，不是承諾。若哪天加上限制，折讓對帳會靜默壞掉 | 不可假設折讓查得到更久；我方 DB 自行落地完整折讓內容 |
| **E10** ⚑ | **註銷（C0701）沒有公開 API** | `invoice_type` 有 `C0701`；`project.order` 列了 `Invoice_Void` 但**無對應端點定義**；【實測】`/json/f0701` 端點**實際存在**（錯誤碼族 `307xxxx`、參數名 `VoidInvoiceNumber`、`code` 回 String），但請求形狀／成功回應／可註銷條件**全部未知** | 需要註銷的情境**無法自動化**，只能走後台人工 runbook；且我方狀態機必須容納 `C0701` 這個過渡態 | **不要把註銷納入本期範圍**；要用必須向客服索取正式文件 |
| **E11** | **`order_id` 是否允許一單多張發票** | 錯誤訊息提到「正在註銷重開中」，暗示同一 `OrderId` 生命週期內可能對應多張發票號碼，但 `invoice_query(type=order)` **只回單一物件、不是陣列** | 註銷重開後查 order_id 拿到哪一張（舊的還是新的）**未定義** | 我方 DB 保存完整歷史；不依賴 AMEGO 回傳判斷 |
| **E12** | **`AllowanceDate` 的 API 端合法窗口是否等同後台 UI 的 2 天／7 天** | S14 描述的是後台操作介面的可選範圍；API 端只有 `4040122 AllowanceDate 錯誤` | 回填日期可能被拒但不知邊界 | **保守做法：一律送「今天」** |
| **E13** | **`file_url` 10 分鐘有效期的精確語意** | 「連結僅10分鐘有效」——是「產生後 10 分鐘」還是「首次存取後 10 分鐘」？過期後回什麼？（【實測】竄改 token 回 HTTP 200 + 中文純文字，但**未實測真正過期**） | 重試邏輯無法設計；且過期若同樣是 HTTP 200，我方必須以**內容**判斷而非狀態碼 | 一律以 `Content-Type` / magic bytes 判斷；取得後立即下載，不要延遲使用 |

### 5.F 業務語意

| # | 未載明的事 | 實查證據 | 不知道會導致什麼 | 建議處置 |
|---|---|---|---|---|
| **F1** | **B2C 與 B2B 沒有分開的端點，靠 `BuyerIdentifier` 判別**——但這點文件沒有正面說明 | 是從欄位說明反推的 | 「打統編／不打統編」造成 `TaxAmount`、`SalesAmount`、`DetailVat`、載具可用性**四處連動改變**，但**沒有一段文字把這個因果講清楚** ⇒ 容易漏改其中一處，而漏改的症狀是 `3040174`（萬用錯誤，訊息還會誤導） | 我方封裝成兩個明確的 usecase（B2C / B2B），不要讓呼叫端自己組欄位 |
| **F2** ⚑| **「載具 / 捐贈 / 統編」三者互斥的規則只寫在 API 配號那節，自動配號那節沒有** | `f0401_custom` 錯誤碼明列「列印發票不可設定載具／捐贈碼」「捐贈發票不可設定載具」；`f0401` **沒有這三條，且【實測】真的不擋**（三種並存組合全部通過） | **並存時 AMEGO 會怎麼處理（以哪個為準）、財政部會不會退件，文件完全沒說。** 開出去才發現有問題就只能作廢重開 | **由我方在後端強制三選一**；並向客服確認並存時的實際行為 |
| **F3** | **`f0401` 沒有 `PrintMark` 欄位，那它算「列印」還是「不列印」？** | `f0401_custom` 有 `PrintMark` 必填，`f0401` 沒有 | 帶了 `PrinterType` 之後是否就變成「列印」而不能用載具？**未載明** | 向客服確認；在確認前不要同時帶 `PrinterType` 與載具 |
| **F4** | **統編有效性驗證的責任歸屬** | `3040122 BuyerIdentifier 格式錯誤`（只驗檢查碼）；S7 說 `ban_query`「並非百分之百都查詢的到名稱」「管委會、會計事務所..等統編，都不在資料集內」 | **買方統編打錯（格式正確但不存在）會開出一張錯誤的 B2B 發票**，只能事後作廢。且**不可用 `ban_query` 查不到當作統編無效**，那會擋掉合法買方 | 前端做檢查碼驗證 + 抬頭確認 UI；`ban_query` 只用於「顯示抬頭供人核對」不用於「阻擋」 |
| **F5** ⚑| **捐贈碼沒有任何格式說明，AMEGO 也不驗存在性；清單取得沒有自動化管道** | 參數表只有「捐贈碼」三個字；【實測】`3040137「NPOBAN 不存在」`其實只是「不是 3–7 碼純數字」，`"999"`/`"000"` 一律通過；官方要你自己去財政部 ODS 抓（E3/E4），**其中 CSV 下載需要圖形驗證碼** | 打錯的捐贈碼會**成功開出發票**，捐贈失效且**不可逆**（只能作廢重開）。而清單的自動化更新**沒有官方支援路徑** | 我方自建捐贈碼清單 + `^[0-9]{3,7}$` 驗證；清單更新方式（人工／爬蟲）需先決策 |
| **F6** | **自訂會員載具的申請流程未載明** | 只有一句「載具類別亦可帶入貴公司自行申請的會員載具」 | 沒說去哪申請、代碼怎麼配、要不要報備財政部 | 需要時向客服詢問 |
| **F7** | **賣貨便 / 好賣+ / mo店+ 三平台的會員載具格式未載明** | S6 的 regex 表只有 6 列（手機/信箱/蝦皮/露天/Yahoo/8591）；S11 的查詢連結規則卻列了這三個平台的 type | 若要支援這三個來源的載具，**格式規則無據可循** | 需要時向客服索取 |
| **F8** | **折讓 `g0401` 的 `BuyerEmailAddress` 是否觸發寄信** | 說明只有「買方電子信箱」五個字，**沒有 `f0401` 的寄信說明** | 無法確定開折讓時買家是否會收到通知信 ⇒ 退款流程的通知設計會**落空或重複** | 向客服確認；在確認前由我方自行寄退款通知 |
| **F9** | **開立通知信的觸發時機、內容格式、寄件網域、重寄機制** | 只說會寄（正式環境）、測試不寄、後台有「補發發票開立通知」按鈕（**且只有當期的可以補發**）。**沒有樣張、沒有寄件人網域、沒有說是否附 PDF、沒有 API** | 無法評估是否要改用我方自建寄信；也無法預先設定 SPF/DMARC 相關的客訴預期；**「重寄」後端無法自動化** | 若需要重寄功能，改為我方自建寄信（取 PDF 後自寄） |
| **F10** ⚑| **`invoice_lottery` 的實際值域（80~88）沒有任何對照表** | 文件只說「獎項代碼，請參考 [獎項定義] API」，但【實測】1,500 筆抽樣全部落在 80~88，**與 `lottery_type` 的 11~23 零重疊**；官方 `invoice_list` 範例本身就是 `87`；另一輪實測新發票為 `0` | **文件的指引在實際資料上不成立**。用 enum 嚴格解析會在解析階段就炸；也無法判斷 `87` 是「已對獎未中」還是別的狀態 | **存原始整數，禁止 enum 嚴格解析**；顯示層 fallback；向客服索取 80~88 與 0 的定義 |
| **F11** | **中獎查詢的資料保留期限；`order_query` 頁的 `type=103`** | 只有「建議雙月1號才查詢」的建議；`type=103`（蝦皮宅配訂單）**只存在於實際頁面下拉選單，S11 完全沒記載** | 中獎通知的排程時點只能照建議做，沒有保證；type 列舉值不完整 | 照官方建議排程；type mapping 以實際頁面為準 |
| **F12** | **一張折讓單只能對一張發票的建模影響** | 只出現在錯誤碼 `4040132`/`4040133`，參數表沒提 | 一筆訂單若開了 N 張發票，退款要開 N 張折讓單。**若資料模型設成「折讓單:訂單」會整個做錯** | 資料模型必須是「折讓單 : 發票 = 1 : 1」 |
| **F13** | **錯誤碼表本身不完整，且有 4 支 API 沒有錯誤碼分節** | 【實測】`3040113`（ProductItem 資料應為陣列）與 `4040145`（AllowanceType 不可為 1）**都不在官方表內**（本次 curl 重驗 mid=71 確認 `3040113` grep 命中 0）；S4 目錄**沒有** `[折讓 - 折讓查詢]`、`[折讓 - 折讓列表]`、`[折讓 - 折讓列印]`、`[中獎 - *]` 分節 | **程式不可假設「回來的 code 一定在文件表內」**——用 exhaustive switch 會漏；那 4 支的錯誤碼要靠實測補齊 | 錯誤碼對照必須有「未知碼」的 fallback 分支；上線前對那 4 支做錯誤路徑實測 |
| **F14** ⚑ | **B2B（三聯式／交換發票）是否為另一份文件** | bundle 內出現「B2B 交換發票開立／作廢／退回」「B2B 存證發票…」等 `invoice_type` 字串，但 24 支 API 定義中**沒有對應端點**；header 也只說「B2C 及 B2B API 的欄位內容則需編碼成 JSON…」 | 若需要開立三聯式／B2B 交換發票，**不能假設用同一組端點** | 向客服索取 B2B 文件 |

### 5.G 契約穩定性

| # | 未載明的事 | 實查證據 | 不知道會導致什麼 | 建議處置 |
|---|---|---|---|---|
| **G1** | **沒有 API 版本化機制；舊路徑無退場日期** | 所有 API 的 `version` 都是 `1.0.0`；路徑改名（c0401→f0401）後只說「舊版 API 目前尚可繼續使用」，**無停用日期** | 舊路徑何時停用未知 | **新程式一律用 f/g 路徑**，不要為了相容留兩套 |
| **G2** | **沒有機器可讀的 changelog** | 只有 `info_detail?mid=76` 一個 HTML 頁，無 RSS／feed | **AMEGO 會在不改版號的情況下加欄位／改路徑**（2024.10.24 加 `allowance[]`、2024.12.13 加零稅率原因、2025.01.14 改 9999 筆）⇒ 無法自動監控契約變動 | 反序列化**必須容忍未知欄位**（Go 預設忽略；Java Jackson 設 `FAIL_ON_UNKNOWN_PROPERTIES=false`）；做一支排程抓那頁 diff 告警 |
| **G3** | **文件本身沒有版本標記可對照** | bundle 內 `generator.time = Wed Jun 10 2026 11:24:08 GMT+0800`，但那是 apiDoc 產生時間不是契約版本 | 無法判斷「文件有沒有變」 | 用 bundle 的 md5 當基準做 drift 偵測（本次 `195b40448cc2ddc19b96454a8e2e5f70`，與線上版位元相同） |
| **G4** | **文件的 optional/required 標示不可信** | `invoice_query` 把 `order_id` 與 `invoice_number` **都標成 REQUIRED**，但說明寫「擇一查詢」；`f0401_custom` 範例出現參數表沒有的 `SequenceNumber` | DTO 若照文件的必填標示建，會做出錯的驗證 | **必填性以實測為準**，不以文件標示為準 |
| **G5** | **錯誤碼一碼多義，無法程式化分支** | `invoice_file` 的 `53` 出現 2 次語意不同；`invoice_print` 的 `55` 出現 3 次；`track_get` 全部 7 種都是 `99`；`f0401_custom` 全部 50+ 種都是 `99`；`ban_query` 兩種都是 `99`；`3040171` 一碼兩義 | **無法只用 code 做錯誤分支，必須解析中文 msg，而 msg 不是契約** | 選型上避開 `f0401_custom`；必要的 msg 比對寫測試釘住；log 必須完整記錄 `api_path` + `code` + `msg` |
| **G6** | **`msg` 文字與文件表格不一致** | 文件 15「Time 錯誤」/16「簽名驗證錯誤」/17「資料不可為空」 vs 實際「time(時間戳記)錯誤」/「sign(簽名)驗證錯誤」/「data 不可為空」 | 照文件寫 msg 比對必壞 | 只用 `code` 分支；msg 原樣落庫 |

### 5.H 建議的「向光貿客服書面確認清單」（標 ⚑ 者彙整）

按優先序（上線前必答 → 上線後可補）：

**P0（不答就不能上正式）**
1. **B1** — 正式帳號是否啟用來源 IP 白名單？如何登記？可登幾組？支援 CIDR 嗎？（決定要不要走固定出口 IP）
2. **B5** — 正式 App Key 如何取得／輪替／撤銷？可否雙 Key 並行？外洩時多久能換掉？
3. **A6** — QPS／每分鐘上限是多少？`code 21` 的冷卻時間？有沒有可訂閱的配額資訊？
4. **F2** — 「打統編 / 載具 / 捐贈」三者並存時，`f0401` 實際會怎麼處理？財政部會不會退件？
5. **E8** — `invoice_status` / `allowance_status` 的查詢時間窗到底是什麼規則？（我方已實測為「約當期+前兩期」，需要官方確認）
6. **C1** — `Round()` 是四捨五入還是其他捨入模式？

**P1（影響設計但可先用保守值）**
7. **D1 / D2 / D3** — 各欄位的精確長度上限；`ProductItem.Remark` 到底 120 還 40；`AllowanceNumber` 到底 16 還 40
8. **E4** — `status = 91 錯誤` 之後的標準處置流程？可以重送嗎？發票號碼會被回收嗎？
9. **E6 / E7** — 「發票修改期限」「折讓修改期限」的精確天數？與關帳日是同一件事嗎？
10. **A2 / A5** — 官方建議的 timeout 與重試策略？逾時但實際已開立時，除了 `invoice_query` 還有別的對帳方式嗎？
11. **F10** — `invoice_lottery` 的 80~88 與 0 各代表什麼？
12. **B2** — 測試環境的發票會不會真的上傳到財政部？

**P2（補齊文件缺口）**
13. **C3** — 0 元發票是否合法？
14. **F5** — 捐贈碼清單有沒有不需圖形驗證碼的取得管道？
15. **F7** — 賣貨便／好賣+／mo店+ 的會員載具格式 regex？
16. **F8** — 折讓的 `BuyerEmailAddress` 會不會觸發寄信？
17. **F14** — B2B 交換發票（三聯式）有沒有另一份 API 文件？
18. **E10** — `/json/f0701`（註銷）的正式規格？
19. **A8 / A9 / A10** — 批次寫入的部分成功語意、批次查詢上限、body 大小上限？
20. **F13** — 錯誤碼表可否補齊（`3040113`、`4040145`）並補上折讓查詢／列表／列印與中獎的錯誤碼分節？

---

## 6. 交叉檢查 —— 文件之間互相矛盾的地方

> 每一條都列出**兩邊各自說什麼**、**出處 URL**、以及**本檔採用哪一邊與理由**。
> ⚠️ 官方 API 文件是 SPA，所有 API 定義的「URL」都是同頁錨點，實體來源是同一個 bundle
> （`https://invoice-static.amego.tw/www/api_doc/assets/main.bundle.js?2026061001`）。
> 因此下表的「出處」同時給**錨點 URL** 與**在 bundle 內的位置**（來源檔名／欄位路徑），才能精確定位。

| # | 矛盾點 | A 說法（出處） | B 說法（出處） | 判定與理由 |
|---|---|---|---|---|
| **1** | **`ProductItem.Remark` 長度：120 vs 40** | 「備註，**不可超過120字**」<br>`POST /json/f0401` 與 `/json/f0401_custom` 的 parameter<br>https://invoice.amego.tw/api_doc/#api-%E7%99%BC%E7%A5%A8-Invoice<br>（bundle：`c0401.php` / `c0401_custom.php` → `parameter.fields.ProductItem.Remark`） | 「備註，**不可超過40字**」<br>`POST /json/invoice_query` 的 success 欄位 `data.product_item.remark`<br>https://invoice.amego.tw/api_doc/#api-%E7%99%BC%E7%A5%A8-Invoice_Query<br>（bundle：`invoice_query.php` → `success.fields.…remark`） | **取 40 保守值**。寫入端宣告 120、讀取端宣告 40，若真是 40 則超過會被截斷或報錯，行為未知。已列 §5-D2 待確認 |
| **2** | **`AllowanceNumber` 長度：16 vs 40** | 「折讓單編號，不可重複，**不可超過16字**」<br>`POST /json/g0401`（bundle 來源檔 **`d0401.php`**）<br>https://invoice.amego.tw/api_doc/#api-%E6%8A%98%E8%AE%93-Allowance<br>另 `allowance_query`／`allowance_file`／`allowance_print` 的 `allowance_number` 亦寫「不可超過16字」，錯誤碼 `4040121` 亦寫「超過16字」 | 「折讓單編號，不可重複，**不可超過40字**」<br>同一個 `POST /json/g0401` 的**第二份定義**（bundle 來源檔 **`d0401_custom.php`**，group/name/version 完全相同）<br>（本次以 `api_data.json` 覆核：兩筆並存） | **取 16**。理由三重：① 錯誤碼表寫 16 ② 三支查詢類 API 都寫 16 ③【實測】17 字被拒（`4040121`）。`d0401_custom.php` 那份還把 `AllowanceType=1` 寫成仍可用（也是舊的）⇒ **視為殘留** |
| **3** | **折讓品名長度：256 vs 500** | 「品名，**不可超過256字**」<br>`g0401` 的 `ProductItem.OriginalDescription`（寫入端）<br>＋ `allowance_query` 的 `data.product_item.description`<br>https://invoice.amego.tw/api_doc/#api-%E6%8A%98%E8%AE%93-Allowance_Query | 「品名，**不可超過500字**」<br>`allowance_list` 的 `data.product_item.description`<br>https://invoice.amego.tw/api_doc/#api-%E6%8A%98%E8%AE%93-Allowance_List | **取 256**（寫入端的宣告 + 兩票對一票）。已列 §5-D2 待確認 |
| **4** | **`allowance_type` 型別：String vs Number；`allowance[].invoice_type` 標 Number 但值是字串** | `allowance_query.data.allowance_type` 型別 **String**<br>`invoice_query.data.allowance.invoice_type` 型別 **Number**（值卻是 `D0401`）<br>https://invoice.amego.tw/api_doc/#api-%E7%99%BC%E7%A5%A8-Invoice_Query | `allowance_list.data.allowance_type` 型別 **Number**<br>https://invoice.amego.tw/api_doc/#api-%E6%8A%98%E8%AE%93-Allowance_List<br>（本次以 `api_data.json` 覆核兩處型別確實不同） | **DTO 必須寬鬆解析**：`allowance_type` 兩種都吃；`invoice_type` 一律當 String（值域是 `D0401` 這類字串，型別標示是錯的） |
| **5** | **`invoice_date` 型別：文件 String vs 實際 Number** | 文件：「發票日期 格式：YYYYMMDD，例如：20161028」，type **String**<br>`invoice_query.data.invoice_date`（bundle `invoice_query.php`） | 【實測 2026-08-24】回 `"invoice_date":20260824`（**Number**）<br>且**官方自己的 `invoice_list` response 範例**也是 `"invoice_date": 20230101`（無引號）<br>https://invoice.amego.tw/api_doc/#api-%E7%99%BC%E7%A5%A8-Invoice_List | **以實測為準：Number**。收 Number 再自行轉字串。⚠️ 連帶推論：**所有標 String 的欄位都需重新確認實際型別** |
| **6** | **`code` 的 JSON 型別：Number vs String** | 文件所有 API 的 success 欄位表：`code` 型別 **Number**<br>（bundle 全部 24 筆定義一致） | 【實測 2026-08-24】<br>`POST /json/f0501` → `{"code":3050125,...}` Number<br>`POST /json/g0501` → `{"code":"4050134",...}` **String**<br>`POST /json/g0401` → `{"code":"4040121",...}` **String**<br>`POST /json/f0701` → `{"code":"3070111",...}` **String**<br>`POST /json/allowance_status` / `allowance_query` → Number | **兩者都會出現，DTO 必須同時吃**。用 `int` 直接綁會在**折讓寫入的錯誤路徑**上 unmarshal 失敗——而那正是最需要讀到錯誤碼的路徑 |
| **7** | **`CarrierType`「應為6碼」vs `amego` 是 5 碼** | 錯誤碼 `3040155「載具類別應為6碼」`<br>https://invoice.amego.tw/info_detail?mid=71 | `f0401` 參數說明指定「光貿會員載具 **amego**」（5 碼）<br>https://invoice.amego.tw/api_doc/#api-%E7%99%BC%E7%A5%A8-Invoice | **兩者都真**：【實測】`amego` 是唯一被特別放行的 5 碼值（`AMEGO`/`Amego` 會回 3040155，大小寫敏感）。⇒ **6 碼規則只適用於財政部標準載具與自訂載具，`amego` 是硬編碼例外。驗證邏輯必須依載具種類分流** |
| **8** | **熱感應機支援表：二手抄錄與官方原表不符** | 二手抄錄：代碼 3–12 的「設定是否列印明細」欄為 **V**、「折讓單」欄**空白** | **官方原表**（2026-08-24 本次 curl + HTML 表格解析）：代碼 3–12 的「設定是否列印明細」欄**空白**、「折讓單」欄為 **V**；只有代碼 2 兩欄皆 V<br>https://invoice.amego.tw/info_detail?mid=77 | **以官方原表為準**（本檔 §3.17 已更正）。這也與參數說明「`PrintDetail` 目前僅支援 PrinterType >= 2」不一致——參數說明說 ≥2 都支援，表格卻只有 2 打勾 ⇒ **這一項本身也需向客服確認** |
| **9** | **`PrinterType` 代碼 2 的機型名稱** | 「2 Xprinter **芯燁通用**」<br>https://invoice.amego.tw/info_detail?mid=77 | 「2：**芯燁XP-Q90EC**」<br>`f0401_custom` 的 `PrinterType` 參數說明<br>https://invoice.amego.tw/api_doc/#api-%E8%87%AA%E8%A1%8C%E9%85%8D%E8%99%9F%E5%B0%88%E7%94%A8-Invoice_Custom | **以 mid=77 的機型支援功能表為準**（它是被三支 API 共同引用的權威表）。`f0401_custom` 的說明是舊的（只列 1、2） |
| **10** | **`PrinterLang` 值域：3 值 vs 2 值** | 「1：BIG5 2：GBK **3：UTF-8**」<br>`f0401` / `invoice_print` / `allowance_print` 參數說明 | 「1：BIG5 2：GBK」（**沒有 3**）<br>`f0401_custom` 參數說明 | **以三值為準**。mid=77 的編碼欄確實有機型支援 UTF-8（代碼 2、9）⇒ `f0401_custom` 的說明是舊的 |
| **11** | **`ProductItem.TaxType` 必填性** | `f0401`：**必填**（`optional=False`）<br>（bundle `c0401.php`） | `f0401_custom`：**optional**，「課稅別，**預設為應稅**」<br>（bundle `c0401_custom.php`） | **兩支確實不同，不是抄錯**（已用 `api_data.json` 覆核）。⇒ 兩支的 DTO 必填性不可共用 |
| **12** | **訂單編號欄名與必填性** | `f0401`：**`OrderId`**（大寫 O），**必填** | `f0401_custom`：**`order_id`**（小寫底線），**optional** | **兩支確實不同**。⇒ request DTO 不可共用；mapping 要分開寫 |
| **13** | **`data` 非 JSON 時的錯誤碼：20 vs 17** | 通用錯誤碼表：`20 data 非 JSON 字串格式`<br>https://invoice.amego.tw/info_detail?mid=71 | 【實測】送 `data=abc` / `data=notjson{` / `data=not-a-json` → 一律回 `{"code":17,"msg":"data 不可為空"}`（**不是 20**） | **兩個碼都要能處理**。實務上 17 才是實際會遇到的；`20` 可能只在某些特定形狀下觸發 |
| **14** | **錯誤訊息 `msg` 文字與文件表格不一致** | 文件表格：`15 Time 錯誤`／`16 簽名驗證錯誤`／`17 資料不可為空`<br>https://invoice.amego.tw/info_detail?mid=71 | 【實測】實際回：`time(時間戳記)錯誤`／`sign(簽名)驗證錯誤`／`data 不可為空`<br>（`11`/`12`/`23` 則一致） | **msg 不是契約。所有分支只准用 `code`**，msg 原樣落庫／原樣顯示 |
| **15** | **`status` 列舉不含 `-1`，但官方自己的範例出現 `-1`** | `invoice_status.data.status` 欄位說明：「1：待處理 2：上傳中 3：已上傳 31：處理中 32：處理完成／待確認 91：錯誤 99：完成」（**無 -1**） | 同一支 API 的官方 response 範例：`"type": "NOT_FOUND", "status": -1`<br>https://invoice.amego.tw/api_doc/#api-%E7%99%BC%E7%A5%A8-Invoice_Status<br>（`allowance_status` 同）；【實測】確認 | **enum 必須容納 `-1` 且要有 default 分支**。是否還有其他未列值未知 |
| **16** | **`invoice_status` / `allowance_status` 同時列出兩種不同的 `data` 形狀，都標「回應成功」** | 「回應成功(新版)」：`data` 是**陣列** | 「回應成功(舊版)」：`data` 是**以號碼為 key 的物件**<br>兩者並列於同一支 API 的 success examples，**無切換條件說明** | 【實測 2026-08-24】目前回**新版陣列**。⇒ **解析器必須兩種都吃**（先判 array/object），否則哪天切回舊版整條輪詢靜默失效 |
| **17** | **`download_style`：說「四種樣式」卻列五項；且驗不驗值域兩支不同** | `invoice_file`：「有打統編發票，**四種**樣式 0/1/2/3/**5**」（實列 5 項，**沒有 4**）；錯誤碼表**沒有** `34`；【實測】傳 9 仍回 `code 0` | `allowance_file`：「**三種**樣式 0/1/**3**」（沒有 2、沒有 5）；錯誤碼表**有** `34 download_style 參數錯誤` | **兩者值域不同，不可共用 enum**；且 `invoice_file` **不驗值域**，必須我方白名單 `{0,1,2,3,5}` 擋 |
| **18** | **同一支 API 的錯誤碼表出現重複 code、語意不同** | `invoice_file`：`53` 出現 **2 次**（「載具發票，中獎後才可以下載」／「該發票類型不可下載」） | `invoice_print`：`55` 出現 **3 次**、`53` 出現 2 次<br>https://invoice.amego.tw/info_detail?mid=71 | **官方表格就是這樣重複列出，不是抄錯**。⇒ **不可只靠 code 分支，log 必須同時記錄 `msg`** |
| **19** | **`f0501` 的 `CancelReason`：參數表沒有，錯誤碼卻有** | `f0501` parameter 只列 `CancelInvoiceNumber` | 錯誤碼 `3050113 第n筆 CancelReason 超過 20 字`（本次 curl 重驗 mid=71 **命中**）<br>【實測】送 21 字 → 3050113；送 20 字 → 通過長度檢查往下走 | **`CancelReason` 確實存在，optional，上限 20 字**。我方 DTO 應加入（作廢原因對帳有用） |
| **20** | **錯誤碼表不完整：實測存在的碼不在表內** | 官方錯誤碼表（本次 curl 全文 grep）：**沒有** `3040113`（grep 命中 0）、**沒有** `4040145`<br>https://invoice.amego.tw/info_detail?mid=71 | 【實測】`ProductItem: []` → `{"code":3040113,"msg":"ProductItem 資料應為陣列"}`<br>【實測】`AllowanceType=1` → `{"code":"4040145","msg":"第1筆 自114年1月1日起，AllowanceType 不可為 1 買方開立折讓證明單"}` | **錯誤碼表本身不完整。程式不可假設「回來的 code 一定在表內」**，必須有未知碼 fallback 分支 |
| **21** | **`ban_query` 官方範例的公司名稱已過時** | 官方範例：`{"ban":"96999966","name":"紅磚數位**有限公司**"}` | 【實測 2026-08-24】回 `{"ban":"96999966","name":"紅磚數位**股份**有限公司"}` | **資料是即時的，文件範例已過時**。不要拿範例值寫進測試斷言 |
| **22** | **`invoice_lottery`：文件叫你查 `lottery_type`，但兩者值域零重疊** | 文件：`invoice_lottery`「獎項代碼，請參考 [獎項定義] API」；`lottery_type` 實際回傳 **11~23** | 【實測】抽樣 1,500 筆 `invoice_list`，`invoice_lottery` 全部落在 **80~88**（87 佔 915 筆），**零筆落在 11~23**；官方 `invoice_list` 範例本身就是 `87`；另一輪實測新開發票為 `0` | **文件的指引在實際資料上不成立。** 存原始整數、禁止 enum 嚴格解析、顯示層 fallback；80~88 與 0 的定義需向客服索取 |
| **23** | **`invoice_query` 的兩個查詢參數都標 REQUIRED，說明卻寫「擇一查詢」** | parameter 表：`order_id` 與 `invoice_number` **皆 `optional=False`（必填）** | 同一份說明文字：「查詢類型 order：訂單編號 invoice：發票號碼，**擇一查詢**」；【實測】只帶其中一個皆回 `code:0` | **以實測與說明文字為準：擇一**。⇒ **文件的 optional 旗標不可信**，DTO 不可把兩個都設 NOT NULL |
| **24** | **`BuyerName`「不可填0、00、000及0000」但實測不擋** | `f0401` 參數說明：「3. **不可填0、00、 000及0000**」 | 【實測】`BuyerName="0000"` → **通過開票** | **規則是真的（財政部要求），但 AMEGO 不強制。我方必須自己擋** |
| **25** | **光貿會員載具 regex 與實作寬鬆度不符** | S6 規則表：手機載具 `^a09[0-9]{8}$`<br>https://invoice.amego.tw/info_detail?mid=73 | 【實測】`A0911222333`（大寫 A）**通過**、`0911222333`（無 `a` 前綴）**通過** | **AMEGO 實作比文件寬鬆。我方應以文件 regex 為準自行嚴格驗證**，否則會存進格式不一致的載具號（未來對帳困難） |
| **26** | **`lottery_type`：「data 欄位不需要傳入資料」但簽章基準未說** | 文件描述：「中獎的發票類型定義 **data 欄位不需要傳入資料 ***」 | 【實測】`data=""` + `sign=md5(""+time+key)` → `code 0`；<br>`data="{}"` + `sign=md5("{}"+time+key)` → **`code 16` 簽名驗證錯誤** | **伺服器固定以空字串驗簽。** 呼叫時 `data` 必須送空字串且 sign 用空字串算——**文件完全沒提這點** |
| **27** | **180 天邊界的兩份實測敘述取樣粒度不同** | 抄錄 A：發票日 **2026-02-25（−180 天）→ `code 0`**；**2026-02-24（−181 天）→ `code 51`** | 抄錄 B：**20260226（−179 天）→ `code 0`**；**20260220（−185 天）→ `code 51`** | **兩者不衝突，只是取樣粒度不同**（後者沒測到 −180/−181 的精確切點）。**結論一致且採用前者的精確邊界：發票日 ≥ 今日−180 天（含第 180 天）** |
| **28** | **`invoice_status` 批次「回傳筆數 ＝ 送出筆數」的宣稱** | 抄錄 A：「實測單次送 50/300/1000/3000 筆皆 code 0 且**回傳筆數與送出筆數相同**」 | 抄錄 B：送 **600 筆重複**號碼**只回 1 筆**；`ban_query` 送 500 筆重複統編也只回 1 筆 | **前提不同**（相異鍵值 vs 重複鍵值），但該宣稱**在有重複值時不成立**。⇒ **採用保守結論：回傳筆數不保證等於請求筆數，必須用鍵值 join，不可索引對位** |
| **29** | **`AllowanceType=1` 在不同頁面的可用性標示** | 查詢類 API（`allowance_query` / `allowance_list` / `invoice_query.allowance[]`）的說明**平鋪列出**「1:買方開立折讓證明單 2:賣方折讓證明通知單」，沒有任何廢止標記 | `g0401` 的參數說明把「1」加了**刪除線**並註明 114 年新制；【實測】送 1 回 `4040145` 明確拒絕 | **寫入端只能用 2**；**讀取端仍須容納 1**（歷史資料可能有）。兩邊不是矛盾，是**寫入限制 vs 讀取相容**，但文件沒有把這層講清楚 |
| **30** | **`BuyerIdentifier` 長度標示 8 vs 實際要 10 個 0** | 部分頁面把 `BuyerIdentifier` 的長度標成 **8**（統編長度） | 同欄位說明寫「沒有則填入 **`0000000000`**」（**10 個 0**）；【實測】`00000000`（8 個零）回 `3040122 格式錯誤` | **DB 欄位長度必須 ≥10**（`VARCHAR(10)`）。且**不可對 8 碼統編做左補零**——補成 10 碼會被拒 |
| **31** | **`CarrierType` 標 optional，但空值時可能報「不可為空」** | parameter 表：`CarrierType` 為 **optional** | 錯誤碼 `3040154「載具類別不可為空」`；【實測】`CarrierType` 空但 `CarrierId1`/`CarrierId2` 有值即觸發 | **不是矛盾而是條件必填**：三個載具欄位要嘛全空、要嘛全填。文件沒有把這個條件寫出來 |
| **32** | **`BrandName` 標 optional，但錯誤碼說「不可為空」** | parameter 表：`BrandName` 為 **optional**，無長度說明 | 錯誤碼 `3040160「BrandName 不可為空或過長」` | **不是矛盾而是條件必填**：**一旦帶了這個欄位就不能是空字串**，要嘛不帶要嘛帶有效值 |
| **33** | **`f0401_custom` 的官方範例出現參數表沒有的欄位** | parameter 表沒有 `SequenceNumber` | 官方 request 範例的 `ProductItem` 內含 `"SequenceNumber": "1"` | **參數表不完整**。`SequenceNumber`（品項序號）可能被接受但無文件；不要依賴它 |
| **34** | **`allowance_file` 官方 response 範例的 URL 路徑錯誤** | 官方範例：`file_url` 為 `https://invoice.amego.tw/user/**invoice_print_type**?token=…`（＝發票的路徑） | 【實測】折讓實際回 `https://invoice.amego.tw/user/**invoice_d0401_print_type**?token=…` | **官方範例誤植**。⇒ **不要對 `file_url` 的路徑做硬編碼比對，直接使用回傳值** |
| **35** | **「載具發票中獎後才可下載」vs 中獎判定時點無說明** | `invoice_file` 描述：「載具發票**中獎後**才可下載，非載具發票可無限次下載」；錯誤碼 `53` 對應 | 中獎資訊要自己查 `lottery_status`（官方建議雙月 1 號後），**但 `invoice_file` 內部用什麼條件放行完全沒說** | 不是嚴格矛盾，是**缺口**：我方無法預判某張載具發票何時會變成可下載。已列 §5-F10 相關 |

### 6.1 矛盾的分佈與含意

- **「同一份文件兩處不同」型**（#1、#2、#3、#4、#10、#11、#12、#17）——多半是**寫入端 vs 讀取端**、或**新舊兩份定義並存**。
  處理原則：**取寫入端的宣告 + 取保守值**，因為寫入端才會擋你。
- **「文件 vs 實際行為」型**（#5、#6、#13、#14、#20、#21、#22、#23、#24、#25、#26、#34）——文件是舊的或錯的。
  處理原則：**以實測為準，但實測不是承諾**，要在設計書標註來源。
- **「文件內部不完整」型**（#15、#18、#19、#31、#32、#33、#35）——欄位表與錯誤碼表互為補充，兩邊都要讀。
  處理原則：**錯誤碼表是隱藏的參數規格書**，設計時必須逐條讀完。
- **「二手抄錄有誤」型**（#8、#27、#28）——用官方原始頁面／交叉比對定案，本檔已更正。
  處理原則：**引用二手抄錄前，對關鍵表格重抓一次原始頁面**（成本很低，本次三次覆核各只花數秒）。

---

## 7. 附錄

### 7.1 完整可執行範例（本次實跑過，可直接複製）

#### 7.1.1 Bash — 開立 B2C 發票 → 查詢 → 作廢

```bash
#!/usr/bin/env bash
set -euo pipefail

INVOICE="12345678"                    # 統編（正式改成貴公司統編）
APPKEY="sHeq7t8G1wiQvhAuIM27"         # App Key（正式向客服申請）
BASE="https://invoice-api.amego.tw"   # 測試/正式同一個 base URL

# 共用呼叫函式：sign = md5(data + time + appkey)，全部走 x-www-form-urlencoded
amego() {
  local path="$1" data="$2"
  local t s
  t=$(date -u +%s)                                   # Unix 秒，須與伺服器誤差 ±60 秒內
  s=$(printf '%s' "${data}${t}${APPKEY}" | md5 -q)   # Linux 用 `md5sum | cut -d" " -f1`
  curl -sS -X POST "$BASE$path" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    --data-urlencode "invoice=$INVOICE" \
    --data-urlencode "data=$data" \
    --data-urlencode "time=$t" \
    --data-urlencode "sign=$s"
  echo
}

OID="ORD-$(date -u +%Y%m%d%H%M%S)"    # OrderId 不可重複、不可超過 40 字

# ---- 1) 開立 B2C 發票（不打統編，含稅價，總計 168 元）----
DATA=$(cat <<JSON | tr -d '\n'
{"OrderId":"$OID",
 "BuyerIdentifier":"0000000000",
 "BuyerName":"客人",
 "BuyerAddress":"","BuyerTelephoneNumber":"","BuyerEmailAddress":"",
 "MainRemark":"","CarrierType":"","CarrierId1":"","CarrierId2":"","NPOBAN":"",
 "ProductItem":[
   {"Description":"測試商品1","Quantity":"1","UnitPrice":"170","Amount":"170","Remark":"","TaxType":"1"},
   {"Description":"會員折抵","Quantity":"1","UnitPrice":"-2","Amount":"-2","Remark":"","TaxType":"1"}
 ],
 "SalesAmount":"168","FreeTaxSalesAmount":"0","ZeroTaxSalesAmount":"0",
 "TaxType":"1","TaxRate":"0.05","TaxAmount":"0","TotalAmount":"168"}
JSON
)
echo "== f0401 =="; amego /json/f0401 "$DATA"

# ---- 2) 用 OrderId 查回來 ----
echo "== invoice_query =="; amego /json/invoice_query "{\"type\":\"order\",\"order_id\":\"$OID\"}"

# ---- 3) 作廢（把號碼換成上一步拿到的 invoice_number）----
# echo "== f0501 =="; amego /json/f0501 '[{"CancelInvoiceNumber":"ZA10063775","CancelReason":"測試作廢"}]'
```

⚠️ **`tr -d '\n'` 這步是必要的**：簽章算的是「送出去的那個字串」，多一個換行 md5 就不同。
⚠️ **B2B（打統編）版本**：把 `BuyerIdentifier` 換成統編、`BuyerName` 換成公司名，
並改成 `"SalesAmount":"160","TaxAmount":"8","TotalAmount":"168","DetailVat":1`。

**本次實跑輸出（2026-08-24 21:13 CST）**：
```
== f0401 ==
{"code":0,"msg":"","invoice_number":"ZA10063775","invoice_time":1787577229,"random_number":"9690",
 "barcode":"11508ZA100637759690",
 "qrcode_left":"ZA1006377511508249690000000a0000000a8000000001234567867y6BkUlQxCwz4v/rc5j6w==:**********:1:2:0:",
 "qrcode_right":"**測試商品1:1:170"}

== invoice_query ==
{"code":0,"msg":"","data":{"invoice_number":"ZA10063775","invoice_type":"C0401","invoice_status":2,
 "invoice_date":20260824,"invoice_time":"21:13:49","buyer_identifier":"0000000000","buyer_name":"客人",
 ... "random_number":"9690","order_id":"HKTEST-B2C-20260824131349","detail_vat":1,
 "product_item":[{"tax_type":1,"description":"測試商品1","unit_price":170,"quantity":1,"unit":"","amount":170,"remark":""},
                 {"tax_type":1,"description":"會員折抵","unit_price":-2,"quantity":1,"unit":"","amount":-2,"remark":""}],
 "wait":[],"allowance":[]}}

== f0501 ==
{"code":0,"msg":""}
```
（本次實測共開立 2 張測試發票 `ZA10063775` / `ZA10063776`，皆已於同一輪呼叫 `f0501` 作廢，未留下未處理資料。）

#### 7.1.2 Python 3（零依賴，可直接跑）

```python
#!/usr/bin/env python3
# 用法：python3 amego_call.py /json/f0501 '[{"CancelInvoiceNumber":"AB00001111"}]'
import hashlib, json, time, sys, urllib.parse, urllib.request

INVOICE = "12345678"                     # 統編（測試公司）
APP_KEY = "sHeq7t8G1wiQvhAuIM27"         # App Key（測試公司）
BASE    = "https://invoice-api.amego.tw"

def call(path, data_str):
    t = int(time.time())
    sign = hashlib.md5((data_str + str(t) + APP_KEY).encode("utf-8")).hexdigest()
    body = urllib.parse.urlencode({
        "invoice": INVOICE, "data": data_str, "time": t, "sign": sign
    }).encode("utf-8")
    req = urllib.request.Request(
        BASE + path, data=body,
        headers={"Content-Type": "application/x-www-form-urlencoded"})
    with urllib.request.urlopen(req, timeout=25) as r:
        return r.read().decode("utf-8")

if __name__ == "__main__":
    print(call(sys.argv[1], sys.argv[2]))

# 1) 手機條碼驗證（唯讀，可安心重跑）
#    call("/json/barcode", '{"barCode":"/TRM+O+P"}')
#    實測輸出：{"code": 0, "msg": ""}
#
# 2) 公司抬頭查詢（唯讀，可批次）
#    call("/json/ban_query", '[{"ban":"45444444"},{"ban":"96999966"}]')
#    實測輸出：{"code":0,"msg":"","data":[{"ban":"45444444","name":"光貿科技股份有限公司"},
#                                          {"ban":"96999966","name":"紅磚數位股份有限公司"}]}
```

**Go / Java 實作要點**：`data_str` **只序列化一次**，同一個字串同時拿去算 sign 與放進 form。
不要用「struct → 算 sign → 再 marshal 一次」的寫法（§1.3.4）。

#### 7.1.3 官方 PHP 範例（S3 逐字抄錄）

```php
<?php
$sUrl = 'https://invoice-api.amego.tw/json/f0401';
$sInvoice = '12345678';                 // 統編
$sApp_Key = 'sHeq7t8G1wiQvhAuIM27';     // APP KEY
$nTime = time();                        // Unix Timesatmp 10位數，不含毫秒
$aData = array( /* ...發票欄位... */ );
$sData_String = json_encode($aData, JSON_UNESCAPED_UNICODE);
$sSign = md5($sData_String . ((string) $nTime) . $sApp_Key);
$ch = curl_init();
curl_setopt($ch, CURLOPT_URL, $sUrl);
curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
curl_setopt($ch, CURLOPT_POST, true);
curl_setopt($ch, CURLOPT_POSTFIELDS, http_build_query(array(
    'invoice' => $sInvoice,
    'data'    => $sData_String,
    'time'    => $nTime,
    'sign'    => $sSign,
)));
$sOutput = curl_exec($ch);
curl_close($ch);
$aReturn = json_decode($sOutput, true);
```

#### 7.1.4 取發票 PDF 的完整流程（實測取得 323,646 bytes 的 PDF）

```bash
# 1) 取得 file_url（10 分鐘有效）
amego /json/invoice_file '{"type":"invoice","invoice_number":"AB10000316","download_style":0}'
# -> {"code":0,"msg":"","data":{"file_url":"https://invoice.amego.tw/user/invoice_print_type?token=..."}}

# 2) 下載 PDF —— 必須驗 Content-Type / magic bytes，錯誤時 AMEGO 仍回 HTTP 200
curl -sS -D /tmp/h.txt -o /tmp/inv.pdf "$FILE_URL"
grep -i '^content-type' /tmp/h.txt          # 必須是 application/pdf
head -c 5 /tmp/inv.pdf                       # 必須是 %PDF-
```

#### 7.1.5 最小成本的憑證有效性自檢（無副作用）

```bash
# (a) 免簽章，驗網路可達 + 校時
curl -sS https://invoice-api.amego.tw/json/time
# -> {"timestamp":1787578776,"text":"2026\/08\/24 21:39:36",...}

# (b) 故意對 ban_query 送物件（不是陣列）—— 回 code 23 就代表「簽章已通過」
amego /json/ban_query '{"ban":"45444444"}'
# -> {"code":23,"msg":"此 API 支援傳輸多筆資料，data 欄位資料應為陣列字串"}
#    若回 16 = 簽章錯；回 12 = 統編錯；回 15 = 時鐘偏移
```

### 7.2 實作前自查清單

**傳輸層**
- [ ] `Content-Type` 是 `application/x-www-form-urlencoded`（不是 json）
- [ ] `data` **只序列化一次**，同一字串同時用於 sign 與送出
- [ ] `time` 是 10 位整數 Unix 秒；**每次重試都重算 time + sign**
- [ ] 成敗**只看 body 的 `code`**，不看 HTTP status，不比對 `msg`
- [ ] `code` 的 DTO 型別能同時吃 Number 與 String
- [ ] envelope 至少三種形狀：標準 / 分頁 / `f0401` 頂層攤平；`/json/time` 另有第四種
- [ ] 反序列化容忍未知欄位（AMEGO 會就地加欄位）
- [ ] `invoice_status` / `allowance_status` 的 `data` 陣列與 map 兩種形狀都能解析
- [ ] `status` enum 容納 `-1` 且有 default 分支
- [ ] 打錯路徑會回 HTTP 200 + `error` 純文字 ⇒ 反序列化失敗要有獨立分支

**業務層**
- [ ] `OrderId` **送出前先落庫**，重試沿用同鍵；`3040171` 走查詢而非重送
- [ ] `3040171` 的兩種 msg 分開處理（永久衝突 vs 註銷重開中）
- [ ] `AllowanceNumber` 我方產生、≤16 字、唯一、**作廢後重開要換新號**
- [ ] 折讓金額**一律未稅**，`Tax`/`TaxAmount` 整數，尾差吸收到某一品項
- [ ] 折讓單 : 發票 = **1 : 1**
- [ ] 「打統編 / 載具 / 捐贈」三選一由**我方強制**
- [ ] 手機條碼先 regex 再 `/json/barcode`；自然人憑證只能 regex（AMEGO 不驗存在性）
- [ ] 捐贈碼比對我方維護的財政部清單（AMEGO 不驗）
- [ ] 統編用檢查碼演算法自驗；`ban_query` 只用於顯示抬頭不用於阻擋
- [ ] `BuyerIdentifier` 無統編填 `0000000000`（10 碼），**不可對 8 碼統編左補零**
- [ ] 金額算式與官方 `Round` 位置逐位元一致；用整數運算，禁浮點
- [ ] 折扣做成**負數品項列**，不是扣總額

**流程與可靠性**
- [ ] 開立成功 ≠ 上傳完成 ⇒ 輪詢 `invoice_status` 到 `99`（**不可用它判存在性**）
- [ ] 冪等對帳走 `invoice_query`（180 天內）／`invoice_list`（超過 180 天）／`allowance_query`
- [ ] 作廢／折讓前先確認原單 `status = 99`
- [ ] 作廢 API 回 0 ≠ 已作廢 ⇒ 輪詢 `invoice_query` 看 `invoice_type` 變 `C0501`
- [ ] 有「距離法定逾期（B2C 2 天 / B2B 7 天）剩幾天」的告警，不是只有重試次數
- [ ] 跨期／過關帳日的退款走**折讓**不走作廢
- [ ] 輪詢 worker 在多副本部署下有 lease claim
- [ ] 所有跨服務呼叫有明確 deadline
- [ ] log 記錄 `api_path` + `code` + `msg` 三者（code 跨 API 多義）
- [ ] `file_url` **不落庫、不進 log、不回傳前端**；下載後驗 magic bytes 再轉存
- [ ] 非 prod 環境斷言 `AMEGO_INVOICE == 12345678`
- [ ] `date_start`/`date_end` 我方嚴格驗 `YYYYMMDD`（否則會靜默拉回全部歷史）
- [ ] 分頁迴圈用「`data` 為空 或 `page > page_total`」終止，不依賴錯誤碼
- [ ] 批次回應用**鍵值 join**，不用索引對位

### 7.3 本次實測證據清單（可複驗）

全部以官方公開測試統編 `12345678` + 公開測試 App Key 進行，全程 `Content-Type: application/x-www-form-urlencoded`。

| # | 呼叫 | 結果 | 坐實了什麼 |
|---|---|---|---|
| 1 | `GET /json/time` | `{"timestamp":1787578776,...}` | base URL 可達、校時來源、無 code/msg 的例外形狀 |
| 2 | `POST /json/ban_query` 正常簽章 | `code:0` + 公司名 | **簽章公式 `md5(data+time+appkey)` 正確** |
| 3 | 錯誤簽章 / 大寫 hex | `code:16` | 簽章嚴格比對 |
| 4 | `time` 偏移 −60/−59/+60 → 0；−61/+61/毫秒/缺欄位 → 15；小數 → 16 | — | **±60 秒窗口（含端點）** |
| 5 | `invoice=99999999` | `code:12` | 驗證順序第一關 |
| 6 | `Content-Type: application/json` / `text/plain` | `code:11`（誤導性），HTTP 200 | 不可用 JSON body 及其失敗形狀 |
| 7 | `data=abc` / `notjson{` | `code:17`（**非文件宣稱的 20**） | 文件與實作不符 |
| 8 | `data` 雙重 urlencode（兩種 sign 取法） | `16` / `17` | **「請 url encode」＝標準一次 form 編碼** |
| 9 | `{"barCode":"A%20B"}` sign 取原字串 | `9000112`（非 16） | **伺服器只解碼一次** |
| 10 | 官方 Python/Java 自檢 md5 | 兩者皆重現 | **不做 canonicalization，序列化只能做一次** |
| 11 | `POST /json/f0401` B2C / B2B | `code:0`，`ZA10063775` / `ZA10063776` | **補上官方缺少的 f0401 response 範例**；坐實頂層平鋪 |
| 12 | 重複 `OrderId` | `3040171` | 冪等鍵行為 |
| 13 | `SalesAmount` 不平 / `BuyerName` 空 / 未打統編帶 TaxAmount / 免稅放錯欄位 | `3040174` / `3040123` / `3040174` / `3040174` | **`3040174` 是萬用錯誤且訊息會誤導** |
| 14 | `f0401` 送陣列 / `ProductItem: []` | `3040112` / **`3040113`** | 只收單張；**3040113 不在官方錯誤碼表** |
| 15 | 各欄位長度二分（BuyerName 60/61、MainRemark 200/201、Description 256/257、Remark 120/121、CarrierId1 64/65、CarrierType 6/7、NPOBAN 3~7） | 各自對應錯誤碼 | **補上文件未載明的長度上限** |
| 16 | `BuyerIdentifier` 值域（`0000000000`/合法統編/`00000000`/`12345678`/長度異常） | `0`/`0`/`3040122`/`3040122`/`3040121` | **AMEGO 有驗統編檢查碼**；測試統編本身不合法 |
| 17 | `NPOBAN` 全值域掃描 | 3–7 碼純數字皆過，`999`/`000` 亦過 | **AMEGO 不驗捐贈碼存在性** |
| 18 | 載具三種類型 × 顯隱碼組合 | `3040131`/`3040132`/`3040133`/`3040134`/`3040135` 分流 | **釐清四個描述相同／相近的錯誤碼** |
| 19 | `CarrierType` 任意 6 碼（`ZZ0001`/`abcdef`…） | 通過 | **AMEGO 不驗載具類別白名單** |
| 20 | 自然人憑證 `TP12345678901234`（隨手編） | 通過 | **AMEGO 不驗自然人憑證存在性** |
| 21 | 載具+捐贈+統編三者並存 | 全部通過 | **AMEGO 不擋三選一互斥** |
| 22 | `POST /json/invoice_query`（order / invoice / 不存在 / 各日期） | `0` / `71` / `51` | **180 天邊界精確成立**；`invoice_date` 回 **Number** |
| 23 | `POST /json/invoice_status` 9 筆跨期別發票 | 4 筆正常、5 筆 **NOT_FOUND**（但實際存在） | **靜默 false negative，禁用於存在性判斷** |
| 24 | `invoice_status` 50/300/1000/3000 筆；600 筆重複 | 皆 `code 0`；重複只回 1 筆 | 批次容量；**回應會去重** |
| 25 | `POST /json/f0501` 作廢 / 重複作廢 / 不存在 / `CancelReason` 21 字 | `0` / `3050131` / `3050125` / `3050113` | **作廢非同步**；**`CancelReason` 是未文件化欄位，上限 20 字** |
| 26 | `POST /json/g0401` 各錯誤路徑（含 `AllowanceType=1`、多發票、17 字單號） | `4040145` / `4040132` / `4040121` | **一單一發票**；**`4040145` 不在官方表** |
| 27 | `POST /json/g0501` / `f0701` / `c0701` | `code` 回 **String**；`f0701` 回 `3070111` | **code 型別不一致**；**註銷端點實際存在** |
| 28 | `POST /json/allowance_query` / `allowance_status`（591 天前） | `0`（完整資料） / `NOT_FOUND` | **折讓查詢無期限；折讓狀態同樣有靜默窗口** |
| 29 | `POST /json/invoice_list`（多種日期／limit／page） | `data_total 330376`；`limit` 越界 `34`；page 超界 `code 0` 空陣列；日期格式錯**靜默回全部歷史** | **無 180 天限制**；**日期格式錯不報錯** |
| 30 | `POST /json/invoice_file` 非載具 / 載具 / 超 180 天 | `0`+PDF 323,646 bytes / `53` / `51` | **載具發票中獎前不可下載**；PDF 端到端可用 |
| 31 | `file_url` 重複使用 / 竄改 token / 改 `type` 參數 | 可重用 / HTTP 200 + 中文純文字 / 回不同 PDF | **失敗時仍是 200，必須驗 magic bytes** |
| 32 | `POST /json/allowance_file`（近期 / 590 天前） | 皆 `0` + PDF | **折讓無 180 天限制**；token 路徑與發票不同 |
| 33 | `POST /json/barcode` 12 種輸入 | `0` / `9000111` / `9000112` / `9000113` | 格式與存在性可分辨；**不支援批次** |
| 34 | `POST /json/ban_query` 批次含壞值 / 重複值 | `code 99` 整批無結果 / 只回 1 筆 | **all-or-nothing**；**會去重** |
| 35 | `POST /json/lottery_type`（`data=""` vs `"{}"`） | `code:0` 13 筆 / `code:16` | **以空字串驗簽** |
| 36 | `POST /json/track_all` / `track_status`（Year 2026 / Period 3、4） | `code:0` + 真實字軌 | `Period` 是 0–5 期別；三層巢狀結構 |
| 37 | `invoice_list` 抽樣 1,500 筆的 `invoice_lottery` 分布 | 全部落在 80~88 | **與 `lottery_type` 值域零重疊** |
| 38 | `GET .../user/order_query?type=2&...` | HTTP 200 + Turnstile 表單，錯誤 random 回應大小相同 | **消費者查詢頁有人機驗證，不可深連結／內嵌** |
| 39 | 完全相同請求連送 3 次 | 3 次皆 `code:0` | **無 nonce、無伺服器端去重** |
| 40 | 重抓 mid=71 / mid=77 / `/json/time` | 見 §0.4 | **修正二手抄錄的機型表錯誤**；確認 `3040113` 不在官方表 |

**未實打的 API 與理由**：
- `/json/track_get`（字軌取號）—— **有副作用**，會真的消耗測試公司的字軌本數
- `/json/f0401_custom`（API 配號）—— 需先 `track_get`，同上
- `/json/invoice_print` / `allowance_print` —— 需熱感應機格式驗證，且無實機
- `/json/lottery_status` —— 需跨期已開獎的發票
- `/json/f0701` 的成功路徑 —— **刻意只打不存在的號碼**，避免命中真實單據

**測試資料清理**：本次開立的 2 張測試發票 `ZA10063775`、`ZA10063776` 已於同一輪呼叫 `POST /json/f0501` 作廢（回 `code:0`）。
其餘寫入類探測（`f0501`/`g0401`/`g0501`/`f0701`）全部打在不存在的單號上，只觸發驗證層／查無資料，**未在測試帳號建立或變更任何單據**。

---

*（若對某條有疑義，回頭讀 §0.3 的官方 URL 重抓一次原始頁面——官方 API 文件無版本號、無變更日誌，隨時可能異動。）*
