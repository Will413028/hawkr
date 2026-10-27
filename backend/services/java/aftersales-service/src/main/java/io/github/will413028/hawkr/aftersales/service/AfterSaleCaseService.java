package io.github.will413028.hawkr.aftersales.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.aftersales.config.AfterSalesConstants;
import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseItemEntity;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleEvidenceEntity;
import io.github.will413028.hawkr.aftersales.exception.AfterSalesBusinessException;
import io.github.will413028.hawkr.aftersales.grpc.client.OrderGrpcClient;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseItemMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleEvidenceMapper;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.utils.code.AfterSalesEventType;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.*;
import io.github.will413028.hawkr.common.utils.code.helper.AfterSaleCaseStatusRules;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.order.grpc.GetOrderForAfterSalesResponse;
import io.github.will413028.hawkr.order.grpc.OrderAfterSalesView;
import io.github.will413028.hawkr.order.grpc.OrderItemForAfterSales;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.LocalDateTime;
import java.time.temporal.ChronoUnit;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * 售後案件核心服務。
 * 負責案件建立、補充舉證、取消等生命週期管理。
 */
@Service
@RequiredArgsConstructor
public class AfterSaleCaseService {

    private static final ObsLogger log = ObsLogger.forClass(AfterSaleCaseService.class);

    private final AfterSaleCaseMapper caseMapper;
    private final AfterSaleCaseItemMapper caseItemMapper;
    private final AfterSaleEvidenceMapper evidenceMapper;
    private final OrderGrpcClient orderGrpcClient;
    private final EvidenceService evidenceService;
    private final AfterSaleAuditService auditService;
    private final AfterSalesProperties properties;
    private final AfterSalesEventPublisher eventPublisher;

    /** 允許申請售後的訂單狀態：僅已送達(40)；確認收貨(50)後自動撥款，不再受理售後 */
    private static final Set<String> APPLY_ALLOWED_ORDER_STATUSES = Set.of(
            OrderStatus.DELIVERED.value()
    );

    /** 允許補充舉證 / 取消的案件狀態：僅待處理(0)（reopen 已於 #1228 淘汰，65 不再可達） */
    private static final Set<String> MODIFIABLE_CASE_STATUSES = Set.of(
            AfterSaleCaseStatus.PENDING.value()
    );

    /**
     * 建立售後案件。
     *
     * @param buyerId     買家 buyer_id（buyers PK surrogate，Gateway resolver 解析後傳入）
     * @param orderNo     訂單編號
     * @param reason      退款原因（RefundReason code）
     * @param description 問題描述
     * @param photoTokens 舉證圖片 staged tokens（可為 null 或空）
     * @return 建立的售後案件 Entity
     */
    @Transactional
    public AfterSaleCaseEntity createCase(Long buyerId, String orderNo, String reason,
                                           String description, List<String> photoTokens) {
        // 1. 驗證退款原因
        CodeMetaValidator.requireValidValue(RefundReason.class, reason, "reason");

        // 2. 查詢訂單資訊
        GetOrderForAfterSalesResponse orderResp = orderGrpcClient.getOrderForAfterSales(orderNo);
        // order-service 失敗時會拋 StatusRuntimeException，由上層 RPC catch-all 處理
        OrderAfterSalesView order = orderResp.getData();

        // 3. 驗證買家身分
        Long orderBuyerId = ProtoConversionUtils.parseLongRequired(order.getBuyerId(), "buyerId");
        if (!orderBuyerId.equals(buyerId)) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_NOT_OWNER);
        }

        // 4. 驗證訂單狀態（僅已送達；確認收貨後不可申請）
        if (!APPLY_ALLOWED_ORDER_STATUSES.contains(order.getStatus())) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_INVALID_STATUS);
        }

        // 5. 驗證送達時間 + 截止時限
        if (order.getDeliveredAt().isEmpty()) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_INVALID_STATUS);
        }
        LocalDateTime deliveredAt = ProtoConversionUtils.parseIso8601Required(order.getDeliveredAt(), "deliveredAt");
        int deadlineHours = properties.getCaseConfig().getApplyDeadlineHours();
        if (ChronoUnit.HOURS.between(deliveredAt, DateTimeUtils.nowUtc()) > deadlineHours) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_EXPIRED);
        }

        // 6. 檢查是否已有進行中的售後案件
        Long existingCount = caseMapper.selectCount(
                new LambdaQueryWrapper<AfterSaleCaseEntity>()
                        .eq(AfterSaleCaseEntity::getOrderNo, orderNo)
                        .notIn(AfterSaleCaseEntity::getStatus,
                                AfterSaleCaseStatus.CLOSED.value(),
                                AfterSaleCaseStatus.CANCELLED.value())
        );
        if (existingCount > 0) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_DUPLICATE);
        }

        // 7. 生成案件編號
        String caseNo = "A" + String.format("%09d", Math.abs(TsidUtil.generate() % 1_000_000_000L));

        // 8. 計算可退款金額 = orders.total_actual_amount（= PayUni 實收，已含運費與折扣，見 §2.5）
        Long refundableAmount = ProtoConversionUtils.parseLongRequired(order.getTotalActualAmount(), "totalActualAmount");
        Long shippingFee = ProtoConversionUtils.parseLongOrDefault(order.getShippingFee(), 0L);

        // 9. 建立案件 Entity
        AfterSaleCaseEntity caseEntity = new AfterSaleCaseEntity();
        caseEntity.setId(TsidUtil.generate());
        caseEntity.setCaseNo(caseNo);
        caseEntity.setOrderNo(orderNo);
        caseEntity.setBuyerId(orderBuyerId);
        caseEntity.setStoreId(ProtoConversionUtils.parseLongRequired(order.getStoreId(), "storeId"));
        caseEntity.setStatus(AfterSaleCaseStatus.PENDING.value());
        caseEntity.setReason(reason);
        caseEntity.setDescription(description);
        caseEntity.setRefundableAmount(refundableAmount);
        caseEntity.setShippingFee(shippingFee);
        caseEntity.setDeliveryTemp(order.getDeliveryTemp());
        caseEntity.setBuyerName(order.getBuyerName());
        caseEntity.setBuyerPhone(order.getBuyerPhone());
        caseEntity.setBuyerCity(order.getBuyerCity());
        caseEntity.setBuyerDistrict(order.getBuyerDistrict());
        caseEntity.setBuyerAddressDetail(order.getBuyerAddressDetail());
        caseEntity.setBuyerPostalCode(order.getBuyerPostalCode());
        caseEntity.setTransactionId(order.getTransactionId());
        caseEntity.setPaymentMethod(order.getPaymentMethod());
        caseEntity.setMerId(order.getMerId());
        caseEntity.setShippingMethod(order.getShippingMethod());
        caseEntity.setDeliveredAt(deliveredAt);
        // §5.10 賣家處理期限：建立後 N 小時內未決策由系統自動升級至 admin
        caseEntity.setProcessDeadline(DateTimeUtils.nowUtc()
                .plusHours(properties.getCaseConfig().getProcessDeadlineHours()));
        caseEntity.setAutoEscalated(Boolean.FALSE);

        caseMapper.insert(caseEntity);

        // 10. 建立案件商品明細
        for (OrderItemForAfterSales item : order.getItemsList()) {
            AfterSaleCaseItemEntity itemEntity = new AfterSaleCaseItemEntity();
            itemEntity.setId(TsidUtil.generate());
            itemEntity.setCaseNo(caseNo);
            itemEntity.setOrderItemId(ProtoConversionUtils.parseLongRequired(item.getId(), "orderItemId"));
            // non-catalog line（RFQ 成交）沒有平台 SKU ⇒ order-service 對 NULL 輸出空字串
            // （ProtoConversionUtils.formatLong 是 null-safe 的），這裡必須容忍並寫回 NULL。
            // ⚠️ 只有 sku_id 可缺：同段其餘數值欄位缺值代表上游壞了，一律維持 parseLongRequired。
            itemEntity.setSkuId(ProtoConversionUtils.parseLongOrNull(item.getSkuId()));
            itemEntity.setSkuName(item.getSkuName());
            itemEntity.setQuantity(ProtoConversionUtils.parseIntRequired(item.getQuantity(), "quantity"));
            // 從 OrderItemForAfterSales.unit_price（重命名自 original_price）讀取，aftersales 內部 entity 仍稱為 originalPrice
            itemEntity.setOriginalPrice(ProtoConversionUtils.parseLongRequired(item.getUnitPrice(), "unitPrice"));
            itemEntity.setActualPayAmount(ProtoConversionUtils.parseLongRequired(item.getActualPayAmount(), "actualPayAmount"));
            itemEntity.setImageObjectKey(item.getImageObjectKey());
            caseItemMapper.insert(itemEntity);
        }

        // 11. 處理舉證圖片
        evidenceService.finalizeAndInsert(caseNo, buyerId, photoTokens, AfterSaleEvidenceType.PHOTO.value());

        // 12. 稽核日誌
        auditService.log(caseNo, null, buyerId, AfterSaleAuditActor.BUYER.value(), AfterSaleAuditAction.CREATE_CASE.value(),
                Map.of("orderNo", orderNo, "reason", reason));

        log.info("AfterSaleCaseService.createCase.success",
                "售後案件建立成功, orderNo=" + orderNo + ", buyerId=" + buyerId, "caseNo", caseNo);

        // 發布主流程狀態變更事件供 notification-service 推播（best-effort）
        eventPublisher.publishCaseTransition(
                AfterSalesEventType.CASE_APPLIED.value(),
                MqConstants.EVENT_TYPE_AFTERSALES_CASE_APPLIED,
                caseNo, orderNo,
                caseEntity.getBuyerId(), caseEntity.getStoreId(),
                "buyer");

        return caseEntity;
    }

    /**
     * 補充舉證。
     *
     * @param buyerId        買家 buyer_id（buyers PK surrogate，Gateway resolver 解析後傳入）
     * @param caseNo         售後單編號
     * @param photoTokens    舉證圖片 staged tokens
     * @param additionalDesc 補充說明（可為 null）
     */
    @Transactional
    public void supplementEvidence(Long buyerId, String caseNo,
                                    List<String> photoTokens, String additionalDesc) {
        // 1. 查詢案件並驗證
        AfterSaleCaseEntity caseEntity = getCaseForBuyer(buyerId, caseNo);
        assertNotUnderAppeal(caseEntity);
        validateModifiableStatus(caseEntity);

        // 2. 檢查舉證數量上限
        int maxEvidence = properties.getCaseConfig().getMaxEvidenceCount();
        Long existingCount = evidenceMapper.selectCount(
                new LambdaQueryWrapper<AfterSaleEvidenceEntity>()
                        .eq(AfterSaleEvidenceEntity::getCaseNo, caseNo)
        );
        int newCount = (photoTokens != null) ? photoTokens.size() : 0;
        if (existingCount + newCount > maxEvidence) {
            throw new IllegalArgumentException(
                    String.format("舉證圖片數量超過上限（最多 %d 張，目前已有 %d 張）", maxEvidence, existingCount));
        }

        // 3. 處理新圖片
        evidenceService.finalizeAndInsert(caseNo, buyerId, photoTokens, AfterSaleEvidenceType.PHOTO.value());

        // 4. 補充描述（§2.1 INV-3：mapper 自訂 SQL，WHERE status IN ('0','65') 防 §5.10 worker race）
        if (additionalDesc != null && !additionalDesc.isBlank()) {
            String updatedDesc = (caseEntity.getDescription() == null ? "" : caseEntity.getDescription() + "\n")
                    + additionalDesc;
            int updated = caseMapper.appendDescription(caseNo, updatedDesc);
            if (updated == 0) {
                throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_INVALID_STATUS);
            }
        }

        // 5. 稽核日誌
        auditService.log(caseNo, null, buyerId, AfterSaleAuditActor.BUYER.value(), AfterSaleAuditAction.SUPPLEMENT_EVIDENCE.value(),
                Map.of("newPhotoCount", newCount, "hasAdditionalDesc", additionalDesc != null));

        log.info("AfterSaleCaseService.supplementEvidence.success",
                "補充舉證成功, newPhotos=" + newCount, "caseNo", caseNo);
    }

    /**
     * 買家取消售後案件。
     */
    @Transactional
    public void cancelCase(Long buyerId, String caseNo) {
        AfterSaleCaseEntity caseEntity = getCaseForBuyer(buyerId, caseNo);
        assertNotUnderAppeal(caseEntity);
        validateModifiableStatus(caseEntity);

        // §2.1 INV-3：mapper 自訂 SQL transition（PENDING → CANCELLED）
        int updated = caseMapper.markCancelledByBuyer(caseNo);
        if (updated == 0) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_INVALID_STATUS);
        }

        auditService.log(caseNo, null, buyerId, AfterSaleAuditActor.BUYER.value(), AfterSaleAuditAction.CANCEL_CASE.value(), null);

        log.info("AfterSaleCaseService.cancelCase.success",
                "買家取消售後案件, buyerId=" + buyerId, "caseNo", caseNo);

        eventPublisher.publishCaseTransition(
                AfterSalesEventType.CASE_CANCELLED_BY_BUYER.value(),
                MqConstants.EVENT_TYPE_AFTERSALES_CASE_CANCELLED_BY_BUYER,
                caseNo, caseEntity.getOrderNo(),
                caseEntity.getBuyerId(), caseEntity.getStoreId(),
                "buyer");
    }

    /** §2.1.1 / §8.2.1（#1225）確認收貨同步處置結果。 */
    public enum ConfirmReceiptCancelOutcome { NO_ACTIVE_CASE, TERMINATED, BLOCKED }

    /** 處置結果：outcome + caseNo（TERMINATED/BLOCKED 時帶）+ blockingStatus（BLOCKED 時帶擋住的 case status value）。 */
    public record ConfirmReceiptCancelResult(ConfirmReceiptCancelOutcome outcome, String caseNo, String blockingStatus) {}

    /**
     * §2.1.1 / §8.2.1（#1225）確認收貨同步處置進行中售後 case。
     * <p>
     * 由 order-service confirmReceipt 於完成訂單前同步呼叫。撈該訂單唯一活躍 case（duplicate guard 保證 ≤1），
     * 以 {@code markCancelledByConfirmReceipt} CAS（WHERE status IN 0,65）作為 A 桶權威定義：
     * <ul>
     *   <li>無活躍 case → {@code NO_ACTIVE_CASE}（訂單可完成）</li>
     *   <li>A 桶（CAS affected=1）→ 終結至 71 + audit + publishCaseTransition（自動 realtime + 通知）→ {@code TERMINATED}</li>
     *   <li>B 桶（active 但 CAS affected=0，已進退貨/退款/申訴或競態離開 0/65）→ 重讀分類 → {@code BLOCKED} + blockingStatus</li>
     * </ul>
     */
    @Transactional
    public ConfirmReceiptCancelResult cancelActiveCaseOnConfirmReceipt(Long buyerId, String orderNo) {
        // 撈該訂單所有 case，記憶體以 canonical isActive 過濾出唯一活躍 case（同訂單歷史可有多筆終態 case + ≤1 活躍）
        List<AfterSaleCaseEntity> cases = caseMapper.selectList(
                new LambdaQueryWrapper<AfterSaleCaseEntity>().eq(AfterSaleCaseEntity::getOrderNo, orderNo));
        AfterSaleCaseEntity active = cases.stream()
                .filter(c -> AfterSaleCaseStatusRules.isActive(
                        AfterSaleCaseStatus.fromValue(c.getStatus()), c.getAppealState()))
                .findFirst().orElse(null);
        if (active == null) {
            return new ConfirmReceiptCancelResult(ConfirmReceiptCancelOutcome.NO_ACTIVE_CASE, null, null);
        }

        String caseNo = active.getCaseNo();
        // A 桶 CAS（WHERE status IN '0','65'）為 A 桶權威定義，無需另查 status set
        int updated = caseMapper.markCancelledByConfirmReceipt(caseNo);
        if (updated == 1) {
            auditService.log(caseNo, null, buyerId, AfterSaleAuditActor.BUYER.value(),
                    AfterSaleAuditAction.CANCELLED_BY_CONFIRM_RECEIPT.value(),
                    Map.of("orderNo", orderNo, "priorStatus", active.getStatus()));
            log.info("AfterSaleCaseService.cancelActiveCaseOnConfirmReceipt.terminated",
                    "確認收貨自動取消售後案件, orderNo=" + orderNo + ", priorStatus=" + active.getStatus(), "caseNo", caseNo);
            eventPublisher.publishCaseTransition(
                    AfterSalesEventType.CASE_CANCELLED_BY_CONFIRM_RECEIPT.value(),
                    MqConstants.EVENT_TYPE_AFTERSALES_CASE_CANCELLED_BY_CONFIRM_RECEIPT,
                    caseNo, orderNo,
                    active.getBuyerId(), active.getStoreId(),
                    "buyer");
            return new ConfirmReceiptCancelResult(ConfirmReceiptCancelOutcome.TERMINATED, caseNo, null);
        }

        // affected=0：非 A 桶 或 競態剛離開 0/65。重讀分類（§8.2.1 競態矩陣）
        AfterSaleCaseEntity reread = caseMapper.selectOne(
                new LambdaQueryWrapper<AfterSaleCaseEntity>().eq(AfterSaleCaseEntity::getCaseNo, caseNo));
        if (reread == null || !AfterSaleCaseStatusRules.isActive(
                AfterSaleCaseStatus.fromValue(reread.getStatus()), reread.getAppealState())) {
            return new ConfirmReceiptCancelResult(ConfirmReceiptCancelOutcome.NO_ACTIVE_CASE, null, null);
        }
        return new ConfirmReceiptCancelResult(ConfirmReceiptCancelOutcome.BLOCKED, caseNo, reread.getStatus());
    }

    /**
     * §10 凍結檢查：申訴中（appeal_state != NONE）禁止所有改寫操作。
     * 由買家 cancel/supplement 與賣家 resolve/confirm_receipt 入口呼叫。
     */
    public void assertNotUnderAppeal(AfterSaleCaseEntity caseEntity) {
        String state = caseEntity.getAppealState();
        if (state != null && !AfterSaleAppealState.NONE.value().equals(state)) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_CASE_UNDER_APPEAL);
        }
    }

    /**
     * 依案件編號查詢，不存在則拋 NOT_FOUND。
     */
    public AfterSaleCaseEntity getCase(String caseNo) {
        AfterSaleCaseEntity caseEntity = caseMapper.selectOne(
                new LambdaQueryWrapper<AfterSaleCaseEntity>()
                        .eq(AfterSaleCaseEntity::getCaseNo, caseNo)
        );
        if (caseEntity == null) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_NOT_FOUND);
        }
        return caseEntity;
    }

    /**
     * 依案件編號查詢，並驗證買家身分。
     */
    public AfterSaleCaseEntity getCaseForBuyer(Long buyerId, String caseNo) {
        AfterSaleCaseEntity caseEntity = getCase(caseNo);
        if (!caseEntity.getBuyerId().equals(buyerId)) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_NOT_OWNER);
        }
        return caseEntity;
    }

    /**
     * 依案件編號查詢，並驗證賣家身分。
     */
    public AfterSaleCaseEntity getCaseForSeller(Long storeId, String caseNo) {
        AfterSaleCaseEntity caseEntity = getCase(caseNo);
        if (!caseEntity.getStoreId().equals(storeId)) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_NOT_OWNER);
        }
        return caseEntity;
    }

    // ==================== 私有方法 ====================

    /**
     * 驗證案件是否處於可修改狀態（待處理 / 已重啟）。
     */
    private void validateModifiableStatus(AfterSaleCaseEntity caseEntity) {
        if (!MODIFIABLE_CASE_STATUSES.contains(caseEntity.getStatus())) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_INVALID_STATUS);
        }
    }

}
