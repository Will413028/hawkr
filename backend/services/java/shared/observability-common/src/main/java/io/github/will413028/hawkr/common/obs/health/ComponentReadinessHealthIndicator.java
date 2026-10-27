package io.github.will413028.hawkr.common.obs.health;

import java.util.List;
import java.util.LinkedHashMap;
import java.util.Map;

import org.springframework.boot.actuate.health.AbstractHealthIndicator;
import org.springframework.boot.actuate.health.Health;
import org.springframework.boot.actuate.health.Status;

/**
 * 統一收集所有 {@link ComponentReadiness} bean 並對應到 Spring Boot Actuator HealthIndicator。
 *
 * Bean 名稱與 health indicator 名稱：Spring Boot 會將 bean 名稱去除 "HealthIndicator" 後綴後做為
 * health 群組的 key。因此這個類別在 health endpoint 下呈現為 "componentReadiness"。
 *
 * 在 /actuator/health/readiness 群組中（observability-defaults.properties 設定 include=*），
 * 任一元件未 ready → 整體 readiness 報 DOWN → K8s readinessProbe 失敗 → pod 不接流量。
 */
public class ComponentReadinessHealthIndicator extends AbstractHealthIndicator {

    private final List<ComponentReadiness> components;

    public ComponentReadinessHealthIndicator(List<ComponentReadiness> components) {
        super("ComponentReadiness check failed");
        this.components = components;
    }

    @Override
    protected void doHealthCheck(Health.Builder builder) {
        if (components.isEmpty()) {
            builder.status(Status.UP).withDetail("components", "none registered");
            return;
        }

        Map<String, String> details = new LinkedHashMap<>();
        boolean allReady = true;
        for (ComponentReadiness c : components) {
            boolean ready = c.isReady();
            details.put(c.name(), ready ? "UP" : "DOWN");
            if (!ready) {
                allReady = false;
            }
        }

        builder.status(allReady ? Status.UP : Status.DOWN).withDetails(details);
    }
}
