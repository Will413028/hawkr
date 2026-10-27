package io.github.will413028.hawkr.payment.scheduler;

import io.github.will413028.hawkr.payment.config.ReconcileProperties;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.NoSuchBeanDefinitionException;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.SpringBootConfiguration;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.context.ApplicationContext;
import org.springframework.context.annotation.Import;
import org.springframework.test.context.TestPropertySource;

import static org.junit.jupiter.api.Assertions.*;

/**
 * H10: ReconcileScheduler @ConditionalOnProperty 邊界
 *
 * 對應審計報告 §6 H10：production 緊急需要關掉 reconcile 時，
 * 這個開關必須真的能讓 scheduler bean 不被建立。
 *
 * 完全不依賴 DataSource / RabbitMQ：只 Import scheduler class，
 * 觀察 enabled=false 時 bean 是否真的不存在。
 */
@SpringBootTest(classes = ReconcileSchedulerDisabledTest.MinimalConfig.class)
@TestPropertySource(properties = {
        "hawkr.payment.reconcile.pending.enabled=false"
})
class ReconcileSchedulerDisabledTest {

    @SpringBootConfiguration
    @EnableConfigurationProperties(ReconcileProperties.class)
    @Import(PendingPaymentReconcileScheduler.class)
    static class MinimalConfig {
    }

    @Autowired
    private ApplicationContext context;

    @Test
    @DisplayName("H10: enabled=false 時 PendingPaymentReconcileScheduler bean 不該被建立")
    void disabledFlag_schedulerBeanShouldNotExist() {
        assertThrows(NoSuchBeanDefinitionException.class,
                () -> context.getBean(PendingPaymentReconcileScheduler.class),
                "enabled=false 時 scheduler bean 應該不存在");
    }
}
