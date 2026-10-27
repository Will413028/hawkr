package io.github.will413028.hawkr.shipping.provider.uber;

import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.ManifestItem;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.UberWaypoint;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.io.IOException;
import java.util.ArrayList;
import java.util.List;

/**
 * {@code uber_quotes.pickup_json}／{@code dropoff_json}／{@code items_json} 的<b>唯一</b>編解碼。
 *
 * <h2>🔴 為什麼這一支必須存在（而不是兩邊各自 {@code readTree}）</h2>
 * 這三欄是 JSONB，schema 只規定「是 JSON」，<b>沒有規定欄名</b>。
 * 寫入者是報價路徑（結帳）、讀取者是建單路徑（本 adapter）——
 * 兩邊各自手寫一份「我覺得對方會寫成這樣」的形狀，就是
 * {@code 03-delegation-templates.md} §0.3 講的「自己跟自己對，必然全綠」：
 * 欄名寫錯不會編譯錯、不會測試紅，症狀是<b>建單時取件地址靜默全空</b>。
 *
 * <p>⇒ 讓那份形狀只有<b>一個定義</b>：兩邊都呼叫本類別。
 * ⚠️ 報價路徑（W2 的另一條 lane）落地時<b>必須</b>用 {@link #waypointToJson}／{@link #itemsToJson}
 * 寫入，不得自己組 JSON。這一點已列入交付報告的「主對話待處理」。
 *
 * <p>key 一律 <b>camelCase</b>（10 檔 §11.1 JSONB 慣例），與 Uber wire 的 snake_case 分開
 * ——那是兩件事：本類別是<b>我方的持久化形狀</b>，wire 形狀在 {@link UberApiClient} 內組。
 */
public final class UberPayloadCodec {

    private final ObjectMapper objectMapper;

    public UberPayloadCodec(ObjectMapper objectMapper) {
        this.objectMapper = objectMapper;
    }

    public String waypointToJson(UberWaypoint wp) {
        ObjectNode n = objectMapper.createObjectNode();
        // structuredAddress 本身是物件，不是字串——存成字串會變成 JSON-in-JSON，
        // 而那種資料一旦落庫就只能靠 unescape 讀，任何 SQL 查詢都對它無能為力。
        n.set("structuredAddress", parseOrNull(wp.structuredAddressJson()));
        n.put("name", wp.name());
        n.put("phone", wp.phone());
        if (wp.latitude() != null) {
            n.put("latitude", wp.latitude());
        }
        if (wp.longitude() != null) {
            n.put("longitude", wp.longitude());
        }
        n.put("notes", wp.notes());
        return n.toString();
    }

    /**
     * @throws IllegalArgumentException JSON 壞掉。🔴 <b>不回 null</b>：
     *         一個欄位全空的 waypoint 打到 Uber 只會拿到 400，而錯誤訊息會指向 Uber
     *         ——真正的原因是我方的持久化壞了
     */
    public UberWaypoint waypointFromJson(String json) {
        JsonNode n = requireObject(json, "waypoint");
        JsonNode addr = n.path("structuredAddress");
        return new UberWaypoint(
                addr.isMissingNode() || addr.isNull() ? null : addr.toString(),
                text(n, "name"),
                text(n, "phone"),
                n.hasNonNull("latitude") ? n.get("latitude").asDouble() : null,
                n.hasNonNull("longitude") ? n.get("longitude").asDouble() : null,
                text(n, "notes"));
    }

    public String itemsToJson(List<ManifestItem> items) {
        ArrayNode arr = objectMapper.createArrayNode();
        if (items != null) {
            for (ManifestItem item : items) {
                ObjectNode n = objectMapper.createObjectNode();
                n.put("name", item.name());
                n.put("quantity", item.quantity());
                n.put("size", item.size());
                if (item.priceMinorUnits() != null) {
                    n.put("priceMinorUnits", item.priceMinorUnits());
                }
                n.put("externalId", item.externalId());
                arr.add(n);
            }
        }
        return arr.toString();
    }

    public List<ManifestItem> itemsFromJson(String json) {
        List<ManifestItem> out = new ArrayList<>();
        if (json == null || json.isBlank()) {
            return out;
        }
        JsonNode arr;
        try {
            arr = objectMapper.readTree(json);
        } catch (IOException e) {
            throw new IllegalArgumentException("uber_quotes.items_json 不是合法 JSON", e);
        }
        if (!arr.isArray()) {
            throw new IllegalArgumentException("uber_quotes.items_json 必須是陣列");
        }
        for (JsonNode n : arr) {
            out.add(new ManifestItem(
                    text(n, "name"),
                    n.path("quantity").asInt(0),
                    text(n, "size"),
                    n.hasNonNull("priceMinorUnits") ? n.get("priceMinorUnits").asLong() : null,
                    text(n, "externalId")));
        }
        return out;
    }

    private JsonNode parseOrNull(String json) {
        if (json == null || json.isBlank()) {
            return objectMapper.nullNode();
        }
        try {
            return objectMapper.readTree(json);
        } catch (IOException e) {
            throw new IllegalArgumentException("結構化地址不是合法 JSON", e);
        }
    }

    private JsonNode requireObject(String json, String what) {
        if (json == null || json.isBlank()) {
            throw new IllegalArgumentException(what + " JSON 為空");
        }
        try {
            JsonNode n = objectMapper.readTree(json);
            if (!n.isObject()) {
                throw new IllegalArgumentException(what + " JSON 必須是物件");
            }
            return n;
        } catch (IOException e) {
            throw new IllegalArgumentException(what + " JSON 無法解析", e);
        }
    }

    private static String text(JsonNode n, String field) {
        JsonNode v = n.path(field);
        return v.isMissingNode() || v.isNull() ? null : v.asText();
    }
}
