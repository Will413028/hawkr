package io.github.will413028.hawkr.common.obs.config;

import org.springframework.boot.actuate.autoconfigure.tracing.OpenTelemetryAutoConfiguration;
import org.springframework.boot.autoconfigure.AutoConfiguration;

/**
 * 測試用：類別全名排在 ObsAutoConfiguration 之前，且聲明在 Spring Boot 的 OTel 設定之後，
 * 模擬別的自動設定把 OTel 設定拉到前面的情境。
 */
@AutoConfiguration(after = OpenTelemetryAutoConfiguration.class)
class AaPullsOtelFirstAutoConfiguration {
}
