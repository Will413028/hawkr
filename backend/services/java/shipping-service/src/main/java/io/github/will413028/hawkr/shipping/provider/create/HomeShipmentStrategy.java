package io.github.will413028.hawkr.shipping.provider.create;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.common.utils.validation.InputValidator;
import io.github.will413028.hawkr.common.utils.validation.PayUniShippingConstraints;
import io.github.will413028.hawkr.shipping.exception.ProviderCreateFailedException;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.order.grpc.OrderShippingInfo;
import io.github.will413028.hawkr.payuni.shipping.DeliveryTimeTag;
import io.github.will413028.hawkr.payuni.shipping.ServiceType;
import io.github.will413028.hawkr.payuni.shipping.ShippingClient;
import io.github.will413028.hawkr.payuni.shipping.request.CreateHomeShipmentRequest;
import io.github.will413028.hawkr.payuni.shipping.response.CreateHomeShipmentResponse;
import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.service.HolidayService;
import io.github.will413028.hawkr.shipping.service.PayUniParamMapper;
import io.github.will413028.hawkr.shipping.util.AddressNormalizer;
import org.springframework.stereotype.Component;

import java.time.LocalDate;
import java.time.LocalDateTime;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;

/**
 * 宅配出貨策略
 *
 * 呼叫 PayUni 宅配物流單 API 建立物流單（僅建單，不做 PDF）。
 * 寄件人資訊來自 seller_shipment_settings（各配送方式獨立設定）。
 * PDF 生成移至 ShippingService.generateHomeLabelPdf（賣家安排出貨時觸發）。
 */
@Component
public class HomeShipmentStrategy implements ShipmentStrategy {

    private static final ObsLogger log = ObsLogger.forClass(HomeShipmentStrategy.class);
    private static final DateTimeFormatter DATE_FMT = DateTimeFormatter.ofPattern("yyyyMMdd");
    private static final String SERVICE_TYPE = ServiceType.NO_CASH_ON_DELIVERY.code();
    private static final String DEFAULT_DELIVERY_TIME_TAG = DeliveryTimeTag.ANY.code();

    private final ShippingClient shippingClient;
    private final HolidayService holidayService;

    public HomeShipmentStrategy(ShippingClient shippingClient, HolidayService holidayService) {
        this.shippingClient = shippingClient;
        this.holidayService = holidayService;
    }

    @Override
    public ShipmentEntity createShipmentFromOrder(String orderNo,
                                                   OrderShippingInfo orderInfo,
                                                   SellerShipmentSettingEntity onboardingSetting) {
        Long shipmentId = TsidUtil.generate();
        String shipmentNo = generateShipmentNo();
        String goodsType = PayUniParamMapper.toGoodsType(orderInfo.getDeliveryTemp());
        String deliveryTimeTag = DEFAULT_DELIVERY_TIME_TAG;

        // 寄件人資訊來自 seller_shipment_settings（物流設定獨立化）
        String senderName = onboardingSetting.getSenderName();
        String senderPhone = onboardingSetting.getSenderPhone();
        String senderCity = onboardingSetting.getSenderCity();
        String senderDistrict = onboardingSetting.getSenderDistrict();
        String senderAddressDetail = onboardingSetting.getSenderAddressDetail();
        String senderPostalCode = onboardingSetting.getSenderPostalCode();
        String senderAddress = senderCity + senderDistrict
                + AddressNormalizer.normalizeAddressDetail(senderCity, senderDistrict, senderAddressDetail);

        // merId 來自訂單快照
        String merId = orderInfo.getMerId();

        // 收件地址（Order proto 提供合併地址）；買家自由輸入，去里鄰後才符合 PayUni 格式
        String receiverAddress = AddressNormalizer.normalizeFullAddress(orderInfo.getShippingAddress());
        String receiverZipCode = extractZipCode(receiverAddress);

        // 商品名稱為系統背景填寫（訂單商品撈取），送 PayUni 前依黑貓約束截斷（顯示長度上限 20），避免超長建單失敗（06 §2.5）
        String goodsName = InputValidator.truncateByDisplayLength(
                orderInfo.getProductName(), PayUniShippingConstraints.HOME_GOODS_NAME_MAX);

        // 組裝 PayUni 宅配物流單請求（MerTradeNo 統一使用 orderNo）
        CreateHomeShipmentRequest payUniReq = new CreateHomeShipmentRequest(
                orderNo,
                goodsType,
                senderName,
                senderPhone,
                senderPostalCode,
                senderAddress,
                orderInfo.getRecipientName(),
                orderInfo.getRecipientPhone(),
                receiverZipCode,
                receiverAddress,
                deliveryTimeTag,
                orderInfo.getTotalActualAmount(),
                goodsName
        );

        // 呼叫 PayUni API 建立宅配單（僅建單，不做 PDF；收斂規範 §8.1：同步 retry 最多一次）
        CreateHomeShipmentResponse payUniResp = shippingClient.createHomeShipment(merId, payUniReq);
        if (!payUniResp.isSuccess()) {
            log.warn("HomeShipmentStrategy.createShipment.retry",
                    "PayUni 宅配建單首次失敗，retry: " + payUniResp.apiResponse().message());
            payUniResp = shippingClient.createHomeShipment(merId, payUniReq);
        }

        if (!payUniResp.isSuccess()) {
            String providerStatus = payUniResp.apiResponse().status();
            String providerMessage = payUniResp.apiResponse().message();
            log.error("HomeShipmentStrategy.createShipment.error",
                    "PayUni 宅配建單 retry 後仍失敗: " + providerMessage);
            throw new ProviderCreateFailedException(
                    GrpcResponseCode.SHIPMENT_PROVIDER_CREATE_FAILED,
                    providerStatus, providerMessage);
        }

        log.info("HomeShipmentStrategy.createShipment.success",
                "PayUni 宅配建單成功，shipTradeNo=" + payUniResp.shipTradeNo());

        // 計算 Spec 並存入
        int dimensionSum = 0;
        String dimensionSumStr = orderInfo.getDimensionSum();
        if (dimensionSumStr != null && !dimensionSumStr.isEmpty()) {
            dimensionSum = Integer.parseInt(dimensionSumStr);
        }
        String spec = (dimensionSum > 0) ? PayUniParamMapper.toSpec(dimensionSum, goodsType) : null;

        // 組裝 Entity
        ShipmentEntity entity = new ShipmentEntity();
        entity.setId(shipmentId);
        entity.setShipmentNo(shipmentNo);
        entity.setOrderNo(orderNo);
        entity.setStoreId(ProtoConversionUtils.parseLongRequired(orderInfo.getStoreId(), "store_id"));
        entity.setBuyerId(ProtoConversionUtils.parseLongRequired(orderInfo.getBuyerId(), "buyerId"));
        entity.setShippingMethod(ShippingMethod.TAQBIN.value());
        entity.setMerId(merId);
        entity.setShipTradeNo(payUniResp.shipTradeNo());
        entity.setStatus(ShippingStatus.UNPROCESSED.value());
        entity.setServiceType(SERVICE_TYPE);
        entity.setGoodsType(goodsType);
        entity.setSpec(spec);
        entity.setTradeAmt(ProtoConversionUtils.parseLongRequired(orderInfo.getTotalActualAmount(), "totalActualAmount"));
        entity.setSenderName(senderName);
        entity.setSenderPhone(senderPhone);
        entity.setSenderCity(senderCity);
        entity.setSenderDistrict(senderDistrict);
        entity.setSenderAddress(senderAddress);
        entity.setSenderZipCode(senderPostalCode);
        entity.setRecipientName(orderInfo.getRecipientName());
        entity.setRecipientPhone(orderInfo.getRecipientPhone());
        entity.setRecipientAddress(receiverAddress);
        entity.setDeliveryTimeTag(deliveryTimeTag);
        entity.setProdDesc(orderInfo.getProductName());

        // §5.9 賣家處理期限：宅配 3 工作日（台北時間 23:59:59 → 轉 UTC）
        LocalDate createdDateTaipei = DateTimeUtils.todayInTaipei();
        LocalDate deadlineDate = holidayService.nextBusinessDay(createdDateTaipei, 3);
        LocalDateTime deadlineUtc = deadlineDate
                .atTime(23, 59, 59)
                .atZone(DateTimeUtils.PAYUNI_ZONE)
                .withZoneSameInstant(ZoneOffset.UTC)
                .toLocalDateTime();
        entity.setProcessingDeadline(deadlineUtc);

        return entity;
    }

    @Override
    public String getShippingMethod() {
        return ShippingMethod.TAQBIN.value();
    }

    /**
     * 從合併地址擷取郵遞區號（台灣 3 碼數字前綴）
     */
    private String extractZipCode(String address) {
        if (address != null && address.length() >= 3) {
            String prefix = address.substring(0, 3);
            if (prefix.chars().allMatch(Character::isDigit)) {
                return prefix;
            }
        }
        return "";
    }

    private String generateShipmentNo() {
        long seq = TsidUtil.generate() % 1_000_000_000L;
        return "S" + String.format("%09d", seq);
    }
}
