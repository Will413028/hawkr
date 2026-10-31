// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'auto_login_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 冷啟動自動登入：有保存的 session token 時以 GetSession 恢復帳號。
///
/// 登入是否仍然有效由伺服器判斷：失效時請求收到 401，由中央 Dio 登出（見 `dio_provider.dart`）。
///
/// keepAlive：整段流程只碰自己的 `ref`，與呼叫它的 widget 的生命週期脫鉤。widget 若在流程中途
/// dispose（導頁離開），續行程式碼不會對已失效的 `WidgetRef` 呼叫 `read`；session 狀態一變化，
/// GoRouter 的 redirect 照樣會導頁。

@ProviderFor(AutoLoginController)
final autoLoginControllerProvider = AutoLoginControllerProvider._();

/// 冷啟動自動登入：有保存的 session token 時以 GetSession 恢復帳號。
///
/// 登入是否仍然有效由伺服器判斷：失效時請求收到 401，由中央 Dio 登出（見 `dio_provider.dart`）。
///
/// keepAlive：整段流程只碰自己的 `ref`，與呼叫它的 widget 的生命週期脫鉤。widget 若在流程中途
/// dispose（導頁離開），續行程式碼不會對已失效的 `WidgetRef` 呼叫 `read`；session 狀態一變化，
/// GoRouter 的 redirect 照樣會導頁。
final class AutoLoginControllerProvider
    extends $NotifierProvider<AutoLoginController, void> {
  /// 冷啟動自動登入：有保存的 session token 時以 GetSession 恢復帳號。
  ///
  /// 登入是否仍然有效由伺服器判斷：失效時請求收到 401，由中央 Dio 登出（見 `dio_provider.dart`）。
  ///
  /// keepAlive：整段流程只碰自己的 `ref`，與呼叫它的 widget 的生命週期脫鉤。widget 若在流程中途
  /// dispose（導頁離開），續行程式碼不會對已失效的 `WidgetRef` 呼叫 `read`；session 狀態一變化，
  /// GoRouter 的 redirect 照樣會導頁。
  AutoLoginControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'autoLoginControllerProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$autoLoginControllerHash();

  @$internal
  @override
  AutoLoginController create() => AutoLoginController();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(void value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<void>(value),
    );
  }
}

String _$autoLoginControllerHash() =>
    r'0bc59244034ed45b78c050fd61756ffcc63467d5';

/// 冷啟動自動登入：有保存的 session token 時以 GetSession 恢復帳號。
///
/// 登入是否仍然有效由伺服器判斷：失效時請求收到 401，由中央 Dio 登出（見 `dio_provider.dart`）。
///
/// keepAlive：整段流程只碰自己的 `ref`，與呼叫它的 widget 的生命週期脫鉤。widget 若在流程中途
/// dispose（導頁離開），續行程式碼不會對已失效的 `WidgetRef` 呼叫 `read`；session 狀態一變化，
/// GoRouter 的 redirect 照樣會導頁。

abstract class _$AutoLoginController extends $Notifier<void> {
  void build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<void, void>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<void, void>,
              void,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
