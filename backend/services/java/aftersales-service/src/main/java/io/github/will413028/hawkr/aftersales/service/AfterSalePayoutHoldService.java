package io.github.will413028.hawkr.aftersales.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import io.github.will413028.hawkr.common.utils.code.helper.AfterSaleCaseStatusRules;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

import java.time.LocalDateTime;
import java.util.List;

/**
 * 撥款資格閘門（backend#3262）：回答「這張訂單<b>還會不會再生出退款</b>」。
 *
 * <p><b>要關的洞</b>：售後 case 被駁回（60）或待寄回逾時自動關案（33）之後，
 * {@code AfterSaleCaseStatusRules.isActive} 回 false ⇒ 訂單倒數恢復 / 確認收貨不被擋
 * ⇒ 訂單完成、同事務建撥款任務、{@code GrantPaymentWorker} 30 秒內撥款。
 * 但買家在 {@code appeal.deadline-hours}（72h）內仍可申訴，Admin 裁決 FULL_REFUND 就會建退款
 * ——而全 repo 沒有任何賣家扣回機制 ⇒ 撥款出去就是平台資損。
 *
 * <p><b>本 service 不改變任何既有業務裁決</b>：申訴窗不動、確認收貨語意（#1225）不動、
 * 訂單完成時點不動。只在「撥款」這一個動作上加資格判定，而
 * 「還可能生出退款就不撥款」是既有政策（{@code isActive} javadoc 逐字：「訂單款項不可撥付」）。
 *
 * <p><b>deny-by-default</b>：狀態分類走 canonical 的
 * {@link AfterSaleCaseStatusRules#payoutBlock}，只有兩類放行，其餘一律擋。
 * status 值讀不出來（未來新增碼值 / 髒資料）也擋——看不到就擋，不是看不到就放行。
 *
 * <p><b>不會永久卡死</b>：{@code UNTIL_APPEAL_DEADLINE} 這一類帶時間界，
 * 到期自動放行（呼叫端據 {@code holdUntil} 把任務推遲到該時點）。
 * 這正是 issue #3262 comment #4 指出「C5③ 借用 {@code uk_aftersale_cases_active_order}
 * 的述詞會讓撥款永久卡死」的修正——dev 上停在 60/33、未申訴、窗已過的 32 筆就是那個形狀的樣本，
 * 在本實作下它們一律放行。
 *
 * <p><b>TOCTOU 怎麼關的（backend#3262 QA FAIL 後改版）</b>：
 * 殘留的競態是「{@code submitAppeal} 的 A3 檢查通過（{@code now <= deadline}）→ 交易 commit」
 * 這段期間——那筆申訴對本 service 不可見（MVCC），若剛好讀到「窗已過」就會誤放行。
 *
 * <p>舊版用 {@code grace}（15 分）的<b>長度</b>去證明它不會發生，前提是「該交易有已知的最壞耗時」。
 * 那個前提是<b>假的</b>：{@code evidenceService.finalizeAndInsert} 的 media gRPC 當時無 deadline、
 * 全 repo 無 {@code statement_timeout} / {@code socketTimeout} / {@code @Transactional(timeout)}、
 * gRPC server 端不會中止執行中的交易，加上 JVM STW 與 pod 排程本質不可界
 * ⇒ 用時間長度證明資金安全不成立（獨立 QA 以 Testcontainers 坐實：deadline 過了 28 小時，
 * {@code markAppealed} 仍回 {@code UPDATE 1}）。
 *
 * <p>現行版本改成<b>順序保證</b>：{@code submitAppeal} 在做任何資格判定<b>之前</b>先
 * {@code SELECT … FOR UPDATE} 鎖住該 case（{@code AfterSaleCaseMapper#selectByCaseNoForUpdate}），
 * 鎖持有到 commit/rollback；本 service 在「因窗已過而放行」之前先以
 * {@code FOR UPDATE SKIP LOCKED} 探測同一列（{@code tryLockCaseForPayoutRelease}），
 * 探不到就 fail-closed。於是對任一筆通過 A3 的申訴，撥款放行與該申訴落地<b>不可能同時成立</b>，
 * 而且這個結論<b>不依賴任何耗時假設</b>。
 *
 * <p><b>grace 現在的角色</b>：只剩<b>時鐘偏移預算</b>（deadline 由讀到 {@code last_rejected_at}
 * 的那個 pod 用自己的 {@code nowUtc()} 比較；提交申訴的 pod 與本查詢的 pod 可能不同）。
 * 它是額外的安全餘裕，<b>不再是唯一的 correctness mechanism</b>——把 grace 設成 0
 * 也不會讓「撥款後仍可成立退款」發生，只會少掉時鐘偏移的緩衝。
 *
 * <p>⚠️ 刻意<b>不</b>把 deadline 塞進 {@code markAppealed} 的 WHERE：那會在交易跨越截止時
 * 把買家的申訴打掉＝縮減既有權利。方向固定為「平台多等一下」，不是「買家少一點權利」。
 */
@Service
@RequiredArgsConstructor
public class AfterSalePayoutHoldService {

    private static final ObsLogger log = ObsLogger.forClass(AfterSalePayoutHoldService.class);

    private final AfterSaleCaseMapper caseMapper;
    private final AfterSalesProperties properties;

    /** hold 原因（觀測用；gRPC 層映射到 proto enum）。 */
    public enum HoldReason {
        /** 無 hold，可撥款。 */
        NONE,
        /** 售後案件進行中（含申訴審理中、退款處理中）——無時間界。 */
        ACTIVE_CASE,
        /** 案件已駁回 / 自動關案，但買家仍可申訴——有時間界。 */
        APPEAL_WINDOW_OPEN,
        /**
         * backend#3262 P0：申訴窗（含 grace）看似已過，但該 case 正被別的交易持鎖
         * ——可能是一筆已通過 A3、尚未 commit 的申訴。fail-closed，無時間界（推遲後重問）。
         */
        APPEAL_IN_FLIGHT
    }

    /**
     * @param hold      true = 仍可能生出退款，禁止撥款
     * @param holdUntil hold 且<b>有時間界</b>時帶（＝申訴截止 + grace）；無時間界與不 hold 一律 null
     * @param reason    觀測用
     * @param caseNo    造成 hold 的 case（觀測用；不 hold 時 null）
     */
    public record PayoutHold(boolean hold, LocalDateTime holdUntil, HoldReason reason, String caseNo) {

        static PayoutHold released() {
            return new PayoutHold(false, null, HoldReason.NONE, null);
        }
    }

    /**
     * 查該訂單的撥款 hold 狀態。
     *
     * <p>掃該訂單<b>全部</b> case（同一訂單歷史可有多筆終態 case）：
     * 任一張擋住就擋；{@code INDEFINITE} 支配 {@code UNTIL_APPEAL_DEADLINE}；
     * 多張都是時間界時取<b>最晚</b>的那個時點。
     */
    public PayoutHold getPayoutHold(String orderNo) {
        List<AfterSaleCaseEntity> cases = caseMapper.selectList(
                new LambdaQueryWrapper<AfterSaleCaseEntity>().eq(AfterSaleCaseEntity::getOrderNo, orderNo));
        if (cases == null || cases.isEmpty()) {
            // 絕大多數訂單走這條：沒有售後 case ⇒ 完全不受本閘門影響，撥款 cadence 不變
            return PayoutHold.released();
        }

        int deadlineHours = properties.getAppeal().getDeadlineHours();
        int graceMinutes = properties.getAppeal().getPayoutHoldGraceMinutes();
        LocalDateTime now = DateTimeUtils.nowUtc();

        LocalDateTime latestBoundedUntil = null;
        String latestBoundedCaseNo = null;

        for (AfterSaleCaseEntity c : cases) {
            AfterSaleCaseStatus status = AfterSaleCaseStatus.isValidValue(c.getStatus())
                    ? AfterSaleCaseStatus.fromValue(c.getStatus())
                    : null;   // 讀不出來 ⇒ payoutBlock 回 INDEFINITE（deny-by-default）

            switch (AfterSaleCaseStatusRules.payoutBlock(status, c.getAppealState())) {
                case NONE -> {
                    // 這張 case 確定不會再生出退款
                }
                case INDEFINITE -> {
                    // 無時間界者支配一切，可直接短路
                    log.info("AfterSalePayoutHoldService.hold.indefinite",
                            "撥款 hold（無時間界）status=" + c.getStatus()
                                    + ", appealState=" + c.getAppealState()
                                    + ", orderNo=" + orderNo,
                            "caseNo", c.getCaseNo());
                    return new PayoutHold(true, null, HoldReason.ACTIVE_CASE, c.getCaseNo());
                }
                case UNTIL_APPEAL_DEADLINE -> {
                    LocalDateTime deadline = AppealDeadlineRules.computeDeadline(c, deadlineHours);
                    if (deadline == null) {
                        // 起算時間戳缺漏 ⇒ submitAppeal 的 A3 也會拒絕 ⇒ 申訴不可能成立 ⇒ 不需 hold
                        continue;
                    }
                    LocalDateTime until = deadline.plusMinutes(graceMinutes);
                    if (!now.isBefore(until)) {
                        // 窗（含 grace）已過——但這是本 service 唯一會「放行」的出口，
                        // 放行前必須通過 backend#3262 P0 的兩道確認（修復本體）：
                        //   (a) 沒有一筆已通過 A3、尚未 commit 的申訴正壓在這一列上
                        //   (b) 上面那個 selectList 快照不是舊的（申訴可能在快照之後才 commit）
                        // 正確性來自 submitAppeal 的持鎖順序，不是來自 grace 的長度。
                        if (!confirmStillReleasableUnderLock(c, orderNo, deadlineHours, graceMinutes)) {
                            return new PayoutHold(true, null, HoldReason.APPEAL_IN_FLIGHT, c.getCaseNo());
                        }
                        continue;   // 窗已過、無人持鎖、且新鮮那一列仍可放行 ⇒ 買家再也不能申訴 ⇒ 放行
                    }
                    if (latestBoundedUntil == null || until.isAfter(latestBoundedUntil)) {
                        latestBoundedUntil = until;
                        latestBoundedCaseNo = c.getCaseNo();
                    }
                }
            }
        }

        if (latestBoundedUntil == null) {
            return PayoutHold.released();
        }
        log.info("AfterSalePayoutHoldService.hold.appealWindow",
                "撥款 hold（申訴窗未關）holdUntil=" + latestBoundedUntil + ", orderNo=" + orderNo,
                "caseNo", latestBoundedCaseNo);
        return new PayoutHold(true, latestBoundedUntil, HoldReason.APPEAL_WINDOW_OPEN, latestBoundedCaseNo);
    }

    /**
     * backend#3262 P0：放行前的最後確認——<b>持鎖重讀</b>。
     *
     * <p>兩件事一次做完，缺一都會留下缺口：
     * <ol>
     *   <li><b>可鎖性</b>：{@code FOR UPDATE SKIP LOCKED} 取不到 ⇒ 有交易正壓在這一列上。
     *       {@code submitAppeal} 的第一個動作就是持鎖（在任何資格判定之前，見
     *       {@link io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper#selectByCaseNoForUpdate}）
     *       ⇒ 任何可能通過 A3 的申訴，在它落地之前都會讓這裡取不到鎖 ⇒ fail-closed。</li>
     *   <li><b>新鮮度</b>：取得鎖之後用<b>回傳的那一列</b>重跑分類，不沿用 {@code selectList}
     *       的快照。少了這一步會漏掉這個交錯：閘門讀到 {@code appeal_state='0'}（當時申訴還持鎖）
     *       → 申訴 commit、鎖釋放 → 探測「鎖得到」→ 放行，而申訴其實已經成立。</li>
     * </ol>
     *
     * <p>⚠️ 探測必須<b>非阻塞</b>（{@code SKIP LOCKED} 而非等待）：本方法跑在
     * {@code GrantPaymentWorker} 的租約窗口內（經 5 秒 gRPC deadline），等待會把持鎖者的耗時
     * 灌進租約下界的算式（《多副本排程搶佔規範》§三第 5 步）。
     *
     * <p>⚠️ 鎖在本語句結束即釋放（本 service 非 {@code @Transactional}，autocommit）。
     * 釋放後仍然安全：之後才想落地的申訴必須先取得鎖<b>再</b>過 A3，而此刻已過 deadline
     * ⇒ A3 必然拒絕。這正是「順序保證」與「時間長度保證」的差別。
     *
     * @return true = 確認可以放行；false = fail-closed，呼叫端必須回無時間界 hold
     */
    private boolean confirmStillReleasableUnderLock(AfterSaleCaseEntity candidate, String orderNo,
                                                   int deadlineHours, int graceMinutes) {
        AfterSaleCaseEntity fresh = caseMapper.tryLockCaseForPayoutRelease(candidate.getCaseNo());
        if (fresh == null) {
            log.warn("AfterSalePayoutHoldService.hold.appealInFlight",
                    "申訴窗看似已過但該 case 正被持鎖（可能有在途申訴），fail-closed 不放行，"
                            + "orderNo=" + orderNo,
                    "caseNo", candidate.getCaseNo());
            return false;
        }

        AfterSaleCaseStatus freshStatus = AfterSaleCaseStatus.isValidValue(fresh.getStatus())
                ? AfterSaleCaseStatus.fromValue(fresh.getStatus())
                : null;   // 讀不出來 ⇒ payoutBlock 回 INDEFINITE（deny-by-default）
        boolean releasable = switch (AfterSaleCaseStatusRules.payoutBlock(
                freshStatus, fresh.getAppealState())) {
            case NONE -> true;
            case INDEFINITE -> false;
            case UNTIL_APPEAL_DEADLINE -> {
                LocalDateTime freshDeadline = AppealDeadlineRules.computeDeadline(fresh, deadlineHours);
                // 起算時間戳缺漏 ⇒ A3 也會拒絕 ⇒ 申訴不可能成立 ⇒ 可放行（與主迴圈同語意）
                yield freshDeadline == null
                        || !DateTimeUtils.nowUtc().isBefore(freshDeadline.plusMinutes(graceMinutes));
            }
        };

        if (!releasable) {
            log.warn("AfterSalePayoutHoldService.hold.staleSnapshot",
                    "放行前持鎖重讀發現該 case 已變更（快照 appealState=" + candidate.getAppealState()
                            + "/status=" + candidate.getStatus()
                            + " → 實際 appealState=" + fresh.getAppealState()
                            + "/status=" + fresh.getStatus() + "），fail-closed 不放行，"
                            + "orderNo=" + orderNo,
                    "caseNo", candidate.getCaseNo());
        }
        return releasable;
    }
}
