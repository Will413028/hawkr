package io.github.will413028.hawkr.common.providercall;

import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.context.annotation.Bean;

/**
 * 註冊 {@link ProviderCallExecutor} Bean。
 *
 * <p>微服務在 {@code application.yml} 不需做任何配置；引入 common-core 即自動生效。
 */
@AutoConfiguration
public class ProviderCallAutoConfiguration {

    @Bean
    @ConditionalOnMissingBean(ProviderCallExecutor.class)
    public ProviderCallExecutor providerCallExecutor() {
        return new DefaultProviderCallExecutor();
    }
}
