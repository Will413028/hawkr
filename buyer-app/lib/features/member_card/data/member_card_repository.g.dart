// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'member_card_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(memberCardRepository)
final memberCardRepositoryProvider = MemberCardRepositoryProvider._();

final class MemberCardRepositoryProvider
    extends
        $FunctionalProvider<
          MemberCardRepository,
          MemberCardRepository,
          MemberCardRepository
        >
    with $Provider<MemberCardRepository> {
  MemberCardRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'memberCardRepositoryProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$memberCardRepositoryHash();

  @$internal
  @override
  $ProviderElement<MemberCardRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  MemberCardRepository create(Ref ref) {
    return memberCardRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(MemberCardRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<MemberCardRepository>(value),
    );
  }
}

String _$memberCardRepositoryHash() =>
    r'3b9e76cc86543fbd54df85957c4c0dc41aaf5bd4';
