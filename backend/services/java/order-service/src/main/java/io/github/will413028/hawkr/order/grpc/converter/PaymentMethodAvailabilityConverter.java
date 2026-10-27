package io.github.will413028.hawkr.order.grpc.converter;

import io.github.will413028.hawkr.common.utils.code.helper.PaymentMethodLimitRules;
import io.github.will413028.hawkr.order.grpc.AvailablePaymentMethod;

import java.util.List;
import java.util.stream.Collectors;

/**
 * 依訂單金額算出付款方式可用性並 map 成 proto {@link AvailablePaymentMethod}。
 * <p>
 * 結帳預覽（CheckoutService.preview）與再次付款（OrderGrpcService.getOrder）共用。
 * 規格見 20-結帳模組設計書 §3.4.1。限制值 SSOT 為 common-core
 * {@link PaymentMethodLimitRules}，本 class 僅負責 Java record → proto 的轉換。
 */
public final class PaymentMethodAvailabilityConverter {

    private PaymentMethodAvailabilityConverter() {
    }

    /**
     * 依金額算 availablePaymentMethods。amount==0（零元訂單）回空清單（前端「無需付款」）。
     */
    public static List<AvailablePaymentMethod> resolve(long amount) {
        return PaymentMethodLimitRules.resolveAvailability(amount).stream()
                .map(a -> AvailablePaymentMethod.newBuilder()
                        .setPaymentType(a.method().value())
                        .setEligible(a.eligible())
                        .setIneligibleReason(a.ineligibleReason() == null ? "" : a.ineligibleReason().value())
                        .build())
                .collect(Collectors.toList());
    }
}
