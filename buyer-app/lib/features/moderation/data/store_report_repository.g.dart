// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'store_report_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(storeReportRepository)
final storeReportRepositoryProvider = StoreReportRepositoryProvider._();

final class StoreReportRepositoryProvider
    extends
        $FunctionalProvider<
          StoreReportRepository,
          StoreReportRepository,
          StoreReportRepository
        >
    with $Provider<StoreReportRepository> {
  StoreReportRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'storeReportRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$storeReportRepositoryHash();

  @$internal
  @override
  $ProviderElement<StoreReportRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  StoreReportRepository create(Ref ref) {
    return storeReportRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(StoreReportRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<StoreReportRepository>(value),
    );
  }
}

String _$storeReportRepositoryHash() =>
    r'3ef8280dd87b04b68434b1c84c720c8e8df454b1';
