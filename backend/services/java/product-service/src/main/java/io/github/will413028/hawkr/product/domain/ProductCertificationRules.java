package io.github.will413028.hawkr.product.domain;

import io.github.will413028.hawkr.common.utils.code.CertificationType;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationSource;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationStatus;
// 嚴格類的字元白名單與長度上限唯一來源。設計書 §6.2.1 ③ 明令「不得再新增第三份字面值」，
// 因此這裡引用 service 層的 static（純函式、無執行期耦合），而不是把 regex 再抄一份到 domain。
import io.github.will413028.hawkr.product.service.GovCertificationService;

import java.util.Map;
import java.util.Set;

/**
 * 商品認證申請的業務規則（19 號設計書《農業認證驗證》§5.1 狀態機、§8.5 自動核准適用範圍）。
 *
 * <p>純函式 helper。CodeMeta enum 是跨語言公共型別，<b>不得</b>承載業務輔助邏輯
 * （docs/開發規範/Java 微服務開發規範.md §6），所以「哪些轉移合法」「哪些類型可自動核准」
 * 放在這裡而不是 {@link ProductCertificationStatus} 內。
 */
public final class ProductCertificationRules {

    private ProductCertificationRules() {
    }

    /**
     * §5.1 合法轉移表。四個終態（REJECTED／WITHDRAWN／SUPERSEDED／REVOKED）不再轉出
     * ⇒ 它們不在 key 集合內，{@link #isLegalTransition} 對它們一律回 false。
     */
    private static final Map<String, Set<String>> LEGAL_TRANSITIONS = Map.of(
            ProductCertificationStatus.SUBMITTED.value(), Set.of(
                    ProductCertificationStatus.APPROVED.value(),
                    ProductCertificationStatus.REJECTED.value(),
                    ProductCertificationStatus.WITHDRAWN.value(),
                    ProductCertificationStatus.SUPERSEDED.value()),
            ProductCertificationStatus.APPROVED.value(), Set.of(
                    ProductCertificationStatus.SUPERSEDED.value(),
                    ProductCertificationStatus.WITHDRAWN.value(),
                    ProductCertificationStatus.REVOKED.value()));

    /** 該狀態轉移是否合法（§5.1）；不合法時呼叫端回 {@code 3027}。 */
    public static boolean isLegalTransition(String fromStatus, String toStatus) {
        if (fromStatus == null || toStatus == null) {
            return false;
        }
        return LEGAL_TRANSITIONS.getOrDefault(fromStatus, Set.of()).contains(toStatus);
    }

    /**
     * 該類型是否允許<b>系統自動核准</b>（§8.5，實測導致的收窄）。
     *
     * <p>「查得到就自動核准」的隱含前提是「查得到 ＝ 證書有效且屬於這個商品」，
     * 【實測】此前提只對 3701 有機與 3702 產銷履歷成立：
     * <ul>
     *   <li><b>3701 有機</b> ✅ 有權威 {@code Status} 欄（通過／結束／終止／暫終）＋ 效期，編號一對一。</li>
     *   <li><b>3702 產銷履歷</b> ⚠️ 技術上允許：無狀態欄（只能靠 {@code ValidDate} 推），
     *       且 API 實測 33–82 秒 ⇒ 同步快取幾乎永遠 miss，實務上幾乎全走人工審。</li>
     *   <li><b>3703 CAS</b> ❌ {@code Emblem_ID} 是<b>工廠</b>標誌編號，實測 {@code 011002} 對到
     *       <b>173 個產品</b> ⇒ 查得到只證明「該工廠有 CAS 資格」，<b>不證明這個商品是 CAS 產品</b>。
     *       自動核准等於讓賣家用工廠編號給任意商品掛 CAS 標章。</li>
     *   <li><b>3704／3705／3706</b> ❌ 無政府開放資料介面，落 {@code UNSUPPORTED}，永遠人工審。</li>
     * </ul>
     */
    public static boolean isAutoApprovable(CertificationType type) {
        return type == CertificationType.ORGANIC || type == CertificationType.TAP;
    }

    /**
     * 依申請來源決定稽核事件的 {@code actor_role}（§4.2）。
     *
     * <p>賣家自助與代行是<b>不同的人</b>做同一件事，稽核必須分得出來；匯入／seed／存量遷移
     * 沒有具體的人在按按鈕，一律記 SYSTEM。
     */
    public static String actorRoleForSource(String source) {
        if (ProductCertificationSource.SELLER.value().equals(source)) {
            return ACTOR_SELLER;
        }
        if (ProductCertificationSource.DELEGATED_ADMIN.value().equals(source)) {
            return ACTOR_ADMIN;
        }
        return ACTOR_SYSTEM;
    }

    /** {@code product_certification_review_event.actor_role} 的三個值（DB VARCHAR(8)）。 */
    public static final String ACTOR_SYSTEM = "SYSTEM";
    public static final String ACTOR_SELLER = "SELLER";
    public static final String ACTOR_ADMIN = "ADMIN";

    /**
     * 依認證類型正規化並驗證證書編號（19 號設計書 §6.2.9）。<b>Gateway filter 已照同一條規則實作，
     * 這裡是第二道防線</b>（gRPC 直呼、匯入路徑、未來新增的第五條入口都只會經過這裡）。
     *
     * <table>
     *   <caption>兩類規則</caption>
     *   <tr><th>類型</th><th>規則</th></tr>
     *   <tr><td>3701／3702／3703（可線上驗證）</td>
     *       <td>嚴格 {@code [A-Za-z0-9_-]{1,64}}，<b>對原值判、不 trim</b></td></tr>
     *   <tr><td>3704／3705／3706（僅人工審核）</td>
     *       <td>trim 後非空、<b>字元數</b> ≤ 64、擋控制字元與 {@code <} {@code >}；其餘允許</td></tr>
     * </table>
     *
     * <p>🔴 <b>為什麼不能對六型一律套嚴格白名單</b>：{@code ISO 22000:2018} 含冒號、HACCP／GMP 的
     * 證書編號常含空格與斜線 ⇒ 這三型的<b>真實編號會被剔除</b>，而剔除的症狀是
     * 「賣家送出成功、不建申請、不回錯」——零訊號。前三型的值要組進政府 API 的查詢字串，
     * 格式由對方決定；後三型不會被組進任何外部 URL，格式由 ADM 人眼判斷。
     * advisory 查驗端點對這三型本來就不套白名單，本規則與它一致。
     *
     * <p>🔴 <b>嚴格類刻意不 trim</b>：它與 {@link GovCertificationService#isValidCertNo} 是同一條規則的
     * 拷貝，多做一次 trim 會讓「兩邊逐字一致」變得不精確。
     *
     * <p>🔴 <b>驗什麼就存什麼</b>：放寬類回傳的是 <b>trim 後</b>的值，呼叫端必須拿回傳值寫進 DB。
     * 若「驗 trim 後的值、存原值」，從試算表貼進來的尾端換行會通過驗證然後原樣落進申請表
     * ——驗證看起來有做，被擋的東西卻進去了。
     *
     * <p>⚠️ <b>長度用字元數不是位元組</b>：GMP 編號可能是「食品GMP認證字第○○號」，
     * 用 byte 長度會讓 22 個中文字就爆掉。
     *
     * @return 正規化後、可直接寫進 DB 的編號；<b>{@code null} = 不合法（呼叫端剔除或回業務碼）</b>
     */
    public static String normalizeCertNo(CertificationType type, String rawCertNo) {
        if (type == null) {
            return null;
        }
        if (CertificationTypeRules.isVerifiable(type)) {
            // 嚴格類：原值直接餵同一支 static，不 trim、不改寫
            return GovCertificationService.isValidCertNo(rawCertNo) ? rawCertNo : null;
        }
        if (rawCertNo == null) {
            return null;
        }
        String normalized = rawCertNo.trim();
        if (normalized.isEmpty()) {
            return null;
        }
        if (normalized.codePointCount(0, normalized.length())
                > GovCertificationService.CERT_NO_MAX_LENGTH) {
            return null;
        }
        for (int i = 0; i < normalized.length(); ) {
            int cp = normalized.codePointAt(i);
            // 控制字元（含 tab／換行／零寬控制）與角括號一律擋：前者讓值在 log 與 ADM 畫面上失真，
            // 後者是 ADM 詳情頁渲染時的注入面（政府原始資料已有未跳脫 HTML 的前例）。
            if (Character.isISOControl(cp) || cp == '<' || cp == '>') {
                return null;
            }
            i += Character.charCount(cp);
        }
        return normalized;
    }
}
