package io.github.will413028.hawkr.shipping.service;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.CodeMetaValidator;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ShipmentAnomalyAckAction;
import io.github.will413028.hawkr.common.utils.code.ShipmentAnomalyType;
import io.github.will413028.hawkr.shipping.dto.ShipmentAnomalyRow;
import io.github.will413028.hawkr.shipping.dto.ShipmentAnomalyTypeCountRow;
import io.github.will413028.hawkr.shipping.entity.ShipmentAnomalyAcknowledgmentEntity;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.shipping.mapper.ShipmentAnomalyAcknowledgmentMapper;
import io.github.will413028.hawkr.shipping.mapper.ShipmentAnomalyQueryMapper;
import io.github.will413028.hawkr.shipping.mapper.ShipmentMapper;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.LocalDateTime;
import java.util.List;

/**
 * §5.21 配送異常賣家通知 — Dashboard 查詢、ack/ignore 操作 service。
 *
 * <p>核心設計：
 * <ul>
 *   <li>異常存在性由 shipments 既有欄位推導（{@link ShipmentAnomalyQueryMapper}），不存獨立狀態</li>
 *   <li>ack 表只記使用者操作；UNIQUE(shipment_id, anomaly_type) 變更走 UPDATE 同筆</li>
 *   <li>所有寫入點透過 CodeMetaValidator.requireValidValue 驗 Enum 合法性（docs/開發規範/Java 微服務開發規範.md §6.4）</li>
 *   <li>查 shipment 屬於該 seller 防 IDOR</li>
 * </ul>
 *
 * 設計書：06 §5.21
 */
@Service
public class ShipmentAnomalyService {

    private static final ObsLogger log = ObsLogger.forClass(ShipmentAnomalyService.class);

    private final ShipmentAnomalyQueryMapper anomalyQueryMapper;
    private final ShipmentAnomalyAcknowledgmentMapper ackMapper;
    private final ShipmentMapper shipmentMapper;

    public ShipmentAnomalyService(ShipmentAnomalyQueryMapper anomalyQueryMapper,
                                  ShipmentAnomalyAcknowledgmentMapper ackMapper,
                                  ShipmentMapper shipmentMapper) {
        this.anomalyQueryMapper = anomalyQueryMapper;
        this.ackMapper = ackMapper;
        this.shipmentMapper = shipmentMapper;
    }

    /** Dashboard 列表（賣家視角）。 */
    public PageResult<ShipmentAnomalyRow> listSellerAnomalies(long sellerId,
                                                              String anomalyType,
                                                              String severity,
                                                              boolean showAcknowledged,
                                                              int page,
                                                              int pageSize) {
        int p = Math.max(1, page);
        int ps = Math.max(1, Math.min(pageSize, 100));
        int offset = (p - 1) * ps;
        List<ShipmentAnomalyRow> rows = anomalyQueryMapper.selectSellerAnomalies(
                sellerId, anomalyType, severity, showAcknowledged, ps, offset);
        long total = anomalyQueryMapper.countSellerAnomalies(
                sellerId, anomalyType, severity, showAcknowledged);
        return new PageResult<>(rows, total, p, ps);
    }

    /** Dashboard 統計（賣家視角）。 */
    public StatsResult getSellerAnomalyStats(long sellerId) {
        List<ShipmentAnomalyTypeCountRow> byType = anomalyQueryMapper.countSellerAnomaliesByType(sellerId);
        long total = 0;
        long high = 0, medium = 0, low = 0;
        for (ShipmentAnomalyTypeCountRow row : byType) {
            total += row.getCnt();
            ShipmentAnomalyType type = ShipmentAnomalyType.fromValue(row.getAnomalyType());
            switch (io.github.will413028.hawkr.common.utils.code.helper.ShipmentAnomalyRules.severityOf(type)) {
                case HIGH -> high += row.getCnt();
                case MEDIUM -> medium += row.getCnt();
                case LOW -> low += row.getCnt();
            }
        }
        return new StatsResult(total, high, medium, low, byType);
    }

    /** Admin 端列表（可選 sellerId 過濾）。 */
    public PageResult<ShipmentAnomalyRow> listAdminAnomalies(Long sellerId,
                                                             String anomalyType,
                                                             String severity,
                                                             boolean showAcknowledged,
                                                             int page,
                                                             int pageSize) {
        int p = Math.max(1, page);
        int ps = Math.max(1, Math.min(pageSize, 100));
        int offset = (p - 1) * ps;
        List<ShipmentAnomalyRow> rows = anomalyQueryMapper.selectAdminAnomalies(
                sellerId, anomalyType, severity, showAcknowledged, ps, offset);
        long total = anomalyQueryMapper.countAdminAnomalies(
                sellerId, anomalyType, severity, showAcknowledged);
        return new PageResult<>(rows, total, p, ps);
    }

    /** actor_type 值（對應 V6 CHECK 約束；設計書 06 §5.21.4）。 */
    private static final String ACTOR_TYPE_SELLER = "SELLER";
    private static final String ACTOR_TYPE_ADMIN = "ADMIN";

    /** 賣家標記已處理。 */
    @Transactional
    public ShipmentAnomalyAcknowledgmentEntity acknowledgeAnomaly(long sellerId,
                                                                  long shipmentId,
                                                                  String anomalyType) {
        return upsertSellerAck(sellerId, shipmentId, anomalyType, ShipmentAnomalyAckAction.ACKNOWLEDGED);
    }

    /** 賣家標記已忽略。 */
    @Transactional
    public ShipmentAnomalyAcknowledgmentEntity ignoreAnomaly(long sellerId,
                                                             long shipmentId,
                                                             String anomalyType) {
        return upsertSellerAck(sellerId, shipmentId, anomalyType, ShipmentAnomalyAckAction.IGNORED);
    }

    /** Admin 標記已處理（issue #1327）。 */
    @Transactional
    public ShipmentAnomalyAcknowledgmentEntity acknowledgeAnomalyForAdmin(long adminAccountId,
                                                                          long shipmentId,
                                                                          String anomalyType) {
        return upsertAdminAck(adminAccountId, shipmentId, anomalyType, ShipmentAnomalyAckAction.ACKNOWLEDGED);
    }

    /** Admin 標記已忽略（issue #1327）。 */
    @Transactional
    public ShipmentAnomalyAcknowledgmentEntity ignoreAnomalyForAdmin(long adminAccountId,
                                                                     long shipmentId,
                                                                     String anomalyType) {
        return upsertAdminAck(adminAccountId, shipmentId, anomalyType, ShipmentAnomalyAckAction.IGNORED);
    }

    /**
     * 賣家寫路徑：actor_type='SELLER'，含 IDOR 歸屬檢查（shipment 必須屬於該 seller）。
     */
    private ShipmentAnomalyAcknowledgmentEntity upsertSellerAck(long sellerId,
                                                                long shipmentId,
                                                                String anomalyType,
                                                                ShipmentAnomalyAckAction action) {
        ShipmentEntity shipment = requireShipment(shipmentId, anomalyType);
        // 防 IDOR：shipment 必須屬於該 seller
        if (shipment.getStoreId() == null || shipment.getStoreId() != sellerId) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_PERMISSION_DENIED);
        }
        return upsertAck(shipment, ACTOR_TYPE_SELLER, sellerId, anomalyType, action);
    }

    /**
     * Admin 寫路徑：actor_type='ADMIN'，actor_id=admin account_id；無 IDOR 歸屬限制（admin 可處置任一單）。
     */
    private ShipmentAnomalyAcknowledgmentEntity upsertAdminAck(long adminAccountId,
                                                               long shipmentId,
                                                               String anomalyType,
                                                               ShipmentAnomalyAckAction action) {
        ShipmentEntity shipment = requireShipment(shipmentId, anomalyType);
        return upsertAck(shipment, ACTOR_TYPE_ADMIN, adminAccountId, anomalyType, action);
    }

    /** anomalyType 合法值驗證 + shipment 存在性檢查。 */
    private ShipmentEntity requireShipment(long shipmentId, String anomalyType) {
        // §8.4 CodeMeta 合法值驗證
        try {
            CodeMetaValidator.requireValidValue(ShipmentAnomalyType.class, anomalyType, "anomalyType");
        } catch (IllegalArgumentException e) {
            throw new ShippingBusinessException(GrpcResponseCode.INVALID_PARAM, e.getMessage());
        }
        ShipmentEntity shipment = shipmentMapper.selectById(shipmentId);
        if (shipment == null) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_NOT_FOUND);
        }
        return shipment;
    }

    /**
     * 依 actor_type 維度 upsert。
     *
     * <p>硬性正確性（設計書 06 §5.21.4）：selectOne 條件必須帶 actor_type，
     * 否則 V6 後同 (shipment, anomaly_type) 可同時存在 SELLER 與 ADMIN 兩列，
     * selectOne 命中 2 筆會拋 TooManyResultsException。
     */
    private ShipmentAnomalyAcknowledgmentEntity upsertAck(ShipmentEntity shipment,
                                                          String actorType,
                                                          long actorId,
                                                          String anomalyType,
                                                          ShipmentAnomalyAckAction action) {
        LocalDateTime now = DateTimeUtils.nowUtc();

        // UNIQUE (shipment_id, anomaly_type, actor_type)：先查（帶 actor_type）再 insert / update
        QueryWrapper<ShipmentAnomalyAcknowledgmentEntity> qw = new QueryWrapper<>();
        qw.eq("shipment_id", shipment.getId())
          .eq("anomaly_type", anomalyType)
          .eq("actor_type", actorType);
        ShipmentAnomalyAcknowledgmentEntity existing = ackMapper.selectOne(qw);

        if (existing != null) {
            existing.setAction(action.value());
            existing.setActedAt(now);
            existing.setActorId(actorId);
            ackMapper.updateById(existing);
            log.info("ShipmentAnomalyService.upsertAck.update",
                    "更新異常處置 actorType=" + actorType + " actorId=" + actorId
                            + " shipmentId=" + shipment.getId()
                            + " anomalyType=" + anomalyType + " action=" + action.value(),
                    "orderNo", shipment.getOrderNo());
            return existing;
        }

        ShipmentAnomalyAcknowledgmentEntity entity = new ShipmentAnomalyAcknowledgmentEntity();
        entity.setShipmentId(shipment.getId());
        entity.setOrderNo(shipment.getOrderNo());
        entity.setAnomalyType(anomalyType);
        entity.setActorType(actorType);
        entity.setActorId(actorId);
        entity.setAction(action.value());
        entity.setActedAt(now);
        ackMapper.insert(entity);
        log.info("ShipmentAnomalyService.upsertAck.insert",
                "新增異常處置 actorType=" + actorType + " actorId=" + actorId
                        + " shipmentId=" + shipment.getId()
                        + " anomalyType=" + anomalyType + " action=" + action.value(),
                "orderNo", shipment.getOrderNo());
        return entity;
    }

    public record PageResult<T>(List<T> records, long total, int page, int pageSize) {}

    public record StatsResult(long total, long highCount, long mediumCount, long lowCount,
                              List<ShipmentAnomalyTypeCountRow> byType) {}
}
