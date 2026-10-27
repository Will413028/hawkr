// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'session_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 登入的帳號由 Clerk 的 session 與 GetSession 決定。

@ProviderFor(Session)
final sessionProvider = SessionProvider._();

/// 登入的帳號由 Clerk 的 session 與 GetSession 決定。
final class SessionProvider
    extends $AsyncNotifierProvider<Session, SessionModel?> {
  /// 登入的帳號由 Clerk 的 session 與 GetSession 決定。
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
}

String _$sessionHash() => r'0bfb308fdeaa659d83820703c84abbaa6c1d6092';

/// 登入的帳號由 Clerk 的 session 與 GetSession 決定。

abstract class _$Session extends $AsyncNotifier<SessionModel?> {
  FutureOr<SessionModel?> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<SessionModel?>, SessionModel?>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<SessionModel?>, SessionModel?>,
              AsyncValue<SessionModel?>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
