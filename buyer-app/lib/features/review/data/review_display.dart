import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.pb.dart' as pb;
import 'package:hawkr_api/images.dart';
import 'package:hawkr_api/taipei_time.dart';
import 'package:hawkr_buyer/shared/time/taipei_display.dart';

/// 評價在畫面上的顯示值（只做取值與格式化，不產生文字句子）。
extension ReviewDisplay on pb.Review {
  /// 作者暱稱；沒有設定時是空字串，由畫面決定顯示什麼。
  String get authorName => hasAuthor() && author.hasName() ? author.name : '';

  /// 作者頭像；沒有時是 null。
  Image? get authorAvatar =>
      hasAuthor() && author.hasAvatar() ? author.avatar : null;

  /// 建立日期 `YYYY-MM-DD`（臺北日期，裝置偏移不是 +8 時附標籤）；沒有時是空字串。
  String createDateText(TaipeiDisplay t) {
    if (!hasCreateTime()) return '';
    return t.format(createTime.toDateTime(), TaipeiFormat.date);
  }
}
