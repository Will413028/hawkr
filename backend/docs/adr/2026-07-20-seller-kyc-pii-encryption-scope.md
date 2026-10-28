# 賣家實名認證 PII 的 at-rest 加密範圍

| 項目 | 值 |
|---|---|
| 狀態 | accepted；A3（2026-09-29，Will 決定）核對：範圍維持（後台仍需以姓名模糊搜尋）。「不拆表」「停機回填」的限制在新 schema 不成立但不改變結論；PayUni 的 `raw_request` 存了明文身分證號、PII 清理不完整，在 A3b 修正。A3b 7c（2026-10-02）：範圍不變，加密實作改用 `internal/platform/seal` 的金鑰環、密文存 `bytea`，見 `2026-10-19-seller-kyc-payuni-consolidated.md`（取代 `2026-10-02-seller-kyc-payuni-in-monolith.md`） |
| 日期 | 2026-07-20 |
| 影響範圍 | seller-service `certification_applications`；`docs/modules/seller.md` §13（KYC 個資） |

## 脈絡

`certification_applications` 是扁平表，同時承載認證 workflow、賣家 PII 與商業文件。
盤點時該表的 `id_card_no` 與 `bank_account` 為明文存放。

當時不可動的約束：

- **不拆表**：`certification_identity` 拆表是另一條路線，本決策只處理「當前扁平表上能立刻做什麼」。
- **admin 需要對 `real_name` 做模糊搜尋**（`ILIKE`），而加密欄位無法被 `ILIKE` 命中。
- 加密實作沿用 account-service 已有的 AES-256-GCM（後抽為 `pkg/crypto`），單一
  `ENCRYPTION_MASTER_KEY`。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 全表 PII 一次加密（含 `real_name`、`birth_date`） | `real_name` 需改 blind-index 才能保留 admin 模糊搜尋；`birth_date` 需 `date` → `bytea` 的型別破壞，而它既無查詢也無遮蔽需求 | ❌ 否決 |
| **只加密 `id_card_no`（後續補 `bank_account`）** | 同一列仍有其他明文 PII；安全性提升有上限 | ✅ 採用 |
| 密文存 `bytea` | 與 media-service 既有慣例不一致；未來併入整包 `pii_ciphertext` 時還要再轉一次 | ❌ 否決 |
| 密文存 `varchar(255)` 的 Base64 | 欄位長度必須先放寬（14 字帳號的 Base64 約 56 字） | ✅ 採用 |
| 存量以 migration 內加密 | Go 的 GCM 輸出格式在 SQL 內無法重現 | ❌ 否決 |
| 存量以一次性 Go backfill（停機窗口） | 需要停機窗口；dev 可改 wipe + reseed | ✅ 採用 |
| 賣家自查連 `real_name` 一起遮蔽 | 賣家看自己的姓名屬正常行為，過度遮蔽傷可用性 | ❌ 否決 |

## 決策

1. **加密 `id_card_no`**；`real_name` 保留明文（保住 admin `ILIKE` 搜尋）。
2. **`birth_date` 不加密**。
3. **密文型別為 `varchar(255)` 存 Base64**，對齊 media-service。
4. **存量以停機窗口 + 一次性 Go backfill 遷移**，不雙軌、不寫進 codebase。
5. **賣家自查只遮 `id_card_no`**。
6. 本決策是**不拆表的立即前置**：`certification_identity` 全拆落地時，單欄密文併入整包
   `pii_ciphertext`，遮蔽與 RPC 分層邏輯原樣沿用。

`bank_account` 後續依同一組判準納入加密範圍（同表、同一次寫入、同等敏感度）。

## 後果

- 接受「同一列仍有明文 PII」——攻擊者若讀得到該列，`real_name` 與 `birth_date` 仍是明文。
  因此只加密 JSONB 副本而不加密對應欄位**買不到任何安全性**，這也是 `step_data` 的機敏 key
  清單只涵蓋 `id_card_no` 與 `bank_account` 兩者拼法的理由。
- **重新評估的條件**：① `certification_identity` 拆表落地 ② admin 搜尋改用 blind-index
  ③ 法遵要求 `real_name` / `birth_date` at-rest 加密。
  其中任一成立時，`real_name` / `birth_date` 必須重新評估，且**必須在同一個 PR 內**把它們的
  `step_data` 拼法一併加進機敏 key 清單。
