# seller/internal/profile

店家 profile 的寫入。

## 不變量

- 寫入以版本樂觀鎖，拒絕都在 token 轉正之前。守門：`TestRejectedProfileUpdateDoesNotConsumeTheStagedToken`。規格：§3。
