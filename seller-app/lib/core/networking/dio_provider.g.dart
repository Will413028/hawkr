// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'dio_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Dio instance 的 Riverpod Provider
///
/// 新的 feature repository 應透過 `ref.watch(dioProvider)` 取得 Dio instance，
/// 而非直接使用 `DioManager.instance.dio`。
///
/// Interceptor 順序：
/// 1. Auth interceptor — 注入 identity 的 session token（[sessionTokenInterceptor]）
/// 2. Logging interceptor — 統一日誌輸出
/// 3. Unauthorized interceptor — 401 登出
/// 4. Suspension interceptor — 9240 全域偵測（46 §7.2）

@ProviderFor(dio)
final dioProvider = DioProvider._();

/// Dio instance 的 Riverpod Provider
///
/// 新的 feature repository 應透過 `ref.watch(dioProvider)` 取得 Dio instance，
/// 而非直接使用 `DioManager.instance.dio`。
///
/// Interceptor 順序：
/// 1. Auth interceptor — 注入 identity 的 session token（[sessionTokenInterceptor]）
/// 2. Logging interceptor — 統一日誌輸出
/// 3. Unauthorized interceptor — 401 登出
/// 4. Suspension interceptor — 9240 全域偵測（46 §7.2）

final class DioProvider extends $FunctionalProvider<Dio, Dio, Dio>
    with $Provider<Dio> {
  /// Dio instance 的 Riverpod Provider
  ///
  /// 新的 feature repository 應透過 `ref.watch(dioProvider)` 取得 Dio instance，
  /// 而非直接使用 `DioManager.instance.dio`。
  ///
  /// Interceptor 順序：
  /// 1. Auth interceptor — 注入 identity 的 session token（[sessionTokenInterceptor]）
  /// 2. Logging interceptor — 統一日誌輸出
  /// 3. Unauthorized interceptor — 401 登出
  /// 4. Suspension interceptor — 9240 全域偵測（46 §7.2）
  DioProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'dioProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$dioHash();

  @$internal
  @override
  $ProviderElement<Dio> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  Dio create(Ref ref) {
    return dio(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(Dio value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<Dio>(value),
    );
  }
}

String _$dioHash() => r'8123f87a1a05a956ab8ff52dcaf507423d62d916';
