package io.github.will413028.hawkr.payment.application.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.common.utils.code.RefundStatus;
import io.github.will413028.hawkr.payment.constant.PayUniConstants;
import io.github.will413028.hawkr.payment.grpc.AdminGrantSummary;
import io.github.will413028.hawkr.payment.grpc.AdminPaymentRefundSummary;
import io.github.will413028.hawkr.payment.grpc.AdminPaymentTransactionDetail;
import io.github.will413028.hawkr.payment.grpc.AdminPaymentTransactionSummary;
import io.github.will413028.hawkr.payment.grpc.ListGrantsForAdminData;
import io.github.will413028.hawkr.payment.grpc.ListGrantsForAdminRequest;
import io.github.will413028.hawkr.payment.grpc.ListPaymentTransactionsForAdminData;
import io.github.will413028.hawkr.payment.grpc.ListPaymentTransactionsForAdminRequest;
import io.github.will413028.hawkr.payment.grpc.ListRefundsForAdminData;
import io.github.will413028.hawkr.payment.grpc.ListRefundsForAdminRequest;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.PaymentTransactionEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.RefundTransactionEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentTransactionMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.RefundTransactionMapper;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;

import java.time.LocalDateTime;
import java.util.Arrays;
import java.util.List;
import java.util.Optional;
import java.util.Set;

/**
 * Admin 異常訂單管理 API 的付款端查詢服務（§29）。
 *
 * 涵蓋 P1~P5：付款 / 退款 / 撥款三類異常列表 + 詳情。
 */
@Service
public class PaymentAdminQueryService {

    private static final ObsLogger log = ObsLogger.forClass(PaymentAdminQueryService.class);
    private static final int MAX_PAGE_SIZE = 100;

    /** 付款異常 status values：FAILED / EXPIRED / CONFLICT。 */
    private static final Set<String> ABNORMAL_PAYMENT_STATUS_VALUES = Set.of(
            PaymentTransactionStatus.FAILED.value(),
            PaymentTransactionStatus.EXPIRED.value(),
            PaymentTransactionStatus.CONFLICT.value()
    );

    private final PaymentTransactionMapper paymentMapper;
    private final RefundTransactionMapper refundMapper;

    public PaymentAdminQueryService(PaymentTransactionMapper paymentMapper,
                                    RefundTransactionMapper refundMapper) {
        this.paymentMapper = paymentMapper;
        this.refundMapper = refundMapper;
    }

    // ==================== P1：列付款異常 ====================

    public ListPaymentTransactionsForAdminData listPayments(ListPaymentTransactionsForAdminRequest req) {
        int pageNum = parseIntOrDefault(req.getPage(), 1);
        int pageSize = Math.min(parseIntOrDefault(req.getPageSize(), 20), MAX_PAGE_SIZE);

        LambdaQueryWrapper<PaymentTransactionEntity> q = new LambdaQueryWrapper<>();

        if (req.getAbnormalOnly()) {
            q.and(w -> w.in(PaymentTransactionEntity::getStatus, ABNORMAL_PAYMENT_STATUS_VALUES)
                    .or()
                    .isNotNull(PaymentTransactionEntity::getRescueAction));
        }
        if (StringUtils.hasText(req.getProviderStatus())) {
            q.eq(PaymentTransactionEntity::getProviderStatus, req.getProviderStatus());
        }
        if (StringUtils.hasText(req.getStatus())) {
            List<String> list = Arrays.asList(req.getStatus().split(","));
            q.in(PaymentTransactionEntity::getStatus, list);
        }
        if (StringUtils.hasText(req.getOrderNo())) {
            q.eq(PaymentTransactionEntity::getOrderId, req.getOrderNo());
        }
        if (StringUtils.hasText(req.getBuyerId())) {
            q.eq(PaymentTransactionEntity::getBuyerId, req.getBuyerId());
        }
        if (StringUtils.hasText(req.getPaymentType())) {
            q.eq(PaymentTransactionEntity::getPaymentType, req.getPaymentType());
        }
        LocalDateTime createdFrom = parseIso(req.getCreatedFrom());
        LocalDateTime createdTo = parseIso(req.getCreatedTo());
        if (createdFrom != null) q.ge(PaymentTransactionEntity::getCreatedAt, createdFrom);
        if (createdTo != null) q.lt(PaymentTransactionEntity::getCreatedAt, createdTo);

        applyPaymentSort(q, normalizeSort(req.getSort()));

        Page<PaymentTransactionEntity> page = new Page<>(pageNum, pageSize);
        var result = paymentMapper.selectPage(page, q);

        ListPaymentTransactionsForAdminData.Builder b = ListPaymentTransactionsForAdminData.newBuilder()
                .setTotal(String.valueOf(result.getTotal()))
                .setPage(String.valueOf(result.getCurrent()))
                .setPageSize(String.valueOf(result.getSize()));
        for (PaymentTransactionEntity e : result.getRecords()) {
            b.addRecords(toPaymentSummary(e));
        }
        return b.build();
    }

    // ==================== P2：付款詳情 ====================

    public Optional<AdminPaymentTransactionDetail> getPaymentDetail(String tradeNo) {
        if (!StringUtils.hasText(tradeNo)) return Optional.empty();
        PaymentTransactionEntity entity = paymentMapper.selectOne(
                new LambdaQueryWrapper<PaymentTransactionEntity>()
                        .eq(PaymentTransactionEntity::getTradeNo, tradeNo));
        return Optional.ofNullable(entity).map(this::toPaymentDetail);
    }

    // ==================== P3：列退款異常 ====================

    public ListRefundsForAdminData listRefunds(ListRefundsForAdminRequest req) {
        int pageNum = parseIntOrDefault(req.getPage(), 1);
        int pageSize = Math.min(parseIntOrDefault(req.getPageSize(), 20), MAX_PAGE_SIZE);

        LambdaQueryWrapper<RefundTransactionEntity> q = new LambdaQueryWrapper<>();

        if (req.getAbnormalOnly()) {
            // 異常 = FAILED 或 PROCESSING 超時 24h
            LocalDateTime processingCutoff = LocalDateTime.now().minusHours(24);
            q.and(w -> w.eq(RefundTransactionEntity::getStatus, RefundStatus.FAILED.value())
                    .or(inner -> inner
                            .eq(RefundTransactionEntity::getStatus, RefundStatus.PROCESSING.value())
                            .lt(RefundTransactionEntity::getUpdatedAt, processingCutoff)));
        }
        if (StringUtils.hasText(req.getProviderStatus())) {
            q.eq(RefundTransactionEntity::getProviderStatus, req.getProviderStatus());
        }
        if (StringUtils.hasText(req.getStatus())) {
            List<String> list = Arrays.asList(req.getStatus().split(","));
            q.in(RefundTransactionEntity::getStatus, list);
        }
        if (StringUtils.hasText(req.getTradeNo())) {
            q.eq(RefundTransactionEntity::getTradeNo, req.getTradeNo());
        }
        if (StringUtils.hasText(req.getOrderNo())) {
            q.eq(RefundTransactionEntity::getOrderId, req.getOrderNo());
        }
        LocalDateTime createdFrom = parseIso(req.getCreatedFrom());
        LocalDateTime createdTo = parseIso(req.getCreatedTo());
        if (createdFrom != null) q.ge(RefundTransactionEntity::getCreatedAt, createdFrom);
        if (createdTo != null) q.lt(RefundTransactionEntity::getCreatedAt, createdTo);

        applyRefundSort(q, normalizeSort(req.getSort()));

        Page<RefundTransactionEntity> page = new Page<>(pageNum, pageSize);
        var result = refundMapper.selectPage(page, q);

        ListRefundsForAdminData.Builder b = ListRefundsForAdminData.newBuilder()
                .setTotal(String.valueOf(result.getTotal()))
                .setPage(String.valueOf(result.getCurrent()))
                .setPageSize(String.valueOf(result.getSize()));
        for (RefundTransactionEntity e : result.getRecords()) {
            b.addRecords(toRefundSummary(e));
        }
        return b.build();
    }

    // ==================== P4：退款詳情 ====================

    public Optional<AdminPaymentRefundSummary> getRefundDetail(String refundNo) {
        if (!StringUtils.hasText(refundNo)) return Optional.empty();
        RefundTransactionEntity entity = refundMapper.selectOne(
                new LambdaQueryWrapper<RefundTransactionEntity>()
                        .eq(RefundTransactionEntity::getRefundNo, refundNo));
        return Optional.ofNullable(entity).map(this::toRefundSummary);
    }

    // ==================== P5：列撥款異常 ====================

    public ListGrantsForAdminData listGrants(ListGrantsForAdminRequest req) {
        int pageNum = parseIntOrDefault(req.getPage(), 1);
        int pageSize = Math.min(parseIntOrDefault(req.getPageSize(), 20), MAX_PAGE_SIZE);

        LambdaQueryWrapper<PaymentTransactionEntity> q = new LambdaQueryWrapper<>();

        // 本清單只關心有嘗試撥款過的紀錄：granted_at IS NOT NULL 或 grant_status IS NOT NULL
        q.and(w -> w.isNotNull(PaymentTransactionEntity::getGrantedAt)
                .or()
                .isNotNull(PaymentTransactionEntity::getGrantStatus));

        if (req.getAbnormalOnly()) {
            // 異常：grant_status 存在且非 SUCCESS。
            // PaymentApplicationService.grantPayment 成功時寫 PayUniConstants.RESPONSE_SUCCESS（"SUCCESS"），
            // 失敗時寫 PayUni 回傳的 tradeStatus 或 "FAILED"。
            q.isNotNull(PaymentTransactionEntity::getGrantStatus)
             .ne(PaymentTransactionEntity::getGrantStatus, PayUniConstants.RESPONSE_SUCCESS);
        }
        if (StringUtils.hasText(req.getGrantStatus())) {
            q.eq(PaymentTransactionEntity::getGrantStatus, req.getGrantStatus());
        }
        if (StringUtils.hasText(req.getOrderNo())) {
            q.eq(PaymentTransactionEntity::getOrderId, req.getOrderNo());
        }
        LocalDateTime grantedFrom = parseIso(req.getGrantedFrom());
        LocalDateTime grantedTo = parseIso(req.getGrantedTo());
        if (grantedFrom != null) q.ge(PaymentTransactionEntity::getGrantedAt, grantedFrom);
        if (grantedTo != null) q.lt(PaymentTransactionEntity::getGrantedAt, grantedTo);

        applyPaymentSort(q, normalizeSort(req.getSort()));

        Page<PaymentTransactionEntity> page = new Page<>(pageNum, pageSize);
        var result = paymentMapper.selectPage(page, q);

        ListGrantsForAdminData.Builder b = ListGrantsForAdminData.newBuilder()
                .setTotal(String.valueOf(result.getTotal()))
                .setPage(String.valueOf(result.getCurrent()))
                .setPageSize(String.valueOf(result.getSize()));
        for (PaymentTransactionEntity e : result.getRecords()) {
            b.addRecords(toGrantSummary(e));
        }
        return b.build();
    }

    // ==================== 轉換 ====================

    private AdminPaymentTransactionSummary toPaymentSummary(PaymentTransactionEntity e) {
        return AdminPaymentTransactionSummary.newBuilder()
                .setTradeNo(nullToEmpty(e.getTradeNo()))
                .setOrderNo(nullToEmpty(e.getOrderId()))
                .setBuyerId(nullToEmpty(e.getBuyerId()))
                .setMerId(nullToEmpty(e.getMerId()))
                .setAmount(e.getAmount() == null ? "" : String.valueOf(e.getAmount()))
                .setPaymentType(nullToEmpty(e.getPaymentType()))
                .setStatus(nullToEmpty(e.getStatus()))
                .setProviderStatus(nullToEmpty(e.getProviderStatus()))
                .setProviderMessage(nullToEmpty(e.getProviderMessage()))
                .setProviderTradeNo(nullToEmpty(e.getProviderTradeNo()))
                .setCreatedAt(formatIso(e.getCreatedAt()))
                .setUpdatedAt(formatIso(e.getUpdatedAt()))
                .build();
    }

    private AdminPaymentTransactionDetail toPaymentDetail(PaymentTransactionEntity e) {
        return AdminPaymentTransactionDetail.newBuilder()
                .setTradeNo(nullToEmpty(e.getTradeNo()))
                .setOrderNo(nullToEmpty(e.getOrderId()))
                .setBuyerId(nullToEmpty(e.getBuyerId()))
                .setMerId(nullToEmpty(e.getMerId()))
                .setAmount(e.getAmount() == null ? "" : String.valueOf(e.getAmount()))
                .setPaymentType(nullToEmpty(e.getPaymentType()))
                .setProductName(nullToEmpty(e.getProductName()))
                .setStatus(nullToEmpty(e.getStatus()))
                .setProviderStatus(nullToEmpty(e.getProviderStatus()))
                .setProviderMessage(nullToEmpty(e.getProviderMessage()))
                .setProviderTradeNo(nullToEmpty(e.getProviderTradeNo()))
                .setAtmBankCode(nullToEmpty(e.getAtmBankCode()))
                .setAtmVirtualAccount(nullToEmpty(e.getAtmVirtualAccount()))
                .setAtmExpireDate(e.getAtmExpireDate() == null ? "" : e.getAtmExpireDate().toString())
                .setPayTime(formatIso(e.getPayTime()))
                .setProviderCreateAt(formatIso(e.getProviderCreateAt()))
                .setProviderPaidAt(formatIso(e.getProviderPaidAt()))
                .setExpireAt(formatIso(e.getExpireAt()))
                .setClosedAt(formatIso(e.getClosedAt()))
                .setGrantedAt(formatIso(e.getGrantedAt()))
                .setGrantStatus(nullToEmpty(e.getGrantStatus()))
                .setGrantMessage(nullToEmpty(e.getGrantMessage()))
                .setIsDuplicate(Boolean.TRUE.equals(e.getIsDuplicate()) ? "true" : "false")
                .setDuplicateOfPaymentId(e.getDuplicateOfPaymentId() == null ? "" : String.valueOf(e.getDuplicateOfPaymentId()))
                .setDuplicateSequence(e.getDuplicateSequence() == null ? "" : String.valueOf(e.getDuplicateSequence()))
                .setRescuedAt(formatIso(e.getRescuedAt()))
                .setRescueAction(nullToEmpty(e.getRescueAction()))
                .setCreatedAt(formatIso(e.getCreatedAt()))
                .setUpdatedAt(formatIso(e.getUpdatedAt()))
                .build();
    }

    private AdminPaymentRefundSummary toRefundSummary(RefundTransactionEntity e) {
        return AdminPaymentRefundSummary.newBuilder()
                .setRefundNo(nullToEmpty(e.getRefundNo()))
                .setTradeNo(nullToEmpty(e.getTradeNo()))
                .setOrderNo(nullToEmpty(e.getOrderId()))
                .setAmount(e.getAmount() == null ? "" : String.valueOf(e.getAmount()))
                .setStatus(nullToEmpty(e.getStatus()))
                .setProviderStatus(nullToEmpty(e.getProviderStatus()))
                .setProviderMessage(nullToEmpty(e.getProviderMessage()))
                .setProviderRefundNo(nullToEmpty(e.getProviderRefundNo()))
                .setReason(nullToEmpty(e.getReason()))
                .setCompletedAt(formatIso(e.getCompletedAt()))
                .setCreatedAt(formatIso(e.getCreatedAt()))
                .setUpdatedAt(formatIso(e.getUpdatedAt()))
                .build();
    }

    private AdminGrantSummary toGrantSummary(PaymentTransactionEntity e) {
        return AdminGrantSummary.newBuilder()
                .setTradeNo(nullToEmpty(e.getTradeNo()))
                .setOrderNo(nullToEmpty(e.getOrderId()))
                .setAmount(e.getAmount() == null ? "" : String.valueOf(e.getAmount()))
                .setGrantStatus(nullToEmpty(e.getGrantStatus()))
                .setGrantMessage(nullToEmpty(e.getGrantMessage()))
                .setGrantedAt(formatIso(e.getGrantedAt()))
                .setProviderTradeNo(nullToEmpty(e.getProviderTradeNo()))
                .setCreatedAt(formatIso(e.getCreatedAt()))
                .setUpdatedAt(formatIso(e.getUpdatedAt()))
                .build();
    }

    // ==================== 排序 ====================

    private String normalizeSort(String sort) {
        if (!StringUtils.hasText(sort)) return "updated_at_desc";
        String s = sort.trim().toLowerCase();
        return switch (s) {
            case "updated_at_desc", "updated_at_asc", "created_at_desc", "created_at_asc" -> s;
            default -> "updated_at_desc";
        };
    }

    private void applyPaymentSort(LambdaQueryWrapper<PaymentTransactionEntity> q, String sort) {
        switch (sort) {
            case "updated_at_asc" -> q.orderByAsc(PaymentTransactionEntity::getUpdatedAt);
            case "created_at_desc" -> q.orderByDesc(PaymentTransactionEntity::getCreatedAt);
            case "created_at_asc" -> q.orderByAsc(PaymentTransactionEntity::getCreatedAt);
            default -> q.orderByDesc(PaymentTransactionEntity::getUpdatedAt);
        }
    }

    private void applyRefundSort(LambdaQueryWrapper<RefundTransactionEntity> q, String sort) {
        switch (sort) {
            case "updated_at_asc" -> q.orderByAsc(RefundTransactionEntity::getUpdatedAt);
            case "created_at_desc" -> q.orderByDesc(RefundTransactionEntity::getCreatedAt);
            case "created_at_asc" -> q.orderByAsc(RefundTransactionEntity::getCreatedAt);
            default -> q.orderByDesc(RefundTransactionEntity::getUpdatedAt);
        }
    }

    // ==================== 工具 ====================

    private int parseIntOrDefault(String s, int fallback) {
        if (!StringUtils.hasText(s)) return fallback;
        try {
            int v = Integer.parseInt(s.trim());
            return v > 0 ? v : fallback;
        } catch (NumberFormatException e) {
            return fallback;
        }
    }

    private LocalDateTime parseIso(String s) {
        if (!StringUtils.hasText(s)) return null;
        try {
            return DateTimeUtils.parseIso8601(s);
        } catch (Exception e) {
            log.warn("payment.admin.parseIso.fail", s);
            return null;
        }
    }

    /** ISO 8601 UTC with Z suffix（docs/開發規範/Java 微服務開發規範.md §2.3）。 */
    private String formatIso(LocalDateTime t) {
        return DateTimeUtils.formatIso8601(t);
    }

    private String nullToEmpty(String s) {
        return s == null ? "" : s;
    }
}
