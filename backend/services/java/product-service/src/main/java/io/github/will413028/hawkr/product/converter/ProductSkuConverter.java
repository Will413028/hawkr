package io.github.will413028.hawkr.product.converter;

import io.github.will413028.hawkr.product.dto.request.ProductSkuDTO;
import io.github.will413028.hawkr.product.dto.request.ShippingMethodDTO;
import io.github.will413028.hawkr.product.entity.ProductSkuEntity;
import org.springframework.stereotype.Component;

import java.util.ArrayList;
import java.util.List;
import java.util.stream.Collectors;

/**
 * ProductSkuEntity 與 DTO 之間的轉換器
 */
@Component
public class ProductSkuConverter {

    /**
     * ProductSkuDTO → ProductSkuEntity
     */
    public ProductSkuEntity toEntity(ProductSkuDTO dto, Long productId) {
        ProductSkuEntity entity = new ProductSkuEntity();
        entity.setProductId(productId);
        entity.setSkuName(dto.getSkuName());
        entity.setWeight(dto.getWeight());
        entity.setUnit(dto.getUnit());
        entity.setPrice(dto.getPrice());
        entity.setSalePrice(dto.getSalePrice());
        entity.setStockQty(dto.getStockQty() != null ? dto.getStockQty() : 0);
        entity.setLengthCm(dto.getLengthCm());
        entity.setWidthCm(dto.getWidthCm());
        entity.setHeightCm(dto.getHeightCm());

        // 運送方式
        if (dto.getShippingMethods() != null) {
            List<ProductSkuEntity.ShippingMethodItem> methods = dto.getShippingMethods().stream()
                    .map(sm -> {
                        ProductSkuEntity.ShippingMethodItem item = new ProductSkuEntity.ShippingMethodItem();
                        item.setType(sm.getType());
                        item.setTemperature(sm.getTemperature());
                        item.setFreeThreshold(sm.getFreeThreshold());
                        item.setFeePayer(sm.getFeePayer());
                        return item;
                    })
                    .collect(Collectors.toList());
            entity.setShippingMethod(methods);
        } else {
            entity.setShippingMethod(new ArrayList<>());
        }

        return entity;
    }

    /**
     * ProductSkuEntity → ProductSkuDTO
     */
    public ProductSkuDTO toDTO(ProductSkuEntity entity) {
        ProductSkuDTO dto = ProductSkuDTO.builder()
                .id(entity.getId().toString())
                .skuName(entity.getSkuName())
                .weight(entity.getWeight())
                .unit(entity.getUnit())
                .price(entity.getPrice())
                .salePrice(entity.getSalePrice())
                .stockQty(entity.getStockQty())
                .lengthCm(entity.getLengthCm())
                .widthCm(entity.getWidthCm())
                .heightCm(entity.getHeightCm())
                // SKU 樂觀鎖版本（#2545）：client 必須讀得到它才有辦法在下次寫回時回送
                // expectedVersion；不下發就等於這把鎖永遠沒有人拿得到鑰匙。
                .version(entity.getVersion() != null ? entity.getVersion().toString() : null)
                .build();

        // 運送方式
        if (entity.getShippingMethod() != null) {
            List<ShippingMethodDTO> smDTOs = entity.getShippingMethod().stream()
                    .map(sm -> ShippingMethodDTO.builder()
                            .type(sm.getType())
                            .temperature(sm.getTemperature())
                            .freeThreshold(sm.getFreeThreshold())
                            .feePayer(sm.getFeePayer())
                            .build())
                    .collect(Collectors.toList());
            dto.setShippingMethods(smDTOs);
        } else {
            dto.setShippingMethods(new ArrayList<>());
        }

        return dto;
    }

    /**
     * 更新 ProductSkuEntity（從 ProductSkuDTO）
     */
    public void updateEntity(ProductSkuEntity entity, ProductSkuDTO dto) {
        if (dto.getSkuName() != null && !dto.getSkuName().isBlank()) {
            entity.setSkuName(dto.getSkuName());
        }
        if (dto.getWeight() != null) {
            entity.setWeight(dto.getWeight());
        }
        if (dto.getUnit() != null && !dto.getUnit().isBlank()) {
            entity.setUnit(dto.getUnit());
        }
        if (dto.getPrice() != null) {
            entity.setPrice(dto.getPrice());
        }
        // 特價更新：dto.salePrice == null 視為「未提供」or「清空」？
        // 採用設計書語意：updateProduct 接受清空 → 採以下規則：
        // - 若 dto.getSalePrice() != null：更新為該值
        // - 若 dto 物件 SaleClear 標記未來可加（本期不支援單獨清空欄位的 API 標記）
        // 由於 UpdateProductDTO 不區分「null=清空」與「null=未變更」，
        // 為避免歧義，本期明確規定：updateProduct 傳入 SKU 時，salePrice 一律以 dto 的值為準（null=清空）
        entity.setSalePrice(dto.getSalePrice());
        if (dto.getStockQty() != null) {
            entity.setStockQty(dto.getStockQty());
        }
        if (dto.getLengthCm() != null) {
            entity.setLengthCm(dto.getLengthCm());
        }
        if (dto.getWidthCm() != null) {
            entity.setWidthCm(dto.getWidthCm());
        }
        if (dto.getHeightCm() != null) {
            entity.setHeightCm(dto.getHeightCm());
        }

        // 運送方式
        if (dto.getShippingMethods() != null) {
            List<ProductSkuEntity.ShippingMethodItem> methods = dto.getShippingMethods().stream()
                    .map(sm -> {
                        ProductSkuEntity.ShippingMethodItem item = new ProductSkuEntity.ShippingMethodItem();
                        item.setType(sm.getType());
                        item.setTemperature(sm.getTemperature());
                        item.setFreeThreshold(sm.getFreeThreshold());
                        item.setFeePayer(sm.getFeePayer());
                        return item;
                    })
                    .collect(Collectors.toList());
            entity.setShippingMethod(methods);
        }
    }
}
