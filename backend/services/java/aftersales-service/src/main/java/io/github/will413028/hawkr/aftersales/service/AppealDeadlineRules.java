package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;

import java.time.LocalDateTime;

/**
 * §10 申訴截止時點的唯一計算來源（純函式，無外部依賴）。
 *
 * <p>存在的理由是「同一條規則不要有第二份」：申訴截止同時被三個地方需要——
 * {@link AfterSaleAppealService#submitAppeal}（A3 守衛）、
 * {@code canAppeal} / detail assembler（前端顯示），以及
 * {@link AfterSalePayoutHoldService}（backend#3262 的撥款閘門）。
 * 三份各自演化的話，撥款閘門會與買家實際能申訴的時間錯開，而那正是本票要關的洞。
 *
 * <p>起算點依前置 status 分流（2026-06-15 起）：
 * <ul>
 *   <li>{@code REJECTED(60)} → {@code last_rejected_at + deadlineHours}</li>
 *   <li>{@code AUTO_COMPLETED_RETURN_PENDING_TIMEOUT(33)} → {@code closed_at + deadlineHours}</li>
 *   <li>其餘 status → {@code null}（不可申訴）</li>
 * </ul>
 *
 * <p>⚠️ 時間戳為 null 時回 {@code null} ＝「申訴不可能成立」。這對兩個消費端都是安全的：
 * {@code submitAppeal} 直接拒絕；撥款閘門視為無需 hold（買家本來就申訴不了）。
 * 兩支寫入 60 的 SQL（{@code markAutoRejected} / {@code markRejectedByResolution}）都
 * stamp {@code last_rejected_at}，寫入 33 的（{@code markReturnPendingTimeoutAutoClosed}）
 * stamp {@code closed_at} ⇒ 正常路徑不會走到 null 分支。
 */
public final class AppealDeadlineRules {

    private AppealDeadlineRules() {
    }

    /**
     * 計算申訴截止時點。
     *
     * @param caseEntity    售後案件
     * @param deadlineHours {@code aftersales.appeal.deadline-hours}
     * @return 截止時點；該 status 不可申訴或起算時間戳缺漏時回 {@code null}
     */
    public static LocalDateTime computeDeadline(AfterSaleCaseEntity caseEntity, int deadlineHours) {
        if (caseEntity == null) {
            return null;
        }
        String status = caseEntity.getStatus();
        if (AfterSaleCaseStatus.REJECTED.value().equals(status)) {
            return plusHoursOrNull(caseEntity.getLastRejectedAt(), deadlineHours);
        }
        if (AfterSaleCaseStatus.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT.value().equals(status)) {
            return plusHoursOrNull(caseEntity.getClosedAt(), deadlineHours);
        }
        return null;
    }

    private static LocalDateTime plusHoursOrNull(LocalDateTime base, int hours) {
        return base == null ? null : base.plusHours(hours);
    }
}
