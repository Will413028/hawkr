// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'shipment_anomaly_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(shipmentAnomalyRepository)
final shipmentAnomalyRepositoryProvider = ShipmentAnomalyRepositoryProvider._();

final class ShipmentAnomalyRepositoryProvider
    extends
        $FunctionalProvider<
          ShipmentAnomalyRepository,
          ShipmentAnomalyRepository,
          ShipmentAnomalyRepository
        >
    with $Provider<ShipmentAnomalyRepository> {
  ShipmentAnomalyRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'shipmentAnomalyRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$shipmentAnomalyRepositoryHash();

  @$internal
  @override
  $ProviderElement<ShipmentAnomalyRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ShipmentAnomalyRepository create(Ref ref) {
    return shipmentAnomalyRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ShipmentAnomalyRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ShipmentAnomalyRepository>(value),
    );
  }
}

String _$shipmentAnomalyRepositoryHash() =>
    r'fe6cfb00fe3e08654569123d37f7086670b0f898';
