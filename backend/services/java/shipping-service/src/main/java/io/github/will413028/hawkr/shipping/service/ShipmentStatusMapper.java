package io.github.will413028.hawkr.shipping.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.common.utils.code.helper.ShippingStatusRules;
import org.springframework.stereotype.Component;

import java.util.Map;
import java.util.Set;

/**
 * provider 原生貨態 → 正規化配送狀態（{@link ShippingStatus}）的映射，以及狀態轉移的 allowed-from 判定。
 *
 * <h2>🔴 為什麼 PayUni 的映射是恆等的，卻仍然必須顯式寫出來</h2>
 * 既有 24 值中有 20 個就是 PayUni 的 wire code，所以 {@code normalized ≡ raw}。
 * 舊實作因此寫成一句 {@code ShippingStatus.fromValue(payUniCode)}——沒有轉換表。
 * 那有兩個後果：
 * <ul>
 *   <li>其他 provider 寫映射表時<b>沒有對稱的參照物</b>（別人有表、PayUni 沒有）</li>
 *   <li>PayUni 日後調整某個碼的語意，平台的正規語彙<b>跟著變且無處攔截</b></li>
 * </ul>
 * ⇒ 判準：<b>恆等映射與「沒有映射」在程式上長得一樣，差別是前者可以被測試指著問「這一格為什麼是它」。</b>
 *
 * <p>⚠️ 本表<b>只收 PayUni 的 20 個兩位數原生碼</b>。三位數是平台自訂值（叫件成功、取號成功、
 * 賣家確認、建單失敗、以及本波新增的七個），PayUni 永遠不會回傳它們；
 * 收進來會讓「provider 回了一個它不可能回的值」變成合法輸入。
 */
@Component
public class ShipmentStatusMapper {

    private static final ObsLogger log = ObsLogger.forClass(ShipmentStatusMapper.class);

    /**
     * PayUni wire code → normalized 值的顯式對照（恆等，但顯式）。
     *
     * <p>號碼慣例：<b>兩位數＝PayUni 原生值，三位數＝平台自訂值</b>。
     */
    private static final Map<String, ShippingStatus> PAYUNI_WIRE_TO_NORMALIZED = Map.ofEntries(
            Map.entry("11", ShippingStatus.PICKED_UP),
            Map.entry("21", ShippingStatus.PENDING_SHIPMENT),
            Map.entry("22", ShippingStatus.LOGISTICS_CHECK),
            Map.entry("31", ShippingStatus.IN_TRANSIT),
            Map.entry("32", ShippingStatus.WAITING_PICKUP),
            Map.entry("33", ShippingStatus.ABNORMAL),
            Map.entry("41", ShippingStatus.CANCELLED),
            Map.entry("43", ShippingStatus.COMPENSATION),
            Map.entry("44", ShippingStatus.PACKAGE_LOST),
            Map.entry("46", ShippingStatus.PACKAGE_ABANDONED),
            Map.entry("51", ShippingStatus.RETURN_GENERAL),
            Map.entry("52", ShippingStatus.BUYER_NOT_PICKED),
            Map.entry("53", ShippingStatus.RETURN_TO_WAREHOUSE),
            Map.entry("55", ShippingStatus.SELLER_NOT_PICKED),
            Map.entry("56", ShippingStatus.RETURN_BY_HOME_DELIVERY),
            Map.entry("81", ShippingStatus.STORE_TRANSFER),
            Map.entry("82", ShippingStatus.PENDING_HOME_RETURN),
            Map.entry("91", ShippingStatus.UNPROCESSED),
            Map.entry("92", ShippingStatus.PROCESSING),
            Map.entry("98", ShippingStatus.PROCESSING_ACCEPTED)
    );

    /**
     * 出貨前狀態（可互相轉移，PayUni 可能跳號推進）。
     *
     * <p>🔴 {@code PROCESSING_ACCEPTED(98)} <b>已移出本集合</b>：在 B2C 大宗寄倉語意下它是
     * 「我方已受理進倉」＝本地處理完成，不是「等 provider 出貨」。留在這裡會讓
     * {@code 920 → 98} 被判為非法 ⇒ B2C 批次流程在第一步就卡住。
     *
     * <p>{@code CREATE_UNKNOWN(991)} 加入本集合：建單結果未知時，回查排程查到答案後要能寫回
     * {@code 91}（成功）或 {@code 990}（失敗）。不加就是「回查永遠寫不進去」，
     * 而日誌只有一行 unknownTransition warn。
     */
    private static final Set<ShippingStatus> PRE_SHIPMENT = Set.of(
            ShippingStatus.UNPROCESSED,
            ShippingStatus.PROCESSING,
            ShippingStatus.PENDING_SHIPMENT,
            ShippingStatus.LOGISTICS_CHECK,
            ShippingStatus.CREATE_UNKNOWN
    );

    /**
     * 平台本地處理完成狀態。
     *
     * <p>這些狀態不是 provider 查詢會回傳的主貨態，代表平台已完成賣家側動作。
     * 主動查詢時 provider 可能仍回 21/92 等出貨前狀態；<b>不可因此覆蓋本地進度</b>。
     *
     * <p>本波加入：{@code PROCESSING_ACCEPTED(98)}（B2C 我方受理進倉）、
     * {@code COURIER_ASSIGNED(933)}（已指派外送員——不加會被 provider 的出貨前狀態倒退覆蓋）。
     */
    private static final Set<ShippingStatus> LOCAL_PROCESSING_DONE = Set.of(
            ShippingStatus.TCAT_CALLED,
            ShippingStatus.CVS_CODE_RECEIVED,
            ShippingStatus.SELLER_CONFIRMED,
            ShippingStatus.PROCESSING_ACCEPTED,
            ShippingStatus.COURIER_ASSIGNED
    );

    /**
     * 配送中狀態。
     *
     * <p>本波加入：{@code HANDED_OVER(930)}（已交運）、{@code DC_ACCEPTED(931)}（物流中心驗收通過）。
     * 不加的話 {@code 933 → 930}（Uber 取件完成）與 {@code 930 → 931}（B2C 驗收）都會被拒。
     */
    private static final Set<ShippingStatus> IN_DELIVERY = Set.of(
            ShippingStatus.IN_TRANSIT,
            ShippingStatus.WAITING_PICKUP,
            ShippingStatus.HANDED_OVER,
            ShippingStatus.DC_ACCEPTED
    );

    /** 終態（不可再轉移）。 */
    private static final Set<ShippingStatus> TERMINAL = Set.of(
            ShippingStatus.PICKED_UP,
            ShippingStatus.CANCELLED
    );

    /**
     * PayUni 原生貨態碼 → normalized 值。
     *
     * @return 對應的 {@link ShippingStatus}；未知／空值／sandbox 佔位符（如 {@code "-"}）皆回 {@code null}，
     *         由呼叫端走 unknown 分支。🔴 呼叫端<b>必須</b>：保存 raw 值、發告警、<b>不推進狀態機</b>；
     *         把 null 當成「沒有變化」而跳過，等於把 fail-closed 寫成 fail-open。
     */
    public ShippingStatus mapPayUniStatus(String payUniCode) {
        if (payUniCode == null || payUniCode.isBlank()) {
            return null;
        }
        ShippingStatus mapped = PAYUNI_WIRE_TO_NORMALIZED.get(payUniCode);
        if (mapped == null) {
            log.warn("ShipmentStatusMapper.unknownProviderStatus",
                    "PayUni 回報了不在映射表內的貨態碼，已 fail-closed 不推進狀態機",
                    "providerRawStatus", payUniCode);
        }
        return mapped;
    }

    /** 本 mapper 目前承認的 PayUni 原生碼（供契約測試斷言映射表的完整性）。 */
    public static Set<String> payUniWireCodes() {
        return PAYUNI_WIRE_TO_NORMALIZED.keySet();
    }

    /**
     * 驗證狀態轉移是否合法（allowed-from 判定）。
     *
     * <p>規則（嚴格版，禁止倒退到出貨前）：
     * <ul>
     *   <li>同狀態不可重複轉移</li>
     *   <li>終態（已取貨／已取消）不可再轉移</li>
     *   <li><b>取消申請（992）的入邊</b>：任何非終態都可以申請取消</li>
     *   <li><b>取消申請（992）的出邊</b>：全值域唯一允許回退的狀態</li>
     *   <li><b>建單失敗（990）的出邊</b>：只有 Admin 重建（91）與 Admin 取消（41）</li>
     *   <li>出貨前 → 任何狀態合法（流程可能跳號，例如直接 91→31）</li>
     *   <li>本地處理完成 → 配送中／終態／異常，＋ B2C 批次的三條顯式邊</li>
     *   <li>配送中 → 終態、異常、其他配送中；<b>禁止</b>倒退到出貨前</li>
     *   <li>異常 → 異常互轉、終態、重回配送中；<b>禁止</b>倒退到出貨前</li>
     * </ul>
     */
    public boolean isValidTransition(ShippingStatus from, ShippingStatus to) {
        if (from == to) {
            return false;
        }
        // 終態不可再轉移
        if (TERMINAL.contains(from)) {
            return false;
        }

        // ── 🔴 取消申請的專屬入邊（S0：不加這一段，配送中申請取消恆被拒） ──────────────
        //
        // 992 不屬於 TERMINAL／IN_DELIVERY／abnormal 任一集合，而下面三個分支的 to 判定
        // 一律是「TERMINAL || abnormal || IN_DELIVERY」⇒ isValidTransition(31, 992) 恆 false。
        //
        // 失敗症狀是完全靜默的：買家在配送中按取消 → 寫入被擋 → webhook 處理只留一行
        // invalidTransition warn 就 return ⇒ 單子停在原狀、前端顯示「取消已受理」、零告警。
        // ⚠️ 對照：990 沒有這個問題，因為它由建單失敗殼「直接 insert」不走本方法；992 沒有等價路徑。
        if (to == ShippingStatus.CANCEL_PENDING) {
            // ⚠️ 建單失敗（990）除外：它的出邊只有 {91 重建, 41 取消}（見下方分支）。
            //    990 根本沒有 provider 側的單子可取消，走 992「取消處理中」會卡在一個
            //    永遠等不到 provider 回覆的狀態。QA 的 L2 lane 實測到這條邊被允許，
            //    而同一支方法的 javadoc 說它不存在——先前是**順序**造成的：
            //    這個 blanket allow 排在 CREATE_FAILED 分支之前。
            return from != ShippingStatus.CREATE_FAILED;
        }

        // ── 🔴 取消申請的出邊：全值域唯一允許回退的狀態 ────────────────────────────
        //
        // provider 拒絕取消時貨照送，下一個實際貨態必須進得來；否則該筆會停在
        // 「取消處理中」直到人工介入。
        // ⚠️ 刻意**不做**「回到取消前的狀態」——那需要一個 cancel_from_status 快照欄，
        //    而 allowed-from 對不上就是 rows=0 靜默卡死。
        if (from == ShippingStatus.CANCEL_PENDING) {
            return true;
        }

        // ── 🔴 建單失敗的出邊（本波補上；在此之前 990 恆為非法轉移、只能人工改 DB） ────
        if (from == ShippingStatus.CREATE_FAILED) {
            return to == ShippingStatus.UNPROCESSED   // Admin 重建
                    || to == ShippingStatus.CANCELLED; // Admin 取消（＋觸發退款鏈）
        }

        // 出貨前 → 任何狀態合法（可能跳號推進）
        if (PRE_SHIPMENT.contains(from)) {
            return true;
        }

        // 平台本地處理完成 → 只能接收實際物流進展，禁止被出貨前狀態倒退覆蓋
        if (LOCAL_PROCESSING_DONE.contains(from)) {
            // B2C 大宗寄倉的三條顯式邊：920 → 98（我方受理進倉）、98 → 920（批次仍 OPEN 時退回取號）、
            // 98 → 930（交運）。前兩條都是 LOCAL_PROCESSING_DONE 內部轉移，
            // 不顯式列出就會落到下面的 return 而被拒。
            if (from == ShippingStatus.CVS_CODE_RECEIVED && to == ShippingStatus.PROCESSING_ACCEPTED) {
                return true;
            }
            if (from == ShippingStatus.PROCESSING_ACCEPTED && to == ShippingStatus.CVS_CODE_RECEIVED) {
                return true;
            }
            // 🔴 920 → 921「超商賣家確認出貨」（BLOCKER 3，2026-09-10 補）。
            //    這條邊 production **一直在走**（ShippingService.confirmCvsShipment 的 CAS 接受它），
            //    而本方法拒絕它。在只有 provider 回報路徑呼叫本方法時，那個不一致沒有症狀；
            //    Shared Foundation 讓 allowedActions 也開始問本方法之後，它變成
            //    「契約說 handover 不可用（9527）、端點照做並成功」——
            //    前端一旦依 改由 allowedActions 驅動，超商賣家將永遠無法確認出貨。
            //    ⚠️ 修的是**本表**不是端點：端點的行為是對的，錯的是這份判定漏了一條合法邊。
            if (from == ShippingStatus.CVS_CODE_RECEIVED && to == ShippingStatus.SELLER_CONFIRMED) {
                return true;
            }
            return TERMINAL.contains(to) || ShippingStatusRules.isAbnormal(to) || IN_DELIVERY.contains(to);
        }

        // 配送中 → 只能前進：終態、異常、其他配送中；禁止倒退到出貨前
        if (IN_DELIVERY.contains(from)) {
            return TERMINAL.contains(to) || ShippingStatusRules.isAbnormal(to) || IN_DELIVERY.contains(to);
        }

        // 異常 → 可互相轉移、可到終態、可重回配送中（門市關轉後重回配送）；禁止倒退到出貨前
        if (ShippingStatusRules.isAbnormal(from)) {
            return ShippingStatusRules.isAbnormal(to) || TERMINAL.contains(to) || IN_DELIVERY.contains(to);
        }

        log.warn("ShipmentStatusMapper.unknownTransition",
                "未預期的狀態轉移: " + from.label() + " → " + to.label());
        return false;
    }
}
