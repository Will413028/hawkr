// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'force_logout_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 強制登出提醒旗標。
///
/// 當 session 失效（請求收到 401／unauthenticated）時設為 true，
/// 由 app 根層 widget 監聽此狀態顯示對話框，
/// 對話框與未登入提示一致；無論取消或前往登入都會清除失效 session。

@ProviderFor(ForceLogout)
final forceLogoutProvider = ForceLogoutProvider._();

/// 強制登出提醒旗標。
///
/// 當 session 失效（請求收到 401／unauthenticated）時設為 true，
/// 由 app 根層 widget 監聽此狀態顯示對話框，
/// 對話框與未登入提示一致；無論取消或前往登入都會清除失效 session。
final class ForceLogoutProvider extends $NotifierProvider<ForceLogout, bool> {
  /// 強制登出提醒旗標。
  ///
  /// 當 session 失效（請求收到 401／unauthenticated）時設為 true，
  /// 由 app 根層 widget 監聽此狀態顯示對話框，
  /// 對話框與未登入提示一致；無論取消或前往登入都會清除失效 session。
  ForceLogoutProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'forceLogoutProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$forceLogoutHash();

  @$internal
  @override
  ForceLogout create() => ForceLogout();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(bool value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<bool>(value),
    );
  }
}

String _$forceLogoutHash() => r'987d1f5e593c4946f5a4adc1bd9ac1d8c9eca352';

/// 強制登出提醒旗標。
///
/// 當 session 失效（請求收到 401／unauthenticated）時設為 true，
/// 由 app 根層 widget 監聽此狀態顯示對話框，
/// 對話框與未登入提示一致；無論取消或前往登入都會清除失效 session。

abstract class _$ForceLogout extends $Notifier<bool> {
  bool build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<bool, bool>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<bool, bool>,
              bool,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
