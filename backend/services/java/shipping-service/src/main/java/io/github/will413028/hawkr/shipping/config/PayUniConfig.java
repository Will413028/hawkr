package io.github.will413028.hawkr.shipping.config;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.http.PayUniHttpClient;
import io.github.will413028.hawkr.payuni.shipping.ShippingClient;
import io.github.will413028.hawkr.shipping.audit.ShippingPayUniAuditLogger;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.micrometer.core.instrument.MeterRegistry;
import jakarta.annotation.PostConstruct;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

import java.util.Map;

@Configuration
public class PayUniConfig {

    private static final ObsLogger log = ObsLogger.forClass(PayUniConfig.class);

    /**
     * 🔴 {@code :} 預設值不可移除（backend#3329）。
     *
     * <p>沒有預設值時，{@code PAYUNI_AGENT_KEY} 缺席會讓 Spring 在解析這個 {@code @Value}
     * 當下丟 {@code IllegalArgumentException: Could not resolve placeholder}，
     * 於是本 bean 建不起來 ⇒ context refresh 失敗 ⇒ <b>整個 shipping-service 起不來</b>，
     * 連運費計算（結帳 hot path、完全不碰 PayUni）一起死。
     *
     * <p>空值 ＝ PayUni 物流尚未開通，是合法的部署狀態。fail-closed 在使用點：
     * 見 {@link #payUniHttpClient}。
     */
    @Value("${payuni.agent.key:}")
    private String agentKey;

    @Value("${payuni.agent.iv:}")
    private String agentIv;

    @Value("${payuni.api.base-url}")
    private String baseUrl;

    @Value("${payuni.shipping.notify-url:}")
    private String shippingNotifyUrl;

    @Value("${payuni.shipping.store-map-return-url:}")
    private String storeMapReturnUrl;

    @Value("${payuni.shipping.store-map-mer-id:HAWK}")
    private String storeMapMerId;

    @Value("${gateway.url:}")
    private String gatewayUrl;

    private final MeterRegistry meterRegistry;

    public PayUniConfig(MeterRegistry meterRegistry) {
        this.meterRegistry = meterRegistry;
    }

    /** PayUni 代理商憑證是否已設定（兩者皆非空才算）。 */
    boolean credentialsConfigured() {
        return agentKey != null && !agentKey.isBlank()
                && agentIv != null && !agentIv.isBlank();
    }

    /** 憑證只設了一半 —— 那是配置錯誤，不是「尚未開通」。 */
    private boolean credentialsPartiallyConfigured() {
        boolean hasKey = agentKey != null && !agentKey.isBlank();
        boolean hasIv = agentIv != null && !agentIv.isBlank();
        return hasKey != hasIv;
    }

    /**
     * 開機時把「PayUni 到底能不能用」寫進 log 與指標（backend#3329）。
     *
     * <p>🔴 這一段不是裝飾。#3329 把憑證從「啟動必要條件」改成選填之後，「沒開通」從一個
     * 會讓 pod CrashLoop 的狀態，變成一個<b>完全安靜</b>的狀態——把 CrashLoop 換成靜默
     * 失能不是修復。{@code shipping_payuni_credentials_configured} 這支 gauge 就是那個差別。
     *
     * <p>採 warn-log 而非 fail-fast，與 payment-service 的 {@code ApplePayProperties}
     * 同一個取捨：沒打算接 PayUni 物流的環境仍能正常啟動跑運費計算與平台流程；
     * 真的漏配時 ops 在開機當下就看得到，不必等賣家按下叫件才發現。
     *
     * <p>🔴 三個狀態不是兩個：全有＝已開通、全無＝尚未開通（皆為合法），<b>只有一半</b>
     * 才是配置錯誤，那一格用 ERROR 而不是 WARN。合成一個布林會讓「env 名打錯」長得跟
     * 「還沒開通」一模一樣，而兩者的正確處置完全相反。
     */
    @PostConstruct
    void reportCredentialStatus() {
        meterRegistry.gauge("shipping_payuni_credentials_configured",
                credentialsConfigured() ? 1 : 0);
        meterRegistry.gauge("shipping_payuni_credentials_incomplete",
                credentialsPartiallyConfigured() ? 1 : 0);

        if (credentialsConfigured()) {
            log.info("ready", "PayUni 物流憑證已載入，PayUni 配送方式可用");
        } else if (credentialsPartiallyConfigured()) {
            log.error("incomplete", "PayUni 物流憑證只有一半（PAYUNI_AGENT_KEY 與 PAYUNI_AGENT_IV 必須成對），"
                    + "所有 PayUni 呼叫將回 FEATURE_SHIPPING_PROVIDER_DISABLED (9705)；"
                    + "運費計算與平台流程不受影響");
        } else {
            log.warn("not-provisioned", "PayUni 物流尚未開通，所有 PayUni 呼叫將回 "
                    + "FEATURE_SHIPPING_PROVIDER_DISABLED (9705)；運費計算與平台流程不受影響");
        }
    }

    @Bean
    public PayUniCryptoService payUniCryptoService() {
        return new PayUniCryptoService(agentKey, agentIv);
    }

    /**
     * PayUni HTTP client；<b>憑證未設定時回一個 fail-closed 的守門版本</b>（backend#3329）。
     *
     * <p>🔴 為什麼把閘門放在這一層，而不是各個 service／adapter 的入口：
     * shipping-service 有五個地方會打 PayUni（兩個 fulfillment adapter、兩個建單 strategy、
     * onboarding、store map、label），逐一加守衛只要有一個新呼叫點忘了加就是
     * 一次帶著空金鑰的真實 PayUni 呼叫。{@link PayUniHttpClient} 的三支 call 方法是
     * <b>所有</b> PayUni 網路流量的唯一出口，守在這裡忘不掉。
     *
     * <p>（{@code ShipmentLabelService} 的 cat_file_download 二次 POST 確實不經本 client，
     * 但它解析的是前一次 {@code scheduleHomeLabelHtml} 回來的 HTML ⇒ 前一次已經被擋下，
     * 走不到那裡。）
     *
     * <p>🔴 <b>拋例外，不是回 null／空回應。</b> 回一個空的 {@link PayUniApiResponse}
     * 會讓「憑證沒設」長得像「PayUni 說這筆查無資料」，賣家的叫件會靜默失敗而監控上
     * 什麼都看不到——那正是本 issue 要消滅的失效形狀。
     */
    @Bean
    public PayUniHttpClient payUniHttpClient(PayUniCryptoService cryptoService,
                                             ShippingPayUniAuditLogger auditLogger) {
        PayUniHttpClient client = credentialsConfigured()
                ? new PayUniHttpClient(cryptoService, baseUrl)
                : new CredentialGuardedPayUniHttpClient(cryptoService, baseUrl);
        // 06 §10.3 audit logging：注入 onExchange logger 將每次 PayUni exchange 寫入 payuni_audit_logs 專表
        client.setRequestLogger(auditLogger);
        return client;
    }

    /**
     * 憑證未設定時取代 {@link PayUniHttpClient} 的守門實作：任何一次 PayUni 呼叫都直接
     * 拋 {@code FEATURE_SHIPPING_PROVIDER_DISABLED}（9705），一個位元組都不送出去。
     *
     * <p>⚠️ 9705 而非既有的 9503 {@code FULFILLMENT_PROVIDER_NOT_ENABLED}：9503 是 403
     * 「這個賣家沒開通這個配送方式」（賣家自己去建店就能解決），9705 是 424「我方的
     * PayUni 憑證沒設定」（賣家做什麼都沒用，要動的是維運）。用 9503 會把賣家導去
     * 重複申請建店。
     */
    static final class CredentialGuardedPayUniHttpClient extends PayUniHttpClient {

        CredentialGuardedPayUniHttpClient(PayUniCryptoService cryptoService, String baseUrl) {
            super(cryptoService, baseUrl);
        }

        @Override
        public PayUniApiResponse callApi(String merId, String path, Map<String, String> params, String version) {
            throw disabled();
        }

        @Override
        public PayUniApiResponse callAgentApi(String agentId, String path, Map<String, String> params, String version) {
            throw disabled();
        }

        @Override
        public String callApiHtml(String merId, String path, Map<String, String> params, String version) {
            throw disabled();
        }

        private static ShippingBusinessException disabled() {
            return new ShippingBusinessException(GrpcResponseCode.FEATURE_SHIPPING_PROVIDER_DISABLED);
        }
    }

    @Bean
    public ShippingClient shippingClient(PayUniHttpClient httpClient) {
        String notifyUrl = (shippingNotifyUrl != null && !shippingNotifyUrl.isBlank())
                ? shippingNotifyUrl : null;
        return new ShippingClient(httpClient, notifyUrl);
    }

    public String getBaseUrl() {
        return baseUrl;
    }

    public String getStoreMapReturnUrl() {
        return storeMapReturnUrl;
    }

    public String getStoreMapMerId() {
        return storeMapMerId;
    }

    public String getGatewayUrl() {
        return gatewayUrl;
    }
}
