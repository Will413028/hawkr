package io.github.will413028.hawkr.webhook;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.EnableScheduling;

@SpringBootApplication
@EnableScheduling
public class PayuniWebhookServiceApplication {

    public static void main(String[] args) {
        SpringApplication.run(PayuniWebhookServiceApplication.class, args);
    }
}
