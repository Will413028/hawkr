package io.github.will413028.hawkr.productimport.service;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.ImportJobStatus;
import io.github.will413028.hawkr.common.utils.code.ProductImportEventType;
import io.github.will413028.hawkr.productimport.entity.ProductImportJobEntity;
import io.github.will413028.hawkr.productimport.helper.CsvParser;
import io.github.will413028.hawkr.productimport.helper.L1FileValidator;
import io.github.will413028.hawkr.productimport.helper.L2ContentValidator;
import io.github.will413028.hawkr.productimport.mapper.ProductImportJobMapper;
import io.github.will413028.hawkr.productimport.publisher.ImportEvents.CsvParseEvent;
import io.github.will413028.hawkr.productimport.storage.ProductImportStorage;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.Status;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionTemplate;

import java.io.IOException;
import java.security.MessageDigest;
import java.time.LocalDateTime;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;

/**
 * ImportJob 業務服務（40 號設計書 §5.1 / §5.1a）。
 *
 * <p><b>CSV 上傳一律在事務外，且排在所有會拋錯的檢查之後。</b>R2 不參與 DB 事務：把
 * {@code uploadCsv} 放在 {@code @Transactional} 方法內、後面又跟著會拋 INVALID_ARGUMENT 的
 * parse/L2，任何一次拋錯都留下「R2 有 CSV、{@code product_import_job} 無列」的孤兒，
 * 而 {@link io.github.will413028.hawkr.productimport.scheduler.ProductImportGcJob} 是照 job 列刪物件的，
 * 沒有列就永遠掃不到。</p>
 */
@Service
public class ImportJobService {

    private static final ObsLogger log = ObsLogger.forClass(ImportJobService.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final ProductImportJobMapper jobMapper;
    private final ObsRabbitPublisher publisher;
    private final ProductImportStorage csvStorage;
    private final L1FileValidator l1Validator;
    private final L2ContentValidator l2Validator;
    private final CsvParser csvParser;

    @Value("${hawkr.product-import.job-ttl-days:7}")
    private int jobTtlDays;

    @Value("${hawkr.product-import.csv-invalid-ttl-days:3}")
    private int csvInvalidTtlDays;

    @Value("${hawkr.product-import.max-active-jobs-per-seller:5}")
    private int maxActiveJobsPerSeller;

    @Value("${hawkr.product-import.max-daily-jobs-per-seller:10}")
    private int maxDailyJobsPerSeller;

    @Value("${hawkr.product-import.csv.max-file-size-bytes:2097152}")
    private long csvMaxFileSizeBytes;

    /** DB 寫入段自行控制事務（上傳必須留在事務外，見類別註解）。 */
    private final TransactionTemplate txTemplate;

    public ImportJobService(ProductImportJobMapper jobMapper,
                            ObsRabbitPublisher publisher,
                            ProductImportStorage csvStorage,
                            L1FileValidator l1Validator,
                            L2ContentValidator l2Validator,
                            CsvParser csvParser,
                            PlatformTransactionManager transactionManager) {
        this.jobMapper = jobMapper;
        this.publisher = publisher;
        this.csvStorage = csvStorage;
        this.l1Validator = l1Validator;
        this.l2Validator = l2Validator;
        this.csvParser = csvParser;
        this.txTemplate = new TransactionTemplate(transactionManager);
    }

    public CreateJobResult createJob(Long storeId, String filename, byte[] content, String targetStatus) {
        if (filename == null || filename.isBlank()) {
            throw Status.INVALID_ARGUMENT.withDescription("filename required").asRuntimeException();
        }
        L1FileValidator.L1Result l1 = l1Validator.validate(content);
        if (!l1.passed()) {
            throw Status.INVALID_ARGUMENT
                    .withDescription("L1 validation failed: " + describe(l1.errors()))
                    .asRuntimeException();
        }

        String hash = sha256Hex(content);
        LocalDateTime now = DateTimeUtils.nowUtc();
        LocalDateTime since = now.minusHours(24);

        ProductImportJobEntity existing = jobMapper.findRecentByHash(storeId, hash, since);
        if (existing != null) {
            log.info("ImportJobService.createJob.dedup_hit",
                    "CSV dedup hit, jobId=" + existing.getId() + ", storeId=" + storeId);
            return new CreateJobResult(existing.getId(), existing.getStatus(), true, List.of());
        }

        int filenameConflict = jobMapper.countActiveByFilename(storeId, filename);
        if (filenameConflict > 0) {
            throw Status.ALREADY_EXISTS
                    .withDescription("filename already active for this account: " + filename)
                    .asRuntimeException();
        }

        int activeCount = jobMapper.countActiveByAccount(storeId);
        if (activeCount >= maxActiveJobsPerSeller) {
            throw Status.RESOURCE_EXHAUSTED
                    .withDescription("max active import jobs reached: " + maxActiveJobsPerSeller)
                    .asRuntimeException();
        }

        // §5.1 step 3.1 / Round 5 BUG #14：當日已建 job 數上限（含已 cancel / committed / failed）
        // 防 DoS / cost overrun；Gateway 將 RESOURCE_EXHAUSTED 翻 HTTP 429
        int dailyCount = jobMapper.countCreatedTodayByAccount(storeId);
        if (dailyCount >= maxDailyJobsPerSeller) {
            throw Status.RESOURCE_EXHAUSTED
                    .withDescription("max daily import jobs reached: " + maxDailyJobsPerSeller)
                    .asRuntimeException();
        }

        Long jobId = TsidUtil.generate();

        // parse 先於上傳：parse 失敗會拋 INVALID_ARGUMENT，此時尚未寫任何物件到 R2。
        // L2 失敗不拋（照樣建 CSV_INVALID 列供賣家重傳），故可留在上傳之後。
        List<CsvParser.ParsedRow> rows;
        try {
            rows = csvParser.parse(content);
        } catch (IOException ioe) {
            throw Status.INVALID_ARGUMENT
                    .withDescription("CSV parse error: " + ioe.getMessage())
                    .asRuntimeException();
        }
        L2ContentValidator.L2Result l2 = l2Validator.validate(rows);

        String objectKey = csvStorage.uploadCsv(storeId, jobId, content);

        ProductImportJobEntity job = new ProductImportJobEntity();
        job.setId(jobId);
        job.setStoreId(storeId);
        job.setSourceFilename(filename);
        job.setSourceObjectKey(objectKey);
        job.setSourceFileSize((long) content.length);
        job.setSourceFileHash(hash);
        job.setTotalRows(rows.size());
        job.setTargetStatus(targetStatus == null || targetStatus.isBlank() ? "ACTIVE" : targetStatus);
        if (!l2.passed()) {
            job.setStatus(ImportJobStatus.CSV_INVALID.value());
            job.setValidationErrors(l2.errors());
            job.setExpiresAt(now.plusDays(csvInvalidTtlDays));
        } else {
            job.setStatus(ImportJobStatus.PARSING.value());
            job.setExpiresAt(now.plusDays(jobTtlDays));
        }

        // DB 寫入 + 派工（事務內）；失敗即補償刪除剛上傳的 CSV
        try {
            txTemplate.executeWithoutResult(status -> {
                jobMapper.insert(job);
                if (l2.passed()) {
                    publishCsvParseEvent(jobId, storeId);
                }
            });
        } catch (RuntimeException e) {
            deleteCsvQuietly(objectKey, "createJob");
            throw e;
        }

        if (!l2.passed()) {
            log.info("ImportJobService.createJob.csv_invalid",
                    "L2 failed, jobId=" + jobId + ", errorCount=" + l2.errors().size());
            return new CreateJobResult(jobId, job.getStatus(), false, l2.errors());
        }

        log.info("ImportJobService.createJob.success",
                "Created import job jobId=" + jobId + ", storeId=" + storeId + ", rows=" + rows.size());
        return new CreateJobResult(jobId, job.getStatus(), false, List.of());
    }

    /** 補償刪除：DB 沒建成列時把剛上傳的 CSV 清掉，避免 R2 留下沒有任何一張表認得的物件。 */
    private void deleteCsvQuietly(String objectKey, String stage) {
        if (objectKey == null || objectKey.isBlank()) {
            return;
        }
        try {
            csvStorage.deleteObject(objectKey);
            log.info("ImportJobService." + stage + ".compensateDeleted",
                    "落庫失敗，已刪除剛上傳的 CSV objectKey=" + objectKey);
        } catch (RuntimeException e) {
            log.error("ImportJobService." + stage + ".compensateFailed",
                    "補償刪除失敗，R2 將留下無主 CSV objectKey=" + objectKey, e);
        }
    }

    /** §5.1a ReuploadImportJobCsv：限 status=CSV_INVALID。 */
    public ReuploadResult reuploadCsv(Long storeId, Long importJobId, String filename, byte[] content) {
        ProductImportJobEntity job = jobMapper.selectById(importJobId);
        if (job == null || !job.getStoreId().equals(storeId)) {
            throw Status.NOT_FOUND.withDescription("import job not found").asRuntimeException();
        }
        if (!ImportJobStatus.CSV_INVALID.value().equals(job.getStatus())) {
            throw Status.FAILED_PRECONDITION
                    .withDescription("only CSV_INVALID job can be reuploaded, current=" + job.getStatus())
                    .asRuntimeException();
        }
        if (filename == null || filename.isBlank()) {
            throw Status.INVALID_ARGUMENT.withDescription("filename required").asRuntimeException();
        }
        L1FileValidator.L1Result l1 = l1Validator.validate(content);
        if (!l1.passed()) {
            throw Status.INVALID_ARGUMENT
                    .withDescription("L1 validation failed: " + describe(l1.errors()))
                    .asRuntimeException();
        }

        int filenameConflict = jobMapper.countActiveByFilename(storeId, filename);
        if (filenameConflict > 0 && !filename.equals(job.getSourceFilename())) {
            throw Status.ALREADY_EXISTS
                    .withDescription("filename already active for this account: " + filename)
                    .asRuntimeException();
        }

        // parse 先於上傳（同 createJob）：parse 失敗此時尚未動過 R2。
        List<CsvParser.ParsedRow> rows;
        try {
            rows = csvParser.parse(content);
        } catch (IOException ioe) {
            throw Status.INVALID_ARGUMENT
                    .withDescription("CSV parse error: " + ioe.getMessage())
                    .asRuntimeException();
        }
        L2ContentValidator.L2Result l2 = l2Validator.validate(rows);

        String oldKey = job.getSourceObjectKey();
        String newKey = csvStorage.uploadCsv(storeId, importJobId, content);

        LocalDateTime now = DateTimeUtils.nowUtc();
        job.setSourceFilename(filename);
        job.setSourceObjectKey(newKey);
        job.setSourceFileSize((long) content.length);
        job.setSourceFileHash(sha256Hex(content));
        job.setTotalRows(rows.size());
        if (!l2.passed()) {
            job.setStatus(ImportJobStatus.CSV_INVALID.value());
            job.setValidationErrors(l2.errors());
            job.setExpiresAt(now.plusDays(csvInvalidTtlDays));
        } else {
            job.setStatus(ImportJobStatus.PARSING.value());
            job.setValidationErrors(null);
            job.setExpiresAt(now.plusDays(jobTtlDays));
        }

        try {
            txTemplate.executeWithoutResult(status -> {
                jobMapper.updateById(job);
                if (l2.passed()) {
                    publishCsvParseEvent(importJobId, storeId);
                }
            });
        } catch (RuntimeException e) {
            if (!newKey.equals(oldKey)) {
                deleteCsvQuietly(newKey, "reuploadCsv");
            }
            throw e;
        }

        // 舊物件只在新 key 確定落庫之後才刪。舊版排在 updateById 之前，一旦後面的 parse
        // 或落庫失敗就 rollback，job.source_object_key 仍指著已被刪掉的 oldKey（DB 死指標）。
        if (oldKey != null && !oldKey.equals(newKey)) {
            deleteCsvQuietly(oldKey, "reuploadCsv.old");
        }

        return new ReuploadResult(importJobId, job.getStatus(), l2.passed() ? List.of() : l2.errors());
    }

    public PageResult<ProductImportJobEntity> listJobs(Long storeId, int page, int pageSize) {
        Page<ProductImportJobEntity> p = new Page<>(page, pageSize);
        QueryWrapper<ProductImportJobEntity> qw = new QueryWrapper<>();
        qw.eq("store_id", storeId).orderByDesc("created_at");
        Page<ProductImportJobEntity> result = jobMapper.selectPage(p, qw);
        return new PageResult<>(result.getRecords(), result.getTotal(), page, pageSize);
    }

    public PageResult<ProductImportJobEntity> listJobs(Long storeId, List<String> statusFilter,
                                                       int page, int pageSize) {
        Page<ProductImportJobEntity> p = new Page<>(page, pageSize);
        QueryWrapper<ProductImportJobEntity> qw = new QueryWrapper<>();
        qw.eq("store_id", storeId);
        if (statusFilter != null && !statusFilter.isEmpty()) {
            qw.in("status", statusFilter);
        }
        qw.orderByDesc("created_at");
        Page<ProductImportJobEntity> result = jobMapper.selectPage(p, qw);
        return new PageResult<>(result.getRecords(), result.getTotal(), page, pageSize);
    }

    public ProductImportJobEntity getJob(Long storeId, Long importJobId) {
        ProductImportJobEntity job = jobMapper.selectById(importJobId);
        if (job == null || !job.getStoreId().equals(storeId)) {
            return null;
        }
        return job;
    }

    @Transactional
    public boolean cancelJob(Long storeId, Long importJobId) {
        ProductImportJobEntity job = jobMapper.selectById(importJobId);
        if (job == null || !job.getStoreId().equals(storeId)) {
            return false;
        }
        String s = job.getStatus();
        boolean isTerminal = ImportJobStatus.COMMITTED.value().equals(s)
                || ImportJobStatus.EXPIRED.value().equals(s)
                || ImportJobStatus.CANCELLED.value().equals(s)
                || ImportJobStatus.PARSE_FAILED.value().equals(s);
        if (isTerminal) {
            throw Status.FAILED_PRECONDITION
                    .withDescription("cannot cancel job in terminal status: " + s)
                    .asRuntimeException();
        }
        job.setStatus(ImportJobStatus.CANCELLED.value());
        job.setCancelledAt(DateTimeUtils.nowUtc());
        int affected = jobMapper.updateById(job);
        return affected > 0;
    }

    private void publishCsvParseEvent(Long jobId, Long storeId) {
        try {
            CsvParseEvent event = new CsvParseEvent(jobId, storeId);
            byte[] body = MAPPER.writeValueAsBytes(event);
            publisher.publish(MqConstants.EXCHANGE_PRODUCT_IMPORT,
                    MqConstants.RK_PRODUCT_IMPORT_CSV_PARSE,
                    body,
                    ProductImportEventType.PRODUCT_IMPORT_CSV_PARSE.value());
        } catch (Exception e) {
            log.error("ImportJobService.publishCsvParseEvent.error",
                    "Failed to publish CsvParseEvent", e);
            throw new IllegalStateException("publish CsvParseEvent failed", e);
        }
    }

    private static String describe(List<Map<String, Object>> errors) {
        if (errors == null || errors.isEmpty()) return "";
        StringBuilder sb = new StringBuilder();
        int n = Math.min(errors.size(), 3);
        for (int i = 0; i < n; i++) {
            if (i > 0) sb.append("; ");
            sb.append(errors.get(i).get("message"));
        }
        if (errors.size() > 3) sb.append(" (+" + (errors.size() - 3) + " more)");
        return sb.toString();
    }

    private static String sha256Hex(byte[] content) {
        try {
            MessageDigest md = MessageDigest.getInstance("SHA-256");
            byte[] digest = md.digest(content);
            return HexFormat.of().formatHex(digest);
        } catch (Exception e) {
            throw new IllegalStateException("SHA-256 not available", e);
        }
    }

    public record CreateJobResult(Long importJobId, String status, boolean dedupHit,
                                  List<Map<String, Object>> validationErrors) {}

    public record ReuploadResult(Long importJobId, String status,
                                 List<Map<String, Object>> validationErrors) {}

    public record PageResult<T>(List<T> records, long total, int page, int pageSize) {}
}
