import 'package:hawkr_buyer/shared/utils/hk_log.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sentry_flutter/sentry_flutter.dart';

void main() {
  tearDown(() {
    HKLog.sentryExceptionSink = HKLog.defaultSentryExceptionSink;
    HKLog.sentryBreadcrumbSink = HKLog.defaultSentryBreadcrumbSink;
    HKLog.sentryMessageSink = HKLog.defaultSentryMessageSink;
  });

  test('logE with an error object forwards to the exception sink', () {
    Object? capturedThrowable;
    StackTrace? capturedStackTrace;
    HKLog.sentryExceptionSink = (throwable, stackTrace) {
      capturedThrowable = throwable;
      capturedStackTrace = stackTrace;
    };

    final error = Exception('boom');
    final trace = StackTrace.current;
    HKLog.logE('something broke', error, trace);

    expect(capturedThrowable, error);
    expect(capturedStackTrace, trace);
  });

  test(
    'logE without an error object forwards the message as a SentryEvent capture',
    () {
      String? capturedMessage;
      HKLog.sentryMessageSink = (message) => capturedMessage = message;

      HKLog.logE('something broke with no error object');

      expect(capturedMessage, 'something broke with no error object');
    },
  );

  test('logW forwards a warning-level breadcrumb, not an exception', () {
    Breadcrumb? captured;
    HKLog.sentryBreadcrumbSink = (crumb) => captured = crumb;
    var exceptionSinkCalled = false;
    HKLog.sentryExceptionSink = (_, __) => exceptionSinkCalled = true;

    HKLog.logW('validation failed, expected');

    expect(captured?.message, 'validation failed, expected');
    expect(captured?.level, SentryLevel.warning);
    expect(exceptionSinkCalled, isFalse);
  });

  test('logD does not touch either sink', () {
    var breadcrumbCalled = false;
    var exceptionCalled = false;
    HKLog.sentryBreadcrumbSink = (_) => breadcrumbCalled = true;
    HKLog.sentryExceptionSink = (_, __) => exceptionCalled = true;

    HKLog.logD('verbose request/response dump');

    expect(breadcrumbCalled, isFalse);
    expect(exceptionCalled, isFalse);
  });

  test('redacts a known secret before warning and error Sentry sinks', () {
    const sentinel = 'KNOWN-SECRET-SENTINEL';
    Breadcrumb? breadcrumb;
    String? errorMessage;
    HKLog.sentryBreadcrumbSink = (value) => breadcrumb = value;
    HKLog.sentryMessageSink = (value) => errorMessage = value;

    HKLog.logW('device_token=$sentinel');
    HKLog.logE('Authorization: Bearer $sentinel');

    expect(breadcrumb?.message, isNot(contains(sentinel)));
    expect(errorMessage, isNot(contains(sentinel)));
  });
}
