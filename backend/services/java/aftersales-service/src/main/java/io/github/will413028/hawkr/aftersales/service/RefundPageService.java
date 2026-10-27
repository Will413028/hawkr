package io.github.will413028.hawkr.aftersales.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.aftersales.config.AfterSalesConstants;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditActor;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditAction;
import io.github.will413028.hawkr.common.utils.code.AfterSaleEvidenceType;
import io.github.will413028.hawkr.common.utils.code.AppType;
import io.github.will413028.hawkr.common.utils.code.ClientType;
import io.github.will413028.hawkr.common.utils.code.RefundStatus;
import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import io.github.will413028.hawkr.aftersales.entity.SellerCancelRequestEntity;
import io.github.will413028.hawkr.aftersales.exception.AfterSalesBusinessException;
import io.github.will413028.hawkr.aftersales.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.aftersales.mapper.SellerCancelRequestMapper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.payment.grpc.GetRefundPageUrlResponse;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

import java.util.Map;

/**
 * ATM 退款頁面服務。
 * 為 ATM 付款的退款任務提供 PayUni 退款頁面 URL。
 */
@Service
@RequiredArgsConstructor
public class RefundPageService {

    private static final ObsLogger log = ObsLogger.forClass(RefundPageService.class);

    private final RefundTaskMapper refundTaskMapper;
    private final AfterSaleCaseMapper caseMapper;
    private final SellerCancelRequestMapper cancelMapper;
    private final PaymentGrpcClient paymentGrpcClient;
    private final AfterSaleAuditService auditService;
    private final AfterSalesProperties properties;

    /**
     * 取得 ATM 退款頁面 URL。
     *
     * @param buyerId    買家 buyer_id（buyers PK surrogate，Gateway resolver 解析後傳入）
     * @param orderNo    訂單編號
     * @param clientType 請求端類型（空字串視為 MOBILE_APP，向下相容）
     * @param appType    App 身份（空字串視為 BUYER，向下相容）
     * @return PayUni 退款頁面 redirect URL
     */
    public String getRefundPageUrl(Long buyerId, String orderNo, String clientType, String appType) {
        // 空字串視為預設值（向下相容 §5.4.2）
        String resolvedClientType = (clientType == null || clientType.isBlank())
                ? ClientType.MOBILE_APP.value()
                : clientType;
        String resolvedAppType = (appType == null || appType.isBlank())
                ? AppType.BUYER.value()
                : appType;
        if (!ClientType.isValidValue(resolvedClientType) || !AppType.isValidValue(resolvedAppType)) {
            throw new AfterSalesBusinessException(GrpcResponseCode.INVALID_PARAM);
        }

        // 1. 查詢等待買家操作的退款任務
        RefundTaskEntity refundTask = refundTaskMapper.selectOne(
                new LambdaQueryWrapper<RefundTaskEntity>()
                        .eq(RefundTaskEntity::getOrderNo, orderNo)
                        .eq(RefundTaskEntity::getStatus, WorkerTaskStatus.AWAITING_BUYER_REFUND_PAGE.value())
                        .last("LIMIT 1")
        );
        if (refundTask == null) {
            throw new AfterSalesBusinessException(GrpcResponseCode.REFUND_PAGE_NOT_AVAILABLE);
        }

        // 2. 驗證買家身分
        verifyBuyerOwnership(buyerId, refundTask);

        // 3. 覆寫每次點擊的 client_type / app_type（買家可能切換 App / Web；最後一次決定 PayUni 回調出口）
        refundTaskMapper.markRefundPageClient(refundTask.getId(), resolvedClientType, resolvedAppType);

        // 4. 組裝 returnUrl（完整路徑 + query params）
        String atmReturnUrl = properties.getAtmRefund().getReturnUrl();
        String returnUrl = atmReturnUrl + "?order_no=" + orderNo + "&app=buyer";

        // 5. 呼叫 Payment Service 取得退款頁面 URL
        GetRefundPageUrlResponse resp = paymentGrpcClient.getRefundPageUrl(
                refundTask.getTransactionId(),
                ProtoConversionUtils.formatLong(refundTask.getRefundAmount()),
                returnUrl
        );
        // payment-service 失敗時會拋 StatusRuntimeException，由上層 RPC catch-all 處理
        String redirectUrl = resp.getData().getRedirectUrl();

        // 6. 稽核日誌
        String caseNo = refundTask.getCaseNo();
        Long cancelId = refundTask.getCancelId();
        auditService.log(caseNo, cancelId, buyerId, AfterSaleAuditActor.BUYER.value(), AfterSaleAuditAction.REFUND_PAGE_REQUESTED.value(),
                Map.of("orderNo", orderNo, "refundTaskId", refundTask.getId(),
                        "clientType", resolvedClientType, "appType", resolvedAppType));

        log.info("RefundPageService.getRefundPageUrl.success",
                "取得退款頁面 URL 成功, buyerId=" + buyerId, "orderNo", orderNo);

        return redirectUrl;
    }

    /**
     * 驗證退款任務的買家身分。
     * 依來源（售後案件 / 賣家取消）查找對應的買家帳號。
     */
    private void verifyBuyerOwnership(Long buyerId, RefundTaskEntity refundTask) {
        if (refundTask.getCaseNo() != null) {
            // 來自售後案件
            AfterSaleCaseEntity caseEntity = caseMapper.selectOne(
                    new LambdaQueryWrapper<AfterSaleCaseEntity>()
                            .eq(AfterSaleCaseEntity::getCaseNo, refundTask.getCaseNo())
            );
            if (caseEntity == null || !caseEntity.getBuyerId().equals(buyerId)) {
                throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_NOT_OWNER);
            }
        } else if (refundTask.getCancelId() != null) {
            // 來自賣家取消出貨
            SellerCancelRequestEntity cancelEntity = cancelMapper.selectById(refundTask.getCancelId());
            if (cancelEntity == null || !cancelEntity.getBuyerId().equals(buyerId)) {
                throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_NOT_OWNER);
            }
        } else {
            log.error("RefundPageService.verifyBuyerOwnership.noSource",
                    "退款任務無來源, refundTaskId=" + refundTask.getId());
            throw new AfterSalesBusinessException(GrpcResponseCode.REFUND_PAGE_NOT_AVAILABLE);
        }
    }
}
