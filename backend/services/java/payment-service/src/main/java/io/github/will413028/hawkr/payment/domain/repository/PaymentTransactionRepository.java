package io.github.will413028.hawkr.payment.domain.repository;

import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;

import java.time.LocalDateTime;
import java.time.LocalDate;
import java.util.List;
import io.github.will413028.hawkr.payment.infrastructure.persistence.dto.ProviderStatusRow;
import java.util.Map;
import java.util.Optional;

/**
 * 支付交易 Repository 介面
 */
public interface PaymentTransactionRepository {

    /**
     * 儲存交易
     */
    void save(PaymentTransaction transaction);

    /**
     * 根據交易編號查詢（單筆假設）。
     *
     * @deprecated §2A.6 之後同 trade_no 可有多 row，會在 mapper 端 throw
     * TooManyResultsException。請改用 {@link #findCanonicalByTradeNo}（取主 row）
     * 或 {@link #findById}（取特定 row，如 webhook reload）。
     */
    @Deprecated
    Optional<PaymentTransaction> findByTradeNo(String tradeNo);

    /**
     * §2A.6 之後對外查詢標準入口：取同 trade_no 的「目前生效的主 row」。
     * 排序：is_duplicate ASC, created_at DESC LIMIT 1。
     */
    Optional<PaymentTransaction> findCanonicalByTradeNo(String tradeNo);

    /**
     * §5.5.5 L1：取同 trade_no 的 canonical 主 row 並 FOR UPDATE 鎖住（退款併發序列化）。
     * 必須在 @Transactional 內呼叫，鎖持有至事務結束。
     */
    Optional<PaymentTransaction> findCanonicalByTradeNoForUpdate(String tradeNo);

    /**
     * 依 row id 精準查詢（webhook reload / 內部路徑專用）。
     */
    Optional<PaymentTransaction> findById(Long id);

    /**
     * 根據訂單編號查詢（可能多筆）
     */
    List<PaymentTransaction> findByOrderId(String orderId);

    /**
     * 查詢訂單的活躍交易（INIT 或 PAID），用於冪等性檢查
     * 防止同一訂單重複建立支付交易
     */
    Optional<PaymentTransaction> findActiveByOrderId(String orderId);

    /**
     * §5.6：取同 order_id 的 canonical 主 PAID row 並 FOR UPDATE 鎖住（撥款併發序列化）。
     * 必須在 @Transactional 內呼叫，鎖持有至事務結束（跨 PayUni Grant 呼叫）。
     */
    Optional<PaymentTransaction> findActiveByOrderIdForUpdate(String orderId);

    /**
     * 58 號 §4.1：取訂單的 canonical <b>已付款</b>交易（唯讀、不加鎖），供 {@code GetPaymentFactsInternal}。
     * <p>過濾 {@code is_duplicate = FALSE}，同層級取 {@code created_at} 最新。查無回 {@code Optional.empty()}
     * ——呼叫端必須 fail-loud，不得把「查無」當成「有交易但沒付款時間」。
     */
    Optional<PaymentTransaction> findCanonicalPaidByOrderId(String orderId);

    /**
     * §2.2 業務 pivot (2026-05-14)：以 MerID 查最近一筆 PENDING (status=INIT/0) transaction，
     * 供 UnknownPaymentFallbackConsumer 在收到無類型 key 的 PayUni webhook 時嘗試綁定。
     * 過濾 {@code is_duplicate=false}（避免抓到 §2A.6 重複付款保留 row）。
     * 找不到回傳 {@code Optional.empty()}。
     */
    Optional<PaymentTransaction> findLatestPendingByMerId(String merId);

    /**
     * §2.2 業務 pivot (2026-05-14)：UnknownPaymentFallbackConsumer 將 PayUni 訊息 stamp 到 provider_message。
     * 樂觀鎖 {@code WHERE provider_message IS NULL}：避免覆蓋 PendingPaymentReconcileService 已寫入的訊息。
     * @return 影響行數
     */
    int stampUnknownFallbackMessage(Long id, String providerMessage);

    /**
     * §2A.3.6 查詢同 order_id + paymentType 的活躍 row（給切換偵測用）。
     */
    Optional<PaymentTransaction> findActiveByOrderIdAndPaymentType(String orderId, PaymentMethod paymentType);

    /**
     * §2A.3.6 取得同 order_id 目前最大的 attempt_seq。
     * 用於生成下一個 trade_no = orderId + format("%03d", maxSeq + 1)。
     */
    Optional<Integer> findMaxAttemptSeqByOrderId(String orderId);

    /**
     * §2A.3 切換流程：將舊 row 標為 CLOSED_BY_SWITCH。
     *
     * @return 影響行數（0 表示樂觀鎖失敗或 row 已不在 INIT/PAID）
     */
    int markClosedBySwitch(Long id, LocalDateTime closedAt, int expectedVersion);

    /** §2A.3.6 段 1：依 PayUni provider_trade_no 精準查找（webhook 用） */
    Optional<PaymentTransaction> findByProviderTradeNo(String providerTradeNo);

    /** §2A.3.6 段 3：列出同 order_id 所有歷史 row */
    List<PaymentTransaction> findHistoryByOrderId(String orderId);

    /** §2A.6 找同 order_id 其他 PAID + 非重複 row */
    List<PaymentTransaction> findOtherPaidRowsForSameOrder(String orderId, Long excludeId);

    /** §2A.6 鎖既有「主」PAID row（FOR UPDATE） */
    Optional<PaymentTransaction> lockExistingPaidByOrderId(String orderId);

    /** §2A.6 取得目前最大 duplicate_sequence */
    int findMaxDuplicateSequenceByOrderId(String orderId);

    /** §2A.6 標記某 row 為重複付款的副 row */
    int markAsDuplicate(Long id, Long mainId, Integer sequence);

    /** §2A.6 翻轉：取消某 row 的 duplicate 標記 */
    int unmarkDuplicate(Long id);

    /** §10.5 D7 寫入 PayUni 真實時間軸 */
    int updateProviderTimes(Long id, LocalDateTime providerCreateAt, LocalDateTime providerPaidAt);

    /** §2A.3.6 段 1.b reactivate row：CLOSED_BY_SWITCH → PAID + 標 duplicate */
    int reactivateRowAsDuplicate(Long id, Long mainId, Integer sequence, LocalDateTime providerPaidAt);

    /** §2A.6.5 rescue 退款後標 row 為 CLOSED（PAID → CLOSED） */
    int markRescueRefunded(Long id, LocalDateTime closedAt);

    /**
     * 條件式更新狀態為 PAID（僅當狀態為 INIT）
     *
     * @return 影響行數（0 表示條件不符）
     */
    int updateToPaid(String tradeNo, String providerTradeNo, String providerStatus,
                     String providerMessage, LocalDateTime payTime, int expectedVersion);

    /**
     * 條件式更新狀態為 FAILED（僅當狀態為 INIT）
     */
    int updateToFailed(String tradeNo, String providerStatus, String providerMessage,
                       String providerTradeNo, int expectedVersion);

    /**
     * 條件式更新狀態為 EXPIRED（僅當狀態為 INIT 且已過期）
     */
    int updateToExpired(String tradeNo, int expectedVersion);

    /**
     * 條件式更新狀態為 CONFLICT（僅當狀態為 PAID 或 FAILED）
     *
     * 適用場景：Apple Pay / Google Pay 同步終態與 Webhook 結果矛盾
     */
    int updateToConflict(String tradeNo, String providerStatus, String providerMessage, int expectedVersion);

    /**
     * 條件式更新狀態為 CLOSED（僅當狀態為 INIT）
     */
    int updateToClosed(String tradeNo, LocalDateTime closedAt, int expectedVersion);

    /**
     * 更新 ATM 資訊
     */
    int updateAtmInfo(String tradeNo, String providerTradeNo, String atmBankCode, String atmVirtualAccount,
                      LocalDate atmExpireDate, LocalDateTime expireAt, int expectedVersion);

    /**
     * §2A.5.4 ATM 復活：鎖定同 order_id 既有 INIT row（FOR UPDATE）。
     */
    Optional<PaymentTransaction> findActiveInitByOrderIdForUpdate(String orderId);

    /**
     * §2A.5.4 ATM 復活：查找同 order_id 可復活的 ATM CLOSED_BY_SWITCH row（FOR UPDATE）。
     * 配合外層 REPEATABLE_READ + 鎖既有 INIT row 順序使用。
     */
    Optional<PaymentTransaction> findReactivableAtmRowForUpdate(String orderId, LocalDateTime nowUtc);

    /**
     * §2A.5.4 ATM 復活：將 CLOSED_BY_SWITCH row 翻回 INIT 並寫入 reactivated_at。
     * 條件：status = '6'。回傳影響筆數。
     */
    int markReactivated(Long id, LocalDateTime nowUtc);

    /**
     * 查詢已逾期但狀態仍為 INIT 的交易
     */
    List<PaymentTransaction> findExpiredTransactions(int limit);

    /**
     * L2 補救路徑：撈出 reconcile 候選交易
     */
    List<PaymentTransaction> findReconcileCandidates(
            LocalDateTime initialDelayBefore,
            LocalDateTime graceFloor,
            LocalDateTime retryBefore,
            int limit
    );

    /**
     * L2 補救路徑：搶佔一筆 reconcile 候選（issue #2294 H6，lease on {@code last_reconciled_at}）。
     *
     * <p>取代原本的 {@code updateLastReconciledAt}（非 CAS，兩個 pod 各得 affected=1）。
     * 呼叫端必須在<b>任何副作用之前</b>呼叫，回 0 即 skip 該筆。
     *
     * @return 1 = 搶到；0 = 已被另一個 pod 認領
     */
    int claimForReconcile(String tradeNo, LocalDateTime now, LocalDateTime retryBefore);

    /**
     * L2 補救路徑：標記 rescue 動作完成（僅當 status=EXPIRED 且 rescued_at IS NULL 時生效）
     */
    int markRescued(String tradeNo, LocalDateTime rescuedAt, String rescueAction);

    /**
     * 撥款「成功」終態寫入（收斂規範 §6.2 grant_* 封閉欄位）。
     * grantedAt=NOW（里程碑）、grantStatus='SUCCESS'、grantMessage=PayUni 訊息。
     */
    int updateGrantResult(Long id, LocalDateTime grantedAt, String grantStatus, String grantMessage);

    /**
     * §5.6 去重硬化 A：撥款「失敗」終態寫入（{@code PENDING_CONFIRM} 哨兵值 / PayUni 錯誤碼）。
     * <p>
     * 與 {@link #updateGrantResult} 的關鍵差異：<b>不寫 {@code granted_at}</b>——里程碑僅 SUCCESS 分支
     * 寫入，失敗路徑保持 {@code null}（收斂規範 §3.4；亦為去重硬化 B 對帳區間起點取 {@code updated_at}
     * 而非 {@code granted_at} 的前提）。只更新 {@code grant_status} / {@code grant_message} /
     * {@code updated_at}（每次撥款嘗試落地終態時刷新，作為下輪對帳的有效區間錨點）。
     *
     * @return 影響行數
     */
    int updateGrantFailure(Long id, String grantStatus, String grantMessage, LocalDateTime updatedAt);

    /**
     * 批次查詢：依 order_id 取每 order 的 canonical row 之 provider_status/message。
     * 收斂規範 §6.3 聚合批次查詢。
     */
    List<ProviderStatusRow> findProviderStatusByOrderIds(List<String> orderIds);
}
