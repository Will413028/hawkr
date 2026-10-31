// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'session_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Session 狀態管理 Provider（@Riverpod codegen）
///
/// 登入由保存的 session token 與 GetSession 決定。
///
/// 用法：
/// ```dart
/// // 讀取 session
/// final session = ref.watch(sessionProvider);
///
/// // 寫入 session（登入、GetSession 之後）
/// await ref.read(sessionProvider.notifier).login(accountId: 'acc-1');
/// ```

@ProviderFor(Session)
final sessionProvider = SessionProvider._();

/// Session 狀態管理 Provider（@Riverpod codegen）
///
/// 登入由保存的 session token 與 GetSession 決定。
///
/// 用法：
/// ```dart
/// // 讀取 session
/// final session = ref.watch(sessionProvider);
///
/// // 寫入 session（登入、GetSession 之後）
/// await ref.read(sessionProvider.notifier).login(accountId: 'acc-1');
/// ```
final class SessionProvider extends $NotifierProvider<Session, SessionModel> {
  /// Session 狀態管理 Provider（@Riverpod codegen）
  ///
  /// 登入由保存的 session token 與 GetSession 決定。
  ///
  /// 用法：
  /// ```dart
  /// // 讀取 session
  /// final session = ref.watch(sessionProvider);
  ///
  /// // 寫入 session（登入、GetSession 之後）
  /// await ref.read(sessionProvider.notifier).login(accountId: 'acc-1');
  /// ```
  SessionProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sessionProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sessionHash();

  @$internal
  @override
  Session create() => Session();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SessionModel value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SessionModel>(value),
    );
  }
}

String _$sessionHash() => r'5f857c98b40f858ccb8dac624b4dc1a2cd71d6c8';

/// Session 狀態管理 Provider（@Riverpod codegen）
///
/// 登入由保存的 session token 與 GetSession 決定。
///
/// 用法：
/// ```dart
/// // 讀取 session
/// final session = ref.watch(sessionProvider);
///
/// // 寫入 session（登入、GetSession 之後）
/// await ref.read(sessionProvider.notifier).login(accountId: 'acc-1');
/// ```

abstract class _$Session extends $Notifier<SessionModel> {
  SessionModel build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<SessionModel, SessionModel>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<SessionModel, SessionModel>,
              SessionModel,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}

@ProviderFor(SessionRestoreError)
final sessionRestoreErrorProvider = SessionRestoreErrorProvider._();

final class SessionRestoreErrorProvider
    extends $NotifierProvider<SessionRestoreError, String?> {
  SessionRestoreErrorProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sessionRestoreErrorProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sessionRestoreErrorHash();

  @$internal
  @override
  SessionRestoreError create() => SessionRestoreError();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String?>(value),
    );
  }
}

String _$sessionRestoreErrorHash() =>
    r'0a635b8238a27f689cdb1eece5da5f84547e1cac';

abstract class _$SessionRestoreError extends $Notifier<String?> {
  String? build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<String?, String?>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<String?, String?>,
              String?,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
