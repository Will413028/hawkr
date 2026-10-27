package io.github.will413028.hawkr.payment.infrastructure.persistence.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.RefundTransactionEntity;
import org.apache.ibatis.annotations.*;

import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;
import io.github.will413028.hawkr.payment.infrastructure.persistence.dto.ProviderStatusRow;

/**
 * 退款交易 Mapper
 */
@Mapper
public interface RefundTransactionMapper extends BaseMapper<RefundTransactionEntity> {

    /**
     * 依退款編號查詢
     */
    @Select("SELECT * FROM payment_refunds WHERE refund_no = #{refundNo}")
    RefundTransactionEntity findByRefundNo(@Param("refundNo") String refundNo);

    /**
     * 依原交易編號查詢所有退款
     */
    @Select("SELECT * FROM payment_refunds WHERE trade_no = #{tradeNo} ORDER BY created_at")
    List<RefundTransactionEntity> findByTradeNo(@Param("tradeNo") String tradeNo);

    /**
     * §5.5.5 L2 可退餘額用累計：計算已「用掉」的退款額度（在途 + 成功）。
     * <p>
     * status IN ('0','1','2') = REQUESTED + PROCESSING + SUCCEEDED。可退餘額 = 交易金額 − 此 SUM。
     * 刻意含在途（REQUESTED/PROCESSING）：否則尚未終態的退款不計入已退累計，併發時各自誤判仍有餘額
     * 而超額退款。與 {@link #hasProcessingRefund}（僅 '0','1'，職責為「是否有未終結申請」）狀態集合刻意不同。
     */
    @Select("""
            SELECT COALESCE(SUM(amount), 0)
            FROM payment_refunds
            WHERE trade_no = #{tradeNo}
              AND status IN ('0', '1', '2')
            """)
    long sumRefundedAmount(@Param("tradeNo") String tradeNo);

    /**
     * §5.5.5 L2「排除本筆」版本：同 {@link #sumRefundedAmount}（在途 + 成功 status IN ('0','1','2')），
     * 但排除 refund_no = #{excludeRefundNo} 的本筆退款。
     * <p>
     * 給「本筆退款已預建於表中」的 caller（如 getRefundPageUrl：ATM 退款先預建一筆 REQUESTED('0') row，
     * 買家取退款頁時本筆已存在）。若沿用不排除版，會把本筆自己算進已退累計，導致全額 / 大額 ATM 退款
     * remaining=0 而誤報 REFUND_AMOUNT_EXCEEDED，買家永遠拿不到退款頁。語意 = 「其他退款已用掉的額度」。
     * excludeRefundNo 為 null 時退化為含全部（等同 {@link #sumRefundedAmount}）。
     */
    @Select("""
            SELECT COALESCE(SUM(amount), 0)
            FROM payment_refunds
            WHERE trade_no = #{tradeNo}
              AND status IN ('0', '1', '2')
              AND (#{excludeRefundNo,jdbcType=VARCHAR} IS NULL
                   OR refund_no <> #{excludeRefundNo,jdbcType=VARCHAR})
            """)
    long sumRefundedAmountByTradeNoExcluding(@Param("tradeNo") String tradeNo,
                                             @Param("excludeRefundNo") String excludeRefundNo);

    /**
     * §5.5.5 L2 未終結申請檢查：同 trade_no 是否已有一筆尚未終結的退款申請。
     * <p>
     * status IN ('0','1') = REQUESTED + PROCESSING，<b>絕不含 SUCCEEDED('2')</b>：本服務支援同一交易的
     * 多次 / 部分退款，若把 SUCCEEDED 也算「處理中」，交易一旦有一筆退款成功此值恆為 true、後續所有部分
     * 退款被永久擋死。已成功退款的額度佔用由 {@link #sumRefundedAmount}（含 '2'）負責，兩者職責嚴格分離。
     */
    @Select("""
            SELECT EXISTS(
                SELECT 1 FROM payment_refunds
                WHERE trade_no = #{tradeNo}
                  AND status IN ('0', '1')
            )
            """)
    boolean hasProcessingRefund(@Param("tradeNo") String tradeNo);

    /**
     * §5.6.1 撥款接管主判準：同 trade_no 是否存在已完成退款（僅終態 SUCCEEDED，值由 caller 帶
     * {@code RefundStatus.SUCCEEDED.value()}）。刻意不含在途 '0'/'1'：接管的領域事實以退款完成為準。
     */
    @Select("""
            SELECT EXISTS(
                SELECT 1 FROM payment_refunds
                WHERE trade_no = #{tradeNo}
                  AND status = #{succeeded}
            )
            """)
    boolean hasSucceededRefund(@Param("tradeNo") String tradeNo,
                               @Param("succeeded") String succeeded);

    /**
     * 條件更新為處理中（僅當狀態為 REQUESTED 時）
     */
    @Update("""
            UPDATE payment_refunds
            SET status = '1',
                version = version + 1
            WHERE refund_no = #{refundNo}
              AND status = '0'
              AND version = #{version}
            """)
    int updateToProcessing(
            @Param("refundNo") String refundNo,
            @Param("version") int version
    );

    /**
     * 條件更新為成功（僅當狀態為 REQUESTED 或 PROCESSING 時）。
     * 收斂規範 §3.4：bank_date = ATM webhook RefundStatus=1 帶回的實際到帳日；信用卡 sync 為 null。
     */
    @Update("""
            UPDATE payment_refunds
            SET status = '2',
                provider_refund_no = COALESCE(#{providerRefundNo}, provider_refund_no),
                provider_status = #{providerStatus},
                provider_message = #{providerMessage},
                bank_date = COALESCE(#{bankDate}, bank_date),
                completed_at = #{completedAt},
                version = version + 1
            WHERE refund_no = #{refundNo}
              AND status IN ('0', '1')
              AND version = #{version}
            """)
    int updateToSucceeded(
            @Param("refundNo") String refundNo,
            @Param("providerRefundNo") String providerRefundNo,
            @Param("providerStatus") String providerStatus,
            @Param("providerMessage") String providerMessage,
            @Param("bankDate") LocalDate bankDate,
            @Param("completedAt") LocalDateTime completedAt,
            @Param("version") int version
    );

    /**
     * 依 PayUni 退款編號查詢
     */
    @Select("SELECT * FROM payment_refunds WHERE provider_refund_no = #{providerRefundNo}")
    RefundTransactionEntity findByProviderRefundNo(@Param("providerRefundNo") String providerRefundNo);

    /**
     * 依冪等鍵查詢退款記錄
     */
    @Select("SELECT * FROM payment_refunds WHERE idempotency_key = #{idempotencyKey}")
    RefundTransactionEntity findByIdempotencyKey(@Param("idempotencyKey") String idempotencyKey);

    /**
     * 條件更新為失敗（僅當狀態為 REQUESTED 或 PROCESSING 時）
     */
    @Update("""
            UPDATE payment_refunds
            SET status = '3',
                provider_status = #{providerStatus},
                provider_message = #{providerMessage},
                version = version + 1
            WHERE refund_no = #{refundNo}
              AND status IN ('0', '1')
              AND version = #{version}
            """)
    int updateToFailed(
            @Param("refundNo") String refundNo,
            @Param("providerStatus") String providerStatus,
            @Param("providerMessage") String providerMessage,
            @Param("version") int version
    );

    /**
     * ATM sync（頁面版受理）：REQUESTED → PROCESSING + 寫入 estimated_refund_date。
     * 依收斂規範 §3.4 里程碑，estimated_refund_date 為 PayUni 回報的預計扣款日（可 null）。
     * 不檢 version（頁面版回調路徑可能重試，冪等由狀態機約束）。
     */
    @Update("""
            UPDATE payment_refunds
            SET status = '1',
                provider_status = #{providerStatus},
                provider_message = #{providerMessage},
                estimated_refund_date = COALESCE(#{estimatedRefundDate}, estimated_refund_date),
                version = version + 1
            WHERE refund_no = #{refundNo}
              AND status = '0'
            """)
    int updateToProcessingWithEstimate(
            @Param("refundNo") String refundNo,
            @Param("providerStatus") String providerStatus,
            @Param("providerMessage") String providerMessage,
            @Param("estimatedRefundDate") LocalDate estimatedRefundDate
    );

    /**
     * 取該 trade_no 最近一筆非終態的退款（狀態為 REQUESTED 或 PROCESSING）。
     * ATM async webhook 以此方法定位對應 payment_refunds row。
     */
    @Select("""
            SELECT * FROM payment_refunds
            WHERE trade_no = #{tradeNo}
              AND status IN ('0', '1')
            ORDER BY created_at DESC
            LIMIT 1
            """)
    RefundTransactionEntity findActiveByTradeNo(@Param("tradeNo") String tradeNo);

    /**
     * 取該 trade_no 最新一筆退款（不限狀態）。
     * ATM webhook finalize 找不到 active row 時，以此判定最新退款終態（SUCCEEDED / FAILED），
     * 決定冪等略過、衝突校正或告警孤兒（05 §5.5.4.1）。
     */
    @Select("""
            SELECT * FROM payment_refunds
            WHERE trade_no = #{tradeNo}
            ORDER BY created_at DESC
            LIMIT 1
            """)
    RefundTransactionEntity findLatestByTradeNo(@Param("tradeNo") String tradeNo);

    /**
     * ATM webhook finalize → SUCCEEDED（狀態機冪等，無 version 條件）。
     * webhook 帶回外部權威終態，套用不能因本地 version 競態失敗；比照 updateToProcessingWithEstimate
     * 的既定 pattern（回調可能重試，冪等由狀態機約束）。只要退款尚未終態必套用，已終態則 WHERE 不命中天然冪等。
     * 與信用卡同步路徑共用的 updateToSucceeded（帶 version 守衛）分離，兩路徑不共用同一守衛。
     */
    @Update("""
            UPDATE payment_refunds
            SET status = '2',
                provider_refund_no = COALESCE(#{providerRefundNo}, provider_refund_no),
                provider_status = #{providerStatus},
                provider_message = #{providerMessage},
                bank_date = COALESCE(#{bankDate}, bank_date),
                completed_at = #{completedAt},
                version = version + 1
            WHERE refund_no = #{refundNo}
              AND status IN ('0', '1')
            """)
    int finalizeAtmRefundSucceeded(
            @Param("refundNo") String refundNo,
            @Param("providerRefundNo") String providerRefundNo,
            @Param("providerStatus") String providerStatus,
            @Param("providerMessage") String providerMessage,
            @Param("bankDate") LocalDate bankDate,
            @Param("completedAt") LocalDateTime completedAt
    );

    /**
     * ATM webhook finalize → FAILED（狀態機冪等，無 version 條件）。理由同 finalizeAtmRefundSucceeded。
     */
    @Update("""
            UPDATE payment_refunds
            SET status = '3',
                provider_status = #{providerStatus},
                provider_message = #{providerMessage},
                version = version + 1
            WHERE refund_no = #{refundNo}
              AND status IN ('0', '1')
            """)
    int finalizeAtmRefundFailed(
            @Param("refundNo") String refundNo,
            @Param("providerStatus") String providerStatus,
            @Param("providerMessage") String providerMessage
    );

    /**
     * ATM webhook 狀態衝突校正 → 從 FAILED 校正為 SUCCEEDED（狀態機冪等，無 version 條件）。
     * 專用於「我方誤標 FAILED、PayUni webhook 回成功」的校正路徑：finalizeAtmRefundSucceeded 的
     * WHERE status IN('0','1') 不含 '3'，對已 FAILED 的 row 命中 0 rows（校正靜默 no-op），故另立此方法。
     * WHERE status = '3'：僅從 FAILED 校正，重入時 status 已 '2' → 0 rows 天然冪等，不會把非 FAILED 的 row 誤改。
     */
    @Update("""
            UPDATE payment_refunds
            SET status = '2',
                provider_refund_no = COALESCE(#{providerRefundNo}, provider_refund_no),
                provider_status = #{providerStatus},
                provider_message = #{providerMessage},
                bank_date = COALESCE(#{bankDate}, bank_date),
                completed_at = #{completedAt},
                version = version + 1
            WHERE refund_no = #{refundNo}
              AND status = '3'
            """)
    int correctFailedToSucceeded(
            @Param("refundNo") String refundNo,
            @Param("providerRefundNo") String providerRefundNo,
            @Param("providerStatus") String providerStatus,
            @Param("providerMessage") String providerMessage,
            @Param("bankDate") LocalDate bankDate,
            @Param("completedAt") LocalDateTime completedAt
    );

    /**
     * 批次查詢：以 case_ids（= idempotency_key）查每個 case 最新一筆 payment_refunds 的 provider_* 欄位。
     * 使用 DISTINCT ON 取每 idempotency_key 最新一筆；收斂規範 §6.3。
     */
    @Select({"<script>",
            "SELECT DISTINCT ON (idempotency_key) idempotency_key AS key, provider_status, provider_message",
            "FROM payment_refunds",
            "WHERE idempotency_key IN",
            "<foreach collection='caseIds' item='cid' open='(' separator=',' close=')'>#{cid}</foreach>",
            "ORDER BY idempotency_key, created_at DESC",
            "</script>"})
    List<ProviderStatusRow> findProviderStatusByCaseIds(@Param("caseIds") List<String> caseIds);

    /**
     * 批次查詢：以 order_ids 查每個 order 最新一筆 payment_refunds 的 provider_* 欄位。
     * 收斂規範 §6.3（供 Order 聚合使用）。
     * 回傳 Map&lt;String, String&gt;：避免全域 JsonbMapTypeHandler 把每個欄位當 JSONB 解析。
     */
    @Select({"<script>",
            "SELECT DISTINCT ON (order_id) order_id AS key, provider_status, provider_message",
            "FROM payment_refunds",
            "WHERE order_id IN",
            "<foreach collection='orderIds' item='oid' open='(' separator=',' close=')'>#{oid}</foreach>",
            "ORDER BY order_id, created_at DESC",
            "</script>"})
    List<ProviderStatusRow> findProviderStatusByOrderIds(@Param("orderIds") List<String> orderIds);
}
