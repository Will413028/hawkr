# Sign in with Apple 的後端驗證機制選型

| 項目 | 值 |
|---|---|
| 狀態 | accepted；遷移計畫 A2 起在單體的 identity 模組：撤銷的 outbox 改成 River job（單體 ADR D5），token 交換移到登入的 transaction 之前；刪除帳號時撤銷在此之前其實沒有實作（只在解綁時撤銷），A2 補上。「尚未採用」的 web 登入已經存在，Services ID 換 token 要帶 redirect URI（identity 的 `AppleConfig.RedirectURIs`） |
| 修訂 | 2026-10-08：刪帳號撤銷的保存與觸發在單體重做，見 `2026-09-27-identity-clerk.md` 文末〈修訂：Apple 授權由伺服器保存並在刪帳時撤銷〉 |
| 日期 | 2026-06-25 |
| 影響範圍 | `account-service`（`internal/client/apple/`、`internal/applerevoke/`）、Gateway `auth` handler |
| 規格 | [30-Sign in with Apple 認證（Apple OAuth）](../詳細設計書/30-Sign%20in%20with%20Apple%20認證（Apple%20OAuth）詳細設計書.md) |

## 脈絡

Apple App Store Guideline 4.8 要求：只要 App 提供任何第三方登入，就必須等價提供
Sign in with Apple，否則送審必拒。同時 Guideline 5.1.1(v) 要求 App 提供帳號刪除，
而 Apple 對「帳號刪除時要不要 revoke Apple 授權」另有要求。

Apple 對後端提供兩種可用的機制，各自能取得的東西不同：

| 機制 | 驗證對象 | 需要的 secret | 能拿到什麼 |
|---|---|---|---|
| JWKS 驗證 | `identityToken`（JWT） | 無（Apple 公鑰公開） | `sub`／`email`／`exp`／`aud` |
| Token 交換 | `authorizationCode`（一次性） | `client_secret`（ES256 `.p8` 私鑰簽出，上限 6 個月） | `refresh_token`／`access_token`／`id_token`，以及呼叫 Revoke API 的能力 |

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 只做 JWKS 驗證 | 拿不到 `refresh_token` ⇒ 帳號刪除時無法對 Apple revoke 授權，也偵測不到使用者在 Apple 設定中撤銷授權 | 不足 |
| 只做 Token 交換 | 每次登入都要打 Apple token endpoint，登入路徑多一個外部同步依賴與一份要輪替的 `.p8` 私鑰 | 不必要 |
| **兩者各司其職** | 要同時維護 JWKS cache 與 `.p8` 私鑰輪替 | ✓ |

## 決策

**兩條路徑並存，依用途分工：**

- **登入**走 JWKS 驗證 `identityToken`。登入是高頻路徑，不該為它綁一個會過期的私鑰與一次
  外部 POST；`identityToken` 有效期只有 10 分鐘，後端驗一次之後改用自己簽的 JWT 維持
  session，不再依賴 Apple token 的有效性。
- **帳號刪除**走 token 交換 ＋ Revoke API。拿 `authorizationCode` 換 `refresh_token`
  保存，刪除帳號時用它對 Apple revoke。

## 後果

- 必須維護 ES256 `.p8` 私鑰並在到期前輪替（`client_secret` 上限 6 個月）。
- revoke 是對外部的同步呼叫，不能掛在帳號刪除的關鍵路徑上 ⇒ 走 outbox ＋ worker 重試
  （`apple_revoke_outbox`），失敗會退避重試到上限後終止並留稽核事件。
- 登入路徑不受 Apple token endpoint 可用性影響；只受 JWKS endpoint 影響，而那一層有 cache。

## 尚未採用

`Sign in with Apple for Web` 不在範圍內——目前只有兩個原生 App 需要 Apple 登入。
