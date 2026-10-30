// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'auth_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 登入。
///
/// keepAlive：登入流程含原生 picker / 畫面切換的多個 await，期間 read 此
/// controller 的 widget（AuthScreen 只 read 不 watch）期間 provider 無 listener。
/// autoDispose 下 picker 回來後再用 ref 會拋「Cannot use the Ref ... after it
/// has been disposed」，導致流程直接中斷。keepAlive 讓 controller 在整段非同步流程存活（與 buyer app
/// 對齊）。

@ProviderFor(AuthController)
final authControllerProvider = AuthControllerProvider._();

/// 登入。
///
/// keepAlive：登入流程含原生 picker / 畫面切換的多個 await，期間 read 此
/// controller 的 widget（AuthScreen 只 read 不 watch）期間 provider 無 listener。
/// autoDispose 下 picker 回來後再用 ref 會拋「Cannot use the Ref ... after it
/// has been disposed」，導致流程直接中斷。keepAlive 讓 controller 在整段非同步流程存活（與 buyer app
/// 對齊）。
final class AuthControllerProvider
    extends $AsyncNotifierProvider<AuthController, void> {
  /// 登入。
  ///
  /// keepAlive：登入流程含原生 picker / 畫面切換的多個 await，期間 read 此
  /// controller 的 widget（AuthScreen 只 read 不 watch）期間 provider 無 listener。
  /// autoDispose 下 picker 回來後再用 ref 會拋「Cannot use the Ref ... after it
  /// has been disposed」，導致流程直接中斷。keepAlive 讓 controller 在整段非同步流程存活（與 buyer app
  /// 對齊）。
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
}

String _$authControllerHash() => r'59b9fa58ef9103e0548705002a47d066d2da1ded';

/// 登入。
///
/// keepAlive：登入流程含原生 picker / 畫面切換的多個 await，期間 read 此
/// controller 的 widget（AuthScreen 只 read 不 watch）期間 provider 無 listener。
/// autoDispose 下 picker 回來後再用 ref 會拋「Cannot use the Ref ... after it
/// has been disposed」，導致流程直接中斷。keepAlive 讓 controller 在整段非同步流程存活（與 buyer app
/// 對齊）。

abstract class _$AuthController extends $AsyncNotifier<void> {
  FutureOr<void> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<void>, void>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<void>, void>,
              AsyncValue<void>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
