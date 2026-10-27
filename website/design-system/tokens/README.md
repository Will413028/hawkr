# Tokens

Hawkr 採三層 token：

| 層級 | 用途 | 例子 |
| --- | --- | --- |
| Raw | 品牌或 primitive value | `brand-primary-strong`, `brand-secondary-soft`, `radius-md` |
| Semantic | 可隨 theme 切換的 UI intent | `surface-page`, `text-heading`, `action-primary` |
| Component | semantic token 不足時的元件專屬角色 | `button-primary-bg`, `card-border`, `nav-bg-selected` |

## Tailwind 對照

Tailwind v4 theme variables 目前在 `styles/globals.css` 透過 `@theme` 定義。

Examples:

```css
@theme {
  --color-hk-surface-page: var(--theme-surface-page);
  --color-hk-heading: var(--theme-text-heading);
  --color-hk-primary: var(--theme-action-primary);
  --radius-hk-md: 12px;
  --shadow-hk-card: var(--theme-shadow-brand-soft);
}
```

## 命名規則

- Tailwind 對外暴露的專案 utility 使用 `hk-*`。
- UI code 優先使用 semantic 名稱：`bg-hk-surface-card`、`text-hk-heading`、`border-hk-border-soft`。
- 新元件避免直接寫 raw hex；除非是在記錄新的 token candidate。
- 只有 reusable component 有穩定設計角色、且 semantic token 無法描述時，才新增 component token。

## Tailwind-first Component Rules

- Component variants must use static class maps. Do not build class names with
  template strings like `bg-${color}-600`.
- Use `cn()` from `@/lib/utils/cn` whenever a component combines base classes,
  variant maps, and caller-provided `className`.
- During migration, component code may reference existing CSS variables such as
  `--hkui-primary` through Tailwind arbitrary values:

```tsx
className={cn(
  "rounded-[var(--hkui-radius-md)] bg-[var(--hkui-primary)]",
  "text-[var(--hkui-primary-foreground)]",
)}
```

- Compatibility CSS may remain temporarily, but migrated primitives should not
  depend on legacy `hkui-*` selector classes.
