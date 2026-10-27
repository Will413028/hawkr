# Hawkr Admin product context

## Product and platform

Hawkr Admin is the web console used by authorized platform staff to review, support, and govern the Hawkr marketplace. It is a Next.js App Router application and uses the same-origin `/api/bff` boundary to call the platform gateway with the administrator's opaque session cookie.

## Primary users

- ADM platform operators handling seller review, onboarding, content support, and governance.
- Support and operations leads who need attributable, reviewable actions rather than access to a seller's identity.

## Delegated operations purpose

ADM delegated operations lets an authorized administrator help a specific seller maintain permitted storefront, product, and post content without logging in as that seller.

The administrator remains the authenticated actor. The backend must issue a short-lived operation session bound to:

- the administrator actor;
- one target seller account and store;
- a reason and case or ticket reference;
- an explicit capability set;
- an expiry and revocation state.

The Admin UI may enter the delegated workspace only after a server preflight confirms eligibility and a server-issued operation session is active. Missing, expired, revoked, mismatched, or unsupported sessions must fail closed.

## Non-negotiable safety boundaries

- Never request, expose, exchange, or reuse a seller login token or seller session cookie.
- Never route ADM through seller-authenticated `/app/...` APIs.
- Never imply that a merchant can be operated when the backend capability is unavailable.
- Never permit order fulfillment, shipment, cancellation, refund, payout, bank-account, identity, password, or other sensitive-account actions from the delegated workspace.
- Orders may be shown only as read-only context when the server capability explicitly permits it.
- Every write must carry operation-session context, an idempotency key, target resource version, and audit metadata; the backend remains the authorization authority.
- Session end, expiry, revocation, target mismatch, or capability loss immediately disables writes and provides a safe route back to the seller record.
- Client-side hiding is not authorization. UI capability checks improve clarity; server enforcement is mandatory.

## Experience commitments

- Traditional Chinese is the default product language; controls name the actual action and recovery path.
- The delegated state is continuously visible through a persistent amber context bar showing the target store, operator context, reason or case, remaining time, and a clear end action.
- The workspace reuses familiar Seller Center information architecture where safe, but repositories, mutations, routes, and authorization adapters are Admin-specific.
- Hawkr semantic colors are the baseline: brick `#b8441f`, brand orange `#e4572e`, cream `#f8f6ef`, paper `#fffdf7`, amber `#f2a541`, ink `#191c18`, body `#42493f`, muted `#72796e`, and AA small-text muted `#5f675b`.
- Motion is functional and restrained. Support reduced-motion preferences.

## Accessibility requirements

- The delegated context is a named landmark and session-status changes are announced through `aria-live`.
- Dialogs trap focus, close with Escape when safe, restore trigger focus, and make the background inert.
- Interactive targets are at least 44 by 44 CSS pixels with visible keyboard focus.
- Text and controls meet WCAG AA contrast, remain usable at 200% zoom, and reflow at a 1024px viewport without hiding safety information.

## Current delivery boundary

This repository can implement the Admin entry, preflight experience, same-origin BFF contract, delegated workspace shell, capability-aware navigation, expiry or revocation blocker, and safe session ending.

The production backend endpoints and authorization policy for delegated operation sessions are not yet verified in `backend-payuni`. Until those endpoints exist and are deployed, the implementation must show an explicit unavailable or failed state and must not fabricate a successful session.

## Evidence and references

- Product specification: `Hawkr_ADM代行營運_詳細設計書_v2.0.docx` supplied through Google Docs.
- Implementation plan: removed from the repository once its decisions were recorded; the original (`docs/superpowers/plans/2026-08-06-adm-delegated-operations.md`) is in the private archive.
- Public interaction references: Refero dashboard patterns for Harvest, Cal.com, and ClickUp. They inform hierarchy, density, and safety-state clarity only; Hawkr tokens and product boundaries remain authoritative.

## Open decisions

- Final backend endpoint paths, response envelopes, capability names, TTL policy, and revocation semantics.
- Whether shared Seller/Admin presentation primitives live in a versioned package or are first extracted within each host before an ADR-backed move.
- Final audit-event schema, outbox delivery contract, notification policy, and production rollout feature flag.
