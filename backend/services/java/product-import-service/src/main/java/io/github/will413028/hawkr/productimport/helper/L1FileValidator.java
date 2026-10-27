package io.github.will413028.hawkr.productimport.helper;

import io.github.will413028.hawkr.common.utils.code.ImportItemErrorType;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.charset.CharacterCodingException;
import java.nio.charset.CodingErrorAction;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * L1 同步驗證器（40 號設計書 §11.3 Layer 1）。
 *
 * <p>檔案結構層：編碼 / size / header / row 數 / BOM。L1 fail → 400 不建 job。
 */
@Component
public class L1FileValidator {

    private static final Set<String> REQUIRED_HEADERS = Set.of(
            "product_key", "product_name", "category_id", "product_image_1",
            "sku_price", "sku_stock_qty", "sku_weight", "sku_unit",
            "sku_length_cm", "sku_width_cm", "sku_height_cm"
    );

    private static final Set<String> SHIPPING_HEADERS = Set.of(
            "sku_shipping_taqbin", "sku_shipping_seven", "sku_shipping_uber"
    );

    @Value("${hawkr.product-import.csv.max-file-size-bytes:2097152}")
    private long maxFileSizeBytes;

    @Value("${hawkr.product-import.csv.max-rows:500}")
    private int maxRows;

    private final CsvParser csvParser;

    public L1FileValidator(CsvParser csvParser) {
        this.csvParser = csvParser;
    }

    public L1Result validate(byte[] content) {
        List<Map<String, Object>> errors = new ArrayList<>();

        if (content == null || content.length == 0) {
            errors.add(err(0, "file", ImportItemErrorType.REQUIRED_MISSING.value(), "CSV 內容為空", ""));
            return new L1Result(false, errors);
        }
        if (content.length > maxFileSizeBytes) {
            errors.add(err(0, "file", ImportItemErrorType.REQUIRED_MISSING.value(),
                    "檔案超過上限 " + maxFileSizeBytes + " bytes", String.valueOf(content.length)));
            return new L1Result(false, errors);
        }
        if (!isValidUtf8(content)) {
            errors.add(err(0, "file", ImportItemErrorType.ENCODING_NOT_UTF8.value(),
                    "CSV 編碼非 UTF-8", ""));
            return new L1Result(false, errors);
        }

        try {
            CsvParser.HeaderProbe probe = csvParser.scanHeaderAndRowCount(content);
            List<String> headers = probe.headerNames();
            for (String required : REQUIRED_HEADERS) {
                if (!headers.contains(required)) {
                    errors.add(err(1, required, ImportItemErrorType.REQUIRED_MISSING.value(),
                            "缺必填欄位 " + required, ""));
                }
            }
            boolean anyShipping = SHIPPING_HEADERS.stream().anyMatch(headers::contains);
            if (!anyShipping) {
                errors.add(err(1, "sku_shipping_*",
                        ImportItemErrorType.SHIPPING_AT_LEAST_ONE_REQUIRED.value(),
                        "sku_shipping_taqbin / sku_shipping_seven / sku_shipping_uber 至少需 1 欄 header",
                        ""));
            }
            if (probe.rowCount() == 0) {
                errors.add(err(0, "file", ImportItemErrorType.REQUIRED_MISSING.value(),
                        "CSV 沒有任何 data row", ""));
            }
            if (probe.rowCount() > maxRows) {
                errors.add(err(0, "file", ImportItemErrorType.REQUIRED_MISSING.value(),
                        "row 數超過上限 " + maxRows, String.valueOf(probe.rowCount())));
            }
        } catch (IOException ioe) {
            errors.add(err(0, "file", ImportItemErrorType.REQUIRED_MISSING.value(),
                    "CSV 解析失敗: " + ioe.getMessage(), ""));
        }

        return new L1Result(errors.isEmpty(), errors);
    }

    private static boolean isValidUtf8(byte[] content) {
        try {
            int off = 0;
            if (content.length >= 3
                    && (content[0] & 0xFF) == 0xEF
                    && (content[1] & 0xFF) == 0xBB
                    && (content[2] & 0xFF) == 0xBF) {
                off = 3;
            }
            StandardCharsets.UTF_8.newDecoder()
                    .onMalformedInput(CodingErrorAction.REPORT)
                    .onUnmappableCharacter(CodingErrorAction.REPORT)
                    .decode(java.nio.ByteBuffer.wrap(content, off, content.length - off));
            return true;
        } catch (CharacterCodingException e) {
            return false;
        }
    }

    private static Map<String, Object> err(int line, String field, String errorType, String message, String raw) {
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("csv_line", line);
        m.put("field", field);
        m.put("error_type", errorType);
        m.put("message", message);
        m.put("raw_value", raw);
        return m;
    }

    public record L1Result(boolean passed, List<Map<String, Object>> errors) {}
}
