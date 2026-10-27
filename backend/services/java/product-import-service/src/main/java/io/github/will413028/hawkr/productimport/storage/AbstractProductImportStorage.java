package io.github.will413028.hawkr.productimport.storage;

import org.springframework.beans.factory.annotation.Value;

import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentMap;

/**
 * Product Import 儲存共用基底：objectKey 生成與 local dev fallback（in-memory cache），
 * 由 {@link ProductImportR2Storage} 沿用（抽象基底保留，便於未來替換/新增 backend 時共用，
 * 工程契約 §15）。
 *
 * <p>子類實作 {@link #enabled()} 與 backend IO（{@link #doPut}/{@link #doGet}/{@link #doDelete}）；
 * 未配置（dev）時走 {@code localCache} fallback，僅回傳虛擬 object_key。
 */
public abstract class AbstractProductImportStorage implements ProductImportStorage {

    @Value("${hawkr.product-import.storage.temp-prefix:tmp}")
    protected String tempPrefix;

    /** Local dev fallback：backend 未配置時，把 CSV bytes 暫存在 memory 給 worker 取回。 */
    private final ConcurrentMap<String, byte[]> localCache = new ConcurrentHashMap<>();

    @Override
    public String uploadCsv(Long storeId, Long jobId, byte[] content) {
        String objectKey = tempPrefix + "/product-import/" + storeId + "/" + jobId + ".csv";
        if (!enabled()) {
            localCache.put(objectKey, content);
            onLocalCache("uploadCsv", objectKey);
            return objectKey;
        }
        doPut(objectKey, content);
        return objectKey;
    }

    @Override
    public byte[] downloadCsv(String objectKey) {
        if (!enabled()) {
            byte[] cached = localCache.get(objectKey);
            if (cached != null) {
                onLocalCache("downloadCsv", objectKey);
            }
            return cached;
        }
        return doGet(objectKey);
    }

    @Override
    public void deleteObject(String objectKey) {
        if (objectKey == null || objectKey.isBlank()) {
            return;
        }
        if (!enabled()) {
            localCache.remove(objectKey);
            return;
        }
        doDelete(objectKey);
    }

    /** backend 是否已配置就緒。 */
    protected abstract boolean enabled();

    protected abstract void doPut(String objectKey, byte[] content);

    protected abstract byte[] doGet(String objectKey);

    protected abstract void doDelete(String objectKey);

    /** local cache 命中時的 log hook（子類覆寫以帶服務名事件）。 */
    protected void onLocalCache(String op, String objectKey) {
        // no-op by default
    }
}
