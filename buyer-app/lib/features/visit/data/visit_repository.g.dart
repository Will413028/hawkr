// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'visit_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(visitRepository)
final visitRepositoryProvider = VisitRepositoryProvider._();

final class VisitRepositoryProvider
    extends
        $FunctionalProvider<VisitRepository, VisitRepository, VisitRepository>
    with $Provider<VisitRepository> {
  VisitRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'visitRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$visitRepositoryHash();

  @$internal
  @override
  $ProviderElement<VisitRepository> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  VisitRepository create(Ref ref) {
    return visitRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(VisitRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<VisitRepository>(value),
    );
  }
}

String _$visitRepositoryHash() => r'cae37756d717cd7034eb6ecd08b780dd5a3bdc94';
