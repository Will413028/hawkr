// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'after_sale_order_lookup.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 快取「哪些 orderNo 有活躍售後案件」+ 該案件狀態，供 order card 查詢。
/// keepAlive 避免每次 rebuild 都重撈。

@ProviderFor(AfterSaleOrderLookup)
final afterSaleOrderLookupProvider = AfterSaleOrderLookupProvider._();

/// 快取「哪些 orderNo 有活躍售後案件」+ 該案件狀態，供 order card 查詢。
/// keepAlive 避免每次 rebuild 都重撈。
final class AfterSaleOrderLookupProvider
    extends
        $AsyncNotifierProvider<
          AfterSaleOrderLookup,
          Map<String, AfterSaleCaseSnapshot>
        > {
  /// 快取「哪些 orderNo 有活躍售後案件」+ 該案件狀態，供 order card 查詢。
  /// keepAlive 避免每次 rebuild 都重撈。
  AfterSaleOrderLookupProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'afterSaleOrderLookupProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$afterSaleOrderLookupHash();

  @$internal
  @override
  AfterSaleOrderLookup create() => AfterSaleOrderLookup();
}

String _$afterSaleOrderLookupHash() =>
    r'5d8be4906be0ae61a6c0a9c6f67b0d0d0bcdcfa9';

/// 快取「哪些 orderNo 有活躍售後案件」+ 該案件狀態，供 order card 查詢。
/// keepAlive 避免每次 rebuild 都重撈。

abstract class _$AfterSaleOrderLookup
    extends $AsyncNotifier<Map<String, AfterSaleCaseSnapshot>> {
  FutureOr<Map<String, AfterSaleCaseSnapshot>> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref
            as $Ref<
              AsyncValue<Map<String, AfterSaleCaseSnapshot>>,
              Map<String, AfterSaleCaseSnapshot>
            >;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<
                AsyncValue<Map<String, AfterSaleCaseSnapshot>>,
                Map<String, AfterSaleCaseSnapshot>
              >,
              AsyncValue<Map<String, AfterSaleCaseSnapshot>>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
