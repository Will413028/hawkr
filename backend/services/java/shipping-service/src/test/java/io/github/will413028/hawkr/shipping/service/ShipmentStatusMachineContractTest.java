package io.github.will413028.hawkr.shipping.service;

import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.Set;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * 正規化配送狀態機的契約測試（65 物流履約平台 Shared Foundation）。
 *
 * <p>與既有 {@code ShipmentStatusMapperTest} 的分工：那一支驗的是「舊規則沒被改壞」，
 * 本支驗的是「本波新增的邊真的走得通」——兩者都要留。
 *
 * <p>🔴 <b>本支測試的價值在於它證明五條 provider 鏈「可達」</b>。
 * 狀態機的失效模式不是拋例外，而是 {@code isValidTransition} 回 false 之後
 * 呼叫端只留一行 warn 就 return ⇒ 單子停在原狀、前端顯示已受理、零告警。
 * 逐條走過整條鏈是唯一能在合併前抓到它的方法。
 */
@DisplayName("正規化配送狀態機契約")
class ShipmentStatusMachineContractTest {

    private final ShipmentStatusMapper mapper = new ShipmentStatusMapper();

    /** 逐段走完一條鏈，任一段被拒即失敗並指出是哪一段。 */
    private void assertChainReachable(String chainName, List<ShippingStatus> chain) {
        for (int i = 0; i < chain.size() - 1; i++) {
            ShippingStatus from = chain.get(i);
            ShippingStatus to = chain.get(i + 1);
            assertTrue(mapper.isValidTransition(from, to),
                    () -> chainName + " 斷在 " + from.value() + "(" + from.label() + ")"
                            + " → " + to.value() + "(" + to.label() + ")"
                            + "：這條邊被 isValidTransition 拒絕，實際症狀會是「單子停在原狀、前端顯示已受理、零告警」");
        }
    }

    @Nested
    @DisplayName("S0：取消申請（992）的入邊")
    class CancelPendingInbound {

        /**
         * 🔴 本波修復的 S0。修復前 {@code isValidTransition(31, 992)} 恆 false：
         * 992 不屬於 TERMINAL／IN_DELIVERY／abnormal 任一集合，而配送中分支的 to 判定
         * 只認那三個集合。買家在配送中按取消 ⇒ 寫入被擋 ⇒ 完全靜默。
         */
        @Test
        @DisplayName("配送中可以申請取消（修復前恆 false）")
        void inTransitCanRequestCancel() {
            assertTrue(mapper.isValidTransition(ShippingStatus.IN_TRANSIT, ShippingStatus.CANCEL_PENDING));
        }

        @Test
        @DisplayName("任何非終態都可以申請取消")
        void everyNonTerminalCanRequestCancel() {
            for (ShippingStatus from : ShippingStatus.values()) {
                if (from == ShippingStatus.CANCEL_PENDING) {
                    continue; // from == to 由另一條規則擋
                }
                boolean terminal = from == ShippingStatus.PICKED_UP || from == ShippingStatus.CANCELLED;
                // 🔴 990 建單失敗是第三個例外（2026-09-10 補）：它的出邊只有 {91 重建, 41 取消}，
                //    provider 側根本沒有單子可取消 ⇒ 走 992「取消處理中」會停在一個
                //    永遠等不到 provider 回覆的狀態。本方法的 javadoc 一直是這樣寫的，
                //    先前允許它只是因為 CANCEL_PENDING 的 blanket allow 排在 CREATE_FAILED 分支之前。
                boolean createFailed = from == ShippingStatus.CREATE_FAILED;
                assertEquals(!terminal && !createFailed,
                        mapper.isValidTransition(from, ShippingStatus.CANCEL_PENDING),
                        () -> "從 " + from.value() + "(" + from.label() + ") 申請取消的判定不符預期");
            }
        }

        @Test
        @DisplayName("終態不可申請取消")
        void terminalCannotRequestCancel() {
            assertFalse(mapper.isValidTransition(ShippingStatus.PICKED_UP, ShippingStatus.CANCEL_PENDING));
            assertFalse(mapper.isValidTransition(ShippingStatus.CANCELLED, ShippingStatus.CANCEL_PENDING));
        }
    }

    @Nested
    @DisplayName("取消申請（992）的出邊：全值域唯一允許回退的狀態")
    class CancelPendingOutbound {

        @Test
        @DisplayName("provider 確認取消 → 41")
        void confirmedCancel() {
            assertTrue(mapper.isValidTransition(ShippingStatus.CANCEL_PENDING, ShippingStatus.CANCELLED));
        }

        @Test
        @DisplayName("provider 拒絕取消後，貨照送——下一個實際貨態必須進得來")
        void rejectedCancelLetsDeliveryContinue() {
            assertTrue(mapper.isValidTransition(ShippingStatus.CANCEL_PENDING, ShippingStatus.IN_TRANSIT));
            assertTrue(mapper.isValidTransition(ShippingStatus.CANCEL_PENDING, ShippingStatus.WAITING_PICKUP));
            assertTrue(mapper.isValidTransition(ShippingStatus.CANCEL_PENDING, ShippingStatus.PICKED_UP));
            assertTrue(mapper.isValidTransition(ShippingStatus.CANCEL_PENDING, ShippingStatus.ABNORMAL));
        }
    }

    @Nested
    @DisplayName("建單失敗（990）的出邊：本波之前它是死狀態")
    class CreateFailedOutbound {

        @Test
        @DisplayName("Admin 重建 → 91；Admin 取消 → 41")
        void adminExits() {
            assertTrue(mapper.isValidTransition(ShippingStatus.CREATE_FAILED, ShippingStatus.UNPROCESSED));
            assertTrue(mapper.isValidTransition(ShippingStatus.CREATE_FAILED, ShippingStatus.CANCELLED));
        }

        @Test
        @DisplayName("其餘出邊仍然關閉——990 不是一般狀態，只有兩個明確出口")
        void noOtherExits() {
            assertFalse(mapper.isValidTransition(ShippingStatus.CREATE_FAILED, ShippingStatus.IN_TRANSIT));
            assertFalse(mapper.isValidTransition(ShippingStatus.CREATE_FAILED, ShippingStatus.PICKED_UP));
        }
    }

    @Nested
    @DisplayName("建單結果未知（991）：回查排程要寫得回去")
    class CreateUnknown {

        @Test
        @DisplayName("查到成功 → 91；查到失敗 → 990；超出回查窗 → 41")
        void requeryOutcomes() {
            assertTrue(mapper.isValidTransition(ShippingStatus.CREATE_UNKNOWN, ShippingStatus.UNPROCESSED));
            assertTrue(mapper.isValidTransition(ShippingStatus.CREATE_UNKNOWN, ShippingStatus.CREATE_FAILED));
            assertTrue(mapper.isValidTransition(ShippingStatus.CREATE_UNKNOWN, ShippingStatus.CANCELLED));
        }
    }

    @Nested
    @DisplayName("五條 provider 鏈可達性")
    class ProviderChains {

        @Test
        @DisplayName("3001 黑貓宅配：91 → 210 → 31 → 11")
        void taqbinChain() {
            assertChainReachable("3001 黑貓", List.of(
                    ShippingStatus.UNPROCESSED,
                    ShippingStatus.TCAT_CALLED,
                    ShippingStatus.IN_TRANSIT,
                    ShippingStatus.PICKED_UP));
        }

        /**
         * 🔴 <b>2026-09-10 改判：{@code 920 → 921} 是合法邊，本鏈是連續的。</b>
         *
         * <p>本測試原本斷言這條邊<b>不可達</b>，理由是「它由我方端點的 CAS 直接寫入，
         * 不經 isValidTransition，本方法的 production 呼叫點只有兩個、都在 provider 回報路徑」。
         * <b>那個理由在 Shared Foundation 落地那一刻就失效了</b>：{@code AllowedActionResolver}
         * 成為第三、第四個呼叫點，而它正是用這條邊判定「賣家能不能按確認出貨」。
         *
         * <p>後果是 QA 在 dev 實測到的矛盾：契約回 {@code handover enabled=false（9527）}，
         * 而同一時刻 {@code POST /shipments/&#123;no&#125;/confirm} 回 200 並把 920 推到 921。
         * 前端一旦依 改由 {@code allowedActions} 驅動，<b>超商賣家將永遠無法確認出貨</b>。
         *
         * <p>⚠️ <b>不是「放寬 LOCAL_PROCESSING_DONE 的內部轉移」</b>——舊註解擔心的是
         * provider 的出貨前狀態倒退覆蓋本地進度，而這裡加的是一條<b>具名的單向邊</b>
         * （{@code CVS_CODE_RECEIVED → SELLER_CONFIRMED}），反向仍然被拒。
         * 防倒退的保證由 {@link #localProgressCannotBeOverwrittenByPreShipment} 繼續守著。
         */
        @Test
        @DisplayName("3002 超商 C2C：取號前段 ＋ 賣家確認後的配送段（中間那條邊不走本方法）")
        void cvsC2cChain() {
            // 整條鏈連續可達：建單 → 上游確認 → 取號 → **賣家確認出貨** → provider 回報的每一步
            assertChainReachable("3002 超商 C2C 全鏈", List.of(
                    ShippingStatus.UNPROCESSED,
                    ShippingStatus.PROCESSING,
                    ShippingStatus.CVS_CODE_RECEIVED,
                    ShippingStatus.SELLER_CONFIRMED,
                    ShippingStatus.IN_TRANSIT,
                    ShippingStatus.WAITING_PICKUP,
                    ShippingStatus.PICKED_UP));

            // 🔴 反向仍被拒：加的是一條**具名單向邊**，不是把 LOCAL_PROCESSING_DONE 整組打開。
            //    這一條同時是本測試的鑑別力來源——若有人真的去放寬那個集合，這裡會轉紅。
            assertFalse(mapper.isValidTransition(
                            ShippingStatus.SELLER_CONFIRMED, ShippingStatus.CVS_CODE_RECEIVED),
                    "921 → 920 是倒退，必須維持被拒；若這裡變成 true，"
                            + "表示有人放寬了 LOCAL_PROCESSING_DONE 的內部轉移，"
                            + "而那會讓 provider 的出貨前狀態可以倒退覆蓋本地進度");
        }

        @Test
        @DisplayName("3002 逾期未取分支：32 → 52 → 53 → 55 → 82 → 56")
        void cvsC2cUnclaimedChain() {
            assertChainReachable("3002 逾期未取", List.of(
                    ShippingStatus.WAITING_PICKUP,
                    ShippingStatus.BUYER_NOT_PICKED,
                    ShippingStatus.RETURN_TO_WAREHOUSE,
                    ShippingStatus.SELLER_NOT_PICKED,
                    ShippingStatus.PENDING_HOME_RETURN,
                    ShippingStatus.RETURN_BY_HOME_DELIVERY));
        }

        @Test
        @DisplayName("3003 Uber 即時：91 → 933 → 930 → 31 → 11")
        void uberChain() {
            assertChainReachable("3003 Uber", List.of(
                    ShippingStatus.UNPROCESSED,
                    ShippingStatus.COURIER_ASSIGNED,
                    ShippingStatus.HANDED_OVER,
                    ShippingStatus.IN_TRANSIT,
                    ShippingStatus.PICKED_UP));
        }

        @Test
        @DisplayName("3005 商家自送：91 → 930 → 31 → 11")
        void merchantDeliveryChain() {
            assertChainReachable("3005 商家自送", List.of(
                    ShippingStatus.UNPROCESSED,
                    ShippingStatus.HANDED_OVER,
                    ShippingStatus.IN_TRANSIT,
                    ShippingStatus.PICKED_UP));
        }

        @Test
        @DisplayName("理賠鏈：44 → 993 → 43")
        void claimChain() {
            assertChainReachable("理賠", List.of(
                    ShippingStatus.PACKAGE_LOST,
                    ShippingStatus.CLAIM_PENDING,
                    ShippingStatus.COMPENSATION));
        }
    }

    @Nested
    @DisplayName("M-1：本地處理完成狀態不被倒退覆蓋")
    class BatchExplicitEdges {

        @Test
        @DisplayName("933 不得被出貨前狀態倒退覆蓋")
        void courierAssignedIsNotOverwritten() {
            assertFalse(mapper.isValidTransition(ShippingStatus.COURIER_ASSIGNED, ShippingStatus.PROCESSING));
            assertFalse(mapper.isValidTransition(ShippingStatus.COURIER_ASSIGNED, ShippingStatus.PENDING_SHIPMENT));
        }
    }

    @Nested
    @DisplayName("PayUni 顯式映射表（恆等，但必須存在）")
    class PayUniMapping {

        @Test
        @DisplayName("19 個原生兩位數碼全部映射到自己")
        void identityMappingIsExplicitAndComplete() {
            Set<String> codes = ShipmentStatusMapper.payUniWireCodes();
            assertEquals(19, codes.size(),
                    "PayUni 原生碼應為 19 個（正規化值域中的兩位數）；數量變了表示有人動了映射表");
            for (String code : codes) {
                ShippingStatus mapped = mapper.mapPayUniStatus(code);
                assertNotNull(mapped, () -> "PayUni 碼 " + code + " 映射不出結果");
                assertEquals(code, mapped.value(),
                        () -> "PayUni 碼 " + code + " 的映射不是恆等——若這是刻意的，請在映射表旁寫下理由");
            }
        }

        @Test
        @DisplayName("三位數是平台自訂值，PayUni 永遠不會回傳 ⇒ 不在映射表內")
        void platformOwnCodesAreNotProviderCodes() {
            for (String platformCode : List.of("210", "920", "921", "930", "933", "990", "991", "992", "993")) {
                assertNull(mapper.mapPayUniStatus(platformCode),
                        () -> "平台自訂碼 " + platformCode + " 不該被當成 PayUni 原生碼接受——"
                                + "收進來會讓「provider 回了一個它不可能回的值」變成合法輸入");
            }
        }

        @Test
        @DisplayName("未知 provider 碼 fail-closed 回 null（不猜、不預設放行）")
        void unknownProviderCodeFailsClosed() {
            assertNull(mapper.mapPayUniStatus("77"));
            assertNull(mapper.mapPayUniStatus("-"));
            assertNull(mapper.mapPayUniStatus(""));
            assertNull(mapper.mapPayUniStatus(null));
        }
    }

    @Nested
    @DisplayName("本方法的守備範圍（避免下一個人誤以為它守住了所有寫入路徑）")
    class TransitionGuardScope {

        /**
         * 🔴 <b>2026-09-10 改判（BLOCKER 3）：本方法現在守<u>全部</u>寫入路徑，不只 provider 回報。</b>
         *
         * <p>本測試原本斷言的是相反的事，其 javadoc 逐字寫著
         * 「{@code isValidTransition} 的 production 呼叫點<b>只有兩個</b>，且都在 provider 回報路徑」，
         * 並據此推論「我方端點寫入的邊被拒 → 不影響，因為它根本不問這裡」。
         * <b>那個前提已經是假的</b>：Shared Foundation 讓 {@code AllowedActionResolver}
         * 成為第三、第四個呼叫點（契約側），而賣家按下去的五支 write path 一支都不問它
         * ——於是同一個動作在契約與執行面得到相反的答案。
         *
         * <p>本波把 {@code ShippingService.confirmCvsShipment} 與 {@code callTcat} 接上
         * {@code AllowedActionResolver.require(...)}，兩側從此共用同一份判定。
         * ⇒ 「某條邊被本方法拒絕」現在只有一種意思：<b>那個動作真的做不了</b>。
         *
         * <p>⚠️ 舊 javadoc 還寫著「真正釘住呼叫點數量的是 CI 的靜態檢查」——
         * 實查 {@code scripts/} 與 {@code .github/workflows/} 對 {@code isValidTransition} 零命中，
         * <b>那個靜態檢查不存在</b>。真正釘住讀寫共用的是
         * {@code CanonicalActionRuleParityTest}（本波新增，走行為而非字串）。
         */
        @Test
        @DisplayName("本方法守全部寫入路徑：920→921 現在是合法邊，且反向仍被拒")
        void isValidTransitionGuardsEveryWritePath() {
            // 920 → 921：我方端點真的在走這條邊 ⇒ 判定必須承認它
            assertTrue(mapper.isValidTransition(
                    ShippingStatus.CVS_CODE_RECEIVED, ShippingStatus.SELLER_CONFIRMED));

            // 反向是倒退，仍必須被拒（鑑別力：證明上一行不是「整組放行」造成的）
            assertFalse(mapper.isValidTransition(
                    ShippingStatus.SELLER_CONFIRMED, ShippingStatus.CVS_CODE_RECEIVED));

            // provider 回報路徑上的同一段（921 之後的貨態）必須進得來
            assertTrue(mapper.isValidTransition(
                    ShippingStatus.SELLER_CONFIRMED, ShippingStatus.IN_TRANSIT));
        }

        /**
         * ⚠️ {@code 992} 的<b>入邊</b>在本波尚無 production 呼叫端——取消申請端點屬後續波次。
         * 先備好的理由：它是 provider 回報路徑上唯一「從配送中往回走」的邊，
         * 而取消端點落地時若沿用統一的 transition 判定，缺這一段就會靜默卡住。
         * 依「每個新狀態的四問」，這一格的「誰把它推進來」目前的答案是<b>尚無</b>，本註解即為登記。
         */
        @Test
        @DisplayName("992 入邊已備好，但本波尚無推進者（登記在案）")
        void cancelPendingInboundHasNoWriterYet() {
            assertTrue(mapper.isValidTransition(ShippingStatus.IN_TRANSIT, ShippingStatus.CANCEL_PENDING));
        }
    }

    @Nested
    @DisplayName("既有規則沒被改壞（回歸護欄）")
    class LegacyGuards {

        @Test
        @DisplayName("終態不可再轉移")
        void terminalIsTerminal() {
            assertFalse(mapper.isValidTransition(ShippingStatus.PICKED_UP, ShippingStatus.IN_TRANSIT));
            assertFalse(mapper.isValidTransition(ShippingStatus.CANCELLED, ShippingStatus.IN_TRANSIT));
        }

        @Test
        @DisplayName("同狀態不重複轉移")
        void sameStatusRejected() {
            assertFalse(mapper.isValidTransition(ShippingStatus.IN_TRANSIT, ShippingStatus.IN_TRANSIT));
        }

        @Test
        @DisplayName("本地處理完成不得被出貨前狀態倒退覆蓋（既有防倒退保證）")
        void localProgressNotOverwritten() {
            assertFalse(mapper.isValidTransition(ShippingStatus.TCAT_CALLED, ShippingStatus.PENDING_SHIPMENT));
            assertFalse(mapper.isValidTransition(ShippingStatus.CVS_CODE_RECEIVED, ShippingStatus.PROCESSING));
            assertFalse(mapper.isValidTransition(ShippingStatus.SELLER_CONFIRMED, ShippingStatus.UNPROCESSED));
        }

        @Test
        @DisplayName("配送中不得倒退到出貨前")
        void inDeliveryNoRegression() {
            assertFalse(mapper.isValidTransition(ShippingStatus.IN_TRANSIT, ShippingStatus.UNPROCESSED));
            assertFalse(mapper.isValidTransition(ShippingStatus.WAITING_PICKUP, ShippingStatus.PENDING_SHIPMENT));
        }
    }
}
