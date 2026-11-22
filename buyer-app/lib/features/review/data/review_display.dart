import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.pb.dart' as pb;
import 'package:hawkr_api/images.dart';

/// 作者沒有設定暱稱時顯示的名稱。
const anonymousReviewerName = '匿名會員';

/// 評價在畫面上的顯示值（只做取值與格式化，不產生文字句子）。
extension ReviewDisplay on pb.Review {
  /// 作者暱稱；沒有設定時是空字串，由畫面決定顯示什麼。
  String get authorName => hasAuthor() && author.hasName() ? author.name : '';

  /// 作者頭像；沒有時是 null。
  Image? get authorAvatar =>
      hasAuthor() && author.hasAvatar() ? author.avatar : null;

  /// 建立日期 `YYYY-MM-DD`（本地時區）；沒有時是空字串。
  String get createDateText {
    if (!hasCreateTime()) return '';
    final d = createTime.toDateTime().toLocal();
    return '${d.year}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';
  }
}
