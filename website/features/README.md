# Features Architecture

這個目錄是產品 feature modules 的主要 ownership。Feature 可以擁有 page composition、local components、hooks、adapters、types。`app/` route files 應盡量維持薄層，只負責 routing、metadata、layout boundary。

建議 page pattern：

```tsx
// app/seller-center/products/page.tsx
import { SellerProductsPage } from "@/features/seller-center/products";

export default function Page() {
	return <SellerProductsPage />;
}
```

Feature module shape：

```txt
features/<domain>/<feature>/
  index.ts
  <FeaturePage>.tsx
  screens/
  components/
  hooks/
  adapters/
  types.ts
```

規則：

- `app/**/page.tsx` owns routing, metadata, and layout boundaries only.
- `app/**/page.tsx` 不放 `*Client.tsx` 或厚實作；route-specific 畫面請放進 `features/<domain>/screens` 或 feature page。
- Feature pages may compose UI and call feature hooks.
- `screens/` 用來放 route-facing client screens 或較厚的 page implementation，例如 `features/account/screens/orders/OrderDetailClient.tsx`。
- Feature hooks call services/repos from `lib/**`.
- Cross-domain UI belongs in `components/shared` or `components/ui`, not inside a feature.
- Feature-specific CSS should live in `styles/features/<feature>.css`.
- Domain components 不放在 `components/<domain>`；請放在 `features/<domain>/components`。
- `components/` 只保留 `shared/` 與 `ui/`。
- `app` 到 feature 的 import 應使用 `@/features/...`，避免 `from "./..."` 讓 route folder 重新長出私有元件。
- Feature 不再建立自己的 `images/`、`icons/` 平行資產資料夾；import 型資產放 `assets/`，URL 型資產放 `public/`。
