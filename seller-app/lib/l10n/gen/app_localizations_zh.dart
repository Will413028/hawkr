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
  String get commonRetry => '重試';

  @override
  String get commonReload => '重新載入';

  @override
  String get commonLoadMore => '載入更多';

  @override
  String get chatLoadMoreFailedRetry => '載入更多失敗，點此重試';

  @override
  String get chatBuyerFallbackName => '顧客';

  @override
  String get chatSendFailedResend => '送出失敗，點一下重送';

  @override
  String get chatSending => '傳送中…';

  @override
  String get chatReadPrefix => '已讀 ';

  @override
  String get chatLoadFailedTitle => '目前無法載入訊息';

  @override
  String get chatLoadFailedDescription => '請確認網路連線後再試一次。';

  @override
  String get customerOrdersTitle => '顧客訂單';

  @override
  String get customerOrdersEmpty => '暫無訂單';

  @override
  String get customerBuyerFallbackName => '買家';

  @override
  String get couponPickFromCustomers => '從客戶名單選擇';

  @override
  String get couponPickFromCustomersHint => '從客戶名單挑選這次發放的會員';

  @override
  String get reportMessageTitle => '檢舉訊息';

  @override
  String get customerPickerTitle => '選擇客戶';

  @override
  String get customerPickerEmpty => '沒有可選的客戶';

  @override
  String customerPickerAdd(int count) {
    return '加入名單（$count）';
  }
}
