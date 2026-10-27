package io.github.will413028.hawkr.aftersales.config;

import lombok.Data;
import org.springframework.boot.context.properties.ConfigurationProperties;

@Data
@ConfigurationProperties("aftersales")
public class AfterSalesProperties {

    private CaseConfig caseConfig = new CaseConfig();

    private ReturnConfig returnConfig = new ReturnConfig();

    private RefundConfig refund = new RefundConfig();

    private AtmRefundConfig atmRefund = new AtmRefundConfig();

    private AppealConfig appeal = new AppealConfig();

    @Data
    public static class CaseConfig {
        /** 申請售後截止時間（小時） */
        private int applyDeadlineHours = 60;
        /** §5.10 賣家處理期限（小時）；逾時由 CaseProcessTimeoutWorker 依 timeoutPolicy 代為決策 */
        private int processDeadlineHours = 72;
        /** §5.10 逾時策略：AUTO_REJECT（預設）= 系統代為拒絕；AUTO_ESCALATE（備援）= 升級 admin 仲裁 */
        private TimeoutPolicy timeoutPolicy = TimeoutPolicy.AUTO_REJECT;
        /** 最大舉證數量 */
        private int maxEvidenceCount = 3;
    }

    /** §5.10 賣家處理期限逾時策略 */
    public enum TimeoutPolicy {
        /** 系統代為拒絕（reject_reason=SELLER_NO_RESPONSE，actor=SYSTEM） */
        AUTO_REJECT,
        /** 升級至 admin 仲裁（appeal_state=APPEALED，auto_escalated=true） */
        AUTO_ESCALATE
    }

    @Data
    public static class ReturnConfig {
        /** §5.3.8 買家寄回退貨逾時天數（status=20 進入起算；2026-06-15 政策修訂從 30 → 7）*/
        private int timeoutDays = 7;
        /** §5.3.8 ReturnTimeoutWorker cron 表達式（2026-06-15 從每小時改為 10 分鐘，對齊 31/32 cadence）*/
        private String timeoutSchedulerCron = "0 */10 * * * ?";
        /** §5.3.10.1 自動確認收貨小時數（status=27 進入後 N 小時 scheduler 自動 confirm） */
        private int autoConfirmReceiptHours = 48;
        /** §5.3.10.1 AutoConfirmReceiptScheduler cron 表達式 */
        private String autoConfirmSchedulerCron = "0 */10 * * * ?";
        /** §5.3.10.2 超商待取逾時小時數（status=26 進入後 N 小時 scheduler 自動完成）。2026-05-22 新增 */
        private int storePickupTimeoutHours = 72;
        /** §5.3.10.2 StorePickupTimeoutScheduler cron 表達式 */
        private String storePickupSchedulerCron = "0 */10 * * * ?";
        /** §5.3.10.3 ReturnTwentyFourHourWarnScheduler cron 表達式（每 5 分鐘掃 20/26/27 + 24h 截止視窗）*/
        private String warnSchedulerCron = "0 */5 * * * ?";
    }

    @Data
    public static class RefundConfig {
        /** 退款 worker 輪詢間隔（毫秒） */
        private long workerIntervalMs = 10000;
        /** 最大重試次數 */
        private int maxRetryCount = 10;
    }

    @Data
    public static class AtmRefundConfig {
        /** ATM 退款完成導回 URL（完整路徑，如 https://test-api.example.com/atm-refund/return） */
        private String returnUrl;
    }

    /** §10 申訴與平台裁決參數 */
    @Data
    public static class AppealConfig {
        /** 申訴期限（小時），從最後一次 rejected_at 起算 */
        private int deadlineHours = 72;
        /**
         * backend#3262 撥款閘門的安全餘裕（分鐘）：撥款 hold 到「申訴截止 + 本值」。
         * <p>用途<b>不是</b>延長買家的申訴權，而是關掉一個 TOCTOU：
         * {@code submitAppeal} 在 A3 通過（{@code now <= deadline}）之後還要跑完整個交易
         * （含 media gRPC）才 commit，期間撥款閘門若剛好讀到「窗已過」就會誤放行。
         * 本值只要大於該交易最壞耗時 ＋ pod 間時鐘偏移，誤放行即不可能發生。
         * 15 分鐘相對於秒級 gRPC deadline 與毫秒級 NTP 偏移有 2~3 個數量級餘裕。
         */
        private int payoutHoldGraceMinutes = 15;
        /** 申訴最大證據照片數 */
        private int maxEvidenceCount = 5;
        /** Admin 列表預設每頁筆數 */
        private int defaultPageSize = 20;
        /** Admin 列表最大每頁筆數 */
        private int maxPageSize = 100;
    }
}
