package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.product.entity.IdempotencyRequestEntity;
import io.github.will413028.hawkr.product.exception.IdempotencyConflictException;
import io.github.will413028.hawkr.product.repository.IdempotencyRepository;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Propagation;
import org.springframework.transaction.annotation.Transactional;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.HexFormat;
import java.util.Optional;

/**
 * 冪等性檢查服務
 * 確保相同請求只會被處理一次
 */
@Service
public class IdempotencyService {

    private static final ObsLogger log = ObsLogger.forClass(IdempotencyService.class);

    private final IdempotencyRepository idempotencyRepository;
    private final ObjectMapper objectMapper;

    public IdempotencyService(IdempotencyRepository idempotencyRepository, ObjectMapper objectMapper) {
        this.idempotencyRepository = idempotencyRepository;
        this.objectMapper = objectMapper;
    }

    /**
     * 計算請求內容的 SHA-256 雜湊值
     *
     * @param payload 請求內容物件
     * @return SHA-256 雜湊值（十六進位字串）
     */
    public String calculateHash(Object payload) {
        try {
            String json = objectMapper.writeValueAsString(payload);
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            byte[] hash = digest.digest(json.getBytes(StandardCharsets.UTF_8));
            return HexFormat.of().formatHex(hash);
        } catch (JsonProcessingException | NoSuchAlgorithmException e) {
            log.error("hashFailed", "計算請求雜湊失敗: " + e.getMessage());
            // 如果無法計算 hash，返回空字串（降級處理，不阻擋業務）
            return "";
        }
    }

    /**
     * 嘗試取得冪等鎖
     *
     * @param key 冪等鍵
     * @param storeId 賣家 store 代理鍵
     * @param endpoint 端點名稱（如：CreateProduct）
     * @param requestHash 請求內容的雜湊值
     * @return 如果是重複請求，回傳之前的結果；否則回傳 empty
     * @throws IdempotencyConflictException 如果請求正在處理中或請求內容不一致
     */
    @Transactional
    public <T> Optional<T> acquireLock(String key, Long storeId, String endpoint, String requestHash, Class<T> responseType) {
        // 嘗試插入新記錄
        int inserted = idempotencyRepository.tryInsert(key, storeId, endpoint, requestHash);

        if (inserted > 0) {
            // 成功插入，這是新請求
            log.info("success", "取得冪等鎖成功，endpoint=" + endpoint, "key", key);
            return Optional.empty();
        }

        // 已存在記錄，檢查狀態
        // 帶 storeId 撈：不帶的話，撞鍵時撈到的是另一個賣家的記錄（#2306）
        Optional<IdempotencyRequestEntity> existing = idempotencyRepository.findByKeyAndStoreId(key, storeId);
        if (existing.isEmpty()) {
            // 理論上不應該發生
            log.error("inconsistent", "冪等記錄狀態不一致", "key", key);
            throw new IdempotencyConflictException(key, null, "記錄狀態不一致");
        }

        IdempotencyRequestEntity record = existing.get();

        // 驗證請求內容是否一致。
        //
        // 這一關的作用是「確認撞到的確實是同一個請求」，所以確認不了時的正解是
        // 拒絕，不是放行。原本寫成「hash 為空就跳過比對」，配上 key 沒有主體維度
        // （PK 只有 key，見 findByKey 不帶 storeId），效果是：另一個賣家用同一把
        // key 進來時，會拿到 HTTP 200 加上前一位賣家的完整商品資料。
        //
        // 空 hash 不是理論情況：calculateHash 序列化失敗時回 ""（見該方法），
        // request_hash 欄位本身也是 nullable。
        String existingHash = record.getRequestHash();
        if (requestHash == null || requestHash.isEmpty()
                || existingHash == null || existingHash.isEmpty()) {
            log.warn("hashUnavailable", "無法比對請求內容，拒絕重用既有冪等記錄", "key", key);
            throw new IdempotencyConflictException(key, record.getStatus(),
                    "無法確認這個 idempotency_key 對應的是同一個請求，請使用新的 key");
        }
        if (!existingHash.equals(requestHash)) {
            log.warn("hashMismatch", "相同 key 但請求內容不同", "key", key);
            throw new IdempotencyConflictException(key, record.getStatus(),
                    "相同的 idempotency_key 但請求內容不同，請使用新的 key");
        }

        // 使用 CodeMeta 數值：8001=IN_PROGRESS, 8002=SUCCEEDED, 8003=FAILED
        switch (record.getStatus()) {
            case "8001":  // IN_PROGRESS
                log.warn("inProgress", "請求正在處理中", "key", key);
                throw new IdempotencyConflictException(key, record.getStatus(), "請求正在處理中，請稍後重試");

            case "8002":  // SUCCEEDED
                log.info("duplicate", "返回先前成功的結果", "key", key);
                // 解析並返回之前的結果
                if (record.getResponseBody() != null) {
                    try {
                        T response = objectMapper.readValue(record.getResponseBody(), responseType);
                        return Optional.of(response);
                    } catch (JsonProcessingException e) {
                        log.error("parseFailed", "解析先前結果失敗: " + e.getMessage(), "key", key);
                        // 回傳空，讓業務重新處理
                        return Optional.empty();
                    }
                }
                return Optional.empty();

            case "8003":  // FAILED
                // 之前失敗了，允許重試（先刪除再插入）
                idempotencyRepository.deleteByKeyAndStoreId(key, storeId);
                idempotencyRepository.tryInsert(key, storeId, endpoint, requestHash);
                log.info("retry", "先前請求失敗，允許重試", "key", key);
                return Optional.empty();

            default:
                log.error("unknownStatus", "未知的冪等狀態: " + record.getStatus(), "key", key);
                throw new IdempotencyConflictException(key, record.getStatus(), "未知狀態: " + record.getStatus());
        }
    }

    /**
     * 標記請求成功並保存回應
     */
    @Transactional
    public void markSucceeded(String key, Long storeId, Object response) {
        try {
            String responseJson = objectMapper.writeValueAsString(response);
            idempotencyRepository.markSucceeded(key, storeId, responseJson);
            log.info("success", "標記冪等請求成功", "key", key);
        } catch (JsonProcessingException e) {
            log.error("serializeFailed", "序列化回應失敗: " + e.getMessage(), "key", key);
            // 即使序列化失敗，仍然標記為成功（但不保存 response_body）
            idempotencyRepository.markSucceeded(key, storeId, null);
        }
    }

    /**
     * 標記請求失敗
     * 使用 REQUIRES_NEW 確保在新事務中執行，避免原事務失敗後無法更新
     */
    @Transactional(propagation = Propagation.REQUIRES_NEW)
    public void markFailed(String key, Long storeId) {
        idempotencyRepository.markFailed(key, storeId);
        log.info("success", "標記冪等請求失敗", "key", key);
    }

    /**
     * 清理過期的冪等記錄
     */
    @Transactional
    public int cleanupExpired() {
        int deleted = idempotencyRepository.deleteExpired();
        log.info("success", "清理過期冪等記錄，刪除 " + deleted + " 筆");
        return deleted;
    }
}
