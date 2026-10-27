package io.github.will413028.hawkr.shipping.audit;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.CodeMetaValidator;
import io.github.will413028.hawkr.common.utils.code.PayUniAuditDirection;
import io.github.will413028.hawkr.payuni.http.PayUniAuditMapper;
import io.github.will413028.hawkr.payuni.http.PayUniExchangeContext;
import io.github.will413028.hawkr.payuni.http.PayUniRequestLogger;
import io.github.will413028.hawkr.shipping.entity.PayUniAuditLogEntity;
import io.github.will413028.hawkr.shipping.mapper.PayUniAuditLogMapper;
import org.springframework.stereotype.Component;

/**
 * shipping-service 的 PayUni 互動 audit logger（payuni_audit_logs 唯一寫入入口）。
 * <p>
 * persist-only：
 * <ul>
 *   <li>{@link #onExchange} 由 payuni-java-client hook 回呼（scheduleHomeLabel / print_label / query 等）。</li>
 *   <li>{@link #recordOutbound} 供 ShipmentLabelService 二次 POST（cat_file_download）手動補稽核（§10.4）。</li>
 * </ul>
 * 兩路共用同一 persist 方法，direction=OUTBOUND。寫入失敗全吞 + ObsLogger.warn，不阻斷物流主流程（見 06 §10.6）。
 */
@Component
public class ShippingPayUniAuditLogger implements PayUniRequestLogger {

    private static final ObsLogger log = ObsLogger.forClass(ShippingPayUniAuditLogger.class);

    private final PayUniAuditLogMapper mapper;

    public ShippingPayUniAuditLogger(PayUniAuditLogMapper mapper) {
        this.mapper = mapper;
    }

    /** hook 落庫：payuni-java-client 每次 exchange 結束回呼一次。 */
    @Override
    public void onExchange(PayUniExchangeContext ctx) {
        persistOutbound(ctx);
    }

    /**
     * 手動補稽核（§10.4）：ShipmentLabelService 的 cat_file_download 二次 POST 不經 PayUniHttpClient，
     * 由呼叫端組裝等價 context 後呼叫本方法。與 hook 共用同一 persist 路徑。
     */
    public void recordOutbound(PayUniExchangeContext ctx) {
        persistOutbound(ctx);
    }

    private void persistOutbound(PayUniExchangeContext ctx) {
        try {
            PayUniAuditLogEntity e = new PayUniAuditLogEntity();
            e.setDirection(PayUniAuditDirection.OUTBOUND.value());
            PayUniAuditMapper.applyTo(PayUniAuditMapper.toAuditRecord(ctx), e);
            CodeMetaValidator.requireValidValue(PayUniAuditDirection.class, e.getDirection(), "direction");
            mapper.insert(e);
        } catch (Exception ex) {
            log.warn("ShippingPayUniAuditLogger.insert.failed",
                    "audit log 寫入失敗 apiPath=" + ctx.getPath() + " err=" + ex.getMessage());
        }
    }
}
