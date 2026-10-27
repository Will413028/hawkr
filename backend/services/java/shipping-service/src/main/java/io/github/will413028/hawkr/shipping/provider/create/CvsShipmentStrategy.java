package io.github.will413028.hawkr.shipping.provider.create;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.validation.InputValidator;
import io.github.will413028.hawkr.common.utils.validation.PayUniShippingConstraints;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.shipping.exception.ProviderCreateFailedException;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.order.grpc.OrderShippingInfo;
import io.github.will413028.hawkr.payuni.shipping.LgsType;
import io.github.will413028.hawkr.payuni.shipping.ServiceType;
import io.github.will413028.hawkr.payuni.shipping.ShipType;
import io.github.will413028.hawkr.payuni.shipping.ShippingClient;
import io.github.will413028.hawkr.payuni.shipping.request.CreateCvsShipmentRequest;
import io.github.will413028.hawkr.payuni.shipping.response.CreateCvsShipmentResponse;
import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.service.PayUniParamMapper;
import org.springframework.stereotype.Component;

import java.time.LocalDate;
import java.time.format.DateTimeFormatter;

/**
 * 超商出貨策略（7-ELEVEN C2C 交貨便）
 *
 * 呼叫 PayUni 超商物流單 API 建立物流單，取得 ShipTradeNo。
 * 寄件人資訊來自 seller_shipment_settings（各配送方式獨立設定）。
 */
@Component
public class CvsShipmentStrategy implements ShipmentStrategy {

    private static final ObsLogger log = ObsLogger.forClass(CvsShipmentStrategy.class);
    private static final DateTimeFormatter DATE_FMT = DateTimeFormatter.ofPattern("yyyyMMdd");
    private static final String SERVICE_TYPE = ServiceType.NO_CASH_ON_DELIVERY.code();
    private static final String SHIP_TYPE_SEVEN_ELEVEN = ShipType.SEVEN.code();
    private static final String LGS_TYPE_C2C = LgsType.C2C.code();

    private final ShippingClient shippingClient;

    public CvsShipmentStrategy(ShippingClient shippingClient) {
        this.shippingClient = shippingClient;
    }

    @Override
    public ShipmentEntity createShipmentFromOrder(String orderNo,
                                                   OrderShippingInfo orderInfo,
                                                   SellerShipmentSettingEntity onboardingSetting) {
        Long shipmentId = TsidUtil.generate();
        String shipmentNo = generateShipmentNo();
        String goodsType = PayUniParamMapper.toGoodsType(orderInfo.getDeliveryTemp());
        // 商品名稱為系統背景填寫（訂單商品撈取），送 PayUni 前依超商約束截斷（顯示長度上限 500），避免超長建單失敗（06 §2.5）
        String goodsName = InputValidator.truncateByDisplayLength(
                orderInfo.getProductName(), PayUniShippingConstraints.CVS_GOODS_NAME_MAX);

        log.info("CvsShipmentStrategy.createShipment.start",
                "超商物流單建立",
                "orderNo", orderNo);

        // 寄件人資訊來自 seller_shipment_settings（物流設定獨立化）
        String senderName = onboardingSetting.getSenderName();
        String senderPhone = onboardingSetting.getSenderPhone();
        String merId = orderInfo.getMerId();

        // 組裝 PayUni 超商物流單請求（MerTradeNo 統一使用 orderNo）
        CreateCvsShipmentRequest payUniReq = new CreateCvsShipmentRequest(
                orderNo,
                SHIP_TYPE_SEVEN_ELEVEN,
                LGS_TYPE_C2C,
                goodsType,
                SERVICE_TYPE,
                senderName,
                senderPhone,
                orderInfo.getRecipientName(),
                orderInfo.getRecipientPhone(),
                orderInfo.getCvsStoreId(),
                orderInfo.getTotalActualAmount(),
                goodsName,
                onboardingSetting.getReturnStoreCode()
        );

        // 呼叫 PayUni API 建立超商物流單（收斂規範 §8.1：同步 retry 最多一次）
        CreateCvsShipmentResponse payUniResp = shippingClient.createCvsShipment(merId, payUniReq);
        if (!payUniResp.isSuccess()) {
            log.warn("CvsShipmentStrategy.createShipment.retry",
                    "PayUni 超商建單首次失敗，retry: " + payUniResp.apiResponse().message());
            payUniResp = shippingClient.createCvsShipment(merId, payUniReq);
        }

        if (!payUniResp.isSuccess()) {
            String providerStatus = payUniResp.apiResponse().status();
            String providerMessage = payUniResp.apiResponse().message();
            log.error("CvsShipmentStrategy.createShipment.error",
                    "PayUni 超商建單 retry 後仍失敗: " + providerMessage);
            throw new ProviderCreateFailedException(
                    GrpcResponseCode.SHIPMENT_PROVIDER_CREATE_FAILED,
                    providerStatus, providerMessage);
        }

        log.info("CvsShipmentStrategy.createShipment.success",
                "PayUni 超商建單成功，shipTradeNo=" + payUniResp.shipTradeNo());

        // 組裝 Entity
        ShipmentEntity entity = new ShipmentEntity();
        entity.setId(shipmentId);
        entity.setShipmentNo(shipmentNo);
        entity.setOrderNo(orderNo);
        entity.setStoreId(ProtoConversionUtils.parseLongRequired(orderInfo.getStoreId(), "store_id"));
        entity.setBuyerId(ProtoConversionUtils.parseLongRequired(orderInfo.getBuyerId(), "buyerId"));
        entity.setShippingMethod(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value());
        entity.setMerId(merId);
        entity.setShipTradeNo(payUniResp.shipTradeNo());
        entity.setStatus(ShippingStatus.UNPROCESSED.value());
        entity.setShipType(SHIP_TYPE_SEVEN_ELEVEN);
        entity.setLgsType(LGS_TYPE_C2C);
        entity.setGoodsType(goodsType);
        entity.setServiceType(SERVICE_TYPE);
        entity.setTradeAmt(ProtoConversionUtils.parseLongRequired(orderInfo.getTotalActualAmount(), "totalActualAmount"));
        entity.setSenderName(senderName);
        entity.setSenderPhone(senderPhone);
        entity.setRecipientName(orderInfo.getRecipientName());
        entity.setRecipientPhone(orderInfo.getRecipientPhone());
        entity.setCvsStoreCode(orderInfo.getCvsStoreId());
        entity.setStoreName(orderInfo.getCvsStoreName());
        entity.setProdDesc(orderInfo.getProductName());

        // §5.9 賣家處理期限：店到店 3 自然日（72 小時）
        entity.setProcessingDeadline(DateTimeUtils.nowUtc().plusHours(72));

        return entity;
    }

    @Override
    public String getShippingMethod() {
        return ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value();
    }

    private String generateShipmentNo() {
        long seq = TsidUtil.generate() % 1_000_000_000L;
        return "S" + String.format("%09d", seq);
    }
}
