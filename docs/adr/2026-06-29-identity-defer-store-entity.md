# 不引入 `store` 實體：一人一店以 seller aggregate 表達

| 項目 | 值 |
|---|---|
| 狀態 | accepted；遷移計畫 A2（2026-09-27）不影響：決策在 seller 的 aggregate，A3 盤點（2026-09-29）核對：仍是一人一店、沒有店員的需求，沿用。identity 只存帳號的角色，賣家角色的 `source_id` 記授予它的 store_id（#3189 的柵欄） |
| 日期 | 2026-06-29 |
| 影響範圍 | seller-service、所有 seller-scoped 查詢、IM、PayUni MerID 推導 |

## 脈絡

「同一個人刪店後重新開店要拿到全新的 PayUni 商店身份」這個需求，可以用兩種粒度解：
把「店」升格成獨立實體，或讓賣家 aggregate 自己換一次身份。當時（也仍然）成立的約束是
**一個 account 至多一個現行店鋪**，沒有一人多店、店鋪轉讓或多人共管的需求。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| ① 現在就拆 `store` 三表（Account / Store / StoreStaff） | 系統級 re-key：`created_by → store_id`、MerID 推導、IM conversation 鍵、**所有** seller-scoped 查詢全部要跟著動 | 否決 |
| ② 賣家 aggregate 自己換身份（stint surrogate） | 爆炸半徑限於 seller-service；下游只需要換一個鍵的來源 | **採用** |

## 決策

**採用 ②。** `store` 實體 deferred，不是永久否決。

## 後果

- 一人多店、店鋪轉讓、多人共管在現行模型下**做不到**，要做就得回來拆 `store`。
- 重新評估的觸發條件（任一成立即應重啟本決策）：
  - 一個 account 需要**同時**持有多間店
  - 買賣家身份需要**硬**分離（合規要求）
  - 維持「一人一店」假設的成本超過遷移成本
