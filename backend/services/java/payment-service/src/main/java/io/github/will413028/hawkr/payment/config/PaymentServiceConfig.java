package io.github.will413028.hawkr.payment.config;

import io.github.will413028.hawkr.payment.domain.service.PaymentDomainService;
import io.github.will413028.hawkr.payment.domain.service.RefundDomainService;
import org.mybatis.spring.annotation.MapperScan;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * Payment Service 配置類別
 */
@Configuration
@MapperScan("io.github.will413028.hawkr.payment.infrastructure.persistence.mapper")
public class PaymentServiceConfig {

    /**
     * 支付領域服務
     */
    @Bean
    public PaymentDomainService paymentDomainService() {
        return new PaymentDomainService();
    }

    /**
     * 退款領域服務
     */
    @Bean
    public RefundDomainService refundDomainService() {
        return new RefundDomainService();
    }

}
