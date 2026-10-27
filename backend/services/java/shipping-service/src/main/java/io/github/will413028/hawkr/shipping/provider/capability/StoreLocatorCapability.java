package io.github.will413028.hawkr.shipping.provider.capability;

import java.util.Map;

/** 門市地圖能力（買家選店）。只有超商軸有。 */
public interface StoreLocatorCapability {

    // ── buildRedirectParams 的回傳鍵名（呼叫端一律引用常數，不得自己打字串） ──────────
    /** provider 表單的提交位址 */
    String PARAM_FORM_ACTION = "formAction";
    /** provider 端的商家識別 */
    String PARAM_MERCHANT_ID = "merchantId";
    /** provider 表單的版本 */
    String PARAM_VERSION = "version";
    /** 加密後的請求內容 */
    String PARAM_PAYLOAD = "payload";
    /** 請求內容的簽章 */
    String PARAM_SIGNATURE = "signature";
    /** 我方中轉頁的一次性導向網址（瀏覽器實際打開的位址） */
    String PARAM_REDIRECT_URL = "redirectUrl";

    // ── parseResult 的輸入鍵名 ────────────────────────────────────────────────
    /** provider 導回的加密內容 */
    String RESULT_PAYLOAD = "payload";
    /** provider 導回的簽章 */
    String RESULT_SIGNATURE = "signature";
    /** 發起端種類（{@code buyer} / {@code seller} / {@code web}），決定導回出口 */
    String RESULT_CLIENT_KIND = "clientKind";

    /**
     * 組出導向 provider 門市地圖所需的參數。
     *
     * <p>⚠️ 回傳的是<b>中性的 key→value</b>：呼叫端只負責把它們原樣送出，
     * 不得知道哪一個 key 是哪個 provider 的加密欄位。鍵名見本介面的 {@code PARAM_*} 常數。
     */
    Map<String, String> buildRedirectParams(StoreMapRequest request);

    /**
     * 解析 provider 導回的選店結果。
     *
     * <p>🔴 <b>無法解析時丟例外（fail-closed：不猜門市）。</b>
     * 猜一個空門市代碼的後果不是報錯，是買家帶著空的 storeCode 完成結帳，
     * 而錯誤會出現在出貨當天。
     */
    StoreSelection parseResult(Map<String, String> rawResult);

    /**
     * 門市地圖請求。
     *
     * <p>⚠️ <b>刻意不帶溫層與正逆向</b>：本介面的前一版帶了 {@code temperature} 與 {@code isReturn}，
     * 而實查 provider 端的門市地圖對兩者都不分——固定以冷凍門市查詢（冷凍門市可處理所有溫層），
     * 退貨選店與取貨選店走同一張地圖。帶著永遠不被讀的參數，會讓下一個人以為它有作用。
     *
     * @param principalId 發起選店的登入 principal（provider 以此回填識別，供導回時比對）
     * @param clientKind  發起端種類（{@code buyer} / {@code seller} / {@code web}）
     * @param mobile      是否為手機版版面
     */
    record StoreMapRequest(long principalId, String clientKind, boolean mobile) {}

    /**
     * @param storeCode    門市代碼。⚠️ 這是 CVS 門市代號，<b>與賣家 store surrogate 是完全無關的兩個東西</b>
     * @param insularArea  離島註記（影響可否配送與運費級距）
     * @param storeType    門市型態（provider 的通路分類）
     * @param redirectUrl  選店完成後的導回出口（deep link 或網頁）
     */
    record StoreSelection(String storeCode, String storeName, String storeAddress,
                          String insularArea, String storeType, String redirectUrl) {}
}
