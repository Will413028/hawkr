import 'package:hawkr_api/gen/hawkr/common/v1alpha1/deadline.pb.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart';

Timestamp _ts(DateTime t) => Timestamp.fromDateTime(t.toUtc());

Inquiry inquiryOf({
  String id = 'inq-1',
  InquiryKind kind = InquiryKind.INQUIRY_KIND_RECORDS,
  String message = '請提供進貨來源紀錄',
  InquiryState state = InquiryState.INQUIRY_STATE_OPEN,
  DateTime? created,
  DateTime? due,
  InquiryReply? reply,
  bool? canReply,
  int attachmentMax = 5,
}) => Inquiry(
  id: id,
  kind: kind,
  message: message,
  state: state,
  createTime: _ts(created ?? DateTime.utc(2026, 11, 20, 2)),
  replyDeadline: state == InquiryState.INQUIRY_STATE_OPEN
      ? Deadline(
          startTime: _ts(created ?? DateTime.utc(2026, 11, 20, 2)),
          dueTime: _ts(due ?? DateTime.utc(2026, 11, 27, 2)),
        )
      : null,
  reply: reply,
  canReply: canReply ?? state == InquiryState.INQUIRY_STATE_OPEN,
  replyAttachmentMax: attachmentMax,
);
