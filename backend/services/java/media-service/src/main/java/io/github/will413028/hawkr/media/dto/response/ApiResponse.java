package io.github.will413028.hawkr.media.dto.response;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

/**
 * 通用 API 回應包裝
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class ApiResponse<T> {

    /**
     * 狀態碼
     */
    private int code;

    /**
     * 訊息
     */
    private String message;

    /**
     * 資料
     */
    private T data;

    /**
     * 成功回應
     */
    public static <T> ApiResponse<T> success(T data) {
        return ApiResponse.<T>builder()
            .code(0)
            .message("success")
            .data(data)
            .build();
    }

    /**
     * 成功回應（無資料）
     */
    public static <T> ApiResponse<T> success() {
        return ApiResponse.<T>builder()
            .code(0)
            .message("success")
            .build();
    }

    /**
     * 錯誤回應
     */
    public static <T> ApiResponse<T> error(int code, String message) {
        return ApiResponse.<T>builder()
            .code(code)
            .message(message)
            .build();
    }
}
