package io.github.will413028.hawkr.common.mq;

/**
 * RabbitMQ constants for exchanges, queues, and routing keys
 */
public final class MqConstants {

    private MqConstants() {}

    // ==================== Exchanges ====================

    public static final String EXCHANGE_ORDER = "hawkr.order";
    public static final String EXCHANGE_PAYMENT = "payment.events";
    public static final String EXCHANGE_MARKETING = "hawkr.marketing";
    public static final String EXCHANGE_NOTIFICATION = "hawkr.notification";
    public static final String EXCHANGE_DELAYED = "hawkr.delayed";
    public static final String EXCHANGE_SELLER_BIZ = "hawkr.seller";
    public static final String EXCHANGE_USER = "hawkr.user";

    /** 買家領域（buyer-service Go 服務 publish；02 §5.14 / 36 §3.6a） */
    public static final String EXCHANGE_BUYER = "hawkr.buyer";

    /** 帳號領域（account-service Go 服務 publish；39 §5.8.1 / 07 §5.19 / 36 §3.6c） */
    public static final String EXCHANGE_ACCOUNT = "hawkr.account";
    // 賣家領域 exchange "hawkr.seller" 用既有的 EXCHANGE_SELLER_BIZ；seller.deleted 走該 exchange

    /** Coupon Service 私有 DLX（07 §5.19，2026-06-04） */
    public static final String EXCHANGE_COUPON_DLX = "hawkr.coupon.dlx";

    /** 商品領域（product-service Java 服務 publish 補貨事件；02 §5.13b / 36 §3.6h，2026-06 #942 新建） */
    public static final String EXCHANGE_PRODUCT = "hawkr.product";

    // Product Service DLX
    public static final String EXCHANGE_PRODUCT_DLX = "hawkr.product.dlx";

    // Dead letter exchanges
    public static final String EXCHANGE_ORDER_DLX = "hawkr.order.dlx";
    public static final String EXCHANGE_PAYMENT_DLX = "hawkr.payment.dlx";
    /**
     * payment-service 的<b>第二個</b> DLX。
     *
     * <p>⚠️ <b>它與 {@link #EXCHANGE_PAYMENT_DLX} 並存不是重複</b>：兩者都已在
     * {@code infra/mq/topology/topology.yaml} 上佈建且各有既有 queue 掛著
     * （{@code payment.events.dlx} ← buyer／seller 的 membership payment result；
     * {@code hawkr.payment.dlx} ← payment.rescue.refund）。
     * 合併它們是一次破壞性 topology 變更（刪 queue ＋ 改 binding），不在任何一波的射程內。
     *
     * <p>🔴 新增 queue 時<b>照設計書指定的那一個寫</b>，不要憑「payment 的 DLX 應該是哪個」猜
     * ——猜錯的症狀是死信落到另一個 exchange，而該 queue 的死信查詢會回**空的**
     * （看起來像「沒有死信」，其實是找錯地方）。
     * N16 的 payment 側 queue 依 36 §3.3.1 用本常數。
     */
    public static final String EXCHANGE_PAYMENT_EVENTS_DLX = "payment.events.dlx";
    public static final String EXCHANGE_MARKETING_DLX = "hawkr.marketing.dlx";
    public static final String EXCHANGE_NOTIFICATION_DLX = "hawkr.notification.dlx";
    public static final String EXCHANGE_SELLER_BIZ_DLX = "hawkr.seller.dlx";
    public static final String EXCHANGE_USER_DLX = "hawkr.user.dlx";
    /** Cart Service 私有 DLX（#1426：cart 首個 consumer seller.profile.changed 的死信出口） */
    public static final String EXCHANGE_CART_DLX = "hawkr.cart.dlx";

    // ==================== Routing Keys ====================

    // Order routing keys
    public static final String ROUTING_KEY_ORDER_PAID = "order.paid";
    public static final String ROUTING_KEY_ORDER_COMPLETED = "order.completed";
    public static final String ROUTING_KEY_PAYOUT_COMPLETED = "payout.completed";
    public static final String ROUTING_KEY_ORDER_CANCELLED = "order.cancelled";
    public static final String ROUTING_KEY_ORDER_SALES_ACCUMULATED = "order.sales.accumulated";
    /**
     * 訂單層唯一權威退款事件（58 §6.3.2.3）。exchange 沿用 {@link #EXCHANGE_ORDER}——
     * 這個事件屬「訂單領域」而非發票領域，另立一個 exchange 常數只會製造第二個真相。
     *
     * <p>producer 是 order-service 的三個發布點（全部在既有交易內落 {@code order_outbox}）：
     * {@code AfterSalesRefundEventHandler.handleRefundCompleted}（僅 {@code duplicate == false} 分支）／
     * {@code AutoRefundTxHandler.completeAutoRefund}／{@code SellerCancelRefundTxHandler.handleCancelCompleted}。
     * consumer 是 invoice-service 的 {@code invoice.order.refund.finalized}，據此判定作廢／折讓。
     */
    public static final String ROUTING_KEY_ORDER_REFUND_FINALIZED = "order.refund.finalized";

    // Seller workspace realtime invalidation（order/aftersales → gateway SSE，僅 web 賣家中心，36 §3.6i / 41 §四之二，[B-207] #1181）
    public static final String EXCHANGE_SELLER_WORKSPACE = "hawkr.seller_workspace";
    public static final String ROUTING_KEY_SELLER_ORDER_UPDATED = "seller.order.updated";
    public static final String ROUTING_KEY_SELLER_AFTER_SALE_UPDATED = "seller.after_sale.updated";

    /** 買家收藏新增（buyer-service publish；02 §5.14 FavoriteCountConsumer consume） */
    public static final String ROUTING_KEY_BUYER_FAVORITE_ADDED = "buyer.favorite.added";
    /** 買家取消收藏（同上） */
    public static final String ROUTING_KEY_BUYER_FAVORITE_REMOVED = "buyer.favorite.removed";

    /** 商品補貨（product-service publish；buyer-service restock_consumer fan-out；02 §5.13b / 39 §5.5a / 36 §3.6h，2026-06 #942） */
    public static final String ROUTING_KEY_PRODUCT_RESTOCK = "product.restock";
    /** 補貨通知 per-subscriber target（buyer-service publish；notification-service template consume；39 §5.5a / 12 §6.1） */
    public static final String ROUTING_KEY_PRODUCT_RESTOCK_TARGET = "product.restock.target";

    /**
     * 商品認證背景查驗請求（19 §12 / 36 §3.6h.1，#2755）。
     *
     * <p>🔴 product-service <b>自己 publish、自己 consume</b>——本 exchange 上第一個這種形狀。
     * queue {@link #QUEUE_PRODUCT_CERTIFICATION_VERIFY} 必須在同一個 PR 內登記進
     * {@code infra/mq/topology/topology.yaml}：服務端已 passive declare，漏登記＝訂閱靜默失效
     * （不 crash、不進 DLQ、無告警），而登記一致性 gate 只掃 Go，Java 側沒有 CI 擋得住。
     */
    public static final String ROUTING_KEY_PRODUCT_CERTIFICATION_VERIFY_REQUESTED =
            "product.certification.verify.requested";
    /** 商品認證已核准，通知賣家（19 §12.3 / 36 §3.6h.1；⚠️ 同步快取命中的即刻核准不發） */
    public static final String ROUTING_KEY_PRODUCT_CERTIFICATION_APPROVED = "product.certification.approved";
    /** 商品認證已駁回，通知賣家（payload 帶 reasonCode／publicReason，🔴 不含 internalNote） */
    public static final String ROUTING_KEY_PRODUCT_CERTIFICATION_REJECTED = "product.certification.rejected";
    /** 商品認證已撤銷，通知賣家（標章已從公開投影移除，必須告知） */
    public static final String ROUTING_KEY_PRODUCT_CERTIFICATION_REVOKED = "product.certification.revoked";
    /** 帳號刪除事件（account-service publish；39 §5.8 / 07 §5.19） */
    public static final String ROUTING_KEY_ACCOUNT_DELETED = "account.deleted";

    /** 賣家刪除事件（seller-service publish；product-service consume 下架該賣家商品） */
    public static final String ROUTING_KEY_SELLER_DELETED = "seller.deleted";

    /** 買家刪除事件（buyer-service publish；coupon-service consume 作廢該買家 stint 全部未使用優惠券；
     *  無條件於買家角色 DELETE transition 發布，即使帳號因仍有其他角色存活未刪；
     *  取代 coupon-service 端原掛的 account.deleted 綁定，見 W2/W3 設計書 §3.2 D6） */
    public static final String ROUTING_KEY_BUYER_DELETED = "buyer.deleted";

    /** 賣家資料變更事件（seller-service publish；cart-service consume evict seller:meta；#1426 方案 A / 36 §3.6f） */
    public static final String ROUTING_KEY_SELLER_PROFILE_CHANGED = "seller.profile.changed";
    /** cart-service 消費 seller.profile.changed 的 queue（#1426，cart 首個 consumer） */
    public static final String QUEUE_CART_SELLER_PROFILE_CHANGED = "cart.seller.profile.changed";
    /** 上述 queue 的 DLQ，走 cart 自有 DLX EXCHANGE_CART_DLX（#1426） */
    public static final String QUEUE_CART_SELLER_PROFILE_CHANGED_DLQ = "cart.seller.profile.changed.dlq";

    /** 賣家停權事件（seller-service publish；order/coupon consume；44 §5.10a/03 §5.1a.2/07 §5.20，36 §3.6f） */
    public static final String ROUTING_KEY_SELLER_SUSPENDED = "seller.suspended";
    /** 賣家解除停權事件（同上，僅 coupon consume） */
    public static final String ROUTING_KEY_SELLER_UNSUSPENDED = "seller.unsuspended";

    /** 店家可曝光狀態事件（單體 seller 發出，legacy relay 轉送；product-service consume，讀取時隱藏不可曝光的店的商品） */
    public static final String ROUTING_KEY_SELLER_EXPOSURE_CHANGED = "seller.exposure.changed";
    /** product-service 消費 seller.exposure.changed */
    public static final String QUEUE_PRODUCT_SELLER_EXPOSURE_CHANGED = "product.seller.exposure.changed";
    public static final String QUEUE_PRODUCT_SELLER_EXPOSURE_CHANGED_DLQ = "product.seller.exposure.changed.dlq";

    /** order-service 消費 seller.suspended（03 §5.1a.2，批次取消未付款單；不消費 unsuspended） */
    public static final String QUEUE_ORDER_SELLER_SUSPENDED = "order.seller.suspended";
    public static final String QUEUE_ORDER_SELLER_SUSPENDED_DLQ = "order.seller.suspended.dlq";

    /** coupon-service 消費 seller.suspended/unsuspended（07 §5.20，模板停用／還原）。
     *  DLQ 命名沿用 36 §3.6f 文字為準的前綴式（與既有 coupon.account.deleted.dlq 後綴式不同，非本次疏漏）。 */
    public static final String QUEUE_COUPON_SELLER_SUSPENDED = "coupon.seller.suspended";
    public static final String QUEUE_COUPON_SELLER_SUSPENDED_DLQ = "dlq.coupon.seller.suspended";
    public static final String QUEUE_COUPON_SELLER_UNSUSPENDED = "coupon.seller.unsuspended";
    public static final String QUEUE_COUPON_SELLER_UNSUSPENDED_DLQ = "dlq.coupon.seller.unsuspended";

    /** Coupon Service 消費 buyer.deleted（07 §5.19 → W2/W3 設計書 §3.2 D6 遷移，2026-07-16；
     *  取代原 coupon.account.deleted 綁定，見 BuyerDeletedConsumer） */
    public static final String QUEUE_COUPON_BUYER_DELETED = "coupon.buyer.deleted";
    public static final String QUEUE_COUPON_BUYER_DELETED_DLQ = "coupon.buyer.deleted.dlq";
    // §8.11 付款後建單兜底（Backstop）：重試建單 / 建單逾時取消
    public static final String ROUTING_KEY_ORDER_SHIPMENT_CREATE_REQUESTED = "order.shipment.create_requested";
    public static final String ROUTING_KEY_ORDER_SHIPMENT_CREATION_FAILED = "order.shipment.creation_failed";

    // Payment routing keys
    public static final String ROUTING_KEY_PAYMENT_SUCCESS = "payment.success";
    public static final String ROUTING_KEY_PAYMENT_FAILED = "payment.failed";
    public static final String ROUTING_KEY_PAYMENT_EXPIRED = "payment.expired";
    public static final String ROUTING_KEY_PAYMENT_CLOSED = "payment.closed";
    public static final String ROUTING_KEY_PAYMENT_CONFLICT = "payment.conflict";
    public static final String ROUTING_KEY_PAYMENT_MEMBERSHIP_BUYER_SUCCESS = "payment.membership.buyer.success";
    public static final String ROUTING_KEY_PAYMENT_MEMBERSHIP_BUYER_FAILED = "payment.membership.buyer.failed";
    public static final String ROUTING_KEY_PAYMENT_MEMBERSHIP_SELLER_SUCCESS = "payment.membership.seller.success";
    public static final String ROUTING_KEY_PAYMENT_MEMBERSHIP_SELLER_FAILED = "payment.membership.seller.failed";
    public static final String ROUTING_KEY_PAYMENT_RESCUE_REFUND_INITIATED = "payment.rescue.refund_initiated";
    public static final String ROUTING_KEY_PAYMENT_RESCUE_REFUND_COMPLETED = "payment.rescue.refund_completed";

    /** §2A.6 RescueRefundConsumer 監聽 queue */
    public static final String QUEUE_PAYMENT_RESCUE_REFUND = "payment.rescue.refund.queue";
    /** §2A.6 RescueRefundConsumer DLQ */
    public static final String QUEUE_PAYMENT_RESCUE_REFUND_DLQ = "payment.rescue.refund.dlq";

    // Seller routing keys (PAYUNi integration)
    public static final String ROUTING_KEY_SELLER_PAYUNI_REGISTER = "seller.payuni.register";
    public static final String ROUTING_KEY_SELLER_PAYUNI_RESULT = "seller.payuni.result";
    public static final String ROUTING_KEY_SELLER_PAYUNI_STATUS_CHECK = "seller.payuni.status.check";
    public static final String ROUTING_KEY_SELLER_PAYUNI_STATUS_UPDATED = "seller.payuni.status.updated";

    // Seller routing keys（賣家審核結果，由 Payment Webhook 觸發）
    public static final String EXCHANGE_SELLER = "seller.events";
    public static final String ROUTING_KEY_SELLER_REVIEW_RESULT = "seller.review.result";

    // User routing keys
    public static final String ROUTING_KEY_MEMBER_REGISTERED = "member.registered";
    public static final String ROUTING_KEY_MEMBER_PROFILE_UPDATED = "member.profile.updated";
    public static final String ROUTING_KEY_MEMBER_PASSWORD_CHANGED = "member.password.changed";
    public static final String ROUTING_KEY_MEMBER_ADDRESS_ADDED = "member.address.added";
    public static final String ROUTING_KEY_MEMBER_ADDRESS_UPDATED = "member.address.updated";
    public static final String ROUTING_KEY_MEMBER_AVATAR_CHANGED = "member.avatar.changed";
    public static final String ROUTING_KEY_SELLER_APPROVED = "seller.approved";

    // Catalog routing keys (Product/Review)
    public static final String EXCHANGE_CATALOG = "hawkr.catalog";
    public static final String EXCHANGE_CATALOG_DLX = "hawkr.catalog.dlx";
    public static final String ROUTING_KEY_PRODUCT_CREATED = "product.created";
    public static final String ROUTING_KEY_PRODUCT_UPDATED = "product.updated";
    public static final String ROUTING_KEY_PRODUCT_STATUS_CHANGED = "product.status.changed";
    public static final String ROUTING_KEY_REVIEW_POSTED = "review.posted";

    // Refund routing keys
    public static final String ROUTING_KEY_REFUND_SUCCEEDED = "refund.succeeded";
    public static final String ROUTING_KEY_REFUND_FAILED = "refund.failed";
    public static final String ROUTING_KEY_REFUND_COMPLETED = "refund.completed";

    // Fulfillment routing keys (Shipping)
    public static final String EXCHANGE_SHIPPING = "hawkr.shipping";
    public static final String EXCHANGE_SHIPPING_DLX = "hawkr.shipping.dlx";
    public static final String ROUTING_KEY_SHIPMENT_CREATED = "shipment.created";
    public static final String ROUTING_KEY_SHIPMENT_LABEL_UPDATED = "shipment.label.updated";
    public static final String ROUTING_KEY_SHIPMENT_DELIVERED = "shipment.delivered";
    public static final String ROUTING_KEY_SHIPMENT_PHOTO_UPDATED = "shipment.photo.updated";

    // §5.21 配送異常賣家通知（shipping-service / aftersales-service 共用 publish 至 hawkr.shipping）
    public static final String ROUTING_KEY_SHIPMENT_ANOMALY_DETECTED = "shipment.anomaly.detected";
    public static final String EVENT_TYPE_SHIPMENT_ANOMALY_DETECTED = "SHIPMENT_ANOMALY_DETECTED";

    // ── 履約責任判定（N16／N17，65 §5.5.2／36 §3.3）─────────────────────────
    // 🔴 producer 是 **payment-service**，不是 shipping-service：fulfillment_exception_cases
    //    與 fulfillment_case_resolutions 由它擁有（65 §5.0 T12），而事件必須與那兩筆寫入
    //    同交易（INV-E1）⇒ shipping-service 無法在同一個交易內寫那兩張表。
    //    exchange 仍是 hawkr.shipping——值域錨在 routing key ＋ exchange，不在 publisher
    //    服務（ShippingEventType 值域邊界第 4 條）。
    /** 責任判定未決：證據不足，尚未決定退不退款。🔴 不帶任何 financially-final 語意。 */
    public static final String ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_UNRESOLVED =
            "fulfillment.responsibility.unresolved";
    public static final String EVENT_TYPE_FULFILLMENT_RESPONSIBILITY_UNRESOLVED =
            "FULFILLMENT_RESPONSIBILITY_UNRESOLVED";
    /**
     * 責任判定 financially final。
     *
     * 🔴 是 {@code finalized} 不是 {@code resolved}：後者只表示「有人做了判定」，而 consumer 要的是
     * 「不再有任何會改變退款權利的待覆核提案」。留 {@code resolved} 這個名字，consumer 會在判定
     * 還可能翻案時就把 Order 35 推進 54 並建撥款任務，而 54 是 immutable financial terminal。
     */
    public static final String ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_FINALIZED =
            "fulfillment.responsibility.finalized";
    public static final String EVENT_TYPE_FULFILLMENT_RESPONSIBILITY_FINALIZED =
            "FULFILLMENT_RESPONSIBILITY_FINALIZED";

    /**
     * provider 這條 fulfillment leg <b>已產生成本</b>（70 §7.1）。
     *
     * <p>🔴 <b>語意不是「送達完成」</b>——送達／取件後取消／買家無法投遞／返程 leg
     * 四種情形都算（70 §3.1.1）。
     *
     * <p>🔴 <b>Cost Fact ≠ Responsibility Decision</b>：本事件<b>不攜帶</b>責任成因，
     * 唯一的 responsibility authority 是 {@link #ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_FINALIZED}。
     * 兩個事實是獨立的 business fact，任一方都不得成為另一方的發送前置條件；
     * 它們如何 durable join（且抵達順序不影響最終結果）見 70 §3.1.4。
     *
     * <p>⚠️ producer 是 <b>shipping-service</b>（domain ownership：「這趟配送 provider
     * 成本多少」是 shipping domain fact），但 exchange 仍是
     * {@link #EXCHANGE_SHIPPING}——值域錨在 routing key ＋ exchange，不在 publisher 服務。
     */
    public static final String ROUTING_KEY_LOGISTICS_COST_INCURRED =
            "logistics.cost.incurred";
    public static final String EVENT_TYPE_LOGISTICS_COST_INCURRED =
            "LOGISTICS_COST_INCURRED";

    // ==================== Shipping Service 常數 ====================

    // Shipping Service 消費（來自 Webhook Service）
    public static final String QUEUE_SHIPPING_PAYUNI_WEBHOOK = "shipping.payuni.webhook";
    public static final String QUEUE_SHIPPING_PAYUNI_WEBHOOK_DLQ = "shipping.payuni.webhook.dlq";
    public static final String EXCHANGE_SHIPPING_WEBHOOK_DLX = "shipping.webhook.dlx";
    /** Uber Direct 貨態回調 queue（66 §6.4.1；綁 {@link #ROUTING_KEY_UBER_SHIPPING_STATUS}）。 */
    public static final String QUEUE_SHIPPING_UBER_WEBHOOK = "shipping.uber.webhook";
    public static final String QUEUE_SHIPPING_UBER_WEBHOOK_DLQ = "shipping.uber.webhook.dlq";

    // Shipping Service 發布路由鍵（STATUS_CHANGED，SHIPMENT_CREATED 已存在）
    public static final String ROUTING_KEY_SHIPMENT_STATUS_CHANGED = "shipment.status.changed";
    public static final String ROUTING_KEY_RETURN_SHIPMENT_STATUS_CHANGED = "shipment.return.status_changed";
    public static final String EVENT_TYPE_RETURN_SHIPMENT_STATUS_CHANGED = "RETURN_SHIPMENT_STATUS_CHANGED";

    // §5.9 賣家處理期限自動取消（09 §5.9 / 06 §5.20）
    public static final String ROUTING_KEY_SHIPMENT_PROCESSING_TIMEOUT = "shipment.processing.timeout";
    public static final String ROUTING_KEY_SHIPMENT_PROCESSING_CLEARED = "shipment.processing.cleared";
    public static final String QUEUE_AFTERSALES_SHIPMENT_PROCESSING_TIMEOUT = "aftersales.shipment.processing.timeout";
    public static final String QUEUE_ORDER_SHIPMENT_PROCESSING_CLEARED = "order.shipment.processing.cleared";
    public static final String EVENT_TYPE_SHIPMENT_PROCESSING_TIMEOUT = "SHIPMENT_PROCESSING_TIMEOUT";
    public static final String EVENT_TYPE_SHIPMENT_PROCESSING_CLEARED = "SHIPMENT_PROCESSING_CLEARED";

    // Shipping Service 事件類型（SHIPMENT_CREATED / SHIPMENT_DELIVERED 已存在）
    public static final String EVENT_TYPE_SHIPMENT_STATUS_CHANGED = "SHIPMENT_STATUS_CHANGED";

    // Order Service 消費 Shipping 事件 Queue
    public static final String QUEUE_ORDER_SHIPPING_CREATED = "order.shipping.created";
    public static final String QUEUE_ORDER_SHIPPING_LABEL = "order.shipping.label";
    public static final String QUEUE_ORDER_SHIPPING_STATUS = "order.shipping.status";
    public static final String QUEUE_ORDER_SHIPPING_DELIVERED = "order.shipping.delivered";
    public static final String QUEUE_ORDER_SHIPPING_PHOTO = "order.shipping.photo";

    // Order Service 發布：訂單已送達事件（通知前端/推播）
    public static final String ROUTING_KEY_ORDER_DELIVERED = "order.delivered";
    // Order 相關 event type 已遷移至 OrderEventType enum（內部技術碼，使用 routing key 點分式 value），不再宣告為 String 常數

    // Order Service 發布：確認收貨事件（通知 Shipping Service 標記 receipt_confirmed）
    public static final String ROUTING_KEY_ORDER_RECEIPT_CONFIRMED = "order.receipt.confirmed";

    // Shipping Service 消費確認收貨事件 Queue
    public static final String QUEUE_SHIPPING_ORDER_RECEIPT_CONFIRMED = "shipping.order.receipt.confirmed";

    // Shipping Service 消費付款成功事件 Queue（自動建單）
    public static final String QUEUE_SHIPPING_ORDER_PAID = "shipping.order.paid";
    public static final String QUEUE_SHIPPING_ORDER_PAID_DLQ = "shipping.order.paid.dlq";

    // §8.11 Backstop 重試建單（order → shipping）
    public static final String QUEUE_SHIPPING_SHIPMENT_CREATE_REQUESTED = "shipping.shipment.create_requested";
    public static final String QUEUE_SHIPPING_SHIPMENT_CREATE_REQUESTED_DLQ = "shipping.shipment.create_requested.dlq";
    // §8.11 Backstop 建單逾時取消（order → aftersales；DLQ 走 aftersales 共用 DLX）
    public static final String QUEUE_AFTERSALES_SHIPMENT_CREATION_FAILED = "aftersales.shipment.creation_failed";

    // Engagement routing keys (Content/Social)
    public static final String EXCHANGE_CONTENT = "hawkr.content";
    public static final String EXCHANGE_CONTENT_DLX = "hawkr.content.dlx";
    public static final String ROUTING_KEY_POST_CREATED = "post.created";
    public static final String ROUTING_KEY_POST_UPDATED = "post.updated";
    public static final String ROUTING_KEY_POST_DELETED = "post.deleted";
    public static final String ROUTING_KEY_COMMENT_ADDED = "comment.added";
    public static final String ROUTING_KEY_COMMENT_DELETED = "comment.deleted";
    public static final String ROUTING_KEY_LIKE_ADDED = "like.added";
    public static final String ROUTING_KEY_LIKE_REMOVED = "like.removed";
    public static final String ROUTING_KEY_FOLLOW_ADDED = "follow.added";
    public static final String ROUTING_KEY_FOLLOW_REMOVED = "follow.removed";

    // IM routing keys
    public static final String EXCHANGE_IM = "hawkr.im";
    public static final String EXCHANGE_IM_DLX = "hawkr.im.dlx";
    public static final String ROUTING_KEY_CHAT_MESSAGE_SENT = "chat.message.sent";

    // Media routing keys (OSS cleanup)
    public static final String EXCHANGE_MEDIA = "hawkr.media";
    public static final String EXCHANGE_MEDIA_DLX = "hawkr.media.dlx";

    // PayUni Webhook Events（由 payuni-webhook-service 發布）
    public static final String EXCHANGE_PAYUNI_WEBHOOK = "payuni.events";
    public static final String ROUTING_KEY_PAYUNI_PAYMENT_UOP = "payuni.payment.uop";
    public static final String ROUTING_KEY_PAYUNI_PAYMENT_RECURRING = "payuni.payment.recurring";
    public static final String ROUTING_KEY_PAYUNI_SHIPPING_STATUS = "payuni.shipping.status";
    // AGENT_NOTIFY 的 payuni.events 路由常數已於 #1757 移除（無 consumer 的過渡期冗餘路由）；
    // AGENT_NOTIFY 改單通道走 seller.events/seller.review.result。
    public static final String ROUTING_KEY_PAYUNI_REFUND_NOTIFY = "payuni.refund.notify";
    public static final String ROUTING_KEY_PAYUNI_REFUND_ACCEPTED = "payuni.refund.accepted";
    /**
     * §2.2 業務 pivot (2026-05-14)：UNKNOWN webhook 保守失敗處理。
     * 當 PayUni 推送的 webhook 無法被 parser 識別（含 MerID 但缺類型 key），投遞此路由給
     * aftersales / payment 兩個 fallback consumer 同時訂閱，依當下 awaiting/pending task 上下文做保守失敗處理。
     */
    public static final String ROUTING_KEY_PAYUNI_UNKNOWN_FALLBACK = "payuni.unknown.fallback";

    /**
     * Uber Direct 貨態回調（66 §6.4.1，本波唯一新增的 routing key）。
     *
     * <p>🔴 <b>exchange 沿用 {@link #EXCHANGE_PAYUNI_WEBHOOK}</b>（{@code payuni.events}）：
     * 66 §6.4.1 逐字「沿用既有 PayUni 的形狀（那條路徑已經證明可行）」，36 §3.5 註記
     * 「exchange 名 66 §6.4.1 未逐字指定（沿用 payuni.events 家族形狀）⇒ 待坐實」。
     * 本常數即為那個坐實。取捨：
     * <ul>
     *   <li>✅ 不新增 exchange ⇒ topology pin 的 exchanges 不動、broker 少一個要先建的前置</li>
     *   <li>✅ 該 exchange 上<b>沒有任何 {@code #} 綁定</b>（實查 topology.yaml，七條 binding 全是明確
     *       routing key）⇒ Uber 事件不會外溢到 PayUni 的 consumer</li>
     *   <li>⚠️ 代價：exchange 名字叫 {@code payuni.events} 而承載的不只是 PayUni。
     *       這是<b>已知的命名債</b>，登記在 36 §3.5；要還的話是整個 domain 改名，不是本波的事</li>
     * </ul>
     */
    public static final String ROUTING_KEY_UBER_SHIPPING_STATUS = "uber.shipping.status";

    // PayUni Webhook Event Types
    public static final String EVENT_TYPE_PAYUNI_PAYMENT_UOP = "PAYUNI_PAYMENT_UOP";
    public static final String EVENT_TYPE_PAYUNI_PAYMENT_RECURRING = "PAYUNI_PAYMENT_RECURRING";
    public static final String EVENT_TYPE_PAYUNI_SHIPPING_STATUS = "PAYUNI_SHIPPING_STATUS";
    public static final String EVENT_TYPE_PAYUNI_AGENT_NOTIFY = "PAYUNI_AGENT_NOTIFY";
    public static final String EVENT_TYPE_PAYUNI_REFUND_NOTIFY = "PAYUNI_REFUND_NOTIFY";
    public static final String EVENT_TYPE_PAYUNI_REFUND_ACCEPTED = "PAYUNI_REFUND_ACCEPTED";
    public static final String EVENT_TYPE_PAYUNI_UNKNOWN_FALLBACK = "PAYUNI_UNKNOWN_FALLBACK";

    // ==================== Queues ====================

    // PayUni Webhook 下游消費 Queue
    public static final String QUEUE_PAYMENT_PAYUNI_WEBHOOK = "payment.payuni.webhook";
    public static final String QUEUE_PAYMENT_PAYUNI_RECURRING = "payment.payuni.membership.recurring";
    public static final String QUEUE_PAYMENT_PAYUNI_RECURRING_DLQ = "payment.payuni.membership.recurring.dlq";
    public static final String QUEUE_PAYMENT_PAYUNI_REFUND_NOTIFY = "payment.payuni.refund.notify";
    public static final String QUEUE_PAYMENT_PAYUNI_REFUND_NOTIFY_DLQ = "payment.payuni.refund.notify.dlq";
    public static final String QUEUE_PAYMENT_PAYUNI_REFUND_ACCEPTED = "payment.payuni.refund.accepted";
    public static final String QUEUE_PAYMENT_PAYUNI_REFUND_ACCEPTED_DLQ = "payment.payuni.refund.accepted.dlq";
    /** §2.2 pivot (2026-05-14): payment-service 訂閱 UNKNOWN fallback。*/
    public static final String QUEUE_PAYMENT_PAYUNI_UNKNOWN_FALLBACK = "payment.payuni.unknown.fallback";
    public static final String QUEUE_PAYMENT_PAYUNI_UNKNOWN_FALLBACK_DLQ = "payment.payuni.unknown.fallback.dlq";
    /** §2.2 pivot (2026-05-14): aftersales-service 訂閱 UNKNOWN fallback。*/
    public static final String QUEUE_AFTERSALES_PAYUNI_UNKNOWN_FALLBACK = "aftersales.payuni.unknown.fallback";

    // PayUni Webhook DLQ
    public static final String EXCHANGE_PAYMENT_WEBHOOK_DLX = "payment.webhook.dlx";
    public static final String QUEUE_PAYMENT_WEBHOOK_DLQ = "payment.payuni.webhook.dlq";

    // Order service queues
    public static final String QUEUE_ORDER_PAYMENT_RESULT = "order.payment.result";
    public static final String QUEUE_ORDER_REFUND_RESULT = "order.refund.result";

    // Order service DLQ queues
    public static final String QUEUE_ORDER_PAYMENT_RESULT_DLQ = "order.payment.result.dlq";
    public static final String QUEUE_ORDER_REFUND_RESULT_DLQ = "order.refund.result.dlq";

    // order-service Shipping consumer DLQ（§23 P2.3-1，2026-05-20）
    public static final String QUEUE_ORDER_SHIPPING_CREATED_DLQ = "order.shipping.created.dlq";
    public static final String QUEUE_ORDER_SHIPPING_STATUS_DLQ = "order.shipping.status.dlq";
    public static final String QUEUE_ORDER_SHIPPING_LABEL_DLQ = "order.shipping.label.dlq";
    public static final String QUEUE_ORDER_SHIPPING_DELIVERED_DLQ = "order.shipping.delivered.dlq";
    public static final String QUEUE_ORDER_SHIPPING_PHOTO_DLQ = "order.shipping.photo.dlq";
    public static final String QUEUE_ORDER_SHIPMENT_PROCESSING_CLEARED_DLQ = "order.shipment.processing.cleared.dlq";

    // order-service AfterSales consumer DLQ（§23 P2.3-2，2026-05-20）
    public static final String QUEUE_ORDER_AFTERSALES_REFUND_COMPLETED_DLQ = "order.aftersales.refund.completed.dlq";
    public static final String QUEUE_ORDER_AFTERSALES_CANCEL_COMPLETED_DLQ = "order.aftersales.cancel.completed.dlq";
    public static final String QUEUE_ORDER_AFTERSALES_AWAITING_BUYER_PAGE_DLQ = "order.aftersales.awaiting.buyer.page.dlq";
    public static final String QUEUE_ORDER_AFTERSALES_COUNTDOWN_DLQ = "order.aftersales.countdown.dlq";
    // N16 的死信。🔴 DLX 跟著 **consumer 服務**走（hawkr.order.dlx），不跟著來源
    // exchange（hawkr.shipping）——每個服務才能在自己的 DLX 上看齊自己全部的死信。
    // 既有先例：aftersales.payment.refund.result 的 binding 來自 payment.events，
    // 而它的 via 是 aftersales.events.dlx。
    public static final String QUEUE_ORDER_FULFILLMENT_RESPONSIBILITY_FINALIZED_DLQ =
            "order.fulfillment.responsibility_finalized.dlq";

    // order-service Review consumer DLQ（§23 P2.3-3，2026-05-20）
    public static final String QUEUE_ORDER_REVIEW_CREATED_DLQ = "order.review.created.dlq";

    // shipping-service Order consumer DLQ（§23 P2.4，2026-05-20）
    public static final String QUEUE_SHIPPING_ORDER_RECEIPT_CONFIRMED_DLQ = "shipping.order.receipt.confirmed.dlq";

    // Product Service 消費取消訂單事件 Queue（庫存歸還）
    public static final String QUEUE_PRODUCT_ORDER_CANCELLED = "product.order.cancelled";
    public static final String QUEUE_PRODUCT_ORDER_CANCELLED_DLQ = "product.order.cancelled.dlq";

    // Product Service 消費銷量累計事件 Queue（35 設計書）
    public static final String QUEUE_PRODUCT_ORDER_SALES_ACCUMULATED = "product.order.sales.accumulated";
    public static final String QUEUE_PRODUCT_ORDER_SALES_ACCUMULATED_DLQ = "product.order.sales.accumulated.dlq";

    /** product-service 消費 buyer.favorite.added（02 §5.14） */
    public static final String QUEUE_PRODUCT_BUYER_FAVORITE_ADDED = "product.buyer.favorite.added";
    public static final String QUEUE_PRODUCT_BUYER_FAVORITE_ADDED_DLQ = "product.buyer.favorite.added.dlq";

    /** product-service 消費 buyer.favorite.removed（02 §5.14） */
    public static final String QUEUE_PRODUCT_BUYER_FAVORITE_REMOVED = "product.buyer.favorite.removed";
    public static final String QUEUE_PRODUCT_BUYER_FAVORITE_REMOVED_DLQ = "product.buyer.favorite.removed.dlq";

    /** product-service 消費 seller.deleted → 下架該賣家所有商品 */
    public static final String QUEUE_PRODUCT_SELLER_DELETED = "product.seller.deleted";
    public static final String QUEUE_PRODUCT_SELLER_DELETED_DLQ = "product.seller.deleted.dlq";

    // ==================== Event Types ====================

    // Order 相關 event type 已遷移至 OrderEventType enum（內部技術碼，使用 routing key 點分式 value），不再宣告為 String 常數
    // Payment 相關 event type 已遷移至 PaymentEventType enum（內部技術碼），不再宣告為 String 常數

    // Seller event types (PAYUNi integration)
    public static final String EVENT_TYPE_SELLER_PAYUNI_REGISTER = "SELLER_PAYUNI_REGISTER";
    public static final String EVENT_TYPE_SELLER_PAYUNI_REGISTERED = "SELLER_PAYUNI_REGISTERED";
    public static final String EVENT_TYPE_SELLER_PAYUNI_FAILED = "SELLER_PAYUNI_FAILED";
    public static final String EVENT_TYPE_SELLER_PAYUNI_STATUS_CHECK = "SELLER_PAYUNI_STATUS_CHECK";
    public static final String EVENT_TYPE_SELLER_PAYUNI_STATUS_UPDATED = "SELLER_PAYUNI_STATUS_UPDATED";

    // Seller event types（賣家審核結果）
    public static final String EVENT_TYPE_SELLER_REVIEW_RESULT_RECEIVED = "SELLER_REVIEW_RESULT_RECEIVED";

    // User event types
    public static final String EVENT_TYPE_MEMBER_REGISTERED = "MEMBER_REGISTERED";
    public static final String EVENT_TYPE_MEMBER_PROFILE_UPDATED = "MEMBER_PROFILE_UPDATED";
    public static final String EVENT_TYPE_MEMBER_PASSWORD_CHANGED = "MEMBER_PASSWORD_CHANGED";
    public static final String EVENT_TYPE_MEMBER_ADDRESS_ADDED = "MEMBER_ADDRESS_ADDED";
    public static final String EVENT_TYPE_MEMBER_ADDRESS_UPDATED = "MEMBER_ADDRESS_UPDATED";
    public static final String EVENT_TYPE_MEMBER_AVATAR_CHANGED = "MEMBER_AVATAR_CHANGED";
    public static final String EVENT_TYPE_SELLER_APPROVED = "SELLER_APPROVED";

    // Catalog event types
    public static final String EVENT_TYPE_PRODUCT_CREATED = "PRODUCT_CREATED";
    public static final String EVENT_TYPE_PRODUCT_UPDATED = "PRODUCT_UPDATED";
    public static final String EVENT_TYPE_PRODUCT_STATUS_CHANGED = "PRODUCT_STATUS_CHANGED";
    public static final String EVENT_TYPE_REVIEW_POSTED = "REVIEW_POSTED";

    // Fulfillment event types
    public static final String EVENT_TYPE_SHIPMENT_CREATED = "SHIPMENT_CREATED";
    public static final String EVENT_TYPE_SHIPMENT_LABEL_UPDATED = "SHIPMENT_LABEL_UPDATED";
    public static final String EVENT_TYPE_SHIPMENT_DELIVERED = "SHIPMENT_DELIVERED";
    public static final String EVENT_TYPE_SHIPMENT_PHOTO_UPDATED = "SHIPMENT_PHOTO_UPDATED";

    // Engagement event types (Content/Social)
    public static final String EVENT_TYPE_POST_CREATED = "POST_CREATED";
    public static final String EVENT_TYPE_POST_UPDATED = "POST_UPDATED";
    public static final String EVENT_TYPE_POST_DELETED = "POST_DELETED";
    public static final String EVENT_TYPE_COMMENT_ADDED = "COMMENT_ADDED";
    public static final String EVENT_TYPE_COMMENT_DELETED = "COMMENT_DELETED";
    public static final String EVENT_TYPE_LIKE_ADDED = "LIKE_ADDED";
    public static final String EVENT_TYPE_LIKE_REMOVED = "LIKE_REMOVED";
    public static final String EVENT_TYPE_FOLLOW_ADDED = "FOLLOW_ADDED";
    public static final String EVENT_TYPE_FOLLOW_REMOVED = "FOLLOW_REMOVED";

    // IM event types
    public static final String EVENT_TYPE_CHAT_MESSAGE_SENT = "CHAT_MESSAGE_SENT";

    // Media event types (OSS cleanup)

    // Seller CRM event types
    public static final String EVENT_TYPE_CUSTOMER_STATUS_CHANGED = "CUSTOMER_STATUS_CHANGED";
    public static final String EVENT_TYPE_CUSTOMER_TAGGED = "CUSTOMER_TAGGED";
    public static final String EVENT_TYPE_BATCH_COUPON_TASK_CREATED = "BATCH_COUPON_TASK_CREATED";
    public static final String EVENT_TYPE_BATCH_COUPON_TASK_COMPLETED = "BATCH_COUPON_TASK_COMPLETED";

    // AfterSales routing keys 已遷至 CodeMeta enum AfterSalesEventType（2026-05-20，docs/開發規範/MQ 與事件管理規範.md）
    // 取用方式：AfterSalesEventType.XXX.value()
    public static final String EXCHANGE_AFTERSALES = "aftersales.events";

    // Order Service 消費 AfterSales 事件 Queue
    public static final String QUEUE_ORDER_AFTERSALES_REFUND_COMPLETED = "order.aftersales.refund.completed";
    public static final String QUEUE_ORDER_AFTERSALES_CANCEL_COMPLETED = "order.aftersales.cancel.completed";
    public static final String QUEUE_ORDER_AFTERSALES_AWAITING_BUYER_PAGE = "order.aftersales.awaiting.buyer.page";
    // 自動完成 60h 倒數的暫停/恢復事件（03 §5.8）
    public static final String QUEUE_ORDER_AFTERSALES_COUNTDOWN = "order.aftersales.countdown";

    // ── N16 的 payment 側 consumer（Logistics Finance，36 §3.3.1）──────────────
    // 🔴 與 order-service 那條是**同一條 routing key、兩條 queue**：兩邊對同一件事做兩件
    //    不同的副作用（訂單終態轉換 vs settlement counterparty 與 receivable 的處置）。
    //    合併成一條再由單一 consumer 代打，會讓其中一邊的失敗把另一邊一起拖進 DLQ。
    //    topic exchange 對同一個 routing key 投遞給**每一條**綁定的 queue，這正是它存在的理由。
    public static final String QUEUE_PAYMENT_FULFILLMENT_RESPONSIBILITY_FINALIZED =
            "payment.fulfillment.responsibility_finalized";
    public static final String QUEUE_PAYMENT_FULFILLMENT_RESPONSIBILITY_FINALIZED_DLQ =
            "payment.fulfillment.responsibility_finalized.dlq";

    /**
     * Logistics Finance 消費 provider 成本事實（70 §3.1.4 durable join 的成本側入口）。
     *
     * <p>🔴 DLX 走 {@link #EXCHANGE_PAYMENT_EVENTS_DLX}——<b>DLX 跟著 consumer 服務走，
     * 不是跟著來源 exchange</b>（36 §6.2 註）。來源是 {@link #EXCHANGE_SHIPPING}，
     * 但死信要落在 payment 自己的 DLX 上，否則排查時會跑到 shipping 的死信去找。
     */
    public static final String QUEUE_PAYMENT_LOGISTICS_COST_INCURRED =
            "payment.logistics.cost_incurred";
    public static final String QUEUE_PAYMENT_LOGISTICS_COST_INCURRED_DLQ =
            "payment.logistics.cost_incurred.dlq";

    // ── N16 履約責任判定 financially final（36 §3.3.1／03 §5.7.3）──────────────
    // 🔴 order-service 是**兩個** consumer 之一（另一個是 payment-service 的 Logistics
    //    Finance）。同一條 routing key、兩條 queue 是刻意的：兩邊對同一件事做兩件不同的
    //    副作用（訂單終態轉換 vs receivable 建立與否），合併成一條再由單一 consumer 代打
    //    會讓其中一邊的失敗把另一邊一起拖進 DLQ。
    // 🔴 這條 queue 是 `35 → 54` 與建撥款任務的**唯一入口**：order-service 不提供人工改
    //    狀態端點、也沒有補推排程 ⇒ 漏登記 topology.yaml 就是整個終態變成死碼，而
    //    passive declare 讓它**不 crash、不進 DLQ、不報錯**。
    public static final String QUEUE_ORDER_FULFILLMENT_RESPONSIBILITY_FINALIZED =
            "order.fulfillment.responsibility_finalized";

    // AfterSales DLX
    public static final String EXCHANGE_AFTERSALES_DLX = "aftersales.events.dlx";

    // AfterSales Service 消費其他服務事件 Queue
    public static final String QUEUE_AFTERSALES_RETURN_SHIPMENT_STATUS = "aftersales.shipping.return.status";
    public static final String QUEUE_AFTERSALES_REFUND_RESULT = "aftersales.payment.refund.result";
    public static final String QUEUE_AFTERSALES_SHIPMENT_LABEL = "aftersales.shipping.label";

    // AfterSales 事件類型
    public static final String EVENT_TYPE_AFTERSALES_REFUND_COMPLETED = "AFTERSALES_REFUND_COMPLETED";
    public static final String EVENT_TYPE_AFTERSALES_CANCEL_COMPLETED = "AFTERSALES_CANCEL_COMPLETED";
    public static final String EVENT_TYPE_AFTERSALES_AWAITING_BUYER_REFUND_PAGE = "AFTERSALES_AWAITING_BUYER_REFUND_PAGE";

    // AfterSales routing keys 已全部遷至 AfterSalesEventType enum（取用方式：AfterSalesEventType.XXX.value()）
    // EVENT_TYPE_AFTERSALES_* metadata 仍保留作為 publish 第 4 個參數的 event_type 字串
    public static final String EVENT_TYPE_AFTERSALES_APPEAL_SUBMITTED = "AFTER_SALE_APPEALED";
    public static final String EVENT_TYPE_AFTERSALES_PLATFORM_DECISION_MADE = "AFTER_SALE_DECISION_MADE";
    public static final String EVENT_TYPE_AFTERSALES_CASE_AUTO_REJECTED = "AFTERSALES_CASE_AUTO_REJECTED";
    public static final String EVENT_TYPE_AFTERSALES_CASE_AUTO_ESCALATED = "AFTERSALES_CASE_AUTO_ESCALATED";
    public static final String EVENT_TYPE_AFTERSALES_CASE_APPLIED = "AFTERSALES_CASE_APPLIED";
    public static final String EVENT_TYPE_AFTERSALES_CASE_RETURN_APPROVED = "AFTERSALES_CASE_RETURN_APPROVED";
    public static final String EVENT_TYPE_AFTERSALES_CASE_REFUND_APPROVED = "AFTERSALES_CASE_REFUND_APPROVED";
    public static final String EVENT_TYPE_AFTERSALES_CASE_COUPON_COMPENSATED = "AFTERSALES_CASE_COUPON_COMPENSATED";
    public static final String EVENT_TYPE_AFTERSALES_CASE_REJECTED = "AFTERSALES_CASE_REJECTED";
    public static final String EVENT_TYPE_AFTERSALES_CASE_CANCELLED_BY_BUYER = "AFTERSALES_CASE_CANCELLED_BY_BUYER";
    // #1225 確認收貨自動取消售後
    public static final String EVENT_TYPE_AFTERSALES_CASE_CANCELLED_BY_CONFIRM_RECEIPT = "AFTERSALES_CASE_CANCELLED_BY_CONFIRM_RECEIPT";
    public static final String EVENT_TYPE_AFTERSALES_CASE_RETURN_RECEIVED = "AFTERSALES_CASE_RETURN_RECEIVED";
    public static final String EVENT_TYPE_AFTERSALES_CASE_RETURN_IN_TRANSIT = "AFTERSALES_CASE_RETURN_IN_TRANSIT";
    public static final String EVENT_TYPE_AFTERSALES_CASE_RETURN_DELIVERED = "AFTERSALES_CASE_RETURN_DELIVERED";
    public static final String EVENT_TYPE_AFTERSALES_CASE_RETURN_FAILED = "AFTERSALES_CASE_RETURN_FAILED";
    // 2026-05-22 逆物流退貨政策修訂新增（5 個事件；CASE_AUTO_RECEIPT_CONFIRMED 已由 _TIMEOUT 細分取代）
    public static final String EVENT_TYPE_AFTERSALES_CASE_RETURN_AT_STORE = "AFTERSALES_CASE_RETURN_AT_STORE";
    public static final String EVENT_TYPE_AFTERSALES_CASE_RETURN_AT_STORE_24H = "AFTERSALES_CASE_RETURN_AT_STORE_24H";
    public static final String EVENT_TYPE_AFTERSALES_CASE_RETURN_DELIVERED_24H = "AFTERSALES_CASE_RETURN_DELIVERED_24H";
    public static final String EVENT_TYPE_AFTERSALES_CASE_AUTO_COMPLETED_STORE_PICKUP_TIMEOUT = "AFTERSALES_CASE_AUTO_COMPLETED_STORE_PICKUP_TIMEOUT";
    public static final String EVENT_TYPE_AFTERSALES_CASE_AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT = "AFTERSALES_CASE_AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT";
    // 2026-06-15 待寄回逾時政策修訂新增（2 個事件）
    public static final String EVENT_TYPE_AFTERSALES_CASE_RETURN_PENDING_24H = "AFTERSALES_CASE_RETURN_PENDING_24H";
    public static final String EVENT_TYPE_AFTERSALES_CASE_AUTO_COMPLETED_RETURN_PENDING_TIMEOUT = "AFTERSALES_CASE_AUTO_COMPLETED_RETURN_PENDING_TIMEOUT";

    // Review Events
    public static final String EXCHANGE_REVIEW = "hawkr.review";
    public static final String ROUTING_KEY_REVIEW_CREATED = "review.created";
    public static final String ROUTING_KEY_REVIEW_HIDDEN = "review.hidden"; // #1156：admin 隱藏評論後重算聚合
    public static final String QUEUE_ORDER_REVIEW_CREATED = "order.review.created";

    /** product-service 消費 review.created + review.hidden 更新 product.rating / review_count（02 §5.14a，#1156） */
    public static final String QUEUE_PRODUCT_REVIEW_STATS = "product.review.stats";
    public static final String QUEUE_PRODUCT_REVIEW_STATS_DLQ = "product.review.stats.dlq";

    /**
     * product-service 消費自家 {@link #ROUTING_KEY_PRODUCT_CERTIFICATION_VERIFY_REQUESTED}
     * 做認證背景查驗（19 §12 / 36 §3.6h.1，#2755）。Java manual-ack ⇒ topology 走
     * {@code dlq: {via, routing_key}}，且 {@code routing_key} 必須恰為 {@code {queue}.dlq}
     * （{@code hawkr.product.dlx} 是 topic exchange，寫別的名字死信會沒有 binding 命中而被丟棄）。
     */
    public static final String QUEUE_PRODUCT_CERTIFICATION_VERIFY = "product.certification.verify";
    public static final String QUEUE_PRODUCT_CERTIFICATION_VERIFY_DLQ = "product.certification.verify.dlq";

    // Seller CRM routing keys
    public static final String ROUTING_KEY_CUSTOMER_STATUS_CHANGED = "seller.customer.status.changed";
    public static final String ROUTING_KEY_CUSTOMER_TAGGED = "seller.customer.tagged";
    public static final String ROUTING_KEY_BATCH_COUPON_TASK_CREATED = "seller.marketing.batch.created";
    public static final String ROUTING_KEY_BATCH_COUPON_TASK_COMPLETED = "seller.marketing.batch.completed";

    // ============================================================================
    // Product Import 領域（40 號設計書 §6 + 36 號設計書 §3.6b）
    // ============================================================================

    // Exchange
    public static final String EXCHANGE_PRODUCT_IMPORT = "hawkr.product.import";
    public static final String EXCHANGE_PRODUCT_IMPORT_DLX = "hawkr.product.import.dlx";

    // Routing keys（內部 + 跨服務 + 跨團隊）
    public static final String RK_PRODUCT_IMPORT_CSV_PARSE = "csv.parse";
    public static final String RK_PRODUCT_IMPORT_HTML_FETCH = "html.fetch";
    public static final String RK_PRODUCT_IMPORT_ASSET_DOWNLOAD = "asset.download";
    public static final String RK_PRODUCT_IMPORT_COMMIT = "commit";
    public static final String RK_PRODUCT_IMPORT_GC_EXPIRE_JOB = "gc.expire_job";
    public static final String RK_PRODUCT_IMPORT_JOB_PARSED = "job.parsed";
    public static final String RK_PRODUCT_IMPORT_JOB_COMMIT_FINISHED = "job.commit_finished";

    // 主 queue（product-import-service 內部）
    public static final String QUEUE_PRODUCT_IMPORT_CSV_PARSE = "product.import.csv.parse";
    public static final String QUEUE_PRODUCT_IMPORT_HTML_FETCH = "product.import.html.fetch";
    public static final String QUEUE_PRODUCT_IMPORT_ASSET_DOWNLOAD = "product.import.asset.download";
    public static final String QUEUE_PRODUCT_IMPORT_COMMIT = "product.import.commit";

    // Retry queue（依 36 號 §3.6b 設定 TTL 三階退避，binding 由各 service Config 決定 routing key）
    public static final String QUEUE_PRODUCT_IMPORT_CSV_PARSE_RETRY = "product.import.csv.parse.retry";
    public static final String QUEUE_PRODUCT_IMPORT_HTML_FETCH_RETRY = "product.import.html.fetch.retry";
    public static final String QUEUE_PRODUCT_IMPORT_ASSET_DOWNLOAD_RETRY = "product.import.asset.download.retry";
    public static final String QUEUE_PRODUCT_IMPORT_COMMIT_RETRY = "product.import.commit.retry";

    // DLQ
    public static final String QUEUE_PRODUCT_IMPORT_CSV_PARSE_DLQ = "dlq.product.import.csv.parse";
    public static final String QUEUE_PRODUCT_IMPORT_HTML_FETCH_DLQ = "dlq.product.import.html.fetch";
    public static final String QUEUE_PRODUCT_IMPORT_ASSET_DOWNLOAD_DLQ = "dlq.product.import.asset.download";
    public static final String QUEUE_PRODUCT_IMPORT_COMMIT_DLQ = "dlq.product.import.commit";
    public static final String QUEUE_PRODUCT_IMPORT_JOB_PARSED_DLQ = "dlq.product.import.job.parsed";
    public static final String QUEUE_PRODUCT_IMPORT_JOB_COMMIT_FINISHED_DLQ = "dlq.product.import.job.commit_finished";

    // 跨服務（media-service 訂閱 gc.expire_job）
    public static final String QUEUE_MEDIA_PRODUCT_IMPORT_GC_EXPIRE_JOB = "media.product.import.gc.expire_job";
    public static final String QUEUE_MEDIA_PRODUCT_IMPORT_GC_EXPIRE_JOB_DLQ = "dlq.media.product.import.gc.expire_job";

    // 跨團隊（notification-service 訂閱 job.parsed / job.commit_finished — 名稱待 notification team 確定 binding 後同步補）

    // ============================================================================
    // Media Service Callback for Product Import（08 號 §7.2a）
    //   Producer: media-service worker; Consumer: product-import-service MediaCallbackListener
    //   Exchange: 沿用既有 EXCHANGE_MEDIA
    // ============================================================================

    public static final String RK_MEDIA_IMPORT_ASSET_DOWNLOADED = "import.asset.downloaded";
    public static final String QUEUE_PRODUCT_IMPORT_MEDIA_CALLBACK = "product.import.media.callback";
    public static final String QUEUE_PRODUCT_IMPORT_MEDIA_CALLBACK_RETRY = "product.import.media.callback.retry";
    public static final String QUEUE_PRODUCT_IMPORT_MEDIA_CALLBACK_DLQ = "dlq.media.product.import.callback";

    // ============================================================================
    // Media Service 內部下載 worker pool（08 號 §7.2a；純 Media 內部，與對外 callback 區隔）
    // ============================================================================

    public static final String RK_MEDIA_IMPORT_ASSET_DOWNLOAD_INTERNAL = "import.asset.download.internal";
    public static final String QUEUE_MEDIA_IMPORT_ASSET_DOWNLOAD_INTERNAL = "media.import.asset.download.internal";
    public static final String QUEUE_MEDIA_IMPORT_ASSET_DOWNLOAD_INTERNAL_RETRY = "media.import.asset.download.internal.retry";
    public static final String QUEUE_MEDIA_IMPORT_ASSET_DOWNLOAD_INTERNAL_DLQ = "dlq.media.import.asset.download.internal";
}
