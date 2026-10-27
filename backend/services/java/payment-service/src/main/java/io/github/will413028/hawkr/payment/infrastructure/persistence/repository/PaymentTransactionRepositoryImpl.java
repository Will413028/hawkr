package io.github.will413028.hawkr.payment.infrastructure.persistence.repository;

import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.PaymentTransactionEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentTransactionMapper;
import org.springframework.stereotype.Repository;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.time.LocalDateTime;
import java.util.List;
import io.github.will413028.hawkr.payment.infrastructure.persistence.dto.ProviderStatusRow;
import java.util.Map;
import java.util.Optional;

/**
 * 支付交易 Repository 實作
 */
@Repository
public class PaymentTransactionRepositoryImpl implements PaymentTransactionRepository {

    private final PaymentTransactionMapper mapper;

    public PaymentTransactionRepositoryImpl(PaymentTransactionMapper mapper) {
        this.mapper = mapper;
    }

    @Override
    public void save(PaymentTransaction transaction) {
        PaymentTransactionEntity entity = toEntity(transaction);
        mapper.insert(entity);
    }

    @Override
    @SuppressWarnings("deprecation")
    public Optional<PaymentTransaction> findByTradeNo(String tradeNo) {
        PaymentTransactionEntity entity = mapper.findByTradeNo(tradeNo);
        return Optional.ofNullable(entity).map(this::toDomain);
    }

    @Override
    public Optional<PaymentTransaction> findCanonicalByTradeNo(String tradeNo) {
        PaymentTransactionEntity entity = mapper.findCanonicalByTradeNo(tradeNo);
        return Optional.ofNullable(entity).map(this::toDomain);
    }

    @Override
    public Optional<PaymentTransaction> findCanonicalByTradeNoForUpdate(String tradeNo) {
        PaymentTransactionEntity entity = mapper.findCanonicalByTradeNoForUpdate(tradeNo);
        return Optional.ofNullable(entity).map(this::toDomain);
    }

    @Override
    public Optional<PaymentTransaction> findById(Long id) {
        if (id == null) return Optional.empty();
        return Optional.ofNullable(mapper.selectById(id)).map(this::toDomain);
    }

    @Override
    public List<PaymentTransaction> findByOrderId(String orderId) {
        return mapper.findByOrderId(orderId).stream()
                .map(this::toDomain)
                .toList();
    }

    @Override
    public Optional<PaymentTransaction> findActiveByOrderId(String orderId) {
        PaymentTransactionEntity entity = mapper.findActiveByOrderId(orderId);
        return Optional.ofNullable(entity).map(this::toDomain);
    }

    @Override
    public Optional<PaymentTransaction> findActiveByOrderIdForUpdate(String orderId) {
        PaymentTransactionEntity entity = mapper.findActiveByOrderIdForUpdate(orderId);
        return Optional.ofNullable(entity).map(this::toDomain);
    }

    @Override
    public Optional<PaymentTransaction> findCanonicalPaidByOrderId(String orderId) {
        // 狀態值不寫字面量：走 CodeMeta 生成的 enum（§8 禁止硬編碼）
        PaymentTransactionEntity entity = mapper.findCanonicalPaidByOrderId(
                orderId, PaymentTransactionStatus.PAID.value());
        return Optional.ofNullable(entity).map(this::toDomain);
    }

    @Override
    public Optional<PaymentTransaction> findLatestPendingByMerId(String merId) {
        PaymentTransactionEntity entity = mapper.findLatestPendingByMerId(merId);
        return Optional.ofNullable(entity).map(this::toDomain);
    }

    @Override
    public int stampUnknownFallbackMessage(Long id, String providerMessage) {
        return mapper.stampUnknownFallbackMessage(id, providerMessage);
    }

    @Override
    public Optional<PaymentTransaction> findActiveByOrderIdAndPaymentType(String orderId, PaymentMethod paymentType) {
        PaymentTransactionEntity entity = mapper.findActiveByOrderIdAndPaymentType(orderId, paymentType.value());
        return Optional.ofNullable(entity).map(this::toDomain);
    }

    @Override
    public Optional<Integer> findMaxAttemptSeqByOrderId(String orderId) {
        return Optional.ofNullable(mapper.findMaxAttemptSeqByOrderId(orderId));
    }

    @Override
    public int markClosedBySwitch(Long id, LocalDateTime closedAt, int expectedVersion) {
        return mapper.markClosedBySwitch(id, closedAt, expectedVersion);
    }

    @Override
    public Optional<PaymentTransaction> findByProviderTradeNo(String providerTradeNo) {
        if (providerTradeNo == null || providerTradeNo.isBlank()) return Optional.empty();
        PaymentTransactionEntity entity = mapper.findByProviderTradeNo(providerTradeNo);
        return Optional.ofNullable(entity).map(this::toDomain);
    }

    @Override
    public List<PaymentTransaction> findHistoryByOrderId(String orderId) {
        return mapper.findHistoryByOrderId(orderId).stream().map(this::toDomain).toList();
    }

    @Override
    public List<PaymentTransaction> findOtherPaidRowsForSameOrder(String orderId, Long excludeId) {
        return mapper.findOtherPaidRowsForSameOrder(orderId, excludeId).stream().map(this::toDomain).toList();
    }

    @Override
    public Optional<PaymentTransaction> lockExistingPaidByOrderId(String orderId) {
        PaymentTransactionEntity entity = mapper.lockExistingPaidByOrderId(orderId);
        return Optional.ofNullable(entity).map(this::toDomain);
    }

    @Override
    public int findMaxDuplicateSequenceByOrderId(String orderId) {
        Integer v = mapper.findMaxDuplicateSequenceByOrderId(orderId);
        return v != null ? v : 0;
    }

    @Override
    public int markAsDuplicate(Long id, Long mainId, Integer sequence) {
        return mapper.markAsDuplicate(id, mainId, sequence);
    }

    @Override
    public int unmarkDuplicate(Long id) {
        return mapper.unmarkDuplicate(id);
    }

    @Override
    public int updateProviderTimes(Long id, LocalDateTime providerCreateAt, LocalDateTime providerPaidAt) {
        return mapper.updateProviderTimes(id, providerCreateAt, providerPaidAt);
    }

    @Override
    public int reactivateRowAsDuplicate(Long id, Long mainId, Integer sequence, LocalDateTime providerPaidAt) {
        return mapper.reactivateRowAsDuplicate(id, mainId, sequence, providerPaidAt);
    }

    @Override
    public int markRescueRefunded(Long id, LocalDateTime closedAt) {
        return mapper.markRescueRefunded(id, closedAt);
    }

    @Override
    public Optional<PaymentTransaction> findActiveInitByOrderIdForUpdate(String orderId) {
        PaymentTransactionEntity entity = mapper.findActiveInitByOrderIdForUpdate(orderId);
        return Optional.ofNullable(entity).map(this::toDomain);
    }

    @Override
    public Optional<PaymentTransaction> findReactivableAtmRowForUpdate(String orderId, LocalDateTime nowUtc) {
        PaymentTransactionEntity entity = mapper.findReactivableAtmRowForUpdate(orderId, nowUtc);
        return Optional.ofNullable(entity).map(this::toDomain);
    }

    @Override
    public int markReactivated(Long id, LocalDateTime nowUtc) {
        return mapper.markReactivated(id, nowUtc);
    }

    @Override
    public int updateToPaid(String tradeNo, String providerTradeNo, String providerStatus,
                            String providerMessage, LocalDateTime payTime, int expectedVersion) {
        return mapper.updateToPaid(tradeNo, providerTradeNo, providerStatus, providerMessage, payTime, expectedVersion);
    }

    @Override
    public int updateToFailed(String tradeNo, String providerStatus, String providerMessage,
                              String providerTradeNo, int expectedVersion) {
        return mapper.updateToFailed(tradeNo, providerStatus, providerMessage, providerTradeNo, expectedVersion);
    }

    @Override
    public int updateToExpired(String tradeNo, int expectedVersion) {
        return mapper.updateToExpired(tradeNo, DateTimeUtils.nowUtc(), expectedVersion);
    }

    @Override
    public int updateToConflict(String tradeNo, String providerStatus, String providerMessage, int expectedVersion) {
        return mapper.updateToConflict(tradeNo, providerStatus, providerMessage, expectedVersion);
    }

    @Override
    public int updateToClosed(String tradeNo, LocalDateTime closedAt, int expectedVersion) {
        return mapper.updateToClosed(tradeNo, closedAt, expectedVersion);
    }

    @Override
    public int updateAtmInfo(String tradeNo, String providerTradeNo, String atmBankCode, String atmVirtualAccount,
                             java.time.LocalDate atmExpireDate, LocalDateTime expireAt, int expectedVersion) {
        return mapper.updateAtmInfo(tradeNo, providerTradeNo, atmBankCode, atmVirtualAccount, atmExpireDate, expireAt, expectedVersion);
    }

    @Override
    public List<PaymentTransaction> findExpiredTransactions(int limit) {
        return mapper.findExpiredTransactions(DateTimeUtils.nowUtc(), limit).stream()
                .map(this::toDomain)
                .toList();
    }

    @Override
    public List<PaymentTransaction> findReconcileCandidates(
            LocalDateTime initialDelayBefore,
            LocalDateTime graceFloor,
            LocalDateTime retryBefore,
            int limit
    ) {
        return mapper.findReconcileCandidates(initialDelayBefore, graceFloor, retryBefore, limit).stream()
                .map(this::toDomain)
                .toList();
    }

    @Override
    public int claimForReconcile(String tradeNo, LocalDateTime now, LocalDateTime retryBefore) {
        return mapper.claimForReconcile(tradeNo, now, retryBefore);
    }

    @Override
    public int markRescued(String tradeNo, LocalDateTime rescuedAt, String rescueAction) {
        return mapper.markRescued(tradeNo, rescuedAt, rescueAction);
    }

    @Override
    public int updateGrantResult(Long id, LocalDateTime grantedAt, String grantStatus, String grantMessage) {
        return mapper.updateGrantResult(id, grantedAt, grantStatus, grantMessage);
    }

    @Override
    public int updateGrantFailure(Long id, String grantStatus, String grantMessage, LocalDateTime updatedAt) {
        return mapper.updateGrantFailure(id, grantStatus, grantMessage, updatedAt);
    }

    @Override
    public List<ProviderStatusRow> findProviderStatusByOrderIds(List<String> orderIds) {
        if (orderIds == null || orderIds.isEmpty()) {
            return List.of();
        }
        return mapper.findProviderStatusByOrderIds(orderIds);
    }

    /**
     * Entity 轉 Domain Model
     */
    private PaymentTransaction toDomain(PaymentTransactionEntity entity) {
        return PaymentTransaction.builder()
                .id(entity.getId())
                .tradeNo(entity.getTradeNo())
                .orderId(entity.getOrderId())
                .attemptSeq(entity.getAttemptSeq() != null ? entity.getAttemptSeq() : 1)
                .merId(entity.getMerId())
                .buyerId(entity.getBuyerId())
                .paymentType(PaymentMethod.fromValue(entity.getPaymentType()))
                .amount(entity.getAmount())
                .productName(entity.getProductName())
                .status(PaymentTransactionStatus.fromValue(entity.getStatus()))
                .providerTradeNo(entity.getProviderTradeNo())
                .providerStatus(entity.getProviderStatus())
                .providerMessage(entity.getProviderMessage())
                // 撥款欄位（§5.6）：撥款前冪等短路（grant_status==SUCCESS）與去重硬化 B（非首撥判定 +
                // updated_at 對帳區間錨點）皆依賴這些欄位載入；漏 map 會使短路/對帳在正式環境永不生效。
                .grantedAt(entity.getGrantedAt())
                .grantStatus(entity.getGrantStatus())
                .grantMessage(entity.getGrantMessage())
                .atmBankCode(entity.getAtmBankCode())
                .atmVirtualAccount(entity.getAtmVirtualAccount())
                .atmExpireDate(entity.getAtmExpireDate())
                .payTime(entity.getPayTime())
                .closedAt(entity.getClosedAt())
                .expireAt(entity.getExpireAt())
                .createdAt(entity.getCreatedAt())
                .updatedAt(entity.getUpdatedAt())
                .lastReconciledAt(entity.getLastReconciledAt())
                .rescuedAt(entity.getRescuedAt())
                .rescueAction(entity.getRescueAction())
                .isDuplicate(entity.getIsDuplicate() != null && entity.getIsDuplicate())
                .duplicateOfPaymentId(entity.getDuplicateOfPaymentId())
                .duplicateSequence(entity.getDuplicateSequence())
                .providerCreateAt(entity.getProviderCreateAt())
                .providerPaidAt(entity.getProviderPaidAt())
                .reactivatedAt(entity.getReactivatedAt())
                .clientType(entity.getClientType())
                .appType(entity.getAppType())
                .version(entity.getVersion() != null ? entity.getVersion() : 0)
                .build();
    }

    /**
     * Domain Model 轉 Entity
     */
    private PaymentTransactionEntity toEntity(PaymentTransaction domain) {
        PaymentTransactionEntity entity = new PaymentTransactionEntity();
        entity.setId(domain.id());
        entity.setTradeNo(domain.tradeNo());
        entity.setOrderId(domain.orderId());
        entity.setAttemptSeq(domain.attemptSeq());
        entity.setMerId(domain.merId());
        entity.setBuyerId(domain.buyerId());
        entity.setPaymentType(domain.paymentType().value());
        entity.setAmount(domain.amount());
        entity.setProductName(domain.productName());
        entity.setStatus(domain.status().value());
        entity.setProviderTradeNo(domain.providerTradeNo());
        entity.setProviderStatus(domain.providerStatus());
        entity.setProviderMessage(domain.providerMessage());
        entity.setGrantedAt(domain.grantedAt());
        entity.setGrantStatus(domain.grantStatus());
        entity.setGrantMessage(domain.grantMessage());
        entity.setAtmBankCode(domain.atmBankCode());
        entity.setAtmVirtualAccount(domain.atmVirtualAccount());
        entity.setAtmExpireDate(domain.atmExpireDate());
        entity.setPayTime(domain.payTime());
        entity.setClosedAt(domain.closedAt());
        entity.setExpireAt(domain.expireAt());
        entity.setCreatedAt(domain.createdAt());
        entity.setUpdatedAt(domain.updatedAt());
        entity.setLastReconciledAt(domain.lastReconciledAt());
        entity.setRescuedAt(domain.rescuedAt());
        entity.setRescueAction(domain.rescueAction());
        entity.setIsDuplicate(domain.isDuplicate());
        entity.setDuplicateOfPaymentId(domain.duplicateOfPaymentId());
        entity.setDuplicateSequence(domain.duplicateSequence());
        entity.setProviderCreateAt(domain.providerCreateAt());
        entity.setProviderPaidAt(domain.providerPaidAt());
        entity.setReactivatedAt(domain.reactivatedAt());
        entity.setClientType(domain.clientType());
        entity.setAppType(domain.appType());
        entity.setVersion(domain.version());
        return entity;
    }
}
