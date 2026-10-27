package io.github.will413028.hawkr.shipping.config;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.shipping.client.MediaGrpcClient;
import io.github.will413028.hawkr.shipping.mapper.UberStoreMappingMapper;
import io.github.will413028.hawkr.shipping.mapper.UberQuoteMapper;
import io.github.will413028.hawkr.shipping.mapper.PackingPolicyMapper;
import io.github.will413028.hawkr.shipping.client.BuyerGrpcClient;
import io.github.will413028.hawkr.shipping.service.ShippingFeeService;
import io.github.will413028.hawkr.shipping.service.uber.UberQuoteService;
import io.github.will413028.hawkr.shipping.service.uber.UberQuoteWriter;
import io.github.will413028.hawkr.shipping.provider.uber.MediaUberPodStorage;
import io.github.will413028.hawkr.shipping.provider.uber.OkHttpUberTransport;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiClient;
import io.github.will413028.hawkr.shipping.provider.uber.UberHttpTransport;
import io.github.will413028.hawkr.shipping.provider.uber.UberOAuthTokenProvider;
import io.github.will413028.hawkr.shipping.provider.uber.UberPayloadCodec;
import io.github.will413028.hawkr.shipping.provider.uber.UberPodStorage;
import io.github.will413028.hawkr.shipping.provider.uber.UberStoreResolver;
import io.github.will413028.hawkr.common.webhook.UberWebhookVerifier;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.micrometer.core.instrument.MeterRegistry;
import jakarta.annotation.PostConstruct;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

import java.time.Clock;
import java.util.Map;

/**
 * Uber Direct 的設定與機密（66 §8.5）。
 *
 * <h2>🔴 每個 {@code @Value} 都必須帶 {@code :} 預設值</h2>
 * 與 {@link PayUniConfig} 同一個理由（backend#3329）：沒有預設值時，環境變數缺席會讓 Spring
 * 在解析 placeholder 當下丟 {@code IllegalArgumentException} ⇒ 本 bean 建不起來
 * ⇒ context refresh 失敗 ⇒ <b>整個 shipping-service 起不來</b>，
 * 連運費計算（結帳 hot path、完全不碰 Uber）一起死。
 * 空值 ＝ Uber 尚未開通，是<b>合法</b>的部署狀態；fail-closed 在使用點。
 *
 * <h2>🔴 secret 不進 yml、不落庫</h2>
 * {@code client-secret} 與 webhook signing key 來自 k8s secret {@code uber-direct-credentials}
 * （66 §8.5）。{@code V13} 的表註解逐字寫著「client_secret <b>不在本表</b>」。
 * 🔴 它們也<b>永不進 log，連遮罩後的長度都不要記</b>（66 §8.6）。
 *
 * <h2>🔴 fail-closed：憑證缺席 ⇒ Uber 選項不可用，而不是帶空字串去打 API</h2>
 * 帶空字串的後果是一連串 401，而 401 對外看起來像<b>我們被停權</b>（66 §4.5.1）。
 * 落點在 {@link UberOAuthTokenProvider#bearerToken()}：拿不到 token 就拋，不 fail-open。
 */
@Configuration
public class UberConfig {

    private static final ObsLogger log = ObsLogger.forClass(UberConfig.class);

    /** 總開關。⚠️ 這一格切成 true 而程式還在 Phase 1 ＝ 66 陷阱 A1（買家付錢、建出沒人送的單、零告警）。 */
    @Value("${uber.enabled:false}")
    private boolean enabled;

    /** 🔴 base 含 {@code /v1}：66 §4.5.2 端點表的路徑是相對於 {@code https://api.uber.com/v1}。 */
    @Value("${uber.base-url:https://api.uber.com/v1}")
    private String baseUrl;

    @Value("${uber.auth-url:https://auth.uber.com/oauth/v2/token}")
    private String authUrl;

    /** Direct API 的 scope（總覽 §1.1：Direct API＝{@code eats.deliveries}）。 */
    @Value("${uber.scope:eats.deliveries}")
    private String scope;

    @Value("${uber.client-id:}")
    private String clientId;

    /** 🔴 來自 k8s secret；永不落庫、永不進 log。 */
    @Value("${uber.client-secret:}")
    private String clientSecret;

    /**
     * 🔴 <b>一組不是一個</b>：JSON map {@code {webhookId: key}}（66 §8.5 逐字）。
     *
     * <p>只放一把 ⇒ 多註冊一個 webhook 就<b>靜默</b>驗簽失敗（陷阱 C1），
     * 而我方回 401 ⇒ Uber 不重試 4xx ⇒ 該 webhook 的事件<b>全丟</b>，
     * 且 401 在我方看起來像「有人在打我們」。
     */
    @Value("${uber.webhook.signing-keys:}")
    private String webhookSigningKeysJson;

    private final MeterRegistry meterRegistry;
    private final ObjectMapper objectMapper;

    public UberConfig(MeterRegistry meterRegistry, ObjectMapper objectMapper) {
        this.meterRegistry = meterRegistry;
        this.objectMapper = objectMapper;
    }

    /** 兩者皆非空才算已開通。 */
    boolean credentialsConfigured() {
        return clientId != null && !clientId.isBlank()
                && clientSecret != null && !clientSecret.isBlank();
    }

    private boolean credentialsPartiallyConfigured() {
        boolean hasId = clientId != null && !clientId.isBlank();
        boolean hasSecret = clientSecret != null && !clientSecret.isBlank();
        return hasId != hasSecret;
    }

    /**
     * 開機時把「Uber 到底能不能用」寫進 log 與指標。
     *
     * <p>🔴 三個狀態不是兩個（與 {@link PayUniConfig#reportCredentialStatus} 同一個取捨）：
     * 全有＝已開通、全無＝尚未開通（皆為合法），<b>只有一半</b>才是配置錯誤。
     * 合成一個布林會讓「env 名打錯」長得跟「還沒開通」一模一樣，而兩者的正確處置完全相反。
     *
     * <p>⚠️ 另外記 signing key 的<b>把數</b>（不是內容）：0 把代表所有 webhook 都會驗簽失敗，
     * 而那個症狀（我方回 401、Uber 不重試、事件全丟）在 log 上看起來像「有人在打我們」。
     */
    @PostConstruct
    void reportCredentialStatus() {
        int keyCount = webhookSigningKeys().size();
        meterRegistry.gauge("shipping_uber_enabled", enabled ? 1 : 0);
        meterRegistry.gauge("shipping_uber_credentials_configured", credentialsConfigured() ? 1 : 0);
        meterRegistry.gauge("shipping_uber_credentials_incomplete", credentialsPartiallyConfigured() ? 1 : 0);
        meterRegistry.gauge("shipping_uber_webhook_signing_key_count", keyCount);

        if (!enabled) {
            log.info("not-enabled", "Uber Direct 總開關為 false，3003 不會出現在任何可用配送方式中");
        } else if (credentialsPartiallyConfigured()) {
            log.error("incomplete", "Uber 憑證只有一半（UBER_CLIENT_ID 與 UBER_CLIENT_SECRET 必須成對）；"
                    + "所有 Uber 呼叫將 fail-closed，運費計算與其他配送方式不受影響");
        } else if (!credentialsConfigured()) {
            log.warn("not-provisioned", "Uber Direct 尚未開通（憑證未設定）；所有 Uber 呼叫將 fail-closed");
        } else if (keyCount == 0) {
            log.error("webhook-keys-missing",
                    "Uber 憑證已載入但 UBER_WEBHOOK_SIGNING_KEYS 為空：webhook 將全數驗簽失敗回 401，"
                            + "而 Uber 不重試 4xx ⇒ 貨態事件會全部遺失");
        } else {
            log.info("ready", "Uber Direct 已就緒", "webhookSigningKeyCount", String.valueOf(keyCount));
        }
    }

    /**
     * {@code {webhookId: key}} 的解析結果。
     *
     * <p>⚠️ 解析失敗回空 map 而不是拋例外：拋例外會讓 shipping-service 整個起不來
     * （#3329 的同一個形狀），而空 map 的後果被 {@link #reportCredentialStatus} 的
     * ERROR log ＋ gauge 盯著。
     */
    Map<String, String> webhookSigningKeys() {
        if (webhookSigningKeysJson == null || webhookSigningKeysJson.isBlank()) {
            return Map.of();
        }
        try {
            Map<String, String> parsed = objectMapper.readValue(
                    webhookSigningKeysJson, new TypeReference<Map<String, String>>() { });
            return parsed == null ? Map.of() : parsed;
        } catch (Exception e) {
            // 🔴 不記內容：解析失敗的字串裡就是金鑰本身（66 §8.6「永不進 log，連遮罩後的長度都不要記」）。
            log.error("webhook-keys-malformed",
                    "UBER_WEBHOOK_SIGNING_KEYS 不是合法的 JSON map，視為零把金鑰");
            return Map.of();
        }
    }

    /**
     * 🔴 {@link Clock} 走 DI 而不是各處 {@code Instant.now()}。
     *
     * <p>本波有兩處時間敏感邏輯：token 的「剩餘壽命 &lt; 20%」與
     * {@code pickup_ready_dt = now + prep_sla_minutes}。用靜態 {@code now()} 的話，
     * 測試只能靠 sleep 或撞運氣——而那是 CLAUDE.md §21 明令要避免的不穩定測試。
     *
     * <p>⚠️ 一律 {@link Clock#systemUTC()}：application 與 DB 時間以 UTC 為準，
     * 不得以 server local timezone 製造隱性行為（10 檔 §4）。
     */
    @Bean
    public Clock uberClock() {
        return Clock.systemUTC();
    }

    @Bean
    public UberPayloadCodec uberPayloadCodec() {
        return new UberPayloadCodec(objectMapper);
    }

    @Bean
    public UberHttpTransport uberHttpTransport() {
        return new OkHttpUberTransport();
    }

    @Bean
    public UberOAuthTokenProvider uberOAuthTokenProvider(UberHttpTransport transport) {
        return new UberOAuthTokenProvider(transport, objectMapper, Clock.systemUTC(),
                authUrl, clientId, clientSecret, scope);
    }

    @Bean
    public UberApiClient uberApiClient(UberHttpTransport transport,
                                       UberOAuthTokenProvider tokenProvider) {
        return new UberApiClient(transport, tokenProvider, objectMapper, baseUrl);
    }

    @Bean
    public UberWebhookVerifier uberWebhookVerifier() {
        return UberWebhookVerifier.ofKeyMap(webhookSigningKeys());
    }

    @Bean
    public UberStoreResolver uberStoreResolver(UberStoreMappingMapper storeMappingMapper) {
        return new UberStoreResolver(storeMappingMapper);
    }

    @Bean
    public UberPodStorage uberPodStorage(MediaGrpcClient mediaGrpcClient) {
        return new MediaUberPodStorage(mediaGrpcClient);
    }

    /** 供 adapter 判斷「這個 provider 現在可不可用」。 */
    /**
     * Uber 即時報價（66 §6.1.3；Stage 8B W2-2）。
     *
     * <p>🔴 <b>總開關關閉時這個 bean 照樣建立</b>，而且那是刻意的：
     * 用 {@code @ConditionalOnProperty} 把它拿掉的話，呼叫端就得處理
     * 「這個 bean 在不在」，而那件事在 Spring 裡的症狀是<b>啟動失敗</b>或
     * 需要在每個呼叫點加 null 判斷。
     * 讓 bean 永遠存在、由它自己回 {@code UBER_DISABLED}，呼叫端只要處理一種形狀。
     */
    @Bean
    public UberQuoteService uberQuoteService(UberStoreResolver storeResolver,
                                             UberApiClient apiClient,
                                             UberPayloadCodec payloadCodec,
                                             UberQuoteMapper quoteMapper,
                                             PackingPolicyMapper packingPolicyMapper,
                                             UberQuoteWriter writer,
                                             BuyerGrpcClient buyerGrpcClient,
                                             ShippingFeeService shippingFeeService,
                                             ObjectMapper objectMapper,
                                             Clock uberClock) {
        return new UberQuoteService(storeResolver, apiClient, payloadCodec, quoteMapper,
                packingPolicyMapper, writer, buyerGrpcClient, shippingFeeService,
                objectMapper, uberClock, enabled);
    }

    @Bean
    public UberAvailability uberAvailability() {
        return new UberAvailability(enabled, credentialsConfigured());
    }

    /**
     * Uber 可用性的具名載體。
     *
     * <p>🔴 兩個布林分開而不是合成一個：{@code enabled=false} 是<b>刻意關閉</b>
     * （§3.4 三層的第 ① 層，是要求不是缺陷），{@code credentialsConfigured=false} 是<b>配置缺口</b>。
     * 合成一個之後，維運看到「Uber 不可用」分不出該去按開關還是該去掛 secret。
     */
    public record UberAvailability(boolean enabled, boolean credentialsConfigured) {
        public boolean usable() {
            return enabled && credentialsConfigured;
        }
    }
}
