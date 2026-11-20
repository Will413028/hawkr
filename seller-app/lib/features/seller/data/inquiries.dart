import 'package:hawkr_api/errors.dart' show DefaultErrorText;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/go_text.dart';
import 'package:hawkr_api/submit.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';
import '../../../core/session/session_provider.dart';
import '../../../shared/utils/hk_log.dart';
import 'account_scope.dart';

part 'inquiries.g.dart';

/// 一頁最多 100 筆（契約上限）；用 `next_page_token` 讀完全部分頁。
const inquiriesPageSize = 100;

/// 回覆內容的字數上限（契約 `ReplyToInquiryRequest.body` 1–4000 字）。
const inquiryReplyMaxLength = 4000;

/// 回覆附件上傳的 docType：gateway 的 secure upload 不依 docType 分流，這個值只標記用途。
const inquiryEvidenceDocType = 'inquiry_evidence';

/// 全部狀態（`ListMyInquiries` 的 `state` 為 UNSPECIFIED 就是不篩）。
const allInquiryStates = InquiryState.INQUIRY_STATE_UNSPECIFIED;

/// 讀完 `ListMyInquiries` 的所有分頁；`next_page_token` 回到曾經用過的值時丟 [StateError]，不無限迴圈。
Future<List<Inquiry>> fetchMyInquiries(
  SellerAppServiceClient client, {
  InquiryState state = allInquiryStates,
}) async {
  final inquiries = <Inquiry>[];
  final seenTokens = <String>{};
  var pageToken = '';
  do {
    final response = await client.listMyInquiries(
      ListMyInquiriesRequest(
        pageSize: inquiriesPageSize,
        pageToken: pageToken,
        state: state,
      ),
    );
    inquiries.addAll(response.inquiries);
    final next = response.nextPageToken;
    if (next.isNotEmpty && !seenTokens.add(next)) {
      throw StateError('ListMyInquiries 的 next_page_token 重複出現：$next');
    }
    pageToken = next;
  } while (pageToken.isNotEmpty);
  return inquiries;
}

/// [accountId] 這個帳號的詢問（帳號層級，店家關閉後也讀得到）。以帳號為鍵，換帳號不會看到上一個帳號的列表；
/// 讀取失敗是 error 狀態，不當成沒有詢問。
@Riverpod(retry: noRetry)
class MyInquiries extends _$MyInquiries {
  @override
  Future<List<Inquiry>> build(String accountId) =>
      fetchMyInquiries(ref.watch(sellerClientProvider));

  /// 把伺服器剛回的這一筆（回覆的回應、重讀）以 id 取代已讀到的列表中的那一列；沒有讀過列表或列表裡沒有這一筆時不動。
  void adopt(Inquiry latest) {
    final current = state.value;
    if (current == null || !current.any((i) => i.id == latest.id)) return;
    state = AsyncData([
      for (final i in current)
        if (i.id == latest.id) latest else i,
    ]);
  }
}

/// 目前登入帳號的詢問列表；沒有登入時是 null。
@riverpod
AsyncValue<List<Inquiry>>? currentMyInquiries(Ref ref) {
  final accountId = signedInAccountId(ref.watch(sessionProvider));
  if (accountId == null) return null;
  return ref.watch(myInquiriesProvider(accountId));
}

/// [accountId] 待回覆（OPEN，逾期仍算）的詢問數，伺服器算好的 `pending_reply_count`；只問一筆，不讀完整個列表。
@Riverpod(retry: noRetry)
Future<int> pendingInquiryCount(Ref ref, String accountId) async {
  final response = await ref
      .watch(sellerClientProvider)
      .listMyInquiries(ListMyInquiriesRequest(pageSize: 1));
  return response.pendingReplyCount;
}

/// 目前登入帳號的待回覆數；沒有登入、讀取中或失敗時是 0（徽章只是提示，失敗不阻擋入口）。
@riverpod
int currentPendingInquiryCount(Ref ref) {
  final accountId = signedInAccountId(ref.watch(sessionProvider));
  if (accountId == null) return 0;
  return ref.watch(pendingInquiryCountProvider(accountId)).value ?? 0;
}

/// [accountId] 的 [inquiryId] 這一則詢問：已讀過列表而且列表裡有就用列表的，否則 `GetMyInquiry`。
@Riverpod(retry: noRetry)
Future<Inquiry> inquiryById(Ref ref, String accountId, String inquiryId) async {
  final list = myInquiriesProvider(accountId);
  if (ref.exists(list)) {
    final found = ref.read(list).value?.where((i) => i.id == inquiryId);
    if (found != null && found.isNotEmpty) return found.first;
  }
  final response = await ref
      .watch(sellerClientProvider)
      .getMyInquiry(GetMyInquiryRequest(inquiryId: inquiryId));
  return response.inquiry;
}

/// 逾期不是狀態：OPEN 而且 [now] 晚於 `reply_deadline.due_time`。
bool isInquiryOverdue(Inquiry inquiry, DateTime now) {
  if (inquiry.state != InquiryState.INQUIRY_STATE_OPEN) return false;
  if (!inquiry.hasReplyDeadline() || !inquiry.replyDeadline.hasDueTime()) {
    return false;
  }
  return now.isAfter(inquiry.replyDeadline.dueTime.toDateTime());
}

/// 一次回覆的結果。
sealed class InquiryReplyResult {
  const InquiryReplyResult();
}

/// 回覆已送出；[inquiry] 是伺服器回的最新一筆。
final class InquiryReplySubmitted extends InquiryReplyResult {
  const InquiryReplySubmitted(this.inquiry);

  final Inquiry inquiry;
}

/// 詢問已不能回覆（已回覆、已撤回；`failed_precondition`）：[latest] 是重讀到的最新內容（重讀失敗時 null），[message] 是伺服器的說明。
final class InquiryReplyClosed extends InquiryReplyResult {
  const InquiryReplyClosed(this.latest, this.message);

  final Inquiry? latest;
  final String message;
}

/// 詢問不存在或不屬於這個帳號（`not_found`）。
final class InquiryReplyNotFound extends InquiryReplyResult {
  const InquiryReplyNotFound(this.message);

  final String message;
}

/// 內容欄位被拒：[message] 顯示在回覆欄位。
final class InquiryReplyBodyInvalid extends InquiryReplyResult {
  const InquiryReplyBodyInvalid(this.message);

  final String message;
}

/// 附件欄位被拒：[byIndex] 是指到某一張的錯誤（索引同送出的 token 清單），[evidenceMessage] 是整個欄位的錯誤。
final class InquiryReplyEvidenceInvalid extends InquiryReplyResult {
  const InquiryReplyEvidenceInvalid(
    this.byIndex, {
    this.evidenceMessage,
    this.bodyMessage,
  });

  final Map<int, String> byIndex;
  final String? evidenceMessage;
  final String? bodyMessage;
}

/// 其他失敗：[message] 是要顯示給賣家的文字。
final class InquiryReplyFailed extends InquiryReplyResult {
  const InquiryReplyFailed(this.message);

  final String message;
}

/// 送出 [inquiry] 的回覆（trim 後的 [body]、已上傳的附件 staged token）。
Future<InquiryReplyResult> submitInquiryReply(
  SellerAppServiceClient client, {
  required Inquiry inquiry,
  required String body,
  required List<String> evidenceTokens,
  required DefaultErrorText defaults,
}) async {
  try {
    final response = await client.replyToInquiry(
      ReplyToInquiryRequest(
        inquiryId: inquiry.id,
        body: trimGoSpace(body),
        evidenceStagedTokens: evidenceTokens,
      ),
    );
    return InquiryReplySubmitted(response.inquiry);
  } catch (error) {
    final failure = submitError(error, defaults: defaults);
    final code = ConnectException.from(error).code;
    if (code == Code.notFound) {
      return InquiryReplyNotFound(_messageOf(failure, defaults));
    }
    if (code == Code.failedPrecondition) {
      return _reloadAfterClosed(
        client,
        inquiry.id,
        _messageOf(failure, defaults),
      );
    }
    return switch (failure) {
      SubmitFields() => _fieldsResult(failure),
      SubmitConflict() => InquiryReplyFailed(defaults(Code.aborted)),
      SubmitMessage(:final message) => InquiryReplyFailed(message),
    };
  }
}

String _messageOf(SubmitError failure, DefaultErrorText defaults) =>
    switch (failure) {
      SubmitMessage(:final message) => message,
      SubmitFields(:final message, :final fields) =>
        message ?? fields.values.first,
      SubmitConflict() => defaults(Code.aborted),
    };

final _evidenceField = RegExp(r'^evidence_staged_tokens\[(\d+)\]');

InquiryReplyResult _fieldsResult(SubmitFields failure) {
  final onBody = fieldMessage(failure.fields, const ['body']);
  final onEvidence = <int, String>{};
  final onEvidenceField = failure.fields['evidence_staged_tokens'];
  for (final MapEntry(key: field, value: message) in failure.fields.entries) {
    final match = _evidenceField.firstMatch(field);
    if (match != null) onEvidence[int.parse(match.group(1)!)] = message;
  }
  if (onEvidence.isNotEmpty || onEvidenceField != null) {
    return InquiryReplyEvidenceInvalid(
      onEvidence,
      evidenceMessage: onEvidenceField,
      bodyMessage: onBody,
    );
  }
  if (onBody != null) return InquiryReplyBodyInvalid(onBody);
  return InquiryReplyFailed(failure.message ?? failure.fields.values.first);
}

Future<InquiryReplyResult> _reloadAfterClosed(
  SellerAppServiceClient client,
  String inquiryId,
  String message,
) async {
  try {
    final response = await client.getMyInquiry(
      GetMyInquiryRequest(inquiryId: inquiryId),
    );
    return InquiryReplyClosed(response.inquiry, message);
  } catch (error) {
    HKLog.logW('[Inquiries] reload after closed failed: $error');
    return InquiryReplyClosed(null, message);
  }
}
