package io.github.will413028.hawkr.common.utils;

import com.github.f4b6a3.tsid.TsidCreator;
import com.github.f4b6a3.tsid.TsidFactory;

/**
 * TSID (Time-Sorted ID) 工具類
 * <p>
 * 提供全域唯一且時序排序的 64-bit Long ID
 * <p>
 * 結構：42-bit timestamp + 10-bit node + 12-bit sequence
 * - timestamp: 毫秒級時間戳（可用約 139 年）
 * - node: 節點 ID（支援 0-1023）
 * - sequence: 單 ms 內序號（最多 4096 個 ID）
 * <p>
 * Node ID 分配規則：
 * - payment-service: 0-99
 * - media-service: 100-199
 * - product-service: 200-299
 * <p>
 * 使用方式：
 * <pre>
 * // 在服務啟動時初始化（通常在 Configuration 類別）
 * TsidUtil.init(nodeIdBase, podIndex);
 *
 * // 生成 TSID
 * Long id = TsidUtil.generate();
 * </pre>
 *
 * @see com.github.f4b6a3.tsid.TsidFactory
 */
public class TsidUtil {

    private static volatile TsidFactory factory;
    private static volatile boolean initialized = false;

    /**
     * 初始化 TSID Factory
     *
     * @param nodeIdBase 服務基礎 node id（payment=0, media=100, product=200）
     * @param podIndex   Pod 索引（K8s StatefulSet 提供，預設 0）
     * @throws IllegalStateException 如果已經初始化
     * @throws IllegalArgumentException 如果 node id 超出範圍 (0-1023)
     */
    public static synchronized void init(int nodeIdBase, int podIndex) {
        if (initialized) {
            throw new IllegalStateException("TsidUtil 已經初始化，不可重複初始化");
        }

        int nodeId = nodeIdBase + podIndex;

        if (nodeId < 0 || nodeId > 1023) {
            throw new IllegalArgumentException(
                String.format("Node ID 必須在 0-1023 範圍內，當前值：%d (base=%d + pod=%d)",
                    nodeId, nodeIdBase, podIndex)
            );
        }

        factory = TsidFactory.builder()
            .withNodeBits(10)      // 10-bit node id (0-1023)
            .withNode(nodeId)       // 實際 node id
            .build();

        initialized = true;
    }

    /**
     * 生成 TSID
     *
     * @return 64-bit Long ID
     * @throws IllegalStateException 如果尚未初始化
     */
    public static Long generate() {
        if (!initialized) {
            throw new IllegalStateException(
                "TsidUtil 尚未初始化，請先呼叫 TsidUtil.init(nodeIdBase, podIndex)"
            );
        }
        return factory.create().toLong();
    }

    /**
     * 使用預設 TSID Creator 生成（不需初始化，適合測試用途）
     * <p>
     * 注意：此方法使用隨機 node id，不保證跨 JVM 唯一性
     *
     * @return 64-bit Long ID
     */
    public static Long generateDefault() {
        return TsidCreator.getTsid().toLong();
    }

    /**
     * 檢查是否已初始化
     *
     * @return true 如果已初始化
     */
    public static boolean isInitialized() {
        return initialized;
    }

    /**
     * 重置初始化狀態（僅供測試使用）
     */
    static synchronized void reset() {
        factory = null;
        initialized = false;
    }
}
