package io.github.will413028.hawkr.shipping.config;

import io.github.will413028.hawkr.shipping.client.MediaGrpcClient;
import io.github.will413028.hawkr.shipping.mapper.UberStoreMappingMapper;
import io.github.will413028.hawkr.shipping.mapper.UberQuoteMapper;
import io.github.will413028.hawkr.shipping.mapper.PackingPolicyMapper;
import io.github.will413028.hawkr.shipping.client.BuyerGrpcClient;
import io.github.will413028.hawkr.shipping.service.ShippingFeeService;
import io.github.will413028.hawkr.shipping.service.uber.UberQuoteWriter;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiClient;
import io.github.will413028.hawkr.common.webhook.UberWebhookVerifier;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;
import org.springframework.boot.autoconfigure.AutoConfigurations;
import org.springframework.boot.autoconfigure.context.PropertyPlaceholderAutoConfiguration;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * Uber 設定的開機行為（66 §8.5）——與 {@link PayUniConfigProvisioningTest} 同一個 bug class。
 *
 * <h2>🔴 這支測試要防的是 backend#3329 的重演</h2>
 * {@code @Value} 少了 {@code :} 預設值時，環境變數缺席會讓 Spring 在解析 placeholder 當下丟
 * {@code IllegalArgumentException: Could not resolve placeholder} ⇒ bean 建不起來
 * ⇒ context refresh 失敗 ⇒ <b>整個 shipping-service 起不來</b>，
 * 連運費計算（結帳 hot path、完全不碰 Uber）一起死。
 *
 * <p>⚠️ Uber 的 env 有六個（{@code UBER_ENABLED}／{@code UBER_BASE_URL}／{@code UBER_AUTH_URL}／
 * {@code UBER_SCOPE}／{@code UBER_CLIENT_ID}／{@code UBER_CLIENT_SECRET}）＋ signing keys，
 * 任何一個漏了預設值都是同一個後果——<b>而 Uber 在 dev/prod 的常態就是「還沒開通」</b>
 * （{@code uber.enabled} 預設 false），所以這條路徑是每天都在走的那一條。
 */
@DisplayName("Uber 設定：憑證缺席仍須開機")
class UberConfigProvisioningTest {

    private final ApplicationContextRunner runner = new ApplicationContextRunner()
            .withConfiguration(AutoConfigurations.of(PropertyPlaceholderAutoConfiguration.class))
            .withUserConfiguration(TestDeps.class, UberConfig.class);

    @Test
    @DisplayName("🔴 六個 env 全缺 ⇒ context 照樣起得來（少一個 : 預設值就會轉紅）")
    void contextStartsWithoutAnyUberEnv() {
        runner.run(ctx -> {
            assertThat(ctx).hasNotFailed();
            assertThat(ctx).hasSingleBean(UberApiClient.class);
            assertThat(ctx).hasSingleBean(UberWebhookVerifier.class);
        });
    }

    @Test
    @DisplayName("🔴 起得來 ≠ 放行：憑證缺席時 Uber 一律不可用（fail-closed）")
    void startingUpDoesNotMeanUsable() {
        // 帶空字串去打 API 的後果是一連串 401，而 401 對外看起來像我們被停權（66 §4.5.1）。
        runner.run(ctx -> assertThat(ctx.getBean(UberConfig.UberAvailability.class).usable())
                .isFalse());
    }

    @Test
    @DisplayName("🔴 憑證齊全但總開關關著 ⇒ 仍然不可用（兩個條件是 AND）")
    void enabledAndCredentialsAreBothRequired() {
        runner.withPropertyValues("uber.client-id=cid", "uber.client-secret=sec")
                .run(ctx -> {
                    UberConfig.UberAvailability a = ctx.getBean(UberConfig.UberAvailability.class);
                    assertThat(a.credentialsConfigured()).isTrue();
                    assertThat(a.enabled())
                            .as("uber.enabled 預設 false —— 66 陷阱 A1：切成 true 而程式還沒好"
                                    + "＝買家付錢、建出沒人送的單、零告警")
                            .isFalse();
                    assertThat(a.usable()).isFalse();
                });
    }

    @Test
    @DisplayName("全開 ⇒ usable（陽性對照：沒有它，把 usable() 寫成恆 false 也會綠）")
    void fullyConfiguredIsUsable() {
        runner.withPropertyValues("uber.enabled=true", "uber.client-id=cid", "uber.client-secret=sec")
                .run(ctx -> assertThat(ctx.getBean(UberConfig.UberAvailability.class).usable()).isTrue());
    }

    @Test
    @DisplayName("🔴 signing keys 是 JSON map（一組不是一個）；壞掉的 JSON 不得讓服務起不來")
    void signingKeysAreAMapAndMalformedJsonDoesNotKillTheService() {
        runner.withPropertyValues("uber.webhook.signing-keys={\"wh_1\":\"k1\",\"wh_2\":\"k2\"}")
                .run(ctx -> assertThat(ctx.getBean(UberConfig.class).webhookSigningKeys())
                        .containsOnlyKeys("wh_1", "wh_2"));

        runner.withPropertyValues("uber.webhook.signing-keys=not-json")
                .run(ctx -> {
                    assertThat(ctx).hasNotFailed();
                    assertThat(ctx.getBean(UberConfig.class).webhookSigningKeys())
                            .as("解析失敗視為零把金鑰（被開機 ERROR log ＋ gauge 盯著），"
                                    + "拋例外會讓整個服務起不來 —— 同 #3329 的形狀")
                            .isEmpty();
                });
    }

    @Configuration
    static class TestDeps {
        @Bean
        MeterRegistry meterRegistry() {
            return new SimpleMeterRegistry();
        }

        @Bean
        ObjectMapper objectMapper() {
            return new ObjectMapper();
        }

        @Bean
        UberStoreMappingMapper uberStoreMappingMapper() {
            return Mockito.mock(UberStoreMappingMapper.class);
        }

        @Bean
        MediaGrpcClient mediaGrpcClient() {
            return Mockito.mock(MediaGrpcClient.class);
        }

        // ── 以下是 uberQuoteService 的依賴（W2-2 報價，2026-09-12 新增）──
        //
        // 🔴 它們與本測試要守的性質**無關**，但必須在 slice 裡存在，否則 context
        //    會因為「缺 mapper」而起不來——而那個失敗訊息長得跟「缺了 : 預設值」
        //    一模一樣（都是 context refresh 失敗），會把本測試變成一支**指向錯誤
        //    根因**的紅燈。本測試的鑑別力來自「它只可能因為 placeholder 而紅」。

        @Bean
        UberQuoteMapper uberQuoteMapper() {
            return Mockito.mock(UberQuoteMapper.class);
        }

        @Bean
        PackingPolicyMapper packingPolicyMapper() {
            return Mockito.mock(PackingPolicyMapper.class);
        }

        @Bean
        UberQuoteWriter uberQuoteWriter() {
            return Mockito.mock(UberQuoteWriter.class);
        }

        @Bean
        BuyerGrpcClient buyerGrpcClient() {
            return Mockito.mock(BuyerGrpcClient.class);
        }

        @Bean
        ShippingFeeService shippingFeeService() {
            return Mockito.mock(ShippingFeeService.class);
        }
    }
}
