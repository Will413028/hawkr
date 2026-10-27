package io.github.will413028.hawkr.shipping.provider.uber;

import io.github.will413028.hawkr.shipping.util.AddressNormalizer;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.util.Set;

/**
 * 我方地址三欄 → Uber 的<b>結構化地址物件</b>（66 §6.5）。
 *
 * <h2>🔴 這裡不是「地址解析器」，而且刻意不是</h2>
 * 66 §6.5 把收件地址標為「本節最大落差」，前提是
 * 「{@code orders.shipping_address} 是<b>單一字串</b>」——那句話對 <b>order 的攤平快照</b>成立，
 * 對<b>權威來源</b>不成立：{@code hawkr_buyer.shipping_addresses} 實讀有
 * {@code city}／{@code district}／{@code detail_address} <b>三個獨立欄位</b>，
 * 而 {@code buyer.proto} 的 {@code AddressInfo} 原樣暴露它們。
 *
 * <p>⇒ 正確的做法是<b>回頭讀結構化的那一份</b>，而不是對攤平後的字串做逆向工程。
 * 對字串反解的解析率是一個要量測、會退化、且每個沒解出來的樣本都等於一個看不到選項的買家；
 * 讀權威來源沒有這個風險。⚠️ 因此本類別<b>不接受單一字串</b>——那個多載一旦存在，
 * 就會有呼叫端拿 {@code orders.shipping_address} 餵進來，而那正是要避免的路徑。
 *
 * <h2>行政層級怎麼對應到 street／city／state</h2>
 * 台灣的兩層行政區劃對到國際位址欄位，本 repo 的定案（與 Google address component 的
 * 台灣慣例一致）：
 * <ul>
 *   <li>{@code state} ← <b>縣市</b>（第一級，如「台北市」「新竹縣」）</li>
 *   <li>{@code city} ← <b>鄉鎮市區</b>（第二級，如「大安區」「竹北市」）</li>
 *   <li>{@code street} ← {@code detail_address} 去掉樓層／室之後的部分</li>
 *   <li>{@code country} ← {@code TW}（ISO 3166-1 alpha-2）</li>
 * </ul>
 *
 * <h2>🔴 {@code zip_code} 不送——理由是「不該在這裡造第三份拷貝」，不是「查不到」</h2>
 * ⚠️ <b>2026-09-12 更正</b>：本節原本寫「本 repo 沒有那份對照表的權威來源」，<b>那句話是錯的</b>。
 * 實查存在兩份：權威來源在 {@code hawkr_buyer/lib/shared/utils/taiwan_address_data.dart}，
 * {@code hawkr-website/lib/addresses/taiwan-postal-codes.ts} 是由它生成的拷貝
 * （該檔第一行逐字「Generated from …taiwan_address_data.dart」），而且宅配已經在用它。
 *
 * <p>更正後的理由有兩層：
 * <ul>
 *   <li><b>後端拿不到</b>：{@code shipping_addresses} 沒有郵遞區號欄，兩支 App 是<b>用戶端</b>推導的
 *       ⇒ 這個值沒有跟著地址進到後端</li>
 *   <li><b>不該在後端複製第三份</b>（10 檔 §15）：一份會漂移的行政區對照表，
 *       漂移的症狀是包裹被導到別的行政區，而且完全無聲</li>
 * </ul>
 * ⇒ 真要送 zip，正解是讓它<b>跟著地址一起進到後端</b>（新增欄位或由呼叫端帶），
 * 不是在 shipping-service 內建一份表。
 * ⚠️ 官方文件（{@code docs/第三方整合/Uber Direct API 總覽.md} §2.1）只載
 * {@code pickup_address}／{@code dropoff_address} 為必填，<b>沒有逐欄列出結構</b>
 * ⇒「Uber 是否要求 zip_code」屬未查證，登記為 Enablement Blocker。
 *
 * <h2>🔴 樓層／室進 notes 不進 street</h2>
 * 66 §6.5 逐字「塞進 street 會讓地理編碼失敗」。切點錨在<b>第一個「號」</b>：
 * 「號」之前（含）是門牌、之後是樓層與室。
 *
 * <p>⚠️ <b>第一個而不是最後一個</b>，而這一格我一開始判錯了。台灣門牌的結構是
 * 「路／街 → 段 → 巷 → 弄 → 號」，「號」是門牌的<b>終結符</b>，它之後不可能再有門牌資訊。
 * 反過來，樓層與室<b>可以</b>含「號」（「1樓之2號房」「B1號櫃」）
 * ⇒ 用最後一個當錨會把樓層的一部分吃進 street，而那正是本節要避免的事。
 * <p>⚠️「之」<b>不能</b>當錨：它在門牌裡也合法（「1之2號」），用它切會把門牌截斷。
 */
public final class UberAddressStructure {

    /** ISO 3166-1 alpha-2。 */
    public static final String COUNTRY_TW = "TW";

    /**
     * 台灣第一級行政區（22 個）。
     *
     * <p>🔴 這份清單的用途是<b>驗證</b>不是<b>推導</b>：{@code city} 欄拿到的值不在其中，
     * 代表買家地址的權威來源本身就壞了（或欄位語意被改過）⇒ fail-closed 成
     * {@code ADDRESS_UNPARSEABLE}，而不是帶著壞值去打 Uber 拿一個指向 Uber 的錯誤訊息。
     *
     * <p>⚠️「台」與「臺」兩種寫法都收：{@link #normalizeCounty} 把「臺」正規化成「台」再比對，
     * 兩者在真實資料裡混用，而它們是同一個行政區。
     */
    private static final Set<String> TW_COUNTIES = Set.of(
            "台北市", "新北市", "桃園市", "台中市", "台南市", "高雄市",
            "基隆市", "新竹市", "嘉義市",
            "新竹縣", "苗栗縣", "彰化縣", "南投縣", "雲林縣", "嘉義縣",
            "屏東縣", "宜蘭縣", "花蓮縣", "台東縣", "澎湖縣", "金門縣", "連江縣");

    /**
     * 🔴 <b>結構化地址的 wire 欄名，四個常數集中在這裡。</b>
     *
     * <p>值取自本 repo 的 66 §6.5 逐字「結構化物件（street／city／state／zip／country）」，
     * <b>不是</b>取自我對 Uber API 的印象——{@code docs/第三方整合/Uber Direct API 總覽.md} §2.1
     * 只載了 {@code pickup_address}／{@code dropoff_address} 為必填，<b>沒有逐欄列出結構</b>。
     *
     * <p>⚠️ ⇒「這四個欄名與 Uber 實際接受的形狀是否一致」屬<b>未查證的外部契約</b>，
     * 登記為 Enablement Blocker（要用 sandbox 的第一次 E1 回應坐實）。
     * 把它們集中成常數，是為了讓那次坐實之後<b>只有一個地方要改</b>；
     * 散在 {@code n.put("…")} 裡的話，改動點會分佈到每一個組 payload 的位置。
     */
    static final String KEY_STREET = "street";
    static final String KEY_CITY = "city";
    static final String KEY_STATE = "state";
    static final String KEY_COUNTRY = "country";

    private UberAddressStructure() {
    }

    /**
     * 結構化結果。
     *
     * @param addressJson Uber {@code *_address} 的物件 JSON（欄名為 Uber wire 的 snake_case）
     * @param notes       樓層／室，進 {@code *_notes}。無則為 {@code null}
     */
    public record Structured(String addressJson, String notes) {
    }

    /**
     * @param county 第一級行政區（{@code shipping_addresses.city}）
     * @param district 第二級行政區（{@code shipping_addresses.district}）
     * @param detail 門牌與樓層（{@code shipping_addresses.detail_address}）
     * @throws UberAddressUnparseableException 任一欄缺席，或 {@code county} 不在 22 個第一級行政區內
     */
    public static Structured of(ObjectMapper objectMapper, String county, String district, String detail) {
        String c = normalizeCounty(county);
        if (c == null || !TW_COUNTIES.contains(c)) {
            throw new UberAddressUnparseableException(
                    "縣市不在台灣第一級行政區值域內（收到：" + safe(county) + "）");
        }
        if (isBlank(district)) {
            throw new UberAddressUnparseableException("鄉鎮市區為空");
        }
        if (isBlank(detail)) {
            throw new UberAddressUnparseableException("門牌為空");
        }

        // 🔴 先過 AddressNormalizer，兩件事都不是可選的：
        //  ① strip 掉前端誤塞進 detail 的重複「縣市＋鄉鎮市區」前綴 ——
        //     不 strip 的話結構化地址會同時在 state／city 與 street 裡各帶一次城市名，
        //     而重複的城市名正是地理編碼失敗的典型成因（症狀是 Uber 回 400 或定位到別處）。
        //  ② strip 掉「里／村／鄰」token —— 那是 PayUni 明文不接受的（DEF01003），
        //     而 Uber 這一側沒有官方說法 ⇒ 沿用同一個保守處置，理由見下。
        //
        // ⚠️ 本類別原本自己從 detail 切，**漏了這一層**（2026-09-12 施工時實查補上）。
        //    它已經存在於本服務的 util 底下，是 10 檔 §15 的漏網。
        String normalizedDetail = AddressNormalizer.normalizeAddressDetail(county, district, detail);
        if (isBlank(normalizedDetail)) {
            throw new UberAddressUnparseableException("門牌在正規化後為空");
        }
        String trimmedDetail = normalizedDetail.trim();
        int cut = trimmedDetail.indexOf('號');
        String street;
        String notes;
        if (cut >= 0 && cut < trimmedDetail.length() - 1) {
            street = trimmedDetail.substring(0, cut + 1);
            notes = trimmedDetail.substring(cut + 1).trim();
            if (notes.isEmpty()) {
                notes = null;
            }
        } else {
            // 沒有「號」，或「號」就是結尾 ⇒ 整串都是門牌，沒有樓層資訊。
            // 🔴 不猜：切不出來就不切，而不是用長度或其他字元硬找一個切點。
            //    ⚠️「之」尤其不能拿來當備用錨——它在門牌裡合法（「1之2號」）。
            street = trimmedDetail;
            notes = null;
        }

        ObjectNode n = objectMapper.createObjectNode();
        n.put(KEY_STREET, street);
        n.put(KEY_CITY, district.trim());
        n.put(KEY_STATE, c);
        n.put(KEY_COUNTRY, COUNTRY_TW);
        return new Structured(n.toString(), notes);
    }

    /** 「臺」→「台」，並去除空白。回 {@code null} 表示輸入為空。 */
    static String normalizeCounty(String county) {
        if (isBlank(county)) {
            return null;
        }
        return county.trim().replace('臺', '台');
    }

    private static boolean isBlank(String s) {
        return s == null || s.isBlank();
    }

    private static String safe(String s) {
        return s == null ? "(null)" : s;
    }
}
