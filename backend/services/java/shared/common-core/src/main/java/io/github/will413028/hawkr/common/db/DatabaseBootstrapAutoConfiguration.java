package io.github.will413028.hawkr.common.db;

import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.boot.autoconfigure.jdbc.DataSourceAutoConfiguration;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.PropertySource;
import org.springframework.core.env.Environment;

/**
 * DatabaseBootstrap 自動配置。
 * <p>
 * 透過 @AutoConfiguration(before = DataSourceAutoConfiguration.class) 確保本 AutoConfig
 * 早於 Spring Boot 的 DataSource 自動配置處理。
 * <p>
 * static @Bean 工廠方法 + BeanFactoryPostProcessor 介面，確保 DatabaseBootstrapInitializer
 * 在所有業務 bean（含 DataSource）建立之前執行。
 * <p>
 * 詳見 38-Flyway 自動同步與 DB 自動初始化詳細設計書 §3.4。
 */
@AutoConfiguration(before = DataSourceAutoConfiguration.class)
@EnableConfigurationProperties(DatabaseBootstrapProperties.class)
@PropertySource("classpath:common-core-defaults.properties")
@ConditionalOnProperty(prefix = "hawkr.db.bootstrap", name = "enabled", havingValue = "true", matchIfMissing = true)
public class DatabaseBootstrapAutoConfiguration {

    @Bean
    public static DatabaseBootstrapInitializer databaseBootstrapInitializer(Environment environment) {
        return new DatabaseBootstrapInitializer(environment);
    }
}
