import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/taipei_time.dart';

import 'inquiries.dart' show isInquiryOverdue;

/// 詢問畫面的顯示文字。契約只給列舉，這三個列舉只有賣家端用（不在共用文案目錄），名稱在這裡對照；
/// 能不能回覆讀 `can_reply`、逾期以 `reply_deadline.due_time` 判斷，都不從其他欄位推導。

enum InquiryTone { success, danger, warning, neutral }

const inquiriesTitle = '平台詢問';
const inquiriesLoadFailedTitle = '無法載入平台詢問';
const inquiriesLoadFailedDescription = '請檢查網路後再試一次。';
const inquiriesEmptyTitle = '目前沒有平台詢問';
const inquiriesEmptyDescription = '平台需要你提供資料或說明時，會顯示在這裡。';
const inquiryNotFoundTitle = '找不到這則詢問';
const inquiryNotFoundDescription = '這則詢問不存在，或不屬於目前的帳號。';
const inquiryNotFoundAction = '回到平台詢問';
const inquiryReplyTitle = '回覆詢問';

String? inquiryKindLabel(InquiryKind kind) => switch (kind) {
  InquiryKind.INQUIRY_KIND_RECORDS => '提供紀錄',
  InquiryKind.INQUIRY_KIND_EXPLANATION => '請說明',
  _ => null,
};

String? inquiryReplyChannelLabel(InquiryReplyChannel channel) =>
    switch (channel) {
      InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_PHONE => '電話',
      InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_EMAIL => '電子信箱',
      InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_MESSAGING => '通訊軟體',
      InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_OTHER => '其他管道',
      InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_SELLER_CENTER => '賣家中心',
      _ => null,
    };

/// 詢問狀態的標籤；逾期（OPEN 且過了期限）顯示「已逾期」。沒有有效值（UNSPECIFIED）時 null。
({String label, InquiryTone tone})? inquiryStateChip(
  Inquiry inquiry,
  DateTime now,
) {
  if (isInquiryOverdue(inquiry, now)) {
    return (label: '已逾期', tone: InquiryTone.danger);
  }
  return switch (inquiry.state) {
    InquiryState.INQUIRY_STATE_OPEN => (
      label: '待回覆',
      tone: InquiryTone.warning,
    ),
    InquiryState.INQUIRY_STATE_REPLIED => (
      label: '已回覆',
      tone: InquiryTone.success,
    ),
    InquiryState.INQUIRY_STATE_WITHDRAWN => (
      label: '已撤回',
      tone: InquiryTone.neutral,
    ),
    _ => null,
  };
}

/// 「回覆期限：2026/11/20 10:00」；沒有期限（不是待回覆）時 null。
String? inquiryDeadlineText(Inquiry inquiry) {
  if (!inquiry.hasReplyDeadline() || !inquiry.replyDeadline.hasDueTime()) {
    return null;
  }
  final text = formatTaipeiTimestamp(inquiry.replyDeadline.dueTime);
  return text.isEmpty ? null : '回覆期限：$text';
}

/// 「提供紀錄 · 2026/11/20 10:00」之類的副標：種類與建立時間。
String inquiryMeta(Inquiry inquiry) {
  final parts = [
    inquiryKindLabel(inquiry.kind),
    if (inquiry.hasCreateTime()) formatTaipeiTimestamp(inquiry.createTime),
  ].whereType<String>().where((part) => part.isNotEmpty);
  return parts.join(' · ');
}

/// 已回覆詢問的回覆摘要：管道與時間（管理員記下的系統外回覆與賣家中心回覆同形）。
String? inquiryReplyMeta(Inquiry inquiry) {
  if (!inquiry.hasReply()) return null;
  final reply = inquiry.reply;
  final parts = [
    inquiryReplyChannelLabel(reply.channel),
    if (reply.hasReceiveTime()) formatTaipeiTimestamp(reply.receiveTime),
  ].whereType<String>().where((part) => part.isNotEmpty);
  return parts.isEmpty ? null : parts.join(' · ');
}

/// 回覆附件只讀得到數量。
String? inquiryAttachmentText(Inquiry inquiry) {
  if (!inquiry.hasReply() || inquiry.reply.attachmentCount == 0) return null;
  return '附件 ${inquiry.reply.attachmentCount} 份';
}

/// 待回覆的一行說明（設定頁入口）。
String pendingInquiryDescription(int count) =>
    count > 0 ? '有 $count 則待回覆' : '查看平台對案件的詢問與你的回覆';
