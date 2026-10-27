package io.github.will413028.hawkr.common.obs.config;

import net.devh.boot.grpc.server.interceptor.GrpcGlobalServerInterceptor;

import org.springframework.amqp.rabbit.config.AbstractRabbitListenerContainerFactory;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.boot.actuate.autoconfigure.tracing.OpenTelemetryAutoConfiguration;
import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.autoconfigure.condition.ConditionalOnClass;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Primary;
import org.springframework.context.annotation.PropertySource;

import io.github.will413028.hawkr.common.grpc.GrpcAuthClientInterceptor;
import io.github.will413028.hawkr.common.obs.context.ObsContext;
import io.github.will413028.hawkr.common.obs.grpc.GrpcChannelRegistry;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.grpc.ObsGrpcClientInterceptor;
import io.github.will413028.hawkr.common.obs.grpc.ObsGrpcServerInterceptor;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitListenerAdvice;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;

import io.opentelemetry.api.trace.Tracer;
import io.opentelemetry.api.trace.propagation.W3CTraceContextPropagator;
import io.opentelemetry.context.propagation.TextMapPropagator;
import jakarta.annotation.PostConstruct;

/**
 * Auto-configuration for observability library. Automatically registers all
 * required beans based on classpath availability.
 */
// 自動設定的處理順序預設依類別全名排序；obsTextMapPropagator 的 @ConditionalOnMissingBean 與
// obsRabbitPublisher 的 @Qualifier("obsTextMapPropagator") 要求本類別先於 Spring Boot 的 OTel 傳遞設定
// （它會註冊 noopTextMapPropagator）。順序明寫，不靠套件名的字母序。
@AutoConfiguration(before = OpenTelemetryAutoConfiguration.class)
@EnableConfigurationProperties(ObsProperties.class)
@PropertySource("classpath:observability-defaults.properties")
public class ObsAutoConfiguration {

    @Bean
    @ConditionalOnMissingBean
    public ObsContext obsContext() {
        return new ObsContext();
    }

    // obsTracer 已移除：改由 Spring Boot OpenTelemetryAutoConfiguration 提供
    // 正確接線到 OTLP exporter pipeline，避免 noop Tracer 黑洞

    @Bean
    @ConditionalOnMissingBean
    @Primary
    public TextMapPropagator obsTextMapPropagator() {
        return W3CTraceContextPropagator.getInstance();
    }

    @Bean
    @GrpcGlobalServerInterceptor
    public ObsGrpcServerInterceptor obsGrpcServerInterceptor(
            ObsContext obsContext,
            ObsProperties obsProperties,
            Tracer tracer,
            TextMapPropagator propagator) {
        return new ObsGrpcServerInterceptor(obsContext, obsProperties, tracer, propagator);
    }

    @Bean
    public ObsGrpcClientInterceptor obsGrpcClientInterceptor(
            ObsContext obsContext,
            TextMapPropagator propagator) {
        return new ObsGrpcClientInterceptor(obsContext, propagator);
    }

    @Bean
    public GrpcAuthClientInterceptor grpcAuthClientInterceptor(ObsProperties obsProperties) {
        return new GrpcAuthClientInterceptor(obsProperties.getService());
    }

    @Bean
    public GrpcChannelRegistry grpcChannelRegistry(
            ObsProperties obsProperties,
            ObsGrpcClientInterceptor obsInterceptor,
            GrpcAuthClientInterceptor authInterceptor) {
        return new GrpcChannelRegistry(obsProperties, obsInterceptor, authInterceptor);
    }

    @Bean
    public GrpcStubFactory grpcStubFactory(GrpcChannelRegistry registry) {
        return new GrpcStubFactory(registry);
    }

    /**
     * RabbitMQ-specific configuration. Only loaded if RabbitTemplate is on the
     * classpath.
     */
    @Configuration
    @ConditionalOnClass(RabbitTemplate.class)
    static class RabbitMqConfiguration {

        @Bean
        @ConditionalOnMissingBean
        public ObsRabbitPublisher obsRabbitPublisher(
                RabbitTemplate rabbitTemplate,
                ObsContext obsContext,
                @Qualifier("obsTextMapPropagator") TextMapPropagator propagator) {
            return new ObsRabbitPublisher(rabbitTemplate, obsContext, propagator);
        }

        @Bean
        public ObsRabbitListenerAdvice obsRabbitListenerAdvice(
                ObsContext obsContext,
                ObsProperties obsProperties,
                Tracer tracer,
                @Qualifier("obsTextMapPropagator") TextMapPropagator propagator) {
            return new ObsRabbitListenerAdvice(obsContext, obsProperties, tracer, propagator);
        }

        @Bean
        public RabbitListenerContainerFactoryCustomizer rabbitListenerContainerFactoryCustomizer(
                ObsRabbitListenerAdvice advice,
                ObjectProvider<AbstractRabbitListenerContainerFactory<?>> containerFactories) {
            return new RabbitListenerContainerFactoryCustomizer(advice, containerFactories);
        }
    }

    /**
     * Customizer that adds ObsRabbitListenerAdvice to all
     * RabbitListenerContainerFactories.
     */
    static class RabbitListenerContainerFactoryCustomizer {

        private final ObsRabbitListenerAdvice advice;
        private final ObjectProvider<AbstractRabbitListenerContainerFactory<?>> containerFactories;

        RabbitListenerContainerFactoryCustomizer(
                ObsRabbitListenerAdvice advice,
                ObjectProvider<AbstractRabbitListenerContainerFactory<?>> containerFactories) {
            this.advice = advice;
            this.containerFactories = containerFactories;
        }

        @PostConstruct
        public void customize() {
            containerFactories.forEach(factory -> {
                factory.setAdviceChain(advice);
            });
        }
    }
}
