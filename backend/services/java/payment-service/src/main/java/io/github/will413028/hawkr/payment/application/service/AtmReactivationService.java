package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.payment.adapter.grpc.client.OrderGrpcClient;
import io.github.will413028.hawkr.payment.application.dto.CreatePaymentRequest;
import io.github.will413028.hawkr.payment.application.dto.CreatePaymentResponse;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.exception.PaymentBusinessException;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.LocalDateTime;
import java.util.Optional;

/**
 * §2A.5.4 ATM 復活服務（獨立 bean）。
 * <p>
 * 設計上抽為獨立 service 是為了單一職責 + 易於單元測試，而非為了 isolation override。
 * <p>
 * <b>關於 transaction isolation</b>：本方法宣告 {@code @Transactional}（預設 propagation
 * REQUIRED + 預設 isolation），由呼叫端 {@code PaymentApplicationService.createPayment}
 * 既有的 @Transactional 提供 transaction 範圍。實際隔離級別為 PostgreSQL 預設
 * READ_COMMITTED。雖然 §2A.5.4 早期設計提議 REPEATABLE_READ，但 Spring 在 propagation
 * REQUIRED 加入既有 TX 時會忽略 isolation override。
 * <p>
 * <b>正確性保證</b>：兩次 SELECT 都用 {@code FOR UPDATE} 鎖定具體 row（findActiveInit /
 * findReactivableAtm），鎖定後的 row 不可能被併發事務改動，序列化等價於
 * REPEATABLE_READ + FOR UPDATE。READ_COMMITTED + FOR UPDATE 在 lock-then-modify
 * 模式下與 REPEATABLE_READ + FOR UPDATE 在本路徑等價。
 * <p>
 * 流程細節見 {@link #tryReactivate}。
 */
@Service
public class AtmReactivationService {

    private static final ObsLogger log = ObsLogger.forClass(AtmReactivationService.class);

    private final PaymentTransactionRepository transactionRepository;
    private final OrderGrpcClient orderGrpcClient;

    public AtmReactivationService(PaymentTransactionRepository transactionRepository,
                                  OrderGrpcClient orderGrpcClient) {
        this.transactionRepository = transactionRepository;
        this.orderGrpcClient = orderGrpcClient;
    }

    /**
     * §2A.5.4 嘗試復活舊 ATM CLOSED_BY_SWITCH row：
     * <ol>
     *   <li>FOR UPDATE 鎖既有 INIT row（其他付款方式正在進行中）</li>
     *   <li>FOR UPDATE 鎖可復活的 ATM CLOSED_BY_SWITCH row（取 expire_at 最新那筆）</li>
     *   <li>金額一致檢查（不符 → 拒絕復活，呼叫端 fall back 新建）</li>
     *   <li>翻轉順序保護 partial unique index：先 markClosedBySwitch(currentInit) 騰出 INIT 集合槽位，再 markReactivated(reactivable)</li>
     *   <li>orders.payment_method snapshot 同步為 ATM (§2A.7 觸發點 #3)</li>
     *   <li>回傳與第一次取號完全相同的 atmVirtualAccount + expireAt</li>
     * </ol>
     * 整個流程包在 REPEATABLE_READ 內。<b>不打 PayUni、不發新帳號、不經 §2A.4 冷卻</b>。
     *
     * @return Optional.of(response) 復活成功；Optional.empty() 無可復活 row 或前置條件不符，呼叫端應走新建流程
     */
    @Transactional
    public Optional<CreatePaymentResponse> tryReactivate(CreatePaymentRequest request, long amount) {
        String orderNo = request.orderNo();
        LocalDateTime now = DateTimeUtils.nowUtc();

        // 1. 鎖定既有 INIT row（必然存在，否則不會進到 4A 切換分支）
        Optional<PaymentTransaction> currentInitOpt =
                transactionRepository.findActiveInitByOrderIdForUpdate(orderNo);
        if (currentInitOpt.isEmpty()) {
            log.warn("AtmReactivationService.tryReactivate.noInitRow",
                    "復活流程入口：找不到 INIT row order_no=" + orderNo);
            return Optional.empty();
        }
        PaymentTransaction currentInit = currentInitOpt.get();

        // 2. 鎖定可復活的 ATM CLOSED_BY_SWITCH row
        Optional<PaymentTransaction> reactivableOpt =
                transactionRepository.findReactivableAtmRowForUpdate(orderNo, now);
        if (reactivableOpt.isEmpty()) {
            log.info("AtmReactivationService.tryReactivate.noCandidate",
                    "無可復活 ATM row，走新建流程 order_no=" + orderNo);
            return Optional.empty();
        }
        PaymentTransaction reactivable = reactivableOpt.get();

        // 3. 金額一致檢查（防禦性，不該發生）
        if (reactivable.amount() != amount) {
            log.error("AtmReactivationService.tryReactivate.amountMismatch",
                    "ATM 復活金額不符，拒絕復活 order_no=" + orderNo
                            + " reactivable=" + reactivable.amount() + " requested=" + amount);
            return Optional.empty();
        }

        // 4a. currentInit → CLOSED_BY_SWITCH（騰出 partial unique index 槽位）
        int closed = transactionRepository.markClosedBySwitch(
                currentInit.id(), now, currentInit.version());
        if (closed == 0) {
            log.warn("AtmReactivationService.tryReactivate.closeFailed",
                    "currentInit markClosedBySwitch 失敗 trade_no=" + currentInit.tradeNo());
            return Optional.empty();
        }

        // 4b. reactivable → INIT（寫入 reactivated_at）
        int reactivated = transactionRepository.markReactivated(reactivable.id(), now);
        if (reactivated == 0) {
            log.error("AtmReactivationService.tryReactivate.reactivateFailed",
                    "markReactivated 失敗 reactivable_id=" + reactivable.id());
            throw new PaymentBusinessException(GrpcResponseCode.INTERNAL_ERROR,
                    "ATM row 復活失敗 id=" + reactivable.id());
        }

        // 5. §2A.7 訂單付款方式快照同步（觸發點 #3）
        orderGrpcClient.snapshotPaymentInfo(
                orderNo,
                reactivable.tradeNo(),
                PaymentMethod.ATM.value(),
                reactivable.atmBankCode(),
                reactivable.atmVirtualAccount(),
                reactivable.atmExpireDate() != null ? reactivable.atmExpireDate().toString() : null
        );

        log.info("AtmReactivationService.tryReactivate.success",
                "ATM row 復活成功 order_no=" + orderNo
                        + " reactivated_id=" + reactivable.id()
                        + " virtual_account=" + reactivable.atmVirtualAccount()
                        + " closed_init_id=" + currentInit.id());

        return Optional.of(CreatePaymentResponse.forAtm(
                reactivable.tradeNo(),
                reactivable.orderId(),
                reactivable.amount(),
                reactivable.atmBankCode(),
                reactivable.atmVirtualAccount(),
                reactivable.atmExpireDate(),
                reactivable.expireAt()
        ));
    }
}
