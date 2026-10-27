package io.github.will413028.hawkr.common.obs.config;

import static org.assertj.core.api.Assertions.assertThat;

import io.opentelemetry.api.trace.propagation.W3CTraceContextPropagator;
import io.opentelemetry.context.propagation.TextMapPropagator;
import io.opentelemetry.api.OpenTelemetry;
import io.opentelemetry.api.trace.Tracer;
import io.opentelemetry.sdk.resources.Resource;
import org.junit.jupiter.api.Test;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.boot.actuate.autoconfigure.tracing.OpenTelemetryAutoConfiguration;
import org.springframework.boot.autoconfigure.AutoConfigurations;
import org.springframework.boot.test.context.FilteredClassLoader;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;

/**
 * 自動設定的處理順序由 {@code @AutoConfiguration} 的 before 決定，不是 runner 列出的順序；
 * 本測試確認 obsTextMapPropagator 先於 Spring Boot 的 OTel 傳遞設定註冊，不被 noop 版本取代。
 */
class ObsAutoConfigurationOrderTest {

    @Test
    void obsPropagatorIsRegisteredBeforeSpringBootOpenTelemetryConfig() {
        new ApplicationContextRunner()
                .withClassLoader(new FilteredClassLoader(RabbitTemplate.class))
                .withBean(Resource.class, Resource::getDefault)
                .withBean(Tracer.class, () -> OpenTelemetry.noop().getTracer("test"))
                .withConfiguration(AutoConfigurations.of(
                        AaPullsOtelFirstAutoConfiguration.class,
                        OpenTelemetryAutoConfiguration.class,
                        ObsAutoConfiguration.class))
                .run(ctx -> {
                    assertThat(ctx).hasBean("obsTextMapPropagator");
                    assertThat(ctx).doesNotHaveBean("noopTextMapPropagator");
                    assertThat(ctx.getBean("obsTextMapPropagator", TextMapPropagator.class))
                            .isSameAs(W3CTraceContextPropagator.getInstance());
                });
    }
}
