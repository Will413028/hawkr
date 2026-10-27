package io.github.will413028.hawkr.product.converter;

import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.product.domain.ProductPublicationPolicy;
import io.github.will413028.hawkr.product.dto.request.CertificationDTO;
import io.github.will413028.hawkr.product.dto.request.CreateProductDTO;
import io.github.will413028.hawkr.product.dto.request.DescriptionBlockDTO;
import io.github.will413028.hawkr.product.dto.request.InlineRunDTO;
import io.github.will413028.hawkr.product.dto.request.ProductDescriptionDTO;
import io.github.will413028.hawkr.product.dto.request.ProductSkuDTO;
import io.github.will413028.hawkr.product.dto.request.ShippingMethodDTO;
import io.github.will413028.hawkr.product.dto.request.UpdateProductDTO;
import io.github.will413028.hawkr.product.dto.response.GalleryItemDTO;
import io.github.will413028.hawkr.product.dto.response.ProductDetailDTO;
import io.github.will413028.hawkr.product.dto.response.ProductListItemDTO;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.entity.ProductSkuEntity;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Component;

import io.github.will413028.hawkr.common.utils.TsidUtil;

import java.util.ArrayList;
import java.util.Collections;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.time.LocalDateTime;
import java.util.stream.Collectors;

/**
 * ProductEntity 與 DTO 之間的轉換器
 */
@Component
public class ProductEntityConverter {

    private final ProductSkuConverter skuConverter;
    private final ObjectMapper objectMapper;

    public ProductEntityConverter(ProductSkuConverter skuConverter, ObjectMapper objectMapper) {
        this.skuConverter = skuConverter;
        this.objectMapper = objectMapper;
    }

    /**
     * CreateProductDTO → ProductEntity
     */
    public ProductEntity toEntity(CreateProductDTO dto) {
        ProductEntity entity = new ProductEntity();
        // 手動生成 TSID（因為使用 IdType.INPUT）
        entity.setId(TsidUtil.generate());
        entity.setName(dto.getName());
        entity.setCategory(dto.getCategoryId());
        entity.setDescription(convertDescriptionToEntity(dto.getDescription()));
        entity.setOrigin(dto.getOrigin());
        entity.setStatus(dto.getStatus() != null && !dto.getStatus().isBlank() ? dto.getStatus() : ProductStatus.ACTIVE.value());
        entity.setFirstPublishedAt(ProductPublicationPolicy.firstPublishedAtOnActivation(
                null, entity.getStatus(), LocalDateTime.now()));
        entity.setCreatedBy(dto.getStoreId());          // audit（② re-key 後不承載擁有權）
        entity.setStoreId(dto.getStoreId());            // owner：store 代理鍵（resolver 注入的 surrogate）

        // certifications 一律以空陣列落地（19 號設計書 §6.1）。
        //
        // 🔴 這裡刻意<b>不</b>把 dto.getCertifications() 寫進投影：product.certifications 已降級為
        // 「由當前所有 APPROVED 申請重算」的 approved-only 公開投影，唯一寫入者是
        // ProductCertificationApplicationService.rebuildProjection()。直寫回去等於回到
        // 「賣家填了就公開」——本設計書要消滅的正是這件事。
        // 空陣列而非 null：JSONB 存成 {} 會讓讀取路徑炸掉。
        entity.setCertifications(new ArrayList<>());

        return entity;
    }

    /**
     * 更新 ProductEntity（從 UpdateProductDTO）
     */
    public void updateEntity(ProductEntity entity, UpdateProductDTO dto) {
        if (dto.getName() != null && !dto.getName().isBlank()) {
            entity.setName(dto.getName());
        }
        if (dto.getCategoryId() != null) {
            entity.setCategory(dto.getCategoryId());
        }
        if (dto.getDescription() != null) {
            entity.setDescription(convertDescriptionToEntity(dto.getDescription()));
        }
        if (dto.getOrigin() != null) {
            entity.setOrigin(dto.getOrigin());
        }
        // status 不在 UpdateProduct 中處理，賣家狀態變更走 UpdateSellerProductStatus
        entity.setUpdatedBy(dto.getStoreId());

        // 🔴 certifications 刻意不在這裡處理（19 號設計書 §6.1）。
        //
        // 投影只由 ProductCertificationApplicationService.rebuildProjection() 產生；
        // 這裡直寫會讓賣家送出的（尚未審核的）宣稱立刻出現在公開面。
        // entity 保留從 DB 讀出的既有投影值，後續 updateById 原樣寫回不影響它，
        // 真正的變更由收斂點在同一交易內完成（ProductService.updateProduct 步驟 6.1）。
    }

    /**
     * ProductEntity + ProductSkuEntity list → ProductDetailDTO
     *
     * @param storeExposable 商品所屬店家目前是否可曝光（{@code store_exposure}，沒有列 = false）
     */
    public ProductDetailDTO toDetailDTO(ProductEntity entity, List<ProductSkuEntity> skus, boolean storeExposable) {
        ProductDetailDTO dto = ProductDetailDTO.builder()
                .id(entity.getId().toString())
                .name(entity.getName())
                .rating(entity.getRating())
                .status(entity.getStatus())
                .publicationState(ProductPublicationPolicy.deriveState(
                        entity.getFirstPublishedAt(), entity.getStatus(),
                        entity.getDeletedAt(), entity.getSellerDeletedAt(), storeExposable).value())
                .purchasable(ProductPublicationPolicy.isPurchasable(
                        entity.getFirstPublishedAt(), entity.getStatus(), entity.getStockQty(),
                        entity.getDeletedAt(), entity.getSellerDeletedAt(), storeExposable))
                .categoryId(entity.getCategory())
                .origin(entity.getOrigin())
                .description(convertDescriptionToDTO(entity.getDescription()))
                .priceMin(entity.getPriceMin())
                .priceMax(entity.getPriceMax())
                .listPriceMin(entity.getListPriceMin())
                .listPriceMax(entity.getListPriceMax())
                .salePriceMin(entity.getSalePriceMin())
                .salePriceMax(entity.getSalePriceMax())
                .salesCount(entity.getSalesCount() != null ? entity.getSalesCount() : 0L)
                .stockQty(entity.getStockQty() != null ? entity.getStockQty() : 0L)
                .favoriteCount(entity.getFavoriteCount() != null ? entity.getFavoriteCount() : 0L)
                .reviewCount(entity.getReviewCount() != null ? entity.getReviewCount() : 0L)
                .createdAt(entity.getCreatedAt())
                .updatedAt(entity.getUpdatedAt())
                .createdBy(entity.getCreatedBy())               // audit
                .storeId(entity.getStoreId())                   // owner（② re-key，D7 Stage 2 更名）
                .version(entity.getVersion() != null ? entity.getVersion() : 0L)
                .build();

        // 轉換 gallery
        if (entity.getGallery() != null) {
            List<GalleryItemDTO> galleryDTOs = entity.getGallery().stream()
                    .map(g -> GalleryItemDTO.builder()
                    .assetId(g.getAssetId())
                    .objectKey(g.getObjectKey())
                    .build())
                    .collect(Collectors.toList());
            dto.setGallery(galleryDTOs);
        } else {
            dto.setGallery(new ArrayList<>());
        }

        // 轉換 certifications
        if (entity.getCertifications() != null) {
            List<CertificationDTO> certDTOs = entity.getCertifications().stream()
                    .map(c -> CertificationDTO.builder()
                    .name(c.getName())
                    .certNo(c.getCertNo())
                    .build())
                    .collect(Collectors.toList());
            dto.setCertifications(certDTOs);
        } else {
            dto.setCertifications(new ArrayList<>());
        }

        // 配送溫層 / 運送方式：改從 product.shipping_methods 反正規化欄位讀（02 §5.3.6 / §5.13a）
        List<ShippingMethodDTO> allMethods = new ArrayList<>();
        Set<String> allTemps = new LinkedHashSet<>();
        if (entity.getShippingMethods() != null) {
            for (ProductSkuEntity.ShippingMethodItem sm : entity.getShippingMethods()) {
                if (sm.getTemperature() != null) {
                    allTemps.add(sm.getTemperature());
                }
                allMethods.add(ShippingMethodDTO.builder()
                        .type(sm.getType())
                        .temperature(sm.getTemperature())
                        .freeThreshold(sm.getFreeThreshold())
                        .feePayer(sm.getFeePayer())
                        .build());
            }
        }
        dto.setDeliveryTemp(new ArrayList<>(allTemps));
        dto.setShippingMethods(allMethods);

        // 轉換 skus
        if (skus != null) {
            List<ProductSkuDTO> skuDTOs = skus.stream()
                    .map(skuConverter::toDTO)
                    .collect(Collectors.toList());
            dto.setSkus(skuDTOs);
        } else {
            dto.setSkus(new ArrayList<>());
        }

        return dto;
    }

    /**
     * ProductEntity → ProductListItemDTO（不含 skus）
     */
    public ProductListItemDTO toListItemDTO(ProductEntity entity) {
        ProductListItemDTO dto = ProductListItemDTO.builder()
                .id(entity.getId().toString())
                .name(entity.getName())
                .rating(entity.getRating())
                .status(entity.getStatus())
                .categoryId(entity.getCategoryLv2())
                .origin(entity.getOrigin())
                .priceMin(entity.getPriceMin())
                .priceMax(entity.getPriceMax())
                .listPriceMin(entity.getListPriceMin())
                .listPriceMax(entity.getListPriceMax())
                .salePriceMin(entity.getSalePriceMin())
                .salePriceMax(entity.getSalePriceMax())
                .salesCount(entity.getSalesCount() != null ? entity.getSalesCount() : 0L)
                .stockQty(entity.getStockQty() != null ? entity.getStockQty() : 0L)
                .favoriteCount(entity.getFavoriteCount() != null ? entity.getFavoriteCount() : 0L)
                .reviewCount(entity.getReviewCount() != null ? entity.getReviewCount().intValue() : 0)
                .createdAt(entity.getCreatedAt())
                .storeId(entity.getStoreId())
                .version(entity.getVersion() != null ? entity.getVersion() : 0L)
                .build();

        // 轉換 gallery
        if (entity.getGallery() != null) {
            List<GalleryItemDTO> galleryDTOs = entity.getGallery().stream()
                    .map(g -> GalleryItemDTO.builder()
                    .assetId(g.getAssetId())
                    .objectKey(g.getObjectKey())
                    .build())
                    .collect(Collectors.toList());
            dto.setGallery(galleryDTOs);
        } else {
            dto.setGallery(new ArrayList<>());
        }

        // 轉換 certifications（02 §5.3.4，商品卡 badge）
        if (entity.getCertifications() != null) {
            List<CertificationDTO> certDTOs = entity.getCertifications().stream()
                    .map(c -> CertificationDTO.builder()
                            .name(c.getName())
                            .certNo(c.getCertNo())
                            .build())
                    .collect(Collectors.toList());
            dto.setCertifications(certDTOs);
        } else {
            dto.setCertifications(new ArrayList<>());
        }

        // 反正規化欄位：shipping_methods（02 §3.2 / §5.13a）
        if (entity.getShippingMethods() != null) {
            dto.setShippingMethods(new ArrayList<>(entity.getShippingMethods()));
        } else {
            dto.setShippingMethods(new ArrayList<>());
        }

        // deliveryTemp 派生（02 §5.3.6）— 從 shippingMethods 取 distinct temperature
        if (dto.getShippingMethods() != null && !dto.getShippingMethods().isEmpty()) {
            dto.setDeliveryTemp(dto.getShippingMethods().stream()
                    .map(ProductSkuEntity.ShippingMethodItem::getTemperature)
                    .filter(t -> t != null && !t.isEmpty())
                    .distinct()
                    .sorted()
                    .toList());
        } else {
            dto.setDeliveryTemp(List.of());
        }

        return dto;
    }

    // ========== Description 轉換方法 ==========

    /**
     * ProductDescriptionDTO → ProductEntity.ProductDescription
     */
    public ProductEntity.ProductDescription convertDescriptionToEntity(ProductDescriptionDTO dto) {
        if (dto == null) {
            return null;
        }

        ProductEntity.ProductDescription description = new ProductEntity.ProductDescription();
        description.setVersion(dto.getVersion());

        if (dto.getBlocks() != null) {
            List<ProductEntity.DescriptionBlock> blocks = dto.getBlocks().stream()
                    .map(this::convertBlockToEntity)
                    .collect(Collectors.toList());
            description.setBlocks(blocks);
        } else {
            description.setBlocks(Collections.emptyList());
        }

        return description;
    }

    /**
     * DescriptionBlockDTO → ProductEntity.DescriptionBlock
     */
    private ProductEntity.DescriptionBlock convertBlockToEntity(DescriptionBlockDTO dto) {
        ProductEntity.DescriptionBlock block = new ProductEntity.DescriptionBlock();
        block.setType(dto.getType());
        block.setId(dto.getId());

        // text 類型欄位
        block.setContent(dto.getContent());
        if (dto.getStyleJson() != null && !dto.getStyleJson().isBlank()) {
            try {
                Map<String, Object> style = objectMapper.readValue(
                        dto.getStyleJson(),
                        new TypeReference<Map<String, Object>>() {}
                );
                block.setStyle(style);
            } catch (JsonProcessingException e) {
                // 解析失敗時保留空的 style
                block.setStyle(Collections.emptyMap());
            }
        }

        // image 類型欄位
        block.setStagedToken(dto.getStagedToken());
        block.setAssetId(dto.getAssetId());
        block.setObjectKey(dto.getObjectKey());
        block.setWidth(dto.getWidth());
        block.setHeight(dto.getHeight());
        block.setAlt(dto.getAlt());

        // inline-run 模型欄位（02 §3.6）
        if (dto.getRuns() != null) {
            block.setRuns(dto.getRuns().stream()
                    .map(this::convertRunToEntity)
                    .collect(Collectors.toList()));
        }
        block.setTableJson(dto.getTableJson());
        block.setListJson(dto.getListJson());

        return block;
    }

    /**
     * InlineRunDTO → ProductEntity.InlineRun
     */
    private ProductEntity.InlineRun convertRunToEntity(InlineRunDTO dto) {
        ProductEntity.InlineRun run = new ProductEntity.InlineRun();
        run.setText(dto.getText());
        run.setBold(dto.getBold());
        run.setItalic(dto.getItalic());
        run.setUnderline(dto.getUnderline());
        run.setColor(dto.getColor());
        run.setHref(dto.getHref());
        return run;
    }

    /**
     * ProductEntity.ProductDescription → ProductDescriptionDTO
     */
    public ProductDescriptionDTO convertDescriptionToDTO(ProductEntity.ProductDescription entity) {
        if (entity == null) {
            return null;
        }

        ProductDescriptionDTO dto = new ProductDescriptionDTO();
        dto.setVersion(entity.getVersion());

        if (entity.getBlocks() != null) {
            List<DescriptionBlockDTO> blocks = entity.getBlocks().stream()
                    .map(this::convertBlockToDTO)
                    .collect(Collectors.toList());
            dto.setBlocks(blocks);
        } else {
            dto.setBlocks(Collections.emptyList());
        }

        return dto;
    }

    /**
     * ProductEntity.DescriptionBlock → DescriptionBlockDTO
     * 注意：url 不在此處組裝，由 Service 層透過 CDN_BASE + object_key 組合
     */
    private DescriptionBlockDTO convertBlockToDTO(ProductEntity.DescriptionBlock entity) {
        DescriptionBlockDTO dto = new DescriptionBlockDTO();
        dto.setType(entity.getType());
        dto.setId(entity.getId());

        // text 類型欄位
        dto.setContent(entity.getContent());
        if (entity.getStyle() != null && !entity.getStyle().isEmpty()) {
            try {
                String styleJson = objectMapper.writeValueAsString(entity.getStyle());
                dto.setStyleJson(styleJson);
            } catch (JsonProcessingException e) {
                // 序列化失敗時保留空
                dto.setStyleJson(null);
            }
        }

        // image 類型欄位（不含 url，url 由 Service 層組裝）
        dto.setAssetId(entity.getAssetId());
        dto.setObjectKey(entity.getObjectKey());
        dto.setWidth(entity.getWidth());
        dto.setHeight(entity.getHeight());
        dto.setAlt(entity.getAlt());

        // inline-run 模型欄位（02 §3.6）
        if (entity.getRuns() != null) {
            dto.setRuns(entity.getRuns().stream()
                    .map(this::convertRunToDTO)
                    .collect(Collectors.toList()));
        }
        dto.setTableJson(entity.getTableJson());
        dto.setListJson(entity.getListJson());

        return dto;
    }

    /**
     * ProductEntity.InlineRun → InlineRunDTO
     */
    private InlineRunDTO convertRunToDTO(ProductEntity.InlineRun entity) {
        return InlineRunDTO.builder()
                .text(entity.getText())
                .bold(entity.getBold())
                .italic(entity.getItalic())
                .underline(entity.getUnderline())
                .color(entity.getColor())
                .href(entity.getHref())
                .build();
    }
}
