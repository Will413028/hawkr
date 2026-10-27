package io.github.will413028.hawkr.shipping.config;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.payuni.http.PayUniHttpClient;
import io.github.will413028.hawkr.shipping.audit.ShippingPayUniAuditLogger;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;
import org.springframework.boot.autoconfigure.AutoConfigurations;
import org.springframework.boot.autoconfigure.context.PropertyPlaceholderAutoConfiguration;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * backend#3329：PayUni 憑證未設定時，shipping-service 必須照常啟動。
 *
 * <p>🔴 舊行為已實測坐實：{@code payuni.agent.key} 沒有預設值時，
 * {@code ApplicationContextRunner} 對 {@link PayUniConfig} 回 {@code hasFailed()}，
 * root cause 是 {@code IllegalArgumentException: Could not resolve placeholder
 * 'payuni.agent.key'}。在真實部署裡那是 context refresh 失敗 ⇒ <b>整個服務起不來</b>，
 * 連運費計算（結帳 hot path、完全不碰 PayUni）一起死。
 *
 * <p>期望值一律硬寫字面值（9705），不引用被測物算出來的東西。
 */
class PayUniConfigProvisioningTest {

    private final ApplicationContextRunner runner = new ApplicationContextRunner()
            .withConfiguration(AutoConfigurations.of(PropertyPlaceholderAutoConfiguration.class))
            .withUserConfiguration(TestDeps.class, PayUniConfig.class)
            .withPropertyValues("payuni.api.base-url=https://sandbox-api.payuni.com.tw");

    /**
     * 🔴 本檔的核心斷言：完全沒有 PayUni 憑證時 context 必須起得來。
     *
     * <p>變異驗證：把 {@code @Value("${payuni.agent.key:}")} 的 {@code :} 拿掉
     * ⇒ 本測試轉紅（Could not resolve placeholder）。
     */
    @Test
    void contextStartsWithoutPayUniCredentials() {
        runner.run(ctx -> {
            assertThat(ctx).hasNotFailed();
            assertThat(ctx).hasSingleBean(PayUniHttpClient.class);
        });
    }

    /**
     * 🔴 陰性斷言：起得來 ≠ 放行。憑證未設定時任何一次 PayUni 呼叫都必須拋 9705，
     * 一個位元組都不送出去。
     *
     * <p>這是本次改動最危險的退化方向：讓 context 起來之後，如果 client 照常帶著
     * 空金鑰去打 PayUni，症狀會是對方回一串看不懂的簽章錯誤，而且是<b>真的發出去的請求</b>。
     */
    @Test
    void payUniCallsFailClosedWhenUnprovisioned() {
        runner.run(ctx -> {
            PayUniHttpClient client = ctx.getBean(PayUniHttpClient.class);
            assertThat(client).isInstanceOf(PayUniConfig.CredentialGuardedPayUniHttpClient.class);

            assertThatThrownBy(() -> client.callApi("HAWK123", "/api/trade", Map.of(), "1.0"))
                    .isInstanceOf(ShippingBusinessException.class)
                    .satisfies(e -> assertThat(((ShippingBusinessException) e).code()).isEqualTo(9705));

            assertThatThrownBy(() -> client.callAgentApi("AGENT", "/api/agent", Map.of(), "1.0"))
                    .isInstanceOf(ShippingBusinessException.class)
                    .satisfies(e -> assertThat(((ShippingBusinessException) e).code()).isEqualTo(9705));

            assertThatThrownBy(() -> client.callApiHtml("HAWK123", "/api/label", Map.of(), "1.0"))
                    .isInstanceOf(ShippingBusinessException.class)
                    .satisfies(e -> assertThat(((ShippingBusinessException) e).code()).isEqualTo(9705));
        });
    }

    /**
     * 🔴 9705 而非既有的 9503：9503 是 403「這個賣家沒開通這個配送方式」（賣家自己
     * 去建店就能解決），9705 是 424「我方的憑證沒設定」（賣家做什麼都沒用）。
     * 用錯會把賣家導去重複申請建店。
     */
    @Test
    void usesProvisioningCodeNotSellerEligibilityCode() {
        assertThat(GrpcResponseCode.FEATURE_SHIPPING_PROVIDER_DISABLED.code()).isEqualTo(9705);
        assertThat(GrpcResponseCode.FULFILLMENT_PROVIDER_NOT_ENABLED.code()).isEqualTo(9503);
        runner.run(ctx -> {
            PayUniHttpClient client = ctx.getBean(PayUniHttpClient.class);
            assertThatThrownBy(() -> client.callApi("HAWK123", "/api/trade", Map.of(), "1.0"))
                    .satisfies(e -> assertThat(((ShippingBusinessException) e).code())
                            .as("未開通必須是 9705，不得塌成 9503")
                            .isNotEqualTo(9503));
        });
    }

    /**
     * 陽性對照：憑證齊全時必須是<b>真的</b> client，不是守門版本。
     *
     * <p>少了這一格，「永遠回守門版本」也會讓上面三支全綠——那等於把 PayUni 物流
     * 整個關掉，而且沒有任何測試會發現。
     */
    @Test
    void realClientWiredWhenCredentialsPresent() {
        runner.withPropertyValues(
                        "payuni.agent.key=12345678901234567890123456789012",
                        "payuni.agent.iv=1234567890123456")
                .run(ctx -> {
                    assertThat(ctx).hasNotFailed();
                    assertThat(ctx.getBean(PayUniHttpClient.class))
                            .as("憑證齊全時不得回守門版本，否則 PayUni 物流被永久關閉")
                            .isNotInstanceOf(PayUniConfig.CredentialGuardedPayUniHttpClient.class);
                });
    }

    /** 憑證只設了一半 ＝ 配置錯誤，仍然 fail-closed（不得帶著半套金鑰打 PayUni）。 */
    @Test
    void partialCredentialsAlsoFailClosed() {
        runner.withPropertyValues("payuni.agent.key=12345678901234567890123456789012")
                .run(ctx -> {
                    assertThat(ctx).hasNotFailed();
                    PayUniHttpClient client = ctx.getBean(PayUniHttpClient.class);
                    assertThatThrownBy(() -> client.callApi("HAWK123", "/api/trade", Map.of(), "1.0"))
                            .isInstanceOf(ShippingBusinessException.class)
                            .satisfies(e -> assertThat(((ShippingBusinessException) e).code()).isEqualTo(9705));
                });
    }

    /** 指標必須真的被寫入：停用狀態不現形的話，本次改動只是把 CrashLoop 換成靜默失能。 */
    @Test
    void credentialStatusIsObservable() {
        runner.run(ctx -> {
            MeterRegistry registry = ctx.getBean(MeterRegistry.class);
            assertThat(registry.get("shipping_payuni_credentials_configured").gauge().value())
                    .as("未開通時 shipping_payuni_credentials_configured 必須是 0")
                    .isEqualTo(0.0);
            assertThat(registry.get("shipping_payuni_credentials_incomplete").gauge().value())
                    .as("兩者皆空＝尚未開通，不是配置錯誤")
                    .isEqualTo(0.0);
        });

        runner.withPropertyValues("payuni.agent.key=12345678901234567890123456789012")
                .run(ctx -> {
                    MeterRegistry registry = ctx.getBean(MeterRegistry.class);
                    assertThat(registry.get("shipping_payuni_credentials_incomplete").gauge().value())
                            .as("只有一半＝配置錯誤，必須是 1")
                            .isEqualTo(1.0);
                });

        runner.withPropertyValues(
                        "payuni.agent.key=12345678901234567890123456789012",
                        "payuni.agent.iv=1234567890123456")
                .run(ctx -> {
                    MeterRegistry registry = ctx.getBean(MeterRegistry.class);
                    assertThat(registry.get("shipping_payuni_credentials_configured").gauge().value())
                            .isEqualTo(1.0);
                });
    }

    @Configuration
    static class TestDeps {
        @Bean
        MeterRegistry meterRegistry() {
            return new SimpleMeterRegistry();
        }

        @Bean
        ShippingPayUniAuditLogger auditLogger() {
            return Mockito.mock(ShippingPayUniAuditLogger.class);
        }
    }
}
