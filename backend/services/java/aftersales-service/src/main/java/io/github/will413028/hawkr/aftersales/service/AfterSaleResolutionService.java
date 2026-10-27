package io.github.will413028.hawkr.aftersales.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.aftersales.config.AfterSalesConstants;
import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleResolutionEntity;
import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import io.github.will413028.hawkr.aftersales.exception.AfterSalesBusinessException;
import io.github.will413028.hawkr.aftersales.grpc.client.CouponGrpcClient;
import io.github.will413028.hawkr.aftersales.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleResolutionMapper;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.utils.code.AfterSalesEventType;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.*;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.coupon.grpc.GrantSellerCompensationCouponResponse;
import io.github.will413028.hawkr.payment.grpc.CreateRefundResponse;
import io.github.will413028.hawkr.shipping.grpc.CreateReturnShipmentResponse;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.LocalDateTime;
import java.util.Map;
import java.util.Set;

/**
 * 售後決議服務。
 * 處理賣家對售後案件的 5 種決策：退貨退款、部分退款、全額退款（免退貨）、補償優惠券、拒絕。
 */
@Service
@RequiredArgsConstructor
public class AfterSaleResolutionService {

    private static final ObsLogger log = ObsLogger.forClass(AfterSaleResolutionService.class);

    private final AfterSaleCaseMapper caseMapper;
    private final AfterSaleResolutionMapper resolutionMapper;
    private final RefundTaskMapper refundTaskMapper;
    private final ShippingGrpcClient shippingGrpcClient;
    private final CouponGrpcClient couponGrpcClient;
    private final AfterSaleAuditService auditService;
    private final AfterSalesProperties properties;
    private final AfterSalesEventPublisher afterSalesEventPublisher;
    private final AfterSaleCaseService caseService;
    private final io.github.will413028.hawkr.aftersales.grpc.client.PaymentGrpcClient paymentGrpcClient;

    /** ATM 付款方式（Order DB 存 value） */
    private static final String ATM_PAYMENT_METHOD = PaymentMethod.ATM.value();

    /** 允許決議的案件狀態：僅待處理(0)（reopen 已於 #1228 淘汰，65 不再可達） */
    private static final Set<String> RESOLVABLE_STATUSES = Set.of(
            AfterSaleCaseStatus.PENDING.value()
    );

    /**
     * 賣家決議處理。
     *
     * @param storeId               賣家 store_id（Gateway sellerid resolver 解析後注入）
     * @param caseNo                售後單編號
     * @param resolutionType        決議類型（AfterSaleResolutionType code）
     * @param refundAmount          退款金額（部分退款用）
     * @param returnShippingMethod  退貨配送方式（退貨退款用）
     * @param rejectReason          拒絕原因 code（拒絕用）
     * @param rejectMessage         拒絕說明（拒絕用，可選）
     * @param couponAmount          優惠券面額（優惠券補償用）
     * @param couponMinSpend        優惠券最低消費（優惠券補償用）
     * @param couponMessage         優惠券附言（優惠券補償用，可選）
     */
    @Transactional
    public void resolve(Long storeId, String caseNo, String resolutionType,
                        Long refundAmount, String returnShippingMethod,
                        String rejectReason, String rejectMessage,
                        Long couponAmount, Long couponMinSpend, String couponMessage) {
        // 1. 驗證決議類型
        CodeMetaValidator.requireValidValue(AfterSaleResolutionType.class, resolutionType, "resolutionType");

        // 2. 查詢並鎖定案件
        AfterSaleCaseEntity caseEntity = caseMapper.selectOne(
                new LambdaQueryWrapper<AfterSaleCaseEntity>()
                        .eq(AfterSaleCaseEntity::getCaseNo, caseNo)
        );
        if (caseEntity == null) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_NOT_FOUND);
        }

        // 3. 驗證賣家身分
        if (!caseEntity.getStoreId().equals(storeId)) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_NOT_OWNER);
        }

        // §10 凍結檢查：申訴中禁止賣家處置
        caseService.assertNotUnderAppeal(caseEntity);

        // 4. 驗證案件狀態
        if (!RESOLVABLE_STATUSES.contains(caseEntity.getStatus())) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_INVALID_STATUS);
        }

        // 5. 依決議類型分流處理
        AfterSaleResolutionType type = AfterSaleResolutionType.fromValue(resolutionType);

        // §2A.1.4：零元訂單沒有金流退款概念，禁止所有「退款型」決議（含 RETURN_REFUND）。
        // 禁止 RETURN_REFUND 的理由：售後鏈路 refund.completed 在 order-service 端會把訂單標
        // PARTIALLY_REFUNDED / FULLY_REFUNDED 並寫 refund financial_log，但不歸還優惠券；
        // 對沒收錢的零元訂單，「退貨退款」會讓買家退商品卻沒拿回券，業務上不公平。
        // 僅允許 COUPON_COMPENSATION（補一張新券）、REJECT（拒絕）。
        if (PaymentMethod.ZERO_AMOUNT.value().equals(caseEntity.getPaymentMethod())
                && (type == AfterSaleResolutionType.RETURN_REFUND
                        || type == AfterSaleResolutionType.PARTIAL_REFUND
                        || type == AfterSaleResolutionType.FULL_REFUND_NO_RETURN)) {
            throw new AfterSalesBusinessException(
                    GrpcResponseCode.AFTERSALE_RESOLUTION_NOT_ALLOWED_FOR_ZERO_AMOUNT, type.label());
        }

        switch (type) {
            case RETURN_REFUND -> handleReturnRefund(caseEntity, storeId, returnShippingMethod);
            case PARTIAL_REFUND -> handlePartialRefund(caseEntity, storeId, refundAmount);
            case FULL_REFUND_NO_RETURN -> handleFullRefundNoReturn(caseEntity, storeId);
            case COUPON_COMPENSATION -> handleCouponCompensation(caseEntity, storeId, couponAmount, couponMinSpend, couponMessage);
            case REJECT -> handleReject(caseEntity, storeId, rejectReason, rejectMessage);
            // INSPECTION_DEDUCTION 僅限 confirmReceipt 路徑插入（§2.5.2 / §5.3.3）
            case INSPECTION_DEDUCTION -> throw new AfterSalesBusinessException(GrpcResponseCode.INVALID_PARAM);
        }
    }

    // ==================== 各類型處理 ====================

    /**
     * 退貨退款：建立退貨物流 → 更新狀態為待退貨。
     */
    private void handleReturnRefund(AfterSaleCaseEntity caseEntity, Long storeId,
                                     String returnShippingMethod) {
        // §5.2.1.1 必須是合法 ShippingMethod value，禁止字面 code（如 "SEVEN_ELEVEN_STORE_PICKUP"）
        // null / blank 由 CodeMetaValidator 統一拋 INVALID_PARAM
        CodeMetaValidator.requireValidValue(ShippingMethod.class, returnShippingMethod, "return_shipping_method");

        // §5.2.1.2 enforcement：return 必須 = forward；Uber 暫不支援
        String forwardMethod = caseEntity.getShippingMethod();
        if (ShippingMethod.UBER_INSTANT.value().equals(forwardMethod)) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_RETURN_NOT_SUPPORTED_FOR_UBER);
        }
        if (!returnShippingMethod.equals(forwardMethod)) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_RETURN_METHOD_MISMATCH);
        }

        String caseNo = caseEntity.getCaseNo();

        // 呼叫 Shipping 建立退貨物流
        CreateReturnShipmentResponse shipResp;
        if (ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value().equals(returnShippingMethod)) {
            shipResp = shippingGrpcClient.createReturnCvsShipment(
                    caseEntity.getOrderNo(),
                    ProtoConversionUtils.formatLong(caseEntity.getStoreId()),
                    caseEntity.getMerId(),
                    caseEntity.getBuyerName(),
                    caseEntity.getBuyerPhone(),
                    caseEntity.getDeliveryTemp(),
                    ProtoConversionUtils.formatLong(caseEntity.getRefundableAmount())
            );
        } else {
            shipResp = shippingGrpcClient.createReturnHomeShipment(
                    caseEntity.getOrderNo(),
                    ProtoConversionUtils.formatLong(caseEntity.getStoreId()),
                    caseEntity.getMerId(),
                    caseEntity.getBuyerName(),
                    caseEntity.getBuyerPhone(),
                    caseEntity.getBuyerCity(),
                    caseEntity.getBuyerDistrict(),
                    caseEntity.getBuyerAddressDetail(),
                    caseEntity.getBuyerPostalCode(),
                    caseEntity.getDeliveryTemp()
            );
        }

        // envelope-code 檢查（依《Java gRPC 錯誤處理規範》§7.0 現行約定）：
        // shipping-service 建單失敗以 envelope code≠0 回傳（onNext，非 StatusRuntimeException），必須檢查。
        // 屬「無不可逆前置」情境 → throw 透傳下游訊息，@Transactional rollback，case 維持 PENDING
        // 不推進 RETURN_PENDING（#1150 同類：原本不檢查 code 直接取 getData() 推進，物流單實際未建成）。
        if (shipResp.getCode() != GrpcResponseCode.SUCCESS.code()) {
            throw new AfterSalesBusinessException(
                    GrpcResponseCode.RETURN_SHIPMENT_FAILED, shipResp.getMessage());
        }

        // 插入決議記錄
        AfterSaleResolutionEntity resolution = new AfterSaleResolutionEntity();
        resolution.setId(TsidUtil.generate());
        resolution.setCaseNo(caseNo);
        resolution.setResolutionType(AfterSaleResolutionType.RETURN_REFUND.value());
        resolution.setRefundAmount(caseEntity.getRefundableAmount());
        resolution.setReturnShippingMethod(returnShippingMethod);
        resolution.setResolvedBy(storeId);
        resolutionMapper.insert(resolution);

        // §2.1 INV-3：mapper 自訂 SQL transition（PENDING → RETURN_PENDING）
        // §5.3.8 2026-06-15 新增：同 SQL stamp ship_back_deadline_at = NOW + return.timeout-days（預設 7 天）
        String shipTradeNo = shipResp.getData().getShipTradeNo();
        java.time.LocalDateTime shipBackDeadlineAt = DateTimeUtils.nowUtc()
                .plusDays(properties.getReturnConfig().getTimeoutDays());
        int updated = caseMapper.markReturnPending(
                caseNo,
                shipResp.getData().getShipmentNo(),
                returnShippingMethod,
                (shipTradeNo != null && !shipTradeNo.isBlank()) ? shipTradeNo : null,
                shipBackDeadlineAt);
        if (updated == 0) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_INVALID_STATUS);
        }

        auditService.log(caseNo, null, storeId, AfterSaleAuditActor.SELLER.value(), AfterSaleAuditAction.RESOLVE_RETURN_REFUND.value(),
                Map.of("returnShippingMethod", returnShippingMethod,
                        "shipmentNo", shipResp.getData().getShipmentNo()));

        log.info("AfterSaleResolutionService.returnRefund.success",
                "退貨退款決議完成, shipmentNo=" + shipResp.getData().getShipmentNo(), "caseNo", caseNo);

        afterSalesEventPublisher.publishCaseTransition(
                AfterSalesEventType.CASE_RETURN_APPROVED.value(),
                MqConstants.EVENT_TYPE_AFTERSALES_CASE_RETURN_APPROVED,
                caseNo, caseEntity.getOrderNo(),
                caseEntity.getBuyerId(), caseEntity.getStoreId(),
                "seller");
    }

    /**
     * 部分退款：驗證金額 → 建立退款任務 → 更新狀態。
     */
    private void handlePartialRefund(AfterSaleCaseEntity caseEntity, Long storeId, Long refundAmount) {
        String caseNo = caseEntity.getCaseNo();

        if (refundAmount == null || refundAmount <= 0) {
            throw new IllegalArgumentException("退款金額必須大於 0");
        }
        if (refundAmount > caseEntity.getRefundableAmount()) {
            throw new AfterSalesBusinessException(GrpcResponseCode.REFUND_AMOUNT_EXCEEDED);
        }

        // 插入決議記錄
        AfterSaleResolutionEntity resolution = new AfterSaleResolutionEntity();
        resolution.setId(TsidUtil.generate());
        resolution.setCaseNo(caseNo);
        resolution.setResolutionType(AfterSaleResolutionType.PARTIAL_REFUND.value());
        resolution.setRefundAmount(refundAmount);
        resolution.setResolvedBy(storeId);
        resolutionMapper.insert(resolution);

        // 建立退款任務（部分退款 outcome）
        boolean isAtm = ATM_PAYMENT_METHOD.equals(caseEntity.getPaymentMethod());
        RefundTaskEntity refundTask = createRefundTask(caseEntity, refundAmount, isAtm, resolution.getId(),
                AfterSalePlatformDecisionOutcome.PARTIAL_REFUND);

        // §2.1 INV-3：mapper 自訂 SQL transition（PENDING → REFUND_PROCESSING）
        int updated = caseMapper.markRefundProcessingFromResolution(caseNo);
        if (updated == 0) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_INVALID_STATUS);
        }

        // ATM 需等待買家操作退款頁面（§2.3.1 INV-5：createRefund 預建失敗時 task=FAILED，不發等待頁事件）
        if (isAtm && WorkerTaskStatus.AWAITING_BUYER_REFUND_PAGE.value().equals(refundTask.getStatus())) {
            afterSalesEventPublisher.publishAwaitingBuyerRefundPage(
                    caseEntity.getOrderNo(), caseNo, refundTask.getId(), caseEntity.getBuyerId());
        }

        auditService.log(caseNo, null, storeId, AfterSaleAuditActor.SELLER.value(), AfterSaleAuditAction.RESOLVE_PARTIAL_REFUND.value(),
                Map.of("refundAmount", refundAmount, "isAtm", isAtm));

        log.info("AfterSaleResolutionService.partialRefund.success",
                "部分退款決議完成, refundAmount=" + refundAmount, "caseNo", caseNo);

        afterSalesEventPublisher.publishCaseTransition(
                AfterSalesEventType.CASE_REFUND_APPROVED.value(),
                MqConstants.EVENT_TYPE_AFTERSALES_CASE_REFUND_APPROVED,
                caseNo, caseEntity.getOrderNo(),
                caseEntity.getBuyerId(), caseEntity.getStoreId(),
                "seller");
    }

    /**
     * 全額退款（免退貨）：退全額 → 建立退款任務 → 更新狀態。
     */
    private void handleFullRefundNoReturn(AfterSaleCaseEntity caseEntity, Long storeId) {
        String caseNo = caseEntity.getCaseNo();
        Long refundAmount = caseEntity.getRefundableAmount();

        // 插入決議記錄
        AfterSaleResolutionEntity resolution = new AfterSaleResolutionEntity();
        resolution.setId(TsidUtil.generate());
        resolution.setCaseNo(caseNo);
        resolution.setResolutionType(AfterSaleResolutionType.FULL_REFUND_NO_RETURN.value());
        resolution.setRefundAmount(refundAmount);
        resolution.setResolvedBy(storeId);
        resolutionMapper.insert(resolution);

        // 建立退款任務（全額退款 outcome）
        boolean isAtm = ATM_PAYMENT_METHOD.equals(caseEntity.getPaymentMethod());
        RefundTaskEntity refundTask = createRefundTask(caseEntity, refundAmount, isAtm, resolution.getId(),
                AfterSalePlatformDecisionOutcome.FULL_REFUND);

        // §2.1 INV-3：mapper 自訂 SQL transition（PENDING → REFUND_PROCESSING）
        int updated = caseMapper.markRefundProcessingFromResolution(caseNo);
        if (updated == 0) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_INVALID_STATUS);
        }

        // ATM 需等待買家操作退款頁面（§2.3.1 INV-5：createRefund 預建失敗時 task=FAILED，不發等待頁事件）
        if (isAtm && WorkerTaskStatus.AWAITING_BUYER_REFUND_PAGE.value().equals(refundTask.getStatus())) {
            afterSalesEventPublisher.publishAwaitingBuyerRefundPage(
                    caseEntity.getOrderNo(), caseNo, refundTask.getId(), caseEntity.getBuyerId());
        }

        auditService.log(caseNo, null, storeId, AfterSaleAuditActor.SELLER.value(), AfterSaleAuditAction.RESOLVE_FULL_REFUND_NO_RETURN.value(),
                Map.of("refundAmount", refundAmount, "isAtm", isAtm));

        log.info("AfterSaleResolutionService.fullRefundNoReturn.success",
                "全額退款（免退貨）決議完成, refundAmount=" + refundAmount, "caseNo", caseNo);

        afterSalesEventPublisher.publishCaseTransition(
                AfterSalesEventType.CASE_REFUND_APPROVED.value(),
                MqConstants.EVENT_TYPE_AFTERSALES_CASE_REFUND_APPROVED,
                caseNo, caseEntity.getOrderNo(),
                caseEntity.getBuyerId(), caseEntity.getStoreId(),
                "seller");
    }

    /**
     * 補償優惠券：驗證規則 → 呼叫 Coupon 發放 → 更新狀態為已結案。
     */
    private void handleCouponCompensation(AfterSaleCaseEntity caseEntity, Long storeId,
                                           Long couponAmount, Long couponMinSpend, String couponMessage) {
        String caseNo = caseEntity.getCaseNo();

        // 驗證集中化（依 coupon 設計書 §5.12）：
        // - 金額 > 0、業務規則（minSpend >= amount*5、minSpend - amount >= 100）由 coupon-service 統一驗
        // - 失敗時 coupon-service 回傳 INVALID_PARAM / COUPON_RULE_VIOLATED，會被下方 catch 透傳
        // - 此處只做 null 防禦避免 NPE 透傳到 RPC 層
        if (couponAmount == null || couponMinSpend == null) {
            throw new AfterSalesBusinessException(GrpcResponseCode.INVALID_PARAM);
        }

        // 呼叫 Coupon Service 發放補償優惠券
        GrantSellerCompensationCouponResponse couponResp = couponGrpcClient.grantSellerCompensationCoupon(
                ProtoConversionUtils.formatLong(caseEntity.getStoreId()),
                ProtoConversionUtils.formatLong(caseEntity.getBuyerId()),
                ProtoConversionUtils.formatLong(couponAmount),
                ProtoConversionUtils.formatLong(couponMinSpend),
                couponMessage != null ? couponMessage : "",
                caseNo
        );

        // envelope-code 檢查（依《Java gRPC 錯誤處理規範》§7.0 現行約定）：
        // coupon-service 業務失敗（amount<=0 / minSpend 規則違反等）以 envelope code≠0 回傳（onNext，非
        // StatusRuntimeException），caller 必須檢查。屬「無不可逆前置」情境 → throw 透傳下游訊息，
        // @Transactional rollback，case 維持 PENDING 不關案（#1150：原本不檢查 code 直接取
        // getData().getTemplateId()（失敗時為空字串）insert + 關案，導致案件 CLOSED 但買家無券）。
        if (couponResp.getCode() != GrpcResponseCode.SUCCESS.code()) {
            throw new AfterSalesBusinessException(
                    GrpcResponseCode.AFTERSALE_COUPON_COMPENSATION_FAILED, couponResp.getMessage());
        }

        String couponTemplateId = couponResp.getData().getTemplateId();

        // 插入決議記錄
        AfterSaleResolutionEntity resolution = new AfterSaleResolutionEntity();
        resolution.setId(TsidUtil.generate());
        resolution.setCaseNo(caseNo);
        resolution.setResolutionType(AfterSaleResolutionType.COUPON_COMPENSATION.value());
        resolution.setCouponAmount(couponAmount);
        resolution.setCouponMinSpend(couponMinSpend);
        resolution.setCouponMessage(couponMessage);
        resolution.setCouponTemplateId(couponTemplateId);
        resolution.setResolvedBy(storeId);
        resolutionMapper.insert(resolution);

        // §2.1 INV-3：mapper 自訂 SQL transition（PENDING → CLOSED；coupon 路徑專用）
        int updated = caseMapper.closeByCouponCompensation(caseNo, DateTimeUtils.nowUtc());
        if (updated == 0) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_INVALID_STATUS);
        }

        auditService.log(caseNo, null, storeId, AfterSaleAuditActor.SELLER.value(), AfterSaleAuditAction.RESOLVE_COUPON_COMPENSATION.value(),
                Map.of("couponAmount", couponAmount, "couponMinSpend", couponMinSpend,
                        "couponTemplateId", couponTemplateId));

        log.info("AfterSaleResolutionService.couponCompensation.success",
                "優惠券補償決議完成, couponTemplateId=" + couponTemplateId, "caseNo", caseNo);

        afterSalesEventPublisher.publishCaseTransition(
                AfterSalesEventType.CASE_COUPON_COMPENSATED.value(),
                MqConstants.EVENT_TYPE_AFTERSALES_CASE_COUPON_COMPENSATED,
                caseNo, caseEntity.getOrderNo(),
                caseEntity.getBuyerId(), caseEntity.getStoreId(),
                "seller");
    }

    /**
     * 拒絕售後：驗證拒絕原因 → 更新狀態為已拒絕。
     */
    private void handleReject(AfterSaleCaseEntity caseEntity, Long storeId,
                               String rejectReason, String rejectMessage) {
        String caseNo = caseEntity.getCaseNo();

        CodeMetaValidator.requireValidValue(AfterSaleRejectReason.class, rejectReason, "rejectReason");

        // 插入決議記錄
        AfterSaleResolutionEntity resolution = new AfterSaleResolutionEntity();
        resolution.setId(TsidUtil.generate());
        resolution.setCaseNo(caseNo);
        resolution.setResolutionType(AfterSaleResolutionType.REJECT.value());
        resolution.setRejectReason(rejectReason);
        resolution.setRejectMessage(rejectMessage);
        resolution.setResolvedBy(storeId);
        resolutionMapper.insert(resolution);

        // §2.1 INV-3：mapper 自訂 SQL transition（PENDING → REJECTED + stamp last_rejected_at）
        int updated = caseMapper.markRejectedByResolution(caseNo, DateTimeUtils.nowUtc());
        if (updated == 0) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_INVALID_STATUS);
        }

        auditService.log(caseNo, null, storeId, AfterSaleAuditActor.SELLER.value(), AfterSaleAuditAction.RESOLVE_REJECT.value(),
                Map.of("rejectReason", rejectReason));

        log.info("AfterSaleResolutionService.reject.success",
                "拒絕售後決議完成, reason=" + rejectReason, "caseNo", caseNo);

        afterSalesEventPublisher.publishCaseTransition(
                AfterSalesEventType.CASE_REJECTED.value(),
                MqConstants.EVENT_TYPE_AFTERSALES_CASE_REJECTED,
                caseNo, caseEntity.getOrderNo(),
                caseEntity.getBuyerId(), caseEntity.getStoreId(),
                "seller");
    }

    // ==================== 共用方法 ====================

    /**
     * 建立退款任務。ATM 付款方式初始狀態為「等待買家操作退款頁面」，其他為「待執行」。
     *
     * <p>ATM 路徑預建 {@code payment_refunds} row（status=REQUESTED），以供：
     * <ul>
     *   <li>{@code AfterSales.handleRefundPageReturn} 成功後呼叫 {@code ackAtmRefundAccepted} 有 row 可更新</li>
     *   <li>{@code PayUniRefundAcceptedConsumer} / {@code handleAtmRefundWebhook} 用 tradeNo 能找到 active refund</li>
     * </ul>
     * 冪等：idempotency key 用 {@code CASE:{caseNo}:{resolutionId}:{attempt}}，同 key 重複呼叫 Payment 回既有 row。
     * §2.2 業務 pivot (2026-05-14)：key 加 {@code :{attempt}} 後綴避免 buyer 重填時命中既有 row。
     * 首次建立 attempt=1。</p>
     */
    private RefundTaskEntity createRefundTask(AfterSaleCaseEntity caseEntity, Long refundAmount,
                                                boolean isAtm, Long resolutionId,
                                                AfterSalePlatformDecisionOutcome outcome) {
        short initialAttempt = 1;
        String externalRefId = null;
        // §2.3.1 INV-5：ATM createRefund 預建失敗時不 throw，建終態 FAILED refund_task 持久化 PayUni 原話
        boolean refundPrebuildFailed = false;
        String payuniStatus = null;
        String payuniMessage = null;
        String lastError = null;
        if (isAtm) {
            String idempotencyKey = "CASE:" + caseEntity.getCaseNo() + ":" + resolutionId
                    + ":" + initialAttempt;
            CreateRefundResponse resp = null;
            try {
                resp = paymentGrpcClient.createRefund(
                        caseEntity.getTransactionId(),
                        ProtoConversionUtils.formatLong(refundAmount),
                        "售後退款",
                        idempotencyKey);
            } catch (io.grpc.StatusRuntimeException e) {
                refundPrebuildFailed = true;
                lastError = "createRefund StatusRuntimeException: " + e.getMessage();
                log.warn("AfterSaleResolutionService.createRefundTask.createRefundUnavailable",
                        "ATM 預建退款 payment 不可用, caseNo=" + caseEntity.getCaseNo() + ", " + e.getMessage(),
                        "caseNo", caseEntity.getCaseNo());
            }
            if (!refundPrebuildFailed && resp.getCode() != 0) {
                refundPrebuildFailed = true;
                payuniStatus = resp.getData().getProviderStatus();
                payuniMessage = resp.getData().getProviderMessage();
                lastError = resp.getMessage();
                log.warn("AfterSaleResolutionService.createRefundTask.createRefundFailed",
                        "ATM 預建退款失敗, caseNo=" + caseEntity.getCaseNo() + ", code=" + resp.getCode()
                                + ", msg=" + resp.getMessage(), "caseNo", caseEntity.getCaseNo());
            } else if (!refundPrebuildFailed) {
                externalRefId = resp.getData().getRefundNo();
            }
        }

        RefundTaskEntity task = new RefundTaskEntity();
        task.setId(TsidUtil.generate());
        task.setCaseNo(caseEntity.getCaseNo());
        task.setOrderNo(caseEntity.getOrderNo());
        task.setTransactionId(caseEntity.getTransactionId());
        task.setRefundAmount(refundAmount);
        task.setPaymentMethod(caseEntity.getPaymentMethod());
        task.setMerId(caseEntity.getMerId());
        task.setRefundOutcome(outcome.value());
        task.setExternalRefId(externalRefId);
        if (refundPrebuildFailed) {
            // 終態 FAILED：Worker 不再撈，持久化 PayUni 原話進 admin 異常清單
            task.setStatus(WorkerTaskStatus.FAILED.value());
            task.setNextRetryTime(null);
            task.setPayuniStatus(payuniStatus);
            task.setPayuniMessage(payuniMessage);
            task.setLastError(lastError);
        } else {
            task.setStatus(isAtm
                    ? WorkerTaskStatus.AWAITING_BUYER_REFUND_PAGE.value()
                    : WorkerTaskStatus.PENDING.value());
        }
        task.setRetryCount(0);
        task.setMaxRetryCount(properties.getRefund().getMaxRetryCount());
        task.setCurrentAttempt(initialAttempt);
        refundTaskMapper.insert(task);
        return task;
    }
}
