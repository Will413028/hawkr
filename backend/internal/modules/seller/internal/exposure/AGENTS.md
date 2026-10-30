# seller/internal/exposure

店家可曝光狀態的計算與公告。

## 不變量

- 可曝光公告只經 `exposure.Announce`，先鎖再算。守門：`TestAnnouncementLocksBeforeItComputes`。規格：§12。
