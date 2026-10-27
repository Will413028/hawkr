package io.github.will413028.hawkr.shipping.service;

import io.github.will413028.hawkr.common.utils.code.ShippingTemperature;
import io.github.will413028.hawkr.payuni.shipping.GoodsType;

/**
 * PayUni 物流 API 參數轉換器
 * 將內部 Enum / 業務值轉換為 PayUni API 所需的參數格式
 */
public final class PayUniParamMapper {

    private PayUniParamMapper() {
    }

    /**
     * 配送溫層 → PayUni GoodsType
     *
     * @param deliveryTemp ShippingTemperature code（AMBIENT / FROZEN / CHILLED）
     * @return PayUni GoodsType（"1"=常溫, "2"=冷凍, "3"=冷藏）
     */
    public static String toGoodsType(String deliveryTemp) {
        if (deliveryTemp == null || deliveryTemp.isEmpty()) {
            throw new IllegalArgumentException("配送溫層不可為空");
        }
        ShippingTemperature temp = ShippingTemperature.fromValue(deliveryTemp);
        return switch (temp) {
            case AMBIENT -> GoodsType.NORMAL.code();
            case FROZEN -> GoodsType.FROZEN.code();
            case CHILLED -> GoodsType.REFRIGERATED.code();
        };
    }

    /**
     * 材積和 → PayUni Spec（宅配規格代碼）
     *
     * @param dimensionSum 材積和（長+寬+高 cm）
     * @param goodsType    PayUni GoodsType（"1"=常溫, "2"=冷凍, "3"=冷藏）
     * @return PayUni Spec（"1"=60cm, "2"=90cm, "3"=120cm, "4"=150cm 僅常溫）
     */
    public static String toSpec(int dimensionSum, String goodsType) {
        if (dimensionSum <= 60) return "1";
        if (dimensionSum <= 90) return "2";
        if (dimensionSum <= 120) return "3";
        // 150 cm 僅限常溫（goodsType "1"）
        if (dimensionSum <= 150 && GoodsType.NORMAL.code().equals(goodsType)) return "4";
        throw new IllegalArgumentException(
                "不支援的材積規格: dimensionSum=" + dimensionSum + ", goodsType=" + goodsType);
    }
}
