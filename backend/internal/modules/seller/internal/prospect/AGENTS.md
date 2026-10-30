# seller/internal/prospect

商家名單。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/seller.md`。

## 不變量

- 商家名單統編在未封存業者間唯一。守門：`TestTaxIDIsUniqueAmongLiveProspects`。規格：§15。
