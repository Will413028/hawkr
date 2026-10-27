package io.github.will413028.hawkr.common.providercall;

import java.util.function.BooleanSupplier;
import java.util.function.Consumer;
import java.util.function.Supplier;

/**
 * 「呼叫外部 API 並等結果」場景的共用骨架。
 *
 * <p>強制流程（嚴格依序）：
 * <ol>
 *   <li>{@code acquireRequested.getAsBoolean()} — 以 SQL UPDATE 嘗試 transition (NONE | FAILED) → REQUESTED；
 *       回傳 {@code true} 才繼續，{@code false} 表示已被另一執行緒占用 → 拋 {@link ProviderCallBusyException}</li>
 *   <li>{@code invoke.get()} — 在事務外呼叫外部 API；可能拋 exception</li>
 *   <li>成功 → {@code markSucceeded.accept(result)}（負責 transition REQUESTED → SUCCEEDED）<br/>
 *       失敗 → {@code markFailed.accept(throwable)}（負責 transition REQUESTED → FAILED）→ 重新拋出原例外</li>
 * </ol>
 *
 * <p>設計參見 06-Shipping §2.4.4。
 */
public interface ProviderCallExecutor {

    /**
     * 對單一資源執行一次外部呼叫。
     *
     * @param ctx             呼叫情境（callKind + resourceId）
     * @param acquireRequested 原子 transition (NONE|FAILED) → REQUESTED；true 表 acquire 成功
     * @param invoke          實際的外部 API 呼叫；可能拋 exception
     * @param markSucceeded   收到成功結果時的 transition（業務層負責 SQL）
     * @param markFailed      收到失敗時的 transition（業務層負責落地 provider_status / provider_message）
     * @param <T>             外部呼叫的回傳型別
     * @return invoke 的回傳值（成功時）
     * @throws ProviderCallBusyException acquireRequested 失敗
     * @throws RuntimeException          invoke 失敗時重拋原例外（已先 markFailed）
     */
    <T> T execute(
        ProviderCallContext ctx,
        BooleanSupplier acquireRequested,
        Supplier<T> invoke,
        Consumer<T> markSucceeded,
        Consumer<Throwable> markFailed
    );
}
