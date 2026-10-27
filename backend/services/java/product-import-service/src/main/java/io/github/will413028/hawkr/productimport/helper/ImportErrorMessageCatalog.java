package io.github.will413028.hawkr.productimport.helper;

import io.github.will413028.hawkr.common.utils.code.ImportItemErrorType;

import java.util.LinkedHashMap;
import java.util.Map;

/**
 * 40 號設計書 §4.6 / §11.3：商品匯入錯誤 / 警告的「欄位 → 中文 label」與「錯誤碼 → 訊息模板」對照表。
 *
 * <p>用途：
 * <ul>
 *   <li>前端列表頁 / 編輯頁渲染需要把 internal {@code field}（如 {@code gallery_images}）翻成「商品圖」</li>
 *   <li>{@link ImportErrorMessageBuilder} 把多筆同 (field, code) violations 聚合為一條對賣家友善的訊息</li>
 *   <li>本檔僅 lookup 表 — 沒有業務邏輯；validator 那端產出的中文 message 仍會優先被採用，
 *       這裡的模板做 fallback / 預設詞彙</li>
 * </ul>
 *
 * <p>對應的 error_type 來源涵蓋：
 * <ul>
 *   <li>§4.3a {@link io.github.will413028.hawkr.common.utils.code.ImportItemErrorType ProductPublishValidator} 規則目錄</li>
 *   <li>§4.6 GALLERY_TRUNCATED_EXCEEDS_LIMIT / VALIDATION_DRIFT</li>
 *   <li>§4.6 IMPORT_ITEM_ALREADY_COMMITTED（屬 gRPC 錯誤碼 + 對應 UI 訊息）</li>
 * </ul>
 */
public final class ImportErrorMessageCatalog {

    private ImportErrorMessageCatalog() {}

    /** Field → 中文 label。鍵為 internal field 名（CSV header / proto field），值為前端顯示用標籤。 */
    private static final Map<String, String> FIELD_LABELS = new LinkedHashMap<>();

    /** (field, error_code) → 中文訊息模板。validator 端如有自帶中文訊息則優先採用此 fallback 不會觸發。 */
    private static final Map<String, String> MESSAGE_TEMPLATES = new LinkedHashMap<>();

    static {
        // === Field 中文標籤 ===
        FIELD_LABELS.put("name", "商品名稱");
        FIELD_LABELS.put("product_name", "商品名稱");
        FIELD_LABELS.put("category_id", "商品分類");
        FIELD_LABELS.put("gallery_images", "商品圖");
        FIELD_LABELS.put("description", "商品描述");
        FIELD_LABELS.put("description_json", "商品描述");
        FIELD_LABELS.put("origin", "產地");
        FIELD_LABELS.put("target_status", "上架狀態");
        FIELD_LABELS.put("certifications", "認證");
        FIELD_LABELS.put("hashtags", "標籤");
        FIELD_LABELS.put("skus", "商品規格");
        FIELD_LABELS.put("sku_name", "規格名稱");
        FIELD_LABELS.put("sku_shipping", "配送方式");
        FIELD_LABELS.put("shipping_methods", "運送方式");
        FIELD_LABELS.put("sku_price", "規格售價");
        FIELD_LABELS.put("sku_sale_price", "規格特價");
        FIELD_LABELS.put("sku_stock_qty", "規格庫存");
        FIELD_LABELS.put("sku_unit", "規格單位");
        FIELD_LABELS.put("sku_weight", "規格重量");
        FIELD_LABELS.put("sku_dimension", "規格長寬高");

        // === 訊息模板（key = field + "|" + code）===
        // §4.3a ProductPublishValidator 規則目錄
        put("name", "NAME_REQUIRED", "商品名稱必填");
        put("name", "NAME_LENGTH_EXCEEDS", "商品名稱超過 80 字");
        put("category_id", "CATEGORY_NOT_LEAF", "請選擇葉節點分類");
        put("category_id", "CATEGORY_NOT_FOUND", "找不到指定分類");
        put("gallery_images", "GALLERY_REQUIRED", "商品圖至少需 1 張");
        put("gallery_images", "GALLERY_EXCEEDS_LIMIT", "商品圖超過 9 張上限");
        put("skus", "SKUS_REQUIRED", "至少需要一組商品規格");
        put("skus", "SKUS_EXCEEDS_LIMIT", "商品規格數量超過上限");
        put("description_json", "DESC_BLOCK_INVALID", "商品描述 blocks 格式不合法");
        put("gallery_images", "ASSET_NOT_READY", "圖片尚未上傳完成，請稍候");
        put("target_status", "TARGET_STATUS_INVALID", "目標上架狀態不合法（必須為 ACTIVE / DRAFT / PENDING_REVIEW）");

        // §4.6 import-service 端產出
        put("gallery_images", "GALLERY_TRUNCATED_EXCEEDS_LIMIT",
                "商品圖超過上限已截前 9 張，請至編輯頁確認");
        put(null, "VALIDATION_DRIFT", "上架規則已變更，請重新檢視商品內容後再 commit");
        put(null, "IMPORT_ITEM_ALREADY_COMMITTED",
                "該商品已併入正式商品，無法再編輯或移除");

        // === ImportItemErrorType code-only fallback ===
        // 每個 ImportItemErrorType 都至少有一條 code-only fallback，
        // 確保 validator/CSV parse 任何來源送來的 (field, code) 對都能翻成中文。
        // 同 code 不同 field 的更精確翻譯仍以上方 field-specific entry 為主（lookup 優先 field|code）。
        put(null, "REQUIRED_MISSING", "必填欄位缺漏，請至編輯頁補齊");
        put(null, "ENUM_INVALID", "欄位值不在可選清單內");
        put(null, "NUMERIC_INVALID", "數字欄位格式不正確");
        put(null, "SALE_PRICE_INVALID", "特價必須大於 0 且小於售價");
        put(null, "PRODUCT_FIELD_INCONSISTENT", "同一商品多筆規格的商品欄位不一致（已取第一筆）");
        put(null, "CATEGORY_INVALID", "商品分類不合法或非葉節點，請至編輯頁重新選擇");
        put(null, "URL_INVALID", "網址格式不正確（必須為 https）");
        put(null, "IMAGE_DOWNLOAD_FAILED", "圖片下載失敗，請確認圖片網址可正常開啟");
        put(null, "HTML_PARSE_WARNING", "商品描述 HTML 解析有警告，已盡量保留樣式");
        put(null, "ENCODING_NOT_UTF8", "CSV 編碼非 UTF-8，請另存為 UTF-8 後重新上傳");
        put(null, "COVER_IMAGE_MISSING", "封面圖（第 1 張商品圖）缺漏");
        put(null, "SHIPPING_AT_LEAST_ONE_REQUIRED", "至少需勾選一種配送方式");
        put(null, "SHIPPING_DIMENSION_MISSING", "缺長寬高，無法判斷配送方式可用範圍");
        put(null, "SHIPPING_WEIGHT_EXCEEDED", "重量超出該配送方式可承載範圍，已自動停用");
        put(null, "SHIPPING_DIMENSION_EXCEEDED", "尺寸超出該配送方式可承載範圍，已自動停用");
        put(null, "SHIPPING_TEMP_INCOMPATIBLE", "該配送方式不支援目前溫層，已自動停用");
        put(null, "SHIPPING_NOT_ENABLED", "您尚未開通該配送方式");
        put(null, "SHIPPING_ALL_DOWNGRADED", "所有配送方式都不可用，請調整尺寸 / 重量或啟用其他配送方式");
        put(null, "SHIPPING_DIM_WEIGHT_TEMP_DOWNGRADED", "尺寸 / 重量超出該配送方式上限，已自動停用");
        put(null, "DUPLICATE_NAME_IN_JOB", "此批次內有重複的商品名稱，請改名");
        put(null, "DUPLICATE_NAME_VS_EXISTING",
                "與既有商品同名（如為相同商品請勾選「允許重名」一併匯入）");
        // #2755：severity 由 error 降 warning，語意從「驗證失敗擋下 commit」變成「已轉送人工審核」。
        // 商品照常建立，只是該筆認證標章要等 ADM 核准後才公開 ⇒ 文案不得再讓賣家以為匯入失敗。
        put(null, "CERT_VERIFY_FAILED", "認證編號查驗未通過，已轉送人工審核；商品可正常上架，標章待審核通過後顯示");
        put(null, "CERT_VERIFY_PENDING", "認證資訊驗證逾時，將於背景重試");
        put(null, "CERT_FORMAT_INVALID", "認證資訊欄位格式不正確");
        put(null, "CERT_ENUM_INVALID", "認證代碼不在可選清單內，已自動略過該筆認證");
        put(null, "CERT_NO_MISSING", "認證編號缺漏，已自動略過該筆認證");
        put(null, "HASHTAG_LENGTH_EXCEEDED", "標籤超過 40 字，已自動略過");
        put(null, "HASHTAG_COUNT_EXCEEDED", "標籤數量超過 10 個，已自動截至前 10 個");
        put(null, "HASHTAG_INVALID_CHAR", "標籤包含不允許的特殊字元，已自動略過");
        put(null, "HASHTAG_EMPTY_AFTER_NORMALIZE", "標籤經整理後為空，已自動略過");
        put(null, "NAME_REQUIRED", "商品名稱必填");
        put(null, "NAME_LENGTH_EXCEEDS", "商品名稱超過 80 字");
        put(null, "CATEGORY_NOT_LEAF", "請選擇葉節點分類");
        put(null, "CATEGORY_NOT_FOUND", "找不到指定分類");
        put(null, "SKUS_REQUIRED", "至少需要一組商品規格");
        put(null, "SKUS_EXCEEDS_LIMIT", "商品規格數量超過上限");
        put(null, "PRICE_INVALID", "規格售價或特價不合法（售價必須 > 0，特價必須 > 0 且 < 售價）");
        put(null, "DESC_BLOCK_INVALID", "商品描述格式不合法");
        put(null, "ASSET_NOT_READY", "圖片尚未上傳完成，請稍候");
        put(null, "GALLERY_REQUIRED", "商品圖至少需 1 張");
        put(null, "GALLERY_EXCEEDS_LIMIT", "商品圖超過 9 張上限");
    }

    /** field → 中文 label；查無則回原 field 字串（避免空白）。 */
    public static String fieldLabel(String field) {
        if (field == null || field.isBlank()) return "";
        if (field.startsWith("description_json")) return FIELD_LABELS.get("description_json");
        String hit = FIELD_LABELS.get(field);
        return hit != null ? hit : field;
    }

    /**
     * (field, code) → 中文訊息模板；查無回 null。caller 收 null 時應回退使用 validator 自帶 message。
     * 若 field 不重要可帶 null（查 code-only fallback）。
     */
    public static String messageTemplate(String field, String code) {
        if (code == null || code.isBlank()) return null;
        if (field != null && !field.isBlank()) {
            String exact = MESSAGE_TEMPLATES.get(field + "|" + code);
            if (exact != null) return exact;
            if (field.startsWith("description_json")) {
                String descriptionTemplate = MESSAGE_TEMPLATES.get("description_json|" + code);
                if (descriptionTemplate != null) return descriptionTemplate;
            }
        }
        return MESSAGE_TEMPLATES.get("|" + code);
    }

    /**
     * API 顯示用訊息。已知 field/code 一律優先使用 catalog，避免把內部欄位名或底層 parser
     * exception 直接顯示給賣家；未知錯誤才保留來源訊息。
     */
    public static String displayMessage(String field, String code, String sourceMessage) {
        String template = messageTemplate(field, code);
        if (template != null && !template.isBlank()) return template;
        return sourceMessage == null ? "" : sourceMessage;
    }

    private static void put(String field, String code, String message) {
        String key = (field == null ? "" : field) + "|" + code;
        MESSAGE_TEMPLATES.put(key, message);
    }
}
