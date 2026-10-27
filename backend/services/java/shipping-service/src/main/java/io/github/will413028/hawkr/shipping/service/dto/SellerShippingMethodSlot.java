package io.github.will413028.hawkr.shipping.service.dto;

/**
 * 賣家配送方式槽位 DTO
 *
 * 用於 ListSellerShippingMethods API 回傳固定 4 個槽位（宅配 / 超商常溫 / 超商冷凍 / Uber）
 */
public record SellerShippingMethodSlot(
        String shippingMethod,
        String goodsType,
        String displayName,
        String description,
        boolean available,
        boolean onboardingRequired,
        String onboardingStatus,
        Long onboardingId,
        String enabled        // "true" / "false" / ""（未建店）
) {}
