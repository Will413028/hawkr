package io.github.will413028.hawkr.order.grpc.client;

import io.github.will413028.hawkr.cart.grpc.CartServiceGrpc;
import io.github.will413028.hawkr.cart.grpc.CheckoutLockRequest;
import io.github.will413028.hawkr.cart.grpc.CheckoutLockResponse;
import io.github.will413028.hawkr.common.grpc.GrpcAuthClientInterceptor;
import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.grpc.GrpcAuthServerInterceptor;
import io.github.will413028.hawkr.common.obs.config.GrpcTarget;
import io.github.will413028.hawkr.common.obs.config.ObsProperties;
import io.github.will413028.hawkr.common.obs.context.ObsContext;
import io.github.will413028.hawkr.common.obs.grpc.GrpcChannelRegistry;
import io.github.will413028.hawkr.common.obs.grpc.GrpcStubFactory;
import io.github.will413028.hawkr.common.obs.grpc.ObsGrpcClientInterceptor;
import io.grpc.Context;
import io.grpc.Server;
import io.grpc.ServerBuilder;
import io.grpc.stub.StreamObserver;
import io.opentelemetry.context.propagation.TextMapPropagator;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.util.List;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.assertEquals;

/**
 * Discriminating regression test for the checkout/confirm CHECKOUT_LOCK_FAILED (6005)
 * bug hit by every buyer whose buyer_id has diverged from their login account_id
 * (buyer_id rekey epic, blocked Phase 7 live verification 2026-07-19).
 *
 * <p>Root cause: {@code CartGrpcClient}'s "cart" gRPC channel has
 * {@link GrpcAuthClientInterceptor} globally baked in via {@link GrpcChannelRegistry}
 * (every Java service's outbound channel gets it — it propagates order-service's own
 * <b>ambient</b>/incoming {@code x-account-id} onto every outbound call). The old
 * implementation additionally attached an explicit {@code x-account-id} Metadata header
 * carrying the correct, resolved buyer_id. Because grpc-java's duplicate-header
 * {@code Metadata.get()} returns the <b>last</b>-appended value, and the baked-in
 * interceptor's {@code start()} runs <b>after</b> the explicitly attached one, the stale
 * ambient value silently won on the wire — cart-service locked the wrong buyer_id and
 * {@code UPDATE cart_items ... WHERE buyer_id = ?} matched 0 rows.
 *
 * <p>This test wires the exact same shared interceptor
 * ({@link GrpcAuthClientInterceptor}, the real production bean, not a fake) around a
 * real localhost gRPC server so a regression of the duplicate-header bug is caught the
 * same way it actually manifested: the caller's ambient identity ({@code accountId}
 * attached to {@link GrpcAuthContext#ACCOUNT_ID} before the call, standing in for
 * order-service's own unresolved account_id from its inbound request) is deliberately
 * DIFFERENT from the resolved buyer_id passed as {@link CartGrpcClient}'s
 * explicit argument — a same-value test cannot distinguish the bug (wrong id silently
 * wins because it happens to equal the right one) from the fix.
 */
class CartGrpcClientCheckoutLockAccountIdTest {

    private static final long STALE_AMBIENT_ACCOUNT_ID = 111L; // order-service's own raw/unresolved account_id
    private static final long RESOLVED_BUYER_ID = 999L; // gateway-resolved buyer_id (diverged case)

    private final AtomicReference<Long> receivedAccountId = new AtomicReference<>();

    private Server server;
    private GrpcChannelRegistry registry;
    private CartGrpcClient cartGrpcClient;

    @BeforeEach
    void setUp() throws IOException {
        // Fake cart-service: records whatever account id it actually sees via
        // GrpcAuthContext, the exact same extraction cart-service's real
        // CartGrpcService.checkoutLock uses (GrpcAuthContext.requireAccountId()).
        CartServiceGrpc.CartServiceImplBase fakeCartService = new CartServiceGrpc.CartServiceImplBase() {
            @Override
            public void checkoutLock(CheckoutLockRequest request, StreamObserver<CheckoutLockResponse> responseObserver) {
                receivedAccountId.set(GrpcAuthContext.requireAccountId());
                responseObserver.onNext(CheckoutLockResponse.newBuilder().setCode(0).build());
                responseObserver.onCompleted();
            }
        };

        server = ServerBuilder.forPort(0)
                .addService(fakeCartService)
                .intercept(new GrpcAuthServerInterceptor()) // same metadata->Context extraction cart-service runs
                .build()
                .start();

        ObsProperties obsProperties = new ObsProperties();
        obsProperties.setService("order-service-test");
        GrpcTarget cartTarget = new GrpcTarget();
        cartTarget.setHost("localhost");
        cartTarget.setPort(server.getPort());
        cartTarget.setPlaintext(true);
        obsProperties.getGrpc().getTargets().put("cart", cartTarget);

        // Mirrors ObsAutoConfiguration's real wiring: every channel gets
        // [ObsGrpcClientInterceptor, GrpcAuthClientInterceptor] baked in, unconditionally.
        registry = new GrpcChannelRegistry(
                obsProperties,
                new ObsGrpcClientInterceptor(new ObsContext(), TextMapPropagator.noop()),
                new GrpcAuthClientInterceptor("order-service-test"));

        cartGrpcClient = new CartGrpcClient(new GrpcStubFactory(registry));
    }

    @AfterEach
    void tearDown() {
        registry.shutdownAll();
        server.shutdownNow();
    }

    @Test
    @DisplayName("checkoutLock sends the resolved buyer_id, not order-service's own ambient/raw account id")
    void checkoutLock_usesResolvedBuyerId_notAmbientCallerAccountId() {
        // Simulate order-service handling a request where ITS OWN inbound identity
        // (GrpcAuthContext.ACCOUNT_ID, extracted by GrpcAuthServerInterceptor from the
        // gateway's call) is the raw, unresolved login account_id — while the caller
        // explicitly passes the gateway-resolved buyer_id as the argument
        // (exactly what CheckoutService.confirm -> OrderService.createOrder does:
        // request.getBuyerId() is the resolved value, GrpcAuthContext is the raw one).
        Context ctx = Context.current().withValue(GrpcAuthContext.ACCOUNT_ID, STALE_AMBIENT_ACCOUNT_ID);
        Context previous = ctx.attach();
        try {
            int code = cartGrpcClient.checkoutLock(RESOLVED_BUYER_ID, "checkout-1", List.of("100"));
            assertEquals(0, code);
        } finally {
            ctx.detach(previous);
        }

        assertEquals(RESOLVED_BUYER_ID, receivedAccountId.get(),
                "cart-service must see the resolved buyer_id CartGrpcClient.checkoutLock was called "
                        + "with, not order-service's own stale ambient account id -- a duplicate x-account-id "
                        + "header regression silently sends the wrong buyer_id and cart-service's UPDATE ... "
                        + "WHERE buyer_id = ? matches 0 rows (CHECKOUT_LOCK_FAILED / 6005) for any buyer whose "
                        + "buyer_id has diverged from their login account_id");
    }
}
