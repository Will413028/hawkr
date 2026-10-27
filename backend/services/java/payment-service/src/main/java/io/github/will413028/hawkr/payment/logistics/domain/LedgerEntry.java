package io.github.will413028.hawkr.payment.logistics.domain;

import io.github.will413028.hawkr.common.utils.code.LedgerAccountCode;
import io.github.will413028.hawkr.common.utils.code.LedgerOwnerType;

/**
 * 一列分錄（{@code ledger_entries} 的一列）。
 *
 * <h2>為什麼欄位是 {@code String} 而不是 CodeMeta enum</h2>
 * 本型別是 DDL CHECK 的<b>應用層鏡像</b>：{@code account_code} / {@code owner_type} /
 * {@code direction} 在 DB 都是 {@code varchar}，由 {@code chk_ledgentry_*} 守值域。
 * 寫成 enum 會讓「未知值」在 Java 變成不可表達 —— 看似更強，實際是把檢查<b>搬到</b>
 * 某個尚未存在的邊界層，而本層就再也驗不到那一條了。維持 {@code String} ＋
 * {@link #validate(int)} 內的 CodeMeta 值域檢查，讓每一條守則都留在同一個收斂點、
 * 且每一條都測得到。
 *
 * @param lineSeq    表頭內序號，從 1 起連號（{@code uq_ledgentry_txn_seq}）
 * @param accountCode CodeMeta {@link LedgerAccountCode} 的 code
 * @param ownerType  CodeMeta {@link LedgerOwnerType} 的 code
 * @param ownerRefId PLATFORM ⇒ 0；SELLER ⇒ {@code store_id}；PROVIDER ⇒ ShippingMethod 碼值
 * @param direction  {@link #DIRECTION_DEBIT} 或 {@link #DIRECTION_CREDIT}
 * @param amount     元（整數），<b>恆為正</b>；正負由 {@code direction} 表達
 * @param memo       備註，可為 null
 */
public record LedgerEntry(
        short lineSeq,
        String accountCode,
        String ownerType,
        long ownerRefId,
        String direction,
        long amount,
        String memo) {

    public static final String DIRECTION_DEBIT = "DEBIT";
    public static final String DIRECTION_CREDIT = "CREDIT";

    public boolean isDebit() {
        return DIRECTION_DEBIT.equals(direction);
    }

    public boolean isCredit() {
        return DIRECTION_CREDIT.equals(direction);
    }

    /** 借方一列。 */
    public static LedgerEntry debit(int lineSeq, String accountCode, String ownerType,
                                    long ownerRefId, long amount, String memo) {
        return new LedgerEntry((short) lineSeq, accountCode, ownerType, ownerRefId,
                DIRECTION_DEBIT, amount, memo);
    }

    /** 貸方一列。 */
    public static LedgerEntry credit(int lineSeq, String accountCode, String ownerType,
                                     long ownerRefId, long amount, String memo) {
        return new LedgerEntry((short) lineSeq, accountCode, ownerType, ownerRefId,
                DIRECTION_CREDIT, amount, memo);
    }

    /**
     * 檢查單列分錄的形狀。
     *
     * @param idx 這一列在表頭內的索引（0 起），只用於錯誤訊息定位
     * @throws LedgerValidationException 形狀不合法
     */
    void validate(int idx) {
        if (lineSeq < 1) {
            throw LedgerValidationException.invalid(
                    "entries[%d].line_seq=%d，必須 >= 1", idx, lineSeq);
        }
        if (!LedgerAccountCode.isValidCode(accountCode)) {
            throw LedgerValidationException.invalid(
                    "entries[%d] 未知的 account_code %s", idx, accountCode);
        }
        if (!LedgerOwnerType.isValidCode(ownerType)) {
            throw LedgerValidationException.invalid(
                    "entries[%d] 未知的 owner_type %s", idx, ownerType);
        }
        // 與 chk_ledgentry_ref 同一條規則：PLATFORM 恆 0，其餘恆正。
        if (LedgerOwnerType.PLATFORM.code().equals(ownerType)) {
            if (ownerRefId != 0L) {
                throw LedgerValidationException.invalid(
                        "entries[%d] PLATFORM 的 owner_ref_id 必須是 0，實得 %d", idx, ownerRefId);
            }
        } else if (ownerRefId <= 0L) {
            throw LedgerValidationException.invalid(
                    "entries[%d] %s 的 owner_ref_id 必須為正，實得 %d", idx, ownerType, ownerRefId);
        }
        if (!isDebit() && !isCredit()) {
            throw LedgerValidationException.invalid(
                    "entries[%d] 未知的 direction %s", idx, direction);
        }
        // INV-L2 ＋「金額恆為正、方向由 direction 表達」。
        // 允許負數 ⇒ 同一筆金額有兩種寫法 ⇒ 彙總時必然有人算錯一邊。
        if (amount <= 0L) {
            throw LedgerValidationException.invalid(
                    "entries[%d].amount=%d，必須為正（方向由 direction 表達）", idx, amount);
        }
    }
}
