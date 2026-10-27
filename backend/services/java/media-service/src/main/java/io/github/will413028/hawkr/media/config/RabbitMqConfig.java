package io.github.will413028.hawkr.media.config;

import io.github.will413028.hawkr.common.mq.MqConstants;
import org.springframework.amqp.core.Binding;
import org.springframework.amqp.core.BindingBuilder;
import org.springframework.amqp.core.Queue;
import org.springframework.amqp.core.QueueBuilder;
import org.springframework.amqp.core.TopicExchange;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * media-service RabbitMQ topology（08 號設計書 §7.2a / §7.5 + 36 號設計書 §3.6b）。
 *
 * <p>本服務作為兩個獨立 topology 的同時 owner / consumer：</p>
 *
 * <ol>
 *   <li><b>內部下載 worker pool</b>（owner，exchange={@code hawkr.media}）
 *     <ul>
 *       <li>main queue {@code media.import.asset.download.internal} → 主處理路徑</li>
 *       <li>retry queue {@code ....retry}（TTL 60s + 死信回主 routing key 重投）</li>
 *       <li>DLQ {@code dlq.media.import.asset.download.internal}（手動處理）</li>
 *     </ul>
 *   </li>
 *   <li><b>對外 callback publisher</b>（owner，exchange={@code hawkr.media}，routing key={@code import.asset.downloaded}）
 *     <ul><li>由本服務 {@code obsRabbitPublisher.publish} 發送，consumer 在 product-import-service。</li></ul>
 *   </li>
 *   <li><b>跨服務 gc.expire_job 消費端</b>（consumer，exchange={@code hawkr.product.import}）
 *     <ul>
 *       <li>queue {@code media.product.import.gc.expire_job} bind routing key {@code gc.expire_job}</li>
 *       <li>DLQ {@code dlq.media.product.import.gc.expire_job}</li>
 *     </ul>
 *   </li>
 * </ol>
 *
 * <p>每個 {@code @RabbitListener} 都有對應 {@code @Bean Queue + @Bean Binding}
 * （docs/開發規範/MQ 與事件管理規範.md §1.3 強制），確保服務在全新 RabbitMQ 實例上能自行啟動。</p>
 */
@Configuration
public class RabbitMqConfig {

    // ============================================================================
    // 共用 Media exchange（內部 worker queue + 對外 callback 共用）
    //
    // 36 號設計書原稱 hawkr.media「宣告但未實際使用」；08 號設計書 §7.2a 首次啟用。
    // ============================================================================

    @Bean
    public TopicExchange mediaExchange() {
        // durable=true、autoDelete=false
        return new TopicExchange(MqConstants.EXCHANGE_MEDIA, true, false);
    }

    @Bean
    public TopicExchange mediaDlxExchange() {
        // 內部 worker DLQ 用獨立 DLX（避免污染 import 領域 dlx）
        return new TopicExchange("hawkr.media.dlx", true, false);
    }

    // ============================================================================
    // 內部下載 worker pool（08 號 §7.2a）
    // ============================================================================

    /**
     * 主 queue：worker 取出後執行 SafeFetch + OSS tmp 上傳 + UPDATE。
     * <p>失敗（業務面：SafeFetch / OSS）已在 service 內標 FAILED，listener 仍 ACK；
     * 系統面例外（DB / 反序列化）listener NACK 經 DLX 進 retry queue。</p>
     */
    @Bean
    public Queue mediaImportAssetDownloadInternalQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_MEDIA_IMPORT_ASSET_DOWNLOAD_INTERNAL)
            .withArgument("x-dead-letter-exchange", "hawkr.media.dlx")
            .withArgument("x-dead-letter-routing-key",
                MqConstants.QUEUE_MEDIA_IMPORT_ASSET_DOWNLOAD_INTERNAL_RETRY)
            .build();
    }

    @Bean
    public Binding bindMediaImportAssetDownloadInternalQueue(
        Queue mediaImportAssetDownloadInternalQueue,
        TopicExchange mediaExchange
    ) {
        return BindingBuilder.bind(mediaImportAssetDownloadInternalQueue)
            .to(mediaExchange)
            .with(MqConstants.RK_MEDIA_IMPORT_ASSET_DOWNLOAD_INTERNAL);
    }

    /**
     * Retry queue：TTL 60s 後 dead-letter 回主 exchange + 主 routing key 重投。
     * <p>實作 36 號 §3.6b 退避基礎；目前 1 階退避（後續需要可改 5s/30s/2min 三階分 queue）。</p>
     */
    @Bean
    public Queue mediaImportAssetDownloadInternalRetryQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_MEDIA_IMPORT_ASSET_DOWNLOAD_INTERNAL_RETRY)
            .withArgument("x-message-ttl", 60_000)
            .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_MEDIA)
            .withArgument("x-dead-letter-routing-key",
                MqConstants.RK_MEDIA_IMPORT_ASSET_DOWNLOAD_INTERNAL)
            .build();
    }

    @Bean
    public Binding bindMediaImportAssetDownloadInternalRetryQueue(
        Queue mediaImportAssetDownloadInternalRetryQueue,
        TopicExchange mediaDlxExchange
    ) {
        // retry queue 由 DLX 接收，routing key = retry queue 名稱
        return BindingBuilder.bind(mediaImportAssetDownloadInternalRetryQueue)
            .to(mediaDlxExchange)
            .with(MqConstants.QUEUE_MEDIA_IMPORT_ASSET_DOWNLOAD_INTERNAL_RETRY);
    }

    /**
     * DLQ：3 次失敗（或 retry 還是失敗）後最終留存；運維手動排查。
     */
    @Bean
    public Queue mediaImportAssetDownloadInternalDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_MEDIA_IMPORT_ASSET_DOWNLOAD_INTERNAL_DLQ).build();
    }

    @Bean
    public Binding bindMediaImportAssetDownloadInternalDlq(
        Queue mediaImportAssetDownloadInternalDlq,
        TopicExchange mediaDlxExchange
    ) {
        return BindingBuilder.bind(mediaImportAssetDownloadInternalDlq)
            .to(mediaDlxExchange)
            .with(MqConstants.QUEUE_MEDIA_IMPORT_ASSET_DOWNLOAD_INTERNAL_DLQ);
    }

    // ============================================================================
    // 跨服務 gc.expire_job consumer（08 號 §7.5 + 40 號 §6）
    // ============================================================================

    /**
     * product-import 領域 exchange。本服務不擁有，宣告 declare=false 避免覆蓋。
     */
    @Bean
    public TopicExchange productImportExchange() {
        TopicExchange ex = new TopicExchange(MqConstants.EXCHANGE_PRODUCT_IMPORT, true, false);
        ex.setShouldDeclare(false);
        return ex;
    }

    /**
     * product-import DLX。失敗的 gc.expire_job 訊息 dead-letter 到此處再進 DLQ。
     * 一般由 product-import-service owner 宣告；此處 declare=false。
     */
    @Bean
    public TopicExchange productImportDlxExchange() {
        TopicExchange ex = new TopicExchange(MqConstants.EXCHANGE_PRODUCT_IMPORT_DLX, true, false);
        ex.setShouldDeclare(false);
        return ex;
    }

    @Bean
    public Queue mediaProductImportGcExpireJobQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_MEDIA_PRODUCT_IMPORT_GC_EXPIRE_JOB)
            .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PRODUCT_IMPORT_DLX)
            .withArgument("x-dead-letter-routing-key",
                MqConstants.QUEUE_MEDIA_PRODUCT_IMPORT_GC_EXPIRE_JOB_DLQ)
            .build();
    }

    @Bean
    public Binding bindMediaProductImportGcExpireJobQueue(
        Queue mediaProductImportGcExpireJobQueue,
        TopicExchange productImportExchange
    ) {
        return BindingBuilder.bind(mediaProductImportGcExpireJobQueue)
            .to(productImportExchange)
            .with(MqConstants.RK_PRODUCT_IMPORT_GC_EXPIRE_JOB);
    }

    /**
     * gc.expire_job DLQ：依 40 號設計書由 Media 端宣告（owner = consumer）。
     */
    @Bean
    public Queue mediaProductImportGcExpireJobDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_MEDIA_PRODUCT_IMPORT_GC_EXPIRE_JOB_DLQ).build();
    }

    @Bean
    public Binding bindMediaProductImportGcExpireJobDlq(
        Queue mediaProductImportGcExpireJobDlq,
        TopicExchange productImportDlxExchange
    ) {
        return BindingBuilder.bind(mediaProductImportGcExpireJobDlq)
            .to(productImportDlxExchange)
            .with(MqConstants.QUEUE_MEDIA_PRODUCT_IMPORT_GC_EXPIRE_JOB_DLQ);
    }
}
