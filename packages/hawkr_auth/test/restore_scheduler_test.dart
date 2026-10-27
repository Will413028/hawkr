import 'dart:async';

import 'package:hawkr_auth/restore_scheduler.dart';
import 'package:fake_async/fake_async.dart';
import 'package:test/test.dart';

void main() {
  test('三次背景重試依 1.5、3、6 秒退避後停止', () {
    fakeAsync((clock) {
      final scheduler = RestoreScheduler();
      var calls = 0;
      var unavailable = 0;
      scheduler.retry<String>(
        request: () async {
          calls++;
          throw StateError('offline');
        },
        onSuccess: (_) async => fail('unexpected success'),
        isDenied: (_) => false,
        onDenied: (_) async => fail('unexpected rejection'),
        onUnavailable: (_) => unavailable++,
      );
      for (final delay in [1500, 3000, 6000]) {
        final before = calls;
        clock.elapse(Duration(milliseconds: delay - 1));
        expect(calls, before);
        clock.elapse(const Duration(milliseconds: 1));
        expect(calls, before + 1);
      }
      expect(calls, 3);
      expect(unavailable, 3);
      expect(clock.pendingTimers, isEmpty);
      clock.elapse(const Duration(days: 1));
      expect(calls, 3);
    });
  });

  for (final denied in [false, true]) {
    test('背景${denied ? "拒絕" : "成功"}只完成一次', () {
      fakeAsync((clock) {
        final scheduler = RestoreScheduler();
        var calls = 0;
        final values = <String>[];
        final errors = <Object>[];
        final rejection = StateError('denied');
        scheduler.retry<String>(
          request: () async {
            calls++;
            if (denied) throw rejection;
            return 'account';
          },
          onSuccess: (value) async => values.add(value),
          isDenied: (error) => identical(error, rejection),
          onDenied: (error) async => errors.add(error),
          onUnavailable: (_) => fail('unexpected unavailable'),
        );
        clock.elapse(const Duration(days: 1));
        expect(calls, 1);
        expect(values, denied ? isEmpty : ['account']);
        expect(errors, denied ? [rejection] : isEmpty);
        expect(clock.pendingTimers, isEmpty);
      });
    });
  }

  for (final fails in [false, true]) {
    test('取消後忽略進行中請求的${fails ? "失敗" : "成功"}', () {
      fakeAsync((clock) {
        final scheduler = RestoreScheduler();
        final response = Completer<String>();
        var started = false;
        scheduler.retry<String>(
          request: () {
            started = true;
            return response.future;
          },
          onSuccess: (_) async => fail('stale success'),
          isDenied: (_) => true,
          onDenied: (_) async => fail('stale rejection'),
          onUnavailable: (_) => fail('stale unavailable'),
        );
        clock.elapse(const Duration(milliseconds: 1500));
        expect(started, isTrue);
        scheduler.cancel();
        if (fails) {
          response.completeError(StateError('denied'));
        } else {
          response.complete('stale');
        }
        clock.flushMicrotasks();
        expect(clock.pendingTimers, isEmpty);
      });
    });
  }

  test('重新排程只保留新工作，取消會移除 timer', () {
    fakeAsync((clock) {
      final scheduler = RestoreScheduler();
      final values = <String>[];
      void schedule(String value) => scheduler.retry<String>(
        request: () async => value,
        onSuccess: (value) async => values.add(value),
        isDenied: (_) => false,
        onDenied: (_) async => fail('unexpected rejection'),
        onUnavailable: (_) => fail('unexpected unavailable'),
      );
      final generation = scheduler.cancel();
      schedule('old');
      schedule('new');
      clock.elapse(const Duration(milliseconds: 1500));
      expect(values, ['new']);
      schedule('cancelled');
      scheduler.cancel();
      expect(scheduler.isCurrent(generation), isFalse);
      clock.elapse(const Duration(days: 1));
      expect(values, ['new']);
      expect(clock.pendingTimers, isEmpty);
    });
  });

  test('完成副作用失敗不重新執行 GetSession 或完成副作用', () {
    fakeAsync((clock) {
      final scheduler = RestoreScheduler();
      var calls = 0;
      var completions = 0;
      final errors = <Object>[];
      final failure = StateError('side effect');
      scheduler.retry<String>(
        request: () async {
          calls++;
          return 'account';
        },
        onSuccess: (_) async {
          completions++;
          throw failure;
        },
        isDenied: (_) => true,
        onDenied: (_) async => fail('unexpected rejection'),
        onUnavailable: errors.add,
      );
      clock.elapse(const Duration(days: 1));
      expect(calls, 1);
      expect(completions, 1);
      expect(errors, [failure]);
      expect(clock.pendingTimers, isEmpty);
    });
  });
}
