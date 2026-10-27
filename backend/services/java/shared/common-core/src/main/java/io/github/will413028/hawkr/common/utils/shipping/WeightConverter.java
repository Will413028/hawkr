package io.github.will413028.hawkr.common.utils.shipping;

import io.github.will413028.hawkr.common.utils.code.WeightUnit;

/**
 * 重量單位轉換工具
 * 將各種重量單位統一轉換為公克
 */
public final class WeightConverter {

    private WeightConverter() {}

    private static final long KG_TO_GRAMS = 1_000L;
    private static final long CATTY_TO_GRAMS = 600L;

    /**
     * 將重量值轉換為公克
     *
     * @param weight 重量值字串
     * @param unit   重量單位
     * @return 公克數
     */
    public static long convertToGrams(String weight, WeightUnit unit) {
        if (weight == null || weight.isEmpty()) {
            throw new IllegalArgumentException("重量不可為空");
        }
        double parsed;
        try {
            parsed = Double.parseDouble(weight);
        } catch (NumberFormatException e) {
            throw new IllegalArgumentException("無效的重量格式: " + weight, e);
        }
        if (parsed <= 0) {
            throw new IllegalArgumentException("重量必須大於 0");
        }
        // 先乘以換算係數再四捨五入，避免小數被提前截斷（如 0.5 KG 應為 500g）
        double grams = switch (unit) {
            case KG -> parsed * KG_TO_GRAMS;
            case CATTY -> parsed * CATTY_TO_GRAMS;
            case GRAM -> parsed;
        };
        return Math.round(grams);
    }
}
