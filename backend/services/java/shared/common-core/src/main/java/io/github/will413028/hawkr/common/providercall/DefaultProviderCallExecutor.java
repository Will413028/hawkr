package io.github.will413028.hawkr.common.providercall;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.function.BooleanSupplier;
import java.util.function.Consumer;
import java.util.function.Supplier;

/**
 * {@link ProviderCallExecutor} 預設實作。
 *
 * <p>由 {@link ProviderCallAutoConfiguration} 註冊為 Spring Bean；微服務直接注入即可。
 */
public class DefaultProviderCallExecutor implements ProviderCallExecutor {

    private static final Logger log = LoggerFactory.getLogger(DefaultProviderCallExecutor.class);

    @Override
    public <T> T execute(
        ProviderCallContext ctx,
        BooleanSupplier acquireRequested,
        Supplier<T> invoke,
        Consumer<T> markSucceeded,
        Consumer<Throwable> markFailed
    ) {
        if (!acquireRequested.getAsBoolean()) {
            log.info("ProviderCallExecutor.acquire.busy kind={} resourceId={}",
                    ctx.callKind(), ctx.resourceId());
            throw new ProviderCallBusyException(ctx);
        }

        log.info("ProviderCallExecutor.acquire.ok kind={} resourceId={}",
                ctx.callKind(), ctx.resourceId());

        T result;
        try {
            result = invoke.get();
        } catch (Throwable t) {
            log.warn("ProviderCallExecutor.invoke.failed kind={} resourceId={} error={}",
                    ctx.callKind(), ctx.resourceId(), t.toString());
            try {
                markFailed.accept(t);
            } catch (Throwable t2) {
                log.error("ProviderCallExecutor.markFailed.error kind={} resourceId={} (markFailed itself threw)",
                        ctx.callKind(), ctx.resourceId(), t2);
            }
            // 必須重新拋出原 exception 讓上層感知，否則 caller 認為操作成功
            if (t instanceof RuntimeException re) {
                throw re;
            }
            if (t instanceof Error err) {
                throw err;
            }
            throw new RuntimeException(t);
        }

        try {
            markSucceeded.accept(result);
        } catch (Throwable t) {
            // markSucceeded 失敗代表 invoke 已成功但 DB transition 失敗（極少見）
            // 不拋例外覆蓋成功訊號，由 reconciliation 補正
            log.error("ProviderCallExecutor.markSucceeded.error kind={} resourceId={}",
                    ctx.callKind(), ctx.resourceId(), t);
        }

        log.info("ProviderCallExecutor.invoke.success kind={} resourceId={}",
                ctx.callKind(), ctx.resourceId());
        return result;
    }
}
