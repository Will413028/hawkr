package io.github.will413028.hawkr.payment.logistics.scheduler;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import io.github.will413028.hawkr.payment.agentfee.AgentFeeGrossUp;
import io.github.will413028.hawkr.payment.agentfee.AgentFeeService;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsCollectionFakes;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsCollectionService;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsCollectionTxService;
import io.github.will413028.hawkr.payment.logistics.repository.LedgerRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsCollectionAttemptRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsReceivableRepository;
import io.github.will413028.hawkr.payment.logistics.repository.SellerLogisticsAccountRepository;
import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import java.time.LocalDateTime;
import java.util.Optional;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.boot.autoconfigure.AutoConfigurations;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Import;
import org.springframework.transaction.IllegalTransactionStateException;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.annotation.EnableTransactionManagement;
import org.springframework.transaction.support.AbstractPlatformTransactionManager;
import org.springframework.transaction.support.DefaultTransactionStatus;
import org.springframework.transaction.support.TransactionTemplate;

/**
 * 線 B 收款三個 bean 在 Spring 底下<b>真的裝得起來</b>。
 *
 * <h2>為什麼非有這一支不可</h2>
 * payment-service 其餘的測試全是直接 {@code new} 出被測物的單元測試——它們一支都碰不到
 * <b>裝配面</b>：{@code @Value} 的預設值解不解得開、{@code Optional<T>} 在沒有 bean 時
 * 是不是真的注入 empty（而不是啟動失敗）、{@code @ConditionalOnProperty} 的開關有沒有效、
 * 以及<b>建構子裡的租約驗算會不會在啟動時就把錯誤配置擋下來</b>。
 *
 * <p>🔴 最後那一項是重點：把不變式寫進建構子，只有在「它真的在 Spring 啟動路徑上被執行到」
 * 的前提下才是保證。沒有這一支，{@link LogisticsCollectionScheduler#validateLeaseBounds}
 * 就只是一個被單元測試直接呼叫的靜態方法——那證明它<b>算得對</b>，
 * 證明不了它<b>擋得住</b>（同 {@code 02-judgment.md} §2.1.2：新保證要先驗那個機制本身是活的）。
 */
@DisplayName("線 B 收款 worker 的 Spring 裝配")
class LogisticsCollectionSchedulerWiringTest {

    private final ApplicationContextRunner runner = new ApplicationContextRunner()
            .withConfiguration(AutoConfigurations.of())
            .withUserConfiguration(StubDependencies.class, WorkerBeans.class);

    /** 真實的 repository／通道在這一層之外；這裡只要「型別對得上的 bean」。 */
    @Configuration(proxyBeanMethods = false)
    static class StubDependencies {

        private final LogisticsCollectionFakes.TestClock clock =
                new LogisticsCollectionFakes.TestClock(LocalDateTime.of(2026, 9, 14, 3, 0));

        @Bean
        LogisticsReceivableRepository receivables() {
            return new LogisticsCollectionFakes.FakeReceivables(clock);
        }

        @Bean
        LogisticsCollectionAttemptRepository attempts() {
            return new LogisticsCollectionFakes.FakeAttempts(clock);
        }

        @Bean
        SellerLogisticsAccountRepository accounts() {
            return new LogisticsCollectionFakes.FakeAccounts();
        }

        @Bean
        LedgerRepository ledger() {
            return new LogisticsCollectionFakes.FakeLedger();
        }

        @Bean
        AgentFeeService agentFee() {
            return new LogisticsCollectionFakes.FakeAgentFee("HAWK1");
        }

        @Bean
        AgentFeeGrossUp grossUp() {
            return LogisticsCollectionFakes.verifiedGrossUp();
        }

        @Bean
        MeterRegistry meterRegistry() {
            return new SimpleMeterRegistry();
        }
    }

    @Configuration(proxyBeanMethods = false)
    @Import({LogisticsCollectionService.class,
            LogisticsCollectionTxService.class,
            LogisticsCollectionScheduler.class})
    static class WorkerBeans {
    }

    @Test
    @DisplayName("出貨預設值（batch 5／租約 600s／寬限 24h）裝得起來，且兩道 gate 預設是關的")
    void shippedDefaultsStartUpWithGatesClosed() {
        runner.run(context -> {
            assertThat(context).hasNotFailed();
            LogisticsCollectionScheduler scheduler = context.getBean(LogisticsCollectionScheduler.class);
            // 🔴 沒有 SellerMerchantIdResolver bean ⇒ Optional 必須注入 empty，而不是啟動失敗。
            assertThat(context.getBeanNamesForType(
                    LogisticsCollectionService.SellerMerchantIdResolver.class)).isEmpty();
            assertThat(scheduler.gatesOpen())
                    .as("auto-submit 預設 false（EV-P6）＋ resolver 未接線 ⇒ gate 必須是關的")
                    .isFalse();
            // tick 跑得起來且什麼都不做（不得因為 gate 關著就拋）。
            scheduler.runSubmit();
            scheduler.runQuery();
        });
    }

    /**
     * 🔴 {@code Propagation.NEVER} <b>真的被執行到</b>，不只是掛在那裡。
     *
     * <p>它守的東西很具體：attempt 的 pre-commit 必須<b>真的 commit</b> 才呼叫 PayUni。
     * 被外層交易包住的話，那一列直到 PayUni 回來（或逾時 90 秒）都還沒 commit
     * ⇒「先落地才呼叫」整條保證失效，而且<b>完全沒有訊號</b>——
     * 是 {@code 06-no-small-tails.md} §3.10 的典型 fail-open。
     *
     * <p>⚠️ 沒有這一支，{@code NEVER} 只是一個「我宣告了」的註記：
     * proxy 沒建起來、或 {@code @EnableTransactionManagement} 不在路徑上時，它一聲不吭。
     */
    @Test
    @DisplayName("🔴 呼叫端把 submitOne 包在交易裡 → 當場拋（NEVER 真的在執行，不是掛好看的）")
    void submitOneRefusesToRunInsideACallerTransaction() {
        txRunner().run(context -> {
            assertThat(context).hasNotFailed();
            LogisticsCollectionService service = context.getBean(LogisticsCollectionService.class);
            TransactionTemplate tx = new TransactionTemplate(
                    context.getBean(PlatformTransactionManager.class));

            assertThatThrownBy(() -> tx.execute(status -> service.submitOne(
                    new io.github.will413028.hawkr.payment.logistics.repository.entity
                            .LogisticsReceivableEntity(), "HAWK1")))
                    .isInstanceOf(IllegalTransactionStateException.class);
        });
    }

    @Test
    @DisplayName("不在交易裡呼叫 → 正常執行（NEVER 沒有把正常路徑也擋掉）")
    void submitOneRunsNormallyOutsideATransaction() {
        txRunner().run(context -> {
            assertThat(context).hasNotFailed();
            LogisticsCollectionService service = context.getBean(LogisticsCollectionService.class);
            // auto-submit 預設關 ⇒ 回 DISABLED 而不是拋。這一半證明上一支測到的是交易、不是別的。
            assertThat(service.submitOne(
                    new io.github.will413028.hawkr.payment.logistics.repository.entity
                            .LogisticsReceivableEntity(), "HAWK1"))
                    .isEqualTo(LogisticsCollectionService.SubmitResult.DISABLED);
        });
    }

    private ApplicationContextRunner txRunner() {
        return runner.withUserConfiguration(TxEnabled.class);
    }

    /**
     * 交易基礎建設的最小替身：只要讓 {@code @Transactional} 的 proxy 真的建起來、
     * 且「目前有沒有交易」答得出來就夠了——本檔不驗 commit／rollback 的資料效果
     * （那需要真的 DB，屬整合測試）。
     */
    @Configuration(proxyBeanMethods = false)
    @EnableTransactionManagement
    static class TxEnabled {

        @Bean
        PlatformTransactionManager transactionManager() {
            return new NoopTransactionManager();
        }
    }

    static final class NoopTransactionManager extends AbstractPlatformTransactionManager {

        private static final ThreadLocal<Boolean> ACTIVE = ThreadLocal.withInitial(() -> Boolean.FALSE);

        @Override
        protected Object doGetTransaction() {
            return new Object();
        }

        /** 🔴 這一格是重點：{@code NEVER} 的判定完全走它，回死 false 的話本檔的斷言會失去鑑別力。 */
        @Override
        protected boolean isExistingTransaction(Object transaction) {
            return ACTIVE.get();
        }

        @Override
        protected void doBegin(Object transaction, TransactionDefinition definition) {
            ACTIVE.set(Boolean.TRUE);
        }

        @Override
        protected void doCommit(DefaultTransactionStatus status) {
            ACTIVE.set(Boolean.FALSE);
        }

        @Override
        protected void doRollback(DefaultTransactionStatus status) {
            ACTIVE.set(Boolean.FALSE);
        }
    }

    @Test
    @DisplayName("兩道 gate 都打開 → gatesOpen() 為 true")
    void bothGatesOpen() {
        runner.withPropertyValues("hawkr.payment.logistics.collection.auto-submit-enabled=true")
                .withBean(LogisticsCollectionService.SellerMerchantIdResolver.class,
                        () -> storeId -> Optional.of("HAWK1"))
                .run(context -> {
                    assertThat(context).hasNotFailed();
                    assertThat(context.getBean(LogisticsCollectionScheduler.class).gatesOpen()).isTrue();
                });
    }

    @Test
    @DisplayName("enabled=false → scheduler bean 根本不被建立（緊急關掉的開關要真的有效）")
    void disabledFlagRemovesTheBean() {
        runner.withPropertyValues("hawkr.payment.logistics.collection.enabled=false")
                .run(context -> {
                    assertThat(context).hasNotFailed();
                    assertThat(context).doesNotHaveBean(LogisticsCollectionScheduler.class);
                });
    }

    /**
     * 🔴 <b>把 gate 弄壞、看它真的擋下來</b>（{@code 02-judgment.md} §2.1.2 的機械形式）。
     *
     * <p>調大 {@code batch-size} 而租約沒動，是最容易發生的一種漂移。
     * {@code 50 × 100s = 5000s ≥ 600s} ⇒ 批次跑到後面幾筆時租約已經過期，
     * 另一個 pod 會<b>合法</b>搶走同一列並再打一次 PayUni——那個失效沒有任何錯誤訊息。
     * ⇒ 它必須在<b>啟動時</b>就爆，而不是在 prod 上變成一筆重複扣款。
     */
    @Test
    @DisplayName("🔴 batch-size 調大而租約沒動 → **啟動就失敗**（不是上線後才變成重複扣款）")
    void oversizedBatchFailsStartup() {
        runner.withPropertyValues("hawkr.payment.logistics.collection.batch-size=50")
                // ⚠️ 斷言要打在 **root cause** 上：Spring 把建構子例外包成 BeanCreationException，
                //    而外層那一句的訊息只寫「Constructor threw exception」——
                //    對外層斷言會在「context 確實啟動失敗」時仍然轉紅，看起來像 gate 沒生效。
                .run(context -> assertThat(context)
                        .hasFailed()
                        .getFailure()
                        .rootCause()
                        .hasMessageContaining("submit-lease-seconds"));
    }

    @Test
    @DisplayName("🔴 查詢租約 ≥ 寬限期 → 啟動就失敗（崩掉的持有者會延後 UNCONFIRMED 判定）")
    void queryLeaseLongerThanGraceFailsStartup() {
        runner.withPropertyValues(
                        "hawkr.payment.logistics.collection.fund-date-grace-hours=1",
                        "hawkr.payment.logistics.collection.query-lease-seconds=3600",
                        "hawkr.payment.logistics.collection.batch-size=1")
                .run(context -> assertThat(context)
                        .hasFailed()
                        .getFailure()
                        .rootCause()
                        .hasMessageContaining("query-lease-seconds"));
    }
}
