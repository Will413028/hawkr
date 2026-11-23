// ignore: unused_import
import 'package:intl/intl.dart' as intl;
import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for Chinese (`zh`).
class AppLocalizationsZh extends AppLocalizations {
  AppLocalizationsZh([String locale = 'zh']) : super(locale);

  @override
  String get languageName => '繁體中文';

  @override
  String get timeAgoJustNow => '剛剛';

  @override
  String timeAgoMinutes(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count分鐘前',
    );
    return '$_temp0';
  }

  @override
  String timeAgoHours(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count小時前',
    );
    return '$_temp0';
  }

  @override
  String timeAgoDays(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count天前',
    );
    return '$_temp0';
  }

  @override
  String timeAgoMonthDay(int month, int day) {
    return '$month/$day';
  }

  @override
  String timeAgoFullDate(int year, int month, int day) {
    return '$year/$month/$day';
  }

  @override
  String get taipeiTimeLabel => '台北時間';

  @override
  String get taipeiTimeTableNote => '時間為台北時間';

  @override
  String get viewedJustNow => '剛剛看過';

  @override
  String viewedMinutes(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count 分鐘前看過',
    );
    return '$_temp0';
  }

  @override
  String viewedHours(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count 小時前看過',
    );
    return '$_temp0';
  }

  @override
  String get viewedYesterday => '昨天看過';

  @override
  String viewedDays(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count 天前看過',
    );
    return '$_temp0';
  }

  @override
  String viewedDate(int month, int day) {
    return '$month/$day 看過';
  }

  @override
  String get viewedRecently => '最近看過';
}
