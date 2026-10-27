import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart' show kIsWeb;

import '../../../../core/networking/dio_provider.dart'
    show kSkipAuthInterceptorExtra;
import '../../../../shared/utils/hk_log.dart';

const String _devicesPath = '/api/v1/app/notifications/device-token';

enum DeviceMutationDisposition { success, retryableFailure, rejected, skipped }

/// Typed result for device-token mutations.
///
/// Response bodies are intentionally not retained: they may echo a device
/// token and must not accidentally flow into logs, crashes, or telemetry.
class DeviceMutationOutcome {
  const DeviceMutationOutcome._(this.disposition, {this.statusCode});

  const DeviceMutationOutcome.success({int? statusCode})
    : this._(DeviceMutationDisposition.success, statusCode: statusCode);

  const DeviceMutationOutcome.retryableFailure({int? statusCode})
    : this._(
        DeviceMutationDisposition.retryableFailure,
        statusCode: statusCode,
      );

  const DeviceMutationOutcome.rejected({int? statusCode})
    : this._(DeviceMutationDisposition.rejected, statusCode: statusCode);

  const DeviceMutationOutcome.skipped()
    : this._(DeviceMutationDisposition.skipped);

  final DeviceMutationDisposition disposition;
  final int? statusCode;

  bool get isSuccess => disposition == DeviceMutationDisposition.success;
  bool get isRetryable =>
      disposition == DeviceMutationDisposition.retryableFailure;
}

abstract interface class DeviceRegistrationGateway {
  Future<DeviceMutationOutcome> registerDevice({required String fcmToken});

  Future<DeviceMutationOutcome> removeDevice({required String fcmToken});
}

/// Gateway-backed device registration.
///
/// The injected Dio must be the app's central [dioProvider] client, which owns
/// auth injection, 401 logout, and 9240 suspension.
class NotificationDeviceRegistrar implements DeviceRegistrationGateway {
  NotificationDeviceRegistrar(this._dio, {String? platform})
    : _platformOverride = platform;

  final Dio _dio;
  final String? _platformOverride;

  String get _platform {
    final override = _platformOverride;
    if (override != null) return override;
    if (kIsWeb) return 'web';
    return 'ios';
  }

  @override
  Future<DeviceMutationOutcome> registerDevice({
    required String fcmToken,
  }) async {
    if (fcmToken.isEmpty) return const DeviceMutationOutcome.skipped();

    try {
      final response = await _dio.post<dynamic>(
        _devicesPath,
        data: {'token': fcmToken, 'platform': _platform},
        options: Options(validateStatus: (_) => true),
      );
      final outcome = _outcomeForStatus(response.statusCode);
      if (outcome.isSuccess) {
        HKLog.logI(
          '[NotificationService] Device registered '
          '(status=${response.statusCode ?? 0})',
        );
      } else {
        HKLog.logW(
          '[NotificationService] registerDevice failed '
          '(status=${response.statusCode ?? 0}, '
          'retryable=${outcome.isRetryable})',
        );
      }
      return outcome;
    } on DioException catch (error) {
      final outcome = _outcomeForDioException(error);
      HKLog.logW(
        '[NotificationService] registerDevice transport failure '
        '(type=${error.type}, retryable=${outcome.isRetryable})',
      );
      return outcome;
    } catch (_) {
      HKLog.logW(
        '[NotificationService] registerDevice unexpected transport failure',
      );
      return const DeviceMutationOutcome.retryableFailure();
    }
  }

  @override
  Future<DeviceMutationOutcome> removeDevice({required String fcmToken}) async {
    if (fcmToken.isEmpty) {
      return const DeviceMutationOutcome.skipped();
    }

    try {
      final response = await _dio.delete<dynamic>(
        _devicesPath,
        data: {'token': fcmToken},
        // 註銷發生在登出流程中，憑證隨時可能已失效；那個 401 是預期結果，
        // 不該觸發 refresh / logout，否則 logout 會再發一次註銷（#106 迴圈）。
        options: Options(
          extra: const {kSkipAuthInterceptorExtra: true},
          validateStatus: (_) => true,
        ),
      );
      final outcome = _outcomeForStatus(response.statusCode);
      if (outcome.isSuccess) {
        HKLog.logI(
          '[NotificationService] Device removed '
          '(status=${response.statusCode ?? 0})',
        );
      } else {
        HKLog.logW(
          '[NotificationService] removeDevice failed '
          '(status=${response.statusCode ?? 0})',
        );
      }
      return outcome;
    } on DioException catch (error) {
      final outcome = _outcomeForDioException(error);
      HKLog.logW(
        '[NotificationService] removeDevice transport failure '
        '(type=${error.type})',
      );
      return outcome;
    } catch (_) {
      HKLog.logW(
        '[NotificationService] removeDevice unexpected transport failure',
      );
      return const DeviceMutationOutcome.retryableFailure();
    }
  }
}

DeviceMutationOutcome _outcomeForStatus(int? statusCode) {
  final status = statusCode ?? 0;
  if (status >= 200 && status < 300) {
    return DeviceMutationOutcome.success(statusCode: status);
  }
  if (status == 408 ||
      status == 425 ||
      status == 429 ||
      status >= 500 ||
      status == 0) {
    return DeviceMutationOutcome.retryableFailure(statusCode: status);
  }
  return DeviceMutationOutcome.rejected(statusCode: status);
}

DeviceMutationOutcome _outcomeForDioException(DioException error) {
  final status = error.response?.statusCode;
  if (status != null) return _outcomeForStatus(status);
  return switch (error.type) {
    DioExceptionType.connectionTimeout ||
    DioExceptionType.sendTimeout ||
    DioExceptionType.receiveTimeout ||
    DioExceptionType.connectionError ||
    DioExceptionType.unknown => const DeviceMutationOutcome.retryableFailure(),
    DioExceptionType.badCertificate ||
    DioExceptionType.badResponse ||
    DioExceptionType.cancel => const DeviceMutationOutcome.rejected(),
  };
}

typedef RegistrationDelay = Future<void> Function(Duration duration);
typedef RegistrationClock = DateTime Function();

/// Owns retry, same-token single-flight, rotation, and success throttling.
///
/// Only retryable transport/5xx outcomes receive immediate capped backoff.
/// Rejected 4xx outcomes remain retryable on a later app resume because a
/// failure never advances [lastSuccessfulAt].
class DeviceRegistrationCoordinator {
  DeviceRegistrationCoordinator({
    required DeviceRegistrationGateway gateway,
    RegistrationDelay? delay,
    RegistrationClock? clock,
    List<Duration> retryDelays = const [
      Duration(milliseconds: 250),
      Duration(seconds: 1),
    ],
    int maxCallsPerWindow = defaultMaxCallsPerWindow,
    Duration breakerWindow = defaultBreakerWindow,
  }) : _gateway = gateway,
       _delay = delay ?? Future<void>.delayed,
       _clock = clock ?? DateTime.now,
       _retryDelays = List.unmodifiable(retryDelays),
       _maxCallsPerWindow = maxCallsPerWindow,
       _breakerWindow = breakerWindow;

  /// 客戶端斷路器預設閾值：register + remove 合計的 gateway 呼叫上限。
  ///
  /// 正常使用一個視窗內最多是「登入註冊 + token rotation + 登出註銷」各一次
  /// （回前景重註冊另有 24h 節流），即使每次都吃滿 bounded retry 也遠低於此值；
  /// 反之任何自我循環（#106 實測 ~4.8 req/s）幾秒內就會撞到，之後直接短路
  /// 不再送出，把「客戶端 bug 放大成對後端的 DoS」這條路從結構上切斷。
  static const int defaultMaxCallsPerWindow = 12;
  static const Duration defaultBreakerWindow = Duration(minutes: 5);

  final DeviceRegistrationGateway _gateway;
  final RegistrationDelay _delay;
  final RegistrationClock _clock;
  final List<Duration> _retryDelays;
  final int _maxCallsPerWindow;
  final Duration _breakerWindow;
  final List<DateTime> _recentCalls = [];

  String? _latestRequestedToken;
  String? _lastSuccessfulToken;
  DateTime? _lastSuccessfulAt;
  int _generation = 0;
  String? _activeToken;
  Future<DeviceMutationOutcome>? _activeOperation;
  String? _queuedToken;
  int? _queuedGeneration;
  Completer<DeviceMutationOutcome>? _queuedCompleter;
  String? _activeRemovalToken;
  Future<DeviceMutationOutcome>? _activeRemoval;

  DateTime? get lastSuccessfulAt => _lastSuccessfulAt;
  String? get lastSuccessfulToken => _lastSuccessfulToken;

  Future<DeviceMutationOutcome> register(String fcmToken) {
    if (fcmToken.isEmpty) {
      return Future.value(const DeviceMutationOutcome.skipped());
    }

    final active = _activeOperation;
    if (active != null) {
      if (_activeToken == fcmToken) return active;
      if (_queuedToken == fcmToken) return _queuedCompleter!.future;

      // Keep at most one rotation behind the active mutation. A newer token
      // supersedes an older queued token instead of creating another retry
      // chain and amplifying a transient outage.
      _queuedCompleter?.complete(const DeviceMutationOutcome.skipped());
      _latestRequestedToken = fcmToken;
      _queuedToken = fcmToken;
      _queuedGeneration = ++_generation;
      _queuedCompleter = Completer<DeviceMutationOutcome>();
      return _queuedCompleter!.future;
    }

    _latestRequestedToken = fcmToken;
    return _startRegistration(fcmToken, ++_generation);
  }

  Future<DeviceMutationOutcome> _startRegistration(
    String fcmToken,
    int generation,
  ) {
    late final Future<DeviceMutationOutcome> operation;
    operation = _registerWithRetry(fcmToken)
        .then((outcome) {
          if (outcome.isSuccess &&
              generation == _generation &&
              _latestRequestedToken == fcmToken) {
            _lastSuccessfulToken = fcmToken;
            _lastSuccessfulAt = _clock();
          }
          return outcome;
        })
        .whenComplete(() {
          if (identical(_activeOperation, operation)) {
            _activeOperation = null;
            _activeToken = null;
            _startQueuedRegistration();
          }
        });
    _activeToken = fcmToken;
    _activeOperation = operation;
    return operation;
  }

  void _startQueuedRegistration() {
    final token = _queuedToken;
    final generation = _queuedGeneration;
    final completer = _queuedCompleter;
    _queuedToken = null;
    _queuedGeneration = null;
    _queuedCompleter = null;
    if (token == null || generation == null || completer == null) return;

    _startRegistration(
      token,
      generation,
    ).then(completer.complete, onError: completer.completeError);
  }

  Future<DeviceMutationOutcome> _registerWithRetry(String fcmToken) async {
    final opposing = _activeRemoval;
    if (opposing != null) await _settleOpposing(opposing);
    var outcome = await _guarded(
      () => _gateway.registerDevice(fcmToken: fcmToken),
    );
    for (final backoff in _retryDelays) {
      if (!outcome.isRetryable) return outcome;
      await _delay(backoff);
      outcome = await _guarded(
        () => _gateway.registerDevice(fcmToken: fcmToken),
      );
    }
    return outcome;
  }

  Future<DeviceMutationOutcome> _removeWithRetry(String fcmToken) async {
    final opposing = _activeOperation;
    if (opposing != null) await _settleOpposing(opposing);
    var outcome = await _guarded(
      () => _gateway.removeDevice(fcmToken: fcmToken),
    );
    for (final backoff in _retryDelays) {
      if (!outcome.isRetryable) return outcome;
      await _delay(backoff);
      outcome = await _guarded(() => _gateway.removeDevice(fcmToken: fcmToken));
    }
    return outcome;
  }

  /// 等待反向 mutation 落地：register 與 remove 在後端寫的是同一把
  /// `(user_id, user_type, token)` 鑰匙的相反值（`UpsertDeviceToken` →
  /// `is_active=true`／`DeactivateDeviceToken` → `is_active=false`）。兩者各自
  /// single-flight 但彼此不互斥時，交錯落地會留下錯誤的終態——最惡性的是登出的
  /// DELETE 慢一步蓋掉重新登入的註冊：已登入卻整台裝置收不到推播，而 client
  /// 以為註冊成功、24h 內不再重試，等於長時間靜默且無法自癒。
  ///
  /// 只等「在自己之前就存在」的反向操作，晚生者永遠等早生者，不可能成環；
  /// 反向操作失敗與自己無關，吞掉不傳染。
  Future<void> _settleOpposing(Future<DeviceMutationOutcome> opposing) async {
    try {
      await opposing;
    } catch (_) {
      // 反向操作自己的失敗已在該路徑記錄，這裡只需要它「結束了」。
    }
  }

  /// 斷路器：register / remove 合計超過視窗閾值即短路，連請求都不送出。
  ///
  /// 這是保護層而非業務邏輯——正常使用永遠碰不到（見
  /// [defaultMaxCallsPerWindow]）；觸發即代表上游有迴圈，此時「少打」永遠
  /// 比「打好打滿」正確。
  Future<DeviceMutationOutcome> _guarded(
    Future<DeviceMutationOutcome> Function() call,
  ) {
    final now = _clock();
    _recentCalls.removeWhere((at) => now.difference(at) >= _breakerWindow);
    if (_recentCalls.length >= _maxCallsPerWindow) {
      HKLog.logW(
        '[NotificationService] device-token 呼叫短路：'
        '${_breakerWindow.inSeconds}s 內已達 $_maxCallsPerWindow 次上限',
      );
      return Future.value(const DeviceMutationOutcome.skipped());
    }
    _recentCalls.add(now);
    return call();
  }

  bool shouldThrottle(
    String fcmToken, {
    Duration throttle = const Duration(hours: 24),
  }) {
    final successAt = _lastSuccessfulAt;
    return successAt != null &&
        _lastSuccessfulToken == fcmToken &&
        _clock().difference(successAt) < throttle;
  }

  /// Mirrors [register]'s protections: same-token single-flight plus bounded
  /// backoff limited to retryable transport/5xx outcomes. A rejected 4xx
  /// (401/403 on a session that is already gone) is terminal — retrying it
  /// can never succeed and only amplifies load.
  Future<DeviceMutationOutcome> remove(String fcmToken) {
    if (fcmToken.isEmpty) {
      return Future.value(const DeviceMutationOutcome.skipped());
    }

    final active = _activeRemoval;
    if (active != null && _activeRemovalToken == fcmToken) return active;

    late final Future<DeviceMutationOutcome> operation;
    operation = _removeWithRetry(fcmToken).whenComplete(() {
      if (identical(_activeRemoval, operation)) {
        _activeRemoval = null;
        _activeRemovalToken = null;
      }
    });
    _activeRemovalToken = fcmToken;
    _activeRemoval = operation;
    return operation;
  }

  /// Invalidates success state and prevents late in-flight results from
  /// advancing the next session's throttle.
  ///
  /// [_recentCalls] is deliberately preserved: logout calls [reset], and the
  /// loop this guards against runs through logout — clearing the breaker here
  /// would hand every iteration a fresh budget.
  void reset() {
    _generation++;
    _latestRequestedToken = null;
    _lastSuccessfulToken = null;
    _lastSuccessfulAt = null;
    _queuedToken = null;
    _queuedGeneration = null;
    _queuedCompleter?.complete(const DeviceMutationOutcome.skipped());
    _queuedCompleter = null;
  }
}
