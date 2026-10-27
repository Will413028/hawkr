package io.github.will413028.hawkr.productimport.helper;

import io.github.will413028.hawkr.common.utils.code.ImportItemErrorType;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class ImportErrorMessageCatalogTest {

    /**
     * 強制 enum-driven coverage：每一個 {@link ImportItemErrorType} 必有對應 code-only fallback。
     * 新增 type 後若漏補 catalog，本 test 失敗以阻擋技術文案外洩到 UI。
     *
     * <p>白名單豁免 type：{@code HTML_PARSE_WARNING} / {@code GALLERY_TRUNCATED_EXCEEDS_LIMIT}
     * 等通常 validator 自帶完整中文 message 的，不在此強制；但仍建議補。
     */
    @Test
    void everyImportItemErrorTypeHasCodeOnlyCatalogEntry() {
        List<String> missing = new ArrayList<>();
        for (ImportItemErrorType t : ImportItemErrorType.values()) {
            // VALIDATION_DRIFT、IMPORT_ITEM_ALREADY_COMMITTED 已有 entry；HTML_PARSE_WARNING / GALLERY_TRUNCATED_EXCEEDS_LIMIT
            // 為 warning 類已有 field-specific entry，code-only 不強制。
            if (t == ImportItemErrorType.HTML_PARSE_WARNING
                    || t == ImportItemErrorType.GALLERY_TRUNCATED_EXCEEDS_LIMIT
                    || t == ImportItemErrorType.VALIDATION_DRIFT) {
                continue;
            }
            String template = ImportErrorMessageCatalog.messageTemplate(null, t.value());
            if (template == null || template.isBlank()) {
                missing.add(t.value());
            }
        }
        assertThat(missing)
                .as("ImportItemErrorType 缺 catalog entry（請在 ImportErrorMessageCatalog 補 put(null, \"CODE\", \"中文\") ）")
                .isEmpty();
    }

    @Test
    void displayMessageUsesFriendlyDescriptionMessageForInternalParserErrors() {
        String message = ImportErrorMessageCatalog.displayMessage(
                "description_json",
                "DESC_BLOCK_INVALID",
                "description_json 解析失敗：Protocol message end-group tag did not match expected tag.");

        assertThat(message).isEqualTo("商品描述 blocks 格式不合法");
    }

    @Test
    void nestedDescriptionFieldsUseDescriptionLabelAndTemplate() {
        assertThat(ImportErrorMessageCatalog.fieldLabel("description_json.blocks[0].type"))
                .isEqualTo("商品描述");
        assertThat(ImportErrorMessageCatalog.messageTemplate(
                "description_json.blocks[0].type", "DESC_BLOCK_INVALID"))
                .isEqualTo("商品描述 blocks 格式不合法");
    }

    @Test
    void skusRequiredUsesSellerFriendlyText() {
        assertThat(ImportErrorMessageCatalog.fieldLabel("skus"))
                .isEqualTo("商品規格");
        assertThat(ImportErrorMessageCatalog.displayMessage(
                "skus", "SKUS_REQUIRED", "至少需要 1 個 SKU"))
                .isEqualTo("至少需要一組商品規格");
    }
}
