package io.github.will413028.hawkr.common.config;

import org.springframework.amqp.core.AcknowledgeMode;
import org.springframework.amqp.rabbit.config.SimpleRabbitListenerContainerFactory;
import org.springframework.amqp.rabbit.connection.ConnectionFactory;
import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.autoconfigure.condition.ConditionalOnClass;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.context.annotation.Bean;

/**
 * RabbitMQ 共用 Bean 自動配置。
 * <p>
 * 提供 Manual ACK 的 ListenerContainerFactory，
 * 避免各微服務重複定義相同 Bean。
 * </p>
 */
@AutoConfiguration
@ConditionalOnClass(ConnectionFactory.class)
public class RabbitMqAutoConfiguration {

    /**
     * 手動確認模式的 RabbitMQ 監聽容器工廠。
     * prefetch = 10，適用於需要手動 ACK/NACK 的消費者。
     */
    @Bean
    @ConditionalOnMissingBean(name = "manualAckContainerFactory")
    public SimpleRabbitListenerContainerFactory manualAckContainerFactory(ConnectionFactory connectionFactory) {
        SimpleRabbitListenerContainerFactory factory = new SimpleRabbitListenerContainerFactory();
        factory.setConnectionFactory(connectionFactory);
        factory.setAcknowledgeMode(AcknowledgeMode.MANUAL);
        factory.setPrefetchCount(10);
        return factory;
    }
}
