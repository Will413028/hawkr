// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'legacy_report_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(legacyReportRepository)
final legacyReportRepositoryProvider = LegacyReportRepositoryProvider._();

final class LegacyReportRepositoryProvider
    extends
        $FunctionalProvider<
          LegacyReportRepository,
          LegacyReportRepository,
          LegacyReportRepository
        >
    with $Provider<LegacyReportRepository> {
  LegacyReportRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'legacyReportRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$legacyReportRepositoryHash();

  @$internal
  @override
  $ProviderElement<LegacyReportRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  LegacyReportRepository create(Ref ref) {
    return legacyReportRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(LegacyReportRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<LegacyReportRepository>(value),
    );
  }
}

String _$legacyReportRepositoryHash() =>
    r'30267235494150b499376ba69ce72455a1e33a6d';
