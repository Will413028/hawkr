package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Assessment;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Check;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.CheckOutcome;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Context;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Decision;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Item;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Probe;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.QuoteAssessor;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Status;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;
import org.junit.jupiter.params.provider.NullSource;
import org.junit.jupiter.params.provider.ValueSource;

import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.lang.reflect.Modifier;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.EnumMap;
import java.util.EnumSet;
import java.util.HashSet;
import java.util.List;
import java.util.Set;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.catchThrowableOfType;

/**
 * {@link UberOrderEligibilityGuard} 本身的行為。
 *
 * <h2>三半缺一不可</h2>
 * <ul>
 *   <li><b>陽性</b>：shipping 說可用 ⇒ 放行，六項全 PASSED。少了它，一個「永遠拒絕」的
 *       實作會讓另外兩半全綠——而那正是接線前的狀態。</li>
 *   <li><b>陰性（理由正確）</b>：shipping 說某一項不成立 ⇒ 只有那一項是 blocking。
 *       只斷言「有拒絕」對「拒絕理由指錯項目」零鑑別力，而錯的理由會讓維運去查錯的系統。</li>
 *   <li><b>陰性（不受影響）</b>：非 3003 完全不受影響。少了這一半，一個「對所有 method
 *       都拒絕」的實作會讓前兩半全綠——那個 bug 會擋掉平台上<b>每一張訂單</b>。</li>
 * </ul>
 *
 * <h2>🔴 fake 的鑑別力來自它拒絕什麼</h2>
 * {@link FixedAssessor} 只接受<b>唯一一個</b>預期的 probe，其餘一律拋。
 * 一個「照單全收」的 fake 會讓「probe 組錯了」（買家與賣家 id 對調、溫層填錯、
 * 地址三欄漏一欄）的實作全部通過。
 */
class UberOrderEligibilityGuardTest {

    /** Uber Direct 的 shipping_method 值，硬寫字面值（不從 ShippingMethod enum 取，那會跟著被測物一起變）。 */
    private static final String UBER = "3003";

    /** 硬寫字面值的 probe——欄位形狀抄自 {@code shipping.proto} 的 UberQuoteDropoff／UberQuoteItem。 */
    private static Probe probe() {
        return Probe.forQuote(
                2623900520L, 2685914682L, "3201",
                "王小明", "0912345678",
                "台北市", "中正區", "測試路1號",
                List.of(new Item(5001L, "蘋果", 2, 10, 12, 8, 1500L, 500L)));
    }

    /**
     * 只對預期的 probe 回答，其餘一律拋。
     *
     * <p>🔴 「其餘一律拋」是本 fake 的全部價值：它把「呼叫端把 probe 組錯了」變成失敗，
     * 而不是變成一個看起來正常的通過。
     */
    private static final class FixedAssessor implements QuoteAssessor {
        private final Probe expected;
        private final Assessment answer;
        private final RuntimeException boom;
        private int calls;

        private FixedAssessor(Probe expected, Assessment answer, RuntimeException boom) {
            this.expected = expected;
            this.answer = answer;
            this.boom = boom;
        }

        static FixedAssessor answering(Assessment answer) {
            return new FixedAssessor(probe(), answer, null);
        }

        static FixedAssessor throwing(RuntimeException boom) {
            return new FixedAssessor(probe(), null, boom);
        }

        @Override
        public Assessment assess(Probe actual) {
            calls++;
            if (!expected.equals(actual)) {
                throw new AssertionError("probe 與預期不符：expected=" + expected + " actual=" + actual);
            }
            if (boom != null) {
                throw boom;
            }
            return answer;
        }
    }

    private static UberOrderEligibilityGuard guardAnswering(Assessment answer) {
        return new UberOrderEligibilityGuard(FixedAssessor.answering(answer));
    }

    // ===== 🔴 陽性：shipping 說可用 ⇒ 放行 =====

    @Test
    @DisplayName("🔴 陽性：報價可用 ⇒ applicable=true、allowed=true、六項全 PASSED、不拋")
    void eligibleQuoteIsAllowed() {
        UberOrderEligibilityGuard guard = guardAnswering(
                Assessment.eligible("dqt_abc123", "115", "TWD", "2026-09-12T08:30:00Z"));

        Decision decision = guard.evaluate(Context.of(UBER, probe()));

        assertThat(decision.applicable()).isTrue();
        assertThat(decision.allowed()).isTrue();
        assertThat(decision.blockingChecks()).isEmpty();
        assertThat(decision.denyReason()).isEmpty();
        assertThat(decision.outcomes())
                .hasSize(Check.values().length)
                .allSatisfy(o -> assertThat(o.status()).isEqualTo(Status.PASSED));

        // 報價本體要帶回呼叫端——結帳頁要的是 quote_id 與運費，不是一個布林
        assertThat(decision.assessment().quoteId()).isEqualTo("dqt_abc123");
        assertThat(decision.assessment().feeAmount()).isEqualTo("115");
        assertThat(decision.assessment().currency()).isEqualTo("TWD");
        assertThat(decision.assessment().expiresAt()).isEqualTo("2026-09-12T08:30:00Z");

        guard.requireEligible(Context.of(UBER, probe())); // 不拋即通過
    }

    @Test
    @DisplayName("🔴 只問一次 shipping——閘門不得在 evaluate 內重複求值（重複＝兩個可能不同的答案）")
    void assessorIsConsultedExactlyOnce() {
        FixedAssessor assessor = FixedAssessor.answering(
                Assessment.eligible("dqt_abc123", "115", "TWD", "2026-09-12T08:30:00Z"));

        new UberOrderEligibilityGuard(assessor).evaluate(Context.of(UBER, probe()));

        assertThat(assessor.calls).isEqualTo(1);
    }

    // ===== 🔴 陰性一：報價不可用／過期 ⇒ 拒絕，且理由指向正確的那一項 =====

    @ParameterizedTest(name = "reason={0} ⇒ 只有 {1} 是 blocking")
    @CsvSource({
            // 理由碼硬寫字面值：它們是跨服務 wire 契約，抄自 shipping 原始碼，
            // 不從被測物的 ProviderReason 常數取（那會跟著被測物一起變）。
            "UBER_DISABLED,              PROVIDER_DISABLED",
            "UBER_METHOD_NOT_ENABLED,    PROVIDER_DISABLED",
            "UBER_STORE_NOT_MAPPED,      SELLER_NOT_ENABLED",
            "QUOTE_FAILED,               QUOTE_MISSING",
            "QUOTE_NOT_ACCEPTABLE,       QUOTE_EXPIRED",
            "UBER_QUOTE_PRICE_CHANGED,   QUOTE_EXPIRED",
            "ADDRESS_UNPARSEABLE,        ADDRESS_UNSERVICEABLE",
            "PACKING_POLICY_MISSING,     DELIVERY_DATA_INCOMPLETE",
            "PACKING_POLICY_INVALID,     DELIVERY_DATA_INCOMPLETE",
            "ITEM_QUANTITY_INVALID,      DELIVERY_DATA_INCOMPLETE",
            "ITEM_OVER_DIMENSION_SUM,    DELIVERY_DATA_INCOMPLETE",
            "ITEM_OVER_LONGEST_SIDE,     DELIVERY_DATA_INCOMPLETE",
            "ITEM_OVER_WEIGHT,           DELIVERY_DATA_INCOMPLETE",
    })
    @DisplayName("🔴 shipping 的每一個理由碼都恰好歸到一項，其餘五項 PASSED（理由指錯項目＝維運查錯系統）")
    void everyProviderReasonBlocksExactlyItsOwnCheck(String reason, String expectedCheck) {
        Decision decision = guardAnswering(Assessment.unavailable(reason))
                .evaluate(Context.of(UBER, probe()));

        assertThat(decision.allowed()).isFalse();
        assertThat(decision.blockingChecks())
                .as("理由 %s 應該只擋 %s", reason, expectedCheck)
                .containsExactly(Check.valueOf(expectedCheck));
        assertThat(decision.providerReason()).isEqualTo(reason);
        assertThat(decision.denyReason()).contains(reason);

        // 其餘五項必須是 PASSED——全部拒絕也會讓 blockingChecks 含正確那一項，
        // 所以上面那一句對「一律全拒」零鑑別力，要靠這一句。
        assertThat(decision.outcomes())
                .filteredOn(o -> o.check() != Check.valueOf(expectedCheck))
                .hasSize(Check.values().length - 1)
                .allSatisfy(o -> assertThat(o.status()).isEqualTo(Status.PASSED));
    }

    @Test
    @DisplayName("報價不可用時 requireEligible 拋 9503 FULFILLMENT_PROVIDER_NOT_ENABLED")
    void requireEligibleThrows9503WhenUnavailable() {
        OrderBusinessException ex = catchThrowableOfType(
                () -> guardAnswering(Assessment.unavailable("UBER_STORE_NOT_MAPPED"))
                        .requireEligible(Context.of(UBER, probe())),
                OrderBusinessException.class);
        assertThat(ex).isNotNull();
        // 硬寫字面值：9503 是跨 repo 契約（gateway bizhttp 已登錄為 403），不從 enum 取
        assertThat(ex.code()).isEqualTo(9503);
    }

    @Test
    @DisplayName("拒絕碼不是 9510（配置缺陷，會讓維運去建一張不該建的 config 列）也不是 4008（吐不出原因）")
    void denyCodeIsNeither9510Nor4008() {
        OrderBusinessException ex = catchThrowableOfType(
                () -> guardAnswering(Assessment.unavailable("QUOTE_FAILED"))
                        .requireEligible(Context.of(UBER, probe())),
                OrderBusinessException.class);
        assertThat(ex).isNotNull();
        assertThat(ex.code()).isNotIn(9510, 4008, 9504);
    }

    // ===== 🔴 取不到證據的三種來源，處置一律相同：六項全拒 =====

    @Test
    @DisplayName("🔴 沒有 probe（methodOnly）⇒ 六項全 DENIED_NO_EVIDENCE，不是靜默放行")
    void missingProbeIsDenied() {
        Decision decision = guardAnswering(
                Assessment.eligible("dqt_x", "1", "TWD", "2026-09-12T08:30:00Z"))
                .evaluate(Context.methodOnly(UBER));

        assertThat(decision.applicable()).isTrue();
        assertThat(decision.allowed()).isFalse();
        assertThat(decision.blockingChecks()).hasSize(Check.values().length);
        assertThat(decision.outcomes())
                .allSatisfy(o -> assertThat(o.status()).isEqualTo(Status.DENIED_NO_EVIDENCE));
        assertThat(decision.assessment()).isNull();
        assertThat(decision.providerReason()).isEmpty();
    }

    @Test
    @DisplayName("🔴 沒有 QuoteAssessor（無參建構子）⇒ 六項全拒——「沒接證據來源」不得等於「通過」")
    void missingAssessorIsDenied() {
        Decision decision = new UberOrderEligibilityGuard().evaluate(Context.of(UBER, probe()));

        assertThat(decision.allowed()).isFalse();
        assertThat(decision.blockingChecks()).hasSize(Check.values().length);
    }

    @Test
    @DisplayName("🔴 assessor 拋例外（shipping 掛了／逾時）⇒ 六項全拒，不是吞掉當可用")
    void assessorFailureIsDenied() {
        Decision decision = new UberOrderEligibilityGuard(
                FixedAssessor.throwing(new IllegalStateException("UNAVAILABLE: io exception")))
                .evaluate(Context.of(UBER, probe()));

        assertThat(decision.allowed()).isFalse();
        assertThat(decision.blockingChecks()).hasSize(Check.values().length);
        assertThat(decision.denyReason()).contains("IllegalStateException");
        // 具名理由碼：回空字串會讓前端只能顯示「不可用」、維運也查不到是哪一段斷了
        assertThat(decision.providerReason()).isEqualTo("UBER_QUOTE_ASSESSOR_UNAVAILABLE");
    }

    @ParameterizedTest(name = "未知理由 {0} ⇒ 六項全拒")
    @ValueSource(strings = {"SOMETHING_NEW_IN_SHIPPING", "", "   "})
    @DisplayName("🔴 不認得的理由碼 ⇒ 六項全拒，絕不挑一項來擋（shipping 新增理由時退化方向是拒絕）")
    void unknownReasonDeniesEveryCheck(String reason) {
        Decision decision = guardAnswering(Assessment.unavailable(reason))
                .evaluate(Context.of(UBER, probe()));

        assertThat(decision.allowed()).isFalse();
        assertThat(decision.blockingChecks()).hasSize(Check.values().length);
        assertThat(decision.outcomes())
                .allSatisfy(o -> assertThat(o.status()).isEqualTo(Status.DENIED_NO_EVIDENCE));
    }

    @Test
    @DisplayName("🔴 assessor 回 null ⇒ 六項全拒（NPE 也是一種 fail-open，不能靠它）")
    void nullAssessmentIsDenied() {
        Decision decision = new UberOrderEligibilityGuard(p -> null)
                .evaluate(Context.of(UBER, probe()));

        assertThat(decision.allowed()).isFalse();
        assertThat(decision.blockingChecks()).hasSize(Check.values().length);
    }

    // ===== 🔴 陰性二：非 3003 完全不受影響 =====

    @ParameterizedTest(name = "shipping_method={0} 不受本閘門影響")
    @ValueSource(strings = {"3001", "3002", "3005", "9999", "", "UBER_INSTANT", "30031", "003"})
    @NullSource
    @DisplayName("非 3003（含 null／值域外／相近字串）：applicable=false、allowed=true、六項全 NOT_APPLICABLE")
    void nonUberIsUntouched(String shippingMethod) {
        // 用「會拒絕的」assessor：非 3003 若誤入判定路徑，這一支就紅
        Decision decision = guardAnswering(Assessment.unavailable("UBER_DISABLED"))
                .evaluate(Context.methodOnly(shippingMethod));

        assertThat(decision.applicable()).isFalse();
        assertThat(decision.allowed()).isTrue();
        assertThat(decision.blockingChecks()).isEmpty();
        assertThat(decision.denyReason()).isEmpty();
        assertThat(decision.outcomes())
                .hasSize(Check.values().length)
                .allSatisfy(o -> assertThat(o.status()).isEqualTo(Status.NOT_APPLICABLE));
    }

    @ParameterizedTest(name = "requireEligible({0}) 不拋")
    @ValueSource(strings = {"3001", "3002", "3005", "9999", ""})
    @NullSource
    @DisplayName("requireEligible 對非 3003 靜默放行——本閘門不兼任配送方式合法性驗證（那是 OrderPolicyResolver 的 9504）")
    void requireEligibleDoesNotThrowForNonUber(String shippingMethod) {
        guardAnswering(Assessment.unavailable("UBER_DISABLED"))
                .requireEligible(Context.methodOnly(shippingMethod)); // 不拋即通過
    }

    @Test
    @DisplayName("🔴 非 3003 一次也不問 shipping——對每張訂單多打一次 gRPC 是 hot path 事故")
    void nonUberNeverConsultsShipping() {
        FixedAssessor assessor = FixedAssessor.answering(
                Assessment.eligible("dqt_x", "1", "TWD", "2026-09-12T08:30:00Z"));

        new UberOrderEligibilityGuard(assessor).evaluate(Context.of("3001", probe()));

        assertThat(assessor.calls).isZero();
    }

    // ===== 對照表本身的結構保證 =====

    @Test
    @DisplayName("🔴 ProviderReason 的每一個常數都被歸到某一項（ASSESSOR_UNAVAILABLE 例外且必須是例外）")
    void everyProviderReasonConstantIsMapped() throws Exception {
        List<String> unmapped = new ArrayList<>();
        int seen = 0;
        for (Field f : UberOrderEligibilityGuard.ProviderReason.class.getDeclaredFields()) {
            if (!Modifier.isStatic(f.getModifiers()) || f.getType() != String.class) {
                continue;
            }
            seen++;
            String value = (String) f.get(null);
            Check owner = UberOrderEligibilityGuard.checkFor(value);
            if ("ASSESSOR_UNAVAILABLE".equals(f.getName())) {
                assertThat(owner)
                        .as("ASSESSOR_UNAVAILABLE 是 order 側的『我問不到』，不得歸到任何一項檢查")
                        .isNull();
                continue;
            }
            if (owner == null) {
                unmapped.add(f.getName());
            }
        }
        assertThat(seen).as("ProviderReason 至少要有 14 個常數，反射抓不到＝這支測試是空的").isGreaterThanOrEqualTo(14);
        assertThat(unmapped)
                .as("新增 ProviderReason 常數時必須同時決定它屬於哪一個 Check，否則它會落進『不認得』分支")
                .isEmpty();
    }

    @Test
    @DisplayName("🔴 一個理由碼不得同時屬於兩項——否則 checkFor 的答案取決於 enum 宣告順序")
    void providerReasonsAreDisjoint() {
        Set<String> seen = new HashSet<>();
        for (Check check : Check.values()) {
            for (String reason : check.providerReasons()) {
                assertThat(seen.add(reason))
                        .as("理由碼 %s 重複出現在多個 Check", reason)
                        .isTrue();
            }
        }
        assertThat(seen).hasSizeGreaterThanOrEqualTo(13);
    }

    @Test
    @DisplayName("每一項都有具體的 evidence 與已接線的證據來源，不是空字串、不是 TODO、不是指向未來")
    void everyCheckCarriesEvidenceAndEvidenceSource() {
        Decision decision = guardAnswering(Assessment.unavailable("UBER_DISABLED"))
                .evaluate(Context.of(UBER, probe()));
        for (CheckOutcome outcome : decision.outcomes()) {
            assertThat(outcome.evidence()).isNotBlank();
            assertThat(outcome.evidence()).doesNotContainIgnoringCase("TODO");
            assertThat(outcome.check().evidenceSource()).isNotBlank();
            assertThat(outcome.check().evidenceSource()).doesNotContainIgnoringCase("TODO");
            // 🔴 接上之後不得留下指向未來的敘述：evidenceSource 講的是「現在誰給的答案」
            assertThat(outcome.check().evidenceSource())
                    .as("%s 的證據來源仍寫成待辦", outcome.check())
                    .doesNotContain("W2 ")
                    .doesNotContain("尚無")
                    .doesNotContain("未實作");
            assertThat(outcome.check().providerReasons()).isNotEmpty();
        }
    }

    // ===== fail-closed 的結構保證 =====

    @Test
    @DisplayName("Status 的 blocking 旗標：兩種 DENIED 都擋，PASSED／NOT_APPLICABLE 不擋")
    void onlyDeniedStatusesBlock() {
        assertThat(Status.DENIED_BY_PROVIDER.blocking()).isTrue();
        assertThat(Status.DENIED_NO_EVIDENCE.blocking()).isTrue();
        assertThat(Status.PASSED.blocking()).isFalse();
        assertThat(Status.NOT_APPLICABLE.blocking()).isFalse();
    }

    @Test
    @DisplayName("🔴 每一個 Check 常數都必須在 evaluate 內被接線——新增常數卻忘了接，這支就紅")
    void everyCheckConstantIsWired() {
        Decision decision = guardAnswering(Assessment.unavailable("UBER_DISABLED"))
                .evaluate(Context.of(UBER, probe()));

        EnumSet<Check> reported = EnumSet.noneOf(Check.class);
        decision.outcomes().forEach(o -> reported.add(o.check()));
        assertThat(reported).containsExactlyElementsOf(Arrays.asList(Check.values()));

        // 🔴 上面那一行對「漏接」零鑑別力——decide() 會補一筆 fail-closed outcome，
        //    所以集合永遠是完整的。真正有鑑別力的是下面這一句。
        assertThat(decision.outcomes())
                .as("Check 常數新增後必須在 evaluate() 內 put 一筆帶具體 evidence 的 outcome")
                .allSatisfy(o -> assertThat(o.evidence())
                        .isNotEqualTo(UberOrderEligibilityGuard.UNWIRED_EVIDENCE));
    }

    @Test
    @DisplayName("🔴 fail-closed 分支：完全沒接線的表送進 decide()，六項仍全部拒絕（不是靜默放行）")
    void unwiredChecksAreDeniedNotPassed() throws Exception {
        // 用反射直接打 decide()——這是這條分支唯一測得到的方式：
        // evaluate() 的每一條路徑都把六項接滿了，走不到 null 分支。
        // 反射在本專案有既有慣例（PaymentMerIdPolicyContractTest.invokeResolve）。
        Method decide = UberOrderEligibilityGuard.class
                .getDeclaredMethod("decide", EnumMap.class, Assessment.class);
        decide.setAccessible(true);

        Decision decision = (Decision) decide.invoke(
                null, new EnumMap<Check, CheckOutcome>(Check.class), null);

        assertThat(decision.applicable()).isTrue();
        assertThat(decision.allowed())
                .as("接線表全空時必須拒絕——若這裡是 true，代表『新增 Check 卻忘了接』會靜默放行")
                .isFalse();
        assertThat(decision.blockingChecks()).hasSize(Check.values().length);
        assertThat(decision.outcomes())
                .allSatisfy(o -> assertThat(o.evidence())
                        .isEqualTo(UberOrderEligibilityGuard.UNWIRED_EVIDENCE));
    }

    @Test
    @DisplayName("Check 常數數量是 6——新增／刪除檢查項必須回到本檔與 §2 的清單表一起更新")
    void checkCountIsPinned() {
        assertThat(Check.values()).hasSize(6);
    }
}
