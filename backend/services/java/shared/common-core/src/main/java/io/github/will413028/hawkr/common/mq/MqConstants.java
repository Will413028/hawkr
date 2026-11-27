package io.github.will413028.hawkr.common.mq;

/**
 * RabbitMQ constants for exchanges, queues, and routing keys
 */
public final class MqConstants {

    private MqConstants() {}


    public static final String EXCHANGE_ORDER = "hawkr.order";
    public static final String EXCHANGE_PAYMENT = "payment.events";
    public static final String EXCHANGE_MARKETING = "hawkr.marketing";
    public static final String EXCHANGE_DELAYED = "hawkr.delayed";
    public static final String EXCHANGE_SELLER_BIZ = "hawkr.seller";
    public static final String EXCHANGE_USER = "hawkr.user";

    /** 買家領域（buyer-service Go 服務 publish；02 §5.14 / 36 §3.6a） */
    public static final String EXCHANGE_BUYER = "hawkr.buyer";

    /** 帳號領域（account-service Go 服務 publish；39 §5.8.1 / 07 §5.19 / 36 §3.6c） */
    public static final String EXCHANGE_ACCOUNT = "hawkr.account";

    /** Coupon Service 私有 DLX（07 §5.19，2026-06-04） */
    public static final String EXCHANGE_COUPON_DLX = "hawkr.coupon.dlx";

    /** 商品領域（product-service Java 服務 publish 補貨事件；02 §5.13b / 36 §3.6h，2026-06 #942 新建） */
    public static final String EXCHANGE_PRODUCT = "hawkr.product";

    public static final String EXCHANGE_PRODUCT_DLX = "hawkr.product.dlx";

    public static final String EXCHANGE_ORDER_DLX = "hawkr.order.dlx";
    public static final String EXCHANGE_PAYMENT_DLX = "hawkr.payment.dlx";
    public static final String EXCHANGE_MARKETING_DLX = "hawkr.marketing.dlx";
    public static final String EXCHANGE_SELLER_BIZ_DLX = "hawkr.seller.dlx";
    public static final String EXCHANGE_USER_DLX = "hawkr.user.dlx";
    /** Cart Service 私有 DLX（#1426：cart 首個 consumer seller.profile.changed 的死信出口） */
    public static final String EXCHANGE_CART_DLX = "hawkr.cart.dlx";


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
    public static final String ROUTING_KEY_ORDER_SHIPMENT_CREATE_REQUESTED = "order.shipment.create_requested";
    public static final String ROUTING_KEY_ORDER_SHIPMENT_CREATION_FAILED = "order.shipment.creation_failed";

    public static final String ROUTING_KEY_PAYMENT_SUCCESS = "payment.success";
    public static final String ROUTING_KEY_PAYMENT_FAILED = "payment.failed";
    public static final String ROUTING_KEY_PAYMENT_EXPIRED = "payment.expired";
    public static final String ROUTING_KEY_PAYMENT_CLOSED = "payment.closed";
    public static final String ROUTING_KEY_PAYMENT_CONFLICT = "payment.conflict";

    public static final String EXCHANGE_SELLER = "seller.events";
    public static final String ROUTING_KEY_SELLER_REVIEW_RESULT = "seller.review.result";

    public static final String ROUTING_KEY_MEMBER_REGISTERED = "member.registered";
    public static final String ROUTING_KEY_MEMBER_PROFILE_UPDATED = "member.profile.updated";
    public static final String ROUTING_KEY_MEMBER_PASSWORD_CHANGED = "member.password.changed";
    public static final String ROUTING_KEY_MEMBER_ADDRESS_ADDED = "member.address.added";
    public static final String ROUTING_KEY_MEMBER_ADDRESS_UPDATED = "member.address.updated";
    public static final String ROUTING_KEY_MEMBER_AVATAR_CHANGED = "member.avatar.changed";
    public static final String ROUTING_KEY_SELLER_APPROVED = "seller.approved";

    public static final String EXCHANGE_CATALOG = "hawkr.catalog";
    public static final String EXCHANGE_CATALOG_DLX = "hawkr.catalog.dlx";
    public static final String ROUTING_KEY_PRODUCT_CREATED = "product.created";
    public static final String ROUTING_KEY_PRODUCT_UPDATED = "product.updated";
    public static final String ROUTING_KEY_PRODUCT_STATUS_CHANGED = "product.status.changed";
    public static final String ROUTING_KEY_REVIEW_POSTED = "review.posted";

    public static final String ROUTING_KEY_REFUND_SUCCEEDED = "refund.succeeded";
    public static final String ROUTING_KEY_REFUND_FAILED = "refund.failed";

    public static final String EXCHANGE_SHIPPING = "hawkr.shipping";
    public static final String EXCHANGE_SHIPPING_DLX = "hawkr.shipping.dlx";
    public static final String ROUTING_KEY_SHIPMENT_CREATED = "shipment.created";
    public static final String ROUTING_KEY_SHIPMENT_LABEL_UPDATED = "shipment.label.updated";
    public static final String ROUTING_KEY_SHIPMENT_DELIVERED = "shipment.delivered";
    public static final String ROUTING_KEY_SHIPMENT_PHOTO_UPDATED = "shipment.photo.updated";

    public static final String ROUTING_KEY_SHIPMENT_ANOMALY_DETECTED = "shipment.anomaly.detected";
    public static final String EVENT_TYPE_SHIPMENT_ANOMALY_DETECTED = "SHIPMENT_ANOMALY_DETECTED";

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


    public static final String QUEUE_SHIPPING_PAYUNI_WEBHOOK = "shipping.payuni.webhook";
    public static final String QUEUE_SHIPPING_PAYUNI_WEBHOOK_DLQ = "shipping.payuni.webhook.dlq";
    public static final String EXCHANGE_SHIPPING_WEBHOOK_DLX = "shipping.webhook.dlx";
    /** Uber Direct 貨態回調 queue（綁 {@link #ROUTING_KEY_UBER_SHIPPING_STATUS}）。 */
    public static final String QUEUE_SHIPPING_UBER_WEBHOOK = "shipping.uber.webhook";
    public static final String QUEUE_SHIPPING_UBER_WEBHOOK_DLQ = "shipping.uber.webhook.dlq";

    public static final String ROUTING_KEY_SHIPMENT_STATUS_CHANGED = "shipment.status.changed";
    public static final String ROUTING_KEY_RETURN_SHIPMENT_STATUS_CHANGED = "shipment.return.status_changed";
    public static final String EVENT_TYPE_RETURN_SHIPMENT_STATUS_CHANGED = "RETURN_SHIPMENT_STATUS_CHANGED";

    public static final String ROUTING_KEY_SHIPMENT_PROCESSING_TIMEOUT = "shipment.processing.timeout";
    public static final String ROUTING_KEY_SHIPMENT_PROCESSING_CLEARED = "shipment.processing.cleared";
    public static final String QUEUE_AFTERSALES_SHIPMENT_PROCESSING_TIMEOUT = "aftersales.shipment.processing.timeout";
    public static final String QUEUE_ORDER_SHIPMENT_PROCESSING_CLEARED = "order.shipment.processing.cleared";
    public static final String EVENT_TYPE_SHIPMENT_PROCESSING_TIMEOUT = "SHIPMENT_PROCESSING_TIMEOUT";
    public static final String EVENT_TYPE_SHIPMENT_PROCESSING_CLEARED = "SHIPMENT_PROCESSING_CLEARED";

    public static final String EVENT_TYPE_SHIPMENT_STATUS_CHANGED = "SHIPMENT_STATUS_CHANGED";

    public static final String QUEUE_ORDER_SHIPPING_CREATED = "order.shipping.created";
    public static final String QUEUE_ORDER_SHIPPING_LABEL = "order.shipping.label";
    public static final String QUEUE_ORDER_SHIPPING_STATUS = "order.shipping.status";
    public static final String QUEUE_ORDER_SHIPPING_DELIVERED = "order.shipping.delivered";
    public static final String QUEUE_ORDER_SHIPPING_PHOTO = "order.shipping.photo";

    public static final String ROUTING_KEY_ORDER_DELIVERED = "order.delivered";

    public static final String ROUTING_KEY_ORDER_RECEIPT_CONFIRMED = "order.receipt.confirmed";

    public static final String QUEUE_SHIPPING_ORDER_RECEIPT_CONFIRMED = "shipping.order.receipt.confirmed";

    public static final String QUEUE_SHIPPING_ORDER_PAID = "shipping.order.paid";
    public static final String QUEUE_SHIPPING_ORDER_PAID_DLQ = "shipping.order.paid.dlq";

    public static final String QUEUE_SHIPPING_SHIPMENT_CREATE_REQUESTED = "shipping.shipment.create_requested";
    public static final String QUEUE_SHIPPING_SHIPMENT_CREATE_REQUESTED_DLQ = "shipping.shipment.create_requested.dlq";
    public static final String QUEUE_AFTERSALES_SHIPMENT_CREATION_FAILED = "aftersales.shipment.creation_failed";

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

    public static final String EXCHANGE_IM = "hawkr.im";
    public static final String EXCHANGE_IM_DLX = "hawkr.im.dlx";
    public static final String ROUTING_KEY_CHAT_MESSAGE_SENT = "chat.message.sent";

    public static final String EXCHANGE_PAYUNI_WEBHOOK = "payuni.events";
    public static final String ROUTING_KEY_PAYUNI_SHIPPING_STATUS = "payuni.shipping.status";

    /**
     * Uber Direct 貨態回調（本波唯一新增的 routing key）。
     *
     * <p>🔴 <b>exchange 沿用 {@link #EXCHANGE_PAYUNI_WEBHOOK}</b>（{@code payuni.events}）：
     * 沿用既有 PayUni 的形狀（那條路徑已經證明可行），36 §3.5 註記
     * 「exchange 名 未逐字指定（沿用 payuni.events 家族形狀）⇒ 待坐實」。
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

    public static final String EVENT_TYPE_PAYUNI_SHIPPING_STATUS = "PAYUNI_SHIPPING_STATUS";
    public static final String EVENT_TYPE_PAYUNI_AGENT_NOTIFY = "PAYUNI_AGENT_NOTIFY";


    public static final String QUEUE_ORDER_PAYMENT_RESULT = "order.payment.result";
    public static final String QUEUE_ORDER_REFUND_RESULT = "order.refund.result";

    public static final String QUEUE_ORDER_PAYMENT_RESULT_DLQ = "order.payment.result.dlq";
    public static final String QUEUE_ORDER_REFUND_RESULT_DLQ = "order.refund.result.dlq";

    public static final String QUEUE_ORDER_SHIPPING_CREATED_DLQ = "order.shipping.created.dlq";
    public static final String QUEUE_ORDER_SHIPPING_STATUS_DLQ = "order.shipping.status.dlq";
    public static final String QUEUE_ORDER_SHIPPING_LABEL_DLQ = "order.shipping.label.dlq";
    public static final String QUEUE_ORDER_SHIPPING_DELIVERED_DLQ = "order.shipping.delivered.dlq";
    public static final String QUEUE_ORDER_SHIPPING_PHOTO_DLQ = "order.shipping.photo.dlq";
    public static final String QUEUE_ORDER_SHIPMENT_PROCESSING_CLEARED_DLQ = "order.shipment.processing.cleared.dlq";

    public static final String QUEUE_ORDER_AFTERSALES_REFUND_COMPLETED_DLQ = "order.aftersales.refund.completed.dlq";
    public static final String QUEUE_ORDER_AFTERSALES_CANCEL_COMPLETED_DLQ = "order.aftersales.cancel.completed.dlq";
    public static final String QUEUE_ORDER_AFTERSALES_COUNTDOWN_DLQ = "order.aftersales.countdown.dlq";
    public static final String QUEUE_ORDER_REVIEW_CREATED_DLQ = "order.review.created.dlq";

    public static final String QUEUE_SHIPPING_ORDER_RECEIPT_CONFIRMED_DLQ = "shipping.order.receipt.confirmed.dlq";

    public static final String QUEUE_PRODUCT_ORDER_CANCELLED = "product.order.cancelled";
    public static final String QUEUE_PRODUCT_ORDER_CANCELLED_DLQ = "product.order.cancelled.dlq";

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



    public static final String EVENT_TYPE_SELLER_REVIEW_RESULT_RECEIVED = "SELLER_REVIEW_RESULT_RECEIVED";

    public static final String EVENT_TYPE_MEMBER_REGISTERED = "MEMBER_REGISTERED";
    public static final String EVENT_TYPE_MEMBER_PROFILE_UPDATED = "MEMBER_PROFILE_UPDATED";
    public static final String EVENT_TYPE_MEMBER_PASSWORD_CHANGED = "MEMBER_PASSWORD_CHANGED";
    public static final String EVENT_TYPE_MEMBER_ADDRESS_ADDED = "MEMBER_ADDRESS_ADDED";
    public static final String EVENT_TYPE_MEMBER_ADDRESS_UPDATED = "MEMBER_ADDRESS_UPDATED";
    public static final String EVENT_TYPE_MEMBER_AVATAR_CHANGED = "MEMBER_AVATAR_CHANGED";
    public static final String EVENT_TYPE_SELLER_APPROVED = "SELLER_APPROVED";

    public static final String EVENT_TYPE_PRODUCT_CREATED = "PRODUCT_CREATED";
    public static final String EVENT_TYPE_PRODUCT_UPDATED = "PRODUCT_UPDATED";
    public static final String EVENT_TYPE_PRODUCT_STATUS_CHANGED = "PRODUCT_STATUS_CHANGED";
    public static final String EVENT_TYPE_REVIEW_POSTED = "REVIEW_POSTED";

    public static final String EVENT_TYPE_SHIPMENT_CREATED = "SHIPMENT_CREATED";
    public static final String EVENT_TYPE_SHIPMENT_LABEL_UPDATED = "SHIPMENT_LABEL_UPDATED";
    public static final String EVENT_TYPE_SHIPMENT_DELIVERED = "SHIPMENT_DELIVERED";
    public static final String EVENT_TYPE_SHIPMENT_PHOTO_UPDATED = "SHIPMENT_PHOTO_UPDATED";

    public static final String EVENT_TYPE_POST_CREATED = "POST_CREATED";
    public static final String EVENT_TYPE_POST_UPDATED = "POST_UPDATED";
    public static final String EVENT_TYPE_POST_DELETED = "POST_DELETED";
    public static final String EVENT_TYPE_COMMENT_ADDED = "COMMENT_ADDED";
    public static final String EVENT_TYPE_COMMENT_DELETED = "COMMENT_DELETED";
    public static final String EVENT_TYPE_LIKE_ADDED = "LIKE_ADDED";
    public static final String EVENT_TYPE_LIKE_REMOVED = "LIKE_REMOVED";
    public static final String EVENT_TYPE_FOLLOW_ADDED = "FOLLOW_ADDED";
    public static final String EVENT_TYPE_FOLLOW_REMOVED = "FOLLOW_REMOVED";

    public static final String EVENT_TYPE_CHAT_MESSAGE_SENT = "CHAT_MESSAGE_SENT";


    public static final String EVENT_TYPE_CUSTOMER_STATUS_CHANGED = "CUSTOMER_STATUS_CHANGED";
    public static final String EVENT_TYPE_CUSTOMER_TAGGED = "CUSTOMER_TAGGED";
    public static final String EVENT_TYPE_BATCH_COUPON_TASK_CREATED = "BATCH_COUPON_TASK_CREATED";
    public static final String EVENT_TYPE_BATCH_COUPON_TASK_COMPLETED = "BATCH_COUPON_TASK_COMPLETED";

    public static final String EXCHANGE_AFTERSALES = "aftersales.events";

    public static final String QUEUE_ORDER_AFTERSALES_REFUND_COMPLETED = "order.aftersales.refund.completed";
    public static final String QUEUE_ORDER_AFTERSALES_CANCEL_COMPLETED = "order.aftersales.cancel.completed";
    public static final String QUEUE_ORDER_AFTERSALES_COUNTDOWN = "order.aftersales.countdown";

    public static final String EXCHANGE_AFTERSALES_DLX = "aftersales.events.dlx";

    public static final String QUEUE_AFTERSALES_RETURN_SHIPMENT_STATUS = "aftersales.shipping.return.status";
    public static final String QUEUE_AFTERSALES_SHIPMENT_LABEL = "aftersales.shipping.label";

    public static final String EVENT_TYPE_AFTERSALES_REFUND_COMPLETED = "AFTERSALES_REFUND_COMPLETED";
    public static final String EVENT_TYPE_AFTERSALES_CANCEL_COMPLETED = "AFTERSALES_CANCEL_COMPLETED";

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
    public static final String EVENT_TYPE_AFTERSALES_CASE_CANCELLED_BY_CONFIRM_RECEIPT = "AFTERSALES_CASE_CANCELLED_BY_CONFIRM_RECEIPT";
    public static final String EVENT_TYPE_AFTERSALES_CASE_RETURN_RECEIVED = "AFTERSALES_CASE_RETURN_RECEIVED";
    public static final String EVENT_TYPE_AFTERSALES_CASE_RETURN_IN_TRANSIT = "AFTERSALES_CASE_RETURN_IN_TRANSIT";
    public static final String EVENT_TYPE_AFTERSALES_CASE_RETURN_DELIVERED = "AFTERSALES_CASE_RETURN_DELIVERED";
    public static final String EVENT_TYPE_AFTERSALES_CASE_RETURN_FAILED = "AFTERSALES_CASE_RETURN_FAILED";
    public static final String EVENT_TYPE_AFTERSALES_CASE_RETURN_AT_STORE = "AFTERSALES_CASE_RETURN_AT_STORE";
    public static final String EVENT_TYPE_AFTERSALES_CASE_RETURN_AT_STORE_24H = "AFTERSALES_CASE_RETURN_AT_STORE_24H";
    public static final String EVENT_TYPE_AFTERSALES_CASE_RETURN_DELIVERED_24H = "AFTERSALES_CASE_RETURN_DELIVERED_24H";
    public static final String EVENT_TYPE_AFTERSALES_CASE_AUTO_COMPLETED_STORE_PICKUP_TIMEOUT = "AFTERSALES_CASE_AUTO_COMPLETED_STORE_PICKUP_TIMEOUT";
    public static final String EVENT_TYPE_AFTERSALES_CASE_AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT = "AFTERSALES_CASE_AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT";
    public static final String EVENT_TYPE_AFTERSALES_CASE_RETURN_PENDING_24H = "AFTERSALES_CASE_RETURN_PENDING_24H";
    public static final String EVENT_TYPE_AFTERSALES_CASE_AUTO_COMPLETED_RETURN_PENDING_TIMEOUT = "AFTERSALES_CASE_AUTO_COMPLETED_RETURN_PENDING_TIMEOUT";

    public static final String EXCHANGE_REVIEW = "hawkr.review";
    public static final String ROUTING_KEY_REVIEW_CREATED = "review.created";
    public static final String ROUTING_KEY_REVIEW_HIDDEN = "review.hidden";
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

    public static final String ROUTING_KEY_CUSTOMER_STATUS_CHANGED = "seller.customer.status.changed";
    public static final String ROUTING_KEY_CUSTOMER_TAGGED = "seller.customer.tagged";
    public static final String ROUTING_KEY_BATCH_COUPON_TASK_CREATED = "seller.marketing.batch.created";
    public static final String ROUTING_KEY_BATCH_COUPON_TASK_COMPLETED = "seller.marketing.batch.completed";
}
