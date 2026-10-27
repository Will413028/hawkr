package io.github.will413028.hawkr.common.config;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.InitializingBean;
import org.springframework.boot.actuate.autoconfigure.metrics.CompositeMeterRegistryAutoConfiguration;
import org.springframework.boot.actuate.autoconfigure.metrics.MetricsAutoConfiguration;
import org.springframework.boot.actuate.autoconfigure.metrics.export.simple.SimpleMetricsExportAutoConfiguration;
import org.springframework.boot.autoconfigure.AutoConfigurations;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;

/**
 * 自動設定的處理順序由 {@code @AutoConfiguration} 的 after 決定，不是 runner 列出的順序；
 * 本測試確認綁定 bean 在 MeterRegistry 存在時一定註冊。
 */
class MqErrorHandlerMetricsAutoConfigurationTest {

    private final ApplicationContextRunner runner = new ApplicationContextRunner();

    @Test
    void registersBinderWhenMeterRegistryIsProvidedByBootMetricsConfigs() {
        runner.withConfiguration(AutoConfigurations.of(
                        MqErrorHandlerMetricsAutoConfiguration.class,
                        MetricsAutoConfiguration.class,
                        CompositeMeterRegistryAutoConfiguration.class,
                        SimpleMetricsExportAutoConfiguration.class))
                .run(ctx -> assertThat(ctx).hasBean("mqErrorHandlerMeterRegistryBinder")
                        .getBean("mqErrorHandlerMeterRegistryBinder").isInstanceOf(InitializingBean.class));
    }

    @Test
    void skipsBinderWhenNoMeterRegistryExists() {
        runner.withConfiguration(AutoConfigurations.of(MqErrorHandlerMetricsAutoConfiguration.class))
                .run(ctx -> assertThat(ctx).doesNotHaveBean("mqErrorHandlerMeterRegistryBinder"));
    }
}
