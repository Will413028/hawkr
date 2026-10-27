package io.github.will413028.hawkr.aftersales.grpc;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import io.github.will413028.hawkr.aftersales.config.AfterSalesConstants;
import io.github.will413028.hawkr.aftersales.entity.*;
import io.github.will413028.hawkr.aftersales.entity.projection.SellerCaseRef;
import io.github.will413028.hawkr.aftersales.mapper.*;
import io.github.will413028.hawkr.aftersales.service.*;
import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.obs.grpc.GrpcErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.*;
import io.github.will413028.hawkr.common.utils.code.helper.AfterSaleCaseStatusRules;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.grpc.stub.StreamObserver;
import net.devh.boot.grpc.server.service.GrpcService;
import org.slf4j.MDC;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.util.StringUtils;

import java.util.Arrays;
import java.util.HashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * 售後服務 gRPC 實作。
 * 提供買家端、賣家端、內部 RPC 共 13 個介面。
 */
@GrpcService
public class AfterSalesGrpcService extends AfterSalesServiceGrpc.AfterSalesServiceImplBase {

    private static final ObsLogger log = ObsLogger.forClass(AfterSalesGrpcService.class);
    private static final String CLASS_NAME = "AfterSalesGrpcService";

    private final AfterSaleCaseService caseService;
    private final AfterSaleResolutionService resolutionService;
    private final RefundPageService refundPageService;
    private final ReturnReceiptService returnReceiptService;
    private final SellerCancelService sellerCancelService;
    private final AfterSaleLabelRetryService labelRetryService;
    private final AfterSaleAppealService appealService;
    private final PlatformDecisionService platformDecisionService;
    private final io.github.will413028.hawkr.aftersales.service.AfterSalePayoutHoldService payoutHoldService;
    private final AfterSaleCaseMapper caseMapper;
    private final AfterSaleCaseItemMapper caseItemMapper;
    private final AfterSaleEvidenceMapper evidenceMapper;
    private final AfterSaleResolutionMapper resolutionMapper;
    private final RefundTaskMapper refundTaskMapper;
    private final SellerCancelRequestMapper cancelMapper;
    private final io.github.will413028.hawkr.aftersales.config.AfterSalesProperties afterSalesProperties;
    private final io.github.will413028.hawkr.aftersales.grpc.client.PaymentGrpcClient paymentGrpcClient;
    private final io.github.will413028.hawkr.aftersales.grpc.client.BuyerGrpcClient buyerGrpcClient;
    private final io.github.will413028.hawkr.aftersales.grpc.client.SellerGrpcClient sellerGrpcClient;
    private final io.github.will413028.hawkr.aftersales.grpc.client.ShippingGrpcClient shippingGrpcClient;
    private final io.github.will413028.hawkr.aftersales.grpc.client.OrderGrpcClient orderGrpcClient;
    private final io.github.will413028.hawkr.aftersales.service.AtmRefundReconciler atmRefundReconciler;
    // handleRefundPageReturn 的 REF02037 / 缺陷 B 對帳分支含多筆寫入（task transition + close case
    // + audit + publish），須以 TransactionTemplate 程式式包覆確保原子（gRPC handler 無 @Transactional）；
    // 且 payment 端狀態查詢的同步 gRPC 必須在 tx 外先做（P1-1 / P1-2，issue #1288）。
    private final org.springframework.transaction.support.TransactionTemplate refundReconcileTxTemplate;

    @Value("${hawkr.web.frontend-url}")
    private String webFrontendUrl;

    public AfterSalesGrpcService(AfterSaleCaseService caseService,
                                  AfterSaleResolutionService resolutionService,
                                  RefundPageService refundPageService,
                                  ReturnReceiptService returnReceiptService,
                                  SellerCancelService sellerCancelService,
                                  AfterSaleLabelRetryService labelRetryService,
                                  AfterSaleAppealService appealService,
                                  PlatformDecisionService platformDecisionService,
                                  io.github.will413028.hawkr.aftersales.service.AfterSalePayoutHoldService payoutHoldService,
                                  AfterSaleCaseMapper caseMapper,
                                  AfterSaleCaseItemMapper caseItemMapper,
                                  AfterSaleEvidenceMapper evidenceMapper,
                                  AfterSaleResolutionMapper resolutionMapper,
                                  RefundTaskMapper refundTaskMapper,
                                  SellerCancelRequestMapper cancelMapper,
                                  io.github.will413028.hawkr.aftersales.config.AfterSalesProperties afterSalesProperties,
                                  io.github.will413028.hawkr.aftersales.grpc.client.PaymentGrpcClient paymentGrpcClient,
                                  io.github.will413028.hawkr.aftersales.grpc.client.BuyerGrpcClient buyerGrpcClient,
                                  io.github.will413028.hawkr.aftersales.grpc.client.SellerGrpcClient sellerGrpcClient,
                                  io.github.will413028.hawkr.aftersales.grpc.client.ShippingGrpcClient shippingGrpcClient,
                                  io.github.will413028.hawkr.aftersales.grpc.client.OrderGrpcClient orderGrpcClient,
                                  io.github.will413028.hawkr.aftersales.service.AtmRefundReconciler atmRefundReconciler,
                                  org.springframework.transaction.PlatformTransactionManager txManager) {
        this.caseService = caseService;
        this.resolutionService = resolutionService;
        this.refundPageService = refundPageService;
        this.returnReceiptService = returnReceiptService;
        this.sellerCancelService = sellerCancelService;
        this.labelRetryService = labelRetryService;
        this.appealService = appealService;
        this.platformDecisionService = platformDecisionService;
        this.payoutHoldService = payoutHoldService;
        this.caseMapper = caseMapper;
        this.caseItemMapper = caseItemMapper;
        this.evidenceMapper = evidenceMapper;
        this.resolutionMapper = resolutionMapper;
        this.refundTaskMapper = refundTaskMapper;
        this.cancelMapper = cancelMapper;
        this.afterSalesProperties = afterSalesProperties;
        this.paymentGrpcClient = paymentGrpcClient;
        this.buyerGrpcClient = buyerGrpcClient;
        this.sellerGrpcClient = sellerGrpcClient;
        this.shippingGrpcClient = shippingGrpcClient;
        this.orderGrpcClient = orderGrpcClient;
        this.atmRefundReconciler = atmRefundReconciler;
        this.refundReconcileTxTemplate =
                new org.springframework.transaction.support.TransactionTemplate(txManager);
    }

    // ==================== 買家端 ====================

    @Override
    public void createAfterSaleCase(CreateAfterSaleCaseRequest request,
                                     StreamObserver<CreateAfterSaleCaseResponse> observer) {
        // #1871 OBS-09：order_no 進 MDC，讓這個方法內所有 log 行都能被 Loki structured
        // metadata 查到（見 k8s/monitoring/promtail-daemonset.yaml）。
        MDC.put("order_no", request.getOrderNo());
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = request.getBuyerId();

            AfterSaleCaseEntity caseEntity = caseService.createCase(
                    buyerId,
                    request.getOrderNo(),
                    request.getReason(),
                    request.getDescription(),
                    request.getPhotoTokensList().isEmpty() ? null : request.getPhotoTokensList()
            );

            var summary = AfterSaleCaseSummary.newBuilder()
                    .setCaseNo(caseEntity.getCaseNo())
                    .setOrderNo(caseEntity.getOrderNo())
                    .setStatus(caseEntity.getStatus())
                    .setReason(caseEntity.getReason())
                    .setRefundableAmount(ProtoConversionUtils.formatLong(caseEntity.getRefundableAmount()))
                    .setCreatedAt(ProtoConversionUtils.formatIso8601(caseEntity.getCreatedAt()))
                    .setNeedRefundPage(false)
                    .build();

            observer.onNext(CreateAfterSaleCaseResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(summary)
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer, CreateAfterSaleCaseResponse.newBuilder(),
                    e, CLASS_NAME, "createAfterSaleCase");
        } finally {
            MDC.remove("order_no");
        }
    }

    @Override
    public void getMyAfterSaleCases(GetMyAfterSaleCasesRequest request,
                                     StreamObserver<GetMyAfterSaleCasesResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = request.getBuyerId();
            int pageNum = ProtoConversionUtils.parseIntOrDefault(request.getPage(), 1);
            int pageSize = ProtoConversionUtils.parseIntOrDefault(request.getPageSize(), 20);
            String statusFilter = request.getStatus();

            List<String> statusList = StringUtils.hasText(statusFilter)
                    ? Arrays.asList(statusFilter.split(","))
                    : List.of();
            Page<AfterSaleCaseEntity> page = new Page<>(pageNum, pageSize);
            var wrapper = new LambdaQueryWrapper<AfterSaleCaseEntity>()
                    .eq(AfterSaleCaseEntity::getBuyerId, buyerId)
                    .in(!statusList.isEmpty(), AfterSaleCaseEntity::getStatus, statusList)
                    .orderByDesc(AfterSaleCaseEntity::getCreatedAt);
            var result = caseMapper.selectPage(page, wrapper);

            var dataBuilder = AfterSaleCaseListData.newBuilder()
                    .setTotal(String.valueOf(result.getTotal()))
                    .setPage(String.valueOf(result.getCurrent()))
                    .setPageSize(String.valueOf(result.getSize()));

            // 收斂規範 §6.3：批次解析 provider 狀態（REFUND_PROCESSING 讀本地 / RETURN_SHIPPING 呼 Shipping batch）
            Map<String, ProviderStatusPair> providerMap =
                    batchResolveCaseProviderStatus(result.getRecords());
            for (AfterSaleCaseEntity entity : result.getRecords()) {
                ProviderStatusPair pair = providerMap.getOrDefault(entity.getCaseNo(), ProviderStatusPair.EMPTY);
                dataBuilder.addRecords(toSummary(entity, "", pair));
            }

            observer.onNext(GetMyAfterSaleCasesResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(dataBuilder.build())
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer, GetMyAfterSaleCasesResponse.newBuilder(),
                    e, CLASS_NAME, "getMyAfterSaleCases");
        }
    }

    @Override
    public void getAfterSaleCaseDetail(GetAfterSaleCaseDetailRequest request,
                                        StreamObserver<GetAfterSaleCaseDetailResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = request.getBuyerId();
            String caseNo = request.getCaseNo();

            AfterSaleCaseEntity caseEntity = caseService.getCaseForBuyer(buyerId, caseNo);

            observer.onNext(GetAfterSaleCaseDetailResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(buildDetailView(caseEntity))
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer, GetAfterSaleCaseDetailResponse.newBuilder(),
                    e, CLASS_NAME, "getAfterSaleCaseDetail");
        }
    }

    @Override
    public void supplementEvidence(SupplementEvidenceRequest request,
                                    StreamObserver<SupplementEvidenceResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = request.getBuyerId();

            caseService.supplementEvidence(
                    buyerId,
                    request.getCaseNo(),
                    request.getPhotoTokensList().isEmpty() ? null : request.getPhotoTokensList(),
                    request.getAdditionalDescription()
            );

            observer.onNext(SupplementEvidenceResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer, SupplementEvidenceResponse.newBuilder(),
                    e, CLASS_NAME, "supplementEvidence");
        }
    }

    @Override
    public void cancelAfterSaleCase(CancelAfterSaleCaseRequest request,
                                     StreamObserver<CancelAfterSaleCaseResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = request.getBuyerId();

            caseService.cancelCase(buyerId, request.getCaseNo());

            observer.onNext(CancelAfterSaleCaseResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer, CancelAfterSaleCaseResponse.newBuilder(),
                    e, CLASS_NAME, "cancelAfterSaleCase");
        }
    }

    @Override
    public void getRefundPageUrl(io.github.will413028.hawkr.aftersales.grpc.GetRefundPageUrlRequest request,
                                  StreamObserver<io.github.will413028.hawkr.aftersales.grpc.GetRefundPageUrlResponse> observer) {
        // #1871 OBS-09：order_no 進 MDC，讓這個方法內所有 log 行都能被 Loki structured
        // metadata 查到（見 k8s/monitoring/promtail-daemonset.yaml）。
        MDC.put("order_no", request.getOrderNo());
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = request.getBuyerId();

            String redirectUrl = refundPageService.getRefundPageUrl(
                    buyerId, request.getOrderNo(),
                    request.getClientType(), request.getAppType());

            observer.onNext(io.github.will413028.hawkr.aftersales.grpc.GetRefundPageUrlResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(RefundPageUrlData.newBuilder()
                            .setRedirectUrl(redirectUrl)
                            .build())
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    io.github.will413028.hawkr.aftersales.grpc.GetRefundPageUrlResponse.newBuilder(),
                    e, CLASS_NAME, "getRefundPageUrl");
        } finally {
            MDC.remove("order_no");
        }
    }

    // ==================== 賣家端 ====================

    @Override
    public void getSellerAfterSaleCases(GetSellerAfterSaleCasesRequest request,
                                         StreamObserver<GetSellerAfterSaleCasesResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER);
            long storeId = request.getStoreId();
            int pageNum = ProtoConversionUtils.parseIntOrDefault(request.getPage(), 1);
            int pageSize = ProtoConversionUtils.parseIntOrDefault(request.getPageSize(), 20);
            String statusFilter = request.getStatus();

            List<String> statusList = StringUtils.hasText(statusFilter)
                    ? Arrays.asList(statusFilter.split(","))
                    : List.of();
            Page<AfterSaleCaseEntity> page = new Page<>(pageNum, pageSize);
            var wrapper = new LambdaQueryWrapper<AfterSaleCaseEntity>()
                    .eq(AfterSaleCaseEntity::getStoreId, storeId)
                    .in(!statusList.isEmpty(), AfterSaleCaseEntity::getStatus, statusList)
                    .orderByDesc(AfterSaleCaseEntity::getCreatedAt);
            var result = caseMapper.selectPage(page, wrapper);

            var dataBuilder = AfterSaleCaseListData.newBuilder()
                    .setTotal(String.valueOf(result.getTotal()))
                    .setPage(String.valueOf(result.getCurrent()))
                    .setPageSize(String.valueOf(result.getSize()));

            // 批次取得買家頭像（不快照，列表時即時查詢，與 order-service 列表邏輯一致）
            List<Long> buyerIds = result.getRecords().stream()
                    .map(AfterSaleCaseEntity::getBuyerId)
                    .filter(java.util.Objects::nonNull)
                    .distinct()
                    .toList();
            java.util.Map<Long, io.github.will413028.hawkr.buyer.grpc.BuyerDisplay> buyerDisplayMap = buyerIds.isEmpty()
                    ? java.util.Collections.emptyMap()
                    : buyerGrpcClient.batchGetBuyerDisplayByBuyerIds(buyerIds);

            // 收斂規範 §6.3：批次解析 provider 狀態
            Map<String, ProviderStatusPair> providerMap =
                    batchResolveCaseProviderStatus(result.getRecords());
            for (AfterSaleCaseEntity entity : result.getRecords()) {
                io.github.will413028.hawkr.buyer.grpc.BuyerDisplay meta = buyerDisplayMap.get(entity.getBuyerId());
                String avatarKey = meta != null ? meta.getAvatarKey() : "";
                ProviderStatusPair pair = providerMap.getOrDefault(entity.getCaseNo(), ProviderStatusPair.EMPTY);
                dataBuilder.addRecords(toSummary(entity, avatarKey, pair));
            }

            observer.onNext(GetSellerAfterSaleCasesResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(dataBuilder.build())
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer, GetSellerAfterSaleCasesResponse.newBuilder(),
                    e, CLASS_NAME, "getSellerAfterSaleCases");
        }
    }

    @Override
    public void getSellerAfterSaleCaseDetail(GetSellerAfterSaleCaseDetailRequest request,
                                              StreamObserver<GetSellerAfterSaleCaseDetailResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER);
            long storeId = request.getStoreId();
            String caseNo = request.getCaseNo();

            AfterSaleCaseEntity caseEntity = caseService.getCaseForSeller(storeId, caseNo);

            observer.onNext(GetSellerAfterSaleCaseDetailResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(buildDetailView(caseEntity))
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer, GetSellerAfterSaleCaseDetailResponse.newBuilder(),
                    e, CLASS_NAME, "getSellerAfterSaleCaseDetail");
        }
    }

    @Override
    public void resolveAfterSaleCase(ResolveAfterSaleCaseRequest request,
                                      StreamObserver<ResolveAfterSaleCaseResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER);
            long storeId = request.getStoreId();

            String resolutionType = request.getResolutionType();

            Long refundAmount = ProtoConversionUtils.parseLongOrNull(request.getRefundAmount());
            Long couponAmount = ProtoConversionUtils.parseLongOrNull(request.getCouponAmount());
            Long couponMinSpend = ProtoConversionUtils.parseLongOrNull(request.getCouponMinSpend());

            resolutionService.resolve(
                    storeId,
                    request.getCaseNo(),
                    resolutionType,
                    refundAmount,
                    request.getReturnShippingMethod(),
                    request.getRejectReason(),
                    request.getRejectMessage(),
                    couponAmount,
                    couponMinSpend,
                    request.getCouponMessage()
            );

            // 重新查詢最新狀態
            AfterSaleCaseEntity updatedCase = caseService.getCase(request.getCaseNo());
            boolean needRefundPage = isNeedRefundPage(updatedCase);

            observer.onNext(ResolveAfterSaleCaseResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(ResolveResultData.newBuilder()
                            .setCaseNo(updatedCase.getCaseNo())
                            .setStatus(updatedCase.getStatus())
                            .setReturnShipmentNo(nullToEmpty(updatedCase.getReturnShipmentNo()))
                            .setNeedRefundPage(needRefundPage)
                            .build())
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer, ResolveAfterSaleCaseResponse.newBuilder(),
                    e, CLASS_NAME, "resolveAfterSaleCase");
        }
    }

    @Override
    public void confirmReturnReceipt(ConfirmReturnReceiptRequest request,
                                      StreamObserver<ConfirmReturnReceiptResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER);
            long storeId = request.getStoreId();

            // 解析 optional actualRefundAmount（空字串 = 未傳 = 全額退款）
            Long actualRefundAmount = null;
            String actualRaw = request.getActualRefundAmount();
            if (actualRaw != null && !actualRaw.isBlank()) {
                actualRefundAmount = ProtoConversionUtils.parseLongRequired(actualRaw, "actualRefundAmount");
            }

            // issue #2294 B7：事件身分反解（無 deadline gRPC）必須發生在 service 的 @Transactional 之外。
            // API 路徑走 best-effort——賣家按一次「確認收貨」不該因 buyer-service 抖一下就整筆失敗。
            io.github.will413028.hawkr.aftersales.service.EventPrincipals principals =
                    returnReceiptService.resolvePrincipalsForCaseBestEffort(request.getCaseNo());

            returnReceiptService.confirmReceipt(
                    storeId,
                    request.getCaseNo(),
                    actualRefundAmount,
                    request.getDeductionReason(),
                    request.getInspectionPhotoTokensList(),
                    request.getInspectionNote(),
                    principals
            );

            observer.onNext(ConfirmReturnReceiptResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer, ConfirmReturnReceiptResponse.newBuilder(),
                    e, CLASS_NAME, "confirmReturnReceipt");
        }
    }

    @Override
    public void retryPrintReturnLabel(io.github.will413028.hawkr.aftersales.grpc.RetryPrintReturnLabelRequest request,
                                       StreamObserver<io.github.will413028.hawkr.aftersales.grpc.RetryPrintReturnLabelResponse> observer) {
        try {
            // §5.3.6：admin/ops 介入重試 7-11 退貨出貨便代碼
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            long accountId = request.getAccountId();
            String caseNo = request.getCaseNo();

            labelRetryService.retryManual(accountId, caseNo);

            observer.onNext(io.github.will413028.hawkr.aftersales.grpc.RetryPrintReturnLabelResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    io.github.will413028.hawkr.aftersales.grpc.RetryPrintReturnLabelResponse.newBuilder(),
                    e, CLASS_NAME, "retryPrintReturnLabel");
        }
    }

    @Override
    public void cancelShipment(CancelShipmentRequest request,
                                StreamObserver<CancelShipmentResponse> observer) {
        // #1871 OBS-09：order_no 進 MDC，讓這個方法內所有 log 行都能被 Loki structured
        // metadata 查到（見 k8s/monitoring/promtail-daemonset.yaml）。
        MDC.put("order_no", request.getOrderNo());
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER);
            long storeId = request.getStoreId();

            sellerCancelService.cancelShipment(
                    storeId,
                    request.getOrderNo(),
                    request.getReason(),
                    request.getRemark()
            );

            // 查詢取消請求結果
            SellerCancelRequestEntity cancelEntity = cancelMapper.selectOne(
                    new LambdaQueryWrapper<SellerCancelRequestEntity>()
                            .eq(SellerCancelRequestEntity::getOrderNo, request.getOrderNo())
                            .orderByDesc(SellerCancelRequestEntity::getCreatedAt)
                            .last("LIMIT 1")
            );

            boolean needRefundPage = false;
            Long refundAmount = 0L;
            if (cancelEntity != null) {
                refundAmount = cancelEntity.getRefundAmount();
                if (PaymentMethod.ATM.value().equals(cancelEntity.getPaymentMethod())
                        && SellerCancelStatus.REFUND_PROCESSING.value().equals(cancelEntity.getStatus())) {
                    needRefundPage = true;
                }
            }

            observer.onNext(CancelShipmentResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(CancelShipmentResultData.newBuilder()
                            .setOrderNo(request.getOrderNo())
                            .setRefundAmount(ProtoConversionUtils.formatLong(refundAmount))
                            .setNeedRefundPage(needRefundPage)
                            .build())
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer, CancelShipmentResponse.newBuilder(),
                    e, CLASS_NAME, "cancelShipment");
        } finally {
            MDC.remove("order_no");
        }
    }

    // ==================== 內部 RPC ====================

    /**
     * ATM 退款頁面完成回調。
     * Gateway 接收 PayUni Form POST → 提取 EncryptInfo + HashInfo → 呼叫此 RPC。
     * 委派 Payment Service 解密 → 更新退款任務狀態 → 回傳 Deep Link。
     */
    @Override
    public void handleRefundPageReturn(HandleRefundPageReturnRequest request,
                                        StreamObserver<HandleRefundPageReturnResponse> observer) {
        try {
            String encryptInfo = request.getEncryptInfo();
            String hashInfo = request.getHashInfo();

            // 1. 委派 Payment Service 解密；失敗時 payment-service 會拋 StatusRuntimeException，
            //    由下方 catch 處理
            var decryptResp = paymentGrpcClient.decryptPayUniResult(encryptInfo, hashInfo);
            var data = decryptResp.getData();
            String payuniStatus = data.getStatus();
            String merTradeNo = data.getMerTradeNo();
            String refundAmt = data.getRefundAmt();
            String estimatedRefundDate = data.getEstimatedRefundDate();

            // Gateway 從 return_url query param 傳的 order_no，作為 MerTradeNo 空字串時的 fallback
            // Why: PayUni 失敗回傳（如 REF02xxx）MerTradeNo 可能為空，光靠 MerTradeNo 會找不到 refund_task，
            //      導致 §5.3 UX 持久性（失敗 snapshot）失效
            String fallbackOrderNo = request.getOrderNo();
            log.info("AfterSalesGrpcService.handleRefundPageReturn.decrypted",
                    "解密成功, status=" + payuniStatus + ", merTradeNo=" + merTradeNo
                            + ", fallbackOrderNo=" + fallbackOrderNo);

            // 2. 查詢退款任務：優先用 MerTradeNo(=transaction_id)；為空時用 order_no fallback
            LambdaQueryWrapper<RefundTaskEntity> wrapper = new LambdaQueryWrapper<RefundTaskEntity>()
                    .eq(RefundTaskEntity::getStatus, WorkerTaskStatus.AWAITING_BUYER_REFUND_PAGE.value())
                    .last("LIMIT 1");
            if (merTradeNo != null && !merTradeNo.isEmpty()) {
                wrapper.eq(RefundTaskEntity::getTransactionId, merTradeNo);
            } else if (fallbackOrderNo != null && !fallbackOrderNo.isEmpty()) {
                wrapper.eq(RefundTaskEntity::getOrderNo, fallbackOrderNo);
            } else {
                // 兩個都沒有：無法定位 refund_task，只能跳過
                log.warn("AfterSalesGrpcService.handleRefundPageReturn.noIdentifier",
                        "缺少 MerTradeNo 與 order_no，無法定位退款任務");
                observer.onNext(HandleRefundPageReturnResponse.newBuilder()
                        .setCode(GrpcResponseCode.SUCCESS.code()).setMessage(GrpcResponseCode.SUCCESS.message())
                        .setRedirectUrl(buildRefundErrorRedirectUrl(
                                ClientType.MOBILE_APP.value(), AppType.BUYER.value(),
                                "", merTradeNo, payuniStatus, "退款結果遺失，請聯繫客服"))
                        .build());
                observer.onCompleted();
                return;
            }
            RefundTaskEntity refundTask = refundTaskMapper.selectOne(wrapper);

            if (refundTask == null) {
                log.warn("AfterSalesGrpcService.handleRefundPageReturn.noTask",
                        "找不到等待中的退款任務", "merTradeNo", merTradeNo);
                observer.onNext(HandleRefundPageReturnResponse.newBuilder()
                        .setCode(GrpcResponseCode.SUCCESS.code()).setMessage(GrpcResponseCode.SUCCESS.message())
                        .setRedirectUrl(buildRefundErrorRedirectUrl(
                                ClientType.MOBILE_APP.value(), AppType.BUYER.value(),
                                fallbackOrderNo != null ? fallbackOrderNo : "",
                                merTradeNo, payuniStatus, "找不到退款任務"))
                        .build());
                observer.onCompleted();
                return;
            }

            String orderNo = refundTask.getOrderNo();
            String clientType = refundTask.getClientType();
            String appType = refundTask.getAppType();

            // 3. 依 PayUni 回傳狀態更新退款任務（INV-3 對齊：禁用 updateById 全欄位，走 conditional update SQL）
            if (AfterSalesConstants.PAYUNI_REFUND_PAGE_SUCCESS.equals(payuniStatus)) {
                // 成功：買家已填寫銀行資訊 → 推進為「等待 Webhook」
                // 09 §2.2 INV-3 對齊：transition AWAITING_BUYER_REFUND_PAGE(5) → AWAITING_WEBHOOK(15)
                java.time.LocalDate estimatedRefundDateParsed = parseEstimatedRefundDate(estimatedRefundDate);
                int rows = refundTaskMapper.markAtmRefundPageSucceeded(
                        refundTask.getId(), payuniStatus, estimatedRefundDateParsed);
                if (rows == 0) {
                    log.info("AfterSalesGrpcService.handleRefundPageReturn.staleState",
                            "refundTask 已不在 AWAITING_BUYER_REFUND_PAGE，跳過 transition",
                            "refundTaskId", String.valueOf(refundTask.getId()));
                }

                // 收斂規範 §4.4 sync 階段：通知 Payment 推進為 PROCESSING + estimatedRefundDate 落地
                try {
                    String refundNo = refundTask.getExternalRefId();
                    if (refundNo != null && !refundNo.isBlank()) {
                        paymentGrpcClient.ackAtmRefundAccepted(refundNo, payuniStatus, "",
                                estimatedRefundDate);
                    } else {
                        log.warn("AfterSalesGrpcService.handleRefundPageReturn.missingExternalRefId",
                                "refund_tasks.external_ref_id 為空，跳過 Payment sync ack",
                                "refundTaskId", String.valueOf(refundTask.getId()));
                    }
                } catch (Exception ackEx) {
                    log.warn("AfterSalesGrpcService.handleRefundPageReturn.ackFailed",
                            "Payment ackAtmRefundAccepted 失敗，不阻斷主流程 err=" + ackEx.getMessage());
                }

                log.info("AfterSalesGrpcService.handleRefundPageReturn.success",
                        "退款任務推進為等待回調", "refundTaskId", String.valueOf(refundTask.getId()));

                observer.onNext(HandleRefundPageReturnResponse.newBuilder()
                        .setCode(GrpcResponseCode.SUCCESS.code()).setMessage(GrpcResponseCode.SUCCESS.message())
                        .setRedirectUrl(buildRefundSuccessRedirectUrl(
                                clientType, appType, orderNo, merTradeNo, refundAmt, estimatedRefundDate))
                        .build());
            } else {
                // 失敗：issue #1288 健壯化（09 §5.4.3.1）
                String errMsg = data.getErrorMessage();

                // P1-2：payment 端狀態查詢（含同步 gRPC）在 DB tx 外先執行，只把判定結果帶進 tx 內做 CAS 寫入，
                //       避免同步 gRPC 佔用 DB tx 連線耗盡連線池。
                var paymentState = atmRefundReconciler.queryPaymentRefundState(refundTask);
                final RefundTaskEntity boundTask = refundTask;

                if (AfterSalesConstants.PAYUNI_REFUND_NO_BALANCE.equals(payuniStatus)) {
                    // 缺陷 C：REF02037（無足夠金額可退款 = 已退過）不 revert，改對帳 payment 端狀態。
                    // P1-1：多筆寫入（task transition + close case + audit + publish）以 tx 包覆確保原子。
                    var state = refundReconcileTxTemplate.execute(status ->
                            atmRefundReconciler.reconcileNoBalance(paymentState, boundTask, payuniStatus, errMsg));
                    String redirectUrl = switch (state) {
                        // SUCCEEDED / PROCESSING → 退款完成 / 處理中（success 螢幕不含重填按鈕，打斷迴圈）；
                        // UNDETERMINED → MANUAL_INTERVENTION，同樣導向非重填螢幕等待客服
                        case SUCCEEDED, PROCESSING, UNDETERMINED -> buildRefundSuccessRedirectUrl(
                                clientType, appType, orderNo, merTradeNo, refundAmt, estimatedRefundDate);
                    };
                    log.warn("AfterSalesGrpcService.handleRefundPageReturn.noBalance",
                            "REF02037 對帳處置 state=" + state, "order_no", orderNo);
                    observer.onNext(HandleRefundPageReturnResponse.newBuilder()
                            .setCode(GrpcResponseCode.SUCCESS.code()).setMessage(GrpcResponseCode.SUCCESS.message())
                            .setRedirectUrl(redirectUrl)
                            .build());
                } else if (Boolean.TRUE.equals(refundReconcileTxTemplate.execute(status ->
                        atmRefundReconciler.advanceIfPaymentSucceeded(paymentState, boundTask, payuniStatus, errMsg)))) {
                    // 缺陷 B：一般失敗碼但 payment 端已 SUCCEEDED → 不 revert，改推進完成（推進寫入包在 tx 內）
                    observer.onNext(HandleRefundPageReturnResponse.newBuilder()
                            .setCode(GrpcResponseCode.SUCCESS.code()).setMessage(GrpcResponseCode.SUCCESS.message())
                            .setRedirectUrl(buildRefundSuccessRedirectUrl(
                                    clientType, appType, orderNo, merTradeNo, refundAmt, estimatedRefundDate))
                            .build());
                } else {
                    // 一般失敗：state 保持 AWAITING_BUYER_REFUND_PAGE，stamp payuni_status / payuni_message + attempt++
                    // §2.2 業務 pivot (2026-05-14)：sync return URL 失敗統一走 revertFromAnyAwaitingToAwaitingPage
                    //   (status=5 → 5，attempt++)，與 async webhook 失敗路徑邏輯一致
                    refundTaskMapper.revertFromAnyAwaitingToAwaitingPage(refundTask.getId(), payuniStatus, errMsg);

                    log.warn("AfterSalesGrpcService.handleRefundPageReturn.failed",
                            "PayUni 退款頁面失敗, status=" + payuniStatus, "order_no", orderNo);

                    observer.onNext(HandleRefundPageReturnResponse.newBuilder()
                            .setCode(GrpcResponseCode.SUCCESS.code()).setMessage(GrpcResponseCode.SUCCESS.message())
                            .setRedirectUrl(buildRefundErrorRedirectUrl(
                                    clientType, appType, orderNo, merTradeNo, payuniStatus, errMsg))
                            .build());
                }
            }
            observer.onCompleted();

        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer, HandleRefundPageReturnResponse.newBuilder(),
                    e, CLASS_NAME, "handleRefundPageReturn");
        }
    }

    // ==================== Deep Link 建構 ====================

    /**
     * ATM 退款成功 Deep Link。
     */
    /**
     * 把 PayUni 回的 estimatedRefundDate（ISO 8601 YYYY-MM-DD）解析為 LocalDate。
     * 空 / 格式錯誤時回 null（不阻斷流程，markAtmRefundPageSucceeded 容許 NULL）。
     */
    private java.time.LocalDate parseEstimatedRefundDate(String s) {
        if (s == null || s.isBlank()) return null;
        try {
            return java.time.LocalDate.parse(s);
        } catch (Exception e) {
            log.warn("AfterSalesGrpcService.handleRefundPageReturn.parseDateFailed",
                    "estimatedRefundDate parse 失敗: " + s);
            return null;
        }
    }

    /**
     * 依 client_type × app_type 矩陣組裝成功 redirect_url。詳見 09 §5.4.2。
     * MOBILE_APP 走 deep link，WEB_BROWSER 走 {WEB_FRONTEND_URL}/orders/.../refund。
     * WEB_BROWSER + SELLER（Phase 1 不支援）fallback 到 WEB_BROWSER + BUYER 並寫日誌。
     */
    private String buildRefundSuccessRedirectUrl(String clientType, String appType,
                                                  String orderNo, String tradeNo,
                                                  String refundAmount, String estimatedRefundDate) {
        String safeOrderNo = orderNo != null ? orderNo : "";
        String safeAmount = refundAmount != null ? refundAmount : "";
        String safeDate = estimatedRefundDate != null ? estimatedRefundDate : "";
        String safeTradeNo = tradeNo != null ? tradeNo : "";
        String resolvedClient = resolveClientType(clientType);
        String resolvedApp = resolveAppType(appType);

        if (ClientType.WEB_BROWSER.value().equals(resolvedClient)) {
            resolveWebAppFallback(resolvedApp, safeOrderNo);
            String base = stripTrailingSlash(webFrontendUrl) + "/orders/" + urlEncode(safeOrderNo) + "/refund";
            return base + "?status=success"
                    + "&trade_no=" + urlEncode(safeTradeNo)
                    + "&estimated_refund_date=" + urlEncode(safeDate);
        }

        String scheme = AppType.SELLER.value().equals(resolvedApp) ? "hawkrseller" : "hawkrbuyer";
        return scheme + "://refund/result?orderNo=" + urlEncode(safeOrderNo)
                + "&status=success"
                + "&refundAmount=" + urlEncode(safeAmount)
                + "&estimatedRefundDate=" + urlEncode(safeDate);
    }

    /**
     * 依 client_type × app_type 矩陣組裝失敗 redirect_url。
     */
    private String buildRefundErrorRedirectUrl(String clientType, String appType,
                                                String orderNo, String tradeNo,
                                                String code, String errorMessage) {
        String safeOrderNo = orderNo != null ? orderNo : "";
        String safeError = errorMessage != null ? errorMessage : "退款失敗";
        String safeTradeNo = tradeNo != null ? tradeNo : "";
        String safeCode = code != null ? code : "";
        String resolvedClient = resolveClientType(clientType);
        String resolvedApp = resolveAppType(appType);

        if (ClientType.WEB_BROWSER.value().equals(resolvedClient)) {
            resolveWebAppFallback(resolvedApp, safeOrderNo);
            String base = stripTrailingSlash(webFrontendUrl) + "/orders/" + urlEncode(safeOrderNo) + "/refund";
            return base + "?status=failed"
                    + "&trade_no=" + urlEncode(safeTradeNo)
                    + "&code=" + urlEncode(safeCode);
        }

        String scheme = AppType.SELLER.value().equals(resolvedApp) ? "hawkrseller" : "hawkrbuyer";
        return scheme + "://refund/result?orderNo=" + urlEncode(safeOrderNo)
                + "&status=failed"
                + "&error=" + urlEncode(safeError);
    }

    private String resolveClientType(String value) {
        return ClientType.isValidValue(value) ? value : ClientType.MOBILE_APP.value();
    }

    private String resolveAppType(String value) {
        return AppType.isValidValue(value) ? value : AppType.BUYER.value();
    }

    // Phase 1: WEB_BROWSER + SELLER 不支援 → 退回 BUYER 路徑並留下軌跡
    private void resolveWebAppFallback(String appType, String orderNo) {
        if (AppType.SELLER.value().equals(appType)) {
            log.warn("AfterSalesGrpcService.handleRefundPageReturn.webSellerFallback",
                    "WEB_BROWSER+SELLER 尚未支援，fallback 為 BUYER", "order_no", orderNo);
        }
    }

    private static String stripTrailingSlash(String url) {
        if (url == null || url.isEmpty()) return "";
        return url.endsWith("/") ? url.substring(0, url.length() - 1) : url;
    }

    private static String urlEncode(String value) {
        return java.net.URLEncoder.encode(value, java.nio.charset.StandardCharsets.UTF_8);
    }

    // ==================== 私有方法 ====================

    /**
     * 組裝案件列表摘要。
     */
    private AfterSaleCaseSummary toSummary(AfterSaleCaseEntity entity, String buyerAvatarKey,
                                             ProviderStatusPair providerPair) {
        RefundTaskSnapshot snapshot = loadRefundTaskSnapshot(entity.getCaseNo());
        return AfterSaleCaseSummary.newBuilder()
                .setCaseNo(entity.getCaseNo())
                .setOrderNo(entity.getOrderNo())
                .setStatus(entity.getStatus())
                .setReason(nullToEmpty(entity.getReason()))
                .setRefundableAmount(ProtoConversionUtils.formatLong(entity.getRefundableAmount()))
                .setCreatedAt(ProtoConversionUtils.formatIso8601(entity.getCreatedAt()))
                .setNeedRefundPage(isNeedRefundPage(entity))
                .setBuyerName(nullToEmpty(entity.getBuyerName()))
                .setBuyerAvatarKey(nullToEmpty(buyerAvatarKey)) // 49 §6.6 S2+S9：只填 key，Gateway 組
                .setShippingAddress(composeShippingAddress(entity))
                .setReturnShippingMethod(nullToEmpty(entity.getReturnShippingMethod()))
                .setTemperatureCode(nullToEmpty(entity.getDeliveryTemp()))
                .setAppealState(nullToEmpty(entity.getAppealState()))
                .setRefundTaskStatus(snapshot.status)
                .setRefundFailureMessage(snapshot.failureMessage)
                .setEstimatedRefundDate(snapshot.estimatedRefundDate)
                .setBankDate(snapshot.bankDate)
                .setProviderStatus(providerPair.status)
                .setProviderMessage(providerPair.message)
                .setAutoConfirmAt(entity.getAutoConfirmAt() != null
                        ? ProtoConversionUtils.formatIso8601(entity.getAutoConfirmAt())
                        : "")
                // 2026-05-22 逆物流退貨政策修訂：自動完成原因由 status 31/32 直接表達，前端從 AfterSaleCaseStatus.label lookup
                .setStorePickupDeadlineAt(entity.getStorePickupDeadlineAt() != null
                        ? ProtoConversionUtils.formatIso8601(entity.getStorePickupDeadlineAt())
                        : "")
                // 2026-06-15 待寄回逾時政策修訂
                .setShipBackDeadlineAt(entity.getShipBackDeadlineAt() != null
                        ? ProtoConversionUtils.formatIso8601(entity.getShipBackDeadlineAt())
                        : "")
                .build();
    }

    /**
     * 批次解析一組 case 的 providerStatus / providerMessage（收斂規範 §6.3 禁止 N+1）。
     * REFUND_PROCESSING → 本地 refund_tasks 批次查；RETURN_SHIPPING → Shipping batch RPC（isReturn=true）。
     */
    private Map<String, ProviderStatusPair> batchResolveCaseProviderStatus(
            List<AfterSaleCaseEntity> cases) {
        Map<String, ProviderStatusPair> out = new HashMap<>();
        if (cases == null || cases.isEmpty()) return out;

        List<String> refundCaseNos = new java.util.ArrayList<>();
        List<AfterSaleCaseEntity> returnCases = new java.util.ArrayList<>();
        for (AfterSaleCaseEntity c : cases) {
            String st = c.getStatus();
            if (AfterSaleCaseStatus.REFUND_PROCESSING.value().equals(st)) {
                if (c.getCaseNo() != null && !c.getCaseNo().isBlank()) {
                    refundCaseNos.add(c.getCaseNo());
                }
            } else if (AfterSaleCaseStatus.RETURN_SHIPPING.value().equals(st)) {
                returnCases.add(c);
            }
        }

        // Bucket 1: REFUND_PROCESSING — 批次讀 refund_tasks.payuni_*（本地 snapshot）
        if (!refundCaseNos.isEmpty()) {
            List<RefundTaskEntity> tasks = refundTaskMapper.selectList(
                    new LambdaQueryWrapper<RefundTaskEntity>()
                            .in(RefundTaskEntity::getCaseNo, refundCaseNos));
            // 一個 case 可能有多筆 refund_task（重試等），取最新一筆
            Map<String, RefundTaskEntity> latestByCase = new HashMap<>();
            for (RefundTaskEntity t : tasks) {
                RefundTaskEntity prev = latestByCase.get(t.getCaseNo());
                if (prev == null) {
                    latestByCase.put(t.getCaseNo(), t);
                } else {
                    var prevAt = prev.getCreatedAt();
                    var curAt = t.getCreatedAt();
                    if (curAt != null && (prevAt == null || curAt.isAfter(prevAt))) {
                        latestByCase.put(t.getCaseNo(), t);
                    }
                }
            }
            for (Map.Entry<String, RefundTaskEntity> e : latestByCase.entrySet()) {
                out.put(e.getKey(), new ProviderStatusPair(
                        nullToEmpty(e.getValue().getPayuniStatus()),
                        nullToEmpty(e.getValue().getPayuniMessage())));
            }
        }

        // Bucket 2: RETURN_SHIPPING — 一次 batch RPC 到 Shipping
        if (!returnCases.isEmpty()) {
            out.putAll(resolveReturnShippingProviderStatus(returnCases));
        }

        return out;
    }

    /**
     * 組裝買家收件地址：city + district + addressDetail（不含郵遞區號）。
     */
    private String composeShippingAddress(AfterSaleCaseEntity entity) {
        StringBuilder sb = new StringBuilder();
        if (StringUtils.hasText(entity.getBuyerCity())) {
            sb.append(entity.getBuyerCity());
        }
        if (StringUtils.hasText(entity.getBuyerDistrict())) {
            sb.append(entity.getBuyerDistrict());
        }
        if (StringUtils.hasText(entity.getBuyerAddressDetail())) {
            sb.append(entity.getBuyerAddressDetail());
        }
        return sb.toString();
    }

    /**
     * 組裝案件詳情（含商品明細、舉證、決議歷程）。
     */
    private AfterSaleCaseDetailView buildDetailView(AfterSaleCaseEntity caseEntity) {
        String caseNo = caseEntity.getCaseNo();

        // 商品明細
        List<AfterSaleCaseItemEntity> items = caseItemMapper.selectList(
                new LambdaQueryWrapper<AfterSaleCaseItemEntity>()
                        .eq(AfterSaleCaseItemEntity::getCaseNo, caseNo)
        );

        // 舉證
        List<AfterSaleEvidenceEntity> evidenceList = evidenceMapper.selectList(
                new LambdaQueryWrapper<AfterSaleEvidenceEntity>()
                        .eq(AfterSaleEvidenceEntity::getCaseNo, caseNo)
                        .orderByAsc(AfterSaleEvidenceEntity::getCreatedAt)
        );

        // 決議歷程
        List<AfterSaleResolutionEntity> resolutions = resolutionMapper.selectList(
                new LambdaQueryWrapper<AfterSaleResolutionEntity>()
                        .eq(AfterSaleResolutionEntity::getCaseNo, caseNo)
                        .orderByAsc(AfterSaleResolutionEntity::getCreatedAt)
        );

        var detailBuilder = AfterSaleCaseDetailView.newBuilder()
                .setCaseNo(caseEntity.getCaseNo())
                .setOrderNo(caseEntity.getOrderNo())
                .setBuyerId(ProtoConversionUtils.formatLong(caseEntity.getBuyerId()))
                .setStoreId(ProtoConversionUtils.formatLong(caseEntity.getStoreId()))
                .setStatus(caseEntity.getStatus())
                .setReason(nullToEmpty(caseEntity.getReason()))
                .setDescription(nullToEmpty(caseEntity.getDescription()))
                .setRefundableAmount(ProtoConversionUtils.formatLong(caseEntity.getRefundableAmount()))
                .setReturnShipmentNo(nullToEmpty(caseEntity.getReturnShipmentNo()))
                .setReturnShippingMethod(nullToEmpty(caseEntity.getReturnShippingMethod()))
                .setShipTradeNo(nullToEmpty(caseEntity.getShipTradeNo()))
                .setPartnerId(nullToEmpty(caseEntity.getPartnerId()))
                .setOdno(nullToEmpty(caseEntity.getOdno()))
                .setValidationNo(nullToEmpty(caseEntity.getValidationNo()))
                .setNeedRefundPage(isNeedRefundPage(caseEntity))
                .setCreatedAt(ProtoConversionUtils.formatIso8601(caseEntity.getCreatedAt()));

        if (caseEntity.getClosedAt() != null) {
            detailBuilder.setClosedAt(ProtoConversionUtils.formatIso8601(caseEntity.getClosedAt()));
        }

        // 賣家出貨照片（從 Order Service 透傳 objectKey；Gateway 組 URL。失敗降級為空陣列，不影響主資料）
        try {
            var orderResp = orderGrpcClient.getOrderForAfterSales(caseEntity.getOrderNo());
            if (orderResp.getCode() == GrpcResponseCode.SUCCESS.code()
                    && orderResp.getData().getShipmentPhotoKeysCount() > 0) {
                detailBuilder.addAllShipmentPhotoKeys(orderResp.getData().getShipmentPhotoKeysList());
            }
        } catch (Exception e) {
            log.error("AfterSalesGrpcService.buildDetailView.shipmentPhotoKeys.fail",
                    "查詢出貨照片失敗，降級為空陣列, caseNo=" + caseNo + ": " + e.getMessage(),
                    "order_no", caseEntity.getOrderNo(), e);
        }

        // 商品明細
        for (AfterSaleCaseItemEntity item : items) {
            // 49 §6.6：只填 objectKey，Gateway 組 imageUrl
            String imageKey = item.getImageObjectKey() != null ? item.getImageObjectKey() : "";
            detailBuilder.addItems(AfterSaleCaseItemView.newBuilder()
                    .setOrderItemId(ProtoConversionUtils.formatLong(item.getOrderItemId()))
                    .setSkuId(ProtoConversionUtils.formatLong(item.getSkuId()))
                    .setSkuName(nullToEmpty(item.getSkuName()))
                    .setQuantity(ProtoConversionUtils.formatInt(item.getQuantity()))
                    .setOriginalPrice(ProtoConversionUtils.formatLong(item.getOriginalPrice()))
                    .setActualPayAmount(ProtoConversionUtils.formatLong(item.getActualPayAmount()))
                    .setImageKey(imageKey)
                    .build());
        }

        // 舉證圖片
        for (AfterSaleEvidenceEntity evidence : evidenceList) {
            // 49 §6.6：只填 objectKey，Gateway 組 imageUrl
            String imageKey = evidence.getObjectKey() != null ? evidence.getObjectKey() : "";
            detailBuilder.addEvidence(EvidenceView.newBuilder()
                    .setAssetId(ProtoConversionUtils.formatLong(evidence.getAssetId()))
                    .setImageKey(imageKey)
                    .setEvidenceType(nullToEmpty(evidence.getEvidenceType()))
                    .setCreatedAt(ProtoConversionUtils.formatIso8601(evidence.getCreatedAt()))
                    .build());
        }

        // 決議歷程
        for (AfterSaleResolutionEntity resolution : resolutions) {
            detailBuilder.addResolutions(ResolutionView.newBuilder()
                    .setResolutionType(nullToEmpty(resolution.getResolutionType()))
                    .setRefundAmount(resolution.getRefundAmount() != null
                            ? ProtoConversionUtils.formatLong(resolution.getRefundAmount()) : "")
                    .setCouponAmount(resolution.getCouponAmount() != null
                            ? ProtoConversionUtils.formatLong(resolution.getCouponAmount()) : "")
                    .setCouponMinSpend(resolution.getCouponMinSpend() != null
                            ? ProtoConversionUtils.formatLong(resolution.getCouponMinSpend()) : "")
                    .setCouponMessage(nullToEmpty(resolution.getCouponMessage()))
                    .setRejectReason(nullToEmpty(resolution.getRejectReason()))
                    .setRejectMessage(nullToEmpty(resolution.getRejectMessage()))
                    .setNegotiationReason(nullToEmpty(resolution.getNegotiationReason()))
                    .setCreatedAt(ProtoConversionUtils.formatIso8601(resolution.getCreatedAt()))
                    .setDeductionReason(nullToEmpty(resolution.getDeductionReason()))
                    .setInspectionNote(nullToEmpty(resolution.getInspectionNote()))
                    .build());
        }

        // §10 appeal 狀態欄位
        detailBuilder.setAppealState(nullToEmpty(caseEntity.getAppealState()));
        java.time.LocalDateTime deadline = appealService.computeAppealDeadline(caseEntity);
        if (deadline != null
                && AfterSaleCaseStatus.REJECTED.value().equals(caseEntity.getStatus())
                && AfterSaleAppealState.NONE.value().equals(nullToEmpty(caseEntity.getAppealState()))) {
            detailBuilder.setAppealDeadline(ProtoConversionUtils.formatIso8601(deadline));
        }
        detailBuilder.setCanAppeal(appealService.canAppeal(caseEntity));

        // §2.2 退款任務欄位（snapshot from refund_tasks）
        RefundTaskSnapshot snapshot = loadRefundTaskSnapshot(caseEntity.getCaseNo());
        detailBuilder.setRefundTaskStatus(snapshot.status);
        detailBuilder.setRefundFailureMessage(snapshot.failureMessage);
        detailBuilder.setEstimatedRefundDate(snapshot.estimatedRefundDate);
        detailBuilder.setBankDate(snapshot.bankDate);

        // 收斂規範 §6.2：依 case 當下 status 決定 provider 取源
        ProviderStatusPair providerPair = resolveCaseProviderStatus(caseEntity, snapshot);
        detailBuilder.setProviderStatus(providerPair.status);
        detailBuilder.setProviderMessage(providerPair.message);

        // §5.10 賣家處理期限欄位（僅 status=PENDING 時 process_deadline 才有值）
        if (caseEntity.getProcessDeadline() != null) {
            detailBuilder.setProcessDeadline(
                    ProtoConversionUtils.formatIso8601(caseEntity.getProcessDeadline()));
        }
        detailBuilder.setAutoEscalated(Boolean.TRUE.equals(caseEntity.getAutoEscalated()));
        detailBuilder.setProcessDeadlineHours(
                afterSalesProperties.getCaseConfig().getProcessDeadlineHours());

        // §5.3.10 / §5.3.11 退貨送達自動確認 + 物流失敗欄位
        if (caseEntity.getAutoConfirmAt() != null) {
            detailBuilder.setAutoConfirmAt(
                    ProtoConversionUtils.formatIso8601(caseEntity.getAutoConfirmAt()));
        }
        if (caseEntity.getReturnDeliveredAt() != null) {
            detailBuilder.setReturnDeliveredAt(
                    ProtoConversionUtils.formatIso8601(caseEntity.getReturnDeliveredAt()));
        }
        if (caseEntity.getReturnProviderStatus() != null) {
            detailBuilder.setReturnProviderStatus(caseEntity.getReturnProviderStatus());
        }
        if (caseEntity.getReturnProviderMessage() != null) {
            detailBuilder.setReturnProviderMessage(caseEntity.getReturnProviderMessage());
        }
        // 2026-05-22 逆物流退貨政策修訂欄位
        if (caseEntity.getStoreArrivedAt() != null) {
            detailBuilder.setStoreArrivedAt(
                    ProtoConversionUtils.formatIso8601(caseEntity.getStoreArrivedAt()));
        }
        if (caseEntity.getStorePickupDeadlineAt() != null) {
            detailBuilder.setStorePickupDeadlineAt(
                    ProtoConversionUtils.formatIso8601(caseEntity.getStorePickupDeadlineAt()));
        }
        if (caseEntity.getSellerPickedUpAt() != null) {
            detailBuilder.setSellerPickedUpAt(
                    ProtoConversionUtils.formatIso8601(caseEntity.getSellerPickedUpAt()));
        }
        if (caseEntity.getAutoCompleteAtStamp() != null) {
            detailBuilder.setAutoCompleteAtStamp(
                    ProtoConversionUtils.formatIso8601(caseEntity.getAutoCompleteAtStamp()));
        }
        // 2026-06-15 待寄回逾時政策修訂
        if (caseEntity.getShipBackDeadlineAt() != null) {
            detailBuilder.setShipBackDeadlineAt(
                    ProtoConversionUtils.formatIso8601(caseEntity.getShipBackDeadlineAt()));
        }
        // 自動完成原因由 status 31/32 表達；前端從 AfterSaleCaseStatus.label lookup
        // 賣家「確認收貨」按鈕：前端直接以 status === "27" 判斷，後端不再回 can_confirm_receipt 冗餘 boolean

        return detailBuilder.build();
    }

    /** case 聚合 provider 結果，空字串代表無值（收斂規範 §6.2 部分失敗容忍）。 */
    private record ProviderStatusPair(String status, String message) {
        static final ProviderStatusPair EMPTY = new ProviderStatusPair("", "");
    }

    /**
     * 依 case 當下 status 決定 providerStatus / providerMessage 取源：
     * - 退款相關（REFUND_PROCESSING）→ refund_tasks.payuni_*（本地 snapshot）
     * - 退貨物流相關（RETURN_SHIPPING）→ 呼 Shipping.BatchGetProviderStatusByOrderIds(isReturn=true)（單筆）
     * - 其他狀態 → 空字串
     */
    private ProviderStatusPair resolveCaseProviderStatus(AfterSaleCaseEntity caseEntity,
                                                          RefundTaskSnapshot snapshot) {
        String status = caseEntity.getStatus();
        if (AfterSaleCaseStatus.REFUND_PROCESSING.value().equals(status)) {
            // 從 refund_tasks 讀 payuni_status / payuni_message 的 raw 值（不挑失敗才透傳）
            RefundTaskEntity task = refundTaskMapper.selectOne(
                    new LambdaQueryWrapper<RefundTaskEntity>()
                            .eq(RefundTaskEntity::getCaseNo, caseEntity.getCaseNo())
                            .orderByDesc(RefundTaskEntity::getCreatedAt)
                            .last("LIMIT 1"));
            if (task == null) return ProviderStatusPair.EMPTY;
            return new ProviderStatusPair(nullToEmpty(task.getPayuniStatus()),
                    nullToEmpty(task.getPayuniMessage()));
        }
        if (AfterSaleCaseStatus.RETURN_SHIPPING.value().equals(status)) {
            return resolveReturnShippingProviderStatus(List.of(caseEntity)).getOrDefault(
                    caseEntity.getCaseNo(), ProviderStatusPair.EMPTY);
        }
        return ProviderStatusPair.EMPTY;
    }

    /**
     * 批次取 RETURN_SHIPPING case 的 provider 狀態（呼叫 Shipping.BatchGetProviderStatusByOrderIds(isReturn=true)）。
     * 部分失敗容忍：整體失敗則所有 case 欄位回空，不阻擋主回應。
     */
    private Map<String, ProviderStatusPair> resolveReturnShippingProviderStatus(List<AfterSaleCaseEntity> cases) {
        Map<String, ProviderStatusPair> out = new HashMap<>();
        if (cases == null || cases.isEmpty()) return out;
        List<String> orderNos = cases.stream()
                .map(AfterSaleCaseEntity::getOrderNo)
                .filter(n -> n != null && !n.isBlank())
                .distinct()
                .toList();
        if (orderNos.isEmpty()) return out;
        try {
            var resp = shippingGrpcClient.batchGetProviderStatusByOrderIds(orderNos, true);
            Map<String, io.github.will413028.hawkr.shipping.grpc.ProviderStatusEntry> entries = resp.getEntriesMap();
            for (AfterSaleCaseEntity c : cases) {
                var entry = entries.get(c.getOrderNo());
                if (entry != null) {
                    out.put(c.getCaseNo(), new ProviderStatusPair(
                            nullToEmpty(entry.getProviderStatus()),
                            nullToEmpty(entry.getProviderMessage())));
                }
            }
        } catch (Exception e) {
            log.warn("AfterSalesGrpcService.resolveReturnShipping.fail",
                    "批次 Shipping provider 查詢失敗，欄位留空: " + e.getMessage());
        }
        return out;
    }

    /**
     * 退款任務 snapshot（供 Summary / Detail 組裝使用）。
     * 無對應 refund_task 時所有欄位為空字串。
     */
    private static final class RefundTaskSnapshot {
        static final RefundTaskSnapshot EMPTY = new RefundTaskSnapshot("", "", "", "");
        final String status;
        final String failureMessage;
        final String estimatedRefundDate;
        final String bankDate;

        RefundTaskSnapshot(String status, String failureMessage,
                           String estimatedRefundDate, String bankDate) {
            this.status = status;
            this.failureMessage = failureMessage;
            this.estimatedRefundDate = estimatedRefundDate;
            this.bankDate = bankDate;
        }
    }

    private RefundTaskSnapshot loadRefundTaskSnapshot(String caseNo) {
        if (caseNo == null || caseNo.isEmpty()) {
            return RefundTaskSnapshot.EMPTY;
        }
        RefundTaskEntity task = refundTaskMapper.selectOne(
                new LambdaQueryWrapper<RefundTaskEntity>()
                        .eq(RefundTaskEntity::getCaseNo, caseNo)
                        .last("LIMIT 1")
        );
        if (task == null) {
            return RefundTaskSnapshot.EMPTY;
        }
        // refund_failure_message 透傳規則見 §2.2.1：
        //  - state=5 (AWAITING_BUYER_REFUND_PAGE)：若 payuni_message 有值，代表買家上次填寫失敗，需讓前端看到錯誤原因並「重新填寫」
        //  - state=30 (FAILED)：終態失敗，客服介入
        //  - 其他狀態：payuni_message 可能是成功訊息（如 "退款轉匯處理成功"），不透傳以免污染 refund_failure_message 語意
        String failureMessage = "";
        String taskStatus = task.getStatus();
        if (WorkerTaskStatus.FAILED.value().equals(taskStatus)
                || WorkerTaskStatus.AWAITING_BUYER_REFUND_PAGE.value().equals(taskStatus)) {
            failureMessage = nullToEmpty(task.getPayuniMessage());
        }
        return new RefundTaskSnapshot(
                nullToEmpty(task.getStatus()),
                failureMessage,
                task.getEstimatedRefundDate() != null ? task.getEstimatedRefundDate().toString() : "",
                task.getBankDate() != null ? task.getBankDate().toString() : ""
        );
    }

    /**
     * 判斷案件是否需要買家操作 ATM 退款頁面。
     */
    private boolean isNeedRefundPage(AfterSaleCaseEntity entity) {
        if (!PaymentMethod.ATM.value().equals(entity.getPaymentMethod())) {
            return false;
        }
        if (!AfterSaleCaseStatus.REFUND_PROCESSING.value().equals(entity.getStatus())) {
            return false;
        }
        Long count = refundTaskMapper.selectCount(
                new LambdaQueryWrapper<RefundTaskEntity>()
                        .eq(RefundTaskEntity::getCaseNo, entity.getCaseNo())
                        .eq(RefundTaskEntity::getStatus, WorkerTaskStatus.AWAITING_BUYER_REFUND_PAGE.value())
        );
        return count != null && count > 0;
    }

    private static String nullToEmpty(String value) {
        return value != null ? value : "";
    }

    // ==================== §10 申訴與平台裁決 ====================

    @Override
    public void submitAppeal(SubmitAppealRequest request,
                              StreamObserver<SubmitAppealResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = request.getBuyerId();

            String appealId = appealService.submitAppeal(
                    buyerId,
                    request.getCaseNo(),
                    request.getReason(),
                    request.getDescription(),
                    request.getEvidencePhotoTokensList().isEmpty()
                            ? null : request.getEvidencePhotoTokensList()
            );

            observer.onNext(SubmitAppealResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(SubmitAppealData.newBuilder()
                            .setAppealId(appealId)
                            .setAppealState(AfterSaleAppealState.APPEALED.value())
                            .build())
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer, SubmitAppealResponse.newBuilder(),
                    e, CLASS_NAME, "submitAppeal");
        }
    }

    @Override
    public void getArbitration(GetArbitrationRequest request,
                                StreamObserver<GetArbitrationResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER);
            long buyerId = request.getBuyerId();

            var result = appealService.getArbitration(buyerId, request.getCaseNo());
            observer.onNext(buildArbitrationResponse(result));
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer, GetArbitrationResponse.newBuilder(),
                    e, CLASS_NAME, "getArbitration");
        }
    }

    @Override
    public void getSellerArbitration(GetSellerArbitrationRequest request,
                                      StreamObserver<GetArbitrationResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER);
            long storeId = request.getStoreId();

            var result = appealService.getSellerArbitration(storeId, request.getCaseNo());
            observer.onNext(buildArbitrationResponse(result));
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer, GetArbitrationResponse.newBuilder(),
                    e, CLASS_NAME, "getSellerArbitration");
        }
    }

    @Override
    public void listAppealsForAdmin(ListAppealsForAdminRequest request,
                                     StreamObserver<ListAppealsForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            int page = ProtoConversionUtils.parseIntOrDefault(request.getPage(), 1);
            int pageSize = ProtoConversionUtils.parseIntOrDefault(request.getPageSize(), 20);

            var result = appealService.listAppealsForAdmin(
                    request.getAppealState(), page, pageSize);

            // §10.6.6 批次查 buyer + seller profile（降級：失敗回空字串）
            java.util.List<Long> buyerIds = new java.util.ArrayList<>();
            java.util.List<Long> sellerIds = new java.util.ArrayList<>();
            for (AfterSaleCaseEntity entity : result.getRecords()) {
                if (entity.getBuyerId() != null) buyerIds.add(entity.getBuyerId());
                if (entity.getStoreId() != null) sellerIds.add(entity.getStoreId());
            }
            java.util.Map<Long, io.github.will413028.hawkr.buyer.grpc.BuyerDisplay> buyerMap =
                    buyerIds.isEmpty() ? java.util.Collections.emptyMap()
                            : safeBatchGetBuyerDisplay(buyerIds);
            java.util.Map<Long, io.github.will413028.hawkr.seller.grpc.SellerInfo> sellerMap =
                    sellerIds.isEmpty() ? java.util.Collections.emptyMap()
                            : sellerGrpcClient.getSellerProfiles(sellerIds);

            var dataBuilder = ListAppealsForAdminData.newBuilder()
                    .setTotal(String.valueOf(result.getTotal()))
                    .setPage(String.valueOf(result.getPage()))
                    .setPageSize(String.valueOf(result.getPageSize()));
            for (AfterSaleCaseEntity entity : result.getRecords()) {
                var buyer = buyerMap.get(entity.getBuyerId());
                var seller = sellerMap.get(entity.getStoreId());
                dataBuilder.addRecords(AppealSummaryView.newBuilder()
                        .setCaseNo(nullToEmpty(entity.getCaseNo()))
                        .setOrderNo(nullToEmpty(entity.getOrderNo()))
                        .setBuyerId(ProtoConversionUtils.formatLong(entity.getBuyerId()))
                        .setStoreId(ProtoConversionUtils.formatLong(entity.getStoreId()))
                        .setAppealState(nullToEmpty(entity.getAppealState()))
                        .setAppealSubmittedAt(entity.getAppealSubmittedAt() != null
                                ? ProtoConversionUtils.formatIso8601(entity.getAppealSubmittedAt()) : "")
                        .setLastRejectedAt(entity.getLastRejectedAt() != null
                                ? ProtoConversionUtils.formatIso8601(entity.getLastRejectedAt()) : "")
                        .setRefundableAmount(ProtoConversionUtils.formatLong(entity.getRefundableAmount()))
                        .setBuyerName(nullToEmpty(entity.getBuyerName()))
                        .setBuyerAvatarKey(buyer != null ? nullToEmpty(buyer.getAvatarKey()) : "") // 49 §6.6 S2+S9
                        .setSellerName(seller != null ? nullToEmpty(seller.getName()) : "")
                        .setSellerAvatarKey(seller != null ? nullToEmpty(seller.getLogoKey()) : "") // 49 §6.6 S2+S9
                        .build());
            }
            observer.onNext(ListAppealsForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(dataBuilder.build())
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer, ListAppealsForAdminResponse.newBuilder(),
                    e, CLASS_NAME, "listAppealsForAdmin");
        }
    }

    /** §10.6.6 Buyer profile 降級包裝：查詢失敗回空 map。 */
    private java.util.Map<Long, io.github.will413028.hawkr.buyer.grpc.BuyerDisplay> safeBatchGetBuyerDisplay(java.util.List<Long> buyerIds) {
        try {
            return buyerGrpcClient.batchGetBuyerDisplayByBuyerIds(buyerIds);
        } catch (Exception e) {
            log.warn("AfterSalesGrpcService.safeBatchGetBuyerDisplay.error",
                    "買家資訊查詢失敗，降級: " + e.getMessage(), "count", String.valueOf(buyerIds.size()));
            return java.util.Collections.emptyMap();
        }
    }

    @Override
    public void decideAppealForAdmin(DecideAppealForAdminRequest request,
                                      StreamObserver<DecideAppealForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            long adminId = request.getAccountId();
            Long refundAmount = ProtoConversionUtils.parseLongOrNull(request.getRefundAmount());

            platformDecisionService.decide(
                    adminId,
                    request.getCaseNo(),
                    request.getOutcome(),
                    refundAmount,
                    request.getReason()
            );

            observer.onNext(DecideAppealForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(DecideAppealForAdminData.newBuilder()
                            .setCaseNo(request.getCaseNo())
                            .setOutcome(request.getOutcome())
                            .setRefundAmount(nullToEmpty(request.getRefundAmount()))
                            .build())
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer, DecideAppealForAdminResponse.newBuilder(),
                    e, CLASS_NAME, "decideAppealForAdmin");
        }
    }

    /**
     * 組裝 GetArbitrationResponse（買家 / 賣家共用）。
     */
    private GetArbitrationResponse buildArbitrationResponse(AfterSaleAppealService.ArbitrationResult result) {
        var dataBuilder = ArbitrationData.newBuilder()
                .setState(nullToEmpty(result.getAppealState()));

        if (result.getEvents() != null) {
            for (AfterSaleAppealEventEntity ev : result.getEvents()) {
                dataBuilder.addTimeline(AppealEventView.newBuilder()
                        .setEventType(nullToEmpty(ev.getEventType()))
                        .setTimestamp(ProtoConversionUtils.formatIso8601(ev.getCreatedAt()))
                        .setLabel(nullToEmpty(ev.getLabel()))
                        .setDescription(nullToEmpty(ev.getDescription()))
                        .build());
            }
        }

        if (result.getDecision() != null) {
            var d = result.getDecision();
            dataBuilder.setDecision(PlatformDecisionView.newBuilder()
                    .setOutcome(nullToEmpty(d.getOutcome()))
                    .setRefundAmount(d.getRefundAmount() != null
                            ? ProtoConversionUtils.formatLong(d.getRefundAmount()) : "0")
                    .setReason(nullToEmpty(d.getReason()))
                    .setDecidedAt(ProtoConversionUtils.formatIso8601(d.getDecidedAt()))
                    .build());
        }
        return GetArbitrationResponse.newBuilder()
                .setCode(GrpcResponseCode.SUCCESS.code())
                .setMessage(GrpcResponseCode.SUCCESS.message())
                .setData(dataBuilder.build())
                .build();
    }

    // ==================== §10.6.6 Admin 單案詳情 ====================

    @Override
    public void getAppealDetailForAdmin(GetAppealDetailForAdminRequest request,
                                         StreamObserver<GetAppealDetailForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            String caseNo = request.getCaseNo();

            var result = appealService.getAppealDetailForAdmin(caseNo);
            AfterSaleCaseEntity caseEntity = result.getCaseEntity();

            // 1) caseDetail 重用 buildDetailView
            AfterSaleCaseDetailView caseDetail = buildDetailView(caseEntity);

            // 2) arbitration 重用組裝（appeal_state=NONE 時 state="0"、timeline/decision 空）
            ArbitrationData.Builder arbBuilder = ArbitrationData.newBuilder()
                    .setState(nullToEmpty(caseEntity.getAppealState()));
            if (result.getAppealEvents() != null) {
                for (AfterSaleAppealEventEntity ev : result.getAppealEvents()) {
                    arbBuilder.addTimeline(AppealEventView.newBuilder()
                            .setEventType(nullToEmpty(ev.getEventType()))
                            .setTimestamp(ProtoConversionUtils.formatIso8601(ev.getCreatedAt()))
                            .setLabel(nullToEmpty(ev.getLabel()))
                            .setDescription(nullToEmpty(ev.getDescription()))
                            .build());
                }
            }
            if (result.getDecision() != null) {
                var d = result.getDecision();
                arbBuilder.setDecision(PlatformDecisionView.newBuilder()
                        .setOutcome(nullToEmpty(d.getOutcome()))
                        .setRefundAmount(d.getRefundAmount() != null
                                ? ProtoConversionUtils.formatLong(d.getRefundAmount()) : "0")
                        .setReason(nullToEmpty(d.getReason()))
                        .setDecidedAt(ProtoConversionUtils.formatIso8601(d.getDecidedAt()))
                        .build());
            }

            // 3) adminContext — profile + 從最新 REJECT resolution 與 decision 抽取
            io.github.will413028.hawkr.buyer.grpc.BuyerDisplay buyerDisplay = null;
            if (caseEntity.getBuyerId() != null) {
                var m = safeBatchGetBuyerDisplay(java.util.List.of(caseEntity.getBuyerId()));
                buyerDisplay = m.get(caseEntity.getBuyerId());
            }
            io.github.will413028.hawkr.seller.grpc.SellerInfo sellerInfo = null;
            if (caseEntity.getStoreId() != null) {
                var m = sellerGrpcClient.getSellerProfiles(java.util.List.of(caseEntity.getStoreId()));
                sellerInfo = m.get(caseEntity.getStoreId());
            }

            AdminContextView.Builder adminBuilder = AdminContextView.newBuilder()
                    .setBuyerAppealReason(nullToEmpty(caseEntity.getAppealReason()))
                    .setBuyerAppealDescription(nullToEmpty(caseEntity.getAppealDescription()))
                    .setBuyerName(nullToEmpty(caseEntity.getBuyerName()))
                    .setBuyerAvatarKey(buyerDisplay != null ? nullToEmpty(buyerDisplay.getAvatarKey()) : "") // 49 §6.6 S2+S9
                    .setSellerName(sellerInfo != null ? nullToEmpty(sellerInfo.getName()) : "")
                    .setSellerAvatarKey(sellerInfo != null ? nullToEmpty(sellerInfo.getLogoKey()) : ""); // 49 §6.6 S2+S9

            if (result.getLatestRejectResolution() != null) {
                var rej = result.getLatestRejectResolution();
                adminBuilder.setSellerRejectReason(nullToEmpty(rej.getRejectReason()))
                        .setSellerRejectMessage(nullToEmpty(rej.getRejectMessage()));
            }
            if (result.getDecision() != null) {
                adminBuilder.setDecidedByAdminId(
                        ProtoConversionUtils.formatLong(result.getDecision().getDecidedByAdminId()));
            }

            // 4) providerContext — 依 case.status 聚合 Payment / Shipping SoT（§29）
            AdminProviderContext providerContext = buildProviderContextForCase(caseEntity);

            AppealDetailForAdminData.Builder dataBuilder = AppealDetailForAdminData.newBuilder()
                    .setCaseDetail(caseDetail)
                    .setArbitration(arbBuilder.build())
                    .setAdminContext(adminBuilder.build());
            if (providerContext != null) {
                dataBuilder.setProviderContext(providerContext);
            }

            observer.onNext(GetAppealDetailForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(dataBuilder.build())
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer, GetAppealDetailForAdminResponse.newBuilder(),
                    e, CLASS_NAME, "getAppealDetailForAdmin");
        }
    }

    // ==================== §29 Admin 異常列表 ====================

    @Override
    public void listAfterSaleCasesForAdmin(ListAfterSaleCasesForAdminRequest request,
                                            StreamObserver<ListAfterSaleCasesForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            int pageNum = ProtoConversionUtils.parseIntOrDefault(request.getPage(), 1);
            int pageSize = Math.min(ProtoConversionUtils.parseIntOrDefault(request.getPageSize(), 20), 100);

            var wrapper = new com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper<AfterSaleCaseEntity>();

            // 異常 filter：REFUND_PROCESSING(40) / RETURN_SHIPPING(25) / RETURN_PENDING(20) 為主要異常候選狀態
            if (request.getAbnormalOnly()) {
                wrapper.in(AfterSaleCaseEntity::getStatus, java.util.List.of(
                        io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus.REFUND_PROCESSING.value(),
                        io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus.RETURN_PENDING.value(),
                        io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus.RETURN_SHIPPING.value()
                ));
            }
            if (org.springframework.util.StringUtils.hasText(request.getStatus())) {
                wrapper.in(AfterSaleCaseEntity::getStatus,
                        java.util.Arrays.asList(request.getStatus().split(",")));
            }
            if (org.springframework.util.StringUtils.hasText(request.getBuyerId())) {
                try {
                    wrapper.eq(AfterSaleCaseEntity::getBuyerId, Long.parseLong(request.getBuyerId()));
                } catch (NumberFormatException ignore) {}
            }
            if (org.springframework.util.StringUtils.hasText(request.getStoreId())) {
                try {
                    wrapper.eq(AfterSaleCaseEntity::getStoreId, Long.parseLong(request.getStoreId()));
                } catch (NumberFormatException ignore) {}
            }
            if (org.springframework.util.StringUtils.hasText(request.getOrderNo())) {
                wrapper.eq(AfterSaleCaseEntity::getOrderNo, request.getOrderNo());
            }

            String sort = request.getSort();
            if (!org.springframework.util.StringUtils.hasText(sort)) sort = "updated_at_desc";
            switch (sort) {
                case "updated_at_asc" -> wrapper.orderByAsc(AfterSaleCaseEntity::getUpdatedAt);
                case "created_at_desc" -> wrapper.orderByDesc(AfterSaleCaseEntity::getCreatedAt);
                case "created_at_asc" -> wrapper.orderByAsc(AfterSaleCaseEntity::getCreatedAt);
                default -> wrapper.orderByDesc(AfterSaleCaseEntity::getUpdatedAt);
            }

            com.baomidou.mybatisplus.extension.plugins.pagination.Page<AfterSaleCaseEntity> page =
                    new com.baomidou.mybatisplus.extension.plugins.pagination.Page<>(pageNum, pageSize);
            var result = caseMapper.selectPage(page, wrapper);

            // 批次聚合 refund provider_* （僅對 REFUND_PROCESSING 的 case 調用 Payment.BatchGetRefundProviderStatusByCaseIds）
            java.util.List<String> refundCaseNos = new java.util.ArrayList<>();
            String refundStatusVal = io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus.REFUND_PROCESSING.value();
            for (AfterSaleCaseEntity e : result.getRecords()) {
                if (refundStatusVal.equals(e.getStatus())) {
                    refundCaseNos.add(e.getCaseNo());
                }
            }
            java.util.Map<String, io.github.will413028.hawkr.payment.grpc.ProviderStatusEntry> refundMap = java.util.Collections.emptyMap();
            if (!refundCaseNos.isEmpty()) {
                try {
                    var resp = paymentGrpcClient.batchGetRefundProviderStatusByCaseIds(refundCaseNos);
                    if (resp.getCode() == 0) {
                        refundMap = resp.getEntriesMap();
                    }
                } catch (Exception ex) {
                    log.warn("AfterSalesGrpcService.listAfterSaleCasesForAdmin.batchRefundFailed",
                            "批次聚合退款 provider 狀態失敗: " + ex.getMessage());
                }
            }
            // 物流 provider 聚合（退貨物流異常狀態）— 以 order_no 為 key，is_return=true
            java.util.List<String> shippingOrderNos = new java.util.ArrayList<>();
            String returnPendingVal = io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus.RETURN_PENDING.value();
            String returnShippingVal = io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus.RETURN_SHIPPING.value();
            for (AfterSaleCaseEntity e : result.getRecords()) {
                if (returnPendingVal.equals(e.getStatus()) || returnShippingVal.equals(e.getStatus())) {
                    if (e.getOrderNo() != null) shippingOrderNos.add(e.getOrderNo());
                }
            }
            java.util.Map<String, io.github.will413028.hawkr.shipping.grpc.ProviderStatusEntry> shippingMap = java.util.Collections.emptyMap();
            if (!shippingOrderNos.isEmpty()) {
                try {
                    var resp = shippingGrpcClient.batchGetProviderStatusByOrderIds(shippingOrderNos, true);
                    if (resp.getCode() == 0) {
                        shippingMap = resp.getEntriesMap();
                    }
                } catch (Exception ex) {
                    log.warn("AfterSalesGrpcService.listAfterSaleCasesForAdmin.batchShippingFailed",
                            "批次聚合物流 provider 狀態失敗: " + ex.getMessage());
                }
            }

            ListAfterSaleCasesForAdminData.Builder dataBuilder = ListAfterSaleCasesForAdminData.newBuilder()
                    .setTotal(String.valueOf(result.getTotal()))
                    .setPage(String.valueOf(result.getCurrent()))
                    .setPageSize(String.valueOf(result.getSize()));

            for (AfterSaleCaseEntity e : result.getRecords()) {
                String ps = "", pm = "";
                if (refundStatusVal.equals(e.getStatus())) {
                    var entry = refundMap.get(e.getCaseNo());
                    if (entry != null) {
                        ps = entry.getProviderStatus();
                        pm = entry.getProviderMessage();
                    }
                } else if (returnPendingVal.equals(e.getStatus()) || returnShippingVal.equals(e.getStatus())) {
                    var entry = shippingMap.get(e.getOrderNo());
                    if (entry != null) {
                        ps = entry.getProviderStatus();
                        pm = entry.getProviderMessage();
                    }
                }
                dataBuilder.addRecords(AdminAfterSaleCaseSummary.newBuilder()
                        .setCaseNo(nullToEmpty(e.getCaseNo()))
                        .setOrderNo(nullToEmpty(e.getOrderNo()))
                        .setBuyerId(e.getBuyerId() == null ? "" : String.valueOf(e.getBuyerId()))
                        .setStoreId(e.getStoreId() == null ? "" : String.valueOf(e.getStoreId()))
                        .setStatus(nullToEmpty(e.getStatus()))
                        .setRefundableAmount(e.getRefundableAmount() == null ? "" : String.valueOf(e.getRefundableAmount()))
                        .setPaymentMethod(nullToEmpty(e.getPaymentMethod()))
                        .setProviderStatus(ps)
                        .setProviderMessage(pm)
                        .setCreatedAt(ProtoConversionUtils.formatIso8601(e.getCreatedAt()))
                        .setUpdatedAt(ProtoConversionUtils.formatIso8601(e.getUpdatedAt()))
                        .build());
            }

            observer.onNext(ListAfterSaleCasesForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(dataBuilder.build())
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer, ListAfterSaleCasesForAdminResponse.newBuilder(),
                    e, CLASS_NAME, "listAfterSaleCasesForAdmin");
        }
    }

    /**
     * §5.3.12 Admin 強制確認收貨。
     * 允許 status ∈ 20/25/27/28；走 ReturnReceiptService.forceConfirmReceiptByAdmin 共用流程。
     */
    @Override
    public void forceConfirmReceiptForAdmin(ForceConfirmReceiptForAdminRequest request,
                                              StreamObserver<ForceConfirmReceiptForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            long adminId = request.getAccountId();
            String caseNo = request.getCaseNo();
            String reason = request.getReason();

            if (reason == null || reason.isBlank()) {
                throw new io.github.will413028.hawkr.aftersales.exception.AfterSalesBusinessException(
                        GrpcResponseCode.INVALID_PARAM, "reason is required");
            }

            Long actualRefundAmount = null;
            String actualRaw = request.getActualRefundAmount();
            if (actualRaw != null && !actualRaw.isBlank()) {
                actualRefundAmount = ProtoConversionUtils.parseLongRequired(actualRaw, "actualRefundAmount");
            }

            // issue #2294 B7：同 confirmReturnReceipt——反解在 tx 外、API 路徑 best-effort
            io.github.will413028.hawkr.aftersales.service.EventPrincipals principals =
                    returnReceiptService.resolvePrincipalsForCaseBestEffort(caseNo);

            ReturnReceiptService.ForceConfirmResult result = returnReceiptService.forceConfirmReceiptByAdmin(
                    adminId, caseNo, reason, actualRefundAmount, request.getDeductionReason(), principals);

            observer.onNext(ForceConfirmReceiptForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(ForceConfirmReceiptForAdminData.newBuilder()
                            .setCaseNo(result.caseNo())
                            .setPriorStatus(result.priorStatus())
                            .setRefundTaskId(result.refundTaskId())
                            .setFinalRefundAmount(String.valueOf(result.finalRefundAmount()))
                            .build())
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer, ForceConfirmReceiptForAdminResponse.newBuilder(),
                    e, CLASS_NAME, "forceConfirmReceiptForAdmin");
        }
    }

    /**
     * §29 依 case 當下 status 聚合 provider_*。
     * REFUND_PROCESSING → 呼叫 Payment.GetRefund(refund_no=case_no)
     * RETURN_PENDING / RETURN_SHIPPING → 依 order_no 查 Shipping (is_return=true)
     * 其他 → null
     */
    private AdminProviderContext buildProviderContextForCase(AfterSaleCaseEntity caseEntity) {
        if (caseEntity == null || caseEntity.getStatus() == null) return null;
        String status = caseEntity.getStatus();
        String refundStatus = io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus.REFUND_PROCESSING.value();
        String returnPending = io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus.RETURN_PENDING.value();
        String returnShipping = io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus.RETURN_SHIPPING.value();

        if (refundStatus.equals(status)) {
            try {
                var resp = paymentGrpcClient.getRefund(caseEntity.getCaseNo());
                if (resp.getCode() == 0) {
                    var d = resp.getData();
                    return AdminProviderContext.newBuilder()
                            .setProviderSource(ProviderSource.PAYMENT_REFUND.value())
                            .setProviderStatus(nullToEmpty(d.getProviderStatus()))
                            .setProviderMessage(nullToEmpty(d.getProviderMessage()))
                            .setProviderRefundNo(nullToEmpty(d.getProviderRefundNo()))
                            .build();
                }
            } catch (Exception ex) {
                log.warn("AfterSalesGrpcService.buildProviderContextForCase.refundFailed",
                        "查詢退款 provider 狀態失敗: " + ex.getMessage());
            }
        } else if (returnPending.equals(status) || returnShipping.equals(status)) {
            // 退貨物流：用批次 RPC（傳單一 order_no）
            try {
                var resp = shippingGrpcClient.batchGetProviderStatusByOrderIds(
                        java.util.List.of(caseEntity.getOrderNo()), true);
                if (resp.getCode() == 0) {
                    var entry = resp.getEntriesMap().get(caseEntity.getOrderNo());
                    if (entry != null) {
                        return AdminProviderContext.newBuilder()
                                .setProviderSource(ProviderSource.SHIPPING.value())
                                .setProviderStatus(nullToEmpty(entry.getProviderStatus()))
                                .setProviderMessage(nullToEmpty(entry.getProviderMessage()))
                                .setShipTradeNo(nullToEmpty(caseEntity.getShipTradeNo()))
                                .build();
                    }
                }
            } catch (Exception ex) {
                log.warn("AfterSalesGrpcService.buildProviderContextForCase.shippingFailed",
                        "查詢退貨物流 provider 狀態失敗: " + ex.getMessage());
            }
        }
        return null;
    }

    /**
     * 訂單撥款資格反查（Order Service {@code GrantPaymentWorker} 撥款前呼叫，backend#3262）。
     *
     * <p>與 {@link #hasActiveCase} 是<b>不同的問題</b>：那支問「倒數要不要 hold」，
     * 這支問「還會不會再生出退款」——兩者在 {@code 60/33 ＋ 未申訴} 上分歧，而那個分歧
     * 正是本票的資損鏈路。
     *
     * <p>例外一律 fail-loud（回非 0 code）：呼叫端 fail-closed 視為 hold。
     * 撥錯款追不回來，撥晚了只是等——方向固定往「不撥」倒。
     */
    @Override
    public void getOrderPayoutHold(io.github.will413028.hawkr.aftersales.grpc.GetOrderPayoutHoldRequest request,
                                   StreamObserver<io.github.will413028.hawkr.aftersales.grpc.GetOrderPayoutHoldResponse> observer) {
        String orderNo = request.getOrderNo();
        if (orderNo == null || orderNo.isEmpty()) {
            observer.onNext(io.github.will413028.hawkr.aftersales.grpc.GetOrderPayoutHoldResponse.newBuilder()
                    .setCode(io.github.will413028.hawkr.common.utils.code.GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("order_no 為必填")
                    .build());
            observer.onCompleted();
            return;
        }
        try {
            io.github.will413028.hawkr.aftersales.service.AfterSalePayoutHoldService.PayoutHold hold =
                    payoutHoldService.getPayoutHold(orderNo);
            io.github.will413028.hawkr.aftersales.grpc.GetOrderPayoutHoldResponse.Builder builder =
                    io.github.will413028.hawkr.aftersales.grpc.GetOrderPayoutHoldResponse.newBuilder()
                            .setCode(0)
                            .setMessage("success")
                            .setHold(hold.hold())
                            .setReason(toProtoReason(hold.reason()))
                            .setCaseNo(nullToEmpty(hold.caseNo()));
            if (hold.holdUntil() != null) {
                builder.setHoldUntil(
                        io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils.formatIso8601(hold.holdUntil()));
            }
            observer.onNext(builder.build());
            observer.onCompleted();
        } catch (Exception e) {
            log.error("AfterSalesGrpcService.getOrderPayoutHold.error",
                    "撥款資格反查失敗 orderNo=" + orderNo, e);
            observer.onNext(io.github.will413028.hawkr.aftersales.grpc.GetOrderPayoutHoldResponse.newBuilder()
                    .setCode(io.github.will413028.hawkr.common.utils.code.GrpcResponseCode.INTERNAL_ERROR.code())
                    .setMessage("撥款資格反查失敗")
                    .build());
            observer.onCompleted();
        }
    }

    private static io.github.will413028.hawkr.aftersales.grpc.GetOrderPayoutHoldResponse.Reason toProtoReason(
            io.github.will413028.hawkr.aftersales.service.AfterSalePayoutHoldService.HoldReason reason) {
        return switch (reason) {
            case NONE -> io.github.will413028.hawkr.aftersales.grpc.GetOrderPayoutHoldResponse.Reason.NONE;
            case ACTIVE_CASE -> io.github.will413028.hawkr.aftersales.grpc.GetOrderPayoutHoldResponse.Reason.ACTIVE_CASE;
            case APPEAL_WINDOW_OPEN ->
                    io.github.will413028.hawkr.aftersales.grpc.GetOrderPayoutHoldResponse.Reason.APPEAL_WINDOW_OPEN;
            case APPEAL_IN_FLIGHT ->
                    io.github.will413028.hawkr.aftersales.grpc.GetOrderPayoutHoldResponse.Reason.APPEAL_IN_FLIGHT;
        };
    }

    /**
     * 訂單活躍 case 反查（Order Service 自動完成倒數恢復前呼叫，03 §5.8 / 09 §4.4.1）。
     */
    @Override
    public void hasActiveCase(io.github.will413028.hawkr.aftersales.grpc.HasActiveCaseRequest request,
                              StreamObserver<io.github.will413028.hawkr.aftersales.grpc.HasActiveCaseResponse> observer) {
        String orderNo = request.getOrderNo();
        if (orderNo == null || orderNo.isEmpty()) {
            observer.onNext(io.github.will413028.hawkr.aftersales.grpc.HasActiveCaseResponse.newBuilder()
                    .setCode(io.github.will413028.hawkr.common.utils.code.GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("order_no 為必填")
                    .build());
            observer.onCompleted();
            return;
        }

        int activeCount = caseMapper.countActiveCasesByOrder(orderNo);
        observer.onNext(io.github.will413028.hawkr.aftersales.grpc.HasActiveCaseResponse.newBuilder()
                .setCode(0)
                .setMessage("success")
                .setHasActive(activeCount > 0)
                .setActiveCount(activeCount)
                .build());
        observer.onCompleted();
    }

    /**
     * 確認收貨同步處置進行中售後 case（order-service confirmReceipt 完成訂單前同步呼叫，09 §2.1.1 / §8.2.1，#1225）。
     * <p>internal RPC，免外部權限（由 order-service 已驗證訂單歸屬）。</p>
     */
    @Override
    public void cancelActiveCaseOnConfirmReceipt(
            io.github.will413028.hawkr.aftersales.grpc.CancelActiveCaseOnConfirmReceiptRequest request,
            StreamObserver<io.github.will413028.hawkr.aftersales.grpc.CancelActiveCaseOnConfirmReceiptResponse> observer) {
        try {
            String orderNo = request.getOrderNo();
            if (orderNo == null || orderNo.isEmpty()) {
                observer.onNext(io.github.will413028.hawkr.aftersales.grpc.CancelActiveCaseOnConfirmReceiptResponse.newBuilder()
                        .setCode(io.github.will413028.hawkr.common.utils.code.GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("order_no 為必填")
                        .build());
                observer.onCompleted();
                return;
            }

            AfterSaleCaseService.ConfirmReceiptCancelResult result =
                    caseService.cancelActiveCaseOnConfirmReceipt(request.getBuyerId(), orderNo);

            io.github.will413028.hawkr.aftersales.grpc.CancelActiveCaseOnConfirmReceiptResponse.Result protoResult =
                    switch (result.outcome()) {
                        case NO_ACTIVE_CASE -> io.github.will413028.hawkr.aftersales.grpc.CancelActiveCaseOnConfirmReceiptResponse.Result.NO_ACTIVE_CASE;
                        case TERMINATED -> io.github.will413028.hawkr.aftersales.grpc.CancelActiveCaseOnConfirmReceiptResponse.Result.TERMINATED;
                        case BLOCKED -> io.github.will413028.hawkr.aftersales.grpc.CancelActiveCaseOnConfirmReceiptResponse.Result.BLOCKED;
                    };

            io.github.will413028.hawkr.aftersales.grpc.CancelActiveCaseOnConfirmReceiptResponse.Builder builder =
                    io.github.will413028.hawkr.aftersales.grpc.CancelActiveCaseOnConfirmReceiptResponse.newBuilder()
                            .setCode(GrpcResponseCode.SUCCESS.code())
                            .setMessage(GrpcResponseCode.SUCCESS.message())
                            .setResult(protoResult);
            if (result.caseNo() != null) {
                builder.setCaseNo(result.caseNo());
            }
            if (result.blockingStatus() != null) {
                builder.setBlockingStatus(result.blockingStatus());
            }
            observer.onNext(builder.build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    io.github.will413028.hawkr.aftersales.grpc.CancelActiveCaseOnConfirmReceiptResponse.newBuilder(),
                    e, CLASS_NAME, "cancelActiveCaseOnConfirmReceipt");
        }
    }

    /**
     * 賣家工作台 Dashboard 售後分類（09 §4.4，order-service 內部反查）。
     * <p>
     * 撈該賣家所有 case 的 (orderNo, status, appealState)，以 {@link AfterSaleCaseStatusRules}
     * canonical 在記憶體分類：active（→ aftersales queue + 從訂單三桶排除）、
     * terminal-override（case status∈{33,50} 無申訴且無 active → 歸已完成）。active 優先。
     */
    @Override
    public void getSellerCaseClassification(io.github.will413028.hawkr.aftersales.grpc.GetSellerCaseClassificationRequest request,
                                            StreamObserver<io.github.will413028.hawkr.aftersales.grpc.GetSellerCaseClassificationResponse> observer) {
        try {
            long storeId = request.getStoreId();
            List<SellerCaseRef> refs = caseMapper.selectStoreCaseRefs(storeId);

            long activeCaseCount = 0;
            Set<String> activeOrderNos = new LinkedHashSet<>();
            Set<String> overrideOrderNos = new LinkedHashSet<>();
            for (SellerCaseRef ref : refs) {
                if (!AfterSaleCaseStatus.isValidValue(ref.status())) {
                    // 髒資料 fail-loud：拋出讓上層記錄，不靜默漏算（docs/開發規範/Java 微服務開發規範.md §13.4）
                    throw new IllegalArgumentException("Unknown aftersale case status: " + ref.status()
                            + ", orderNo=" + ref.orderNo());
                }
                AfterSaleCaseStatus status = AfterSaleCaseStatus.fromValue(ref.status());
                if (AfterSaleCaseStatusRules.isActive(status, ref.appealState())) {
                    activeCaseCount++;
                    activeOrderNos.add(ref.orderNo());
                } else if (AfterSaleCaseStatusRules.isTerminalOverrideToCompleted(status, ref.appealState())) {
                    overrideOrderNos.add(ref.orderNo());
                }
            }
            // active 優先：同訂單若同時有 active 與 terminal-override case，歸 active
            overrideOrderNos.removeAll(activeOrderNos);

            observer.onNext(io.github.will413028.hawkr.aftersales.grpc.GetSellerCaseClassificationResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setActiveCaseCount(String.valueOf(activeCaseCount))
                    .addAllActiveOrderNos(activeOrderNos)
                    .addAllCompletedOverrideOrderNos(overrideOrderNos)
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    io.github.will413028.hawkr.aftersales.grpc.GetSellerCaseClassificationResponse.newBuilder(),
                    e, CLASS_NAME, "getSellerCaseClassification");
        }
    }
}
