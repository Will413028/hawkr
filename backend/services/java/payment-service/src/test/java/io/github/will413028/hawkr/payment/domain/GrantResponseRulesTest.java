package io.github.will413028.hawkr.payment.domain;

import io.github.will413028.hawkr.payment.domain.GrantResponseRules.GrantResponseClass;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * §5.6 去重硬化 A/B 純函式 helper 測試（無 I/O、無狀態）。
 */
class GrantResponseRulesTest {

    // ==================== A：撥款回應分類 ====================

    /** 被評估就 fail 的 supplier：驗證 AMBIGUOUS 路徑不觸發退款判準（延遲查詢契約）。 */
    private static java.util.function.BooleanSupplier mustNotEvaluate() {
        return () -> {
            throw new AssertionError("AMBIGUOUS 路徑不得評估 hasCompletedRefund supplier（延遲查詢契約）");
        };
    }

    @Test
    @DisplayName("transport / 不可信回應碼 → AMBIGUOUS，且不評估退款判準 supplier（延遲查詢）")
    void ambiguousCodes_classifyAsAmbiguous_withoutEvaluatingSupplier() {
        for (String code : new String[]{
                "IO_ERROR", "IO_EXCEPTION", "HTTP_ERROR", "EMPTY_RESPONSE", "PARSE_ERROR", "HASH_INVALID",
                "http_error", "hash_invalid"}) {   // 大小寫不敏感
            assertEquals(GrantResponseClass.AMBIGUOUS,
                    GrantResponseRules.classifyFailure(code, mustNotEvaluate()),
                    "code=" + code + " 應為模稜兩可");
        }
    }

    @Test
    @DisplayName("null / 空白碼 → AMBIGUOUS（無法確認，fail-safe 不重試；不評估 supplier）")
    void nullOrBlank_classifyAsAmbiguous() {
        assertEquals(GrantResponseClass.AMBIGUOUS, GrantResponseRules.classifyFailure(null, mustNotEvaluate()));
        assertEquals(GrantResponseClass.AMBIGUOUS, GrantResponseRules.classifyFailure("", mustNotEvaluate()));
        assertEquals(GrantResponseClass.AMBIGUOUS, GrantResponseRules.classifyFailure("   ", mustNotEvaluate()));
    }

    @Test
    @DisplayName("可信的 PayUni 業務拒絕碼（無已完成退款）→ DEFINITELY_NOT_SENT（可安全重試一次）")
    void payUniRejectCode_classifyAsDefinitelyNotSent() {
        for (String code : new String[]{"GRANT02001", "TRADE99999", "PARAM_ERROR_X", "SOME_REJECT"}) {
            assertEquals(GrantResponseClass.DEFINITELY_NOT_SENT,
                    GrantResponseRules.classifyFailure(code, () -> false),
                    "code=" + code + " 應為明確未發動");
        }
    }

    // ==================== §5.6.1：退款接管第三類（PROVIDER_TOOK_OVER）====================

    @Test
    @DisplayName("可信拒絕 + 存在已完成退款 → PROVIDER_TOOK_OVER（碼不可知主判準，不重試）")
    void payUniRejectWithCompletedRefund_classifyAsProviderTookOver() {
        // 線索碼家族（GRANT30023 退款接管實測 / 30018 已撥款 / 30019 已有撥款日期）與任意拒絕碼皆同判
        for (String code : new String[]{"GRANT30023", "GRANT30018", "GRANT30019", "GRANT02001", "SOME_REJECT"}) {
            assertEquals(GrantResponseClass.PROVIDER_TOOK_OVER,
                    GrantResponseRules.classifyFailure(code, () -> true),
                    "code=" + code + " + 已完成退款 應為 PayUni 接管");
        }
    }

    @Test
    @DisplayName("GRANT30023 無已完成退款（請款批次未結時間性場景）→ DEFINITELY_NOT_SENT（不收斂、可重試）")
    void grant30023WithoutCompletedRefund_stillDefinitelyNotSent() {
        assertEquals(GrantResponseClass.DEFINITELY_NOT_SENT,
                GrantResponseRules.classifyFailure("GRANT30023", () -> false));
    }

    @Test
    @DisplayName("模稜兩可（transport / 不可信）即使有已完成退款 → 仍 AMBIGUOUS（無可信 PayUni 決定，不得斷言接管）")
    void ambiguousWithCompletedRefund_staysAmbiguous() {
        for (String code : new String[]{"HTTP_ERROR", "HASH_INVALID", "", null}) {
            assertEquals(GrantResponseClass.AMBIGUOUS, GrantResponseRules.classifyFailure(code, () -> true),
                    "code=" + code + " 應維持模稜兩可");
        }
    }

    // ==================== B：grant_query 對帳比對 ====================

    @Test
    @DisplayName("TradeNo 精確相符 → 命中（已入帳，不重發）")
    void reconciliationHit_exactTradeNoMatch() {
        assertTrue(GrantResponseRules.isReconciliationHit("UNI_123", "UNI_123"));
    }

    @Test
    @DisplayName("TradeNo 不符 / null / 空白 → 未命中")
    void reconciliationHit_mismatchOrNull() {
        assertFalse(GrantResponseRules.isReconciliationHit("OTHER", "UNI_123"));
        assertFalse(GrantResponseRules.isReconciliationHit(null, "UNI_123"));
        assertFalse(GrantResponseRules.isReconciliationHit("UNI_123", null));
        assertFalse(GrantResponseRules.isReconciliationHit("UNI_123", "  "));
    }

    // ==================== B：grant_query 逐日「有效查完」判定（issue #1329）====================

    @Test
    @DisplayName("SUCCESS → 該日有效查完（正常回應，續比對 Result[]）")
    void grantQueryDayComplete_success() {
        // isSuccess=true 時 status 不影響（呼叫端已判 SUCCESS）
        assertTrue(GrantResponseRules.isGrantQueryDayComplete(true, "SUCCESS"));
        assertTrue(GrantResponseRules.isGrantQueryDayComplete(true, null));
    }

    @Test
    @DisplayName("QUERY03002（查無撥款資料）→ 該日有效查完（有效空頁，續掃下一日）")
    void grantQueryDayComplete_noData_isEmptyValidDay() {
        assertTrue(GrantResponseRules.isGrantQueryDayComplete(false, GrantResponseRules.GRANT_QUERY_NO_DATA));
        assertTrue(GrantResponseRules.isGrantQueryDayComplete(false, "QUERY03002"));
        assertTrue(GrantResponseRules.isGrantQueryDayComplete(false, " query03002 "));   // 大小寫 / 前後空白不敏感
    }

    @Test
    @DisplayName("QUERY02004（超 30 天窗）/ transport / 其他非預期 / null → 未查完（INCONCLUSIVE）")
    void grantQueryDayComplete_outOfWindowOrUnexpected_notComplete() {
        assertFalse(GrantResponseRules.isGrantQueryDayComplete(false, GrantResponseRules.GRANT_QUERY_OUT_OF_WINDOW));
        assertFalse(GrantResponseRules.isGrantQueryDayComplete(false, "QUERY02004"));
        assertFalse(GrantResponseRules.isGrantQueryDayComplete(false, "HTTP_ERROR"));
        assertFalse(GrantResponseRules.isGrantQueryDayComplete(false, "SOME_UNEXPECTED"));
        assertFalse(GrantResponseRules.isGrantQueryDayComplete(false, null));
        assertFalse(GrantResponseRules.isGrantQueryDayComplete(false, ""));
    }
}
