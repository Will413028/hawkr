package io.github.will413028.hawkr.shipping.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.time.LocalDateTime;
import java.util.List;

/**
 * seller_shipment_settings 自訂 SQL 與 PayUniSubmitStatus 狀態機 transition 入口。
 * 設計：06-Shipping §2.4.5.3 / §8.3.3
 */
@Mapper
public interface SellerShipmentSettingMapper extends BaseMapper<SellerShipmentSettingEntity> {

    /**
     * PayUni 建店送審 acquireRequested：transition (NONE | FAILED) → REQUESTED
     * 用於「重新提交」分支；首次建店走 INSERT 直接帶 '4302'。
     */
    @Update("""
        UPDATE seller_shipment_settings
        SET payuni_submit_status = '4302',
            payuni_submit_requested_at = NOW(),
            updated_at = NOW()
        WHERE id = #{id}
          AND payuni_submit_status IN ('4301', '4304')
        """)
    int acquirePayUniSubmitRequested(@Param("id") Long id);

    /**
     * PayUni 建店送審 markSucceeded：transition REQUESTED → SUCCEEDED
     * 同步寫入 PayUni 回傳的 partner_id / eship_id / onboarding_status / provider_*。
     */
    @Update("""
        UPDATE seller_shipment_settings
        SET payuni_submit_status = '4303',
            payuni_submitted_at = NOW(),
            partner_id = #{partnerId},
            eship_id = #{eshipId},
            onboarding_status = #{onboardingStatus},
            provider_status = #{providerStatus},
            provider_message = #{providerMessage},
            updated_at = NOW()
        WHERE id = #{id}
          AND payuni_submit_status = '4302'
        """)
    int markPayUniSubmitSucceeded(
            @Param("id") Long id,
            @Param("partnerId") String partnerId,
            @Param("eshipId") String eshipId,
            @Param("onboardingStatus") String onboardingStatus,
            @Param("providerStatus") String providerStatus,
            @Param("providerMessage") String providerMessage
    );

    /**
     * PayUni 建店送審 markFailed：transition REQUESTED → FAILED
     * 用於「重新提交」分支失敗；首次建店失敗走 ROLLBACK INSERT，不到此處。
     */
    @Update("""
        UPDATE seller_shipment_settings
        SET payuni_submit_status = '4304',
            provider_status = #{providerStatus},
            provider_message = #{providerMessage},
            updated_at = NOW()
        WHERE id = #{id}
          AND payuni_submit_status = '4302'
        """)
    int markPayUniSubmitFailed(
            @Param("id") Long id,
            @Param("providerStatus") String providerStatus,
            @Param("providerMessage") String providerMessage
    );

    /**
     * §5.19.3.D 建店送審 REQUESTED 卡死候選人（ops 告警）。
     */
    @Select("""
        SELECT *
        FROM seller_shipment_settings
        WHERE payuni_submit_status = '4302'
          AND payuni_submit_requested_at < #{requestedBefore}
          AND payuni_submit_requested_at >= #{maxAgeCutoff}
        LIMIT #{limit}
        """)
    List<SellerShipmentSettingEntity> findSubmitStuckCandidates(
            @Param("requestedBefore") LocalDateTime requestedBefore,
            @Param("maxAgeCutoff") LocalDateTime maxAgeCutoff,
            @Param("limit") int limit
    );

    /**
     * 切換啟用/停用：賣家手動 toggle 該物流通路 enabled 旗標。
     * 純本地切換，不影響 PayUni 設定。
     */
    @Update("""
        UPDATE seller_shipment_settings
        SET enabled = #{enabled},
            updated_at = NOW()
        WHERE id = #{id}
        """)
    int updateEnabled(@Param("id") Long id, @Param("enabled") boolean enabled);

    /**
     * §2.4.5.3 UPDATE 路徑使用：更新賣家在重送 PayUni 時帶入的「非 PayUni 回傳」欄位（C2C）。
     * <p>使用 COALESCE：傳 null 表示不更新該欄位（用於「地址 readonly」場景）。
     */
    @Update("""
        UPDATE seller_shipment_settings
        SET sender_name = COALESCE(#{senderName}, sender_name),
            sender_phone = COALESCE(#{senderPhone}, sender_phone),
            return_store_code = COALESCE(#{returnStoreCode}, return_store_code),
            return_store_name = COALESCE(#{returnStoreName}, return_store_name),
            return_store_address = COALESCE(#{returnStoreAddress}, return_store_address),
            label_company_name = COALESCE(#{labelCompanyName}, label_company_name),
            updated_at = NOW()
        WHERE id = #{id}
        """)
    int updateCvsSellerInputFields(
            @Param("id") Long id,
            @Param("senderName") String senderName,
            @Param("senderPhone") String senderPhone,
            @Param("returnStoreCode") String returnStoreCode,
            @Param("returnStoreName") String returnStoreName,
            @Param("returnStoreAddress") String returnStoreAddress,
            @Param("labelCompanyName") String labelCompanyName
    );

    /**
     * §2.4.5.3 UPDATE 路徑使用：更新賣家在重送 PayUni 時帶入的「非 PayUni 回傳」欄位（HOME）。
     * <p>使用 COALESCE：地址欄位（sender_*, return_*）已 SUCCEEDED 後 caller 傳 null 鎖定（§2.4.5.4）。
     */
    @Update("""
        UPDATE seller_shipment_settings
        SET sender_name = COALESCE(#{senderName}, sender_name),
            sender_phone = COALESCE(#{senderPhone}, sender_phone),
            product_category_code = COALESCE(#{productCategoryCode}, product_category_code),
            sender_city = COALESCE(#{senderCity}, sender_city),
            sender_district = COALESCE(#{senderDistrict}, sender_district),
            sender_address_detail = COALESCE(#{senderAddressDetail}, sender_address_detail),
            sender_postal_code = COALESCE(#{senderPostalCode}, sender_postal_code),
            return_name = COALESCE(#{returnName}, return_name),
            return_phone = COALESCE(#{returnPhone}, return_phone),
            return_city = COALESCE(#{returnCity}, return_city),
            return_district = COALESCE(#{returnDistrict}, return_district),
            return_address_detail = COALESCE(#{returnAddressDetail}, return_address_detail),
            return_postal_code = COALESCE(#{returnPostalCode}, return_postal_code),
            updated_at = NOW()
        WHERE id = #{id}
        """)
    int updateHomeSellerInputFields(
            @Param("id") Long id,
            @Param("senderName") String senderName,
            @Param("senderPhone") String senderPhone,
            @Param("productCategoryCode") String productCategoryCode,
            @Param("senderCity") String senderCity,
            @Param("senderDistrict") String senderDistrict,
            @Param("senderAddressDetail") String senderAddressDetail,
            @Param("senderPostalCode") String senderPostalCode,
            @Param("returnName") String returnName,
            @Param("returnPhone") String returnPhone,
            @Param("returnCity") String returnCity,
            @Param("returnDistrict") String returnDistrict,
            @Param("returnAddressDetail") String returnAddressDetail,
            @Param("returnPostalCode") String returnPostalCode
    );
}
