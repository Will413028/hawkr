import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/features/chat/data/latest_request.dart';

void main() {
  test('較早發出、較晚回來的結果被標為過時，不蓋掉較新的', () async {
    final latest = LatestRequest();
    final older = Completer<String>();
    final newer = Completer<String>();

    final first = latest.run(() => older.future);
    final second = latest.run(() => newer.future);
    newer.complete('new');
    older.complete('old');

    expect((await second).superseded, isFalse);
    expect((await second).value, 'new');
    expect((await first).superseded, isTrue);
  });

  test('過時的請求失敗被吞掉，最新的請求失敗照丟', () async {
    final latest = LatestRequest();
    final older = Completer<String>();

    final first = latest.run(() => older.future);
    final second = latest.run(() async => 'ok');
    older.completeError(StateError('old'));

    expect((await first).superseded, isTrue);
    expect((await second).value, 'ok');
    await expectLater(
      latest.run<String>(() async => throw StateError('latest')),
      throwsA(isA<StateError>()),
    );
  });
}
