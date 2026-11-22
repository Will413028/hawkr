import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/features/moderation/data/report_target_type.dart';

void main() {
  group('ReportTargetType.value 對齊後端合法值', () {
    test('六種 target_type 字面值正確', () {
      expect(ReportTargetType.review.value, 'REVIEW');
      expect(ReportTargetType.post.value, 'POST');
      expect(ReportTargetType.postComment.value, 'POST_COMMENT');
      expect(ReportTargetType.sellerForum.value, 'SELLER_FORUM');
      expect(ReportTargetType.chatMessage.value, 'CHAT_MESSAGE');
      expect(ReportTargetType.user.value, 'USER');
    });
  });

  group('ReportReasonCode.code 對齊後端合法值', () {
    test('八種 reason_code 字面值正確', () {
      expect(ReportReasonCode.spam.code, 'SPAM');
      expect(ReportReasonCode.sexual.code, 'SEXUAL');
      expect(ReportReasonCode.fraud.code, 'FRAUD');
      expect(ReportReasonCode.harassment.code, 'HARASSMENT');
      expect(ReportReasonCode.foodSafety.code, 'FOOD_SAFETY');
      expect(ReportReasonCode.political.code, 'POLITICAL');
      expect(ReportReasonCode.fakeNews.code, 'FAKE_NEWS');
      expect(ReportReasonCode.other.code, 'OTHER');
    });
  });

  group('reportReasonsFor', () {
    test('USER 不含 FOOD_SAFETY', () {
      final reasons = reportReasonsFor(ReportTargetType.user);
      expect(reasons.contains(ReportReasonCode.foodSafety), isFalse);
      expect(reasons.length, ReportReasonCode.values.length - 1);
    });

    test('POST 含全部 8 種原因', () {
      final reasons = reportReasonsFor(ReportTargetType.post);
      expect(reasons.length, ReportReasonCode.values.length);
    });

    test('REVIEW 含全部 8 種原因', () {
      final reasons = reportReasonsFor(ReportTargetType.review);
      expect(reasons.length, ReportReasonCode.values.length);
    });
  });
}
