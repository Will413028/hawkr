# Common Core 模組

Hawkr 微服務共用核心模組，提供跨服務共用的工具類、配置與常數定義。

---

## 📦 模組內容

### 1. TSID 時序 ID 生成器

#### 1.1 什麼是 TSID？

TSID (Time-Sorted ID) 是一個 64-bit 的時序唯一識別碼，用於替代傳統的 UUID 或自增主鍵。

**結構組成**：
```
64-bit TSID = 42-bit timestamp + 10-bit node + 12-bit sequence
```

**優勢**：
- ✅ **時序性**：ID 按時間遞增，天然支援按建立時間排序
- ✅ **效能優異**：BIGINT 比 UUID (128-bit) 節省 50% 儲存空間與索引大小
- ✅ **索引友善**：B-tree 索引效能優於 UUID，避免索引碎片
- ✅ **分散式唯一**：支援多節點部署，透過 node id 確保全域唯一性
- ✅ **相容性佳**：可轉換為 Long 型別，易於序列化與傳輸

**與其他方案比較**：

| 方案 | 儲存空間 | 索引效能 | 時序性 | 分散式唯一 | 可讀性 |
|------|---------|---------|--------|-----------|--------|
| 自增主鍵 (BIGSERIAL) | 8 bytes | ⭐⭐⭐⭐⭐ | ✅ | ❌ | ⭐⭐⭐⭐⭐ |
| UUID v4 | 16 bytes | ⭐⭐ | ❌ | ✅ | ⭐ |
| TSID | 8 bytes | ⭐⭐⭐⭐⭐ | ✅ | ✅ | ⭐⭐⭐⭐ |

---

#### 1.2 Node ID 分配策略

為確保分散式環境下的唯一性，每個微服務預分配 100 個 Node ID：

| 微服務 | node-id-base | Node ID 範圍 | 用途 |
|--------|--------------|--------------|------|
| **Payment Service** | 0 | 0-99 | 支付交易、退款記錄 |
| **Media Service** | 100 | 100-199 | 媒體資產 |
| **Product Service** | 200 | 200-299 | 商品主表 |
| **Order Service** | 300 | 300-399 | 訂單 |
| **Cart Service** | 400 | 400-499 | 購物車 |
| **Coupon Service** | 500 | 500-599 | 優惠券 |
| **Shipping Service** | 600 | 600-699 | 物流 |
| **AfterSales Service** | 700 | 700-799 | 售後 / 退貨退款 |
| **Product Import Service** | 800 | 800-899 | 商品匯入精靈 |
| *預留* | 900-1023 | - | 未來服務擴展 |

> `payuni-webhook-service` 與 Coupon 共用 500：該服務全表 BIGSERIAL，不生成任何 TSID，無衝突可能。
>
> 各服務的實際值以 `services/java/{service}/src/main/resources/application.yml` 的 `hawkr.tsid.node-id-base` 為準（新增服務時**必須**同時更新本表與 [Java 微服務開發規範](../../../../docs/開發規範/Java%20微服務開發規範.md) §3）。

**多實例部署時的 Node ID 計算**：
```
實際 Node ID = node-id-base + pod-index
```

範例：
- Payment Service Pod 0: node-id = 0 + 0 = 0
- Payment Service Pod 1: node-id = 0 + 1 = 1
- Payment Service Pod 2: node-id = 0 + 2 = 2
- Media Service Pod 0: node-id = 100 + 0 = 100
- Media Service Pod 1: node-id = 100 + 1 = 101

> ⚠️ **重要**：node id 要跨 pod 唯一，同一微服務的不同實例就必須拿到不同的 `POD_INDEX`。
>
> 🚨 **但目前 `k8s/` 內沒有任何地方注入 `POD_INDEX`** —— chart 用的是 `Deployment` 不是 `StatefulSet`（`k8s/charts/hawkr-service/templates/deployment.yaml`），於是每個 pod 都吃 `application.yml` 的預設 `${POD_INDEX:0}`，**同一服務的多個 pod 共用同一個 node id**。prod 目前 `replicas: 2` 的 Java 服務有 `cart` / `order` / `payment` / `payuni-webhook`（`k8s/values/overlays/prod/`，其餘服務為 1），這幾支的 TSID 唯一性現在只靠 `TsidFactory` 的毫秒內序列，**不靠 node id 區分**。要把任何 Java 服務調到多副本並依賴 node id 隔離之前，得先讓部署真的把 `POD_INDEX` 傳進去。

---

#### 1.3 使用方式

##### Step 1: 配置 application.yml

```yaml
hawkr:
  obs:
    service: <service-name>
  tsid:
    enabled: true
    node-id-base: <分配的基礎值>  # payment:0, media:100, product:200
    pod-index: ${POD_INDEX:0}     # 從環境變數讀取，預設為 0
```

**配置說明**：
- `enabled`: 是否啟用 TSID（建議設為 true）
- `node-id-base`: 該服務的 Node ID 基礎值，參考上表分配
- `pod-index`: Pod 索引（K8s 環境由 StatefulSet 注入，本地開發預設 0）

##### Step 2: Entity 定義

```java
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

@Data
@TableName("payment_transactions")
public class PaymentTransactionEntity {

    @TableId(type = IdType.INPUT)  // 必須設為 INPUT
    private Long id;  // 使用 Long 型別（非 UUID）

    private String tradeNo;
    private Long amount;
    // ... 其他欄位
}
```

##### Step 3: Service 層生成 ID

```java
import io.github.will413028.hawkr.common.utils.TsidUtil;
import org.springframework.stereotype.Service;

@Service
public class PaymentService {

    public void createPayment(CreatePaymentRequest request) {
        PaymentTransactionEntity entity = new PaymentTransactionEntity();

        // 生成 TSID
        entity.setId(TsidUtil.generate());

        entity.setTradeNo(request.getTradeNo());
        entity.setAmount(request.getAmount());

        // 儲存到資料庫
        paymentMapper.insert(entity);
    }
}
```

##### Step 4: Migration SQL

```sql
CREATE TABLE payment_transactions (
    id BIGINT PRIMARY KEY,  -- TSID (時序 ID)
    trade_no VARCHAR(32) NOT NULL UNIQUE,
    amount BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 索引：TSID 天然支援時序排序
CREATE INDEX idx_payment_created_at ON payment_transactions(created_at DESC);
```

---

#### 1.4 主鍵選擇準則

根據業務場景選擇合適的主鍵策略：

**使用 TSID（推薦）**：
- ✅ ID 會暴露在客戶端 API 回應的業務實體表
- ✅ 需要時序性（按建立時間排序）的業務資料
- ✅ 需要分散式唯一性保證的主表
- 📝 例如：`payment_transactions`, `media_asset`, `product`, `refund_transactions`

**使用 BIGSERIAL（自增）**：
- ✅ 純內部使用（日誌表、關聯表、事件表）
- ✅ 高頻 JOIN 查詢場景
- ✅ 不對外暴露的內部流水號
- 📝 例如：`payment_logs`, `payment_webhooks`, `payment_outbox`, `asset_reference`

**使用 UUID**：
- ✅ 需要前端回傳的臨時性資源
- ✅ 特殊安全需求（完全隨機、不可預測）
- 📝 例如：`upload_session`（Media Service）

**優先順序**：TSID > BIGSERIAL > UUID（除非有特殊需求）

---

#### 1.5 常見問題

##### Q1: TSID 是否會重複？

不會。TSID 的唯一性由三個因素保證：
1. **時間戳** (42-bit)：精確到毫秒
2. **節點 ID** (10-bit)：透過 node-id-base + pod-index 區分不同實例
3. **序列號** (12-bit)：同一毫秒內最多生成 4096 個 ID

只要確保每個 Pod 的 `POD_INDEX` 不同，就能保證全域唯一性。⚠️ **這個前提目前不成立** —— 部署沒有注入 `POD_INDEX`，見 §1.2 的警告。

##### Q2: 如何在 K8s 部署時設置 POD_INDEX？

**本 repo 目前沒有設置**（§1.2）。Deployment 沒有「pod 序號」這種概念，要補上只有兩條路：改用 StatefulSet（K8s 自動為每個 Pod 分配序號），或由部署流程替每個副本指派固定值。StatefulSet 的寫法：

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: payment-service
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: payment-service
        env:
        - name: POD_INDEX
          valueFrom:
            fieldRef:
              fieldPath: metadata.labels['apps.kubernetes.io/pod-index']
```

##### Q3: 本地開發如何測試多實例？

本地開發時，可手動設置環境變數：

```bash
# Terminal 1 - Pod 0
export POD_INDEX=0
(cd services/java && ./mvnw spring-boot:run -pl :payment-service)

# Terminal 2 - Pod 1
export POD_INDEX=1
(cd services/java && ./mvnw spring-boot:run -pl :payment-service -Dserver.port=9925)
```

##### Q4: TSID 與 Proto/gRPC 如何對應？

Proto 的數字欄位（含 TSID 主鍵、金額、數量）**一律宣告成 `string`**，在 gRPC 邊界用 `parseLong()` / `formatLong()` 轉換；Java 內部一律 `Long`。理由與例外清單見 [Java 微服務開發規範](../../../../docs/開發規範/Java%20微服務開發規範.md) §2.1。

```protobuf
message PaymentTransaction {
  string id = 1;      // TSID（64-bit，超出 JS Number 安全範圍，故走 string）
  string trade_no = 2;
  string amount = 3;  // 金額，單位「元」
}
```

> 例外只有三種：Gateway 注入且前端不可控的身分欄位（`account_id` / `buyer_id` …）、Response 的 `int32 code`、以及完全不經 Gateway 暴露的純內部欄位。**TSID 主鍵不在例外之列。**

##### Q5: 可以在運行時修改 node-id-base 嗎？

⚠️ **不可以**！`node-id-base` 是服務的固定配置，修改後會導致 ID 衝突。若需調整，必須：
1. 停止所有實例
2. 清空資料庫（或確保無 ID 衝突）
3. 更新配置後重新部署

---

#### 1.6 禁止行為

- ❌ 禁止使用 `@TableId(type = IdType.AUTO)` 搭配 TSID
- ❌ 禁止在資料庫使用 `GENERATED BY DEFAULT AS IDENTITY` 搭配 TSID
- ❌ 禁止手動計算或猜測 TSID 值
- ❌ 禁止在不同服務使用相同 `node-id-base`
- ❌ 禁止在運行中的服務修改 `node-id-base` 或 `POD_INDEX`

---

### 2. 其他共用工具

- **Enum / Constants**：`io.github.will413028.hawkr.common.utils.code` —— 全部由 `docs/代碼管理（CodeMeta）/*.md` 生成，**禁止手改生成檔**
- **Type Handlers**：JSONB 自訂類型轉換
- **Validators**：通用驗證工具

> 本節不逐一列舉。共用工具（運費輔助、`DateTimeUtils`、`CodeMetaValidator`、`InputValidator` 等）與共用配置（`ObsAutoConfiguration`、`TsidConfiguration`、`MyBatisPlusAutoConfiguration` 等）的完整表格在 [Java 微服務開發規範](../../../../docs/開發規範/Java%20微服務開發規範.md) §7 與 §10；新增共用工具前先查那兩節，禁止重複實作。

---

## 📚 參考資料

- TSID Library: [f4b6a3/tsid-creator](https://github.com/f4b6a3/tsid-creator)
- 專案規範：[Java 微服務開發規範](../../../../docs/開發規範/Java%20微服務開發規範.md) §3「主鍵與 TSID」（型別對應見同檔 §2）
- Migration SQL：各服務的 `src/main/resources/db/migration/V1__*.sql`（V1 已凍結，變更一律走 `V{N+1}`）

---

**維護者**: Hawkr 開發團隊
