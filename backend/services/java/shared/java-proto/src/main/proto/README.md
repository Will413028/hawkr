# API Proto 定義

## ✅ 這是 Proto 定義的唯一真實來源（Single Source of Truth）

**所有 proto 定義的修改都必須在這裡進行！**

---

## 修改 Proto 後的同步流程

### 1. 修改 Proto 定義（在此目錄）

```bash
# 修改 proto 檔案
vim api/proto/product/product.proto
```

### 2. 生成 Go 程式碼

```bash
# 從專案根目錄執行
sh scripts/gen-go-proto.sh

# 生成的 Go 程式碼位於：pkg/proto/
```

### 3. 同步到 Java Proto 模組

```bash
# 同步 proto 檔案
rsync -av --delete api/proto/ services/java/shared/java-proto/src/main/proto/

# 編譯 Java proto
cd services/java/shared/java-proto
mvn clean install
```

---

## 目錄結構

```
api/proto/                              ← 你在這裡！（唯一修改位置）
├── product/
│   └── product.proto
├── payment/
│   └── payment.proto
├── media/
│   └── media.proto
└── ...
```

---

## Proto 選項規範

每個 proto 檔案必須包含以下選項：

```protobuf
syntax = "proto3";

package hawkr.{service};

// Go 生成設定
option go_package = "github.com/Will413028/hawkr/go-pkg/proto/{service}";

// Java 生成設定
option java_package = "io.github.will413028.hawkr.{service}.grpc";
option java_multiple_files = true;
```

**注意：**
- `go_package`：統一格式為 `github.com/Will413028/hawkr/go-pkg/proto/{service}`
- `java_package`：統一格式為 `io.github.will413028.hawkr.{service}.grpc`
- 兩種選項在同一個 proto 檔案中共存，互不影響

---

## 使用者

### Go 服務（Gateway）

```go
import pb "github.com/Will413028/hawkr/go-pkg/proto/product"

// 使用生成的 Go 程式碼
client := pb.NewProductServiceClient(conn)
```

生成流程：
```
api/proto/ → gen-go-proto.sh → pkg/proto/ → Gateway 使用
```

### Java 服務（Product Service 等）

```xml
<dependency>
    <groupId>io.github.will413028.hawkr</groupId>
    <artifactId>proto-common</artifactId>
    <version>1.0.0</version>
</dependency>
```

生成流程：
```
api/proto/ → rsync → services/java/shared/java-proto/ → mvn install →
proto-common.jar → Java 微服務使用
```

---

## ⚠️ 重要提醒

1. **永遠只修改 `api/proto/`**
2. **不要直接修改 `services/java/shared/java-proto/src/main/proto/`**（那是副本）
3. **不要直接修改 `pkg/proto/`**（那是生成的程式碼）
4. 修改後記得同步到 Java 模組並重新編譯

---

## 完整修改檢查清單

- [ ] 修改 `api/proto/{service}/{service}.proto`
- [ ] 執行 `sh scripts/gen-go-proto.sh` 生成 Go 程式碼
- [ ] 執行 `rsync -av --delete api/proto/ services/java/shared/java-proto/src/main/proto/` 同步到 Java
- [ ] 執行 `cd services/java/shared/java-proto && mvn clean install` 編譯 Java proto
- [ ] 測試 Gateway（Go）是否能正常編譯和使用
- [ ] 測試相關 Java 微服務是否能正常編譯和使用
- [ ] Commit 變更（包含 api/proto/ 和 services/java/shared/java-proto/）
