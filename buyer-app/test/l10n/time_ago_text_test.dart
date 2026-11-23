import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/taipei_time.dart';
import 'package:hawkr_buyer/l10n/gen/app_localizations.dart';
import 'package:hawkr_buyer/l10n/time_ago_text.dart';

void main() {
  final zh = lookupAppLocalizations(const Locale('zh'));
  final en = lookupAppLocalizations(const Locale('en'));

  test('composes zh text from each unit', () {
    expect(timeAgoText(zh, const TimeAgoJustNow()), '剛剛');
    expect(timeAgoText(zh, const TimeAgoMinutes(5)), '5分鐘前');
    expect(timeAgoText(zh, const TimeAgoHours(3)), '3小時前');
    expect(timeAgoText(zh, const TimeAgoDays(2)), '2天前');
    expect(timeAgoText(zh, const TimeAgoMonthDay(7, 1)), '7/1');
    expect(timeAgoText(zh, const TimeAgoFullDate(2025, 7, 1)), '2025/7/1');
  });

  test('composes en text with plural forms', () {
    expect(timeAgoText(en, const TimeAgoMinutes(1)), '1 minute ago');
    expect(timeAgoText(en, const TimeAgoMinutes(5)), '5 minutes ago');
    expect(timeAgoText(en, const TimeAgoHours(1)), '1 hour ago');
    expect(timeAgoText(en, const TimeAgoDays(2)), '2 days ago');
  });

  test('units are not swapped: minutes, hours and days differ', () {
    final texts = {
      timeAgoText(zh, const TimeAgoMinutes(2)),
      timeAgoText(zh, const TimeAgoHours(2)),
      timeAgoText(zh, const TimeAgoDays(2)),
    };
    expect(texts.length, 3);
  });

  test('Taipei time label', () {
    expect(zh.taipeiTimeLabel, '台北時間');
    expect(en.taipeiTimeLabel, 'Taipei time');
  });

  test('history label: yesterday is decided by the Taipei calendar day', () {
    // 2026-07-31 00:10 Taipei.
    final now = DateTime.utc(2026, 7, 30, 16, 10);
    expect(
      historyViewedText(zh, DateTime.utc(2026, 7, 29, 16, 5), now: now),
      '昨天看過',
    );
    expect(
      historyViewedText(zh, DateTime.utc(2026, 7, 29, 15, 30), now: now),
      '2 天前看過',
    );
  });

  test('history label: older entries show days or Taipei date', () {
    final now = DateTime.utc(2026, 8, 5, 2);
    expect(
      historyViewedText(zh, DateTime.utc(2026, 8, 2, 2), now: now),
      '3 天前看過',
    );
    expect(
      historyViewedText(zh, DateTime.utc(2026, 7, 20, 2), now: now),
      '7/20 看過',
    );
    expect(
      historyViewedText(zh, DateTime.utc(2025, 7, 20, 2), now: now),
      '7/20 看過',
    );
    expect(
      historyViewedText(zh, now.subtract(const Duration(seconds: 5)), now: now),
      '剛剛看過',
    );
    expect(
      historyViewedText(zh, now.subtract(const Duration(minutes: 5)), now: now),
      '5 分鐘前看過',
    );
    expect(
      historyViewedText(zh, now.subtract(const Duration(hours: 5)), now: now),
      '5 小時前看過',
    );
  });
}
