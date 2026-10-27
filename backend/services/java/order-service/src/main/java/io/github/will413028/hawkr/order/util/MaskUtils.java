package io.github.will413028.hawkr.order.util;

/**
 * 代行讀取訂單時的收件資訊遮罩（56 §8.1.5）。
 *
 * <p>遮罩在 order-service 執行、不在 Gateway：放 Gateway 等於完整收件資訊在每一次列表請求時
 * 跨 gRPC 傳輸並落進 Gateway 記憶體與（若有疏漏）log，把 deny-by-default 降級成「傳過來再遮掉」。
 *
 * <p>兩個方法皆為純靜態、null-safe（傳 null 回 null）。收件人姓名不遮——姓名對「確認是不是
 * 同一筆訂單」是必要的，且不足以單獨定位個人。
 */
public final class MaskUtils {

    /** 手機保留的前綴長度 */
    private static final int PHONE_PREFIX = 4;
    /** 手機保留的後綴長度 */
    private static final int PHONE_SUFFIX = 3;
    /** 短於此長度一律全遮，否則遮罩後反而露出大部分 */
    private static final int PHONE_MIN_LENGTH = 8;
    /** 找不到行政區字元時的保留長度 */
    private static final int ADDRESS_FALLBACK_LENGTH = 6;

    /** 鄉鎮市區級字元。區／鄉／鎮優先於市，理由見 {@link #maskAddressToDistrict} */
    private static final char[] DISTRICT_MARKERS = {'區', '鄉', '鎮'};

    private MaskUtils() {}

    /**
     * 手機遮罩：保留前 4 碼與後 3 碼，中間固定 3 個 {@code *}（{@code 0912-***-234}）。
     * 長度不足 8 碼者全遮。
     */
    public static String maskPhone(String phone) {
        if (phone == null || phone.isEmpty()) {
            return phone;
        }
        if (phone.length() < PHONE_MIN_LENGTH) {
            return "***";
        }
        return phone.substring(0, PHONE_PREFIX)
                + "-***-"
                + phone.substring(phone.length() - PHONE_SUFFIX);
    }

    /**
     * 地址遮罩：只回 city ＋ district 兩級，不回路名、門牌、樓層。
     *
     * <p>{@code orders} 表沒有分離的收件人 city／district 欄位（只有一個 {@code shipping_address}），
     * 故採近似規則而非中文地址 parser——parser 是一個沒有維護者的規則庫，且它的錯誤方向是
     * 「遮太少」（洩漏）；近似規則的錯誤方向是「遮太多」（可接受）。
     *
     * <p><b>區／鄉／鎮先於市</b>：直轄市地址（台北市大安區…）若取第一個「區鄉鎮市」會停在「市」，
     * district 永遠是空的，而代行 DTO 的 {@code recipient} 是 {@code {name, maskedPhone, city, district}}
     * 兩欄分開 ⇒ 那個契約會直接對不上。縣轄市（新竹縣竹北市…）沒有區鄉鎮字元，才落到「市」，
     * 此時「市」正是它的 district。兩種形狀都止於行政區級，不會露出街道。
     */
    public static String maskAddressToDistrict(String address) {
        if (address == null || address.isEmpty()) {
            return address;
        }
        int cut = -1;
        for (char marker : DISTRICT_MARKERS) {
            int idx = address.indexOf(marker);
            if (idx >= 0 && (cut < 0 || idx < cut)) {
                cut = idx;
            }
        }
        if (cut < 0) {
            cut = address.indexOf('市');
        }
        if (cut < 0) {
            return address.length() <= ADDRESS_FALLBACK_LENGTH
                    ? address : address.substring(0, ADDRESS_FALLBACK_LENGTH);
        }
        return address.substring(0, cut + 1);
    }
}
