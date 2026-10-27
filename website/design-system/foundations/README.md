# Foundations

Foundations 是穩定的產品層規則。這一層應該慢慢變動，並優先透過 tokens 反映到元件，而不是直接散落在各頁 CSS。

## 規範範圍

| 範圍 | 責任 |
| --- | --- |
| Color | 品牌色、semantic surfaces、text、border、action roles、status roles |
| Typography | 字體、字級、行高、字重、閱讀密度 |
| Spacing | Page gutters、layout gaps、component spacing、compact/dense modes |
| Radius | 元件圓角層級與 pill shape 使用時機 |
| Elevation | Shadow tiers、overlays、sticky nav、modal/dropdown depth |
| Motion | Duration、easing、page transitions、loading indicators |
| Accessibility | Focus visibility、contrast、reduced motion、keyboard behavior |
| Content | 語氣、label、empty state、error、seller/buyer 術語 |

## 目前工程對照

目前 Tailwind bridge 放在 `styles/globals.css` 的 `@theme`。新的 utility-facing token 先統一使用 `hk-*` namespace，等 foundation 層成熟後再評估是否抽成正式 token package。
