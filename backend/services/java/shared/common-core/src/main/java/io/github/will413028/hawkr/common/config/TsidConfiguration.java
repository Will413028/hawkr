package io.github.will413028.hawkr.common.config;

import io.github.will413028.hawkr.common.utils.TsidUtil;
import jakarta.annotation.PostConstruct;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Configuration;

/**
 * TSID 自動配置類
 * <p>
 * 在服務啟動時自動初始化 TsidUtil
 * <p>
 * 配置方式（application.yml）：
 * <pre>
 * hawkr:
 *   tsid:
 *     enabled: true                    # 啟用 TSID（預設 true）
 *     node-id-base: 0                  # 服務基礎 node id
 *     pod-index: ${POD_INDEX:0}        # Pod 索引（從環境變數讀取）
 * </pre>
 * <p>
 * Node ID 分配規則：
 * - payment-service: node-id-base=0
 * - media-service: node-id-base=100
 * - product-service: node-id-base=200
 */
@Slf4j
@Configuration
@ConditionalOnProperty(
    prefix = "hawkr.tsid",
    name = "enabled",
    havingValue = "true",
    matchIfMissing = true
)
public class TsidConfiguration {

    @Value("${hawkr.tsid.node-id-base:0}")
    private int nodeIdBase;

    @Value("${hawkr.tsid.pod-index:${POD_INDEX:0}}")
    private int podIndex;

    @PostConstruct
    public void init() {
        if (TsidUtil.isInitialized()) {
            log.warn("TsidUtil 已經初始化，跳過重複初始化");
            return;
        }

        try {
            TsidUtil.init(nodeIdBase, podIndex);
            int actualNodeId = nodeIdBase + podIndex;
            log.info("✅ TsidUtil 初始化成功 - Node ID: {} (base={}, pod={})",
                actualNodeId, nodeIdBase, podIndex);
        } catch (Exception e) {
            log.error("❌ TsidUtil 初始化失敗", e);
            throw new IllegalStateException("TSID 初始化失敗，服務無法啟動", e);
        }
    }
}
