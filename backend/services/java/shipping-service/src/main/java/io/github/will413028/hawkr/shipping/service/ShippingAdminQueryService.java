package io.github.will413028.hawkr.shipping.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import io.github.will413028.hawkr.common.config.MediaUrlProperties;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.OnboardingStatus;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.grpc.AdminSellerShipmentSettingSummary;
import io.github.will413028.hawkr.shipping.grpc.AdminShipmentDetail;
import io.github.will413028.hawkr.shipping.grpc.AdminShipmentSummary;
import io.github.will413028.hawkr.shipping.grpc.ListSellerShipmentSettingsForAdminData;
import io.github.will413028.hawkr.shipping.grpc.ListSellerShipmentSettingsForAdminRequest;
import io.github.will413028.hawkr.shipping.grpc.ListShipmentsForAdminData;
import io.github.will413028.hawkr.shipping.grpc.ListShipmentsForAdminRequest;
import io.github.will413028.hawkr.shipping.mapper.SellerShipmentSettingMapper;
import io.github.will413028.hawkr.shipping.mapper.ShipmentMapper;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;

import java.time.LocalDateTime;
import java.util.Arrays;
import java.util.List;
import java.util.Optional;
import java.util.Set;

/**
 * Admin 異常訂單管理 API 的查詢服務（§29）。
 *
 * 純讀取聚合，無寫入副作用。走 SoT（shipments / seller_shipment_settings 表）。
 */
@Service
public class ShippingAdminQueryService {

    private static final ObsLogger log = ObsLogger.forClass(ShippingAdminQueryService.class);
    private static final int MAX_PAGE_SIZE = 100;

    /** 視為異常的 ShippingStatus value 清單（建單失敗 / 配送異常相關）。 */
    private static final Set<String> ABNORMAL_SHIPMENT_STATUS_VALUES = Set.of(
            ShippingStatus.CREATE_FAILED.value(),
            ShippingStatus.ABNORMAL.value(),
            ShippingStatus.COMPENSATION.value(),
            ShippingStatus.PACKAGE_LOST.value(),
            ShippingStatus.PACKAGE_ABANDONED.value(),
            ShippingStatus.STORE_TRANSFER.value(),
            ShippingStatus.PENDING_HOME_RETURN.value()
    );

    private final ShipmentMapper shipmentMapper;
    private final SellerShipmentSettingMapper settingMapper;
    private final MediaUrlProperties mediaUrlProperties;

    public ShippingAdminQueryService(ShipmentMapper shipmentMapper,
                                     SellerShipmentSettingMapper settingMapper,
                                     MediaUrlProperties mediaUrlProperties) {
        this.shipmentMapper = shipmentMapper;
        this.settingMapper = settingMapper;
        this.mediaUrlProperties = mediaUrlProperties;
    }

    /**
     * 列物流異常。
     */
    public ListShipmentsForAdminData listShipments(ListShipmentsForAdminRequest req) {
        int pageNum = parseIntOrDefault(req.getPage(), 1);
        int pageSize = Math.min(parseIntOrDefault(req.getPageSize(), 20), MAX_PAGE_SIZE);
        boolean abnormalOnly = req.getAbnormalOnly();
        String providerStatus = req.getProviderStatus();
        String statusCsv = req.getStatus();
        String isReturn = req.getIsReturn();
        String shippingMethod = req.getShippingMethod();
        String sellerIdStr = req.getStoreId();
        String orderNo = req.getOrderNo();
        LocalDateTime createdFrom = parseIso(req.getCreatedFrom());
        LocalDateTime createdTo = parseIso(req.getCreatedTo());
        String sort = normalizeSort(req.getSort());

        LambdaQueryWrapper<ShipmentEntity> q = new LambdaQueryWrapper<>();

        // 異常 filter
        //
        // 🔴 provider 側判準是 provider_error_code（V12 拆出的失敗代碼欄），**不是** provider_status。
        // provider_status 保存的是物流商原始貨態（含正常軌跡與尚未 mapping 的未知值），
        // 以「有值」判定異常會讓每一張正常單都進異常清單——那正是 V12 要修掉的 regression。
        // 未知貨態的可觀測性由 metric hawkr_shipping_provider_status_unknown 承擔，不走本清單。
        if (abnormalOnly) {
            q.and(w -> w.in(ShipmentEntity::getStatus, ABNORMAL_SHIPMENT_STATUS_VALUES)
                    .or()
                    .apply("provider_error_code IS NOT NULL AND provider_error_code <> ''"));
        }

        // 具體 provider_status exact
        if (StringUtils.hasText(providerStatus)) {
            q.eq(ShipmentEntity::getProviderStatus, providerStatus);
        }
        // status 多值
        if (StringUtils.hasText(statusCsv)) {
            List<String> statusList = Arrays.asList(statusCsv.split(","));
            q.in(ShipmentEntity::getStatus, statusList);
        }
        // is_return
        if ("true".equalsIgnoreCase(isReturn)) {
            q.eq(ShipmentEntity::getIsReturn, true);
        } else if ("false".equalsIgnoreCase(isReturn)) {
            q.eq(ShipmentEntity::getIsReturn, false);
        }
        // shipping_method
        if (StringUtils.hasText(shippingMethod)) {
            q.eq(ShipmentEntity::getShippingMethod, shippingMethod);
        }
        // store_id（surrogate；proto 欄位 Phase 2 已改名 store_id，見 :75）
        if (StringUtils.hasText(sellerIdStr)) {
            try {
                q.eq(ShipmentEntity::getStoreId, Long.parseLong(sellerIdStr));
            } catch (NumberFormatException ignore) { /* 忽略非法值 */ }
        }
        // order_no
        if (StringUtils.hasText(orderNo)) {
            q.eq(ShipmentEntity::getOrderNo, orderNo);
        }
        // 時間範圍
        if (createdFrom != null) {
            q.ge(ShipmentEntity::getCreatedAt, createdFrom);
        }
        if (createdTo != null) {
            q.lt(ShipmentEntity::getCreatedAt, createdTo);
        }

        applySort(q, sort);

        Page<ShipmentEntity> page = new Page<>(pageNum, pageSize);
        var result = shipmentMapper.selectPage(page, q);

        ListShipmentsForAdminData.Builder builder = ListShipmentsForAdminData.newBuilder()
                .setTotal(String.valueOf(result.getTotal()))
                .setPage(String.valueOf(result.getCurrent()))
                .setPageSize(String.valueOf(result.getSize()));

        for (ShipmentEntity e : result.getRecords()) {
            builder.addRecords(toSummary(e));
        }
        return builder.build();
    }

    /**
     * 取物流單詳情（by TSID）。
     */
    public Optional<AdminShipmentDetail> getShipmentDetail(String shipmentIdStr) {
        long id;
        try {
            id = Long.parseLong(shipmentIdStr);
        } catch (NumberFormatException e) {
            return Optional.empty();
        }
        ShipmentEntity entity = shipmentMapper.selectById(id);
        if (entity == null) {
            return Optional.empty();
        }
        return Optional.of(toDetail(entity));
    }

    /**
     * 列建店異常。
     */
    public ListSellerShipmentSettingsForAdminData listSettings(ListSellerShipmentSettingsForAdminRequest req) {
        int pageNum = parseIntOrDefault(req.getPage(), 1);
        int pageSize = Math.min(parseIntOrDefault(req.getPageSize(), 20), MAX_PAGE_SIZE);
        boolean abnormalOnly = req.getAbnormalOnly();
        String providerStatus = req.getProviderStatus();
        String onboardingStatusCsv = req.getOnboardingStatus();
        String shippingType = req.getShippingType();
        String storeIdStr = req.getStoreId();
        LocalDateTime createdFrom = parseIso(req.getCreatedFrom());
        LocalDateTime createdTo = parseIso(req.getCreatedTo());
        String sort = normalizeSort(req.getSort());

        LambdaQueryWrapper<SellerShipmentSettingEntity> q = new LambdaQueryWrapper<>();

        if (abnormalOnly) {
            q.eq(SellerShipmentSettingEntity::getOnboardingStatus, OnboardingStatus.REJECTED.value());
        }

        if (StringUtils.hasText(providerStatus)) {
            q.eq(SellerShipmentSettingEntity::getProviderStatus, providerStatus);
        }
        if (StringUtils.hasText(onboardingStatusCsv)) {
            List<String> list = Arrays.asList(onboardingStatusCsv.split(","));
            q.in(SellerShipmentSettingEntity::getOnboardingStatus, list);
        }
        if (StringUtils.hasText(shippingType)) {
            q.eq(SellerShipmentSettingEntity::getShippingType, shippingType);
        }
        if (StringUtils.hasText(storeIdStr)) {
            try {
                q.eq(SellerShipmentSettingEntity::getStoreId, Long.parseLong(storeIdStr));
            } catch (NumberFormatException ignore) {}
        }
        if (createdFrom != null) {
            q.ge(SellerShipmentSettingEntity::getCreatedAt, createdFrom);
        }
        if (createdTo != null) {
            q.lt(SellerShipmentSettingEntity::getCreatedAt, createdTo);
        }

        applySettingSort(q, sort);

        Page<SellerShipmentSettingEntity> page = new Page<>(pageNum, pageSize);
        var result = settingMapper.selectPage(page, q);

        ListSellerShipmentSettingsForAdminData.Builder builder = ListSellerShipmentSettingsForAdminData.newBuilder()
                .setTotal(String.valueOf(result.getTotal()))
                .setPage(String.valueOf(result.getCurrent()))
                .setPageSize(String.valueOf(result.getSize()));

        for (SellerShipmentSettingEntity e : result.getRecords()) {
            builder.addRecords(toSettingSummary(e));
        }
        return builder.build();
    }

    // ---- 轉換 ----

    private AdminShipmentSummary toSummary(ShipmentEntity e) {
        return AdminShipmentSummary.newBuilder()
                .setShipmentId(e.getId() == null ? "" : String.valueOf(e.getId()))
                .setShipmentNo(nullToEmpty(e.getShipmentNo()))
                .setOrderNo(nullToEmpty(e.getOrderNo()))
                .setStoreId(e.getStoreId() == null ? "" : String.valueOf(e.getStoreId()))
                .setShippingMethod(nullToEmpty(e.getShippingMethod()))
                .setIsReturn(Boolean.TRUE.equals(e.getIsReturn()) ? "true" : "false")
                .setStatus(nullToEmpty(e.getStatus()))
                .setProviderStatus(nullToEmpty(e.getProviderStatus()))
                .setProviderMessage(nullToEmpty(e.getProviderMessage()))
                .setProviderErrorCode(nullToEmpty(e.getProviderErrorCode()))
                .setProviderErrorMessage(nullToEmpty(e.getProviderErrorMessage()))
                .setShipTradeNo(nullToEmpty(e.getShipTradeNo()))
                .setMerId(nullToEmpty(e.getMerId()))
                .setCreatedAt(formatIso(e.getCreatedAt()))
                .setUpdatedAt(formatIso(e.getUpdatedAt()))
                .build();
    }

    private AdminShipmentDetail toDetail(ShipmentEntity e) {
        AdminShipmentDetail.Builder b = AdminShipmentDetail.newBuilder()
                .setShipmentId(e.getId() == null ? "" : String.valueOf(e.getId()))
                .setShipmentNo(nullToEmpty(e.getShipmentNo()))
                .setOrderNo(nullToEmpty(e.getOrderNo()))
                .setStoreId(e.getStoreId() == null ? "" : String.valueOf(e.getStoreId()))
                .setBuyerId(e.getBuyerId() == null ? "" : String.valueOf(e.getBuyerId()))
                .setShippingMethod(nullToEmpty(e.getShippingMethod()))
                .setIsReturn(Boolean.TRUE.equals(e.getIsReturn()) ? "true" : "false")
                .setStatus(nullToEmpty(e.getStatus()))
                .setProviderStatus(nullToEmpty(e.getProviderStatus()))
                .setProviderMessage(nullToEmpty(e.getProviderMessage()))
                .setProviderStatusTime(formatIso(e.getProviderStatusTime()))
                .setProviderErrorCode(nullToEmpty(e.getProviderErrorCode()))
                .setProviderErrorMessage(nullToEmpty(e.getProviderErrorMessage()))
                .setProviderErrorAt(formatIso(e.getProviderErrorAt()))
                .setShipTradeNo(nullToEmpty(e.getShipTradeNo()))
                .setMerId(nullToEmpty(e.getMerId()))
                .setShipType(nullToEmpty(e.getShipType()))
                .setLgsType(nullToEmpty(e.getLgsType()))
                .setGoodsType(nullToEmpty(e.getGoodsType()))
                .setSpec(nullToEmpty(e.getSpec()))
                .setTradeAmt(e.getTradeAmt() == null ? "" : String.valueOf(e.getTradeAmt()))
                .setSenderName(nullToEmpty(e.getSenderName()))
                .setSenderPhone(nullToEmpty(e.getSenderPhone()))
                .setRecipientName(nullToEmpty(e.getRecipientName()))
                .setRecipientPhone(nullToEmpty(e.getRecipientPhone()))
                .setRecipientAddress(nullToEmpty(e.getRecipientAddress()))
                .setCvsStoreCode(nullToEmpty(e.getCvsStoreCode()))
                .setStoreName(nullToEmpty(e.getStoreName()))
                .setPartnerId(nullToEmpty(e.getPartnerId()))
                .setValidationNo(nullToEmpty(e.getValidationNo()))
                .setOdno(nullToEmpty(e.getOdno()))
                .setLabelPrintStatus(nullToEmpty(e.getLabelPrintStatus()))
                .setTcatCallStatus(nullToEmpty(e.getTcatCallStatus()))
                .setTcatCallTime(formatIso(e.getTcatCallTime()))
                // 49 §6.6：只填 objectKey，Gateway 組 labelUrl
                .setLabelKey(e.getLabelObjectKey() != null ? e.getLabelObjectKey() : "")
                .setLastReconciledAt(formatIso(e.getLastReconciledAt()))
                .setCreatedAt(formatIso(e.getCreatedAt()))
                .setUpdatedAt(formatIso(e.getUpdatedAt()));

        if (e.getShipmentPhotos() != null) {
            for (ShipmentEntity.ShipmentPhoto p : e.getShipmentPhotos()) {
                if (p != null && p.getObjectKey() != null) {
                    b.addShipmentPhotoKeys(p.getObjectKey()); // 49 §6.6：只填 key，Gateway 組 URL
                }
            }
        }
        return b.build();
    }

    private AdminSellerShipmentSettingSummary toSettingSummary(SellerShipmentSettingEntity e) {
        return AdminSellerShipmentSettingSummary.newBuilder()
                .setSettingId(e.getId() == null ? "" : String.valueOf(e.getId()))
                .setStoreId(e.getStoreId() == null ? "" : String.valueOf(e.getStoreId()))
                .setShippingType(nullToEmpty(e.getShippingType()))
                .setGoodsType(nullToEmpty(e.getGoodsType()))
                .setOnboardingStatus(nullToEmpty(e.getOnboardingStatus()))
                .setProviderStatus(nullToEmpty(e.getProviderStatus()))
                .setProviderMessage(nullToEmpty(e.getProviderMessage()))
                .setMerId(nullToEmpty(e.getMerId()))
                .setPartnerId(nullToEmpty(e.getPartnerId()))
                .setEshipId(nullToEmpty(e.getEshipId()))
                .setPayuniSubmitStatus(nullToEmpty(e.getPayuniSubmitStatus()))
                .setEnabled(Boolean.TRUE.equals(e.getEnabled()) ? "true" : "false")
                .setCreatedAt(formatIso(e.getCreatedAt()))
                .setUpdatedAt(formatIso(e.getUpdatedAt()))
                .build();
    }

    // ---- 排序 ----

    private String normalizeSort(String sort) {
        if (!StringUtils.hasText(sort)) return "updated_at_desc";
        String s = sort.trim().toLowerCase();
        return switch (s) {
            case "updated_at_desc", "updated_at_asc", "created_at_desc", "created_at_asc" -> s;
            default -> "updated_at_desc";
        };
    }

    private void applySort(LambdaQueryWrapper<ShipmentEntity> q, String sort) {
        switch (sort) {
            case "updated_at_asc" -> q.orderByAsc(ShipmentEntity::getUpdatedAt);
            case "created_at_desc" -> q.orderByDesc(ShipmentEntity::getCreatedAt);
            case "created_at_asc" -> q.orderByAsc(ShipmentEntity::getCreatedAt);
            default -> q.orderByDesc(ShipmentEntity::getUpdatedAt);
        }
    }

    private void applySettingSort(LambdaQueryWrapper<SellerShipmentSettingEntity> q, String sort) {
        switch (sort) {
            case "updated_at_asc" -> q.orderByAsc(SellerShipmentSettingEntity::getUpdatedAt);
            case "created_at_desc" -> q.orderByDesc(SellerShipmentSettingEntity::getCreatedAt);
            case "created_at_asc" -> q.orderByAsc(SellerShipmentSettingEntity::getCreatedAt);
            default -> q.orderByDesc(SellerShipmentSettingEntity::getUpdatedAt);
        }
    }

    // ---- 小工具 ----

    private int parseIntOrDefault(String s, int fallback) {
        if (!StringUtils.hasText(s)) return fallback;
        try {
            int v = Integer.parseInt(s.trim());
            return v > 0 ? v : fallback;
        } catch (NumberFormatException e) {
            return fallback;
        }
    }

    private LocalDateTime parseIso(String s) {
        if (!StringUtils.hasText(s)) return null;
        try {
            return DateTimeUtils.parseIso8601(s);
        } catch (Exception e) {
            log.warn("shipping.admin.parseIso.fail", s);
            return null;
        }
    }

    /** ISO 8601 UTC with Z suffix（docs/開發規範/Java 微服務開發規範.md §2.3）。 */
    private String formatIso(LocalDateTime t) {
        return DateTimeUtils.formatIso8601(t);
    }

    private String nullToEmpty(String s) {
        return s == null ? "" : s;
    }
}
