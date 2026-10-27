package io.github.will413028.hawkr.payment.infrastructure.persistence.repository;

import io.github.will413028.hawkr.common.utils.code.RefundStatus;
import io.github.will413028.hawkr.payment.domain.model.RefundTransaction;
import io.github.will413028.hawkr.payment.domain.repository.RefundTransactionRepository;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.RefundTransactionEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.RefundTransactionMapper;
import org.springframework.stereotype.Repository;

import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.List;
import io.github.will413028.hawkr.payment.infrastructure.persistence.dto.ProviderStatusRow;
import java.util.Map;
import java.util.Optional;

/**
 * 退款交易 Repository 實作
 */
@Repository
public class RefundTransactionRepositoryImpl implements RefundTransactionRepository {

    private final RefundTransactionMapper mapper;

    public RefundTransactionRepositoryImpl(RefundTransactionMapper mapper) {
        this.mapper = mapper;
    }

    @Override
    public RefundTransaction save(RefundTransaction refund) {
        RefundTransactionEntity entity = toEntity(refund);
        mapper.insert(entity);
        return refund;
    }

    @Override
    public Optional<RefundTransaction> findByRefundNo(String refundNo) {
        RefundTransactionEntity entity = mapper.findByRefundNo(refundNo);
        return Optional.ofNullable(entity).map(this::toDomain);
    }

    @Override
    public List<RefundTransaction> findByTradeNo(String tradeNo) {
        return mapper.findByTradeNo(tradeNo).stream()
                .map(this::toDomain)
                .toList();
    }

    @Override
    public Optional<RefundTransaction> findByProviderRefundNo(String providerRefundNo) {
        RefundTransactionEntity entity = mapper.findByProviderRefundNo(providerRefundNo);
        return Optional.ofNullable(entity).map(this::toDomain);
    }

    @Override
    public int updateStatusToProcessing(String refundNo, String providerRefundNo, int currentVersion) {
        return mapper.updateToProcessing(refundNo, currentVersion);
    }

    @Override
    public int updateStatusToSucceeded(String refundNo, String providerRefundNo,
                                       String providerStatus, String providerMessage,
                                       LocalDate bankDate,
                                       LocalDateTime completedAt, int currentVersion) {
        return mapper.updateToSucceeded(refundNo, providerRefundNo, providerStatus,
                providerMessage, bankDate, completedAt, currentVersion);
    }

    @Override
    public int updateStatusToFailed(String refundNo, String providerStatus,
                                    String providerMessage, int currentVersion) {
        return mapper.updateToFailed(refundNo, providerStatus, providerMessage, currentVersion);
    }

    @Override
    public int markAtmRefundAccepted(String refundNo, String providerStatus, String providerMessage,
                                     LocalDate estimatedRefundDate) {
        return mapper.updateToProcessingWithEstimate(refundNo, providerStatus, providerMessage, estimatedRefundDate);
    }

    @Override
    public Optional<RefundTransaction> findActiveByTradeNo(String tradeNo) {
        RefundTransactionEntity entity = mapper.findActiveByTradeNo(tradeNo);
        return Optional.ofNullable(entity).map(this::toDomain);
    }

    @Override
    public Optional<RefundTransaction> findLatestByTradeNo(String tradeNo) {
        RefundTransactionEntity entity = mapper.findLatestByTradeNo(tradeNo);
        return Optional.ofNullable(entity).map(this::toDomain);
    }

    @Override
    public int finalizeAtmRefundSucceeded(String refundNo, String providerRefundNo,
                                          String providerStatus, String providerMessage,
                                          LocalDate bankDate, LocalDateTime completedAt) {
        return mapper.finalizeAtmRefundSucceeded(refundNo, providerRefundNo, providerStatus,
                providerMessage, bankDate, completedAt);
    }

    @Override
    public int finalizeAtmRefundFailed(String refundNo, String providerStatus, String providerMessage) {
        return mapper.finalizeAtmRefundFailed(refundNo, providerStatus, providerMessage);
    }

    @Override
    public int correctFailedToSucceeded(String refundNo, String providerRefundNo,
                                        String providerStatus, String providerMessage,
                                        LocalDate bankDate, LocalDateTime completedAt) {
        return mapper.correctFailedToSucceeded(refundNo, providerRefundNo, providerStatus,
                providerMessage, bankDate, completedAt);
    }

    @Override
    public List<ProviderStatusRow> findProviderStatusByCaseIds(List<String> caseIds) {
        if (caseIds == null || caseIds.isEmpty()) {
            return List.of();
        }
        return mapper.findProviderStatusByCaseIds(caseIds);
    }

    @Override
    public List<ProviderStatusRow> findProviderStatusByOrderIds(List<String> orderIds) {
        if (orderIds == null || orderIds.isEmpty()) {
            return List.of();
        }
        return mapper.findProviderStatusByOrderIds(orderIds);
    }

    @Override
    public long sumRefundedAmountByTradeNo(String tradeNo) {
        return mapper.sumRefundedAmount(tradeNo);
    }

    @Override
    public long sumRefundedAmountByTradeNoExcluding(String tradeNo, String excludeRefundNo) {
        return mapper.sumRefundedAmountByTradeNoExcluding(tradeNo, excludeRefundNo);
    }

    @Override
    public Optional<RefundTransaction> findByIdempotencyKey(String idempotencyKey) {
        RefundTransactionEntity entity = mapper.findByIdempotencyKey(idempotencyKey);
        return Optional.ofNullable(entity).map(this::toDomain);
    }

    @Override
    public boolean hasProcessingRefund(String tradeNo) {
        // §5.5.5 L2：SQL 直判 status IN ('0','1')（REQUESTED + PROCESSING，不含 SUCCEEDED），
        // 取代舊的 Java 端只過濾 PROCESSING（漏 REQUESTED 缺口）。狀態集合權威在 mapper SQL。
        return mapper.hasProcessingRefund(tradeNo);
    }

    @Override
    public boolean hasSucceededRefund(String tradeNo) {
        // §5.6.1 撥款接管主判準：僅終態成功 SUCCEEDED('2')，不含在途
        return mapper.hasSucceededRefund(tradeNo, RefundStatus.SUCCEEDED.value());
    }

    /**
     * Entity 轉 Domain Model
     */
    private RefundTransaction toDomain(RefundTransactionEntity entity) {
        return RefundTransaction.builder()
                .id(entity.getId())
                .refundNo(entity.getRefundNo())
                .paymentId(entity.getPaymentId())
                .tradeNo(entity.getTradeNo())
                .orderId(entity.getOrderId())
                .amount(entity.getAmount())
                .status(RefundStatus.fromValue(entity.getStatus()))
                .reason(entity.getReason())
                .idempotencyKey(entity.getIdempotencyKey())
                .providerRefundNo(entity.getProviderRefundNo())
                .providerStatus(entity.getProviderStatus())
                .providerMessage(entity.getProviderMessage())
                .estimatedRefundDate(entity.getEstimatedRefundDate())
                .bankDate(entity.getBankDate())
                .completedAt(entity.getCompletedAt())
                .createdAt(entity.getCreatedAt())
                .updatedAt(entity.getUpdatedAt())
                .version(entity.getVersion())
                .build();
    }

    /**
     * Domain Model 轉 Entity
     */
    private RefundTransactionEntity toEntity(RefundTransaction domain) {
        RefundTransactionEntity entity = new RefundTransactionEntity();
        entity.setId(domain.id());
        entity.setRefundNo(domain.refundNo());
        entity.setPaymentId(domain.paymentId());
        entity.setTradeNo(domain.tradeNo());
        entity.setOrderId(domain.orderId());
        entity.setAmount(domain.amount());
        entity.setStatus(domain.status().value());
        entity.setReason(domain.reason());
        entity.setIdempotencyKey(domain.idempotencyKey());
        entity.setProviderRefundNo(domain.providerRefundNo());
        entity.setProviderStatus(domain.providerStatus());
        entity.setProviderMessage(domain.providerMessage());
        entity.setEstimatedRefundDate(domain.estimatedRefundDate());
        entity.setBankDate(domain.bankDate());
        entity.setCompletedAt(domain.completedAt());
        entity.setCreatedAt(domain.createdAt());
        entity.setUpdatedAt(domain.updatedAt());
        entity.setVersion(domain.version());
        return entity;
    }
}
