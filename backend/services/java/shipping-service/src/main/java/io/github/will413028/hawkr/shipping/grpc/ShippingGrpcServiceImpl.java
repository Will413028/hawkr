package io.github.will413028.hawkr.shipping.grpc;

import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.obs.grpc.GrpcErrorHandler;
import io.github.will413028.hawkr.common.grpc.GrpcServiceHelper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AccountType;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.shipping.config.ShippingBatchProperties;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.shipping.entity.BuyerStorePickupEntity;
import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.shipping.provider.capability.StoreLocatorCapability;
import io.github.will413028.hawkr.common.utils.code.ShippingTemperature;
import io.github.will413028.hawkr.shipping.entity.ShippingFeeTierEntity;
import io.github.will413028.hawkr.shipping.entity.ShippingMethodConfigEntity;
import io.github.will413028.hawkr.shipping.entity.projection.HolidayYearCoverageRef;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.ShipmentAnomalyType;
import io.github.will413028.hawkr.common.utils.code.helper.ShipmentAnomalyRules;
import io.github.will413028.hawkr.shipping.dto.ShipmentAnomalyRow;
import io.github.will413028.hawkr.shipping.entity.ShipmentAnomalyAcknowledgmentEntity;
import io.github.will413028.hawkr.shipping.service.BatchHomeLabelService;
import io.github.will413028.hawkr.shipping.service.BuyerStorePickupService;
import io.github.will413028.hawkr.shipping.service.HolidayService;
import io.github.will413028.hawkr.shipping.service.OnboardingService;
import io.github.will413028.hawkr.shipping.service.ShipmentAnomalyService;
import io.github.will413028.hawkr.shipping.service.ShipmentLabelService;
import io.github.will413028.hawkr.shipping.service.ShippingAdminQueryService;
import io.github.will413028.hawkr.shipping.service.ShippingFeeService;
import io.github.will413028.hawkr.shipping.provider.uber.UberPackingPlanner;
import io.github.will413028.hawkr.shipping.service.uber.UberDeliveryQueryService;
import io.github.will413028.hawkr.shipping.service.uber.UberQuoteService;
import io.github.will413028.hawkr.shipping.entity.UberQuoteEntity;
import io.github.will413028.hawkr.shipping.service.ShippingService;
import io.github.will413028.hawkr.common.utils.code.ShippingType;
import io.github.will413028.hawkr.shipping.service.StoreMapRedirectFormStore;
import io.github.will413028.hawkr.shipping.service.StoreMapService;
import io.github.will413028.hawkr.shipping.service.dto.SellerShippingMethodSlot;
import io.grpc.stub.StreamObserver;
import net.devh.boot.grpc.server.service.GrpcService;
import org.slf4j.MDC;

import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;

@GrpcService
public class ShippingGrpcServiceImpl extends ShippingServiceGrpc.ShippingServiceImplBase {

    private static final ObsLogger log = ObsLogger.forClass(ShippingGrpcServiceImpl.class);
    private static final DateTimeFormatter ISO_FMT = DateTimeFormatter.ISO_LOCAL_DATE_TIME;

    private final ShippingService shippingService;
    private final ShipmentLabelService labelService;
    private final OnboardingService onboardingService;
    private final StoreMapService storeMapService;
    private final ShippingFeeService shippingFeeService;
    private final BuyerStorePickupService buyerStorePickupService;
    private final HolidayService holidayService;
    private final ShippingAdminQueryService adminQueryService;
    private final BatchHomeLabelService batchHomeLabelService;
    private final ShippingBatchProperties batchProperties;
    private final ShipmentAnomalyService shipmentAnomalyService;

    /**
     * provider 能力的唯一問答點。門市地圖與 {@code allowedActions} 都經由它取得 capability，
     * 本類別不得再以配送方式字面值分支。
     */
    private final io.github.will413028.hawkr.shipping.provider.FulfillmentProviderRegistry fulfillmentProviders;

    /** 領域動作可用性的計算（65 兩層模型的下層）。 */
    private final io.github.will413028.hawkr.shipping.provider.action.AllowedActionResolver allowedActionResolver;
    private final UberQuoteService uberQuoteService;

    /**
     * Uber 配送查詢（66；W6-3 賣家面／W6-4 Admin 面）。
     *
     * <p>它補的是「三張表有資料、{@code grpc/} 底下零引用」那個缺口——資料在、零出口。
     */
    private final UberDeliveryQueryService uberDeliveryQueryService;

    public ShippingGrpcServiceImpl(ShippingService shippingService,
                                    ShipmentLabelService labelService,
                                    OnboardingService onboardingService,
                                    StoreMapService storeMapService,
                                    ShippingFeeService shippingFeeService,
                                    BuyerStorePickupService buyerStorePickupService,
                                    HolidayService holidayService,
                                    ShippingAdminQueryService adminQueryService,
                                    BatchHomeLabelService batchHomeLabelService,
                                    ShippingBatchProperties batchProperties,
                                    ShipmentAnomalyService shipmentAnomalyService,
                                    io.github.will413028.hawkr.shipping.provider.FulfillmentProviderRegistry fulfillmentProviders,
                                    io.github.will413028.hawkr.shipping.provider.action.AllowedActionResolver allowedActionResolver,
                                    UberQuoteService uberQuoteService,
                                    UberDeliveryQueryService uberDeliveryQueryService) {
        this.shippingService = shippingService;
        this.labelService = labelService;
        this.onboardingService = onboardingService;
        this.storeMapService = storeMapService;
        this.shippingFeeService = shippingFeeService;
        this.buyerStorePickupService = buyerStorePickupService;
        this.holidayService = holidayService;
        this.adminQueryService = adminQueryService;
        this.batchHomeLabelService = batchHomeLabelService;
        this.batchProperties = batchProperties;
        this.shipmentAnomalyService = shipmentAnomalyService;
        this.fulfillmentProviders = fulfillmentProviders;
        this.allowedActionResolver = allowedActionResolver;
        this.uberQuoteService = uberQuoteService;
        this.uberDeliveryQueryService = uberDeliveryQueryService;
    }

    // ==================== 賣家端（安排出貨）====================

    @Override
    public void printCvsLabel(PrintCvsLabelRequest request,
                               StreamObserver<PrintCvsLabelResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            long storeId = request.getStoreId();
            String orderNo = request.getOrderNo();
            String shipDate = request.getShipDate();

            ShippingService.CvsLabelResult result =
                    shippingService.printCvsLabel(storeId, orderNo, shipDate);

            CvsLabelData data = CvsLabelData.newBuilder()
                    .setShipmentNo(result.shipmentNo())
                    .setPartnerId(result.partnerId() != null ? result.partnerId() : "")
                    .setOdno(result.odno() != null ? result.odno() : "")
                    .setValidationNo(result.validationNo() != null ? result.validationNo() : "")
                    .setWebhookReceived(result.webhookReceived())
                    .build();

            String message = result.webhookReceived()
                    ? "success"
                    : "系統繁忙中，交貨便代碼即將生成";

            responseObserver.onNext(PrintCvsLabelResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(message)
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(PrintCvsLabelResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, PrintCvsLabelResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "printCvsLabel");
        }
    }

    @Override
    public void confirmCvsShipment(ConfirmCvsShipmentRequest request,
                                     StreamObserver<ConfirmCvsShipmentResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            long storeId = request.getStoreId();
            String orderNo = request.getOrderNo();
            java.util.List<String> stagedTokens = request.getStagedTokensList();

            shippingService.confirmCvsShipment(storeId, orderNo, stagedTokens);

            responseObserver.onNext(ConfirmCvsShipmentResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, ConfirmCvsShipmentResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "confirmCvsShipment");
        }
    }

    @Override
    public void batchPrintCvsLabel(BatchPrintCvsLabelRequest request,
                                     StreamObserver<BatchPrintCvsLabelResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            long storeId = request.getStoreId();
            List<String> orderNos = request.getOrderNosList();
            String shipDate = request.getShipDate();

            if (orderNos.isEmpty()) {
                responseObserver.onNext(BatchPrintCvsLabelResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("orderNos 不可為空")
                        .build());
                responseObserver.onCompleted();
                return;
            }
            if (orderNos.size() > batchProperties.getMaxSize()) {
                responseObserver.onNext(BatchPrintCvsLabelResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("單次最多 " + batchProperties.getMaxSize() + " 筆")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            ShippingService.BatchPrintResult result =
                    shippingService.batchPrintCvsLabel(storeId, orderNos, shipDate);

            BatchCvsLabelData.Builder dataBuilder = BatchCvsLabelData.newBuilder()
                    .setTotalCount(String.valueOf(result.totalCount()))
                    .setSuccessCount(String.valueOf(result.successCount()));

            for (ShippingService.BatchPrintItemResult item : result.results()) {
                dataBuilder.addResults(BatchCvsLabelResult.newBuilder()
                        .setOrderNo(item.orderNo())
                        .setShipmentNo(item.shipmentNo() != null ? item.shipmentNo() : "")
                        .setSuccess(item.success())
                        .setMessage(item.message() != null ? item.message() : "")
                        .build());
            }

            int grpcCode = (result.successCount() > 0)
                    ? GrpcResponseCode.SUCCESS.code()
                    : GrpcResponseCode.INVALID_PARAM.code();
            String grpcMsg = (result.successCount() == result.totalCount())
                    ? "success"
                    : "批次列印完成，" + result.successCount() + "/" + result.totalCount() + " 筆成功";

            responseObserver.onNext(BatchPrintCvsLabelResponse.newBuilder()
                    .setCode(grpcCode)
                    .setMessage(grpcMsg)
                    .setData(dataBuilder.build())
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(BatchPrintCvsLabelResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, BatchPrintCvsLabelResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "batchPrintCvsLabel");
        }
    }

    @Override
    public void generateHomeLabelPdf(GenerateHomeLabelPdfRequest request,
                                      StreamObserver<GenerateHomeLabelPdfResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            long storeId = request.getStoreId();
            String orderNo = request.getOrderNo();
            String shipDate = request.getShipDate();
            String deliveryDate = request.getDeliveryDate();
            java.util.List<String> stagedTokens = request.getStagedTokensList();

            String labelKey = shippingService.generateHomeLabelPdf(
                    storeId, orderNo, shipDate, deliveryDate, stagedTokens);

            // 查詢 shipmentNo
            ShipmentEntity entity = shippingService.getShipmentByOrderNo(orderNo);
            String shipmentNo = (entity != null) ? entity.getShipmentNo() : "";

            HomeLabelData data = HomeLabelData.newBuilder()
                    .setLabelKey(labelKey) // 49 §6.6：只填 objectKey，Gateway 組 labelUrl
                    .setShipmentNo(shipmentNo)
                    .build();

            responseObserver.onNext(GenerateHomeLabelPdfResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(GenerateHomeLabelPdfResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GenerateHomeLabelPdfResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "generateHomeLabelPdf");
        }
    }

    @Override
    public void batchGenerateHomeLabelPdf(BatchGenerateHomeLabelPdfRequest request,
                                          StreamObserver<BatchGenerateHomeLabelPdfResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            long storeId = request.getStoreId();
            List<BatchHomeLabelItem> protoItems = request.getItemsList();

            if (protoItems.isEmpty()) {
                responseObserver.onNext(BatchGenerateHomeLabelPdfResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("items 不可為空")
                        .build());
                responseObserver.onCompleted();
                return;
            }
            if (protoItems.size() > batchProperties.getMaxSize()) {
                responseObserver.onNext(BatchGenerateHomeLabelPdfResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("單次最多 " + batchProperties.getMaxSize() + " 筆")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            List<BatchHomeLabelService.Item> items = new java.util.ArrayList<>(protoItems.size());
            for (BatchHomeLabelItem pi : protoItems) {
                items.add(new BatchHomeLabelService.Item(
                        pi.getOrderNo(), pi.getShipDate(), pi.getDeliveryDate()));
            }

            BatchHomeLabelService.BatchResult result = batchHomeLabelService.batchGenerate(storeId, items);

            BatchHomeLabelData.Builder dataBuilder = BatchHomeLabelData.newBuilder()
                    .setTotalCount(String.valueOf(result.totalCount()))
                    .setSuccessCount(String.valueOf(result.successCount()));

            for (BatchHomeLabelService.ItemResult ir : result.results()) {
                dataBuilder.addResults(BatchHomeLabelResult.newBuilder()
                        .setOrderNo(ir.orderNo())
                        .setShipmentNo(ir.shipmentNo() != null ? ir.shipmentNo() : "")
                        .setLabelKey(ir.labelKey() != null ? ir.labelKey() : "") // 49 §6.6：只填 objectKey
                        .setSuccess(ir.success())
                        .setMessage(ir.message() != null ? ir.message() : "")
                        .build());
            }

            int grpcCode = (result.successCount() > 0)
                    ? GrpcResponseCode.SUCCESS.code()
                    : GrpcResponseCode.INVALID_PARAM.code();
            String grpcMsg = (result.successCount() == result.totalCount())
                    ? "success"
                    : "批次宅配取號完成，" + result.successCount() + "/" + result.totalCount() + " 筆成功";

            responseObserver.onNext(BatchGenerateHomeLabelPdfResponse.newBuilder()
                    .setCode(grpcCode)
                    .setMessage(grpcMsg)
                    .setData(dataBuilder.build())
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(BatchGenerateHomeLabelPdfResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, BatchGenerateHomeLabelPdfResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "batchGenerateHomeLabelPdf");
        }
    }

    @Override
    public void callTcat(CallTcatRequest request,
                          StreamObserver<CallTcatResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            long storeId = request.getStoreId();
            String orderNo = request.getOrderNo();

            String callTime = shippingService.callTcat(
                    storeId, orderNo,
                    request.getIsContact(), request.getIsTrolley(), request.getMemo());

            // 查詢 shipmentNo
            ShipmentEntity entity = shippingService.getShipmentByOrderNo(orderNo);
            String shipmentNo = (entity != null) ? entity.getShipmentNo() : "";

            CallTcatData data = CallTcatData.newBuilder()
                    .setCallTime(callTime)
                    .setShipmentNo(shipmentNo)
                    .build();

            responseObserver.onNext(CallTcatResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(CallTcatResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CallTcatResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "callTcat");
        }
    }

    // ==================== 服務間呼叫 ====================

    @Override
    public void getShipmentsByOrderNos(GetShipmentsByOrderNosRequest request,
                                        StreamObserver<GetShipmentsByOrderNosResponse> responseObserver) {
        try {
            java.util.List<ShipmentEntity> entities =
                    shippingService.getShipmentsByOrderNos(request.getOrderNosList());
            GetShipmentsByOrderNosResponse.Builder respBuilder = GetShipmentsByOrderNosResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message());
            for (ShipmentEntity e : entities) {
                respBuilder.addData(buildDetailView(e));
            }
            responseObserver.onNext(respBuilder.build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    GetShipmentsByOrderNosResponse.newBuilder(), e,
                    "ShippingGrpcServiceImpl", "getShipmentsByOrderNos");
        }
    }

    @Override
    public void getShipmentByOrderNo(GetShipmentByOrderNoRequest request,
                                      StreamObserver<GetShipmentResponse> responseObserver) {
        // #1871 OBS-09：order_no 進 MDC，讓這個方法內所有 log 行都能被 Loki structured
        // metadata 查到（見 k8s/monitoring/promtail-daemonset.yaml）。
        MDC.put("order_no", request.getOrderNo());
        try {
            ShipmentEntity entity = shippingService.getShipmentByOrderNo(request.getOrderNo());
            if (entity == null) {
                responseObserver.onNext(GetShipmentResponse.newBuilder()
                        .setCode(GrpcResponseCode.NOT_FOUND.code())
                        .setMessage("物流單不存在")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            responseObserver.onNext(GetShipmentResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(buildDetailView(entity))
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(GetShipmentResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetShipmentResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "getShipmentByOrderNo");
        } finally {
            MDC.remove("order_no");
        }
    }

    @Override
    public void queryShipmentStatus(QueryShipmentStatusRequest request,
                                     StreamObserver<GetShipmentResponse> responseObserver) {
        try {
            ShipmentEntity entity = shippingService.queryAndSyncStatus(request.getShipmentNo());

            responseObserver.onNext(GetShipmentResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(buildDetailView(entity))
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(GetShipmentResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetShipmentResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "queryShipmentStatus");
        }
    }

    @Override
    public void getShipmentTracking(GetShipmentTrackingRequest request,
                                    StreamObserver<GetShipmentResponse> responseObserver) {
        try {
            ShipmentEntity entity = shippingService.getBuyerTrackingByOrderNo(
                    request.getOrderNo(), request.getBuyerId());

            responseObserver.onNext(GetShipmentResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(buildDetailView(entity))
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(GetShipmentResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetShipmentResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "getShipmentTracking");
        }
    }

    @Override
    public void getSellerShipmentTracking(GetSellerShipmentTrackingRequest request,
                                          StreamObserver<GetShipmentResponse> responseObserver) {
        try {
            ShipmentEntity entity = shippingService.getSellerTrackingByOrderNo(
                    request.getOrderNo(), request.getStoreId(), request.getIsReturn());

            responseObserver.onNext(GetShipmentResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(buildDetailView(entity))
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(GetShipmentResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetShipmentResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "getSellerShipmentTracking");
        }
    }

    // ==================== 建店開通 ====================

    @Override
    public void onboardCvsShipping(OnboardCvsShippingRequest request,
                                    StreamObserver<OnboardShippingResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            long storeId = request.getStoreId();
            SellerShipmentSettingEntity result = onboardingService.onboardCvsShipping(
                    storeId, request.getGoodsType(), request.getReturnStoreCode(),
                    request.getReturnStoreName(), request.getReturnStoreAddress(),
                    request.getSenderName(), request.getSenderPhone());

            responseObserver.onNext(OnboardShippingResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(buildOnboardingView(result))
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(OnboardShippingResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, OnboardShippingResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "onboardCvsShipping");
        }
    }

    @Override
    public void onboardHomeDeliveryShipping(OnboardHomeDeliveryRequest request,
                                             StreamObserver<OnboardShippingResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            long storeId = request.getStoreId();
            SellerShipmentSettingEntity result = onboardingService.onboardHomeDelivery(
                    storeId, request.getProductCategory(),
                    request.getSenderName(), request.getSenderPhone(),
                    request.getSenderCity(), request.getSenderDistrict(),
                    request.getSenderAddressDetail(), request.getSenderPostalCode(),
                    request.getReturnName(), request.getReturnPhone(),
                    request.getReturnCity(), request.getReturnDistrict(),
                    request.getReturnAddressDetail(), request.getReturnPostalCode());

            responseObserver.onNext(OnboardShippingResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(buildOnboardingView(result))
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(OnboardShippingResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, OnboardShippingResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "onboardHomeDeliveryShipping");
        }
    }

    @Override
    public void listShippingOnboardings(ListShippingOnboardingsRequest request,
                                         StreamObserver<ListShippingOnboardingsResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            long storeId = request.getStoreId();
            List<SellerShipmentSettingEntity> settings =
                    onboardingService.listOnboardings(storeId);

            ListShippingOnboardingsResponse.Builder respBuilder =
                    ListShippingOnboardingsResponse.newBuilder()
                            .setCode(GrpcResponseCode.SUCCESS.code())
                            .setMessage(GrpcResponseCode.SUCCESS.message());

            for (SellerShipmentSettingEntity s : settings) {
                respBuilder.addOnboardings(buildOnboardingView(s));
            }

            responseObserver.onNext(respBuilder.build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(ListShippingOnboardingsResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, ListShippingOnboardingsResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "listShippingOnboardings");
        }
    }

    @Override
    public void getShippingOnboarding(GetShippingOnboardingRequest request,
                                       StreamObserver<GetShippingOnboardingResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            long storeId = request.getStoreId();
            long onboardingId = ProtoConversionUtils.parseLongRequired(request.getOnboardingId(), "onboardingId");
            SellerShipmentSettingEntity result =
                    onboardingService.getOnboarding(storeId, onboardingId);

            responseObserver.onNext(GetShippingOnboardingResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(buildOnboardingView(result))
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(GetShippingOnboardingResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetShippingOnboardingResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "getShippingOnboarding");
        }
    }

    // ==================== 啟用/停用切換 ====================

    @Override
    public void toggleShippingOnboarding(ToggleShippingOnboardingRequest request,
                                          StreamObserver<OnboardShippingResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            long storeId = request.getStoreId();
            long onboardingId = ProtoConversionUtils.parseLongRequired(request.getOnboardingId(), "onboardingId");
            boolean enabled = request.getEnabled();

            onboardingService.toggleEnabled(storeId, onboardingId, enabled);

            responseObserver.onNext(OnboardShippingResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(OnboardShippingResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, OnboardShippingResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "toggleShippingOnboarding");
        }
    }

    // ==================== 賣家配送方式查詢 ====================

    @Override
    public void listSellerShippingMethods(ListSellerShippingMethodsRequest request,
                                           StreamObserver<ListSellerShippingMethodsResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            long storeId = request.getStoreId();
            List<SellerShippingMethodSlot> slots =
                    onboardingService.listSellerShippingMethods(storeId);

            ListSellerShippingMethodsResponse.Builder respBuilder =
                    ListSellerShippingMethodsResponse.newBuilder()
                            .setCode(GrpcResponseCode.SUCCESS.code())
                            .setMessage(GrpcResponseCode.SUCCESS.message());

            for (SellerShippingMethodSlot slot : slots) {
                respBuilder.addMethods(SellerShippingMethodView.newBuilder()
                        .setShippingMethod(slot.shippingMethod())
                        .setGoodsType(slot.goodsType())
                        .setDisplayName(slot.displayName())
                        .setDescription(slot.description())
                        .setAvailable(slot.available())
                        .setOnboardingRequired(slot.onboardingRequired())
                        .setOnboardingStatus(slot.onboardingStatus())
                        .setOnboardingId(slot.onboardingId() != null ? String.valueOf(slot.onboardingId()) : "")
                        .setEnabled(slot.enabled())
                        .build());
            }

            responseObserver.onNext(respBuilder.build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(ListSellerShippingMethodsResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, ListSellerShippingMethodsResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "listSellerShippingMethods");
        }
    }

    // ==================== 超商門市地圖 ====================

    @Override
    public void getStoreMapParams(GetStoreMapParamsRequest request,
                                   StreamObserver<GetStoreMapParamsResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(
                    AccountType.BUYER, AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            long accountId = request.getAccountId();
            String appType = request.getAppType();
            String mobileTag = request.getMobileTag();

            // 🔴 門市地圖是 provider 能力，不是本層的知識：參數怎麼加密、表單打去哪、
            //    版本號是什麼，全部由 StoreLocatorCapability 決定，本層只做 key→proto 的搬運。
            Map<String, String> params = requireStoreLocator().buildRedirectParams(
                    new StoreLocatorCapability.StoreMapRequest(
                            accountId, appType, !"N".equalsIgnoreCase(mobileTag)));

            StoreMapFormData data = StoreMapFormData.newBuilder()
                    .setFormAction(params.get(StoreLocatorCapability.PARAM_FORM_ACTION))
                    .setMerId(params.get(StoreLocatorCapability.PARAM_MERCHANT_ID))
                    .setVersion(params.get(StoreLocatorCapability.PARAM_VERSION))
                    .setEncryptInfo(params.get(StoreLocatorCapability.PARAM_PAYLOAD))
                    .setHashInfo(params.get(StoreLocatorCapability.PARAM_SIGNATURE))
                    .setRedirectUrl(params.get(StoreLocatorCapability.PARAM_REDIRECT_URL))
                    .build();

            responseObserver.onNext(GetStoreMapParamsResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(GetStoreMapParamsResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetStoreMapParamsResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "getStoreMapParams");
        }
    }

    @Override
    public void decryptStoreMapResult(DecryptStoreMapResultRequest request,
                                       StreamObserver<DecryptStoreMapResultResponse> responseObserver) {
        String appType = request.getAppType();
        try {
            String encryptInfo = request.getEncryptInfo();
            String hashInfo = request.getHashInfo();

            if (encryptInfo.isEmpty() || hashInfo.isEmpty()) {
                String errorUrl = storeMapService.buildErrorDeepLink(appType, "缺少加密參數");
                responseObserver.onNext(DecryptStoreMapResultResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("encryptInfo 與 hashInfo 不可為空")
                        .setRedirectUrl(errorUrl)
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // fail-closed：解不出門市代碼時 capability 丟 IllegalArgumentException，
            // 由下方既有的 catch 組錯誤導回頁——遷移前那條路徑是帶著空門市代碼往下走的。
            StoreLocatorCapability.StoreSelection result = requireStoreLocator().parseResult(Map.of(
                    StoreLocatorCapability.RESULT_PAYLOAD, encryptInfo,
                    StoreLocatorCapability.RESULT_SIGNATURE, hashInfo,
                    StoreLocatorCapability.RESULT_CLIENT_KIND, appType == null ? "" : appType));

            StoreMapResult data = StoreMapResult.newBuilder()
                    .setStoreId(result.storeCode())
                    .setStoreName(result.storeName())
                    .setAddress(result.storeAddress())
                    .setInsularArea(result.insularArea())
                    .setStoreType(result.storeType())
                    .build();

            responseObserver.onNext(DecryptStoreMapResultResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .setRedirectUrl(result.redirectUrl())
                    .build());
            responseObserver.onCompleted();
        } catch (IllegalArgumentException e) {
            log.error("ShippingGrpc.decryptStoreMapResult.error",
                    "門市地圖解密驗證失敗: " + e.getMessage(), e);
            String errorUrl = storeMapService.buildErrorDeepLink(appType, e.getMessage());
            responseObserver.onNext(DecryptStoreMapResultResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage(GrpcServiceHelper.getErrorMessage(e))
                    .setRedirectUrl(errorUrl)
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(DecryptStoreMapResultResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, DecryptStoreMapResultResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "decryptStoreMapResult");
        }
    }

    @Override
    public void getStoreMapRedirectForm(GetStoreMapRedirectFormRequest request,
                                         StreamObserver<GetStoreMapRedirectFormResponse> responseObserver) {
        // 公開端點：門市中轉頁以 opaque id 一次性取回 canonical 表單（不可猜 + 短效 + 一次性；#1394）
        Optional<StoreMapRedirectFormStore.RedirectForm> opt =
                storeMapService.consumeRedirectForm(request.getRedirectId());
        if (opt.isEmpty()) {
            responseObserver.onError(io.grpc.Status.NOT_FOUND
                    .withDescription("redirect form not found or expired")
                    .asRuntimeException());
            return;
        }

        StoreMapRedirectFormStore.RedirectForm form = opt.get();
        responseObserver.onNext(GetStoreMapRedirectFormResponse.newBuilder()
                .setFormAction(nullSafe(form.formAction()))
                .setMerId(nullSafe(form.merId()))
                .setVersion(nullSafe(form.version()))
                .setEncryptInfo(nullSafe(form.encryptInfo()))
                .setHashInfo(nullSafe(form.hashInfo()))
                .build());
        responseObserver.onCompleted();
    }

    /** proto3 string 欄位不接受 null，統一 null → 空字串 */
    private static String nullSafe(String value) {
        return value == null ? "" : value;
    }

    // ==================== 買家常用門市管理 ====================

    @Override
    public void addBuyerStorePickup(AddBuyerStorePickupRequest request,
                                     StreamObserver<AddBuyerStorePickupResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER, AccountType.SELLER_AND_BUYER);

            long buyerId = request.getBuyerId();
            BuyerStorePickupEntity entity = buyerStorePickupService.addStorePickup(
                    buyerId,
                    request.getStoreCode(),
                    request.getStoreName(),
                    request.getStoreAddress(),
                    request.getReceiverName(),
                    request.getReceiverPhone(),
                    request.getIsDefault());

            responseObserver.onNext(AddBuyerStorePickupResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(buildPickupView(entity))
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(AddBuyerStorePickupResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, AddBuyerStorePickupResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "addBuyerStorePickup");
        }
    }

    @Override
    public void updateBuyerStorePickup(UpdateBuyerStorePickupRequest request,
                                        StreamObserver<UpdateBuyerStorePickupResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER, AccountType.SELLER_AND_BUYER);

            long buyerId = request.getBuyerId();
            String pickupIdStr = request.getStorePickupId();
            if (pickupIdStr.isEmpty()) {
                responseObserver.onNext(UpdateBuyerStorePickupResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("storePickupId 不可為空")
                        .build());
                responseObserver.onCompleted();
                return;
            }
            long pickupId = ProtoConversionUtils.parseLongRequired(pickupIdStr, "storePickupId");

            BuyerStorePickupEntity entity = buyerStorePickupService.updateStorePickup(
                    buyerId, pickupId,
                    request.getStoreCode(),
                    request.getStoreName(),
                    request.getStoreAddress(),
                    request.getReceiverName(),
                    request.getReceiverPhone(),
                    request.getIsDefault());

            responseObserver.onNext(UpdateBuyerStorePickupResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(buildPickupView(entity))
                    .build());
            responseObserver.onCompleted();
        } catch (IllegalArgumentException e) {
            log.error("ShippingGrpc.updateBuyerStorePickup.error", "參數格式錯誤: " + e.getMessage(), e);
            responseObserver.onNext(UpdateBuyerStorePickupResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("storePickupId 格式錯誤")
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(UpdateBuyerStorePickupResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, UpdateBuyerStorePickupResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "updateBuyerStorePickup");
        }
    }

    @Override
    public void listBuyerStorePickups(ListBuyerStorePickupsRequest request,
                                       StreamObserver<ListBuyerStorePickupsResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER, AccountType.SELLER_AND_BUYER);

            long buyerId = request.getBuyerId();
            List<BuyerStorePickupEntity> pickups = buyerStorePickupService.listStorePickups(buyerId);

            ListBuyerStorePickupsResponse.Builder respBuilder = ListBuyerStorePickupsResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message());

            for (BuyerStorePickupEntity p : pickups) {
                respBuilder.addData(buildPickupView(p));
            }

            responseObserver.onNext(respBuilder.build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(ListBuyerStorePickupsResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, ListBuyerStorePickupsResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "listBuyerStorePickups");
        }
    }

    @Override
    public void deleteBuyerStorePickup(DeleteBuyerStorePickupRequest request,
                                        StreamObserver<DeleteBuyerStorePickupResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.BUYER, AccountType.SELLER_AND_BUYER);

            long buyerId = request.getBuyerId();
            String pickupIdStr = request.getStorePickupId();
            if (pickupIdStr.isEmpty()) {
                responseObserver.onNext(DeleteBuyerStorePickupResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("storePickupId 不可為空")
                        .build());
                responseObserver.onCompleted();
                return;
            }
            long pickupId = ProtoConversionUtils.parseLongRequired(pickupIdStr, "storePickupId");

            buyerStorePickupService.deleteStorePickup(buyerId, pickupId);

            responseObserver.onNext(DeleteBuyerStorePickupResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .build());
            responseObserver.onCompleted();
        } catch (IllegalArgumentException e) {
            log.error("ShippingGrpc.deleteBuyerStorePickup.error", "參數格式錯誤: " + e.getMessage(), e);
            responseObserver.onNext(DeleteBuyerStorePickupResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("storePickupId 格式錯誤")
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(DeleteBuyerStorePickupResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, DeleteBuyerStorePickupResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "deleteBuyerStorePickup");
        }
    }

    // ==================== 回應組裝 ====================

    private ShippingOnboardingView buildOnboardingView(SellerShipmentSettingEntity entity) {
        String createdAt = "";
        if (entity.getCreatedAt() != null) {
            createdAt = entity.getCreatedAt().format(ISO_FMT);
        }
        String updatedAt = "";
        if (entity.getUpdatedAt() != null) {
            updatedAt = entity.getUpdatedAt().format(ISO_FMT);
        }

        return ShippingOnboardingView.newBuilder()
                .setOnboardingId(String.valueOf(entity.getId()))
                .setShippingType(nullToEmpty(entity.getShippingType()))
                .setGoodsType(nullToEmpty(entity.getGoodsType()))
                .setOnboardingStatus(nullToEmpty(entity.getOnboardingStatus()))
                .setProviderStatus(nullToEmpty(entity.getProviderStatus()))
                .setProviderMessage(nullToEmpty(entity.getProviderMessage()))
                .setPartnerId(nullToEmpty(entity.getPartnerId()))
                .setEshipId(nullToEmpty(entity.getEshipId()))
                .setReturnStoreCode(nullToEmpty(entity.getReturnStoreCode()))
                .setProductCategory(nullToEmpty(entity.getProductCategoryCode()))
                .setLabelCompanyName(nullToEmpty(entity.getLabelCompanyName()))
                .setReturnStoreName(nullToEmpty(entity.getReturnStoreName()))
                .setReturnStoreAddress(nullToEmpty(entity.getReturnStoreAddress()))
                .setEnabled(entity.getEnabled() != null && entity.getEnabled() ? "true" : "false")
                // 寄件人/退貨資訊（物流設定獨立化）
                .setSenderName(nullToEmpty(entity.getSenderName()))
                .setSenderPhone(nullToEmpty(entity.getSenderPhone()))
                .setSenderCity(nullToEmpty(entity.getSenderCity()))
                .setSenderDistrict(nullToEmpty(entity.getSenderDistrict()))
                .setSenderAddressDetail(nullToEmpty(entity.getSenderAddressDetail()))
                .setSenderPostalCode(nullToEmpty(entity.getSenderPostalCode()))
                .setReturnName(nullToEmpty(entity.getReturnName()))
                .setReturnPhone(nullToEmpty(entity.getReturnPhone()))
                .setReturnCity(nullToEmpty(entity.getReturnCity()))
                .setReturnDistrict(nullToEmpty(entity.getReturnDistrict()))
                .setReturnAddressDetail(nullToEmpty(entity.getReturnAddressDetail()))
                .setReturnPostalCode(nullToEmpty(entity.getReturnPostalCode()))
                .setPayuniSubmitStatus(nullToEmpty(entity.getPayuniSubmitStatus()))
                .setCreatedAt(createdAt)
                .setUpdatedAt(updatedAt)
                .build();
    }

    /**
     * 門市地圖能力的唯一取得點。
     *
     * <p>⚠️ {@code forStore(0L, ...)}：買家選店發生在<b>還沒有任何配送單、也還沒選定賣家</b>的時候，
     * 硬湊一個 storeId 只會讓「這個欄位是真的還是湊的」變成下一個人的問題
     * （{@code FulfillmentContext} 的 javadoc 明列本情境）。
     */
    private StoreLocatorCapability requireStoreLocator() {
        return fulfillmentProviders.require(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value())
                .storeLocator(io.github.will413028.hawkr.shipping.provider.FulfillmentContext.forStore(
                        0L, ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP))
                .orElseThrow(() -> new ShippingBusinessException(
                        GrpcResponseCode.FULFILLMENT_PROVIDER_NOT_ENABLED));
    }

    private ShipmentDetailView buildDetailView(ShipmentEntity entity) {
        // 49 §6.6：只填 objectKey，Gateway 組 labelUrl
        String labelKey = "";
        if (entity.getLabelObjectKey() != null && !entity.getLabelObjectKey().isBlank()) {
            labelKey = entity.getLabelObjectKey();
        }

        String statusTime = "";
        if (entity.getProviderStatusTime() != null) {
            statusTime = entity.getProviderStatusTime().format(ISO_FMT);
        }

        String createdAt = "";
        if (entity.getCreatedAt() != null) {
            createdAt = entity.getCreatedAt().format(ISO_FMT);
        }

        return ShipmentDetailView.newBuilder()
                .setShipmentNo(entity.getShipmentNo())
                .setOrderNo(entity.getOrderNo())
                .setShippingMethod(nullToEmpty(entity.getShippingMethod()))
                .setStatus(entity.getStatus())
                .setProviderStatus(nullToEmpty(entity.getProviderStatus()))
                .setProviderMessage(nullToEmpty(entity.getProviderMessage()))
                .setShipTradeNo(nullToEmpty(entity.getShipTradeNo()))
                .setRecipientName(nullToEmpty(entity.getRecipientName()))
                .setRecipientPhone(nullToEmpty(entity.getRecipientPhone()))
                .setRecipientAddress(nullToEmpty(entity.getRecipientAddress()))
                // 語意債收斂：本欄裝的是**超商門市代號**，而同一支 proto 內 Admin 那組的 store_id
                // 裝的是**賣家 store 代理鍵**——同名兩種語意。proto 的 store_id(11) 已 reserved。
                // 實查零外露消費者（gateway 的 OrderLogisticsResp、賣家追蹤 DTO、order-service 的
                // ShippingGrpcClient 皆未讀 GetStoreId）⇒ 可直接切，不需相容窗口。
                .setCvsStoreCode(nullToEmpty(entity.getCvsStoreCode()))
                .setStoreName(nullToEmpty(entity.getStoreName()))
                .setLabelKey(labelKey)
                .setStatusTime(statusTime)
                .setCreatedAt(createdAt)
                .setLabelPrintStatus(nullToEmpty(entity.getLabelPrintStatus()))
                .setTcatCallStatus(nullToEmpty(entity.getTcatCallStatus()))
                .setPartnerId(nullToEmpty(entity.getPartnerId()))
                .setOdno(nullToEmpty(entity.getOdno()))
                .setValidationNo(nullToEmpty(entity.getValidationNo()))
                .addAllShipmentPhotoKeys(buildPhotoKeys(entity))
                .setStatusLabel(resolveStatusLabel(entity.getStatus()))
                // 🔴 前端不得自行從 status 推導可做什麼——本欄是唯一出口。
                //    遷移前這一格從未被填過（proto 有欄位、gateway 有路由表，中間沒有人算），
                //    於是對外契約上它永遠是空陣列。
                .addAllAllowedActions(toProtoActions(allowedActionResolver.resolve(entity)))
                .build();
    }

    /**
     * {@code ActionAvailability}（業務判斷）→ proto。
     *
     * <p>⚠️ {@code disabledReasonCode} 對外<b>是字串</b>（§4 型別規範：對外數字一律字串）；
     * {@code enabled=true} 時留空，不填 "0"——"0" 會被前端讀成一個真的業務碼。
     */
    private static java.util.List<ActionAvailability> toProtoActions(
            java.util.List<io.github.will413028.hawkr.shipping.provider.action.ActionAvailability> availabilities) {
        return availabilities.stream()
                .map(a -> ActionAvailability.newBuilder()
                        .setAction(a.action())
                        .setEnabled(a.enabled())
                        .setDisabledReasonCode(
                                a.disabledReasonCode() == null ? "" : String.valueOf(a.disabledReasonCode()))
                        // 🔴 actor 集合恆非空（record 的建構子擋住空集合）：Gateway 靠它決定
                        //    這一項要不要進買家端／賣家端的回應。少了它，買家會拿到賣家的動作。
                        //    是集合不是單值——track/view_only 買賣家都能做，只是端點不同。
                        .addAllActorAccountTypes(a.actorAccountTypes())
                        .build())
                .toList();
    }

    /** 內部物流狀態 value → 人類可讀 label（ShippingStatus CodeMeta SoT）；未知 value 回空字串防 fromValue 拋例外 */
    private String resolveStatusLabel(String statusValue) {
        if (statusValue == null || !ShippingStatus.isValidValue(statusValue)) {
            return "";
        }
        return ShippingStatus.fromValue(statusValue).label();
    }

    /**
     * 回傳 shipmentPhotos JSONB 中的 objectKey（proto shipment_photo_urls 欄名沿用、值為 objectKey；
     * 顯示 URL 由 Gateway 組，49 §6.2）。
     */
    private java.util.List<String> buildPhotoKeys(ShipmentEntity entity) {
        if (entity.getShipmentPhotos() == null || entity.getShipmentPhotos().isEmpty()) {
            return java.util.Collections.emptyList();
        }
        return entity.getShipmentPhotos().stream()
                .map(ShipmentEntity.ShipmentPhoto::getObjectKey)
                .filter(java.util.Objects::nonNull)
                .toList();
    }

    // ==================== 運費計算（服務間呼叫）====================

    @Override
    public void getShippingFeeConfigs(GetShippingFeeConfigsRequest request,
                                       StreamObserver<GetShippingFeeConfigsResponse> responseObserver) {
        try {
            List<ShippingMethodConfigEntity> configs = shippingFeeService.getAllConfigs();
            List<ShippingFeeTierEntity> tiers = shippingFeeService.getAllTiers();

            GetShippingFeeConfigsResponse.Builder builder = GetShippingFeeConfigsResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message());

            for (ShippingMethodConfigEntity c : configs) {
                builder.addMethodConfigs(ShippingMethodConfigProto.newBuilder()
                        .setShippingMethod(c.getShippingMethod())
                        .setDeliveryTemp(c.getDeliveryTemp())
                        .setMaxDimensionSumCm(String.valueOf(c.getMaxDimensionSumCm()))
                        .setMaxSingleSideCm(String.valueOf(c.getMaxSingleSideCm()))
                        .setMaxWeightGrams(String.valueOf(c.getMaxWeightGrams()))
                        .setEstimatedDays(String.valueOf(c.getEstimatedDays()))
                        .setSettlementDays(String.valueOf(c.getSettlementDays()))
                        .setCommissionRate(c.getCommissionRate().toPlainString())
                        .setCommissionMin(String.valueOf(c.getCommissionMin()))
                        .build());
            }

            for (ShippingFeeTierEntity t : tiers) {
                builder.addFeeTiers(ShippingFeeTierProto.newBuilder()
                        .setShippingMethod(t.getShippingMethod())
                        .setDeliveryTemp(t.getDeliveryTemp())
                        .setDeliveryZone(t.getDeliveryZone())
                        .setMaxDimensionCm(String.valueOf(t.getMaxDimensionCm()))
                        .setFee(String.valueOf(t.getFee()))
                        .build());
            }

            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(GetShippingFeeConfigsResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetShippingFeeConfigsResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "getShippingFeeConfigs");
        }
    }

    @Override
    public void lookupShippingFee(LookupShippingFeeRequest request,
                                   StreamObserver<LookupShippingFeeResponse> responseObserver) {
        try {
            String method = request.getShippingMethod();
            String temp = request.getDeliveryTemp();
            String zone = request.getDeliveryZone();
            String dimStr = request.getDimensionSum();

            if (method.isEmpty() || temp.isEmpty() || zone.isEmpty()) {
                responseObserver.onNext(LookupShippingFeeResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("缺少必要參數: shippingMethod, deliveryTemp, deliveryZone")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            int dimensionSum = ProtoConversionUtils.parseIntOrDefault(dimStr, 0);

            Long fee = shippingFeeService.lookupFee(method, temp, zone, dimensionSum);

            LookupShippingFeeResponse.Builder builder = LookupShippingFeeResponse.newBuilder();
            if (fee != null) {
                ShippingMethodConfigEntity config = shippingFeeService.getConfig(method, temp);
                builder.setCode(GrpcResponseCode.SUCCESS.code())
                        .setMessage(GrpcResponseCode.SUCCESS.message())
                        .setFee(String.valueOf(fee));
                if (config != null) {
                    builder.setEstimatedDays(String.valueOf(config.getEstimatedDays()))
                            .setSettlementDays(String.valueOf(config.getSettlementDays()));
                }
            } else {
                builder.setCode(GrpcResponseCode.SUCCESS.code())
                        .setMessage("該配送方式不可用或超過材積限制");
            }

            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
        } catch (IllegalArgumentException e) {
            log.error("ShippingGrpc.lookupShippingFee.error", "參數格式錯誤: " + e.getMessage(), e);
            responseObserver.onNext(LookupShippingFeeResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("參數格式錯誤: " + e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(LookupShippingFeeResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, LookupShippingFeeResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "lookupShippingFee");
        }
    }

    /**
     * 查詢賣家物流開通狀態與寄件郵遞區號（供 Order Service / Product Service 使用）
     */
    @Override
    public void getSellerShippingConfig(GetSellerShippingConfigRequest request,
                                         StreamObserver<GetSellerShippingConfigResponse> responseObserver) {
        try {
            long storeId = request.getStoreId();
            if (storeId <= 0) {
                responseObserver.onNext(GetSellerShippingConfigResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("store_id 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            List<SellerShippingMethodSlot> slots =
                    onboardingService.listSellerShippingMethods(storeId);

            GetSellerShippingConfigResponse.Builder builder = GetSellerShippingConfigResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message());

            for (SellerShippingMethodSlot slot : slots) {
                // 查詢對應的 setting 以取得 sender_postal_code
                String postalCode = "";
                String city = "";
                String district = "";
                if (slot.available() && slot.onboardingId() != null) {
                    SellerShipmentSettingEntity setting = onboardingService.getSettingById(slot.onboardingId());
                    if (setting != null) {
                        if (setting.getSenderPostalCode() != null) {
                            postalCode = setting.getSenderPostalCode();
                        }
                        if (setting.getSenderCity() != null) {
                            city = setting.getSenderCity();
                        }
                        if (setting.getSenderDistrict() != null) {
                            district = setting.getSenderDistrict();
                        }
                    }
                }

                builder.addConfigs(SellerShippingConfigItem.newBuilder()
                        .setShippingMethod(slot.shippingMethod())
                        .setGoodsType(slot.goodsType())
                        .setAvailable(slot.available())
                        .setSenderPostalCode(postalCode)
                        .setSenderCity(city)
                        .setSenderDistrict(district)
                        .build());
            }

            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(GetSellerShippingConfigResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetSellerShippingConfigResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "getSellerShippingConfig");
        }
    }

    @Override
    public void calculateShippingOptions(CalculateShippingOptionsRequest request,
                                          StreamObserver<CalculateShippingOptionsResponse> responseObserver) {
        try {
            String lengthStr = request.getLengthCm();
            String widthStr = request.getWidthCm();
            String heightStr = request.getHeightCm();
            String weightStr = request.getWeightGrams();

            if (lengthStr.isEmpty() || widthStr.isEmpty() || heightStr.isEmpty() || weightStr.isEmpty()) {
                responseObserver.onNext(CalculateShippingOptionsResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("缺少必要參數: lengthCm, widthCm, heightCm, weightGrams")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            int lengthCm = ProtoConversionUtils.parseIntRequired(lengthStr, "lengthCm");
            int widthCm = ProtoConversionUtils.parseIntRequired(widthStr, "widthCm");
            int heightCm = ProtoConversionUtils.parseIntRequired(heightStr, "heightCm");
            long weightGrams = ProtoConversionUtils.parseLongRequired(weightStr, "weightGrams");
            String deliveryZone = request.getDeliveryZone();

            // 賣家開通狀態過濾（物流設定獨立化）
            Set<String> sellerAvailableMethods = null;
            long storeId = request.getStoreId();
            if (storeId > 0) {
                sellerAvailableMethods = new HashSet<>();
                List<SellerShippingMethodSlot> slots =
                        onboardingService.listSellerShippingMethods(storeId);
                for (SellerShippingMethodSlot slot : slots) {
                    if (slot.available()) {
                        // 宅配 key="TAQBIN:", 超商 key="SEVEN_ELEVEN_STORE_PICKUP:AMBIENT"
                        sellerAvailableMethods.add(slot.shippingMethod() + ":" + slot.goodsType());
                    }
                }
            }

            ShippingFeeService.CalculateResult result =
                    shippingFeeService.calculateShippingOptions(
                            lengthCm, widthCm, heightCm, weightGrams, deliveryZone,
                            sellerAvailableMethods);

            CalculateShippingOptionsResponse.Builder builder = CalculateShippingOptionsResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setWeightGrams(String.valueOf(result.weightGrams()))
                    .setDimensionSum(String.valueOf(result.dimensionSum()))
                    .setLongestSide(String.valueOf(result.longestSide()));

            for (Map.Entry<ShippingTemperature, List<ShippingFeeService.ShippingOptionItem>> entry
                    : result.temperatureGroups().entrySet()) {
                ShippingTemperature temp = entry.getKey();
                ShippingOptionGroup.Builder groupBuilder = ShippingOptionGroup.newBuilder()
                        .setTemperature(temp.value())
                        .setDisplayName(temp.label());

                for (ShippingFeeService.ShippingOptionItem item : entry.getValue()) {
                    groupBuilder.addMethods(ShippingOptionDetail.newBuilder()
                            .setShippingMethod(item.shippingMethod())
                            .setDisplayName(item.displayName())
                            .setFee(String.valueOf(item.fee()))
                            .setEstimatedDays(String.valueOf(item.estimatedDays()))
                            .setSettlementDays(String.valueOf(item.settlementDays()))
                            .setMaxDimensionSum(String.valueOf(item.maxDimensionSumCm()))
                            .setMaxLongestSide(String.valueOf(item.maxSingleSideCm()))
                            .setMaxWeightGrams(String.valueOf(item.maxWeightGrams()))
                            .build());
                }

                builder.addTemperatureGroups(groupBuilder.build());
            }

            // 無任何可用選項時，回報「真正原因」的錯誤碼 + 訊息供上游透傳（避免一律誤指尺寸）：
            // sellerAvailableMethods 為空集合 = 賣家一個配送方式都沒開通 → SELLER_NOT_ONBOARDED，引導去物流設定；
            // 已開通但全被材積/重量擋下 → SHIPPING_ALL_METHODS_EXCEED_LIMITS。
            if (result.temperatureGroups().isEmpty()) {
                boolean noneOnboarded = sellerAvailableMethods != null && sellerAvailableMethods.isEmpty();
                if (noneOnboarded) {
                    builder.setCode(GrpcResponseCode.SELLER_NOT_ONBOARDED.code())
                            .setMessage("您尚未開通任何配送方式，請先至「物流設定」開通後再設定商品配送");
                } else {
                    builder.setCode(GrpcResponseCode.SHIPPING_ALL_METHODS_EXCEED_LIMITS.code())
                            .setMessage(GrpcResponseCode.SHIPPING_ALL_METHODS_EXCEED_LIMITS.message());
                }
            }

            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
        } catch (IllegalArgumentException e) {
            log.error("ShippingGrpc.calculateShippingOptions.error", "參數格式錯誤: " + e.getMessage(), e);
            responseObserver.onNext(CalculateShippingOptionsResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("參數格式錯誤: " + e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(CalculateShippingOptionsResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CalculateShippingOptionsResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "calculateShippingOptions");
        }
    }

    private BuyerStorePickupView buildPickupView(BuyerStorePickupEntity entity) {
        String createdAt = "";
        if (entity.getCreatedAt() != null) {
            createdAt = entity.getCreatedAt().format(ISO_FMT);
        }
        String updatedAt = "";
        if (entity.getUpdatedAt() != null) {
            updatedAt = entity.getUpdatedAt().format(ISO_FMT);
        }

        return BuyerStorePickupView.newBuilder()
                .setId(String.valueOf(entity.getId()))
                .setStoreCode(nullToEmpty(entity.getStoreCode()))
                .setStoreName(nullToEmpty(entity.getStoreName()))
                .setStoreAddress(nullToEmpty(entity.getStoreAddress()))
                .setReceiverName(nullToEmpty(entity.getReceiverName()))
                .setReceiverPhone(nullToEmpty(entity.getReceiverPhone()))
                .setIsDefault(entity.getIsDefault() != null && entity.getIsDefault() ? "true" : "false")
                .setCreatedAt(createdAt)
                .setUpdatedAt(updatedAt)
                .build();
    }

    // ==================== 假日管理 ====================

    @Override
    public void importHolidays(ImportHolidaysRequest request,
                                StreamObserver<ImportHolidaysResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            String csvUrl = request.getCsvUrl();
            if (csvUrl == null || csvUrl.isBlank()) {
                responseObserver.onNext(ImportHolidaysResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("csv_url 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            HolidayService.ImportResult result = holidayService.importFromCsvUrl(csvUrl);

            responseObserver.onNext(ImportHolidaysResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(ImportHolidaysData.newBuilder()
                            .setYear(String.valueOf(result.year()))
                            .setImportedCount(String.valueOf(result.importedCount()))
                            .setDeletedCount(String.valueOf(result.deletedCount()))
                            .build())
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(ImportHolidaysResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, ImportHolidaysResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "importHolidays");
        }
    }

    @Override
    public void importHolidaysFromContent(ImportHolidaysFromContentRequest request,
                                           StreamObserver<ImportHolidaysResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            String year = request.getYear();
            String csvContent = request.getCsvContent();
            if (year == null || year.isBlank()) {
                responseObserver.onNext(ImportHolidaysResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("year 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }
            if (csvContent == null || csvContent.isBlank()) {
                responseObserver.onNext(ImportHolidaysResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("csv_content 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            HolidayService.ImportResult result = holidayService.importFromCsvContent(year, csvContent);

            responseObserver.onNext(ImportHolidaysResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(ImportHolidaysData.newBuilder()
                            .setYear(String.valueOf(result.year()))
                            .setImportedCount(String.valueOf(result.importedCount()))
                            .setDeletedCount(String.valueOf(result.deletedCount()))
                            .build())
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(ImportHolidaysResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, ImportHolidaysResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "importHolidaysFromContent");
        }
    }

    @Override
    public void getHolidayCoverage(GetHolidayCoverageRequest request,
                                    StreamObserver<GetHolidayCoverageResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            HolidayService.HolidayCoverage coverage = holidayService.getHolidayCoverage();

            HolidayCoverageData.Builder dataBuilder = HolidayCoverageData.newBuilder()
                    .setCoverageDays(String.valueOf(coverage.coverageDays()));
            for (HolidayYearCoverageRef ref : coverage.years()) {
                dataBuilder.addYears(HolidayYearCoverage.newBuilder()
                        .setYear(String.valueOf(ref.year()))
                        .setCount(String.valueOf(ref.count()))
                        .setMaxHolidayDate(ref.maxHolidayDate() != null ? ref.maxHolidayDate().toString() : "")
                        .build());
            }

            responseObserver.onNext(GetHolidayCoverageResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(dataBuilder.build())
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(GetHolidayCoverageResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetHolidayCoverageResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "getHolidayCoverage");
        }
    }

    /**
     * 取一段區間內的國定假日（服務間；invoice-service 的行事曆鏡像每日同步）。
     *
     * <p>🔴 授權走 <b>isInternalCall</b> 而不是 requireAccountType：呼叫端是
     * invoice-service 的排程器（INV.gov），沒有使用者身分可傳。內部身分靠
     * HMAC 簽章驗證（52 號設計書），字面值 header 不是信任依據。
     */
    @Override
    public void getHolidaysInternal(GetHolidaysInternalRequest request,
                                     StreamObserver<GetHolidaysInternalResponse> responseObserver) {
        try {
            if (!GrpcAuthContext.isInternalCall()) {
                responseObserver.onNext(GetHolidaysInternalResponse.newBuilder()
                        .setCode(GrpcResponseCode.PERMISSION_DENIED.code())
                        .setMessage(GrpcResponseCode.PERMISSION_DENIED.message())
                        .build());
                responseObserver.onCompleted();
                return;
            }

            java.time.LocalDate from = parseHolidayDate(request.getFromDate(), "from_date");
            java.time.LocalDate to = parseHolidayDate(request.getToDate(), "to_date");

            GetHolidaysInternalResponse.Builder builder = GetHolidaysInternalResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message());
            for (java.time.LocalDate d : holidayService.getHolidaysBetween(from, to)) {
                builder.addHolidayDates(d.toString());
            }
            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(GetHolidaysInternalResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetHolidaysInternalResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "getHolidaysInternal");
        }
    }

    /**
     * 解析 ISO 8601 日期。
     *
     * <p>格式錯誤回 INVALID_PARAM(400) 而不是讓 DateTimeParseException 變成一句 500：
     * 呼叫端是另一個服務，它需要分得出「我送錯了」與「對方壞了」。
     * 形狀沿用同檔的 parseLong。
     */
    private java.time.LocalDate parseHolidayDate(String raw, String field) {
        if (raw == null || raw.isBlank()) {
            throw new ShippingBusinessException(GrpcResponseCode.INVALID_PARAM, field + " 為必填");
        }
        try {
            return java.time.LocalDate.parse(raw);
        } catch (java.time.format.DateTimeParseException e) {
            throw new ShippingBusinessException(GrpcResponseCode.INVALID_PARAM, field + " 格式錯誤（YYYY-MM-DD）");
        }
    }

    @Override
    public void listBusinessDays(ListBusinessDaysRequest request,
                                  StreamObserver<ListBusinessDaysResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            java.util.List<java.time.LocalDate> days = holidayService.getNextBusinessDays(7);
            String[] dayNames = {"", "一", "二", "三", "四", "五", "六", "日"};

            ListBusinessDaysResponse.Builder builder = ListBusinessDaysResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message());

            for (java.time.LocalDate day : days) {
                int dow = day.getDayOfWeek().getValue(); // 1=Monday...7=Sunday
                builder.addBusinessDays(BusinessDay.newBuilder()
                        .setDate(day.toString())
                        .setDayOfWeek(dayNames[dow])
                        .build());
            }

            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(ListBusinessDaysResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, ListBusinessDaysResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "listBusinessDays");
        }
    }

    // ==================== 商品配送驗證 ====================

    @Override
    public void validateProductShipping(ValidateProductShippingRequest request,
                                         StreamObserver<ValidateProductShippingResponse> responseObserver) {
        try {
            long storeId = request.getStoreId();
            if (storeId <= 0) {
                responseObserver.onNext(ValidateProductShippingResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("store_id 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            if (request.getItemsList().isEmpty()) {
                responseObserver.onNext(ValidateProductShippingResponse.newBuilder()
                        .setCode(GrpcResponseCode.SUCCESS.code())
                        .setMessage(GrpcResponseCode.SUCCESS.message())
                        .setAllValid(true)
                        .build());
                responseObserver.onCompleted();
                return;
            }

            // 建立賣家已開通配送方式集合（同 calculateShippingOptions 邏輯）
            Set<String> sellerAvailableMethods = new HashSet<>();
            List<SellerShippingMethodSlot> slots =
                    onboardingService.listSellerShippingMethods(storeId);
            for (SellerShippingMethodSlot slot : slots) {
                if (slot.available()) {
                    sellerAvailableMethods.add(slot.shippingMethod() + ":" + slot.goodsType());
                }
            }

            // 轉換 Proto → Service DTO
            // dim/weight 容忍十進位字串（如 "18.00"），由 ProtoConversionUtils 統一向 0 truncate；
            // 見 22 號設計書 §2.7。
            List<ShippingFeeService.ValidationItem> items = new java.util.ArrayList<>();
            for (ValidateShippingItem protoItem : request.getItemsList()) {
                items.add(new ShippingFeeService.ValidationItem(
                        protoItem.getSkuName(),
                        protoItem.getShippingMethod(),
                        protoItem.getTemperature(),
                        ProtoConversionUtils.parseDecimalToInt(protoItem.getLengthCm(), "lengthCm"),
                        ProtoConversionUtils.parseDecimalToInt(protoItem.getWidthCm(), "widthCm"),
                        ProtoConversionUtils.parseDecimalToInt(protoItem.getHeightCm(), "heightCm"),
                        ProtoConversionUtils.parseDecimalToLong(protoItem.getWeightGrams(), "weightGrams")
                ));
            }

            // 執行驗證
            ShippingFeeService.ValidationResult result =
                    shippingFeeService.validateProductShipping(items, sellerAvailableMethods);

            // 組裝回應
            ValidateProductShippingResponse.Builder builder = ValidateProductShippingResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setAllValid(result.allValid());

            for (ShippingFeeService.ValidationResultItem item : result.results()) {
                ValidateShippingResult.Builder resultBuilder = ValidateShippingResult.newBuilder()
                        .setSkuName(item.skuName())
                        .setShippingMethod(item.shippingMethod())
                        .setTemperature(item.temperature())
                        .setValid(item.valid())
                        .setRejectionReason(item.rejectionReason())
                        .setRejectionMessage(item.rejectionMessage());

                if (item.maxDimensionSumCm() > 0) {
                    resultBuilder.setMaxDimensionSumCm(String.valueOf(item.maxDimensionSumCm()));
                }
                if (item.maxSingleSideCm() > 0) {
                    resultBuilder.setMaxSingleSideCm(String.valueOf(item.maxSingleSideCm()));
                }
                if (item.maxWeightGrams() > 0) {
                    resultBuilder.setMaxWeightGrams(String.valueOf(item.maxWeightGrams()));
                }

                builder.addResults(resultBuilder.build());
            }

            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
        } catch (IllegalArgumentException e) {
            log.error("ShippingGrpc.validateProductShipping.error", "參數格式錯誤: " + e.getMessage(), e);
            responseObserver.onNext(ValidateProductShippingResponse.newBuilder()
                    .setCode(GrpcResponseCode.INVALID_PARAM.code())
                    .setMessage("參數格式錯誤: " + e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(ValidateProductShippingResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, ValidateProductShippingResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "validateProductShipping");
        }
    }

    // ==================== 退貨物流（AfterSales Service 呼叫）====================

    @Override
    public void createReturnCvsShipment(CreateReturnCvsShipmentRequest request,
                                         StreamObserver<CreateReturnShipmentResponse> responseObserver) {
        try {
            String orderNo = request.getOrderNo();
            long storeId = ProtoConversionUtils.parseLongRequired(request.getStoreId(), "store_id");
            String merId = request.getMerId();
            String buyerName = request.getBuyerName();
            String buyerPhone = request.getBuyerPhone();
            String temperature = request.getTemperature();
            long tradeAmt = ProtoConversionUtils.parseLongOrDefault(request.getTradeAmt(), 0);

            if (orderNo.isEmpty() || merId.isEmpty() || buyerName.isEmpty() || buyerPhone.isEmpty() || temperature.isEmpty()) {
                responseObserver.onNext(CreateReturnShipmentResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("缺少必要參數")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            ShippingService.ReturnShipmentResult result =
                    shippingService.createReturnCvsShipment(orderNo, storeId, merId, buyerName, buyerPhone, temperature, tradeAmt);

            ReturnShipmentData data = ReturnShipmentData.newBuilder()
                    .setShipmentNo(result.shipmentNo())
                    .setTrackingCode(result.trackingCode() != null ? result.trackingCode() : "")
                    .setReturnStoreCode(result.returnStoreCode() != null ? result.returnStoreCode() : "")
                    .setReturnStoreName(result.returnStoreName() != null ? result.returnStoreName() : "")
                    .setShipTradeNo(result.shipTradeNo() != null ? result.shipTradeNo() : "")
                    .build();

            responseObserver.onNext(CreateReturnShipmentResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(CreateReturnShipmentResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CreateReturnShipmentResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "createReturnCvsShipment");
        }
    }

    @Override
    public void createReturnHomeShipment(CreateReturnHomeShipmentRequest request,
                                          StreamObserver<CreateReturnShipmentResponse> responseObserver) {
        try {
            String orderNo = request.getOrderNo();
            long storeId = ProtoConversionUtils.parseLongRequired(request.getStoreId(), "store_id");
            String merId = request.getMerId();
            String buyerName = request.getBuyerName();
            String buyerPhone = request.getBuyerPhone();
            String buyerCity = request.getBuyerCity();
            String buyerDistrict = request.getBuyerDistrict();
            String buyerAddress = request.getBuyerAddress();
            String buyerPostalCode = request.getBuyerPostalCode();
            String temperature = request.getTemperature();

            if (orderNo.isEmpty() || merId.isEmpty() || buyerName.isEmpty() || buyerPhone.isEmpty() || temperature.isEmpty()) {
                responseObserver.onNext(CreateReturnShipmentResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("缺少必要參數")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            ShippingService.ReturnShipmentResult result =
                    shippingService.createReturnHomeShipment(orderNo, storeId, merId,
                            buyerName, buyerPhone, buyerCity, buyerDistrict, buyerAddress, buyerPostalCode,
                            temperature);

            ReturnShipmentData data = ReturnShipmentData.newBuilder()
                    .setShipmentNo(result.shipmentNo())
                    .setShipTradeNo(result.shipTradeNo() != null ? result.shipTradeNo() : "")
                    .build();

            responseObserver.onNext(CreateReturnShipmentResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(CreateReturnShipmentResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, CreateReturnShipmentResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "createReturnHomeShipment");
        }
    }

    @Override
    public void getSellerReturnConfig(GetSellerReturnConfigRequest request,
                                       StreamObserver<GetSellerReturnConfigResponse> responseObserver) {
        try {
            long storeId = ProtoConversionUtils.parseLongRequired(request.getStoreId(), "store_id");
            String shippingType = request.getShippingType();

            if (shippingType.isEmpty()) {
                responseObserver.onNext(GetSellerReturnConfigResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("shipping_type 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            SellerShipmentSettingEntity setting =
                    shippingService.getSellerReturnConfig(storeId, shippingType);

            SellerReturnConfigData.Builder dataBuilder = SellerReturnConfigData.newBuilder();
            if (setting != null && Boolean.TRUE.equals(setting.getEnabled())) {
                dataBuilder.setAvailable(true);

                if (ShippingType.CVS_C2C.value().equals(shippingType)) {
                    // 超商退貨設定
                    dataBuilder.setReturnStoreCode(nullToEmpty(setting.getReturnStoreCode()))
                            .setReturnStoreName(nullToEmpty(setting.getReturnStoreName()))
                            .setReturnStoreAddress(nullToEmpty(setting.getReturnStoreAddress()))
                            .setReturnName(nullToEmpty(setting.getSenderName()))
                            .setReturnPhone(nullToEmpty(setting.getSenderPhone()));
                } else {
                    // 宅配退貨設定（優先用退貨專用地址）
                    String returnName = (setting.getReturnName() != null && !setting.getReturnName().isBlank())
                            ? setting.getReturnName() : setting.getSenderName();
                    String returnPhone = (setting.getReturnPhone() != null && !setting.getReturnPhone().isBlank())
                            ? setting.getReturnPhone() : setting.getSenderPhone();
                    String returnCity = (setting.getReturnCity() != null && !setting.getReturnCity().isBlank())
                            ? setting.getReturnCity() : setting.getSenderCity();
                    String returnDistrict = (setting.getReturnDistrict() != null && !setting.getReturnDistrict().isBlank())
                            ? setting.getReturnDistrict() : setting.getSenderDistrict();
                    String returnAddressDetail = (setting.getReturnAddressDetail() != null && !setting.getReturnAddressDetail().isBlank())
                            ? setting.getReturnAddressDetail() : setting.getSenderAddressDetail();

                    dataBuilder.setReturnName(nullToEmpty(returnName))
                            .setReturnPhone(nullToEmpty(returnPhone))
                            .setReturnCity(nullToEmpty(returnCity))
                            .setReturnDistrict(nullToEmpty(returnDistrict))
                            .setReturnAddressDetail(nullToEmpty(returnAddressDetail));
                }
            } else {
                dataBuilder.setAvailable(false);
            }

            responseObserver.onNext(GetSellerReturnConfigResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(dataBuilder.build())
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(GetSellerReturnConfigResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, GetSellerReturnConfigResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "getSellerReturnConfig");
        }
    }

    @Override
    public void retryReturnPrintLabel(io.github.will413028.hawkr.shipping.grpc.RetryReturnPrintLabelRequest request,
                                       StreamObserver<io.github.will413028.hawkr.shipping.grpc.RetryReturnPrintLabelResponse> responseObserver) {
        try {
            String shipTradeNo = request.getShipTradeNo();
            if (shipTradeNo == null || shipTradeNo.isBlank()) {
                responseObserver.onNext(io.github.will413028.hawkr.shipping.grpc.RetryReturnPrintLabelResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("ship_trade_no 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            shippingService.retryReturnPrintLabel(shipTradeNo);

            responseObserver.onNext(io.github.will413028.hawkr.shipping.grpc.RetryReturnPrintLabelResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(io.github.will413028.hawkr.shipping.grpc.RetryReturnPrintLabelResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    io.github.will413028.hawkr.shipping.grpc.RetryReturnPrintLabelResponse.newBuilder(),
                    e, "ShippingGrpcServiceImpl", "retryReturnPrintLabel");
        }
    }

    // ==================== 開發工具（Webhook 模擬器）====================

    @Override
    public void lookupShipTradeNo(LookupShipTradeNoRequest request,
                                   StreamObserver<LookupShipTradeNoResponse> responseObserver) {
        try {
            String orderNo = request.getOrderNo();
            boolean isReturn = request.getIsReturn();

            if (orderNo.isBlank()) {
                responseObserver.onNext(LookupShipTradeNoResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("order_no 為必填欄位")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            ShipmentEntity entity = shippingService.lookupShipment(orderNo, isReturn);
            if (entity == null) {
                String label = isReturn ? "退貨" : "正向";
                responseObserver.onNext(LookupShipTradeNoResponse.newBuilder()
                        .setCode(GrpcResponseCode.NOT_FOUND.code())
                        .setMessage("找不到訂單 " + orderNo + " 的" + label + "物流單")
                        .build());
                responseObserver.onCompleted();
                return;
            }

            responseObserver.onNext(LookupShipTradeNoResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setShipTradeNo(nullToEmpty(entity.getShipTradeNo()))
                    .setLgsType(nullToEmpty(entity.getLgsType()))
                    .setShipType(nullToEmpty(entity.getShipType()))
                    .setMerId(nullToEmpty(entity.getMerId()))
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver, LookupShipTradeNoResponse.newBuilder(), e, "ShippingGrpcServiceImpl", "lookupShipTradeNo");
        }
    }

    // ==================== 批次 provider 狀態聚合（收斂規範 §6.3）====================

    @Override
    public void batchGetProviderStatusByOrderIds(BatchGetProviderStatusByOrderIdsRequest request,
                                                  StreamObserver<BatchGetProviderStatusByOrderIdsResponse> responseObserver) {
        try {
            List<String> orderIds = request.getOrderIdsList();
            boolean isReturn = request.getIsReturn();

            var rows = shippingService.findProviderStatusByOrderNos(orderIds, isReturn);

            BatchGetProviderStatusByOrderIdsResponse.Builder builder =
                    BatchGetProviderStatusByOrderIdsResponse.newBuilder()
                            .setCode(GrpcResponseCode.SUCCESS.code())
                            .setMessage(GrpcResponseCode.SUCCESS.message());

            for (var row : rows) {
                if (row.getKey() == null) continue;
                ProviderStatusEntry.Builder eb = ProviderStatusEntry.newBuilder();
                if (row.getProviderStatus() != null) eb.setProviderStatus(row.getProviderStatus());
                if (row.getProviderMessage() != null) eb.setProviderMessage(row.getProviderMessage());
                builder.putEntries(row.getKey(), eb.build());
            }

            responseObserver.onNext(builder.build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    BatchGetProviderStatusByOrderIdsResponse.newBuilder(),
                    e, "ShippingGrpcServiceImpl", "batchGetProviderStatusByOrderIds");
        }
    }

    // ==================== Admin 異常查詢 API（§29）====================

    @Override
    public void listShipmentsForAdmin(ListShipmentsForAdminRequest request,
                                       StreamObserver<ListShipmentsForAdminResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            ListShipmentsForAdminData data = adminQueryService.listShipments(request);
            responseObserver.onNext(ListShipmentsForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    ListShipmentsForAdminResponse.newBuilder(),
                    e, "ShippingGrpcServiceImpl", "listShipmentsForAdmin");
        }
    }

    @Override
    public void getShipmentForAdmin(GetShipmentForAdminRequest request,
                                     StreamObserver<GetShipmentForAdminResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            var detailOpt = adminQueryService.getShipmentDetail(request.getShipmentId());
            if (detailOpt.isEmpty()) {
                responseObserver.onNext(GetShipmentForAdminResponse.newBuilder()
                        .setCode(GrpcResponseCode.NOT_FOUND.code())
                        .setMessage("shipment not found")
                        .build());
                responseObserver.onCompleted();
                return;
            }
            responseObserver.onNext(GetShipmentForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(detailOpt.get())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    GetShipmentForAdminResponse.newBuilder(),
                    e, "ShippingGrpcServiceImpl", "getShipmentForAdmin");
        }
    }

    @Override
    public void listSellerShipmentSettingsForAdmin(ListSellerShipmentSettingsForAdminRequest request,
                                                    StreamObserver<ListSellerShipmentSettingsForAdminResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            ListSellerShipmentSettingsForAdminData data = adminQueryService.listSettings(request);
            responseObserver.onNext(ListSellerShipmentSettingsForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    ListSellerShipmentSettingsForAdminResponse.newBuilder(),
                    e, "ShippingGrpcServiceImpl", "listSellerShipmentSettingsForAdmin");
        }
    }

    // ==================== Uber 配送查詢（66；W6-3 賣家面／W6-4 Admin 面）====================
    //
    // 🔴 四支 RPC 而不是兩支帶旗標：§5.1 角色分離。賣家面與 Admin 面的差別
    //    （store_id 強制過濾、timeline、provider raw payload）正是最需要被型別分開的部分，
    //    共用一條路徑等於讓「誰看得到什麼」變成一個 if。

    @Override
    public void listSellerUberDeliveries(ListSellerUberDeliveriesRequest request,
                                         StreamObserver<ListSellerUberDeliveriesResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);
            // 🔴 歸屬只認 request.getStoreId()（Gateway 的 sellerid resolver 由 JWT 解析後注入），
            //    而真正的過濾在 UberDeliveryQueryMapper 的 SQL 述詞——本層不做第二份判定。
            ListSellerUberDeliveriesData data = uberDeliveryQueryService.listSellerDeliveries(request);
            responseObserver.onNext(ListSellerUberDeliveriesResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(ListSellerUberDeliveriesResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    ListSellerUberDeliveriesResponse.newBuilder(),
                    e, "ShippingGrpcServiceImpl", "listSellerUberDeliveries");
        }
    }

    @Override
    public void getSellerUberDelivery(GetSellerUberDeliveryRequest request,
                                      StreamObserver<GetSellerUberDeliveryResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);
            var detail = uberDeliveryQueryService.getSellerDelivery(
                    request.getStoreId(), request.getDeliveryNo());
            if (detail.isEmpty()) {
                // ⚠️ 「不存在」與「不是你的」刻意回同一個碼：能區分就等於給了賣家一個
                //    可列舉全平台配送是否存在的 oracle。
                responseObserver.onNext(GetSellerUberDeliveryResponse.newBuilder()
                        .setCode(GrpcResponseCode.SHIPMENT_NOT_FOUND.code())
                        .setMessage(GrpcResponseCode.SHIPMENT_NOT_FOUND.message())
                        .build());
                responseObserver.onCompleted();
                return;
            }
            responseObserver.onNext(GetSellerUberDeliveryResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(detail.get())
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(GetSellerUberDeliveryResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    GetSellerUberDeliveryResponse.newBuilder(),
                    e, "ShippingGrpcServiceImpl", "getSellerUberDelivery");
        }
    }

    @Override
    public void listUberDeliveriesForAdmin(ListUberDeliveriesForAdminRequest request,
                                           StreamObserver<ListUberDeliveriesForAdminResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            ListUberDeliveriesForAdminData data = uberDeliveryQueryService.listAdminDeliveries(request);
            responseObserver.onNext(ListUberDeliveriesForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    ListUberDeliveriesForAdminResponse.newBuilder(),
                    e, "ShippingGrpcServiceImpl", "listUberDeliveriesForAdmin");
        }
    }

    @Override
    public void getUberDeliveryForAdmin(GetUberDeliveryForAdminRequest request,
                                        StreamObserver<GetUberDeliveryForAdminResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            var detail = uberDeliveryQueryService.getAdminDelivery(request.getDeliveryNo());
            if (detail.isEmpty()) {
                responseObserver.onNext(GetUberDeliveryForAdminResponse.newBuilder()
                        .setCode(GrpcResponseCode.SHIPMENT_NOT_FOUND.code())
                        .setMessage(GrpcResponseCode.SHIPMENT_NOT_FOUND.message())
                        .build());
                responseObserver.onCompleted();
                return;
            }
            responseObserver.onNext(GetUberDeliveryForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(detail.get())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    GetUberDeliveryForAdminResponse.newBuilder(),
                    e, "ShippingGrpcServiceImpl", "getUberDeliveryForAdmin");
        }
    }

    // ==================== §5.21 配送異常賣家通知 ====================

    @Override
    public void listSellerShipmentAnomalies(ListSellerShipmentAnomaliesRequest request,
                                            StreamObserver<ListSellerShipmentAnomaliesResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            long storeId = parseStoreId(request.getStoreId());
            String anomalyType = request.getAnomalyType();
            String severity = request.getSeverity();
            boolean showAcked = request.getShowAcknowledged();
            int page = parseIntDefault(request.getPage(), 1);
            int pageSize = parseIntDefault(request.getPageSize(), 20);

            ShipmentAnomalyService.PageResult<ShipmentAnomalyRow> result =
                    shipmentAnomalyService.listSellerAnomalies(storeId, anomalyType, severity, showAcked, page, pageSize);

            ListSellerShipmentAnomaliesData.Builder dataBuilder = ListSellerShipmentAnomaliesData.newBuilder()
                    .setTotal(String.valueOf(result.total()))
                    .setPage(String.valueOf(result.page()))
                    .setPageSize(String.valueOf(result.pageSize()));
            for (ShipmentAnomalyRow row : result.records()) {
                dataBuilder.addRecords(toAnomalyView(row));
            }

            responseObserver.onNext(ListSellerShipmentAnomaliesResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(dataBuilder.build())
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(ListSellerShipmentAnomaliesResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    ListSellerShipmentAnomaliesResponse.newBuilder(),
                    e, "ShippingGrpcServiceImpl", "listSellerShipmentAnomalies");
        }
    }

    @Override
    public void getSellerShipmentAnomalyStats(GetSellerShipmentAnomalyStatsRequest request,
                                              StreamObserver<GetSellerShipmentAnomalyStatsResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            long storeId = parseStoreId(request.getStoreId());
            ShipmentAnomalyService.StatsResult stats = shipmentAnomalyService.getSellerAnomalyStats(storeId);

            GetSellerShipmentAnomalyStatsData.Builder dataBuilder = GetSellerShipmentAnomalyStatsData.newBuilder()
                    .setTotal(String.valueOf(stats.total()))
                    .setHighSeverityCount(String.valueOf(stats.highCount()))
                    .setMediumSeverityCount(String.valueOf(stats.mediumCount()))
                    .setLowSeverityCount(String.valueOf(stats.lowCount()));
            stats.byType().forEach(row -> {
                String typeValue = row.getAnomalyType();
                String label = ShipmentAnomalyType.isValidValue(typeValue)
                        ? ShipmentAnomalyType.fromValue(typeValue).label()
                        : typeValue;
                dataBuilder.addByType(AnomalyTypeCount.newBuilder()
                        .setAnomalyType(typeValue)
                        .setAnomalyTypeLabel(label)
                        .setCount(String.valueOf(row.getCnt()))
                        .build());
            });

            responseObserver.onNext(GetSellerShipmentAnomalyStatsResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(dataBuilder.build())
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(GetSellerShipmentAnomalyStatsResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    GetSellerShipmentAnomalyStatsResponse.newBuilder(),
                    e, "ShippingGrpcServiceImpl", "getSellerShipmentAnomalyStats");
        }
    }

    @Override
    public void acknowledgeShipmentAnomaly(AcknowledgeShipmentAnomalyRequest request,
                                           StreamObserver<AcknowledgeShipmentAnomalyResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            long storeId = parseStoreId(request.getStoreId());
            long shipmentId = parseLong(request.getShipmentId(), "shipmentId");

            ShipmentAnomalyAcknowledgmentEntity ack = shipmentAnomalyService.acknowledgeAnomaly(
                    storeId, shipmentId, request.getAnomalyType());

            AcknowledgeShipmentAnomalyData data = AcknowledgeShipmentAnomalyData.newBuilder()
                    .setAckId(String.valueOf(ack.getId()))
                    .setActedAt(DateTimeUtils.formatIso8601(ack.getActedAt()))
                    .build();
            responseObserver.onNext(AcknowledgeShipmentAnomalyResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(AcknowledgeShipmentAnomalyResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    AcknowledgeShipmentAnomalyResponse.newBuilder(),
                    e, "ShippingGrpcServiceImpl", "acknowledgeShipmentAnomaly");
        }
    }

    @Override
    public void ignoreShipmentAnomaly(IgnoreShipmentAnomalyRequest request,
                                      StreamObserver<IgnoreShipmentAnomalyResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.SELLER, AccountType.SELLER_AND_BUYER);

            long storeId = parseStoreId(request.getStoreId());
            long shipmentId = parseLong(request.getShipmentId(), "shipmentId");

            ShipmentAnomalyAcknowledgmentEntity ack = shipmentAnomalyService.ignoreAnomaly(
                    storeId, shipmentId, request.getAnomalyType());

            AcknowledgeShipmentAnomalyData data = AcknowledgeShipmentAnomalyData.newBuilder()
                    .setAckId(String.valueOf(ack.getId()))
                    .setActedAt(DateTimeUtils.formatIso8601(ack.getActedAt()))
                    .build();
            responseObserver.onNext(IgnoreShipmentAnomalyResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(IgnoreShipmentAnomalyResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    IgnoreShipmentAnomalyResponse.newBuilder(),
                    e, "ShippingGrpcServiceImpl", "ignoreShipmentAnomaly");
        }
    }

    @Override
    public void listShipmentAnomaliesForAdmin(ListShipmentAnomaliesForAdminRequest request,
                                              StreamObserver<ListShipmentAnomaliesForAdminResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            Long storeId = (request.getStoreId() == null || request.getStoreId().isBlank())
                    ? null
                    : parseLong(request.getStoreId(), "store_id");
            int page = parseIntDefault(request.getPage(), 1);
            int pageSize = parseIntDefault(request.getPageSize(), 20);

            ShipmentAnomalyService.PageResult<ShipmentAnomalyRow> result =
                    shipmentAnomalyService.listAdminAnomalies(
                            storeId, request.getAnomalyType(), request.getSeverity(),
                            request.getShowAcknowledged(), page, pageSize);

            ListShipmentAnomaliesForAdminData.Builder dataBuilder = ListShipmentAnomaliesForAdminData.newBuilder()
                    .setTotal(String.valueOf(result.total()))
                    .setPage(String.valueOf(result.page()))
                    .setPageSize(String.valueOf(result.pageSize()));
            for (ShipmentAnomalyRow row : result.records()) {
                dataBuilder.addRecords(toAdminAnomalyView(row));
            }

            responseObserver.onNext(ListShipmentAnomaliesForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(dataBuilder.build())
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(ListShipmentAnomaliesForAdminResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    ListShipmentAnomaliesForAdminResponse.newBuilder(),
                    e, "ShippingGrpcServiceImpl", "listShipmentAnomaliesForAdmin");
        }
    }

    @Override
    public void acknowledgeShipmentAnomalyForAdmin(AcknowledgeShipmentAnomalyForAdminRequest request,
                                                   StreamObserver<AcknowledgeShipmentAnomalyResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            long adminAccountId = requireAdminAccountId(request.getAccountId());
            long shipmentId = parseLong(request.getShipmentId(), "shipmentId");

            ShipmentAnomalyAcknowledgmentEntity ack = shipmentAnomalyService.acknowledgeAnomalyForAdmin(
                    adminAccountId, shipmentId, request.getAnomalyType());

            AcknowledgeShipmentAnomalyData data = AcknowledgeShipmentAnomalyData.newBuilder()
                    .setAckId(String.valueOf(ack.getId()))
                    .setActedAt(DateTimeUtils.formatIso8601(ack.getActedAt()))
                    .build();
            responseObserver.onNext(AcknowledgeShipmentAnomalyResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(AcknowledgeShipmentAnomalyResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    AcknowledgeShipmentAnomalyResponse.newBuilder(),
                    e, "ShippingGrpcServiceImpl", "acknowledgeShipmentAnomalyForAdmin");
        }
    }

    @Override
    public void ignoreShipmentAnomalyForAdmin(IgnoreShipmentAnomalyForAdminRequest request,
                                              StreamObserver<IgnoreShipmentAnomalyResponse> responseObserver) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);

            long adminAccountId = requireAdminAccountId(request.getAccountId());
            long shipmentId = parseLong(request.getShipmentId(), "shipmentId");

            ShipmentAnomalyAcknowledgmentEntity ack = shipmentAnomalyService.ignoreAnomalyForAdmin(
                    adminAccountId, shipmentId, request.getAnomalyType());

            AcknowledgeShipmentAnomalyData data = AcknowledgeShipmentAnomalyData.newBuilder()
                    .setAckId(String.valueOf(ack.getId()))
                    .setActedAt(DateTimeUtils.formatIso8601(ack.getActedAt()))
                    .build();
            responseObserver.onNext(IgnoreShipmentAnomalyResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            responseObserver.onCompleted();
        } catch (ShippingBusinessException e) {
            responseObserver.onNext(IgnoreShipmentAnomalyResponse.newBuilder()
                    .setCode(e.code())
                    .setMessage(e.getMessage())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(responseObserver,
                    IgnoreShipmentAnomalyResponse.newBuilder(),
                    e, "ShippingGrpcServiceImpl", "ignoreShipmentAnomalyForAdmin");
        }
    }

    private static long requireAdminAccountId(long accountId) {
        if (accountId <= 0) {
            throw new ShippingBusinessException(GrpcResponseCode.INVALID_PARAM, "accountId 為必填");
        }
        return accountId;
    }

    private ShipmentAnomalyView toAnomalyView(ShipmentAnomalyRow row) {
        String typeValue = row.getAnomalyType();
        String typeLabel = ShipmentAnomalyType.isValidValue(typeValue)
                ? ShipmentAnomalyType.fromValue(typeValue).label()
                : typeValue;
        return ShipmentAnomalyView.newBuilder()
                .setShipmentId(row.getShipmentId() == null ? "" : String.valueOf(row.getShipmentId()))
                .setOrderNo(nullToEmpty(row.getOrderNo()))
                .setAnomalyType(nullToEmpty(typeValue))
                .setAnomalyTypeLabel(typeLabel)
                .setSeverity(nullToEmpty(row.getSeverity()))
                .setShippingMethod(nullToEmpty(row.getShippingMethod()))
                .setShipmentStatus(nullToEmpty(row.getShipmentStatus()))
                .setDetectedAt(row.getDetectedAt() == null ? "" : DateTimeUtils.formatIso8601(row.getDetectedAt()))
                .setAckAction(nullToEmpty(row.getAckAction()))
                .setActedAt(row.getActedAt() == null ? "" : DateTimeUtils.formatIso8601(row.getActedAt()))
                .build();
    }

    private AdminShipmentAnomalyView toAdminAnomalyView(ShipmentAnomalyRow row) {
        String typeValue = row.getAnomalyType();
        String typeLabel = ShipmentAnomalyType.isValidValue(typeValue)
                ? ShipmentAnomalyType.fromValue(typeValue).label()
                : typeValue;
        return AdminShipmentAnomalyView.newBuilder()
                .setShipmentId(row.getShipmentId() == null ? "" : String.valueOf(row.getShipmentId()))
                .setOrderNo(nullToEmpty(row.getOrderNo()))
                .setStoreId(row.getStoreId() == null ? "" : String.valueOf(row.getStoreId()))
                .setAnomalyType(nullToEmpty(typeValue))
                .setAnomalyTypeLabel(typeLabel)
                .setSeverity(nullToEmpty(row.getSeverity()))
                .setShippingMethod(nullToEmpty(row.getShippingMethod()))
                .setShipmentStatus(nullToEmpty(row.getShipmentStatus()))
                .setDetectedAt(row.getDetectedAt() == null ? "" : DateTimeUtils.formatIso8601(row.getDetectedAt()))
                .setAckAction(nullToEmpty(row.getAckAction()))
                .setActedAt(row.getActedAt() == null ? "" : DateTimeUtils.formatIso8601(row.getActedAt()))
                .setSellerAckAction(nullToEmpty(row.getSellerAckAction()))
                .setSellerActedAt(row.getSellerActedAt() == null ? "" : DateTimeUtils.formatIso8601(row.getSellerActedAt()))
                .build();
    }

    private static long parseStoreId(String raw) {
        if (raw == null || raw.isBlank()) {
            throw new ShippingBusinessException(GrpcResponseCode.INVALID_PARAM, "store_id 為必填");
        }
        try {
            return Long.parseLong(raw);
        } catch (NumberFormatException e) {
            throw new ShippingBusinessException(GrpcResponseCode.INVALID_PARAM, "store_id 格式錯誤");
        }
    }

    private static long parseLong(String raw, String field) {
        if (raw == null || raw.isBlank()) {
            throw new ShippingBusinessException(GrpcResponseCode.INVALID_PARAM, field + " 為必填");
        }
        try {
            return Long.parseLong(raw);
        } catch (NumberFormatException e) {
            throw new ShippingBusinessException(GrpcResponseCode.INVALID_PARAM, field + " 格式錯誤");
        }
    }

    private static int parseIntDefault(String raw, int fallback) {
        if (raw == null || raw.isBlank()) {
            return fallback;
        }
        try {
            return Integer.parseInt(raw);
        } catch (NumberFormatException e) {
            return fallback;
        }
    }

    // ==================== Helper ====================

    private String nullToEmpty(String value) {
        return value != null ? value : "";
    }

    // ==================== Uber 即時報價（Stage 8B W2-2）====================

    /**
     * 取得（或沿用）一筆 Uber 報價。
     *
     * <h2>🔴 任何失敗都回 code=SUCCESS ＋ available=false，不回錯誤碼</h2>
     * 呼叫端是結帳頁的「可用配送選項」計算。Uber 算不出價<b>不是</b>結帳失敗——
     * 買家還有黑貓與超商可選。回非零 code 會讓 order-service 那一側把整個
     * 選項計算當成失敗，症狀是<b>三個配送方式一起消失</b>。
     * ⚠️ 真正的例外（DB 掛了、NPE）仍然往上拋成 INTERNAL，那是另一回事。
     */
    @Override
    public void getUberQuoteForCheckout(GetUberQuoteForCheckoutRequest request,
                                        StreamObserver<GetUberQuoteForCheckoutResponse> responseObserver) {
        try {
            UberQuoteService.Outcome outcome = uberQuoteService.quote(toAsk(
                    request.getBuyerId(), request.getSellerStoreId(), request.getDeliveryTemp(),
                    request.getDropoff(), request.getItemsList()));
            responseObserver.onNext(GetUberQuoteForCheckoutResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("success")
                    .setData(toQuoteData(outcome))
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            log.error("ShippingGrpcServiceImpl.getUberQuoteForCheckout.error", "Uber 報價失敗", e);
            responseObserver.onError(io.grpc.Status.INTERNAL
                    .withDescription("Uber 報價失敗").asRuntimeException());
        }
    }

    /**
     * confirm 前重驗（{@code accept=false}）／capture 後確認（{@code accept=true}）。
     *
     * <p>🔴 {@code accept=true} 而 CAS 未命中 ⇒ 回 {@code valid=false} ＋
     * {@code QUOTE_NOT_ACCEPTABLE}。呼叫端<b>必須</b>當成失敗——那代表這筆報價
     * 已經被換手／過期／被接受過，而錢可能已經收了（
     * 「{@code CHECKOUT_ACCEPTED} 超過 30 分未 {@code CONSUMED} ＝ 收了錢沒建單」
     * 講的就是這條軸）。
     */
    @Override
    public void validateUberQuoteForCheckout(ValidateUberQuoteForCheckoutRequest request,
                                             StreamObserver<ValidateUberQuoteForCheckoutResponse> responseObserver) {
        try {
            UberQuoteService.Ask ask = toAsk(
                    request.getBuyerId(), request.getSellerStoreId(), request.getDeliveryTemp(),
                    request.getDropoff(), request.getItemsList());
            UberQuoteService.Revalidation r = uberQuoteService.revalidate(request.getQuoteId(), ask);

            ValidateUberQuoteData.Builder data = ValidateUberQuoteData.newBuilder();
            if (r.unavailableReason() != null) {
                data.setValid(false).setUnavailableReason(r.unavailableReason());
            } else if (r.priceChanged()) {
                // 🔴 不 capture。金額不同必須讓使用者重新確認（INV-U1）。
                data.setValid(false)
                        .setPriceChanged(true)
                        .setQuoteId(nullSafe(r.quote().getQuoteId()))
                        .setFeeAmount(String.valueOf(r.quote().getFeeAmount()))
                        .setExpiresAt(DateTimeUtils.formatIso8601(r.quote().getExpiresAt()));
            } else if (request.getAccept() && !uberQuoteService.accept(
                    r.quote().getQuoteId(), request.getOrderNo())) {
                data.setValid(false).setUnavailableReason("QUOTE_NOT_ACCEPTABLE");
            } else {
                data.setValid(true)
                        .setQuoteId(nullSafe(r.quote().getQuoteId()))
                        .setFeeAmount(String.valueOf(r.quote().getFeeAmount()))
                        .setExpiresAt(DateTimeUtils.formatIso8601(r.quote().getExpiresAt()));
            }
            responseObserver.onNext(ValidateUberQuoteForCheckoutResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage("success")
                    .setData(data.build())
                    .build());
            responseObserver.onCompleted();
        } catch (Exception e) {
            log.error("ShippingGrpcServiceImpl.validateUberQuoteForCheckout.error", "Uber 報價重驗失敗", e);
            responseObserver.onError(io.grpc.Status.INTERNAL
                    .withDescription("Uber 報價重驗失敗").asRuntimeException());
        }
    }

    private static UberQuoteService.Ask toAsk(long buyerId, long sellerStoreId, String deliveryTemp,
                                              UberQuoteDropoff dropoff, java.util.List<UberQuoteItem> items) {
        java.util.List<UberPackingPlanner.Item> planned = new java.util.ArrayList<>();
        for (UberQuoteItem i : items) {
            planned.add(new UberPackingPlanner.Item(
                    i.getSkuId(),
                    i.getName(),
                    ProtoConversionUtils.parseIntRequired(i.getQuantity(), "quantity"),
                    ProtoConversionUtils.parseIntRequired(i.getLengthCm(), "lengthCm"),
                    ProtoConversionUtils.parseIntRequired(i.getWidthCm(), "widthCm"),
                    ProtoConversionUtils.parseIntRequired(i.getHeightCm(), "heightCm"),
                    ProtoConversionUtils.parseIntRequired(i.getWeightGram(), "weightGram"),
                    ProtoConversionUtils.parseLongRequired(i.getUnitPrice(), "unitPrice")));
        }
        return new UberQuoteService.Ask(buyerId, sellerStoreId, deliveryTemp,
                dropoff.getReceiverName(), dropoff.getReceiverPhone(),
                dropoff.getCounty(), dropoff.getDistrict(), dropoff.getDetailAddress(),
                planned);
    }

    private static UberQuoteData toQuoteData(UberQuoteService.Outcome outcome) {
        UberQuoteData.Builder b = UberQuoteData.newBuilder();
        if (!outcome.available()) {
            // 🔴 不可用時 quote_id／fee 一律留空：用「fee=0」表示不可用會與
            //    「0 元運費」（平台補貼）逐位元無法區分。
            return b.setAvailable(false)
                    .setUnavailableReason(nullSafe(outcome.unavailableReason()))
                    .build();
        }
        UberQuoteEntity q = outcome.quote();
        return b.setAvailable(true)
                .setQuoteId(nullSafe(q.getQuoteId()))
                .setFeeAmount(String.valueOf(q.getFeeAmount()))
                .setCurrency(nullSafe(q.getCurrency()))
                .setExpiresAt(DateTimeUtils.formatIso8601(q.getExpiresAt()))
                .build();
    }

}
