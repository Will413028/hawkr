package io.github.will413028.hawkr.productimport.service;

import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.safefetch.SafeFetchRequest;
import io.github.will413028.hawkr.common.safefetch.SafeFetchResponse;
import io.github.will413028.hawkr.common.safefetch.SafeFetcher;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemEntity;
import io.github.will413028.hawkr.productimport.helper.HtmlToBlocksConverter;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemAssetMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemErrorMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemMapper;
import io.github.will413028.hawkr.productimport.publisher.ImportEvents.HtmlFetchEvent;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.TransactionStatus;
import org.springframework.transaction.support.SimpleTransactionStatus;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * 40 §2.5 / §5.3:HtmlFetchService 兩個 *Body method 必須走 TransactionTemplate,
 * mid-flight throw 必須 rollback。
 */
class HtmlFetchServiceTest {

    @BeforeAll
    static void initTsid() {
        try {
            TsidUtil.init(800, 0);
        } catch (Exception ignored) {
            // 已 init 過(其他 test 先跑了)— 忽略
        }
    }

    private final SafeFetcher safeFetcher = mock(SafeFetcher.class);
    private final HtmlToBlocksConverter htmlConverter = mock(HtmlToBlocksConverter.class);
    private final ProductImportItemMapper itemMapper = mock(ProductImportItemMapper.class);
    private final ProductImportItemAssetMapper assetMapper = mock(ProductImportItemAssetMapper.class);
    private final ProductImportItemErrorMapper errorMapper = mock(ProductImportItemErrorMapper.class);
    private final ObsRabbitPublisher publisher = mock(ObsRabbitPublisher.class);
    private final TestTxManager txManager = new TestTxManager();

    private final HtmlFetchService service = new HtmlFetchService(
            safeFetcher, htmlConverter, itemMapper, assetMapper, errorMapper, publisher, txManager);

    private HtmlFetchEvent buildEvent() {
        return new HtmlFetchEvent(100L, 10L, 20L, "https://example.com/desc.html", 1);
    }

    private ProductImportItemEntity buildItem() {
        ProductImportItemEntity item = new ProductImportItemEntity();
        item.setId(100L);
        item.setImportJobId(10L);
        item.setCommitRevision(1);
        return item;
    }

    @Test
    void recordFetchFailureBodyRunsInTxWhenSafeFetcherThrows() throws Exception {
        when(itemMapper.selectById(100L)).thenReturn(buildItem());
        when(safeFetcher.fetch(any(SafeFetchRequest.class)))
                .thenThrow(new RuntimeException("https not allowed"));

        service.fetchAndPersist(buildEvent());

        assertThat(txManager.getBeginCount())
                .as("recordFetchFailureBody 必須走 TransactionTemplate")
                .isEqualTo(1);
        assertThat(txManager.getCommitCount())
                .as("recordFetchFailure happy 必須 commit")
                .isEqualTo(1);
        assertThat(txManager.getRollbackCount()).isEqualTo(0);
    }

    @Test
    void persistDescriptionBodyMidFlightThrowFiresRollback() throws Exception {
        when(itemMapper.selectById(100L)).thenReturn(buildItem());
        when(safeFetcher.fetch(any(SafeFetchRequest.class)))
                .thenReturn(new SafeFetchResponse("<html>test</html>".getBytes(),
                        "text/html", 17L, "https://example.com/desc.html"));
        when(htmlConverter.convert(any(String.class)))
                .thenReturn(new HtmlToBlocksConverter.Result(
                        null, List.of(), List.of()));
        // item UPDATE 拋例外 → 觸發 rollback
        doThrow(new RuntimeException("simulated item UPDATE failure"))
                .when(itemMapper).updateById(any());

        try {
            service.fetchAndPersist(buildEvent());
        } catch (RuntimeException ignored) {
        }

        assertThat(txManager.getBeginCount())
                .as("persistDescriptionBody 必須走 TransactionTemplate")
                .isGreaterThanOrEqualTo(1);
        assertThat(txManager.getRollbackCount())
                .as("mid-flight throw 必須觸發 rollback")
                .isGreaterThanOrEqualTo(1);
    }

    @Test
    void persistDescriptionBodyHappyPathCommits() throws Exception {
        when(itemMapper.selectById(100L)).thenReturn(buildItem());
        when(safeFetcher.fetch(any(SafeFetchRequest.class)))
                .thenReturn(new SafeFetchResponse("<html>ok</html>".getBytes(),
                        "text/html", 15L, "https://example.com/desc.html"));
        when(htmlConverter.convert(any(String.class)))
                .thenReturn(new HtmlToBlocksConverter.Result(
                        null, List.of(), List.of()));

        service.fetchAndPersist(buildEvent());

        assertThat(txManager.getBeginCount()).isEqualTo(1);
        assertThat(txManager.getCommitCount()).isEqualTo(1);
        assertThat(txManager.getRollbackCount()).isEqualTo(0);
    }

    private static class TestTxManager implements PlatformTransactionManager {
        private int beginCount = 0;
        private int commitCount = 0;
        private int rollbackCount = 0;

        int getBeginCount() { return beginCount; }
        int getCommitCount() { return commitCount; }
        int getRollbackCount() { return rollbackCount; }

        @Override
        public TransactionStatus getTransaction(TransactionDefinition definition) {
            beginCount++;
            return new SimpleTransactionStatus(true);
        }

        @Override
        public void commit(TransactionStatus status) {
            commitCount++;
        }

        @Override
        public void rollback(TransactionStatus status) {
            rollbackCount++;
        }
    }
}
