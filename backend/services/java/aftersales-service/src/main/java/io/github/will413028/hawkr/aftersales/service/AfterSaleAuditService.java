package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.entity.AfterSaleAuditLogEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleAuditLogMapper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditActor;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

/**
 * 售後稽核日誌服務。
 * 記錄所有售後相關操作的稽核軌跡。
 */
@Service
@RequiredArgsConstructor
public class AfterSaleAuditService {

    private static final ObsLogger log = ObsLogger.forClass(AfterSaleAuditService.class);

    private final AfterSaleAuditLogMapper auditLogMapper;
    private final ObjectMapper objectMapper;

    /**
     * 記錄稽核日誌。
     *
     * @param caseNo    售後單編號（可為 null，賣家取消出貨場景）
     * @param cancelId  賣家取消請求 ID（可為 null，售後案件場景）
     * @param actorId   操作者 ID——依 actorType 分屬不同 id space（ADMIN=真實登入帳號 account_id /
     *                  BUYER=buyer_id / SELLER=store_id），內部路由進對應欄位；SYSTEM 傳入 null
     * @param actorType 操作者類型（AfterSaleAuditActor value：8401=BUYER / 8402=SELLER / 8403=SYSTEM / 8404=ADMIN）
     * @param action    操作行為
     * @param detail    詳細資訊（非 null 時序列化為 JSON）
     */
    public void log(String caseNo, Long cancelId, Long actorId,
                    String actorType, String action, Object detail) {
        AfterSaleAuditLogEntity entity = new AfterSaleAuditLogEntity();
        entity.setCaseNo(caseNo);
        entity.setCancelId(cancelId);
        switch (AfterSaleAuditActor.fromValue(actorType)) {
            case ADMIN -> entity.setActorAdminAccountId(actorId);
            case BUYER -> entity.setActorBuyerId(actorId);
            case SELLER -> entity.setActorStoreId(actorId);
            case SYSTEM -> { /* all 3 actor-id columns stay null; actorId is null anyway for SYSTEM callers */ }
        }
        entity.setActorType(actorType);
        entity.setAction(action);

        if (detail != null) {
            try {
                entity.setDetail(objectMapper.writeValueAsString(detail));
            } catch (Exception e) {
                log.warn("AfterSaleAuditService.serializeDetail.failed",
                        "序列化稽核日誌 detail 失敗, action=" + action, "caseNo", caseNo != null ? caseNo : "");
                entity.setDetail(String.valueOf(detail));
            }
        }

        auditLogMapper.insert(entity);
    }
}
