package io.github.will413028.hawkr.common.config;

import com.baomidou.mybatisplus.annotation.DbType;
import com.baomidou.mybatisplus.autoconfigure.ConfigurationCustomizer;
import com.baomidou.mybatisplus.core.handlers.MetaObjectHandler;
import com.baomidou.mybatisplus.extension.plugins.MybatisPlusInterceptor;
import com.baomidou.mybatisplus.extension.plugins.inner.OptimisticLockerInnerInterceptor;
import com.baomidou.mybatisplus.extension.plugins.inner.PaginationInnerInterceptor;
import org.apache.ibatis.reflection.MetaObject;
import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.context.annotation.Bean;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.time.LocalDateTime;

/**
 * MyBatis-Plus 共用配置（分頁插件 + 時間戳自動填充）
 * <p>
 * 所有 Java 微服務共用，禁止各服務自行定義 MetaObjectHandler。
 * 若微服務需要覆蓋，可自行定義同名 Bean（@ConditionalOnMissingBean 會讓步）。
 * <p>
 * 時間一律使用 UTC（{@link DateTimeUtils#nowUtc()}），禁止裸用 LocalDateTime.now()。
 */
@AutoConfiguration
public class MyBatisPlusAutoConfiguration {

    /**
     * MyBatis-Plus 全域 interceptor。
     * <p>順序依官方建議：樂觀鎖 → 分頁。樂觀鎖必須先於分頁，
     * 否則 pagination 改寫 SQL 後 OptimisticLocker 無法正確注入 version。
     * <p>樂觀鎖規則見「Java 微服務開發規範 §10.5」。
     */
    @Bean
    @ConditionalOnMissingBean
    public MybatisPlusInterceptor mybatisPlusInterceptor() {
        MybatisPlusInterceptor interceptor = new MybatisPlusInterceptor();
        interceptor.addInnerInterceptor(new OptimisticLockerInnerInterceptor());
        interceptor.addInnerInterceptor(new PaginationInnerInterceptor(DbType.POSTGRE_SQL));
        return interceptor;
    }

    /**
     * TIMESTAMPTZ ↔ LocalDateTime 全域 TypeHandler。
     * <p>
     * PostgreSQL JDBC 42.x 不允許 TIMESTAMPTZ → LocalDateTime 隱式轉換，
     * 需透過 getTimestamp().toLocalDateTime() 繞過。
     */
    @Bean
    @ConditionalOnMissingBean
    public ConfigurationCustomizer timestampTzTypeHandlerCustomizer() {
        return configuration -> {
            TimestampTzTypeHandler handler = new TimestampTzTypeHandler();
            // 註冊為 LocalDateTime 的預設 handler（覆蓋所有 JdbcType 場景）
            configuration.getTypeHandlerRegistry().register(LocalDateTime.class, handler);
        };
    }

    /**
     * 時間戳自動填充（createdAt / updatedAt）
     * <p>
     * INSERT 時填 createdAt + updatedAt；UPDATE 時填 updatedAt。
     * Entity 欄位需搭配 @TableField(fill = FieldFill.INSERT) 或 FieldFill.INSERT_UPDATE。
     */
    @Bean
    @ConditionalOnMissingBean
    public MetaObjectHandler metaObjectHandler() {
        return new MetaObjectHandler() {
            @Override
            public void insertFill(MetaObject metaObject) {
                this.strictInsertFill(metaObject, "createdAt", LocalDateTime.class, DateTimeUtils.nowUtc());
                this.strictInsertFill(metaObject, "updatedAt", LocalDateTime.class, DateTimeUtils.nowUtc());
            }

            @Override
            public void updateFill(MetaObject metaObject) {
                this.strictUpdateFill(metaObject, "updatedAt", LocalDateTime.class, DateTimeUtils.nowUtc());
            }
        };
    }
}
