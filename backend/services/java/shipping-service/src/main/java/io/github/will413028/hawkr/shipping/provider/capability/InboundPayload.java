package io.github.will413028.hawkr.shipping.provider.capability;

import java.util.Collections;
import java.util.Map;

/**
 * provider 回調的原始承載。
 *
 * <p>🔴 <b>兩種形態並存，不從其中一個推導另一個</b>（65 附錄 {@code ADR-012}）：
 * form-encoded provider（PayUni）填 {@link #formParams()}；JSON provider（Uber）填
 * {@link #rawBody()}。把 JSON 攤平成 {@code Map<String,String>} 會產生「攤平規則」
 * 這個新的 provider-specific 知識，而它無處存放。
 *
 * <h2>為什麼 rawBody 必須到得了 adapter</h2>
 * Uber webhook 的驗簽是 HMAC-SHA256 over <b>raw bytes</b>。先 parse 再 re-serialize
 * 會因欄位順序不同而<b>偶發</b>失敗——失敗率取決於資料內容，測不出來、上線才炸
 * （66 §6.3.1 陷阱 A2）。而「怎麼驗簽」是 provider 專屬知識，正是本介面的職責
 * ⇒ raw bytes 在介面層遺失，驗簽就只能留在 ingress。
 */
public record InboundPayload(Map<String, String> formParams,
                             byte[] rawBody,
                             Map<String, String> headers) {

    public InboundPayload {
        formParams = formParams == null ? Map.of() : Collections.unmodifiableMap(formParams);
        headers = headers == null ? Map.of() : Collections.unmodifiableMap(headers);
    }

    /** form-encoded provider（PayUni）的承載：只有 form params，無 raw body、無 header。 */
    public static InboundPayload ofFormParams(Map<String, String> formParams) {
        return new InboundPayload(formParams, null, Map.of());
    }

    /** JSON provider（Uber）的承載：raw bytes ＋ header（驗簽用），無 form params。 */
    public static InboundPayload ofJson(byte[] rawBody, Map<String, String> headers) {
        return new InboundPayload(Map.of(), rawBody, headers);
    }

    /**
     * header 取值，<b>大小寫不敏感</b>。
     *
     * <p>HTTP header 名在規範上大小寫不敏感，而不同的 ingress（Gin／Spring／代理層）
     * 正規化成不同大小寫 ⇒ 用 {@code headers().get("x-uber-signature")} 直接取會
     * <b>偶發</b>拿到 null，而症狀是「驗簽失敗」而不是「header 沒讀到」。
     */
    public String header(String name) {
        if (name == null) {
            return null;
        }
        for (Map.Entry<String, String> e : headers.entrySet()) {
            if (e.getKey() != null && e.getKey().equalsIgnoreCase(name)) {
                return e.getValue();
            }
        }
        return null;
    }

    public boolean hasRawBody() {
        return rawBody != null && rawBody.length > 0;
    }
}
