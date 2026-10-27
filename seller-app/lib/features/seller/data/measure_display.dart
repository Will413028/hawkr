import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart'
    show LocalizedMessage;
import 'package:hawkr_api/taipei_time.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

/// 措施列表的顯示文字與分組。
///
/// 契約只給列舉（CodeMeta 已凍結，ADR `backend/docs/adr/2026-09-25-codemeta-retirement.md`），名詞與固定句型取自共用文案目錄
/// （`hawkr_ui_text`），純函式的 `l` 是 UI 語系；`UNSPECIFIED` 與不認得的值（目錄回「未知」）不是有效值，回 null，由呼叫端
/// 決定不顯示或用通用文字。能不能申覆讀 `can_appeal`、措施
/// 此刻的狀態讀 `state`（都由伺服器判斷），這裡不自己從時間或其他欄位推導。

const measuresLoadFailedTitle = '無法載入措施紀錄';
const measuresLoadFailedDescription = '請檢查網路後再試一次。';
const measureNotFoundTitle = '找不到這項措施';
const measureNotFoundDescription = '這項措施不存在，或不屬於目前的帳號。';
const measureNotFoundAction = '回到措施與申覆';

/// 不能做某個動作的原因給使用者看的一句話，是伺服器組好的 `LocalizedMessage.message`（與動作被拒絕時錯誤的文字同一份）；
/// 沒有訊息或是空字串時是 null，由呼叫端用該動作的通用說明。`reason` 只用來分支，不在 client 對照成文字。
String? _unavailableText(bool hasMessage, LocalizedMessage message) =>
    hasMessage && message.message.isNotEmpty ? message.message : null;

/// 不能申覆的第一筆原因的句子（`appeal_unavailabilities` 依優先序排好）；沒有原因或沒有句子時是 null。
String? appealUnavailableMessage(Measure measure) {
  if (measure.appealUnavailabilities.isEmpty) return null;
  final first = measure.appealUnavailabilities.first;
  return _unavailableText(first.hasMessage(), first.message);
}

/// 不能請求解除的第一筆原因的句子（`remediation_unavailabilities` 依優先序排好）；沒有原因或沒有句子時是 null。
String? remediationUnavailableMessage(Measure measure) {
  if (measure.remediationUnavailabilities.isEmpty) return null;
  final first = measure.remediationUnavailabilities.first;
  return _unavailableText(first.hasMessage(), first.message);
}

/// 標題：依據的主要認定（契約：第一筆是主要依據）的類別，或「風險：{風險依據}」；沒有有效值時是通用的「措施紀錄」。
/// 與 website 相同。
String measureTitle(Measure measure, UiLocale l) {
  switch (measure.basis) {
    case MeasureBasis.MEASURE_BASIS_VIOLATION:
      final lead = measure.findings.isEmpty
          ? null
          : measure.findings.first.category.uiText(l);
      return lead ?? sellerMeasureTitleFallback(l);
    case MeasureBasis.MEASURE_BASIS_RISK:
      return sellerMeasureTitleRisk(l, ground: measure.riskGround) ??
          sellerMeasureTitleRiskFallback(l);
    case MeasureBasis.MEASURE_BASIS_CONTRACT:
      return measure.basis.uiText(l) ?? sellerMeasureTitleFallback(l);
    default:
      return sellerMeasureTitleFallback(l);
  }
}

/// 「嚴重程度：高 · 措施：停權」（嚴重程度取第一筆，也就是主要依據的認定；風險依據沒有），終止合作加「 · 事由：預告終止（第 21.2 條）」；
/// 都沒有有效值時是 null。
String? measureMeta(Measure measure, UiLocale l) {
  final severity = measure.findings.isEmpty
      ? null
      : sellerMeasureMetaSeverity(l, severity: measure.findings.first.severity);
  final kind = sellerMeasureMetaKind(l, kind: measure.kind);
  final ground = measure.kind == MeasureKind.MEASURE_KIND_TERMINATION
      ? sellerMeasureMetaTerminationGround(l, ground: measure.terminationGround)
      : null;
  final parts = [severity, kind, ground].whereType<String>();
  return parts.isEmpty ? null : parts.join(' · ');
}

enum MeasureChipTone { success, danger, warning, neutral }

/// 措施狀態的 chip：預定生效（`SCHEDULED`，目前只有預告終止）與生效中的終止合作（目錄的 `MeasureState.ACTIVE` 依 `kind` 選
/// 「合作已終止」，生效後 `state` 也是 `ACTIVE`，「停權中」對終止不正確）需要標示；其餘狀態由 [measureEffect] 的文字表達。
({String label, MeasureChipTone tone})? stateChip(Measure measure, UiLocale l) {
  if (measure.kind == MeasureKind.MEASURE_KIND_TERMINATION &&
      measure.state == MeasureState.MEASURE_STATE_ACTIVE) {
    return (
      label: measure.state.uiText(l, kind: measure.kind) ?? commonUnknown(l),
      tone: MeasureChipTone.danger,
    );
  }
  if (measure.state != MeasureState.MEASURE_STATE_SCHEDULED) return null;
  final label = measure.state.uiText(l, kind: measure.kind);
  return label == null ? null : (label: label, tone: MeasureChipTone.warning);
}

/// 卡片上「請求」的狀態來源（契約 `Measure.requests`，新到舊，每筆的 `state` 是伺服器算好的有效狀態）：待處理的請求優先，
/// 其次是最新一筆已裁定（通過或駁回）的請求。結案（CLOSED）的請求不當來源：措施 `state` 已表達撤銷或結束。
MeasureRequest? _statusRequest(Measure measure) {
  for (final request in measure.requests) {
    if (request.state == MeasureRequestState.MEASURE_REQUEST_STATE_PENDING) {
      return request;
    }
  }
  for (final request in measure.requests) {
    if (request.state == MeasureRequestState.MEASURE_REQUEST_STATE_ACCEPTED ||
        request.state == MeasureRequestState.MEASURE_REQUEST_STATE_REJECTED) {
      return request;
    }
  }
  return null;
}

/// 請求的狀態 chip（申覆與請求解除合計，優先序見 [_statusRequest]；文字是目錄的 `with_kind` 變體「{種類}審核中」等）；
/// 沒有請求、結案、或狀態沒有有效值時是 null。
({String label, MeasureChipTone tone})? requestChip(
  Measure measure,
  UiLocale l,
) {
  final request = _statusRequest(measure);
  if (request == null) return null;
  final label = request.state.withKind(l, kind: request.kind);
  if (label == null) return null;
  final tone = switch (request.state) {
    MeasureRequestState.MEASURE_REQUEST_STATE_PENDING =>
      MeasureChipTone.warning,
    MeasureRequestState.MEASURE_REQUEST_STATE_ACCEPTED =>
      MeasureChipTone.success,
    MeasureRequestState.MEASURE_REQUEST_STATE_REJECTED =>
      MeasureChipTone.danger,
    _ => null,
  };
  return tone == null ? null : (label: label, tone: tone);
}

/// 請求的說明行：有待處理的請求時請賣家等候複核；否則是最新一筆已裁定請求的裁定理由（兩種請求共用同一寫法）。
/// 沒有請求、沒有裁定理由或結案時是 null。冷卻期不在這裡，只在 [remediationAction] 的按鈕說明。
String? requestNote(Measure measure, UiLocale l) {
  final request = _statusRequest(measure);
  if (request == null) return null;
  return switch (request.state) {
    MeasureRequestState.MEASURE_REQUEST_STATE_PENDING =>
      request.kind.submittedNote(l),
    MeasureRequestState.MEASURE_REQUEST_STATE_ACCEPTED ||
    MeasureRequestState.MEASURE_REQUEST_STATE_REJECTED =>
      request.decisionReason.isEmpty
          ? null
          : sellerMeasureRequestReplyNote(l, reason: request.decisionReason),
    _ => null,
  };
}

/// 「已改善，請求解除」按鈕的狀態：可不可以按讀伺服器的 `can_request_remediation`；不能按時依
/// `remediation_unavailabilities`（伺服器依優先序排好）的第一筆決定：NOT_RISK／NOT_ACTIVE 不是這項措施該有的按鈕，
/// 回 null 不渲染；其餘 [reason] 是伺服器的句子（解除請求審核中、申覆審核中、冷卻期結束時間都已在句子裡）。用戶端不自己推導資格、原因或時間。
({bool enabled, String? reason})? remediationAction(Measure measure) {
  if (measure.canRequestRemediation) return (enabled: true, reason: null);
  if (measure.remediationUnavailabilities.isEmpty) {
    return (enabled: false, reason: null);
  }
  return switch (measure.remediationUnavailabilities.first.reason) {
    RemediationUnavailableReason.REMEDIATION_UNAVAILABLE_REASON_NOT_RISK ||
    RemediationUnavailableReason.REMEDIATION_UNAVAILABLE_REASON_NOT_ACTIVE =>
      null,
    RemediationUnavailableReason
        .REMEDIATION_UNAVAILABLE_REASON_REMEDIATION_PENDING ||
    RemediationUnavailableReason
        .REMEDIATION_UNAVAILABLE_REASON_APPEAL_PENDING ||
    RemediationUnavailableReason.REMEDIATION_UNAVAILABLE_REASON_COOLDOWN => (
      enabled: false,
      reason: remediationUnavailableMessage(measure),
    ),
    _ => (enabled: false, reason: null),
  };
}

/// 送出前的警告：這筆措施的下一個請求若被駁回會開始冷卻期（`rejection_cooldown`，伺服器給）；沒有冷卻是 null。
/// 天數無條件進位成整日（冷卻期由伺服器以日為單位給）。
String? rejectionCooldownWarning(Measure measure, UiLocale l) {
  if (!measure.hasRejectionCooldown()) return null;
  final seconds = measure.rejectionCooldown.seconds.toInt();
  if (seconds <= 0) return null;
  final days = (seconds + 86399) ~/ 86400;
  return sellerMeasureRejectionCooldownWarning(l, days: days);
}

/// 措施目前的影響（一行文字）與是否仍生效，全由 `state` 與 `kind` 決定；沒有有效的狀態時是 null。
///
/// 沒有結束時間的停權寫「未定結束時間，平台定期複審」（兩端的效果句不同；App 的是目錄的 `seller.Measure.suspension_*`）。終止合作沒有預定結束時間，寫目錄的
/// 「終止生效：{start}」，預告終止因店家關閉而結束時加「合作已於 {end} 結束」。未設的 Timestamp 讀到的是 epoch（1970），所以每一個
/// 時間都先用 `has*()` 確認有設才顯示。
({String text, bool active})? measureEffect(Measure measure, UiLocale l) {
  final end = measure.hasPeriod() && measure.period.hasEndTime()
      ? formatTaipeiTimestamp(measure.period.endTime)
      : '';
  final start = measure.hasPeriod() && measure.period.hasStartTime()
      ? formatTaipeiTimestamp(measure.period.startTime)
      : '';
  final terminated = measure.kind == MeasureKind.MEASURE_KIND_TERMINATION;
  final state = measure.state;
  // 合約：終止合作的 period.start_time 一定有（立即生效是建立時間，預告終止是建立後 30 日）；沒有就不顯示（不寫 1970）。
  final showsEffective =
      terminated &&
      const {
        MeasureState.MEASURE_STATE_SCHEDULED,
        MeasureState.MEASURE_STATE_ACTIVE,
        MeasureState.MEASURE_STATE_ENDED,
      }.contains(state);
  if (showsEffective && start.isEmpty) return null;
  final stateText = state.uiText(l, kind: measure.kind) ?? commonUnknown(l);
  if (state == MeasureState.MEASURE_STATE_REVOKED) {
    return (
      text: _revokeNote(measure.revokeReason, l) ?? stateText,
      active: false,
    );
  }
  if (state == MeasureState.MEASURE_STATE_ISSUED) {
    return (text: stateText, active: false);
  }
  if (showsEffective) {
    final effective = sellerMeasureTerminationEffective(l, start: start);
    // 合約：只有預告終止會是預定生效（見 MEASURE_STATE_SCHEDULED 的註解）。
    if (state == MeasureState.MEASURE_STATE_SCHEDULED) {
      return (text: effective, active: false);
    }
    if (state == MeasureState.MEASURE_STATE_ACTIVE) {
      return (text: effective, active: true);
    }
    return (
      text: end.isEmpty
          ? effective
          : sellerMeasureTerminationEffectiveEnded(l, start: start, end: end),
      active: false,
    );
  }
  if (state == MeasureState.MEASURE_STATE_ACTIVE) {
    return (
      text: end.isEmpty
          ? sellerMeasureSuspensionActiveOpen(l)
          : sellerMeasureSuspensionActiveUntil(l, end: end),
      active: true,
    );
  }
  if (state == MeasureState.MEASURE_STATE_ENDED) {
    return (
      text: end.isEmpty ? stateText : sellerMeasureSuspensionEnded(l, end: end),
      active: false,
    );
  }
  return null;
}

/// 撤銷原因的說明：賣家端只需要說明認定被撤銷、關聯的終止被撤銷（名詞取自目錄的 `RevokeReason`）；接受申覆已由申覆狀態說明，
/// 平台更正、外部決定與其他不另說明，沒有時是 null。
String? _revokeNote(RevokeReason reason, UiLocale l) => switch (reason) {
  RevokeReason.REVOKE_REASON_FINDING_REVOKED ||
  RevokeReason.REVOKE_REASON_SOURCE_REVOKED => reason.uiText(l),
  _ => null,
};

/// 申覆入口的狀態：入口一律顯示；可不可以按讀伺服器的 `can_appeal`，不能按時 [reason] 是 `appeal_unavailabilities`
/// （伺服器依優先序排好）的第一筆的句子，沒有句子時是 null。用戶端不自己推導資格或原因。
({bool enabled, String? reason}) appealAction(Measure measure) {
  if (measure.canAppeal) return (enabled: true, reason: null);
  return (enabled: false, reason: appealUnavailableMessage(measure));
}

/// 終止合作的申覆期限（「申覆期限：2026/10/19 16:30」，臺北時間）；不是終止合作或伺服器沒給期限時是 null。
String? appealDeadlineText(Measure measure, UiLocale l) {
  if (measure.kind != MeasureKind.MEASURE_KIND_TERMINATION ||
      !measure.hasAppealDeadlineTime()) {
    return null;
  }
  return sellerMeasureAppealDeadline(
    l,
    deadline: formatTaipeiTimestamp(measure.appealDeadlineTime),
  );
}

/// 首頁的預告文字：帳號每一筆預定生效（`SCHEDULED`）的終止合作各一則，用它的生效時間（`period.start_time`，伺服器給，不從建立時間推算），
/// 順序照伺服器給的（client 只呈現，不自己排序）；沒有（不是終止、不是預定生效、沒有生效時間）的不算。
List<({String measureId, String text})> terminationNotices(
  Iterable<Measure> measures,
  UiLocale l,
) {
  final scheduled = [
    for (final m in measures)
      if (m.kind == MeasureKind.MEASURE_KIND_TERMINATION &&
          m.state == MeasureState.MEASURE_STATE_SCHEDULED &&
          m.hasPeriod() &&
          m.period.hasStartTime())
        m,
  ];
  return [
    for (final m in scheduled)
      (
        measureId: m.id,
        text: sellerMeasureTerminationNotice(
          l,
          start: formatTaipeiTimestamp(m.period.startTime),
        ),
      ),
  ];
}

/// 「需要處理」：現在可以申覆，或停權還在生效，或預定生效（預告終止，已申覆後 `can_appeal` 是 false 也要留在這裡）。其餘是「歷史」。
bool measureNeedsAttention(Measure measure) =>
    measure.canAppeal ||
    measure.state == MeasureState.MEASURE_STATE_ACTIVE ||
    measure.state == MeasureState.MEASURE_STATE_SCHEDULED;

/// 依 [measureNeedsAttention] 分成兩組，各組保留伺服器給的順序。
({List<Measure> attention, List<Measure> history}) groupMeasures(
  Iterable<Measure> measures,
) {
  final attention = <Measure>[];
  final history = <Measure>[];
  for (final measure in measures) {
    (measureNeedsAttention(measure) ? attention : history).add(measure);
  }
  return (attention: attention, history: history);
}

/// 申覆入口旁的一句說明（與 website 的 `AppealEntry` 相同）：「{原因} · 申覆期限：{時間}」，原因與期限各自可有可無，
/// 兩者都沒有時是 null。
String? appealNoteText(Measure measure, UiLocale l) {
  final parts = [
    appealAction(measure).reason,
    appealDeadlineText(measure, l),
  ].whereType<String>();
  return parts.isEmpty ? null : parts.join(' · ');
}
