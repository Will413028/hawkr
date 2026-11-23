import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/taipei_time.dart';
import 'package:hawkr_seller/l10n/gen/app_localizations.dart';
import 'package:hawkr_seller/l10n/time_ago_text.dart';

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
}
