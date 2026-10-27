package io.github.will413028.hawkr.webhook.uber;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.webhook.UberWebhookVerifier;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.micrometer.core.instrument.MeterRegistry;
import jakarta.annotation.PostConstruct;
import java.util.Map;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * Uber webhook 驗簽金鑰的載入（66 §8.5）。
 *
 * <h2>🔴 為什麼 ingress 這一側也要有金鑰</h2>
 * 驗簽必須發生在寫進 {@code webhook_inbox} <b>之前</b>。
 * inbox 有 {@code uq_inbox_event UNIQUE (provider, webhook_type, event_key)}，
 * 而未驗簽就落地 ⇒ 任何能打到端點的人，搶先送一個<b>同 event_id 的偽造事件</b>，
 * 真正的 Uber 事件到達時就會被判為重複而丟棄
 * ——一個<b>未認證的事件壓制攻擊面</b>：攻擊者不需要偽造得出合法簽章，只要搶先佔住那個鍵。
 *
 * <p>⚠️ shipping-service 側的 {@code UberDirectAdapter.inbound()} 仍然會再驗一次
 * （縱深防禦）。兩處<b>共用 common-core 的同一份實作</b>——安全關鍵的比較邏輯有兩份拷貝時，
 * 修了一邊沒修另一邊是完全無聲的。
 *
 * <h2>🔴 金鑰是一組不是一個</h2>
 * {@code UBER_WEBHOOK_SIGNING_KEYS} 是 {@code {webhookId: key}} 的 JSON map。
 * 只放一把 ⇒ 多註冊一個 webhook 就靜默驗簽失敗，而失敗的下場是我方回 401
 * ⇒ Uber 不重試 4xx ⇒ <b>該來源的事件全丟</b>。
 */
@Configuration
public class UberWebhookSignatureConfig {

    private static final ObsLogger log = ObsLogger.forClass(UberWebhookSignatureConfig.class);

    @Value("${uber.webhook.signing-keys:}")
    private String signingKeysJson;

    private final ObjectMapper objectMapper;
    private final MeterRegistry meterRegistry;

    public UberWebhookSignatureConfig(ObjectMapper objectMapper, MeterRegistry meterRegistry) {
        this.objectMapper = objectMapper;
        this.meterRegistry = meterRegistry;
    }

    /** 驗證器。零把金鑰 ⇒ 每一則 Uber webhook 都會被拒（fail-closed）。 */
    @Bean
    public UberWebhookVerifier uberWebhookVerifier() {
        return UberWebhookVerifier.ofKeyMap(signingKeys());
    }

    /**
     * 開機時把「有幾把金鑰」記成 gauge。
     *
     * <p>🔴 它存在的理由是**兩種完全不同的事在 log 上長得一樣**：
     * 「有人在打我們」與「我方金鑰沒設好」都表現為一連串 401。
     * 前者的 key count &gt; 0、後者是 0 —— 沒有這個 gauge，值班的人分不出來，
     * 而兩者的處置相反（前者觀察、後者立刻補設定）。
     */
    @PostConstruct
    void reportKeyStatus() {
        int keyCount = signingKeys().size();
        meterRegistry.gauge("webhook_uber_signing_key_count", keyCount);
        if (keyCount == 0) {
            log.error("uber-signing-keys-missing",
                    "UBER_WEBHOOK_SIGNING_KEYS 未設定：每一則 Uber webhook 都會被拒（401）。"
                            + "若此環境尚未啟用 Uber，這是預期的；已啟用卻看到這行 ⇒ 事件正在全數遺失");
        }
    }

    Map<String, String> signingKeys() {
        if (signingKeysJson == null || signingKeysJson.isBlank()) {
            return Map.of();
        }
        try {
            Map<String, String> parsed = objectMapper.readValue(
                    signingKeysJson, new TypeReference<Map<String, String>>() { });
            return parsed == null ? Map.of() : parsed;
        } catch (Exception e) {
            // 🔴 不記內容：解析失敗的字串裡就是金鑰本身（66 §8.6「永不進 log，連遮罩後的長度都不要記」）。
            log.error("uber-signing-keys-malformed",
                    "UBER_WEBHOOK_SIGNING_KEYS 不是合法的 JSON map，視為零把金鑰");
            return Map.of();
        }
    }
}
