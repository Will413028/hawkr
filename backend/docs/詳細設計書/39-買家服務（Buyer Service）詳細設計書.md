# 買家服務（Buyer Service）詳細設計書

> 本文件已退場，只留指引。舊版內容在 git 歷史（`git log -- <本檔路徑>`）；其他文件引用的「39 §x」指的是舊版的章節。

## 原本涵蓋

buyer-service（Go 微服務）的買家個人資料與頭像、收貨地址、收藏、追蹤（賣家訂閱）、瀏覽紀錄、補貨訂閱與 fan-out、通知信箱、會員卡聚合、買家自助刪除與後台刪除，以及它的 DB、gRPC、REST、MQ 與快取設計。

## 現況

buyer-service 已刪除（遷移計畫 A3a）。買家的建立、存活與關閉規則和舊版不同，舊版的多段 stint、`is_current`、buyer_id 解析快取、「買家是角色」都不再成立：

- 一個帳號一生只有一個買家，在 identity 建帳的同一個 transaction 內建立；帳號活著 ⇔ 買家活著。
- identity 只儲存 SELLER 角色；買家不是角色。
- buyer_id 只在 buyer 的 handler 與 gateway 轉發時由帳號解析。
- 買家只會隨帳號刪除由 `accountdeletion` 關閉，沒有其他關閉路徑。

## 現行的唯一來源

| 內容 | 位置 |
|---|---|
| 行為規格 | `backend/docs/modules/buyer.md` |
| 職責、擁有的資料、不變量 | `backend/internal/modules/buyer/AGENTS.md` |
| 與舊服務的差異、沿用盤點 | `backend/internal/modules/buyer/CARRYOVER.md` |
| 對外 API | `api/hawkr/buyer/v1alpha1/buyer.proto`（前台）、`api/hawkr/admin/buyer/v1alpha1/buyer.proto`（後台） |
| DB schema | `backend/internal/modules/buyer/db/schema.sql` |
| API 設計與搬遷計畫 | `backend/docs/plans/2026-09-29-a3-buyer-seller/buyer-api.md` |
| 建帳時建立買家 | ADR `backend/docs/adr/2026-09-30-account-created-provisions-buyer.md` |
| 刪除帳號（含關閉買家） | ADR `backend/docs/adr/2026-09-30-account-deletion-module.md` |
| 會員方案與額度 | [設計書 59](59-買家會員分級與%20Agent%20額度方案（Buyer%20Membership%20&%20Entitlement）詳細設計書.md) |

還沒搬進單體的舊服務經 `backend/internal/legacy/buyerserver` 呼叫 buyer，買家事件由 `backend/internal/legacy/buyerrelay` 轉發到 RabbitMQ。
