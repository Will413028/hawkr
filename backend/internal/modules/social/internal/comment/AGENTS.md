# social/internal/comment

留言與回覆。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/social.md`。

## 不變量

- 回覆的 partial index 述詞是旗標。守門：`TestReplyIndexesAreOnTheCountedFlag`。規格：§4。
