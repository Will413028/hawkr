package io.github.will413028.hawkr.product.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.code.ProductEventType;
import io.github.will413028.hawkr.product.mq.dto.ProductCertificationDecidedPayload;
import io.github.will413028.hawkr.product.mq.dto.ProductCertificationVerifyRequestedPayload;
import io.github.will413028.hawkr.product.mq.dto.ProductRestockPayload;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Component;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;

/**
 * 商品事件 MQ 發布者（02 §5.13b / 36 §3.6h / 39 §5.5a）。
 * <p>
 * 發布事件：
 * <ol>
 *   <li>product.restock — 商品補貨（觸發 buyer-service restock_consumer fan-out 到訂閱者）</li>
 *   <li>product.certification.verify.requested — 認證背景查驗請求（<b>product-service 自己消費</b>）</li>
 *   <li>product.certification.approved / .rejected / .revoked — 認證審核結果（notification-service 通知賣家）</li>
 * </ol>
 * <p>
 * ⚠️ 四個認證方法的名稱沿用 36 §3.6h.1 的登記（{@code publishCertificationXxx}，<b>沒有</b>
 * {@code AfterCommit} 字尾），但它們與 {@link #publishRestockAfterCommit} 一樣<b>都是在當前交易
 * commit 之後才真的送出</b> —— 認證的三個決定事件必須等「狀態轉移 ＋ 舊項 SUPERSEDED ＋ 投影重建
 * ＋ audit event」整個交易 commit，否則 rollback 會留下幽靈推播。
 */
@Component
public class ProductEventPublisher {

    private static final ObsLogger log = ObsLogger.forClass(ProductEventPublisher.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final ObsRabbitPublisher obsRabbitPublisher;

    public ProductEventPublisher(ObsRabbitPublisher obsRabbitPublisher) {
        this.obsRabbitPublisher = obsRabbitPublisher;
    }

    /**
     * 發布 {@code product.certification.verify.requested}（19 §12 / 36 §3.6h.1）——
     * 在當前交易 commit 後送出，觸發本服務自己的背景查驗 consumer。
     *
     * <p>只有 {@code gov_verify_state = PENDING}（同步快取未命中）的新申請才需要它；
     * {@code VERIFIED}（已即刻核准）與 {@code UNSUPPORTED}（3704–3706 無政府介面）不發。
     */
    public void publishCertificationVerifyRequested(ProductCertificationVerifyRequestedPayload payload) {
        afterCommit("publishCertificationVerifyRequested",
                "application_id=" + payload.applicationId(),
                () -> doPublish(MqConstants.ROUTING_KEY_PRODUCT_CERTIFICATION_VERIFY_REQUESTED,
                        payload, payload.eventType(),
                        "publishCertificationVerifyRequested",
                        "application_id", payload.applicationId()));
    }

    /**
     * 發布 {@code product.certification.approved}（通知賣家）——在當前交易 commit 後送出。
     *
     * <p>⚠️ <b>同步快取命中的即刻核准不呼叫本方法</b>（賣家當下畫面就看到標章）；
     * 背景補課核准與 ADM 人工核准才發，否則賣家畫面會永遠停在「審核中」（19 §12.3）。
     */
    public void publishCertificationApproved(ProductCertificationDecidedPayload payload) {
        publishDecided(MqConstants.ROUTING_KEY_PRODUCT_CERTIFICATION_APPROVED,
                "publishCertificationApproved", payload);
    }

    /** 發布 {@code product.certification.rejected}（通知賣家，帶 reasonCode／publicReason）。 */
    public void publishCertificationRejected(ProductCertificationDecidedPayload payload) {
        publishDecided(MqConstants.ROUTING_KEY_PRODUCT_CERTIFICATION_REJECTED,
                "publishCertificationRejected", payload);
    }

    /** 發布 {@code product.certification.revoked}（通知賣家；標章已從公開投影移除，必須告知）。 */
    public void publishCertificationRevoked(ProductCertificationDecidedPayload payload) {
        publishDecided(MqConstants.ROUTING_KEY_PRODUCT_CERTIFICATION_REVOKED,
                "publishCertificationRevoked", payload);
    }

    private void publishDecided(String routingKey, String action,
                                ProductCertificationDecidedPayload payload) {
        afterCommit(action, "application_id=" + payload.applicationId(),
                () -> doPublish(routingKey, payload, payload.eventType(), action,
                        "application_id", payload.applicationId()));
    }

    /**
     * 把 {@code action} 掛到當前交易的 afterCommit；沒有事務同步時 fallback 為 eager publish
     * ＋ WARN log（形狀與 {@link #publishRestockAfterCommit} 一致）。
     */
    private void afterCommit(String action, String context, Runnable task) {
        if (TransactionSynchronizationManager.isSynchronizationActive()) {
            TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
                @Override
                public void afterCommit() {
                    task.run();
                }
            });
            return;
        }
        log.warn("ProductEventPublisher." + action + ".noActiveTx",
                "Spring 事務同步未啟用，採 eager publish。caller 應確認在 @Transactional 邊界內呼叫 " + context);
        task.run();
    }

    /**
     * 實際送出。
     * <p>best-effort：失敗僅記 error log，不中斷主流程（不引入 application-level outbox，
     * 見 docs/開發規範/後端服務責任邊界規範.md）。認證事件的可靠性由 MQ 的 retry／DLQ 與
     * ADM「重新查驗」端點提供。
     */
    private void doPublish(String routingKey, Object payload, String eventType,
                           String action, String logKey, String logValue) {
        try {
            byte[] body = MAPPER.writeValueAsBytes(payload);
            obsRabbitPublisher.publish(MqConstants.EXCHANGE_PRODUCT, routingKey, body, eventType);
            log.info("ProductEventPublisher." + action + ".success",
                    "商品認證事件發布成功 routingKey=" + routingKey + " " + logKey + "=" + logValue,
                    logKey, logValue);
        } catch (Exception e) {
            log.error("ProductEventPublisher." + action + ".error",
                    "商品認證事件發布失敗 routingKey=" + routingKey + " " + logKey + "=" + logValue, e);
        }
    }

    /**
     * 在當前事務 commit 後發布 product.restock（02 §5.13b.2）。
     * <ul>
     *   <li>事務同步啟用 → 註冊 afterCommit hook，避免「事務 rollback 仍發 event 造成幽靈推播」</li>
     *   <li>事務同步未啟用（非 @Transactional 路徑） → fallback 為 eager publish，並記 WARN log 提示 caller 可能 misconfigure</li>
     * </ul>
     */
    public void publishRestockAfterCommit(ProductRestockPayload payload) {
        if (TransactionSynchronizationManager.isSynchronizationActive()) {
            TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
                @Override
                public void afterCommit() {
                    publishRestock(payload);
                }
            });
        } else {
            log.warn("ProductEventPublisher.publishRestock.noActiveTx",
                    "Spring 事務同步未啟用，採 eager publish。caller 應確認在 @Transactional 邊界內呼叫"
                            + " productId=" + payload.productId()
                            + " eventId=" + payload.eventId());
            publishRestock(payload);
        }
    }

    /**
     * 直接發布 product.restock（02 §5.13b）。
     * <p>
     * best-effort：失敗僅記 error log，不中斷主流程；下次補貨 transition 自動補發，
     * 不引入 application-level outbox（docs/開發規範/後端服務責任邊界規範.md）。
     */
    private void publishRestock(ProductRestockPayload payload) {
        try {
            byte[] body = MAPPER.writeValueAsBytes(payload);
            obsRabbitPublisher.publish(
                    MqConstants.EXCHANGE_PRODUCT,
                    MqConstants.ROUTING_KEY_PRODUCT_RESTOCK,
                    body,
                    ProductEventType.PRODUCT_RESTOCK.value());
            log.info("ProductEventPublisher.publishRestock.success",
                    "商品補貨事件發布成功 eventId=" + payload.eventId(),
                    "productId", payload.productId());
        } catch (Exception e) {
            log.error("ProductEventPublisher.publishRestock.error",
                    "商品補貨事件發布失敗 productId=" + payload.productId()
                            + " eventId=" + payload.eventId(),
                    e);
        }
    }
}
