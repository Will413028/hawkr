package io.github.will413028.hawkr.common.webhook;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.List;
import java.util.Map;

/**
 * Uber webhook 驗簽（66 §6.3.1、API 總覽 §6.1）。
 *
 * <h2>🔴 簽的是 raw bytes，不是 re-serialize 後的 JSON</h2>
 * 先 parse 再 re-serialize 會因欄位順序不同而<b>偶發</b>失敗——失敗率取決於資料內容，
 * <b>測不出來、上線才炸</b>（陷阱 A2）。
 *
 * <h2>🔴 為什麼住在 common-core：它有<b>兩個</b>消費者，而且兩個都必須是同一份</h2>
 * <ol>
 *   <li><b>ingress</b>（{@code payuni-webhook-service}）—— 在寫進 {@code webhook_inbox}
 *       <b>之前</b>驗。這一格是 2026-09-12 補回來的，補之前有一個具體的洞：
 *       inbox 有 {@code uq_inbox_event UNIQUE (provider, webhook_type, event_key)}，
 *       而落地在驗簽之前 ⇒ 任何能打到端點的人，搶先送一個<b>同 event_id 的偽造事件</b>，
 *       真正的 Uber 事件到達時就會被判為重複而丟棄。
 *       🔴 那是一個<b>未認證的事件壓制攻擊面</b>：攻擊者不需要偽造得出合法簽章，
 *       只需要搶先佔住那個鍵。</li>
 *   <li><b>adapter</b>（{@code shipping-service} 的 {@code UberDirectAdapter.inbound()}）——
 *       縱深防禦。MQ 進來的事件理論上已經驗過了，但「理論上」不是機制。</li>
 * </ol>
 * ⚠️ <b>兩處必須用同一份實作</b>：安全關鍵的比較邏輯有兩份拷貝時，修了一邊沒修另一邊
 * 是完全無聲的。
 *
 * <p>⚠️ 65 {@code ADR-012} 逐字寫「驗簽從 ingress 下放到 adapter」。**那一句在與
 * ingress 端去重組合起來時會開出上面那個洞**，而 66 §6.3.1／§6.3.4 原本就把驗簽放在
 * ingress。⇒ 2026-09-12 依 66 恢復 ingress 驗簽；{@code ADR-012} 的另一半
 * （raw bytes 必須到得了 adapter）<b>不受影響、照舊成立</b>，本類的入參就是那些 bytes。
 *
 * <h2>🔴 金鑰是 per-webhook signing key，<b>不是</b> OAuth {@code client_secret}</h2>
 * API 總覽 §6.1 逐字：「專屬 Webhook Signing Key（建立 webhook 時發給）」。
 * 66 §8.5：{@code UBER_WEBHOOK_SIGNING_KEYS} 是<b>一組不是一個</b>
 * ——只放一把 ⇒ 多註冊一個 webhook 就靜默驗簽失敗（陷阱 C1），
 * 而失敗的下場是我方回 401 ⇒ Uber 不重試 4xx ⇒ <b>事件全丟</b>，
 * 且 401 在我方看起來像「有人在打我們」。
 *
 * <p>⚠️ 事件本身沒有帶「這是哪一個 webhook 註冊」的識別欄
 * （DaaS payload 頂層欄位見 API 總覽 §7.2，無 webhook id）
 * ⇒ 本驗證器逐把試，任一把成立即通過。所有金鑰都是我方自己註冊的，
 * 「在任一把之下成立」正是我們要的語意。
 *
 * <h2>🔴 constant-time 比較</h2>
 * 用 {@link MessageDigest#isEqual} 而不是 {@link String#equals}／{@code Arrays.equals}：
 * 後者會在第一個不同的位元組就返回，讓攻擊者能以回應時間逐位元組還原簽章。
 */
public final class UberWebhookVerifier {

    /** 主要簽章 header（API 總覽 §6.1；Refund webhook <b>必須</b>用這個）。 */
    public static final String SIGNATURE_HEADER = "x-uber-signature";

    /**
     * 舊版 header：Delivery Status／Courier Update 另接受它（API 總覽 §6.1）。
     *
     * <p>⚠️ 只當 {@link #SIGNATURE_HEADER} 缺席時才讀——Refund webhook 不接受它，
     * 優先序寫反會讓一個本該被拒的簽章通過。
     */
    public static final String LEGACY_SIGNATURE_HEADER = "x-postmates-signature";

    private static final String HMAC_SHA256 = "HmacSHA256";

    private final List<String> signingKeys;

    /**
     * @param signingKeys {@code {webhookId: key}} 的值集合（key 本身）。
     *                    🔴 空集合 ＝ 驗簽必然失敗（fail-closed）；
     *                    <b>不得</b>改成「沒有金鑰就放行」——那等於把 webhook 端點開放給任何人寫我方狀態。
     */
    public UberWebhookVerifier(List<String> signingKeys) {
        this.signingKeys = signingKeys == null ? List.of() : List.copyOf(signingKeys);
    }

    /** 從 {@code {webhookId: key}} 的 map 建立（66 §8.5 的 {@code UBER_WEBHOOK_SIGNING_KEYS} 形狀）。 */
    public static UberWebhookVerifier ofKeyMap(Map<String, String> webhookIdToKey) {
        return new UberWebhookVerifier(webhookIdToKey == null
                ? List.of()
                : webhookIdToKey.values().stream().filter(k -> k != null && !k.isBlank()).toList());
    }

    /**
     * 驗證這一則 webhook 的簽章。
     *
     * @param rawBody provider 送來的<b>原始位元組</b>——不得是 parse 後再 serialize 的版本
     * @param headers HTTP header（大小寫不敏感查找）
     * @return true ＝ 至少一把已註冊的 signing key 算得出同一個簽章
     */
    public boolean verify(byte[] rawBody, Map<String, String> headers) {
        if (rawBody == null || rawBody.length == 0) {
            return false;
        }
        String provided = header(headers, SIGNATURE_HEADER);
        if (provided == null || provided.isBlank()) {
            provided = header(headers, LEGACY_SIGNATURE_HEADER);
        }
        if (provided == null || provided.isBlank() || signingKeys.isEmpty()) {
            return false;
        }
        byte[] providedBytes = decodeHex(provided.trim());
        if (providedBytes == null) {
            return false;
        }
        boolean matched = false;
        for (String key : signingKeys) {
            byte[] expected = hmacSha256(key, rawBody);
            // 🔴 不得在命中時 break：提早退出讓「第幾把命中」變成可觀測的時間差。
            //    逐把都算完的成本是固定的幾微秒，而金鑰只有個位數把。
            matched |= MessageDigest.isEqual(expected, providedBytes);
        }
        return matched;
    }

    /**
     * header 取值，<b>大小寫不敏感</b>。
     *
     * <p>HTTP header 名在規範上大小寫不敏感，而不同的 ingress（Spring／Gin／代理層）
     * 正規化成不同大小寫 ⇒ 用 {@code headers.get("x-uber-signature")} 直接取會
     * <b>偶發</b>拿到 null，而症狀是「驗簽失敗」不是「header 沒讀到」——
     * 兩者的處置完全相反（前者該回 401，後者該去修讀取）。
     */
    private static String header(Map<String, String> headers, String name) {
        if (headers == null) {
            return null;
        }
        for (Map.Entry<String, String> e : headers.entrySet()) {
            if (e.getKey() != null && e.getKey().equalsIgnoreCase(name)) {
                return e.getValue();
            }
        }
        return null;
    }

    private static byte[] hmacSha256(String key, byte[] body) {
        try {
            Mac mac = Mac.getInstance(HMAC_SHA256);
            mac.init(new SecretKeySpec(key.getBytes(StandardCharsets.UTF_8), HMAC_SHA256));
            return mac.doFinal(body);
        } catch (Exception e) {
            // 演算法固定為 JDK 內建，走到這裡代表執行環境本身壞了。
            throw new IllegalStateException("HMAC-SHA256 不可用", e);
        }
    }

    /**
     * 十六進位字串 → bytes；非法輸入回 null（不拋例外——那會把「簽章格式錯」
     * 變成 500，而它其實是「驗簽失敗」，兩者的回應碼不同）。
     *
     * <p>⚠️ 先解碼再用 {@link MessageDigest#isEqual} 比 bytes，而不是比字串：
     * 大小寫差異不該造成偶發失敗，而 {@code equalsIgnoreCase} 不是 constant-time。
     */
    static byte[] decodeHex(String hex) {
        int len = hex.length();
        if (len == 0 || len % 2 != 0) {
            return null;
        }
        byte[] out = new byte[len / 2];
        for (int i = 0; i < len; i += 2) {
            int hi = Character.digit(hex.charAt(i), 16);
            int lo = Character.digit(hex.charAt(i + 1), 16);
            if (hi < 0 || lo < 0) {
                return null;
            }
            out[i / 2] = (byte) ((hi << 4) | lo);
        }
        return out;
    }
}
