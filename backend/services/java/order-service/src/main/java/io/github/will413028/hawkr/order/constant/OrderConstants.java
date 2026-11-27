package io.github.will413028.hawkr.order.constant;

import io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus;
import io.github.will413028.hawkr.common.utils.code.StockTxPhase;
import io.github.will413028.hawkr.common.utils.code.StockTxStatus;

/**
 * 訂單服務常數
 */
public final class OrderConstants {

    private OrderConstants() {}


    /**
     * 訂單編號前綴
     */
    public static final String ORDER_NO_PREFIX = "ORD";

    /**
     * 退款申請編號前綴
     */
    public static final String REFUND_REQUEST_NO_PREFIX = "RFD";


    /**
     * order_no 每日流水號 Redis Key 前綴
     * 完整 Key: order:seq:{yyyyMMdd}
     */
    public static final String REDIS_KEY_ORDER_SEQ = "order:seq:";

    /**
     * order_no 每日流水號 TTL（秒），2 天確保跨日安全
     */
    public static final long REDIS_KEY_ORDER_SEQ_TTL_SECONDS = 172800L;


    /**
     * 用戶退款外部參考 ID 前綴
     */
    public static final String REFUND_EXTERNAL_ID_PREFIX = "REFUND_";

    /**
     * 自動退款外部參考 ID 前綴
     */
    public static final String AUTO_REFUND_EXTERNAL_ID_PREFIX = "AUTO_REFUND_";


    public static final String TASK_STATUS_PENDING = WorkerTaskStatus.PENDING.value();
    public static final String TASK_STATUS_PROCESSING = WorkerTaskStatus.PROCESSING.value();
    public static final String TASK_STATUS_DONE = WorkerTaskStatus.DONE.value();
    public static final String TASK_STATUS_FAILED = WorkerTaskStatus.FAILED.value();
    public static final String TASK_STATUS_MANUAL_INTERVENTION = WorkerTaskStatus.MANUAL_INTERVENTION.value();


    public static final String STOCK_TX_PHASE_DEDUCT = StockTxPhase.DEDUCT.value();
    public static final String STOCK_TX_PHASE_RESTORE = StockTxPhase.RESTORE.value();

    public static final String STOCK_TX_STATUS_INIT = StockTxStatus.INIT.value();
    public static final String STOCK_TX_STATUS_SUCCESS = StockTxStatus.SUCCESS.value();
    public static final String STOCK_TX_STATUS_FAILED = StockTxStatus.FAILED.value();
    public static final String STOCK_TX_STATUS_MANUAL_INTERVENTION = StockTxStatus.MANUAL_INTERVENTION.value();
    public static final String STOCK_TX_STATUS_COMPENSATED = StockTxStatus.COMPENSATED.value();


    public static final int DEFAULT_MAX_RETRY_COUNT = 10;

    /** coupon_consumption_tasks.task_type：付款後核銷。 */
    public static final String COUPON_TASK_TYPE_CONSUME = "CONSUME";
    /** coupon_consumption_tasks.task_type：取消或補償後以 order_no 釋券（D3）。 */
    public static final String COUPON_TASK_TYPE_RELEASE = "RELEASE";


    /** 買家手動確認 */
    public static final String CONFIRM_TYPE_MANUAL = "MANUAL";

    /** 物流排程自動確認 */
    public static final String CONFIRM_TYPE_AUTO = "AUTO";
}
