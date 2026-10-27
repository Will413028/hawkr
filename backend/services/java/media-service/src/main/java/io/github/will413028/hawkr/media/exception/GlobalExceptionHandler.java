package io.github.will413028.hawkr.media.exception;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.media.dto.response.ApiResponse;
import jakarta.validation.ConstraintViolationException;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.validation.FieldError;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

import java.util.stream.Collectors;

/**
 * 全域例外處理器
 */
@RestControllerAdvice
public class GlobalExceptionHandler {

    private static final ObsLogger log = ObsLogger.forClass(GlobalExceptionHandler.class);

    /**
     * 處理業務例外
     */
    @ExceptionHandler(MediaServiceException.class)
    public ResponseEntity<ApiResponse<Void>> handleMediaServiceException(MediaServiceException ex) {
        log.warn("exception.business", ex.getMessage());
        return ResponseEntity
            .status(ex.getHttpStatusCode())
            .body(ApiResponse.error(ex.getHttpStatusCode(), ex.getMessage()));
    }

    /**
     * 處理驗證例外（@Valid）
     */
    @ExceptionHandler(MethodArgumentNotValidException.class)
    public ResponseEntity<ApiResponse<Void>> handleValidationException(MethodArgumentNotValidException ex) {
        String message = ex.getBindingResult().getFieldErrors().stream()
            .map(FieldError::getDefaultMessage)
            .collect(Collectors.joining(", "));
        log.warn("exception.validation", message);
        return ResponseEntity
            .status(HttpStatus.BAD_REQUEST)
            .body(ApiResponse.error(400, message));
    }

    /**
     * 處理約束違反例外
     */
    @ExceptionHandler(ConstraintViolationException.class)
    public ResponseEntity<ApiResponse<Void>> handleConstraintViolationException(ConstraintViolationException ex) {
        log.warn("exception.constraint", ex.getMessage());
        return ResponseEntity
            .status(HttpStatus.BAD_REQUEST)
            .body(ApiResponse.error(400, ex.getMessage()));
    }

    /**
     * 處理未預期例外
     */
    @ExceptionHandler(Exception.class)
    public ResponseEntity<ApiResponse<Void>> handleException(Exception ex) {
        log.error("exception.unexpected", "非預期例外", ex);
        return ResponseEntity
            .status(HttpStatus.INTERNAL_SERVER_ERROR)
            .body(ApiResponse.error(500, "內部伺服器錯誤"));
    }
}
