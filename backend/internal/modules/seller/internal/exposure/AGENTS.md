# seller/internal/exposure

店家可曝光狀態的計算與公告。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/seller.md`。

## 不變量

- 可曝光公告只經 `exposure.Announce`，先鎖再算。守門：`TestAnnouncementLocksBeforeItComputes`。規格：§12。
