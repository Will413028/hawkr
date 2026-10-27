---
version: 1
slug: "src-app-admin-merchants-id-operate-page-tsx"
primary_target: "src/app/admin/merchants/[id]/operate/page.tsx"
related_targets: ["src/app/admin/merchants/[id]/page.tsx","src/features/delegated-operations/components/OperationWorkspace.tsx","src/styles/pages/DelegatedOperations.css"]
---

# ADM delegated operations surface

## Job to be done

An authorized administrator needs to help one seller maintain permitted content while remaining visibly and technically an ADM actor. The interface must make target, scope, reason, remaining time, and exit obvious before any write is possible.

## Entry and hierarchy

- Add `開始代行營運` to an eligible seller detail header only after a server preflight confirms that delegated operation is available.
- While eligibility is loading, reserve the action position with a restrained skeleton. If unavailable, omit the action and expose the verified reason near seller governance status rather than presenting a dead button.
- Starting a privileged session uses a focused preflight dialog because the administrator must review the target, prohibited actions, reason, and case reference before creating server state.
- The dialog requires a reason selected from a server-approved list and a case or ticket reference. Its primary action is `確認並開始代行`; cancellation is always available until the request starts.

## Delegated workspace composition

- The Admin sidebar and topbar remain in place to preserve actor identity and escape routes.
- A sticky amber context landmark sits directly below the topbar and above workspace content. It shows `正在代行`, seller name and Store ID, operator label, reason or case, remaining time, and `結束代行`.
- The amber bar uses Hawkr amber as information-bearing context, ink text, one quiet border, and no decorative gradient or glass treatment.
- Below it, a compact workspace header names the seller and explains that actions are recorded under the ADM identity.
- A horizontal section navigation exposes only server-granted capabilities. Initial frontend slice provides an overview of permitted areas and explicit unavailable states for product, post, and storefront adapters that await backend endpoints.
- Orders and fulfillment appear only as a locked boundary explanation; no operational controls are rendered.

## Session states

- Loading: visible shell skeleton with the safety landmark position reserved.
- Active: remaining time updates once per second and changes wording near expiry.
- Near expiry: status changes to a warning announcement without interrupting focus.
- Expired, revoked, target mismatch, or permission loss: unmount the workspace and show a blocking state that names the cause, confirms that no further writes are possible, and offers `返回賣家資料`.
- End session: require a short confirmation dialog; send the server end request first. If termination is not explicitly proven, keep the session context visible, latch writes closed, and allow only retry or return. On proven success, route back to the same seller record.
- Backend unsupported or contract error: show `代行營運尚未啟用` with trace ID when available. Never enter preview or active state from a failed response.

## Visual and interaction contract

- 通用色彩、字型、radius、元件與響應式規則一律遵循根目錄 `DESIGN.md`；此 surface 只增加 ADM 金色安全 context 與下列代行特殊行為。
- 不新增字型或動畫依賴；表格、識別碼與控制項沿用 `DESIGN.md` 的 Roboto UI stack，品牌頁標才使用 openhuninn。
- Cards have 12 to 16px radii, either a border or a soft offset shadow, never both.
- Motion is limited to 150 to 250ms state transitions and respects reduced motion.
- At 1024px, safety context reflows before it truncates. At narrow widths actions become full-row; seller and session identity stay visible.
- All controls are at least 44px high, keyboard focus is visible, dialogs trap focus and restore it, status changes use `aria-live`, and the context bar is a named landmark.

## Copy vocabulary

- Use `代行營運`, `代行工作階段`, `目標店家`, `代行原因`, `案件編號`, `剩餘時間`, and `結束代行` consistently.
- Never use `切換身分`, `登入賣家`, or copy that suggests impersonation.
- Errors state both the problem and recovery. Example: `工作階段已失效，已停止所有代行操作。請返回賣家資料後重新確認資格。`

## Implementation boundary

- Frontend reads eligibility and session state through same-origin dedicated BFF handlers.
- The opaque operation credential stays in an HttpOnly cookie and never enters client storage or page props.
- Client models receive only a safe session summary and capability list.
- No fake production success, seller token bridge, or seller `/app` API fallback is permitted.
