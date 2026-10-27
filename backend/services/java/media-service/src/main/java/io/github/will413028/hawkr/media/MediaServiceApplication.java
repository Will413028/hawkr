package io.github.will413028.hawkr.media;

import org.mybatis.spring.annotation.MapperScan;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.EnableScheduling;

/**
 * 多媒體微服務啟動類
 * <p>
 * 職責：管理靜態資源（Images/Videos）的生命週期與存取權限
 */
@SpringBootApplication
@MapperScan("io.github.will413028.hawkr.media.repository")
@EnableScheduling
public class MediaServiceApplication {

    private static final Logger log = LoggerFactory.getLogger(MediaServiceApplication.class);

    public static void main(String[] args) {
        var context = SpringApplication.run(MediaServiceApplication.class, args);
        String port = context.getEnvironment().getProperty("server.port", "");
        String grpcPort = context.getEnvironment().getProperty("grpc.server.port", "");
        log.info("Media Service Started - HTTP Port: {}, gRPC Port: {}", port, grpcPort);
    }
}
