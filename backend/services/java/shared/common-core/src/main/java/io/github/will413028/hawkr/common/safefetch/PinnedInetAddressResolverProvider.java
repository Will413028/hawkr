package io.github.will413028.hawkr.common.safefetch;

import java.net.Inet4Address;
import java.net.Inet6Address;
import java.net.InetAddress;
import java.net.UnknownHostException;
import java.net.spi.InetAddressResolver;
import java.net.spi.InetAddressResolverProvider;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.stream.Stream;

/**
 * DNS rebinding 防護核心：Java 21 {@link InetAddressResolverProvider} SPI，JVM 全域安裝一次。
 *
 * <p>SafeFetcher 在「驗過 hostname 的所有 IP 都不在黑名單」之後、實際 {@code HttpClient.send}
 * 之前，把 {@code host -> 已驗 IP[]} 寫進 {@link #PINS}（ThreadLocal）。HttpClient 內部連線時會
 * 再次 resolve 同一 hostname，該次 resolve 經過本 resolver：命中 pin 表 → 直接回傳已驗 IP，
 * 完全不查 DNS，攻擊者在「驗證」與「連線」之間切換 DNS 也無法把連線導到內網 IP（消滅 TOCTOU 窗口）。</p>
 *
 * <p>未 pin 的 hostname 一律 delegate 給 {@code configuration.builtinResolver()}，因此對其他
 * 不使用 SafeFetcher 的程式碼（gRPC / DB / 一般 DNS）完全透明、零行為改變。</p>
 */
public final class PinnedInetAddressResolverProvider extends InetAddressResolverProvider {

    /** thread-scoped pin 表：key = 小寫 hostname，value = 已通過黑名單驗證的 IP。 */
    private static final ThreadLocal<Map<String, InetAddress[]>> PINS =
        ThreadLocal.withInitial(HashMap::new);

    /** 寫入 pin。由 {@link SafeFetcherImpl} 在每跳 send 前呼叫。 */
    static void pin(String hostname, InetAddress[] addresses) {
        if (hostname == null || addresses == null || addresses.length == 0) {
            return;
        }
        PINS.get().put(hostname.toLowerCase(Locale.ROOT), addresses);
    }

    /** 清除 pin。由 {@link SafeFetcherImpl} 在 finally 呼叫，pin 表空時整個移除避免 ThreadLocal 洩漏。 */
    static void unpin(String hostname) {
        if (hostname == null) {
            return;
        }
        Map<String, InetAddress[]> map = PINS.get();
        map.remove(hostname.toLowerCase(Locale.ROOT));
        if (map.isEmpty()) {
            PINS.remove();
        }
    }

    @Override
    public InetAddressResolver get(Configuration configuration) {
        return new PinnedResolver(configuration.builtinResolver());
    }

    @Override
    public String name() {
        return "hawkr-safefetch-pinned-resolver";
    }

    private static final class PinnedResolver implements InetAddressResolver {

        private final InetAddressResolver builtin;

        PinnedResolver(InetAddressResolver builtin) {
            this.builtin = builtin;
        }

        @Override
        public Stream<InetAddress> lookupByName(String host, LookupPolicy lookupPolicy)
                throws UnknownHostException {
            if (host != null) {
                InetAddress[] pinned = PINS.get().get(host.toLowerCase(Locale.ROOT));
                if (pinned != null) {
                    List<InetAddress> filtered = filterByPolicy(pinned, lookupPolicy);
                    if (filtered.isEmpty()) {
                        throw new UnknownHostException(host);
                    }
                    return filtered.stream();
                }
            }
            return builtin.lookupByName(host, lookupPolicy);
        }

        @Override
        public String lookupByAddress(byte[] addr) throws UnknownHostException {
            return builtin.lookupByAddress(addr);
        }

        /** 依 LookupPolicy 的 family 旗標過濾並排序已 pin 的 IP，與 builtin resolver 行為對齊。 */
        private static List<InetAddress> filterByPolicy(InetAddress[] pinned, LookupPolicy policy) {
            int chars = policy.characteristics();
            boolean wantV4 = (chars & LookupPolicy.IPV4) != 0;
            boolean wantV6 = (chars & LookupPolicy.IPV6) != 0;
            if (!wantV4 && !wantV6) {
                wantV4 = true;
                wantV6 = true;
            }
            List<InetAddress> v4 = new ArrayList<>();
            List<InetAddress> v6 = new ArrayList<>();
            for (InetAddress a : pinned) {
                if (a instanceof Inet4Address && wantV4) {
                    v4.add(a);
                } else if (a instanceof Inet6Address && wantV6) {
                    v6.add(a);
                }
            }
            List<InetAddress> result = new ArrayList<>(v4.size() + v6.size());
            if ((chars & LookupPolicy.IPV6_FIRST) != 0) {
                result.addAll(v6);
                result.addAll(v4);
            } else {
                result.addAll(v4);
                result.addAll(v6);
            }
            return result;
        }
    }
}
