package io.github.will413028.hawkr.productimport.helper;

import io.github.will413028.hawkr.common.utils.code.CertificationType;
import io.github.will413028.hawkr.common.utils.code.CodeMeta;
import io.github.will413028.hawkr.common.utils.code.CodeMetaValidator;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingTemperature;
import io.github.will413028.hawkr.common.utils.code.WeightUnit;
import io.grpc.Status;

/**
 * CodeMeta NAME / value 正規化 helper（40 號設計書 §11.1 / §11.1a）。
 *
 * <p>CSV 與前端為了可讀性使用 enum NAME（KG / AMBIENT / ORGANIC），
 * 但依 docs/開發規範/Java 微服務開發規範.md §2.1 / §6.4，DB 與跨服務 gRPC 契約一律存 enum value
 * （3301 / 3201 / 3701 …）。本 helper 在寫入邊界把 NAME（或 value）統一
 * 正規化為 value，避免 product-service 端 CodeMetaValidator 失敗。
 *
 * <p>提供兩種變體（依 §11.1a）：
 * <ul>
 *   <li><b>forgiving</b>：{@code weightUnitToValue} / {@code shippingTemperatureToValue}
 *       / {@code certificationTypeToValue}。invalid → null。給 CSV 解析路徑使用
 *       （L2 已先過 ENUM_INVALID blocking error，helper 不重複報錯）。</li>
 *   <li><b>strict</b>：{@code requireValidWeightUnit} / {@code requireValidShippingTemperature}
 *       / {@code requireValidCertificationType}。invalid → {@code Status.INVALID_ARGUMENT}。
 *       給 API 寫入邊界（ItemMutationService 全部 mutation 入口）使用，符合
 *       docs/開發規範/Java 微服務開發規範.md §6.4「寫入 DB 前必須 validate」。</li>
 * </ul>
 *
 * <p>blank（null / 空白）一律回 null，由 caller 決定該欄位是否必填。
 */
public final class CodeMetaNormalizer {

    private CodeMetaNormalizer() {}

    public static String weightUnitToValue(String raw) {
        if (raw == null || raw.isBlank()) return null;
        String s = raw.trim();
        if (WeightUnit.isValidValue(s)) return s;
        if (WeightUnit.isValidCode(s)) return WeightUnit.fromCode(s).value();
        return null;
    }

    public static String shippingTemperatureToValue(String raw) {
        if (raw == null || raw.isBlank()) return null;
        String s = raw.trim();
        if (ShippingTemperature.isValidValue(s)) return s;
        if (ShippingTemperature.isValidCode(s)) return ShippingTemperature.fromCode(s).value();
        return null;
    }

    public static String certificationTypeToValue(String raw) {
        if (raw == null || raw.isBlank()) return null;
        String s = raw.trim();
        if (CertificationType.isValidValue(s)) return s;
        if (CertificationType.isValidCode(s)) return CertificationType.fromCode(s).value();
        return null;
    }

    public static String shippingMethodToValue(String raw) {
        if (raw == null || raw.isBlank()) return null;
        String s = raw.trim();
        if (ShippingMethod.isValidValue(s)) return s;
        if (ShippingMethod.isValidCode(s)) return ShippingMethod.fromCode(s).value();
        return null;
    }

    public static String requireValidWeightUnit(String raw, String fieldName) {
        if (raw == null || raw.isBlank()) return null;
        String v = weightUnitToValue(raw);
        if (v != null) return v;
        throw invalidArgument(fieldName, raw, WeightUnit.class);
    }

    public static String requireValidShippingTemperature(String raw, String fieldName) {
        if (raw == null || raw.isBlank()) return null;
        String v = shippingTemperatureToValue(raw);
        if (v != null) return v;
        throw invalidArgument(fieldName, raw, ShippingTemperature.class);
    }

    public static String requireValidCertificationType(String raw, String fieldName) {
        if (raw == null || raw.isBlank()) return null;
        String v = certificationTypeToValue(raw);
        if (v != null) return v;
        throw invalidArgument(fieldName, raw, CertificationType.class);
    }

    public static String requireValidShippingMethod(String raw, String fieldName) {
        if (raw == null || raw.isBlank()) return null;
        String v = shippingMethodToValue(raw);
        if (v != null) return v;
        throw invalidArgument(fieldName, raw, ShippingMethod.class);
    }

    private static <E extends Enum<E> & CodeMeta> io.grpc.StatusRuntimeException invalidArgument(
            String fieldName, String raw, Class<E> enumClass) {
        return Status.INVALID_ARGUMENT
                .withDescription("無效的 " + fieldName + ": " + raw
                        + "，允許值: " + CodeMetaValidator.allValues(enumClass)
                        + "（或對應 NAME: " + CodeMetaValidator.allCodes(enumClass) + "）")
                .asRuntimeException();
    }
}
