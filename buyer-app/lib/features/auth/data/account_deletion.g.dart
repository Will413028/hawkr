// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'account_deletion.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(accountDeletion)
final accountDeletionProvider = AccountDeletionProvider._();

final class AccountDeletionProvider
    extends
        $FunctionalProvider<AccountDeletion, AccountDeletion, AccountDeletion>
    with $Provider<AccountDeletion> {
  AccountDeletionProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'accountDeletionProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$accountDeletionHash();

  @$internal
  @override
  $ProviderElement<AccountDeletion> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  AccountDeletion create(Ref ref) {
    return accountDeletion(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AccountDeletion value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AccountDeletion>(value),
    );
  }
}

String _$accountDeletionHash() => r'6f70c892b3992db5faa92421a9c8a9769653cf8d';
