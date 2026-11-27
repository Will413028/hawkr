package io.github.will413028.hawkr.product.converter;

import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.product.dto.request.*;
import io.github.will413028.hawkr.product.dto.response.*;
import io.github.will413028.hawkr.product.grpc.*;
import org.springframework.stereotype.Component;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.math.BigDecimal;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Collectors;

/**
 * Proto Messages 與 DTO 之間的轉換器
 * 負責 gRPC Handler 層的資料轉換
 */
@Component
public class ProductProtoConverter {


    /**
     * CreateProductRequest (Proto) → CreateProductDTO
     * category_id 從 string 轉為 Long
     */
    public CreateProductDTO toCreateDTO(CreateProductRequest request) {
        CreateProductDTO dto = new CreateProductDTO();
        dto.setIdempotencyKey(request.getIdempotencyKey());
        dto.setName(request.getName());
        dto.setCategoryId(parseLong(request.getCategoryId()));
        dto.setDescription(toDescriptionDTO(request.getDescription()));
        dto.setStoreId(request.getStoreId());
        dto.setOrigin(request.getOrigin());
        dto.setStatus(request.getStatus());

        if (request.getImagesCount() > 0) {
            List<ProductImageDTO> images = request.getImagesList().stream()
                    .map(this::toImageDTO)
                    .collect(Collectors.toList());
            dto.setImages(images);
        }

        if (request.getSkusCount() > 0) {
            List<ProductSkuDTO> skus = request.getSkusList().stream()
                    .map(this::toSkuDTO)
                    .collect(Collectors.toList());
            dto.setSkus(skus);
        }

        if (request.getCertificationsCount() > 0) {
            List<CertificationDTO> certs = request.getCertificationsList().stream()
                    .map(this::toCertificationDTO)
                    .collect(Collectors.toList());
            dto.setCertifications(certs);
        }

        if (request.getHashtagsCount() > 0) {
            dto.setHashtags(new java.util.ArrayList<>(request.getHashtagsList()));
        }

        dto.setDelegated(request.hasDelegation());

        return dto;
    }

    /**
     * UpdateProductRequest (Proto) → UpdateProductDTO
     * category_id 從 string 轉為 Long
     */
    public UpdateProductDTO toUpdateDTO(UpdateProductRequest request) {
        UpdateProductDTO dto = new UpdateProductDTO();
        dto.setIdempotencyKey(request.getIdempotencyKey());
        dto.setProductId(request.getProductId());
        dto.setStoreId(request.getStoreId());
        dto.setName(request.getName());
        dto.setCategoryId(parseLong(request.getCategoryId()));
        dto.setDescription(toDescriptionDTO(request.getDescription()));
        dto.setOrigin(request.getOrigin());
        dto.setStatus(request.getStatus());
        dto.setExpectedVersion(request.getExpectedVersion());

        if (request.getImagesCount() > 0) {
            List<ProductImageDTO> images = request.getImagesList().stream()
                    .map(this::toImageDTO)
                    .collect(Collectors.toList());
            dto.setImages(images);
        }

        if (request.getSkusCount() > 0) {
            List<ProductSkuDTO> skus = request.getSkusList().stream()
                    .map(this::toSkuDTO)
                    .collect(Collectors.toList());
            dto.setSkus(skus);
        }

        dto.setUpdateCertifications(request.getUpdateCertifications());
        if (request.getUpdateCertifications() || request.getCertificationsCount() > 0) {
            List<CertificationDTO> certs = request.getCertificationsList().stream()
                    .map(this::toCertificationDTO)
                    .collect(Collectors.toList());
            dto.setCertifications(certs);
        }

        dto.setDelegated(request.hasDelegation());

        dto.setUpdateHashtags(request.getUpdateHashtags());
        if (request.getHashtagsCount() > 0) {
            dto.setHashtags(new java.util.ArrayList<>(request.getHashtagsList()));
        }

        return dto;
    }

    /**
     * GetProductListRequest (Proto) → ProductQueryDTO
     *
     * 狀態篩選由 Gateway 入口控制：
     * - 公開 API：Gateway 強制 status=4004（ACTIVE）
     * - 賣家 API：Gateway 傳入 ownerId，status 可選
     *
     * category_id, page, page_size 從 string 轉為數字
     */
    public ProductQueryDTO toQueryDTO(GetProductListRequest request) {
        List<Long> categoryIds = null;
        int rawCatCount = request.getCategoryIdCount();
        if (rawCatCount > 0) {
            java.util.LinkedHashSet<Long> set = new java.util.LinkedHashSet<>();
            for (int i = 0; i < rawCatCount; i++) {
                String raw = request.getCategoryId(i);
                if (raw == null) continue;
                String t = raw.trim();
                if (t.isEmpty() || "0".equals(t)) continue;
                Long v = parseLong(t);
                if (v == null || v <= 0) {
                    throw new IllegalArgumentException("category_id 含非法值: " + raw);
                }
                set.add(v);
            }
            if (!set.isEmpty()) {
                if (set.size() > 50) {
                    throw new IllegalArgumentException("category_id 上限為 50 個，目前 " + set.size());
                }
                categoryIds = new java.util.ArrayList<>(set);
            }
        }

        Integer page = parseIntTreatingZeroAsAbsent(request.getPage());
        Integer pageSize = parseIntTreatingZeroAsAbsent(request.getPageSize());

        Long storeId = parseLong(request.getStoreId());

        List<String> hashtags = request.getHashtagsCount() > 0
                ? new java.util.ArrayList<>(request.getHashtagsList())
                : null;
        boolean matchAll = request.getHashtagMatchMode()
                == io.github.will413028.hawkr.product.grpc.HashtagMatchMode.HASHTAG_MATCH_ALL;

        List<String> deliveryTempFilter = request.getDeliveryTempFilterCount() > 0
                ? new java.util.ArrayList<>(request.getDeliveryTempFilterList())
                : null;
        List<String> certificationTypes = request.getCertificationTypesCount() > 0
                ? new java.util.ArrayList<>(request.getCertificationTypesList())
                : null;
        Long priceMin = parseLong(request.getPriceMinFilter());
        Long priceMax = parseLong(request.getPriceMaxFilter());
        List<Long> storeIds = null;
        if (request.getStoreIdsCount() > 0) {
            storeIds = new java.util.ArrayList<>();
            for (String s : request.getStoreIdsList()) {
                Long v = parseLong(s);
                if (v != null && v > 0) {
                    storeIds.add(v);
                }
            }
            if (storeIds.isEmpty()) {
                storeIds = null;
            }
        }

        return ProductQueryDTO.builder()
                .categoryIds(categoryIds)
                .keyword(request.getKeyword().isEmpty() ? null : request.getKeyword())
                .page(page != null && page > 0 ? page : 1)
                .pageSize(pageSize != null && pageSize > 0 ? pageSize : 20)
                .sort(request.getSort().isEmpty() ? null : request.getSort())
                .status(request.getStatus().isEmpty() ? null : request.getStatus())
                .ownerStoreId(request.getOwnerStoreId() > 0 ? request.getOwnerStoreId() : null)
                .storeId(storeId != null && storeId > 0 ? storeId : null)
                .hashtags(hashtags)
                .hashtagMatchAll(matchAll)
                .deliveryTempFilter(deliveryTempFilter)
                .certificationTypes(certificationTypes)
                .priceMinFilter(priceMin != null && priceMin >= 0 ? priceMin : null)
                .priceMaxFilter(priceMax != null && priceMax >= 0 ? priceMax : null)
                .storeIds(storeIds)
                .sellerCityCode(request.getSellerCityCode().isEmpty() ? null : request.getSellerCityCode())
                .followedProductOnly(request.getFollowedProductOnly())
                .followedSellerOnly(request.getFollowedSellerOnly())
                .viewerBuyerId(request.getViewerBuyerId() > 0 ? request.getViewerBuyerId() : null)
                .build();
    }


    /**
     * ProductDetailDTO → ProductDetail (Proto)
     * 所有數字欄位轉為 string
     */
    public ProductDetail toProductDetailProto(ProductDetailDTO dto) {
        ProductDetail.Builder builder = ProductDetail.newBuilder()
                .setId(dto.getId())
                .setName(dto.getName() != null ? dto.getName() : "")
                .setRating(dto.getRating() != null ? dto.getRating().toPlainString() : "0")
                .setStatus(dto.getStatus() != null ? dto.getStatus() : "")
                .setCategoryId(dto.getCategoryId() != null ? String.valueOf(dto.getCategoryId()) : "0")
                .setOrigin(dto.getOrigin() != null ? dto.getOrigin() : "")
                .setDescription(toDescriptionProto(dto.getDescription()))
                .setPriceMin(formatPrice(dto.getPriceMin()))
                .setPriceMax(formatPrice(dto.getPriceMax()))
                .setListPriceMin(formatPrice(dto.getListPriceMin()))
                .setListPriceMax(formatPrice(dto.getListPriceMax()))
                .setSalePriceMin(formatPrice(dto.getSalePriceMin()))
                .setSalePriceMax(formatPrice(dto.getSalePriceMax()))
                .setSalesCount(dto.getSalesCount() != null ? String.valueOf(dto.getSalesCount()) : "0")
                .setStockQty(dto.getStockQty() != null ? String.valueOf(dto.getStockQty()) : "0")
                .setFavoriteCount(dto.getFavoriteCount() != null ? String.valueOf(dto.getFavoriteCount()) : "0")
                .setReviewCount(dto.getReviewCount() != null ? String.valueOf(dto.getReviewCount()) : "0")
                .setPublicationState(dto.getPublicationState() != null ? dto.getPublicationState() : "")
                .setPurchasable(Boolean.TRUE.equals(dto.getPurchasable()))
                .setCreatedAt(String.valueOf(toTimestamp(dto.getCreatedAt())))
                .setUpdatedAt(String.valueOf(toTimestamp(dto.getUpdatedAt())))
                .setCreatedBy(dto.getCreatedBy() != null ? String.valueOf(dto.getCreatedBy()) : "0")
                .setVersion(dto.getVersion() != null ? String.valueOf(dto.getVersion()) : "0")
                .setCategoryName(dto.getCategoryName() != null ? dto.getCategoryName() : "");

        List<String> imageKeys = new ArrayList<>();
        if (dto.getGallery() != null) {
            List<GalleryItem> galleryItems = dto.getGallery().stream()
                    .filter(g -> g != null)
                    .map(g -> {
                        String objectKey = g.getObjectKey() != null ? g.getObjectKey() : "";
                        imageKeys.add(objectKey);
                        return GalleryItem.newBuilder()
                                .setAssetId(g.getAssetId() != null ? g.getAssetId() : "")
                                .setObjectKey(objectKey)
                                .build();
                    })
                    .collect(Collectors.toList());
            builder.addAllGallery(galleryItems);
        }

        if (!imageKeys.isEmpty()) {
            builder.setImageKey(imageKeys.get(0));
        }
        builder.addAllImageKeys(imageKeys);

        if (dto.getCertifications() != null) {
            List<Certification> certs = dto.getCertifications().stream()
                    .filter(c -> c != null)
                    .map(c -> Certification.newBuilder()
                            .setName(c.getName() != null ? c.getName() : "")
                            .setCertNo(c.getCertNo() != null ? c.getCertNo() : "")
                            .build())
                    .collect(Collectors.toList());
            builder.addAllCertifications(certs);
        }

        if (dto.getSkus() != null) {
            List<ProductSku> skus = dto.getSkus().stream()
                    .filter(v -> v != null)
                    .map(this::toSkuProto)
                    .collect(Collectors.toList());
            builder.addAllSkus(skus);
        }

        if (dto.getCategoryLv3Id() != null) {
            builder.setCategoryLv3Id(String.valueOf(dto.getCategoryLv3Id()));
        }
        if (dto.getCategoryLv3Name() != null) {
            builder.setCategoryLv3Name(dto.getCategoryLv3Name());
        }
        if (dto.getCategoryLv2Id() != null) {
            builder.setCategoryLv2Id(String.valueOf(dto.getCategoryLv2Id()));
        }
        if (dto.getCategoryLv2Name() != null) {
            builder.setCategoryLv2Name(dto.getCategoryLv2Name());
        }
        if (dto.getCategoryLv1Id() != null) {
            builder.setCategoryLv1Id(String.valueOf(dto.getCategoryLv1Id()));
        }
        if (dto.getCategoryLv1Name() != null) {
            builder.setCategoryLv1Name(dto.getCategoryLv1Name());
        }

        if (dto.getDeliveryTemp() != null) {
            List<String> filteredDeliveryTemp = dto.getDeliveryTemp().stream()
                    .filter(d -> d != null)
                    .collect(Collectors.toList());
            builder.addAllDeliveryTemp(filteredDeliveryTemp);
        }

        if (dto.getShippingMethods() != null) {
            List<ShippingMethod> methods = dto.getShippingMethods().stream()
                    .filter(sm -> sm != null)
                    .map(sm -> ShippingMethod.newBuilder()
                            .setType(sm.getType() != null ? sm.getType() : "")
                            .setTemperature(sm.getTemperature() != null ? sm.getTemperature() : "")
                            .setFreeThreshold(sm.getFreeThreshold() != null ? sm.getFreeThreshold() : "")
                            .setFeePayer(sm.getFeePayer() != null ? sm.getFeePayer() : "")
                            .build())
                    .collect(Collectors.toList());
            builder.addAllShippingMethods(methods);
        }

        if (dto.getSeller() != null) {
            SellerInfo.Builder sellerBuilder = SellerInfo.newBuilder();
            if (dto.getSeller().getStoreId() != null) {
                sellerBuilder.setStoreId(String.valueOf(dto.getSeller().getStoreId()));
            }
            if (dto.getSeller().getName() != null) {
                sellerBuilder.setName(dto.getSeller().getName());
            }
            if (dto.getSeller().getRate() != null) {
                sellerBuilder.setRate(dto.getSeller().getRate());
            }
            if (dto.getSeller().getLogoKey() != null) {
                sellerBuilder.setLogoKey(dto.getSeller().getLogoKey());
            }
            if (dto.getSeller().getAddress() != null) {
                sellerBuilder.setAddress(dto.getSeller().getAddress());
            }
            if (dto.getSeller().getCity() != null) {
                sellerBuilder.setCity(dto.getSeller().getCity());
            }
            if (dto.getSeller().getDistrict() != null) {
                sellerBuilder.setDistrict(dto.getSeller().getDistrict());
            }
            if (dto.getSeller().getStoreCode() != null) {
                sellerBuilder.setStoreCode(dto.getSeller().getStoreCode());
            }
            builder.setSeller(sellerBuilder.build());
        }

        if (dto.getReviews() != null) {
            List<ReviewPreview> reviews = dto.getReviews().stream()
                    .filter(r -> r != null)
                    .map(r -> ReviewPreview.newBuilder()
                            .setReviewId(r.getReviewId() != null ? r.getReviewId() : "")
                            .setBuyerId(r.getBuyerId() != null ? String.valueOf(r.getBuyerId()) : "")
                            .setRating(r.getRating() != null ? r.getRating() : "")
                            .setContent(r.getContent() != null ? r.getContent() : "")
                            .setCreatedAt(r.getCreatedAt() != null ? String.valueOf(toTimestamp(r.getCreatedAt())) : "")
                            .build())
                    .collect(Collectors.toList());
            builder.addAllReviews(reviews);
        }

        if (dto.getHashtags() != null && !dto.getHashtags().isEmpty()) {
            builder.addAllHashtags(toHashtagItemProtos(dto.getHashtags()));
        }

        return builder.build();
    }

    /**
     * 共用：HashtagItemDTO → HashtagItem (Proto)。
     */
    public List<io.github.will413028.hawkr.product.grpc.HashtagItem> toHashtagItemProtos(
            List<io.github.will413028.hawkr.product.dto.response.HashtagItemDTO> dtos) {
        if (dtos == null || dtos.isEmpty()) return List.of();
        return dtos.stream()
                .filter(java.util.Objects::nonNull)
                .map(d -> io.github.will413028.hawkr.product.grpc.HashtagItem.newBuilder()
                        .setName(d.getName() != null ? d.getName() : "")
                        .setDisplayName(d.getDisplayName() != null ? d.getDisplayName() : "")
                        .setProductCount(d.getProductCount() != null ? d.getProductCount() : "0")
                        .build())
                .collect(Collectors.toList());
    }

    /**
     * ProductListItemDTO → ProductListItem (Proto)
     * 所有數字欄位轉為 string
     */
    public ProductListItem toProductListItemProto(ProductListItemDTO dto) {
        ProductListItem.Builder builder = ProductListItem.newBuilder()
                .setId(dto.getId())
                .setName(dto.getName() != null ? dto.getName() : "")
                .setRating(dto.getRating() != null ? dto.getRating().toPlainString() : "0")
                .setStatus(dto.getStatus() != null ? dto.getStatus() : "")
                .setCategoryId(dto.getCategoryId() != null ? String.valueOf(dto.getCategoryId()) : "0")
                .setCategoryName(dto.getCategoryName() != null ? dto.getCategoryName() : "")
                .setOrigin(dto.getOrigin() != null ? dto.getOrigin() : "")
                .setPriceMin(formatPrice(dto.getPriceMin()))
                .setPriceMax(formatPrice(dto.getPriceMax()))
                .setListPriceMin(formatPrice(dto.getListPriceMin()))
                .setListPriceMax(formatPrice(dto.getListPriceMax()))
                .setSalePriceMin(formatPrice(dto.getSalePriceMin()))
                .setSalePriceMax(formatPrice(dto.getSalePriceMax()))
                .setSalesCount(dto.getSalesCount() != null ? String.valueOf(dto.getSalesCount()) : "0")
                .setStockQty(dto.getStockQty() != null ? String.valueOf(dto.getStockQty()) : "0")
                .setFavoriteCount(dto.getFavoriteCount() != null ? String.valueOf(dto.getFavoriteCount()) : "0")
                .setCreatedAt(String.valueOf(toTimestamp(dto.getCreatedAt())))
                .setReviewCount(dto.getReviewCount() != null ? String.valueOf(dto.getReviewCount()) : "0")
                .setStoreId(dto.getStoreId() != null ? String.valueOf(dto.getStoreId()) : "")
                .setVersion(dto.getVersion() != null ? String.valueOf(dto.getVersion()) : "0")
                .setCategoryLv2Id(dto.getCategoryId() != null ? String.valueOf(dto.getCategoryId()) : "0")
                .setCategoryLv2Name(dto.getCategoryName() != null ? dto.getCategoryName() : "");

        if (dto.getSeller() != null) {
            SellerInfo.Builder sellerBuilder = SellerInfo.newBuilder();
            if (dto.getSeller().getStoreId() != null) {
                sellerBuilder.setStoreId(String.valueOf(dto.getSeller().getStoreId()));
            }
            if (dto.getSeller().getName() != null) {
                sellerBuilder.setName(dto.getSeller().getName());
            }
            if (dto.getSeller().getLogoKey() != null) {
                sellerBuilder.setLogoKey(dto.getSeller().getLogoKey());
            }
            if (dto.getSeller().getAddress() != null) {
                sellerBuilder.setAddress(dto.getSeller().getAddress());
            }
            if (dto.getSeller().getCity() != null) {
                sellerBuilder.setCity(dto.getSeller().getCity());
            }
            if (dto.getSeller().getDistrict() != null) {
                sellerBuilder.setDistrict(dto.getSeller().getDistrict());
            }
            if (dto.getSeller().getRate() != null) {
                sellerBuilder.setRate(dto.getSeller().getRate());
            }
            if (dto.getSeller().getStoreCode() != null) {
                sellerBuilder.setStoreCode(dto.getSeller().getStoreCode());
            }
            builder.setSeller(sellerBuilder.build());
        }

        List<String> listImageKeys = new ArrayList<>();
        if (dto.getGallery() != null) {
            List<GalleryItem> galleryItems = dto.getGallery().stream()
                    .map(g -> {
                        String objectKey = g.getObjectKey() != null ? g.getObjectKey() : "";
                        listImageKeys.add(objectKey);
                        return GalleryItem.newBuilder()
                                .setAssetId(g.getAssetId() != null ? g.getAssetId() : "")
                                .setObjectKey(objectKey)
                                .build();
                    })
                    .collect(Collectors.toList());
            builder.addAllGallery(galleryItems);
        }

        if (!listImageKeys.isEmpty()) {
            builder.setImageKey(listImageKeys.get(0));
        }
        builder.addAllImageKeys(listImageKeys);

        if (dto.getSkuNames() != null && !dto.getSkuNames().isEmpty()) {
            builder.addAllSkuNames(dto.getSkuNames());
        }

        if (dto.getHashtags() != null && !dto.getHashtags().isEmpty()) {
            builder.addAllHashtags(toHashtagItemProtos(dto.getHashtags()));
        }

        if (dto.getDeliveryTemp() != null && !dto.getDeliveryTemp().isEmpty()) {
            builder.addAllDeliveryTemp(dto.getDeliveryTemp());
        }

        if (dto.getShippingMethods() != null && !dto.getShippingMethods().isEmpty()) {
            List<ShippingMethod> methods = dto.getShippingMethods().stream()
                    .filter(java.util.Objects::nonNull)
                    .map(m -> ShippingMethod.newBuilder()
                            .setType(m.getType() != null ? m.getType() : "")
                            .setTemperature(m.getTemperature() != null ? m.getTemperature() : "")
                            .setFreeThreshold(m.getFreeThreshold() != null ? m.getFreeThreshold() : "")
                            .setFeePayer(m.getFeePayer() != null ? m.getFeePayer() : "")
                            .build())
                    .collect(Collectors.toList());
            builder.addAllShippingMethods(methods);
        }

        if (dto.getCertifications() != null) {
            List<Certification> certs = dto.getCertifications().stream()
                    .filter(c -> c != null)
                    .map(c -> Certification.newBuilder()
                            .setName(c.getName() != null ? c.getName() : "")
                            .setCertNo(c.getCertNo() != null ? c.getCertNo() : "")
                            .build())
                    .collect(Collectors.toList());
            builder.addAllCertifications(certs);
        }

        return builder.build();
    }


    /**
     * ProductImage (Proto) → ProductImageDTO
     */
    private ProductImageDTO toImageDTO(ProductImage image) {
        return ProductImageDTO.builder()
                .id(image.getId().isEmpty() ? null : image.getId())
                .stagedToken(image.getStagedToken().isEmpty() ? null : image.getStagedToken())
                .assetId(image.getAssetId().isEmpty() ? null : image.getAssetId())
                .build();
    }

    /**
     * ProductSku (Proto) → ProductSkuDTO
     * weight, stock_qty, length_cm, width_cm, height_cm 從 string 轉為數字
     */
    private ProductSkuDTO toSkuDTO(ProductSku sku) {
        ProductSkuDTO.ProductSkuDTOBuilder builder = ProductSkuDTO.builder()
                .id(sku.getId().isEmpty() ? null : sku.getId())
                .skuName(sku.getSkuName().isEmpty() ? null : sku.getSkuName())
                .weight(parseBigDecimal(sku.getWeight()))
                .unit(sku.getUnit().isEmpty() ? null : sku.getUnit())
                .price(parsePrice(sku.getPrice()))
                .salePrice(ProtoConversionUtils.parseLongOrNull(sku.getSalePrice()))
                .stockQty(ProtoConversionUtils.parseIntOrNull(sku.getStockQty()))
                .lengthCm(parseBigDecimal(sku.getLengthCm()))
                .widthCm(parseBigDecimal(sku.getWidthCm()))
                .heightCm(parseBigDecimal(sku.getHeightCm()))
                .expectedVersion(sku.getExpectedVersion().isEmpty() ? null : sku.getExpectedVersion());

        if (sku.getShippingMethodsCount() > 0) {
            List<ShippingMethodDTO> methods = sku.getShippingMethodsList().stream()
                    .map(sm -> ShippingMethodDTO.builder()
                            .type(sm.getType())
                            .temperature(sm.getTemperature())
                            .freeThreshold(sm.getFreeThreshold())
                            .feePayer(sm.getFeePayer())
                            .build())
                    .collect(Collectors.toList());
            builder.shippingMethods(methods);
        }

        return builder.build();
    }

    /**
     * Certification (Proto) → CertificationDTO
     */
    private CertificationDTO toCertificationDTO(Certification cert) {
        return CertificationDTO.builder()
                .name(cert.getName())
                .certNo(cert.getCertNo())
                .build();
    }

    /**
     * ProductSkuDTO → ProductSku (Proto)
     * weight, stock_qty, length_cm, width_cm, height_cm 從數字轉為 string
     */
    private ProductSku toSkuProto(ProductSkuDTO dto) {
        ProductSku.Builder builder = ProductSku.newBuilder()
                .setId(dto.getId() != null ? dto.getId() : "")
                .setSkuName(dto.getSkuName() != null ? dto.getSkuName() : "")
                .setWeight(dto.getWeight() != null ? dto.getWeight().toPlainString() : "0")
                .setUnit(dto.getUnit() != null ? dto.getUnit() : "")
                .setPrice(formatPrice(dto.getPrice()))
                .setSalePrice(dto.getSalePrice() != null ? String.valueOf(dto.getSalePrice()) : "")
                .setStockQty(dto.getStockQty() != null ? String.valueOf(dto.getStockQty()) : "0")
                .setLengthCm(dto.getLengthCm() != null ? dto.getLengthCm().toPlainString() : "0")
                .setWidthCm(dto.getWidthCm() != null ? dto.getWidthCm().toPlainString() : "0")
                .setHeightCm(dto.getHeightCm() != null ? dto.getHeightCm().toPlainString() : "0")
                .setVersion(dto.getVersion() != null ? dto.getVersion() : "");

        if (dto.getShippingMethods() != null) {
            List<ShippingMethod> methods = dto.getShippingMethods().stream()
                    .filter(sm -> sm != null)
                    .map(sm -> ShippingMethod.newBuilder()
                            .setType(sm.getType() != null ? sm.getType() : "")
                            .setTemperature(sm.getTemperature() != null ? sm.getTemperature() : "")
                            .setFreeThreshold(sm.getFreeThreshold() != null ? sm.getFreeThreshold() : "")
                            .setFeePayer(sm.getFeePayer() != null ? sm.getFeePayer() : "")
                            .build())
                    .collect(Collectors.toList());
            builder.addAllShippingMethods(methods);
        }

        return builder.build();
    }

    /**
     * 價格字串 → Long（新台幣元）
     * Proto string → Java Long
     * "0" 視為 null（無價格）
     */
    private Long parsePrice(String priceStr) {
        if ("0".equals(priceStr)) return null;
        return ProtoConversionUtils.parseLongOrNull(priceStr);
    }

    /**
     * Long → 價格字串（新台幣元，數字字串格式）
     * Java Long → Proto string
     */
    private String formatPrice(Long price) {
        return ProtoConversionUtils.formatAmount(price);
    }

    /**
     * 字串 → Long（接收 Proto string 欄位）
     * 空字串或 "0" 回傳 null
     */
    private Long parseLong(String value) {
        if ("0".equals(value)) return null;
        return ProtoConversionUtils.parseLongOrNull(value);
    }

    /**
     * 字串 → Integer，且**把 "0" 視為「未提供」**（接收 Proto string 欄位）。
     *
     * ⚠️ 只適用於「0 本來就不是合法值」的欄位：分頁的 page／pageSize（0 頁無意義），
     * 以及描述區塊圖片的 width／height（0 像素無意義）。
     *
     * <p><b>不要拿它處理庫存、數量、金額這類 0 是真實值的欄位。</b>
     * `ProductSkuConverter.updateEntity` 對 null 的語意是「跳過不更新」，所以一旦用了
     * 本方法，「把值設成 0」就會靜默變成 no-op —— 不報錯、不轉紅、稽核還記 SUCCEEDED。
     * 庫存欄位曾經就是這樣壞掉的（賣家無法把 SKU 設為售完），改用
     * {@code ProtoConversionUtils.parseIntOrNull}（空字串才回 null）。
     */
    private Integer parseIntTreatingZeroAsAbsent(String value) {
        if ("0".equals(value)) return null;
        return ProtoConversionUtils.parseIntOrNull(value);
    }

    /**
     * 字串 → BigDecimal（接收 Proto string 欄位）
     * 空字串或 "0" 回傳 BigDecimal.ZERO
     */
    private BigDecimal parseBigDecimal(String value) {
        if (value == null || value.isEmpty() || value.equals("0")) {
            return BigDecimal.ZERO;
        }
        try {
            return new BigDecimal(value);
        } catch (NumberFormatException e) {
            throw new IllegalArgumentException("無效的數字格式: " + value, e);
        }
    }

    /**
     * LocalDateTime → Unix timestamp (秒)
     */
    private long toTimestamp(LocalDateTime dateTime) {
        if (dateTime == null) {
            return 0L;
        }
        return DateTimeUtils.toEpochSeconds(dateTime);
    }

    /**
     * Unix timestamp (秒) → LocalDateTime
     */
    private LocalDateTime fromTimestamp(long timestamp) {
        if (timestamp == 0) {
            return null;
        }
        return DateTimeUtils.fromEpochSeconds(timestamp);
    }


    /**
     * ProductDescription (Proto) → ProductDescriptionDTO
     */
    private ProductDescriptionDTO toDescriptionDTO(io.github.will413028.hawkr.product.grpc.ProductDescription proto) {
        if (proto == null || proto.getSerializedSize() == 0) {
            return null;
        }

        ProductDescriptionDTO dto = new ProductDescriptionDTO();
        dto.setVersion(proto.getVersion().isEmpty() ? null : proto.getVersion());

        if (proto.getBlocksCount() > 0) {
            List<DescriptionBlockDTO> blocks = proto.getBlocksList().stream()
                    .map(this::toDescriptionBlockDTO)
                    .collect(Collectors.toList());
            dto.setBlocks(blocks);
        }

        return dto;
    }

    /**
     * DescriptionBlock (Proto) → DescriptionBlockDTO
     * width, height 從 string 轉為 Integer
     */
    private DescriptionBlockDTO toDescriptionBlockDTO(io.github.will413028.hawkr.product.grpc.DescriptionBlock proto) {
        DescriptionBlockDTO dto = new DescriptionBlockDTO();
        dto.setType(proto.getType().isEmpty() ? null : proto.getType());
        dto.setId(proto.getId().isEmpty() ? null : proto.getId());

        dto.setContent(proto.getContent().isEmpty() ? null : proto.getContent());
        dto.setStyleJson(proto.getStyleJson().isEmpty() ? null : proto.getStyleJson());

        dto.setStagedToken(proto.getStagedToken().isEmpty() ? null : proto.getStagedToken());
        dto.setAssetId(proto.getAssetId().isEmpty() ? null : proto.getAssetId());
        dto.setObjectKey(proto.getObjectKey().isEmpty() ? null : proto.getObjectKey());
        dto.setUrl(proto.getUrl().isEmpty() ? null : proto.getUrl());
        dto.setWidth(parseIntTreatingZeroAsAbsent(proto.getWidth()));
        dto.setHeight(parseIntTreatingZeroAsAbsent(proto.getHeight()));
        dto.setAlt(proto.getAlt().isEmpty() ? null : proto.getAlt());

        if (proto.getRunsCount() > 0) {
            dto.setRuns(proto.getRunsList().stream()
                    .map(this::toInlineRunDTO)
                    .collect(Collectors.toList()));
        }
        dto.setTableJson(proto.getTableJson().isEmpty() ? null : proto.getTableJson());
        dto.setListJson(proto.getListJson().isEmpty() ? null : proto.getListJson());

        return dto;
    }

    /**
     * InlineRun (Proto) → InlineRunDTO
     * proto 為原生 bool（預設 false）→ DTO 用 Boolean，false 轉 null 維持正規形
     */
    private InlineRunDTO toInlineRunDTO(io.github.will413028.hawkr.product.grpc.InlineRun proto) {
        return InlineRunDTO.builder()
                .text(proto.getText())
                .bold(proto.getBold() ? Boolean.TRUE : null)
                .italic(proto.getItalic() ? Boolean.TRUE : null)
                .underline(proto.getUnderline() ? Boolean.TRUE : null)
                .color(proto.getColor().isEmpty() ? null : proto.getColor())
                .href(proto.getHref().isEmpty() ? null : proto.getHref())
                .build();
    }

    /**
     * ProductDescriptionDTO → ProductDescription (Proto)
     */
    private io.github.will413028.hawkr.product.grpc.ProductDescription toDescriptionProto(ProductDescriptionDTO dto) {
        if (dto == null) {
            return io.github.will413028.hawkr.product.grpc.ProductDescription.getDefaultInstance();
        }

        io.github.will413028.hawkr.product.grpc.ProductDescription.Builder builder =
                io.github.will413028.hawkr.product.grpc.ProductDescription.newBuilder()
                        .setVersion(dto.getVersion() != null ? dto.getVersion() : "");

        if (dto.getBlocks() != null) {
            List<io.github.will413028.hawkr.product.grpc.DescriptionBlock> blocks = dto.getBlocks().stream()
                    .map(this::toDescriptionBlockProto)
                    .collect(Collectors.toList());
            builder.addAllBlocks(blocks);
        }

        return builder.build();
    }

    /**
     * DescriptionBlockDTO → DescriptionBlock (Proto)
     * width, height 從 Integer 轉為 string
     * 注意：回應時需要組裝 URL（CDN_BASE + object_key）
     */
    private io.github.will413028.hawkr.product.grpc.DescriptionBlock toDescriptionBlockProto(DescriptionBlockDTO dto) {
        io.github.will413028.hawkr.product.grpc.DescriptionBlock.Builder builder =
                io.github.will413028.hawkr.product.grpc.DescriptionBlock.newBuilder()
                        .setType(dto.getType() != null ? dto.getType() : "")
                        .setId(dto.getId() != null ? dto.getId() : "");

        builder.setContent(dto.getContent() != null ? dto.getContent() : "");
        builder.setStyleJson(dto.getStyleJson() != null ? dto.getStyleJson() : "");

        builder.setAssetId(dto.getAssetId() != null ? dto.getAssetId() : "");
        builder.setObjectKey(dto.getObjectKey() != null ? dto.getObjectKey() : "");

        builder.setWidth(dto.getWidth() != null ? String.valueOf(dto.getWidth()) : "0");
        builder.setHeight(dto.getHeight() != null ? String.valueOf(dto.getHeight()) : "0");
        builder.setAlt(dto.getAlt() != null ? dto.getAlt() : "");

        if (dto.getRuns() != null) {
            for (InlineRunDTO run : dto.getRuns()) {
                builder.addRuns(toInlineRunProto(run));
            }
        }
        builder.setTableJson(dto.getTableJson() != null ? dto.getTableJson() : "");
        builder.setListJson(dto.getListJson() != null ? dto.getListJson() : "");

        return builder.build();
    }

    /**
     * InlineRunDTO → InlineRun (Proto)
     * DTO 為 Boolean（null=false）→ proto 原生 bool，僅 true 時 set
     */
    private io.github.will413028.hawkr.product.grpc.InlineRun toInlineRunProto(InlineRunDTO dto) {
        io.github.will413028.hawkr.product.grpc.InlineRun.Builder b =
                io.github.will413028.hawkr.product.grpc.InlineRun.newBuilder()
                        .setText(dto.getText() != null ? dto.getText() : "");
        if (Boolean.TRUE.equals(dto.getBold())) b.setBold(true);
        if (Boolean.TRUE.equals(dto.getItalic())) b.setItalic(true);
        if (Boolean.TRUE.equals(dto.getUnderline())) b.setUnderline(true);
        if (dto.getColor() != null) b.setColor(dto.getColor());
        if (dto.getHref() != null) b.setHref(dto.getHref());
        return b.build();
    }

}
