package io.github.will413028.hawkr.payment.infrastructure.persistence.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.PaymentTransactionEntity;
import org.apache.ibatis.annotations.*;

import java.time.LocalDateTime;
import java.util.List;
import io.github.will413028.hawkr.payment.infrastructure.persistence.dto.ProviderStatusRow;

/**
 * 支付交易 Mapper
 */
@Mapper
public interface PaymentTransactionMapper extends BaseMapper<PaymentTransactionEntity> {

    /**
     * 依交易編號查詢（單筆假設）。
     *
     * @deprecated §2A.6 之後同 trade_no 可有多 row（CLOSED_BY_SWITCH / 重複付款副 row /
     * reactivated 主 + rescue 副），此 method 在多 row 情境下會 throw
     * {@code TooManyResultsException} 炸 500。請改用 {@link #findCanonicalByTradeNo}
     * 取「目前生效的主 row」，或在需要特定 row 時改以 id / provider_trade_no 查找。
     * 僅供確定 1 trade_no = 1 row 的內部路徑使用（未來逐步移除）。
     */
    @Deprecated
    @Select("SELECT * FROM payment_transactions WHERE trade_no = #{tradeNo}")
    PaymentTransactionEntity findByTradeNo(@Param("tradeNo") String tradeNo);

    /**
     * §2A.6 之後對外查詢的標準入口：取同 trade_no 的「目前生效的主 row」。
     * <p>
     * 排序語意：
     * <ol>
     *   <li>{@code is_duplicate ASC} — 主 row（FALSE=0）優先於副 row（TRUE=1）</li>
     *   <li>{@code created_at DESC} — 同層級取最新（reactivated 後新主在前）</li>
     * </ol>
     * 任何走過 §2A.x（切換 / 重複付款 / ATM 復活）的 trade_no 必須走此 method，
     * 否則會踩到 {@link #findByTradeNo} 多 row 炸 500。
     */
    @Select("""
            SELECT * FROM payment_transactions
            WHERE trade_no = #{tradeNo}
            ORDER BY is_duplicate ASC, created_at DESC
            LIMIT 1
            """)
    PaymentTransactionEntity findCanonicalByTradeNo(@Param("tradeNo") String tradeNo);

    /**
     * §5.5.5 L1 退款併發行鎖：取同 trade_no 的 canonical 主 row 並 {@code FOR UPDATE} 鎖住。
     * <p>
     * 供 {@code RefundApplicationService.createRefund} 在同一事務內先鎖交易 row，序列化同交易的
     * 併發退款建立（第二個 caller 阻塞至前一筆 commit 釋鎖，避免併發讀到過期「已退款累計」而超額退款）。
     * 排序語意與 {@link #findCanonicalByTradeNo} 相同（is_duplicate ASC, created_at DESC LIMIT 1），
     * {@code FOR UPDATE} 僅鎖返回的該筆。
     */
    @Select("""
            SELECT * FROM payment_transactions
            WHERE trade_no = #{tradeNo}
            ORDER BY is_duplicate ASC, created_at DESC
            LIMIT 1
            FOR UPDATE
            """)
    PaymentTransactionEntity findCanonicalByTradeNoForUpdate(@Param("tradeNo") String tradeNo);

    /**
     * 依訂單編號查詢
     */
    @Select("SELECT * FROM payment_transactions WHERE order_id = #{orderId}")
    List<PaymentTransactionEntity> findByOrderId(@Param("orderId") String orderId);

    /**
     * 條件更新為已付款（僅當狀態為 INIT 時）
     *
     * @return 更新的筆數（0 表示條件不符）
     */
    @Update("""
            UPDATE payment_transactions
            SET status = '1',
                provider_trade_no = #{providerTradeNo},
                provider_status = #{providerStatus},
                provider_message = #{providerMessage},
                pay_time = #{payTime},
                version = version + 1
            WHERE trade_no = #{tradeNo}
              AND status = '0'
              AND version = #{version}
            """)
    int updateToPaid(
            @Param("tradeNo") String tradeNo,
            @Param("providerTradeNo") String providerTradeNo,
            @Param("providerStatus") String providerStatus,
            @Param("providerMessage") String providerMessage,
            @Param("payTime") LocalDateTime payTime,
            @Param("version") int version
    );

    /**
     * 條件更新為失敗（僅當狀態為 INIT 時）。
     * 寫回 provider_trade_no 供後續對帳（若 webhook 帶有 TradeNo）。
     */
    @Update("""
            UPDATE payment_transactions
            SET status = '2',
                provider_status = #{providerStatus},
                provider_message = #{providerMessage},
                provider_trade_no = COALESCE(#{providerTradeNo}, provider_trade_no),
                version = version + 1
            WHERE trade_no = #{tradeNo}
              AND status = '0'
              AND version = #{version}
            """)
    int updateToFailed(
            @Param("tradeNo") String tradeNo,
            @Param("providerStatus") String providerStatus,
            @Param("providerMessage") String providerMessage,
            @Param("providerTradeNo") String providerTradeNo,
            @Param("version") int version
    );

    /**
     * 條件更新為逾期（僅當狀態為 INIT 且已過期時）
     */
    @Update("""
            UPDATE payment_transactions
            SET status = '3',
                version = version + 1
            WHERE trade_no = #{tradeNo}
              AND status = '0'
              AND expire_at < #{now}
              AND version = #{version}
            """)
    int updateToExpired(
            @Param("tradeNo") String tradeNo,
            @Param("now") LocalDateTime now,
            @Param("version") int version
    );

    /**
     * 條件更新為衝突（僅當現有狀態為 PAID 或 FAILED 時）
     *
     * 適用場景：Apple Pay / Google Pay 同步 API 寫入終態後，
     * Webhook 帶來矛盾結果（例如 DB=PAID 但 Webhook=FAILED）
     */
    @Update("""
            UPDATE payment_transactions
            SET status = '4',
                provider_status = #{providerStatus},
                provider_message = #{providerMessage},
                version = version + 1
            WHERE trade_no = #{tradeNo}
              AND status IN ('1', '2')
              AND version = #{version}
            """)
    int updateToConflict(
            @Param("tradeNo") String tradeNo,
            @Param("providerStatus") String providerStatus,
            @Param("providerMessage") String providerMessage,
            @Param("version") int version
    );

    /**
     * 條件更新為關閉（僅當狀態為 INIT 時）
     */
    @Update("""
            UPDATE payment_transactions
            SET status = '5',
                closed_at = #{closedAt},
                version = version + 1
            WHERE trade_no = #{tradeNo}
              AND status = '0'
              AND version = #{version}
            """)
    int updateToClosed(
            @Param("tradeNo") String tradeNo,
            @Param("closedAt") LocalDateTime closedAt,
            @Param("version") int version
    );

    /**
     * 更新 ATM 資訊。
     * §2A.3.6 修正：寫入 provider_trade_no（PayUni /api/atm 同步回應的 TradeNo），
     * 否則 §2A.6 webhook 段 1 findByProviderTradeNo 會找不到，破壞三段查找。
     */
    @Update("""
            UPDATE payment_transactions
            SET atm_bank_code = #{atmBankCode},
                atm_virtual_account = #{atmVirtualAccount},
                atm_expire_date = #{atmExpireDate},
                expire_at = #{expireAt},
                provider_trade_no = #{providerTradeNo},
                version = version + 1
            WHERE trade_no = #{tradeNo}
              AND status = '0'
              AND version = #{version}
            """)
    int updateAtmInfo(
            @Param("tradeNo") String tradeNo,
            @Param("providerTradeNo") String providerTradeNo,
            @Param("atmBankCode") String atmBankCode,
            @Param("atmVirtualAccount") String atmVirtualAccount,
            @Param("atmExpireDate") java.time.LocalDate atmExpireDate,
            @Param("expireAt") LocalDateTime expireAt,
            @Param("version") int version
    );

    /**
     * 查詢訂單的活躍交易（INIT 或 PAID），用於冪等性檢查
     * 防止同一訂單重複建立支付交易
     */
    @Select("""
            SELECT * FROM payment_transactions
            WHERE order_id = #{orderId}
              AND status IN ('0', '1')
            ORDER BY created_at DESC
            LIMIT 1
            """)
    PaymentTransactionEntity findActiveByOrderId(@Param("orderId") String orderId);

    /**
     * §5.6 撥款併發行鎖：取同 order_id 的 canonical 主 PAID row 並 {@code FOR UPDATE} 鎖住。
     * <p>
     * 供 {@code PaymentApplicationService.grantPayment} 在同一事務內先鎖交易 row，序列化同交易的
     * 併發 / 重試撥款；鎖持有至事務結束（涵蓋 PayUni Grant 呼叫與 grant_* 寫入），使第二次撥款取鎖後
     * 看到 grant_status='SUCCESS' 而短路，杜絕時序型重撥。過濾 is_duplicate=FALSE 取主 row，LIMIT 1
     * 避免重複付款副 row 造成多筆。
     */
    @Select("""
            SELECT * FROM payment_transactions
            WHERE order_id = #{orderId}
              AND status = '1'
              AND is_duplicate = FALSE
            ORDER BY created_at DESC
            LIMIT 1
            FOR UPDATE
            """)
    PaymentTransactionEntity findActiveByOrderIdForUpdate(@Param("orderId") String orderId);

    /**
     * 58 號 §4.1「05 支付」：取訂單的 canonical 已付款交易（唯讀，供 {@code GetPaymentFactsInternal}）。
     * <p>
     * 述詞與 {@link #findActiveByOrderIdForUpdate} 相同但<b>不加 {@code FOR UPDATE}</b>——
     * 發票側只讀不寫，加鎖會讓唯讀 RPC 排在退款／撥款事務後面，且無任何寫入需要被序列化。
     * <p>
     * {@code is_duplicate = FALSE} 是契約的一部分而非最佳化：重複付款副 row 不開票也不折讓
     * （58 號 §7.1.2 註 3），選錯 row 會讓發票金額與 {@code orders} 永久對不上。
     */
    @Select("""
            SELECT * FROM payment_transactions
            WHERE order_id = #{orderId}
              AND status = #{paidStatus}
              AND is_duplicate = FALSE
            ORDER BY created_at DESC
            LIMIT 1
            """)
    PaymentTransactionEntity findCanonicalPaidByOrderId(@Param("orderId") String orderId,
                                                        @Param("paidStatus") String paidStatus);

    /**
     * §2.2 業務 pivot (2026-05-14)：UnknownPaymentFallbackConsumer 用 MerID 找最近一筆等待付款的 transaction。
     * <p>過濾 {@code is_duplicate=false} 避免抓到 §2A.6 重複付款保留 row。</p>
     */
    @Select("""
            SELECT * FROM payment_transactions
            WHERE mer_id = #{merId}
              AND status = '0'
              AND is_duplicate = false
            ORDER BY updated_at DESC
            LIMIT 1
            """)
    PaymentTransactionEntity findLatestPendingByMerId(@Param("merId") String merId);

    /**
     * §2.2 業務 pivot (2026-05-14)：UnknownPaymentFallbackConsumer stamp provider_message。
     * <p>樂觀鎖 {@code WHERE provider_message IS NULL}：避免覆蓋 PendingPaymentReconcileService 已寫入的訊息。</p>
     */
    @org.apache.ibatis.annotations.Update("""
            UPDATE payment_transactions
            SET provider_message = #{providerMessage},
                updated_at = NOW()
            WHERE id = #{id}
              AND status = '0'
              AND provider_message IS NULL
            """)
    int stampUnknownFallbackMessage(@Param("id") Long id,
                                    @Param("providerMessage") String providerMessage);

    /**
     * §2A.3.6 查詢同 order_id + paymentType 的活躍 row（給切換偵測用）。
     * 同 type 找到 → 走冪等回 cached；找不到再 fallback 到 findActiveByOrderId 看是否要切換。
     */
    @Select("""
            SELECT * FROM payment_transactions
            WHERE order_id = #{orderId}
              AND payment_type = #{paymentType}
              AND status IN ('0', '1')
            ORDER BY created_at DESC
            LIMIT 1
            """)
    PaymentTransactionEntity findActiveByOrderIdAndPaymentType(
            @Param("orderId") String orderId,
            @Param("paymentType") String paymentType);

    /**
     * §2A.4 取得同 order_id 最近一筆交易的 created_at（不分 status，含 CLOSED_BY_SWITCH/EXPIRED）。
     * @deprecated 冷卻機制已移除（MerTradeNo 序列化），改用 findMaxAttemptSeqByOrderId。
     */
    @Deprecated
    @Select("""
            SELECT MAX(created_at) FROM payment_transactions
            WHERE order_id = #{orderId}
            """)
    LocalDateTime findMaxCreatedAtByOrderId(@Param("orderId") String orderId);

    /**
     * §2A.3 切換流程：將舊 row 標為 CLOSED_BY_SWITCH。
     * 條件：status IN ('0','1') 且 version 樂觀鎖匹配。
     */
    @org.apache.ibatis.annotations.Update("""
            UPDATE payment_transactions
               SET status = '6',
                   closed_at = #{closedAt},
                   updated_at = #{closedAt},
                   version = version + 1
             WHERE id = #{id}
               AND status IN ('0', '1')
               AND version = #{expectedVersion}
            """)
    int markClosedBySwitch(
            @Param("id") Long id,
            @Param("closedAt") LocalDateTime closedAt,
            @Param("expectedVersion") int expectedVersion);

    /**
     * §2A.5.4 ATM 復活：鎖定同 order_id 既有 INIT row（FOR UPDATE）。
     * 與 findReactivableAtmRowForUpdate 配對使用。
     */
    @Select("""
            SELECT * FROM payment_transactions
            WHERE order_id = #{orderId}
              AND status = '0'
            ORDER BY created_at DESC
            LIMIT 1
            FOR UPDATE
            """)
    PaymentTransactionEntity findActiveInitByOrderIdForUpdate(@Param("orderId") String orderId);

    /**
     * §2A.5.4 ATM 復活：查找同 order_id 可復活的 ATM CLOSED_BY_SWITCH row（FOR UPDATE）。
     * <p>
     * 條件：
     * <ul>
     *   <li>payment_type = '2004' (ATM)</li>
     *   <li>status = '6' (CLOSED_BY_SWITCH)</li>
     *   <li>expire_at > nowUtc（仍未過期）</li>
     *   <li>provider_trade_no IS NOT NULL（已成功取過 PayUni 虛擬帳號）</li>
     * </ul>
     * 若多筆符合 → ORDER BY expire_at DESC LIMIT 1（剩餘時間最長那筆）。
     * <p>
     * 配合外層 REPEATABLE_READ + 鎖既有 INIT row 順序，避免併發 race。
     */
    @Select("""
            SELECT * FROM payment_transactions
            WHERE order_id = #{orderId}
              AND payment_type = '2004'
              AND status = '6'
              AND expire_at > #{nowUtc}
              AND provider_trade_no IS NOT NULL
            ORDER BY expire_at DESC
            LIMIT 1
            FOR UPDATE
            """)
    PaymentTransactionEntity findReactivableAtmRowForUpdate(
            @Param("orderId") String orderId,
            @Param("nowUtc") LocalDateTime nowUtc);

    /**
     * §2A.5.4 ATM 復活：將 CLOSED_BY_SWITCH row 翻回 INIT 並寫入 reactivated_at。
     * <p>
     * 條件：status = '6' (CLOSED_BY_SWITCH)，避免併發復活/EXPIRED scheduler 競爭。
     * 不檢 version：呼叫端已用 FOR UPDATE 行鎖。
     */
    @org.apache.ibatis.annotations.Update("""
            UPDATE payment_transactions
               SET status = '0',
                   reactivated_at = #{nowUtc},
                   updated_at = #{nowUtc},
                   closed_at = NULL,
                   version = version + 1
             WHERE id = #{id}
               AND status = '6'
            """)
    int markReactivated(@Param("id") Long id, @Param("nowUtc") LocalDateTime nowUtc);

    /**
     * §2A.3.6 段 1：依 provider_trade_no 精準查找（webhook handler 用）。
     * provider_trade_no 受 partial unique index 保證唯一。
     */
    @Select("SELECT * FROM payment_transactions WHERE provider_trade_no = #{providerTradeNo}")
    PaymentTransactionEntity findByProviderTradeNo(@Param("providerTradeNo") String providerTradeNo);

    /**
     * §2A.3.6 段 3：列出同 order_id 的所有歷史 row，含 CLOSED_BY_SWITCH。
     */
    @Select("""
            SELECT * FROM payment_transactions
            WHERE order_id = #{orderId}
            ORDER BY created_at DESC
            """)
    List<PaymentTransactionEntity> findHistoryByOrderId(@Param("orderId") String orderId);

    /**
     * §2A.3.6 取得同 order_id 目前最大的 attempt_seq。
     */
    @Select("SELECT MAX(attempt_seq) FROM payment_transactions WHERE order_id = #{orderId}")
    Integer findMaxAttemptSeqByOrderId(@Param("orderId") String orderId);

    /**
     * §2A.6.2 管道 B：找同 order_id 其他已 PAID 且非重複的 row（排除自己）。
     */
    @Select("""
            SELECT * FROM payment_transactions
            WHERE order_id = #{orderId}
              AND id != #{excludeId}
              AND status = '1'
              AND is_duplicate = FALSE
            ORDER BY created_at ASC
            """)
    List<PaymentTransactionEntity> findOtherPaidRowsForSameOrder(
            @Param("orderId") String orderId,
            @Param("excludeId") Long excludeId);

    /**
     * §2A.6 管道 A：鎖定既有「主」PAID row（FOR UPDATE）。
     * 用於 DuplicatePaymentService 的 REPEATABLE_READ 事務內。
     */
    @Select("""
            SELECT * FROM payment_transactions
            WHERE order_id = #{orderId}
              AND status = '1'
              AND is_duplicate = FALSE
            ORDER BY created_at ASC
            LIMIT 1
            FOR UPDATE
            """)
    PaymentTransactionEntity lockExistingPaidByOrderId(@Param("orderId") String orderId);

    /**
     * §2A.6 取得同 order_id 目前最大 duplicate_sequence。
     */
    @Select("""
            SELECT COALESCE(MAX(duplicate_sequence), 0)
            FROM payment_transactions
            WHERE order_id = #{orderId}
            """)
    Integer findMaxDuplicateSequenceByOrderId(@Param("orderId") String orderId);

    /**
     * §2A.6 標記某 row 為重複付款的「副」row。
     */
    @org.apache.ibatis.annotations.Update("""
            UPDATE payment_transactions
               SET is_duplicate = TRUE,
                   duplicate_of_payment_id = #{mainId},
                   duplicate_sequence = #{sequence},
                   updated_at = now()
             WHERE id = #{id}
            """)
    int markAsDuplicate(
            @Param("id") Long id,
            @Param("mainId") Long mainId,
            @Param("sequence") Integer sequence);

    /**
     * §2A.6 翻轉：取消某 row 的 duplicate 標記（重新成為「主」）。
     * <p>
     * 同時清掉 duplicate_sequence（避免新主殘留 stale sequence，污染後續
     * findMaxDuplicateSequenceByOrderId 的 MAX 計算）。is_duplicate=FALSE
     * 是業務權威來源，sequence 與 of_payment_id 在新主上應該為 NULL。
     */
    @org.apache.ibatis.annotations.Update("""
            UPDATE payment_transactions
               SET is_duplicate = FALSE,
                   duplicate_of_payment_id = NULL,
                   duplicate_sequence = NULL,
                   updated_at = now()
             WHERE id = #{id}
            """)
    int unmarkDuplicate(@Param("id") Long id);

    /**
     * §2A.6.5 RescueRefundConsumer 退款後標 row 為 CLOSED（PAID → CLOSED）。
     * 與 updateToClosed 的差異：updateToClosed 是 INIT → CLOSED（人工/排程關閉），
     * 此方法是 PAID → CLOSED（已付款後 rescue 退款完成）。
     * 不檢查 version（因為 row 可能剛被翻轉過 version 已 +N），用 id + status='1' 防誤更新。
     */
    @org.apache.ibatis.annotations.Update("""
            UPDATE payment_transactions
               SET status = '5',
                   closed_at = #{closedAt},
                   updated_at = now() at time zone 'utc',
                   version = version + 1
             WHERE id = #{id}
               AND status = '1'
            """)
    int markRescueRefunded(@Param("id") Long id, @Param("closedAt") LocalDateTime closedAt);

    /**
     * §2A.3.6 段 1.b 「切換後舊管道完成」reactivate 路徑：
     * CLOSED_BY_SWITCH → PAID + 標 duplicate（duplicate_of_payment_id, duplicate_sequence）+ 寫入 provider_paid_at + pay_time。
     * <p>
     * 條件：status='6'（CLOSED_BY_SWITCH）且 row 存在。
     * provider_trade_no 不變（同一筆 PayUni 交易）。
     */
    @org.apache.ibatis.annotations.Update("""
            UPDATE payment_transactions
               SET status = '1',
                   is_duplicate = TRUE,
                   duplicate_of_payment_id = #{mainId},
                   duplicate_sequence = #{sequence},
                   pay_time = COALESCE(pay_time, now() at time zone 'utc'),
                   provider_paid_at = COALESCE(#{providerPaidAt}, provider_paid_at),
                   updated_at = now() at time zone 'utc',
                   version = version + 1
             WHERE id = #{id}
               AND status = '6'
            """)
    int reactivateRowAsDuplicate(
            @Param("id") Long id,
            @Param("mainId") Long mainId,
            @Param("sequence") Integer sequence,
            @Param("providerPaidAt") LocalDateTime providerPaidAt);

    /**
     * §10.5 D7 寫入 PayUni 真實時間軸（webhook handler 解析後呼叫）。
     */
    @org.apache.ibatis.annotations.Update("""
            UPDATE payment_transactions
               SET provider_create_at = COALESCE(#{providerCreateAt}, provider_create_at),
                   provider_paid_at = COALESCE(#{providerPaidAt}, provider_paid_at),
                   updated_at = now()
             WHERE id = #{id}
            """)
    int updateProviderTimes(
            @Param("id") Long id,
            @Param("providerCreateAt") LocalDateTime providerCreateAt,
            @Param("providerPaidAt") LocalDateTime providerPaidAt);

    /**
     * 查詢已逾期但狀態仍為 INIT 的交易
     */
    @Select("""
            SELECT * FROM payment_transactions
            WHERE status = '0'
              AND expire_at IS NOT NULL
              AND expire_at < #{now}
            ORDER BY expire_at
            LIMIT #{limit}
            """)
    List<PaymentTransactionEntity> findExpiredTransactions(
            @Param("now") LocalDateTime now,
            @Param("limit") int limit
    );

    /**
     * L2 補救路徑：撈出 reconcile 候選交易
     *
     * 條件（全部成立）：
     * 1. status IN ('0', '3') — INIT 或 EXPIRED
     * 2. created_at < initialDelayBefore — 給 webhook 主路徑優先窗口
     * 3. expire_at > graceFloor — grace 視窗內（過期後仍保留 30 分鐘觀察期）
     * 4. last_reconciled_at IS NULL OR < retryBefore — 退避控制
     * 5. EXPIRED 額外限制：尚未 rescue 過（rescued_at IS NULL）
     */
    @Select("""
            SELECT * FROM payment_transactions
            WHERE status IN ('0', '3')
              AND payment_type != '2900'
              AND created_at < #{initialDelayBefore}
              AND expire_at IS NOT NULL
              AND expire_at > #{graceFloor}
              AND (last_reconciled_at IS NULL OR last_reconciled_at < #{retryBefore})
              AND (status = '0' OR rescued_at IS NULL)
            ORDER BY created_at
            LIMIT #{limit}
            """)
    List<PaymentTransactionEntity> findReconcileCandidates(
            @Param("initialDelayBefore") LocalDateTime initialDelayBefore,
            @Param("graceFloor") LocalDateTime graceFloor,
            @Param("retryBefore") LocalDateTime retryBefore,
            @Param("limit") int limit
    );

    /**
     * 搶佔一筆 reconcile 候選（issue #2294 H6，<b>lease on {@code last_reconciled_at}</b>）。
     *
     * <p>取代原本的 {@code updateLastReconciledAt}（{@code WHERE trade_no=?} 非 CAS ⇒
     * 兩個 pod 各得 affected=1，各自打一次 PayUni 查詢、各寫一筆 rescue audit log、
     * 後到者拿 unique violation 誤觸發 {@code PaymentReconcileConflict} 告警）。
     *
     * <p>《多副本排程搶佔規範》§二 六步套用結果：claim 寫入 {@code now}（一個之後會被
     * {@code retryBefore} 判為「已過期」的時間）⇒ 該列暫時離開候選集合、退避間隔後自動回來
     * ⇒ 是 <b>lease</b> 不是永久旗標。述詞與候選 SQL 的第 4 條完全一致。
     *
     * <p><b>租約長度 vs 候選窗口</b>（§三）：租約 = {@code retryIntervalMinutes}（預設 5 分鐘）；
     * 候選窗口的下限來自 {@code expire_at > graceFloor}（{@code graceMinutes} 預設 30 分鐘）
     * ⇒ 尾段最短 30 分鐘 ⇒ 窗口內仍有約 6 次機會 ✅。
     *
     * <p><b>副作用（誠實標註）</b>：退避間隔由「上次結束起算」變成「上次開始起算」。
     * {@code retryIntervalMinutes} 是分鐘級、單筆工作是一次 HTTP（+ 可能一次退款），
     * 差值遠小於間隔 ⇒ 行為等價。
     *
     * @return 1 = 搶到（呼叫端才可以往下做副作用）；0 = 已被另一個 pod 認領
     */
    @Update("""
            UPDATE payment_transactions
            SET last_reconciled_at = #{now}
            WHERE trade_no = #{tradeNo}
              AND (last_reconciled_at IS NULL OR last_reconciled_at < #{retryBefore})
            """)
    int claimForReconcile(
            @Param("tradeNo") String tradeNo,
            @Param("now") LocalDateTime now,
            @Param("retryBefore") LocalDateTime retryBefore
    );

    /**
     * 標記 rescue 動作完成（僅當 status='3'(EXPIRED) 且 rescued_at IS NULL 時）
     *
     * @param rescueAction REFUNDED 或 REFUND_PENDING_MANUAL
     */
    @Update("""
            UPDATE payment_transactions
            SET rescued_at = #{rescuedAt},
                rescue_action = #{rescueAction}
            WHERE trade_no = #{tradeNo}
              AND status = '3'
              AND rescued_at IS NULL
            """)
    int markRescued(
            @Param("tradeNo") String tradeNo,
            @Param("rescuedAt") LocalDateTime rescuedAt,
            @Param("rescueAction") String rescueAction
    );

    /**
     * 撥款欄位更新（成功與失敗共用；收斂規範 §6.2 撥款欄位封閉規則的 SoT 寫入點）
     * 條件：status='1'(PAID) — 僅允許對已付款交易寫入撥款資訊
     * 不檢 version：grant API 與其他路徑正交，且 retry 語義由呼叫端確保冪等
     */
    @Update("""
            UPDATE payment_transactions
            SET granted_at = #{grantedAt},
                grant_status = #{grantStatus},
                grant_message = #{grantMessage},
                updated_at = #{grantedAt},
                version = version + 1
            WHERE id = #{id}
              AND status = '1'
            """)
    int updateGrantResult(
            @Param("id") Long id,
            @Param("grantedAt") LocalDateTime grantedAt,
            @Param("grantStatus") String grantStatus,
            @Param("grantMessage") String grantMessage
    );

    /**
     * §5.6 去重硬化 A：撥款失敗終態寫入（PENDING_CONFIRM / PayUni 錯誤碼）。
     * 條件：status='1'(PAID)。<b>不寫 granted_at</b>（里程碑僅 SUCCESS 分支寫入，失敗路徑保持 null，
     * 為去重硬化 B 對帳區間起點取 updated_at 的前提）；刷新 updated_at 作為下輪對帳的區間錨點。
     * 不檢 version：grant API 與其他路徑正交，冪等由呼叫端（FOR UPDATE + grant_query 對帳）確保。
     */
    @Update("""
            UPDATE payment_transactions
            SET grant_status = #{grantStatus},
                grant_message = #{grantMessage},
                updated_at = #{updatedAt},
                version = version + 1
            WHERE id = #{id}
              AND status = '1'
            """)
    int updateGrantFailure(
            @Param("id") Long id,
            @Param("grantStatus") String grantStatus,
            @Param("grantMessage") String grantMessage,
            @Param("updatedAt") LocalDateTime updatedAt
    );

    /**
     * 批次查詢付款 provider 狀態（收斂規範 §6.3）。
     * 對每個 order_id 取當下 canonical row（is_duplicate ASC, created_at DESC 第一筆）。
     * 使用 DISTINCT ON 一次完成，避免 N+1。
     * 回傳 Map 欄位：key / provider_status / provider_message。
     */
    @Select({"<script>",
            "SELECT DISTINCT ON (order_id) order_id AS key, provider_status, provider_message",
            "FROM payment_transactions",
            "WHERE order_id IN",
            "<foreach collection='orderIds' item='oid' open='(' separator=',' close=')'>#{oid}</foreach>",
            "ORDER BY order_id, is_duplicate ASC, created_at DESC",
            "</script>"})
    List<ProviderStatusRow> findProviderStatusByOrderIds(@Param("orderIds") List<String> orderIds);
}
