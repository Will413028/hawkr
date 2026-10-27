# 身份分離採 distributed principal + per-role aggregate

| 項目 | 值 |
|---|---|
| 狀態 | accepted；遷移計畫 A2 起 account 那一側在單體的 identity 模組（principal 與角色集合），buyer、seller 的 aggregate 在 A3 搬遷，決策本身不變 |
| 日期 | 2026-07-07 |
| 影響範圍 | account-service、buyer-service、seller-service、所有跨角色守衛 |

## 脈絡

PayUni 代理商的 MerID 是**一次性綁定**：同一個 MerID 重複申請回 `AGENT04017`。
而 MerID 由賣家身份推導，因此「刪店重開」必須拿得到一個 PayUni 沒看過的賣家身份。
同時，同一個登入帳號可以同時是買家與賣家，「不能對自己下單／發券／私訊」這類守衛
必須仍然認得出兩端是同一個人。

## 1. 為什麼不是用旗標隱藏商品

**考慮過**：① 完整身份分離 ② 只加 `product.seller_deleted_at` 旗標把舊商品隱藏起來

**決策：① 完整身份分離。**

**否決 ②**：旗標解不了 order／coupon／cart 的歷史連續性，更關鍵的是它**完全沒有碰到
MerID 上限**——而那才是唯一的 forcing function。旗標版本的實作已廢棄。

## 2. aggregate id 放在哪裡

**考慮過**：

| 方案 | 代價 | 判定 |
|---|---|---|
| (A) 分散 principal：各角色服務自持 aggregate id，`account_id` 維持純登入身份 | 下游要改鍵，但每個服務各自可控 | **採用** |
| (B) 集中在 accounts 表 + role 欄位 | 要把 role 塞進 `email` / `(provider, sub)` 的唯一鍵裡，動到登入死結熱區 | 否決 |
| (C) 只給賣家一個 surrogate `seller_id` 型別 | 需要約 300 處 `created_by → seller_id` re-key，且沒有解決買家側的對稱問題 | 否決 |

**決策：(A)。** account-service 維持純 principal，buyer-service 持有 `buyer_id`、
seller-service 持有 `store_id`，兩者都以 `account_id` 錨回 principal。
對齊的業界模型：WorkOS 的 user↔membership、Stripe 的 Customer vs Connect account、
DDD 的 Party-Role。

## 後果

- 跨角色守衛不能直接比對 id，必須先經 `ResolvePrincipals` 還原成 principal 再比
  （fail-open / fail-closed 分工見 44 號 §8.3）。
- `account_id` 從此**不是**任何業務表的歸屬鍵；任何「用 account_id 查業務資料」的
  新程式碼都是 bug。
- 賣家與買家的 aggregate id 換新語意見
  `2026-07-11-identity-store-stint-symmetry.md`。
