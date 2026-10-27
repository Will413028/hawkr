package io.github.will413028.hawkr.common.grpc;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;

import java.util.List;

/**
 * {@link DelegationContext} 的判定步驟 S1a–S5（56 §6.2.4）。
 *
 * <p>Go 側的對稱實作是 {@code pkg/delegation.Validate}；兩份只要有一步不同，就會出現
 * 「同一個請求打 product 被拒、打 seller 通過」的分裂授權，而兩邊各自的測試都會通過
 * ⇒ 兩份共用同一組等價性 fixture（E1–E13）。
 *
 * <p><b>S4 只驗得到「歸屬鍵存在」，驗不到「歸屬鍵是對的」</b>：{@link DelegationContext}
 * 刻意不含 target_store_id。「被讀寫的資源真的屬於這個 store」是 Q4，由領域服務在查詢層
 * 以 {@code WHERE id = ? AND store_id = ?} 守住，不符回 9275。
 */
public final class DelegationContextValidator {

    private DelegationContextValidator() {}

    /**
     * 無 idempotency_key 欄位的 request 用（S5 跳過）。
     *
     * @param d          代行語境；{@code null} 代表賣家本人路徑（S0），直接 return
     * @param storeId    本次 request 的歸屬鍵（各 RPC 自 request 取）
     * @param capability 本 RPC <b>宣告</b>的 capability，必須是呼叫處寫死的字面常數
     */
    public static void validate(DelegationContext d, long storeId, String capability) {
        validate(d, storeId, capability, "");
    }

    /**
     * @param requestIdempotencyKey request 自身的 idempotency_key（沒有該欄位時傳空字串，S5 跳過）
     */
    public static void validate(DelegationContext d, long storeId, String capability,
                                String requestIdempotencyKey) {
        // S0：不存在 ⇒ 賣家本人路徑
        if (d == null) {
            return;
        }

        // S1a：三個必填欄位恆驗
        if (isBlank(d.getSessionId()) || isBlank(d.getOperatorAdminId()) || isBlank(d.getCapability())) {
            throw contractMismatch("代行語境缺少必要欄位");
        }

        // S1b：只在必落稽核的操作上驗（.write / .publish），判定輸入是「本 RPC 宣告的 cap」
        if (DelegationCapability.requiresAuditLog(capability) && isBlank(d.getAuditLogId())) {
            throw contractMismatch("代行寫入必須帶稽核列 ID");
        }

        // S2：落在封閉列舉（大小寫敏感）
        if (!DelegationCapability.ALL.contains(d.getCapability())) {
            throw contractMismatch("代行能力宣告與本 RPC 的契約不符");
        }

        // S3：與本 RPC 宣告相符
        if (!d.getCapability().equals(capability)) {
            throw contractMismatch("代行能力宣告與本 RPC 的契約不符");
        }

        // S4：歸屬鍵存在
        if (storeId <= 0) {
            throw contractMismatch("代行請求缺少店家歸屬鍵");
        }

        // S5：request 自身有帶 idempotency_key 時必須與代行語境逐字元相同
        if (!isBlank(requestIdempotencyKey) && !isBlank(d.getIdempotencyKey())
                && !requestIdempotencyKey.equals(d.getIdempotencyKey())) {
            throw contractMismatch("冪等鍵與代行語境不一致");
        }
    }

    /**
     * 批次寫入的判定（56 §7.2.8 步驟 ④）。
     *
     * <p>與 {@link #validate} 的唯一差別在 <b>S1b 的判定對象</b>：批次是 N 筆稽核不是一筆，
     * {@code audit_log_id} 由 Gateway 逐項填在 items 上，{@link DelegationContext} 自身那一欄
     * 沒有值可填 ⇒ S1b 改為「每一項都必須帶 audit_log_id」，任一項為空即整批拒。
     * 空清單同樣拒——否則零項的批次會靜默通過。
     */
    public static void validateBatch(DelegationContext d, long storeId, String capability,
                                     String requestIdempotencyKey, List<String> itemAuditLogIds) {
        if (d == null) {
            return;
        }
        if (itemAuditLogIds == null || itemAuditLogIds.isEmpty()) {
            throw contractMismatch("代行批次必須至少一項");
        }
        for (String auditLogId : itemAuditLogIds) {
            if (isBlank(auditLogId)) {
                throw contractMismatch("代行批次每一項都必須帶稽核列 ID");
            }
        }
        // S1b 已由上方逐項完成，故以 .read 語意跑其餘步驟；S3 仍以真正宣告的 capability 比對
        validateWithoutAuditLog(d, storeId, capability, requestIdempotencyKey);
    }

    private static void validateWithoutAuditLog(DelegationContext d, long storeId, String capability,
                                                String requestIdempotencyKey) {
        if (isBlank(d.getSessionId()) || isBlank(d.getOperatorAdminId()) || isBlank(d.getCapability())) {
            throw contractMismatch("代行語境缺少必要欄位");
        }
        if (!DelegationCapability.ALL.contains(d.getCapability())
                || !d.getCapability().equals(capability)) {
            throw contractMismatch("代行能力宣告與本 RPC 的契約不符");
        }
        if (storeId <= 0) {
            throw contractMismatch("代行請求缺少店家歸屬鍵");
        }
        if (!isBlank(requestIdempotencyKey) && !isBlank(d.getIdempotencyKey())
                && !requestIdempotencyKey.equals(d.getIdempotencyKey())) {
            throw contractMismatch("冪等鍵與代行語境不一致");
        }
    }

    /** Q4 歸屬斷言：資源的 store_id 必須等於 request 的歸屬鍵。 */
    public static void requireResourceOwned(long resourceStoreId, long ownerStoreId) {
        if (resourceStoreId != ownerStoreId) {
            throw new DelegationDeniedException(GrpcResponseCode.DELEGATION_RESOURCE_NOT_OWNED);
        }
    }

    /**
     * 本 validator 產生的每一個拒絕（#2744）。
     *
     * <p>S1a–S5 沒有一條是操作者造成得了的：{@link DelegationContext} 由 Gateway 依工作階段
     * 組裝，不來自任何人手動填的欄位。缺 session_id、capability 落在封閉列舉之外、capability 與
     * 本 RPC 自己的宣告不符 —— 每一條都代表 Gateway 組出了一個與自己契約矛盾的語境，重試多少次
     * 都一樣。操作者的下一步是「去找工程師」，與 9260（你填錯了）和 9280（稍後再試）都不同，
     * 而那個差別正是這三個碼分開存在的理由。
     *
     * <p>⚠️ 這與 Go 側 {@code pkg/delegation.Validate} 的 {@code contractMismatch} 是孿生實作，
     * 兩邊的 E1–E13 fixture 表跑同一組案例；任一邊單獨改動即為漂移。
     */
    private static DelegationDeniedException contractMismatch(String message) {
        return new DelegationDeniedException(GrpcResponseCode.DELEGATION_INTERNAL_CONTRACT_MISMATCH, message);
    }

    private static boolean isBlank(String s) {
        return s == null || s.isEmpty();
    }
}
