# `storeid.Resolver` 用專屬 RPC，不解析共用 RPC 的業務碼

| 項目 | 值 |
|---|---|
| 狀態 | superseded by `2026-09-29-seller-suspension-and-business-identity-in-monolith.md`（A3，2026-09-29）：入口解析 `store_id` 後以 header 帶給 gateway；「目前店家」只以型別明確的模組 API 回答、不解析業務碼的原則沿用 |
| 日期 | 2026-08-19 |
| 影響範圍 | gateway `storeid.Resolver`、seller-service gRPC |

## 脈絡

gateway 的 `storeid.Resolver` 要回答一個純內部的布林問題：**這個 account 現在有沒有店**。
它一度靠呼叫 `GetSeller` 並解析回傳的業務碼來判斷。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| (a) 維持呼叫 `GetSeller`，在 resolver 端維護業務碼白名單 | seller-service **每新增一個**語意上屬於「無現任 store」的業務碼，resolver 都要手動同步一份字串前綴白名單 | 否決 |
| (b) 新增 `GetCurrentStoreID` 專屬 RPC，「無現任 store」只用真正的 gRPC `NotFound` 表達 | 多一支窄 RPC | **採用** |

## 決策

**(b)。** `GetSeller` 的業務碼是為**終端使用者** self-service 畫面設計的（例如
「找不到您的商家資料」這種中文 toast）。讓一個內部 resolver 借用那個錯誤碼平面去回答
布林問題，等於把 UX 文案變成內部契約——白名單漏掉任何一個新碼都會變成死碼分類 bug，
而症狀是「查詢回 409 而不是空結果」這種不會有人聯想到 resolver 的表象。
同構的先例已存在：`ResolveSellerAccountID` 也是 narrow RPC、無 Code/Message envelope、
錯誤走真實 gRPC status。

## 後果

- 任何新的「解析目前身份」需求都應複製這個形狀，**不要**共用有 UX 業務碼的 RPC。
- 買家側的 `buyerid.Resolver` 目前呼叫的 `GetProfile` 尚無 UX 業務碼轉譯，暫時沒有同款
  風險；**若日後為買家也加上中文業務碼，必須先照本 ADR 新增專屬 RPC**，否則會重演同一個
  死碼分類 bug。
