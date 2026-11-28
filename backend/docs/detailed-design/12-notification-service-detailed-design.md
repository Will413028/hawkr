# 通知微服務（Notification Service）詳細設計書

> **已退場（2026-11-26，A8-8）。** 本文原本描述 notification-service 的服務邊界、gRPC 介面、RabbitMQ 消費者、FCM 推播與裝置 token；notification-service 已刪除，由單體的 `internal/modules/notification` 取代，原文隨之退場（歷史在 git）。

現行規格：

- 行為規格（唯一來源）：[notification 模組行為規格](../modules/notification.md)。
- 維運（投遞重送、未觸達查詢）：`backend/internal/modules/notification/RUNBOOK.md`。
- 決策：ADR `docs/adr/2026-10-05-notification-module.md`（含 2026-11-23 修訂）、遷移計畫 `backend/docs/plans/2026-11-23-a8-notification.md`。
- 舊機制的去向逐項記在 `backend/internal/modules/notification/CARRYOVER.md` 的沿用盤點。

對外介面是 Connect（`api/hawkr/notification/v1alpha1/`、`api/hawkr/admin/notification/v1alpha1/`），不再有 gRPC 的 `NotificationService` 與 `/api/v1/app/notifications*` REST。
