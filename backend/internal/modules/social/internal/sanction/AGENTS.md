# social/internal/sanction

買家的社群處分與隱藏紀錄。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/social.md`。

## 不變量

- 隱藏與還原先鎖目標內容列。守門：`TestDirectHideRowRacingADeleteCountsOnce`。規格：§8。
- 隱藏、還原、警告的理由先 `trimtext.Trim`。守門：DB 的 `trimtext.Check` CHECK、`TestTrimmedTextChecksUseTheCanonicalPattern`（`cmd/hawkr`）。規格：§8。
