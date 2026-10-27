package io.github.will413028.hawkr.shipping.provider.uber;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.concurrent.atomic.AtomicReference;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * OAuth token 的集中快取（66 §4.5.1）。
 *
 * <h2>🔴 為什麼快取是硬需求</h2>
 * 官方明載 token 發放速率 <b>100 requests / hour</b>、有效期 <b>30 天</b>
 * （{@code docs/第三方整合/Uber Direct API 總覽.md} §1.1）。
 * 每次請求重新取 token 會撞上 100/hr 上限，而撞上的症狀是「Uber 突然全面 401」
 * ——看起來完全像被停權。
 */
@DisplayName("Uber OAuth token 快取")
class UberOAuthTokenProviderTest {

    private static final String AUTH_URL = "https://auth.uber.test/oauth/v2/token";
    private static final Instant T0 = Instant.parse("2026-09-12T00:00:00Z");
    /** 官方 token 有效期：30 天。 */
    private static final long THIRTY_DAYS_SECONDS = 2_592_000L;

    @Test
    @DisplayName("🔴 連續 5 次取用只打一次 token 端點（沒有快取 ⇒ 5 次 ⇒ 撞 100/hr 上限）")
    void tokenIsFetchedOnceAndReused() {
        FakeUberTransport transport = new FakeUberTransport(THIRTY_DAYS_SECONDS);
        UberOAuthTokenProvider provider = provider(transport, Clock.fixed(T0, ZoneOffset.UTC));

        for (int i = 0; i < 5; i++) {
            assertThat(provider.bearerToken()).isEqualTo("tok_test");
        }

        assertThat(transport.tokenCallCount())
                .as("🔴 每次重取 ＝ 官方 100 req/hr 的上限會被業務流量直接撞破")
                .isEqualTo(1L);
    }

    @Test
    @DisplayName("剩餘壽命跌破 20% ⇒ 換新（陽性對照：沒有它，把快取寫成永不過期也會綠）")
    void tokenIsRefreshedWhenBelowThreshold() {
        FakeUberTransport transport = new FakeUberTransport(THIRTY_DAYS_SECONDS);
        AtomicReference<Instant> now = new AtomicReference<>(T0);
        UberOAuthTokenProvider provider = provider(transport, movingClock(now));

        provider.bearerToken();
        assertThat(transport.tokenCallCount()).isEqualTo(1L);

        // 還剩 25%（> 20% 門檻）⇒ 不換新
        now.set(T0.plus(Duration.ofSeconds((long) (THIRTY_DAYS_SECONDS * 0.75))));
        provider.bearerToken();
        assertThat(transport.tokenCallCount()).isEqualTo(1L);

        // 剩 10%（< 20% 門檻）⇒ 換新
        now.set(T0.plus(Duration.ofSeconds((long) (THIRTY_DAYS_SECONDS * 0.90))));
        provider.bearerToken();
        assertThat(transport.tokenCallCount()).isEqualTo(2L);
    }

    @Test
    @DisplayName("🔴 刷新失敗不得 fail-open：拿不到新 token 就拋，絕不退回快取中的舊 token")
    void refreshFailureNeverFallsBackToTheCachedToken() {
        // 66 §4.5.1 逐字：「『先用舊 token 試試看』會把一次認證失敗變成 N 次 401，
        // 而 401 對外看起來像我們被停權。」
        FakeUberTransport transport = new FakeUberTransport(THIRTY_DAYS_SECONDS).tokenFailsAfter(1);
        AtomicReference<Instant> now = new AtomicReference<>(T0);
        UberOAuthTokenProvider provider = provider(transport, movingClock(now));

        assertThat(provider.bearerToken()).isEqualTo("tok_test");

        now.set(T0.plus(Duration.ofSeconds((long) (THIRTY_DAYS_SECONDS * 0.95))));
        assertThatThrownBy(provider::bearerToken)
                .isInstanceOf(UberApiException.class)
                .satisfies(e -> assertThat(((UberApiException) e).kind())
                        .isEqualTo(UberFailureKind.AUTH));
    }

    @Test
    @DisplayName("🔴 憑證缺席 ⇒ fail-closed，一個位元組都不送出去（不得帶空字串去打 API）")
    void missingCredentialsNeverHitTheNetwork() {
        FakeUberTransport transport = new FakeUberTransport();
        UberOAuthTokenProvider provider = new UberOAuthTokenProvider(
                transport, new ObjectMapper(), Clock.fixed(T0, ZoneOffset.UTC),
                AUTH_URL, "", "", "eats.deliveries");

        assertThat(provider.credentialsConfigured()).isFalse();
        assertThatThrownBy(provider::bearerToken).isInstanceOf(UberApiException.class);
        assertThat(transport.calls())
                .as("帶空字串上路的後果是一連串 401，對外看起來像我們被停權（66 §4.5.1）")
                .isEmpty();
    }

    @Test
    @DisplayName("token 端點的 deadline 有值（禁止無 deadline 的跨服務呼叫）")
    void tokenCallHasDeadline() {
        FakeUberTransport transport = new FakeUberTransport();
        provider(transport, Clock.fixed(T0, ZoneOffset.UTC)).bearerToken();

        assertThat(transport.calls()).singleElement()
                .satisfies(c -> assertThat(c.deadline()).isEqualTo(Duration.ofSeconds(10)));
    }

    private static UberOAuthTokenProvider provider(FakeUberTransport transport, Clock clock) {
        return new UberOAuthTokenProvider(transport, new ObjectMapper(), clock,
                AUTH_URL, "client-id", "client-secret", "eats.deliveries");
    }

    /** 可推進的時鐘——時間敏感邏輯不得靠 sleep 驗（CLAUDE.md §21）。 */
    private static Clock movingClock(AtomicReference<Instant> now) {
        return new Clock() {
            @Override
            public ZoneOffset getZone() {
                return ZoneOffset.UTC;
            }

            @Override
            public Clock withZone(java.time.ZoneId zone) {
                return this;
            }

            @Override
            public Instant instant() {
                return now.get();
            }
        };
    }
}
