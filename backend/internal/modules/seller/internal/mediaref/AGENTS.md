# seller/internal/mediaref

店家圖片的 media 引用同步。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/seller.md`。

## 不變量

- media 引用只由 `seller.media_sync` 寫，transaction 外只 `Finalize`，退場資產不能寫回。守門：`TestImagesLinkByStoreIDAndUseVersion`。規格：§9。
