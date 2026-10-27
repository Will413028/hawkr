package io.github.will413028.hawkr.payment;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.EnableScheduling;

/**
 * 支付服務啟動類別
 *
 * 職責： - 整合 PayUni 金流 - 提供信用卡、ATM、Apple Pay、Google Pay 等支付方式 - 管理支付交易狀態 - 處理退款流程
 */
@SpringBootApplication
@EnableScheduling
public class PaymentServiceApplication {

    private static final Logger log = LoggerFactory.getLogger(PaymentServiceApplication.class);

    public static void main(String[] args) {
        var context = SpringApplication.run(PaymentServiceApplication.class, args);
        String port = context.getEnvironment().getProperty("server.port", "");
        String grpcPort = context.getEnvironment().getProperty("grpc.server.port", "");
        log.info("Payment Service Started - HTTP Port: {}, gRPC Port: {}", port, grpcPort);
    }
}
