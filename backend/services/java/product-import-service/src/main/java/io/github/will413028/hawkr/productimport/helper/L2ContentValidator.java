package io.github.will413028.hawkr.productimport.helper;

import io.github.will413028.hawkr.common.utils.code.ImportItemErrorType;
import io.github.will413028.hawkr.common.utils.code.ShippingTemperature;
import io.github.will413028.hawkr.common.utils.code.WeightUnit;
import io.github.will413028.hawkr.productimport.helper.CsvParser.ParsedRow;
import org.springframework.stereotype.Component;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * L2 同步驗證器（40 號設計書 §11.3 Layer 2）— 純記憶體內容驗證。
 *
 * <p>失敗 → status=CSV_INVALID + validation_errors 回 200。
 */
@Component
public class L2ContentValidator {

    private static final Set<String> ALLOWED_TARGET_STATUS = Set.of("ACTIVE", "DRAFT", "PENDING_REVIEW");
    private static final Set<String> ALLOWED_TEMPERATURE = Set.of("AMBIENT", "CHILLED", "FROZEN");
    private static final Set<String> ALLOWED_WEIGHT_UNIT = Set.of("KG", "CATTY", "GRAM");
    private static final Set<String> ALLOWED_CERT_TYPE = Set.of("ORGANIC", "TAP", "CAS", "HACCP", "ISO", "GMP");

    public L2Result validate(List<ParsedRow> rows) {
        List<Map<String, Object>> errors = new ArrayList<>();
        for (ParsedRow r : rows) {
            checkRow(r, errors);
        }
        return new L2Result(errors.isEmpty(), errors);
    }

    private void checkRow(ParsedRow r, List<Map<String, Object>> errors) {
        requireNonBlank(r, "product_key", r.productKey(), errors);
        requireNonBlank(r, "product_name", r.productName(), errors);
        requireNonBlank(r, "category_id", r.categoryId(), errors);
        requireNonBlank(r, "product_image_1", r.productImage1(), errors);

        if (!r.productImage1().isBlank() && !isHttpsUrl(r.productImage1())) {
            errors.add(err(r.csvLine(), "product_image_1", ImportItemErrorType.COVER_IMAGE_MISSING.value(),
                    "封面圖必須為 https URL", r.productImage1()));
        }
        for (int i = 2; i <= 5; i++) {
            String url = r.productImage(i);
            if (!url.isBlank() && !isHttpsUrl(url)) {
                errors.add(err(r.csvLine(), "product_image_" + i, ImportItemErrorType.URL_INVALID.value(),
                        "product_image_" + i + " 必須為 https URL", url));
            }
        }

        if (!r.targetStatus().isBlank() && !ALLOWED_TARGET_STATUS.contains(r.targetStatus())) {
            errors.add(err(r.csvLine(), "target_status", ImportItemErrorType.ENUM_INVALID.value(),
                    "target_status 非合法值（ACTIVE/DRAFT/PENDING_REVIEW）", r.targetStatus()));
        }

        if (!ALLOWED_WEIGHT_UNIT.contains(r.skuUnit())) {
            errors.add(err(r.csvLine(), "sku_unit", ImportItemErrorType.ENUM_INVALID.value(),
                    "sku_unit 非合法值（KG/CATTY/GRAM）", r.skuUnit()));
        }

        Long price = requirePositiveLong(r, "sku_price", r.skuPrice(), errors);
        if (!r.skuSalePrice().isBlank()) {
            Long salePrice = parsePositiveLongOrNull(r.skuSalePrice());
            if (salePrice == null || salePrice <= 0) {
                errors.add(err(r.csvLine(), "sku_sale_price", ImportItemErrorType.NUMERIC_INVALID.value(),
                        "sku_sale_price 必須為正整數", r.skuSalePrice()));
            } else if (price != null && salePrice >= price) {
                errors.add(err(r.csvLine(), "sku_sale_price", ImportItemErrorType.SALE_PRICE_INVALID.value(),
                        "sku_sale_price 必須小於 sku_price", r.skuSalePrice()));
            }
        }
        requireNonNegativeInt(r, "sku_stock_qty", r.skuStockQty(), errors);
        requirePositiveDecimal(r, "sku_weight", r.skuWeight(), errors);
        requirePositiveDecimal(r, "sku_length_cm", r.skuLengthCm(), errors);
        requirePositiveDecimal(r, "sku_width_cm", r.skuWidthCm(), errors);
        requirePositiveDecimal(r, "sku_height_cm", r.skuHeightCm(), errors);

        boolean anyShipping = false;
        anyShipping |= checkShippingCell(r, "sku_shipping_taqbin", r.skuShippingTaqbin(), errors);
        anyShipping |= checkShippingCell(r, "sku_shipping_seven", r.skuShippingSeven(), errors);
        anyShipping |= checkShippingCell(r, "sku_shipping_uber", r.skuShippingUber(), errors);
        if (!anyShipping) {
            errors.add(err(r.csvLine(), "sku_shipping_*",
                    ImportItemErrorType.SHIPPING_AT_LEAST_ONE_REQUIRED.value(),
                    "sku_shipping_taqbin / sku_shipping_seven / sku_shipping_uber 至少要填 1 個", ""));
        }

        if (!r.skuFreeThreshold().isBlank()) {
            Long ft = parseLongOrNull(r.skuFreeThreshold());
            if (ft == null || ft < 0) {
                errors.add(err(r.csvLine(), "sku_free_threshold", ImportItemErrorType.NUMERIC_INVALID.value(),
                        "sku_free_threshold 必須為非負整數", r.skuFreeThreshold()));
            }
        }

        if (!r.certifications().isBlank()) {
            String[] parts = r.certifications().split(",");
            for (String p : parts) {
                String s = p.trim();
                if (s.isEmpty()) continue;
                int colon = s.indexOf(':');
                if (colon <= 0 || colon == s.length() - 1) {
                    errors.add(err(r.csvLine(), "certifications", ImportItemErrorType.CERT_FORMAT_INVALID.value(),
                            "certifications 格式必須為 TYPE:CERT_NO（半形冒號）", s));
                    continue;
                }
                String certType = s.substring(0, colon).trim();
                String certNo = s.substring(colon + 1).trim();
                if (!ALLOWED_CERT_TYPE.contains(certType)) {
                    errors.add(err(r.csvLine(), "certifications", ImportItemErrorType.CERT_ENUM_INVALID.value(),
                            "certification type 非合法值（ORGANIC/TAP/CAS/HACCP/ISO/GMP）", certType));
                }
                if (certNo.isEmpty()) {
                    errors.add(err(r.csvLine(), "certifications", ImportItemErrorType.CERT_NO_MISSING.value(),
                            "certification cert_no 空", s));
                }
            }
        }

        if (!r.hashtags().isBlank()) {
            List<String> tokens = HashtagInputSplitter.split(r.hashtags());
            if (tokens.size() > HashtagInputSplitter.MAX_TOKENS_PER_PRODUCT) {
                errors.add(err(r.csvLine(), "hashtags", ImportItemErrorType.HASHTAG_COUNT_EXCEEDED.value(),
                        "hashtag 數量 > " + HashtagInputSplitter.MAX_TOKENS_PER_PRODUCT, r.hashtags()));
            }
            for (String token : tokens) {
                HashtagInputSplitter.TokenIssue issue = HashtagInputSplitter.checkToken(token);
                switch (issue) {
                    case EMPTY -> errors.add(err(r.csvLine(), "hashtags",
                            ImportItemErrorType.HASHTAG_EMPTY_AFTER_NORMALIZE.value(),
                            "hashtag normalize 後為空", token));
                    case INVALID_CHAR -> errors.add(err(r.csvLine(), "hashtags",
                            ImportItemErrorType.HASHTAG_INVALID_CHAR.value(),
                            "hashtag 含不允許字元（僅接受中文/英文/數字）", token));
                    case LENGTH_EXCEEDED -> errors.add(err(r.csvLine(), "hashtags",
                            ImportItemErrorType.HASHTAG_LENGTH_EXCEEDED.value(),
                            "hashtag 長度 > " + HashtagInputSplitter.MAX_TOKEN_LENGTH + " 字", token));
                    case NONE -> {}
                }
            }
        }
    }

    private boolean checkShippingCell(ParsedRow r, String field, String value, List<Map<String, Object>> errors) {
        if (value == null || value.isBlank()) return false;
        if (!ALLOWED_TEMPERATURE.contains(value)) {
            errors.add(err(r.csvLine(), field, ImportItemErrorType.ENUM_INVALID.value(),
                    field + " 非合法溫層（AMBIENT/CHILLED/FROZEN）", value));
            return false;
        }
        return true;
    }

    private static void requireNonBlank(ParsedRow r, String field, String value, List<Map<String, Object>> errors) {
        if (value == null || value.isBlank()) {
            errors.add(err(r.csvLine(), field, ImportItemErrorType.REQUIRED_MISSING.value(),
                    field + " 必填", ""));
        }
    }

    private static Long requirePositiveLong(ParsedRow r, String field, String value, List<Map<String, Object>> errors) {
        Long n = parseLongOrNull(value);
        if (n == null || n <= 0) {
            errors.add(err(r.csvLine(), field, ImportItemErrorType.NUMERIC_INVALID.value(),
                    field + " 必須為正整數", value));
            return null;
        }
        return n;
    }

    private static void requireNonNegativeInt(ParsedRow r, String field, String value, List<Map<String, Object>> errors) {
        Long n = parseLongOrNull(value);
        if (n == null || n < 0) {
            errors.add(err(r.csvLine(), field, ImportItemErrorType.NUMERIC_INVALID.value(),
                    field + " 必須為非負整數", value));
        }
    }

    private static void requirePositiveDecimal(ParsedRow r, String field, String value, List<Map<String, Object>> errors) {
        try {
            double v = Double.parseDouble(value);
            if (v <= 0) {
                errors.add(err(r.csvLine(), field, ImportItemErrorType.NUMERIC_INVALID.value(),
                        field + " 必須大於 0", value));
            }
        } catch (NumberFormatException e) {
            errors.add(err(r.csvLine(), field, ImportItemErrorType.NUMERIC_INVALID.value(),
                    field + " 必須為數字", value));
        }
    }

    private static Long parseLongOrNull(String v) {
        if (v == null || v.isBlank()) return null;
        try { return Long.parseLong(v.trim()); } catch (NumberFormatException e) { return null; }
    }

    private static Long parsePositiveLongOrNull(String v) {
        Long n = parseLongOrNull(v);
        return n == null || n <= 0 ? null : n;
    }

    private static boolean isHttpsUrl(String v) {
        return v != null && v.startsWith("https://");
    }

    private static Map<String, Object> err(int line, String field, String type, String message, String raw) {
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("csv_line", line);
        m.put("field", field);
        m.put("error_type", type);
        m.put("message", message);
        m.put("raw_value", raw);
        return m;
    }

    public record L2Result(boolean passed, List<Map<String, Object>> errors) {}
}
