// This is a generated file - do not edit.
//
// Generated from hawkr/notification/v1alpha1/notification.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// NotificationAudience 是通知的收件身分面。
class NotificationAudience extends $pb.ProtobufEnum {
  static const NotificationAudience NOTIFICATION_AUDIENCE_UNSPECIFIED =
      NotificationAudience._(
          0, _omitEnumNames ? '' : 'NOTIFICATION_AUDIENCE_UNSPECIFIED');

  /// 買家（買家 App、網站買家端）。
  static const NotificationAudience NOTIFICATION_AUDIENCE_BUYER =
      NotificationAudience._(
          1, _omitEnumNames ? '' : 'NOTIFICATION_AUDIENCE_BUYER');

  /// 店家（賣家 App、網站賣家中心）。
  static const NotificationAudience NOTIFICATION_AUDIENCE_STORE =
      NotificationAudience._(
          2, _omitEnumNames ? '' : 'NOTIFICATION_AUDIENCE_STORE');

  static const $core.List<NotificationAudience> values = <NotificationAudience>[
    NOTIFICATION_AUDIENCE_UNSPECIFIED,
    NOTIFICATION_AUDIENCE_BUYER,
    NOTIFICATION_AUDIENCE_STORE,
  ];

  static final $core.List<NotificationAudience?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static NotificationAudience? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const NotificationAudience._(super.value, super.name);
}

/// NotificationCategory 是偏好分類：使用者設定偏好的單位，也是收件匣的篩選單位。
/// 交易必要的分類，收件匣與 email 不可關；互動與行銷的分類全部可關、預設開；公告收件匣與 email 不可關、推播可關。
class NotificationCategory extends $pb.ProtobufEnum {
  static const NotificationCategory NOTIFICATION_CATEGORY_UNSPECIFIED =
      NotificationCategory._(
          0, _omitEnumNames ? '' : 'NOTIFICATION_CATEGORY_UNSPECIFIED');

  /// 訂單（交易必要）。
  static const NotificationCategory NOTIFICATION_CATEGORY_ORDER =
      NotificationCategory._(
          1, _omitEnumNames ? '' : 'NOTIFICATION_CATEGORY_ORDER');

  /// 出貨（交易必要）。
  static const NotificationCategory NOTIFICATION_CATEGORY_SHIPMENT =
      NotificationCategory._(
          2, _omitEnumNames ? '' : 'NOTIFICATION_CATEGORY_SHIPMENT');

  /// 配送異常（交易必要，只有店家；推播也不可關）。
  static const NotificationCategory NOTIFICATION_CATEGORY_SHIPMENT_ANOMALY =
      NotificationCategory._(
          3, _omitEnumNames ? '' : 'NOTIFICATION_CATEGORY_SHIPMENT_ANOMALY');

  /// 退款與售後（交易必要）。
  static const NotificationCategory NOTIFICATION_CATEGORY_AFTERSALES =
      NotificationCategory._(
          4, _omitEnumNames ? '' : 'NOTIFICATION_CATEGORY_AFTERSALES');

  /// 撥款（交易必要，只有店家）。
  static const NotificationCategory NOTIFICATION_CATEGORY_PAYOUT =
      NotificationCategory._(
          5, _omitEnumNames ? '' : 'NOTIFICATION_CATEGORY_PAYOUT');

  /// 帳號與治理（交易必要）。
  static const NotificationCategory NOTIFICATION_CATEGORY_ACCOUNT =
      NotificationCategory._(
          6, _omitEnumNames ? '' : 'NOTIFICATION_CATEGORY_ACCOUNT');

  /// 聊天（互動）。
  static const NotificationCategory NOTIFICATION_CATEGORY_CHAT =
      NotificationCategory._(
          7, _omitEnumNames ? '' : 'NOTIFICATION_CATEGORY_CHAT');

  /// 評價（互動，只有店家）。
  static const NotificationCategory NOTIFICATION_CATEGORY_REVIEW =
      NotificationCategory._(
          8, _omitEnumNames ? '' : 'NOTIFICATION_CATEGORY_REVIEW');

  /// 留言（互動）。
  static const NotificationCategory NOTIFICATION_CATEGORY_COMMENT =
      NotificationCategory._(
          9, _omitEnumNames ? '' : 'NOTIFICATION_CATEGORY_COMMENT');

  /// 讚（互動）。
  static const NotificationCategory NOTIFICATION_CATEGORY_LIKE =
      NotificationCategory._(
          10, _omitEnumNames ? '' : 'NOTIFICATION_CATEGORY_LIKE');

  /// 被追蹤（互動，只有店家）。
  static const NotificationCategory NOTIFICATION_CATEGORY_FOLLOW =
      NotificationCategory._(
          11, _omitEnumNames ? '' : 'NOTIFICATION_CATEGORY_FOLLOW');

  /// 追蹤的店發文（互動，只有買家）。
  static const NotificationCategory NOTIFICATION_CATEGORY_FOLLOWED_STORE_POST =
      NotificationCategory._(12,
          _omitEnumNames ? '' : 'NOTIFICATION_CATEGORY_FOLLOWED_STORE_POST');

  /// 優惠券（行銷，只有買家）。
  static const NotificationCategory NOTIFICATION_CATEGORY_COUPON =
      NotificationCategory._(
          13, _omitEnumNames ? '' : 'NOTIFICATION_CATEGORY_COUPON');

  /// 補貨（行銷，只有買家）。
  static const NotificationCategory NOTIFICATION_CATEGORY_RESTOCK =
      NotificationCategory._(
          14, _omitEnumNames ? '' : 'NOTIFICATION_CATEGORY_RESTOCK');

  /// 後台系統公告（兩個受眾都有；收件匣一定收到，推播可在偏好關閉）。
  static const NotificationCategory NOTIFICATION_CATEGORY_ANNOUNCEMENT =
      NotificationCategory._(
          15, _omitEnumNames ? '' : 'NOTIFICATION_CATEGORY_ANNOUNCEMENT');

  static const $core.List<NotificationCategory> values = <NotificationCategory>[
    NOTIFICATION_CATEGORY_UNSPECIFIED,
    NOTIFICATION_CATEGORY_ORDER,
    NOTIFICATION_CATEGORY_SHIPMENT,
    NOTIFICATION_CATEGORY_SHIPMENT_ANOMALY,
    NOTIFICATION_CATEGORY_AFTERSALES,
    NOTIFICATION_CATEGORY_PAYOUT,
    NOTIFICATION_CATEGORY_ACCOUNT,
    NOTIFICATION_CATEGORY_CHAT,
    NOTIFICATION_CATEGORY_REVIEW,
    NOTIFICATION_CATEGORY_COMMENT,
    NOTIFICATION_CATEGORY_LIKE,
    NOTIFICATION_CATEGORY_FOLLOW,
    NOTIFICATION_CATEGORY_FOLLOWED_STORE_POST,
    NOTIFICATION_CATEGORY_COUPON,
    NOTIFICATION_CATEGORY_RESTOCK,
    NOTIFICATION_CATEGORY_ANNOUNCEMENT,
  ];

  static final $core.List<NotificationCategory?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 15);
  static NotificationCategory? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const NotificationCategory._(super.value, super.name);
}

/// NotificationChannel 是通知送達的渠道。
class NotificationChannel extends $pb.ProtobufEnum {
  static const NotificationChannel NOTIFICATION_CHANNEL_UNSPECIFIED =
      NotificationChannel._(
          0, _omitEnumNames ? '' : 'NOTIFICATION_CHANNEL_UNSPECIFIED');

  /// 收件匣。
  static const NotificationChannel NOTIFICATION_CHANNEL_INBOX =
      NotificationChannel._(
          1, _omitEnumNames ? '' : 'NOTIFICATION_CHANNEL_INBOX');

  /// App 推播。
  static const NotificationChannel NOTIFICATION_CHANNEL_PUSH =
      NotificationChannel._(
          2, _omitEnumNames ? '' : 'NOTIFICATION_CHANNEL_PUSH');

  /// Email。
  static const NotificationChannel NOTIFICATION_CHANNEL_EMAIL =
      NotificationChannel._(
          3, _omitEnumNames ? '' : 'NOTIFICATION_CHANNEL_EMAIL');

  static const $core.List<NotificationChannel> values = <NotificationChannel>[
    NOTIFICATION_CHANNEL_UNSPECIFIED,
    NOTIFICATION_CHANNEL_INBOX,
    NOTIFICATION_CHANNEL_PUSH,
    NOTIFICATION_CHANNEL_EMAIL,
  ];

  static final $core.List<NotificationChannel?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static NotificationChannel? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const NotificationChannel._(super.value, super.name);
}

/// InboxPresentation 是收件匣的呈現分類，決定圖示與分組；不是每個通知型別一個值，client 不用它判斷點擊要去哪（用 deep_link）。
/// client 遇到 UNSPECIFIED 或不認得的值，用 SYSTEM 的呈現。
class InboxPresentation extends $pb.ProtobufEnum {
  static const InboxPresentation INBOX_PRESENTATION_UNSPECIFIED =
      InboxPresentation._(
          0, _omitEnumNames ? '' : 'INBOX_PRESENTATION_UNSPECIFIED');
  static const InboxPresentation INBOX_PRESENTATION_ORDER =
      InboxPresentation._(1, _omitEnumNames ? '' : 'INBOX_PRESENTATION_ORDER');
  static const InboxPresentation INBOX_PRESENTATION_REFUND =
      InboxPresentation._(2, _omitEnumNames ? '' : 'INBOX_PRESENTATION_REFUND');
  static const InboxPresentation INBOX_PRESENTATION_PAYOUT =
      InboxPresentation._(3, _omitEnumNames ? '' : 'INBOX_PRESENTATION_PAYOUT');
  static const InboxPresentation INBOX_PRESENTATION_MESSAGE =
      InboxPresentation._(
          4, _omitEnumNames ? '' : 'INBOX_PRESENTATION_MESSAGE');
  static const InboxPresentation INBOX_PRESENTATION_SOCIAL =
      InboxPresentation._(5, _omitEnumNames ? '' : 'INBOX_PRESENTATION_SOCIAL');
  static const InboxPresentation INBOX_PRESENTATION_REVIEW =
      InboxPresentation._(6, _omitEnumNames ? '' : 'INBOX_PRESENTATION_REVIEW');
  static const InboxPresentation INBOX_PRESENTATION_PROMOTION =
      InboxPresentation._(
          7, _omitEnumNames ? '' : 'INBOX_PRESENTATION_PROMOTION');
  static const InboxPresentation INBOX_PRESENTATION_PROCUREMENT =
      InboxPresentation._(
          8, _omitEnumNames ? '' : 'INBOX_PRESENTATION_PROCUREMENT');
  static const InboxPresentation INBOX_PRESENTATION_ACCOUNT =
      InboxPresentation._(
          9, _omitEnumNames ? '' : 'INBOX_PRESENTATION_ACCOUNT');
  static const InboxPresentation INBOX_PRESENTATION_SYSTEM =
      InboxPresentation._(
          10, _omitEnumNames ? '' : 'INBOX_PRESENTATION_SYSTEM');

  static const $core.List<InboxPresentation> values = <InboxPresentation>[
    INBOX_PRESENTATION_UNSPECIFIED,
    INBOX_PRESENTATION_ORDER,
    INBOX_PRESENTATION_REFUND,
    INBOX_PRESENTATION_PAYOUT,
    INBOX_PRESENTATION_MESSAGE,
    INBOX_PRESENTATION_SOCIAL,
    INBOX_PRESENTATION_REVIEW,
    INBOX_PRESENTATION_PROMOTION,
    INBOX_PRESENTATION_PROCUREMENT,
    INBOX_PRESENTATION_ACCOUNT,
    INBOX_PRESENTATION_SYSTEM,
  ];

  static final $core.List<InboxPresentation?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 10);
  static InboxPresentation? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const InboxPresentation._(super.value, super.name);
}

/// StateRefreshHint 是推播與串流訊號帶的狀態重讀提示：這則通知代表哪一類狀態變了，client 順便重讀對應的資料。
/// UNSPECIFIED 表示不需要重讀。
class StateRefreshHint extends $pb.ProtobufEnum {
  static const StateRefreshHint STATE_REFRESH_HINT_UNSPECIFIED =
      StateRefreshHint._(
          0, _omitEnumNames ? '' : 'STATE_REFRESH_HINT_UNSPECIFIED');

  /// 訂單狀態。
  static const StateRefreshHint STATE_REFRESH_HINT_ORDER =
      StateRefreshHint._(1, _omitEnumNames ? '' : 'STATE_REFRESH_HINT_ORDER');

  /// 售後狀態。
  static const StateRefreshHint STATE_REFRESH_HINT_AFTER_SALE =
      StateRefreshHint._(
          2, _omitEnumNames ? '' : 'STATE_REFRESH_HINT_AFTER_SALE');

  /// 賣家狀態（店家、停權、開通）。
  static const StateRefreshHint STATE_REFRESH_HINT_SELLER_STATUS =
      StateRefreshHint._(
          3, _omitEnumNames ? '' : 'STATE_REFRESH_HINT_SELLER_STATUS');

  static const $core.List<StateRefreshHint> values = <StateRefreshHint>[
    STATE_REFRESH_HINT_UNSPECIFIED,
    STATE_REFRESH_HINT_ORDER,
    STATE_REFRESH_HINT_AFTER_SALE,
    STATE_REFRESH_HINT_SELLER_STATUS,
  ];

  static final $core.List<StateRefreshHint?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static StateRefreshHint? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const StateRefreshHint._(super.value, super.name);
}

/// NotificationApp 是裝置 token 所屬的 App。BUYER_APP 只收 BUYER 受眾的推播，SELLER_APP 只收 STORE 受眾的。
class NotificationApp extends $pb.ProtobufEnum {
  static const NotificationApp NOTIFICATION_APP_UNSPECIFIED = NotificationApp._(
      0, _omitEnumNames ? '' : 'NOTIFICATION_APP_UNSPECIFIED');
  static const NotificationApp NOTIFICATION_APP_BUYER_APP =
      NotificationApp._(1, _omitEnumNames ? '' : 'NOTIFICATION_APP_BUYER_APP');
  static const NotificationApp NOTIFICATION_APP_SELLER_APP =
      NotificationApp._(2, _omitEnumNames ? '' : 'NOTIFICATION_APP_SELLER_APP');

  static const $core.List<NotificationApp> values = <NotificationApp>[
    NOTIFICATION_APP_UNSPECIFIED,
    NOTIFICATION_APP_BUYER_APP,
    NOTIFICATION_APP_SELLER_APP,
  ];

  static final $core.List<NotificationApp?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static NotificationApp? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const NotificationApp._(super.value, super.name);
}

/// SellerWorkspaceChange 是賣家工作區哪一類資料變了。
class SellerWorkspaceChange extends $pb.ProtobufEnum {
  static const SellerWorkspaceChange SELLER_WORKSPACE_CHANGE_UNSPECIFIED =
      SellerWorkspaceChange._(
          0, _omitEnumNames ? '' : 'SELLER_WORKSPACE_CHANGE_UNSPECIFIED');

  /// 訂單（含物流與付款狀態）。
  static const SellerWorkspaceChange SELLER_WORKSPACE_CHANGE_ORDER =
      SellerWorkspaceChange._(
          1, _omitEnumNames ? '' : 'SELLER_WORKSPACE_CHANGE_ORDER');

  /// 售後。
  static const SellerWorkspaceChange SELLER_WORKSPACE_CHANGE_AFTER_SALE =
      SellerWorkspaceChange._(
          2, _omitEnumNames ? '' : 'SELLER_WORKSPACE_CHANGE_AFTER_SALE');

  static const $core.List<SellerWorkspaceChange> values =
      <SellerWorkspaceChange>[
    SELLER_WORKSPACE_CHANGE_UNSPECIFIED,
    SELLER_WORKSPACE_CHANGE_ORDER,
    SELLER_WORKSPACE_CHANGE_AFTER_SALE,
  ];

  static final $core.List<SellerWorkspaceChange?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static SellerWorkspaceChange? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SellerWorkspaceChange._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
