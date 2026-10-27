package io.github.will413028.hawkr.payment.logistics.domain;

import io.github.will413028.hawkr.common.obs.grpc.GrpcBusinessException;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;

/**
 * 帳本交易不合法時拋出的例外。
 *
 * <p>Go 版（{@code settlement-service} 的 {@code domain.ErrLedgerUnbalanced} /
 * {@code ErrLedgerInvalid} / {@code ErrAmountOverflow}）用三個 sentinel error 區分三種失敗；
 * 本類用 {@link Kind} 承載同一個區分。<b>合成一種會讓呼叫端分不出「分錄不平衡」與「形狀寫錯」</b>
 * —— 前者是帳務不變式 INV-L1 被破壞（有專屬業務碼 9566），後者是建構分錄的程式寫錯。
 *
 * <h2>🔴 這不取代 {@code trg_ledger_balanced}</h2>
 * DB 的 deferred constraint trigger 是唯一擋得住「繞過 domain 的寫入路徑」的東西
 * （維運 SQL、未來的第二條 code path）。兩道都要 —— 本例外讓錯誤在寫入前就帶著業務語意現形，
 * trigger 讓它在任何情況下都不會落庫。
 */
public class LedgerValidationException extends RuntimeException implements GrpcBusinessException {

    /** 失敗類別；每一種對應一個業務碼。 */
    public enum Kind {
        /** INV-L1 不成立（Σ DEBIT ≠ Σ CREDIT）。 */
        UNBALANCED(GrpcResponseCode.LEDGER_TRANSACTION_UNBALANCED),
        /** 分錄形狀不合法（科目／方向／持有人／序號／金額／商流相容性）。 */
        INVALID(GrpcResponseCode.INVALID_PARAM),
        /** 借方或貸方合計溢位。 */
        OVERFLOW(GrpcResponseCode.INVALID_PARAM);

        private final GrpcResponseCode responseCode;

        Kind(GrpcResponseCode responseCode) {
            this.responseCode = responseCode;
        }

        public GrpcResponseCode responseCode() {
            return responseCode;
        }
    }

    private final Kind kind;

    private LedgerValidationException(Kind kind, String detail) {
        super(kind.responseCode().message() + "：" + detail);
        this.kind = kind;
    }

    public Kind kind() {
        return kind;
    }

    @Override
    public int code() {
        return kind.responseCode().code();
    }

    /** INV-L1：Σ DEBIT ≠ Σ CREDIT。 */
    static LedgerValidationException unbalanced(long debit, long credit) {
        return new LedgerValidationException(Kind.UNBALANCED,
                "debit=" + debit + " credit=" + credit);
    }

    /** 形狀不合法。{@code detail} 用 {@link String#format} 的樣板。 */
    static LedgerValidationException invalid(String detail, Object... args) {
        return new LedgerValidationException(Kind.INVALID,
                args.length == 0 ? detail : String.format(detail, args));
    }

    /** 合計溢位。 */
    static LedgerValidationException overflow(String detail) {
        return new LedgerValidationException(Kind.OVERFLOW, detail);
    }
}
