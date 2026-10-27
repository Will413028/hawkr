package io.github.will413028.hawkr.productimport.helper;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * 40 號設計書 §4.3a / §4.6：把多筆 violations 按 (field, code) 聚合為對賣家友善的顯示訊息。
 *
 * <p>典型場景：3 筆 (field=gallery_images, code=GALLERY_REQUIRED) 同時出現於同一 item
 * → 不重複顯示「商品圖至少需 1 張 / 商品圖至少需 1 張 / 商品圖至少需 1 張」，
 * 而是合併為一筆 `{field=gallery_images, fieldLabel=商品圖, code=GALLERY_REQUIRED,
 * message=商品圖至少需 1 張, occurrences=3}`。
 *
 * <p>使用慣例：
 * <ul>
 *   <li>{@link Violation} 為純資料；caller（service 層）從 {@code product_import_item_error} 或
 *       {@code ValidateProductDraftResponse.violations} 拼出 List 後丟給 {@link #buildMessages}</li>
 *   <li>輸入 violations 已自帶中文 message → builder 優先採用；fallback 才走
 *       {@link ImportErrorMessageCatalog#messageTemplate(String, String)}</li>
 *   <li>buildMessages 對相同 (field, code) 合併並計算 occurrences；不同 severity 視為不同分組</li>
 * </ul>
 */
public final class ImportErrorMessageBuilder {

    private ImportErrorMessageBuilder() {}

    /**
     * 把 violations 群組為 displayable 訊息。
     *
     * @param violations 來源 violations（可能含多筆同 (field, code)）
     * @return 聚合後依「首次出現」順序回傳
     */
    public static List<DisplayMessage> buildMessages(List<Violation> violations) {
        if (violations == null || violations.isEmpty()) return List.of();
        // 用 LinkedHashMap 維持出現順序，避免前端看到的訊息順序與 DB 不同造成困惑
        Map<String, Acc> acc = new LinkedHashMap<>();
        for (Violation v : violations) {
            if (v == null) continue;
            String severity = v.severity() == null ? "ERROR" : v.severity();
            String field = v.field() == null ? "" : v.field();
            String code = v.code() == null ? "" : v.code();
            String key = severity + "|" + field + "|" + code;
            Acc a = acc.computeIfAbsent(key, k -> {
                String msg = (v.message() != null && !v.message().isBlank())
                        ? v.message()
                        : ImportErrorMessageCatalog.messageTemplate(field, code);
                if (msg == null || msg.isBlank()) msg = code; // 最後 fallback 給 code，避免空字串
                return new Acc(field, ImportErrorMessageCatalog.fieldLabel(field), severity, code, msg);
            });
            a.occurrences++;
        }
        List<DisplayMessage> out = new ArrayList<>(acc.size());
        for (Acc a : acc.values()) {
            out.add(new DisplayMessage(a.field, a.fieldLabel, a.severity, a.code, a.message, a.occurrences));
        }
        return out;
    }

    /** Builder 累積中間結構（mutable）。 */
    private static final class Acc {
        final String field;
        final String fieldLabel;
        final String severity;
        final String code;
        final String message;
        int occurrences;
        Acc(String field, String fieldLabel, String severity, String code, String message) {
            this.field = field;
            this.fieldLabel = fieldLabel;
            this.severity = severity;
            this.code = code;
            this.message = message;
        }
    }

    /**
     * 來源 violation（對齊 {@code DraftViolation} proto + {@code product_import_item_error} entity）。
     * 任一欄位可為 null；builder 內會做 null-safe 處理。
     */
    public record Violation(String field, String code, String severity, String message, String rawValue) {}

    /** 聚合後的顯示訊息。 */
    public record DisplayMessage(String field, String fieldLabel, String severity, String code,
                                  String message, int occurrences) {}
}
