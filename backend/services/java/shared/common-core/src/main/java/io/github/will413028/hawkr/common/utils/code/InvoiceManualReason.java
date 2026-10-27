package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 系統決定「這一筆不自己處理、轉人工」時的理由碼。**三個來源**：① 送出前的不變式斷言失敗（開立側 A1–A15／A20–A23、折讓側 A16–A19）② 決策層的 fail-closed（平台出資券、混合稅別、期限已過等）③ **58 §6.3 P7 `invoice.operation.review_required` 的 `review_reason_code`**（2026-08-25 新增末三值，共用同一值域、**不另開新 enum**，裁決見 58 §6.3.3.3）。**每個碼 <= 32 字元**（欄位寬度），新增前先數字元。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceManualReason implements CodeMeta {

    PROVIDER_RESULT_UNKNOWN("PROVIDER_RESULT_UNKNOWN", "PROVIDER_RESULT_UNKNOWN", "服務商結果未確認", "逾時／連線中斷且重查仍未明；受控人工出口的入口條件"),
    PLATFORM_FUNDED_DISCOUNT("PLATFORM_FUNDED_DISCOUNT", "PLATFORM_FUNDED_DISCOUNT", "折扣資金來源待判定", "A22：偵測到 created_by=ADMIN 的券或判不出資金來源；fail-closed，不得靜默以折扣後金額開票"),
    MIXED_TAX_CATEGORY("MIXED_TAX_CATEGORY", "MIXED_TAX_CATEGORY", "混合稅別待人工判定", "一張文件含多種稅別而無法自動推導表頭 TaxType"),
    CONSOLIDATED_ORDER("CONSOLIDATED_ORDER", "CONSOLIDATED_ORDER", "合併訂單待人工處理", "一筆付款對應多張訂單，開票拆分規則未定"),
    ISSUER_RETIRED("ISSUER_RETIRED", "ISSUER_RETIRED", "開票主體已停止營運", "issuer 已 retired，無有效稅籍可用"),
    DEADLINE_MISSED("DEADLINE_MISSED", "DEADLINE_MISSED", "已逾法定存證期限", "由 idx_invdoc_deadline 的逾期掃描寫入"),
    AMOUNT_MISMATCH("AMOUNT_MISMATCH", "AMOUNT_MISMATCH", "明細加總與訂單金額不符", "A1"),
    TAX_SPLIT_MISMATCH("TAX_SPLIT_MISMATCH", "TAX_SPLIT_MISMATCH", "稅額拆解不平", "A2：Sales + Free + Zero + Tax != Total"),
    NEGATIVE_HEADER_AMOUNT("NEGATIVE_HEADER_AMOUNT", "NEGATIVE_HEADER_AMOUNT", "表頭金額為負", "A3"),
    NEGATIVE_BUCKET("NEGATIVE_BUCKET", "NEGATIVE_BUCKET", "稅別桶加總為負", "A4"),
    LINE_SIGN_INVALID("LINE_SIGN_INVALID", "LINE_SIGN_INVALID", "品項行正負號錯誤", "A5：DISCOUNT 行必須為負、其餘必須非負"),
    ZERO_LINE_EMITTED("ZERO_LINE_EMITTED", "ZERO_LINE_EMITTED", "輸出了零元品項行", "A6"),
    B2C_TAX_NOT_ZERO("B2C_TAX_NOT_ZERO", "B2C_TAX_NOT_ZERO", "二聯式稅額非零", "A7"),
    B2B_TAX_MISCALC("B2B_TAX_MISCALC", "B2B_TAX_MISCALC", "三聯式稅額計算錯誤", "A8"),
    HEADER_TAXTYPE_MISMATCH("HEADER_TAXTYPE_MISMATCH", "HEADER_TAXTYPE_MISMATCH", "表頭稅別與明細不符", "A9"),
    ZERO_RATE_NOT_SUPPORTED("ZERO_RATE_NOT_SUPPORTED", "ZERO_RATE_NOT_SUPPORTED", "零稅率尚未支援", "A10：AMEGO 另需 CustomsClearanceMark／ZeroTaxRateReason"),
    B2B_MIXED_TAX_UNVERIFIED("B2B_MIXED_TAX_UNVERIFIED", "B2B_MIXED_TAX_UNVERIFIED", "三聯式混合稅別未查證", "A11：fail-closed"),
    LINE_COUNT_INVALID("LINE_COUNT_INVALID", "LINE_COUNT_INVALID", "品項行數超出範圍", "A12：1 <= n <= 9999"),
    DESCRIPTION_INVALID("DESCRIPTION_INVALID", "DESCRIPTION_INVALID", "品項名稱不合法", "A13：空值或超過 256 字"),
    DETAIL_VAT_INVALID("DETAIL_VAT_INVALID", "DETAIL_VAT_INVALID", "明細含稅旗標錯誤", "A14"),
    BUYER_INFO_CONFLICT("BUYER_INFO_CONFLICT", "BUYER_INFO_CONFLICT", "統編、載具與捐贈碼互斥違反", "A15：AMEGO 不擋，必須由我方擋"),
    ALLOWANCE_SPLIT_MISMATCH("ALLOWANCE_SPLIT_MISMATCH", "ALLOWANCE_SPLIT_MISMATCH", "折讓稅額拆解不符退款金額", "A16"),
    ALLOWANCE_EXCEEDS_INVOICE("ALLOWANCE_EXCEEDS_INVOICE", "ALLOWANCE_EXCEEDS_INVOICE", "累計折讓超過原發票金額", "A17：上游允許超額退款，對帳責任在發票端"),
    ALLOWANCE_TAX_MISMATCH("ALLOWANCE_TAX_MISMATCH", "ALLOWANCE_TAX_MISMATCH", "全額折讓稅額與原發票不符", "A18"),
    ALLOWANCE_MULTI_INVOICE("ALLOWANCE_MULTI_INVOICE", "ALLOWANCE_MULTI_INVOICE", "一張折讓單綁多張發票", "A19：AMEGO 硬約束（4040132）"),
    AMOUNT_EXCEEDS_LIMIT("AMOUNT_EXCEEDS_LIMIT", "AMOUNT_EXCEEDS_LIMIT", "金額超過單張上限", "A20：199,999"),
    SUBTOTAL_MISMATCH("SUBTOTAL_MISMATCH", "SUBTOTAL_MISMATCH", "訂單小計與逐項乘積不符", "A21：來源資料本身壞了，不猜不修正"),
    ALLOWANCE_BASE_MISMATCH("ALLOWANCE_BASE_MISMATCH", "ALLOWANCE_BASE_MISMATCH", "可折讓基數與總額不符", "A23"),
    PREFERENCE_OWNERSHIP_MISMATCH("PREFERENCE_OWNERSHIP_MISMATCH", "PREFERENCE_OWNERSHIP_MISMATCH", "常用發票資料不屬於本訂單買家", "解析 buyer_invoice_preference_id 時歸屬驗證失敗；**必須拒絕開票，不得 fallback 成無載具**（會把 IDOR 靜默吞掉）"),
    SELF_ISSUE_AUTHORIZED("SELF_ISSUE_AUTHORIZED", "SELF_ISSUE_AUTHORIZED", "已授權賣家自行開立", "58 §7 J29：ADM 雙人覆核解鎖受控人工出口，賣家可自行開立並回報。**不要用 `MANUAL_PROCESS_ALLOWED`**——那是 InvoiceOperationStatus 的狀態值"),
    VOID_WINDOW_EXPIRED("VOID_WINDOW_EXPIRED", "VOID_WINDOW_EXPIRED", "作廢等待逾限，改走折讓", "58 §7.2.2 註 B：等不到 AMEGO 上傳確認且逾 wait_deadline，自動改走 ALLOWANCE"),
    DEADLINE_APPROACHING("DEADLINE_APPROACHING", "DEADLINE_APPROACHING", "法定存證期限逼近", "58 §7.5.6 的 T−6h 階梯；與既有的 DEADLINE_MISSED（T+0 已逾期）是兩個時點，不可合併"),
    PROVIDER_RETRYABLE_ERROR("PROVIDER_RETRYABLE_ERROR", "PROVIDER_RETRYABLE_ERROR", "服務商暫時性失敗", "58 #3082 第二階段：AMEGO 回可重試類錯誤（連線／逾時／3050121 上一個異動還在排隊等），退避後會再送一次。**不是最終結果** ⇒ 與 PROVIDER_REJECTED_PERMANENT 分開；AMEGO 的碼在 provider_code 與 reason_detail"),
    RETRY_BUDGET_EXHAUSTED("RETRY_BUDGET_EXHAUSTED", "RETRY_BUDGET_EXHAUSTED", "重試次數已用盡", "58 #3082 第二階段：attempt_count 達 max_attempts。**與「服務商永久拒絕」不同**：後者第一次就不該再試，本碼是試過 N 次都失敗 —— ADM 的下一步也不同（前者要改資料，後者要看每一次的失敗是不是同一個原因）"),
    SELLER_TRACK_INSUFFICIENT("SELLER_TRACK_INSUFFICIENT", "SELLER_TRACK_INSUFFICIENT", "賣家字軌不足", "58 #3082 第二階段：AMEGO 3040111 那一類 —— **要賣家去財政部配字軌**，不是我方改資料、也不是等 provider 恢復。行動指引與其他 provider 失敗完全相反，所以不共用碼（J32 → SELLER_ACTION_REQUIRED 的落點）"),
    ALLOWANCE_NUMBER_MISSING("ALLOWANCE_NUMBER_MISSING", "ALLOWANCE_NUMBER_MISSING", "折讓單缺單號", "58 #3082：折讓單自己沒有 allowance_number ⇒ 無從向 AMEGO 查詢確認，重試也長不出單號來。**我方判定**，與 ALLOWANCE_NO_SOURCE_INVOICE（原發票沒號碼）是兩件事"),
    ALLOWANCE_NO_SOURCE_INVOICE("ALLOWANCE_NO_SOURCE_INVOICE", "ALLOWANCE_NO_SOURCE_INVOICE", "原發票沒有發票號碼", "58 #3082：折讓無所依附（原 document 的 provider_invoice_no 為空）。**我方判定**、不是 provider 拒絕 ⇒ 與 PROVIDER_REJECTED_PERMANENT 分開。在此之前這一格與「明細為空」都被硬編碼成 ALLOWANCE_EXCEEDS_INVOICE（超額），三種完全不同的原因在 ADM 上長得一模一樣"),
    PROVIDER_REJECTED_PERMANENT("PROVIDER_REJECTED_PERMANENT", "PROVIDER_REJECTED_PERMANENT", "服務商永久拒絕", "58 #3082（DEC-20260904-120527 選 A）：AMEGO 明確拒絕且不可重試，而**真實原因不在本碼**——它在 invoice_operations.provider_code／provider_message 與 invoice_status_history.reason_detail。本碼刻意中性：在此之前折讓軸把所有永久失敗都硬編碼成 ALLOWANCE_EXCEEDS_INVOICE，於是一筆「原發票不存在」在 ADM 被分類成「超額」，把診斷方向帶去查金額。**猜一個具體分類比不分類更糟**"),
    RECEIPT_AUTH_MISSING("RECEIPT_AUTH_MISSING", "RECEIPT_AUTH_MISSING", "賣家尚未授權平台代製收據", "58 §6.1.9 S05：收據的**出據人是賣家**（Q-B7），平台沒有他的授權就不得以他的名義產出文件 ⇒ fail-closed 落 AWAITING_MANUAL，由 ADM 去催授權。**不是 NOT_APPLICABLE**：那是終態，會讓這筆訂單再也沒有人去補收據"),
    CARRIER_INVALID("CARRIER_INVALID", "CARRIER_INVALID", "載具驗證未通過", "開票前以 /json/barcode 驗手機條碼，AMEGO 回 9000112（格式錯誤）或 9000113（不存在）⇒ 不送 f0401、轉人工。設計書 §P12 要求的送出前驗證；三種失敗碼可分辨為 2026-09-08 實測（X13 結案）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceManualReason(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceManualReason> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceManualReason::code, e -> e));

    private static final Map<String, InvoiceManualReason> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceManualReason::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceManualReason fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceManualReason fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
