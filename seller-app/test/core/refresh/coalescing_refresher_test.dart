import 'dart:async';

import 'package:hawkr_seller/core/refresh/coalescing_refresher.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  late DateTime now;
  late List<Completer<bool>> rounds;
  late List<bool> settledOnlyArgs;
  late CoalescingRefresher refresher;

  setUp(() {
    now = DateTime.utc(2026, 10, 12, 12);
    rounds = [];
    settledOnlyArgs = [];
    refresher = CoalescingRefresher(
      clock: () => now,
      round: (settledOnly) {
        settledOnlyArgs.add(settledOnly);
        final round = Completer<bool>();
        rounds.add(round);
        return round.future;
      },
    );
  });

  test('沒有進行中的一輪就開始一輪，進行中時共用，不另開', () async {
    final first = refresher.refresh();
    final second = refresher.refresh();
    expect(rounds, hasLength(1));
    expect(refresher.isRunning, isTrue);

    rounds.single.complete(true);
    expect(await first, isTrue);
    expect(await second, isTrue);
    expect(refresher.isRunning, isFalse);

    unawaited(refresher.refresh());
    expect(rounds, hasLength(2));
  });

  test('fresh：進行中時排一輪接在後面，進行期間所有 fresh 共用，且後續那輪開始時沒有空檔', () async {
    final base = refresher.refresh();
    final freshA = refresher.refresh(fresh: true);
    final freshB = refresher.refresh(fresh: true);
    expect(rounds, hasLength(1));

    rounds[0].complete(true);
    expect(await base, isTrue);
    // 前一輪結束的同一刻，後續一輪已經開始；中間插進來的非 fresh 呼叫共用它。
    await Future<void>.delayed(Duration.zero);
    expect(rounds, hasLength(2));
    expect(refresher.isRunning, isTrue);
    final joined = refresher.refresh();

    rounds[1].complete(false);
    expect(await freshA, isFalse);
    expect(await freshB, isFalse);
    expect(await joined, isFalse);
    expect(rounds, hasLength(2));
    expect(refresher.isRunning, isFalse);
  });

  test('沒有進行中的一輪時 fresh 直接開始一輪（不多排一輪）', () async {
    unawaited(refresher.refresh(fresh: true));
    expect(rounds, hasLength(1));
    rounds[0].complete(true);
    await Future<void>.delayed(Duration.zero);
    expect(rounds, hasLength(1));
  });

  test('settledOnly 只傳給直接開始的那一輪，後續那輪是 false', () async {
    unawaited(refresher.refresh(settledOnly: true));
    unawaited(refresher.refresh(fresh: true));
    rounds[0].complete(true);
    await Future<void>.delayed(Duration.zero);
    expect(settledOnlyArgs, [true, false]);
  });

  test('後續那輪只有等它的 fresh 呼叫全部要求 settledOnly 才縮小，有一個要完整重讀就完整重讀', () async {
    // 全部 settledOnly：縮小。
    unawaited(refresher.refresh());
    unawaited(refresher.refresh(fresh: true, settledOnly: true));
    unawaited(refresher.refresh(fresh: true, settledOnly: true));
    rounds[0].complete(true);
    await Future<void>.delayed(Duration.zero);
    expect(settledOnlyArgs, [false, true]);
    rounds[1].complete(true);
    await Future<void>.delayed(Duration.zero);

    // 先要求 settledOnly、後來一個完整的：完整重讀。
    settledOnlyArgs.clear();
    rounds.clear();
    unawaited(refresher.refresh());
    unawaited(refresher.refresh(fresh: true, settledOnly: true));
    unawaited(refresher.refresh(fresh: true));
    rounds[0].complete(true);
    await Future<void>.delayed(Duration.zero);
    expect(settledOnlyArgs, [false, false]);
    rounds[1].complete(true);
    await Future<void>.delayed(Duration.zero);

    // 上一個後續輪的設定不會帶到下一次。
    settledOnlyArgs.clear();
    rounds.clear();
    unawaited(refresher.refresh());
    unawaited(refresher.refresh(fresh: true, settledOnly: true));
    rounds[0].complete(true);
    await Future<void>.delayed(Duration.zero);
    expect(settledOnlyArgs, [false, true]);
  });

  test('maxAge：30 秒內成功過就不讀，剛好到期或沒成功過才讀；進行中的一輪照樣共用', () async {
    const maxAge = Duration(seconds: 30);
    expect(refresher.isStale(maxAge), isTrue);

    final first = refresher.refresh(maxAge: maxAge);
    expect(rounds, hasLength(1));
    refresher.markSucceeded();
    // 進行中：maxAge 不影響共用。
    final joined = refresher.refresh(maxAge: maxAge);
    expect(rounds, hasLength(1));
    rounds[0].complete(true);
    await first;
    await joined;

    now = now.add(const Duration(seconds: 29));
    expect(await refresher.refresh(maxAge: maxAge), isTrue);
    expect(rounds, hasLength(1));

    now = now.add(const Duration(seconds: 1));
    expect(refresher.isStale(maxAge), isTrue);
    unawaited(refresher.refresh(maxAge: maxAge));
    expect(rounds, hasLength(2));
  });

  test('沒有 maxAge 時一定讀；失敗的一輪沒有 markSucceeded 就不算新鮮', () async {
    final failed = refresher.refresh();
    rounds[0].complete(false);
    expect(await failed, isFalse);
    unawaited(refresher.refresh(maxAge: const Duration(seconds: 30)));
    expect(rounds, hasLength(2));
  });

  test('一輪丟例外：呼叫端收到例外，之後仍能開新的一輪', () async {
    final failing = refresher.refresh();
    final error = expectLater(failing, throwsA(isA<StateError>()));
    rounds[0].completeError(StateError('boom'));
    await error;
    await Future<void>.delayed(Duration.zero);
    expect(refresher.isRunning, isFalse);
    unawaited(refresher.refresh());
    expect(rounds, hasLength(2));
  });

  test('adopt：收編別處開始的一輪，之後的呼叫共用它，結束後才能再開', () async {
    final external = Completer<bool>();
    refresher.adopt(external.future);
    expect(refresher.isRunning, isTrue);

    final joined = refresher.refresh(maxAge: const Duration(seconds: 30));
    expect(rounds, isEmpty);
    external.complete(true);
    expect(await joined, isTrue);
    await Future<void>.delayed(Duration.zero);
    expect(refresher.isRunning, isFalse);
  });

  test('adopt 之後的 fresh 在收編的那輪結束後接一輪', () async {
    final external = Completer<bool>();
    refresher.adopt(external.future);
    final fresh = refresher.refresh(fresh: true);
    external.complete(true);
    await Future<void>.delayed(Duration.zero);
    expect(rounds, hasLength(1));
    rounds[0].complete(true);
    expect(await fresh, isTrue);
  });

  test('round 同步丟出：這一輪以該錯誤結束，不留下進行中；之後的呼叫照常開始新的一輪', () async {
    var throwing = true;
    final calls = <Completer<bool>>[];
    final syncRefresher = CoalescingRefresher(
      clock: () => now,
      round: (_) {
        if (throwing) throw StateError('sync');
        final round = Completer<bool>();
        calls.add(round);
        return round.future;
      },
    );

    await expectLater(syncRefresher.refresh(), throwsA(isA<StateError>()));
    await Future<void>.delayed(Duration.zero);
    expect(syncRefresher.isRunning, isFalse);

    throwing = false;
    final next = syncRefresher.refresh();
    expect(calls, hasLength(1));
    calls.single.complete(true);
    expect(await next, isTrue);
  });
}
