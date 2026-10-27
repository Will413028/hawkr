package io.github.will413028.hawkr.order.constant;

import io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus;
import io.github.will413028.hawkr.common.utils.code.StockTxPhase;
import io.github.will413028.hawkr.common.utils.code.StockTxStatus;

/**
 * 訂單服務常數
 */
public final class OrderConstants {

    private OrderConstants() {}

    // ==================== order_no 前綴 ====================

    /**
     * 訂單編號前綴
     */
    public static final String ORDER_NO_PREFIX = "ORD";

    /**
     * 退款申請編號前綴
     */
    public static final String REFUND_REQUEST_NO_PREFIX = "RFD";

    // ==================== Redis Key ====================

    /**
     * order_no 每日流水號 Redis Key 前綴
     * 完整 Key: order:seq:{yyyyMMdd}
     */
    public static final String REDIS_KEY_ORDER_SEQ = "order:seq:";

    /**
     * order_no 每日流水號 TTL（秒），2 天確保跨日安全
     */
    public static final long REDIS_KEY_ORDER_SEQ_TTL_SECONDS = 172800L;

    // ==================== 退款外部參考 ID 前綴 ====================

    /**
     * 用戶退款外部參考 ID 前綴
     */
    public static final String REFUND_EXTERNAL_ID_PREFIX = "REFUND_";

    /**
     * 自動退款外部參考 ID 前綴
     */
    public static final String AUTO_REFUND_EXTERNAL_ID_PREFIX = "AUTO_REFUND_";

    // ==================== 任務狀態 ====================

    // WorkerTaskStatus 對應常數（Phase B 完成：DB 與程式碼皆使用 value）
    public static final String TASK_STATUS_PENDING = WorkerTaskStatus.PENDING.value();
    public static final String TASK_STATUS_PROCESSING = WorkerTaskStatus.PROCESSING.value();
    public static final String TASK_STATUS_DONE = WorkerTaskStatus.DONE.value();
    public static final String TASK_STATUS_FAILED = WorkerTaskStatus.FAILED.value();
    public static final String TASK_STATUS_MANUAL_INTERVENTION = WorkerTaskStatus.MANUAL_INTERVENTION.value();
    public static final String TASK_STATUS_AWAITING_BUYER_REFUND_PAGE = WorkerTaskStatus.AWAITING_BUYER_REFUND_PAGE.value();
    public static final String TASK_STATUS_PROVIDER_AUTO = WorkerTaskStatus.PROVIDER_AUTO.value();

    // ==================== 撥款 gRPC 結構化結果（proto GrantPaymentResponse.grant_outcome） ====================

    /** PayUni 已對退款交易接管撥款（payment-service §5.6.1）→ task 收斂 PROVIDER_AUTO（03 §3.11.2） */
    public static final String GRANT_OUTCOME_PROVIDER_TOOK_OVER = "PROVIDER_TOOK_OVER";

    // ==================== 庫存交易協調 ====================

    public static final String STOCK_TX_PHASE_DEDUCT = StockTxPhase.DEDUCT.value();
    public static final String STOCK_TX_PHASE_RESTORE = StockTxPhase.RESTORE.value();

    public static final String STOCK_TX_STATUS_INIT = StockTxStatus.INIT.value();
    public static final String STOCK_TX_STATUS_SUCCESS = StockTxStatus.SUCCESS.value();
    public static final String STOCK_TX_STATUS_FAILED = StockTxStatus.FAILED.value();
    public static final String STOCK_TX_STATUS_MANUAL_INTERVENTION = StockTxStatus.MANUAL_INTERVENTION.value();
    public static final String STOCK_TX_STATUS_COMPENSATED = StockTxStatus.COMPENSATED.value();

    // ==================== 預設重試 ====================

    public static final int DEFAULT_MAX_RETRY_COUNT = 10;

    // ==================== 確認收貨方式 ====================

    /** 買家手動確認 */
    public static final String CONFIRM_TYPE_MANUAL = "MANUAL";

    /** 物流排程自動確認 */
    public static final String CONFIRM_TYPE_AUTO = "AUTO";

    /**
     * 🔴 系統判定觸發的撥款，<b>不是收貨確認</b>（03 §5.7.3）。
     *
     * <p>用於 {@code grant_payment_tasks.confirm_type}：履約責任判定 financially final ＝
     * {@code BUYER_CAUSED} ⇒ {@code SHIPPING_ABNORMAL(35)} →
     * {@code DELIVERY_FAILED_NO_REFUND(54)} 時建立的撥款任務。
     *
     * <p>🔴 <b>為什麼不沿用 {@code AUTO}</b>：{@code MANUAL} 與 {@code AUTO} 的語意都是
     * 「買家收到貨了」（前者買家自己按、後者倒數到期視同收貨）。而這條路徑上
     * <b>買家從頭到尾沒有收到貨</b>——{@code 54} 不經過 {@code DELIVERED(40)}。
     * 借用 {@code AUTO} 會讓稽核看到一筆「排程自動確認收貨」的紀錄，
     * 那是一件<b>沒有發生過的事</b>（同 03 §11.1「命名讓人誤信」的教訓）。
     *
     * <p>⚠️ 值長度 6 ≤ {@code VARCHAR(10)}；該欄無 CHECK 約束。
     * 實查 {@code grant_payment_tasks.confirm_type} <b>零讀取端</b>（純稽核欄，
     * 全 repo 只有 INSERT 時的 setter）⇒ 新增第三個值不會改變任何分支行為。
     * 🔴 日後若有人開始依它分流，必須把這三個值一併納入判斷。
     */
    public static final String CONFIRM_TYPE_SYSTEM = "SYSTEM";
}
