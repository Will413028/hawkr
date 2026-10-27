package io.github.will413028.hawkr.product.converter;

import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.product.dto.request.CreateProductDTO;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import static org.junit.jupiter.api.Assertions.assertEquals;

/**
 * ② re-key：toEntity 必須同時 stamp 擁有權欄位 store_id（= 請求 storeId，
 * resolver 上線後為 store surrogate）與 audit 欄位 created_by。
 */
@ExtendWith(MockitoExtension.class)
class ProductEntityConverterOwnerTest {

    @Mock ProductSkuConverter skuConverter;

    @BeforeAll
    static void initTsid() {
        try {
            TsidUtil.init(200, 0);
        } catch (IllegalStateException ignored) {
            // 已初始化則忽略（node-id-base 200-299 為 product-service 範圍）
        }
    }

    @Test
    @DisplayName("toEntity：storeId 與 createdBy 都 stamp 為請求 storeId（owner + audit 雙寫）")
    void toEntityStampsOwnerAndAudit() {
        ProductEntityConverter converter = new ProductEntityConverter(skuConverter, new ObjectMapper());
        CreateProductDTO dto = new CreateProductDTO();
        dto.setName("有機小白菜");
        // 請求 storeId（resolver 上線後 = store surrogate）；owner 與 audit 應同值
        final long requestStoreId = 7412345678901234567L;
        dto.setStoreId(requestStoreId);

        ProductEntity entity = converter.toEntity(dto);

        assertEquals(requestStoreId, entity.getStoreId());
        assertEquals(requestStoreId, entity.getCreatedBy());
    }
}
