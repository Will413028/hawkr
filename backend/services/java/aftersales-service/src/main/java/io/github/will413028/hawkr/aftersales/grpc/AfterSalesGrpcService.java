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
    public AfterSalesGrpcService(AfterSaleCaseService caseService,
                                  AfterSaleResolutionService resolutionService,
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
                                  io.github.will413028.hawkr.aftersales.grpc.client.OrderGrpcClient orderGrpcClient) {
        this.caseService = caseService;
        this.resolutionService = resolutionService;
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
    }


    @Override
    public void createAfterSaleCase(CreateAfterSaleCaseRequest request,
                                     StreamObserver<CreateAfterSaleCaseResponse> observer) {
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

            List<Long> buyerIds = result.getRecords().stream()
                    .map(AfterSaleCaseEntity::getBuyerId)
                    .filter(java.util.Objects::nonNull)
                    .distinct()
                    .toList();
            java.util.Map<Long, io.github.will413028.hawkr.buyer.grpc.BuyerDisplay> buyerDisplayMap = buyerIds.isEmpty()
                    ? java.util.Collections.emptyMap()
                    : buyerGrpcClient.batchGetBuyerDisplayByBuyerIds(buyerIds);

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

            AfterSaleCaseEntity updatedCase = caseService.getCase(request.getCaseNo());

            observer.onNext(ResolveAfterSaleCaseResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(ResolveResultData.newBuilder()
                            .setCaseNo(updatedCase.getCaseNo())
                            .setStatus(updatedCase.getStatus())
                            .setReturnShipmentNo(nullToEmpty(updatedCase.getReturnShipmentNo()))
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

            Long actualRefundAmount = null;
            String actualRaw = request.getActualRefundAmount();
            if (actualRaw != null && !actualRaw.isBlank()) {
                actualRefundAmount = ProtoConversionUtils.parseLongRequired(actualRaw, "actualRefundAmount");
            }

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

            SellerCancelRequestEntity cancelEntity = cancelMapper.selectOne(
                    new LambdaQueryWrapper<SellerCancelRequestEntity>()
                            .eq(SellerCancelRequestEntity::getOrderNo, request.getOrderNo())
                            .orderByDesc(SellerCancelRequestEntity::getCreatedAt)
                            .last("LIMIT 1")
            );

            Long refundAmount = cancelEntity != null ? cancelEntity.getRefundAmount() : 0L;

            observer.onNext(CancelShipmentResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(CancelShipmentResultData.newBuilder()
                            .setOrderNo(request.getOrderNo())
                            .setRefundAmount(ProtoConversionUtils.formatLong(refundAmount))
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
                .setBuyerName(nullToEmpty(entity.getBuyerName()))
                .setBuyerAvatarKey(nullToEmpty(buyerAvatarKey))
                .setShippingAddress(composeShippingAddress(entity))
                .setReturnShippingMethod(nullToEmpty(entity.getReturnShippingMethod()))
                .setTemperatureCode(nullToEmpty(entity.getDeliveryTemp()))
                .setAppealState(nullToEmpty(entity.getAppealState()))
                .setRefundTaskStatus(snapshot.status())
                .setRefundFailureMessage(snapshot.failureMessage())
                .setProviderStatus(providerPair.status)
                .setProviderMessage(providerPair.message)
                .setAutoConfirmAt(entity.getAutoConfirmAt() != null
                        ? ProtoConversionUtils.formatIso8601(entity.getAutoConfirmAt())
                        : "")
                .setStorePickupDeadlineAt(entity.getStorePickupDeadlineAt() != null
                        ? ProtoConversionUtils.formatIso8601(entity.getStorePickupDeadlineAt())
                        : "")
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

        if (!refundCaseNos.isEmpty()) {
            List<RefundTaskEntity> tasks = refundTaskMapper.selectList(
                    new LambdaQueryWrapper<RefundTaskEntity>()
                            .in(RefundTaskEntity::getCaseNo, refundCaseNos));
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
                        nullToEmpty(e.getValue().getProviderStatus()),
                        nullToEmpty(e.getValue().getProviderMessage())));
            }
        }

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

        List<AfterSaleCaseItemEntity> items = caseItemMapper.selectList(
                new LambdaQueryWrapper<AfterSaleCaseItemEntity>()
                        .eq(AfterSaleCaseItemEntity::getCaseNo, caseNo)
        );

        List<AfterSaleEvidenceEntity> evidenceList = evidenceMapper.selectList(
                new LambdaQueryWrapper<AfterSaleEvidenceEntity>()
                        .eq(AfterSaleEvidenceEntity::getCaseNo, caseNo)
                        .orderByAsc(AfterSaleEvidenceEntity::getCreatedAt)
        );

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
                .setCreatedAt(ProtoConversionUtils.formatIso8601(caseEntity.getCreatedAt()));

        if (caseEntity.getClosedAt() != null) {
            detailBuilder.setClosedAt(ProtoConversionUtils.formatIso8601(caseEntity.getClosedAt()));
        }

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

        for (AfterSaleCaseItemEntity item : items) {
            String imageKey = item.getImageObjectKey() != null ? item.getImageObjectKey() : "";
            detailBuilder.addItems(AfterSaleCaseItemView.newBuilder()
                    .setOrderItemId(ProtoConversionUtils.formatLong(item.getOrderItemId()))
                    .setSkuId(ProtoConversionUtils.formatUuid(item.getSkuId()))
                    .setSkuName(nullToEmpty(item.getSkuName()))
                    .setQuantity(ProtoConversionUtils.formatInt(item.getQuantity()))
                    .setOriginalPrice(ProtoConversionUtils.formatLong(item.getOriginalPrice()))
                    .setActualPayAmount(ProtoConversionUtils.formatLong(item.getActualPayAmount()))
                    .setImageKey(imageKey)
                    .build());
        }

        for (AfterSaleEvidenceEntity evidence : evidenceList) {
            String imageKey = evidence.getObjectKey() != null ? evidence.getObjectKey() : "";
            detailBuilder.addEvidence(EvidenceView.newBuilder()
                    .setAssetId(ProtoConversionUtils.formatLong(evidence.getAssetId()))
                    .setImageKey(imageKey)
                    .setEvidenceType(nullToEmpty(evidence.getEvidenceType()))
                    .setCreatedAt(ProtoConversionUtils.formatIso8601(evidence.getCreatedAt()))
                    .build());
        }

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

        detailBuilder.setAppealState(nullToEmpty(caseEntity.getAppealState()));
        java.time.LocalDateTime deadline = appealService.computeAppealDeadline(caseEntity);
        if (deadline != null
                && AfterSaleCaseStatus.REJECTED.value().equals(caseEntity.getStatus())
                && AfterSaleAppealState.NONE.value().equals(nullToEmpty(caseEntity.getAppealState()))) {
            detailBuilder.setAppealDeadline(ProtoConversionUtils.formatIso8601(deadline));
        }
        detailBuilder.setCanAppeal(appealService.canAppeal(caseEntity));

        RefundTaskSnapshot snapshot = loadRefundTaskSnapshot(caseEntity.getCaseNo());
        detailBuilder.setRefundTaskStatus(snapshot.status());
        detailBuilder.setRefundFailureMessage(snapshot.failureMessage());

        ProviderStatusPair providerPair = resolveCaseProviderStatus(caseEntity, snapshot);
        detailBuilder.setProviderStatus(providerPair.status);
        detailBuilder.setProviderMessage(providerPair.message);

        if (caseEntity.getProcessDeadline() != null) {
            detailBuilder.setProcessDeadline(
                    ProtoConversionUtils.formatIso8601(caseEntity.getProcessDeadline()));
        }
        detailBuilder.setAutoEscalated(Boolean.TRUE.equals(caseEntity.getAutoEscalated()));
        detailBuilder.setProcessDeadlineHours(
                afterSalesProperties.getCaseConfig().getProcessDeadlineHours());

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
        if (caseEntity.getShipBackDeadlineAt() != null) {
            detailBuilder.setShipBackDeadlineAt(
                    ProtoConversionUtils.formatIso8601(caseEntity.getShipBackDeadlineAt()));
        }

        return detailBuilder.build();
    }

    /** case 聚合 provider 結果，空字串代表無值（收斂規範 §6.2 部分失敗容忍）。 */
    private record ProviderStatusPair(String status, String message) {
        static final ProviderStatusPair EMPTY = new ProviderStatusPair("", "");
    }

    /**
     * 依 case 當下 status 決定 providerStatus / providerMessage 取源：
     * - 退款相關（REFUND_PROCESSING）→ refund_tasks.provider_*（本地 snapshot）
     * - 退貨物流相關（RETURN_SHIPPING）→ 呼 Shipping.BatchGetProviderStatusByOrderIds(isReturn=true)（單筆）
     * - 其他狀態 → 空字串
     */
    private ProviderStatusPair resolveCaseProviderStatus(AfterSaleCaseEntity caseEntity,
                                                          RefundTaskSnapshot snapshot) {
        String status = caseEntity.getStatus();
        if (AfterSaleCaseStatus.REFUND_PROCESSING.value().equals(status)) {
            RefundTaskEntity task = refundTaskMapper.selectOne(
                    new LambdaQueryWrapper<RefundTaskEntity>()
                            .eq(RefundTaskEntity::getCaseNo, caseEntity.getCaseNo())
                            .orderByDesc(RefundTaskEntity::getCreatedAt)
                            .last("LIMIT 1"));
            if (task == null) return ProviderStatusPair.EMPTY;
            return new ProviderStatusPair(nullToEmpty(task.getProviderStatus()),
                    nullToEmpty(task.getProviderMessage()));
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
    private record RefundTaskSnapshot(String status, String failureMessage) {
        static final RefundTaskSnapshot EMPTY = new RefundTaskSnapshot("", "");
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
        String failureMessage = WorkerTaskStatus.FAILED.value().equals(task.getStatus())
                ? nullToEmpty(task.getProviderMessage())
                : "";
        return new RefundTaskSnapshot(nullToEmpty(task.getStatus()), failureMessage);
    }

    private static String nullToEmpty(String value) {
        return value != null ? value : "";
    }


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
                        .setBuyerAvatarKey(buyer != null ? nullToEmpty(buyer.getAvatarKey()) : "")
                        .setSellerName(seller != null ? nullToEmpty(seller.getName()) : "")
                        .setSellerAvatarKey(seller != null ? nullToEmpty(seller.getLogoKey()) : "")
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


    @Override
    public void getAppealDetailForAdmin(GetAppealDetailForAdminRequest request,
                                         StreamObserver<GetAppealDetailForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            String caseNo = request.getCaseNo();

            var result = appealService.getAppealDetailForAdmin(caseNo);
            AfterSaleCaseEntity caseEntity = result.getCaseEntity();

            AfterSaleCaseDetailView caseDetail = buildDetailView(caseEntity);

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
                    .setBuyerAvatarKey(buyerDisplay != null ? nullToEmpty(buyerDisplay.getAvatarKey()) : "")
                    .setSellerName(sellerInfo != null ? nullToEmpty(sellerInfo.getName()) : "")
                    .setSellerAvatarKey(sellerInfo != null ? nullToEmpty(sellerInfo.getLogoKey()) : "");

            if (result.getLatestRejectResolution() != null) {
                var rej = result.getLatestRejectResolution();
                adminBuilder.setSellerRejectReason(nullToEmpty(rej.getRejectReason()))
                        .setSellerRejectMessage(nullToEmpty(rej.getRejectMessage()));
            }
            if (result.getDecision() != null) {
                adminBuilder.setDecidedByAdminId(
                        ProtoConversionUtils.formatLong(result.getDecision().getDecidedByAdminId()));
            }

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


    @Override
    public void listAfterSaleCasesForAdmin(ListAfterSaleCasesForAdminRequest request,
                                            StreamObserver<ListAfterSaleCasesForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            int pageNum = ProtoConversionUtils.parseIntOrDefault(request.getPage(), 1);
            int pageSize = Math.min(ProtoConversionUtils.parseIntOrDefault(request.getPageSize(), 20), 100);

            var wrapper = new com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper<AfterSaleCaseEntity>();

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
