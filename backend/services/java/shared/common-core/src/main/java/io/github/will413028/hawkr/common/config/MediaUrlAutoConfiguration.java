package io.github.will413028.hawkr.common.config;

import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.PropertySource;

/**
 * 媒體 URL 組裝共用配置自動注入。
 * <p>
 * 透過 @PropertySource 載入共用預設值，優先級低於微服務 application.yml。
 * 微服務只需 @Autowired MediaUrlProperties 即可使用。
 */
@AutoConfiguration
@EnableConfigurationProperties(MediaUrlProperties.class)
@PropertySource("classpath:common-core-defaults.properties")
public class MediaUrlAutoConfiguration {
}
