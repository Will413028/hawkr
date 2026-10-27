package io.github.will413028.hawkr.productimport.service;

import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.code.ImportItemStatus;
import io.github.will413028.hawkr.common.utils.code.ImportJobStatus;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportJobEntity;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportJobMapper;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

class CommitJobServiceTest {

    private final ProductImportJobMapper jobMapper = mock(ProductImportJobMapper.class);
    private final ProductImportItemMapper itemMapper = mock(ProductImportItemMapper.class);
    private final ObsRabbitPublisher publisher = mock(ObsRabbitPublisher.class);
    private final CommitJobService service = new CommitJobService(jobMapper, itemMapper, publisher);

    @Test
    void commitJobReturnsAllSkippedBodyWhenNoItemIsEligible() {
        ProductImportJobEntity job = new ProductImportJobEntity();
        job.setId(10L);
        job.setStoreId(20L);
        job.setStatus(ImportJobStatus.READY.value());
        job.setTargetStatus("ACTIVE");
        when(jobMapper.selectById(10L)).thenReturn(job);

        ProductImportItemEntity blocked = new ProductImportItemEntity();
        blocked.setId(100L);
        blocked.setStatus(ImportItemStatus.READY.value());
        blocked.setBlockingErrorCount(2);
        when(itemMapper.selectList(any())).thenReturn(List.of(blocked));

        CommitJobService.CommitResult result = service.commitJob(20L, 10L, false, List.of());

        assertThat(result.status()).isEqualTo(ImportJobStatus.READY.value());
        assertThat(result.queuedCount()).isZero();
        assertThat(result.outcome()).isEqualTo(CommitJobService.CommitOutcome.ALL_SKIPPED);
        assertThat(result.skipped()).containsExactly(new CommitJobService.SkipEntry(
                100L,
                "BLOCKING_ERROR",
                "尚有未修正的錯誤（2 筆）"
        ));
        verify(jobMapper, never()).updateById(any());
        verifyNoInteractions(publisher);
    }
}
