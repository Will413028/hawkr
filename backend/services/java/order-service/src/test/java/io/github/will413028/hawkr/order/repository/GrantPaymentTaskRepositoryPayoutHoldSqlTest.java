package io.github.will413028.hawkr.order.repository;

import io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus;
import org.apache.ibatis.annotations.Update;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.time.LocalDateTime;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;

/**
 * backend#3262：{@code deferForPayoutHold} 的 SQL 形狀與參數注入。
 *
 * <p>讀正式碼的 {@code @Update} 字面值，不抄一份 SQL 進測試——抄過來的那份會與正式碼各自演化
 * （同 {@link MapperSql} 的存在理由）。
 */
class GrantPaymentTaskRepositoryPayoutHoldSqlTest {

    private static String deferSql() {
        try {
            return MapperSql.flatten(GrantPaymentTaskRepository.class
                    .getMethod("deferForPayoutHoldByStatus", long.class, LocalDateTime.class,
                            String.class, String.class, String.class)
                    .getAnnotation(Update.class).value());
        } catch (NoSuchMethodException e) {
            throw new AssertionError("mapper 方法簽章已變更", e);
        }
    }

    @Test
    @DisplayName("CAS 必須帶 status = PROCESSING：只有持有租約者能推遲（防遲到者覆蓋 admin 重發 / 新持有者）")
    void hasProcessingCas() {
        assertThat(deferSql())
                .containsIgnoringCase("WHERE task_id = #{taskId}")
                .containsIgnoringCase("AND status = #{processing}");
    }

    @Test
    @DisplayName("推的是 next_retry_time（候選述詞認的就是它）＋ 落 last_error 供運維辨識為 hold 而非全新任務")
    void pushesNextRetryTimeAndRecordsReason() {
        assertThat(deferSql())
                .containsIgnoringCase("next_retry_time = #{nextEligibleAt}")
                .containsIgnoringCase("last_error = #{reason}");
    }

    @Test
    @DisplayName("刻意不寫 retry_count：hold 是資格未到，不是失敗，不得消耗重試額度")
    void doesNotTouchRetryCount() {
        assertThat(deferSql())
                .as("推遲若動了 retry_count，一筆申訴審理數天的訂單會被重問幾次就推進 MANUAL_INTERVENTION")
                .doesNotContainIgnoringCase("retry_count");
    }

    @Test
    @DisplayName("SQL 內不寫字面狀態碼（§6.1 禁止硬編碼，狀態值一律由 WorkerTaskStatus 注入）")
    void noLiteralStatusCodes() {
        assertThat(deferSql())
                .doesNotContain("'0'")
                .doesNotContain("'10'")
                .doesNotContain("'30'");
    }

    @Test
    @DisplayName("default method 注入的狀態值：目標 PENDING、前置 PROCESSING（順序寫反會讓 CAS 永遠不命中）")
    void defaultMethodInjectsCorrectStatuses() {
        GrantPaymentTaskRepository repo = mock(GrantPaymentTaskRepository.class,
                org.mockito.Mockito.CALLS_REAL_METHODS);
        LocalDateTime at = LocalDateTime.of(2026, 9, 9, 0, 0);

        repo.deferForPayoutHold(1L, at, "reason");

        ArgumentCaptor<String> pending = ArgumentCaptor.forClass(String.class);
        ArgumentCaptor<String> processing = ArgumentCaptor.forClass(String.class);
        verify(repo).deferForPayoutHoldByStatus(anyLong(), any(), anyString(),
                pending.capture(), processing.capture());
        assertThat(pending.getValue()).isEqualTo(WorkerTaskStatus.PENDING.value());
        assertThat(processing.getValue()).isEqualTo(WorkerTaskStatus.PROCESSING.value());
    }

    @Test
    @DisplayName("P1-b：admin 人工重發不得叫醒被 hold 的 PENDING 任務（本斷言在 CI 跑得到，IT 跑不到）")
    void adminResetMustNotWakeHeldPendingTasks() {
        String sql;
        try {
            sql = MapperSql.flatten(GrantPaymentTaskRepository.class
                    .getMethod("resetForRetryByStatus", String.class, String.class,
                            String.class, String.class, String.class)
                    .getAnnotation(Update.class).value());
        } catch (NoSuchMethodException e) {
            throw new AssertionError("mapper 方法簽章已變更", e);
        }

        java.util.regex.Matcher m = java.util.regex.Pattern
                .compile("status\\s+IN\\s*\\(([^)]*)\\)", java.util.regex.Pattern.CASE_INSENSITIVE)
                .matcher(sql);
        assertThat(m.find()).as("救濟 SQL 必須有 status IN (...) 的 allowed-from 集合").isTrue();
        String allowedFrom = m.group(1);

        assertThat(allowedFrom)
                .as("allowed-from 必須恰為 FAILED / MANUAL_INTERVENTION / PROCESSING")
                .contains("#{failed}").contains("#{manual}").contains("#{processing}");
        assertThat(allowedFrom)
                .as("""
                        PENDING 不得進 allowed-from：被撥款閘門 hold 的任務就停在 PENDING ＋ 未來的 \
                        next_retry_time，把它納入會讓 admin「重發」清掉推遲、繞過 backend#3262 的閘門。\
                        （閘門在執行點仍會重問，所以不是直接資損；但那條防線不該靠下游兜底。）\
                        獨立 QA 的 M7 變異就是把它加進來——當時只有 CI 不跑的 IT 轉紅，本斷言補上 CI 可見的那一半。""")
                .doesNotContain("#{pending}");
    }

    @Test
    @DisplayName("候選述詞把時間算進去：推遲之所以有效，靠的是 next_retry_time <= NOW() 這一條")
    void candidatePredicateHonoursNextRetryTime() {
        String sql = MapperSql.select(GrantPaymentTaskRepository.class, "findPendingTasksByStatus",
                int.class, String.class, String.class, String.class);
        assertThat(sql)
                .as("這一條若被拿掉，deferForPayoutHold 寫再對也擋不住撥款")
                .containsIgnoringCase("next_retry_time IS NULL OR next_retry_time <= NOW()");
    }
}
