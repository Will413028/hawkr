# Baseline fact-check for step10e-3-brief-v1.md (D1–D5)

Checked 2026-10-16. EUR-Lex itself returned an empty or JS-challenge page to both curl and WebFetch, so the DSA text comes from the Springlex reproduction of OJ L 277, 27.10.2022.

## 1. Zendesk — PARTLY
- Linkable standalone ticket pages and internal note vs public reply: well known, but I did not fetch a primary page for either. Treat both as unverified-by-fetch, not as contradicted.
- SLA policies are configured server-side: VERIFIED. https://support.zendesk.com/hc/en-us/articles/4408829459866 says: "Each SLA policy consists of conditions that a ticket must meet and targets that measure metrics you define"; "Your target status will display the metric closest to becoming breached".
- Views sort by SLA: VERIFIED, with a wording difference. https://support.zendesk.com/hc/en-us/articles/5604663490458 says you add an SLA column, then "Order by > SLA ... Ascending" so "tickets whose targets are most breached or are closest to breaching will get attention first". The column shows "the calendar time left before the next target on any given ticket will be breached". The docs have no field called "next SLA breach time". The ticket API exposes `policy_metrics[].breach_at`.
- API shape (https://developer.zendesk.com/api-reference/ticketing/tickets/tickets/ example response): `{"policy_metrics":[{"breach_at":"2023-01-19T02:14:27+00:00","metric":"first_reply_time","minutes":-19,"stage":"active"}]}`. This means Zendesk outputs the due timestamp plus a signed `minutes` value derived at read time. Negative means breached. There is no stored overdue boolean.

## 2. Jira Service Management — PARTLY
- Server-computed SLAs: VERIFIED. The JSM REST spec (https://dac-static.atlassian.com/cloud/jira/service-desk/swagger.v3.json) has SlaInformationOngoingCycleDTO with these fields: `breachTime` ("Time and date at which the SLA cycle would have breached its limit"), `breached` ("Indicates whether the SLA has been breached (true) or not (false)"), `remainingTime`, `goalDuration`, `paused` and `withinCalendarHours`.
- Queue sorting by SLA: VERIFIED. https://support.atlassian.com/jira-service-management-cloud/docs/what-are-queues/ says: "Queues are often sorted by a service level agreement (SLA) or goal ..."; "a clock on the work item indicates the time until your team's next target is due." JQL supports SLA functions such as `remaining()`: https://support.atlassian.com/jira-service-management-cloud/docs/write-jql-queries-for-slas/.
- "Each queue is a JQL": PARTLY. The current Cloud article says requests are "filtered into queues based on specific criteria" and does not mention JQL. Queues are JQL-backed in practice, but the page I fetched does not say so.
- Internal comment vs reply to customer: VERIFIED. https://support.atlassian.com/jira-service-management-cloud/docs/talk-to-the-customer-or-team-members-from-the-new-issue-view has "Reply to customer" and "Add internal note (the customer cannot see these)".
- "Queues are views over one resource type, not mixed": PARTLY. A queue contains work items of one service space, which is true, but the docs do not state it as a principle. It is an inference.

## 3. Google Merchant API — VERIFIED (exact names)
Source: issueresolution v1beta `renderaccountissues`, Action object (https://googleapis.github.io/google-api-python-client/docs/dyn/merchantapi_issueresolution_v1beta.issueresolution.html; REST reference https://developers.google.com/merchant/api/reference/rest/issueresolution_v1/issueresolution/renderaccountissues).
- `isAvailable`: "Controlling whether the button is active or disabled ... If the action is not available then a reason will be present. If (your) third-party application shows a disabled button for action that is not available, then it should also show reasons."
- `reasons[]`: "List of reasons why the action is not available ... empty if the action is available. If there is only one reason, it can be displayed next to the disabled button. If there are more reasons, all of them should be displayed, for example in a pop-up dialog."
- Reason fields: `message` ("summarizing the reason, why the action is not available"), `detail` ("Should be displayed as a hint if present") and `action`, an optional action that fixes the cause.
- The brief writes `is_available` + `reasons`. In proto/RPC naming that is correct; the REST JSON name is `isAvailable`. Merchant also adds a `detail` and a remediation `action` per reason, which is a possible refinement for D5.

## 4. GitHub merge button — PARTLY
- Merging is blocked until required checks and reviews pass: VERIFIED. https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches says "all required status checks must pass before collaborators can merge"; an approval dismissal means "the pull request cannot be merged until someone approves the work again".
- "Button greyed out with stated reason": the docs I fetched do not describe the button's look. The API has a coarse server-computed `MergeStateStatus` enum (public schema https://docs.github.com/public/fpt/schema.docs.graphql: BEHIND, "BLOCKED: The merge is blocked.", CLEAN, DIRTY, UNSTABLE ...). The specific causes come from separate fields (`reviewDecision`, status-check rollup), not from a reasons list. Merchant is the cleaner precedent; GitHub supports "the server decides blocked-ness", but not "the server returns reason text".

## 5. Amazon Account Health — PARTLY
https://sellercentral.amazon.com/gp/help/external/200370560 says: "Select the issue within the Product Policy Compliance column. Follow the instructions within the page to submit your appeal". This shows a per-issue page with its own appeal flow. Full help page is login-gated; I only saw the search excerpt. "One page per issue" is supported for listing/policy issues; account deactivation uses a single "Reactivate your account" flow on the Account Health page. In short, it is mostly but not uniformly per-issue.

## 6. EU DSA (Regulation (EU) 2022/2065) — VERIFIED with one attribution fix
Text from https://www.springlex.eu/en/packages/dsa/dsa-regulation/article-16/ and .../article-20/ (reproduction of OJ text; EUR-Lex https://eur-lex.europa.eu/legal-content/EN/TXT/?uri=CELEX:32022R2065 was not fetchable):
- Art. 16(5): "The provider shall also, without undue delay, notify that individual or entity of its decision in respect of the information to which the notice relates, providing information on the possibilities for redress in respect of that decision."
- Art. 16(6): "Providers of hosting services shall process any notices that they receive under the mechanisms referred to in paragraph 1 and take their decisions in respect of the information to which the notices relate, in a timely, diligent, non-arbitrary and objective manner. Where they use automated means ..."
- Art. 20(4): "Providers of online platforms shall handle complaints submitted through their internal complaint-handling system in a timely, non-discriminatory, diligent and non-arbitrary manner. Where a complaint contains sufficient grounds ... it shall reverse its decision ... without undue delay."
- Fix: Art. 20(4) says "non-discriminatory", not "objective"; "objective" is in 16(6). The brief's D2 cites only 20(4) "timely", and its D4 citation of 16(5)–(6) for diligent/objective processing and notification is accurate. Neither article mandates internal notes; "internal records are the common way to evidence this" is the brief's inference, labelled as such.

## Stored/output overdue boolean vs due timestamp (D2 option c)
- JSM outputs both `breachTime` and a `breached` boolean on the per-request SLA endpoint. The boolean is computed when the API is called, and SLA clocks can be paused or limited to calendar hours, which a client cannot reproduce from a timestamp alone.
- Zendesk outputs `breach_at` plus a read-time signed `minutes`; there is no boolean.
- Neither stores "overdue" as a persisted column; both always give the due timestamp.
- So the brief's (c) line "業界不這樣做" is too strong. The accurate version: industry always outputs the due time, and some systems add a derived boolean or remaining time at read time. That is mostly because their clocks pause or follow business calendars. The brief's due time is a fixed 7×24h with no pause, so a client compare is exact and (b) still holds. The staleness argument stands.
