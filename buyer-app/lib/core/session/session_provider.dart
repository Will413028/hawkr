import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:hawkr_auth/restore_scheduler.dart';

import '../../features/auth/data/auth_repository.dart';
import '../../features/cart/data/cart_repository.dart';
import '../storage/auth_storage.dart';
import '../../features/notification/data/push/background_handler.dart';
import '../../features/notification/data/push/manager.dart';
import '../../features/order/data/receipt_photo_repository.dart';
import '../../shared/utils/hk_log.dart';
import '../exceptions/app_exception.dart';
import 'force_logout_provider.dart';
import 'auth_session_provider.dart';
import 'session_model.dart';

part 'session_provider.g.dart';

/// 是否登入。登入是否仍然有效由伺服器判斷：session 失效（過期或被撤銷）時請求收到 401／unauthenticated，走強制登出。
bool hasUsableSession(SessionModel? session) {
  return session != null && session.userId.isNotEmpty;
}

/// 登入的帳號由保存的 session token 與 GetSession 決定。
@Riverpod(keepAlive: true)
class Session extends _$Session {
  final _restore = RestoreScheduler();
  @override
  Future<SessionModel?> build() async {
    final generation = _restore.cancel();
    ref.onDispose(_restore.cancel);
    if (!e2eAuthEnabled) {
      try {
        final authSession = await ref.watch(authSessionProvider.future);
        if (!authSession.isSignedIn) return null;
      } catch (e, st) {
        HKLog.logE('[Session] 登入狀態無法讀取，以訪客開始', e, st);
        return null;
      }
    }
    final String accountId;
    try {
      accountId = await ref.read(authRepositoryProvider).currentAccountId();
    } catch (error, stack) {
      if (!ref.mounted || !_restore.isCurrent(generation)) return null;
      if (error is UnauthorizedException || error is SuspendedException) {
        if (!e2eAuthEnabled) await _clearAuthSession();
        if (error is SuspendedException) {
          ref
              .read(forceLogoutProvider.notifier)
              .trigger(message: error.message);
        }
      } else {
        HKLog.logW('[Session] 帳號暫時無法恢復', error, stack);
        retryRestore();
      }
      return null;
    }
    if (!ref.mounted || !_restore.isCurrent(generation)) return null;
    return _restoredSession(accountId);
  }

  Future<SessionModel> _restoredSession(String accountId) async {
    final session = await _prepareSession(accountId);
    unawaited(
      Future<void>(() async {
        if (!ref.mounted) return;
        await _mergeGuestCart();
        final nickname = await fetchNickname();
        if (ref.mounted &&
            state.value?.userId == accountId &&
            nickname != null) {
          await updateNickname(nickname);
        }
      }),
    );
    // 每次 App 啟動都重新上報 FCM Token，確保 token refresh 後仍能收到推播。註冊請求要等 FCM
    // token 的原生呼叫回來才送出，那時這個 session 已經是 state，中央 Dio 帶得出憑證。
    PushNotificationManager.instance.onUserLoggedIn();
    return session;
  }

  void cancelRestore() => _restore.cancel();

  void retryRestore({Future<void> Function(String)? onSuccess}) {
    _restore.retry(
      request: () => ref.read(authRepositoryProvider).currentAccountId(),
      onSuccess: onSuccess ?? _restoreAccount,
      isDenied: (error) =>
          error is UnauthorizedException || error is SuspendedException,
      onDenied: (error) async {
        await logout();
        if (ref.mounted && error is SuspendedException) {
          ref
              .read(forceLogoutProvider.notifier)
              .trigger(message: error.message);
        }
      },
      onUnavailable: (error) => HKLog.logD('[Session] restore skipped: $error'),
    );
  }

  Future<void> _restoreAccount(String accountId) async {
    state = AsyncData(await _restoredSession(accountId));
  }

  /// 登入完成（session token 已保存、GetSession 取得帳號）之後寫入。
  Future<void> login({required String userId, String? nickname}) async {
    cancelRestore();
    state = AsyncData(await _prepareSession(userId, nickname: nickname));
    await _mergeGuestCart();
    await PushNotificationManager.instance.onUserLoggedIn();
  }

  Future<SessionModel> _prepareSession(
    String accountId, {
    String? nickname,
  }) async {
    String? previousOwner;
    try {
      previousOwner = await AuthStorage.cacheOwner();
    } catch (error, stack) {
      HKLog.logW('[Session] cache owner read failed', error, stack);
    }
    if (previousOwner != null && previousOwner != accountId) {
      await _invalidateUserScopedCaches();
    }
    try {
      await AuthStorage.setCacheOwner(accountId);
    } catch (error, stack) {
      HKLog.logW('[Session] cache owner write failed', error, stack);
    }
    return SessionModel(userId: accountId, nickname: nickname);
  }

  Future<void>? _mergingGuestCart;

  Future<void> _mergeGuestCart() => _mergingGuestCart ??= _mergeCart()
      .whenComplete(() => _mergingGuestCart = null);

  Future<void> _mergeCart() async {
    try {
      final guestId = await AuthStorage.existingGuestCartId();
      if (guestId == null || guestId.isEmpty) return;
      await ref.read(cartRepositoryProvider).mergeCarts(guestId);
      await AuthStorage.clearGuestCartId(guestId);
    } catch (error) {
      HKLog.logD('[Session] mergeCarts skipped: $error');
    }
  }

  Future<String?> fetchNickname() async {
    try {
      final profile = await ref.read(authRepositoryProvider).tryGetProfile();
      final nickname = (profile?.displayName ?? '').trim();
      return nickname.isEmpty ? null : nickname;
    } catch (error) {
      HKLog.logD('[Session] profile sync skipped: $error');
      return null;
    }
  }

  /// 更新暱稱（登入後讀到的個人資料），其他欄位不變；沒有登入時不做事。
  Future<void> updateNickname(String nickname) async {
    final current = state.value;
    if (current == null) return;
    state = AsyncData(current.copyWith(nickname: nickname));
  }

  /// 登出：先註銷裝置（請求由中央 Dio 帶 session token，要在清除 token 之前送出），再在伺服器撤銷 session
  /// （`SignOut`），清除保存的 token，最後清除本機的帳號與快取。
  Future<void> logout() async {
    cancelRestore();
    await PushNotificationManager.instance.onUserLoggedOut();
    if (!e2eAuthEnabled) {
      await _revokeServerSession();
      await _clearAuthSession();
    }
    state = const AsyncData(null);
    await _invalidateUserScopedCaches();
  }

  /// 在伺服器撤銷這個 session。失敗（含 session 已經失效）只記 log，本機照樣登出。
  Future<void> _revokeServerSession() async {
    try {
      final authSession = await ref.read(authSessionProvider.future);
      if (!authSession.isSignedIn) return;
      await ref.read(authRepositoryProvider).signOut();
    } catch (e, st) {
      HKLog.logW('[Session] 伺服器端登出失敗', e, st);
    }
  }

  /// 清除保存的 session token 並登出 Google、LINE 的 SDK。
  Future<void> _clearAuthSession() async {
    try {
      await (await ref.read(authSessionProvider.future)).clear();
    } catch (e, st) {
      HKLog.logE('[Session] 清除登入狀態失敗', e, st);
    }
  }

  /// 切換帳號時必須丟棄的 user-scoped 快取（keepAlive provider + 無帳號 scope
  /// 的本機持久化存儲）。新增這類快取時，請加進這個清單。
  ///
  /// 注意：**只能列出「沒有 ref.watch(sessionProvider)」的 provider**。
  /// 會 watch session 的 controller（如 addressController、afterSaleOrderLookup、
  /// couponController）
  /// 在 session state 變動時本來就會自動重建，不需也**不可**在此 invalidate——
  /// 從 Session 自身去 invalidate 依賴 Session 的 provider 會形成循環依賴
  /// （CircularDependencyError）。
  Future<void> _invalidateUserScopedCaches() async {
    // notification 列表/偏好、coupon 列表皆已改為 ref.watch(sessionProvider)，
    // 會自動重建，不在此 invalidate（避免循環依賴）。
    // SharedPreferences 持久化、無帳號 scope，provider invalidate 救不了，
    // 必須直接清底層存儲。
    await Future.wait([
      ref.read(receiptPhotoRepositoryProvider).clearAll(),
      clearFcmProcessedIds(),
    ]);
  }
}
