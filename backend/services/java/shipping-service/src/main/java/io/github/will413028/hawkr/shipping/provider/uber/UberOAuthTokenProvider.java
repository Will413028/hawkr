package io.github.will413028.hawkr.shipping.provider.uber;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import java.io.IOException;
import java.net.URLEncoder;
import java.nio.charset.StandardCharsets;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.Map;

/**
 * Uber OAuth2 {@code client_credentials} token 的集中快取（66 §4.5.1）。
 *
 * <h2>🔴 為什麼「快取」是硬需求而不是最佳化</h2>
 * 官方明載 token 發放速率 <b>100 requests / hour</b>
 * （{@code docs/第三方整合/Uber Direct API 總覽.md} §1.1），而 token 有效期是 <b>30 天</b>。
 * 每次請求重新取 token，光這一項就會撞上 100/hr 上限——而撞上的症狀是
 * 「Uber 突然全面 401」，看起來完全像被停權。
 *
 * <h2>🔴 刷新失敗不得 fail-open</h2>
 * 66 §4.5.1 逐字：「拿不到 token ⇒ 立刻拋、選項 {@code available=false}。
 * 『先用舊 token 試試看』會把一次認證失敗變成 N 次 401，而 401 對外看起來像我們被停權。」
 * ⇒ {@link #bearerToken()} 在刷新失敗時<b>拋例外</b>，<b>不</b>退回快取中的舊值。
 *
 * <h2>⚠️ 刻意不放 Redis</h2>
 * 66 §4.5.1 逐字：「token 快取放行程內即可——多副本各自取自己的 token 是允許的；
 * <b>不要</b>為此引入 Redis（那會讓 token 變成一個『不能失敗的狀態』，
 * 觸發《硬上限依賴接入治理規範（配額與容量）》的接入前置三件）。」
 *
 * <p>⚠️ 副本數 × 刷新頻率仍要落在 100/hr 內：30 天有效期 ＋ 20% 門檻 ⇒ 單一副本約
 * 每 24 天刷一次，即使 {@code replicas} 拉到兩位數也遠低於上限。
 */
public class UberOAuthTokenProvider {

    private static final ObsLogger log = ObsLogger.forClass(UberOAuthTokenProvider.class);

    /**
     * 剩餘壽命低於這個比例就換新（66 §4.5.1 逐字「剩餘壽命 &lt; 20% 時背景換新」）。
     */
    static final double REFRESH_THRESHOLD_RATIO = 0.20;

    /**
     * token 端點的 deadline。
     *
     * <p>⚠️ 🔴 <b>這個值是我方裁量，不是 66 §0.2 的表列值</b>——該表只給了 E1~E7 七支業務端點。
     * 取 10s 的理由：它必須大於任何一支業務呼叫的連線建立成本、又要小於最短的業務 deadline
     * （E1 的 8s）＋ 一次重試，否則 token 取得會吃掉呼叫端自己的預算。
     */
    static final Duration TOKEN_DEADLINE = Duration.ofSeconds(10);

    private final UberHttpTransport transport;
    private final ObjectMapper objectMapper;
    private final Clock clock;
    private final String authUrl;
    private final String clientId;
    private final String clientSecret;
    private final String scope;

    private final Object refreshLock = new Object();
    private volatile CachedToken cached;

    public UberOAuthTokenProvider(UberHttpTransport transport,
                                  ObjectMapper objectMapper,
                                  Clock clock,
                                  String authUrl,
                                  String clientId,
                                  String clientSecret,
                                  String scope) {
        this.transport = transport;
        this.objectMapper = objectMapper;
        this.clock = clock;
        this.authUrl = authUrl;
        this.clientId = clientId;
        this.clientSecret = clientSecret;
        this.scope = scope;
    }

    /**
     * 取得可用的 bearer token。
     *
     * @throws UberApiException 拿不到 token。🔴 <b>絕不</b>退回快取中的舊 token。
     */
    public String bearerToken() {
        Instant now = clock.instant();
        CachedToken current = cached;
        if (current != null && !current.needsRefresh(now)) {
            return current.accessToken();
        }
        synchronized (refreshLock) {
            // double-check：多執行緒同時發現要換新時，只有第一個真的去打 token 端點。
            // 🔴 少了這一格，一次冷啟動的併發尖峰就會打出 N 次 token 請求，而上限是 100/hr。
            CachedToken again = cached;
            if (again != null && !again.needsRefresh(clock.instant())) {
                return again.accessToken();
            }
            CachedToken fresh = requestToken();
            cached = fresh;
            return fresh.accessToken();
        }
    }

    /** 憑證是否齊備。缺席時 Uber 選項必須 {@code available=false}，而不是帶空字串去打 API。 */
    public boolean credentialsConfigured() {
        return clientId != null && !clientId.isBlank()
                && clientSecret != null && !clientSecret.isBlank();
    }

    private CachedToken requestToken() {
        if (!credentialsConfigured()) {
            // 🔴 fail-closed（66 §8.5）：拿空字串去打 API 會變成一連串 401，
            //    對外看起來像我們被停權，而真正的問題是 secret 沒掛上。
            throw new UberApiException(UberFailureKind.AUTH, 0, "UBER_CREDENTIALS_MISSING",
                    "Uber OAuth 憑證未設定（uber-direct-credentials 的 client-id／client-secret）");
        }
        String form = "grant_type=client_credentials"
                + "&client_id=" + enc(clientId)
                + "&client_secret=" + enc(clientSecret)
                + "&scope=" + enc(scope);
        UberHttpTransport.UberHttpResponse resp;
        try {
            resp = transport.send("POST", authUrl,
                    Map.of("Content-Type", "application/x-www-form-urlencoded",
                            "Accept", "application/json"),
                    form, TOKEN_DEADLINE);
        } catch (IOException e) {
            // 🔴 訊息裡不得出現 clientSecret，連長度都不記（66 §8.6 逐字）。
            throw new UberApiException(UberFailureKind.NON_DETERMINISTIC, 0, "UBER_TOKEN_IO",
                    "Uber token 端點連線失敗", e);
        }
        if (!resp.isSuccess()) {
            throw new UberApiException(UberFailureKind.AUTH, resp.statusCode(), "UBER_TOKEN_REJECTED",
                    "Uber token 端點回 " + resp.statusCode());
        }
        try {
            JsonNode node = objectMapper.readTree(resp.body());
            String accessToken = node.path("access_token").asText(null);
            long expiresIn = node.path("expires_in").asLong(0L);
            if (accessToken == null || accessToken.isBlank() || expiresIn <= 0) {
                throw new UberApiException(UberFailureKind.AUTH, resp.statusCode(), "UBER_TOKEN_MALFORMED",
                        "Uber token 回應缺 access_token 或 expires_in");
            }
            Instant issuedAt = clock.instant();
            log.info("UberOAuthTokenProvider.refresh.success",
                    "Uber token 已換新", "expiresInSeconds", String.valueOf(expiresIn));
            return new CachedToken(accessToken, issuedAt, issuedAt.plusSeconds(expiresIn));
        } catch (UberApiException e) {
            throw e;
        } catch (IOException e) {
            throw new UberApiException(UberFailureKind.AUTH, resp.statusCode(), "UBER_TOKEN_MALFORMED",
                    "Uber token 回應無法解析", e);
        }
    }

    private static String enc(String v) {
        return URLEncoder.encode(v == null ? "" : v, StandardCharsets.UTF_8);
    }

    /**
     * 快取中的 token。
     *
     * @param issuedAt 用來算「剩餘壽命比例」的基準——🔴 必須記發放時間，
     *                 只記到期時間的話算不出 20% 門檻（分母不見了）。
     */
    record CachedToken(String accessToken, Instant issuedAt, Instant expiresAt) {

        boolean needsRefresh(Instant now) {
            long lifetimeSeconds = expiresAt.getEpochSecond() - issuedAt.getEpochSecond();
            if (lifetimeSeconds <= 0) {
                return true;
            }
            long remainingSeconds = expiresAt.getEpochSecond() - now.getEpochSecond();
            return remainingSeconds <= lifetimeSeconds * REFRESH_THRESHOLD_RATIO;
        }
    }
}
