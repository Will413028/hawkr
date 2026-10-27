package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * CSV 解析與校驗錯誤類型，用於前端依類型翻譯訊息與分組顯示。詳見 [40 號設計書 §11.3](../詳細設計書/40-商品匯入精靈服務（Product%20Import%20Service）詳細設計書.md#113-驗證規則)。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ImportItemErrorType implements CodeMeta {

    REQUIRED_MISSING("REQUIRED_MISSING", "REQUIRED_MISSING", "必填欄位缺漏", "CSV row 缺少必填欄位（product_name / category_id / sku_price / sku_stock_qty / sku_unit / sku_shipping_methods）"),
    ENUM_INVALID("ENUM_INVALID", "ENUM_INVALID", "Enum 值非法", "欄位值不在合法 enum 清單內"),
    NUMERIC_INVALID("NUMERIC_INVALID", "NUMERIC_INVALID", "數字格式非法", "數字欄位無法 parse 或超出正負範圍"),
    SALE_PRICE_INVALID("SALE_PRICE_INVALID", "SALE_PRICE_INVALID", "特價超出範圍", "sale_price ≥ price 或 ≤ 0"),
    PRODUCT_FIELD_INCONSISTENT("PRODUCT_FIELD_INCONSISTENT", "PRODUCT_FIELD_INCONSISTENT", "群組內欄位不一致", "同 product_key 內 product-level 欄位多筆值不同（取第一筆）"),
    CATEGORY_INVALID("CATEGORY_INVALID", "CATEGORY_INVALID", "分類非法", "category_id 不存在、非葉節點或已停用"),
    URL_INVALID("URL_INVALID", "URL_INVALID", "URL 格式錯誤", "非 https URL 或格式錯誤"),
    IMAGE_DOWNLOAD_FAILED("IMAGE_DOWNLOAD_FAILED", "IMAGE_DOWNLOAD_FAILED", "圖片下載失敗", "Media 端下載失敗（詳細原因見 ImportAssetFailureReason）"),
    HTML_PARSE_WARNING("HTML_PARSE_WARNING", "HTML_PARSE_WARNING", "HTML 轉換警告", "description HTML→JSON 轉換產生 warning（詳見 HtmlConversionWarning）"),
    ENCODING_NOT_UTF8("ENCODING_NOT_UTF8", "ENCODING_NOT_UTF8", "CSV 編碼非 UTF-8", "L1 整檔 reject"),
    COVER_IMAGE_MISSING("COVER_IMAGE_MISSING", "COVER_IMAGE_MISSING", "封面圖缺", "product_image_1 必填，gallery[0]"),
    SHIPPING_AT_LEAST_ONE_REQUIRED("SHIPPING_AT_LEAST_ONE_REQUIRED", "SHIPPING_AT_LEAST_ONE_REQUIRED", "shipping 至少 1 個必填", "sku_shipping_taqbin/seven/uber 全空"),
    SHIPPING_DIMENSION_MISSING("SHIPPING_DIMENSION_MISSING", "SHIPPING_DIMENSION_MISSING", "缺長寬高", "必填欄位缺，無法驗證 shipping policy"),
    SHIPPING_WEIGHT_EXCEEDED("SHIPPING_WEIGHT_EXCEEDED", "SHIPPING_WEIGHT_EXCEEDED", "shipping 因重量降級", "warning，停用該方式"),
    SHIPPING_DIMENSION_EXCEEDED("SHIPPING_DIMENSION_EXCEEDED", "SHIPPING_DIMENSION_EXCEEDED", "shipping 因尺寸降級", "warning"),
    SHIPPING_TEMP_INCOMPATIBLE("SHIPPING_TEMP_INCOMPATIBLE", "SHIPPING_TEMP_INCOMPATIBLE", "shipping × temp 不相容降級", "warning"),
    SHIPPING_NOT_ENABLED("SHIPPING_NOT_ENABLED", "SHIPPING_NOT_ENABLED", "賣家未開通該 shipping", "warning"),
    SHIPPING_DIM_WEIGHT_TEMP_DOWNGRADED("SHIPPING_DIM_WEIGHT_TEMP_DOWNGRADED", "SHIPPING_DIM_WEIGHT_TEMP_DOWNGRADED", "shipping 因尺寸/重量/溫層降級", "warning，由 product-service ProductPublishValidator 經 shipping-service ValidateProductShipping 判定（DIMENSION_SUM_EXCEEDED / SINGLE_SIDE_EXCEEDED / WEIGHT_EXCEEDED 三類合一）"),
    SHIPPING_ALL_DOWNGRADED("SHIPPING_ALL_DOWNGRADED", "SHIPPING_ALL_DOWNGRADED", "shipping 全降級無可用", "ERROR，必須改 dim 或啟用其他 shipping"),
    DUPLICATE_NAME_IN_JOB("DUPLICATE_NAME_IN_JOB", "DUPLICATE_NAME_IN_JOB", "同 job 內重名", "ERROR，必須改名"),
    DUPLICATE_NAME_VS_EXISTING("DUPLICATE_NAME_VS_EXISTING", "DUPLICATE_NAME_VS_EXISTING", "與既有商品重名", "ERROR，可勾選 allow_duplicate_name 通過"),
    CERT_VERIFY_FAILED("CERT_VERIFY_FAILED", "CERT_VERIFY_FAILED", "cert 政府平台驗證失敗", "ERROR"),
    CERT_VERIFY_PENDING("CERT_VERIFY_PENDING", "CERT_VERIFY_PENDING", "cert 驗證 timeout", "warning"),
    CERT_FORMAT_INVALID("CERT_FORMAT_INVALID", "CERT_FORMAT_INVALID", "cert cell 格式錯誤", "L2 / JOB"),
    CERT_ENUM_INVALID("CERT_ENUM_INVALID", "CERT_ENUM_INVALID", "cert ENUM 非合法", "warning，跳過該 cert"),
    CERT_NO_MISSING("CERT_NO_MISSING", "CERT_NO_MISSING", "cert_no 空", "warning，跳過該 cert"),
    HASHTAG_LENGTH_EXCEEDED("HASHTAG_LENGTH_EXCEEDED", "HASHTAG_LENGTH_EXCEEDED", "hashtag 個別 > 40 字", "warning，跳過該 hashtag（與 33 §3.2 對齊）"),
    HASHTAG_COUNT_EXCEEDED("HASHTAG_COUNT_EXCEEDED", "HASHTAG_COUNT_EXCEEDED", "hashtag 總數 > 10", "warning，取前 10"),
    HASHTAG_INVALID_CHAR("HASHTAG_INVALID_CHAR", "HASHTAG_INVALID_CHAR", "hashtag 切分後 token 含不允許字元", "warning，跳過該 hashtag；與 product-service `HashtagInputValidator` 規則對齊（中/英/數字以外字元視為違規）"),
    HASHTAG_EMPTY_AFTER_NORMALIZE("HASHTAG_EMPTY_AFTER_NORMALIZE", "HASHTAG_EMPTY_AFTER_NORMALIZE", "hashtag 切分後 token 空", "warning，跳過該 hashtag；normalize（trim + strip `#` + NFC）後 0 字元"),
    GALLERY_TRUNCATED_EXCEEDS_LIMIT("GALLERY_TRUNCATED_EXCEEDS_LIMIT", "GALLERY_TRUNCATED_EXCEEDS_LIMIT", "商品圖超過上限已截斷", "warning，CSV 來源圖片數 > 9 時截前 9 張保留，與正常上架 9 張上限對齊（40 §4.3a / §5.2 step 5.2.f）"),
    VALIDATION_DRIFT("VALIDATION_DRIFT", "VALIDATION_DRIFT", "上架規則漂移導致 commit 失敗", "ERROR，§5.7 commit final guard：parse / save 時通過但 commit 時 ValidateProductDraft 失敗（例：賣家編輯期間另一 tab 停用某 shipping、或 product-service 規則更新後既有 item 不再合規）"),
    GALLERY_REQUIRED("GALLERY_REQUIRED", "GALLERY_REQUIRED", "商品圖至少需 1 張", "ERROR，由 product-service ProductPublishValidator 產出，寫入 source=VALIDATE_PRODUCT_DRAFT"),
    GALLERY_EXCEEDS_LIMIT("GALLERY_EXCEEDS_LIMIT", "GALLERY_EXCEEDS_LIMIT", "商品圖超過 9 張上限", "ERROR，IMPORT_UPDATE / IMPORT_COMMIT / PUBLISH 時觸發；CSV_PARSE 不觸發（parse 已截斷）"),
    NAME_REQUIRED("NAME_REQUIRED", "NAME_REQUIRED", "商品名稱必填", "ERROR，ProductPublishValidator"),
    NAME_LENGTH_EXCEEDS("NAME_LENGTH_EXCEEDS", "NAME_LENGTH_EXCEEDS", "商品名稱超過字數上限", "ERROR，ProductPublishValidator 上限 80"),
    CATEGORY_NOT_LEAF("CATEGORY_NOT_LEAF", "CATEGORY_NOT_LEAF", "分類非葉節點", "ERROR，ProductPublishValidator"),
    CATEGORY_NOT_FOUND("CATEGORY_NOT_FOUND", "CATEGORY_NOT_FOUND", "分類不存在", "ERROR，ProductPublishValidator"),
    SKUS_REQUIRED("SKUS_REQUIRED", "SKUS_REQUIRED", "SKU 至少需 1 個", "ERROR，ProductPublishValidator"),
    SKUS_EXCEEDS_LIMIT("SKUS_EXCEEDS_LIMIT", "SKUS_EXCEEDS_LIMIT", "SKU 數超過上限", "ERROR，ProductPublishValidator"),
    PRICE_INVALID("PRICE_INVALID", "PRICE_INVALID", "SKU 價格不合法", "ERROR，ProductPublishValidator"),
    DESC_BLOCK_INVALID("DESC_BLOCK_INVALID", "DESC_BLOCK_INVALID", "description blocks 格式不合法", "ERROR，ProductPublishValidator"),
    ASSET_NOT_READY("ASSET_NOT_READY", "ASSET_NOT_READY", "圖片資產尚未 UPLOADED", "ERROR，IMPORT_COMMIT context 嚴格檢；parse / update 放寬");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ImportItemErrorType(String code, String value, String label, String description) {
        this.code = code;
        this.value = value;
        this.label = label;
        this.description = description;
    }

    @Override
    public String code() { return code; }

    @Override
    public String value() { return value; }

    @Override
    public String label() { return label; }

    @Override
    public String description() { return description; }

    private static final Map<String, ImportItemErrorType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ImportItemErrorType::code, e -> e));

    private static final Map<String, ImportItemErrorType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ImportItemErrorType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ImportItemErrorType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ImportItemErrorType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
