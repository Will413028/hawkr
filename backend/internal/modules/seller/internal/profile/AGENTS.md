# seller/internal/profile

店家 profile 的寫入。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/seller.md`。

## 不變量

- 寫入以版本樂觀鎖，拒絕都在 token 轉正之前。守門：`TestRejectedProfileUpdateDoesNotConsumeTheStagedToken`。規格：§3。
