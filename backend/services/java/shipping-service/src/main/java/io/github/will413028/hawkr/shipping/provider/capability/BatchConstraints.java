package io.github.will413028.hawkr.shipping.provider.capability;

import java.util.Set;

/**
 * provider 對批次操作的硬約束。
 *
 * <p>🔴 <b>判準：任何出現在業務層的 provider 數字（單批 50 筆、180 分鐘、30 秒 deadline）都是漏出來的約束。</b>
 * 呼叫端只讀本物件、不寫死數字——把切組邏輯留在 service 層等於把 provider 約束漏出 adapter 邊界。
 *
 * @param maxBatchSize   單批上限（1 表示只支援單筆）
 * @param groupingKeys   同一批必須一致的欄位名（例：溫層、配送方式、出貨日）。空集合表示無分組要求
 */
public record BatchConstraints(int maxBatchSize, Set<String> groupingKeys) {

    public static final BatchConstraints SINGLE_ONLY = new BatchConstraints(1, Set.of());

    public BatchConstraints {
        if (maxBatchSize < 1) {
            throw new IllegalArgumentException("maxBatchSize 必須 >= 1，實得 " + maxBatchSize);
        }
        groupingKeys = Set.copyOf(groupingKeys);
    }

    public boolean supportsBatch() {
        return maxBatchSize > 1;
    }
}
