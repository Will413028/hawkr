import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart'
    show LocalizedMessage;
import 'package:protobuf/well_known_types/google/protobuf/duration.pb.dart'
    as pb;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart';

const reasonRemediationPending = RemediationUnavailableReason
    .REMEDIATION_UNAVAILABLE_REASON_REMEDIATION_PENDING;
const reasonAppealPending =
    RemediationUnavailableReason.REMEDIATION_UNAVAILABLE_REASON_APPEAL_PENDING;
const reasonCooldown =
    RemediationUnavailableReason.REMEDIATION_UNAVAILABLE_REASON_COOLDOWN;
const reasonNotRisk =
    RemediationUnavailableReason.REMEDIATION_UNAVAILABLE_REASON_NOT_RISK;
const reasonNotActive =
    RemediationUnavailableReason.REMEDIATION_UNAVAILABLE_REASON_NOT_ACTIVE;

/// 伺服器對各個「不能做」的原因給的句子（設計 step10e-Lprime-design.md §7(a)，與 seller 模組 `internal/text/zh-TW.json` 相同）。
/// 假資料依原因補上對應的句子，client 的畫面必須原樣顯示；client 的程式碼不得有這張表。
const appealRevokedSentence = '這筆措施已撤銷，不需要申覆';
const appealAlreadyAppealedSentence = '這筆措施已提出過申覆，每筆措施只能申覆一次';
const appealWindowEndedSentence = '終止合作的申覆期限已過';
const appealRequestPendingSentence = '這筆措施有待處理的解除請求，審核完成後才能申覆';

const remediationPendingSentence = '這筆措施的解除請求審核中，審核完成前不能再提出';
const remediationAppealPendingSentence = '這筆措施的申覆審核中，審核完成前不能請求解除';
const remediationCooldownSentence = '解除請求目前在冷卻期內，2026/10/20 10:06 後可再提出';
const remediationNotRiskSentence = '只有依據風險類型的措施可以請求解除';
const remediationNotActiveSentence = '這筆措施目前不在生效中';

const appealSentences = <AppealUnavailableReason, String>{
  AppealUnavailableReason.APPEAL_UNAVAILABLE_REASON_REVOKED:
      appealRevokedSentence,
  AppealUnavailableReason.APPEAL_UNAVAILABLE_REASON_ALREADY_APPEALED:
      appealAlreadyAppealedSentence,
  AppealUnavailableReason.APPEAL_UNAVAILABLE_REASON_APPEAL_WINDOW_ENDED:
      appealWindowEndedSentence,
  AppealUnavailableReason.APPEAL_UNAVAILABLE_REASON_REQUEST_PENDING:
      appealRequestPendingSentence,
};

const remediationSentences = <RemediationUnavailableReason, String>{
  RemediationUnavailableReason
          .REMEDIATION_UNAVAILABLE_REASON_REMEDIATION_PENDING:
      remediationPendingSentence,
  RemediationUnavailableReason.REMEDIATION_UNAVAILABLE_REASON_APPEAL_PENDING:
      remediationAppealPendingSentence,
  RemediationUnavailableReason.REMEDIATION_UNAVAILABLE_REASON_COOLDOWN:
      remediationCooldownSentence,
  RemediationUnavailableReason.REMEDIATION_UNAVAILABLE_REASON_NOT_RISK:
      remediationNotRiskSentence,
  RemediationUnavailableReason.REMEDIATION_UNAVAILABLE_REASON_NOT_ACTIVE:
      remediationNotActiveSentence,
};

LocalizedMessage serverMessage(String text) =>
    LocalizedMessage(locale: 'zh-TW', message: text);

AppealUnavailability appealUnavailability(
  AppealUnavailableReason reason, {
  String? message,
}) => AppealUnavailability(
  reason: reason,
  message: serverMessage(message ?? appealSentences[reason] ?? '伺服器的句子'),
);

RemediationUnavailability remediationUnavailability(
  RemediationUnavailableReason reason, {
  String? message,
}) => RemediationUnavailability(
  reason: reason,
  message: serverMessage(message ?? remediationSentences[reason] ?? '伺服器的句子'),
);

Timestamp ts(DateTime utc) => Timestamp.fromDateTime(utc);

/// [end] 為 null 表示沒有設 end_time（proto 讀到的是 epoch，顯示層必須用 `hasEndTime()` 擋掉）。
MeasurePeriod measurePeriod({DateTime? start, DateTime? end}) => MeasurePeriod(
  startTime: start == null ? null : ts(start),
  endTime: end == null ? null : ts(end),
);

MeasureFinding finding({
  String id = 'f1',
  PolicyCategory category = PolicyCategory.POLICY_CATEGORY_FOOD_SAFETY,
  ViolationSeverity severity = ViolationSeverity.VIOLATION_SEVERITY_HIGH,
}) => MeasureFinding(id: id, category: category, severity: severity);

/// 預設是一筆生效中、可以申覆的食安停權（期間 10/5–10/19，臺北 10/19 的 16:30 到期）。
/// [findings] 不給時，VIOLATION 依據帶一筆食安認定；RISK 依據沒有認定。
Measure measureOf({
  String id = 'm1',
  MeasureKind kind = MeasureKind.MEASURE_KIND_SUSPENSION,
  MeasureBasis basis = MeasureBasis.MEASURE_BASIS_VIOLATION,
  List<MeasureFinding>? findings,
  RiskGround riskGround = RiskGround.RISK_GROUND_UNSPECIFIED,
  String statement = '商品標示與實品不符',
  MeasureState state = MeasureState.MEASURE_STATE_ACTIVE,
  MeasureRequest? appeal,
  List<MeasureRequest> requests = const [],
  List<RemediationUnavailableReason> remediationUnavailableReasons = const [],
  List<RemediationUnavailability>? remediationUnavailabilities,
  DateTime? createTime,
  bool withCreateTime = true,
  String etag = 'etag-1',
  MeasurePeriod? period,
  bool canAppeal = true,
  RevokeReason revokeReason = RevokeReason.REVOKE_REASON_UNSPECIFIED,
  List<MeasureRequest> remediationRequests = const [],
  bool canRequestRemediation = false,
  DateTime? nextRemediationTime,
  Duration? rejectionCooldown,
  TerminationGround terminationGround =
      TerminationGround.TERMINATION_GROUND_UNSPECIFIED,
  DateTime? appealDeadlineTime,
  List<AppealUnavailableReason>? appealUnavailableReasons,
  List<AppealUnavailability>? appealUnavailabilities,
}) => Measure(
  id: id,
  kind: kind,
  basis: basis,
  findings:
      findings ??
      (basis == MeasureBasis.MEASURE_BASIS_VIOLATION
          ? [finding()]
          : const <MeasureFinding>[]),
  riskGround: riskGround,
  statement: statement,
  state: state,
  period: period,
  requests: [
    ...requests,
    for (final r in remediationRequests)
      r.deepCopy()..kind = MeasureRequestKind.MEASURE_REQUEST_KIND_REMEDIATION,
    if (appeal != null)
      appeal.deepCopy()..kind = MeasureRequestKind.MEASURE_REQUEST_KIND_APPEAL,
  ],
  remediationUnavailabilities:
      remediationUnavailabilities ??
      [
        for (final r in remediationUnavailableReasons)
          remediationUnavailability(r),
      ],
  createTime: withCreateTime
      ? ts(createTime ?? DateTime.utc(2026, 10, 5, 1, 30))
      : null,
  etag: etag,
  canAppeal: canAppeal,
  revokeReason: revokeReason,
  canRequestRemediation: canRequestRemediation,
  nextRemediationTime: nextRemediationTime == null
      ? null
      : ts(nextRemediationTime),
  rejectionCooldown: rejectionCooldown == null
      ? null
      : pb.Duration.fromDart(rejectionCooldown),
  terminationGround: terminationGround,
  appealDeadlineTime: appealDeadlineTime == null
      ? null
      : ts(appealDeadlineTime),
  appealUnavailabilities:
      appealUnavailabilities ??
      [
        for (final r in _appealReasons(
          canAppeal: canAppeal,
          state: state,
          hasAppeal: appeal != null,
          explicit: appealUnavailableReasons,
        ))
          appealUnavailability(r),
      ],
);

/// 契約：`can_appeal` 為否時 `appeal_unavailabilities` 至少一筆（依優先序），為是時為空。測試假資料不能造出違反契約的組合：
/// 不給 [explicit] 時依措施的情況補一個合理的原因，給了但與 [canAppeal] 矛盾就丟 [ArgumentError]。
List<AppealUnavailableReason> _appealReasons({
  required bool canAppeal,
  required MeasureState state,
  required bool hasAppeal,
  required List<AppealUnavailableReason>? explicit,
}) {
  if (explicit != null) {
    if (explicit.contains(
      AppealUnavailableReason.APPEAL_UNAVAILABLE_REASON_UNSPECIFIED,
    )) {
      throw ArgumentError('appeal_unavailabilities 的 reason 不能是 UNSPECIFIED');
    }
    if (canAppeal != explicit.isEmpty) {
      throw ArgumentError(
        'can_appeal=$canAppeal 與 appeal_unavailabilities=$explicit 矛盾',
      );
    }
    return explicit;
  }
  if (canAppeal) return const [];
  if (hasAppeal) {
    return [AppealUnavailableReason.APPEAL_UNAVAILABLE_REASON_ALREADY_APPEALED];
  }
  if (state == MeasureState.MEASURE_STATE_REVOKED) {
    return [AppealUnavailableReason.APPEAL_UNAVAILABLE_REASON_REVOKED];
  }
  return [AppealUnavailableReason.APPEAL_UNAVAILABLE_REASON_REQUEST_PENDING];
}

/// 讓 [measure] 的第一個不能申覆原因是 `UNSPECIFIED` 且沒有句子（伺服器新增了這個版本不認得的原因、又沒帶句子時的樣子）。
/// 這是唯一能在假資料裡出現 UNSPECIFIED 原因的路徑：[measureOf] 一律拒絕，避免測試造出違反契約的措施。
Measure withUnrecognizedAppealReason(Measure measure) => measure.deepCopy()
  ..canAppeal = false
  ..appealUnavailabilities.clear()
  ..appealUnavailabilities.add(
    AppealUnavailability(
      reason: AppealUnavailableReason.APPEAL_UNAVAILABLE_REASON_UNSPECIFIED,
    ),
  );

/// 把 [measure] 兩種不能做的原因的句子都拿掉（伺服器沒有帶 `LocalizedMessage` 時的樣子）；原因本身不動。
Measure withoutUnavailableMessages(Measure measure) {
  final copy = measure.deepCopy();
  for (final u in copy.appealUnavailabilities) {
    u.clearMessage();
  }
  for (final u in copy.remediationUnavailabilities) {
    u.clearMessage();
  }
  return copy;
}

/// 終止合作：依據 CONTRACT，預設是預告終止（NOTICE），預定 [start] 生效（`period.start_time`，與建立時間刻意不同）。
Measure terminationOf({
  String id = 't1',
  MeasureState state = MeasureState.MEASURE_STATE_SCHEDULED,
  TerminationGround ground = TerminationGround.TERMINATION_GROUND_NOTICE,
  DateTime? start,
  DateTime? end,
  DateTime? createTime,
  DateTime? appealDeadlineTime,
  bool canAppeal = true,
  List<AppealUnavailableReason>? appealUnavailableReasons,
  RevokeReason revokeReason = RevokeReason.REVOKE_REASON_UNSPECIFIED,
  List<MeasureRequest> requests = const [],
}) => measureOf(
  id: id,
  requests: requests,
  kind: MeasureKind.MEASURE_KIND_TERMINATION,
  basis: MeasureBasis.MEASURE_BASIS_CONTRACT,
  state: state,
  terminationGround: ground,
  canAppeal: canAppeal,
  appealUnavailableReasons: appealUnavailableReasons,
  appealDeadlineTime: appealDeadlineTime,
  revokeReason: revokeReason,
  createTime: createTime,
  period: measurePeriod(
    start: start ?? DateTime.utc(2026, 11, 20, 4, 30),
    end: end,
  ),
);

Measure activeSuspension({String id = 'm1', bool canAppeal = true}) =>
    measureOf(
      id: id,
      canAppeal: canAppeal,
      period: measurePeriod(
        start: DateTime.utc(2026, 10, 5),
        end: DateTime.utc(2026, 10, 19, 8, 30),
      ),
    );

/// 風險依據的停權：開放式（沒有結束時間），沒有認定。
Measure riskSuspension({
  String id = 'r1',
  RiskGround ground = RiskGround.RISK_GROUND_TRANSACTION,
  bool canAppeal = true,
  bool canRequestRemediation = false,
  MeasureState state = MeasureState.MEASURE_STATE_ACTIVE,
  MeasureRequest? appeal,
  List<MeasureRequest> remediationRequests = const [],
  List<MeasureRequest> requests = const [],
  List<RemediationUnavailableReason> remediationUnavailableReasons = const [],
  List<RemediationUnavailability>? remediationUnavailabilities,
  DateTime? nextRemediationTime,
  Duration? rejectionCooldown,
}) => measureOf(
  id: id,
  basis: MeasureBasis.MEASURE_BASIS_RISK,
  riskGround: ground,
  canAppeal: canAppeal,
  state: state,
  appeal: appeal,
  canRequestRemediation: canRequestRemediation,
  remediationRequests: remediationRequests,
  requests: requests,
  remediationUnavailableReasons: remediationUnavailableReasons,
  remediationUnavailabilities: remediationUnavailabilities,
  nextRemediationTime: nextRemediationTime,
  rejectionCooldown: rejectionCooldown,
  period: measurePeriod(start: DateTime.utc(2026, 10, 5)),
);
