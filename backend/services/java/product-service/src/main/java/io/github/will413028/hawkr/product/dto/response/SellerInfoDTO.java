package io.github.will413028.hawkr.product.dto.response;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.io.Serializable;

/**
 * 商家資訊 DTO（公開商品詳情用）
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class SellerInfoDTO implements Serializable {
    private static final long serialVersionUID = 1L;

    /** 賣家 store 代理鍵（= product.store_id） */
    private Long storeId;

    /** 店家名稱（Seller Service 提供） */
    private String name;

    /** 評分（Seller Service 提供） */
    private String rate;

    /** 商家 Logo objectKey（enrich 自 Seller Service；49 §6.6 S9，顯示 URL 由 Gateway 組） */
    private String logoKey;

    /** 商家地址（city + district 合成，如「台北市大安區」） */
    private String address;

    /** 城市（如「台北市」），由 Seller Service 提供，可空 */
    private String city;

    /** 區域（如「大安區」），由 Seller Service 提供，可空 */
    private String district;

    /** 賣家短碼（enrich 自 Seller Service，可空） */
    private String storeCode;
}
