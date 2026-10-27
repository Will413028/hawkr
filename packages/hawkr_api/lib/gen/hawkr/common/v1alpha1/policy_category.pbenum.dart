// This is a generated file - do not edit.
//
// Generated from hawkr/common/v1alpha1/policy_category.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// PolicyCategory 是平台的政策分類（step10e-reports-brief R10）：檢舉人選的類別、平台判定類別與認定的類別共用一份，
/// 各入口只接受自己的子集（見各值的「入口」）。店家治理的值對應賣家合作條款的依據，給賣家的理由要能指到條款；社群內容的值（SPAM、HARASSMENT、HATE）給買家寫的內容的隱藏與警告用，買家條款訂出後補條款依據。
class PolicyCategory extends $pb.ProtobufEnum {
  static const PolicyCategory POLICY_CATEGORY_UNSPECIFIED =
      PolicyCategory._(0, _omitEnumNames ? '' : 'POLICY_CATEGORY_UNSPECIFIED');

  /// 有健康風險或疑似食物中毒，或未揭露安全疑慮的商品（條款第十一條、第十六條第 1 款）。入口：檢舉人、平台判定與認定。
  static const PolicyCategory POLICY_CATEGORY_FOOD_SAFETY =
      PolicyCategory._(1, _omitEnumNames ? '' : 'POLICY_CATEGORY_FOOD_SAFETY');

  /// 違法、違禁或來源不明，而沒有健康風險的商品（第十六條第 1 款）。入口：檢舉人、平台判定與認定。
  static const PolicyCategory POLICY_CATEGORY_PROHIBITED_GOODS =
      PolicyCategory._(
          2, _omitEnumNames ? '' : 'POLICY_CATEGORY_PROHIBITED_GOODS');

  /// 依法應有的登錄、許可或資格沒有完成就販售（第十一條第 2 項）。入口：檢舉人、平台判定與認定。
  static const PolicyCategory POLICY_CATEGORY_UNLICENSED_SALE =
      PolicyCategory._(
          3, _omitEnumNames ? '' : 'POLICY_CATEGORY_UNLICENSED_SALE');

  /// 虛假資料、標示不實或誤導的商品或店家資訊（第十六條第 2 款）。入口：檢舉人、平台判定與認定。
  static const PolicyCategory POLICY_CATEGORY_MISLEADING_INFO =
      PolicyCategory._(
          4, _omitEnumNames ? '' : 'POLICY_CATEGORY_MISLEADING_INFO');

  /// 不實的認證、檢驗或標章（第十六條第 2 款的一種）。入口：檢舉人、平台判定與認定。
  static const PolicyCategory POLICY_CATEGORY_FAKE_CERTIFICATION =
      PolicyCategory._(
          5, _omitEnumNames ? '' : 'POLICY_CATEGORY_FAKE_CERTIFICATION');

  /// 冒用他人商標或外觀的商品本身（第十六條第 1 款）。入口：檢舉人、平台判定與認定。
  static const PolicyCategory POLICY_CATEGORY_COUNTERFEIT =
      PolicyCategory._(6, _omitEnumNames ? '' : 'POLICY_CATEGORY_COUNTERFEIT');

  /// 侵害他人商標、著作權、專利、營業秘密、肖像或其他權利的使用（第十六條第 5 款），例如店名或商品標題用了別人的商標；仿冒商品本身是 COUNTERFEIT。入口：檢舉人、平台判定與認定。
  static const PolicyCategory POLICY_CATEGORY_IP_INFRINGEMENT =
      PolicyCategory._(
          7, _omitEnumNames ? '' : 'POLICY_CATEGORY_IP_INFRINGEMENT');

  /// 偽造交易、洗錢、詐欺、濫用優惠、操縱評價或規避風險控管（第十六條第 2 款）。入口：檢舉人、平台判定與認定。
  static const PolicyCategory POLICY_CATEGORY_FRAUD =
      PolicyCategory._(8, _omitEnumNames ? '' : 'POLICY_CATEGORY_FRAUD');

  /// 引導買家繞過平台付款，或規避平台服務費（第十六條第 3 款）。入口：檢舉人、平台判定與認定。
  static const PolicyCategory POLICY_CATEGORY_OFF_PLATFORM_TRADE =
      PolicyCategory._(
          9, _omitEnumNames ? '' : 'POLICY_CATEGORY_OFF_PLATFORM_TRADE');

  /// 未經授權蒐集、利用、揭露或出售買家個人資料（第十六條第 4 款）。入口：檢舉人、平台判定與認定。
  static const PolicyCategory POLICY_CATEGORY_PRIVACY =
      PolicyCategory._(10, _omitEnumNames ? '' : 'POLICY_CATEGORY_PRIVACY');

  /// 冒用平台名義、未經授權的保證，或讓買家誤認交易主體（第十六條第 8 款）。入口：檢舉人、平台判定與認定。
  static const PolicyCategory POLICY_CATEGORY_IMPERSONATION = PolicyCategory._(
      11, _omitEnumNames ? '' : 'POLICY_CATEGORY_IMPERSONATION');

  /// 干擾、探測或破壞平台與資訊安全，或以爬蟲、自動化不當取得資料（第十六條第 6、7 款）。入口：只有平台判定與認定。
  static const PolicyCategory POLICY_CATEGORY_PLATFORM_ABUSE = PolicyCategory._(
      12, _omitEnumNames ? '' : 'POLICY_CATEGORY_PLATFORM_ABUSE');

  /// 檢舉人無法歸類時選。理由要指到條款，所以分案時要改判成有依據的類別，或直接設定結果。入口：只有檢舉人。
  static const PolicyCategory POLICY_CATEGORY_OTHER =
      PolicyCategory._(13, _omitEnumNames ? '' : 'POLICY_CATEGORY_OTHER');

  /// 社群內容的垃圾訊息：重複、無關或以廣告洗版的貼文、留言與討論區留言（社群，A7）。入口：社群內容的平台判定與認定。
  static const PolicyCategory POLICY_CATEGORY_SPAM =
      PolicyCategory._(14, _omitEnumNames ? '' : 'POLICY_CATEGORY_SPAM');

  /// 社群內容的騷擾或霸凌：針對特定的人或店家的辱罵、威脅或持續打擾（社群，A7）。入口：社群內容的平台判定與認定。
  static const PolicyCategory POLICY_CATEGORY_HARASSMENT =
      PolicyCategory._(15, _omitEnumNames ? '' : 'POLICY_CATEGORY_HARASSMENT');

  /// 社群內容的仇恨或歧視言論：以身分、族群或特徵攻擊他人（社群，A7）。入口：社群內容的平台判定與認定。
  static const PolicyCategory POLICY_CATEGORY_HATE =
      PolicyCategory._(16, _omitEnumNames ? '' : 'POLICY_CATEGORY_HATE');

  static const $core.List<PolicyCategory> values = <PolicyCategory>[
    POLICY_CATEGORY_UNSPECIFIED,
    POLICY_CATEGORY_FOOD_SAFETY,
    POLICY_CATEGORY_PROHIBITED_GOODS,
    POLICY_CATEGORY_UNLICENSED_SALE,
    POLICY_CATEGORY_MISLEADING_INFO,
    POLICY_CATEGORY_FAKE_CERTIFICATION,
    POLICY_CATEGORY_COUNTERFEIT,
    POLICY_CATEGORY_IP_INFRINGEMENT,
    POLICY_CATEGORY_FRAUD,
    POLICY_CATEGORY_OFF_PLATFORM_TRADE,
    POLICY_CATEGORY_PRIVACY,
    POLICY_CATEGORY_IMPERSONATION,
    POLICY_CATEGORY_PLATFORM_ABUSE,
    POLICY_CATEGORY_OTHER,
    POLICY_CATEGORY_SPAM,
    POLICY_CATEGORY_HARASSMENT,
    POLICY_CATEGORY_HATE,
  ];

  static final $core.List<PolicyCategory?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 16);
  static PolicyCategory? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const PolicyCategory._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
