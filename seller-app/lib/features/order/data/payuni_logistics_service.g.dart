// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'payuni_logistics_service.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(payuniLogisticsService)
final payuniLogisticsServiceProvider = PayuniLogisticsServiceProvider._();

final class PayuniLogisticsServiceProvider
    extends
        $FunctionalProvider<
          PayuniLogisticsService,
          PayuniLogisticsService,
          PayuniLogisticsService
        >
    with $Provider<PayuniLogisticsService> {
  PayuniLogisticsServiceProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'payuniLogisticsServiceProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$payuniLogisticsServiceHash();

  @$internal
  @override
  $ProviderElement<PayuniLogisticsService> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  PayuniLogisticsService create(Ref ref) {
    return payuniLogisticsService(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(PayuniLogisticsService value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<PayuniLogisticsService>(value),
    );
  }
}

String _$payuniLogisticsServiceHash() =>
    r'e45dbe842fd89568d4747e8dc8f3d2bec4a56341';
