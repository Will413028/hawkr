package io.github.will413028.hawkr.shipping.service.uber;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.shipping.dto.UberDeliveryRow;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.entity.ShipmentStatusLogEntity;
import io.github.will413028.hawkr.shipping.entity.UberPodArtifactEntity;
import io.github.will413028.hawkr.shipping.entity.UberQuoteEntity;
import io.github.will413028.hawkr.shipping.grpc.ActionAvailability;
import io.github.will413028.hawkr.shipping.grpc.AdminUberDeliveryDetail;
import io.github.will413028.hawkr.shipping.grpc.ListSellerUberDeliveriesData;
import io.github.will413028.hawkr.shipping.grpc.ListSellerUberDeliveriesRequest;
import io.github.will413028.hawkr.shipping.grpc.ListUberDeliveriesForAdminData;
import io.github.will413028.hawkr.shipping.grpc.ListUberDeliveriesForAdminRequest;
import io.github.will413028.hawkr.shipping.grpc.SellerUberDeliveryDetail;
import io.github.will413028.hawkr.shipping.grpc.UberDeliveryQuoteView;
import io.github.will413028.hawkr.shipping.grpc.UberDeliverySummary;
import io.github.will413028.hawkr.shipping.grpc.UberDeliveryTimelineEntry;
import io.github.will413028.hawkr.shipping.grpc.UberPodArtifactView;
import io.github.will413028.hawkr.shipping.mapper.ShipmentMapper;
import io.github.will413028.hawkr.shipping.mapper.ShipmentStatusLogMapper;
import io.github.will413028.hawkr.shipping.mapper.UberDeliveryQueryMapper;
import io.github.will413028.hawkr.shipping.mapper.UberPodArtifactMapper;
import io.github.will413028.hawkr.shipping.mapper.UberQuoteMapper;
import io.github.will413028.hawkr.shipping.provider.action.AllowedActionResolver;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;

import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.Optional;

/**
 * Uber 配送查詢服務（66；Stage 8B W6-3 賣家面／W6-4 Admin 面）。
 *
 * <h2>這個類別存在之前發生了什麼</h2>
 * {@code uber_deliveries}／{@code uber_quotes}／{@code uber_pod_artifacts} 三張表（V13）
 * 有完整的欄位與資料，而 {@code grpc/} 底下對 {@code UberDeliveryEntity} 的引用是 <b>0</b>
 * ——<b>資料在、零出口</b>。賣家看不到自己的 Uber 配送、Admin 查不到任何一筆，
 * 而 UI 畫不出後端沒有的東西。
 *
 * <p>純讀取，無寫入副作用。歸屬（ownership）不在本類別而在
 * {@link UberDeliveryQueryMapper#SELLER_OWNERSHIP_PREDICATE}——見該處說明。
 */
@Service
public class UberDeliveryQueryService {

    private static final ObsLogger log = ObsLogger.forClass(UberDeliveryQueryService.class);

    private static final int MAX_PAGE_SIZE = 100;

    /**
     * timeline 單次回傳上限。
     *
     * <p>⚠️ 它是<b>截斷</b>不是「全部」：Uber 的 {@code courier_update} 每 20 秒一發（U-E），
     * 一趟一小時的配送就可能累積上百列。無上限會讓單一 Admin 請求把整段歷程拉進記憶體，
     * 而那正是最不該在客訴當下失敗的請求。取最新 {@value} 列（回傳前再轉回時間正序）。
     */
    private static final int MAX_TIMELINE_ENTRIES = 200;

    /**
     * 狀態歷程的主體軸。
     *
     * <p>🔴 共用表最致命的失效模式是「讀到的是另一條軸的軌跡」（V11 ⑥ 的原話）。
     * 68 引入 {@code SHIPMENT_BATCH} 之後，不帶這個條件的查詢會把批次軸的列混進單筆 timeline，
     * 而那完全無聲——列數變多看起來像「資料比較完整」。
     */
    private static final String SUBJECT_SHIPMENT = "SHIPMENT";

    private final UberDeliveryQueryMapper queryMapper;
    private final UberQuoteMapper quoteMapper;
    private final UberPodArtifactMapper podArtifactMapper;
    private final ShipmentMapper shipmentMapper;
    private final ShipmentStatusLogMapper statusLogMapper;
    private final AllowedActionResolver allowedActionResolver;

    public UberDeliveryQueryService(UberDeliveryQueryMapper queryMapper,
                                    UberQuoteMapper quoteMapper,
                                    UberPodArtifactMapper podArtifactMapper,
                                    ShipmentMapper shipmentMapper,
                                    ShipmentStatusLogMapper statusLogMapper,
                                    AllowedActionResolver allowedActionResolver) {
        this.queryMapper = queryMapper;
        this.quoteMapper = quoteMapper;
        this.podArtifactMapper = podArtifactMapper;
        this.shipmentMapper = shipmentMapper;
        this.statusLogMapper = statusLogMapper;
        this.allowedActionResolver = allowedActionResolver;
    }

    // ==================== 賣家面 ====================

    /** 列賣家自己的 Uber 配送。歸屬由 SQL 述詞強制，本方法只轉交 {@code store_id}。 */
    public ListSellerUberDeliveriesData listSellerDeliveries(ListSellerUberDeliveriesRequest req) {
        int page = parseIntOrDefault(req.getPage(), 1);
        int pageSize = Math.min(parseIntOrDefault(req.getPageSize(), 20), MAX_PAGE_SIZE);
        List<String> statusList = splitCsv(req.getStatus());
        Boolean isReturn = parseTriState(req.getIsReturn());
        LocalDateTime from = parseIso(req.getCreatedFrom());
        LocalDateTime to = parseIso(req.getCreatedTo());
        String sort = normalizeSort(req.getSort());

        long total = queryMapper.countSellerDeliveries(req.getStoreId(), statusList,
                emptyToNull(req.getProviderStatus()), emptyToNull(req.getOrderNo()), isReturn,
                emptyToNull(req.getOutcome()), from, to);

        List<UberDeliveryRow> rows = queryMapper.selectSellerDeliveries(req.getStoreId(), statusList,
                emptyToNull(req.getProviderStatus()), emptyToNull(req.getOrderNo()), isReturn,
                emptyToNull(req.getOutcome()), from, to, sort, pageSize, (page - 1) * pageSize);

        ListSellerUberDeliveriesData.Builder b = ListSellerUberDeliveriesData.newBuilder()
                .setTotal(String.valueOf(total))
                .setPage(String.valueOf(page))
                .setPageSize(String.valueOf(pageSize));
        for (UberDeliveryRow row : rows) {
            b.addRecords(toSummary(row));
        }
        return b.build();
    }

    /**
     * 賣家看單筆。
     *
     * <p>🔴 「不存在」與「不是你的」在這裡回同一個 {@link Optional#empty()}——
     * 能區分的話，賣家就得到一個可列舉全平台配送是否存在的 oracle。
     */
    public Optional<SellerUberDeliveryDetail> getSellerDelivery(long storeId, String deliveryNo) {
        if (!StringUtils.hasText(deliveryNo)) {
            return Optional.empty();
        }
        UberDeliveryRow row = queryMapper.selectSellerDeliveryDetail(storeId, deliveryNo);
        if (row == null) {
            return Optional.empty();
        }
        SellerUberDeliveryDetail.Builder b = SellerUberDeliveryDetail.newBuilder()
                .setSummary(toSummary(row))
                // 🔴 只有遮罩值。DB 裡沒有明碼可回（66 §5.1.5：原值不落庫），
                //    要回明碼就得先把明碼寫進 DB——本方法不是能做那個決定的地方。
                .setCourierPhoneMasked(nullToEmpty(row.getCourierPhoneMasked()))
                .setCourierLocationJson(nullToEmpty(row.getCourierLocation()))
                .setRelatedDeliveriesJson(nullToEmpty(row.getRelatedDeliveries()))
                .addAllPodArtifacts(loadPodArtifacts(row.getDeliveryId()))
                .addAllAllowedActions(resolveActions(row.getShipmentId()));
        loadQuote(row.getQuoteId()).ifPresent(b::setQuote);
        return Optional.of(b.build());
    }

    // ==================== Admin 面 ====================

    /** Admin 列平台全部 Uber 配送（W6-4 的九個篩選維度）。 */
    public ListUberDeliveriesForAdminData listAdminDeliveries(ListUberDeliveriesForAdminRequest req) {
        int page = parseIntOrDefault(req.getPage(), 1);
        int pageSize = Math.min(parseIntOrDefault(req.getPageSize(), 20), MAX_PAGE_SIZE);
        Long storeId = parseLongOrNull(req.getStoreId());
        List<String> statusList = splitCsv(req.getStatus());
        LocalDateTime from = parseIso(req.getCreatedFrom());
        LocalDateTime to = parseIso(req.getCreatedTo());
        String sort = normalizeSort(req.getSort());

        long total = queryMapper.countAdminDeliveries(storeId, emptyToNull(req.getShipmentNo()),
                emptyToNull(req.getDeliveryId()), statusList, emptyToNull(req.getProviderStatus()),
                emptyToNull(req.getOrderNo()), emptyToNull(req.getOutcome()),
                req.getFailureOnly(), req.getUnknownOnly(), from, to);

        List<UberDeliveryRow> rows = queryMapper.selectAdminDeliveries(storeId,
                emptyToNull(req.getShipmentNo()), emptyToNull(req.getDeliveryId()), statusList,
                emptyToNull(req.getProviderStatus()), emptyToNull(req.getOrderNo()),
                emptyToNull(req.getOutcome()), req.getFailureOnly(), req.getUnknownOnly(),
                from, to, sort, pageSize, (page - 1) * pageSize);

        ListUberDeliveriesForAdminData.Builder b = ListUberDeliveriesForAdminData.newBuilder()
                .setTotal(String.valueOf(total))
                .setPage(String.valueOf(page))
                .setPageSize(String.valueOf(pageSize));
        for (UberDeliveryRow row : rows) {
            b.addRecords(toSummary(row));
        }
        return b.build();
    }

    /** Admin 看單筆，再加 timeline（含 provider 原始 payload）。 */
    public Optional<AdminUberDeliveryDetail> getAdminDelivery(String deliveryNo) {
        if (!StringUtils.hasText(deliveryNo)) {
            return Optional.empty();
        }
        UberDeliveryRow row = queryMapper.selectAdminDeliveryDetail(deliveryNo);
        if (row == null) {
            return Optional.empty();
        }
        AdminUberDeliveryDetail.Builder b = AdminUberDeliveryDetail.newBuilder()
                .setSummary(toSummary(row))
                .setBuyerId(row.getBuyerId() == null ? "" : String.valueOf(row.getBuyerId()))
                .setCourierPhoneMasked(nullToEmpty(row.getCourierPhoneMasked()))
                .setCourierLocationJson(nullToEmpty(row.getCourierLocation()))
                .setRelatedDeliveriesJson(nullToEmpty(row.getRelatedDeliveries()))
                .addAllPodArtifacts(loadPodArtifacts(row.getDeliveryId()))
                .addAllAllowedActions(resolveActions(row.getShipmentId()))
                .addAllTimeline(loadTimeline(row.getShipmentId()));
        loadQuote(row.getQuoteId()).ifPresent(b::setQuote);
        return Optional.of(b.build());
    }

    // ==================== 組裝 ====================

    private UberDeliverySummary toSummary(UberDeliveryRow row) {
        return UberDeliverySummary.newBuilder()
                .setDeliveryNo(nullToEmpty(row.getDeliveryNo()))
                .setDeliveryId(nullToEmpty(row.getDeliveryId()))
                .setShipmentId(row.getShipmentId() == null ? "" : String.valueOf(row.getShipmentId()))
                .setShipmentNo(nullToEmpty(row.getShipmentNo()))
                .setOrderNo(nullToEmpty(row.getOrderNo()))
                .setStoreId(row.getStoreId() == null ? "" : String.valueOf(row.getStoreId()))
                .setShippingMethod(nullToEmpty(row.getShippingMethod()))
                .setIsReturn(Boolean.TRUE.equals(row.getIsReturn()) ? "true" : "false")
                .setStatus(nullToEmpty(row.getStatus()))
                .setStatusLabel(resolveStatusLabel(row.getStatus()))
                // 🔴 raw：與上面的 status 是兩個欄位，不得互相填充。
                .setProviderStatus(nullToEmpty(row.getProviderStatus()))
                .setOutcome(nullToEmpty(row.getOutcome()))
                .setUndeliverableReason(nullToEmpty(row.getUndeliverableReason()))
                .setCancelationReason(nullToEmpty(row.getCancelationReason()))
                .setEtaDropoffAt(formatIso(row.getEtaDropoffAt()))
                .setCourierName(nullToEmpty(row.getCourierName()))
                .setCourierImminent(Boolean.TRUE.equals(row.getCourierImminent()))
                .setProviderErrorCode(nullToEmpty(row.getProviderErrorCode()))
                .setProviderErrorMessage(nullToEmpty(row.getProviderErrorMessage()))
                .setProviderErrorAt(formatIso(row.getProviderErrorAt()))
                .setQuoteId(nullToEmpty(row.getQuoteId()))
                .setPodArtifactCount(String.valueOf(
                        row.getPodArtifactCount() == null ? 0L : row.getPodArtifactCount()))
                .setCreatedAt(formatIso(row.getCreatedAt()))
                .setUpdatedAt(formatIso(row.getUpdatedAt()))
                .build();
    }

    private Optional<UberDeliveryQuoteView> loadQuote(String quoteId) {
        if (!StringUtils.hasText(quoteId)) {
            return Optional.empty();
        }
        UberQuoteEntity q = quoteMapper.selectOne(new LambdaQueryWrapper<UberQuoteEntity>()
                .eq(UberQuoteEntity::getQuoteId, quoteId));
        if (q == null) {
            return Optional.empty();
        }
        return Optional.of(UberDeliveryQuoteView.newBuilder()
                .setQuoteId(nullToEmpty(q.getQuoteId()))
                // 元整數字串（禁 BigDecimal／Double）
                .setFeeAmount(q.getFeeAmount() == null ? "" : String.valueOf(q.getFeeAmount()))
                .setCurrency(nullToEmpty(q.getCurrency()))
                .setStatus(nullToEmpty(q.getStatus()))
                .setExpiresAt(formatIso(q.getExpiresAt()))
                .setPackingPolicyVersion(q.getPackingPolicyVersion() == null
                        ? "" : String.valueOf(q.getPackingPolicyVersion()))
                .setCreatedAt(formatIso(q.getCreatedAt()))
                .build());
    }

    /**
     * 送達憑證。
     *
     * <p>🔴 {@code pincodeValue} <b>刻意不進回應</b>：它是「持有即可領取包裹」的憑證
     * （§10.3：憑證問的是「拿到它能做什麼」，不是「這是誰的個資」）。判斷 POD 在不在
     * 不需要它，而一旦外露就收不回來。
     */
    private List<UberPodArtifactView> loadPodArtifacts(String deliveryId) {
        if (!StringUtils.hasText(deliveryId)) {
            return List.of();
        }
        List<UberPodArtifactEntity> artifacts = podArtifactMapper.selectList(
                new LambdaQueryWrapper<UberPodArtifactEntity>()
                        .eq(UberPodArtifactEntity::getDeliveryId, deliveryId)
                        .orderByAsc(UberPodArtifactEntity::getArtifactType));
        List<UberPodArtifactView> out = new ArrayList<>(artifacts.size());
        for (UberPodArtifactEntity a : artifacts) {
            boolean available = a.getFetchedAt() != null && StringUtils.hasText(a.getObjectKey());
            out.add(UberPodArtifactView.newBuilder()
                    .setArtifactType(nullToEmpty(a.getArtifactType()))
                    // 🔴 只給 objectKey（secure/ 前綴），不組 URL——理由見 proto 註解。
                    .setObjectKey(nullToEmpty(a.getObjectKey()))
                    .setFetchedAt(formatIso(a.getFetchedAt()))
                    .setFetchAttempts(String.valueOf(
                            a.getFetchAttempts() == null ? 0 : a.getFetchAttempts()))
                    .setAvailable(available)
                    .build());
        }
        return out;
    }

    /**
     * 狀態歷程（Admin 專屬）。
     *
     * <p>取<b>最新</b> {@value #MAX_TIMELINE_ENTRIES} 列後轉回時間正序：被截掉的必須是最舊的那些，
     * 反過來會讓「現在發生什麼事」這個唯一有時效的問題永遠看不到答案。
     */
    private List<UberDeliveryTimelineEntry> loadTimeline(Long shipmentId) {
        if (shipmentId == null) {
            return List.of();
        }
        Page<ShipmentStatusLogEntity> page = new Page<>(1, MAX_TIMELINE_ENTRIES);
        List<ShipmentStatusLogEntity> logs = statusLogMapper.selectPage(page,
                new LambdaQueryWrapper<ShipmentStatusLogEntity>()
                        .eq(ShipmentStatusLogEntity::getSubjectType, SUBJECT_SHIPMENT)
                        .eq(ShipmentStatusLogEntity::getShipmentId, shipmentId)
                        .orderByDesc(ShipmentStatusLogEntity::getCreatedAt)
                        .orderByDesc(ShipmentStatusLogEntity::getId)).getRecords();

        List<UberDeliveryTimelineEntry> out = new ArrayList<>(logs.size());
        for (int i = logs.size() - 1; i >= 0; i--) {
            ShipmentStatusLogEntity e = logs.get(i);
            out.add(UberDeliveryTimelineEntry.newBuilder()
                    .setFromStatus(nullToEmpty(e.getFromStatus()))
                    .setToStatus(nullToEmpty(e.getToStatus()))
                    .setProviderStatus(nullToEmpty(e.getProviderStatus()))
                    .setProviderMessage(nullToEmpty(e.getProviderMessage()))
                    .setActorType(nullToEmpty(e.getActorType()))
                    .setTriggerSource(nullToEmpty(e.getTriggerSource()))
                    .setCorrelationId(nullToEmpty(e.getCorrelationId()))
                    .setRawPayload(nullToEmpty(e.getRawPayload()))
                    .setOccurredAt(formatIso(e.getCreatedAt()))
                    .build());
        }
        return out;
    }

    /**
     * 領域動作可用性。
     *
     * <p>🔴 走的是<b>與訂單詳情同一個</b> {@link AllowedActionResolver}——第二份判定就是第二份
     * 業務矩陣，而兩份矩陣遲早會對不起來（65 §4.4.1a 的整段教訓）。
     *
     * <p>⚠️ 這裡的 proto 轉換與 {@code ShippingGrpcServiceImpl.toProtoActions} 形狀相同但無法共用：
     * 那支是 {@code grpc} 套件的 private static，而本類別在 {@code service.uber}。
     * 兩者都只是欄位搬運，真正的判定在 resolver 一份。
     */
    private List<ActionAvailability> resolveActions(Long shipmentId) {
        if (shipmentId == null) {
            return List.of();
        }
        ShipmentEntity shipment = shipmentMapper.selectById(shipmentId);
        if (shipment == null) {
            log.warn("UberDeliveryQueryService.resolveActions.shipmentMissing",
                    "uber 配送查得到、對應 shipment 查不到（INV-U3 應保證 1:1）",
                    "shipmentId", String.valueOf(shipmentId));
            return List.of();
        }
        return allowedActionResolver.resolve(shipment).stream()
                .map(a -> ActionAvailability.newBuilder()
                        .setAction(a.action())
                        .setEnabled(a.enabled())
                        .setDisabledReasonCode(a.disabledReasonCode() == null
                                ? "" : String.valueOf(a.disabledReasonCode()))
                        .addAllActorAccountTypes(a.actorAccountTypes())
                        .build())
                .toList();
    }

    // ==================== 小工具 ====================

    /** 未知 value 回空字串，避免 {@code fromValue} 對髒資料拋例外把整個查詢打掉。 */
    private String resolveStatusLabel(String statusValue) {
        if (statusValue == null || !ShippingStatus.isValidValue(statusValue)) {
            return "";
        }
        return ShippingStatus.fromValue(statusValue).label();
    }

    /** {@code ""} / {@code "true"} / {@code "false"} 三態：空字串表「不過濾」而不是 false。 */
    private Boolean parseTriState(String raw) {
        if (!StringUtils.hasText(raw)) {
            return null;
        }
        if ("true".equalsIgnoreCase(raw)) {
            return Boolean.TRUE;
        }
        if ("false".equalsIgnoreCase(raw)) {
            return Boolean.FALSE;
        }
        return null;
    }

    private List<String> splitCsv(String csv) {
        if (!StringUtils.hasText(csv)) {
            return List.of();
        }
        return Arrays.stream(csv.split(","))
                .map(String::trim)
                .filter(StringUtils::hasText)
                .toList();
    }

    private String normalizeSort(String sort) {
        if (!StringUtils.hasText(sort)) {
            return "updated_at_desc";
        }
        String s = sort.trim().toLowerCase();
        return switch (s) {
            case "updated_at_desc", "updated_at_asc", "created_at_desc", "created_at_asc" -> s;
            default -> "updated_at_desc";
        };
    }

    private int parseIntOrDefault(String s, int fallback) {
        if (!StringUtils.hasText(s)) {
            return fallback;
        }
        try {
            int v = Integer.parseInt(s.trim());
            return v > 0 ? v : fallback;
        } catch (NumberFormatException e) {
            return fallback;
        }
    }

    /** 非法值回 null＝不過濾（與 {@code ShippingAdminQueryService} 對 store_id 的既有處置一致）。 */
    private Long parseLongOrNull(String s) {
        if (!StringUtils.hasText(s)) {
            return null;
        }
        try {
            return Long.parseLong(s.trim());
        } catch (NumberFormatException e) {
            return null;
        }
    }

    private LocalDateTime parseIso(String s) {
        if (!StringUtils.hasText(s)) {
            return null;
        }
        try {
            return DateTimeUtils.parseIso8601(s);
        } catch (Exception e) {
            log.warn("UberDeliveryQueryService.parseIso.fail", s);
            return null;
        }
    }

    private String formatIso(LocalDateTime t) {
        return DateTimeUtils.formatIso8601(t);
    }

    private String emptyToNull(String s) {
        return StringUtils.hasText(s) ? s : null;
    }

    private String nullToEmpty(String s) {
        return s == null ? "" : s;
    }
}
