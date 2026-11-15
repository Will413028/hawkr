package io.github.will413028.hawkr.common.cache;

import static org.assertj.core.api.Assertions.assertThat;

import java.net.http.HttpClient;
import org.junit.jupiter.api.Test;
import org.springframework.boot.autoconfigure.AutoConfigurations;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

class RevalidateAutoConfigurationTest {

    private final ApplicationContextRunner runner = new ApplicationContextRunner()
            .withConfiguration(AutoConfigurations.of(RevalidateAutoConfiguration.class));

    @Test
    void clientUsesHttp11() {
        runner.run(ctx -> assertThat(ctx.getBean(RevalidateClient.class).httpClient().version())
                .isEqualTo(HttpClient.Version.HTTP_1_1));
    }

    @Test
    void clientIgnoresServiceLevelHttpClientBean() {
        runner.withUserConfiguration(Http2ClientConfig.class).run(ctx -> {
            HttpClient used = ctx.getBean(RevalidateClient.class).httpClient();
            assertThat(ctx.getBean(HttpClient.class).version()).isEqualTo(HttpClient.Version.HTTP_2);
            assertThat(used).isNotSameAs(ctx.getBean(HttpClient.class));
            assertThat(used.version()).isEqualTo(HttpClient.Version.HTTP_1_1);
        });
    }

    @Test
    void doesNotPublishGenericHttpClientBean() {
        runner.run(ctx -> assertThat(ctx).doesNotHaveBean(HttpClient.class));
    }

    @Configuration(proxyBeanMethods = false)
    static class Http2ClientConfig {
        @Bean
        HttpClient serviceHttpClient() {
            return HttpClient.newBuilder().version(HttpClient.Version.HTTP_2).build();
        }
    }
}
