package io.github.will413028.hawkr.order.config;

import lombok.Data;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.stereotype.Component;

/**
 * 訂單業務配置
 */
@Data
@Component
@ConfigurationProperties(prefix = "hawkr.order")
public class OrderConfig {

    /**
     * 付款期限（秒），預設 86400（24 小時）
     */
    private long paymentDeadlineSeconds = 86400;

    /**
     * RFQ 成交建單的付款期限（秒），預設 43200（12 小時）。
     *
     * <p>64 號 §6.4.11 逐字指定「建單 +12h」，與結帳路徑的 24 小時<b>刻意不同</b>：
     * RFQ 的報價有 {@code valid_until}，賣家把貨留給這筆成交的時間遠短於一般商品。
     * ⛔ 不得改讀 {@link #paymentDeadlineSeconds} 省一個欄位——那會讓兩條路徑的產品
     * 決策綁在一起，而它們的依據不同。
     */
    private long rfqPaymentDeadlineSeconds = 43200;

    /**
     * backend#3262 撥款 hold 的重問間隔（秒），預設 600（10 分鐘）。
     *
     * <p>用在<b>無時間界</b>的 hold（售後案件進行中 / 申訴審理中）與 aftersales 反查失敗
     * （fail-closed）兩種情形：這兩者沒有可預期的解除時點，只能定期重問。
     * 有時間界的 hold 直接推到 {@code hold_until}，不吃這個值。
     *
     * <p>取 10 分鐘的理由：解除時點由人（Admin 裁決）或退款管線決定，分鐘級延遲對賣家無感，
     * 而 30 秒級的重問會在一筆申訴審理數天期間打出上萬次無意義的跨服務呼叫。
     */
    private long payoutHoldRecheckSeconds = 600;

    /**
     * 付款超時掃描間隔（毫秒），預設 15000（15 秒）
     */
    private long cancelScanIntervalMs = 15000;

    /**
     * order.paid Outbox 輪詢間隔（毫秒），預設 5000（03 §8.10）
     */
    private long outboxPublishIntervalMs = 5000;
}
