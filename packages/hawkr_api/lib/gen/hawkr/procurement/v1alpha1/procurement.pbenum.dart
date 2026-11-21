// This is a generated file - do not edit.
//
// Generated from hawkr/procurement/v1alpha1/procurement.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// SlotKey 是需求的一個欄位。
class SlotKey extends $pb.ProtobufEnum {
  static const SlotKey SLOT_KEY_UNSPECIFIED =
      SlotKey._(0, _omitEnumNames ? '' : 'SLOT_KEY_UNSPECIFIED');

  /// 品項（買家的原話）。
  static const SlotKey SLOT_KEY_ITEM =
      SlotKey._(1, _omitEnumNames ? '' : 'SLOT_KEY_ITEM');

  /// 品項被判定成的平台分類。
  static const SlotKey SLOT_KEY_ITEM_RESOLUTION =
      SlotKey._(2, _omitEnumNames ? '' : 'SLOT_KEY_ITEM_RESOLUTION');
  static const SlotKey SLOT_KEY_QUANTITY =
      SlotKey._(3, _omitEnumNames ? '' : 'SLOT_KEY_QUANTITY');
  static const SlotKey SLOT_KEY_UNIT =
      SlotKey._(4, _omitEnumNames ? '' : 'SLOT_KEY_UNIT');
  static const SlotKey SLOT_KEY_PACKAGING =
      SlotKey._(5, _omitEnumNames ? '' : 'SLOT_KEY_PACKAGING');
  static const SlotKey SLOT_KEY_BUDGET_PER_UNIT =
      SlotKey._(6, _omitEnumNames ? '' : 'SLOT_KEY_BUDGET_PER_UNIT');
  static const SlotKey SLOT_KEY_DELIVERY_CITY =
      SlotKey._(7, _omitEnumNames ? '' : 'SLOT_KEY_DELIVERY_CITY');
  static const SlotKey SLOT_KEY_DELIVERY_DATE =
      SlotKey._(8, _omitEnumNames ? '' : 'SLOT_KEY_DELIVERY_DATE');
  static const SlotKey SLOT_KEY_EXACT_MATCH_REQUIRED =
      SlotKey._(9, _omitEnumNames ? '' : 'SLOT_KEY_EXACT_MATCH_REQUIRED');

  /// 「品項有多個可能的分類，請買家選一個」這個問題。
  static const SlotKey SLOT_KEY_ITEM_DISAMBIGUATION =
      SlotKey._(10, _omitEnumNames ? '' : 'SLOT_KEY_ITEM_DISAMBIGUATION');

  static const $core.List<SlotKey> values = <SlotKey>[
    SLOT_KEY_UNSPECIFIED,
    SLOT_KEY_ITEM,
    SLOT_KEY_ITEM_RESOLUTION,
    SLOT_KEY_QUANTITY,
    SLOT_KEY_UNIT,
    SLOT_KEY_PACKAGING,
    SLOT_KEY_BUDGET_PER_UNIT,
    SLOT_KEY_DELIVERY_CITY,
    SLOT_KEY_DELIVERY_DATE,
    SLOT_KEY_EXACT_MATCH_REQUIRED,
    SLOT_KEY_ITEM_DISAMBIGUATION,
  ];

  static final $core.List<SlotKey?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 10);
  static SlotKey? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SlotKey._(super.value, super.name);
}

/// RejectionReason 是一個欄位的內容被丟棄或被修正的原因。顯示文字在共用文案目錄，client 依值查，不用伺服器組好的句子。
class RejectionReason extends $pb.ProtobufEnum {
  static const RejectionReason REJECTION_REASON_UNSPECIFIED = RejectionReason._(
      0, _omitEnumNames ? '' : 'REJECTION_REASON_UNSPECIFIED');

  /// 內容是空的。
  static const RejectionReason REJECTION_REASON_EMPTY =
      RejectionReason._(1, _omitEnumNames ? '' : 'REJECTION_REASON_EMPTY');

  /// 文字超過長度上限，已截斷後採用（值有保留）。
  static const RejectionReason REJECTION_REASON_TRUNCATED =
      RejectionReason._(2, _omitEnumNames ? '' : 'REJECTION_REASON_TRUNCATED');
  static const RejectionReason REJECTION_REASON_QUANTITY_RANGE =
      RejectionReason._(
          3, _omitEnumNames ? '' : 'REJECTION_REASON_QUANTITY_RANGE');
  static const RejectionReason REJECTION_REASON_BUDGET_RANGE =
      RejectionReason._(
          4, _omitEnumNames ? '' : 'REJECTION_REASON_BUDGET_RANGE');
  static const RejectionReason REJECTION_REASON_UNIT_NOT_ALLOWED =
      RejectionReason._(
          5, _omitEnumNames ? '' : 'REJECTION_REASON_UNIT_NOT_ALLOWED');
  static const RejectionReason REJECTION_REASON_CITY_UNRECOGNIZED =
      RejectionReason._(
          6, _omitEnumNames ? '' : 'REJECTION_REASON_CITY_UNRECOGNIZED');
  static const RejectionReason REJECTION_REASON_DATE_FORMAT = RejectionReason._(
      7, _omitEnumNames ? '' : 'REJECTION_REASON_DATE_FORMAT');

  /// 日期只有月日、沒有西元年；不替買家補年份。
  static const RejectionReason REJECTION_REASON_DATE_MISSING_YEAR =
      RejectionReason._(
          8, _omitEnumNames ? '' : 'REJECTION_REASON_DATE_MISSING_YEAR');
  static const RejectionReason REJECTION_REASON_DATE_PAST =
      RejectionReason._(9, _omitEnumNames ? '' : 'REJECTION_REASON_DATE_PAST');
  static const RejectionReason REJECTION_REASON_DATE_TOO_FAR =
      RejectionReason._(
          10, _omitEnumNames ? '' : 'REJECTION_REASON_DATE_TOO_FAR');

  /// 品項被丟棄（或還沒有品項），所以不能附上分類判定。
  static const RejectionReason REJECTION_REASON_RESOLUTION_NO_ITEM =
      RejectionReason._(
          11, _omitEnumNames ? '' : 'REJECTION_REASON_RESOLUTION_NO_ITEM');
  static const RejectionReason REJECTION_REASON_RESOLUTION_NO_LABEL =
      RejectionReason._(
          12, _omitEnumNames ? '' : 'REJECTION_REASON_RESOLUTION_NO_LABEL');
  static const RejectionReason REJECTION_REASON_RESOLUTION_UNKNOWN_KIND =
      RejectionReason._(
          13, _omitEnumNames ? '' : 'REJECTION_REASON_RESOLUTION_UNKNOWN_KIND');

  static const $core.List<RejectionReason> values = <RejectionReason>[
    REJECTION_REASON_UNSPECIFIED,
    REJECTION_REASON_EMPTY,
    REJECTION_REASON_TRUNCATED,
    REJECTION_REASON_QUANTITY_RANGE,
    REJECTION_REASON_BUDGET_RANGE,
    REJECTION_REASON_UNIT_NOT_ALLOWED,
    REJECTION_REASON_CITY_UNRECOGNIZED,
    REJECTION_REASON_DATE_FORMAT,
    REJECTION_REASON_DATE_MISSING_YEAR,
    REJECTION_REASON_DATE_PAST,
    REJECTION_REASON_DATE_TOO_FAR,
    REJECTION_REASON_RESOLUTION_NO_ITEM,
    REJECTION_REASON_RESOLUTION_NO_LABEL,
    REJECTION_REASON_RESOLUTION_UNKNOWN_KIND,
  ];

  static final $core.List<RejectionReason?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 13);
  static RejectionReason? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const RejectionReason._(super.value, super.name);
}

/// RequestStatus 是採購需求的進度。
class RequestStatus extends $pb.ProtobufEnum {
  static const RequestStatus REQUEST_STATUS_UNSPECIFIED =
      RequestStatus._(0, _omitEnumNames ? '' : 'REQUEST_STATUS_UNSPECIFIED');

  /// 還在釐清需求。
  static const RequestStatus REQUEST_STATUS_CLARIFYING =
      RequestStatus._(1, _omitEnumNames ? '' : 'REQUEST_STATUS_CLARIFYING');

  /// 必填欄位已齊備，可以搜尋候選。
  static const RequestStatus REQUEST_STATUS_REQUIREMENT_READY = RequestStatus._(
      2, _omitEnumNames ? '' : 'REQUEST_STATUS_REQUIREMENT_READY');

  /// 正在比對候選賣家。
  static const RequestStatus REQUEST_STATUS_MATCHING =
      RequestStatus._(3, _omitEnumNames ? '' : 'REQUEST_STATUS_MATCHING');

  static const $core.List<RequestStatus> values = <RequestStatus>[
    REQUEST_STATUS_UNSPECIFIED,
    REQUEST_STATUS_CLARIFYING,
    REQUEST_STATUS_REQUIREMENT_READY,
    REQUEST_STATUS_MATCHING,
  ];

  static final $core.List<RequestStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static RequestStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const RequestStatus._(super.value, super.name);
}

/// RunType 是 run 的種類。
class RunType extends $pb.ProtobufEnum {
  static const RunType RUN_TYPE_UNSPECIFIED =
      RunType._(0, _omitEnumNames ? '' : 'RUN_TYPE_UNSPECIFIED');

  /// 買家送出一則訊息的一輪。
  static const RunType RUN_TYPE_TURN =
      RunType._(1, _omitEnumNames ? '' : 'RUN_TYPE_TURN');

  /// 候選搜尋。
  static const RunType RUN_TYPE_CANDIDATE_SEARCH =
      RunType._(2, _omitEnumNames ? '' : 'RUN_TYPE_CANDIDATE_SEARCH');

  /// 回覆中斷之後接續的一輪。
  static const RunType RUN_TYPE_CLARIFICATION_RESUME =
      RunType._(3, _omitEnumNames ? '' : 'RUN_TYPE_CLARIFICATION_RESUME');

  /// 詢價發布。
  static const RunType RUN_TYPE_PUBLICATION =
      RunType._(4, _omitEnumNames ? '' : 'RUN_TYPE_PUBLICATION');

  static const $core.List<RunType> values = <RunType>[
    RUN_TYPE_UNSPECIFIED,
    RUN_TYPE_TURN,
    RUN_TYPE_CANDIDATE_SEARCH,
    RUN_TYPE_CLARIFICATION_RESUME,
    RUN_TYPE_PUBLICATION,
  ];

  static final $core.List<RunType?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static RunType? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const RunType._(super.value, super.name);
}

/// RunStatus 是 run 的狀態。
class RunStatus extends $pb.ProtobufEnum {
  static const RunStatus RUN_STATUS_UNSPECIFIED =
      RunStatus._(0, _omitEnumNames ? '' : 'RUN_STATUS_UNSPECIFIED');

  /// 已受理，還沒取得執行資源。
  static const RunStatus RUN_STATUS_QUEUED =
      RunStatus._(1, _omitEnumNames ? '' : 'RUN_STATUS_QUEUED');
  static const RunStatus RUN_STATUS_RUNNING =
      RunStatus._(2, _omitEnumNames ? '' : 'RUN_STATUS_RUNNING');

  /// 停在中斷，等買家回答釐清問題；回答的 run 成功之後轉 SUCCEEDED。
  static const RunStatus RUN_STATUS_WAITING_FOR_INPUT =
      RunStatus._(3, _omitEnumNames ? '' : 'RUN_STATUS_WAITING_FOR_INPUT');

  /// 已收到取消，還沒收斂到終態。
  static const RunStatus RUN_STATUS_CANCEL_REQUESTED =
      RunStatus._(4, _omitEnumNames ? '' : 'RUN_STATUS_CANCEL_REQUESTED');

  /// 以下是終態。
  static const RunStatus RUN_STATUS_SUCCEEDED =
      RunStatus._(5, _omitEnumNames ? '' : 'RUN_STATUS_SUCCEEDED');

  /// 失敗，原因見 RunFailureReason。
  static const RunStatus RUN_STATUS_FAILED =
      RunStatus._(6, _omitEnumNames ? '' : 'RUN_STATUS_FAILED');
  static const RunStatus RUN_STATUS_CANCELLED =
      RunStatus._(7, _omitEnumNames ? '' : 'RUN_STATUS_CANCELLED');

  static const $core.List<RunStatus> values = <RunStatus>[
    RUN_STATUS_UNSPECIFIED,
    RUN_STATUS_QUEUED,
    RUN_STATUS_RUNNING,
    RUN_STATUS_WAITING_FOR_INPUT,
    RUN_STATUS_CANCEL_REQUESTED,
    RUN_STATUS_SUCCEEDED,
    RUN_STATUS_FAILED,
    RUN_STATUS_CANCELLED,
  ];

  static final $core.List<RunStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 7);
  static RunStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const RunStatus._(super.value, super.name);
}

/// RunFailureReason 是 run 失敗的機器可讀原因，只在 FAILED 時有值。
class RunFailureReason extends $pb.ProtobufEnum {
  static const RunFailureReason RUN_FAILURE_REASON_UNSPECIFIED =
      RunFailureReason._(
          0, _omitEnumNames ? '' : 'RUN_FAILURE_REASON_UNSPECIFIED');

  /// 執行 run 的 job 消失（程序重啟或被丟棄），由週期 reconciler 標成失敗並退回額度。買家重送。
  static const RunFailureReason RUN_FAILURE_REASON_WORKER_LOST =
      RunFailureReason._(
          1, _omitEnumNames ? '' : 'RUN_FAILURE_REASON_WORKER_LOST');

  /// 候選搜尋失敗。
  static const RunFailureReason RUN_FAILURE_REASON_SEARCH_FAILED =
      RunFailureReason._(
          2, _omitEnumNames ? '' : 'RUN_FAILURE_REASON_SEARCH_FAILED');

  /// 一輪釐清的處理流程中途出錯。
  static const RunFailureReason RUN_FAILURE_REASON_TURN_PIPELINE_ERROR =
      RunFailureReason._(
          3, _omitEnumNames ? '' : 'RUN_FAILURE_REASON_TURN_PIPELINE_ERROR');

  static const $core.List<RunFailureReason> values = <RunFailureReason>[
    RUN_FAILURE_REASON_UNSPECIFIED,
    RUN_FAILURE_REASON_WORKER_LOST,
    RUN_FAILURE_REASON_SEARCH_FAILED,
    RUN_FAILURE_REASON_TURN_PIPELINE_ERROR,
  ];

  static final $core.List<RunFailureReason?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static RunFailureReason? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const RunFailureReason._(super.value, super.name);
}

/// StepStatus 是 run 的一個步驟的狀態。
class StepStatus extends $pb.ProtobufEnum {
  static const StepStatus STEP_STATUS_UNSPECIFIED =
      StepStatus._(0, _omitEnumNames ? '' : 'STEP_STATUS_UNSPECIFIED');
  static const StepStatus STEP_STATUS_PENDING =
      StepStatus._(1, _omitEnumNames ? '' : 'STEP_STATUS_PENDING');
  static const StepStatus STEP_STATUS_RUNNING =
      StepStatus._(2, _omitEnumNames ? '' : 'STEP_STATUS_RUNNING');
  static const StepStatus STEP_STATUS_WAITING_FOR_INPUT =
      StepStatus._(3, _omitEnumNames ? '' : 'STEP_STATUS_WAITING_FOR_INPUT');
  static const StepStatus STEP_STATUS_SUCCEEDED =
      StepStatus._(4, _omitEnumNames ? '' : 'STEP_STATUS_SUCCEEDED');
  static const StepStatus STEP_STATUS_FAILED =
      StepStatus._(5, _omitEnumNames ? '' : 'STEP_STATUS_FAILED');
  static const StepStatus STEP_STATUS_SKIPPED =
      StepStatus._(6, _omitEnumNames ? '' : 'STEP_STATUS_SKIPPED');
  static const StepStatus STEP_STATUS_CANCELLED =
      StepStatus._(7, _omitEnumNames ? '' : 'STEP_STATUS_CANCELLED');

  static const $core.List<StepStatus> values = <StepStatus>[
    STEP_STATUS_UNSPECIFIED,
    STEP_STATUS_PENDING,
    STEP_STATUS_RUNNING,
    STEP_STATUS_WAITING_FOR_INPUT,
    STEP_STATUS_SUCCEEDED,
    STEP_STATUS_FAILED,
    STEP_STATUS_SKIPPED,
    STEP_STATUS_CANCELLED,
  ];

  static final $core.List<StepStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 7);
  static StepStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const StepStatus._(super.value, super.name);
}

/// InterruptKind 是 run 停下來等待的中斷種類。
class InterruptKind extends $pb.ProtobufEnum {
  static const InterruptKind INTERRUPT_KIND_UNSPECIFIED =
      InterruptKind._(0, _omitEnumNames ? '' : 'INTERRUPT_KIND_UNSPECIFIED');

  /// 釐清：品項有多個可能的分類，等買家選。
  static const InterruptKind INTERRUPT_KIND_CLARIFICATION =
      InterruptKind._(1, _omitEnumNames ? '' : 'INTERRUPT_KIND_CLARIFICATION');

  static const $core.List<InterruptKind> values = <InterruptKind>[
    INTERRUPT_KIND_UNSPECIFIED,
    INTERRUPT_KIND_CLARIFICATION,
  ];

  static final $core.List<InterruptKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 1);
  static InterruptKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const InterruptKind._(super.value, super.name);
}

/// ItemResolutionKind 是品項被判定的方式。
class ItemResolutionKind extends $pb.ProtobufEnum {
  static const ItemResolutionKind ITEM_RESOLUTION_KIND_UNSPECIFIED =
      ItemResolutionKind._(
          0, _omitEnumNames ? '' : 'ITEM_RESOLUTION_KIND_UNSPECIFIED');

  /// 已確認成平台分類樹的一個分類。
  static const ItemResolutionKind ITEM_RESOLUTION_KIND_CATEGORY_CONFIRMED =
      ItemResolutionKind._(
          1, _omitEnumNames ? '' : 'ITEM_RESOLUTION_KIND_CATEGORY_CONFIRMED');

  /// 買家在「找不到符合的品項」上自行描述，沒有分類。
  static const ItemResolutionKind ITEM_RESOLUTION_KIND_BUYER_DESCRIBED =
      ItemResolutionKind._(
          2, _omitEnumNames ? '' : 'ITEM_RESOLUTION_KIND_BUYER_DESCRIBED');

  static const $core.List<ItemResolutionKind> values = <ItemResolutionKind>[
    ITEM_RESOLUTION_KIND_UNSPECIFIED,
    ITEM_RESOLUTION_KIND_CATEGORY_CONFIRMED,
    ITEM_RESOLUTION_KIND_BUYER_DESCRIBED,
  ];

  static final $core.List<ItemResolutionKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static ItemResolutionKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ItemResolutionKind._(super.value, super.name);
}

/// ConversationRole 是對話中一則訊息的發言者。
class ConversationRole extends $pb.ProtobufEnum {
  static const ConversationRole CONVERSATION_ROLE_UNSPECIFIED =
      ConversationRole._(
          0, _omitEnumNames ? '' : 'CONVERSATION_ROLE_UNSPECIFIED');
  static const ConversationRole CONVERSATION_ROLE_USER =
      ConversationRole._(1, _omitEnumNames ? '' : 'CONVERSATION_ROLE_USER');
  static const ConversationRole CONVERSATION_ROLE_ASSISTANT =
      ConversationRole._(
          2, _omitEnumNames ? '' : 'CONVERSATION_ROLE_ASSISTANT');

  static const $core.List<ConversationRole> values = <ConversationRole>[
    CONVERSATION_ROLE_UNSPECIFIED,
    CONVERSATION_ROLE_USER,
    CONVERSATION_ROLE_ASSISTANT,
  ];

  static final $core.List<ConversationRole?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static ConversationRole? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ConversationRole._(super.value, super.name);
}

/// BlockKind 是回覆的敘述段落種類。
class BlockKind extends $pb.ProtobufEnum {
  static const BlockKind BLOCK_KIND_UNSPECIFIED =
      BlockKind._(0, _omitEnumNames ? '' : 'BLOCK_KIND_UNSPECIFIED');
  static const BlockKind BLOCK_KIND_UNDERSTOOD =
      BlockKind._(1, _omitEnumNames ? '' : 'BLOCK_KIND_UNDERSTOOD');
  static const BlockKind BLOCK_KIND_CONTEXT =
      BlockKind._(2, _omitEnumNames ? '' : 'BLOCK_KIND_CONTEXT');
  static const BlockKind BLOCK_KIND_RATIONALE =
      BlockKind._(3, _omitEnumNames ? '' : 'BLOCK_KIND_RATIONALE');
  static const BlockKind BLOCK_KIND_WARNING =
      BlockKind._(4, _omitEnumNames ? '' : 'BLOCK_KIND_WARNING');
  static const BlockKind BLOCK_KIND_NEXT_STEP =
      BlockKind._(5, _omitEnumNames ? '' : 'BLOCK_KIND_NEXT_STEP');
  static const BlockKind BLOCK_KIND_COMPLETE =
      BlockKind._(6, _omitEnumNames ? '' : 'BLOCK_KIND_COMPLETE');

  static const $core.List<BlockKind> values = <BlockKind>[
    BLOCK_KIND_UNSPECIFIED,
    BLOCK_KIND_UNDERSTOOD,
    BLOCK_KIND_CONTEXT,
    BLOCK_KIND_RATIONALE,
    BLOCK_KIND_WARNING,
    BLOCK_KIND_NEXT_STEP,
    BLOCK_KIND_COMPLETE,
  ];

  static final $core.List<BlockKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 6);
  static BlockKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const BlockKind._(super.value, super.name);
}

/// AudienceMode 是詢價發給誰。
class AudienceMode extends $pb.ProtobufEnum {
  static const AudienceMode AUDIENCE_MODE_UNSPECIFIED =
      AudienceMode._(0, _omitEnumNames ? '' : 'AUDIENCE_MODE_UNSPECIFIED');

  /// 公開池：不指定對象，全站未停權的賣家。
  static const AudienceMode AUDIENCE_MODE_WISH_POOL =
      AudienceMode._(1, _omitEnumNames ? '' : 'AUDIENCE_MODE_WISH_POOL');

  /// 指定對象：從候選搜尋的結果勾選。
  static const AudienceMode AUDIENCE_MODE_DIRECT_RECIPIENTS = AudienceMode._(
      2, _omitEnumNames ? '' : 'AUDIENCE_MODE_DIRECT_RECIPIENTS');

  static const $core.List<AudienceMode> values = <AudienceMode>[
    AUDIENCE_MODE_UNSPECIFIED,
    AUDIENCE_MODE_WISH_POOL,
    AUDIENCE_MODE_DIRECT_RECIPIENTS,
  ];

  static final $core.List<AudienceMode?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static AudienceMode? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const AudienceMode._(super.value, super.name);
}

/// PublicationState 是一次發布的狀態。
class PublicationState extends $pb.ProtobufEnum {
  static const PublicationState PUBLICATION_STATE_UNSPECIFIED =
      PublicationState._(
          0, _omitEnumNames ? '' : 'PUBLICATION_STATE_UNSPECIFIED');

  /// 伺服器已建立發布；不代表已投遞或已被看到。
  static const PublicationState PUBLICATION_STATE_PUBLISHED =
      PublicationState._(
          1, _omitEnumNames ? '' : 'PUBLICATION_STATE_PUBLISHED');

  static const $core.List<PublicationState> values = <PublicationState>[
    PUBLICATION_STATE_UNSPECIFIED,
    PUBLICATION_STATE_PUBLISHED,
  ];

  static final $core.List<PublicationState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 1);
  static PublicationState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const PublicationState._(super.value, super.name);
}

/// InviteDeliveryState 是詢價通知投遞給賣家的狀態。
class InviteDeliveryState extends $pb.ProtobufEnum {
  static const InviteDeliveryState INVITE_DELIVERY_STATE_UNSPECIFIED =
      InviteDeliveryState._(
          0, _omitEnumNames ? '' : 'INVITE_DELIVERY_STATE_UNSPECIFIED');

  /// 已排入投遞，還沒交給投遞管道。
  static const InviteDeliveryState INVITE_DELIVERY_STATE_QUEUED =
      InviteDeliveryState._(
          1, _omitEnumNames ? '' : 'INVITE_DELIVERY_STATE_QUEUED');

  /// 已交給投遞管道；不代表賣家已收到或看到。
  static const InviteDeliveryState INVITE_DELIVERY_STATE_SENT =
      InviteDeliveryState._(
          2, _omitEnumNames ? '' : 'INVITE_DELIVERY_STATE_SENT');

  /// 重試耗盡，投遞失敗。
  static const InviteDeliveryState INVITE_DELIVERY_STATE_FAILED =
      InviteDeliveryState._(
          3, _omitEnumNames ? '' : 'INVITE_DELIVERY_STATE_FAILED');

  static const $core.List<InviteDeliveryState> values = <InviteDeliveryState>[
    INVITE_DELIVERY_STATE_UNSPECIFIED,
    INVITE_DELIVERY_STATE_QUEUED,
    INVITE_DELIVERY_STATE_SENT,
    INVITE_DELIVERY_STATE_FAILED,
  ];

  static final $core.List<InviteDeliveryState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static InviteDeliveryState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const InviteDeliveryState._(super.value, super.name);
}

/// SellerResponseType 是賣家回價的方式。
class SellerResponseType extends $pb.ProtobufEnum {
  static const SellerResponseType SELLER_RESPONSE_TYPE_UNSPECIFIED =
      SellerResponseType._(
          0, _omitEnumNames ? '' : 'SELLER_RESPONSE_TYPE_UNSPECIFIED');

  /// 賣家提出自己的商務條件。
  static const SellerResponseType SELLER_RESPONSE_TYPE_MANUAL_QUOTE =
      SellerResponseType._(
          1, _omitEnumNames ? '' : 'SELLER_RESPONSE_TYPE_MANUAL_QUOTE');

  /// 接受買家的非價格條款，但仍須自行填齊品相與全部金額。
  static const SellerResponseType
      SELLER_RESPONSE_TYPE_ACCEPT_NON_PRICE_TERMS_WITH_QUOTE =
      SellerResponseType._(
          2,
          _omitEnumNames
              ? ''
              : 'SELLER_RESPONSE_TYPE_ACCEPT_NON_PRICE_TERMS_WITH_QUOTE');

  static const $core.List<SellerResponseType> values = <SellerResponseType>[
    SELLER_RESPONSE_TYPE_UNSPECIFIED,
    SELLER_RESPONSE_TYPE_MANUAL_QUOTE,
    SELLER_RESPONSE_TYPE_ACCEPT_NON_PRICE_TERMS_WITH_QUOTE,
  ];

  static final $core.List<SellerResponseType?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static SellerResponseType? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SellerResponseType._(super.value, super.name);
}

/// TaxInclusionBasis 是報價與稅的關係。
class TaxInclusionBasis extends $pb.ProtobufEnum {
  static const TaxInclusionBasis TAX_INCLUSION_BASIS_UNSPECIFIED =
      TaxInclusionBasis._(
          0, _omitEnumNames ? '' : 'TAX_INCLUSION_BASIS_UNSPECIFIED');

  /// total_amount 已含 tax_amount。
  static const TaxInclusionBasis TAX_INCLUSION_BASIS_TAX_INCLUDED =
      TaxInclusionBasis._(
          1, _omitEnumNames ? '' : 'TAX_INCLUSION_BASIS_TAX_INCLUDED');

  /// tax_amount 另計，仍計入 total_amount。
  static const TaxInclusionBasis TAX_INCLUSION_BASIS_TAX_EXCLUDED =
      TaxInclusionBasis._(
          2, _omitEnumNames ? '' : 'TAX_INCLUSION_BASIS_TAX_EXCLUDED');

  static const $core.List<TaxInclusionBasis> values = <TaxInclusionBasis>[
    TAX_INCLUSION_BASIS_UNSPECIFIED,
    TAX_INCLUSION_BASIS_TAX_INCLUDED,
    TAX_INCLUSION_BASIS_TAX_EXCLUDED,
  ];

  static final $core.List<TaxInclusionBasis?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static TaxInclusionBasis? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const TaxInclusionBasis._(super.value, super.name);
}

/// SellerResponseStatus 是賣家回價的一個版本的狀態。
class SellerResponseStatus extends $pb.ProtobufEnum {
  static const SellerResponseStatus SELLER_RESPONSE_STATUS_UNSPECIFIED =
      SellerResponseStatus._(
          0, _omitEnumNames ? '' : 'SELLER_RESPONSE_STATUS_UNSPECIFIED');

  /// 目前最新版、未撤回、未過期。
  static const SellerResponseStatus SELLER_RESPONSE_STATUS_ACTIVE =
      SellerResponseStatus._(
          1, _omitEnumNames ? '' : 'SELLER_RESPONSE_STATUS_ACTIVE');

  /// 賣家已提交更新的版本。
  static const SellerResponseStatus SELLER_RESPONSE_STATUS_SUPERSEDED =
      SellerResponseStatus._(
          2, _omitEnumNames ? '' : 'SELLER_RESPONSE_STATUS_SUPERSEDED');
  static const SellerResponseStatus SELLER_RESPONSE_STATUS_WITHDRAWN =
      SellerResponseStatus._(
          3, _omitEnumNames ? '' : 'SELLER_RESPONSE_STATUS_WITHDRAWN');
  static const SellerResponseStatus SELLER_RESPONSE_STATUS_EXPIRED =
      SellerResponseStatus._(
          4, _omitEnumNames ? '' : 'SELLER_RESPONSE_STATUS_EXPIRED');

  static const $core.List<SellerResponseStatus> values = <SellerResponseStatus>[
    SELLER_RESPONSE_STATUS_UNSPECIFIED,
    SELLER_RESPONSE_STATUS_ACTIVE,
    SELLER_RESPONSE_STATUS_SUPERSEDED,
    SELLER_RESPONSE_STATUS_WITHDRAWN,
    SELLER_RESPONSE_STATUS_EXPIRED,
  ];

  static final $core.List<SellerResponseStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static SellerResponseStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SellerResponseStatus._(super.value, super.name);
}

/// QuoteAcceptability 是買家能不能接受一份回價。
class QuoteAcceptability extends $pb.ProtobufEnum {
  static const QuoteAcceptability QUOTE_ACCEPTABILITY_UNSPECIFIED =
      QuoteAcceptability._(
          0, _omitEnumNames ? '' : 'QUOTE_ACCEPTABILITY_UNSPECIFIED');

  /// 唯一能按下接受的狀態。
  static const QuoteAcceptability QUOTE_ACCEPTABILITY_ACCEPTABLE =
      QuoteAcceptability._(
          1, _omitEnumNames ? '' : 'QUOTE_ACCEPTABILITY_ACCEPTABLE');
  static const QuoteAcceptability QUOTE_ACCEPTABILITY_EXPIRED =
      QuoteAcceptability._(
          2, _omitEnumNames ? '' : 'QUOTE_ACCEPTABILITY_EXPIRED');
  static const QuoteAcceptability QUOTE_ACCEPTABILITY_WITHDRAWN =
      QuoteAcceptability._(
          3, _omitEnumNames ? '' : 'QUOTE_ACCEPTABILITY_WITHDRAWN');
  static const QuoteAcceptability QUOTE_ACCEPTABILITY_SUPERSEDED =
      QuoteAcceptability._(
          4, _omitEnumNames ? '' : 'QUOTE_ACCEPTABILITY_SUPERSEDED');

  /// 不符合接受條件（例如這張需求已成交）。
  static const QuoteAcceptability QUOTE_ACCEPTABILITY_INELIGIBLE =
      QuoteAcceptability._(
          5, _omitEnumNames ? '' : 'QUOTE_ACCEPTABILITY_INELIGIBLE');

  static const $core.List<QuoteAcceptability> values = <QuoteAcceptability>[
    QUOTE_ACCEPTABILITY_UNSPECIFIED,
    QUOTE_ACCEPTABILITY_ACCEPTABLE,
    QUOTE_ACCEPTABILITY_EXPIRED,
    QUOTE_ACCEPTABILITY_WITHDRAWN,
    QUOTE_ACCEPTABILITY_SUPERSEDED,
    QUOTE_ACCEPTABILITY_INELIGIBLE,
  ];

  static final $core.List<QuoteAcceptability?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 5);
  static QuoteAcceptability? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const QuoteAcceptability._(super.value, super.name);
}

/// OrderProvisioningState 是接受報價之後建立訂單的進度。
class OrderProvisioningState extends $pb.ProtobufEnum {
  static const OrderProvisioningState ORDER_PROVISIONING_STATE_UNSPECIFIED =
      OrderProvisioningState._(
          0, _omitEnumNames ? '' : 'ORDER_PROVISIONING_STATE_UNSPECIFIED');

  /// 已接受、價格已凍結、尚未付款，訂單建立中。
  static const OrderProvisioningState ORDER_PROVISIONING_STATE_CREATING_ORDER =
      OrderProvisioningState._(
          1, _omitEnumNames ? '' : 'ORDER_PROVISIONING_STATE_CREATING_ORDER');

  /// 訂單已建立，order_no 與 payment_deadline 有值。
  static const OrderProvisioningState ORDER_PROVISIONING_STATE_ORDER_CREATED =
      OrderProvisioningState._(
          2, _omitEnumNames ? '' : 'ORDER_PROVISIONING_STATE_ORDER_CREATED');

  /// 建立失敗，由平台前滾或補償；接受本身仍成立。
  static const OrderProvisioningState ORDER_PROVISIONING_STATE_ORDER_FAILED =
      OrderProvisioningState._(
          3, _omitEnumNames ? '' : 'ORDER_PROVISIONING_STATE_ORDER_FAILED');

  static const $core.List<OrderProvisioningState> values =
      <OrderProvisioningState>[
    ORDER_PROVISIONING_STATE_UNSPECIFIED,
    ORDER_PROVISIONING_STATE_CREATING_ORDER,
    ORDER_PROVISIONING_STATE_ORDER_CREATED,
    ORDER_PROVISIONING_STATE_ORDER_FAILED,
  ];

  static final $core.List<OrderProvisioningState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static OrderProvisioningState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const OrderProvisioningState._(super.value, super.name);
}

/// CancelDisposition 是取消命令的結果。
class CancelDisposition extends $pb.ProtobufEnum {
  static const CancelDisposition CANCEL_DISPOSITION_UNSPECIFIED =
      CancelDisposition._(
          0, _omitEnumNames ? '' : 'CANCEL_DISPOSITION_UNSPECIFIED');

  /// 已收到取消，run 已經或會收斂到 CANCELLED（以回應的 run_status 為準）。
  static const CancelDisposition CANCEL_DISPOSITION_REQUESTED =
      CancelDisposition._(
          1, _omitEnumNames ? '' : 'CANCEL_DISPOSITION_REQUESTED');
  static const CancelDisposition CANCEL_DISPOSITION_ALREADY_CANCELLED =
      CancelDisposition._(
          2, _omitEnumNames ? '' : 'CANCEL_DISPOSITION_ALREADY_CANCELLED');

  /// run 已經結束，沒有東西可取消。
  static const CancelDisposition CANCEL_DISPOSITION_ALREADY_TERMINAL =
      CancelDisposition._(
          3, _omitEnumNames ? '' : 'CANCEL_DISPOSITION_ALREADY_TERMINAL');

  /// 已經來不及取消（run 即將結束）。
  static const CancelDisposition CANCEL_DISPOSITION_TOO_LATE =
      CancelDisposition._(
          4, _omitEnumNames ? '' : 'CANCEL_DISPOSITION_TOO_LATE');

  static const $core.List<CancelDisposition> values = <CancelDisposition>[
    CANCEL_DISPOSITION_UNSPECIFIED,
    CANCEL_DISPOSITION_REQUESTED,
    CANCEL_DISPOSITION_ALREADY_CANCELLED,
    CANCEL_DISPOSITION_ALREADY_TERMINAL,
    CANCEL_DISPOSITION_TOO_LATE,
  ];

  static final $core.List<CancelDisposition?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static CancelDisposition? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const CancelDisposition._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
