package io.github.will413028.hawkr.common.utils.validation;

/**
 * PayUni 物流欄位約束常數。
 *
 * <p>集中定義 PayUni 物流建單 API 各欄位的長度約束，避免散落硬編碼（docs/開發規範/Java 微服務開發規範.md §6.1）。
 * 長度單位為「顯示長度」：英文字母/半形算 1 位、中文字/全形字符算 2 位，對齊 PayUni 計長規則，
 * 與 {@link InputValidator#displayLength(String)} 一致。
 *
 * <p>置於 common-core（order-service 與 shipping-service 皆依賴）供兩端引用：
 * <ul>
 *   <li>order-service 結帳 confirm：對<b>前端可控</b>欄位（收件人姓名/地址）源頭驗證拒絕
 *       （{@link InputValidator#requireDisplayLengthBetween}）。</li>
 *   <li>shipping-service 建單 Strategy：對<b>系統背景填寫</b>欄位（商品名稱）截斷適配
 *       （{@link InputValidator#truncateByDisplayLength}）。</li>
 * </ul>
 *
 * <p>約束來源（PayUni 官方文件）：
 * <ul>
 *   <li>超商 7-11 取件人姓名：最長 5 個中文字、最短至少 2 個中文字或 4 個英文字（限制長度 10）。</li>
 *   <li>黑貓宅配收件人姓名：限制長度 30（中文/全形算 2、英文算 1）。</li>
 *   <li>黑貓宅配收件人地址：最長 120 位。</li>
 *   <li>黑貓宅配商品名稱：最長 20 位；超商商品說明：限制長度 500。</li>
 * </ul>
 */
public final class PayUniShippingConstraints {

    private PayUniShippingConstraints() {}

    // ==================== 收件人姓名（前端可控，源頭驗證） ====================

    /** 超商 7-11 收件人姓名顯示長度下限（2 中文 / 4 英文）。 */
    public static final int CVS_RECIPIENT_NAME_MIN = 4;

    /** 超商 7-11 收件人姓名顯示長度上限（5 中文）。 */
    public static final int CVS_RECIPIENT_NAME_MAX = 10;

    /** 黑貓宅配收件人姓名顯示長度上限（無官方下限，僅以非空驗證）。 */
    public static final int HOME_RECIPIENT_NAME_MAX = 30;

    // ==================== 寄件人姓名（賣家開通物流設定，前端可控，源頭驗證） ====================

    /** 超商 7-11 建店寄件人/聯絡人姓名（ContactName）顯示長度上限。 */
    public static final int CVS_SENDER_NAME_MAX = 10;

    /** 黑貓宅配建店寄件人姓名（SenderName）顯示長度上限。 */
    public static final int HOME_SENDER_NAME_MAX = 30;

    // ==================== 收件人地址（前端可控，源頭驗證） ====================

    /** 黑貓宅配收件人地址顯示長度上限。 */
    public static final int RECIPIENT_ADDRESS_MAX = 120;

    // ==================== 商品名稱（系統背景填寫，截斷適配） ====================

    /** 黑貓宅配商品名稱顯示長度上限。 */
    public static final int HOME_GOODS_NAME_MAX = 20;

    /** 超商商品說明顯示長度上限。 */
    public static final int CVS_GOODS_NAME_MAX = 500;
}
