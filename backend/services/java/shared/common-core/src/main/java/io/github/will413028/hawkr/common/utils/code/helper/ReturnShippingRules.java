package io.github.will413028.hawkr.common.utils.code.helper;

import io.github.will413028.hawkr.common.utils.code.ShippingStatus;

/**
 * 逆向 leg（退貨配送）的 canonical 分類器：把<b>已正規化的</b> {@link ShippingStatus}
 * 翻成售後案件要做的事。
 *
 * <h2>為什麼它住在 common-core 而不是 aftersales-service</h2>
 * 65 附錄 {@code ADR-013}：aftersales 原本用 5 個服務內私有的
 * {@code PAYUNI_RETURN_*} 字面值集合做同一件事，而那個前綴是<b>謊話</b>——
 * 它比對的是 {@code ShippingStatus.value()}，不是 PayUni wire code
 * （PayUni 的映射恰好恆等，所以謊話至今隱形）。
 * 🔴 <b>照那個名字施工，第二個 provider 進來時會加一套 {@code UBER_RETURN_*}，
 * 而 Uber 的原生值在 shipping-service 就被轉成 {@code 930}／{@code 933}，
 * 永遠到不了 aftersales</b> —— 那是架構上錯誤的修法。
 *
 * <p>本類與正向 leg 的 {@link ShippingStatusRules} <b>同層同宿</b>，入參型別是
 * {@code ShippingStatus} 枚舉而不是裸 {@code String}：裸字串讓「值域外」與
 * 「值域內但本 leg 沒意義」這兩件完全不同的事長得一模一樣。
 *
 * <h2>🔴 全函式（total），沒有 default 分支</h2>
 * {@link #dispositionOf} 是對 {@code ShippingStatus} 的 <b>exhaustive switch
 * expression</b>。新增一個 {@code ShippingStatus} 值而沒有在這裡指派處置
 * ⇒ <b>編譯失敗</b>，不是 runtime 靜默落 else。
 * 這正是 65 {@code :4987} 要求、而實作面一直缺席的那道一致性 gate——
 * 把它做成編譯期的，比任何測試都早一步。
 */
public final class ReturnShippingRules {

    private ReturnShippingRules() {
    }

    /**
     * 逆向 leg 收到某個貨態時，售後案件應該做什麼。
     *
     * <p>🔴 {@link #NO_OP} 與 {@link #REQUIRES_MANUAL_REVIEW} 是<b>兩件事</b>：
     * 前者是「我們知道它會發生、而且確定不必做事」，後者是「值域內但本 leg 沒有對應語意」。
     * 把後者寫成前者，等於把一個未解的設計缺口偽裝成已處理。
     */
    public enum Disposition {
        /** 買家已交件、退貨包裹運送中 ⇒ 案件推進 {@code RETURN_SHIPPING(25)}。 */
        RETURN_IN_TRANSIT,
        /** 退貨包裹配達門市待賣家領取 ⇒ 案件推進門市到達分支。 */
        RETURN_STORE_ARRIVED,
        /** 退貨包裹已送達賣家 ⇒ 案件推進 {@code RETURN_DELIVERED(27)}。 */
        RETURN_DELIVERED,
        /** 退貨配送異常 ⇒ 案件推進 {@code RETURN_FAILED(28)}。 */
        RETURN_ABNORMAL,
        /** 賣家逾期未領退貨包裹 ⇒ 案件走賣家未取分支。 */
        SELLER_NOT_PICKED,
        /**
         * 明確不處理：包裹還沒離開買家手上，或屬於中繼狀態。
         * 🔴 <b>是「明確的 no-op 分支」不是 else</b>——要分得出「我們決定不做事」
         * 與「我們沒認出這個值」。
         */
        NO_OP,
        /**
         * 值域內、但逆向 leg 沒有對應的售後語意 ⇒ <b>建案件交人工，不猜</b>。
         * 目前唯一成員是 {@code CANCELLED(41)}。
         */
        REQUIRES_MANUAL_REVIEW
    }

    /**
     * 逆向 leg 的處置分類。
     *
     * <h2>逐值的依據</h2>
     * <ul>
     *   <li>{@code 930 HANDED_OVER} → {@code RETURN_IN_TRANSIT}：Uber 逆向 {@code pickup_complete}
     *       ＝外送員已從買家手中取件 ＝ 買家已寄出。<b>不決定就是靜默丟棄</b>，而代價是案件永停
     *       {@code RETURN_PENDING(20)} ⇒ 7 天後自動關成「不退款」終態
     *       ⇒ 🔴 <b>買家已交件卻拿不到退款，零告警</b>。</li>
     *   <li>{@code 933 COURIER_ASSIGNED} → {@code NO_OP}：外送員已指派但尚未取件，包裹還在買家手上。</li>
     *   <li>{@code 31}／{@code 92}／{@code 11}／{@code 32}／{@code 33}／{@code 53}／{@code 44}／{@code 55}
     *       沿用既有 {@code PAYUNI_RETURN_*} 的分類，<b>行為不變</b>——本類是搬家與補洞，不是改語意。</li>
     *   <li>{@code 41 CANCELLED} → {@code REQUIRES_MANUAL_REVIEW}：無對應
     *       {@code AfterSaleCaseStatus}。<b>誠實標註，不假裝已處理。</b></li>
     * </ul>
     */
    public static Disposition dispositionOf(ShippingStatus status) {
        if (status == null) {
            throw new IllegalArgumentException("ReturnShippingRules.dispositionOf: status 不得為 null");
        }
        return switch (status) {
            // ── 退貨運送中 ──
            case IN_TRANSIT, PROCESSING, HANDED_OVER -> Disposition.RETURN_IN_TRANSIT;
            // ── 配達退貨門市 ──
            case WAITING_PICKUP -> Disposition.RETURN_STORE_ARRIVED;
            // ── 已送達賣家 ──
            case PICKED_UP -> Disposition.RETURN_DELIVERED;
            // ── 異常 ──
            case ABNORMAL, RETURN_TO_WAREHOUSE, PACKAGE_LOST -> Disposition.RETURN_ABNORMAL;
            // ── 賣家未取 ──
            case SELLER_NOT_PICKED -> Disposition.SELLER_NOT_PICKED;
            // ── 明確不處理（包裹尚未離開買家，或與逆向 leg 無關的正向／中繼狀態）──
            case UNPROCESSED, PENDING_SHIPMENT, LOGISTICS_CHECK,
                 COURIER_ASSIGNED, TCAT_CALLED, CVS_CODE_RECEIVED, SELLER_CONFIRMED,
                 CREATE_FAILED, CREATE_UNKNOWN, CANCEL_PENDING,
                 CLAIM_PENDING, COMPENSATION, PACKAGE_ABANDONED, RETURN_GENERAL,
                 BUYER_NOT_PICKED, RETURN_BY_HOME_DELIVERY, STORE_TRANSFER,
                 PENDING_HOME_RETURN -> Disposition.NO_OP;
            // ── 值域內但逆向 leg 無對應語意（UP-8）──
            case CANCELLED -> Disposition.REQUIRES_MANUAL_REVIEW;
        };
    }

}
