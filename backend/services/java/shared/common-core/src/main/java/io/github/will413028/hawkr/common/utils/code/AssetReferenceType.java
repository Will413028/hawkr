package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 多媒體資產引用來源類型
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum AssetReferenceType implements CodeMeta {

    PRODUCT("PRODUCT", "7101", "PRODUCT", "商品展示圖片引用"),
    PRODUCT_DESC("PRODUCT_DESC", "7102", "PRODUCT_DESC", "商品描述圖片引用"),
    SELLER_LOGO("SELLER_LOGO", "7103", "SELLER_LOGO", "賣家 logo 引用"),
    SHIPPING("SHIPPING", "7104", "SHIPPING", "物流相關圖片引用"),
    SELLER_COVER("SELLER_COVER", "7105", "SELLER_COVER", "賣家封面圖片引用"),
    IDENTITY_VERIFICATION("IDENTITY_VERIFICATION", "7106", "IDENTITY_VERIFICATION", "使用者實名驗證圖片引用"),
    CHAT_MESSAGE("CHAT_MESSAGE", "7107", "CHAT_MESSAGE", "聊天訊息圖片引用"),
    POST("POST", "7108", "POST", "貼文圖片引用"),
    BANNER("BANNER", "7109", "BANNER", "Banner 圖片引用"),
    AFTERSALE("AFTERSALE", "7110", "AFTERSALE", "售後證據照片引用"),
    CERTIFICATION("CERTIFICATION", "7111", "CERTIFICATION", "商家認證文件引用"),
    BUYER_AVATAR("BUYER_AVATAR", "7112", "BUYER_AVATAR", "買家頭像引用"),
    ORDER_RECEIPT("ORDER_RECEIPT", "7113", "ORDER_RECEIPT", "買家確認收貨上傳照片引用"),
    AI_STYLE("AI_STYLE", "7114", "AI_STYLE", "AI 修圖風格縮圖引用"),
    ORDER_REVIEW("ORDER_REVIEW", "7115", "ORDER_REVIEW", "訂單評論圖片引用"),
    PRODUCT_IMPORT_PREVIEW("PRODUCT_IMPORT_PREVIEW", "7116", "PRODUCT_IMPORT_PREVIEW", "商品匯入預覽主圖（暫存階段，FinalizeImportAsset 後轉為 PRODUCT）"),
    PRODUCT_IMPORT_PREVIEW_DESC("PRODUCT_IMPORT_PREVIEW_DESC", "7117", "PRODUCT_IMPORT_PREVIEW_DESC", "商品匯入預覽描述圖（暫存階段，FinalizeImportAsset 後轉為 PRODUCT_DESC）"),
    GOVERNANCE_EVIDENCE("GOVERNANCE_EVIDENCE", "7118", "GOVERNANCE_EVIDENCE", "商家治理檢舉/申訴佐證圖片引用（seller_violation_reports.evidence_asset_ids / seller_violations.appeal_evidence_asset_ids）"),
    RECIPE("RECIPE", "7119", "RECIPE", "食譜主圖引用"),
    INVOICE_DOCUMENT("INVOICE_DOCUMENT", "7120", "INVOICE_DOCUMENT", "電子發票／折讓單 PDF 引用（invoice-service 經 media-service `ServerUploadSecureAndFinalize` 寫入 `secure/invoice_document/` 前綴，僅供三條 PDF 下載端點以 `GetSecureAssetInternal` 取用；58 §8.1）"),
    DELIVERY_POD("DELIVERY_POD", "7121", "DELIVERY_POD", "配送送達憑證（Uber POD 照片／簽名）；shipping-service 經 media-service `ServerUploadSecureAndFinalize` 寫入 `secure/delivery_pod/` 前綴，僅供授權端點以 `GetSecureAssetInternal` 取用（66 §7.8）"),
    SELLER_PROSPECT_LOGO("SELLER_PROSPECT_LOGO", "7122", "SELLER_PROSPECT_LOGO", "站外業者 logo 引用（seller_prospects.logo_key；與正式賣家的 SELLER_LOGO 分開，兩者的 owner 表與生命週期不同）"),
    SELLER_PROSPECT_COVER("SELLER_PROSPECT_COVER", "7123", "SELLER_PROSPECT_COVER", "站外業者封面圖片引用（seller_prospects.cover_key；同上，不與 SELLER_COVER 共用前綴）"),
    OTHER("OTHER", "7199", "OTHER", "未分類圖片引用");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    AssetReferenceType(String code, String value, String label, String description) {
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

    private static final Map<String, AssetReferenceType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AssetReferenceType::code, e -> e));

    private static final Map<String, AssetReferenceType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AssetReferenceType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static AssetReferenceType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static AssetReferenceType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
