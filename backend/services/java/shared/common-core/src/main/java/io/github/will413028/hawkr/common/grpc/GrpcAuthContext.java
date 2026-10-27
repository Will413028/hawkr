package io.github.will413028.hawkr.common.grpc;

import io.github.will413028.hawkr.common.utils.code.AccountType;
import io.github.will413028.hawkr.common.utils.code.helper.AccountTypeRules;
import io.grpc.Context;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;

import java.util.Arrays;
import java.util.stream.Collectors;

/**
 * gRPC Context 中的帳號身分存取工具
 *
 * 由 GrpcAuthServerInterceptor 從 metadata 提取後放入 Context.Key，
 * 各 gRPC Service 方法透過此類取用身分資訊與執行授權檢查。
 *
 * 對應 API 設計規範 §4：account_type 以 code 值傳遞
 * x-account-type header 傳遞 code 值字串（如 "1", "2", "3", "4"）
 *
 * 使用 gRPC Context 而非 ThreadLocal，確保 gRPC 生命週期正確管理。
 *
 * 內部微服務呼叫免權限：
 * 透過 GrpcAuthClientInterceptor 自動注入 x-internal-service 標頭，
 * Server 端偵測後標記為內部呼叫，requireAccountType / requireAccountId 自動跳過。
 * 安全由內網隔離與服務白名單保障。
 *
 * account-type 檢查可透過設定 hawkr.auth.skip-account-type-check=true
 * 或環境變數 SKIP_ACCOUNT_TYPE_CHECK=true 停用（測試環境用），
 * JWT 驗證（requireAccountId）仍然有效。
 */
public final class GrpcAuthContext {

    /** gRPC metadata / HTTP header key：帳號 ID */
    public static final String HEADER_ACCOUNT_ID = "x-account-id";

    /** gRPC metadata / HTTP header key：帳號類型（code 值字串） */
    public static final String HEADER_ACCOUNT_TYPE = "x-account-type";

    /** gRPC metadata key：內部服務識別（標記微服務間呼叫；caller 名，資訊用途，不再是信任依據） */
    public static final String HEADER_INTERNAL_SERVICE = "x-internal-service";

    /** gRPC metadata key：內部身分簽章時間戳（unix 秒；#1366 HMAC） */
    public static final String HEADER_INTERNAL_TS = "x-internal-ts";

    /** gRPC metadata key：內部身分簽章 nonce（16 bytes hex；#1366 HMAC，forward-compat） */
    public static final String HEADER_INTERNAL_NONCE = "x-internal-nonce";

    /** gRPC metadata key：內部身分 HMAC-SHA256 簽章（base64；#1366） */
    public static final String HEADER_INTERNAL_SIG = "x-internal-sig";

    /**
     * 呼叫端主體 ID（已登入時不為 null）。
     *
     * <p><b>不保證是 JWT 的 raw account_id。</b>這個 channel 承載的是「呼叫端在當前角色下
     * 的主體 ID」，實際語意由該 RPC 的契約決定：
     * <ul>
     *   <li>買家域（cart / payment 的歸屬檢查等）：gateway 會先把 account_id 解析成
     *       <b>現任 buyer_id</b> 再放進來（buyer_id 是每段 stint 重新 TSID 產生的代理鍵，
     *       與 account_id 永遠沒有數值關係）。</li>
     *   <li>賣家域：同理為已解析的 <b>store_id</b>。</li>
     *   <li>其餘：raw account_id。</li>
     * </ul>
     *
     * <p>Java 服務轉呼叫下游時，若下游要的是已解析的 id，<b>必須</b>用
     * {@code Context.current().withValue(ACCOUNT_ID, resolvedId)} 覆寫 ambient context
     * ——{@link GrpcAuthClientInterceptor} 是全域掛載的，額外附加一份 metadata header
     * 會被它後蓋前（grpc-java 的 {@code Metadata.get()} 是 last-put-wins）。範例見
     * order-service 的 {@code CartGrpcClient.callAsAccount}。
     */
    public static final Context.Key<Long> ACCOUNT_ID = Context.key("grpc-auth-account-id");

    /** 帳號類型 code 值字串（如 "1", "2", "3", "4"） */
    public static final Context.Key<String> ACCOUNT_TYPE = Context.key("grpc-auth-account-type");

    /** 內部微服務呼叫標記（由 GrpcAuthServerInterceptor 從 x-internal-service 標頭設定） */
    public static final Context.Key<Boolean> INTERNAL_CALL = Context.key("grpc-auth-internal-call");

    /** 是否跳過 account-type 檢查（測試環境用） */
    private static volatile boolean skipAccountTypeCheck = false;

    private GrpcAuthContext() {}

    /**
     * 設定是否跳過 account-type 檢查
     * 由 GrpcAuthAutoConfiguration 在啟動時呼叫
     */
    public static void setSkipAccountTypeCheck(boolean skip) {
        skipAccountTypeCheck = skip;
    }

    // ==================== 內部呼叫判別 ====================

    /**
     * 當前請求是否為內部微服務呼叫
     *
     * 由 GrpcAuthServerInterceptor 從 metadata 的 x-internal-service 標頭偵測。
     * 內部呼叫時 requireAccountType / requireAccountId 自動跳過，
     * 安全由內網隔離與服務白名單保障。
     */
    public static boolean isInternalCall() {
        return Boolean.TRUE.equals(INTERNAL_CALL.get());
    }

    // ==================== 取值方法（nullable） ====================

    /**
     * 取得當前 gRPC 請求的呼叫端主體 ID（nullable）。
     *
     * <p>語意見 {@link #ACCOUNT_ID}——買家/賣家域的 RPC 拿到的是已解析的
     * buyer_id / store_id，不是 raw account_id。方法名維持 {@code AccountId} 是因為
     * 它同時服務三種語意的路徑，改名會誤導另外兩種；判斷語意請以呼叫端 RPC 的契約為準。
     */
    public static Long getAccountId() {
        return ACCOUNT_ID.get();
    }

    /**
     * 取得當前 gRPC 請求的 account_type（nullable）
     *
     * 從 x-account-type header 的 code 值字串解析為 AccountType。
     *
     * @return AccountType 或 null（未登入/內部呼叫無身分時）
     */
    public static AccountType getAccountType() {
        String raw = ACCOUNT_TYPE.get();
        if (raw == null || raw.isBlank()) {
            return null;
        }
        try {
            return AccountTypeRules.parse(raw.trim());
        } catch (IllegalArgumentException e) {
            return null;
        }
    }

    /**
     * 檢查當前帳號是否涵蓋指定角色的權限
     *
     * 遵守權限涵蓋規則：SELLER_AND_BUYER 涵蓋 BUYER 與 SELLER。
     *
     * @param type 要檢查的角色
     * @return true 若帳號涵蓋該角色權限
     */
    public static boolean hasAccountType(AccountType type) {
        AccountType current = getAccountType();
        if (current == null) {
            return false;
        }
        return AccountTypeRules.covers(current, type);
    }

    // ==================== 授權斷言方法 ====================

    /**
     * 要求已登入（account_id 不為空）
     *
     * 內部微服務呼叫時不強制要求登入：
     * - 若已傳播 accountId（如 Gateway → A → B 鏈路），回傳傳播值
     * - 若無 accountId（純內部排程/MQ），回傳 0
     *
     * @return account_id（內部呼叫時可能為 0）
     * @throws StatusRuntimeException UNAUTHENTICATED 若非內部呼叫且未登入
     */
    public static long requireAccountId() {
        Long accountId = getAccountId();

        // 內部微服務呼叫：回傳已傳播的 accountId 或 0（不拋異常）
        if (isInternalCall()) {
            return accountId != null && accountId > 0 ? accountId : 0L;
        }

        if (accountId == null || accountId <= 0) {
            throw Status.UNAUTHENTICATED
                    .withDescription("請先登入")
                    .asRuntimeException();
        }
        return accountId;
    }

    /**
     * 要求帳號類型涵蓋指定角色之一
     *
     * 遵守權限涵蓋規則：SELLER_AND_BUYER 涵蓋 BUYER 與 SELLER。
     * 例如：requireAccountType(BUYER) 對 BUYER 和 SELLER_AND_BUYER 皆通過。
     *
     * 內部微服務呼叫時直接跳過（安全由內網隔離保障）。
     *
     * 測試環境可透過 SKIP_ACCOUNT_TYPE_CHECK=true 跳過此檢查，
     * 但仍會驗證 JWT 是否有效（requireAccountId）。
     *
     * @param allowed 允許的角色列表
     * @throws StatusRuntimeException PERMISSION_DENIED 若角色不符
     */
    public static void requireAccountType(AccountType... allowed) {
        // 內部微服務呼叫：直接跳過角色檢查
        if (isInternalCall()) {
            return;
        }

        // 測試環境：跳過 account-type 檢查，但仍驗證 JWT
        if (skipAccountTypeCheck) {
            requireAccountId();
            return;
        }

        AccountType currentType = getAccountType();
        if (currentType == null) {
            throw Status.UNAUTHENTICATED
                    .withDescription("請先登入")
                    .asRuntimeException();
        }
        // 涵蓋判斷：帳號類型涵蓋允許列表中的任一角色即通過
        for (AccountType required : allowed) {
            if (AccountTypeRules.covers(currentType, required)) {
                return;
            }
        }
        String allowedStr = Arrays.stream(allowed)
                .map(a -> a.value() + "(" + a.code() + ")")
                .collect(Collectors.joining(", "));
        throw Status.PERMISSION_DENIED
                .withDescription("無權操作：您的帳號類型無法執行此操作（需要: " + allowedStr + "）")
                .asRuntimeException();
    }

    /**
     * 賣家本人或代行操作者皆可通過（56 §6.2.4）。
     *
     * <p>{@code delegation} 為 null ⇒ 賣家本人路徑，等同 {@code requireAccountType(SELLER)}；
     * 非 null ⇒ 代行路徑，交由 {@link DelegationContextValidator} 跑 S1a–S5，<b>不再檢查
     * account_type</b>（代行者是 ADMIN，本來就不是 SELLER）。
     *
     * <p><b>簽章必須帶 {@code delegation} 參數</b>：gRPC interceptor 看不到 request body，
     * 而本案定版不新增任何 gRPC metadata ⇒ delegation 只能由各 RPC 方法從 request 取出傳入，
     * 無法像 {@link #requireAccountType} 那樣零參數從 {@link Context.Key} 拿。
     *
     * @param delegation 代行語境，取法一律為 {@code req.hasDelegation() ? req.getDelegation() : null}
     * @param storeId    本次 request 的歸屬鍵
     * @param capability 本 RPC 宣告的 capability（{@link DelegationCapability} 的字面常數，
     *                   <b>不得從 request 取</b>——那會讓呼叫方自己宣告自己需要什麼權限）
     */
    public static void requireSellerOrDelegated(DelegationContext delegation, long storeId, String capability) {
        requireSellerOrDelegated(delegation, storeId, capability, "");
    }

    /**
     * @param requestIdempotencyKey request 自身的 idempotency_key（無該欄位時傳空字串）
     * @see #requireSellerOrDelegated(DelegationContext, long, String)
     */
    public static void requireSellerOrDelegated(DelegationContext delegation, long storeId, String capability,
                                                String requestIdempotencyKey) {
        if (delegation == null) {
            requireAccountType(AccountType.SELLER);
            return;
        }
        DelegationContextValidator.validate(delegation, storeId, capability, requestIdempotencyKey);
    }

    // ==================== Ownership 檢查 ====================

    /**
     * 驗證資源擁有權（呼叫端主體 ID 匹配）。
     *
     * <p><b>兩邊必須是同一種 id。</b>{@code resourceOwnerId} 要跟 {@link #ACCOUNT_ID}
     * 這條路徑上實際承載的東西同型：買家域的資源（owner 欄位存的是 buyer_id）就必須
     * 確保呼叫端傳進來的也是已解析的 buyer_id。混用不會報錯，只會永遠不相等，降級成
     * 一個看起來很正常的「無權操作」——這正是 cart-service 的
     * {@code CHECKOUT_LOCK_FAILED} (6005) 事故形狀。
     *
     * @param resourceOwnerId 資源擁有者的主體 ID（與 {@link #ACCOUNT_ID} 同語意，見該欄位說明）
     * @throws StatusRuntimeException PERMISSION_DENIED 若不匹配
     */
    public static void requireOwnership(Long resourceOwnerId) {
        long callerId = requireAccountId();
        if (resourceOwnerId == null || !resourceOwnerId.equals(callerId)) {
            throw Status.PERMISSION_DENIED
                    .withDescription("無權操作此資源")
                    .asRuntimeException();
        }
    }
}
