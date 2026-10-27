package io.github.will413028.hawkr.shipping.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.providercall.ProviderCallBusyException;
import io.github.will413028.hawkr.common.providercall.ProviderCallContext;
import io.github.will413028.hawkr.common.providercall.ProviderCallExecutor;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.PayUniSubmitStatus;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingTemperature;
import io.github.will413028.hawkr.common.utils.validation.InputValidator;
import io.github.will413028.hawkr.common.utils.validation.PayUniShippingConstraints;
import io.github.will413028.hawkr.payuni.shipping.GoodsType;
import io.github.will413028.hawkr.payuni.shipping.LgsType;
import io.github.will413028.hawkr.payuni.shipping.ShipType;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.payuni.shipping.ShippingClient;
import io.github.will413028.hawkr.payuni.shipping.request.OnboardCvsRequest;
import io.github.will413028.hawkr.payuni.shipping.request.OnboardHomeRequest;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.shipping.response.OnboardShippingResponse;
import io.github.will413028.hawkr.seller.grpc.BatchGetSellerMetaRequest;
import io.github.will413028.hawkr.seller.grpc.BatchGetSellerMetaResponse;
import io.github.will413028.hawkr.seller.grpc.GetSellerShippingProfileResponse;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.github.will413028.hawkr.shipping.client.SellerGrpcClient;
import io.github.will413028.hawkr.common.utils.code.OnboardingLogAction;
import io.github.will413028.hawkr.common.utils.code.OnboardingLogResult;
import io.github.will413028.hawkr.common.utils.code.OnboardingStatus;
import io.github.will413028.hawkr.common.utils.code.ShippingType;
import io.github.will413028.hawkr.shipping.domain.OnboardingStatusRules;
import io.github.will413028.hawkr.shipping.entity.OnboardingLogEntity;
import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;
import io.github.will413028.hawkr.shipping.mapper.OnboardingLogMapper;
import io.github.will413028.hawkr.shipping.mapper.SellerShipmentSettingMapper;
import io.github.will413028.hawkr.shipping.provider.FulfillmentContext;
import io.github.will413028.hawkr.shipping.provider.FulfillmentProviderRegistry;
import io.github.will413028.hawkr.shipping.provider.capability.OnboardCapability;
import io.github.will413028.hawkr.shipping.util.AddressNormalizer;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import io.github.will413028.hawkr.shipping.service.dto.SellerShippingMethodSlot;

import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.function.Function;
import java.util.stream.Collectors;

@Service
public class OnboardingService {

    private static final ObsLogger log = ObsLogger.forClass(OnboardingService.class);

    private static final String SHIPPING_TYPE_CVS = ShippingType.CVS_C2C.value();
    private static final String SHIPPING_TYPE_HOME = ShippingType.HOME_DELIVERY.value();
    private static final String ACTION_PAYUNI = OnboardingLogAction.PAYUNI_SUBMIT.value();
    private static final String ACTION_LOCAL = OnboardingLogAction.LOCAL_UPDATE.value();
    private static final String RESULT_SUCCESS = OnboardingLogResult.SUCCESS.value();
    private static final String RESULT_FAILED = OnboardingLogResult.FAILED.value();
    private static final String CVS_LGS_TYPE = LgsType.C2C.code();
    private static final String CVS_SHIP_TYPE = ShipType.SEVEN.code();
    private static final String HOME_LGS_TYPE = LgsType.HOME.code();
    private static final String HOME_SHIP_TYPE = ShipType.TAQBIN.code();
    private static final int LABEL_NAME_MAX_LENGTH = 10;

    private final SellerShipmentSettingMapper settingMapper;
    private final OnboardingLogMapper logMapper;
    private final SellerGrpcClient sellerGrpcClient;
    private final ShippingClient shippingClient;
    private final ProviderCallExecutor providerCallExecutor;
    private final ShippingFeeService shippingFeeService;
    private final FulfillmentProviderRegistry fulfillmentProviders;

    @Value("${payuni.shipping.notify-url:}")
    private String notifyUrl;

    public OnboardingService(SellerShipmentSettingMapper settingMapper,
                             OnboardingLogMapper logMapper,
                             SellerGrpcClient sellerGrpcClient,
                             ShippingClient shippingClient,
                             ProviderCallExecutor providerCallExecutor,
                             ShippingFeeService shippingFeeService,
                             FulfillmentProviderRegistry fulfillmentProviders) {
        this.settingMapper = settingMapper;
        this.logMapper = logMapper;
        this.sellerGrpcClient = sellerGrpcClient;
        this.shippingClient = shippingClient;
        this.providerCallExecutor = providerCallExecutor;
        this.shippingFeeService = shippingFeeService;
        this.fulfillmentProviders = fulfillmentProviders;
    }

    // §2.4.4 transient vs permanent helper（與 ShippingService 同步維護）
    private static boolean isTransientPayUniError(Throwable ex) {
        if (!(ex instanceof io.github.will413028.hawkr.payuni.http.PayUniApiException papi)) return false;
        if ("IO_ERROR".equals(papi.getErrorCode())) return true;
        if (papi.getHttpStatus() >= 500) return true;
        String msg = papi.getMessage();
        if (msg == null) return false;
        String low = msg.toLowerCase();
        return msg.contains("處理中")
            || msg.contains("尚在處理")
            || low.contains("trying to access array offset")
            || low.contains("busy")
            || low.contains("timeout");
    }

    private static String extractProviderStatus(Throwable ex) {
        if (ex instanceof io.github.will413028.hawkr.payuni.http.PayUniApiException papi) {
            return papi.getErrorCode() != null ? papi.getErrorCode() : "";
        }
        return "";
    }

    private static String extractProviderMessage(Throwable ex) {
        return ex.getMessage() != null ? ex.getMessage() : ex.toString();
    }

    // ==================== 超商 C2C 建店 ====================

    /**
     * 超商 C2C 建店
     *
     * @param storeId            賣家 store 代理鍵
     * @param goodsType          貨物類型 value: 3201=常溫 / 3203=冷凍
     * @param returnStoreCode    退貨門市代碼（6 碼）
     * @param returnStoreName    退貨門市名稱（選填）
     * @param returnStoreAddress 退貨門市地址（選填）
     * @param senderName         寄件人/退貨聯絡人姓名（前端傳入）
     * @param senderPhone        寄件人/退貨聯絡人電話（前端傳入）
     */
    @Transactional
    public SellerShipmentSettingEntity onboardCvsShipping(long storeId,
                                                          String goodsType,
                                                          String returnStoreCode,
                                                          String returnStoreName,
                                                          String returnStoreAddress,
                                                          String senderName,
                                                          String senderPhone) {
        // 參數驗證
        String mappedGoodsType = mapGoodsType(goodsType);
        if (mappedGoodsType == null) {
            // 若 goodsType 是合法 enum value(僅 7-11 業務不支援該溫層,如冷藏)→ 用 label 提示;
            // 若連 enum value 都不合法 → 透傳原值供前端排錯
            String label;
            try {
                label = ShippingTemperature.fromValue(goodsType).label();
            } catch (Exception ignored) {
                label = goodsType;
            }
            throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_GOODS_TYPE_INVALID, label);
        }
        if (returnStoreCode == null || !returnStoreCode.matches("^\\d{6}$")) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_RETURN_STORE_CODE_INVALID);
        }
        // 寄件人姓名送 PayUni 超商建店 ContactName（顯示長度上限 10），源頭驗證避免開通失敗（#1187 同類）
        InputValidator.requireDisplayLengthBetween(senderName, 1,
                PayUniShippingConstraints.CVS_SENDER_NAME_MAX, "寄件人姓名");
        InputValidator.requireMobile(senderPhone, "寄件人電話");

        // 從 seller-service 取得 merId、sellerName（labelCompanyName 用）、contactEmail
        SellerMeta sellerMeta = getSellerMeta(storeId);
        String merId = sellerMeta.getPayuniMerId();
        String labelCompanyName = truncateToMaxLength(sellerMeta.getSellerName(), LABEL_NAME_MAX_LENGTH);
        // contactEmail 需從 getSellerShippingProfile 取得（SellerMeta 無此欄位）
        String contactEmail = getSellerContactEmail(storeId);

        // 查詢既有記錄
        SellerShipmentSettingEntity existing = findSetting(storeId, SHIPPING_TYPE_CVS, goodsType);

        if (existing != null) {
            return handleExistingSetting(existing, storeId, merId, contactEmail,
                    senderName, senderPhone,
                    returnStoreCode, returnStoreName, returnStoreAddress,
                    labelCompanyName, null,
                    null, null, null, null,
                    null, null, null, null, null, null);
        }

        // 首次建店：呼叫 PayUni
        return submitCvsToPayUni(storeId, merId, contactEmail, senderName, senderPhone,
                mappedGoodsType, goodsType,
                returnStoreCode, returnStoreName, returnStoreAddress, labelCompanyName);
    }

    // ==================== 宅配建店 ====================

    /**
     * 宅配建店
     *
     * @param storeId            賣家 store 代理鍵
     * @param productCategory    商品類別代碼（0001~0015）
     * @param senderName         寄件人姓名（前端傳入，必填）
     * @param senderPhone        寄件人電話（前端傳入，必填）
     * @param senderCity         寄件城市（前端傳入，必填）
     * @param senderDistrict     寄件地區（前端傳入，必填）
     * @param senderAddressDetail 寄件詳細地址（前端傳入，必填）
     * @param senderPostalCode   寄件郵遞區號（前端傳入，必填）
     * @param returnName         退貨收件人姓名（選填，空則複製寄件資訊）
     * @param returnPhone        退貨收件人電話（選填）
     * @param returnCity         退貨城市（選填）
     * @param returnDistrict     退貨地區（選填）
     * @param returnAddressDetail 退貨詳細地址（選填）
     * @param returnPostalCode   退貨郵遞區號（選填）
     */
    @Transactional
    public SellerShipmentSettingEntity onboardHomeDelivery(long storeId,
                                                            String productCategory,
                                                            String senderName,
                                                            String senderPhone,
                                                            String senderCity,
                                                            String senderDistrict,
                                                            String senderAddressDetail,
                                                            String senderPostalCode,
                                                            String returnName,
                                                            String returnPhone,
                                                            String returnCity,
                                                            String returnDistrict,
                                                            String returnAddressDetail,
                                                            String returnPostalCode) {
        // 參數驗證
        if (productCategory == null || !productCategory.matches("^\\d{4}$")) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_PRODUCT_CATEGORY_INVALID);
        }
        // 寄件人姓名送 PayUni 宅配建店 SenderName（顯示長度上限 30），源頭驗證避免開通失敗（#1187 同類）
        InputValidator.requireDisplayLengthBetween(senderName, 1,
                PayUniShippingConstraints.HOME_SENDER_NAME_MAX, "寄件人姓名");
        InputValidator.requireMobile(senderPhone, "寄件人電話");
        InputValidator.requireNotEmpty(senderCity, "寄件城市");
        InputValidator.requireNotEmpty(senderDistrict, "寄件地區");
        InputValidator.requireAddress(senderAddressDetail, "寄件詳細地址");
        InputValidator.requireNotEmpty(senderPostalCode, "寄件郵遞區號");

        // 退貨地址若為空，後端自動複製寄件地址
        // 退貨欄位選填：有填則驗證格式，未填則複製寄件資訊
        if (!isBlank(returnName)) {
            // 退貨收件人姓名送 PayUni 宅配退貨 SenderName（顯示長度上限 30），有值才驗
            InputValidator.requireDisplayLengthBetween(returnName, 1,
                    PayUniShippingConstraints.HOME_SENDER_NAME_MAX, "退貨收件人姓名");
        }
        if (!isBlank(returnPhone)) {
            InputValidator.requireMobile(returnPhone, "退貨聯絡人電話");
        }
        if (!isBlank(returnAddressDetail)) {
            InputValidator.requireAddress(returnAddressDetail, "退貨詳細地址");
        }
        String effectiveReturnName = isBlank(returnName) ? senderName : returnName;
        String effectiveReturnPhone = isBlank(returnPhone) ? senderPhone : returnPhone;
        String effectiveReturnCity = isBlank(returnCity) ? senderCity : returnCity;
        String effectiveReturnDistrict = isBlank(returnDistrict) ? senderDistrict : returnDistrict;
        String effectiveReturnAddressDetail = isBlank(returnAddressDetail) ? senderAddressDetail : returnAddressDetail;
        String effectiveReturnPostalCode = isBlank(returnPostalCode) ? senderPostalCode : returnPostalCode;

        // 從 seller-service 取得 merId
        SellerMeta sellerMeta = getSellerMeta(storeId);
        String merId = sellerMeta.getPayuniMerId();

        // 查詢既有記錄（宅配不分 goodsType）
        SellerShipmentSettingEntity existing = findSetting(storeId, SHIPPING_TYPE_HOME, null);

        if (existing != null) {
            return handleExistingSetting(existing, storeId, merId, null,
                    senderName, senderPhone,
                    null, null, null,
                    null, productCategory,
                    senderCity, senderDistrict, senderAddressDetail, senderPostalCode,
                    effectiveReturnName, effectiveReturnPhone,
                    effectiveReturnCity, effectiveReturnDistrict,
                    effectiveReturnAddressDetail, effectiveReturnPostalCode);
        }

        // 首次建店：呼叫 PayUni
        return submitHomeToPayUni(storeId, merId, productCategory,
                senderName, senderPhone, senderCity, senderDistrict, senderAddressDetail, senderPostalCode,
                effectiveReturnName, effectiveReturnPhone,
                effectiveReturnCity, effectiveReturnDistrict,
                effectiveReturnAddressDetail, effectiveReturnPostalCode);
    }

    // ==================== 查詢 ====================

    /**
     * 列出賣家所有建店設定
     */
    public List<SellerShipmentSettingEntity> listOnboardings(long storeId) {
        return settingMapper.selectList(
                new LambdaQueryWrapper<SellerShipmentSettingEntity>()
                        .eq(SellerShipmentSettingEntity::getStoreId, storeId)
                        .orderByDesc(SellerShipmentSettingEntity::getCreatedAt));
    }

    /**
     * 查詢單一建店設定（含權限驗證）
     */
    public SellerShipmentSettingEntity getOnboarding(long storeId, long onboardingId) {
        SellerShipmentSettingEntity entity = settingMapper.selectById(onboardingId);
        if (entity == null) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_ONBOARDING_NOT_FOUND);
        }
        if (!entity.getStoreId().equals(storeId)) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_ONBOARDING_VIEW_DENIED);
        }
        return entity;
    }

    // ==================== 賣家配送方式查詢 ====================

    /**
     * 固定 4 個配送方式槽位定義
     */
    private record SlotDef(String shippingMethod, String goodsType, String displayName,
                           String description, boolean onboardingRequired, String shippingType) {}

    private static final List<SlotDef> SLOT_DEFINITIONS = List.of(
            new SlotDef(ShippingMethod.TAQBIN.value(), null,
                    "黑貓宅急便", "宅配到府", true, SHIPPING_TYPE_HOME),
            new SlotDef(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value(), ShippingTemperature.AMBIENT.value(),
                    "7-11店到店（常溫）", "超商門市取貨", true, SHIPPING_TYPE_CVS),
            new SlotDef(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value(), ShippingTemperature.FROZEN.value(),
                    "7-11店到店（冷凍）", "超商門市取貨", true, SHIPPING_TYPE_CVS),
            new SlotDef(ShippingMethod.UBER_INSTANT.value(), null,
                    "Uber 即時配送", "即時外送到府", false, null)
    );

    /**
     * 查詢賣家所有可用配送方式（固定 4 槽位 + 建店狀態）
     */
    public List<SellerShippingMethodSlot> listSellerShippingMethods(long storeId) {
        // 一次查詢該賣家所有建店設定
        List<SellerShipmentSettingEntity> allSettings = settingMapper.selectList(
                new LambdaQueryWrapper<SellerShipmentSettingEntity>()
                        .eq(SellerShipmentSettingEntity::getStoreId, storeId));

        // 以 shippingType + goodsType 為 key 建立查找表
        Map<String, SellerShipmentSettingEntity> settingMap = allSettings.stream()
                .collect(Collectors.toMap(
                        s -> buildSlotKey(s.getShippingType(), s.getGoodsType()),
                        Function.identity(),
                        (a, b) -> a));  // 重複取第一筆

        List<SellerShippingMethodSlot> result = new ArrayList<>();
        for (SlotDef slot : SLOT_DEFINITIONS) {
            // 系統層級檢查:該 method 是否有任何 enabled config(如 Uber Phase 1 預留無 config row → 全系統暫停)
            // 任何 slot 不論是否需建店,系統層級不可用一律 available=false
            boolean systemAvailable = shippingFeeService.hasAnyEnabledConfig(slot.shippingMethod());

            if (!slot.onboardingRequired()) {
                // 不需建店(如 Uber): available 完全取決於系統層級
                result.add(new SellerShippingMethodSlot(
                        slot.shippingMethod(), orEmpty(slot.goodsType()),
                        slot.displayName(), slot.description(),
                        systemAvailable, false, "", null, ""));
                continue;
            }

            // 需建店:查找對應設定
            String key = buildSlotKey(slot.shippingType(), slot.goodsType());
            SellerShipmentSettingEntity setting = settingMap.get(key);

            if (setting == null) {
                // 尚未建店
                result.add(new SellerShippingMethodSlot(
                        slot.shippingMethod(), orEmpty(slot.goodsType()),
                        slot.displayName(), slot.description(),
                        false, true, "", null, ""));
            } else {
                // 系統可用 + onboarding ACTIVE + enabled 三者皆需為 true
                boolean available = systemAvailable
                        && OnboardingStatus.ACTIVE.value().equals(setting.getOnboardingStatus())
                        && Boolean.TRUE.equals(setting.getEnabled());
                String enabledStr = Boolean.TRUE.equals(setting.getEnabled()) ? "true" : "false";
                result.add(new SellerShippingMethodSlot(
                        slot.shippingMethod(), orEmpty(slot.goodsType()),
                        slot.displayName(), slot.description(),
                        available, true,
                        orEmpty(setting.getOnboardingStatus()),
                        setting.getId(), enabledStr));
            }
        }
        return result;
    }

    private String buildSlotKey(String shippingType, String goodsType) {
        return shippingType + ":" + (goodsType != null ? goodsType : "");
    }

    private String orEmpty(String value) {
        return value != null ? value : "";
    }

    /**
     * 供建單流程使用：查詢指定配送方式的 ACTIVE 設定。
     *
     * <p>🔴 <b>這個查詢的語意仍然是模糊的，{@code ORDER BY} 只讓它「確定」不是「正確」。</b>
     * {@code shipping_type} <b>不是</b>這張表的唯一鍵——唯一索引是
     * {@code (store_id, shipping_type, COALESCE(goods_type, ''))}
     * （{@code V1__init_shipping_tables.sql} 的 {@code idx_settings_store_type}，
     * V4 migration 更名），所以同一個 {@code shipping_type} 底下<b>合法地</b>有多列
     * （超商的常溫／冷凍各一列）。「這筆單該用哪一個溫層的寄件設定」這個問題
     * <b>在設計上沒有答案</b>：呼叫端只給配送方式、不給溫層。
     * 想真的回答它，得讓呼叫端把溫層一起帶進來（那時候用的是
     * {@code findSetting(storeId, shippingType, goodsType)}，它被唯一索引完全限定）。
     *
     * <p>🔴 <b>為什麼排序鍵是 {@code id}，而且是 ASC</b>：{@code id} 是 PK、NOT NULL、唯一
     * ⇒ 構成<b>全序</b> ⇒ 不論查詢計畫與物理順序怎麼變，{@code LIMIT 1} 的答案只有一個。
     * 方向取 ASC 而非 DESC 是因為 {@code id} 是 TSID（時間有序）：新增的溫層列拿到更大的 id
     * ⇒ ASC 之下<b>賣家多開一個溫層不會改掉既有流程原本選到的那一列</b>，DESC 會。
     *
     * <p>⚠️ <b>目前為什麼無害（dev ＋ prod 皆實測 2026-09-10）</b>：
     * <b>prod 側</b>整張表只有 <b>1 列</b>（1 個 store、{@code shipping_type=3652}），
     * 多列組數 <b>0</b>、{@code goods_type} 非 NULL 的宅配列 <b>0</b>
     * ⇒ prod 上根本沒有「要選哪一列」的問題，本改動在 prod 是<b>零影響</b>。
     * 鑑別力佐證（證明查詢打到對的地方而不是回空）：{@code current_db=hawkr_shipping}、
     * {@code tables_in_db=12}、{@code shipments=0}、shipping 的 {@code flyway_max=10}。
     * <b>dev 側</b>：
     * {@code ACTIVE + enabled} 下有 15 組 {@code (store_id, shipping_type)} 是多列（共 30 列），
     * <b>全部落在 {@code 3651}（CVS_C2C，常溫 3201 ＋ 冷凍 3203）</b>；
     * {@code 3652}（HOME_DELIVERY）的 27 列 {@code goods_type} 全為 NULL ⇒ 每店至多一列
     * ⇒ 宅配／黑貓路徑今天本來就是確定的。那 15 組在<b>每一個被消費的欄位</b>上零分歧：
     * {@code mer_id}／{@code partner_id}／{@code eship_id}／{@code sender_name}／
     * {@code sender_phone}／{@code sender_city}／{@code sender_district}／
     * {@code sender_address_detail}／{@code sender_postal_code}／{@code return_store_code}／
     * {@code return_store_name}／{@code label_company_name}。
     *
     * <p>⚠️ 但「零分歧」<b>只對 payload 欄位成立</b>，不是「選哪一列都一樣」。
     * 實查那 15 組逐欄算 {@code count(DISTINCT)>1} 的組數，分歧的欄位有<b>六個</b>：
     * {@code goods_type} 15（判別欄本身）、{@code id} 15（PK）、{@code created_at} 15、
     * {@code payuni_submitted_at} 12、{@code updated_at} 9、{@code provider_message} 1。
     * 後四個是稽核／狀態欄，不被 {@code ShippingService.buildSenderSnapshot} 或任何
     * {@code ShipmentStrategy} 讀取 ⇒ 不影響「送出去的內容不變」這個結論。
     * ⚠️ 這一段原本寫成「唯二有分歧的是 goods_type 與 id」，那是<b>沒有當場跑一次就寫下的窮舉宣稱</b>，
     * 由獨立驗收實查推翻。留著這句是為了讓下一個人知道：
     * 「唯二」「僅此」「全部」這類措辭，讀的人會直接信。
     * 而 {@code id} 會被
     * {@code ShippingService.buildSenderSnapshot} 寫進 {@code shipments.sender_snapshot}
     * 的 {@code onboardingId} ⇒ <b>選列不同，今天就已經產生不同的落庫資料</b>，
     * 只是那份差異不改變送給 provider 的內容。
     *
     * <p>🔴 <b>加上 {@code ORDER BY} 不是零變化，要說清楚變了什麼</b>（dev 實測 2026-09-10）：
     * 15 組多列裡有 <b>4 組</b>的選列與加 {@code ORDER BY} 之前<b>不同</b>（11 組相同）。
     * 那 4 組經 20 欄逐欄對拍，payload 全部相同（{@code IS NOT DISTINCT FROM} 皆為真），
     * 差的只有 {@code id} 與 {@code goods_type} ⇒ <b>送出去的內容不變、
     * 落庫的 {@code onboardingId} 對那 4 個賣家會換值</b>。
     * ⚠️ 而且「之前選哪一列」本身<b>沒有</b>保證——那只是當下的 heap 順序，
     * 所以「4 組」是一次取樣不是穩定性質，這正是要加 {@code ORDER BY} 的理由。
     *
     * <p>⚠️ <b>不要誤信一個相近但錯的因果</b>：建單寫進 {@code shipments.mer_id} 的值來自
     * {@code orderInfo.getMerId()}，<b>不是</b>這一列的 {@code mer_id}
     * （見 {@code CvsShipmentStrategy} / {@code HomeShipmentStrategy}）；{@code partner_id} /
     * {@code eship_id} 也只在建店寫路徑（{@code OnboardCapability} 的 outcome）被讀。
     * 這一列真正決定的是寄件人聯絡與地址欄位、{@code return_store_code}，
     * 以及上面那個 {@code onboardingId}。
     */
    public SellerShipmentSettingEntity findActiveSetting(long storeId, String shippingMethod) {
        String shippingType = mapShippingMethodToType(storeId, shippingMethod);
        if (shippingType == null) {
            return null;
        }
        return settingMapper.selectOne(
                new LambdaQueryWrapper<SellerShipmentSettingEntity>()
                        .eq(SellerShipmentSettingEntity::getStoreId, storeId)
                        .eq(SellerShipmentSettingEntity::getShippingType, shippingType)
                        .eq(SellerShipmentSettingEntity::getOnboardingStatus, OnboardingStatus.ACTIVE.value())
                        .eq(SellerShipmentSettingEntity::getEnabled, true)
                        // 選列確定化：見本方法 javadoc。MyBatis-Plus 的 last() 一律附加在 SQL 尾端，
                        // 所以與 orderBy 的呼叫先後無關，生成的都是 `... ORDER BY id ASC LIMIT 1`。
                        .orderByAsc(SellerShipmentSettingEntity::getId)
                        .last("LIMIT 1"));
    }

    /**
     * 寫路徑的「賣家當下意願防線」（06 §2.3.1）：查不到 ACTIVE 設定即拋
     * {@link GrpcResponseCode#SHIPMENT_SELLER_NOT_ONBOARDED}。
     *
     * <p>🔴 存在的理由是<b>去重</b>不是包裝：這三行守衛原本在
     * {@code printCvsLabel}／{@code batchPrintCvsLabel}／{@code generateHomeLabelPdf}／
     * {@code BatchHomeLabelService.batchGenerate} 各有一份副本，四份必須同時改才不會漂移，
     * 而沒有任何東西保證它們同步。
     *
     * <p>⚠️ 這裡的 {@code method} 是<b>呼叫端端點本身的配送方式</b>（值引用），不是 provider 分支：
     * 「超商列印」這個端點就是只列印超商的單。真正的 provider 行為判斷走
     * {@code FulfillmentProviderAdapter} 的 capability。
     */
    public SellerShipmentSettingEntity requireActiveSetting(long storeId, ShippingMethod method) {
        SellerShipmentSettingEntity setting = findActiveSetting(storeId, method.value());
        if (setting == null) {
            throw new ShippingBusinessException(
                    GrpcResponseCode.SHIPMENT_SELLER_NOT_ONBOARDED, method.label());
        }
        return setting;
    }

    /**
     * 供退件流程使用：查詢指定配送方式的 historical setting（不過濾 onboarding_status / enabled）。
     * 詳見 06 設計書 §2.3.1：賣家在我方平台取消 onboarding 不應牽連舊單退件。
     *
     * <p>🔴 <b>與 {@link #findActiveSetting} 同一個模糊語意，而且候選集更寬</b>：
     * 唯一索引是 {@code (store_id, shipping_type, COALESCE(goods_type, ''))}，
     * {@code shipping_type} 單獨不唯一 ⇒ 同一個配送方式底下合法地有多列（不同溫層）。
     * 這裡連 {@code onboarding_status} 與 {@code enabled} 都<b>刻意不過濾</b>（那正是它存在的理由），
     * 所以候選集比 {@code findActiveSetting} 更大。「退件該用哪一個溫層的退貨門市／退貨地址」
     * 這個問題同樣<b>在設計上沒有答案</b>，{@code ORDER BY} 只是讓答案<b>不再隨查詢計畫飄移</b>。
     *
     * <p>🔴 排序鍵與方向的理由同 {@link #findActiveSetting}：{@code id} 是 PK ⇒ 全序 ⇒ 唯一答案；
     * ASC ⇒ 新增溫層列（TSID 較大）不會改掉既有退件流程原本選到的那一列。
     *
     * <p>⚠️ <b>dev ＋ prod 皆實測 2026-09-10</b>。<b>prod 側</b>：本方法的候選集
     * （不過濾狀態）多列組數 <b>0</b>、{@code enabled} 分歧組數 <b>0</b>
     * ⇒ 下面那個 {@code enabled} 問題在 prod <b>不存在</b>，只在 dev。
     * <b>dev 側</b>：不過濾狀態時共 16 組
     * {@code (store_id, shipping_type)} 為多列（32 列），仍<b>全部</b>落在 {@code 3651}
     * （實查 {@code GROUP BY shipping_type} 只回一列）；被這條路徑消費的退件欄位
     * （{@code return_store_code}／{@code return_store_name}／{@code return_store_address}／
     * {@code sender_name}／{@code sender_phone}／{@code return_name}／{@code return_phone}／
     * {@code return_city}／{@code return_district}／{@code return_address_detail}／
     * {@code return_postal_code}）零分歧 ⇒ 目前取哪一列結果相同。
     *
     * <p>🔴 <b>其中 1 組的 {@code enabled} 兩列不一致，而且它現在就有消費者</b>
     * （dev 實測 {@code store_id=2619961976} / {@code shipping_type=3651}：
     * {@code id} 較小那列 {@code goods_type=3203}／{@code enabled=t}，
     * 較大那列 {@code goods_type=3201}／{@code enabled=f}）。
     *
     * <p>本方法自己刻意不看 {@code enabled}（退件流程讀的是退貨聯絡與地址欄位），
     * 但它的<b>回傳值</b>會被 {@code ShippingGrpcServiceImpl.getSellerReturnConfig} 拿去做
     * {@code Boolean.TRUE.equals(setting.getEnabled())} 判斷，決定回給前端的
     * {@code available} 是 true 還是 false。⇒ <b>選到哪一列，直接決定那個賣家的退貨管道
     * 在前端顯示為可用或不可用。</b>
     * ⚠️ 這一段原本寫「如果哪天有人把 {@code enabled} 也拿來判斷」——<b>那一天就是今天</b>，
     * 該消費者早就存在。由獨立驗收查出，這裡改成指名它。
     *
     * <p>加 {@code ORDER BY id ASC} 對這一組是<b>改善</b>：原本 {@code available} 取決於
     * 查詢計畫（等於擲骰子），現在釘死成 {@code enabled=t} 那列 ⇒ 恆為 true。
     * 但這是<b>行為變化</b>不是零變化，所以寫在這裡而不是含糊帶過。
     * ⚠️ 真正的修法是讓呼叫端把溫層帶進來（既有私有方法
     * {@code findSetting(storeId, shippingType, goodsType)} 被唯一索引完全限定），
     * 本次刻意不做——它會動到多個呼叫端，超出本次範圍。
     * {@code onboarding_status} 在這 16 組內零分歧。
     */
    public SellerShipmentSettingEntity findSettingByMethod(long storeId, String shippingMethod) {
        String shippingType = mapShippingMethodToType(storeId, shippingMethod);
        if (shippingType == null) {
            return null;
        }
        return settingMapper.selectOne(
                new LambdaQueryWrapper<SellerShipmentSettingEntity>()
                        .eq(SellerShipmentSettingEntity::getStoreId, storeId)
                        .eq(SellerShipmentSettingEntity::getShippingType, shippingType)
                        // 選列確定化：見本方法 javadoc。
                        .orderByAsc(SellerShipmentSettingEntity::getId)
                        .last("LIMIT 1"));
    }

    /**
     * 根據 ID 查詢建店設定
     */
    public SellerShipmentSettingEntity getSettingById(Long settingId) {
        if (settingId == null) {
            return null;
        }
        return settingMapper.selectById(settingId);
    }

    // ==================== 啟用/停用切換 ====================

    /**
     * 切換建店設定的啟用/停用狀態（純平台內部，不與 PayUni 溝通）
     */
    @Transactional
    public SellerShipmentSettingEntity toggleEnabled(long storeId, long onboardingId, boolean enabled) {
        SellerShipmentSettingEntity entity = settingMapper.selectById(onboardingId);
        if (entity == null) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_ONBOARDING_NOT_FOUND);
        }
        if (!entity.getStoreId().equals(storeId)) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_ONBOARDING_UPDATE_DENIED);
        }

        // INV-3 對齊：只更新 enabled 欄位，避免全欄位覆蓋
        settingMapper.updateEnabled(entity.getId(), enabled);
        entity.setEnabled(enabled);

        insertLog(entity.getId(), storeId, ACTION_LOCAL,
                "{\"action\":\"TOGGLE_ENABLED\",\"enabled\":" + enabled + "}", null, RESULT_SUCCESS, null);

        log.info("OnboardingService.toggleEnabled.done",
                "啟用/停用切換完成，enabled=" + enabled + ", onboardingId=" + onboardingId);
        return entity;
    }

    // ==================== 私有方法 ====================

    /**
     * 處理既有建店設定（依據當前狀態決定行為）
     * <p>
     * 超商場景傳入：senderName/senderPhone/returnStoreCode/returnStoreName/returnStoreAddress/labelCompanyName/contactEmail
     * 宅配場景傳入：senderName/senderPhone/productCategory/senderCity~senderPostalCode/returnName~returnPostalCode
     */
    private SellerShipmentSettingEntity handleExistingSetting(SellerShipmentSettingEntity existing,
                                                               long storeId,
                                                               String merId,
                                                               String contactEmail,
                                                               String senderName,
                                                               String senderPhone,
                                                               String returnStoreCode,
                                                               String returnStoreName,
                                                               String returnStoreAddress,
                                                               String labelCompanyName,
                                                               String productCategory,
                                                               String senderCity,
                                                               String senderDistrict,
                                                               String senderAddressDetail,
                                                               String senderPostalCode,
                                                               String returnName,
                                                               String returnPhone,
                                                               String returnCity,
                                                               String returnDistrict,
                                                               String returnAddressDetail,
                                                               String returnPostalCode) {
        OnboardingStatus currentStatus = OnboardingStatus.fromValue(existing.getOnboardingStatus());

        switch (currentStatus) {
            case ACTIVE, REJECTED -> {
                // 重新提交 PayUni（允許重複送，等同更新資訊）
                if (SHIPPING_TYPE_CVS.equals(existing.getShippingType())) {
                    return resubmitCvsToPayUni(existing, merId, contactEmail,
                            senderName, senderPhone, returnStoreCode, labelCompanyName);
                } else {
                    return resubmitHomeToPayUni(existing, merId, productCategory,
                            senderName, senderPhone,
                            senderCity, senderDistrict, senderAddressDetail, senderPostalCode,
                            returnName, returnPhone,
                            returnCity, returnDistrict, returnAddressDetail, returnPostalCode);
                }
            }
            case PENDING_REVIEW -> throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_ONBOARDING_PENDING_REVIEW);
            case PERMANENTLY_CLOSED ->
                    throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_ONBOARDING_PERMANENTLY_CLOSED);
            case CLOSED -> throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_ONBOARDING_CLOSED);
        }
        return existing;
    }

    private SellerShipmentSettingEntity submitCvsToPayUni(long storeId,
                                                           String merId,
                                                           String contactEmail,
                                                           String senderName,
                                                           String senderPhone,
                                                           String mappedGoodsType,
                                                           String goodsType,
                                                           String returnStoreCode,
                                                           String returnStoreName,
                                                           String returnStoreAddress,
                                                           String labelCompanyName) {
        String effectiveNotifyUrl = (notifyUrl != null && !notifyUrl.isBlank()) ? notifyUrl : null;

        OnboardCvsRequest payUniReq = new OnboardCvsRequest(
                mappedGoodsType, CVS_LGS_TYPE, CVS_SHIP_TYPE,
                labelCompanyName,
                senderName,
                senderPhone,
                contactEmail,
                returnStoreCode,
                effectiveNotifyUrl
        );

        OnboardShippingResponse payUniResp;
        try {
            payUniResp = shippingClient.onboardCvsShipping(merId, payUniReq);
        } catch (Exception e) {
            log.error("OnboardingService.submitCvs.error", "PayUni 超商建店呼叫失敗: " + e.getMessage());
            throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_ONBOARDING_FAILED, e.getMessage());
        }

        // PayUni 回傳失敗時直接拋異常，不建立設定記錄
        if (!payUniResp.isSuccess()) {
            String errorMsg = describePayUniOnboardError(payUniResp);
            log.warn("OnboardingService.submitCvs.payuniFailed",
                    "PayUni 超商建店回傳失敗，merId=" + merId + ", error=" + errorMsg);
            throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_ONBOARDING_FAILED, errorMsg);
        }

        // PayUni 成功，建立設定記錄
        OnboardingStatus status = OnboardingStatusRules.fromPayUniSetStatus(payUniResp.setStatus());

        SellerShipmentSettingEntity entity = new SellerShipmentSettingEntity();
        entity.setId(TsidUtil.generate());
        entity.setStoreId(storeId);
        entity.setShippingType(SHIPPING_TYPE_CVS);
        entity.setGoodsType(goodsType);
        entity.setSenderName(senderName);
        entity.setSenderPhone(senderPhone);
        entity.setReturnStoreCode(returnStoreCode);
        if (returnStoreName != null && !returnStoreName.isEmpty()) {
            entity.setReturnStoreName(returnStoreName);
        }
        if (returnStoreAddress != null && !returnStoreAddress.isEmpty()) {
            entity.setReturnStoreAddress(returnStoreAddress);
        }
        entity.setLabelCompanyName(labelCompanyName);
        entity.setMerId(merId);
        entity.setPartnerId(payUniResp.partnerId());
        entity.setEshipId(payUniResp.eshipId());
        entity.setProviderStatus(payUniResp.setStatus()); entity.setProviderMessage(payUniResp.apiResponse().message() != null ? payUniResp.apiResponse().message() : "");
        entity.setOnboardingStatus(status.value());
        entity.setEnabled(true);
        // 首次 INSERT 直接寫 SUCCEEDED（PayUni 同步呼叫已成功，§2.4.5.3）
        entity.setPayuniSubmitStatus(io.github.will413028.hawkr.common.utils.code.PayUniSubmitStatus.SUCCEEDED.value());
        entity.setPayuniSubmittedAt(io.github.will413028.hawkr.common.utils.DateTimeUtils.nowUtc());
        settingMapper.insert(entity);

        // 記錄日誌
        insertLog(entity.getId(), storeId, ACTION_PAYUNI,
                "{\"shippingType\":\"" + SHIPPING_TYPE_CVS + "\",\"goodsType\":\"" + goodsType + "\"}",
                "{\"partnerId\":\"" + payUniResp.partnerId() + "\",\"setStatus\":\"" + payUniResp.setStatus() + "\"}",
                RESULT_SUCCESS, null);

        log.info("OnboardingService.submitCvs.success",
                "超商建店完成，status=" + status.value(),
                "storeId", String.valueOf(storeId));
        return entity;
    }

    /**
     * 將 PayUni 建店失敗回應轉為可讀錯誤訊息。
     * PayUni 部分錯誤（如地址格式 DEF01003）message 欄位為空、只有 status code，
     * 直接透傳 message() 會使對外訊息變成「物流建店失敗：null」，故在此映射為可讀訊息。
     */
    private static String describePayUniOnboardError(OnboardShippingResponse resp) {
        if (resp == null || resp.apiResponse() == null) {
            return "未知錯誤";
        }
        PayUniApiResponse api = resp.apiResponse();
        String message = api.getErrorMessage();
        if (message != null && !message.isBlank()) {
            return message;
        }
        String status = api.status();
        if (status == null || status.isBlank()) {
            return "未知錯誤";
        }
        if ("DEF01003".equals(status)) {
            return "寄件地址格式有誤（PayUni 不接受里/鄰），請確認地址為「縣市＋鄉鎮市區＋路街＋門牌號」格式（代碼 " + status + "）";
        }
        return "PayUni 建店失敗（代碼 " + status + "）";
    }

    private SellerShipmentSettingEntity submitHomeToPayUni(long storeId,
                                                            String merId,
                                                            String productCategory,
                                                            String senderName,
                                                            String senderPhone,
                                                            String senderCity,
                                                            String senderDistrict,
                                                            String senderAddressDetail,
                                                            String senderPostalCode,
                                                            String returnName,
                                                            String returnPhone,
                                                            String returnCity,
                                                            String returnDistrict,
                                                            String returnAddressDetail,
                                                            String returnPostalCode) {
        // 防 app 端誤把全址塞進 detail 欄位導致拼接後 city+district 重複（PayUni 回 DEF01003）
        final String normalizedSenderDetail = AddressNormalizer.normalizeAddressDetail(senderCity, senderDistrict, senderAddressDetail);
        final String normalizedReturnDetail = AddressNormalizer.normalizeAddressDetail(returnCity, returnDistrict, returnAddressDetail);

        String senderAddress = senderCity + senderDistrict + normalizedSenderDetail;
        String effectiveNotifyUrl = (notifyUrl != null && !notifyUrl.isBlank()) ? notifyUrl : null;

        OnboardHomeRequest payUniReq = new OnboardHomeRequest(
                HOME_LGS_TYPE, HOME_SHIP_TYPE,
                senderName,
                senderPhone,
                senderAddress,
                productCategory,
                effectiveNotifyUrl
        );

        OnboardShippingResponse payUniResp;
        try {
            payUniResp = shippingClient.onboardHomeDelivery(merId, payUniReq);
        } catch (Exception e) {
            log.error("OnboardingService.submitHome.error", "PayUni 宅配建店呼叫失敗: " + e.getMessage());
            throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_ONBOARDING_FAILED, e.getMessage());
        }

        // PayUni 回傳失敗時直接拋異常，不建立設定記錄
        if (!payUniResp.isSuccess()) {
            String errorMsg = describePayUniOnboardError(payUniResp);
            log.warn("OnboardingService.submitHome.payuniFailed",
                    "PayUni 宅配建店回傳失敗，merId=" + merId + ", error=" + errorMsg);
            throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_ONBOARDING_FAILED, errorMsg);
        }

        // PayUni 成功，建立設定記錄
        OnboardingStatus status = OnboardingStatusRules.fromPayUniSetStatus(payUniResp.setStatus());

        SellerShipmentSettingEntity entity = new SellerShipmentSettingEntity();
        entity.setId(TsidUtil.generate());
        entity.setStoreId(storeId);
        entity.setShippingType(SHIPPING_TYPE_HOME);
        entity.setProductCategoryCode(productCategory);
        entity.setSenderName(senderName);
        entity.setSenderPhone(senderPhone);
        entity.setSenderCity(senderCity);
        entity.setSenderDistrict(senderDistrict);
        entity.setSenderAddressDetail(normalizedSenderDetail);
        entity.setSenderPostalCode(senderPostalCode);
        entity.setReturnName(returnName);
        entity.setReturnPhone(returnPhone);
        entity.setReturnCity(returnCity);
        entity.setReturnDistrict(returnDistrict);
        entity.setReturnAddressDetail(normalizedReturnDetail);
        entity.setReturnPostalCode(returnPostalCode);
        entity.setMerId(merId);
        entity.setPartnerId(payUniResp.partnerId());
        entity.setEshipId(payUniResp.eshipId());
        entity.setProviderStatus(payUniResp.setStatus()); entity.setProviderMessage(payUniResp.apiResponse().message() != null ? payUniResp.apiResponse().message() : "");
        entity.setOnboardingStatus(status.value());
        entity.setEnabled(true);
        // 首次 INSERT 直接寫 SUCCEEDED（PayUni 同步呼叫已成功，§2.4.5.3）
        entity.setPayuniSubmitStatus(io.github.will413028.hawkr.common.utils.code.PayUniSubmitStatus.SUCCEEDED.value());
        entity.setPayuniSubmittedAt(io.github.will413028.hawkr.common.utils.DateTimeUtils.nowUtc());
        settingMapper.insert(entity);

        // 記錄日誌
        insertLog(entity.getId(), storeId, ACTION_PAYUNI,
                "{\"shippingType\":\"" + SHIPPING_TYPE_HOME + "\",\"productCategory\":\"" + productCategory + "\"}",
                "{\"partnerId\":\"" + payUniResp.partnerId() + "\",\"setStatus\":\"" + payUniResp.setStatus() + "\"}",
                RESULT_SUCCESS, null);

        log.info("OnboardingService.submitHome.success",
                "宅配建店完成，status=" + status.value(),
                "storeId", String.valueOf(storeId));
        return entity;
    }

    /**
     * 重新提交 CVS 建店至 PayUni（UPDATE 路徑）。走 §2.4.4 ProviderCallExecutor 框架。
     * <ul>
     *   <li>acquireRequested：4301/4304 → 4302（防併發）</li>
     *   <li>invoke：呼叫 PayUni</li>
     *   <li>markSucceeded：4302 → 4303 + 寫入 partner_id/eship_id/onboarding_status；同步更新賣家輸入欄位</li>
     *   <li>markFailed：依 §2.4.4 transient 分類 — transient 跳過保留 4302（給 §5.19.3.D ops 告警）；permanent 4302 → 4304</li>
     * </ul>
     */
    private SellerShipmentSettingEntity resubmitCvsToPayUni(SellerShipmentSettingEntity existing,
                                                              String merId,
                                                              String contactEmail,
                                                              String senderName,
                                                              String senderPhone,
                                                              String returnStoreCode,
                                                              String labelCompanyName) {
        String effectiveNotifyUrl = (notifyUrl != null && !notifyUrl.isBlank()) ? notifyUrl : null;
        String mappedGoodsType = mapGoodsType(existing.getGoodsType());

        final OnboardCvsRequest payUniReq = new OnboardCvsRequest(
                mappedGoodsType, CVS_LGS_TYPE, CVS_SHIP_TYPE,
                labelCompanyName,
                senderName,
                senderPhone,
                contactEmail,
                returnStoreCode,
                effectiveNotifyUrl
        );

        final long settingId = existing.getId();

        try {
            providerCallExecutor.execute(
                    ProviderCallContext.of("resubmitCvsToPayUni", settingId),
                    () -> settingMapper.acquirePayUniSubmitRequested(settingId) > 0,
                    () -> shippingClient.onboardCvsShipping(merId, payUniReq),
                    payUniResp -> {
                        if (!payUniResp.isSuccess()) {
                            // PayUni 回業務錯誤（Status != SUCCESS）→ 走 markFailed 路徑
                            String errMsg = describePayUniOnboardError(payUniResp);
                            throw new io.github.will413028.hawkr.payuni.http.PayUniApiException(
                                    payUniResp.setStatus() != null ? payUniResp.setStatus() : "PAYUNI_FAILED",
                                    errMsg, 0);
                        }
                        OnboardingStatus newStatus = OnboardingStatusRules.fromPayUniSetStatus(payUniResp.setStatus());
                        // 1. 寫 PayUni 回傳欄位 + transition 4302→4303
                        settingMapper.markPayUniSubmitSucceeded(
                                settingId,
                                payUniResp.partnerId(),
                                payUniResp.eshipId(),
                                newStatus.value(),
                                payUniResp.setStatus(),
                                payUniResp.apiResponse().message() != null ? payUniResp.apiResponse().message() : "");
                        // 2. 同步更新賣家輸入的非 PayUni 欄位（COALESCE 模式，傳 null 不變）
                        settingMapper.updateCvsSellerInputFields(
                                settingId, senderName, senderPhone,
                                returnStoreCode, null, null,
                                labelCompanyName);
                    },
                    ex -> {
                        if (isTransientPayUniError(ex)) {
                            log.info("OnboardingService.resubmitCvs.transientErrorKept",
                                    "PayUni 暫時性錯誤，保留 REQUESTED 等 ops 告警: " + ex.getMessage(),
                                    "settingId", String.valueOf(settingId));
                            return;
                        }
                        settingMapper.markPayUniSubmitFailed(
                                settingId,
                                extractProviderStatus(ex),
                                extractProviderMessage(ex));
                    }
            );
        } catch (ProviderCallBusyException busy) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_ONBOARDING_FAILED, "建店進行中或已成功，請稍後重試");
        } catch (io.github.will413028.hawkr.payuni.http.PayUniApiException papi) {
            // markFailed 已執行（transient 跳過 / permanent 寫 4304）；對外保持既有契約
            insertLog(settingId, existing.getStoreId(), ACTION_PAYUNI,
                    "{\"action\":\"RESUBMIT\"}", null, RESULT_FAILED, null);
            throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_ONBOARDING_FAILED, papi.getMessage());
        }

        insertLog(settingId, existing.getStoreId(), ACTION_PAYUNI,
                "{\"action\":\"RESUBMIT\"}", null, RESULT_SUCCESS, null);

        // 回傳重新查詢的 entity 給 caller（含 markPayUniSubmitSucceeded + updateCvsSellerInputFields 的最新值）
        return settingMapper.selectById(settingId);
    }

    /**
     * 重新提交 HOME 建店至 PayUni（UPDATE 路徑）。走 §2.4.4 ProviderCallExecutor 框架。
     * <p>地址 readonly 處理（§2.4.5.4）：當前 status='4303' (SUCCEEDED) 表示既有地址 PayUni 已存檔，
     * 不允許更新；caller 把地址欄位傳 null 至 updateHomeSellerInputFields（COALESCE 不變）。
     */
    private SellerShipmentSettingEntity resubmitHomeToPayUni(SellerShipmentSettingEntity existing,
                                                               String merId,
                                                               String productCategory,
                                                               String senderName,
                                                               String senderPhone,
                                                               String senderCity,
                                                               String senderDistrict,
                                                               String senderAddressDetail,
                                                               String senderPostalCode,
                                                               String returnName,
                                                               String returnPhone,
                                                               String returnCity,
                                                               String returnDistrict,
                                                               String returnAddressDetail,
                                                               String returnPostalCode) {
        // 防 app 端誤把全址塞進 detail 欄位導致拼接後 city+district 重複（PayUni 回 DEF01003）
        final String normalizedSenderDetail = AddressNormalizer.normalizeAddressDetail(senderCity, senderDistrict, senderAddressDetail);
        final String normalizedReturnDetail = AddressNormalizer.normalizeAddressDetail(returnCity, returnDistrict, returnAddressDetail);

        // 注意：判斷 alreadySubmitted 必須在 acquireRequested 之前（後者會把狀態改為 4302 失去判斷依據）
        final boolean alreadySubmitted = PayUniSubmitStatus.SUCCEEDED.value().equals(existing.getPayuniSubmitStatus());

        // PayUni 已提交過：地址不允許更新，使用 DB 中的原始地址送 PayUni
        // （對 DB 既有資料也 normalize，防舊資料含全址 detail）
        final String effectiveSenderAddress;
        if (alreadySubmitted) {
            String dbCleanSenderDetail = AddressNormalizer.normalizeAddressDetail(
                    existing.getSenderCity(), existing.getSenderDistrict(), existing.getSenderAddressDetail());
            effectiveSenderAddress = existing.getSenderCity() + existing.getSenderDistrict() + dbCleanSenderDetail;
        } else {
            effectiveSenderAddress = senderCity + senderDistrict + normalizedSenderDetail;
        }

        String effectiveNotifyUrl = (notifyUrl != null && !notifyUrl.isBlank()) ? notifyUrl : null;

        final OnboardHomeRequest payUniReq = new OnboardHomeRequest(
                HOME_LGS_TYPE, HOME_SHIP_TYPE,
                senderName,
                senderPhone,
                effectiveSenderAddress,
                productCategory,
                effectiveNotifyUrl
        );

        final long settingId = existing.getId();

        try {
            providerCallExecutor.execute(
                    ProviderCallContext.of("resubmitHomeToPayUni", settingId),
                    () -> settingMapper.acquirePayUniSubmitRequested(settingId) > 0,
                    () -> shippingClient.onboardHomeDelivery(merId, payUniReq),
                    payUniResp -> {
                        if (!payUniResp.isSuccess()) {
                            String errMsg = describePayUniOnboardError(payUniResp);
                            throw new io.github.will413028.hawkr.payuni.http.PayUniApiException(
                                    payUniResp.setStatus() != null ? payUniResp.setStatus() : "PAYUNI_FAILED",
                                    errMsg, 0);
                        }
                        OnboardingStatus newStatus = OnboardingStatusRules.fromPayUniSetStatus(payUniResp.setStatus());
                        // 1. 寫 PayUni 回傳欄位 + transition 4302→4303
                        settingMapper.markPayUniSubmitSucceeded(
                                settingId,
                                payUniResp.partnerId(),
                                payUniResp.eshipId(),
                                newStatus.value(),
                                payUniResp.setStatus(),
                                payUniResp.apiResponse().message() != null ? payUniResp.apiResponse().message() : "");
                        // 2. 同步更新賣家輸入的非 PayUni 欄位
                        // 已 SUCCEEDED 過：地址鎖定，傳 null 給 COALESCE；非地址欄位（姓名/電話/商品類別）始終可更新
                        settingMapper.updateHomeSellerInputFields(
                                settingId,
                                senderName, senderPhone, productCategory,
                                alreadySubmitted ? null : senderCity,
                                alreadySubmitted ? null : senderDistrict,
                                alreadySubmitted ? null : normalizedSenderDetail,
                                alreadySubmitted ? null : senderPostalCode,
                                alreadySubmitted ? null : returnName,
                                alreadySubmitted ? null : returnPhone,
                                alreadySubmitted ? null : returnCity,
                                alreadySubmitted ? null : returnDistrict,
                                alreadySubmitted ? null : normalizedReturnDetail,
                                alreadySubmitted ? null : returnPostalCode);
                    },
                    ex -> {
                        if (isTransientPayUniError(ex)) {
                            log.info("OnboardingService.resubmitHome.transientErrorKept",
                                    "PayUni 暫時性錯誤，保留 REQUESTED 等 ops 告警: " + ex.getMessage(),
                                    "settingId", String.valueOf(settingId));
                            return;
                        }
                        settingMapper.markPayUniSubmitFailed(
                                settingId,
                                extractProviderStatus(ex),
                                extractProviderMessage(ex));
                    }
            );
        } catch (ProviderCallBusyException busy) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_ONBOARDING_FAILED, "建店進行中或已成功，請稍後重試");
        } catch (io.github.will413028.hawkr.payuni.http.PayUniApiException papi) {
            insertLog(settingId, existing.getStoreId(), ACTION_PAYUNI,
                    "{\"action\":\"RESUBMIT\",\"addressLocked\":" + alreadySubmitted + "}", null, RESULT_FAILED, null);
            throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_ONBOARDING_FAILED, papi.getMessage());
        }

        insertLog(settingId, existing.getStoreId(), ACTION_PAYUNI,
                "{\"action\":\"RESUBMIT\",\"addressLocked\":" + alreadySubmitted + "}", null, RESULT_SUCCESS, null);

        return settingMapper.selectById(settingId);
    }

    /**
     * 從 seller-service 取得賣家 Meta 資訊（merId、店名等）
     */
    private SellerMeta getSellerMeta(long storeId) {
        BatchGetSellerMetaResponse resp =
                sellerGrpcClient.batchGetSellerMeta(List.of(storeId));
        // 失敗時 seller-service 會拋 StatusRuntimeException，由上層 RPC catch-all 處理
        SellerMeta meta = resp.getSellersMap().get(storeId);
        if (meta == null) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_SELLER_META_NOT_FOUND);
        }
        if (meta.getPayuniMerId().isBlank()) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPPING_SELLER_PAYUNI_NOT_REGISTERED);
        }
        return meta;
    }

    /**
     * 從 seller-service 取得賣家聯絡信箱（SellerMeta 不含此欄位，需透過 GetSellerShippingProfile）
     */
    private String getSellerContactEmail(long storeId) {
        try {
            GetSellerShippingProfileResponse profileResp =
                    sellerGrpcClient.getSellerShippingProfile(storeId);
            if (!profileResp.hasData()) {
                log.warn("OnboardingService.getSellerContactEmail.failed",
                        "取得賣家信箱失敗，storeId=" + storeId);
                return "";
            }
            return profileResp.getData().getContactEmail();
        } catch (Exception e) {
            log.warn("OnboardingService.getSellerContactEmail.exception",
                    "取得賣家信箱異常（降級為空），storeId=" + storeId);
            return "";
        }
    }

    private SellerShipmentSettingEntity findSetting(long storeId,
                                                     String shippingType,
                                                     String goodsType) {
        LambdaQueryWrapper<SellerShipmentSettingEntity> query =
                new LambdaQueryWrapper<SellerShipmentSettingEntity>()
                        .eq(SellerShipmentSettingEntity::getStoreId, storeId)
                        .eq(SellerShipmentSettingEntity::getShippingType, shippingType);
        if (goodsType != null) {
            query.eq(SellerShipmentSettingEntity::getGoodsType, goodsType);
        } else {
            query.isNull(SellerShipmentSettingEntity::getGoodsType);
        }
        return settingMapper.selectOne(query);
    }

    private void insertLog(Long settingId, long storeId, String actionType,
                           String requestSnapshot, String responseSnapshot,
                           String resultStatus, String errorMessage) {
        OnboardingLogEntity logEntity = new OnboardingLogEntity();
        logEntity.setSettingId(settingId);
        logEntity.setStoreId(storeId);
        logEntity.setActionType(actionType);
        logEntity.setRequestSnapshot(requestSnapshot);
        logEntity.setResponseSnapshot(responseSnapshot);
        logEntity.setResultStatus(resultStatus);
        logEntity.setErrorMessage(errorMessage);
        logMapper.insert(logEntity);
    }

    /**
     * 將前端 goodsType 映射為 PayUni 參數值
     */
    private String mapGoodsType(String goodsType) {
        if (goodsType == null) return null;
        if (ShippingTemperature.AMBIENT.value().equals(goodsType)) {
            return GoodsType.NORMAL.code();
        }
        if (ShippingTemperature.FROZEN.value().equals(goodsType)) {
            return GoodsType.FROZEN.code();
        }
        return null;
    }

    /**
     * 配送方式 → 建店槽位分類（{@code ShippingType.value()}）。
     *
     * <p>🔴 判斷依據已從「配送方式是不是 3002」換成
     * {@link OnboardCapability#shippingTypeValue()}——那是 provider 的性質，不是流程的性質。
     * 舊實作是一段 {@code if/else} 字面比對，新增 provider 時要記得回來加一格，
     * 而漏加的症狀是<b>靜默</b>：回 {@code null} ⇒ 兩個呼叫端都 {@code return null}
     * ⇒ 建單流程永遠取不到設定，沒有任何錯誤訊息。
     *
     * <p>回 {@code null} 的語意維持不變（查無 adapter，或該 provider 沒有建店概念），
     * 呼叫端的 null 處理因此不必改。
     *
     * @param storeId        用於組 capability 解析的 context（{@code 3005} 的能力集合是 per-store）
     * @param shippingMethod {@code ShippingMethod.value()}
     */
    private String mapShippingMethodToType(long storeId, String shippingMethod) {
        return fulfillmentProviders.find(shippingMethod)
                .flatMap(adapter -> adapter.onboard(
                        FulfillmentContext.forStore(storeId, adapter.supportedMethod())))
                .map(OnboardCapability::shippingTypeValue)
                .orElse(null);
    }

    /**
     * 截斷字串至指定長度（以字元計）
     */
    private String truncateToMaxLength(String value, int maxLength) {
        if (value == null) return "";
        return value.length() > maxLength ? value.substring(0, maxLength) : value;
    }

    private boolean isBlank(String value) {
        return value == null || value.isBlank();
    }
}
