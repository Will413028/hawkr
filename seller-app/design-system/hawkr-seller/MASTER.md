# Hawkr Seller — iOS 26 Liquid Glass Design System

Status: implementation source of truth  
Product brief: `docs/design/SELLER_APP_PRODUCT_UX_MASTER_SPEC.md`

## 1. Design direction

Hawkr Seller is an agricultural commerce operations app. Its interface
combines three compatible directions:

- Liquid Glass for the functional layer: navigation, top bars, tab tracks,
  sheets and compact floating controls.
- Organic biophilic surfaces for the brand layer: brick orange, amber, warm
  cream, restrained produce accents and rounded geometry.
- Soft UI evolution for the content layer: clear contrast, subtle depth,
  stable cards and low-cost motion.

Glass is not a decorative wallpaper. Long lists, forms, media and dense data
use opaque or near-opaque surfaces so text remains readable and scrolling
stays performant.

## 2. Brand and semantic palette

| Role | Token | Value |
|---|---|---|
| Brand mark, large headings, decoration | `brandPrimary` | `#E4572E` |
| Primary action | `brandPrimaryStrong` | `#B8441F` |
| Hover and pressed | `brandPrimaryDeep` | `#8F3315` |
| Highlight | `brandSecondary` | `#F2A541` |
| Brand text | `brandInk` | `#1F2933` |
| Warm border | `neutralPanel` | `#F2E9D6` |
| Page | `surfacePage` | `#F8F6EF` |
| Card | `surfaceCream` | `#FFFDF7` |
| Strong text | `neutralInk` | `#191C18` |
| Body text | `neutralBody` | `#42493F` |
| Muted text | `neutralMutedText` | `#5A6154` |
| Success | `success` | `#2D7F50` |
| Warning | `warning` | `#E0C44C` |
| Danger | `danger` | `#EA6021` |
| Information | `info` | `#1E5FAB` |

Do not replace these colors with the generic red marketplace palette returned
by the external design-system search. Semantic state must always include text
or an icon and must never rely on color alone.

## 3. Surface hierarchy

1. Page canvas: warm opaque `surfacePage`.
2. Content card: `surfaceCream`, 16–24 radius, one subtle ink-tinted shadow.
3. Glass chrome: translucent cream/white gradient, 14–20 sigma blur, 1 px
   white highlight border and restrained depth shadow.
4. Selected glass control: a brighter inner glass capsule; brand green is used
   for icon/text, not as a large filled slab.
5. Modal: compact glass or opaque cream sheet over a readable scrim.

Only small clipped areas may use `BackdropFilter`. Every blurred surface must
be isolated with `RepaintBoundary`.

## 4. Geometry and spacing

- Spacing follows a 4/8 logical-pixel rhythm.
- Page gutter: 16 compact, 24 tablet, 32 wide.
- Touch target: at least 48×48 logical pixels.
- Card radius: 16 standard, 24 feature/dialog, 32 hero.
- Search field: 44 high; primary controls: 48 high.
- Top chrome: 56 high excluding the status safe area.
- Bottom navigation: 68 high plus 12 side/bottom safe-area inset.
- Content under floating navigation reserves navigation height and safe area.
- Layout switches from bottom navigation to a wider constrained layout at
  tablet/wide widths; typography and touch targets do not scale with width.

## 5. Typography

Use the existing `AppTypography` and platform text scaling. The interface does
not add Calistoga, Inter or a second font family.

- Screen title: 20 / 700.
- Section title: 18 / 700.
- Body: 16 / 400–500.
- Metadata: 14 / 400–500.
- Navigation label: 11–12 / 600.
- Price and operational numbers use stable figures where available.

Text wraps before truncating. If truncation is unavoidable, retain a semantic
label containing the complete value.

## 6. Icon contract

- All product icons use `HkIcon` or `AppIcons`, backed by Material Icons.
- Main navigation draws the filled glyph for the current tab and the outlined
  glyph for the others, next to a text label.
- Outline icons are the resting state; their selected/active variants are used
  only for current location or explicit confirmation.
- Standard UI icon size is 24; main navigation may use 28–30.
- Icon-only controls require a tooltip and semantic label.
- No emoji or feature-local Material icon substitutions.
- The Hawkr logo is reserved for home/store identity and may not replace
  an unrelated action icon.

## 7. Navigation model

Top-level navigation has exactly five labeled destinations:

1. 總覽
2. 商店
3. 錢包
4. 訂單
5. 設定

Bottom navigation is stable across all five roots. Store sub-tabs are exactly
貼文 / 商城 / 留言. Chat is a dedicated route and action, not a hidden fourth
store tab. Back navigation preserves branch state and system swipe-back.

## 8. Motion and feedback

- Standard transition: 200 ms, `Cubic(0.22, 1, 0.36, 1)`.
- Press feedback: 100–160 ms, scale no smaller than 0.97.
- Content replacement uses fade/crossfade, not layout-jumping animation.
- Haptics are limited to selection changes and confirmed important actions.
- Respect `MediaQuery.disableAnimations`; all required content remains
  immediately readable without animation.
- Long loading uses a skeleton or branded progress state; errors always offer
  a recovery action.

## 9. Shared state contract

Every network-backed screen supports:

- initial loading with reserved layout;
- refreshing without clearing usable content;
- empty state with purpose-specific guidance and one relevant action;
- offline/timeout/error with cause and retry;
- mutation pending with disabled duplicate submit;
- mutation success or failure feedback;
- partial data without collapsing the whole screen.

## 10. Accessibility and validation

- 4.5:1 contrast for normal text, 3:1 for large text and meaningful graphics.
- VoiceOver labels describe action and state, not just icon shape.
- Reading order matches visual order.
- Dynamic text does not clip at common accessibility sizes.
- Small phone, iPhone safe areas, landscape and tablet constraints are checked.
- No horizontal overflow or nested primary scroll regions.
- Tap response appears within 100 ms.
- Blur is verified with transparency/reduced-motion accessibility settings.

## 11. Flutter implementation map

- Color roles: `lib/core/theme/app_colors.dart`
- Geometry/motion/glass: `lib/core/theme/hk_tokens.dart`
- Global components: `lib/shared/widgets/`
- Icon contract: `lib/shared/icons/` and `lib/shared/widgets/hk_icon.dart`
- Global Material 3 theme: `lib/app.dart`

New feature screens must consume these shared foundations and must not introduce
new raw brand colors, spacing scales, icon libraries or glass recipes.
