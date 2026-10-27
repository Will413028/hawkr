import 'package:logger/logger.dart';
import 'package:sentry_flutter/sentry_flutter.dart';

import 'secret_redaction.dart';

/// Hawkr 統一日誌工具
///
/// 提供統一的全域日誌方法，包含 logD, logI, logW, logE。
/// logE/logW 額外轉發到 Sentry（見 sentryExceptionSink/sentryBreadcrumbSink）：
/// logE 開獨立 event，logW 只留 breadcrumb（避免 Dio 每個 4xx 都洗版 Sentry）。
/// 所有 message 在本地 logger 與 Sentry sink 前都先通過敏感資訊遮罩。
/// logD/logI/logV 不碰 Sentry；logW/logE 仍依下方規則送出遮罩後內容。
class HKLog {
  static final Logger _logger = Logger(
    printer: PrettyPrinter(
      methodCount: 0,
      errorMethodCount: 3,
      lineLength: 80,
      colors: false,
      printEmojis: true,
      dateTimeFormat: DateTimeFormat.none,
    ),
  );

  static void defaultSentryExceptionSink(
    dynamic throwable,
    StackTrace? stackTrace,
  ) {
    Sentry.captureException(throwable, stackTrace: stackTrace);
  }

  static void defaultSentryMessageSink(String message) {
    Sentry.captureMessage(message, level: SentryLevel.error);
  }

  static void defaultSentryBreadcrumbSink(Breadcrumb breadcrumb) {
    Sentry.addBreadcrumb(breadcrumb);
  }

  /// 測試用可替換 seam，正式執行時維持 default（呼叫真的 Sentry API）。
  static void Function(dynamic throwable, StackTrace? stackTrace)
  sentryExceptionSink = defaultSentryExceptionSink;
  static void Function(String message) sentryMessageSink =
      defaultSentryMessageSink;
  static void Function(Breadcrumb breadcrumb) sentryBreadcrumbSink =
      defaultSentryBreadcrumbSink;

  /// Debug 日誌（綠色）
  ///
  /// 用於開發時期的詳細資訊，生產環境不會輸出。不轉發到 Sentry。
  static void logD(String message, [dynamic error, StackTrace? stackTrace]) {
    _logger.d(
      redactSensitiveText(message),
      error: error,
      stackTrace: stackTrace,
    );
  }

  /// Info 日誌（藍色）
  ///
  /// 用於一般資訊記錄。不轉發到 Sentry。
  static void logI(String message, [dynamic error, StackTrace? stackTrace]) {
    _logger.i(
      redactSensitiveText(message),
      error: error,
      stackTrace: stackTrace,
    );
  }

  /// Warning 日誌（橙色）
  ///
  /// 用於警告資訊。轉發為 Sentry breadcrumb（不開獨立 event）。
  static void logW(String message, [dynamic error, StackTrace? stackTrace]) {
    final safeMessage = redactSensitiveText(message);
    _logger.w(safeMessage, error: error, stackTrace: stackTrace);
    sentryBreadcrumbSink(
      Breadcrumb(
        message: safeMessage,
        level: SentryLevel.warning,
        category: 'app.log',
      ),
    );
  }

  /// Error 日誌（紅色）
  ///
  /// 用於錯誤資訊，會包含 stack trace。轉發到 Sentry 開獨立 event：
  /// 有 error 物件 → captureException；只有訊息 → captureMessage(level=error)。
  static void logE(String message, [dynamic error, StackTrace? stackTrace]) {
    final safeMessage = redactSensitiveText(message);
    _logger.e(safeMessage, error: error, stackTrace: stackTrace);
    if (error != null) {
      sentryExceptionSink(error, stackTrace);
    } else {
      sentryMessageSink(safeMessage);
    }
  }

  /// Verbose 日誌
  ///
  /// 用於非常詳細的資訊。不轉發到 Sentry。
  static void logV(String message, [dynamic error, StackTrace? stackTrace]) {
    _logger.t(
      redactSensitiveText(message),
      error: error,
      stackTrace: stackTrace,
    );
  }

  /// 設定日誌等級
  ///
  /// [level] 可以是 Level.debug, Level.info, Level.warning, Level.error
  static void setLevel(Level level) {
    Logger.level = level;
  }

  /// 關閉所有日誌（用於生產環境）
  static void disable() {
    Logger.level = Level.off;
  }

  /// 開啟所有日誌（用於開發環境）
  static void enable() {
    Logger.level = Level.trace;
  }
}
