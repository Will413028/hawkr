package io.github.will413028.hawkr.product.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.product.entity.StockTransactionEntity;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

/**
 * 庫存交易冪等 Repository
 */
@Mapper
public interface StockTransactionRepository extends BaseMapper<StockTransactionEntity> {

    /**
     * 冪等插入（同一 stock_tx_id + operation 重複則忽略）
     *
     * @return 插入筆數（0 = 已存在，1 = 新插入）
     */
    @Insert("""
        INSERT INTO stock_transaction (stock_tx_id, operation, payload, status, created_at)
        VALUES (#{stockTxId}, #{operation}, #{payload}::jsonb, 'SUCCESS', NOW())
        ON CONFLICT (stock_tx_id, operation) DO NOTHING
        """)
    int insertIfAbsent(@Param("stockTxId") String stockTxId,
                       @Param("operation") String operation,
                       @Param("payload") String payload);

    /**
     * 檢查同一 stock_tx_id + operation 是否已存在
     */
    @Select("SELECT COUNT(*) > 0 FROM stock_transaction WHERE stock_tx_id = #{stockTxId} AND operation = #{operation}")
    boolean existsByStockTxIdAndOperation(@Param("stockTxId") String stockTxId,
                                          @Param("operation") String operation);
}
