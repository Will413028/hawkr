package io.github.will413028.hawkr.product.dto.response;

/**
 * 認證未被採用的提示（19 號設計書《農業認證驗證》§6.3.1／§9.5）。
 *
 * <p><b>為什麼一定要有這個欄位</b>：被剔除／被跳過的那一筆<b>既不在 approved-only 投影、
 * 也不在申請表</b> ⇒ 賣家表單一 re-hydrate 就消失了，根本沒有「下次儲存會自動重送」這回事。
 * 不告訴賣家，那筆認證就是靜默蒸發。
 *
 * @param certificationType <b>原樣回送賣家送來的值</b>（可能是非法值，例如中文標籤）
 *                          ⇒ 前端不得假設它是合法碼
 * @param certificateNo     同上
 * @param reason            {@link #REASON_INVALID_TYPE} / {@link #REASON_INVALID_CERT_NO}
 *                          / {@link #REASON_CONCURRENT_CONFLICT}
 * @param message           可直接顯示的中文說明
 */
public record CertificationWarningDTO(String certificationType,
                                      String certificateNo,
                                      String reason,
                                      String message) {

    /** 認證類型不是 {@code CertificationType} 的合法 value（3701–3706）。 */
    public static final String REASON_INVALID_TYPE = "INVALID_TYPE";

    /** 認證編號為空／含白名單外字元／超出長度上限。 */
    public static final String REASON_INVALID_CERT_NO = "INVALID_CERT_NO";

    /**
     * §6.3 Step 5 的 {@code ON CONFLICT DO NOTHING} 未插入任何列。
     *
     * <p>序列情境插不出衝突（Step 4 已先清掉同 type 的異號待審），<b>只有併發才會走到這裡</b>：
     * T2 撞到 T1 未提交的 speculative insertion → 等待 → T1 commit → T2 走 DO NOTHING，
     * 無列、無錯。不顯式檢查就是靜默資料遺失。
     */
    public static final String REASON_CONCURRENT_CONFLICT = "CONCURRENT_CONFLICT";

    public static CertificationWarningDTO invalidType(String type, String certNo) {
        return new CertificationWarningDTO(nullToEmpty(type), nullToEmpty(certNo),
                REASON_INVALID_TYPE, "認證類型格式不正確，這一筆未被採用");
    }

    public static CertificationWarningDTO invalidCertNo(String type, String certNo) {
        return new CertificationWarningDTO(nullToEmpty(type), nullToEmpty(certNo),
                REASON_INVALID_CERT_NO, "認證編號格式不正確，這一筆未被採用");
    }

    public static CertificationWarningDTO concurrentConflict(String type, String certNo) {
        return new CertificationWarningDTO(nullToEmpty(type), nullToEmpty(certNo),
                REASON_CONCURRENT_CONFLICT,
                "另一個視窗剛送出同類型的認證，這一筆未被採用，請重新儲存");
    }

    private static String nullToEmpty(String v) {
        return v == null ? "" : v;
    }
}
