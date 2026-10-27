package io.github.will413028.hawkr.shipping.provider.uber;

import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.ManifestItem;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.UberWaypoint;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HexFormat;
import java.util.List;

/**
 * {@code uber_quotes.payload_hash}（66 §5.1.4）。
 *
 * <pre>
 * SHA256( external_store_id | canonical(dropoff) | canonical(items 依 sku_id 排序) | packing_policy_version )
 * </pre>
 *
 * <h2>🔴 它回答的問題是「這是不是同一筆購物車」</h2>
 * 用途有兩個，而且方向相反：
 * <ul>
 *   <li><b>報價</b>：hash 相同且剩餘 &gt; 180 秒 ⇒ 直接回既有報價，<b>不打 Uber</b>
 *       （E1 的 token 有 100 req/hr 上限，且每次重報都換一個 {@code quote_id}）</li>
 *   <li><b>confirm 前重驗</b>：hash 不同 ⇒ 購物車在報價之後被改過 ⇒ 不得用舊價 capture</li>
 * </ul>
 *
 * <h2>🔴 順序固定，缺一即 66 A9</h2>
 * A9 的形狀是「換了取件門市但 hash 相同 ⇒ 用舊報價建<b>新門市</b>的單」。
 * ⇒ {@code external_store_id} 必須在 hash 裡，而且必須在<b>第一段</b>——
 * 把它漏掉的症狀不是報錯，是報價與實際取件地點對不上，而金額已經收了。
 *
 * <h2>🔴 hash 由伺服端算，前端不得傳</h2>
 * 66 §5.1.4 逐字。前端傳 hash ＝ 讓前端決定「這是不是同一筆購物車」，
 * 而那正是 INV-U1（永不補收）要守住的那一格。
 *
 * <h2>為什麼自己寫 canonical 形式而不是 {@code toString()}</h2>
 * {@code record} 的 {@code toString()} 沒有穩定性保證（欄位增刪、JDK 版本都會改變它），
 * 而 hash 一旦寫進 DB 就必須在<b>未來的行程</b>算得出同一個值。
 * ⇒ 逐欄明列、用分隔符連接，欄位增減時<b>必須有人決定要不要進 hash</b>。
 *
 * <p>⚠️ 分隔符用 {@code U+001F}（UNIT SEPARATOR）而不是逗號或直線：
 * 它不可能出現在地址、品名或 id 裡 ⇒ 不存在「把分隔符打進欄位值就能偽造出相同 hash」的空間。
 */
public final class UberQuotePayloadHash {

    /** 欄位分隔符 {@code U+001F}。見類別註解：選它是為了不可能與欄位值碰撞。 */
    private static final char SEP = (char) 0x1F;

    private UberQuotePayloadHash() {
    }

    /**
     * @param items 🔴 進來的順序<b>不影響</b>結果：本方法內依 {@code externalId}（sku_id）排序。
     *              購物車重排不是「換了一筆購物車」，用進來的順序算會讓同一車算出不同 hash ⇒ 白重報一次。
     * @return 64 個小寫 hex 字元（對應 {@code payload_hash CHAR(64)}）
     */
    public static String of(String externalStoreId,
                            UberWaypoint dropoff,
                            List<ManifestItem> items,
                            Integer packingPolicyVersion) {
        StringBuilder sb = new StringBuilder();
        sb.append(nullSafe(externalStoreId)).append(SEP);
        appendWaypoint(sb, dropoff);
        appendItems(sb, items);
        sb.append(packingPolicyVersion == null ? "" : packingPolicyVersion.toString());
        return sha256Hex(sb.toString());
    }

    /**
     * 🔴 只取<b>會改變報價</b>的欄位。
     *
     * <p>{@code name} 與 {@code phone} 刻意<b>不進 hash</b>：改收件人姓名不會改變運費，
     * 把它們算進去等於每改一個字就重打一次 E1，而 E1 有 100 req/hr 的上限。
     * ⚠️ 反過來，{@code notes}（樓層）也不進——它同樣不影響距離。
     */
    private static void appendWaypoint(StringBuilder sb, UberWaypoint wp) {
        if (wp == null) {
            sb.append(SEP).append(SEP).append(SEP);
            return;
        }
        sb.append(nullSafe(wp.structuredAddressJson())).append(SEP);
        sb.append(wp.latitude() == null ? "" : wp.latitude().toString()).append(SEP);
        sb.append(wp.longitude() == null ? "" : wp.longitude().toString()).append(SEP);
    }

    /** 逐項 {@code externalId/quantity/size}；排序鍵是 {@code externalId}（＝sku_id）。 */
    private static void appendItems(StringBuilder sb, List<ManifestItem> items) {
        List<ManifestItem> sorted = new ArrayList<>(items == null ? List.of() : items);
        sorted.sort(Comparator.comparing(i -> nullSafe(i.externalId())));
        for (ManifestItem item : sorted) {
            sb.append(nullSafe(item.externalId())).append(SEP);
            sb.append(item.quantity()).append(SEP);
            sb.append(nullSafe(item.size())).append(SEP);
        }
    }

    private static String sha256Hex(String input) {
        try {
            MessageDigest md = MessageDigest.getInstance("SHA-256");
            return HexFormat.of().formatHex(md.digest(input.getBytes(StandardCharsets.UTF_8)));
        } catch (NoSuchAlgorithmException e) {
            // SHA-256 是 JDK 必備演算法；走到這裡代表 runtime 被改過。
            throw new IllegalStateException("SHA-256 不可用", e);
        }
    }

    private static String nullSafe(String s) {
        return s == null ? "" : s;
    }
}
