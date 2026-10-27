package io.github.will413028.hawkr.shipping.provider.uber;

import io.github.will413028.hawkr.common.webhook.UberWebhookVerifier;
import io.github.will413028.hawkr.shipping.config.UberConfig;
import io.github.will413028.hawkr.shipping.mapper.UberQuoteMapper;
import io.github.will413028.hawkr.shipping.provider.create.UberShipmentStrategy;
import com.fasterxml.jackson.databind.ObjectMapper;

import java.time.Clock;
import java.time.Instant;
import java.time.ZoneOffset;
import java.util.List;
import java.util.Map;

import static org.mockito.Mockito.mock;

/**
 * Uber adapter 的測試組裝。
 *
 * <h2>⚠️ 這裡組的是<b>正牌</b> {@link UberDirectAdapter}，只有它的協作者是替身</h2>
 * 理由與 {@code TestFulfillmentProviders} 逐字相同：capability 集合是待驗的那件事，
 * 用 stub adapter 等於自己跟自己對。
 */
public final class TestUberAdapters {

    /** 固定時鐘：時間敏感邏輯（token 20% 門檻、pickup_ready_dt）要能被斷言，不能靠 sleep。 */
    public static final Clock FIXED_CLOCK =
            Clock.fixed(Instant.parse("2026-09-12T00:00:00Z"), ZoneOffset.UTC);

    /** 測試用的 webhook signing key（不是真實金鑰，也不是任何環境用過的值）。 */
    public static final String TEST_SIGNING_KEY = "test-uber-webhook-signing-key";

    private TestUberAdapters() {
    }

    /** 全部協作者都是 mock 的 adapter（給只需要「capability 是不是 present」的測試用）。 */
    public static UberDirectAdapter mockedAdapter() {
        return builder().build();
    }

    public static Builder builder() {
        return new Builder();
    }

    public static final class Builder {
        private UberShipmentStrategy strategy = mock(UberShipmentStrategy.class);
        private UberApiClient apiClient = mock(UberApiClient.class);
        private UberStoreResolver storeResolver = mock(UberStoreResolver.class);
        private UberQuoteMapper quoteMapper = mock(UberQuoteMapper.class);
        private UberPodStorage podStorage = mock(UberPodStorage.class);
        private UberWebhookVerifier verifier = new UberWebhookVerifier(List.of(TEST_SIGNING_KEY));
        private UberConfig.UberAvailability availability = new UberConfig.UberAvailability(true, true);
        private final ObjectMapper objectMapper = new ObjectMapper();

        public Builder apiClient(UberApiClient v) {
            this.apiClient = v;
            return this;
        }

        public Builder storeResolver(UberStoreResolver v) {
            this.storeResolver = v;
            return this;
        }

        public Builder quoteMapper(UberQuoteMapper v) {
            this.quoteMapper = v;
            return this;
        }

        public Builder podStorage(UberPodStorage v) {
            this.podStorage = v;
            return this;
        }

        public Builder strategy(UberShipmentStrategy v) {
            this.strategy = v;
            return this;
        }

        /** 用一組完全不同的金鑰建驗證器——驗「偽造簽章會被擋下」時用。 */
        public Builder signingKeys(List<String> keys) {
            this.verifier = new UberWebhookVerifier(keys);
            return this;
        }

        public Builder availability(boolean enabled, boolean credentialsConfigured) {
            this.availability = new UberConfig.UberAvailability(enabled, credentialsConfigured);
            return this;
        }

        public UberDirectAdapter build() {
            return new UberDirectAdapter(strategy, apiClient, storeResolver, quoteMapper,
                    new UberPayloadCodec(objectMapper), podStorage, verifier, availability,
                    objectMapper, FIXED_CLOCK);
        }
    }

    /** 一份 {@code {webhookId: key}}（66 §8.5 的形狀）。 */
    public static Map<String, String> signingKeyMap() {
        return Map.of("wh_test_1", TEST_SIGNING_KEY);
    }
}
