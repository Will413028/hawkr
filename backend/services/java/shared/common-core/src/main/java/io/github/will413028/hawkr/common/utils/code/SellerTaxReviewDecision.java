package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * ADM 對賣家稅務資格的**決定（動詞）**。⚠️ 與 SellerTaxReviewStatus（狀態，名詞）刻意分開命名，兩者不收斂成同一個字（58 §7.3.3 末段）。**8 值定版**＝58 §7.3.3 的七值 ＋ 58 §6 原有的 `KEEP_UNDER_REVIEW`（58 §7.3.3 本輪已補 T16 承載它）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum SellerTaxReviewDecision implements CodeMeta {

    APPROVE_INVOICE_REQUIRED("APPROVE_INVOICE_REQUIRED", "APPROVE_INVOICE_REQUIRED", "核定應開統一發票", "端點 40／58 §7 T07；來源狀態 UNDER_REVIEW／SELF_ISSUE_PENDING；需 expectedVersion CAS"),
    APPROVE_INVOICE_EXEMPT("APPROVE_INVOICE_EXEMPT", "APPROVE_INVOICE_EXEMPT", "核定免用統一發票", "端點 40／T08；另需已取得收據代製授權（S05），且 `confirmed=true` 二次確認"),
    REQUEST_SUPPLEMENT("REQUEST_SUPPLEMENT", "REQUEST_SUPPLEMENT", "要求補件", "端點 40／T05；轉 SUPPLEMENT_REQUIRED；**不重置 60 天期限**"),
    KEEP_UNDER_REVIEW("KEEP_UNDER_REVIEW", "KEEP_UNDER_REVIEW", "暫不裁決（留在佇列）", "端點 40／**T16**；狀態自轉移 UNDER_REVIEW → UNDER_REVIEW，**只寫稽核**，不動 `pending_valid_until` 與 `extension_count`"),
    REJECT("REJECT", "REJECT", "駁回", "端點 40／T09；必填理由；cooldown_until = now + 7 天"),
    REVOKE("REVOKE", "REVOKE", "撤銷已核定資格", "端點 40／T14；雙人確認；來源狀態必為 APPROVED_*"),
    REOPEN_REVIEW("REOPEN_REVIEW", "REOPEN_REVIEW", "發起複審／受理逾期補件", "端點 40／T13／T15／T12；回到 UNDER_REVIEW；必填 recheck_reason；自 EXPIRED 進來免冷卻"),
    EXTEND("EXTEND", "EXTEND", "延長審核期限", "🔴 **落點是端點 41 不是 40**（它需要 `extendDays`，守衛 `extension_count = 0`，失敗碼 9323）；58 §7 T11；狀態不變，就地改寫 pending_valid_until。**仍屬同一個 enum**——端點 41 成功時要往 `decisionHistory` 寫一筆 `decision = \"EXTEND\"`");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    SellerTaxReviewDecision(String code, String value, String label, String description) {
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

    private static final Map<String, SellerTaxReviewDecision> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(SellerTaxReviewDecision::code, e -> e));

    private static final Map<String, SellerTaxReviewDecision> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(SellerTaxReviewDecision::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static SellerTaxReviewDecision fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static SellerTaxReviewDecision fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
