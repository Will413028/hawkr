package io.github.will413028.hawkr.common.grpc;

import java.util.Set;

/**
 * ADM 代行營運的 capability 封閉列舉（56 §6.1.1，九值）。
 *
 * <p>刻意不進 CodeMeta：它與 Gateway 既有的賣家 capability 同一層概念——授權判定用、
 * 不作為前端業務碼（56 §8.1.3）。Go 側對稱實作在 {@code pkg/delegation}。
 *
 * <p><b>比對一律大小寫敏感</b>：capability 是前端 allowlist 的成員，大小寫也是契約的一部分；
 * 先 normalize 再比會讓 Gateway 核發前端不認得的值而不被伺服端擋下。
 */
public final class DelegationCapability {

    public static final String STOREFRONT_READ = "storefront.read";
    public static final String STOREFRONT_WRITE = "storefront.write";
    public static final String PRODUCTS_READ = "products.read";
    public static final String PRODUCTS_WRITE = "products.write";
    public static final String PRODUCTS_PUBLISH = "products.publish";
    public static final String POSTS_READ = "posts.read";
    public static final String POSTS_WRITE = "posts.write";
    public static final String POSTS_PUBLISH = "posts.publish";
    public static final String ORDERS_READ = "orders.read";

    /** 封閉列舉全集；S2 以此判定。 */
    public static final Set<String> ALL = Set.of(
            STOREFRONT_READ, STOREFRONT_WRITE,
            PRODUCTS_READ, PRODUCTS_WRITE, PRODUCTS_PUBLISH,
            POSTS_READ, POSTS_WRITE, POSTS_PUBLISH,
            ORDERS_READ);

    private DelegationCapability() {}

    /**
     * 該 capability 的操作是否必落稽核（S1b 的判定輸入）。
     *
     * <p>動詞段為 {@code .write} / {@code .publish} 者必落；{@code .read} 一律不落——
     * 讀取路徑從不呼叫 RecordAuditIntent，把它一併要求 audit_log_id 會讓每一支代行
     * 讀取端點在領域服務被打回 9260，而只餵寫入 fixture 的單元測試照樣全綠。
     */
    public static boolean requiresAuditLog(String capability) {
        return capability != null
                && (capability.endsWith(".write") || capability.endsWith(".publish"));
    }
}
