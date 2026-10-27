# Patterns

Patterns 是可複用的產品體驗解法。它們把 foundations 與 components 組合成跨 feature 會反覆出現的流程。

## 初始 Pattern Backlog

| Pattern | 主要使用者 | 說明 |
| --- | --- | --- |
| Page header | account、seller-center、market | Title、description、breadcrumbs、primary/secondary actions |
| Resource index | seller-center、account | Table/list、filters、sort、saved views、pagination、empty state |
| Resource details | orders、products、sellers | Summary、status、timeline、action cluster |
| Form flow | seller-center、checkout、auth | Field groups、validation、sticky actions、conflict states |
| Empty state | all domains | Illustration/icon、explanation、primary action、optional secondary link |
| Filter toolbar | market、search、seller-center | Search、chips、advanced filters、reset、result count |
| Product card grid | market、search、home | Product image、seller、price、temperature、CTA、skeleton |
| Modal/sheet | cart、auth、messages | Overlay behavior、mobile sheet behavior、focus management |

## 升級規則

先從 `features/<domain>/components` 開始。至少兩個 feature 使用後，才升級到 `components/shared`。只有移除所有產品/domain 語意後，才升級到 `components/ui`。
