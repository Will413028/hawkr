package io.github.will413028.hawkr.shipping.provider.uber;

import java.io.IOException;
import java.net.SocketTimeoutException;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/**
 * 可程式化的 {@link UberHttpTransport} 替身。
 *
 * <h2>為什麼不是 MockWebServer</h2>
 * {@code shipping-service/pom.xml} 沒有 {@code mockwebserver}（實查：只有 {@code okhttp}），
 * 而 pom.xml 不在本波的可編輯範圍內。把傳輸抽成介面之後，
 * 400／5xx／timeout 三格的分流可以直接餵出來——那三格是 66 陷阱 A8 唯一的防線。
 *
 * <h2>🔴 它會記錄每一次呼叫</h2>
 * token 快取那條測試的鑑別力全在「token 端點被打了<b>幾次</b>」，
 * 只回固定值的替身在那裡完全沒有鑑別力。
 */
public class FakeUberTransport implements UberHttpTransport {

    /** 一次呼叫的留痕。 */
    public record Call(String method, String url, Duration deadline, String body) {
    }

    private final List<Call> calls = new ArrayList<>();
    private final String tokenJson;

    /** 業務端點的回應；{@code null} 表示丟 IOException（逾時）。 */
    private UberHttpResponse businessResponse;
    private IOException businessFailure;

    /** token 端點回幾次成功之後開始失敗（{@code -1} ＝ 永遠成功）。 */
    private int tokenFailAfter = -1;
    private int tokenCalls;

    public FakeUberTransport() {
        this(2_592_000L);
    }

    /** @param tokenTtlSeconds token 的 {@code expires_in}（官方是 30 天＝2,592,000 秒） */
    public FakeUberTransport(long tokenTtlSeconds) {
        this.tokenJson = "{\"access_token\":\"tok_test\",\"expires_in\":" + tokenTtlSeconds + "}";
    }

    public FakeUberTransport respondWith(int statusCode, String body) {
        this.businessResponse = new UberHttpResponse(statusCode, "application/json",
                body == null ? new byte[0] : body.getBytes(StandardCharsets.UTF_8));
        this.businessFailure = null;
        return this;
    }

    /** 逾時／連線失敗 —— 🔴 契約上是 <b>非確定性</b>：請求可能已經到達對方。 */
    public FakeUberTransport failWithTimeout() {
        this.businessFailure = new SocketTimeoutException("timeout");
        this.businessResponse = null;
        return this;
    }

    public FakeUberTransport tokenFailsAfter(int successes) {
        this.tokenFailAfter = successes;
        return this;
    }

    public List<Call> calls() {
        return List.copyOf(calls);
    }

    public long tokenCallCount() {
        return calls.stream().filter(c -> c.url().contains("/oauth/")).count();
    }

    public List<Call> businessCalls() {
        return calls.stream().filter(c -> !c.url().contains("/oauth/")).toList();
    }

    @Override
    public UberHttpResponse send(String method, String url, Map<String, String> headers,
                                 String jsonBody, Duration deadline) throws IOException {
        calls.add(new Call(method, url, deadline, jsonBody));
        if (url.contains("/oauth/")) {
            tokenCalls++;
            if (tokenFailAfter >= 0 && tokenCalls > tokenFailAfter) {
                return new UberHttpResponse(503, "application/json",
                        "{\"code\":\"unavailable\"}".getBytes(StandardCharsets.UTF_8));
            }
            return new UberHttpResponse(200, "application/json",
                    tokenJson.getBytes(StandardCharsets.UTF_8));
        }
        if (businessFailure != null) {
            throw businessFailure;
        }
        if (businessResponse == null) {
            return new UberHttpResponse(200, "application/json", "{}".getBytes(StandardCharsets.UTF_8));
        }
        return businessResponse;
    }
}
