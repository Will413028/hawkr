package io.github.will413028.hawkr.shipping;

import org.mybatis.spring.annotation.MapperScan;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.EnableScheduling;

@SpringBootApplication
@MapperScan("io.github.will413028.hawkr.shipping.mapper")
@EnableScheduling
public class ShippingServiceApplication {

    private static final Logger log = LoggerFactory.getLogger(ShippingServiceApplication.class);

    public static void main(String[] args) {
        var context = SpringApplication.run(ShippingServiceApplication.class, args);
        String port = context.getEnvironment().getProperty("server.port", "");
        String grpcPort = context.getEnvironment().getProperty("grpc.server.port", "");
        log.info("Shipping Service Started - HTTP Port: {}, gRPC Port: {}", port, grpcPort);
    }
}
