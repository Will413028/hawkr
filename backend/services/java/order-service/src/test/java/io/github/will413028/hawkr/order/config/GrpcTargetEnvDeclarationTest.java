package io.github.will413028.hawkr.order.config;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.yaml.snakeyaml.Yaml;

import java.io.InputStream;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * order-service 的 {@code hawkr.obs.grpc.targets} 必須宣告 invoice，且指向 gRPC port。
 *
 * <p>{@code GrpcChannelRegistry.getOrCreate} 是 lazy 的：target 沒宣告時，第一次呼叫才拋
 * {@code IllegalArgumentException}，對結帳鏈的兩支 invoice RPC 而言就是買家按下結帳的那一刻。
 */
class GrpcTargetEnvDeclarationTest {

    @Test
    @DisplayName("58 號 §6.2.4 order-service ①：invoice target 已宣告，且指向 gRPC port 而非 HTTP port")
    void invoiceTargetIsDeclared() {
        Map<String, Map<String, Object>> targets = grpcTargets();

        assertThat(targets)
                .as("checkout preview／confirm 的 ResolveOrderInvoiceModeInternal／"
                        + "ValidateOrderInvoiceInputInternal 都經由這個 target；缺了它，"
                        + "第一次結帳就 IllegalArgumentException")
                .containsKey("invoice");

        Map<String, Object> invoice = targets.get("invoice");
        assertThat(String.valueOf(invoice.get("host"))).contains("${INVOICE_SERVICE_HOST");
        // 10041 是 invoice-service 的 gRPC port。10040 是 HTTP（health/metrics）、
        // 10042 是 PDF 授權下載通道 —— 指到那兩個之中任何一個，症狀都是連得上但講不通 gRPC。
        assertThat(String.valueOf(invoice.get("port"))).contains("${INVOICE_SERVICE_GRPC_PORT:10041}");
    }

    @SuppressWarnings("unchecked")
    private static Map<String, Map<String, Object>> grpcTargets() {
        try (InputStream in = GrpcTargetEnvDeclarationTest.class.getResourceAsStream("/application.yml")) {
            assertThat(in).as("order-service 的 application.yml 必須在 test classpath 上").isNotNull();
            Map<String, Object> root = new Yaml().load(in);
            Map<String, Object> hawkr = (Map<String, Object>) root.get("hawkr");
            Map<String, Object> obs = (Map<String, Object>) hawkr.get("obs");
            Map<String, Object> grpc = (Map<String, Object>) obs.get("grpc");
            return (Map<String, Map<String, Object>>) grpc.get("targets");
        } catch (Exception e) {
            throw new IllegalStateException("讀不到 application.yml 的 grpc targets", e);
        }
    }
}
