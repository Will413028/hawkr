package io.github.will413028.hawkr.common.config;

import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.micrometer.core.instrument.MeterRegistry;
import org.springframework.beans.factory.InitializingBean;
import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.autoconfigure.condition.ConditionalOnBean;
import org.springframework.boot.autoconfigure.condition.ConditionalOnClass;
import org.springframework.context.annotation.Bean;

/**
 * 把 Spring 管理的 {@link MeterRegistry} 綁進靜態工具類 {@link MqErrorHandler}。
 * <p>
 * {@code MqErrorHandler} 是全 static 方法、無 Spring context 的純工具類（被 32 個
 * {@code @RabbitListener} 直接靜態呼叫），無法用建構子注入 {@link MeterRegistry}——
 * 這是唯一的橋接點。{@code @ConditionalOnBean} 確保沒有 {@link MeterRegistry} bean
 * 的服務不會因為缺依賴而啟動失敗，{@link MqErrorHandler} 內部也有對應的 null 防呆。
 * <p>
 * {@code @ConditionalOnBean} 只看已註冊的 bean，自動設定預設依類別全名排序，所以順序明寫：
 * 排在建立 {@link MeterRegistry} 的 Spring Boot 自動設定之後。common-core 的編譯期沒有 actuator，
 * 故以類別名稱字串指定。
 */
@AutoConfiguration(afterName = {
        "org.springframework.boot.actuate.autoconfigure.metrics.MetricsAutoConfiguration",
        "org.springframework.boot.actuate.autoconfigure.metrics.CompositeMeterRegistryAutoConfiguration"
})
@ConditionalOnClass(MeterRegistry.class)
public class MqErrorHandlerMetricsAutoConfiguration {

    @Bean
    @ConditionalOnBean(MeterRegistry.class)
    public InitializingBean mqErrorHandlerMeterRegistryBinder(MeterRegistry meterRegistry) {
        return () -> MqErrorHandler.setMeterRegistry(meterRegistry);
    }
}
