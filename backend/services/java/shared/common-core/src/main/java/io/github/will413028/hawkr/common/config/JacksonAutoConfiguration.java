package io.github.will413028.hawkr.common.config;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.autoconfigure.condition.ConditionalOnClass;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.context.annotation.Bean;

import java.util.TimeZone;

/**
 * Jackson ObjectMapper 共用自動配置。
 * <p>
 * 提供支援 Java 8 時間 API（LocalDateTime 等）的 ObjectMapper，
 * 避免各微服務自行註冊 JavaTimeModule。
 * </p>
 * <p>
 * 注意：MyBatis-Plus TypeHandler 使用裸 new ObjectMapper() 是正確的（§11.1），
 * 本 Bean 僅用於 Spring 注入場景（如 RabbitMQ 訊息序列化、REST Controller 等）。
 * </p>
 */
@AutoConfiguration
@ConditionalOnClass(ObjectMapper.class)
public class JacksonAutoConfiguration {

    @Bean
    @ConditionalOnMissingBean
    public ObjectMapper objectMapper() {
        ObjectMapper mapper = new ObjectMapper();
        mapper.registerModule(new JavaTimeModule());
        mapper.disable(SerializationFeature.WRITE_DATES_AS_TIMESTAMPS);
        mapper.setTimeZone(TimeZone.getTimeZone("UTC"));
        return mapper;
    }
}
