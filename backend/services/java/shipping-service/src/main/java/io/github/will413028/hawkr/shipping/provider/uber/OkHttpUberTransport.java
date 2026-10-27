package io.github.will413028.hawkr.shipping.provider.uber;

import okhttp3.MediaType;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.RequestBody;
import okhttp3.Response;
import okhttp3.ResponseBody;

import java.io.IOException;
import java.time.Duration;
import java.util.Map;
import java.util.concurrent.TimeUnit;

/**
 * {@link UberHttpTransport} 的 OkHttp 實作。
 *
 * <h2>🔴 deadline 用 {@code callTimeout} 而不是 {@code readTimeout}</h2>
 * {@code connectTimeout}／{@code readTimeout}／{@code writeTimeout} 各自計時，
 * 三者相加才是最壞情況——一支宣告 15s 的呼叫實際上可能跑 45s。
 * 66 §0.2 的 T-3~T-7 是<b>整支呼叫</b>的上限，對得上的只有 {@code callTimeout}。
 *
 * <p>⚠️ 逐次呼叫的 deadline 不同（8/10/15/20s）⇒ 用
 * {@link OkHttpClient#newBuilder()} 複製出帶該 timeout 的實例。
 * OkHttp 官方明訂 builder 複製會共用 connection pool 與 dispatcher，
 * 所以這不是「每次建一個新 client」那個反模式。
 */
public class OkHttpUberTransport implements UberHttpTransport {

    private static final MediaType JSON = MediaType.get("application/json; charset=utf-8");
    private static final MediaType FORM = MediaType.get("application/x-www-form-urlencoded");

    private final OkHttpClient baseClient;

    public OkHttpUberTransport(OkHttpClient baseClient) {
        this.baseClient = baseClient;
    }

    public OkHttpUberTransport() {
        this(new OkHttpClient.Builder()
                .connectTimeout(5, TimeUnit.SECONDS)
                .build());
    }

    @Override
    public UberHttpResponse send(String method, String url, Map<String, String> headers,
                                 String jsonBody, Duration deadline) throws IOException {
        OkHttpClient client = baseClient.newBuilder()
                .callTimeout(deadline.toMillis(), TimeUnit.MILLISECONDS)
                .build();

        Request.Builder builder = new Request.Builder().url(url);
        if (headers != null) {
            headers.forEach(builder::addHeader);
        }
        if ("GET".equalsIgnoreCase(method)) {
            builder.get();
        } else {
            MediaType type = (headers != null
                    && "application/x-www-form-urlencoded".equals(headers.get("Content-Type")))
                    ? FORM : JSON;
            builder.method(method, RequestBody.create(jsonBody == null ? "" : jsonBody, type));
        }

        try (Response response = client.newCall(builder.build()).execute()) {
            ResponseBody body = response.body();
            byte[] bytes = body == null ? new byte[0] : body.bytes();
            String contentType = response.header("Content-Type");
            return new UberHttpResponse(response.code(), contentType, bytes);
        }
        // 🔴 逾時在 OkHttp 是 InterruptedIOException（IOException 的子類）⇒ 原樣往上拋。
        //    包成假的 5xx 或吞掉都會讓呼叫端分不出「確定失敗」與「不知道」。
    }
}
