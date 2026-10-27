package io.github.will413028.hawkr.productimport.helper;

import org.apache.commons.csv.CSVFormat;
import org.apache.commons.csv.CSVParser;
import org.apache.commons.csv.CSVRecord;
import org.springframework.stereotype.Component;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.io.InputStreamReader;
import java.io.Reader;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Iterator;
import java.util.List;
import java.util.Map;

/**
 * CSV 解析器（40 號設計書 §11）— 26 欄位。
 *
 * <p>L1 用簡單預掃 ({@link #scanHeaderAndRowCount})；解析後給 L2 / worker 用 {@link #parse}。
 */
@Component
public class CsvParser {

    public HeaderProbe scanHeaderAndRowCount(byte[] content) throws IOException {
        try (Reader reader = stripBomReader(content);
             CSVParser parser = CSVFormat.DEFAULT.builder()
                     .setHeader()
                     .setSkipHeaderRecord(true)
                     .setIgnoreSurroundingSpaces(true)
                     .setAllowMissingColumnNames(false)
                     .build()
                     .parse(reader)) {
            List<String> headers = parser.getHeaderNames();
            long rowCount = 0L;
            Iterator<CSVRecord> it = parser.iterator();
            while (it.hasNext()) {
                CSVRecord r = it.next();
                if (isBlankRow(r)) continue;
                rowCount++;
            }
            return new HeaderProbe(headers, rowCount);
        }
    }

    public List<ParsedRow> parse(byte[] content) throws IOException {
        try (Reader reader = stripBomReader(content);
             CSVParser parser = CSVFormat.DEFAULT.builder()
                     .setHeader()
                     .setSkipHeaderRecord(true)
                     .setIgnoreSurroundingSpaces(true)
                     .setIgnoreEmptyLines(true)
                     .build()
                     .parse(reader)) {
            Map<String, Integer> headerMap = parser.getHeaderMap();
            List<ParsedRow> rows = new ArrayList<>();
            int csvLine = 1;
            for (CSVRecord r : parser) {
                csvLine++;
                if (isBlankRow(r)) continue;
                rows.add(new ParsedRow(
                        csvLine,
                        col(r, headerMap, "product_key"),
                        col(r, headerMap, "product_name"),
                        col(r, headerMap, "category_id"),
                        col(r, headerMap, "origin"),
                        col(r, headerMap, "description"),
                        // 40 §4.3a：對齊正常上架 9 張上限，CSV header 容納 product_image_1 ~ 9；
                        // 未提供的欄位 col() 回空字串。CSV >9 時由 §5.2 step 5.2.f 截斷 + WARNING（CsvParseService）。
                        col(r, headerMap, "product_image_1"),
                        col(r, headerMap, "product_image_2"),
                        col(r, headerMap, "product_image_3"),
                        col(r, headerMap, "product_image_4"),
                        col(r, headerMap, "product_image_5"),
                        col(r, headerMap, "product_image_6"),
                        col(r, headerMap, "product_image_7"),
                        col(r, headerMap, "product_image_8"),
                        col(r, headerMap, "product_image_9"),
                        col(r, headerMap, "certifications"),
                        col(r, headerMap, "hashtags"),
                        col(r, headerMap, "target_status"),
                        col(r, headerMap, "sku_name"),
                        col(r, headerMap, "sku_price"),
                        col(r, headerMap, "sku_sale_price"),
                        col(r, headerMap, "sku_stock_qty"),
                        col(r, headerMap, "sku_weight"),
                        col(r, headerMap, "sku_unit"),
                        col(r, headerMap, "sku_length_cm"),
                        col(r, headerMap, "sku_width_cm"),
                        col(r, headerMap, "sku_height_cm"),
                        col(r, headerMap, "sku_shipping_taqbin"),
                        col(r, headerMap, "sku_shipping_seven"),
                        col(r, headerMap, "sku_shipping_uber"),
                        col(r, headerMap, "sku_free_threshold")
                ));
            }
            return rows;
        }
    }

    private static String col(CSVRecord r, Map<String, Integer> headers, String name) {
        if (!headers.containsKey(name)) return "";
        String v = r.get(name);
        return v == null ? "" : v.trim();
    }

    private static boolean isBlankRow(CSVRecord r) {
        for (String v : r) {
            if (v != null && !v.trim().isEmpty()) return false;
        }
        return true;
    }

    private static Reader stripBomReader(byte[] content) {
        int off = 0;
        if (content.length >= 3
                && (content[0] & 0xFF) == 0xEF
                && (content[1] & 0xFF) == 0xBB
                && (content[2] & 0xFF) == 0xBF) {
            off = 3;
        }
        return new InputStreamReader(new ByteArrayInputStream(content, off, content.length - off),
                StandardCharsets.UTF_8);
    }

    public record HeaderProbe(List<String> headerNames, long rowCount) {}

    public record ParsedRow(int csvLine,
                            String productKey,
                            String productName,
                            String categoryId,
                            String origin,
                            String description,
                            String productImage1,
                            String productImage2,
                            String productImage3,
                            String productImage4,
                            String productImage5,
                            String productImage6,
                            String productImage7,
                            String productImage8,
                            String productImage9,
                            String certifications,
                            String hashtags,
                            String targetStatus,
                            String skuName,
                            String skuPrice,
                            String skuSalePrice,
                            String skuStockQty,
                            String skuWeight,
                            String skuUnit,
                            String skuLengthCm,
                            String skuWidthCm,
                            String skuHeightCm,
                            String skuShippingTaqbin,
                            String skuShippingSeven,
                            String skuShippingUber,
                            String skuFreeThreshold) {

        public boolean isProductImagePresent(int idx) {
            return !productImage(idx).isEmpty();
        }

        public String productImage(int idx) {
            return switch (idx) {
                case 1 -> productImage1;
                case 2 -> productImage2;
                case 3 -> productImage3;
                case 4 -> productImage4;
                case 5 -> productImage5;
                case 6 -> productImage6;
                case 7 -> productImage7;
                case 8 -> productImage8;
                case 9 -> productImage9;
                default -> "";
            };
        }

        /** 40 §4.3a：MAX 9，與 ProductPublishConstants.MAX_GALLERY_IMAGES 對齊。 */
        public static final int MAX_GALLERY_IMAGES = 9;
    }
}
