package io.github.will413028.hawkr.common.utils.code.helper;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.PaymentMethodIneligibleReason;

import java.util.ArrayList;
import java.util.EnumMap;
import java.util.List;
import java.util.Map;

/**
 * §8.5 PaymentMethod 金額範圍規則 helper（跨 order + payment 服務共用）。
 * <p>
 * CodeMeta enum 為跨語言公共型別，禁止包含業務輔助邏輯（docs/開發規範/Java 微服務開發規範.md §6）。
 * 本 helper 將 PayUni 各 paymentMethod 的硬性金額上下限集中於此。
 * <p>
 * 設計依據：05-支付服務設計書 §2A.2「付款方式金額範圍政策」。
 * <p>
 * 規則來源：
 * <ul>
 *   <li>CREDIT_CARD：PayUni 業務確認 1~199,999 元。</li>
 *   <li>APPLE_PAY / GOOGLE_PAY：PayUni 官方文件未明列上下限；實際走 PayUni 幕後專屬 API
 *       （`/api/apple` / `/api/google`，AuthType=4 / 5），與信用卡同為卡片授權通道，
 *       保守沿用信用卡 1~199,999 元範圍。</li>
 *   <li>ATM：PayUni 業務確認 15~49,999 元（dev 實測 14 元被拒 ATM02007）。</li>
 *   <li>ZERO_AMOUNT：不適用，呼叫端以 amount==0 路徑跳過範圍檢核。</li>
 * </ul>
 */
public final class PaymentMethodLimitRules {

    /**
     * 訂單建立時尚未選 paymentMethod，使用最寬鬆上限提早攔截「不可能成立」的訂單。
     * 取所有 paymentMethod 上限的最大值。
     */
    public static final long ORDER_MAX_AMOUNT = 199_999L;

    public record AmountRange(long min, long max) {}

    private static final Map<PaymentMethod, AmountRange> LIMITS;

    static {
        EnumMap<PaymentMethod, AmountRange> map = new EnumMap<>(PaymentMethod.class);
        map.put(PaymentMethod.CREDIT_CARD, new AmountRange(1L, 199_999L));
        map.put(PaymentMethod.APPLE_PAY, new AmountRange(1L, 199_999L));
        map.put(PaymentMethod.GOOGLE_PAY, new AmountRange(1L, 199_999L));
        map.put(PaymentMethod.ATM, new AmountRange(15L, 49_999L));
        LIMITS = Map.copyOf(map);
    }

    private PaymentMethodLimitRules() {
    }

    public static AmountRange limits(PaymentMethod method) {
        AmountRange range = LIMITS.get(method);
        if (range == null) {
            throw new IllegalArgumentException("PaymentMethod 無對應金額範圍規則：" + method);
        }
        return range;
    }

    public static boolean isWithinRange(PaymentMethod method, long amount) {
        AmountRange range = limits(method);
        return amount >= range.min() && amount <= range.max();
    }

    /**
     * 不在範圍內 → 拋 IllegalStateException 並夾帶 {@link GrpcResponseCode#PAYMENT_METHOD_AMOUNT_OUT_OF_RANGE}
     * 所需的 4 個格式化參數，呼叫端以對應的 BusinessException 包裹後丟出即可。
     * <p>
     * 範例（payment-service）：
     * <pre>
     * try {
     *     PaymentMethodLimitRules.validateOrThrow(method, amount);
     * } catch (PaymentMethodLimitRules.OutOfRangeException e) {
     *     throw new PaymentBusinessException(
     *             GrpcResponseCode.PAYMENT_METHOD_AMOUNT_OUT_OF_RANGE,
     *             e.label(), e.amount(), e.min(), e.max());
     * }
     * </pre>
     */
    public static void validateOrThrow(PaymentMethod method, long amount) {
        AmountRange range = limits(method);
        if (amount < range.min() || amount > range.max()) {
            throw new OutOfRangeException(method, amount, range);
        }
    }

    /**
     * 前端付款方式 gating 用：依訂單金額算出各 PayUni 付款方式的可用性。
     * <p>
     * 規格見 20-結帳模組設計書 §3.4.1。回傳順序固定（信用卡 → Apple Pay → Google Pay → ATM）。
     * <ul>
     *   <li>amount == 0（零元訂單）→ 回空清單（前端顯示「無需付款」，不走 PayUni）。</li>
     *   <li>amount &lt; min → eligible=false, reason=AMOUNT_BELOW_MIN。</li>
     *   <li>amount &gt; max → eligible=false, reason=AMOUNT_ABOVE_MAX。</li>
     *   <li>否則 eligible=true, reason=null。</li>
     * </ul>
     * 本方法為純函式，限制值 SSOT 仍為本 class 的 {@link #LIMITS}；呼叫端（order-service）負責 map 成 proto。
     */
    public static List<Availability> resolveAvailability(long amount) {
        if (amount == 0) {
            return List.of();
        }
        List<Availability> result = new ArrayList<>(CANDIDATE_METHODS.size());
        for (PaymentMethod method : CANDIDATE_METHODS) {
            AmountRange range = limits(method);
            if (amount < range.min()) {
                result.add(new Availability(method, false, PaymentMethodIneligibleReason.AMOUNT_BELOW_MIN));
            } else if (amount > range.max()) {
                result.add(new Availability(method, false, PaymentMethodIneligibleReason.AMOUNT_ABOVE_MAX));
            } else {
                result.add(new Availability(method, true, null));
            }
        }
        return result;
    }

    /** 候選付款方式（固定順序，與前端付款方式呈現順序對齊）。ZERO_AMOUNT 不在此列（零元由空清單表達）。 */
    private static final List<PaymentMethod> CANDIDATE_METHODS = List.of(
            PaymentMethod.CREDIT_CARD, PaymentMethod.APPLE_PAY, PaymentMethod.GOOGLE_PAY, PaymentMethod.ATM);

    /**
     * 單一付款方式對某金額的可用性結果（純資料，不依賴 proto）。
     *
     * @param method           付款方式
     * @param eligible         金額是否落在該方式合法區間
     * @param ineligibleReason eligible=false 時的理由；eligible=true 時為 null
     */
    public record Availability(PaymentMethod method, boolean eligible, PaymentMethodIneligibleReason ineligibleReason) {}

    public static final class OutOfRangeException extends RuntimeException {
        private final PaymentMethod method;
        private final long amount;
        private final AmountRange range;

        private OutOfRangeException(PaymentMethod method, long amount, AmountRange range) {
            super("PaymentMethod=" + method + " amount=" + amount
                    + " min=" + range.min() + " max=" + range.max());
            this.method = method;
            this.amount = amount;
            this.range = range;
        }

        public PaymentMethod method() { return method; }
        public String label() { return method.label(); }
        public long amount() { return amount; }
        public long min() { return range.min(); }
        public long max() { return range.max(); }
    }
}
