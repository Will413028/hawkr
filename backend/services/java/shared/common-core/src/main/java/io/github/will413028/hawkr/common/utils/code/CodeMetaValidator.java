package io.github.will413028.hawkr.common.utils.code;

import java.util.Arrays;
import java.util.stream.Collectors;

/**
 * CodeMeta Enum 通用合法值驗證工具
 *
 * 用於在 gRPC 邊界或 Service 層驗證前端傳入的 Enum 值是否合法，
 * 避免非法值直接存入 DB 導致下游流程（如運費計算、訂單結帳）失敗。
 *
 * 使用範例：
 * <pre>
 * // 以 code 驗證（如 WeightUnit: KG/CATTY/GRAM）
 * CodeMetaValidator.requireValidCode(WeightUnit.class, unitStr, "unit");
 *
 * // 以 value 驗證（如 ProductStatus: 4001/4002/4003/4004）
 * CodeMetaValidator.requireValidValue(ProductStatus.class, statusStr, "status");
 * </pre>
 */
public final class CodeMetaValidator {

    private CodeMetaValidator() {}

    /**
     * 驗證 code 是否為指定 Enum 的合法 code，不合法則拋出 IllegalArgumentException。
     *
     * @param enumClass CodeMeta Enum 類別
     * @param code      待驗證的 code 值
     * @param fieldName 欄位名稱（用於錯誤訊息）
     * @param <E>       Enum 型別
     * @throws IllegalArgumentException code 為空或不合法
     */
    public static <E extends Enum<E> & CodeMeta> void requireValidCode(
            Class<E> enumClass, String code, String fieldName) {
        if (code == null || code.isBlank()) {
            throw new IllegalArgumentException(
                    fieldName + " 為必填欄位，允許值: " + allCodes(enumClass));
        }
        if (!isValidCode(enumClass, code)) {
            throw new IllegalArgumentException(
                    "無效的 " + fieldName + ": " + code + "，允許值: " + allCodes(enumClass));
        }
    }

    /**
     * 驗證 value 是否為指定 Enum 的合法 value，不合法則拋出 IllegalArgumentException。
     *
     * @param enumClass CodeMeta Enum 類別
     * @param value     待驗證的 value 值
     * @param fieldName 欄位名稱（用於錯誤訊息）
     * @param <E>       Enum 型別
     * @throws IllegalArgumentException value 為空或不合法
     */
    public static <E extends Enum<E> & CodeMeta> void requireValidValue(
            Class<E> enumClass, String value, String fieldName) {
        if (value == null || value.isBlank()) {
            throw new IllegalArgumentException(
                    fieldName + " 為必填欄位，允許值: " + allValues(enumClass));
        }
        if (!isValidValue(enumClass, value)) {
            throw new IllegalArgumentException(
                    "無效的 " + fieldName + ": " + value + "，允許值: " + allValues(enumClass));
        }
    }

    /**
     * 驗證 code 是否合法（允許 null/空值，僅在有值時驗證）。
     * 適用於更新場景：欄位為選填，有傳才驗證。
     *
     * @return true 表示合法（含 null/空值），false 表示不合法
     */
    public static <E extends Enum<E> & CodeMeta> void validateCodeIfPresent(
            Class<E> enumClass, String code, String fieldName) {
        if (code != null && !code.isBlank() && !isValidCode(enumClass, code)) {
            throw new IllegalArgumentException(
                    "無效的 " + fieldName + ": " + code + "，允許值: " + allCodes(enumClass));
        }
    }

    /**
     * value 為 null 或空白時跳過，否則驗證合法性。
     */
    public static <E extends Enum<E> & CodeMeta> void validateValueIfPresent(
            Class<E> enumClass, String value, String fieldName) {
        if (value != null && !value.isBlank() && !isValidValue(enumClass, value)) {
            throw new IllegalArgumentException(
                    "無效的 " + fieldName + ": " + value + "，允許值: " + allValues(enumClass));
        }
    }

    /**
     * 檢查 code 是否為指定 Enum 的合法 code。
     */
    public static <E extends Enum<E> & CodeMeta> boolean isValidCode(Class<E> enumClass, String code) {
        if (code == null) return false;
        for (E e : enumClass.getEnumConstants()) {
            if (e.code().equals(code)) return true;
        }
        return false;
    }

    /**
     * 檢查 value 是否為指定 Enum 的合法 value。
     */
    public static <E extends Enum<E> & CodeMeta> boolean isValidValue(Class<E> enumClass, String value) {
        if (value == null) return false;
        for (E e : enumClass.getEnumConstants()) {
            if (e.value().equals(value)) return true;
        }
        return false;
    }

    /**
     * 取得指定 Enum 所有合法 code，以逗號分隔。
     */
    public static <E extends Enum<E> & CodeMeta> String allCodes(Class<E> enumClass) {
        return Arrays.stream(enumClass.getEnumConstants())
                .map(CodeMeta::code)
                .collect(Collectors.joining(", "));
    }

    /**
     * 取得指定 Enum 所有合法 value，以逗號分隔。
     */
    public static <E extends Enum<E> & CodeMeta> String allValues(Class<E> enumClass) {
        return Arrays.stream(enumClass.getEnumConstants())
                .map(CodeMeta::value)
                .collect(Collectors.joining(", "));
    }
}
