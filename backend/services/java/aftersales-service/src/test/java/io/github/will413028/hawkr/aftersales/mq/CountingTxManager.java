package io.github.will413028.hawkr.aftersales.mq;

import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.TransactionStatus;
import org.springframework.transaction.support.SimpleTransactionStatus;

/**
 * In-memory {@link PlatformTransactionManager}：不開 DB 連線，只計數 begin / commit / rollback，
 * 讓單元測試能驅動 {@code TransactionTemplate.execute(...)} 的完整路徑並斷言 tx 行為。
 *
 * <p>與 {@code RefundResultConsumerTest.TestTxManager} 同一手法，抽出來給多個 consumer test 共用。
 */
class CountingTxManager implements PlatformTransactionManager {

    int beginCount = 0;
    int commitCount = 0;
    int rollbackCount = 0;

    @Override
    public TransactionStatus getTransaction(TransactionDefinition definition) {
        beginCount++;
        return new SimpleTransactionStatus(true);
    }

    @Override
    public void commit(TransactionStatus status) {
        commitCount++;
    }

    @Override
    public void rollback(TransactionStatus status) {
        rollbackCount++;
    }
}
