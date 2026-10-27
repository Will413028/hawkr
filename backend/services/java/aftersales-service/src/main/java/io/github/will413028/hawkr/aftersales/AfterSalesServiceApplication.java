package io.github.will413028.hawkr.aftersales;

import org.mybatis.spring.annotation.MapperScan;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.EnableScheduling;

@SpringBootApplication
@EnableScheduling
@MapperScan("io.github.will413028.hawkr.aftersales.mapper")
public class AfterSalesServiceApplication {
    private static final Logger log = LoggerFactory.getLogger(AfterSalesServiceApplication.class);

    public static void main(String[] args) {
        var context = SpringApplication.run(AfterSalesServiceApplication.class, args);
        String port = context.getEnvironment().getProperty("server.port", "");
        String grpcPort = context.getEnvironment().getProperty("grpc.server.port", "");
        log.info("AfterSales Service Started - HTTP Port: {}, gRPC Port: {}", port, grpcPort);
    }
}
