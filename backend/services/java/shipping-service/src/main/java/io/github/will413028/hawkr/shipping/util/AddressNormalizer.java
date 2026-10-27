package io.github.will413028.hawkr.shipping.util;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;

import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * PayUni 物流地址正規化。
 *
 * <p>PayUni（黑貓）宅配地址格式要求為「縣市＋鄉鎮市區＋路/街/段＋門牌號(＋樓)」，
 * <b>不接受「里/村」「鄰」資訊</b>（{@code docs/第三方整合/PayUni 物流官方 API 總覽.md}）。
 * 送出含里鄰的地址會被 PayUni 回 {@code DEF01003}，導致宅配建店 / 建單 / 退貨 / 約車失敗。
 *
 * <p>本 helper 負責兩件事：
 * <ol>
 *   <li>strip 掉前端誤塞進 detail 的重複 city+district 前綴（避免拼接後雙倍城市名）。</li>
 *   <li>strip 掉「里/村」與「鄰」token（PayUni 不接受）。</li>
 * </ol>
 *
 * <p>兩種入口：
 * <ul>
 *   <li>{@link #normalizeAddressDetail(String, String, String)}：三段式（city/district 已分離，
 *       用於建店 / 出貨 / 退貨賣家寄退件地址等）。</li>
 *   <li>{@link #normalizeFullAddress(String)}：單一合併地址字串（買家結帳收件地址）。</li>
 * </ul>
 */
public final class AddressNormalizer {

    private static final ObsLogger log = ObsLogger.forClass(AddressNormalizer.class);

    /** 開頭的「里/村」token：里名 1-5 中文字 + 里或村。 */
    private static final Pattern LEADING_VILLAGE = Pattern.compile("^[\\u4e00-\\u9fff]{1,5}[里村]");
    /** 「鄰」token：1-4 位數字 + 鄰。特徵明確（門牌是「號」不是「鄰」），可全域移除。 */
    private static final Pattern NEIGHBORHOOD = Pattern.compile("\\d{1,4}鄰");
    /** 完整地址的「縣市＋鄉鎮市區」前綴（可含前導郵遞區號/空白）。 */
    private static final Pattern CITY_DISTRICT_PREFIX =
            Pattern.compile("^\\s*\\d*\\s*[\\u4e00-\\u9fff]{1,3}[縣市][\\u4e00-\\u9fff]{1,4}[區鄉鎮市]");

    private AddressNormalizer() {}

    /**
     * 三段式地址 detail 正規化：
     * <ol>
     *   <li>若 detail 開頭重複含 city+district，strip 該前綴（保守：只 strip 完整連續前綴）。</li>
     *   <li>strip 開頭的「里/村」與所有「鄰」token。</li>
     * </ol>
     * null / blank detail 原樣回傳；city 或 district 為 null 時跳過前綴 strip，仍執行里鄰 strip。
     */
    public static String normalizeAddressDetail(String city, String district, String detail) {
        if (detail == null || detail.isBlank()) {
            return detail;
        }
        String trimmed = detail.trim();

        // 1. strip 重複 city+district 前綴（保留原保守行為：只 strip 完整連續前綴）
        if (city != null && district != null) {
            String prefix = city + district;
            if (trimmed.startsWith(prefix)) {
                String stripped = trimmed.substring(prefix.length()).trim();
                log.warn("AddressNormalizer.normalizeAddressDetail.prefixStripped",
                        "detail 開頭含 city+district 重複前綴，已 strip。"
                                + "city=" + city + ", district=" + district
                                + ", original=" + detail + ", normalized=" + stripped);
                trimmed = stripped;
            }
        }

        // 2. strip 里/村、鄰（PayUni 不接受）
        return stripVillageAndNeighborhood(trimmed);
    }

    /**
     * 單一合併地址字串正規化（買家收件地址）。
     * 定位「縣市＋鄉鎮市區」前綴後，對其後段 strip 里/村、鄰；前綴（含如「大里區」這類含「里」字的行政區）原樣保留。
     * 定位不到前綴時（買家亂填、缺縣市），保守僅移除「鄰」token（避免誤傷疑似里的路名字）。
     */
    public static String normalizeFullAddress(String address) {
        if (address == null || address.isBlank()) {
            return address;
        }
        String trimmed = address.trim();
        Matcher m = CITY_DISTRICT_PREFIX.matcher(trimmed);
        if (m.find()) {
            String prefix = trimmed.substring(0, m.end());
            String rest = trimmed.substring(m.end());
            return prefix + stripVillageAndNeighborhood(rest);
        }
        // 無法定位縣市+區前綴：保守只移除「鄰」（不動疑似里的中文，避免誤刪路名）
        return NEIGHBORHOOD.matcher(trimmed).replaceAll("").trim();
    }

    /** strip 開頭的「里/村」token，並移除所有「鄰」token。 */
    private static String stripVillageAndNeighborhood(String s) {
        if (s == null || s.isBlank()) {
            return s;
        }
        String result = s.trim();
        // 里/村：只移除開頭（緊接 district 後的村里名），避免誤傷罕見含「里」字的路名
        Matcher village = LEADING_VILLAGE.matcher(result);
        if (village.find()) {
            result = result.substring(village.end()).trim();
        }
        // 鄰：數字+鄰 特徵明確，全域移除（含開頭與非標準位置）
        result = NEIGHBORHOOD.matcher(result).replaceAll("").trim();
        return result;
    }
}
