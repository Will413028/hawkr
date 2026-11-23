import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.pb.dart'
    as pb;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart' as spb;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_seller/features/report/data/moderation_repository.dart';
import 'package:hawkr_seller/features/report/data/store_block_repository.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

void main() {
  test(
    'chat message report carries kind CHAT_MESSAGE, the message id and request_id',
    () async {
      final requests = <pb.SubmitReportRequest>[];
      final repo = ModerationRepository(
        ModerationAppServiceClient(
          FakeTransportBuilder().unary(ModerationAppService.submitReport, (
            r,
            _,
          ) {
            requests.add(r);
            return pb.SubmitReportResponse(reportId: 'rep1');
          }).build(),
        ),
      );
      final id = await repo.reportChatMessage(
        messageId: 'm1',
        category: PolicyCategory.POLICY_CATEGORY_SPAM,
        description: 'd',
        requestId: 'q1',
      );
      expect(id, 'rep1');
      final r = requests.single;
      expect(
        r.target.kind,
        pb.ReportTargetKind.REPORT_TARGET_KIND_CHAT_MESSAGE,
      );
      expect(r.target.id, 'm1');
      expect(r.category, PolicyCategory.POLICY_CATEGORY_SPAM);
      expect(r.requestId, 'q1');
    },
  );

  test('block by message and by review send the reference', () async {
    final requests = <spb.BlockBuyerRequest>[];
    final repo = StoreBlockRepository(
      SellerAppServiceClient(
        FakeTransportBuilder().unary(SellerAppService.blockBuyer, (r, _) {
          requests.add(r);
          return spb.BlockBuyerResponse(changed: true);
        }).build(),
      ),
    );
    await repo.blockByMessage('m1');
    await repo.blockByReview('rv1');
    expect(requests[0].whichSource(), spb.BlockBuyerRequest_Source.messageId);
    expect(requests[0].messageId, 'm1');
    expect(requests[1].whichSource(), spb.BlockBuyerRequest_Source.reviewId);
    expect(requests[1].reviewId, 'rv1');
  });

  test('accepted report categories exclude UNSPECIFIED and have text', () {
    expect(
      moderationSubmitReportRequestCategory,
      isNot(contains(PolicyCategory.POLICY_CATEGORY_UNSPECIFIED)),
    );
    for (final c in moderationSubmitReportRequestCategory) {
      expect(c.uiText(UiLocale.zhTW), isNotEmpty);
    }
  });
}
