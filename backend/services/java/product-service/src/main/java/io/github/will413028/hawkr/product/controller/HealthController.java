package io.github.will413028.hawkr.product.controller;

import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.HashMap;
import java.util.Map;

@RestController
public class HealthController {

    @GetMapping("/health")
    public ResponseEntity<Map<String, Object>> health() {
        Map<String, Object> result = new HashMap<>();
        result.put("status", "healthy");
        result.put("service", "product-service");
        result.put("timestamp", System.currentTimeMillis());
        return ResponseEntity.ok(result);
    }
}
