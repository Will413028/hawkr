import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/taipei_time.dart';
import 'package:hawkr_seller/l10n/gen/app_localizations.dart';
import 'package:hawkr_seller/shared/time/taipei_display.dart';
import '../../support/taipei_display.dart';

/// 2026-07-30 07:05 UTC = 臺北 15:05。
final _instant = DateTime.utc(2026, 7, 30, 7, 5);
final _timestamp = Timestamp.fromDateTime(_instant);

Future<void> _pump(
  WidgetTester tester, {
  required Duration? offset,
  Locale locale = const Locale('zh'),
}) async {
  Widget body = Builder(
    builder: (context) {
      final t = context.taipeiDisplay;
      return Column(
        children: [
          Text(t.timestamp(_timestamp), key: const Key('timestamp')),
          Text(t.dateTime(_instant), key: const Key('dateTime')),
          Text(t.date(_instant), key: const Key('date')),
          Text(t.labelIfNeeded ?? 'none', key: const Key('helper')),
        ],
      );
    },
  );
  if (offset != null) {
    body = TaipeiDisplayScope(deviceOffset: offset, child: body);
  }
  await tester.pumpWidget(
    MaterialApp(
      locale: locale,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: Scaffold(body: body),
    ),
  );
}

String _text(WidgetTester tester, String key) =>
    tester.widget<Text>(find.byKey(Key(key))).data!;

void main() {
  testWidgets('device offset not +8: every Taipei time carries the label', (
    tester,
  ) async {
    await _pump(tester, offset: const Duration(hours: -5));
    expect(_text(tester, 'timestamp'), '2026/07/30 15:05 (台北時間)');
    expect(_text(tester, 'dateTime'), '2026/07/30 15:05 (台北時間)');
    expect(_text(tester, 'date'), '2026/07/30 (台北時間)');
    expect(_text(tester, 'helper'), '台北時間');
  });

  testWidgets('device offset +8: no label', (tester) async {
    await _pump(tester, offset: const Duration(hours: 8));
    expect(_text(tester, 'timestamp'), '2026/07/30 15:05');
    expect(_text(tester, 'dateTime'), '2026/07/30 15:05');
    expect(_text(tester, 'date'), '2026/07/30');
    expect(_text(tester, 'helper'), 'none');
  });

  testWidgets('label follows the app locale', (tester) async {
    await _pump(tester, offset: Duration.zero, locale: const Locale('en'));
    expect(_text(tester, 'timestamp'), '2026/07/30 15:05 (Taipei time)');
  });

  testWidgets('without a scope the real device offset decides', (tester) async {
    await _pump(tester, offset: null);
    final expected = isDeviceInTaipeiOffset()
        ? '2026/07/30 15:05'
        : '2026/07/30 15:05 (台北時間)';
    expect(_text(tester, 'timestamp'), expected);
  });

  test('the three layouts: date, date and time, time of day (24-hour)', () {
    final afternoon = DateTime.utc(2026, 7, 30, 7, 5); // Taipei 15:05
    expect(
      taipeiDeviceDisplay.format(afternoon, TaipeiFormat.dateTime),
      '2026/07/30 15:05',
    );
    expect(
      taipeiDeviceDisplay.format(afternoon, TaipeiFormat.date),
      '2026/07/30',
    );
    expect(taipeiDeviceDisplay.format(afternoon, TaipeiFormat.time), '15:05');
    expect(TaipeiFormat.values, hasLength(3));
  });

  test('null timestamp stays empty without a label', () {
    expect(foreignDeviceDisplay.timestamp(null), '');
  });
}
