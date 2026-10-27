# Proto Common - Java Proto 共用模組

## ⚠️ 重要規則

### Proto 定義的唯一修改位置

**只能修改 `api/proto/*`**

```
✅ 正確：修改 api/proto/product/product.proto
❌ 錯誤：直接修改 services/java/shared/java-proto/src/main/proto/product/product.proto
```

### 為什麼？

- `api/proto/` 是所有 proto 定義的**唯一真實來源**（Single Source of Truth）
- `services/java/shared/java-proto/src/main/proto/` 是從 `api/proto/` 同步的**副本**
- 直接修改副本會在下次同步時被覆蓋

---

## 工作流程

### 1. 修改 Proto 定義

```bash
# 只在 api/proto/ 修改
vim api/proto/product/product.proto
```

### 2. 同步到 Java Proto 模組

```bash
# 方法一：使用 rsync 手動同步
rsync -av --delete api/proto/ services/java/shared/java-proto/src/main/proto/

# 方法二：使用腳本（如果有的話）
# sh scripts/sync-proto-to-java.sh
```

### 3. 編譯 Java Proto

```bash
cd services/java/shared/java-proto
mvn clean install
```

這會：
- 從 proto 檔案生成 Java 類別
- 編譯成 `proto-common-1.0.0.jar`
- 安裝到本地 Maven 倉庫（`~/.m2/repository/io/github/will413028/hawkr/proto-common/1.0.0/`）

### 4. Java 微服務使用

```xml
<!-- services/java/product-service/pom.xml -->
<dependency>
    <groupId>io.github.will413028.hawkr</groupId>
    <artifactId>proto-common</artifactId>
    <version>1.0.0</version>
</dependency>
```

---

## 目錄結構

```
專案根目錄/
├── api/proto/                          ← 唯一真實來源（修改這裡）
│   ├── product/product.proto
│   ├── payment/payment.proto
│   └── ...
│
├── services/java/shared/java-proto/      ← Java Maven 模組（不要直接修改 proto）
│   ├── pom.xml
│   └── src/main/proto/                 ← api/proto/ 的副本
│       ├── product/product.proto       （從 api/proto/ 同步）
│       └── ...
│
└── pkg/proto/                          ← Go 生成的程式碼（由 api/proto/ 生成）
    └── ...
```

---

## Proto 選項說明

Proto 檔案同時包含 Go 和 Java 的選項：

```protobuf
syntax = "proto3";

package hawkr.product;

option go_package = "github.com/Will413028/hawkr/go-pkg/proto/product";  // Go 用
option java_package = "io.github.will413028.hawkr.product.grpc";                // Java 用
option java_multiple_files = true;                                  // Java 用
```

- `go_package`：Go 生成時使用（Java 生成時會忽略）
- `java_package`：Java 生成時使用
- `java_multiple_files`：Java 生成時使用（每個 message 獨立文件）

---

## 常見問題

### Q: 為什麼有兩份 proto 檔案？

**A:** 因為 Go 和 Java 使用不同的構建工具：
- **Go**：直接從 `api/proto/` 讀取，使用 `protoc` 生成到 `pkg/proto/`
- **Java**：需要 proto 在 Maven 專案結構下（`src/main/proto/`），才能用 `protobuf-maven-plugin` 生成

### Q: 我修改了 api/proto/，Java 服務沒有看到更新？

**A:** 需要執行同步和編譯：
```bash
# 1. 同步 proto 到 Java 模組
rsync -av --delete api/proto/ services/java/shared/java-proto/src/main/proto/

# 2. 重新編譯 proto-common
cd services/java/shared/java-proto
mvn clean install

# 3. （可選）重新編譯使用此模組的微服務
cd ../../../services/java/product-service
mvn clean compile
```

### Q: 我可以直接修改 services/java/shared/java-proto/src/main/proto/ 嗎？

**A:** ❌ 不行！下次同步時會被覆蓋。請務必只修改 `api/proto/`。

---

## 自動化建議

考慮建立同步腳本 `scripts/sync-proto-to-java.sh`：

```bash
#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR/.."

echo "同步 proto 定義到 Java 模組..."
rsync -av --delete "$PROJECT_ROOT/api/proto/" "$PROJECT_ROOT/services/java/shared/java-proto/src/main/proto/"

echo "編譯 proto-common..."
cd "$PROJECT_ROOT/services/java/shared/java-proto"
mvn clean install

echo "✅ Proto 同步與編譯完成！"
```

---

## 版本管理

當前版本：**1.0.0**

- 版本號定義在 `pom.xml` 的 `<version>` 標籤
- 更新 proto 定義後，考慮是否需要升級版本號
- 所有依賴此模組的微服務需同步更新依賴版本
