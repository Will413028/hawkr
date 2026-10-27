# App Router

`app/` is the Next.js routing boundary.

Allowed here:

- `page.tsx`
- `layout.tsx`
- `loading.tsx`
- `route.ts`
- metadata and route params/search params parsing

Keep route files thin. Route-facing screens, workflow state, forms, tables,
cards, and modals should live in `features/`.

Preferred page shape:

```tsx
import CartPage, { metadata } from "@/features/cart/CartPage";

export { metadata };
export default CartPage;
```
