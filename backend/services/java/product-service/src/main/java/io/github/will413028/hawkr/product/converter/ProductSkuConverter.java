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
                .version(entity.getVersion() != null ? entity.getVersion().toString() : null)
                .build();

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
