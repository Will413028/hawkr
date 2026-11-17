// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'account_restriction.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 受限狀態：null 是沒有停權。進入受限狀態不登出，session 與 token 都保留。
///
/// 來源有二：GetSession 的 `suspension`（登入與冷啟動），以及請求收到 `ACCOUNT_SUSPENDED`（登入期間才被停權）。

@ProviderFor(AccountRestriction)
final accountRestrictionProvider = AccountRestrictionProvider._();

/// 受限狀態：null 是沒有停權。進入受限狀態不登出，session 與 token 都保留。
///
/// 來源有二：GetSession 的 `suspension`（登入與冷啟動），以及請求收到 `ACCOUNT_SUSPENDED`（登入期間才被停權）。
final class AccountRestrictionProvider
    extends $NotifierProvider<AccountRestriction, AccountSuspension?> {
  /// 受限狀態：null 是沒有停權。進入受限狀態不登出，session 與 token 都保留。
  ///
  /// 來源有二：GetSession 的 `suspension`（登入與冷啟動），以及請求收到 `ACCOUNT_SUSPENDED`（登入期間才被停權）。
  AccountRestrictionProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'accountRestrictionProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$accountRestrictionHash();

  @$internal
  @override
  AccountRestriction create() => AccountRestriction();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AccountSuspension? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AccountSuspension?>(value),
    );
  }
}

String _$accountRestrictionHash() =>
    r'949387b4d7a93ed53ec84fc6d8bf338bffacbf76';

/// 受限狀態：null 是沒有停權。進入受限狀態不登出，session 與 token 都保留。
///
/// 來源有二：GetSession 的 `suspension`（登入與冷啟動），以及請求收到 `ACCOUNT_SUSPENDED`（登入期間才被停權）。

abstract class _$AccountRestriction extends $Notifier<AccountSuspension?> {
  AccountSuspension? build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AccountSuspension?, AccountSuspension?>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AccountSuspension?, AccountSuspension?>,
              AccountSuspension?,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}

/// 「帳號停權中，無法執行這個動作」提示的請求；每次 [raise] 加一，監聽的 widget 依變動顯示 bottom sheet。

@ProviderFor(SuspendedActionNotice)
final suspendedActionNoticeProvider = SuspendedActionNoticeProvider._();

/// 「帳號停權中，無法執行這個動作」提示的請求；每次 [raise] 加一，監聽的 widget 依變動顯示 bottom sheet。
final class SuspendedActionNoticeProvider
    extends $NotifierProvider<SuspendedActionNotice, int> {
  /// 「帳號停權中，無法執行這個動作」提示的請求；每次 [raise] 加一，監聽的 widget 依變動顯示 bottom sheet。
  SuspendedActionNoticeProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'suspendedActionNoticeProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$suspendedActionNoticeHash();

  @$internal
  @override
  SuspendedActionNotice create() => SuspendedActionNotice();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(int value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<int>(value),
    );
  }
}

String _$suspendedActionNoticeHash() =>
    r'b283aeb467bedffbbe0ce6546f4efe5c3c655350';

/// 「帳號停權中，無法執行這個動作」提示的請求；每次 [raise] 加一，監聽的 widget 依變動顯示 bottom sheet。

abstract class _$SuspendedActionNotice extends $Notifier<int> {
  int build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<int, int>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<int, int>,
              int,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
