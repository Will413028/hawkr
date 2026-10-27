# Hawkr Design System

這個目錄是 Hawkr 的產品設計系統層。現階段先以文件與規範為主；實際 React runtime 元件仍放在 `components/ui`、`components/shared`、`features/*`。

## 分層

```txt
foundations/  品牌、色彩、字級、間距、motion、accessibility、content tone
tokens/       raw / semantic / component token 命名與 Tailwind @theme 對照
patterns/     由 UI 元件組成的可複用產品體驗解法
migration/    component inventory、ownership、deprecated 清單、搬遷批次
```

## 規則

- `design-system` 擁有設計決策，不放 route code。
- `components/ui` 實作無產品語意、token-backed 的 generic 元件。
- `components/shared` 實作跨 feature 的產品語意元件。
- `features/*` 擁有 domain workflow 與 feature-only components。
- `app/*` 維持 route shell：params、metadata、layouts、feature imports。
