package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 雙分錄帳本的交易類型，17 值（2026-09-12 依 `ADR-010` 新增 6 值承載 SELLER_DIRECT 的物流財務軸）。🔴 **`LOGISTICS_COST_ACCRUAL` 與 `PROVIDER_SETTLEMENT` 在兩個商流下是同名的兩種分錄**——科目對由 `ledger_transactions.commerce_model` 決定，不由 txn_type 單獨決定；逐條對照見 70 §3.8。⚠️ 退款沖銷**拆成商品與運費兩個值**：兩者的費基影響不同（商品退款進 `fee_base` 減項，運費不進），合成一個值會讓費基算出一個看起來合理但錯的數字。本 enum 為本波新建、無存量資料，故直接以拆分後的形式落地。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum LedgerTxnType implements CodeMeta {

    ORDER_ACCRUAL("ORDER_ACCRUAL", "ORDER_ACCRUAL", "訂單應計", "訂單成立時的應計分錄"),
    REFUND_REVERSAL_GOODS("REFUND_REVERSAL_GOODS", "REFUND_REVERSAL_GOODS", "商品退款沖銷", "🔴 進 `fee_base` 減項"),
    REFUND_REVERSAL_SHIPPING("REFUND_REVERSAL_SHIPPING", "REFUND_REVERSAL_SHIPPING", "運費退款沖銷", "🔴 **不**進 `fee_base`"),
    PAYOUT_FEE("PAYOUT_FEE", "PAYOUT_FEE", "撥款指示費", "費率與費基於指示落地時快照，事後可重算"),
    PLATFORM_SERVICE_FEE("PLATFORM_SERVICE_FEE", "PLATFORM_SERVICE_FEE", "平台服務費", "合約約定的服務費"),
    HOLD_APPLY("HOLD_APPLY", "HOLD_APPLY", "保留款設定", "暫停撥款"),
    HOLD_RELEASE("HOLD_RELEASE", "HOLD_RELEASE", "保留款解除", "解除暫停"),
    PAYOUT_SETTLE("PAYOUT_SETTLE", "PAYOUT_SETTLE", "撥款結清", "撥款指示完成"),
    MANUAL_ADJUSTMENT("MANUAL_ADJUSTMENT", "MANUAL_ADJUSTMENT", "可稽核調整", "必填原因與依據"),
    LOGISTICS_COST_ACCRUAL("LOGISTICS_COST_ACCRUAL", "LOGISTICS_COST_ACCRUAL", "物流成本應計", "平台側物流成本"),
    PROVIDER_SETTLEMENT("PROVIDER_SETTLEMENT", "PROVIDER_SETTLEMENT", "物流商結算", "provider 帳單對帳"),
    LOGISTICS_RECEIVABLE_ACCRUAL("LOGISTICS_RECEIVABLE_ACCRUAL", "LOGISTICS_RECEIVABLE_ACCRUAL", "物流應收認列", "DEBIT SELLER_LOGISTICS_RECEIVABLE／CREDIT PLATFORM_LOGISTICS_SERVICE_INCOME。🔴 金額＝本期成本 − 平台補貼，**不含前期結轉**（結轉是前期已認列、尚未收足的餘額，再認列一次就是重複認列收入）"),
    LOGISTICS_COLLECTION_SETTLED("LOGISTICS_COLLECTION_SETTLED", "LOGISTICS_COLLECTION_SETTLED", "物流收款入帳", "DEBIT PLATFORM_OPERATING_CASH／CREDIT SELLER_LOGISTICS_RECEIVABLE。🔴 金額＝`net_proceeds`（gross − actual_fee），**不是 gross**——手續費是 PayUni 從撥款裡扣掉的，平台從來沒收到那筆錢"),
    LOGISTICS_TRUE_UP("LOGISTICS_TRUE_UP", "LOGISTICS_TRUE_UP", "物流成本 true-up", "暫估 → 實際的**雙向**差額。實際 > 暫估：DEBIT 應收／CREDIT 收入；實際 < 暫估：**方向相反**，同一個 txn_type"),
    LOGISTICS_CREDIT_ISSUE("LOGISTICS_CREDIT_ISSUE", "LOGISTICS_CREDIT_ISSUE", "物流貸記發生", "DEBIT PLATFORM_LOGISTICS_SERVICE_INCOME／CREDIT SELLER_LOGISTICS_CREDIT。⚠️ 借「收入」不是借「費用」：多收的那筆當初是貸記收入，現在要還 ⇒ **沖減當初認列的收入**"),
    LOGISTICS_CREDIT_APPLY("LOGISTICS_CREDIT_APPLY", "LOGISTICS_CREDIT_APPLY", "物流貸記抵扣", "DEBIT SELLER_LOGISTICS_CREDIT／CREDIT SELLER_LOGISTICS_RECEIVABLE。貸記優先抵扣、現金返還為後備（抵扣不經通道 ⇒ 零通道成本），🔴 但不得因成本而拒絕返還"),
    COLLECTION_FEE_ACCRUAL("COLLECTION_FEE_ACCRUAL", "COLLECTION_FEE_ACCRUAL", "收款手續費（平台負擔例外）", "DEBIT PLATFORM_COLLECTION_FEE_EXPENSE／CREDIT PLATFORM_OPERATING_CASH。🔴 **只在平台負擔手續費的例外情況** post，正常路徑不 post");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    LedgerTxnType(String code, String value, String label, String description) {
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

    private static final Map<String, LedgerTxnType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(LedgerTxnType::code, e -> e));

    private static final Map<String, LedgerTxnType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(LedgerTxnType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static LedgerTxnType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static LedgerTxnType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
