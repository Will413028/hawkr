# Coupling analysis: method

Snapshot: backend at monorepo HEAD `6c8bc0c15` (clean tree), analysed 2026-09-24. Nothing in the repo was modified; Go analysis ran on an rsync copy under the scratchpad.

## servers.csv
- The RPC inventory is parsed from `api/proto/**/*.proto`: 30 services, 730 RPCs. `membership/entitlement.proto`, `common/delegation.proto` and `gateway/gateway.proto` define messages only and have no services.
- Implementer: the Go `pb.Register<Svc>Server(...)` calls in `services/*/internal/grpc/di.go`, and Java `@GrpcService` classes that `extends <Svc>Grpc.<Svc>ImplBase`. Every proto service has exactly one implementing deployable.
- Each RPC was checked for a same-named handler method in the implementing module (Go `func (x *T) Rpc(`, Java `public void rpc(`). All 730 matched. This is a name match only, not a type check.

## calls.csv / pairs.csv
- **Go (gateway + 12 services + pkg)**: a small `golang.org/x/tools/go/packages` program with full type info (Tests=false). It records every call whose callee is a method on a generated `*ServiceClient` type in `github.com/Will413028/hawkr/go-pkg/proto/...`. Resolving through types means field names, embedding and aliases don't affect the result. Files that are `_test.go` or carry a `Code generated ... DO NOT EDIT` header are skipped. Nothing in any module failed to load.
- **Shared Go helper `pkg/media`**: its 3 raw stub calls are not counted on their own. Each call to `pkgmedia.FinalizeAndLink` counts as one FinalizeImages site plus one LinkImages site. `LinkImages` and `ReleaseReferences` each count as one site of the RPC with the same name. The site is credited to the calling deployable.
- **Java (10 services)**: `javap -c -l -p` over each service's `target/classes`. I checked beforehand that every `src/main/java` file has a `.class` and that no source file is newer than its class. The program records `invoke*` instructions whose owner matches `*Grpc$*Stub` and drops `AbstractStub` methods (`with*`, `getChannel`, `getCallOptions`, `build`). The LineNumberTable gives the source line. The result was cross-checked against a source regex and both give the same (file, rpc) set.
- **call_sites definition**: the number of distinct non-test source locations that invoke the generated stub method. This counts stub calls, not callers of the wrapper. Almost every deployable funnels calls through a thin client wrapper (`gateway/internal/grpc/*`, `internal/client/<target>/`, `grpc/client/*GrpcClient.java`), so call_sites is usually 1 per RPC. It measures the size of the interface, not how many business code paths depend on it.
- callee = implementer from servers.csv. No self-calls were found.
- A deployable is a Go module (the gateway or `services/<x>-service`) or a Java service directory. Calls under `cmd/` tools would count toward their module, but none were found.

## external.csv
- Built by hand from these sources:
  - absolute URLs in non-test source and config (after excluding docs, reference CSVs and node_modules);
  - third-party SDKs in go.mod and pom.xml (openai-go, genai/adk, firebase, aws-sdk s3, google api idtoken, payuni-java-client, okhttp);
  - outbound HTTP call sites (`http.NewRequest*`, OkHttp, `java.net.http`, RestTemplate, SafeFetcher, S3Client);
  - the `internal/client/<vendor>` directories.
- Shared-package clients (`pkg/sms`, `pkg/geocode`, `pkg/mail`, `pkg/revalidate`, `pkg/payuni`) are credited to the modules that import and wire them.
- First-party web revalidate and edge workers/serverless are included but labelled as such.

## Known blind spots
1. **Async coupling is excluded**: RabbitMQ events, outbox and webhook fan-out. Some pairs that look one-way here are also coupled through events.
2. **Non-gRPC HTTP**: the gateway reverse-proxies raw HTTP to media-service (5 secure upload/download/preview routes), invoice-service (3 PDF routes on file port 10042) and payuni-webhook-service (PayUni notify + 3 simulator routes). None of these are in calls.csv.
3. **Wrapper fan-in is not measured**: a wrapper method called from 20 handlers still counts as 1 site.
4. **Dynamic invocation**: I searched Go for `conn.Invoke`/`NewStream` with method strings and found none. Java reflection or dynamic stubs are not checked; none are expected.
5. **Java bytecode freshness**: the results depend on `target/classes` matching the source. It did at analysis time, but a later source edit without a rebuild would make them stale.
6. **Deploy-time wiring is not modelled**. Examples: gateway `PRODUCT_SERVICE_HOST` serves both ProductService and CategoryService; Java uses Nacos and `GrpcStubFactory` channel names. A proto service could in principle be routed to a different host at runtime. Unverified; the code registration is taken as the truth.
7. **Unreached call sites still count**: calls behind feature flags or code paths that never run in production are included.
8. **22 of 730 RPCs have no cross-deployable caller**, e.g. all 4 MembershipBillingService RPCs, 4 IMService admin RPCs and 5 SellerService PayUni/validate RPCs. They may be dead, MQ-driven or future work (unverified).
