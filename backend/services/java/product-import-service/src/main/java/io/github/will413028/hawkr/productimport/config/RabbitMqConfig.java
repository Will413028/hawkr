package io.github.will413028.hawkr.productimport.config;

import io.github.will413028.hawkr.common.mq.MqConstants;
import org.springframework.amqp.core.Binding;
import org.springframework.amqp.core.BindingBuilder;
import org.springframework.amqp.core.Queue;
import org.springframework.amqp.core.QueueBuilder;
import org.springframework.amqp.core.TopicExchange;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * Product Import Service RabbitMQ 配置（40 號設計書 §6）。
 *
 * <p>本服務 owner 一個 exchange：{@code hawkr.product.import}（topic），
 * 並消費 Media exchange {@code hawkr.media} 的 {@code import.asset.downloaded}。
 *
 * <p>Queue 結構（每個主 queue 配 retry + DLQ）：
 * <ul>
 *   <li>4 主 queue：csv.parse / html.fetch / asset.download / commit</li>
 *   <li>4 retry queue（x-message-ttl + DLX 回主 exchange）</li>
 *   <li>4 worker DLQ + 2 跨團隊 publish DLQ（job.parsed / job.commit_finished）</li>
 *   <li>1 callback inbox 主 queue + retry + DLQ（綁 hawkr.media）</li>
 * </ul>
 *
 * <p>Retry TTL（40 號 §6.2）— 此處以「保守單階」TTL；後續 PR 再依設計書 §6.2 三階退避拆分。
 */
@Configuration
public class RabbitMqConfig {

    private static final int TTL_30S = 30_000;
    private static final int TTL_2M = 120_000;
    private static final int TTL_5M = 300_000;
    private static final int TTL_10S = 10_000;

    // ============================================================================
    // Exchange
    // ============================================================================

    /** 本服務 owner — topic exchange。 */
    @Bean
    public TopicExchange productImportExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_PRODUCT_IMPORT, true, false);
    }

    /** 本服務 owner — DLX（依 36 號設計書）。 */
    @Bean
    public TopicExchange productImportDlxExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_PRODUCT_IMPORT_DLX, true, false);
    }

    /** Media exchange 由 Media owner 宣告；本服務僅 consume，shouldDeclare=false。 */
    @Bean
    public TopicExchange mediaExchangeForImport() {
        TopicExchange ex = new TopicExchange(MqConstants.EXCHANGE_MEDIA, true, false);
        ex.setShouldDeclare(false);
        return ex;
    }

    // ============================================================================
    // 1. csv.parse
    // ============================================================================

    @Bean
    public Queue csvParseQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_IMPORT_CSV_PARSE)
                .withArgument("x-dead-letter-exchange", "")
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_PRODUCT_IMPORT_CSV_PARSE_DLQ)
                .build();
    }

    @Bean
    public Queue csvParseRetryQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_IMPORT_CSV_PARSE_RETRY)
                .withArgument("x-message-ttl", TTL_30S)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PRODUCT_IMPORT)
                .withArgument("x-dead-letter-routing-key", MqConstants.RK_PRODUCT_IMPORT_CSV_PARSE)
                .build();
    }

    @Bean
    public Queue csvParseDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_IMPORT_CSV_PARSE_DLQ).build();
    }

    @Bean
    public Binding csvParseBinding(Queue csvParseQueue, TopicExchange productImportExchange) {
        return BindingBuilder.bind(csvParseQueue).to(productImportExchange)
                .with(MqConstants.RK_PRODUCT_IMPORT_CSV_PARSE);
    }

    // ============================================================================
    // 2. html.fetch
    // ============================================================================

    @Bean
    public Queue htmlFetchQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_IMPORT_HTML_FETCH)
                .withArgument("x-dead-letter-exchange", "")
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_PRODUCT_IMPORT_HTML_FETCH_DLQ)
                .build();
    }

    @Bean
    public Queue htmlFetchRetryQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_IMPORT_HTML_FETCH_RETRY)
                .withArgument("x-message-ttl", TTL_2M)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PRODUCT_IMPORT)
                .withArgument("x-dead-letter-routing-key", MqConstants.RK_PRODUCT_IMPORT_HTML_FETCH)
                .build();
    }

    @Bean
    public Queue htmlFetchDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_IMPORT_HTML_FETCH_DLQ).build();
    }

    @Bean
    public Binding htmlFetchBinding(Queue htmlFetchQueue, TopicExchange productImportExchange) {
        return BindingBuilder.bind(htmlFetchQueue).to(productImportExchange)
                .with(MqConstants.RK_PRODUCT_IMPORT_HTML_FETCH);
    }

    // ============================================================================
    // 3. asset.download
    // ============================================================================

    @Bean
    public Queue assetDownloadQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_IMPORT_ASSET_DOWNLOAD)
                .withArgument("x-dead-letter-exchange", "")
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_PRODUCT_IMPORT_ASSET_DOWNLOAD_DLQ)
                .build();
    }

    @Bean
    public Queue assetDownloadRetryQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_IMPORT_ASSET_DOWNLOAD_RETRY)
                .withArgument("x-message-ttl", TTL_2M)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PRODUCT_IMPORT)
                .withArgument("x-dead-letter-routing-key", MqConstants.RK_PRODUCT_IMPORT_ASSET_DOWNLOAD)
                .build();
    }

    @Bean
    public Queue assetDownloadDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_IMPORT_ASSET_DOWNLOAD_DLQ).build();
    }

    @Bean
    public Binding assetDownloadBinding(Queue assetDownloadQueue, TopicExchange productImportExchange) {
        return BindingBuilder.bind(assetDownloadQueue).to(productImportExchange)
                .with(MqConstants.RK_PRODUCT_IMPORT_ASSET_DOWNLOAD);
    }

    // ============================================================================
    // 4. commit
    // ============================================================================

    @Bean
    public Queue commitQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_IMPORT_COMMIT)
                .withArgument("x-dead-letter-exchange", "")
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_PRODUCT_IMPORT_COMMIT_DLQ)
                .build();
    }

    @Bean
    public Queue commitRetryQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_IMPORT_COMMIT_RETRY)
                .withArgument("x-message-ttl", TTL_10S)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_PRODUCT_IMPORT)
                .withArgument("x-dead-letter-routing-key", MqConstants.RK_PRODUCT_IMPORT_COMMIT)
                .build();
    }

    @Bean
    public Queue commitDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_IMPORT_COMMIT_DLQ).build();
    }

    @Bean
    public Binding commitBinding(Queue commitQueue, TopicExchange productImportExchange) {
        return BindingBuilder.bind(commitQueue).to(productImportExchange)
                .with(MqConstants.RK_PRODUCT_IMPORT_COMMIT);
    }

    // ============================================================================
    // 5. 跨團隊 DLQ（job.parsed / job.commit_finished 由本服務 publish，notification-service consume）
    //    DLQ 在本服務宣告以便發送失敗時兜底。
    // ============================================================================

    @Bean
    public Queue jobParsedDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_IMPORT_JOB_PARSED_DLQ).build();
    }

    @Bean
    public Queue jobCommitFinishedDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_IMPORT_JOB_COMMIT_FINISHED_DLQ).build();
    }

    // ============================================================================
    // 6. Media callback（消費 hawkr.media / import.asset.downloaded）
    // ============================================================================

    @Bean
    public Queue mediaCallbackQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_IMPORT_MEDIA_CALLBACK)
                .withArgument("x-dead-letter-exchange", "")
                .withArgument("x-dead-letter-routing-key", MqConstants.QUEUE_PRODUCT_IMPORT_MEDIA_CALLBACK_DLQ)
                .build();
    }

    @Bean
    public Queue mediaCallbackRetryQueue() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_IMPORT_MEDIA_CALLBACK_RETRY)
                .withArgument("x-message-ttl", TTL_5M)
                .withArgument("x-dead-letter-exchange", MqConstants.EXCHANGE_MEDIA)
                .withArgument("x-dead-letter-routing-key", MqConstants.RK_MEDIA_IMPORT_ASSET_DOWNLOADED)
                .build();
    }

    @Bean
    public Queue mediaCallbackDlq() {
        return QueueBuilder.durable(MqConstants.QUEUE_PRODUCT_IMPORT_MEDIA_CALLBACK_DLQ).build();
    }

    @Bean
    public Binding mediaCallbackBinding(Queue mediaCallbackQueue, TopicExchange mediaExchangeForImport) {
        return BindingBuilder.bind(mediaCallbackQueue).to(mediaExchangeForImport)
                .with(MqConstants.RK_MEDIA_IMPORT_ASSET_DOWNLOADED);
    }
}
