---
version: 1
slug: "app-hawkr-agent-rfq-preview-screen-page-tsx"
primary_target: "app/[locale]/hawkr-agent-rfq-preview/[screen]/page.tsx"
related_targets: ["features/procurement-rfq-prototype/RfqPrototype.tsx","features/procurement-rfq-prototype/rfq-prototype.module.css","features/procurement-rfq-prototype/data.ts","features/procurement-rfq-prototype/types.ts","app/[locale]/hawkr-agent-rfq-preview/page.tsx","features/procurement-assistant/CandidateBrowser.tsx","features/procurement-assistant/AssistantWorkspace.tsx"]
---

# Hawkr Agent v5.0 RFQ target-state preview

## Scope and mode

- Operate mode for the routable `/hawkr-agent-rfq-preview/[screen]` frontend prototype.
- Covers the five-step formal-quote vertical slice plus the later v5 target screens; it does not replace the live procurement session state machine.

## Audience, job, and action

- Buyers confirm recipients, publish a versioned RFQ, compare immutable responses, and stop at shortlist in Wave 1.
- Platform sellers find an RFQ, save a quote draft, and make one explicit formal submission.
- Later-wave screens explain checkout, ATM locking, external-vendor qualification, fulfillment, aftersales freezing, and finance dual review without claiming those contracts exist.

## Proof and constraints

- Use synthetic but operationally specific RFQ, quote, deadline, qualification, payment, fulfillment, and settlement data.
- Every screen must identify its wave, role, and prototype boundary; no automatic acceptance, payment, order creation, seller contact, or fulfillment.
- Only the demo Assistant conversation links into this preview. Live conversations remain on the implemented candidate flow.

## Direction and memorable moment

- Extend Hawkr's warm-white, brick-orange Operate language with a dense role-aware workspace: step rail, version/status bands, task surface, and request summary.
- The memorable moment is the quote comparison boundary: shortlist remains a reversible buyer decision, with later transaction screens visibly separated.

## Unresolved decisions

- Backend RFQ, response, checkout, payment, order, KYC, aftersales, and settlement contracts remain unimplemented.
- Authentication and production role routing for buyer, seller, external vendor, and finance surfaces require backend/API scope before release.
