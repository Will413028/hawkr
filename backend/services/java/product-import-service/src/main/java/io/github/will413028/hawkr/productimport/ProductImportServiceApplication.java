package io.github.will413028.hawkr.productimport;

import org.mybatis.spring.annotation.MapperScan;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.EnableScheduling;

/**
 * Product Import Service 入口（40 號設計書）。
 *
 * <p>商品匯入精靈服務 — 處理賣家 CSV 上傳、L1/L2 驗證、HTML 解析、
 * 圖片下載（透過 Media）、預覽、批次轉正為正式商品（透過 Product Service）。
 */
@SpringBootApplication
@MapperScan("io.github.will413028.hawkr.productimport.mapper")
@EnableScheduling
public class ProductImportServiceApplication {

    private static final Logger log = LoggerFactory.getLogger(ProductImportServiceApplication.class);

    public static void main(String[] args) {
        var context = SpringApplication.run(ProductImportServiceApplication.class, args);
        String port = context.getEnvironment().getProperty("server.port", "");
        String grpcPort = context.getEnvironment().getProperty("grpc.server.port", "");
        log.info("Product Import Service Started - HTTP Port: {}, gRPC Port: {}", port, grpcPort);
    }
}
