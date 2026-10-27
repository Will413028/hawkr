package io.github.will413028.hawkr.common.redis;

import org.springframework.data.redis.core.Cursor;
import org.springframework.data.redis.core.ScanOptions;
import org.springframework.data.redis.core.StringRedisTemplate;

import java.util.ArrayList;
import java.util.List;

/**
 * Redis pattern 失效工具：以非阻塞 SCAN 游標取代阻塞式 KEYS，收集到的 key 以 UNLINK（非阻塞刪除）清除。
 *
 * <p>語意等同舊版 {@code redisTemplate.keys(pattern)} + {@code delete(keys)}：清除的 key 集合完全相同，
 * 差別只在底層機制——SCAN 為漸進式游標（不阻塞 Redis 單執行緒），UNLINK 由 Redis 背景 thread 回收記憶體
 * （不阻塞刪除，Redis 4+）。適用於 hot path 上的 pattern 失效，避免大 keyspace 下 KEYS/DEL 卡住整個 Redis。
 *
 * <p>本工具刻意不吞例外：{@code DataAccessException}（Redis 失聯）等一律往外拋，由各 caller 依自身
 * 降級策略處理（例如 02 §2.7.3 graceful degradation：清除失敗 log.warn 不阻斷觸發清除的寫入業務）。
 * Cursor 以 try-with-resources 關閉，釋放底層 sticky connection。
 */
public final class RedisPatternEvictor {

    /**
     * SCAN 每次往返的掃描 hint 筆數（COUNT，非精確上限）：平衡往返次數與單次掃描量。
     * 200 對常見 pattern 桶大小足夠一兩趟掃完，又不會單趟掃描過久。
     */
    private static final int DEFAULT_SCAN_COUNT = 200;

    private RedisPatternEvictor() {
    }

    /**
     * 以預設 COUNT 掃描並非阻塞刪除符合 {@code pattern} 的所有 key。
     *
     * @return 實際清除的 key 筆數（無命中回 0，不呼叫 UNLINK）
     */
    public static long scanUnlink(StringRedisTemplate redisTemplate, String pattern) {
        return scanUnlink(redisTemplate, pattern, DEFAULT_SCAN_COUNT);
    }

    /**
     * 以指定 COUNT 掃描並非阻塞刪除符合 {@code pattern} 的所有 key。
     *
     * <p>先以 SCAN 游標一路走到 cursor=0 收集完整 key 集合（不漏不重），再一次 UNLINK；
     * 這與舊版 {@code keys(pattern)} 一次取全集 + 單次 {@code delete(keys)} 的清除範圍完全一致。
     *
     * @return 實際清除的 key 筆數（無命中回 0，不呼叫 UNLINK）
     */
    public static long scanUnlink(StringRedisTemplate redisTemplate, String pattern, int count) {
        ScanOptions options = ScanOptions.scanOptions().match(pattern).count(count).build();
        List<String> keys = new ArrayList<>();
        try (Cursor<String> cursor = redisTemplate.scan(options)) {
            while (cursor.hasNext()) {
                keys.add(cursor.next());
            }
        }
        if (keys.isEmpty()) {
            return 0L;
        }
        redisTemplate.unlink(keys);
        return keys.size();
    }
}
