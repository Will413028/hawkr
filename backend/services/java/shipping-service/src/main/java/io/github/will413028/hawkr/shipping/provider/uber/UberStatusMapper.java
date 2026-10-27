package io.github.will413028.hawkr.shipping.provider.uber;

import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;

import java.util.Map;
import java.util.Optional;
import java.util.Set;

/**
 * Uber DaaS 原生 {@code status} → 我方 normalized 值的映射（66 §7.3.1／§7.3.2）。
 *
 * <h2>值域是 7 值，不是 8</h2>
 * {@code pending}／{@code pickup}／{@code pickup_complete}／{@code dropoff}／
 * {@code delivered}／{@code canceled}／{@code returned}。
 * 官方文件列 8 個，第 8 個 {@code shopping_completed} 是 <b>Courier Pick &amp; Pack 專屬</b>，
 * 我方非代購情境不訂閱該模式 ⇒ 66 §7.3.1 把它列為<b>容忍值</b>
 * （counter ＋ 不寫入狀態 ＋ <b>不建 case</b>），它是我方沒訂閱的功能的正常值，不是異常。
 *
 * <h2>🔴 {@code courier_imminent} 不是狀態，是 boolean 欄位</h2>
 * 官方在外送員距取貨／送達點 <b>1 分鐘</b>時把它設為 true
 * （{@code docs/第三方整合/Uber Direct API 總覽.md} §7.1 逐字）。
 * 把它當第 8 個狀態會同時造成兩個錯：多一個永不出現的狀態，
 * 且<b>漏讀「即將抵達」</b>——那正是買家最需要的推播。
 * ⇒ 本類別<b>不</b>把它收進 {@link #FORWARD}，讀取它的是 {@link #courierImminent}。
 *
 * <h2>🔴 正向與逆向的 normalized 值相同，下游語意不同</h2>
 * 66 §7.3.2 兩張表的 normalized 欄完全一致（leg 是維度不是狀態，65 §7.1.2.0 ②），
 * 分歧發生在<b>售後案件軸</b>：同一個 {@code returned}，正向是「送不到、要退款給買家」，
 * 逆向是「退貨包裹送不回賣家」——<b>兩者的錢往完全相反的方向走</b>（66 §7.3.3 逐字）。
 * ⇒ {@link #returnLegCaseStatus} 是逆向 leg 的那一半，與 {@link #normalize} 分開。
 */
public final class UberStatusMapper {

    /** {@code shopping_completed}：Pick &amp; Pack 專屬，容忍值（counter ＋ 不寫入狀態 ＋ 不建 case）。 */
    public static final String TOLERATED_SHOPPING_COMPLETED = "shopping_completed";

    /**
     * 正向 leg（{@code is_return=false}）的映射表（66 §7.3.2 上表）。
     *
     * <p>🔴 {@code returned} 映到 {@code ABNORMAL(33)} 而不是任何「已送達」語意：
     * 它在正向 leg 是<b>失敗</b>（→33→41→退款）。名字容易誤讀成「已退貨完成」。
     */
    private static final Map<String, ShippingStatus> FORWARD = Map.of(
            "pending", ShippingStatus.UNPROCESSED,              // 91
            "pickup", ShippingStatus.COURIER_ASSIGNED,          // 933
            "pickup_complete", ShippingStatus.HANDED_OVER,      // 930
            "dropoff", ShippingStatus.IN_TRANSIT,               // 31
            "delivered", ShippingStatus.PICKED_UP,              // 11（T1 終態）
            "canceled", ShippingStatus.CANCELLED,               // 41（T1 終態）
            "returned", ShippingStatus.ABNORMAL);               // 33 →（同一次處理內續推 41，§7.3.4）

    /**
     * 逆向 leg（{@code is_return=true}）的售後案件語意（66 §7.3.2 下表）。
     *
     * <p>只列<b>有處置</b>的四格。沒列到的三格各有自己的理由，<b>不是漏的</b>：
     * <ul>
     *   <li>{@code pending} → 包裹還在買家手上，案件留 {@code RETURN_PENDING(20)}</li>
     *   <li>{@code pickup} → 🔴 <b>明確不處理（no-op 分支，不是 else）</b>（66 U6）：
     *       要「明確」才不會與「未識別」混在一起</li>
     *   <li>{@code canceled} → 🔴 <b>無對應</b>，66 open item {@code UP-8}；
     *       {@link #returnLegCaseStatus} 對它回 empty，呼叫端必須走「明確不處理 ＋ 建 case，不猜」</li>
     * </ul>
     *
     * <p>🔴 {@code pickup_complete → RETURN_SHIPPING(25)} 是 66 U5 的完整推導所在：
     * 不決定的代價是案件永遠停在 {@code RETURN_PENDING(20)}，出口是
     * {@code ship_back_deadline_at} 逾期 ⇒ {@code AUTO_COMPLETED_RETURN_PENDING_TIMEOUT(33)}
     * ——其 description 逐字「本狀態為『不退款』終態」
     * ⇒ <b>買家明明已交件卻被判定沒寄回而拿不到退款</b>。
     */
    private static final Map<String, AfterSaleCaseStatus> RETURN_LEG_CASE = Map.of(
            "pickup_complete", AfterSaleCaseStatus.RETURN_SHIPPING,   // 25（U5）
            "dropoff", AfterSaleCaseStatus.RETURN_SHIPPING,           // 25
            "delivered", AfterSaleCaseStatus.RETURN_DELIVERED,        // 27
            "returned", AfterSaleCaseStatus.RETURN_FAILED);           // 28

    /** 逆向 leg 上「明確不處理」的 raw 值——與「未識別」分開（66 U6）。 */
    private static final Set<String> RETURN_LEG_EXPLICIT_NO_OP = Set.of("pending", "pickup");

    private UberStatusMapper() {
    }

    /**
     * 原生值 → normalized {@link ShippingStatus}。
     *
     * @return 未知值與容忍值一律 {@link Optional#empty()}。
     *         🔴 呼叫端<b>必須</b>分辨這兩者：{@link #isTolerated} 為 true 時只計數、不建 case；
     *         為 false 才是「provider 回了一個我們不認得的值」⇒ raw 照存 ＋ counter ＋ 建 case。
     */
    public static Optional<ShippingStatus> normalize(String rawStatus) {
        if (rawStatus == null || rawStatus.isBlank()) {
            return Optional.empty();
        }
        return Optional.ofNullable(FORWARD.get(rawStatus));
    }

    /**
     * 這個 raw 值是不是「我方沒訂閱的功能的正常值」。
     *
     * <p>🔴 與「未知值」是兩件事：未知值要建 case（可能是 Uber 新增了狀態），
     * 容忍值建 case 只會產生永遠關不完的噪音。
     */
    public static boolean isTolerated(String rawStatus) {
        return TOLERATED_SHOPPING_COMPLETED.equals(rawStatus);
    }

    /** 本 mapper 承認的 7 個原生值（供契約測試斷言值域完整性）。 */
    public static Set<String> knownRawStatuses() {
        return FORWARD.keySet();
    }

    /**
     * 逆向 leg 的售後案件語意。
     *
     * @return empty 有兩種原因，呼叫端必須用 {@link #isReturnLegExplicitNoOp} 分辨：
     *         <b>明確不處理</b>（{@code pending}／{@code pickup}，U6）
     *         vs <b>無對應</b>（{@code canceled}，UP-8 ⇒ 建 case，不猜）
     */
    public static Optional<AfterSaleCaseStatus> returnLegCaseStatus(String rawStatus) {
        if (rawStatus == null) {
            return Optional.empty();
        }
        return Optional.ofNullable(RETURN_LEG_CASE.get(rawStatus));
    }

    /** 逆向 leg 上「明確不處理」（no-op 分支，不是 else）——66 U6。 */
    public static boolean isReturnLegExplicitNoOp(String rawStatus) {
        return RETURN_LEG_EXPLICIT_NO_OP.contains(rawStatus);
    }

    /**
     * 讀「即將抵達」旗標。
     *
     * <p>⚠️ 呼叫端要用 <b>edge</b>（false→true 那一次）而不是 level 觸發推播：
     * {@code courier_update} 每 20 秒一發，一趟 30 分約 90 則（66 §0.4），
     * level 觸發 ＝ 一趟 90 則推播（陷阱 B4）。
     */
    public static boolean courierImminent(Boolean rawFlag) {
        return Boolean.TRUE.equals(rawFlag);
    }
}
