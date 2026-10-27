package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 賣家稅務資格的審核狀態。**沒有終態**——REJECTED／EXPIRED／REVOKED 都有出邊，一次審核失誤不等於永久逐出平台。⚠️ 本值域是**狀態（名詞）**，ADM 送出的**決定（動詞）**是另一個值域 SellerTaxReviewDecision，兩者不收斂成同一個字。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum SellerTaxReviewStatus implements CodeMeta {

    NOT_STARTED("NOT_STARTED", "NOT_STARTED", "尚未開始", "**不是 DB 值**：`seller_tax_profiles` 查無列時由 usecase 投影出來（`invoice-service/internal/usecase/query_read.go` 的 `taxReviewStatusNotStarted`）。上線當天全體賣家都是這個狀態，把它當錯誤或跳過會讓賣家整批從列表消失"),
    DRAFT("DRAFT", "DRAFT", "草稿", "賣家填寫中；由 REJECTED／EXPIRED 重新申請也回到這裡（需過 7 天冷卻）"),
    SUBMITTED("SUBMITTED", "SUBMITTED", "已送出", "60 天期限自本狀態的 submitted_at 起算"),
    UNDER_REVIEW("UNDER_REVIEW", "UNDER_REVIEW", "審核中", "平台已受理"),
    SUPPLEMENT_REQUIRED("SUPPLEMENT_REQUIRED", "SUPPLEMENT_REQUIRED", "待補件", "ADM 要求補件；**不重置 60 天期限**"),
    SELF_ISSUE_PENDING("SELF_ISSUE_PENDING", "SELF_ISSUE_PENDING", "已具結待審", "已接受自行開票責任條款，審核期間可接單但受筆數與金額上限"),
    APPROVED_INVOICE_REQUIRED("APPROVED_INVOICE_REQUIRED", "APPROVED_INVOICE_REQUIRED", "核定應開統一發票", "必有統編與法定名稱；走 AMEGO 自動開票"),
    APPROVED_INVOICE_EXEMPT("APPROVED_INVOICE_EXEMPT", "APPROVED_INVOICE_EXEMPT", "核定免用統一發票", "不得帶統編；走平台代製收據"),
    REJECTED("REJECTED", "REJECTED", "已駁回", "cooldown_until = now + 7 天"),
    EXPIRED("EXPIRED", "EXPIRED", "已逾期", "60 天屆滿未完成審核；ADM 可受理逾期補件（免冷卻）"),
    REVOKED("REVOKED", "REVOKED", "已撤銷", "ADM 撤銷已核定資格（雙人確認）；可重新審核");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    SellerTaxReviewStatus(String code, String value, String label, String description) {
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

    private static final Map<String, SellerTaxReviewStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(SellerTaxReviewStatus::code, e -> e));

    private static final Map<String, SellerTaxReviewStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(SellerTaxReviewStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static SellerTaxReviewStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static SellerTaxReviewStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
