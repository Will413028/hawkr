package io.github.will413028.hawkr.productimport.listener;

import com.baomidou.mybatisplus.core.conditions.Wrapper;
import io.github.will413028.hawkr.common.mq.events.MediaImportAssetDownloadedEvent;
import io.github.will413028.hawkr.common.utils.code.ImportAssetLocalStatus;
import io.github.will413028.hawkr.common.utils.code.ImportAssetMediaStatus;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemAssetEntity;
import io.github.will413028.hawkr.productimport.helper.AssetSettleHelper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemAssetMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportMediaCallbackInboxMapper;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.junit.jupiter.api.Test;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageProperties;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionStatus;
import org.springframework.transaction.support.SimpleTransactionStatus;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 40 §2.5 / §5.4.2:applyCallbackBody 必須走 TransactionTemplate,
 * per-asset UPDATE + AssetSettleHelper 同事務,mid-flight throw 必須 rollback。
 */
class MediaCallbackListenerTest {

    private final ProductImportMediaCallbackInboxMapper inboxMapper =
            mock(ProductImportMediaCallbackInboxMapper.class);
    private final ProductImportItemAssetMapper assetMapper =
            mock(ProductImportItemAssetMapper.class);
    private final AssetSettleHelper settleHelper = mock(AssetSettleHelper.class);
    private final TestTxManager txManager = new TestTxManager();
    private final MediaCallbackListener listener = new MediaCallbackListener(
            inboxMapper, assetMapper, settleHelper, txManager);

    private static final ObjectMapper MAPPER = new ObjectMapper();

    @Test
    void applyCallbackMidFlightThrowFiresRollback() throws Exception {
        MediaImportAssetDownloadedEvent event = new MediaImportAssetDownloadedEvent(
                "evt-1", "media-asset-1", 10L, 100L, 555L,
                ImportAssetMediaStatus.UPLOADED.value(),
                "tmp/object/key", "https://preview/url", "image/jpeg", 1024L,
                null, null, "2026-06-20T12:00:00Z");
        Message message = new Message(MAPPER.writeValueAsBytes(event), new MessageProperties());
        Channel channel = mock(Channel.class);

        // inbox dedup pass
        when(inboxMapper.insertIfAbsent(eq("evt-1"), eq("media-asset-1"), any())).thenReturn(1);
        // fan-out: 找到一個 row 要 UPDATE
        ProductImportItemAssetEntity asset = new ProductImportItemAssetEntity();
        asset.setId(555L);
        asset.setImportJobId(10L);
        asset.setStatus(ImportAssetLocalStatus.DOWNLOADING.value());
        when(assetMapper.selectList(any(Wrapper.class))).thenReturn(List.of(asset));
        // assetMapper.updateById 拋例外 → 觸發 rollback
        doThrow(new RuntimeException("simulated asset UPDATE failure"))
                .when(assetMapper).updateById(any());

        // listener 內 catch 會把例外交給 MqErrorHandler 處理(它會 nack);這裡關注 txTemplate 是否觸發 rollback
        listener.onMediaCallback(message, channel, 1L);

        assertThat(txManager.getBeginCount())
                .as("applyCallbackBody 必須走 TransactionTemplate")
                .isGreaterThanOrEqualTo(1);
        assertThat(txManager.getRollbackCount())
                .as("mid-flight throw 必須觸發 rollback")
                .isGreaterThanOrEqualTo(1);
    }

    @Test
    void applyCallbackHappyPathCommitsTx() throws Exception {
        MediaImportAssetDownloadedEvent event = new MediaImportAssetDownloadedEvent(
                "evt-2", "media-asset-2", 10L, 100L, 556L,
                ImportAssetMediaStatus.UPLOADED.value(),
                "tmp/object/key2", "https://preview/url2", "image/jpeg", 2048L,
                null, null, "2026-06-20T12:00:01Z");
        Message message = new Message(MAPPER.writeValueAsBytes(event), new MessageProperties());
        Channel channel = mock(Channel.class);

        when(inboxMapper.insertIfAbsent(eq("evt-2"), eq("media-asset-2"), any())).thenReturn(1);
        ProductImportItemAssetEntity asset = new ProductImportItemAssetEntity();
        asset.setId(556L);
        asset.setImportJobId(10L);
        asset.setStatus(ImportAssetLocalStatus.DOWNLOADING.value());
        when(assetMapper.selectList(any(Wrapper.class))).thenReturn(List.of(asset));

        listener.onMediaCallback(message, channel, 2L);

        assertThat(txManager.getBeginCount())
                .as("applyCallbackBody 必須走 TransactionTemplate")
                .isEqualTo(1);
        assertThat(txManager.getCommitCount())
                .as("happy path 必須 commit 而非 rollback")
                .isEqualTo(1);
        assertThat(txManager.getRollbackCount())
                .as("happy path 不應 rollback")
                .isEqualTo(0);
    }

    @Test
    void duplicateInboxEntryDoesNoWork() throws Exception {
        MediaImportAssetDownloadedEvent event = new MediaImportAssetDownloadedEvent(
                "evt-dup", "media-asset-3", 10L, 100L, 557L,
                ImportAssetMediaStatus.UPLOADED.value(),
                "k", "p", "image/jpeg", 1L, null, null, "2026-06-20T12:00:02Z");
        Message message = new Message(MAPPER.writeValueAsBytes(event), new MessageProperties());
        Channel channel = mock(Channel.class);

        // inbox dedup hit (insertIfAbsent return 0 = 已存在)
        when(inboxMapper.insertIfAbsent(eq("evt-dup"), eq("media-asset-3"), any())).thenReturn(0);

        listener.onMediaCallback(message, channel, 3L);

        // dedup claim 移進 tx 之後（見 duplicateClaimIsInsideTransaction），duplicate 仍會開一個
        // 空 tx —— 重點是「什麼工作都不做」，而非「不開 tx」。
        verify(assetMapper, never()).selectList(any(Wrapper.class));
        verify(settleHelper, never()).onAssetSettled(anyLong());
        assertThat(txManager.getRollbackCount()).isEqualTo(0);
    }

    /**
     * 核心保證：inbox dedup INSERT 必須與 callback body <b>同事務</b>。
     *
     * <p>修復前它在 tx 外自動提交 ⇒ body 失敗 → tx rollback → {@code MqErrorHandler} 原樣重投
     * （同一個 {@code event_id}）→ {@code insertIfAbsent} 回 0 → ack skip
     * ⇒ 該 asset <b>永遠停在 DOWNLOADING</b>、job 永遠不完成，且零告警。
     *
     * <p>鑑別力：把 {@code insertIfAbsent} 搬回 {@code txTemplate.execute(...)} 之前
     * ⇒ 記錄到的 beginCount 會是 0 ⇒ 轉紅。
     */
    @Test
    void dedupClaimIsInsideTransactionAndRollsBackWithBody() throws Exception {
        MediaImportAssetDownloadedEvent event = new MediaImportAssetDownloadedEvent(
                "evt-tx", "media-asset-4", 10L, 100L, 558L,
                ImportAssetMediaStatus.UPLOADED.value(),
                "k", "p", "image/jpeg", 1L, null, null, "2026-06-20T12:00:03Z");
        Message message = new Message(MAPPER.writeValueAsBytes(event), new MessageProperties());
        Channel channel = mock(Channel.class);

        int[] beginCountAtClaim = {-1};
        when(inboxMapper.insertIfAbsent(eq("evt-tx"), eq("media-asset-4"), any())).thenAnswer(inv -> {
            beginCountAtClaim[0] = txManager.getBeginCount();
            return 1;
        });
        ProductImportItemAssetEntity asset = new ProductImportItemAssetEntity();
        asset.setId(558L);
        asset.setImportJobId(10L);
        asset.setStatus(ImportAssetLocalStatus.DOWNLOADING.value());
        when(assetMapper.selectList(any(Wrapper.class))).thenReturn(List.of(asset));
        doThrow(new RuntimeException("simulated asset UPDATE failure"))
                .when(assetMapper).updateById(any());

        listener.onMediaCallback(message, channel, 4L);

        assertThat(beginCountAtClaim[0])
                .as("dedup claim 必須發生在 tx 已開啟之後（與 callback body 同生共死）")
                .isEqualTo(1);
        assertThat(txManager.getRollbackCount())
                .as("body 失敗必須把 dedup claim 一起 rollback，否則重投被自己擋掉")
                .isEqualTo(1);
        assertThat(txManager.getCommitCount()).isEqualTo(0);
    }

    private static class TestTxManager implements PlatformTransactionManager {
        private int beginCount = 0;
        private int commitCount = 0;
        private int rollbackCount = 0;

        int getBeginCount() { return beginCount; }
        int getCommitCount() { return commitCount; }
        int getRollbackCount() { return rollbackCount; }

        @Override
        public TransactionStatus getTransaction(
                org.springframework.transaction.TransactionDefinition definition) {
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
