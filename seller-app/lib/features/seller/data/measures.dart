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
import 'measure_display.dart';
import 'version_conflict.dart';

part 'measures.g.dart';

/// 一頁最多 100 筆（契約上限）；用 `next_page_token` 讀完全部分頁。
const measuresPageSize = 100;

/// 請求理由（申覆與請求解除相同）的字數上限與佐證圖片張數上限（契約 `SubmitAppealRequest`、`RequestRemediationRequest`）。
const requestReasonMaxLength = 2000;
const requestMaxEvidence = 5;

/// 請求佐證上傳的 docType：gateway 的 secure upload 不依 docType 分流，這個值只標記用途（同 website）。
const requestEvidenceDocType = 'appeal_evidence';

/// 衝突後仍可申覆時的提示。
const appealConflictNotice = '這項措施已更新，已載入最新內容，請確認後再送出申覆';
const remediationConflictNotice = '這項措施已更新，已載入最新內容，請確認後再送出請求';

/// 伺服器沒有給句子時，申覆與解除請求不能用的通用說明。
const appealUnavailableNotice = '這項措施目前不能提出申覆';
const remediationUnavailableNotice = '這項措施目前不能請求解除';

/// 措施上賣家能送出的請求種類：申覆（`SubmitAppeal`）與請求解除（`RequestRemediation`）。
/// 兩者的欄位（理由 1–2000 字、佐證 ≤5 張、etag）與衝突處理相同，差別只在呼叫的 RPC 與伺服器的 `can_*` 判斷。
enum MeasureRequestAction {
  appeal,
  remediation;

  /// 伺服器此刻允許送出這種請求嗎（`can_appeal`／`can_request_remediation`，不自己推導）。
  bool allowedOn(Measure measure) => switch (this) {
    appeal => measure.canAppeal,
    remediation => measure.canRequestRemediation,
  };
}

/// 全部狀態（`ListMyMeasures` 的 `state` 為 UNSPECIFIED 就是不篩）。
const allMeasureStates = MeasureState.MEASURE_STATE_UNSPECIFIED;

/// 請求（申覆或請求解除）的結果要寫回的列表：全部，以及停權公告頁讀的生效中、首頁預告卡讀的預定生效。
const adoptingMeasureStates = [
  MeasureState.MEASURE_STATE_UNSPECIFIED,
  MeasureState.MEASURE_STATE_ACTIVE,
  MeasureState.MEASURE_STATE_SCHEDULED,
];

/// 讀完 `ListMyMeasures` 的所有分頁（[state] 為 UNSPECIFIED 不篩）；`next_page_token` 回到曾經用過的值（含 A→A、A→B→A）時丟
/// [StateError]，不無限迴圈。
Future<List<Measure>> fetchMyMeasures(
  SellerAppServiceClient client, {
  MeasureState state = allMeasureStates,
}) async {
  final measures = <Measure>[];
  final seenTokens = <String>{};
  var pageToken = '';
  do {
    final response = await client.listMyMeasures(
      ListMyMeasuresRequest(
        pageSize: measuresPageSize,
        pageToken: pageToken,
        state: state,
      ),
    );
    measures.addAll(response.measures);
    final next = response.nextPageToken;
    if (next.isNotEmpty && !seenTokens.add(next)) {
      throw StateError('ListMyMeasures 的 next_page_token 重複出現：$next');
    }
    pageToken = next;
  } while (pageToken.isNotEmpty);
  return measures;
}

/// [accountId] 這個帳號 [filter] 狀態的措施（帳號層級，不需要店家；UNSPECIFIED 是全部）。以帳號與狀態為鍵，換帳號不會看到
/// 上一個帳號的列表（讀取中或失敗時也不會）；沒有畫面在看時釋放。讀取失敗是 error 狀態，不當成沒有措施。
@Riverpod(retry: noRetry)
class MyMeasures extends _$MyMeasures {
  @override
  Future<List<Measure>> build(String accountId, MeasureState filter) =>
      fetchMyMeasures(ref.watch(sellerClientProvider), state: filter);

  /// 把伺服器剛回的這一筆（請求的回應、衝突後的重讀）寫進已讀到的列表，以 id 取代原來的那一列；分組由列表內容重新算，
  /// 所以送出申覆後立刻離開「需要處理」。這份列表有狀態篩選、而最新的這一筆已經不是那個狀態時，把它從列表拿掉。沒有讀過列表、或列表裡
  /// 沒有這一筆時不動（下次讀列表時伺服器會給）。
  ///
  /// 不再整個重讀：這一列已經是伺服器剛回的最新內容，整個重讀只會多打一輪分頁請求，失敗時還會讓畫面停在過期的
  /// `can_appeal`／`can_request_remediation`。
  void adopt(Measure latest) {
    final current = state.value;
    if (current == null || !current.any((m) => m.id == latest.id)) return;
    final leaves = filter != allMeasureStates && latest.state != filter;
    state = AsyncData([
      for (final m in current)
        if (m.id != latest.id) m else if (!leaves) latest,
    ]);
  }
}

/// 目前登入帳號 [filter] 狀態的措施列表；沒有登入時是 null（和值是 null 的 `AsyncData` 不同）。
@riverpod
AsyncValue<List<Measure>>? currentMyMeasures(Ref ref, MeasureState filter) {
  final accountId = signedInAccountId(ref.watch(sessionProvider));
  if (accountId == null) return null;
  return ref.watch(myMeasuresProvider(accountId, filter));
}

/// [accountId] 有沒有任何措施紀錄：只問一筆（`ListMyMeasures` 的 `page_size` 1），不讀完整個列表。
/// 讀取失敗是 error 狀態，由呼叫端決定怎麼處理。
@Riverpod(retry: noRetry)
Future<bool> accountHasMeasures(Ref ref, String accountId) async {
  final response = await ref
      .watch(sellerClientProvider)
      .listMyMeasures(ListMyMeasuresRequest(pageSize: 1));
  return response.measures.isNotEmpty;
}

/// [accountId] 的 [measureId] 這一筆措施：已讀過全部列表而且列表裡有就用列表的，否則用 `GetMyMeasure` 讀（例如不經列表
/// 直接開啟）。只看列表是否已經存在，不會為了找一筆去觸發整個列表的讀取。
@Riverpod(retry: noRetry)
Future<Measure> measureById(Ref ref, String accountId, String measureId) async {
  final list = myMeasuresProvider(accountId, allMeasureStates);
  if (ref.exists(list)) {
    final cached = ref.read(list).value;
    final found = cached?.where((m) => m.id == measureId);
    if (found != null && found.isNotEmpty) return found.first;
  }
  final response = await ref
      .watch(sellerClientProvider)
      .getMyMeasure(GetMyMeasureRequest(measureId: measureId));
  return response.measure;
}

/// 這項措施不能（再）申覆時要告訴賣家的原因：伺服器依優先序排好的 `appeal_unavailabilities` 的第一筆句子（不依 `reason` 組字，也不從
/// `requests` 或 `state` 推導）；沒有句子時用通用說明。與 website 相同。
String appealClosedNotice(Measure measure) =>
    appealUnavailableMessage(measure) ?? appealUnavailableNotice;

/// 這項措施不能請求解除時要告訴賣家的原因：第一筆伺服器句子（沒有解除請求入口的 NOT_RISK、NOT_ACTIVE 也一樣），沒有句子時才用通用說明。
String remediationClosedNotice(Measure measure) =>
    remediationUnavailableMessage(measure) ?? remediationUnavailableNotice;

/// 一次送出請求（申覆或請求解除）的結果。
sealed class MeasureRequestResult {
  const MeasureRequestResult();
}

/// 請求已送出；[measure] 是伺服器回的最新一筆。
final class MeasureRequestSubmitted extends MeasureRequestResult {
  const MeasureRequestSubmitted(this.measure);

  final Measure measure;
}

/// 這項措施已經不能送出這種請求：關閉表單並顯示 [notice]；[latest] 是伺服器的最新一筆。
final class MeasureRequestClosed extends MeasureRequestResult {
  const MeasureRequestClosed(this.latest, this.notice);

  final Measure latest;
  final String notice;
}

/// 措施在送出之前被改過、但仍可送出這種請求：保留輸入，改用 [latest]（新的 etag）並顯示 [notice]。
final class MeasureRequestRefreshed extends MeasureRequestResult {
  const MeasureRequestRefreshed(this.latest, this.notice);

  final Measure latest;
  final String notice;
}

/// 伺服器指出請求理由欄位的錯誤。
final class MeasureRequestReasonInvalid extends MeasureRequestResult {
  const MeasureRequestReasonInvalid(this.message);

  final String message;
}

/// 伺服器指出佐證有問題：[byIndex] 的 key 是送出的 token 清單中的位置（`evidence_staged_tokens[i]`，某幾張的
/// staged token 過期或被拒），value 是伺服器的說明；[evidenceMessage] 是整個佐證欄位的錯誤（`evidence_staged_tokens`
/// 沒有索引，例如張數上限），不指向哪一張；[reasonMessage] 是同一次回應中理由欄位的錯誤（沒有時為 null）。
final class MeasureRequestEvidenceInvalid extends MeasureRequestResult {
  const MeasureRequestEvidenceInvalid(
    this.byIndex, {
    this.evidenceMessage,
    this.reasonMessage,
  });

  final Map<int, String> byIndex;
  final String? evidenceMessage;
  final String? reasonMessage;
}

/// 其他失敗：[message] 是要顯示給賣家的文字。
final class MeasureRequestFailed extends MeasureRequestResult {
  const MeasureRequestFailed(this.message);

  final String message;
}

/// 以讀到的措施的 etag 送出 [kind] 請求（trim 後的理由、已上傳的佐證 staged token）。
///
/// `VERSION_CONFLICT`（平台改過內容）與 `failed_precondition`（已申覆、已有待處理的請求或已撤銷；重送同一請求也是這個）都用
/// `GetMyMeasure` 重讀這一筆，由伺服器的 `can_appeal`（請求解除是 `can_request_remediation`）決定：不能送出就關閉並說明原因，仍可送出就保留輸入換新的 etag。
Future<MeasureRequestResult> submitMeasureRequest(
  SellerAppServiceClient client, {
  required MeasureRequestAction kind,
  required Measure measure,
  required String reason,
  required List<String> evidenceTokens,
  required DefaultErrorText defaults,
}) async {
  try {
    final trimmed = trimGoSpace(reason);
    final Measure latest;
    switch (kind) {
      case MeasureRequestAction.appeal:
        latest = (await client.submitAppeal(
          SubmitAppealRequest(
            measureId: measure.id,
            reason: trimmed,
            evidenceStagedTokens: evidenceTokens,
            etag: measure.etag,
          ),
        )).measure;
      case MeasureRequestAction.remediation:
        latest = (await client.requestRemediation(
          RequestRemediationRequest(
            measureId: measure.id,
            reason: trimmed,
            evidenceStagedTokens: evidenceTokens,
            etag: measure.etag,
          ),
        )).measure;
    }
    return MeasureRequestSubmitted(latest);
  } catch (error) {
    final failure = submitError(error, defaults: defaults);
    final failedPrecondition =
        ConnectException.from(error).code == Code.failedPrecondition;
    return switch (failure) {
      SubmitConflict() => _reloadAfterConflict(client, kind, measure.id),
      _ when failedPrecondition => _reloadAfterConflict(
        client,
        kind,
        measure.id,
      ),
      SubmitFields() => _fieldsResult(failure),
      SubmitMessage(:final message) => MeasureRequestFailed(message),
    };
  }
}

final _evidenceField = RegExp(r'^evidence_staged_tokens\[(\d+)\]');

MeasureRequestResult _fieldsResult(SubmitFields failure) {
  final onReason = fieldMessage(failure.fields, const ['reason']);
  final onEvidence = <int, String>{};
  // 整個欄位的錯誤只認沒有索引的路徑（fieldMessage 也會涵蓋 `[i]`，那是指向某一張的錯誤）。
  final onEvidenceField = failure.fields['evidence_staged_tokens'];
  for (final MapEntry(key: field, value: message) in failure.fields.entries) {
    final match = _evidenceField.firstMatch(field);
    if (match != null) onEvidence[int.parse(match.group(1)!)] = message;
  }
  if (onEvidence.isNotEmpty || onEvidenceField != null) {
    return MeasureRequestEvidenceInvalid(
      onEvidence,
      evidenceMessage: onEvidenceField,
      reasonMessage: onReason,
    );
  }
  if (onReason != null) return MeasureRequestReasonInvalid(onReason);
  return MeasureRequestFailed(failure.message ?? failure.fields.values.first);
}

Future<MeasureRequestResult> _reloadAfterConflict(
  SellerAppServiceClient client,
  MeasureRequestAction kind,
  String measureId,
) async {
  final Measure latest;
  try {
    final response = await client.getMyMeasure(
      GetMyMeasureRequest(measureId: measureId),
    );
    latest = response.measure;
  } catch (error) {
    HKLog.logW('[Measures] reload after conflict failed: $error');
    return const MeasureRequestFailed(versionReloadFailedNotice);
  }
  if (kind.allowedOn(latest)) {
    return MeasureRequestRefreshed(latest, switch (kind) {
      MeasureRequestAction.appeal => appealConflictNotice,
      MeasureRequestAction.remediation => remediationConflictNotice,
    });
  }
  return MeasureRequestClosed(latest, switch (kind) {
    MeasureRequestAction.appeal => appealClosedNotice(latest),
    MeasureRequestAction.remediation => remediationClosedNotice(latest),
  });
}
