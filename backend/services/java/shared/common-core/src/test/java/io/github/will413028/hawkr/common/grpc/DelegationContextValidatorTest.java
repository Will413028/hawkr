package io.github.will413028.hawkr.common.grpc;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;

/**
 * 56 §6.2.4 的等價性 fixture E1–E13。Go 側 {@code pkg/delegation} 跑同一組；
 * 兩份有任一步不同就會出現分裂授權，而各自的測試都會通過。
 */
class DelegationContextValidatorTest {

    private static final long STORE_ID = 2685914682L;

    private static DelegationContext.Builder full() {
        return DelegationContext.newBuilder()
                .setSessionId("2701445588392001")
                .setOperatorAdminId("42")
                .setCapability(DelegationCapability.PRODUCTS_WRITE)
                .setAuditLogId("2701445588392099");
    }

    private static int codeOf(Executable e) {
        DelegationDeniedException ex = assertThrows(DelegationDeniedException.class, e::run);
        return ex.code();
    }

    private interface Executable {
        void run();
    }

    @Test
    @DisplayName("E1 delegation 為 null ⇒ 賣家本人路徑，判定不適用")
    void e1() {
        assertDoesNotThrow(() ->
                DelegationContextValidator.validate(null, STORE_ID, DelegationCapability.PRODUCTS_WRITE));
    }

    @Test
    @DisplayName("E2 四欄齊全、capability 相符、storeId 有值 ⇒ 通過")
    void e2() {
        assertDoesNotThrow(() ->
                DelegationContextValidator.validate(full().build(), STORE_ID, DelegationCapability.PRODUCTS_WRITE));
    }

    @Test
    @DisplayName("E3 .write 而 audit_log_id 為空 ⇒ S1b 失敗 9260")
    void e3() {
        assertEquals(GrpcResponseCode.DELEGATION_INTERNAL_CONTRACT_MISMATCH.code(),
                codeOf(() -> DelegationContextValidator.validate(
                        full().setAuditLogId("").build(), STORE_ID, DelegationCapability.PRODUCTS_WRITE)));
    }

    @Test
    @DisplayName("E4 session_id 為空 ⇒ S1a 失敗 9260")
    void e4() {
        assertEquals(GrpcResponseCode.DELEGATION_INTERNAL_CONTRACT_MISMATCH.code(),
                codeOf(() -> DelegationContextValidator.validate(
                        full().setSessionId("").build(), STORE_ID, DelegationCapability.PRODUCTS_WRITE)));
    }

    @Test
    @DisplayName("E5 capability 不在封閉列舉 ⇒ S2 失敗 9262")
    void e5() {
        assertEquals(GrpcResponseCode.DELEGATION_INTERNAL_CONTRACT_MISMATCH.code(),
                codeOf(() -> DelegationContextValidator.validate(
                        full().setCapability("products.delete").build(), STORE_ID, DelegationCapability.PRODUCTS_WRITE)));
    }

    @Test
    @DisplayName("E6 大寫 capability ⇒ S2 失敗 9262（比對大小寫敏感，不得先 normalize）")
    void e6() {
        assertEquals(GrpcResponseCode.DELEGATION_INTERNAL_CONTRACT_MISMATCH.code(),
                codeOf(() -> DelegationContextValidator.validate(
                        full().setCapability("PRODUCTS.WRITE").build(), STORE_ID, DelegationCapability.PRODUCTS_WRITE)));
    }

    @Test
    @DisplayName("E7 capability 合法但與本 RPC 宣告不符 ⇒ S3 失敗 9262")
    void e7() {
        assertEquals(GrpcResponseCode.DELEGATION_INTERNAL_CONTRACT_MISMATCH.code(),
                codeOf(() -> DelegationContextValidator.validate(
                        full().setCapability(DelegationCapability.PRODUCTS_READ).build(),
                        STORE_ID, DelegationCapability.PRODUCTS_WRITE)));
    }

    @Test
    @DisplayName("E8 storeId 為 0 ⇒ S4 失敗 9260")
    void e8() {
        assertEquals(GrpcResponseCode.DELEGATION_INTERNAL_CONTRACT_MISMATCH.code(),
                codeOf(() -> DelegationContextValidator.validate(
                        full().build(), 0L, DelegationCapability.PRODUCTS_WRITE)));
    }

    @Test
    @DisplayName("E9 request 與代行語境的冪等鍵不同 ⇒ S5 失敗 9260")
    void e9() {
        assertEquals(GrpcResponseCode.DELEGATION_INTERNAL_CONTRACT_MISMATCH.code(),
                codeOf(() -> DelegationContextValidator.validate(
                        full().setIdempotencyKey("ik_b").build(),
                        STORE_ID, DelegationCapability.PRODUCTS_WRITE, "ik_a")));
    }

    @Test
    @DisplayName("E10 兩處冪等鍵相同 ⇒ 通過")
    void e10() {
        assertDoesNotThrow(() -> DelegationContextValidator.validate(
                full().setIdempotencyKey("ik_a").build(),
                STORE_ID, DelegationCapability.PRODUCTS_WRITE, "ik_a"));
    }

    @Test
    @DisplayName("E11 request 沒送冪等鍵 ⇒ 不比對，通過")
    void e11() {
        assertDoesNotThrow(() -> DelegationContextValidator.validate(
                full().setIdempotencyKey("ik_a").build(),
                STORE_ID, DelegationCapability.PRODUCTS_WRITE, ""));
    }

    @Test
    @DisplayName("E12 products.read ＋ 空 audit_log_id ⇒ 通過（S1b 不適用於 .read）")
    void e12() {
        assertDoesNotThrow(() -> DelegationContextValidator.validate(
                full().setCapability(DelegationCapability.PRODUCTS_READ).setAuditLogId("").build(),
                STORE_ID, DelegationCapability.PRODUCTS_READ));
    }

    @Test
    @DisplayName("E13 storefront.read ＋ 空 audit_log_id ⇒ 通過")
    void e13() {
        assertDoesNotThrow(() -> DelegationContextValidator.validate(
                full().setCapability(DelegationCapability.STOREFRONT_READ).setAuditLogId("").build(),
                STORE_ID, DelegationCapability.STOREFRONT_READ));
    }

    @Test
    @DisplayName("orders.read ＋ 空 audit_log_id ⇒ 通過（PII 揭露的 audit_log_id 由呼叫端自行斷言）")
    void ordersReadWithoutAuditLog() {
        assertDoesNotThrow(() -> DelegationContextValidator.validate(
                full().setCapability(DelegationCapability.ORDERS_READ).setAuditLogId("").build(),
                STORE_ID, DelegationCapability.ORDERS_READ));
    }

    @Test
    @DisplayName("Q4 歸屬不符 ⇒ 9275")
    void q4NotOwned() {
        assertEquals(GrpcResponseCode.DELEGATION_RESOURCE_NOT_OWNED.code(),
                codeOf(() -> DelegationContextValidator.requireResourceOwned(999L, STORE_ID)));
        assertDoesNotThrow(() -> DelegationContextValidator.requireResourceOwned(STORE_ID, STORE_ID));
    }
}
