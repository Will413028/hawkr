# TGOS API 整合指南

內政部國土測繪中心（MOI TGOS）提供的地址 geocode 服務，已取代原本的 Google Maps 整合（2026-07-17 接線、2026-07-21 完成 prod cutover 並移除 Google 程式碼，詳見 wiki ADR `2026-07-21-tgos-prod-cutover.md`）。本文件記錄 API 規格與 `pkg/geocode` 的 `TGOSClient` 實作對應。

---

## 一、啟用的 API

TGOS 共用性網路地圖服務帳號下申請了 3 支獨立 API：

| API | 對應現有 Google Maps 角色 | 用途 |
|-----|-----|------|
| `QueryAddr`（全國門牌位置比對服務） | Places Autocomplete + Geocoding | 地址（含模糊/殘缺門牌）→ 坐標，單一 endpoint 同時做模糊比對與轉座標 |
| `GeoQueryAddr`（坐標回傳門牌服務） | 無對應（新增能力） | 坐標 → 門牌地址（reverse geocode），目前功能沒有用到，先記錄規格 |
| `MOI TGOS MAP API 3.0` | 無對應（新增能力） | 前端地圖元件（JS SDK，互動地圖/圖層/定位工具），非 `pkg/geocode` 後端流程需要，屬於另一類產品 |

憑證由維運者本機保管，不進版控。固定出口 IP 白名單：`203.0.113.30`（`example-seller-prod` GCP VPC）。

---

## 二、QueryAddr（地址 → 坐標，模糊比對）

Service：`WSQueryAddr40`（ASMX）

### 端點

```
POST https://addr.tgos.tw/addrws/v40/QueryAddr.asmx
```

或 HTTP GET（ASMX 內建 REST-style 呼叫，免 SOAP envelope）：

```
GET https://addr.tgos.tw/addrws/v40/QueryAddr.asmx/QueryAddr?oAPPId=...&oAPIKey=...&oAddress=...&...
```

**認證方式與 Google 不同**：`oAPPId` / `oAPIKey` 是 **request 參數**（SOAP body 欄位 / query string / form body 三選一，依呼叫方式而定），**不是** HTTP header。

### 參數

| 參數 | 型別 | 說明 |
|------|------|------|
| `oAPPId` | string | 應用程式識別碼 |
| `oAPIKey` | string | API 授權碼，與 `oAPPId` 成對 |
| `oAddress` | string | 要查詢的門牌地址字串 |
| `oSRS` | string | 回傳坐標系統，`EPSG:4326`（WGS84，對齊現有 Google lat/lng）或 `EPSG:3825`~`3828`（TWD97 分帶） |
| `oFuzzyType` | int | 模糊比對機制：`0`=最近號、`1`=奇偶號、`2`=混合 |
| `oFuzzyBuffer` | int | 模糊比對容許誤差，`0`=不限制 |
| `oResultDataType` | string | 回傳格式：`JSON` 或 `XML` |
| `oIsOnlyFullMatch` | bool | 是否只接受完全比對 |
| `oIsSupportPast` | bool | 是否含歷史地址 |
| `oIsShowCodeBase` | bool | 是否顯示統計區資訊 |
| `oIsLockCounty` / `Town` / `Village` / `RoadSection` / `Lane` / `Alley` / `Area` | bool | 鎖定不比對特定地址層級 |
| `oIsSameNumber_SubNumber` | bool | 號之後的附號視為相同 |
| `oCanIgnoreVillage` / `oCanIgnoreNeighborhood` | bool | 查無村里/鄰時允許忽略 |
| `oReturnMaxCount` | int | 最多回傳筆數（1–9，`0`=服務預設 10） |

### SOAP 1.1 Request 範例

```xml
POST /addrws/v40/QueryAddr.asmx HTTP/1.1
Host: addr.tgos.tw
Content-Type: text/xml; charset=utf-8
SOAPAction: "http://tempuri.org/QueryAddr"

<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <QueryAddr xmlns="http://tempuri.org/">
      <oAPPId>string</oAPPId>
      <oAPIKey>string</oAPIKey>
      <oAddress>string</oAddress>
      <oSRS>string</oSRS>
      <oFuzzyType>int</oFuzzyType>
      <oResultDataType>string</oResultDataType>
      <oFuzzyBuffer>int</oFuzzyBuffer>
      <oIsOnlyFullMatch>boolean</oIsOnlyFullMatch>
      <oIsSupportPast>boolean</oIsSupportPast>
      <oIsShowCodeBase>boolean</oIsShowCodeBase>
      <oIsLockCounty>boolean</oIsLockCounty>
      <oIsLockTown>boolean</oIsLockTown>
      <oIsLockVillage>boolean</oIsLockVillage>
      <oIsLockRoadSection>boolean</oIsLockRoadSection>
      <oIsLockLane>boolean</oIsLockLane>
      <oIsLockAlley>boolean</oIsLockAlley>
      <oIsLockArea>boolean</oIsLockArea>
      <oIsSameNumber_SubNumber>boolean</oIsSameNumber_SubNumber>
      <oCanIgnoreVillage>boolean</oCanIgnoreVillage>
      <oCanIgnoreNeighborhood>boolean</oCanIgnoreNeighborhood>
      <oReturnMaxCount>int</oReturnMaxCount>
    </QueryAddr>
  </soap:Body>
</soap:Envelope>
```

Response 外層是 SOAP envelope 包一個 `QueryAddrResult` 字串（HTTP GET 呼叫則是 `<string xmlns="http://tempuri.org/">...</string>` 包同樣的字串）；這個字串本身才是 `oResultDataType` 指定格式（JSON/XML）的實際查詢結果。

> ⚠️ **結果字串內的實際欄位名稱（縣市/鄉鎮市區/村里/鄰/路段/巷弄/門牌號/X/Y 等）官方 ASMX 頁面未列出** — 串接前必須先用真實憑證打一次拿到 raw response 核對欄位名稱與巢狀結構，不要用本文件假設的欄位名稱直接寫 parser。

### HTTP GET 快速呼叫格式

```
GET https://addr.tgos.tw/addrws/v40/QueryAddr.asmx/QueryAddr?oAPPId={APPID}&oAPIKey={APIKEY}&oAddress={address}&oSRS=EPSG:4326&oFuzzyType=0&oFuzzyBuffer=0&oResultDataType=JSON&oIsOnlyFullMatch=false&oIsSupportPast=false&oIsShowCodeBase=false&oReturnMaxCount=5
```

---

## 三、GeoQueryAddr（坐標 → 地址，reverse geocode）

Service：`WSGeoQueryAddr40`（ASMX），端點 `https://addr.tgos.tw/addrws/v40/GeoQueryAddr.asmx`。5 個方法，目前功能面沒有反查需求，記錄最常用的一支供未來參考：

### PointQueryNearAddr（單點找最近門牌）

| 參數 | 型別 | 說明 |
|------|------|------|
| `oAPPId` / `oAPIKey` | string | 同上 |
| `oPX` / `oPY` | double | 查詢點坐標（依 `oSRS`） |
| `oSRS` | string | 坐標系統，同 QueryAddr |
| `oResultDataType` | string | `JSON` / `XML` |
| `oIsShowCodeBase` | bool | 是否顯示統計區資訊 |

呼叫方式（SOAP/HTTP GET/POST）與 QueryAddr 同款式，`SOAPAction: "http://tempuri.org/PointQueryNearAddr"`，GET 格式：

```
GET https://addr.tgos.tw/addrws/v40/GeoQueryAddr.asmx/PointQueryNearAddr?oAPPId={APPID}&oAPIKey={APIKEY}&oPX={lng}&oPY={lat}&oSRS=EPSG:4326&oResultDataType=JSON&oIsShowCodeBase=false
```

其餘 4 個方法（`LineQueryAddr`、`PointQueryNearAddrByBuffer`、`PolygonQueryAddr` 及帶 buffer 版本）用途是線/面/範圍查詢，目前無使用情境，需要時查 `GeoQueryAddr.asmx?op=<method>` 取得該方法完整規格。

---

## 四、MOI TGOS MAP API 3.0（前端地圖元件）

服務首頁：`https://api.tgos.tw/TGOS_MAP_API_3`。這是**前端 JavaScript SDK**（互動地圖、圖層套疊、量測/剖面/街景等工具），跟上面兩支後端地址服務是不同產品線，**不在目前 `pkg/geocode` 後端 geocode 替換範圍內**。只有專案未來需要在網頁/App 內嵌互動地圖（而非單純「查地址拿坐標」）才會用到；載入方式（script tag / API Key 帶法）未在服務首頁公開，需要時走首頁「開啟互動介面」逐項確認或洽客服。

---

## 五、環境變數（建議命名，尚未實作）

比照現有 `GOOGLE_MAPS_API_KEY` 慣例，`pkg/geocode` 加 TGOS provider 時建議：

| 變數 | 說明 |
|------|------|
| `TGOS_QUERYADDR_APPID` / `TGOS_QUERYADDR_APIKEY` | QueryAddr（地址→坐標）憑證 |
| `TGOS_GEOQUERYADDR_APPID` / `TGOS_GEOQUERYADDR_APIKEY` | GeoQueryAddr（坐標→地址）憑證，若有反查需求才接 |

---

## 六、注意事項

- **認證方式跟 Google 不同**：Google 用 header（`X-Goog-Api-Key`）或 query `key`；TGOS 兩支地址服務的 `oAPPId`/`oAPIKey` 一律是 **API 自己定義的 request 參數**（SOAP body 欄位或 query string，非標準 header）。
- **回傳固定出口 IP 白名單**：TGOS 帳號綁定固定 IP（`203.0.113.30`），非白名單來源請求會被拒，正式服務部署需帶 `--network=tgos-vpc --subnet=tgos-subnet --vpc-egress=all-traffic`（見 backend-payuni wiki Pending）。
- **`QueryAddr` 單一 endpoint 可同時取代 Google 的 Autocomplete + Geocoding 兩支 API**（`oFuzzyType`/`oFuzzyBuffer` 做模糊比對、同時回傳坐標），設計 provider 時不必比照 Google 拆兩支呼叫。
- **response 實際 JSON schema 未公開**，串接第一步務必先用真實憑證打一次記錄 raw response，再決定 parser 欄位對應（比照 `pkg/geocode` 現有 Google provider 的解析策略段落寫法）。
- 憑證遺失/需重建：非 self-service，須重新走 TGOS 官方帳號申請流程。
