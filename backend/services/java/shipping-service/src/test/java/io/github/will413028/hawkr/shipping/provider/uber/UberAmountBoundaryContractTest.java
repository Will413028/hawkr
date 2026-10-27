package io.github.will413028.hawkr.shipping.provider.uber;

import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.QuoteResult;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.QuoteRequest;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.UberWaypoint;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * Uber 金額邊界的 wire 契約：<b>cents ↔ 元</b>。
 *
 * <h2>🔴 這支測試存在的理由</h2>
 * 在它之前，<b>E1 quote 的解析全 repo 零測試</b>——而那正是「金額差 100 倍」這個缺陷
 * 能活下來的機制原因：<b>沒有任何東西會在單位錯掉時變紅</b>。
 *
 * <p>當時的實況是<b>兩個獨立的錯、兩個方向都有</b>：
 * <ul>
 *   <li><b>入站</b>：{@code resp.path("fee").asLong(0L)} 直接取值 ⇒ NT$115 的配送存成 {@code 11500}</li>
 *   <li><b>出站</b>：{@code toMinorUnits} 的 TWD 係數寫死 {@code 1} ⇒ 送出的 price 少 100 倍</li>
 * </ul>
 * ⚠️ 舊 javadoc 逐字寫著「靜默套用 1 會讓金額差 100 倍」——<b>它預言了症狀，只是把係數寫反了</b>。
 *
 * <h2>fixture 的來源（不是我編的）</h2>
 * 依 {@code .claude/rules/03-delegation-templates.md} §0.3，跨邊界的 fixture 不得手寫。
 * 對端是 Uber 的 HTTP API、我方沒有它的原始碼 ⇒ 依據是<b>官方文件逐字</b>：
 * <ul>
 *   <li>官方 spec：「Value in cents ( ¹/₁₀₀ of currency unit )」，換算例 {@code $10.99 => 1099}</li>
 *   <li>台灣業務指南 p.37 逐字「乘以 100」；p.108 refund 實例 {@code TWD 7500 ＝ NT$75}</li>
 *   <li>66 號設計書 2026-09-14 Step 5P-0：「本列原寫『TWD 係數 1』，那是錯的」</li>
 * </ul>
 * 🔴 <b>既有的 {@code 115}／{@code 145} fixture 對這件事零鑑別力</b>：那是 order-service
 * 手寫的「shipping gRPC 回應」mock，不是 Uber HTTP 回應 ⇒ 它只證明「大家都以為是元」。
 *
 * <h2>邊界規則（66）</h2>
 * 🔴 DB／domain／MQ payload／對外 API <b>一律元整數</b>；只在本 adapter 邊界換算。
 * <b>Uber 的 minor unit 不得滲透進 domain。</b>
 */
@DisplayName("Uber 金額邊界契約（cents ↔ 元）")
class UberAmountBoundaryContractTest {

    private static final String BASE_URL = "https://api.uber.test";

    @Nested
    @DisplayName("① 換算函式本身")
    class ConversionFunctions {

        @Test
        @DisplayName("出站：元 → cents 是 ×100（官方換算例 $10.99 => 1099 的 TWD 形式）")
        void toMinorUnitsMultipliesByHundred() {
            // 🔴 期望值硬寫字面值，不得由 TWD_MINOR_UNIT_FACTOR 導出——
            //    否則把係數改回 1，這支測試會跟著變成恆真（§0.3 第二種形態）。
            assertThat(UberFieldFormat.toMinorUnits(115L, "TWD")).isEqualTo(11500L);
            assertThat(UberFieldFormat.toMinorUnits(75L, "TWD")).isEqualTo(7500L);
            assertThat(UberFieldFormat.toMinorUnits(0L, "TWD")).isEqualTo(0L);
        }

        @Test
        @DisplayName("入站：cents → 元 是 ÷100（p.108 逐字 TWD 7500 ＝ NT$75）")
        void fromMinorUnitsDividesByHundred() {
            assertThat(UberFieldFormat.fromMinorUnits(11500L, "TWD")).isEqualTo(115L);
            assertThat(UberFieldFormat.fromMinorUnits(7500L, "TWD")).isEqualTo(75L);
            assertThat(UberFieldFormat.fromMinorUnits(0L, "TWD")).isEqualTo(0L);
        }

        @Test
        @DisplayName("round-trip 恆等：元 → cents → 元")
        void roundTripIsIdentity() {
            for (long dollars : new long[] {0L, 1L, 75L, 115L, 999L, 123456L}) {
                assertThat(UberFieldFormat.fromMinorUnits(
                        UberFieldFormat.toMinorUnits(dollars, "TWD"), "TWD"))
                        .as("round-trip %d 元", dollars)
                        .isEqualTo(dollars);
            }
        }

        @Test
        @DisplayName("🔴 入站不整除 ⇒ fail-closed，不截斷也不四捨五入")
        void fromMinorUnitsRejectsNonMultiples() {
            // TWD 沒有小數位 ⇒ 有餘數代表我方對這條 wire 的理解錯了。
            // 靜默截斷會產生「看起來正常、卻對不上 provider 帳單」的收款金額，
            // 而差額要等月結對帳才會發現。
            assertThatThrownBy(() -> UberFieldFormat.fromMinorUnits(11501L, "TWD"))
                    .isInstanceOf(IllegalArgumentException.class)
                    .hasMessageContaining("11501");
            assertThatThrownBy(() -> UberFieldFormat.fromMinorUnits(1L, "TWD"))
                    .isInstanceOf(IllegalArgumentException.class);
        }

        @Test
        @DisplayName("非 TWD 兩個方向都 fail-closed（係數是幣別的性質，不得沿用）")
        void nonTwdIsRejectedBothWays() {
            assertThatThrownBy(() -> UberFieldFormat.toMinorUnits(100L, "USD"))
                    .isInstanceOf(IllegalArgumentException.class);
            assertThatThrownBy(() -> UberFieldFormat.fromMinorUnits(10000L, "USD"))
                    .isInstanceOf(IllegalArgumentException.class);
            assertThatThrownBy(() -> UberFieldFormat.fromMinorUnits(10000L, null))
                    .isInstanceOf(IllegalArgumentException.class);
        }
    }

    @Nested
    @DisplayName("② E1 quote 的實際解析路徑（這條路徑在本測試之前零覆蓋）")
    class QuoteParsing {

        @Test
        @DisplayName("🔴 官方回 cents，QuoteResult.fee 必須是元")
        void quoteFeeIsConvertedToDollars() {
            // 官方 E1 回應形狀（總覽 §2.1 欄位表：id／fee／currency／expires／dropoff_eta）。
            // fee = 11500 cents ＝ NT$115。
            FakeUberTransport transport = new FakeUberTransport().respondWith(200, """
                    {"id":"dqt_test","fee":11500,"currency":"twd",
                     "expires":"2026-09-17T12:00:00Z","dropoff_eta":"2026-09-17T11:30:00Z"}
                    """);

            QuoteResult result = client(transport).createQuote("cus_test", quoteRequest());

            assertThat(result.fee())
                    .as("🔴 fee 必須已換算成元；11500 代表換算沒發生，"
                            + "而這個值會變成向賣家收款的 estimated_cost")
                    .isEqualTo(115L);
        }

        @Test
        @DisplayName("對照組：換一個金額仍成立（防止把 115 寫死當成通過）")
        void conversionIsNotHardcoded() {
            FakeUberTransport transport = new FakeUberTransport().respondWith(200, """
                    {"id":"dqt_test2","fee":7500,"currency":"TWD",
                     "expires":"2026-09-17T12:00:00Z","dropoff_eta":"2026-09-17T11:30:00Z"}
                    """);

            assertThat(client(transport).createQuote("cus_test", quoteRequest()).fee())
                    .isEqualTo(75L);
        }

        @Test
        @DisplayName("🔴 官方回了不整除的 TWD 金額 ⇒ 解析階段就炸，不讓髒值進 domain")
        void nonMultipleFeeFailsClosedAtParse() {
            FakeUberTransport transport = new FakeUberTransport().respondWith(200, """
                    {"id":"dqt_bad","fee":11501,"currency":"TWD",
                     "expires":"2026-09-17T12:00:00Z","dropoff_eta":"2026-09-17T11:30:00Z"}
                    """);

            assertThatThrownBy(() -> client(transport).createQuote("cus_test", quoteRequest()))
                    .isInstanceOf(IllegalArgumentException.class)
                    .hasMessageContaining("11501");
        }
    }

    // =========================================================================
    // helpers
    // =========================================================================

    /** 🔴 sleeper 換成 no-op：退避階梯會讓測試真的睡。 */
    private static UberApiClient client(FakeUberTransport transport) {
        ObjectMapper mapper = new ObjectMapper();
        UberOAuthTokenProvider tokens = new UberOAuthTokenProvider(
                transport, mapper, TestUberAdapters.FIXED_CLOCK,
                "https://auth.uber.test/oauth/v2/token", "cid", "secret", "eats.deliveries");
        return new UberApiClient(transport, tokens, mapper, BASE_URL, millis -> { });
    }

    private static QuoteRequest quoteRequest() {
        return new QuoteRequest(
                "store_test",
                new UberWaypoint("{}", "取件點", "+886912345678", 25.033, 121.565, null),
                new UberWaypoint("{}", "收件人", "+886987654321", 25.041, 121.566, null),
                null);
    }
}
