# Component Migration Inventory

最後更新：2026-05-21

## 已完成搬遷

| From | To | 原因 |
| --- | --- | --- |
| `components/StubPage.tsx` | `components/shared/StubPage.tsx` | 共用 placeholder，不屬於 route |
| `components/LegalPage.tsx` | `features/legal/LegalPage.tsx` | legal routes 應是 feature module |
| `components/AddToCartButton.tsx` | `components/shared/AddToCartButton.tsx` | 跨 market/product/home/search 的產品語意元件 |
| `components/ToggleActionButton.tsx` | `components/ui/ToggleActionButton.tsx` | 無產品語意的 local toggle primitive |
| `components/CartClient.tsx` + `components/cart/*` | `features/cart/*` | cart domain ownership |
| `components/feed/*` + `components/FeedStoryShopping.tsx` | `features/feed/*` | feed domain ownership |
| `components/home/*` + `components/HeroCarousel.tsx` | `features/home/*` | home domain ownership |
| `components/market/*` + market root clients | `features/market/*` | market domain ownership |
| `components/messages/*` | `features/messages/components/*` | buyer/seller message shared feature |
| `components/orders/*` | `features/orders/components/*` | buyer/seller order shared feature |
| `components/product/*`, `ProductGallery`, `ProductPurchaseBox` | `features/product/components/*` | PDP 專用組成與購買流程 |
| `components/seller/*` | `features/seller/components/*` | seller-facing reusable feature components |
| `components/sellers/*`, seller storefront/review clients | `features/sellers/*` | public seller storefront ownership |
| `components/auth/*` | `components/shared/auth/*` | 跨 feature login gate |
| `components/banner/*` | `components/shared/banner/*` | 跨頁 banner 元件 |
| `app/*` top-level route folders | `app/(public|commerce|account|seller|auth|content|internal)/*` | route group 分區，URL 不變 |
| `app/**/\*Client.tsx` route-adjacent screens | `features/*/screens` 或 feature domain folders | route shell 與業務畫面分離 |
| `app/(commerce)/cart/page.tsx` | `features/cart/CartPage.tsx` | cart server composition 下沉到 feature |
| `app/(public)/products/page.tsx` + client page | `features/products/*` | product listing ownership |
| `app/(public)/search/page.tsx` + `_components` | `features/search/*` | search composition 與 rail components 下沉 |
| `app/(public)/page.tsx` | `features/home/HomePage.tsx` | home composition 下沉 |
| `app/(public)/feed/page.tsx` | `features/feed/FeedPage.tsx` | feed server loader 下沉 |
| `app/(content)/contact|faq` | `features/content/contact|faq` | content client pages 下沉 |
| `app/(content)/privacy|terms|return-policy` | `features/legal/pages/*` | legal pages ownership |
| `app/(internal)/ui-kit/page.tsx` | `features/ui-kit/UiKitPage.tsx` | internal showcase 下沉 |
| seller-center client pages | `features/seller-center/*` | seller workspace domain ownership |

## 下一批安全搬遷

| Batch | Files | Target | Risk |
| --- | --- | --- | --- |
| Public seller detail shell | `app/(public)/sellers/[id]/page.tsx`, layout, product detail route | `features/sellers/storefront/` | 低到中 |
| Message layout shells | `app/(account)/account/messages/layout.tsx`, `app/(seller)/seller-center/messages/layout.tsx` | `features/messages/` shared shell | 中，layout boundary 需小心 |
| Remaining stub/content routes | `daily-posts`, `topics`, `seasonal`, `recipes`, `explore` | `features/content` or feature-specific pages | 低 |
| `lib/hooks/*` domain split | account/order/chat hooks | `features/*/hooks` or `lib/<domain>/hooks` | 中，需同步測試 |

## 暫時不要搬

- `lib/services/*` 與 `lib/<domain>/repo.ts` 的去重，等 UI 路徑穩定後再做。
- 任何把 CSS 大量改寫成 Tailwind 的動作，應逐 feature 做 visual smoke test。
