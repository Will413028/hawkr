import 'dart:async';

import 'package:hawkr_auth/restore_scheduler.dart';
import 'package:hawkr_api/transport.dart' show Code;

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../features/chat/data/chat_repository.dart';
import '../../features/auth/data/auth_repository.dart';
import '../../features/notification/data/push/manager.dart';
import 'store_refresh_signal.dart';
import '../../shared/utils/hk_log.dart';
import '../networking/dio_provider.dart';
import '../storage/auth_storage.dart';
import 'clerk_session_provider.dart';
import 'post_login_side_effects.dart';
import 'session_model.dart';

export 'session_model.dart' show SessionModel;

part 'session_provider.g.dart';

/// Session 狀態管理 Provider（@Riverpod codegen）
///
/// 登入由 Clerk 的 session 與 GetSession 決定。
///
/// 用法：
/// ```dart
/// // 讀取 session
/// final session = ref.watch(sessionProvider);
///
/// // 寫入 session（Clerk 登入、GetSession 之後）
/// await ref.read(sessionProvider.notifier).login(accountId: 'acc-1');
/// ```
@Riverpod(keepAlive: true)
class Session extends _$Session {
  @override
  SessionModel build() {
    ref.onDispose(_restore.cancel);
    return const SessionModel();
  }

  /// [logout] 進行中旗標 — 見該方法註解的可重入循環說明。
  bool _loggingOut = false;
  final _restore = RestoreScheduler();
  set restoreError(String? message) =>
      ref.read(sessionRestoreErrorProvider.notifier).setMessage(message);

  /// App 啟動時以 Clerk session 呼叫 GetSession，取得目前的帳號。
  Future<void> initialize() async {
    final generation = _restore.cancel();
    if (!e2eAuthEnabled) {
      try {
        final clerk = await ref.read(clerkSessionProvider.future);
        if (!clerk.isSignedIn) return;
      } catch (error) {
        HKLog.logE('[SessionNotifier] Clerk 無法啟動，以訪客開始: $error');
        return;
      }
    }
    if (!ref.mounted || !_restore.isCurrent(generation)) return;
    restoreError = null;
    try {
      final accountId = await ref
          .read(authRepositoryProvider)
          .currentAccountId();
      if (!ref.mounted || !_restore.isCurrent(generation)) return;
      await AuthStorage.restoreAccount(accountId);
      if (!ref.mounted || !_restore.isCurrent(generation)) return;
      final session = SessionModel(accountId: accountId, isLoggedIn: true);

      state = session;
      _syncToStaticServices();
      await ref.read(postLoginSideEffectsProvider).onSessionRestored();

      HKLog.logD('[SessionNotifier] Session restored from GetSession');
    } catch (e) {
      if (!ref.mounted || !_restore.isCurrent(generation)) return;
      if (e is AuthRequestException && !e.isTransient) {
        if (e.code == Code.permissionDenied) {
          restoreError = e.message;
        }
        await logout();
      } else {
        retryRestore();
      }
      HKLog.logE('[SessionNotifier] Failed to restore session: $e');
    }
  }

  void cancelRestore() => _restore.cancel();

  void retryRestore() {
    _restore.retry(
      request: () => ref.read(authRepositoryProvider).currentAccountId(),
      onSuccess: (accountId) async {
        final generation = _restore.cancel();
        await AuthStorage.restoreAccount(accountId);
        if (!ref.mounted || !_restore.isCurrent(generation)) return;
        state = SessionModel(accountId: accountId, isLoggedIn: true);
        _syncToStaticServices();
        await ref.read(postLoginSideEffectsProvider).onSessionRestored();
      },
      isDenied: (error) => error is AuthRequestException && !error.isTransient,
      onDenied: (error) async {
        final rejection = error as AuthRequestException;
        if (rejection.code == Code.permissionDenied) {
          restoreError = rejection.message;
        }
        await logout();
      },
      onUnavailable: (error) =>
          HKLog.logD('[SessionNotifier] restore skipped: $error'),
    );
  }

  /// 登入 — Clerk 登入、GetSession 取得帳號之後寫入 session。店家（`GetMyStore`）由 `currentStoreProvider` 隨帳號讀取
  /// （46 號設計書「賣家能力權限層」§7.2：登入當下就知道店家、onboarding、停權與能力，不等撞 9240）；登入流程等它讀完。
  /// 在那之前 `needsOnboarding` 是 null，router 不導頁；讀取失敗而沒有快照時，router 顯示重試。
  Future<void> login({required String accountId}) async {
    restoreError = null;
    cancelRestore();
    await AuthStorage.restoreAccount(accountId);
    final session = SessionModel(accountId: accountId, isLoggedIn: true);

    state = session;
    _syncToStaticServices();
    ref.read(postLoginSideEffectsProvider).onLogin();

    HKLog.logD('[SessionNotifier] Logged in: accountId=$accountId');
  }

  /// session 中途收到「賣家身分已停權」的即時訊號（REST 的 9240、Connect 的 `SELLER_SUSPENDED`）：要求重讀
  /// `GetMyStore`（`currentStoreProvider` 收到訊號後重讀，正在重讀時不重複），停權、原因與能力以伺服器的答案為準。
  /// 已經停權時收到訊號代表手上的能力快照過期，同樣重讀。導向停權公告頁由 router 依狀態決定。
  void onSellerSuspendedSignal() {
    if (!state.isLoggedIn) return;
    ref.read(storeRefreshSignalProvider.notifier).request();
  }

  /// 登出 — 登出 Clerk（撤銷 session），清空所有 session 資料
  ///
  /// 可重入保護（[_loggingOut]）：登出流程自己會發 API 請求——
  /// [PushNotificationManager.onUserLoggedOut] 的 removeDevice 走中央
  /// [dioProvider]。session 此時可能已失效，那個 DELETE 回 401 時若又呼叫本方法，
  /// 就會 `logout → removeDevice → 401 → logout` 自我循環。removeDevice 已標記
  /// 401 不登出，這裡再擋一層。
  ///
  /// 順序：先註銷裝置（請求帶 Clerk 的 token），再登出 Clerk，最後清本機。
  Future<void> logout() async {
    cancelRestore();
    if (_loggingOut) return;
    _loggingOut = true;
    try {
      await PushNotificationManager.instance.onUserLoggedOut(
        hasCredential: state.isLoggedIn,
      );
      await _signOutOfClerk();

      state = const SessionModel();

      // 停止 polling
      ref.read(chatRepositoryProvider).stopPolling();
      await AuthStorage.logout();
      // 同步非敏感識別狀態；token 由中央 dioProvider 即時讀 session，不做 static fan-out。
      _syncToStaticServices();

      HKLog.logD('[SessionNotifier] Logged out');
    } finally {
      _loggingOut = false;
    }
  }

  /// 登出 Clerk（撤銷 session、清除保存的狀態）。失敗只記 log，本機照樣登出。
  Future<void> _signOutOfClerk() async {
    if (e2eAuthEnabled) return;
    try {
      await (await ref.read(clerkSessionProvider.future)).signOut();
    } catch (e) {
      HKLog.logW('[SessionNotifier] Clerk sign-out failed: $e');
    }
  }

  /// 同步不含 secret 的 legacy singleton 狀態。
  void _syncToStaticServices() {
    // Gateway auth 由 dioProvider 即時讀 sessionProvider；這裡只保留需要的
    // non-secret identity，以及 chat legacy singleton 的中央 Dio dependency。
    ref.read(chatRepositoryProvider).useHttpClient(ref.read(dioProvider));
  }
}

@Riverpod(keepAlive: true)
class SessionRestoreError extends _$SessionRestoreError {
  @override
  String? build() => null;

  void setMessage(String? message) => state = message;
}
