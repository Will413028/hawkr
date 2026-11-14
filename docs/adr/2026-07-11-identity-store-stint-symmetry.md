# 賣家與買家一律走 store-stint（insert-new-stint），不做同列 remint

| 項目 | 值 |
|---|---|
| 狀態 | accepted；遷移計畫 A2（2026-09-27）不影響：stint 在 seller、buyer 的 aggregate，A3 盤點（2026-09-29）核對：PayUni 的 MerID 一次性綁定仍成立，沿用；resolver 改為入口解析（`2026-09-29-seller-suspension-and-business-identity-in-monolith.md`）。identity 的角色表以 `source_id` 記下授予角色的 stint（store_id、buyer_id），舊 stint 遲到的撤銷不會移除新 stint 的角色（#3189） |
| 日期 | 2026-07-11（買家對稱化 2026-07-15） |
| 影響範圍 | seller-service、buyer-service、gateway resolver、8 個下游服務的歸屬鍵 |

## 脈絡

`2026-07-07-identity-distributed-principal.md` 決定各角色服務自持 aggregate id 之後，
還有兩個問題沒答：**身份換新時是改同一列還是新增一列**，以及**買家要不要跟賣家一樣換新**。

## 1. 身份換新的形狀

**考慮過**：

| 方案 | 代價 | 判定 |
|---|---|---|
| ① 同列重設 + remint（reuse-row）：復活時原地改寫該列並換發新 id | 舊 stint 的軌跡被覆蓋，稽核查不到；`payuni_registrations` 必須在同一交易裡刪掉舊列，否則新舊綁定混淆 | 否決 |
| ② 新增一列（insert-new-stint）：舊列 `is_current=false` 保留，新列帶新 id | `account_id` 必須改成非唯一，所有「找某人現在的店」的查詢要改走 partial unique index | **採用** |

**決策：②。** 舊列保留讓稽核與 PII 清理都有完整軌跡可掃，且不需要為了換身份去刪
任何外部綁定紀錄——舊綁定連同舊列一起留著，新 stint 自己鑄新的 `store_code` 就拿得到
PayUni 沒看過的 MerID。

## 2. 買家要不要對稱

**考慮過**：① 買家維持單一身份、改補事件路由缺口 ② 買家也做 store-stint

一度採用 ①，理由是「買家沒有 PayUni 那種外部一次性綁定，複製 remint 到沒有
forcing function 的 aggregate 是 cargo-cult」。

**決策：改採 ②。**

決定性的不是技術理由而是責任邊界：**`account` 只管登入，業務身份的生命週期一律由
業務服務自己管理**，不透過在 account-service 加欄位或加事件路由來修補。
① 的實際形狀是「下游清理邏輯掛在 `account.deleted` 上」，於是買家角色單獨被移除時
根本不觸發清理——那是把買家的生命週期外包給了 principal。

**接受的行為後果**：買家軟刪除後重新註冊 ＝ 完全全新開始，過去的訂單、售後依據、
評論在新身份下都看不到。這與賣家重新入駐看不到舊上架商品是同一個原則。

## 後果

- 兩邊的 `account_id` 都是非唯一欄位，唯一性靠 `uq_{sellers,buyers}_account_current`
  （`WHERE is_current`）。
- gateway 必須有 `buyerid` / `storeid` 兩個 resolver 把 `account_id` 翻成現行 surrogate；
  任何繞過 resolver 直送 `account_id` 的呼叫點都會在換 stint 之後查到錯誤的資料。
- 下游服務以 surrogate 為歸屬鍵。例外是 social-service：它的
  `cms_comment.user_id` / `reports.reporter_id` / `user_blocks.blocker_id` 刻意維持泛型
  命名，因為同一欄位可能裝賣家操作帶來的值，不是單一 id space。
