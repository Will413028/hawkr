import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_seller/features/seller/data/inquiry_display.dart';

import '../../../support/inquiry_fixtures.dart';

void main() {
  final due = DateTime.utc(2026, 11, 27, 2);

  test('狀態標籤：待回覆、已回覆、已撤回', () {
    final now = due.subtract(const Duration(days: 1));
    expect(inquiryStateChip(inquiryOf(due: due), now)?.label, '待回覆');
    expect(
      inquiryStateChip(
        inquiryOf(state: InquiryState.INQUIRY_STATE_REPLIED),
        now,
      )?.label,
      '已回覆',
    );
    expect(
      inquiryStateChip(
        inquiryOf(state: InquiryState.INQUIRY_STATE_WITHDRAWN),
        now,
      )?.label,
      '已撤回',
    );
  });

  test('逾期的待回覆顯示「已逾期」（危險色）', () {
    final chip = inquiryStateChip(
      inquiryOf(due: due),
      due.add(const Duration(hours: 1)),
    );
    expect(chip?.label, '已逾期');
    expect(chip?.tone, InquiryTone.danger);
  });

  test('UNSPECIFIED 沒有標籤與種類', () {
    final unspecified = Inquiry(state: InquiryState.INQUIRY_STATE_UNSPECIFIED);
    expect(inquiryStateChip(unspecified, due), isNull);
    expect(inquiryKindLabel(InquiryKind.INQUIRY_KIND_UNSPECIFIED), isNull);
  });

  test('期限文字是臺北時間；已回覆沒有期限', () {
    expect(inquiryDeadlineText(inquiryOf(due: due)), '回覆期限：2026/11/27 10:00');
    expect(
      inquiryDeadlineText(inquiryOf(state: InquiryState.INQUIRY_STATE_REPLIED)),
      isNull,
    );
  });

  test('回覆摘要：管道與時間；附件只有數量', () {
    final inquiry = inquiryOf(
      state: InquiryState.INQUIRY_STATE_REPLIED,
      reply: InquiryReply(
        body: 'b',
        channel: InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_PHONE,
        attachmentCount: 2,
      ),
    );
    expect(inquiryReplyMeta(inquiry), '電話');
    expect(inquiryAttachmentText(inquiry), '附件 2 份');
    expect(inquiryAttachmentText(inquiryOf()), isNull);
  });

  test('每個有效的回覆管道都有名稱', () {
    for (final channel in InquiryReplyChannel.values) {
      final label = inquiryReplyChannelLabel(channel);
      expect(
        label == null,
        channel == InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_UNSPECIFIED,
        reason: channel.name,
      );
    }
  });

  test('待回覆說明', () {
    expect(pendingInquiryDescription(3), '有 3 則待回覆');
    expect(pendingInquiryDescription(0), isNot(contains('則待回覆')));
  });
}
