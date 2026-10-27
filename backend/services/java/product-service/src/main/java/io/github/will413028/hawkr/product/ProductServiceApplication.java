package io.github.will413028.hawkr.product;

import org.mybatis.spring.annotation.MapperScan;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.EnableScheduling;

@SpringBootApplication
@MapperScan("io.github.will413028.hawkr.product.repository")
@EnableScheduling
public class ProductServiceApplication {

    private static final Logger log = LoggerFactory.getLogger(ProductServiceApplication.class);

    public static void main(String[] args) {
        var context = SpringApplication.run(ProductServiceApplication.class, args);
        String port = context.getEnvironment().getProperty("server.port", "");
        String grpcPort = context.getEnvironment().getProperty("grpc.server.port", "");
        log.info("Product Service Started - HTTP Port: {}, gRPC Port: {}", port, grpcPort);
    }
}
