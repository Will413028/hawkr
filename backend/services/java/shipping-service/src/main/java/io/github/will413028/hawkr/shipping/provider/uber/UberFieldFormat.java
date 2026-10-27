package io.github.will413028.hawkr.shipping.provider.uber;

/**
 * 我方值 → Uber wire 值的欄位轉換（66 §6.5 outbound 欄位映射）＋ PII 遮罩（66 §8.6）。
 *
 * <p>🔴 <b>寫成具名函式而不是在組 payload 的地方 inline</b>：66 §6.5 對金額那一格逐字
 * 「TWD 係數 1。⚠️ 仍要寫成明確的轉換函式——<b>那個 1 是幣別的性質，不是恆等式</b>」。
 * inline 的話，第二個幣別進來時沒有任何地方會提醒你這裡需要改。
 */
public final class UberFieldFormat {

    /** 台灣國碼。 */
    private static final String TW_COUNTRY_CODE = "+886";

    private UberFieldFormat() {
    }

    /**
     * 電話 → E.164（66 §6.5：{@code 09xxxxxxxx} → {@code +8869xxxxxxxx}）。
     *
     * <p>已是 {@code +} 開頭者原樣回傳（不重複加國碼）；無法判定者<b>原樣回傳</b>而不是猜
     * ——猜錯的症狀是外送員打不通電話，而那時包裹已經在路上。
     */
    public static String toE164(String phone) {
        if (phone == null || phone.isBlank()) {
            return phone;
        }
        String digits = phone.replaceAll("[^0-9+]", "");
        if (digits.startsWith("+")) {
            return digits;
        }
        if (digits.startsWith("0")) {
            return TW_COUNTRY_CODE + digits.substring(1);
        }
        return digits;
    }

    /**
     * Uber 金額最小單位的係數。
     *
     * <p>🔴 <b>官方 spec 逐字「Value in cents ( ¹/₁₀₀ of currency unit )」，
     * 並附換算例 {@code $10.99 => 1099}——規則是「幣別單位的 ¹⁄₁₀₀」，<u>TWD 沒有豁免</u>。</b>
     * 獨立佐證：台灣業務指南 p.37 逐字「乘以 100」、p.108 的 refund 實例 {@code TWD 7500 ＝ NT$75}。
     *
     * <p>⚠️ <b>本常數曾經是 {@code 1}，那是錯的</b>（66 §6.5 原列「TWD 係數 1」已於 2026-09-14
     * Step 5P-0 確認為缺陷並更正）。它造成的症狀是**金額差 100 倍**，而舊 javadoc 自己就寫著
     * 「靜默套用 1 會讓金額差 100 倍」——它預言了症狀，只是把係數寫反了。
     */
    private static final long TWD_MINOR_UNIT_FACTOR = 100L;

    private static void requireTwd(String currency) {
        if (currency == null || !"TWD".equalsIgnoreCase(currency)) {
            throw new IllegalArgumentException(
                    "只支援 TWD 的最小單位換算，收到 currency=" + currency
                            + "——係數是幣別的性質，不得沿用 TWD 的值");
        }
    }

    /**
     * 金額（新台幣「元」整數）→ Uber 的最小單位（cents）。
     *
     * <p>🔴 <b>邊界規則（66）：DB／domain／MQ payload／對外 API 一律維持元整數不變，
     * 只在本 adapter 邊界做換算。Uber 的 minor unit 不得滲透進 domain。</b>
     *
     * @throws IllegalArgumentException 非 TWD —— fail-closed：靜默套用錯係數會讓金額差 100 倍
     */
    public static long toMinorUnits(long amountInDollars, String currency) {
        requireTwd(currency);
        return amountInDollars * TWD_MINOR_UNIT_FACTOR;
    }

    /**
     * Uber 的最小單位（cents）→ 金額（新台幣「元」整數）。
     *
     * <p>🔴 <b>不整除時 fail-closed，不做四捨五入也不截斷。</b>
     * 理由：TWD 實務上沒有小數位，Uber 回的 {@code fee} 應恆為 100 的倍數；
     * 出現餘數代表<b>我方對這條 wire 的理解錯了</b>（幣別變更、對方改語意、或讀錯欄位）。
     * 靜默截斷會產生一個「看起來正常、卻對不上 provider 帳單」的金額，
     * 而這個值會變成向賣家收款的 {@code logistics_receivable.estimated_cost}
     * ——差額要等月結對帳才會發現，那是一個月以後的事。
     *
     * @throws IllegalArgumentException 非 TWD，或不是 100 的倍數
     */
    public static long fromMinorUnits(long amountInMinorUnits, String currency) {
        requireTwd(currency);
        if (amountInMinorUnits % TWD_MINOR_UNIT_FACTOR != 0) {
            throw new IllegalArgumentException(
                    "Uber 回傳的 TWD 金額不是 " + TWD_MINOR_UNIT_FACTOR + " 的倍數："
                            + amountInMinorUnits
                            + "——TWD 沒有小數位，出現餘數代表對這條 wire 的理解有誤，"
                            + "不得靜默截斷（會產生對不上 provider 帳單的收款金額）");
        }
        return amountInMinorUnits / TWD_MINOR_UNIT_FACTOR;
    }

    /**
     * 外送員電話的遮罩形式。
     *
     * <p>🔴 66 §8.6：外送員電話是<b>可撥打的臨時號碼</b> ⇒ 存遮罩值、原值不落庫不落 log。
     * 判準（8.6 逐字）：「PII 問『這是誰的個資』，<b>憑證問『拿到它能做什麼』</b>」。
     *
     * <p>保留末四碼讓客服對得上人，其餘一律 {@code *}。
     */
    public static String maskPhone(String phone) {
        if (phone == null || phone.isBlank()) {
            return phone;
        }
        String trimmed = phone.trim();
        if (trimmed.length() <= 4) {
            return "*".repeat(trimmed.length());
        }
        return "*".repeat(trimmed.length() - 4) + trimmed.substring(trimmed.length() - 4);
    }

    /**
     * {@code external_store_id} 的遮罩形式。
     *
     * <p>🔴 它是<b>憑證等級</b>的值，不是單純識別碼：66 §8.6 逐字
     * 「洩漏 ＋ token 洩漏 ＝ 可代賣家叫車」⇒ body／resp_body／log 全遮。
     */
    public static String maskStoreId(String externalStoreId) {
        if (externalStoreId == null || externalStoreId.isBlank()) {
            return externalStoreId;
        }
        String v = externalStoreId.trim();
        if (v.length() <= 4) {
            return "*".repeat(v.length());
        }
        return v.substring(0, 2) + "*".repeat(v.length() - 4) + v.substring(v.length() - 2);
    }

    /**
     * 取件時間 ＝ {@code now + prep_sla_minutes}（66 §6.5 outbound 映射表最後一列）。
     *
     * <h2>🔴 報價與建單必須用同一支</h2>
     * 兩條路徑都要送 {@code pickup_ready_dt}，而它們如果各算各的，
     * 症狀是<b>報的價與實際建單的取件時間不一致</b>——而 Uber 的報價本來就與時間有關。
     * 這正是 {@link UberPayloadCodec} 類別註解講的同一個形狀：
     * 兩邊各寫一份「我覺得對方會這樣算」。
     *
     * @param prepSlaMinutes 來自 {@code uber_store_mappings.prep_sla_minutes}；
     *                       {@code null} 用 {@value #DEFAULT_PREP_SLA_MINUTES} 分（66 §5.1.3 的欄位預設）
     */
    public static String pickupReadyDt(java.time.Instant now, Integer prepSlaMinutes) {
        int prep = prepSlaMinutes == null ? DEFAULT_PREP_SLA_MINUTES : prepSlaMinutes;
        return java.time.format.DateTimeFormatter.ISO_INSTANT.format(now.plusSeconds(prep * 60L));
    }

    /** {@code uber_store_mappings.prep_sla_minutes} 缺席時的備貨時間（分）。 */
    public static final int DEFAULT_PREP_SLA_MINUTES = 15;
}
