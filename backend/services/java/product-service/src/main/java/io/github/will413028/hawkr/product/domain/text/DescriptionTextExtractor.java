package io.github.will413028.hawkr.product.domain.text;

import io.github.will413028.hawkr.product.entity.ProductEntity;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import java.util.List;

/**
 * 從 description JSONB 萃取純文字（02 設計書 §3.6 / §3.2 pure_description）。
 *
 * <p>萃取規則（inline-run 模型）：
 * <ul>
 *   <li>按 {@code blocks} 陣列順序遍歷</li>
 *   <li>{@code type=text} → 串接每個 run 的 {@code text}（含連結文字）；runs 缺時 fallback legacy {@code content}</li>
 *   <li>{@code type=table} → 依 row / cell 順序串接每個 cell 每個 run 的 {@code text}</li>
 *   <li>{@code type=list} → 依 item 順序串接每個 item 每個 run 的 {@code text}</li>
 *   <li>{@code type=image} → 若 {@code alt} 非空字串，取 {@code alt} 納入</li>
 *   <li>各片段以單一空格串接，最終整體 trim</li>
 * </ul>
 */
public final class DescriptionTextExtractor {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    private DescriptionTextExtractor() {}

    public static String extract(ProductEntity.ProductDescription description) {
        if (description == null || description.getBlocks() == null || description.getBlocks().isEmpty()) {
            return "";
        }
        StringBuilder sb = new StringBuilder();
        for (ProductEntity.DescriptionBlock block : description.getBlocks()) {
            if (block == null || block.getType() == null) continue;
            switch (block.getType()) {
                case "text" -> appendRunsOrContent(sb, block.getRuns(), block.getContent());
                case "table" -> appendTable(sb, block.getTableJson());
                case "list" -> appendList(sb, block.getListJson());
                case "image" -> appendFragment(sb, block.getAlt());
                default -> { /* 未知 type 略過 */ }
            }
        }
        return sb.toString().trim();
    }

    /** text block：串接每個 run 的 text；runs 缺時 fallback legacy content。 */
    private static void appendRunsOrContent(StringBuilder sb, List<ProductEntity.InlineRun> runs, String content) {
        if (runs != null && !runs.isEmpty()) {
            for (ProductEntity.InlineRun run : runs) {
                if (run != null) appendFragment(sb, run.getText());
            }
        } else {
            appendFragment(sb, content);
        }
    }

    /** table block：解析 table_json，依 row / cell 順序串接每個 cell 每個 run 的 text。 */
    private static void appendTable(StringBuilder sb, String tableJson) {
        if (tableJson == null || tableJson.isBlank()) return;
        try {
            JsonNode root = MAPPER.readTree(tableJson);
            JsonNode rows = root.path("rows");
            if (!rows.isArray()) return;
            for (JsonNode row : rows) {
                if (!row.isArray()) continue;
                for (JsonNode cell : row) {
                    JsonNode cellRuns = cell.path("runs");
                    if (!cellRuns.isArray()) continue;
                    for (JsonNode run : cellRuns) {
                        appendFragment(sb, run.path("text").asText(null));
                    }
                }
            }
        } catch (Exception e) {
            // table_json 解析失敗不阻斷萃取（後端 opaque，容錯）
        }
    }

    /** list block：解析 list_json，依 item 順序串接每個 item 每個 run 的 text。 */
    private static void appendList(StringBuilder sb, String listJson) {
        if (listJson == null || listJson.isBlank()) return;
        try {
            JsonNode root = MAPPER.readTree(listJson);
            JsonNode items = root.path("items");
            if (!items.isArray()) return;
            for (JsonNode item : items) {
                JsonNode itemRuns = item.path("runs");
                if (!itemRuns.isArray()) continue;
                for (JsonNode run : itemRuns) {
                    appendFragment(sb, run.path("text").asText(null));
                }
            }
        } catch (Exception e) {
            // list_json 解析失敗不阻斷萃取（後端 opaque，容錯）
        }
    }

    /** 單一片段 trim 後非空則以單一空格串接。 */
    private static void appendFragment(StringBuilder sb, String raw) {
        if (raw == null) return;
        String t = raw.trim();
        if (t.isEmpty()) return;
        if (sb.length() > 0) sb.append(' ');
        sb.append(t);
    }
}
