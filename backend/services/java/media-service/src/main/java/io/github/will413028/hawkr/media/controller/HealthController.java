package io.github.will413028.hawkr.media.controller;

import io.github.will413028.hawkr.media.dto.response.ApiResponse;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.Map;

/**
 * 健康檢查 Controller
 */
@RestController
public class HealthController {

    @GetMapping("/health")
    public ApiResponse<Map<String, String>> health() {
        return ApiResponse.success(Map.of(
            "status", "UP",
            "service", "media-service"
        ));
    }
}
