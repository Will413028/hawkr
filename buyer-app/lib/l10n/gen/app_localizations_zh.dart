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

  @override
  String get commonRetry => '重試';

  @override
  String get productReviewsTitle => '商品評價';

  @override
  String productReviewsTitleWithCount(int count) {
    return '商品評價 ($count)';
  }

  @override
  String get chatStoreFallbackName => '店家';

  @override
  String get paymentProcessing => '處理中...';

  @override
  String get paymentSucceededTitle => '付款成功';

  @override
  String get paymentNotCompletedTitle => '付款未完成';

  @override
  String get paymentSucceededMessage => '付款已完成，感謝你的購買。';

  @override
  String get paymentPendingMessage => '付款處理中，請稍後到「我的訂單」確認結果。';

  @override
  String get paymentCanceledMessage => '你已取消付款，訂單仍待付款，可到「我的訂單」重新付款。';

  @override
  String get paymentFailedMessage => '付款沒有完成，訂單仍待付款，可到「我的訂單」重新付款。';

  @override
  String get paymentAnomalyMessage => '這筆付款需要客服協助處理，暫時無法重新付款，請聯絡客服。';
}
