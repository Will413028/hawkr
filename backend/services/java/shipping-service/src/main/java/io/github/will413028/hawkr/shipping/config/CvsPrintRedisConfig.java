package io.github.will413028.hawkr.shipping.config;

import io.github.will413028.hawkr.shipping.service.CvsPrintWaitingRegistry;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.data.redis.connection.RedisConnectionFactory;
import org.springframework.data.redis.listener.PatternTopic;
import org.springframework.data.redis.listener.RedisMessageListenerContainer;

/**
 * 超商列印 Redis Pub/Sub 配置
 * <p>
 * 訂閱 pattern "cvs:print:*"，收到 publish 後回調 CvsPrintWaitingRegistry.onRedisMessage()
 */
@Configuration
public class CvsPrintRedisConfig {

    @Bean
    public RedisMessageListenerContainer cvsPrintListenerContainer(
            RedisConnectionFactory connectionFactory,
            CvsPrintWaitingRegistry registry) {
        RedisMessageListenerContainer container = new RedisMessageListenerContainer();
        container.setConnectionFactory(connectionFactory);
        container.addMessageListener(
                (message, pattern) -> {
                    String channel = new String(message.getChannel());
                    String body = new String(message.getBody());
                    registry.onRedisMessage(channel, body);
                },
                new PatternTopic("cvs:print:*")
        );
        return container;
    }
}
