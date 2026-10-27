# MQ anomalies (catalog vs code vs topology)

Sources: code (non-test Go/Java), `infra/mq/topology/topology.yaml`, notification templates (all 107 notification-service migrations applied to a scratch Postgres: 94 rows, 92 enabled), catalog 36 §3–§5.
Rule used for notification (catalog §5.2's own rule): consumed only if exchange is in `notificationQueueByExchange` AND an enabled `(exchange, routing_key)` template exists.

## 1. Published, no effective consumer (13 keys)
- **`hawkr.governance` is not provisioned in topology** (not in `exchanges:`), not in the notification exchange map, and has no template. seller-service still publishes 5 keys there: `seller.governance.{violation_created,violation_revoked,appeal_decided,notify.assist_investigation,notify.temporary_takedown_self_check}` (violation.go, food_safety_report.go; fire-and-forget, errors only logged). Catalog §3.8/§4.2/§5.2 says notification-service consumes them. Unverified: runtime result (a publish to a missing exchange normally closes the channel with 404).
- `job.parsed`, `job.commit_finished` (product-import-service → `hawkr.product.import`): no queue binds them, so they are unroutable. Catalog §3.6b/§3.7 says notification-service consumes them. Only their DLQs exist (`dlq.product.import.job.parsed`, `dlq.product.import.job.commit_finished`), and nothing dead-letters into those.
- `payment.membership.{buyer,seller}.{success,failed}` (payment-service): topology has `buyer.membership.payment.result` / `seller.membership.payment.result`, but no code consumes them. These are known pending work (`queue:consumer_impl_pending:info` in infra-alerts.yml). None of the 4 keys is in the catalog.
- `order.delivered`: only reaches the notification `#` queue, which has no template, so it is dropped. Catalog §3.1 says "no effective consumer (#3486)" but §3.7 lists a delivery push. The catalog contradicts itself.
- `fulfillment.responsibility.unresolved` (payment-service → `hawkr.shipping`): the catalog names notification-service as consumer. There is no template, so it is dropped.

## 2. Consumer with no live producer (1)
- `coupon.issued`: the notification template is enabled (buyer:visible), but `CouponEventPublisher.publishCouponIssued` has no caller in main code, so the producer is dead code. The catalog lists it as live.
- No other topology binding matches a key nobody produces (literal bindings checked; `#` excluded).

## 3. Delivered to notification wildcard queues and silently dropped (no template): 39 keys
These are harmless today, but when moving to an in-process queue, avoid fanning them out. Examples: `payment.success/failed/expired/closed/conflict`, `refund.succeeded/failed`, `order.sales.accumulated`, `order.refund.finalized`, `order.shipment.*`, `shipment.label.updated/photo.updated/processing.*/return.status_changed`, `logistics.cost.incurred`, `fulfillment.responsibility.finalized`, `seller.deleted/onboarded/profile.changed/membership_changed`, `buyer.deleted`, `buyer.favorite.removed`, `review.hidden`, `comment.updated/deleted`, `counter.delta`, `product.restock`, `product.certification.verify.requested`, `aftersales.cancel.completed`.

## 4. Topology queues vs code
- 99 non-DLQ/non-retry queues. 97 have a code consumer. 2 have none (the membership queues above). No code consumer queue is missing from topology. The gateway's `gateway.sse.<pod>` is dynamic and not in topology, by design.
- Exchange `hawkr.payment` exists in topology, but nothing publishes to it (only its sibling `hawkr.payment.dlx` is used).
- DLX naming inconsistency: `order.payment.result` and `order.refund.result` (consumer order-service) dead-letter via `hawkr.payment.dlx`. Topology's own comment says "DLX follows the consumer service". Every other order queue uses `hawkr.order.dlx`.

## 5. DLQs: none has a consumer
- 96 DLQ queues: 43 `{q}.dlq` from `retry:` (Go ResilientConsumer), 40 own `{q}.dlq` from `dlq:` (Java), and 13 explicit (`aftersales.events.dlq`, `dlq.coupon.*` ×2, `dlq.product.import.*` ×6, `dlq.media.*` ×3, `payment.rescue.refund.dlq`).
- 0 of 96 are consumed in code, and no redrive tool exists in repo. Handling is the `RabbitMQDLQMessages` alert plus manual work.
- Orphan DLQs (no `dlq:` policy targets them): `dlq.product.import.job.parsed`, `dlq.product.import.job.commit_finished` (no primary queue at all), `dlq.media.import.asset.download.internal` (app publishes to it directly; comment says "manual handling").
- All own-DLQ routing keys match their `{q}.dlq` binding, and every explicit `dlq.queue` target exists and is bound. No broken DLX wiring was found.

## 6. Catalog (36) vs code
- 22 code keys are missing from the §3 full event list:
  - in no section: `payment.membership.*` ×4, `payuni.payment.recurring`, `seller.onboarded` (consumed by gateway resolver cache), `invoice.complaint.responded`, `invoice.complaint.overdue`, `import.asset.download.internal`, `seller.governance.notify.*` ×2
  - only in §1/§3.8/§4.2/§5.2: `chat.message.sent`, `follow.added`, `product.viewed`, `merchant.verification.*` ×5, `seller.governance.{violation_created,violation_revoked,appeal_decided}`
- 19 catalog 🆕 rows have nothing in code (doc-only in events.csv): `order.fulfillment.failed`, `order.receipt.disputed`, `fulfillment.seller.ready`, `fulfillment.provider.*` ×4, `shipment.batch.*` ×6, and recurring-service consumers of `order.{completed,cancelled,refund.finalized}`. recurring-service and its `hawkr.recurring` exchange (15 planned keys) do not exist.
- 🆕-marked but implemented: `logistics.cost.incurred` and `fulfillment.responsibility.finalized`.
- Stale queue names in the catalog that are not in topology:
  - `notification.order.events` (§3.1)
  - `notification.order.paid`, `notification.order.placed`, `notification.governance.seller` (§5.2 sample table)
  - `notification.payment.rescue.refund.completed` (§3.2)
  - The real names are the per-exchange `notification.<domain>` queues.
- `review.created` / `review.hidden` sit under the §3.1 table headed `hawkr.order`. The real exchange is `hawkr.review`, which §3.8 states correctly.
- Stale DLX names in §5.2: `hawkr.content.dlx`, and `hawkr.product.dlx` for `buyer.product.restock`. Topology uses the `retry:`-expanded `{q}.dlx` fanout, and `hawkr.content.dlx` does not exist.

## 7. Not checked / limits
- Java `@Bean` binding declarations (dev-only, `SPRING_RABBITMQ_DYNAMIC`) were not diffed against topology. Go `Subscription.RoutingKeys` were compared only by reading the code.
- Notification templates reflect the seed migrations. Admin runtime edits to `notification_templates` in the prod DB are not visible.
- The payuni-webhook-service producer keys were confirmed via its routing switch plus outbox insert sites. Which PayUni webhook types actually arrive in production is not verifiable from code.
