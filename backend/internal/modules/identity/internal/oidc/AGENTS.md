# identity/internal/oidc

App 原生登入的 ID token 驗證。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/identity.md`。

## 不變量

- App 原生登入的 ID token：Google 驗 `iss`（兩種寫法）、`aud` 為 web client ID、`azp` 在允許清單、`nonce`；LINE 以 ES256 與 JWKS 驗；nonce 先比對 ID token、通過才單次消耗。`RevokeSession` 的重新驗證期限是 `ReauthWindow`。規格：`docs/modules/identity.md` §1。
