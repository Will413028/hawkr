# social/internal/hashtag

hashtag 與熱門標籤。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/social.md`。

## 不變量

- 熱門標籤的窗口最長 30 天。守門：`TestTrendingHashtagsWindowIsCappedAtThirtyDays`。規格：§2。
