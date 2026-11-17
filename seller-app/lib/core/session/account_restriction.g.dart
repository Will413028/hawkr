// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'account_restriction.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 目前登入帳號的受限狀態；null 是沒有受限。進入的來源：GetSession 回報停權，或前景請求收到 `ACCOUNT_SUSPENDED`。

@ProviderFor(AccountRestrictionState)
final accountRestrictionStateProvider = AccountRestrictionStateProvider._();

/// 目前登入帳號的受限狀態；null 是沒有受限。進入的來源：GetSession 回報停權，或前景請求收到 `ACCOUNT_SUSPENDED`。
final class AccountRestrictionStateProvider
    extends $NotifierProvider<AccountRestrictionState, AccountRestriction?> {
  /// 目前登入帳號的受限狀態；null 是沒有受限。進入的來源：GetSession 回報停權，或前景請求收到 `ACCOUNT_SUSPENDED`。
  AccountRestrictionStateProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'accountRestrictionStateProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$accountRestrictionStateHash();

  @$internal
  @override
  AccountRestrictionState create() => AccountRestrictionState();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AccountRestriction? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AccountRestriction?>(value),
    );
  }
}

String _$accountRestrictionStateHash() =>
    r'5b02b6fe4e82a664606ea59f8982294765f9734a';

/// 目前登入帳號的受限狀態；null 是沒有受限。進入的來源：GetSession 回報停權，或前景請求收到 `ACCOUNT_SUSPENDED`。

abstract class _$AccountRestrictionState
    extends $Notifier<AccountRestriction?> {
  AccountRestriction? build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AccountRestriction?, AccountRestriction?>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AccountRestriction?, AccountRestriction?>,
              AccountRestriction?,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
