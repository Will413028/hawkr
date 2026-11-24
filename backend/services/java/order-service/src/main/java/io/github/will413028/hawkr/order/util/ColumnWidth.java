package io.github.will413028.hawkr.order.util;

/**
 * {@code orders} 表文字欄位的寬度上限（V1__init_order_tables.sql）。
 *
 * <p>長度以字元（code point）計，與 PostgreSQL {@code VARCHAR(n)} 一致，不是 PayUni 的顯示長度。
 */
public final class ColumnWidth {

    private ColumnWidth() {}

    /** {@code orders.recipient_name VARCHAR(50)}。 */
    public static final int RECIPIENT_NAME = 50;

    /** {@code orders.shipping_address VARCHAR(255)}。 */
    public static final int SHIPPING_ADDRESS = 255;

    /**
     * 驗證字串長度不超過欄位寬度。
     *
     * @throws IllegalArgumentException 長度超過 {@code max}
     */
    public static void requireWithin(String value, int max, String fieldName) {
        if (value != null && value.codePointCount(0, value.length()) > max) {
            throw new IllegalArgumentException(fieldName + " 長度過長，最多 " + max + " 個字");
        }
    }
}
