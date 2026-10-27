package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 雙分錄帳本的科目代碼，18 值（2026-09-12 依 `ADR-010` 補上「應收」這一側的 5 值；原 13 值零 RECEIVABLE，兩個 PAYABLE 都是「平台欠別人」）。每一筆交易的借貸兩方各取本表一值；收斂點斷言 `SUM(debit) = SUM(credit)`，不平衡一律拒絕寫入。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum LedgerAccountCode implements CodeMeta {

    PLATFORM_COLLECTED_CASH("PLATFORM_COLLECTED_CASH", "PLATFORM_COLLECTED_CASH", "平台代收現金", "平台代理商戶實際收到的買家款"),
    PLATFORM_PAYOUT_CASH("PLATFORM_PAYOUT_CASH", "PLATFORM_PAYOUT_CASH", "平台撥出現金", "撥款指示實際出款"),
    PLATFORM_LOGISTICS_RECOVERY("PLATFORM_LOGISTICS_RECOVERY", "PLATFORM_LOGISTICS_RECOVERY", "平台物流費回收", "向賣家回收的可歸責物流成本"),
    PLATFORM_DISCOUNT_SUBSIDY("PLATFORM_DISCOUNT_SUBSIDY", "PLATFORM_DISCOUNT_SUBSIDY", "平台折扣補貼", "平台負擔的折扣"),
    PLATFORM_FEE_INCOME("PLATFORM_FEE_INCOME", "PLATFORM_FEE_INCOME", "平台手續費收入", "撥款指示費等"),
    PLATFORM_SERVICE_FEE_INCOME("PLATFORM_SERVICE_FEE_INCOME", "PLATFORM_SERVICE_FEE_INCOME", "平台服務費收入", "合約平台服務費"),
    PLATFORM_FEE_EXPENSE("PLATFORM_FEE_EXPENSE", "PLATFORM_FEE_EXPENSE", "平台手續費支出", "平台對外支付的費用"),
    PLATFORM_LOGISTICS_COST("PLATFORM_LOGISTICS_COST", "PLATFORM_LOGISTICS_COST", "平台物流成本", "平台負擔的物流費用"),
    SELLER_PAYABLE("SELLER_PAYABLE", "SELLER_PAYABLE", "應付賣家", "賣家淨額的主科目"),
    SELLER_DISCOUNT_BORNE("SELLER_DISCOUNT_BORNE", "SELLER_DISCOUNT_BORNE", "賣家負擔折扣", "費基公式的減項"),
    SELLER_HOLD("SELLER_HOLD", "SELLER_HOLD", "賣家保留款", "暫停撥款的保留金額"),
    SELLER_MANUAL_ADJUSTMENT("SELLER_MANUAL_ADJUSTMENT", "SELLER_MANUAL_ADJUSTMENT", "賣家可稽核調整", "手動調整必須帶原因與依據"),
    PROVIDER_PAYABLE("PROVIDER_PAYABLE", "PROVIDER_PAYABLE", "應付物流商", "provider 對帳用"),
    SELLER_LOGISTICS_RECEIVABLE("SELLER_LOGISTICS_RECEIVABLE", "SELLER_LOGISTICS_RECEIVABLE", "賣家物流應收", "借方增加＝應收成立；貸方增加＝收款完成／貸記沖抵。🔴 與 SELLER_LOGISTICS_CREDIT 是**兩個科目**不是一個科目的正負——兩者可同時存在（這期欠、上期多收），合成淨額會讓 ADR-008 第 1 條保證失效"),
    PLATFORM_LOGISTICS_SERVICE_INCOME("PLATFORM_LOGISTICS_SERVICE_INCOME", "PLATFORM_LOGISTICS_SERVICE_INCOME", "平台物流服務費收入", "貸方增加＝平台向賣家收的物流服務費收入。🔴 不等於買家付的運費（那是 PLATFORM_LOGISTICS_RECOVERY），ADR-006 三條線互不推導"),
    PLATFORM_COLLECTION_FEE_EXPENSE("PLATFORM_COLLECTION_FEE_EXPENSE", "PLATFORM_COLLECTION_FEE_EXPENSE", "平台收款通道手續費", "🔴 **僅限平台負擔的例外情況**（PLATFORM_CAUSED 的反向沖銷）。正常路徑**不 post 這一筆**——手續費由賣家負擔（ADR-005），對平台不是費用而是「應收未收足的原因」，多 post 一筆會把應收餘額沖平、讓 ADR-011 的結轉機制在帳上失效"),
    SELLER_LOGISTICS_CREDIT("SELLER_LOGISTICS_CREDIT", "SELLER_LOGISTICS_CREDIT", "賣家物流貸記", "貸方增加＝貸記發生（**平台欠賣家**）；借方增加＝貸記動用。🔴 永不過期、不因賣家無後續交易而清零（ADR-008 第 2、3 條）⇒ 不得有任何排程把它歸零"),
    PLATFORM_OPERATING_CASH("PLATFORM_OPERATING_CASH", "PLATFORM_OPERATING_CASH", "平台自有現金", "借方增加＝平台自有現金流入（收到賣家的物流費）；貸方增加＝流出（付給 provider）。🔴 SELLER_DIRECT 下平台**沒有代收** ⇒ PLATFORM_COLLECTED_CASH 在新商流無語意，付給 provider 的對手科目必須是本科目，否則 PROVIDER_SETTLEMENT 湊不出合法分錄而 trg_ledger_balanced 會在 COMMIT 當下擋下");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    LedgerAccountCode(String code, String value, String label, String description) {
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

    private static final Map<String, LedgerAccountCode> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(LedgerAccountCode::code, e -> e));

    private static final Map<String, LedgerAccountCode> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(LedgerAccountCode::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static LedgerAccountCode fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static LedgerAccountCode fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
