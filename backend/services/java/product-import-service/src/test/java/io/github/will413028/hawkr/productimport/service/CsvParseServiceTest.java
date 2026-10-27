package io.github.will413028.hawkr.productimport.service;

import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.code.ImportJobStatus;
import io.github.will413028.hawkr.productimport.entity.ProductImportJobEntity;
import io.github.will413028.hawkr.productimport.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.productimport.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.productimport.helper.CsvParser;
import io.github.will413028.hawkr.productimport.helper.HtmlToBlocksConverter;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemAssetMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemErrorMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemSkuMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportJobMapper;
import io.github.will413028.hawkr.productimport.storage.ProductImportStorage;
import org.junit.jupiter.api.Test;
import org.springframework.test.util.ReflectionTestUtils;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.TransactionStatus;
import org.springframework.transaction.support.SimpleTransactionStatus;
import org.springframework.transaction.support.TransactionTemplate;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * 40 §2.5 / §5.2:CsvParseService 兩個 TransactionTemplate 配置驗證:
 * <ul>
 *   <li>txTemplate(REQUIRED)— persistDraftViolations / finalizeJob / markJobParseFailed</li>
 *   <li>requiresNewTxTemplate(REQUIRES_NEW)— persistOneItem per-item 短 tx 隔離</li>
 * </ul>
 */
class CsvParseServiceTest {

    private final ProductImportJobMapper jobMapper = mock(ProductImportJobMapper.class);
    private final ProductImportItemMapper itemMapper = mock(ProductImportItemMapper.class);
    private final ProductImportItemSkuMapper skuMapper = mock(ProductImportItemSkuMapper.class);
    private final ProductImportItemAssetMapper assetMapper = mock(ProductImportItemAssetMapper.class);
    private final ProductImportItemErrorMapper errorMapper = mock(ProductImportItemErrorMapper.class);
    private final ProductImportStorage csvStorage = mock(ProductImportStorage.class);
    private final ProductGrpcClient productClient = mock(ProductGrpcClient.class);
    private final ShippingGrpcClient shippingClient = mock(ShippingGrpcClient.class);
    private final CsvParser csvParser = mock(CsvParser.class);
    private final HtmlToBlocksConverter htmlConverter = mock(HtmlToBlocksConverter.class);
    private final ObsRabbitPublisher publisher = mock(ObsRabbitPublisher.class);
    private final TestTxManager txManager = new TestTxManager();

    private final CsvParseService service = new CsvParseService(
            jobMapper, itemMapper, skuMapper, assetMapper, errorMapper,
            csvStorage, productClient, shippingClient, csvParser, htmlConverter,
            publisher, txManager);

    @Test
    void requiresNewTxTemplateHasRequiresNewPropagation() {
        TransactionTemplate requiresNew = (TransactionTemplate)
                ReflectionTestUtils.getField(service, "requiresNewTxTemplate");
        TransactionTemplate defaultTx = (TransactionTemplate)
                ReflectionTestUtils.getField(service, "txTemplate");

        assertThat(requiresNew).as("requiresNewTxTemplate field 必須存在").isNotNull();
        assertThat(requiresNew.getPropagationBehavior())
                .as("persistOneItemBody 必須走 REQUIRES_NEW(per-item 短 tx 隔離,§5.2)")
                .isEqualTo(TransactionDefinition.PROPAGATION_REQUIRES_NEW);

        assertThat(defaultTx).as("txTemplate field 必須存在").isNotNull();
        assertThat(defaultTx.getPropagationBehavior())
                .as("default txTemplate 應為 REQUIRED")
                .isEqualTo(TransactionDefinition.PROPAGATION_REQUIRED);
    }

    @Test
    void markJobParseFailedRunsInTxWhenCsvDownloadReturnsNull() throws Exception {
        ProductImportJobEntity job = new ProductImportJobEntity();
        job.setId(10L);
        job.setStatus(ImportJobStatus.PARSING.value());
        job.setSourceObjectKey("path/to/missing.csv");
        when(jobMapper.selectById(10L)).thenReturn(job);
        when(csvStorage.downloadCsv("path/to/missing.csv")).thenReturn(null);

        service.parseAndPersist(10L, 20L);

        assertThat(txManager.getBeginCount())
                .as("markJobParseFailedBody 必須走 TransactionTemplate")
                .isEqualTo(1);
        assertThat(txManager.getCommitCount())
                .as("happy markJobParseFailed 必須 commit")
                .isEqualTo(1);
        assertThat(txManager.getRollbackCount()).isEqualTo(0);
    }

    @Test
    void wrongJobStatusSkipsAllWork() throws Exception {
        ProductImportJobEntity job = new ProductImportJobEntity();
        job.setId(10L);
        // status != PARSING → skip
        job.setStatus(ImportJobStatus.READY.value());
        when(jobMapper.selectById(10L)).thenReturn(job);

        service.parseAndPersist(10L, 20L);

        assertThat(txManager.getBeginCount())
                .as("status not PARSING → 不應進入 tx")
                .isEqualTo(0);
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
