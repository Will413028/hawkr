package io.github.will413028.hawkr.common.obs.health;

import java.util.List;

import org.springframework.beans.factory.ObjectProvider;
import org.springframework.boot.actuate.health.HealthIndicator;
import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.autoconfigure.condition.ConditionalOnClass;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.context.annotation.Bean;

/**
 * 自動註冊 {@link ComponentReadinessHealthIndicator}，並收集 Spring context 內
 * 所有實作 {@link ComponentReadiness} 的 bean。
 *
 * 沒有任何 ComponentReadiness bean 時，HealthIndicator 仍會註冊但永遠回 UP，
 * 確保新服務不會因「沒元件需要 readiness 檢查」而 readiness 永遠失敗。
 */
@AutoConfiguration
@ConditionalOnClass(HealthIndicator.class)
public class ComponentReadinessAutoConfiguration {

    @Bean
    @ConditionalOnMissingBean
    public ComponentReadinessHealthIndicator componentReadinessHealthIndicator(
            ObjectProvider<ComponentReadiness> componentsProvider) {
        List<ComponentReadiness> components = componentsProvider.orderedStream().toList();
        return new ComponentReadinessHealthIndicator(components);
    }
}
