import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/taipei_time.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

import 'inquiries.dart' show isInquiryOverdue;

/// 詢問畫面的顯示文字。契約只給列舉，種類、狀態、回覆管道的名稱取自共用文案目錄（`hawkr_ui_text`），
/// 純函式的 `l` 是 UI 語系，沒有文字的值（UNSPECIFIED、不認得）回 null；
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

/// 詢問狀態的標籤；逾期（OPEN 且過了期限）顯示「已逾期」。沒有有效值（UNSPECIFIED）時 null。
({String label, InquiryTone tone})? inquiryStateChip(
  Inquiry inquiry,
  DateTime now,
  UiLocale l,
) {
  if (isInquiryOverdue(inquiry, now)) {
    return (label: '已逾期', tone: InquiryTone.danger);
  }
  final label = inquiry.state.uiText(l);
  if (label == null) return null;
  return switch (inquiry.state) {
    InquiryState.INQUIRY_STATE_OPEN => (
      label: label,
      tone: InquiryTone.warning,
    ),
    InquiryState.INQUIRY_STATE_REPLIED => (
      label: label,
      tone: InquiryTone.success,
    ),
    InquiryState.INQUIRY_STATE_WITHDRAWN => (
      label: label,
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
String inquiryMeta(Inquiry inquiry, UiLocale l) {
  final parts = [
    inquiry.kind.uiText(l),
    if (inquiry.hasCreateTime()) formatTaipeiTimestamp(inquiry.createTime),
  ].whereType<String>().where((part) => part.isNotEmpty);
  return parts.join(' · ');
}

/// 已回覆詢問的回覆摘要：管道與時間（管理員記下的系統外回覆與賣家中心回覆同形）。
String? inquiryReplyMeta(Inquiry inquiry, UiLocale l) {
  if (!inquiry.hasReply()) return null;
  final reply = inquiry.reply;
  final parts = [
    reply.channel.uiText(l),
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
