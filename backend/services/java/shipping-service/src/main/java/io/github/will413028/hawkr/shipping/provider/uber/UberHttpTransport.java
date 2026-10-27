package io.github.will413028.hawkr.shipping.provider.uber;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.Map;

/**
 * Uber 對外 HTTP 呼叫的傳輸層抽象。
 *
 * <h2>🔴 為什麼要有這一層，而不是讓 {@link UberApiClient} 直接持有 OkHttp</h2>
 * 本 repo 的測試相依裡<b>沒有 MockWebServer</b>（實查 {@code shipping-service/pom.xml}：
 * 只有 {@code okhttp}，沒有 {@code mockwebserver}），而 66 §4.5.4 的 400／5xx 分流
 * 是 A8 陷阱唯一的防線——那一格必須有測試盯著，否則「把 5xx 當建單失敗」會在
 * 上線那天才被發現，代價是兩趟車、重複計費。
 *
 * <p>把傳輸抽成介面之後，錯誤分流可以用「餵一個回 500 的 transport」直接驗，
 * 不必新增 build 相依（那是 pom.xml，不在本波的可編輯範圍內）。
 *
 * <h2>契約：IOException ＝ 非確定性</h2>
 * 🔴 實作<b>必須</b>把「連線失敗／逾時／讀取中斷」表達成 {@link IOException}，
 * 而不是回一個假的 5xx status。呼叫端據此分流成 {@code 991 CREATE_UNKNOWN}：
 * 這兩類的共通點是「請求可能已經到達對方並產生副作用」，而那正是 991 的定義。
 */
public interface UberHttpTransport {

    /**
     * 送出一次 HTTP 呼叫。
     *
     * @param method   HTTP method（{@code GET}／{@code POST}）
     * @param url      完整 URL
     * @param headers  請求標頭（Authorization 由呼叫端填入）
     * @param jsonBody 請求主體；{@code null} 表示無主體
     * @param deadline 🔴 本次呼叫的硬上限（call timeout，非僅連線逾時）。
     *                 逐端點不同，值域見 {@link UberApiClient}（66 §4.5.2 端點表）
     * @throws IOException 連線／逾時／讀取失敗 —— <b>非確定性</b>，呼叫端不得當成 400
     */
    UberHttpResponse send(String method,
                          String url,
                          Map<String, String> headers,
                          String jsonBody,
                          Duration deadline) throws IOException;

    /**
     * 一次 HTTP 回應。
     *
     * <p>{@code body} 保留 byte[] 而不是 String：E7 Get POD 是二進位（66 §4.5.2 逐字「二進位」），
     * 先轉成字串再轉回去會破壞內容。
     */
    record UberHttpResponse(int statusCode, String contentType, byte[] body) {

        public String bodyAsString() {
            return body == null ? "" : new String(body, StandardCharsets.UTF_8);
        }

        public boolean isSuccess() {
            return statusCode >= 200 && statusCode < 300;
        }
    }
}
