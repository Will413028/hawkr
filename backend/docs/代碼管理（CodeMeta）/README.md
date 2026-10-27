# 代碼管理（CodeMeta）

> **2026-09-25 起凍結**（ADR `docs/adr/2026-09-25-codemeta-retirement.md`）：CodeMeta 只服務還沒搬進單體的舊契約（Java、還沒搬的 Go 服務、gateway 的 REST 與使用它們的 client 畫面），不再新增值域。單體與對外 Connect 契約的列舉用 proto enum、錯誤原因用 `api/` 的 `ErrorReason`、儲存的碼由模組定義、顯示文字由各 client 擁有。每個模組搬進單體時，把它用到的值域與錯誤碼移出這裡；最後一個舊消費端退場時整個目錄刪除。本文件以下描述的是舊契約仍在使用的機制。

本目錄是專案共享代碼值的唯一維護入口，涵蓋：

- 業務狀態碼
- 業務類型碼
- 內部技術碼
- 統一錯誤碼

CodeMeta 的目標是把跨服務、跨語言、跨介面的共享值集中定義，避免在 Java、Go、Gateway、Flutter、proto 或 MQ 契約中各自硬編一套。

生成體系演進過程中的決策與踩坑見 [`踩坑與變遷紀錄.md`](./踩坑與變遷紀錄.md)。

---

## 目前實作總覽

### Markdown 來源檔

| 檔案 | 用途 | 生成結果 |
|------|------|----------|
| `業務狀態碼.md` | 會隨生命週期流轉的狀態值 | 生成 Java CodeMeta；`前端可見: Yes` 的項目也生成 Go / Flutter |
| `業務類型碼.md` | 相對固定的類型、分類值 | 同上 |
| `內部技術碼.md` | 僅後端或跨服務內部使用的技術值 | 生成 Java CodeMeta；通常不生成 Go / Flutter |
| `錯誤碼.md` | 統一 gRPC Response Code 與錯誤訊息 | 生成 `GrpcResponseCode.java` |

### 腳本與輸出

| 路徑 | 用途 |
|------|------|
| `scripts/generate.sh` | CodeMeta 生成腳本 |
| `internal/platform/codemeta/code_meta.go`（相對於 backend 根目錄） | Go 輸出（`--go`），屬於單體的 platform。目前只含「前端可見」的 enum，Go modular monolith 遷移計畫階段 0.2 擴充 |
| `buyer-app/lib/core/codemeta/code_meta.dart`、`seller-app/lib/core/codemeta/code_meta.dart`（相對於 monorepo 根目錄） | Flutter 輸出（`--flutter`），檔頭 `// dart format off`，`dart format` 不重排 |
| `website/lib/codemeta/generated.ts` | Web/TypeScript 輸出（`--web`；`lib/codemeta/enums.ts` re-export 此檔並額外維護非 CodeMeta 範疇的手寫工具函式） |
| `admin/src/lib/codemeta/generated.ts` | ADM/TypeScript 輸出（`--admin`；`src/lib/codemeta/index.ts` re-export 此檔並額外維護 `codeLabel`/`toOptions`）。**走 allowlist 而非「前端可見」全掃**，清單見 `generate.sh` 的 `ADMIN_ENUMS`，且**不受前端可見過濾**（同 `GO_LABEL_ENUMS`）——ADM 是內部後台，需要顯示 `DelegationAuditAction` 這種標「前端可見: No」的治理值域，但那些值不該同時外洩給買賣家前端 |
| `services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/` | Java 最終覆蓋輸出目錄 |
| `pkg/errors/bizcode/bizcode.go` | Go 業務碼常數直寫輸出（`--go-errors`；來源 `錯誤碼.md`，只取 code ≥ 1000 的業務碼） |
| `pkg/media/ref_type.go` | Go media 引用類型常數直寫輸出（`--go-media`；`AssetReferenceType`，前端可見=No 故不走 `--go`） |
| `pkg/codemeta/labels.go` | Go Value→Label 對照直寫輸出（`--go-labels`；涵蓋清單見 `generate.sh` 的 `GO_LABEL_ENUMS` 陣列，**不受前端可見過濾**——後端要下發中文 Label 給前端顯示時用） |
| `pkg/codemeta/registry.go` | Go registry 直寫輸出（`--go-registry`；typed 常數 + `AllXxx` + `FromCode`/`FromValue`，涵蓋清單見 `GO_REGISTRY_ENUMS`，**allowlist 非空時完全不看可見性**——`_go_should_emit` 的規則是「空 allowlist ⇒ 看可見性；非空 ⇒ 只看名字在不在清單裡」，實證：`ModerationActionType` 標「前端可見: No」而 registry 確實產出了它）。與 `GO_LABEL_ENUMS` 必須互斥：兩者同寫 `package codemeta`，同一個數字 Value 的 enum 重疊會產生同名符號而編譯失敗 |

### 目前真實生成規則

1. Java 生成來源包含 `業務狀態碼.md`、`業務類型碼.md`、`內部技術碼.md`，並直接覆蓋 `common-core` 下的 Enum。
2. `錯誤碼.md` 不走一般 CodeMeta Enum 流程：Java 側單獨生成 `GrpcResponseCode.java`（`--java`），Go 側以獨立 `--go-errors` 通道直寫 `pkg/errors/bizcode/bizcode.go`（只含 code ≥ 1000 的業務碼，通用錯誤碼不進 Go）。
3. Go、Flutter、Web 只生成 `前端可見: Yes` 的項目，直接寫進 monorepo 裡各 client 的路徑（見上表）。
4. 生成結果以 Markdown 定義為主。若生成檔案與 Markdown 描述不一致，應先修正 Markdown 與生成流程，而不是在生成檔案中直接補規則。
5. Web 生成的 `*_LABEL` map 型別統一為 `Record<string, string>`（非 `Record<XxxValue, string>`）：既有 web 呼叫端有多處以未收斂型別的原始字串（後端傳來、未先轉型）直接查表，嚴格型別會在這類呼叫點產生 TS7053，故統一放寬，避免每個 caller 各自 cast。
6. 第 3 條「只生成 `前端可見: Yes`」有一個 Go 例外：`AssetReferenceType`（前端可見=No）由各 Go 服務帶 `ref_type` 呼叫 media-service 使用，故以獨立 `--go-media` 通道直寫 `pkg/media/ref_type.go`（比照 `--go-errors` 直寫 `bizcode.go`），不受前端可見過濾。此舉杜絕手維護副本 drift。
7. 第 3 條的第二個例外是 `--admin`：它**走 `ADMIN_ENUMS` allowlist 而非可見性過濾**（同 `GO_LABEL_ENUMS` 的理由）。「前端可見」這個旗標實際區分的是「要不要給買賣家前端」，而 ADM 是內部後台——把 `DelegationAuditAction` 改成 `Yes` 只為了讓 ADM 拿到它，會連帶灌進 website 與兩支 App 的生成物。`--web` / `--go` / `--flutter` 三者的可見性語意不變。
8. `--web` 與 `--admin` 共用同一個生成器（`generate_ts_file`），差別只在 allowlist 與檔頭文案。**不要為了 ADM 複製第二份 TS 生成器**：依 Value 去重、剝除 Label 尾綴「（終態）」、`Record<string, string>` 型別放寬這三個細節若各自演化，兩份輸出就會在細微處分歧。

### client 生成物與 drift 檢查

`--web`、`--admin`、`--flutter` 直接覆寫 monorepo 裡的四個檔案，沒有人工拷貝。CI 的 `backend monolith` workflow（「CodeMeta 生成物 drift」）重新生成 `--go --go-media --web --admin --flutter` 後比對工作目錄，任何一份與 Markdown 不一致（手改生成檔、改了 Markdown 沒重新生成）都會轉紅。

- website 與 admin 的 `biome.json` 把 `generated.ts` 排除在 formatter 之外，生成檔與生成器輸出逐位元相同。
- Flutter 生成檔的檔頭是 `// dart format off`，兩個 App 的 `dart format --set-exit-if-changed` 不會要求重排，同樣逐位元相同。

---

## 核心原則

1. Markdown 是唯一真實來源。共享代碼值先改 `.md`，不要先改 Java / Go / Flutter 常數。
2. 外部契約優先使用 `value`。HTTP API、gRPC、MQ payload、跨服務欄位預設都應傳 `value`，不是 `code`。
3. 第三方可判斷的共享值必須納管。只要其他服務、Gateway、前端或外部整合方會依該值分支判斷，就應進入 CodeMeta。
4. 單一服務的純內部 local enum 可不納管。若值不外露、不跨服務、不進契約，可留在服務內。
5. Proto 仍以 `api/proto` 為唯一來源。CodeMeta 不取代 proto；共享欄位設計仍要先改 proto，再同步 Java 副本。

---

## Markdown 格式

每個 enum 區塊以 `## EnumName` 開頭，接屬性區塊與表格。

```md
## OrderStatus

- **類別**: 業務狀態碼
- **介面**: CodeMeta
- **前端可見**: Yes
- **使用服務**: order, cart, payment
- **DB 儲存欄位**: orders.status（存 Value）
- **說明**: 訂單生命週期狀態

| Name | Code | Value | Label | Description |
|------|------|-------|-------|-------------|
| PENDING_PAYMENT | PENDING_PAYMENT | 0 | 待付款 | 庫存已扣減，等待支付事件 |
```

### 屬性欄位

| 欄位 | 必填 | 說明 |
|------|:---:|------|
| 類別 | Y | `業務狀態碼` / `業務類型碼` / `內部技術碼` / `錯誤碼` |
| 介面 | Y | `CodeMeta`（一般 enum）／`ErrorCode`（僅 `錯誤碼.md`）／`Legacy`（見下方說明） |
| 前端可見 | Y | `Yes` / `No`，影響是否生成 Go / Flutter |
| 使用服務 | Y | 目前會使用此值的服務列表 |
| DB 儲存欄位 | N | 若資料庫有持久化，需標註欄位與存的是 `Value` 還是 `Code` |
| 說明 | Y | 一行描述 enum 用途 |

`介面: Legacy` 的語意：該 enum 仍在本目錄留存定義（供查閱與其他語言生成），但**不生成 Java 檔**——
腳本會跳過它並印出 `⏭ Xxx.java (Legacy，跳過)`，用於 Java 側已由別處承載、不希望被覆蓋的舊 enum。
只有 Java 生成階段檢查此值；Go / Flutter / Web 一律只看 `前端可見`，不受 `Legacy` 影響。
目前四份碼表皆無此值，看到跳過訊息時即為此機制。

### 值表格欄位

| 欄位 | 必填 | 說明 |
|------|:---:|------|
| Name | Y | Enum 常數名，使用 `UPPER_SNAKE_CASE` |
| Code | Y | 穩定識別鍵，通常與 `Name` 相同 |
| Value | Y | 對外契約值 |
| Label | Y | 顯示名稱 |
| Description | Y | 詳細說明 |

註記：

- 目前腳本實際解析的是 `Label`，不是舊文件中寫過的 `DisplayName`。
- 只要表格結構仍是第 4 欄顯示名、第 5 欄說明，腳本即可解析，但文件應統一寫成 `Label`。

---

## 生成方式

```bash
cd docs/代碼管理（CodeMeta）/scripts

# 生成全部（等同 --java --go --go-errors --go-media --go-labels --go-registry --flutter --web --admin；不含 --diff）
./generate.sh --all

# 僅生成 Java CodeMeta + GrpcResponseCode
./generate.sh --java

# 僅生成 Go 參考檔
./generate.sh --go

# 僅生成 Go 業務碼常數（直寫 pkg/errors/bizcode/bizcode.go）
./generate.sh --go-errors

# 僅生成 Go media 引用類型常數（直寫 pkg/media/ref_type.go）
./generate.sh --go-media

# 僅生成 Go Value→Label 對照（直寫 pkg/codemeta/labels.go；清單見 GO_LABEL_ENUMS）
./generate.sh --go-labels

# 僅生成 Flutter 參考檔
./generate.sh --flutter

# 僅生成 Web/TypeScript 參考檔（全部前端可見碼表）
./generate.sh --web

# 僅生成 ADM/TypeScript 參考檔（清單見 ADMIN_ENUMS，不受前端可見過濾）
./generate.sh --admin

# 比較 Java 生成結果與現有檔案差異（唯讀，不覆蓋任何檔案）
# 與其他旗標併用時只跑 diff 就結束，不會產出 Go / Flutter / Web
./generate.sh --diff

# 列出可用旗標
./generate.sh --help
```

可用旗標僅上述 `--java` / `--go` / `--go-errors` / `--go-media` / `--go-labels` / `--go-registry` / `--flutter` / `--web` / `--admin` / `--all` / `--diff` / `--help`（`-h`）十二個，傳入其他參數會直接報錯退出。

### 什麼時候用哪個

- 後端 Java 共用值調整：通常至少執行 `--java`
- 需要同步給 Go / Flutter / Web 參考：再執行 `--go`、`--flutter`、`--web` 或 `--all`
- 調整 `錯誤碼.md` 內 code ≥ 1000 的業務碼：**除了 `--java` 外必須再跑 `--go-errors`**，否則 Go 服務拿不到新碼（`bizcode.go` 是 DO NOT EDIT 生成檔，禁止手改）
- 調整 `AssetReferenceType`（media 引用類型）：執行 `--go-media` 直寫 `pkg/media/ref_type.go`
- 調整 `GO_LABEL_ENUMS` 內任一 enum 的 Label 欄，或把新 enum 加進該陣列：執行 `--go-labels` 直寫 `pkg/codemeta/labels.go`（沒加進 `GO_LABEL_ENUMS` 只是 Go 側查不到 Label；但**清單裡的名字打錯會直接中止生成**——`⚠ 解析結果中找不到 X，中止 Go labels 生成` ＋ `return 1`，不是靜默少一張表）
- 需要 Go 側 typed 常數與 `FromCode`/`FromValue`（而非只有 Label）：把 enum 加進 `GO_REGISTRY_ENUMS` 後執行 `--go-registry` 直寫 `pkg/codemeta/registry.go`（**打錯名字會直接中止生成**：`⚠ 解析結果中找不到 X，中止 --go-registry 生成` ＋ `return 1`，一個檔都產不出來；且**不需要標「前端可見: Yes」**——allowlist 非空時 `_go_should_emit` 完全不看可見性）
- 想先看是否會覆蓋現有 Java 常數：先跑 `--diff`
- 同步給 Web：`--web` 直寫 `website/lib/codemeta/generated.ts`
- 同步給 ADM：`--admin` 直寫 `admin/src/lib/codemeta/generated.ts`。**要讓 ADM 拿到一張新碼表，是把 enum 名加進 `generate.sh` 的 `ADMIN_ENUMS`**，不是在 admin repo 手寫一份；清單裡列了 `.md` 中不存在的名字會直接中止生成（不會靜默少一張表）

---

## 實際維護流程

### 新增共享代碼

1. 先判斷這是不是跨服務或對外契約值。
2. 在對應 `.md` 新增 enum 或新增列。
3. 若是錯誤碼，改 `錯誤碼.md`；若是一般狀態/類型/技術值，改其對應分類檔。
4. 執行 `generate.sh --java`。
5. 若新增的是 code ≥ 1000 的業務錯誤碼，再執行 `--go-errors`（Go 端常數來源）。
6. 若前端或 Go 也需要同步參考，再執行 `--go` / `--flutter` / `--web`。
7. 編譯受影響模組驗證。

### 修改既有代碼

1. 修改 `.md`。
2. 重新生成。
3. 檢查 DB、API、proto、MQ、前端是否依賴舊 `value`。
4. 編譯並驗證受影響服務。

### 刪除代碼

1. 從 `.md` 移除定義。
2. 重新生成。
3. 如 Java 舊檔已無對應來源，需手動確認是否要刪除遺留檔案。
4. 確認沒有服務仍引用舊值。

---

## 與 Proto 的關係

當共享欄位同時涉及 CodeMeta 與 gRPC 契約時，流程應為：

1. 先定義或調整 CodeMeta `.md`
2. 修改 `api/proto`
3. 同步到 `services/java/shared/java-proto/src/main/proto`
4. 重新生成 Java protobuf 與 CodeMeta
5. 編譯受影響服務

禁止事項：

- 不要直接修改 `services/java/shared/java-proto/src/main/proto` 當成唯一來源
- 不要在 proto 再定義一套與 CodeMeta 語意重複的裸 enum
- 不要在 Gateway 或服務裡硬編共享值

---

## Value 分配建議

以下是目前 README 內沿用的值段慣例，新增時優先在既有段內延伸，避免衝突。

⚠️ 本表只管 CodeMeta enum 的 `Value`。`錯誤碼.md` 的 gRPC Response Code 是**另一個獨立編號空間**
（號段見「錯誤碼.md」§錯誤碼範圍分配），兩者互不相干、數字重疊不算衝突——例如 `4001~4099` 在本表是
`ProductStatus` 的 Value 段，在錯誤碼那邊是 Order 業務碼段，各自獨立分配。

| 範圍 | 用途 |
|------|------|
| 0~99 | 訂單與交易流程的小型狀態碼 |
| 1101~1199 | PaymentStatus |
| 2001~2099 | PaymentMethod |
| 3001~3099 | ShippingMethod |
| 3101~3199 | ShippingFeePayer |
| 3201~3299 | ShippingTemperature |
| 3301~3399 | WeightUnit |
| 3401~3499 | ShippingArea |
| 3501~3599 | ShippingRoute |
| 3601~3699 | OnboardingStatus（3601~3605）／ShippingType（3651~3654）|
| 4001~4099 | ProductStatus |
| 5001~5099 | RefundReason |
| 7001~7099 | MediaUploadSessionStatus |
| 7101~7199 | AssetReferenceType |
| 7201~7299 | StagedTokenScope |
| 8001~8099 | IdempotencyRequestStatus |
| 9001~9099 | CommonStatus |
| 9601~9699 | SellerSuspensionReason |

若新增 enum 需要新的值段，應先在本 README 補登記再使用。`PricebookQualityReasonCode` 不佔用值段——
其 Value 即 UPPER_SNAKE 字串本身（見「業務狀態碼.md」§PricebookQualityReasonCode），
不是數字碼。
