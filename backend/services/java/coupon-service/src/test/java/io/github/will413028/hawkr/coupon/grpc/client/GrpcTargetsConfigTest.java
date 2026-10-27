package io.github.will413028.hawkr.coupon.grpc.client;

import io.github.will413028.hawkr.common.obs.config.ObsProperties;
import org.junit.jupiter.api.Test;
import org.springframework.boot.context.properties.bind.Binder;
import org.springframework.boot.env.YamlPropertySourceLoader;
import org.springframework.core.env.PropertySource;
import org.springframework.core.env.StandardEnvironment;
import org.springframework.core.io.ClassPathResource;

import java.util.List;
import java.util.Set;

import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * 防止 *GrpcClient 呼叫 {@code grpcStubFactory.blocking("targetName", ...)} 用到的
 * target 名稱，在 application.yml 的 {@code hawkr.obs.grpc.targets} 漏掉對應設定。
 *
 * <p>這類缺口不會被既有 Mockito 單元測試發現 —— 所有 *GrpcClient 呼叫端測試（例如
 * {@link io.github.will413028.hawkr.coupon.grpc.CouponGrpcServiceTargetAccountLookupTest}）都是
 * mock 掉 grpcStubFactory 本身，永遠不會真的打到 GrpcChannelRegistry.getOrCreate()。
 * 只有部署到真實環境、第一次呼叫該 target 時，才會拋
 * {@link IllegalArgumentException}("gRPC target 'X' not configured...")。
 *
 * <p>Root cause 案例（2026-07-19，buyer_id rekey Phase 5）：{@link BuyerGrpcClient} 隨
 * commit 671f2bf66 上線時，application.yml 忘了加對應的 buyer target，導致
 * {@code CouponGrpcService.sellerGetTemplateDetail} 的
 * {@code buyerGrpcClient.resolvePrincipals(...)} 每次呼叫都拋例外 —— 該呼叫點本身有
 * try/catch 降級（記 ERROR log + 空 map），因此請求不會失敗，只會靜默 fallback 用
 * 原始 buyer_id 當 account_id 查 {@link AccountGrpcClient#batchGetAccountInfo}，
 * 對分岔買家（buyer_id != account_id）查出來的帳號永遠是錯的、nickname/phone 永遠
 * 空白 —— 且因為降級路徑本身不報錯，這個缺口在 dev 環境活了一段時間都沒被抓到。
 */
class GrpcTargetsConfigTest {

    /**
     * 目前所有 *GrpcClient 呼叫 grpcStubFactory.blocking("targetName", ...) 用到的
     * target 名稱（來源：coupon-service 的 client 套件逐一 grep confirm）。
     *
     * 新增 *GrpcClient 呼叫新的 target 時，這裡要同步加一行 —— 這是刻意設計成手動維護：
     * 迫使加新 client 的人同時想到 application.yml（以及 k8s/values/coupon-service.yaml、
     * docker-compose-local.yml 的對應 HOST/PORT env var）要一起補，而不是等部署後才發現。
     */
    private static final Set<String> REQUIRED_GRPC_TARGETS = Set.of("seller", "account", "order", "buyer");

    @Test
    void applicationYml_declaresAllRequiredGrpcTargets() throws Exception {
        List<PropertySource<?>> sources = new YamlPropertySourceLoader()
                .load("application", new ClassPathResource("application.yml"));
        StandardEnvironment environment = new StandardEnvironment();
        sources.forEach(environment.getPropertySources()::addLast);
        ObsProperties props = Binder.get(environment).bind("hawkr.obs", ObsProperties.class).get();

        Set<String> configuredTargets = props.getGrpc().getTargets().keySet();

        for (String required : REQUIRED_GRPC_TARGETS) {
            assertTrue(configuredTargets.contains(required),
                    "application.yml 缺少 hawkr.obs.grpc.targets." + required
                            + "（現有設定: " + configuredTargets + "）—— "
                            + "任何呼叫 grpcStubFactory.blocking(\"" + required + "\", ...) 的 *GrpcClient "
                            + "在執行時都會拋 IllegalArgumentException；若呼叫端有 catch-and-log 降級"
                            + "（多數皆如此），這個缺口不會讓請求失敗，只會讓資料靜默不完整，"
                            + "很難被一般測試或人工驗收發現。");
        }
    }
}
