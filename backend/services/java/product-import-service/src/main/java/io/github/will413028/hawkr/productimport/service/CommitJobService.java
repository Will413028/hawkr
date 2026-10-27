package io.github.will413028.hawkr.productimport.service;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.code.ImportItemStatus;
import io.github.will413028.hawkr.common.utils.code.ImportJobStatus;
import io.github.will413028.hawkr.common.utils.code.ProductImportEventType;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportJobEntity;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportJobMapper;
import io.github.will413028.hawkr.productimport.publisher.ImportEvents.CommitItemEvent;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.grpc.Status;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Set;

@Service
public class CommitJobService {

    private static final ObsLogger log = ObsLogger.forClass(CommitJobService.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final ProductImportJobMapper jobMapper;
    private final ProductImportItemMapper itemMapper;
    private final ObsRabbitPublisher publisher;

    public CommitJobService(ProductImportJobMapper jobMapper,
                            ProductImportItemMapper itemMapper,
                            ObsRabbitPublisher publisher) {
        this.jobMapper = jobMapper;
        this.itemMapper = itemMapper;
        this.publisher = publisher;
    }

    @Transactional
    public CommitResult commitJob(Long storeId, Long importJobId,
                                  boolean retryFailedOnly,
                                  List<Long> includeItemIds) {
        ProductImportJobEntity job = jobMapper.selectById(importJobId);
        if (job == null || !job.getStoreId().equals(storeId)) {
            throw Status.NOT_FOUND.withDescription("import job not found").asRuntimeException();
        }
        if (!ImportJobStatus.READY.value().equals(job.getStatus())
                && !ImportJobStatus.COMMITTING.value().equals(job.getStatus())) {
            throw Status.FAILED_PRECONDITION
                    .withDescription("commit only allowed for READY/COMMITTING, current=" + job.getStatus())
                    .asRuntimeException();
        }

        // 40 §5.7 entry-time self-heal：job.status=COMMITTING 但實際無 item 在 COMMITTING
        // → 屬於前次 commit 在 partial commit state machine 修復（PR #1084）之前留下的「卡死狀態」。
        // 直接同事務 demote 回 READY 讓本次 commit 正常進行；保留 commit_finished_event_published_at
        // 避免重發 ImportJobCommitFinishedEvent。caller / 後續 maybeFinalizeJob 仍會以新一輪 batch
        // 完成時推進到 COMMITTED / READY。
        if (ImportJobStatus.COMMITTING.value().equals(job.getStatus())) {
            Long stillCommitting = itemMapper.selectCount(
                    new QueryWrapper<ProductImportItemEntity>()
                            .eq("import_job_id", importJobId)
                            .eq("status", ImportItemStatus.COMMITTING.value()));
            if (stillCommitting == null || stillCommitting == 0) {
                log.info("CommitJobService.commitJob.selfHealStuckCommitting",
                        "importJobId=" + importJobId
                                + " — job=COMMITTING but 0 items committing, demoting to READY");
                job.setStatus(ImportJobStatus.READY.value());
                jobMapper.updateById(job);
            }
        }

        Set<Long> filter = null;
        if (includeItemIds != null && !includeItemIds.isEmpty()) {
            filter = new HashSet<>(includeItemIds);
        }

        // 40 §4.2.5 partial-success：先撈出 job 內所有非 REMOVED 的「曾經想 commit」的 item，
        // 再分類 eligible / skipped (BLOCKING_ERROR / ALREADY_COMMITTED / NOT_READY)。
        // 撈範圍：所有非 REMOVED 的 item（讓 filter 內如指定 COMMITTED / blocked id 都能被「明確剔除」）
        QueryWrapper<ProductImportItemEntity> allQw = new QueryWrapper<>();
        allQw.eq("import_job_id", importJobId)
                .ne("status", ImportItemStatus.REMOVED.value());
        if (filter != null) allQw.in("id", filter);
        List<ProductImportItemEntity> candidates = itemMapper.selectList(allQw);

        List<ProductImportItemEntity> eligible = new ArrayList<>();
        List<SkipEntry> skipped = new ArrayList<>();
        for (ProductImportItemEntity item : candidates) {
            String s = item.getStatus();
            // ALREADY_COMMITTED — 屬唯讀（§2.2 / §4.6）
            if (ImportItemStatus.COMMITTED.value().equals(s)) {
                skipped.add(new SkipEntry(item.getId(), "ALREADY_COMMITTED",
                        "該商品已併入正式商品，無法再次 commit"));
                continue;
            }
            // COMMITTING — 不能再排入，避免重複觸發
            if (ImportItemStatus.COMMITTING.value().equals(s)) {
                skipped.add(new SkipEntry(item.getId(), "NOT_READY",
                        "該商品正在 commit 中，請稍候"));
                continue;
            }
            // retry_failed_only 限制：非 COMMIT_FAILED 一律剔除
            if (retryFailedOnly && !ImportItemStatus.COMMIT_FAILED.value().equals(s)) {
                skipped.add(new SkipEntry(item.getId(), "NOT_READY",
                        "本次僅重試失敗項，此項目狀態=" + s));
                continue;
            }
            // 非 retry_failed_only：限 READY / COMMIT_FAILED
            if (!retryFailedOnly
                    && !ImportItemStatus.READY.value().equals(s)
                    && !ImportItemStatus.COMMIT_FAILED.value().equals(s)) {
                skipped.add(new SkipEntry(item.getId(), "NOT_READY",
                        "該商品尚未就緒（status=" + s + "）"));
                continue;
            }
            // blocking_error_count > 0 → BLOCKING_ERROR
            if (item.getBlockingErrorCount() != null && item.getBlockingErrorCount() > 0) {
                skipped.add(new SkipEntry(item.getId(), "BLOCKING_ERROR",
                        "尚有未修正的錯誤（" + item.getBlockingErrorCount() + " 筆）"));
                continue;
            }
            eligible.add(item);
        }

        int queued = 0;
        for (ProductImportItemEntity item : eligible) {
            String idemp = "import:" + item.getId() + ":rev:" + item.getCommitRevision();
            String targetStatus = item.getTargetStatus() != null && !item.getTargetStatus().isBlank()
                    ? item.getTargetStatus()
                    : job.getTargetStatus();
            CommitItemEvent ev = new CommitItemEvent(item.getId(), importJobId, storeId,
                    item.getCommitRevision(), targetStatus, idemp);
            try {
                publisher.publish(MqConstants.EXCHANGE_PRODUCT_IMPORT,
                        MqConstants.RK_PRODUCT_IMPORT_COMMIT,
                        MAPPER.writeValueAsBytes(ev),
                        ProductImportEventType.PRODUCT_IMPORT_COMMIT.value());
                queued++;
            } catch (Exception e) {
                log.error("CommitJobService.publish.error",
                        "publish commit failed for itemId=" + item.getId(), e);
            }
        }
        if (queued == 0) {
            // §4.2.5 all-skipped：沒有 MQ item 要排，保留 job 狀態並用 body 回傳逐筆 skip 明細。
            return new CommitResult(job.getStatus(), 0, skipped, CommitOutcome.ALL_SKIPPED);
        }

        job.setStatus(ImportJobStatus.COMMITTING.value());
        jobMapper.updateById(job);
        CommitOutcome outcome = skipped.isEmpty()
                ? CommitOutcome.QUEUED
                : CommitOutcome.PARTIAL_SUCCESS;
        return new CommitResult(ImportJobStatus.COMMITTING.value(), queued, skipped, outcome);
    }

    public enum CommitOutcome {
        QUEUED,
        PARTIAL_SUCCESS,
        ALL_SKIPPED
    }

    public record CommitResult(String status, int queuedCount, List<SkipEntry> skipped, CommitOutcome outcome) {}

    /** 40 §4.2.5 CommitSkipReason 的對應 record。 */
    public record SkipEntry(Long importItemId, String reasonCode, String reasonMessage) {}
}
