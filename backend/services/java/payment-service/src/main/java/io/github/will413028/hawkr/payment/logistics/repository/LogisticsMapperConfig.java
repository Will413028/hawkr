package io.github.will413028.hawkr.payment.logistics.repository;

import org.mybatis.spring.annotation.MapperScan;
import org.springframework.context.annotation.Configuration;

/**
 * 把 logistics 的 mapper 套件掛進 MyBatis 的掃描範圍。
 *
 * <p>🔴 <b>沒有這一支，本包底下的 9 個 {@code @Mapper} 介面全部不會被註冊成 bean</b>，
 * 而症狀是應用啟動時 {@code NoSuchBeanDefinitionException}——不是編譯錯。原因：
 * {@code PaymentServiceConfig} 已經有一個具名的
 * {@code @MapperScan("io.github.will413028.hawkr.payment.infrastructure.persistence.mapper")}，
 * 一旦存在顯式 {@code @MapperScan}，mybatis-spring-boot 的自動掃描
 * （{@code AutoConfiguredMapperScannerRegistrar}）就會因為
 * {@code @ConditionalOnMissingBean(MapperScannerConfigurer.class)} 而關閉
 * ⇒ 「只要標了 {@code @Mapper} 就會被撿到」在本服務<b>不成立</b>。
 *
 * <p>⚠️ 刻意<b>不</b>去改 {@code PaymentServiceConfig} 的那一行：{@code @MapperScan} 是
 * {@code @Repeatable}，各自宣告各自的範圍，新增一個領域不必動到別人的檔案
 * （本輪有其他 agent 在同一個工作樹上施工）。
 */
@Configuration
@MapperScan("io.github.will413028.hawkr.payment.logistics.repository.mapper")
public class LogisticsMapperConfig {
}
