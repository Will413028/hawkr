package io.github.will413028.hawkr.payment.logistics.domain;

import io.github.will413028.hawkr.common.utils.code.FulfillmentCaseResolutionType;
import io.github.will413028.hawkr.common.utils.code.FulfillmentExceptionCategory;
import io.github.will413028.hawkr.common.utils.code.ResponsibilityCause;
import java.util.List;
import java.util.Set;

/**
 * 責任判定案件的 lifecycle 規則（65 §5.5.2 E0／E0.1／E0.2）。
 *
 * <p>🔴 <b>碼值本身住在 CodeMeta</b>（{@link FulfillmentExceptionCategory}／
 * {@link FulfillmentCaseResolutionType}／{@link ResponsibilityCause}）。本類別只放規則——
 * CodeMeta enum 不得承載業務輔助邏輯（10 檔 §8.5），與 {@link ResponsibilityRules} 同一個分工。
 *
 * <p>🔴 <b>本類別的每一條都在 DB 有對應的機械保證</b>，兩者不互相取代：
 * <table border="1">
 *   <caption>應用層規則 ↔ DB 約束對照</caption>
 *   <tr><th>規則</th><th>DB 側</th><th>為什麼兩道都要</th></tr>
 *   <tr><td>{@link #isResponsibilityCategory}／{@link #assertResponsibilityType}</td>
 *       <td>{@code chk_fcr_responsibility_type_binding} ＋ {@code fk_fcr_case_category}</td>
 *       <td>DB 那道丟 23514／23503 ⇒ 呼叫端拿到 500；本類別讓它變成可回業務碼的正常拒絕</td></tr>
 *   <tr><td>{@link #isFinalizableCause}</td><td>{@code chk_exc_responsibility_resolved_cause}</td>
 *       <td>同上</td></tr>
 *   <tr><td>{@link #FORBIDS_DISMISS}</td><td>{@code chk_exc_responsibility_no_dismiss}</td>
 *       <td>DB 是最後防線；應用層是唯一能解釋「為什麼不行」的地方</td></tr>
 * </table>
 *
 * <p>⚠️ <b>不得</b>因為 DB 已經守住就把這一層拿掉：DB 的拒絕沒有業務語意，而責任判定的拒絕
 * 需要讓 ADM 操作者知道發生了什麼。反過來也<b>不得</b>因為這一層存在就不加 DB 約束——
 * 應用層的 if 會在下一次重構被抽掉而沒有人發現（65 §6.4.2 硬約束 1 逐字）。
 */
public final class ResponsibilityCaseRules {

    private ResponsibilityCaseRules() {
    }

    /** 責任判定案件的 category（V12 的索引述詞與兩道 CHECK 都內嵌這個值）。 */
    public static final String CATEGORY = FulfillmentExceptionCategory.RESPONSIBILITY_UNRESOLVED.value();

    /** 唯一能結掉責任判定案件的處置型別。 */
    public static final String RESOLUTION_TYPE = FulfillmentCaseResolutionType.RESPONSIBILITY_ATTRIBUTE.value();

    /**
     * 責任判定案件的 subject 軸：一個 provider 配送 leg。
     *
     * <p>🔴 <b>不是 order、不是 shipment PK</b>（65 §5.5.2 D）：同一張訂單可以有正逆向兩條 leg，
     * 各自的責任判定互不相干；把 subject 定成 order 會讓第二條 leg 的案件建不出來，
     * 而 lifetime UNIQUE 使它<b>完全靜默</b>。
     */
    public static final String SUBJECT_TYPE = "SHIPMENT";

    /**
     * {@code source_service} 的合法值域。
     *
     * <p>🔴 <b>它表達的是「這個例外狀況的<u>來源服務</u>」，不是「哪個服務擁有這張表」。</b>
     * 兩張表由 payment-service 擁有、`INSERT` 也由它執行——但那<b>不代表</b>
     * {@code source_service} 就是 {@code payment}。責任判定的觸發來源可能是：
     * <ul>
     *   <li>{@code shipping} —— provider 終局處置格判不出成因（66 §7.3.4）</li>
     *   <li>{@code order} —— 訂單軸偵測到的履約異常</li>
     *   <li>{@code payment} —— 對帳／金流側發現的異常（既有 {@code LogisticsReconciliationService} 走這一格）</li>
     * </ul>
     * ⇒ 由呼叫端依<b>真正的 trigger</b> 決定並傳入，本類別只負責擋掉不合法的值。
     *
     * <p>🔴 <b>Trust boundary（WP2B 之後仍然成立）</b>：這個值是 <b>backend internal metadata</b>，
     * <b>不是</b> Admin 使用者的輸入。WP2B 若開出 Admin 建案路徑，
     * <b>不得</b>把 raw {@code source_service} 交給 ADM client 自行指定——
     * 它必須由 server-side workflow 依觸發來源決定。一旦可由前端指定，
     * 案件的來源歸屬就變成「誰都能宣稱」，而稽核時分不出是哪條路徑建的。
     *
     * <p>⚠️ 本集合是 {@code chk_exc_source} 的複製（schema 事實）⇒ 漂移由
     * {@code LogisticsSchemaContractTest} 逐字對拍 V9 守住。
     */
    public static final Set<String> SOURCE_SERVICES = Set.of("shipping", "order", "payment");

    /** 案件狀態。刻意不建第二份碼表——這四個值的 SSOT 是 {@code chk_exc_status}。 */
    public static final String STATUS_OPEN = "OPEN";
    public static final String STATUS_IN_PROGRESS = "IN_PROGRESS";
    public static final String STATUS_RESOLVED = "RESOLVED";
    public static final String STATUS_DISMISSED = "DISMISSED";

    /** 提案狀態。SSOT 是 {@code chk_fcr_status}。 */
    public static final String RESOLUTION_AWAITING_REVIEW = "AWAITING_REVIEW";
    public static final String RESOLUTION_APPROVED = "APPROVED";
    public static final String RESOLUTION_REJECTED = "REJECTED";

    /**
     * 提案與定案的 allowed-from（兩者相同）。
     *
     * <p>🔴 <b>不含 {@code RESOLVED}</b>：finalization commit 之後，後到的提案拿到鎖時案件已是
     * {@code RESOLVED} ⇒ 直接被擋下。這就是「不可能再成功建立會改 entitlement 的提案」的機械形式。
     * <p>🔴 <b>不含 {@code DISMISSED}</b>：責任判定案件根本到不了那一格（{@link #FORBIDS_DISMISS}）。
     */
    public static final List<String> ALLOWED_FROM = List.of(STATUS_OPEN, STATUS_IN_PROGRESS);

    /**
     * 🔴 責任判定案件<b>不得</b>進入 {@code DISMISSED}。
     *
     * <p>責任判定沒有「不處理」這個結局——它決定的是買家拿不拿得到退款，而 Order 正卡在
     * {@code SHIPPING_ABNORMAL(35)} 等它。一旦 dismiss：不能再提案（allowed-from 不含它）、
     * 不能另開第二張（lifetime UNIQUE）⇒ 永遠到不了 {@code APPROVED} ＋ {@code RESOLVED}
     * ⇒ 永遠不發 N16 ⇒ <b>Order 永遠停在 35，買家的錢無限期卡住</b>，而且每一步都「照規則成功」。
     *
     * <p>⚠️ 這是一個<b>今天無處可掛</b>的守衛：全 repo 目前沒有任何 dismiss 寫入路徑。
     * 🔴 <b>日後新增任何 dismiss 入口，必須在同一個變更裡呼叫 {@link #assertDismissAllowed}</b>
     * ——否則這道禁令會在新增入口的那一天靜默失效（DB 的 {@code chk_exc_responsibility_no_dismiss}
     * 仍會擋，但呼叫端拿到的會是 500 而不是一個說得清楚的業務錯誤）。
     */
    public static final boolean FORBIDS_DISMISS = true;

    /**
     * 可結案的責任成因：四個 resolved 值。
     *
     * <p>🔴 <b>{@code RESPONSIBILITY_UNRESOLVED} 不在其中</b>——它是未結案期間的 category，
     * 不是可結案的 {@code resolution_code}。若讓它結案，finality 三條件會<b>形式上全部成立</b>
     * 而 N16 仍發不出去（payload 值域只有 4 個 resolved 值）⇒ Order 一樣卡在 35，
     * 而且 ADM 佇列那側看不到它（案件已 RESOLVED，離開了 {@code idx_exc_aging} 的觀察窗）。
     * ⇒ 判不出來時的正確處置是讓案件留在 {@code IN_PROGRESS} 繼續補證據。
     *
     * <p>🔴 <b>{@code SYSTEM_CAUSED} 不存在</b>：平台／系統原因一律歸 {@code PLATFORM_CAUSED}
     * （36 §3.3 逐字）。不得為了「看起來完整」而新增它。
     *
     * <p>⚠️ 本集合<b>從 CodeMeta 推導</b>而不是硬寫四個字串：{@link ResponsibilityCause} 日後
     * 新增第五值（66 §1.6.1 已登記 {@code RESPONSIBILITY_UNRESOLVED}）時，這裡會自動把它排除，
     * 而 V12 的 CHECK 是硬寫的 ⇒ 兩者的差異由 {@code LogisticsSchemaContractTest} 逐字對拍抓出來。
     */
    public static final Set<String> FINALIZABLE_CAUSES = Set.of(
            ResponsibilityCause.BUYER_CAUSED.value(),
            ResponsibilityCause.SELLER_CAUSED.value(),
            ResponsibilityCause.PROVIDER_CAUSED.value(),
            ResponsibilityCause.PLATFORM_CAUSED.value());

    /** {@code dedup_key} 的格式由 V9 註解 ＋ {@code LogisticsReconciliationService} 定死，不得自創。 */
    public static String dedupKey(String subjectRef) {
        return CATEGORY + ":" + SUBJECT_TYPE + ":" + subjectRef;
    }

    public static boolean isResponsibilityCategory(String category) {
        return CATEGORY.equals(category);
    }

    public static boolean isResponsibilityType(String resolutionType) {
        return RESOLUTION_TYPE.equals(resolutionType);
    }

    /** 這個成因可否用來結案（＝可否出現在 N16 的 {@code responsibility_cause}）。 */
    public static boolean isFinalizableCause(String cause) {
        return cause != null && FINALIZABLE_CAUSES.contains(cause);
    }

    /** 案件是否仍可提案／定案。 */
    public static boolean isActionable(String caseStatus) {
        return ALLOWED_FROM.contains(caseStatus);
    }

    /**
     * 🔴 fail-closed：<b>判不出 category 或型別就拒絕</b>，不是「不是責任判定才拒絕」。
     *
     * <p>雙向：責任案件只收責任型別，責任型別只能用在責任案件。單向會讓其中一個方向靜默通過，
     * 而兩個方向的後果都是「finality 永遠不成立、Order 永遠停在 35」。
     */
    public static void assertResponsibilityType(String caseCategory, String resolutionType) {
        if (caseCategory == null || resolutionType == null) {
            throw new IllegalArgumentException(
                    "responsibility: category 與 resolutionType 皆不得為 null（fail-closed）"
                            + " category=" + caseCategory + " resolutionType=" + resolutionType);
        }
        if (isResponsibilityCategory(caseCategory) != isResponsibilityType(resolutionType)) {
            throw new IllegalArgumentException(
                    "responsibility: resolutionType 與案件 category 不相容——"
                            + "責任判定案件只接受 " + RESOLUTION_TYPE + "，且該型別不得用於其他 category"
                            + "（category=" + caseCategory + " resolutionType=" + resolutionType + "）");
        }
    }

    /**
     * <b>提案</b>成因守門（V13）：maker 提出責任判定時就必須主張一個成因，且值域同結案碼。
     *
     * <p>🔴 <b>為什麼成因要在提案時就定下來</b>：它是雙人制的標的物本身。
     * 成因若等到 approve 才第一次出現，{@code chk_fcr_two_person} 保證的就只是
     * 「有兩個人碰過這張單」，而不是「第二個人批准了第一個人提的那一件事」——
     * 覆核者可以獨自決定 {@code BUYER_CAUSED}（＝買家拿不到退款），而帳面上雙人制完全成立。
     *
     * <p>🔴 <b>值域與 {@link #FINALIZABLE_CAUSES} 相同不是巧合</b>：approve 會把這個值原封不動
     * 寫進 {@code fulfillment_exception_cases.resolution_code}。兩邊值域若不同，
     * 就會存在「提得出來但結不了案」的成因 ⇒ 案件永久卡在 IN_PROGRESS。
     *
     * <p>⚠️ 判不出成因時的正確處置是<b>不要提案</b>，讓案件留在 {@code IN_PROGRESS} 繼續補證據
     * ——不是提一個 {@code RESPONSIBILITY_UNRESOLVED} 佔位。
     */
    public static void assertProposableCause(String caseCategory, String cause) {
        if (!isResponsibilityCategory(caseCategory)) {
            // 非責任案件不得夾帶成因（與 chk_fcr_proposed_cause_binding 的反向同義）。
            if (cause != null) {
                throw new IllegalArgumentException(
                        "responsibility: 非責任判定案件的提案不得帶 proposedResponsibilityCause"
                                + "（category=" + caseCategory + " cause=" + cause + "）——"
                                + "帶了它會讓稽核看到一個「有責任主張」的一般處置");
            }
            return;
        }
        if (!isFinalizableCause(cause)) {
            throw new IllegalArgumentException(
                    "responsibility: 提案必須主張一個成因，且必須是四個 resolved 值之一 "
                            + FINALIZABLE_CAUSES + "（收到 " + cause + "）；"
                            + "判不出成因時的正確處置是不要提案，讓案件留在 IN_PROGRESS 繼續補證據");
        }
    }

    /**
     * N16 事件 context 守門（WP2A-R2 SIDE_TASK）。
     *
     * <p>🔴 36 §3.3.1 把 {@code order_no} 與 {@code shipping_method} 標為<b>不可 null</b>，
     * 而在此之前實作端<b>一行檢查都沒有</b>——傳 null 會產出一筆違約 payload 且完全靜默。
     * {@code order_no} 正是 order-service 把訂單從 {@code 35} 推到 {@code 54} 的 CAS 錨點
     * ⇒ 它為 null 時那道 CAS 永遠 rows=0，而 Order 停在 35，沒有任何錯誤訊息。
     *
     * <p>⚠️ 這道守衛<b>不能</b>取代真正的修法（把這些值在建案交易內持久化到 case row，
     * 由 finalize 從已鎖住的那一列讀回）。它擋的是「null 靜默出境」，
     * 擋不住「呼叫端傳了一個對的格式但錯的值」——後者需要 server 端持有事實，
     * 屬 WP2B 前置架構缺口（本輪報告 §14）。
     */
    public static void assertEventContext(String orderNo, String shippingMethod) {
        if (orderNo == null || orderNo.isBlank()) {
            throw new IllegalArgumentException(
                    "responsibility: orderNo 不得為 null 或空白（36 §3.3.1 標示不可 null）——"
                            + "它是 order-service 35→54 CAS 的錨，為空會讓那道 CAS 永遠 rows=0 而無訊號");
        }
        if (shippingMethod == null || shippingMethod.isBlank()) {
            throw new IllegalArgumentException(
                    "responsibility: shippingMethod 不得為 null 或空白（36 §3.3.1 標示不可 null）");
        }
    }

    /** 結案碼守門：責任判定案件只能以四個 resolved 成因結案。 */
    public static void assertFinalizableCause(String caseCategory, String cause) {
        if (isResponsibilityCategory(caseCategory) && !isFinalizableCause(cause)) {
            throw new IllegalArgumentException(
                    "responsibility: 結案成因必須是四個 resolved 值之一 " + FINALIZABLE_CAUSES
                            + "（收到 " + cause + "）；判不出成因時應讓案件留在 IN_PROGRESS 繼續補證據，"
                            + "不得結案成 " + CATEGORY);
        }
    }

    /**
     * {@code source_service} 守門：fail-closed，<b>在 INSERT 之前</b>拒絕。
     *
     * <p>🔴 <b>為什麼不等 DB 的 {@code chk_exc_source}</b>：那道 CHECK 會丟 23514，呼叫端拿到的是
     * <b>500</b>——一個「你傳了不合法的來源服務」的問題會被讀成「系統壞了」。
     * 更糟的是在交易中拋出後 PostgreSQL 會把<b>整個交易 abort</b>（25P02），
     * 而建案交易裡還有 outbox 那一列。⇒ 在碰 DB 之前擋下來。
     *
     * <p>⚠️ 兩道方向一致、不互相取代：DB 那道是最後防線（擋住繞過本類別的寫入路徑），
     * 本方法讓它變成可回業務碼的正常拒絕。與 {@code chk_fcr_two_person} ＋ mapper 述詞的先例同形。
     */
    public static void assertSourceService(String sourceService) {
        if (sourceService == null || sourceService.isBlank()) {
            throw new IllegalArgumentException(
                    "responsibility: sourceService 不得為 null 或空白（fail-closed）——"
                            + "它表達「這個例外狀況的來源服務」，合法值 " + SOURCE_SERVICES);
        }
        if (!SOURCE_SERVICES.contains(sourceService)) {
            throw new IllegalArgumentException(
                    "responsibility: sourceService 不在合法值域 " + SOURCE_SERVICES
                            + "（收到 '" + sourceService + "'）。🔴 它是「例外狀況的來源服務」，"
                            + "不是「哪個服務擁有這張表」——兩張表由 payment-service 擁有，"
                            + "但來源可能是 shipping／order");
        }
    }

    /**
     * dismiss 守門。<b>日後新增任何 dismiss 入口時必須呼叫它</b>（見 {@link #FORBIDS_DISMISS}）。
     */
    public static void assertDismissAllowed(String caseCategory) {
        if (caseCategory == null || isResponsibilityCategory(caseCategory)) {
            throw new IllegalStateException(
                    "responsibility: 責任判定案件不得 DISMISSED（category=" + caseCategory + "）"
                            + "——Order 正卡在 SHIPPING_ABNORMAL(35) 等這個判定，"
                            + "dismiss 會讓它永遠到不了終態");
        }
    }
}
