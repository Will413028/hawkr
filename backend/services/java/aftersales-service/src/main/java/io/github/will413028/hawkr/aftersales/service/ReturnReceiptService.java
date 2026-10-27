package io.github.will413028.hawkr.aftersales.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.utils.code.AfterSalesEventType;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditActor;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditAction;
import io.github.will413028.hawkr.common.utils.code.AfterSaleEvidenceType;
import io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome;
import io.github.will413028.hawkr.common.utils.code.AfterSaleResolutionType;
import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleResolutionEntity;
import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import io.github.will413028.hawkr.aftersales.exception.AfterSalesBusinessException;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleResolutionMapper;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus;
import io.github.will413028.hawkr.payment.grpc.CreateRefundResponse;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * 退貨收貨服務（§5.3.7 / §5.3.10 / §5.3.12）。
 *
 * <p>四條路徑（賣家手動 ／ SYSTEM 雙逾時 ／ Admin 強制）共用同一段 tx 內實作
 * （{@code createRefundTaskAndAdvanceStatus}），只在「前置狀態檢查 / mapper transition 選擇 /
 * actor 寫入 audit / publish event」分流：
 * <ul>
 *   <li>{@link #confirmReceipt} — 賣家手動（actor=SELLER，前置 status=27）</li>
 *   <li>{@link #autoConfirmReceipt} — scheduler 自動（actor=SYSTEM，前置 status=27 + auto_confirm_at 到期，固定全額不扣款）</li>
 *   <li>{@link #autoCompleteFromAtStore} — scheduler 自動（actor=SYSTEM，前置 status=26 + store_pickup_deadline_at 到期）</li>
 *   <li>{@link #forceConfirmReceiptByAdmin} — Admin 強制（actor=ADMIN，前置 status ∈ 20/25/26/27/28）</li>
 * </ul>
 *
 * <h2>issue #2294 B5/B6/B7：tx 內不得有<b>身分反解</b>的 gRPC</h2>
 *
 * <p>⚠️ <b>標題的限定詞是認真的（issue #2294 P1-B）</b>：本批達成的是「tx 內零<b>身分反解</b> gRPC」，
 * <b>不是</b>「tx 內零 gRPC」。ATM 路徑的
 * {@code createRefundTaskAndAdvanceStatus → paymentGrpcClient.createRefund} 仍在 tx 內，
 * 剩餘邊界寫在該方法的 javadoc。
 *
 * <p>本 service 的四個進入點<b>全部</b>是 {@code @Transactional}，而它們發的事件（case transition／
 * awaiting_buyer_refund_page）payload 需要 {@code store_id → seller account_id}、
 * {@code buyer_id → notify account_id} 兩次反解。那兩次 gRPC 由共用 {@code GrpcStubFactory} 產生、
 * <b>沒有 deadline</b>，留在 tx 內就是「下游掛住 ⇒ 無限期持有 {@code aftersale_cases} 的 row lock
 * ＋ 一條 Hikari 連線」（《多副本排程搶佔規範》§四「出路 4 的硬性邊界」）。
 *
 * <p>⇒ 四個進入點一律改成<b>由呼叫端在 tx 之外解好 {@link EventPrincipals} 再當參數傳入</b>，
 * service 內只呼叫零 gRPC 的 publish 多載。兩個 helper 就是給呼叫端用的，
 * <b>它們刻意不是 {@code @Transactional}</b>：
 * <ul>
 *   <li>{@link #resolvePrincipalsForCaseStrict(String)} — 排程用。反解失敗即拋
 *       {@link PrincipalResolutionFailedException} ⇒ 該筆本輪不做；此時尚未進 tx、沒有 CAS，
 *       候選述詞未變 ⇒ 下一 tick 必然重撈。</li>
 *   <li>{@link #resolvePrincipalsForCaseBestEffort(String)} — API 用。反解失敗降級
 *       （seller→null／buyer→0L）永不拋 ⇒ 賣家／Admin 的一次操作不會因為 buyer-service
 *       抖一下就整筆失敗（API 沒有下一個 tick）。</li>
 * </ul>
 */
@Service
@RequiredArgsConstructor
public class ReturnReceiptService {

    private static final ObsLogger log = ObsLogger.forClass(ReturnReceiptService.class);

    private final AfterSaleCaseMapper caseMapper;
    private final AfterSaleResolutionMapper resolutionMapper;
    private final RefundTaskMapper refundTaskMapper;
    private final AfterSaleAuditService auditService;
    private final AfterSalesProperties properties;
    private final AfterSalesEventPublisher afterSalesEventPublisher;
    private final EvidenceService evidenceService;
    private final AfterSaleCaseService caseService;
    private final io.github.will413028.hawkr.aftersales.grpc.client.PaymentGrpcClient paymentGrpcClient;

    /** ATM 付款方式（Order DB 存 value） */
    private static final String ATM_PAYMENT_METHOD = PaymentMethod.ATM.value();

    /** §5.3.12 Admin 強制確認收貨允許的前置 status（2026-05-22 擴增 RETURN_AT_STORE） */
    private static final Set<String> ADMIN_FORCE_ALLOWED_STATUSES = Set.of(
            AfterSaleCaseStatus.RETURN_PENDING.value(),
            AfterSaleCaseStatus.RETURN_SHIPPING.value(),
            AfterSaleCaseStatus.RETURN_AT_STORE.value(),
            AfterSaleCaseStatus.RETURN_DELIVERED.value(),
            AfterSaleCaseStatus.RETURN_FAILED.value()
    );

    // ==================== tx 外的身分反解（issue #2294 B5/B6/B7） ====================

    /**
     * 排程路徑用：在<b>進 tx 之前</b>解析事件身分欄位，strict 語意。
     *
     * <p><b>刻意沒有 {@code @Transactional}</b>：整個重點就是這兩次無 deadline 的 gRPC
     * 不能發生在持有 row lock 的交易裡。多一次 {@code SELECT} 載入 case 是這個做法的成本，
     * 換來的是「tx 內零<b>身分反解</b>呼叫」（非 ATM 路徑即等於零網路呼叫；ATM 路徑仍有一次
     * 有 deadline 封頂的 {@code createRefund}，見 {@link #createRefundTaskAndAdvanceStatus}）
     * ——case 的 {@code store_id}／{@code buyer_id} 在
     * 這一刻與 tx 內讀到的必然相同（兩者都是不可變欄位），所以先讀不影響正確性；
     * 狀態競態仍由 tx 內的 CAS 兜住。
     *
     * @throws PrincipalResolutionFailedException 任一側反解失敗 ⇒ 本筆本輪不做、下一 tick 重試
     */
    public EventPrincipals resolvePrincipalsForCaseStrict(String caseNo) {
        AfterSaleCaseEntity caseEntity = loadCase(caseNo);
        return afterSalesEventPublisher.resolveEventPrincipals(
                caseEntity.getStoreId(), caseEntity.getBuyerId());
    }

    /**
     * API 路徑（賣家 confirm／Admin force）用：在<b>進 tx 之前</b>解析事件身分欄位，best-effort 語意。
     *
     * <p>與 {@link #resolvePrincipalsForCaseStrict} 的唯一差別是失敗處置：這裡降級不拋。
     * API 沒有「下一個 tick」，把身分反解的抖動升級成使用者可見的失敗，是拿一個本來只影響
     * 通知路由的問題去換掉整筆業務操作。
     */
    public EventPrincipals resolvePrincipalsForCaseBestEffort(String caseNo) {
        AfterSaleCaseEntity caseEntity = loadCase(caseNo);
        return afterSalesEventPublisher.resolveEventPrincipalsBestEffort(
                caseEntity.getStoreId(), caseEntity.getBuyerId());
    }

    /**
     * 賣家確認收到退貨（含選填驗貨扣款）— SELLER 路徑。
     *
     * <p>前置：status=RETURN_DELIVERED(27)，意即 PayUni 已通知物流送達。20/25/28 一律拒絕（拋 9004）。
     *
     * @param principals 呼叫端<b>在 tx 之外</b>以 {@link #resolvePrincipalsForCaseBestEffort} 解好的身分欄位
     */
    @Transactional
    public void confirmReceipt(Long storeId, String caseNo,
                                Long actualRefundAmount, String deductionReason,
                                List<String> inspectionPhotoTokens, String inspectionNote,
                                EventPrincipals principals) {
        AfterSaleCaseEntity caseEntity = loadCase(caseNo);
        if (!caseEntity.getStoreId().equals(storeId)) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_NOT_OWNER);
        }

        // §10 凍結檢查：申訴中禁止賣家確認收貨
        caseService.assertNotUnderAppeal(caseEntity);

        // §5.3.7 前置：必須 PayUni 通知送達後才能 confirm（status=27）
        if (!AfterSaleCaseStatus.RETURN_DELIVERED.value().equals(caseEntity.getStatus())) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_INVALID_STATUS);
        }

        Long originalRefundAmount = loadOriginalRefundAmount(caseNo);

        Long finalRefundAmount = originalRefundAmount;
        boolean hasDeduction = false;
        if (actualRefundAmount != null) {
            validateActualRefundAmount(actualRefundAmount, originalRefundAmount, deductionReason);
            finalRefundAmount = actualRefundAmount;
            hasDeduction = actualRefundAmount < originalRefundAmount;
        }

        // Media finalize（TX 外 side-effect，失敗則整個請求失敗）
        evidenceService.finalizeAndInsert(caseNo, storeId, inspectionPhotoTokens,
                AfterSaleEvidenceType.INSPECTION_PHOTO.value());

        // §2.1 INV-3 樂觀鎖：WHERE status='27'
        if (caseMapper.markReturnReceived(caseNo) == 0) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_INVALID_STATUS);
        }

        if (hasDeduction) {
            insertInspectionDeduction(caseNo, finalRefundAmount, deductionReason, inspectionNote, storeId);
        }

        // outcome 由 caller scope 已決定：扣款 → PARTIAL_REFUND；無扣款 → FULL_REFUND
        AfterSalePlatformDecisionOutcome outcome = hasDeduction
                ? AfterSalePlatformDecisionOutcome.PARTIAL_REFUND
                : AfterSalePlatformDecisionOutcome.FULL_REFUND;
        String refundTaskId = createRefundTaskAndAdvanceStatus(caseEntity, finalRefundAmount, outcome,
                principals);

        // §5.3.7 audit + event（SELLER 路徑）
        Map<String, Object> auditExtra = baseAuditExtra(originalRefundAmount, finalRefundAmount, hasDeduction,
                deductionReason, inspectionPhotoTokens, caseEntity);
        auditService.log(caseNo, null, storeId, AfterSaleAuditActor.SELLER.value(),
                AfterSaleAuditAction.CONFIRM_RETURN_RECEIPT.value(), auditExtra);

        log.info("ReturnReceiptService.confirmReceipt.success",
                "賣家確認收貨完成, finalRefundAmount=" + finalRefundAmount
                        + ", hasDeduction=" + hasDeduction, "caseNo", caseNo);

        // issue #2294 B5/B6/B7：改用帶 principals 的零 gRPC 多載（反解已在 tx 外做完）。
        // 回傳值<b>刻意忽略</b>＝維持既有 best-effort 語意：CASE_RETURN_RECEIVED 遺失只是少一則通知，
        // 不值得讓賣家的確認收貨整筆失敗（與同 tx 內 ATM 退款頁事件的 fail-closed 裁決不同，
        // 那一個遺失是「錢退不出去且沒人知道」，理由見 createRefundTaskAndAdvanceStatus）。
        afterSalesEventPublisher.tryPublishCaseTransitionWithContext(
                AfterSalesEventType.CASE_RETURN_RECEIVED.value(),
                MqConstants.EVENT_TYPE_AFTERSALES_CASE_RETURN_RECEIVED,
                caseNo, caseEntity.getOrderNo(),
                caseEntity.getBuyerId(), caseEntity.getStoreId(),
                "seller", null, principals);
    }

    /**
     * §5.3.10.1 系統自動完成（27 → AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT 32）— SYSTEM 路徑。
     *
     * <p>由 {@code AutoConfirmReceiptScheduler} 在 {@code auto_confirm_at} 到期時呼叫。
     * 固定全額退款不扣款。
     * <p>2026-05-22 修訂：使用 {@code markAutoCompletedFromDelivered}，直接寫 status=32（不再依賴
     * metadata 欄位 `auto_complete_reason`），同事務 stamp {@code auto_complete_at_stamp}，並 publish
     * 細粒度事件 {@code aftersales.case.auto_completed_confirm_receipt_timeout}（含
     * shipping_method / deadline_at 上下文供 FCM 渲染（snake_case，對齊 CaseTransitionPayload 契約）；原因由 event type 區分）。
     *
     * @param principals scheduler <b>在 tx 之外</b>以 {@link #resolvePrincipalsForCaseStrict} 解好的身分欄位
     * @return true = 已自動確認；false = 賣家已搶先手動 confirm 或 case 狀態改變，scheduler 應 skip
     */
    @Transactional
    public boolean autoConfirmReceipt(String caseNo, EventPrincipals principals) {
        AfterSaleCaseEntity caseEntity = loadCase(caseNo);

        if (!AfterSaleCaseStatus.RETURN_DELIVERED.value().equals(caseEntity.getStatus())) {
            log.info("ReturnReceiptService.autoConfirmReceipt.skipped.notDelivered",
                    "case 已不在 RETURN_DELIVERED(27)，skip; status=" + caseEntity.getStatus(),
                    "caseNo", caseNo);
            return false;
        }

        Long originalRefundAmount = loadOriginalRefundAmount(caseNo);
        java.time.LocalDateTime now = io.github.will413028.hawkr.common.utils.DateTimeUtils.nowUtc();

        if (caseMapper.markAutoCompletedFromDelivered(caseNo, now) == 0) {
            log.info("ReturnReceiptService.autoConfirmReceipt.skipped.raceLost",
                    "賣家搶先手動 confirm 或 Admin 強制完成，scheduler skip", "caseNo", caseNo);
            return false;
        }

        // 系統自動確認固定全額退款（§5.3.10.1）
        String refundTaskId = createRefundTaskAndAdvanceStatus(caseEntity, originalRefundAmount,
                AfterSalePlatformDecisionOutcome.FULL_REFUND, principals);

        Map<String, Object> auditExtra = new HashMap<>();
        auditExtra.put("finalRefundAmount", originalRefundAmount);
        auditExtra.put("autoConfirmAt", caseEntity.getAutoConfirmAt() != null ? caseEntity.getAutoConfirmAt().toString() : null);
        auditExtra.put("returnDeliveredAt", caseEntity.getReturnDeliveredAt() != null ? caseEntity.getReturnDeliveredAt().toString() : null);
        auditExtra.put("refundTaskId", refundTaskId);
        auditService.log(caseNo, null, null, AfterSaleAuditActor.SYSTEM.value(),
                AfterSaleAuditAction.AUTO_CONFIRM_RECEIPT.value(), auditExtra);

        log.info("ReturnReceiptService.autoConfirmReceipt.success",
                "scheduler 自動確認收貨 (status=32, CONFIRM_RECEIPT_TIMEOUT), refundAmount=" + originalRefundAmount, "caseNo", caseNo);

        Map<String, Object> extra = new HashMap<>();
        extra.put("shipping_method", caseEntity.getReturnShippingMethod());
        extra.put("deadline_at", caseEntity.getAutoConfirmAt() != null ? caseEntity.getAutoConfirmAt().toString() : null);

        // issue #2294 B5：status='32' 是永久轉換，publish 失敗即事件永久遺失
        // ⇒ 改用會傳遞失敗的變體，失敗即拋 → 本方法的 @Transactional rollback → 下一 tick 重來。
        // 帶 principals 的多載＝tx 內零 gRPC（反解由 scheduler 在 tx 外先做）。
        boolean published = afterSalesEventPublisher.tryPublishCaseTransitionWithContext(
                AfterSalesEventType.CASE_AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT.value(),
                MqConstants.EVENT_TYPE_AFTERSALES_CASE_AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT,
                caseNo, caseEntity.getOrderNo(),
                caseEntity.getBuyerId(), caseEntity.getStoreId(),
                "system", extra, principals);
        if (!published) {
            throw new EventPublishFailedException(
                    AfterSalesEventType.CASE_AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT.value(), caseNo);
        }
        return true;
    }

    /**
     * §5.3.10.2 系統自動完成（26→30）— SYSTEM 路徑（STORE_PICKUP_TIMEOUT，**2026-05-22 新增**）。
     *
     * <p>由 {@code StorePickupTimeoutScheduler} 在 {@code store_pickup_deadline_at} 到期時呼叫。
     * 超商賣家逾 72 小時未從門市取貨，系統視為自動完成、全額退款。
     * <p>競態：賣家剛從超商取貨觸發 26→27（PayUni 碼 11）→ 本 mapper affected=0 → skip。
     *
     * @param principals scheduler <b>在 tx 之外</b>以 {@link #resolvePrincipalsForCaseStrict} 解好的身分欄位
     * @return true = 已自動完成；false = case 狀態改變，scheduler 應 skip
     */
    @Transactional
    public boolean autoCompleteFromAtStore(String caseNo, EventPrincipals principals) {
        AfterSaleCaseEntity caseEntity = loadCase(caseNo);

        if (!AfterSaleCaseStatus.RETURN_AT_STORE.value().equals(caseEntity.getStatus())) {
            log.info("ReturnReceiptService.autoCompleteFromAtStore.skipped.notAtStore",
                    "case 已不在 RETURN_AT_STORE(26)，skip; status=" + caseEntity.getStatus(),
                    "caseNo", caseNo);
            return false;
        }

        Long originalRefundAmount = loadOriginalRefundAmount(caseNo);
        java.time.LocalDateTime now = io.github.will413028.hawkr.common.utils.DateTimeUtils.nowUtc();

        if (caseMapper.markAutoCompletedFromAtStore(caseNo, now) == 0) {
            log.info("ReturnReceiptService.autoCompleteFromAtStore.skipped.raceLost",
                    "賣家剛從超商取貨或 Admin 強制完成，scheduler skip", "caseNo", caseNo);
            return false;
        }

        // 超商門市逾時自動完成固定全額退款（§5.3.10.2）
        String refundTaskId = createRefundTaskAndAdvanceStatus(caseEntity, originalRefundAmount,
                AfterSalePlatformDecisionOutcome.FULL_REFUND, principals);

        Map<String, Object> auditExtra = new HashMap<>();
        auditExtra.put("finalRefundAmount", originalRefundAmount);
        auditExtra.put("storePickupDeadlineAt",
                caseEntity.getStorePickupDeadlineAt() != null ? caseEntity.getStorePickupDeadlineAt().toString() : null);
        auditExtra.put("storeArrivedAt",
                caseEntity.getStoreArrivedAt() != null ? caseEntity.getStoreArrivedAt().toString() : null);
        auditExtra.put("refundTaskId", refundTaskId);
        auditService.log(caseNo, null, null, AfterSaleAuditActor.SYSTEM.value(),
                AfterSaleAuditAction.AUTO_CONFIRM_RECEIPT.value(), auditExtra);

        log.info("ReturnReceiptService.autoCompleteFromAtStore.success",
                "scheduler 自動完成 (status=31, STORE_PICKUP_TIMEOUT), refundAmount=" + originalRefundAmount, "caseNo", caseNo);

        Map<String, Object> extra = new HashMap<>();
        extra.put("shipping_method", caseEntity.getReturnShippingMethod());
        extra.put("deadline_at",
                caseEntity.getStorePickupDeadlineAt() != null ? caseEntity.getStorePickupDeadlineAt().toString() : null);

        // issue #2294 B6：與 B5 逐字同形（status='31' 永久轉換），兩處必須一起改，
        // 否則同一支 service 會出現兩種保證等級
        boolean published = afterSalesEventPublisher.tryPublishCaseTransitionWithContext(
                AfterSalesEventType.CASE_AUTO_COMPLETED_STORE_PICKUP_TIMEOUT.value(),
                MqConstants.EVENT_TYPE_AFTERSALES_CASE_AUTO_COMPLETED_STORE_PICKUP_TIMEOUT,
                caseNo, caseEntity.getOrderNo(),
                caseEntity.getBuyerId(), caseEntity.getStoreId(),
                "system", extra, principals);
        if (!published) {
            throw new EventPublishFailedException(
                    AfterSalesEventType.CASE_AUTO_COMPLETED_STORE_PICKUP_TIMEOUT.value(), caseNo);
        }
        return true;
    }

    /**
     * §5.3.12 Admin 強制確認收貨 — ADMIN 路徑。
     *
     * <p>用於 PayUni 漏發送達通知 / 物流失敗 / 賣家失聯等情境。允許 status ∈ 20/25/27/28；
     * 推進至 RETURN_RECEIVED(30) → REFUND_PROCESSING(40) 並啟動退款。
     *
     * @param principals 呼叫端<b>在 tx 之外</b>以 {@link #resolvePrincipalsForCaseBestEffort} 解好的身分欄位
     * @return refundTaskId
     */
    @Transactional
    public ForceConfirmResult forceConfirmReceiptByAdmin(Long adminAccountId, String caseNo, String reason,
                                                          Long actualRefundAmount, String deductionReason,
                                                          EventPrincipals principals) {
        AfterSaleCaseEntity caseEntity = loadCase(caseNo);
        String priorStatus = caseEntity.getStatus();

        if (!ADMIN_FORCE_ALLOWED_STATUSES.contains(priorStatus)) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_INVALID_STATUS);
        }

        Long originalRefundAmount = loadOriginalRefundAmount(caseNo);

        Long finalRefundAmount = originalRefundAmount;
        boolean hasDeduction = false;
        if (actualRefundAmount != null) {
            validateActualRefundAmount(actualRefundAmount, originalRefundAmount, deductionReason);
            finalRefundAmount = actualRefundAmount;
            hasDeduction = actualRefundAmount < originalRefundAmount;
        }

        if (caseMapper.markReturnReceivedByAdmin(caseNo) == 0) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_INVALID_STATUS);
        }

        if (hasDeduction) {
            insertInspectionDeduction(caseNo, finalRefundAmount, deductionReason, null, adminAccountId);
        }

        // Admin 強制確認：扣款 → PARTIAL_REFUND；無扣款 → FULL_REFUND
        AfterSalePlatformDecisionOutcome outcome = hasDeduction
                ? AfterSalePlatformDecisionOutcome.PARTIAL_REFUND
                : AfterSalePlatformDecisionOutcome.FULL_REFUND;
        String refundTaskId = createRefundTaskAndAdvanceStatus(caseEntity, finalRefundAmount, outcome,
                principals);

        Map<String, Object> auditExtra = new HashMap<>();
        auditExtra.put("reason", reason);
        auditExtra.put("priorStatus", priorStatus);
        auditExtra.put("originalRefundAmount", originalRefundAmount);
        auditExtra.put("finalRefundAmount", finalRefundAmount);
        auditExtra.put("hasDeduction", hasDeduction);
        auditExtra.put("refundTaskId", refundTaskId);
        auditService.log(caseNo, null, adminAccountId, AfterSaleAuditActor.ADMIN.value(),
                AfterSaleAuditAction.FORCE_CONFIRM_RECEIPT.value(), auditExtra);

        log.info("ReturnReceiptService.forceConfirmReceiptByAdmin.success",
                "Admin 強制確認收貨, priorStatus=" + priorStatus
                        + ", finalRefundAmount=" + finalRefundAmount, "caseNo", caseNo);

        // issue #2294 B5/B6/B7：同 confirmReceipt——零 gRPC 多載 ＋ 回傳值刻意忽略（維持 best-effort）
        afterSalesEventPublisher.tryPublishCaseTransitionWithContext(
                AfterSalesEventType.CASE_RETURN_RECEIVED.value(),
                MqConstants.EVENT_TYPE_AFTERSALES_CASE_RETURN_RECEIVED,
                caseNo, caseEntity.getOrderNo(),
                caseEntity.getBuyerId(), caseEntity.getStoreId(),
                "admin", null, principals);

        return new ForceConfirmResult(caseNo, priorStatus, refundTaskId, finalRefundAmount);
    }

    /** Admin 強制確認回傳值，供 gRPC 組裝 response */
    public record ForceConfirmResult(String caseNo, String priorStatus,
                                      String refundTaskId, Long finalRefundAmount) {}

    // ==================== private helpers ====================

    private AfterSaleCaseEntity loadCase(String caseNo) {
        AfterSaleCaseEntity caseEntity = caseMapper.selectOne(
                new LambdaQueryWrapper<AfterSaleCaseEntity>()
                        .eq(AfterSaleCaseEntity::getCaseNo, caseNo)
        );
        if (caseEntity == null) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_NOT_FOUND);
        }
        return caseEntity;
    }

    private Long loadOriginalRefundAmount(String caseNo) {
        AfterSaleResolutionEntity resolution = resolutionMapper.selectOne(
                new LambdaQueryWrapper<AfterSaleResolutionEntity>()
                        .eq(AfterSaleResolutionEntity::getCaseNo, caseNo)
                        .orderByDesc(AfterSaleResolutionEntity::getCreatedAt)
                        .last("LIMIT 1")
        );
        if (resolution == null || resolution.getRefundAmount() == null) {
            log.error("ReturnReceiptService.loadOriginalRefundAmount.noResolution",
                    "找不到決議記錄", "caseNo", caseNo);
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_NOT_FOUND);
        }
        return resolution.getRefundAmount();
    }

    private void validateActualRefundAmount(Long actualRefundAmount, Long originalRefundAmount,
                                             String deductionReason) {
        if (actualRefundAmount <= 0) {
            throw new AfterSalesBusinessException(GrpcResponseCode.INVALID_PARAM);
        }
        if (actualRefundAmount > originalRefundAmount) {
            throw new AfterSalesBusinessException(GrpcResponseCode.REFUND_AMOUNT_EXCEEDED);
        }
        if (actualRefundAmount < originalRefundAmount) {
            if (deductionReason == null || deductionReason.isBlank()) {
                throw new AfterSalesBusinessException(GrpcResponseCode.INVALID_PARAM);
            }
        }
    }

    private void insertInspectionDeduction(String caseNo, Long finalRefundAmount,
                                            String deductionReason, String inspectionNote, Long resolvedBy) {
        AfterSaleResolutionEntity inspection = new AfterSaleResolutionEntity();
        inspection.setId(TsidUtil.generate());
        inspection.setCaseNo(caseNo);
        inspection.setResolutionType(AfterSaleResolutionType.INSPECTION_DEDUCTION.value());
        inspection.setRefundAmount(finalRefundAmount);
        inspection.setDeductionReason(deductionReason);
        inspection.setInspectionNote(inspectionNote);
        inspection.setResolvedBy(resolvedBy);
        resolutionMapper.insert(inspection);
    }

    /**
     * 建立退款任務 + 推進 status 30→40 + ATM publish。
     * 四條路徑共用（兩條排程 ＋ 兩條 API）：金額差異交由 caller 計算後傳入。
     *
     * <h3>⚠️ 已知邊界：ATM 分支的 {@code createRefund} 仍在 tx 內（issue #2294 P1-B，本批不治本）</h3>
     *
     * <p>本方法被四個 {@code @Transactional} 進入點共用，因此下面這一次 gRPC 是<b>在交易內</b>發生的：
     * {@code isAtm} 為真時的 {@code paymentGrpcClient.createRefund}（ATM 退款預建）。
     * 此時 {@code loadCase} 早已把連線綁定在本交易上，所以這次呼叫的耗時<b>就是</b>
     * 「{@code aftersale_cases} row lock ＋ 一條 Hikari 連線」被持有的時間。
     *
     * <p><b>現況是嚴格改善，不是回歸</b>：這條路徑在 issue #2294 之前<b>無界</b>
     * （{@code GrpcStubFactory} 全域零 deadline），P1-1 把它壓到
     * {@link io.github.will413028.hawkr.aftersales.grpc.client.PaymentGrpcClient#CREATE_REFUND_DEADLINE_SEC}
     * （100 秒）封頂。但 100 秒與《多副本排程搶佔規範》§四出路 4 想要的「短 tx」仍差一個數量級，
     * 所以它是<b>剩餘邊界</b>而非已解問題。
     *
     * <p><b>治本方向（本批刻意不做）</b>：把 ATM 退款預建移到 tx 之外——與 P1-2 對身分反解做的事同形
     * （呼叫端先做、結果當參數傳入）。不在本批做的理由：它會改變「預建失敗」的失敗語意與
     * §2.3.1 INV-5 的 refund_task 落地時機，屬於獨立的行為變更，不該搭載在本批的技術性修正上。
     *
     * <p>⚠️ 這條也是既有測試的<b>盲區</b>：{@code verifyNoInteractions} 只涵蓋 seller／buyer 兩個
     * client，量不到 payment。不要因為那些斷言是綠的就以為 tx 內已經沒有 gRPC。
     *
     * @param principals 呼叫端在 tx 之外解好的身分欄位——排程走 strict、API 走 best-effort，
     *                   差別在<b>解析失敗怎麼辦</b>，不在本方法內（本方法只是使用者）
     * @return refundTaskId（TSID 字串）
     */
    private String createRefundTaskAndAdvanceStatus(AfterSaleCaseEntity caseEntity, Long finalRefundAmount,
                                                     AfterSalePlatformDecisionOutcome outcome,
                                                     EventPrincipals principals) {
        String caseNo = caseEntity.getCaseNo();
        boolean isAtm = ATM_PAYMENT_METHOD.equals(caseEntity.getPaymentMethod());
        short initialAttempt = 1;
        String externalRefId = null;
        // §2.3.1 INV-5：ATM createRefund 預建失敗時不 throw，建終態 FAILED refund_task 持久化 PayUni 原話
        boolean refundPrebuildFailed = false;
        String payuniStatus = null;
        String payuniMessage = null;
        String lastError = null;
        if (isAtm) {
            String idempotencyKey = "CASE:" + caseNo + ":RECEIPT:" + initialAttempt;
            CreateRefundResponse resp = null;
            try {
                resp = paymentGrpcClient.createRefund(
                        caseEntity.getTransactionId(),
                        String.valueOf(finalRefundAmount),
                        "退貨確認退款",
                        idempotencyKey);
            } catch (io.grpc.StatusRuntimeException e) {
                refundPrebuildFailed = true;
                lastError = "createRefund StatusRuntimeException: " + e.getMessage();
                log.warn("ReturnReceiptService.createRefundTaskAndAdvanceStatus.createRefundUnavailable",
                        "ATM 預建退款 payment 不可用, caseNo=" + caseNo + ", " + e.getMessage(), "caseNo", caseNo);
            }
            if (!refundPrebuildFailed && resp.getCode() != 0) {
                refundPrebuildFailed = true;
                payuniStatus = resp.getData().getProviderStatus();
                payuniMessage = resp.getData().getProviderMessage();
                lastError = resp.getMessage();
                log.warn("ReturnReceiptService.createRefundTaskAndAdvanceStatus.createRefundFailed",
                        "ATM 預建退款失敗, caseNo=" + caseNo + ", code=" + resp.getCode()
                                + ", msg=" + resp.getMessage(), "caseNo", caseNo);
            } else if (!refundPrebuildFailed) {
                externalRefId = resp.getData().getRefundNo();
            }
        }
        RefundTaskEntity refundTask = new RefundTaskEntity();
        Long tsid = TsidUtil.generate();
        refundTask.setId(tsid);
        refundTask.setCaseNo(caseNo);
        refundTask.setOrderNo(caseEntity.getOrderNo());
        refundTask.setTransactionId(caseEntity.getTransactionId());
        refundTask.setRefundAmount(finalRefundAmount);
        refundTask.setPaymentMethod(caseEntity.getPaymentMethod());
        refundTask.setMerId(caseEntity.getMerId());
        refundTask.setRefundOutcome(outcome.value());
        refundTask.setExternalRefId(externalRefId);
        if (refundPrebuildFailed) {
            // 終態 FAILED：Worker 不再撈，持久化 PayUni 原話進 admin 異常清單
            refundTask.setStatus(WorkerTaskStatus.FAILED.value());
            refundTask.setNextRetryTime(null);
            refundTask.setPayuniStatus(payuniStatus);
            refundTask.setPayuniMessage(payuniMessage);
            refundTask.setLastError(lastError);
        } else {
            refundTask.setStatus(isAtm
                    ? WorkerTaskStatus.AWAITING_BUYER_REFUND_PAGE.value()
                    : WorkerTaskStatus.PENDING.value());
        }
        refundTask.setRetryCount(0);
        refundTask.setMaxRetryCount(properties.getRefund().getMaxRetryCount());
        refundTask.setCurrentAttempt(initialAttempt);
        refundTaskMapper.insert(refundTask);

        if (caseMapper.markRefundProcessingFromReceived(caseNo) == 0) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_INVALID_STATUS);
        }

        // §2.3.1 INV-5：createRefund 預建失敗時 task=FAILED，不發等待頁事件（退款進 admin 異常清單）
        if (isAtm && WorkerTaskStatus.AWAITING_BUYER_REFUND_PAGE.value().equals(refundTask.getStatus())) {
            // issue #2294 B7：status='40' 是永久轉換，事件遺失＝買家永遠拿不到退款頁連結、
            // 退款卡在 AWAITING_BUYER_REFUND_PAGE ⇒ 改用會傳遞失敗的變體，失敗即 rollback。
            // ⚠️ 本 helper 同時被兩條排程與兩條 API（gRPC confirmReceipt / admin force）共用，
            // 所以這是本 issue 唯一使用者可見的行為變更：MQ 故障時賣家按「確認收貨」會整筆失敗並看到錯誤。
            // 裁決（09 §5.3.7）：接受 fail-closed —— 少了這個事件是「錢退不出去且沒人知道」，
            // 代價高於「賣家在 MQ 故障期間看到一次錯誤並重試」；tx 邊界保證失敗時 DB 不留半套狀態。
            // issue #2294 B7：帶 principals 的多載＝tx 內零 gRPC（buyer_id 反解已在 tx 外做完）
            boolean published = afterSalesEventPublisher.tryPublishAwaitingBuyerRefundPage(
                    caseEntity.getOrderNo(), caseNo, tsid, caseEntity.getBuyerId(), principals);
            if (!published) {
                throw new EventPublishFailedException(
                        AfterSalesEventType.AWAITING_BUYER_PAGE.value(), caseNo);
            }
        }

        return String.valueOf(tsid);
    }

    private Map<String, Object> baseAuditExtra(Long originalRefundAmount, Long finalRefundAmount,
                                                boolean hasDeduction, String deductionReason,
                                                List<String> inspectionPhotoTokens,
                                                AfterSaleCaseEntity caseEntity) {
        Map<String, Object> extra = new HashMap<>();
        extra.put("originalRefundAmount", originalRefundAmount);
        extra.put("actualRefundAmount", finalRefundAmount);
        extra.put("hasDeduction", hasDeduction);
        extra.put("deductionReason", hasDeduction ? deductionReason : null);
        extra.put("inspectionPhotoCount",
                inspectionPhotoTokens == null ? 0 : inspectionPhotoTokens.size());
        extra.put("isAtm", ATM_PAYMENT_METHOD.equals(caseEntity.getPaymentMethod()));
        return extra;
    }
}
