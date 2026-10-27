---
version: 9
slug: "seller-pricing-page-tsx"
primary_target: "app/[locale]/(seller)/seller/pricing/page.tsx"
related_targets: ["features/seller/pricing/SellerPricingPage.tsx", "features/seller/pricing/SellerPricingEffects.ts", "features/seller/pricing/public-data.ts", "styles/features/seller-pricing.css", "app/api/public/seller-leads/route.ts", "third_party/react-bits/NOTICE.md", "features/seller/landing/SellerLandingPage.tsx", "components/shared/Footer.tsx", "components/shared/SellerFooter.tsx"]
---

# Seller pricing and cooperation plans

- Scope: Present `/seller/pricing` as the public self-service storefront entry plus a concise comparison with only Free and Pro, a monthly or annual payment choice, and one optional managed-service add-on. This is a food-marketplace cooperation page, not a generic SaaS pricing template or a seller-center operating screen.
- Audience: Farmers, cooperatives, food brands, and established suppliers deciding how they want to pay for the Hawkr marketplace and transaction foundation.
- Primary task: Start self-service onboarding or compare the two public plans and payment cycles. Human follow-up is optional for sellers who still need help choosing a plan or confirming fit.
- Chosen direction: Preserve the Hawkr warm paper, brick orange, rounded brand type, semantic tokens, shared controls, and one existing Daily Village image. Reuse the home page's tall illustrated backdrop and rounded postcard section rhythm so pricing belongs to the same visual world. Keep Free and Pro isomorphic: the same top edge, border, height, icon treatment, billing-note reserve, and selection affordance. Keep the optional managed add-on inside the plans chapter as a compact full-width choice, not a third card or separate chapter. Borrow VibePrompts' explicit billing-total, selection-recap, and two-step handoff hierarchy; add React Bits-inspired, page-scoped reveal and spotlight effects without copying another brand's palette, text, assets, or continuous motion.
- Direction seed: `free-pro-stable-postcard-interactions-v3`.

## Five-section narrative

1. Hero: one self-service-marketplace headline, one short explanation of the review boundary, one primary onboarding CTA, one secondary comparison CTA, and one static Daily Village image masked with the canonical brand mark.
2. Plans: a native monthly or annual radio switch followed by equal Free and Pro radio cards. Pro annual is a 10-month price for 12 months. Selecting a card never scrolls; the compact managed add-on appears directly below both cards and before the shared continue action. It states the extra 6% fee, the two supported tasks, the seller's retained order and fulfillment responsibilities, and the combined Free or Pro fee examples. Monthly and annual notes remain mounted in the same grid area so billing changes never move the cards, add-on, or later sections.
3. Responsibility boundary: a compact definition list for platform responsibility, seller responsibility, and separately billed costs. Do not restore the former comparison table.
4. FAQ: public questions about Free, Pro, annual payment, managed service, no-sale months, refunds, plan changes, and responsibility. The visible FAQ and JSON-LD must use the same data.
5. Optional human help: collect brand or farm name, contact person, product category, at least one of email or phone, an optional plan, payment cycle, and optional managed-service interest. Submission is not required for self-service onboarding and is not payment, approval, or a signed agreement.

## Public and legacy plan boundary

- The public page offers Free and Pro only.
- Free keeps internal id `starter` and billing code `STARTER`. The card and FAQ must state that the formal agreement and billing name still use Starter until legal and billing documents are migrated.
- Pro keeps internal id and billing code `PRO`.
- The public draft offers Pro monthly at NT$2,990 or annual at NT$29,900. Annual selection must travel to intake as `billingCycle=annual` and must not rewrite the internal plan id.
- The managed add-on travels separately as `serviceInterest=managed`; it must never be modeled as a third plan or silently bundled into Free or Pro.
- Public data lives in `public-data.ts` and must not import or derive from the three-plan legacy array at client runtime.
- The public component, metadata, FAQ JSON-LD, visible form options, image scene labels, and browser bundle must not contain Growth, NT$990, or 3% plan copy.
- The intake API may continue accepting legacy `growth` submissions from stale clients, but must label them as old submissions requiring manual confirmation. It must never silently convert Growth to Free or Pro.
- Existing legal documents remain unchanged in this page redesign. They are not proof that the public two-choice model is contractually migrated.
- The current legal attachment does not yet define annual billing and currently says human-service bundles use fixed pricing rather than a transaction percentage. Annual billing and the managed 6% add-on are therefore target commercial terms that require a new agreement version before release.

## Commercial truth

- Free: NT$0 monthly fee plus 6% transaction service fee on valid marketplace transactions.
- Pro: NT$2,990 monthly fee plus 0% transaction service fee.
- Pro annual draft: NT$29,900 for 12 months, saving NT$5,980 compared with 12 monthly payments.
- Managed add-on draft: 6% of each valid transaction in addition to the selected plan's transaction service fee. It covers listing assistance by the operations-support team and refinement of seller-provided e-commerce images. The seller still accepts orders and remains responsible for stock, fulfillment, consumer customer service, and after-sales.
- Displayed monthly fees and percentages include 5% business tax.
- Payment processing, logistics, ads, and human services remain separate.
- Pro 0% refers only to the transaction service fee.
- The seller remains responsible for food facts and legality, pricing, inventory, order acceptance, fulfillment, customer service, after-sales, and content rights.
- The platform does not guarantee traffic, ranking, conversion, or revenue.

## Interaction and accessibility

- Selecting a billing cycle updates the cards, recap, and form without changing layout geometry. Selecting a card updates the optional plan field and carries the internal `starter` or `pro` id to onboarding and intake, but does not scroll. The shared continue action performs the reduced-motion-aware handoff to the focused application heading. Selecting managed service uses a native checkbox, updates only the add-on field and recap, and does not scroll.
- The hero image is decorative, uses Next Image, reserves its aspect ratio, and has empty alternative text.
- Motion archetype is Corporate and intensity is 3. Use the shared `--ease-standard` curve for 220ms state changes and `--ease-emphasized` for the 480ms one-time reveal. Each below-fold chapter may enter once with opacity and a 14–24px transform; plan cards may use a pointer spotlight, the managed add-on uses only selection feedback, and the form may use a focus-within glow. Price and billing copy change only inside reserved geometry. There is no window scroll listener, sticky pricing bar, continuous loop, parallax, canvas trail, text splitting, blur animation, or calculator animation.
- Effects are progressive enhancement: SSR, no-JavaScript, no-IntersectionObserver, reduced-motion, coarse-pointer, and forced-colors states retain fully visible and usable content. Initial viewport elements remain visible when the controller starts so hydration cannot flash visible content back to hidden.
- React Bits-inspired source and license records live under `third_party/react-bits/`. VibePrompts is a layout reference only; no source, text, or visual asset is copied.
- Preserve semantic sections and headings, native form controls, visible focus, 48px primary actions, reduced-motion behavior, forced-colors support, dark semantic tokens, and no horizontal overflow through 390px.
- Form success requires an HTTP-successful same-origin `/api/public/seller-leads` response with `ok: true`. Success means the intent was received, not approval, signed terms, guaranteed follow-up time, or completed onboarding.

## Evidence and release boundaries

- Local source, unit tests, browser tests, and screenshots prove only the local implementation.
- This redesign does not prove contract migration, billing migration, deployment, production lead delivery, or commercial approval.
- Before public release, business, billing, and legal owners must decide whether to rename Starter, what happens to existing Growth subscribers, approve the NT$29,900 annual amount, define renewal and cancellation, and replace the fixed-price human-service rule with the managed 6% add-on terms.
