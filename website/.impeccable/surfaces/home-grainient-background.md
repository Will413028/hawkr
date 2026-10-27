---
version: 1
slug: "home-grainient-background"
primary_target: "components/shared/SiteGrainientBackground.tsx"
related_targets: ["components/shared/AppProviders.tsx", "components/shared/SiteBackgroundToggle.tsx", "features/home/HomePage.tsx", "lib/animation/grainient-renderer.ts", "styles/components/site-grainient.css", "styles/foundation/00-color-config.css"]
---

# Shared Grainient — four-brand-color refinement

## DEV release authorization — 2026-09-06

- User approved pushing this background batch to DEV. The application changes, regression tests and upstream attribution belong to this release; the separate Impeccable tooling update remains local.
- Pre-push full Vitest caught an unclassified legacy background pause key. Both the current and legacy keys are explicitly device-scoped, storing only a per-tab playback preference; the current constant follows the storage scanner naming convention. No account-private storage or auth behavior is changed.
- Full pre-push verification passed on the 35b43d6b base: 1,545 Vitest tests; 2,321 Node tests with one existing skip; lint (five existing warnings, twelve infos); architecture; production build with 116 static pages. CSS semantic baselines were updated only after confirming their full AST delta is the approved page-canvas paint change; tablet rail assertions now verify class tokens rather than adjacency.
- Historical local-only / not-yet-authorized notes below describe their earlier checkpoints, not this release authorization. CI and served deployment evidence must still be verified separately after push.

## All-page consistency — 2026-09-06 (latest authority)

- User explicitly requires the same background on every page after seeing differing canvases while navigating. This supersedes the former public-route allowlist and private/standalone exceptions below.
- Reference lock: keep the already approved four-brand-color field at 0.25 opacity over the original market solid base. Refero Grain continues to own ambient decoration only; Seed continues to protect product imagery. No changes to page layouts, text, actions or media roles.
- One always-visible background host under AppProviders, outside route transitions. Clear actual route/shell paint owners across public, marketing, auth, account, seller, delegated and RFQ flows; never blanket-clear descendants. Replace old whole-page artwork, bokeh and seller Vanta decoration rather than stacking ambient layers.
- Content cards, forms, dialogs, masks hiding stale data, map/video/story/guidebook stages and message surfaces retain their own functional paint. In-flow route skeletons expose the shared background. Fatal root-layout errors cannot use failed providers and keep a dependency-free solid market-color fallback.
- Full footer variants retain the pause action; standalone/delegated pages get only an in-flow compact pause control, with no buyer/seller navigation links or floating obstruction. Renderer budgets and reduced-motion/Save-Data/visibility/context-loss fallbacks are unchanged.
- Chat conversation panels keep their bounded static produce texture and foreground surfaces; the extra 90-second wallpaper loop is removed in both the eager drawer stylesheet and the lazy message stylesheet. Their geometry/paint ownership contract still passes.
- Batched visual QA found pale external-vendor prototype cards inheriting global dark text colors. Scoped the three legacy text roles, text links and the pale-header logo to that existing light foreground design without changing its shared dark page canvas.
- Implementation and checks are local only; no commit, push or deployment authorized. Historical measurements below are not a new full-site performance benchmark.

### All-page local acceptance

- Final production build passed, including TypeScript and all 116 generated pages. Lint and architecture passed; lint retains 5 existing warnings and 12 infos outside this change. No dependency changes.
- 39/39 background unit tests plus 114/114 focused Node contracts passed (background ownership, auth/footer/onboarding, marketing/pricing/story media, product previews, chat CSS parity and cart hydration/feedback). These are not a full authenticated commerce E2E suite.
- Local Chrome soft navigation through FAQ, About, Contact, Terms, Pricing, Corporate Welfare, Seller and Market kept one canvas/context and a continuously increasing clock. Separate login-to-cart soft navigation did likewise. Representative 1440 desktop / 390 mobile, light/dark captures showed the correct solid base, 0.25 opacity and no horizontal overflow. Portal contrast correction was visually confirmed. Captures: `output/playwright/grainient-all-*`.
- Authenticated account/seller/ADM interiors were checked through source owners and contracts, not a real signed-in session. Root-layout fatal failures use only the static solid fallback by design.
- Manual source detector covered 49 changed UI files. Its full-file legacy findings remain; after recovering the oversized output and matching diagnostics against changed lines, none intersect this change. This is not a whole-site accessibility certification. No further visual polish loop was run.

## Latest user refinement — 2026-09-06

- User asked to make the background lighter and restore the market's original solid base. Decorative layer opacity is now 0.25 (was 0.5), preserving all four brand inputs and animation behavior.
- The base now aliases `--theme-surface-page`, exactly the semantic source of market `--hkui-background`: light #f8f6ef, dark #143126. Removed the trial-only #0d1912 override. The dark shader brightness cap and solid content sections remain intact.
- Reuses the pinned Refero direction and Impeccable quieter guidance: decoration recedes; layout, content, media, typography and CTA are unchanged. Records below describe the earlier 50% checkpoint, not this latest visual configuration.
- The latest score measurement is local production-build Lighthouse only. Verified public DEV and PROD still serve their old illustrated backgrounds; no push/deployment has occurred.
- Latest verification: build passed; 38 background unit tests passed; four desktop/mobile light/dark captures confirmed opacity 0.25, original market solid base and unchanged section surfaces. Local Lighthouse 12.6.1, three fresh-browser runs/profile: mobile 64/69/70 (median 69), desktop 92/96/95 (median 95). Mobile LCP median 7.88s, desktop 1.47s; no old-background A/B or live-CWV improvement claim. Full evidence: `output/playwright/lighthouse-quarter-background/REPORT.md`.

## Current direction — 2026-09-06 (supersedes the initial trial below)

- User direction: use all four brand primary colors at 50% background opacity; prioritize loading speed; continue across pages; dark sections must match and be brighter than the background.
- Palette authority: existing foundation tokens primary-strong #b8441f, primary #e4572e, secondary-soft #fdf1e1, secondary #f2a541. No new CTA/status palette. Preserve the actual React Bits Grainient shader core; adapt its color field to four-corner blending.
- Persistent owner: one host under AppProviders, outside route content and transition containers. No route key or route-driven React state. Exact public DOM owners show it on home, market, feed, search, storefront tabs and published product detail (including intercepted product views). Private workspaces and standalone transaction/auth/seller flows retain their existing canvases; hiding pauses, never destroys the shared renderer.
- Runtime budget: CSS first paint; native WebGL2 chunk only after document handoff, visibility and idle. No new library. At most 320,000 drawing-buffer pixels, DPR <= 0.75 and 24 FPS. No fullscreen filter or blur; 50% opacity on decoration only. Theme changes update a brightness uniform without losing the clock/context. Reduced motion, forced colors, Save-Data, missing visibility APIs, weak/unsupported WebGL and context/chunk failure retain static CSS.
- Dark hierarchy: public major panels use the existing solid #1c2e24 token. Background base #0d1912; shader output capped at 12% brightness before the 50% layer blend. Form/status/CTA/media and inner-card roles are not globally remapped. Light section surfaces remain intact.
- Pause control: stable-size native footer button in both footer variants, backed by a tiny external store. No control over product actions; session pause preference survives route/role switches. No per-frame React updates.
- Refero reference lock remains Grain for ambient fields behind legible content and Seed for product-first imagery. Existing layout, typography, assets, product facts and interaction hierarchy remain the authority.
- Acceptance: real same-canvas identity and increasing animation time through soft navigation; exact computed dark panel equality; desktop/mobile visual checks; failure/reduced/pause gates; build/lint/unit evidence; production chunk absent from initial HTML. These checks do not claim zero GPU cost, improved live CWV or deployment.
- No commit/push/deployment authorized for this refinement. The approved Impeccable update stays a separate worktree change.

## Current local verification — 2026-09-06

- Final `pnpm build`: passed, TypeScript and 116 generated pages included. `pnpm lint`: passed with the same 5 unrelated warnings and 12 infos. Architecture and diff checks passed.
- Targeted Vitest: 38/38 (renderer, persistent host and real external store). Focused Node contracts: 31/31 across initial-home-loading, storefront-product-preview, seller-storefront-stability, seller-storefront-preview and delegated-product-preview-contract. This is targeted verification, not a full transaction E2E suite.
- Real local Chrome: 1440×960 desktop and 390×844 mobile, light/dark, no horizontal overflow. Actual background opacity 0.5. All 15 homepage major surfaces computed rgb(28, 46, 36), no gradient and no backdrop blur. Full-page PDP's 3 rendered major panels matched with blur none; quick-view portal retains its own surface ownership.
- Persistence: home → market → feed → storefront → market → intercepted product → search → home kept the exact same canvas object and one WebGL context; iTime increased from 88.88 → 89.13 → 89.59 and through later transitions. Theme switching kept the same canvas and changed the brightness uniform between 1 and 0.12.
- Runtime budget: desktop 689×464 = 319,696 buffer pixels; mobile 281×633 = 177,873 pixels. Observed 44 draws in 2 seconds. The configured maximum pixel-throughput budget is 320,000×24 vs the trial's 1,000,000×30, a 74.4% reduction in the theoretical ceiling, NOT a measured 74.4% page-speed improvement.
- Native footer pause yielded 0 draws during 600 ms; the off-allowlist FAQ route hid/paused the same context with 0 draws and returning home resumed it. Simulated `document.hidden` yielded 0 draws during 500 ms; native OS background-tab visibility is not claimed.
- Reduced-motion at initial document load: 0 WebGL contexts/canvases; the real `data-hk-initial-content-ready=home` marker remained present. Real WEBGL_lose_context removed the canvas while the full PDP content-ready marker remained. Unit tests cover forced colors, Save-Data, missing visibility APIs, late imports, weak WebGL, cleanup and storage restrictions.
- Production renderer chunk `047-g4p9-75_6.js`: 8,757 bytes raw / 3,518 gzip; absent from initial HTML. This figure is the deferred renderer only, not total page JavaScript. Shared static styles are linked by SSR. No dependencies added. The host imports its stylesheet directly so Turbopack tracks this global persistent dependency; a browser check caught and fixed a missing nested-import stylesheet before completion.
- New public-only `market-public-page` and `seller-public-tab-frame` markers avoid the identically named seller/import preview shells. Preview contracts passed; no authenticated workspace E2E claim. The seller landing's existing illustration declarations and dark filter were preserved.
- Impeccable detector on the new shared host, footer control and stylesheet: zero findings. Existing home-source advisories were outside touched lines and not expanded into a redesign. This is source/style QA, not a complete accessibility audit.
- Screenshots: `output/playwright/grainient-shared-desktop-light.png`, `grainient-shared-desktop-dark.png`, `grainient-shared-mobile-light-top.png`, `grainient-shared-mobile-dark-top.png`, `grainient-shared-mobile-control.png`, `grainient-shared-mobile-static.png`. Screenshot tests use local DEV data, not production inventory evidence.
- Branch remains `dev` at `79ec9209`; no commit, push or deployment. Live Lighthouse / Core Web Vitals were not measured, so zero loading cost or improved live scores are not promised.

## Initial homepage trial (historical baseline, not the current configuration)

- Scope: Local homepage-only trial requested on 2026-09-06. No publish, no global background rollout, no other route or commerce changes.
- Build target / primary reference: https://reactbits.dev/backgrounds/grainient?color1=ffffff&color2=809c7d&color3=b1ae51
- Reference lock: Preserve the actual Grainient shader's flowing color fields and fine grain, with exactly #ffffff, #809c7d and #b1ae51 as decorative input colors. Retain Hawkr's existing layout, imagery, fonts, cards, product facts, CTA hierarchy and light/dark surfaces. No generic replacement background.
- Bounded secondary references: Refero Grain c9b338f6-906c-4eaa-989c-bf29825dbd8c for ambient color behind readable solid content; Seed e6800fa6-1c26-40ac-a93b-68bb5de59d12 for product imagery remaining dominant. Do not borrow either site's fonts, CTA tokens or layout.
- Motion thesis: One homepage background, not a repeated section effect. Preserve upstream light-mode shader parameters. A native pause/play button in the existing bottom padding remembers the session choice; it must not float over product actions or mobile navigation. No entrance animations on content.
- Decision ledger: Replace the previous tall decorative illustration rather than stack backgrounds. Keep its assets for rollback/other consumers. Scope color inputs in the foundation color entry point. Render a CSS fallback in SSR; only load a separate native WebGL2 module after document handoff, viewport visibility and idle. No OGL/Three dependency added. A sticky viewport bounds pixel allocation regardless of page length, with 30 fps, DPR <= 1 and <= 1 million pixels. Hidden/offscreen/page-hidden pauses; unmount destroys resources. Reduced motion, forced colors, Save-Data and WebGL failure retain static CSS. Dark mode attenuates only this decorative layer.
- Acceptance: First content and layout do not wait for animation; cards and navigation stay usable; desktop/mobile and dark remain readable; no horizontal overflow or duplicate canvas after navigation; pause/resume, preference changes, delayed import and cleanup are verified separately from any deployment/performance-score claim.
- Removal condition: This is an explicit user-requested exception to the default no-continuous-ambient rule, not a new site-wide design policy. If rejected, remove the wrapper, renderer and three trial color tokens and restore the previous homepage decoration. Do not change DESIGN.md or unrelated surface briefs.

## Local verification — 2026-09-06

- `pnpm build`: passed, including TypeScript and 116 generated pages. Existing middleware-to-proxy deprecation remains unrelated.
- `pnpm lint`: passed; 5 existing warnings and 12 informational findings in unrelated files were not changed. Architecture check passed.
- Focused Vitest: 32/32 passed across the wrapper and native renderer. Initial-home-loading Node tests: 7/7 passed.
- Browser: 1440×960 desktop and 390×844 mobile, light/dark, no horizontal overflow. Canvas stays viewport-sized at scroll Y=1200. Pause control occupies existing homepage bottom padding, not product content or bottom navigation.
- Real WebGL2: one canvas, desktop drawing buffer 1218×820 = 998,760 pixels; 53 draws over 2 seconds (below the 30 fps ceiling); pause produced zero draws. Simulated hidden-document visibility produced zero draws, then resumed. Native background-tab visibility was not proven by this automation environment (`document.hidden` remained false).
- Route transition: market has zero Grainient canvases, back to home has one. Explicit `WEBGL_lose_context` failure removes the canvas and leaves the homepage content-ready marker intact. Reduced-motion removes both canvas and pause control.
- Fixed the actual CSS optimizer regression: `#ffffff` becomes `#fff` in computed custom properties. Both HEX forms now parse and a runtime unit test protects it. GPU handles are unbound before deletion to avoid deferred program release.
- Production shader chunk: 8,284 bytes raw / 3,322 bytes gzip. Not referenced by the initial HTML. SSR contains the static background and content-ready marker; the former tall background image is no longer in homepage HTML. This is bundle/implementation evidence, not a Lighthouse score or zero-GPU-cost claim.
- Impeccable manual source detector: zero primary findings, one 12px font-size advisory. Reviewed against DESIGN.md line 294 (12px dense controls) and the existing `--text-xs` 0.75rem step; no design-system rewrite or new exception needed. Source detection is not a full accessibility audit.
- Screenshots: `output/playwright/grainient-desktop-light.png`, `grainient-desktop-dark.png`, `grainient-mobile-light.png`, `grainient-mobile-dark.png`, `grainient-mobile-reduced.png`, `grainient-mobile-control.png`. Dark/reduced captures precede the control-only placement fix; final light/control captures confirm it.
- Local browser observed an unrelated `/favicon.ico` 404 after route navigation; no Grainient runtime error escaped its fallback boundary.
- No commit, push, DEV deployment or production deployment was performed. The separately user-approved Impeccable 4.0.4→4.2.1 updater changed its skill files; those are distinct from this application trial.
