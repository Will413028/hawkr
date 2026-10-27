package io.github.will413028.hawkr.payment.infrastructure.audit;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.CodeMetaValidator;
import io.github.will413028.hawkr.common.utils.code.PayUniAuditDirection;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.PayUniAuditLogEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PayUniAuditLogMapper;
import io.github.will413028.hawkr.payuni.http.PayUniAuditMapper;
import io.github.will413028.hawkr.payuni.http.PayUniExchangeContext;
import io.github.will413028.hawkr.payuni.http.PayUniRequestLogger;
import org.springframework.stereotype.Component;

import java.util.Map;

/**
 * PayUni 互動 audit logger（payuni_audit_logs 專表唯一寫入入口）。
 * <p>
 * 同時承載出向與入向：
 * <ul>
 *   <li>出向：{@link #onExchange} 由 payuni-java-client hook 回呼，direction=OUTBOUND。</li>
 *   <li>入向：{@link #recordInboundWebhook} / {@link #recordInboundReturn} 由 controller / consumer 呼叫，
 *       direction=INBOUND，request_params 存解密後 payload。</li>
 * </ul>
 * append-only；寫入失敗全吞 + ObsLogger.warn，不阻斷 PayUni 主流程（副本性質，不建補償任務，見 05 §11.6）。
 */
@Component
public class PayUniAuditLogger implements PayUniRequestLogger {

    private static final ObsLogger log = ObsLogger.forClass(PayUniAuditLogger.class);

    /** 入向 API 路徑（異步 webhook / 同步 return）。 */
    private static final String INBOUND_PATH_WEBHOOK = "/webhook/payuni/notify";
    private static final String INBOUND_PATH_RETURN = "/payment/return";

    private final PayUniAuditLogMapper mapper;

    public PayUniAuditLogger(PayUniAuditLogMapper mapper) {
        this.mapper = mapper;
    }

    /** 出向稽核：payuni-java-client 每次 exchange 結束回呼一次。 */
    @Override
    public void onExchange(PayUniExchangeContext ctx) {
        PayUniAuditLogEntity e = new PayUniAuditLogEntity();
        e.setDirection(PayUniAuditDirection.OUTBOUND.value());
        PayUniAuditMapper.applyTo(PayUniAuditMapper.toAuditRecord(ctx), e);
        insert(e);
    }

    /**
     * 入向稽核：PayUni → 我方 異步 webhook（PayUniWebhookConsumer 解密 envelope 後、dedup check 之前）。
     * 寫在 dedup 之前，PayUni retry 也留稽核軌跡。
     */
    public void recordInboundWebhook(String tradeNo, String providerTradeNo,
                                     Map<String, String> decryptedPayload) {
        insertInbound(tradeNo, INBOUND_PATH_WEBHOOK, decryptedPayload);
    }

    /**
     * 入向稽核：PayUni → 我方 信用卡 UPP Form POST 同步導回（PaymentReturnController 解密成功後）。
     */
    public void recordInboundReturn(String tradeNo, String providerTradeNo,
                                    Map<String, String> decryptedPayload) {
        insertInbound(tradeNo, INBOUND_PATH_RETURN, decryptedPayload);
    }

    private void insertInbound(String tradeNo, String apiPath, Map<String, String> decryptedPayload) {
        PayUniAuditLogEntity e = new PayUniAuditLogEntity();
        e.setDirection(PayUniAuditDirection.INBOUND.value());
        e.setTradeNo(PayUniAuditMapper.clamp(tradeNo, 32));
        e.setMerId(PayUniAuditMapper.clamp(
                decryptedPayload != null ? decryptedPayload.get("MerID") : null, 20));
        e.setApiPath(apiPath);
        e.setRequestParams(PayUniAuditMapper.maskCredentials(decryptedPayload));
        insert(e);
    }

    private void insert(PayUniAuditLogEntity e) {
        try {
            CodeMetaValidator.requireValidValue(PayUniAuditDirection.class, e.getDirection(), "direction");
            mapper.insert(e);
        } catch (Exception ex) {
            log.warn("PayUniAuditLogger.insert.failed",
                    "audit log 寫入失敗 direction=" + e.getDirection()
                            + " apiPath=" + e.getApiPath() + " err=" + ex.getMessage());
        }
    }
}
