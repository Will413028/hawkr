---
version: 2
slug: "account-messages-assistant-conversationid-page-tsx"
primary_target: "app/[locale]/(account)/account/messages/assistant/[conversationId]/page.tsx"
related_targets:
  - "features/home/components/HomeCompactSearch.tsx"
  - "features/search/components/SearchForm.tsx"
  - "features/search/components/SearchResultsClient.tsx"
---

# Hawkr Agent conversation

- Scope: Extend the existing buyer Messages workspace at `/account/messages/assistant/{conversationId}` and add a search-surface entry; do not create a separate AI product shell.
- Mode: Operate.
- Audience and job: An invited buyer turns an informal 30–100 person food purchase request into a quote-ready, versioned requirement without learning procurement terminology.
- Primary task: Answer only missing or conflicting fields one question at a time, review the live Request Snapshot, confirm the request, then inspect platform and external candidate lanes.
- Phase 1 action boundary: The buyer may complete dynamic clarification, rewind answers, confirm the fixture request, inspect public-safe progress, compare a reference price, and explicitly select read-only candidates. Sending RFQs, accepting quotes, payment, order creation, and fulfillment remain unavailable until authoritative backend contracts and feature flags exist.
- Content and proof: Use the v3.0 golden request as clearly labelled synthetic demonstration data. Keep field provenance, request version, data timestamp, candidate qualification and the reason a transaction action is unavailable visible.
- Chosen direction: Use the incumbent human Messages thread as the complete shell. Conversation remains the narrative spine; clarification, snapshot, progress and candidates appear as typed artifacts in that same timeline. Snapshot details open in a drawer, with no dedicated Agent workbench or persistent right rail.
- Memorable moment: A locally selected answer does not change the snapshot until `下一步`; after commit, the answer folds into history and the exact snapshot row changes source/status, making the version boundary visible.
- Constraints: Hawkr tokens and shared UI kit; Iconly only; leaf plus explicit AI label; no human presence/read receipts; no fixed question count, model reasoning, mixed supplier lane, combined send CTA, deposit/preauthorisation copy, auto-selection, or fake live claims.
- Responsive and accessibility: Desktop is Messages sidebar plus one conversation thread; mobile is the same single thread without the sidebar. Snapshot and candidate details use accessible drawers/sheets at every width. Preserve 44px targets, native checkbox/radio semantics, focus return, IME safety, reduced motion and WCAG AA.
- Unresolved: assistant-ui/ReUI adoption method, real Assistant/Procurement API availability, lane deadline policy, and external non-binding interest action.
