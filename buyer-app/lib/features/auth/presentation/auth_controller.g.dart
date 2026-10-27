// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'auth_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 唯一的登入入口：以 Clerk 登入（Google、LINE 經系統瀏覽器，Apple 用 Sign in with Apple），再以 GetSession
/// 取得帳號（第一次登入的使用者由伺服器在這時建立），寫入 session。
///
/// keepAlive：登入流程含原生 picker / 畫面切換的多個 await，期間 watch 此
/// controller 的 widget 可能釋放。Riverpod 3 下 autoDispose 後再用 ref 會拋
/// 「Cannot use the Ref ... after it has been disposed」，導致流程在
/// ref.read 直接中斷。keepAlive 讓 controller 在整段非同步流程存活。

@ProviderFor(AuthController)
final authControllerProvider = AuthControllerProvider._();

/// 唯一的登入入口：以 Clerk 登入（Google、LINE 經系統瀏覽器，Apple 用 Sign in with Apple），再以 GetSession
/// 取得帳號（第一次登入的使用者由伺服器在這時建立），寫入 session。
///
/// keepAlive：登入流程含原生 picker / 畫面切換的多個 await，期間 watch 此
/// controller 的 widget 可能釋放。Riverpod 3 下 autoDispose 後再用 ref 會拋
/// 「Cannot use the Ref ... after it has been disposed」，導致流程在
/// ref.read 直接中斷。keepAlive 讓 controller 在整段非同步流程存活。
final class AuthControllerProvider
    extends $NotifierProvider<AuthController, AsyncValue<void>> {
  /// 唯一的登入入口：以 Clerk 登入（Google、LINE 經系統瀏覽器，Apple 用 Sign in with Apple），再以 GetSession
  /// 取得帳號（第一次登入的使用者由伺服器在這時建立），寫入 session。
  ///
  /// keepAlive：登入流程含原生 picker / 畫面切換的多個 await，期間 watch 此
  /// controller 的 widget 可能釋放。Riverpod 3 下 autoDispose 後再用 ref 會拋
  /// 「Cannot use the Ref ... after it has been disposed」，導致流程在
  /// ref.read 直接中斷。keepAlive 讓 controller 在整段非同步流程存活。
  AuthControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'authControllerProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$authControllerHash();

  @$internal
  @override
  AuthController create() => AuthController();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AsyncValue<void> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AsyncValue<void>>(value),
    );
  }
}

String _$authControllerHash() => r'3ce6d099b82a1a1611d96a176461a0193103712b';

/// 唯一的登入入口：以 Clerk 登入（Google、LINE 經系統瀏覽器，Apple 用 Sign in with Apple），再以 GetSession
/// 取得帳號（第一次登入的使用者由伺服器在這時建立），寫入 session。
///
/// keepAlive：登入流程含原生 picker / 畫面切換的多個 await，期間 watch 此
/// controller 的 widget 可能釋放。Riverpod 3 下 autoDispose 後再用 ref 會拋
/// 「Cannot use the Ref ... after it has been disposed」，導致流程在
/// ref.read 直接中斷。keepAlive 讓 controller 在整段非同步流程存活。

abstract class _$AuthController extends $Notifier<AsyncValue<void>> {
  AsyncValue<void> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<void>, AsyncValue<void>>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<void>, AsyncValue<void>>,
              AsyncValue<void>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
