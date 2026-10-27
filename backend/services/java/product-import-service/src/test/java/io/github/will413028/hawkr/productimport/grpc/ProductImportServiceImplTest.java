package io.github.will413028.hawkr.productimport.grpc;

import io.github.will413028.hawkr.common.utils.code.ImportJobStatus;
import io.github.will413028.hawkr.productimport.helper.ImportJobProgressCalculator;
import io.github.will413028.hawkr.productimport.service.CategoryTokenService;
import io.github.will413028.hawkr.productimport.service.CommitJobService;
import io.github.will413028.hawkr.productimport.service.ImportJobService;
import io.github.will413028.hawkr.productimport.service.ItemMutationService;
import io.github.will413028.hawkr.productimport.service.ItemQueryService;
import com.google.protobuf.ByteString;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatCode;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * gRPC 層防回歸（issue #1664）：CreateImportJob 的 response.import_job_id 在
 * ImportJobService.createJob 的三條 CreateJobResult return 路徑（success / CSV_INVALID / dedup，
 * 見 ImportJobService.java:162 / :152 / :97）皆必須為「非空數字字串」。
 *
 * <p>proto 契約（product_import.proto:143）明載「數字字串；即便 status=CSV_INVALID 也有 job_id」。
 * gRPC 層以單一 setImportJobId(String.valueOf(r.importJobId())) 映射三條路徑
 * （ProductImportServiceImpl.java:78），本測試逐條驗證映射不會漏填，防未來重構回歸。
 */
class ProductImportServiceImplTest {

    private final ImportJobService jobService = mock(ImportJobService.class);
    private final CommitJobService commitJobService = mock(CommitJobService.class);
    private final ItemQueryService itemQueryService = mock(ItemQueryService.class);
    private final ItemMutationService itemMutationService = mock(ItemMutationService.class);
    private final CategoryTokenService categoryTokenService = mock(CategoryTokenService.class);
    private final ImportJobProgressCalculator progressCalculator = mock(ImportJobProgressCalculator.class);

    private final ProductImportServiceImpl service = new ProductImportServiceImpl(
            jobService, commitJobService, itemQueryService, itemMutationService,
            categoryTokenService, progressCalculator);

    @Test
    void createImportJobResponseCarriesNumericJobIdOnSuccessPath() {
        // ImportJobService.java:162 success return
        stubCreateJob(new ImportJobService.CreateJobResult(
                900123456789L, ImportJobStatus.PARSING.value(), false, List.of()));

        CreateImportJobResponse resp = invokeCreateImportJob();

        assertJobIdIsNonBlankNumeric(resp);
        assertThat(resp.getDedupHit()).isFalse();
    }

    @Test
    void createImportJobResponseCarriesNumericJobIdOnCsvInvalidPath() {
        // ImportJobService.java:152 CSV_INVALID return — 即便驗證失敗仍建 job、仍帶 job_id
        stubCreateJob(new ImportJobService.CreateJobResult(
                900987654321L, ImportJobStatus.CSV_INVALID.value(), false,
                List.of(Map.of("csv_line", "2", "field", "price",
                        "error_type", "INVALID", "message", "bad", "raw_value", "x"))));

        CreateImportJobResponse resp = invokeCreateImportJob();

        assertJobIdIsNonBlankNumeric(resp);
        assertThat(resp.getStatus()).isEqualTo(ImportJobStatus.CSV_INVALID.value());
    }

    @Test
    void createImportJobResponseCarriesNumericJobIdOnDedupHitPath() {
        // ImportJobService.java:97 dedup return — 命中既有 job，仍帶既有 job_id
        stubCreateJob(new ImportJobService.CreateJobResult(
                900555555555L, ImportJobStatus.PARSING.value(), true, List.of()));

        CreateImportJobResponse resp = invokeCreateImportJob();

        assertJobIdIsNonBlankNumeric(resp);
        assertThat(resp.getDedupHit()).isTrue();
    }

    // ---- helpers ----------------------------------------------------------

    private void stubCreateJob(ImportJobService.CreateJobResult result) {
        when(jobService.createJob(any(), any(), any(), any())).thenReturn(result);
    }

    private CreateImportJobResponse invokeCreateImportJob() {
        CreateImportJobRequest request = CreateImportJobRequest.newBuilder()
                .setStoreId(20L)
                .setFilename("test.csv")
                .setContent(ByteString.copyFromUtf8("name,price\nfoo,1"))
                .setTargetStatus("ACTIVE")
                .build();
        RecordingObserver<CreateImportJobResponse> observer = new RecordingObserver<>();
        service.createImportJob(request, observer);
        assertThat(observer.error).isNull();
        assertThat(observer.completed).isTrue();
        assertThat(observer.value).isNotNull();
        return observer.value;
    }

    private void assertJobIdIsNonBlankNumeric(CreateImportJobResponse resp) {
        assertThat(resp.getImportJobId()).isNotBlank();
        assertThatCode(() -> Long.parseLong(resp.getImportJobId()))
                .as("import_job_id 必須為數字字串")
                .doesNotThrowAnyException();
    }

    private static final class RecordingObserver<T> implements StreamObserver<T> {
        private T value;
        private Throwable error;
        private boolean completed;

        @Override
        public void onNext(T v) {
            this.value = v;
        }

        @Override
        public void onError(Throwable t) {
            this.error = t;
        }

        @Override
        public void onCompleted() {
            this.completed = true;
        }
    }
}
