package io.github.will413028.hawkr.order.repository;

import io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.time.LocalDateTime;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 03 §8.4：grant_payment_tasks 清理範圍 = DONE + PROVIDER_AUTO，
 * <b>排除 MANUAL_INTERVENTION</b>（撥款無 webhook 閉環，人工介入單是 Admin 異常清單
 * 與重發撥款的依據，未處理完不得清）。
 */
class GrantPaymentTaskRepositoryCleanupScopeTest {

    @Test
    @DisplayName("deleteCompletedBefore 只帶 DONE(20) + PROVIDER_AUTO(50)，不含 MANUAL_INTERVENTION(40)")
    void cleanupScope_doneAndProviderAutoOnly() {
        GrantPaymentTaskRepository repo = mock(GrantPaymentTaskRepository.class);
        LocalDateTime before = LocalDateTime.of(2026, 1, 1, 0, 0);
        when(repo.deleteCompletedBefore(before)).thenCallRealMethod();

        repo.deleteCompletedBefore(before);

        // 簽章僅兩個 status 參數，MANUAL_INTERVENTION 無處可入 → 結構性排除
        verify(repo).deleteCompletedBeforeByStatus(
                eq(before),
                eq(WorkerTaskStatus.DONE.value()),
                eq(WorkerTaskStatus.PROVIDER_AUTO.value()));
        verify(repo, org.mockito.Mockito.never()).deleteCompletedBeforeByStatus(
                any(), eq(WorkerTaskStatus.MANUAL_INTERVENTION.value()), any());
        verify(repo, org.mockito.Mockito.never()).deleteCompletedBeforeByStatus(
                any(), any(), eq(WorkerTaskStatus.MANUAL_INTERVENTION.value()));
    }
}
