package io.github.will413028.hawkr.shipping.provider.uber;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.CreateDeliveryRequest;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.DeliveryResult;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.ManifestItem;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.PodDocument;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.QuoteRequest;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.QuoteResult;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.UberWaypoint;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.io.IOException;
import java.net.URLEncoder;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.time.Instant;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Uber Direct 的 HTTP 呼叫層（66 §4.5.2 端點表）。
 *
 * <h2>端點與 deadline（🔴 逐支不同，不得共用一個值）</h2>
 * <table border="1">
 *   <caption>base {@code https://api.uber.com/v1}</caption>
 *   <tr><th>#</th><th>功能</th><th>Method</th><th>路徑</th><th>deadline</th><th>副作用</th></tr>
 *   <tr><td>E1</td><td>Create Quote</td><td>POST</td><td>{@code /customers/{cid}/delivery_quotes}</td><td>8s</td><td>否</td></tr>
 *   <tr><td>E2</td><td>Create Delivery</td><td>POST</td><td>{@code /customers/{cid}/deliveries}</td><td>15s</td><td><b>是</b></td></tr>
 *   <tr><td>E3</td><td>List Deliveries</td><td>GET</td><td>{@code /customers/{cid}/deliveries}</td><td>10s</td><td>否</td></tr>
 *   <tr><td>E4</td><td>Get Delivery</td><td>GET</td><td>{@code /customers/{cid}/deliveries/{did}}</td><td>10s</td><td>否</td></tr>
 *   <tr><td>E6</td><td>Cancel Delivery</td><td>POST</td><td>{@code /customers/{cid}/deliveries/{did}/cancel}</td><td>15s</td><td><b>是</b></td></tr>
 *   <tr><td>E7</td><td>Get POD</td><td>POST</td><td>{@code /customers/{cid}/deliveries/{did}/proof-of-delivery}</td><td>20s</td><td>否</td></tr>
 * </table>
 *
 * <p>🔴 <b>E5 Update Delivery 刻意不實作</b>：66 §4.5.0 逐字「全書零使用點（{@code E5} 只出現在端點表）
 * ⇒ 本波不實作」。要用時走 65 §4.4.1a 的判準先確認它屬於哪個 capability，
 * 不要直接在 service 上加方法。
 *
 * <h2>🔴 400 與 5xx 的分流</h2>
 * 見 {@link UberFailureKind}。這一格錯了的後果是<b>兩趟車、重複計費</b>（陷阱 A8），
 * 而它在測試上完全看得見——{@code UberApiClientErrorMappingTest} 每一格各一支。
 *
 * <h2>PII 與 log（66 §8.6）</h2>
 * 🔴 本類別<b>不記請求與回應的 body</b>：outbound payload 含收件地址、收件人電話、
 * {@code external_store_id}（憑證等級）。log 只留 method／端點代號／HTTP status／耗時。
 * 8.6 逐字：「遮罩只做 body 沒做 query／outbound ⇒ 地址電話從 outbound log 流出」（陷阱 C2）。
 */
public class UberApiClient {

    private static final ObsLogger log = ObsLogger.forClass(UberApiClient.class);

    // ── deadline（66 §0.2 T-3 ~ T-7 ＋ §4.5.2 端點表） ────────────────────
    static final Duration E1_QUOTE_DEADLINE = Duration.ofSeconds(8);
    static final Duration E2_CREATE_DEADLINE = Duration.ofSeconds(15);
    static final Duration E3_LIST_DEADLINE = Duration.ofSeconds(10);
    static final Duration E4_GET_DEADLINE = Duration.ofSeconds(10);
    static final Duration E6_CANCEL_DEADLINE = Duration.ofSeconds(15);
    static final Duration E7_POD_DEADLINE = Duration.ofSeconds(20);

    /**
     * 有副作用的呼叫的退避階梯（66 §0.3：我方 → Uber（Create）＝ 2 次／2s・6s）。
     *
     * <p>🔴 次數要小：每一次重試都是一次<b>可能建出第二張單</b>的機會。
     * 去重由我方 pre-commit 鍵保證（{@code ADR-014}），不依賴 Uber 官方冪等。
     */
    static final long[] SIDE_EFFECT_BACKOFF_MS = {2_000L, 6_000L};

    /**
     * 無副作用的呼叫的退避階梯（66 §0.3：我方 → Uber（Get／POD）＝ 2 次／1s・3s）。
     *
     * <p>⚠️ E1／E3／E6 <b>不在 §0.3 的表內</b>。本類別按「有沒有副作用」分配：
     * E1／E3 無副作用走本階梯，E6 有副作用走 {@link #SIDE_EFFECT_BACKOFF_MS}。
     * 這是推導不是表列值，已登記在交付報告的「與設計書不符」節。
     */
    static final long[] READ_ONLY_BACKOFF_MS = {1_000L, 3_000L};

    /** 可注入的睡眠器：測試用 no-op，避免重試測試真的睡 8 秒。 */
    @FunctionalInterface
    public interface Sleeper {
        void sleep(long millis) throws InterruptedException;
    }

    private final UberHttpTransport transport;
    private final UberOAuthTokenProvider tokenProvider;
    private final ObjectMapper objectMapper;
    private final String baseUrl;
    private final Sleeper sleeper;

    public UberApiClient(UberHttpTransport transport,
                         UberOAuthTokenProvider tokenProvider,
                         ObjectMapper objectMapper,
                         String baseUrl) {
        this(transport, tokenProvider, objectMapper, baseUrl, Thread::sleep);
    }

    public UberApiClient(UberHttpTransport transport,
                         UberOAuthTokenProvider tokenProvider,
                         ObjectMapper objectMapper,
                         String baseUrl,
                         Sleeper sleeper) {
        this.transport = transport;
        this.tokenProvider = tokenProvider;
        this.objectMapper = objectMapper;
        this.baseUrl = stripTrailingSlash(baseUrl);
        this.sleeper = sleeper;
    }

    // ==================== E1 Create Quote ====================

    /** E1：報價。無副作用；失敗／逾時 ⇒ 呼叫端讓選項 {@code available=false}（66 §4.5.0 E1）。 */
    public QuoteResult createQuote(String customerId, QuoteRequest request) {
        ObjectNode body = objectMapper.createObjectNode();
        body.put("external_store_id", request.externalStoreId());
        putWaypoint(body, "pickup", request.pickup());
        putWaypoint(body, "dropoff", request.dropoff());
        if (request.pickupReadyDt() != null) {
            body.put("pickup_ready_dt", request.pickupReadyDt());
        }
        JsonNode resp = callJson("E1", "POST",
                customerUrl(customerId) + "/delivery_quotes", body,
                E1_QUOTE_DEADLINE, READ_ONLY_BACKOFF_MS);
        // 🔴 入站換算：官方 fee 是 cents，domain 一律元整數（66 邊界規則）。
        //    ⚠️ 這裡曾經直接取值不換算 ⇒ NT$115 的配送存成 11500，向賣家收款會差 100 倍。
        //    `fromMinorUnits` 對不整除 fail-closed，不做截斷——理由見該方法 javadoc。
        String quoteCurrency = text(resp, "currency");
        return new QuoteResult(
                text(resp, "id"),
                UberFieldFormat.fromMinorUnits(resp.path("fee").asLong(0L), quoteCurrency),
                quoteCurrency,
                parseInstant(text(resp, "expires")),
                text(resp, "dropoff_eta"));
    }

    // ==================== E2 Create Delivery ====================

    /**
     * E2：建單。<b>有副作用</b>。
     *
     * @throws UberApiException {@link UberFailureKind#DETERMINISTIC} ⇒ 呼叫端寫 {@code 990}；
     *                          {@link UberFailureKind#NON_DETERMINISTIC} ⇒ 呼叫端寫 {@code 991}，
     *                          🔴 <b>不得當 990</b>
     */
    public DeliveryResult createDelivery(String customerId, CreateDeliveryRequest request) {
        ObjectNode body = objectMapper.createObjectNode();
        body.put("quote_id", request.quoteId());
        body.put("external_store_id", request.externalStoreId());
        // 🔴 我方 pre-commit 鍵（ADR-014）。官方明載會把它原樣回傳
        //    （總覽 §3.1 external_order_id／§7.2 external_id）⇒ 991 的反查錨只能是它。
        body.put("external_id", request.externalId());
        if (request.manifestReference() != null) {
            body.set("manifest", objectMapper.createObjectNode()
                    .put("reference", request.manifestReference()));
        }
        putWaypoint(body, "pickup", request.pickup());
        putWaypoint(body, "dropoff", request.dropoff());
        body.set("manifest_items", manifestItems(request.manifestItems()));
        if (request.pickupReadyDt() != null) {
            body.put("pickup_ready_dt", request.pickupReadyDt());
        }
        JsonNode resp = callJson("E2", "POST", customerUrl(customerId) + "/deliveries", body,
                E2_CREATE_DEADLINE, SIDE_EFFECT_BACKOFF_MS);
        return toDeliveryResult(resp);
    }

    // ==================== E3 List Deliveries ====================

    /**
     * E3：以我方送出的 {@code external_id} 反查。
     *
     * <p>🔴 <b>只用於 {@code 991} 沒有 {@code delivery_id} 時</b>（66 §4.5.0 E3／陷阱 B2）：
     * {@code 991} 誤用 E4 回查 ⇒ 每次 404 ⇒ 永不收斂。
     *
     * <p>⚠️ 「Uber 的 List 回應會不會接受以我方 external id 過濾」<b>尚未查證</b>（66 UP-9）。
     * 不成立時本方法會回空清單，而 {@code 991} 的唯一收斂手段就只剩人工到 Uber Dashboard 查
     * ——那是已登記的 open item，不是本方法的 bug。
     */
    public List<DeliveryResult> listDeliveriesByExternalId(String customerId, String externalId) {
        String url = customerUrl(customerId) + "/deliveries?external_id=" + enc(externalId);
        JsonNode resp = callJson("E3", "GET", url, null, E3_LIST_DEADLINE, READ_ONLY_BACKOFF_MS);
        List<DeliveryResult> out = new ArrayList<>();
        JsonNode arr = resp.isArray() ? resp : resp.path("data");
        if (arr.isArray()) {
            for (JsonNode n : arr) {
                out.add(toDeliveryResult(n));
            }
        }
        return List.copyOf(out);
    }

    // ==================== E4 Get Delivery ====================

    /** E4：以 {@code delivery_id} 回查。無副作用，重試耗盡 ⇒ 呼叫端「留原狀態，下個 tick 再來」。 */
    public DeliveryResult getDelivery(String customerId, String deliveryId) {
        JsonNode resp = callJson("E4", "GET",
                customerUrl(customerId) + "/deliveries/" + enc(deliveryId), null,
                E4_GET_DEADLINE, READ_ONLY_BACKOFF_MS);
        return toDeliveryResult(resp);
    }

    // ==================== E6 Cancel Delivery ====================

    /**
     * E6：取消。<b>有副作用</b>。四個 provider 中只有 Uber 有這支（66 §4.5.0）。
     *
     * @throws UberApiException {@link UberFailureKind#NONCANCELABLE} ⇒ 取消失敗、<b>留原狀態</b>
     *                          （不寫 {@code 992 → 41}）、回 {@code 9522}
     */
    public DeliveryResult cancelDelivery(String customerId, String deliveryId) {
        JsonNode resp = callJson("E6", "POST",
                customerUrl(customerId) + "/deliveries/" + enc(deliveryId) + "/cancel",
                objectMapper.createObjectNode(), E6_CANCEL_DEADLINE, SIDE_EFFECT_BACKOFF_MS);
        return toDeliveryResult(resp);
    }

    // ==================== E7 Get POD ====================

    /**
     * E7：取得送達證明（二進位）。
     *
     * @param waypoint  {@code pickup}／{@code dropoff}／{@code return}
     * @param proofType {@code picture}／{@code signature}／{@code pincode}
     *                  （66 §7.8：{@code leave_at_door} 只有 picture，需簽收才有 signature
     *                  ⇒ INV-U5 是「≥1 筆」而不是「三種都要有」）
     */
    public PodDocument fetchProofOfDelivery(String customerId, String deliveryId,
                                            String waypoint, String proofType) {
        ObjectNode body = objectMapper.createObjectNode();
        body.put("waypoint", waypoint);
        body.put("type", proofType);
        UberHttpTransport.UberHttpResponse resp = callRaw("E7", "POST",
                customerUrl(customerId) + "/deliveries/" + enc(deliveryId) + "/proof-of-delivery",
                body.toString(), E7_POD_DEADLINE, READ_ONLY_BACKOFF_MS);
        // 回應可能是 JSON 包 base64（官方未載於本次實查頁面），也可能直接是二進位。
        // 🔴 兩種都接，但**不猜**：不是 JSON 就當二進位原樣回，不做任何轉碼。
        String contentType = resp.contentType() == null ? "application/octet-stream" : resp.contentType();
        if (contentType.contains("json")) {
            try {
                JsonNode node = objectMapper.readTree(resp.body());
                String doc = text(node, "document");
                if (doc != null && !doc.isBlank()) {
                    return new PodDocument(java.util.Base64.getDecoder().decode(doc), "image/jpeg");
                }
            } catch (IOException | IllegalArgumentException e) {
                throw new UberApiException(UberFailureKind.DETERMINISTIC, resp.statusCode(),
                        "UBER_POD_MALFORMED", "POD 回應無法解析", e);
            }
        }
        return new PodDocument(resp.body(), contentType);
    }

    // ==================== 內部：呼叫與分流 ====================

    private JsonNode callJson(String endpointCode, String method, String url, JsonNode body,
                              Duration deadline, long[] backoffMs) {
        UberHttpTransport.UberHttpResponse resp =
                callRaw(endpointCode, method, url, body == null ? null : body.toString(), deadline, backoffMs);
        if (resp.body() == null || resp.body().length == 0) {
            return objectMapper.createObjectNode();
        }
        try {
            return objectMapper.readTree(resp.body());
        } catch (IOException e) {
            throw new UberApiException(UberFailureKind.DETERMINISTIC, resp.statusCode(),
                    "UBER_RESPONSE_MALFORMED", "Uber 回應不是合法 JSON（" + endpointCode + "）", e);
        }
    }

    /**
     * 送出並重試。
     *
     * <p>🔴 <b>只有 {@link UberFailureKind#retryable()} 才重試</b>：確定性錯誤重送同一份 payload
     * 只會再錯一次（66 §7.6），而多送一次有副作用的請求＝多一次建出第二張單的機會。
     */
    private UberHttpTransport.UberHttpResponse callRaw(String endpointCode, String method, String url,
                                                       String jsonBody, Duration deadline, long[] backoffMs) {
        UberApiException last = null;
        for (int attempt = 0; attempt <= backoffMs.length; attempt++) {
            if (attempt > 0) {
                sleepQuietly(backoffMs[attempt - 1]);
            }
            try {
                return sendOnce(endpointCode, method, url, jsonBody, deadline);
            } catch (UberApiException e) {
                last = e;
                if (!e.kind().retryable()) {
                    throw e;
                }
                log.warn("UberApiClient.retry",
                        "Uber " + endpointCode + " 非確定性失敗，準備重試",
                        "attempt", String.valueOf(attempt + 1));
            }
        }
        throw last;
    }

    private UberHttpTransport.UberHttpResponse sendOnce(String endpointCode, String method, String url,
                                                        String jsonBody, Duration deadline) {
        Map<String, String> headers = new LinkedHashMap<>();
        // 🔴 token 取不到就直接拋（fail-closed，66 §4.5.1）——絕不帶空字串上路。
        headers.put("Authorization", "Bearer " + tokenProvider.bearerToken());
        headers.put("Accept", "application/json");
        if (jsonBody != null) {
            headers.put("Content-Type", "application/json");
        }
        UberHttpTransport.UberHttpResponse resp;
        try {
            resp = transport.send(method, url, headers, jsonBody, deadline);
        } catch (IOException e) {
            // 🔴 逾時／連線失敗 ＝ **非確定性**：請求可能已經到達對方並產生副作用。
            //    當成 400 處理就是陷阱 A8 —— 兩趟車、重複計費。
            throw new UberApiException(UberFailureKind.NON_DETERMINISTIC, 0, "UBER_IO",
                    "Uber " + endpointCode + " 呼叫逾時或連線失敗", e);
        }
        if (resp.isSuccess()) {
            return resp;
        }
        throw classify(endpointCode, resp);
    }

    /**
     * HTTP 回應 → {@link UberFailureKind}（66 §4.5.4 錯誤碼映射表）。
     *
     * <p>🔴 <b>400 的三個分支不可合併</b>：{@code noncancelable_delivery} 要留原狀態、
     * {@code customer_blocked}（403）要標門市 ＋ 通知賣家、其他 400 才是 {@code 990}。
     */
    UberApiException classify(String endpointCode, UberHttpTransport.UberHttpResponse resp) {
        int status = resp.statusCode();
        String uberCode = null;
        String uberMessage = null;
        try {
            JsonNode node = objectMapper.readTree(resp.body());
            uberCode = text(node, "code");
            uberMessage = text(node, "message");
        } catch (Exception ignored) {
            // 錯誤 body 解析不出來不影響分流——分流的主依據是 HTTP status。
        }
        String detail = "Uber " + endpointCode + " 回 " + status
                + (uberCode == null ? "" : "（code=" + uberCode + "）");

        if (status >= 500) {
            // 🔴 非確定性 ⇒ 重試耗盡後 991 CREATE_UNKNOWN，**不是 990**（66 §4.5.4）
            return new UberApiException(UberFailureKind.NON_DETERMINISTIC, status, uberCode, detail);
        }
        if (status == 429) {
            return new UberApiException(UberFailureKind.RATE_LIMITED, status, uberCode, detail);
        }
        if (status == 404) {
            return new UberApiException(UberFailureKind.NOT_FOUND, status, uberCode, detail);
        }
        if ("customer_blocked".equals(uberCode)) {
            // 官方以 403 回這個 code；不硬綁 status 以免對方調整時整格失效。
            return new UberApiException(UberFailureKind.CUSTOMER_BLOCKED, status, uberCode, detail);
        }
        if ("noncancelable_delivery".equals(uberCode)) {
            return new UberApiException(UberFailureKind.NONCANCELABLE, status, uberCode, detail);
        }
        if (status == 401 || status == 403) {
            return new UberApiException(UberFailureKind.AUTH, status, uberCode, detail);
        }
        // 其他 4xx（含 400）：確定性 ⇒ 不重試 ⇒ CREATE_FAILED(990)
        return new UberApiException(UberFailureKind.DETERMINISTIC, status, uberCode,
                detail + (uberMessage == null ? "" : "：" + uberMessage));
    }

    // ==================== 內部：payload 組裝與解析 ====================

    private void putWaypoint(ObjectNode body, String prefix, UberWaypoint wp) {
        if (wp == null) {
            return;
        }
        // 🔴 address 是**結構化物件**（66 §6.5「本節最大落差」），不是單一字串。
        if (wp.structuredAddressJson() != null && !wp.structuredAddressJson().isBlank()) {
            try {
                body.set(prefix + "_address", objectMapper.readTree(wp.structuredAddressJson()));
            } catch (IOException e) {
                throw new IllegalArgumentException(
                        prefix + "_address 不是合法的結構化地址 JSON——地址結構化失敗必須在報價前置"
                                + "以 ADDRESS_UNPARSEABLE 攔下，不得帶著壞值打 Uber", e);
            }
        }
        body.put(prefix + "_name", wp.name());
        // 🔴 09xxxxxxxx → +8869xxxxxxxx（66 §6.5）
        body.put(prefix + "_phone_number", UberFieldFormat.toE164(wp.phone()));
        if (wp.latitude() != null) {
            body.put(prefix + "_latitude", wp.latitude());
        }
        if (wp.longitude() != null) {
            body.put(prefix + "_longitude", wp.longitude());
        }
        // 🔴 樓層／門牌補充進 notes **不進 address**——塞進 street 會讓地理編碼失敗（66 §6.5）
        if (wp.notes() != null && !wp.notes().isBlank()) {
            body.put(prefix + "_notes", wp.notes());
        }
    }

    private ArrayNode manifestItems(List<ManifestItem> items) {
        ArrayNode arr = objectMapper.createArrayNode();
        if (items == null) {
            return arr;
        }
        for (ManifestItem item : items) {
            ObjectNode n = objectMapper.createObjectNode();
            n.put("name", item.name());
            n.put("quantity", item.quantity());
            if (item.size() != null) {
                // 🔴 官方是 string enum（U-K），不是向量
                n.put("size", item.size());
            }
            if (item.priceMinorUnits() != null) {
                n.put("price", item.priceMinorUnits());
            }
            if (item.externalId() != null) {
                n.put("external_id", item.externalId());
            }
            arr.add(n);
        }
        // 🔴 溫層**不送、不假裝**：Uber API 完全沒有溫層欄位（U-M），溫層只活在我方（66 §6.5）。
        return arr;
    }

    private DeliveryResult toDeliveryResult(JsonNode n) {
        List<String> related = new ArrayList<>();
        JsonNode rel = n.path("related_deliveries");
        if (rel.isArray()) {
            for (JsonNode r : rel) {
                String id = r.isTextual() ? r.asText() : text(r, "id");
                if (id != null) {
                    related.add(id);
                }
            }
        }
        JsonNode courier = n.path("courier");
        if (courier.isMissingNode() || courier.isNull()) {
            // Eats deliveries 家族把外送員放在 courier_trips[].courier（總覽 §3.1）。
            courier = n.path("courier_trips").path(0).path("courier");
        }
        return new DeliveryResult(
                text(n, "id"),
                text(n, "status"),
                // 🔴 boolean 欄位不是第 8 個狀態（U-I）
                n.path("courier_imminent").asBoolean(false),
                firstNonNull(text(n, "external_id"), text(n, "external_order_id")),
                firstNonNull(text(n, "tracking_url"), text(n, "order_tracking_url")),
                text(courier.path("contact"), "first_name") != null
                        ? text(courier.path("contact"), "first_name")
                        : text(courier, "name"),
                firstNonNull(text(courier.path("contact"), "phone"), text(courier, "phone_number")),
                parseInstant(firstNonNull(text(n, "updated"), text(n, "created"))),
                text(n, "undeliverable_reason"),
                firstNonNull(text(n.path("cancellation_details").path("reason"), "reason_code"),
                        text(n, "cancelation_reason")),
                List.copyOf(related));
    }

    private static String text(JsonNode node, String field) {
        JsonNode v = node.path(field);
        return v.isMissingNode() || v.isNull() ? null : v.asText();
    }

    private static String firstNonNull(String a, String b) {
        return a != null && !a.isBlank() ? a : b;
    }

    private static Instant parseInstant(String raw) {
        if (raw == null || raw.isBlank()) {
            // 🔴 解析不出來就回 null，**不猜一個時間**：一個編出來的 provider 時間
            //    會讓亂序收斂（66 §6.3.5「provider timestamp 較新者為準」）拿錯的值去比。
            return null;
        }
        try {
            return Instant.parse(raw);
        } catch (RuntimeException e) {
            return null;
        }
    }

    private String customerUrl(String customerId) {
        // ⚠️ customer_id 是**路徑參數**不是 header（總覽 §1.3）。
        return baseUrl + "/customers/" + enc(customerId);
    }

    private static String enc(String v) {
        return URLEncoder.encode(v == null ? "" : v, StandardCharsets.UTF_8);
    }

    private static String stripTrailingSlash(String url) {
        if (url == null) {
            return "";
        }
        return url.endsWith("/") ? url.substring(0, url.length() - 1) : url;
    }

    private void sleepQuietly(long millis) {
        try {
            sleeper.sleep(millis);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new UberApiException(UberFailureKind.NON_DETERMINISTIC, 0, "UBER_INTERRUPTED",
                    "Uber 呼叫重試等待被中斷", e);
        }
    }
}
