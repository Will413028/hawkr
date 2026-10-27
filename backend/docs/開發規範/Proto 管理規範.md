# Proto 管理規範

> **適用範圍（2026-09-24 起）**：只適用遷移期間仍在運作的舊服務。單體的模組之間是 in-process 呼叫，不使用 proto（`docs/adr/2026-09-24-go-modular-monolith.md` D1）；內部 gRPC 與 proto 在遷移計畫階段 C 移除，本規範隨之退場。


本文件規範 Proto 檔案的管理方式、修改流程、版本控制與數字欄位設計規則。

---

## 1. 唯一修改位置

Proto 定義是整個系統的唯一契約來源（Single Source of Truth），只能在一個地方修改：

| 路徑 | 性質 | 可否直接修改 |
|------|------|-------------|
| `api/proto/*` | **唯一真實來源** | **可以** |
| `services/java/shared/java-proto/src/main/proto/` | 副本（rsync 同步） | **禁止** |
| `pkg/proto/` | 生成的 Go 程式碼 | **禁止** |

**任何 Proto 修改都必須從 `api/proto/` 開始**，再透過同步流程更新副本與生成碼。直接修改副本會在下次同步時被覆蓋。

---

## 2. Proto 檔案選項

每個 `.proto` 檔案**必須包含**以下選項：

```protobuf
syntax = "proto3";
package hawkr.{service};
option go_package = "github.com/Will413028/hawkr/go-pkg/proto/{service}";
option java_package = "io.github.will413028.hawkr.{service}.grpc";
option java_multiple_files = true;
```

其中 `{service}` 替換為對應的微服務名稱（如 `product`、`order`、`coupon` 等）。

---

## 3. 修改後同步流程

修改 `api/proto/` 下的任何檔案後，**必須依序**執行以下步驟：

```bash
# 1. 生成 Go 程式碼
sh scripts/gen-go-proto.sh

# 2. 同步到 Java Proto 模組
rsync -av --delete api/proto/ services/java/shared/java-proto/src/main/proto/

# 3. 編譯 Java Proto
cd services/java/shared/java-proto && mvn clean install

# 4. 驗證相關微服務編譯
# 確認所有引用到修改 Proto 的微服務都能正常編譯
```

**注意**：步驟 4 不可省略。Proto 變更可能影響多個微服務，必須逐一驗證編譯通過。

---

## 4. 微服務 Proto 配置禁止事項

微服務**不可以**自行管理 Proto，必須透過共用模組取得：

| 禁止事項 | 說明 |
|---------|------|
| 微服務本地維護 `src/main/proto/` 目錄 | 會造成版本不一致 |
| 配置 `protobuf-maven-plugin` | Proto 編譯統一由 `java-proto` 模組負責 |

**正確做法**：微服務透過 Maven 依賴 `proto-common` JAR 來使用 Proto 生成的 Java 類別。

---

## 5. 版本統一

所有微服務的 gRPC / Protobuf 版本**必須**與 `proto-common` 一致：

| 元件 | 版本 |
|------|------|
| gRPC | 1.71.0 |
| Protobuf | 3.25.5 |

標準版本參考：`services/java/shared/java-proto/pom.xml`

**禁止微服務單獨升級版本**。版本不一致會導致序列化 / 反序列化錯誤，且難以排查。

---

## 6. 數字欄位規則

### 6.1 基本規則

Proto 定義中，所有數字欄位（ID、金額、數量、計數）**一律使用 `string`**。

```protobuf
// 正確
message Product {
  string product_id = 1;
  string price = 2;
  string stock = 3;
}

// 錯誤 - 禁止使用數值型別
message Product {
  int64 product_id = 1;
  int64 price = 2;
  int32 stock = 3;
}
```

**禁止使用**：`int32`、`int64`、`double`（除以下例外）。

### 6.2 例外規則

| # | 項目 | 條件 |
|---|------|------|
| 1 | Gateway JWT 注入的身分欄位（`account_id` 等） | 僅限 Request message、Gateway 注入、前端不可控。**Response 中同欄位仍必須 `string`** |
| 2 | Response `int32 code` | 全專案慣例，暫免 |
| 3 | 純微服務間 gRPC 欄位 | 不經 Gateway 暴露至前端的內部通訊，允許原生數值型別。🔴 **只對 Request 端成立**：`proto-lint-numeric` 從**任何 RPC 的 OUTPUT type** 出發遞移走訪，不判斷該 RPC 對不對外 ⇒ 服務間 RPC 的 **response** 用 `int64` 一樣擋紅（2026-08-30 實測）。要走原生型別只能登記 allowlist，而它只准縮 |

### 6.3 各層型別對應

Proto 的 `string` 要求僅適用於 **gRPC 介面與對外 API**。後端內部使用原生數字型別，在 gRPC 邊界轉換：

| 層級 | 型別 | 說明 |
|------|------|------|
| Proto 定義 | `string` | 所有數字欄位 |
| Gateway HTTP JSON | `string` | 對外回應以字串傳遞 |
| Java gRPC Service | `parseLong()` / `formatLong()` | 在 gRPC 邊界轉換 |
| Go gRPC Service | `strconv.ParseInt()` / `strconv.FormatInt()` | 在 gRPC 邊界轉換 |
| Java Service / Entity | `Long` / `Integer` | 原生數字型別 |
| Go Service / Domain | `int64` / `int32` | 原生數字型別 |
| DB Schema | `BIGINT` / `INTEGER` | 原生數字型別 |

**禁止**：Service 層直接用 Proto string 做數值計算。
**禁止**：Entity 用 `String` 存儲數字欄位。

### 6.4 自動守門

`protonumericlint`（`pkg/cmd/protonumericlint`）掃描所有「對外暴露」（從任一 RPC 回應型別可達）的 message 欄位，禁止 `int64` / `int32` / `double`。語言無關，一條規則同時涵蓋 Go 與 Java 生成碼。

- 既有違規以 `scripts/proto/numeric-allowlist.txt` grandfather；遷移欄位到 `string` 後**刪除對應行**，gate 即保證不回潮。
- §6.2 例外：`code`（int32）自動豁免；其餘例外請加入 allowlist 並註明原因。
- 本地手動執行：`make proto-lint-numeric`（目前沒有接進 CI）。
