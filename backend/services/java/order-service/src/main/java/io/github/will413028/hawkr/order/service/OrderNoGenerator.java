package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import org.springframework.stereotype.Component;

/**
 * 訂單編號生成器
 *
 * 格式：
 * - 訂單編號：O{9位TSID取模}（如 O354931775）
 */
@Component
public class OrderNoGenerator {

    private static final ObsLogger log = ObsLogger.forClass(OrderNoGenerator.class);
    private static final long MODULO = 1_000_000_000L;

    /**
     * 生成訂單編號 O{9位}
     */
    public String generateOrderNo() {
        long seq = TsidUtil.generate() % MODULO;
        String orderNo = "O" + String.format("%09d", seq);
        log.info("OrderNoGenerator.generate.success", "生成訂單編號", "orderNo", orderNo);
        return orderNo;
    }

}
