package io.github.will413028.hawkr.common.mq;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import java.nio.charset.StandardCharsets;
import java.util.Base64;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.Map;

/**
 * provider-neutral 的 webhook 傳輸信封（webhook ingress → MQ → 各 domain consumer）。
 *
 * <h2>為什麼不是擴充 {@link PayUniWebhookEnvelope}</h2>
 * 後者的 {@code payload} 型別是 {@code Map<String,String>} ——
 * 那是 <b>PayUni form-encoded 的形狀</b>，而
 * <ul>
 *   <li>巢狀 JSON（Uber 的 {@code courier}／{@code manifest} 物件、{@code related_deliveries}
 *       陣列）塞不進去；攤平會產生「攤平規則」這個新的 provider 知識，而它無處存放</li>
 *   <li><b>raw bytes 在第一跳就遺失</b>，而 JSON provider 的驗簽是 HMAC-SHA256 over raw body</li>
 * </ul>
 * 而 {@code PayUniWebhookEnvelope} 同時是付款與貨態兩條熱路徑的 wire 契約，
 * 改它的欄位型別會動到不在本波範圍的 consumer ⇒ 新增一個 record，不動舊的。
 *
 * <h2>🔴 {@code rawBodyBase64} 為什麼是 base64 而不是字串</h2>
 * 驗簽對的是<b>那些位元組</b>。JSON body 可以帶任意位元組（{@code &#92;uXXXX} 逸出、
 * 非 NFC 的 Unicode、CRLF 與縮排），經過 {@code String} 來回、或經過
 * 「parse 成 JsonNode 再 re-serialize」都會改變它 ——
 * 而那種失敗是<b>偶發</b>的（失敗率取決於資料內容的欄位順序）⇒ 測不出來、上線才炸
 * （66 §6.3.1 陷阱 A2）。base64 是唯一能在 JSON 字串欄位裡無損攜帶任意位元組的形式。
 *
 * <h2>🔴 producer 與 consumer 共用同一份鍵名</h2>
 * {@link #toWireMap()}（producer 側）與 {@link #parse(byte[])}（consumer 側）
 * 讀的是本類別的同一組 {@code KEY_*} 常數。
 * {@code PayUniWebhookEnvelope} 的 producer 自己另寫了一份 {@code Map.of("event_id", ...)}，
 * 該檔的註解逐字寫著「本方法的鍵名若與 parse() 不一致，consumer 讀到的 metadata 會全空<b>且不報錯</b>」
 * —— 本類別把兩側收在同一個型別裡，讓那個 bug class 沒有地方可以發生。
 *
 * <p>envelope 層欄位一律 snake_case（36 §7.4 Conformance Contract）。
 */
public record ProviderWebhookEnvelope(
        String provider,
        String webhookType,
        String eventKey,
        Map<String, String> formParams,
        String rawBodyBase64,
        Map<String, String> headers,
        String receivedAt
) {

    /** {@code WebhookProvider} 的 <b>Value</b>（數字碼字串），不是 Name。 */
    public static final String KEY_PROVIDER = "provider";
    /** provider 原生事件型別（Uber 的 {@code kind}／PayUni 的 {@code PayUniEventType.value()}）。 */
    public static final String KEY_WEBHOOK_TYPE = "webhook_type";
    /** ingress 端算好的去重鍵；與 {@code webhook_inbox.event_key} 同值。 */
    public static final String KEY_EVENT_KEY = "event_key";
    /** form-encoded provider 的欄位；JSON provider 為空 map。 */
    public static final String KEY_FORM_PARAMS = "form_params";
    /** JSON provider 的原始位元組（base64，標準字母表含 padding）。 */
    public static final String KEY_RAW_BODY_BASE64 = "raw_body_base64";
    /** 驗簽 header（原樣轉發，大小寫由 {@code InboundPayload.header(name)} 處理）。 */
    public static final String KEY_HEADERS = "headers";
    /** ingress 收件時間，ISO 8601 UTC。 */
    public static final String KEY_RECEIVED_AT = "received_at";

    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final TypeReference<Map<String, String>> MAP_TYPE = new TypeReference<>() {};

    public ProviderWebhookEnvelope {
        formParams = formParams == null ? Map.of() : Collections.unmodifiableMap(new LinkedHashMap<>(formParams));
        headers = headers == null ? Map.of() : Collections.unmodifiableMap(new LinkedHashMap<>(headers));
    }

    /**
     * JSON provider（Uber）的信封。
     *
     * @param rawBody provider 送來的<b>原始位元組</b>。呼叫端不得先 parse 再 re-serialize；
     *                拿到的必須是 HTTP body 的那一份（Spring 側用 {@code @RequestBody byte[]}）。
     */
    public static ProviderWebhookEnvelope ofJson(String provider, String webhookType, String eventKey,
                                                 byte[] rawBody, Map<String, String> headers,
                                                 String receivedAt) {
        return new ProviderWebhookEnvelope(provider, webhookType, eventKey, Map.of(),
                encode(rawBody), headers, receivedAt);
    }

    /** form-encoded provider（PayUni 形狀）的信封：只有 form params，無 raw body、無 header。 */
    public static ProviderWebhookEnvelope ofFormParams(String provider, String webhookType, String eventKey,
                                                       Map<String, String> formParams, String receivedAt) {
        return new ProviderWebhookEnvelope(provider, webhookType, eventKey, formParams,
                null, Map.of(), receivedAt);
    }

    /** producer 側：序列化前的 map。鍵名來自本類別的 {@code KEY_*}，與 {@link #parse(byte[])} 同源。 */
    public Map<String, Object> toWireMap() {
        Map<String, Object> m = new LinkedHashMap<>();
        m.put(KEY_PROVIDER, provider);
        m.put(KEY_WEBHOOK_TYPE, webhookType);
        m.put(KEY_EVENT_KEY, eventKey);
        m.put(KEY_FORM_PARAMS, formParams);
        m.put(KEY_RAW_BODY_BASE64, rawBodyBase64);
        m.put(KEY_HEADERS, headers);
        m.put(KEY_RECEIVED_AT, receivedAt);
        return m;
    }

    /**
     * consumer 側：從 MQ 消息 body 解析。
     *
     * <p>🔴 <b>刻意沒有「裸 payload」相容分支</b>：這是本波新增的契約，沒有舊格式要相容。
     * 加一條猜測性的 fallback，代價是「鍵名寫錯」從 exception 退化成靜默的空值。
     *
     * @throws IllegalArgumentException body 為空，或缺少 {@link #KEY_PROVIDER}
     */
    public static ProviderWebhookEnvelope parse(byte[] body) {
        if (body == null || body.length == 0) {
            throw new IllegalArgumentException("ProviderWebhookEnvelope: MQ 消息 body 不得為空");
        }
        JsonNode root;
        try {
            root = MAPPER.readTree(new String(body, StandardCharsets.UTF_8));
        } catch (Exception e) {
            throw new IllegalArgumentException("ProviderWebhookEnvelope: JSON 解析失敗: " + e.getMessage(), e);
        }
        if (!root.hasNonNull(KEY_PROVIDER)) {
            throw new IllegalArgumentException(
                    "ProviderWebhookEnvelope: 缺少必要欄位 " + KEY_PROVIDER + " —— 這不是本契約的訊息");
        }
        return new ProviderWebhookEnvelope(
                root.path(KEY_PROVIDER).asText(""),
                root.path(KEY_WEBHOOK_TYPE).asText(""),
                root.path(KEY_EVENT_KEY).asText(""),
                readMap(root, KEY_FORM_PARAMS),
                root.hasNonNull(KEY_RAW_BODY_BASE64) ? root.path(KEY_RAW_BODY_BASE64).asText() : null,
                readMap(root, KEY_HEADERS),
                root.path(KEY_RECEIVED_AT).asText(""));
    }

    /**
     * 還原 provider 送來的原始位元組。
     *
     * @return 無 raw body 時回 {@code null}（form-encoded provider 的正常狀態）
     * @throws IllegalArgumentException base64 內容毀損 —— 🔴 <b>不得回 null 吞掉</b>：
     *         那會讓驗簽失敗被誤讀成「金鑰錯」
     */
    public byte[] rawBody() {
        if (rawBodyBase64 == null || rawBodyBase64.isEmpty()) {
            return null;
        }
        try {
            return Base64.getDecoder().decode(rawBodyBase64);
        } catch (IllegalArgumentException e) {
            throw new IllegalArgumentException(
                    "ProviderWebhookEnvelope: " + KEY_RAW_BODY_BASE64 + " 不是合法 base64", e);
        }
    }

    public boolean hasRawBody() {
        return rawBodyBase64 != null && !rawBodyBase64.isEmpty();
    }

    private static String encode(byte[] rawBody) {
        return rawBody == null ? null : Base64.getEncoder().encodeToString(rawBody);
    }

    private static Map<String, String> readMap(JsonNode root, String key) {
        JsonNode node = root.path(key);
        if (node.isMissingNode() || node.isNull()) {
            return Map.of();
        }
        return MAPPER.convertValue(node, MAP_TYPE);
    }
}
