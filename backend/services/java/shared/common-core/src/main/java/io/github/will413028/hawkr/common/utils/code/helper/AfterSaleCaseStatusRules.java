package io.github.will413028.hawkr.common.utils.code.helper;

import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAppealState;

/**
 * §8.5 AfterSaleCaseStatus 業務狀態規則 helper（跨 aftersales + order 服務共用）。
 * <p>
 * CodeMeta enum 為跨語言公共型別，禁止包含業務輔助邏輯（docs/開發規範/Java 微服務開發規範.md §6）。
 * 本 helper 收斂「售後 case 是否活躍 / 是否終態覆蓋 / 是否覆蓋訂單生命週期」三組
 * 判定，作為 canonical SSOT（詳見 09 §2.1「售後 active / terminal canonical 判定」）：
 * <ul>
 *   <li>aftersales-service：{@code countActiveCasesByOrder}（倒數恢復反查 HasActiveCase）、
 *       {@code GetSellerCaseClassification}（賣家工作台 Dashboard 分類）</li>
 *   <li>order-service：賣家工作台 Dashboard 四桶 count（03 §2.1 SSOT）</li>
 * </ul>
 * 禁止各處硬編碼第二套 status set。此 helper 為純函式，不引入外部依賴。
 * <p>
 * ⚠️ AUTO_COMPLETED_STORE_PICKUP_TIMEOUT(31) / AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT(32)
 * 為「自動確認收貨」退款<b>中間態</b>（→ REFUND_PROCESSING(40) → CLOSED(50)），退款進行中，
 * 屬 <b>active</b>，<b>非</b>終態。終態僅 {33,50,70}（60 為可再操作態）。
 */
public final class AfterSaleCaseStatusRules {

    private AfterSaleCaseStatusRules() {
    }

    /**
     * 是否為活躍售後 case（賣家工作台「售後處理」queue / 訂單倒數恢復前 hold 判定）。
     * <p>
     * 定義：status NOT IN {33,50,60,70,71}
     *      OR (status=REJECTED(60) AND appeal_state=APPEALED)
     *      OR (status=AUTO_COMPLETED_RETURN_PENDING_TIMEOUT(33) AND appeal_state=APPEALED)
     * <p>
     * 31/32 不在排除集，屬 active（退款中間態）。
     * 71（CANCELLED_BY_CONFIRM_RECEIPT，#1225）比照 70：無條件終態（無申訴例外）。
     */
    public static boolean isActive(AfterSaleCaseStatus status, String appealState) {
        if (status == null) {
            return false;
        }
        boolean terminalOrRejected = status == AfterSaleCaseStatus.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT
                || status == AfterSaleCaseStatus.CLOSED
                || status == AfterSaleCaseStatus.REJECTED
                || status == AfterSaleCaseStatus.CANCELLED
                || status == AfterSaleCaseStatus.CANCELLED_BY_CONFIRM_RECEIPT;
        if (!terminalOrRejected) {
            return true;
        }
        // 60 / 33 申訴中視為活躍（Admin 裁決前案件凍結，訂單款項不可撥付）
        boolean appealed = isAppealed(appealState);
        if (appealed && (status == AfterSaleCaseStatus.REJECTED
                || status == AfterSaleCaseStatus.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT)) {
            return true;
        }
        return false;
    }

    /**
     * 撥款阻擋分類（backend#3262）。回答的問題是
     * <b>「這張 case 還會不會再生出退款」</b>——與 {@link #isActive} 的
     * 「賣家工作台要不要顯示 / 訂單倒數要不要 hold」<b>不是同一個問題</b>。
     *
     * <p>兩者恰好在 {@code {REJECTED(60), AUTO_COMPLETED_RETURN_PENDING_TIMEOUT(33)} ＋ 未申訴}
     * 上分歧：該狀態下 case 不活躍（倒數該恢復、訂單該完成），但買家仍在
     * <b>申訴窗</b>內 ⇒ 一旦申訴且 Admin 裁決退款，退款仍會成立
     * ⇒ 此時撥款出去就再也追不回來（全 repo 無賣家扣回機制）。
     *
     * <p><b>deny-by-default</b>：只有兩類放行，其餘（含未來新增的 status、
     * {@code appeal_state=INVESTIGATING(20)} 這種目前不寫入的值）一律 {@link PayoutBlock#INDEFINITE}。
     * 看不到就擋，不是看不到就放行。
     *
     * <p>⚠️ 本方法只做 <b>status × appeal_state</b> 的分類，<b>不碰時間</b>：
     * {@link PayoutBlock#UNTIL_APPEAL_DEADLINE} 的實際截止時點需要
     * {@code last_rejected_at} / {@code closed_at} 與 {@code appeal.deadline-hours}，
     * 那三個輸入只有 aftersales-service 有（見該服務的 {@code AppealDeadlineRules}）。
     * 這條切線是刻意的——common-core 不引入設定依賴。
     */
    public static PayoutBlock payoutBlock(AfterSaleCaseStatus status, String appealState) {
        if (status == null) {
            // 讀不出 status ＝ 看不到 ⇒ 擋（deny-by-default）
            return PayoutBlock.INDEFINITE;
        }
        // ① 確定結束：退款已完成／已發券結案（50）、買家取消（70）、確認收貨取消（71）
        //    50 只會由 REFUND_PROCESSING(40) 或 33/60＋APPEALED 的 Admin 裁決進入，都是終局。
        if (status == AfterSaleCaseStatus.CLOSED
                || status == AfterSaleCaseStatus.CANCELLED
                || status == AfterSaleCaseStatus.CANCELLED_BY_CONFIRM_RECEIPT) {
            return PayoutBlock.NONE;
        }
        // ② 尚可申訴：60 / 33 且從未申訴 ⇒ 擋到申訴截止為止（有時間界，不會永久卡死）
        if ((status == AfterSaleCaseStatus.REJECTED
                || status == AfterSaleCaseStatus.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT)
                && AfterSaleAppealState.NONE.value().equals(nvlAppealState(appealState))) {
            return PayoutBlock.UNTIL_APPEAL_DEADLINE;
        }
        // ③ 其餘：退款管線進行中（0/20/25/26/27/28/30/31/32/40）、
        //    或 60/33 已申訴（等 Admin 裁決）⇒ 無時間界，等業務終結
        return PayoutBlock.INDEFINITE;
    }

    /** {@link #payoutBlock} 的分類結果。 */
    public enum PayoutBlock {
        /** 不擋：本 case 已確定不會再生出退款。 */
        NONE,
        /** 擋到申訴截止（有時間界）：case 已駁回 / 自動關案，買家仍可申訴。 */
        UNTIL_APPEAL_DEADLINE,
        /** 擋且無時間界：退款管線進行中或申訴審理中，需業務終結才解除。 */
        INDEFINITE
    }

    /** {@code appeal_state} 為 null 視同 NONE（DB 預設 '0'，防歷史列為 null）。 */
    private static String nvlAppealState(String appealState) {
        return appealState == null ? AfterSaleAppealState.NONE.value() : appealState;
    }

    /**
     * 是否為「售後已結但需覆蓋訂單生命週期至已完成」的 case。
     * <p>
     * 定義：status IN {CLOSED(50), AUTO_COMPLETED_RETURN_PENDING_TIMEOUT(33)}
     *      且非申訴重啟（33 + APPEALED 仍屬 active，不在此集）。
     * <p>
     * 用於賣家工作台 Dashboard：訂單主狀態尚未達終態（如 coupon 補償結案 / 待寄回逾時關案，
     * order.status 仍停在 40）時，依售後已結把訂單歸入「已完成」。
     */
    public static boolean isTerminalOverrideToCompleted(AfterSaleCaseStatus status, String appealState) {
        if (status == null) {
            return false;
        }
        if (status == AfterSaleCaseStatus.CLOSED) {
            return true;
        }
        if (status == AfterSaleCaseStatus.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT) {
            return !isAppealed(appealState);
        }
        return false;
    }

    /**
     * 售後 case 是否覆蓋訂單生命週期分類。
     * <p>
     * 定義：status NOT IN {REJECTED(60), CANCELLED(70), CANCELLED_BY_CONFIRM_RECEIPT(71)}。
     * 60/70/71 售後未成立（駁回 / 買家取消 / 確認收貨取消），訂單回到自身 order.status 分類。
     */
    public static boolean overridesOrderLifecycle(AfterSaleCaseStatus status) {
        return status != null
                && status != AfterSaleCaseStatus.REJECTED
                && status != AfterSaleCaseStatus.CANCELLED
                && status != AfterSaleCaseStatus.CANCELLED_BY_CONFIRM_RECEIPT;
    }

    private static boolean isAppealed(String appealState) {
        return AfterSaleAppealState.APPEALED.value().equals(appealState);
    }
}
