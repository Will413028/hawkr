# Hawkr Backend

生鮮電商後端 monorepo。Gateway（Go/Gin）+ 10 個 Go 微服務 + 10 個 Java/Spring Boot 微服務，
透過 gRPC 同步通信、RabbitMQ 異步事件、PostgreSQL/Redis 為主要儲存，部署在 k3s 上。

## 文件入口

| 文件 | 用途 |
|---|---|
| [docs/index.md](docs/index.md) | **完整技術文件**：詳細設計書（29 份）、開發規範、CodeMeta、基礎建設、踩坑紀錄、decisions、postmortems、runbooks |
| [QUICKSTART.md](QUICKSTART.md) | 新機器 git clone 後的環境設定、部署、常用指令 |
| [rules.md](rules.md) | Go API stack 規範（由維護者本機 hook 自動同步，來源不在本 repo） |
| [scripts/README.md](scripts/README.md) | 開發、smoke test、資料同步等本機腳本 |

## 服務一覽

- **Gateway**（`gateway/`, Go/Gin, port 9900）：REST → gRPC 路由、JWT auth、rate limit、circuit breaker、OTel tracing
- **Go 服務**（`services/`）：account / buyer / seller / im / ai / content / crm / social / review / notification — 使用 **Atlas declarative** schema migration
- **Java 服務**（`services/`, Spring Boot）：product / order / media / aftersales / payment / cart / shipping / coupon / payuni-webhook / product-import — 使用 **Flyway** migration

各服務 port、API、資料模型詳見 [docs/詳細設計書/](docs/詳細設計書/)。

## 開發規範（最高優先級）

- [API 設計規範](docs/開發規範/API%20設計規範.md)
- [開發工作流程](docs/開發規範/開發工作流程.md)（設計書 → Code Gen → DB → 程式碼 → 測試 強制順序）
- [Java 微服務開發規範](docs/開發規範/Java%20微服務開發規範.md)
- [Proto 管理規範](docs/開發規範/Proto%20管理規範.md)

其餘規範與設計書見 [docs/index.md](docs/index.md)。
