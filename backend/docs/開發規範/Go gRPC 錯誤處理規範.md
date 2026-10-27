# Go gRPC 錯誤處理規範

> **適用範圍（2026-09-24 起）**：只適用遷移期間仍在運作的舊服務。單體的模組之間是 in-process 呼叫（`docs/adr/2026-09-24-go-modular-monolith.md` D1），內部 gRPC 在遷移計畫階段 C 移除，本規範隨之退場。


> **本文件為所有 Go 微服務 gRPC 錯誤處理的唯一規範**。新增或修改 RPC 方法時必須遵守。

---

## 1. 核心原則

**與 Java 服務統一：使用 Envelope Pattern。** 業務錯誤放在 Response body 的 `code` + `message` 欄位，gRPC 層回傳 OK。

```go
// ✅ 正確：已知業務錯誤 → Response envelope（gRPC OK）
func (s *Server) GetSeller(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    seller, err := s.usecase.GetByID(ctx, req.AccountId)
    if err != nil {
        if code := errors.GetCode(err); code != errors.CodeUnknown {
            return &pb.Response{Code: int32(code), Message: err.Error()}, nil
        }
        return nil, err  // 未知錯誤 → gRPC error（ErrorInterceptor 處理）
    }
    return &pb.Response{Code: 0, Data: toProto(seller)}, nil
}
```

### 1.1 為什麼用 Envelope 而非 gRPC Error？

| 考量 | 說明 |
|------|------|
| **Java/Go 統一** | Java 服務用 Envelope（有細粒度業務碼如 4001、7003），Go 統一避免 Gateway 需要兩套邏輯 |
| **前端統一** | 前端只需解析 `{code, message, data}` 一種格式 |
| **小團隊** | 前端 1 人 + 後端 1 人，兩套規範的認知負擔太大 |

---

## 2. Gateway 自動防護：EnvelopeGuardInterceptor

### 2.1 機制

Gateway 的 gRPC **client interceptor** 會自動將 envelope error 轉換為 gRPC error：

```
Handler 呼叫服務 → gRPC OK + {code: 404, data: nil}
                          ↓
              EnvelopeGuardInterceptor 攔截
                          ↓
              resp.GetCode() != 0 → 轉換為 gRPC NotFound error
                          ↓
              Handler 收到 err != nil（自動處理，不會碰到 nil Data）
```

**效果**：Gateway handler 只需檢查 `err != nil`，不需要手動檢查 `resp.Code`。

### 2.2 Gateway handler 寫法

```go
// ✅ 正確：只需處理 err，不需要 GrpcGuard
func (h *Holder) GetSeller(c *gin.Context) {
    resp, err := h.Seller.GetSeller(c.Request.Context(), req)
    if err != nil {
        response.GrpcErr(c, "seller", err)  // 同時處理 transport error 和 envelope error
        return
    }
    // err == nil → code 一定是 0 → Data 有值
    c.JSON(http.StatusOK, toJSON(resp.Data))
}
```

### 2.3 Envelope Code → gRPC Code 映射

| Envelope Code | gRPC Code | HTTP 狀態碼 |
|:---:|---|:---:|
| 400 | InvalidArgument | 400 |
| 401 | Unauthenticated | 401 |
| 403 | PermissionDenied | 403 |
| 404 | NotFound | 404 |
| 409 | AlreadyExists | 409 |
| 412 | FailedPrecondition | 409 |
| 500 | Internal | 500 |
| 503 | Unavailable | 503 |
| 其他（4001, 7003 等） | FailedPrecondition | 409 |

Java 服務的細粒度業務碼（如 4001 庫存不足）會被映射為 `FailedPrecondition`，原始碼保留在 message 中（`[4001] 庫存不足`）。

### 2.4 實作位置

| 檔案 | 用途 |
|------|------|
| `gateway/internal/grpc/envelope_guard_interceptor.go` | Interceptor 實作 |
| `gateway/internal/grpc/client.go` → `DefaultDialOpts()` | 統一 dial options（含 interceptor） |
| 所有 `*_client.go` | 使用 `DefaultDialOpts()` |

---

### 2.5 越權回應策略（防枚舉）

存取**存在但不屬於自己**的敏感資源（ownership mismatch）時，用 `errors.NewNotFound(msg)`（404）而非 `errors.NewForbidden(msg)`（403），且回應與「資源不存在」一致，避免枚舉探測。完整 Tier A/B/C 界線與強制清單見 [API 設計規範 §14.1](./API%20設計規範.md)。純認證 / 角色門檻失敗仍回 401 / 403。

## 3. Go 服務 Handler 寫法

### 3.1 標準模式

```go
import (
    stderrors "errors"

    pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
)

// domainMessage 安全地取出使用者可見訊息：err 不是 DomainCoder
// （stderrors.As 斷言失敗）時必須回退固定文案，不能無條件呼叫 dc.DomainMessage()
// ——那樣在斷言失敗時 dc 是 nil，會直接 panic。與
// services/ai-service/internal/grpc/handler.go 的 domainMessage() 是同一個
// 寫法。
func domainMessage(err error) string {
    var dc pkgerrors.DomainCoder
    if ok := stderrors.As(err, &dc); ok {
        return dc.DomainMessage()
    }
    return "服務暫時無法使用，請稍後再試"
}

func (s *Server) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.ProductResponse, error) {
    if req.ProductId == 0 {
        return &pb.ProductResponse{
            Code: int32(pkgerrors.CodeInvalidInput), Message: "product_id is required",
        }, nil
    }

    product, err := s.productUC.GetByID(ctx, req.ProductId)
    if err != nil {
        if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
            // 用 DomainMessage()，不要用 err.Error()：DomainError 若是用
            // Wrap(code, msg, cause) 建出，Error() 會把 cause 串進訊息
            // （例如 "庫存不足: pq: ..."），把下游系統細節洩漏進使用者看得到的
            // response——正是 §5「禁止事項」要擋的那件事；DomainMessage() 只回傳
            // Message 本身。
            return &pb.ProductResponse{Code: int32(code), Message: domainMessage(err)}, nil
        }
        return nil, err  // 未知錯誤 → ErrorInterceptor
    }

    return &pb.ProductResponse{Code: 0, Data: toProto(product)}, nil
}
```

### 3.2 Batch API 模式

```go
func (s *Server) GetSellerBatch(ctx context.Context, req *pb.BatchRequest) (*pb.BatchResponse, error) {
    var results []*pb.SellerInfo
    for _, id := range req.AccountIds {
        seller, err := s.sellerUC.GetByID(ctx, id)
        if err != nil {
            if pkgerrors.IsNotFound(err) {
                continue  // NotFound → 跳過
            }
            return nil, err  // 其他錯誤 → 中斷
        }
        results = append(results, toProto(seller))
    }
    return &pb.BatchResponse{Code: 0, Data: results}, nil
}
```

---

## 4. 錯誤碼

### 4.1 go-pkg DomainError

| 建構函式 | Code | 用途 |
|----------|------|------|
| `errors.NewNotFound(msg)` | 404 | 資源不存在 |
| `errors.NewInvalidInput(msg)` | 400 | 參數驗證失敗 |
| `errors.NewConflict(msg)` | 409 | 重複操作 |
| `errors.NewForbidden(msg)` | 403 | 無權限 |
| `errors.NewUnauthorized(msg)` | 401 | 未認證 |
| `errors.NewPreconditionFailed(msg)` | 412 | 前置條件不滿足 |

### 4.2 細粒度業務碼（bizcode）

Go 服務可用 `NewBusiness` 回傳細粒度業務碼，HTTP 回應格式對齊 Java（HTTP 409 + `{"code": 409, "message": "[NNNN] ..."}`）。

**使用流程**：

1. 在 `docs/代碼管理（CodeMeta）/錯誤碼.md` 新增業務碼條目
2. 執行 `./docs/代碼管理（CodeMeta）/scripts/generate.sh --go-errors` 重新生成 `pkg/errors/bizcode/bizcode.go`
3. 在服務 usecase 使用常數：

```go
import (
    pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
    bizcode    "github.com/Will413028/hawkr/go-pkg/errors/bizcode"
)

// usecase 層
return nil, pkgerrors.NewBusiness(bizcode.SelfMessageNotAllowed, "不能傳訊息給自己")
```

**不需修改 gRPC handler**：只要 usecase 回傳 `*DomainError`，現有的 envelope pattern 會自動處理。

**禁止**：直接在 Go 程式碼硬編碼業務碼數字（如 `NewBusiness(9201, ...)`），必須使用 `bizcode` 常數。`bizcode.go` 由 `generate.sh` 生成，**禁止**手動修改。

### 4.3 ErrorInterceptor 備援

掛載在所有 Go 服務 gRPC server 上，處理 `return nil, err` 的未知錯誤：
- `DomainError` → gRPC status code
- 其他 → `codes.Internal`

---

## 5. 禁止事項

| 禁止 | 原因 |
|------|------|
| Gateway handler 直接存取 `resp.Data` 不先處理 `err` | 如果繞過 interceptor（如直接用 proto client），Data 可能是 nil |
| Gateway handler 使用自訂 dial options 不包含 `EnvelopeGuardInterceptor` | 必須用 `DefaultDialOpts()` |
| 在 Response `message` 中放系統錯誤細節 | 可能洩漏 SQL/堆疊 |

---

## 6. 教訓

### 6.1 17 處 Gateway nil dereference（2026-04-06）

服務回傳 `{code: 404, data: nil}` 作為 gRPC OK，Gateway 未檢查 `resp.Code` 就存取 `resp.Data`，導致 nil pointer panic → HTTP 500。22 處中有 17 處遺漏。

**根因**：Envelope Pattern 下 Gateway 需要做兩層檢查，人工容易遺漏。

**解法演進**：
1. `GrpcGuard` helper — 把檢查收成一行（仍需手動呼叫）
2. `EnvelopeGuardInterceptor` — client interceptor 自動轉換（handler 無需知道 envelope 的存在）

**結論**：架構層的問題要用架構層的方案解決，不能靠開發者記憶。
