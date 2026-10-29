// This is a generated file - do not edit.
//
// Generated from hawkr/common/v1alpha1/error_reason.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// ErrorReason 是錯誤 details 裡 google.rpc.ErrorInfo.reason 可以用的值，domain 是 hawkr
/// （ADR 2026-09-25-api-error-model）。reason 放的是值名，所以值名不加 ERROR_REASON_ 前綴，與 googleapis 的
/// google/api/error_reason.proto 相同（api/buf.yaml 只對本檔排除 ENUM_VALUE_PREFIX）。
///
/// 只為 client 會分支的情境新增，而且只搭配 4xx 的錯誤碼。client 遇到不認得的 reason 時照錯誤碼處理。
/// 上線（v1）之後值名就是契約：可以新增，不得改名、刪除或改變搭配的錯誤碼。
class ErrorReason extends $pb.ProtobufEnum {
  static const ErrorReason ERROR_REASON_UNSPECIFIED =
      ErrorReason._(0, _omitEnumNames ? '' : 'ERROR_REASON_UNSPECIFIED');

  /// 依據的版本已經過時：修改時帶的 etag 與目前的不符（搭配 aborted），或以 key 建立時那筆已經存在
  /// （搭配 already_exists）。client 保留使用者的輸入，並顯示最新的版本讓使用者對照後再送出
  /// （ADR 2026-09-25-optimistic-concurrency-etag）。ErrorInfo.metadata 的 conflictTarget 可以說是哪一個物件過時，值由各 API 定義、
  /// client 不認得時照一般版本衝突處理。商家治理（SellerGovernanceAdminService）：`report` 是請求帶的通報 etag 過時，`case` 是案件指令帶的案件 etag 過時，
  /// `target_case` 是分案或移動要加入的案件不成立（版本前進或已關閉），`category_case_opened` 是分案或移動要開新案件、而這個類別剛有了進行中的案件。`authority_order` 是主管機關來函的指令（完成、延長保存、記下告知）帶的來函 etag 過時。
  static const ErrorReason VERSION_CONFLICT =
      ErrorReason._(1, _omitEnumNames ? '' : 'VERSION_CONFLICT');

  /// 賣家帳號停權中，這個操作不在停權時仍可做的範圍（搭配 permission_denied）。client 轉成賣家的停權狀態：
  /// 顯示停權通知，並以 GetMyStore 讀停權的來源與原因。
  static const ErrorReason SELLER_SUSPENDED =
      ErrorReason._(3, _omitEnumNames ? '' : 'SELLER_SUSPENDED');

  /// 商家名單的聯絡方式與其他未封存（進行中或已入駐）的業者相同、而且沒有標記為不是同一家（搭配 failed_precondition）。ErrorInfo.metadata 的
  /// prospectIds 是相符的業者 ID（十進位，由小到大以逗號連接）。client 顯示相符的業者，讓管理員選擇合併，或確認不是同一家後
  /// 在請求帶上 confirmed_distinct_prospect_ids 再送一次（ADR 2026-10-04-prospect-contact-points-and-matching）。
  static const ErrorReason PROSPECT_MATCH_FOUND =
      ErrorReason._(4, _omitEnumNames ? '' : 'PROSPECT_MATCH_FOUND');

  /// 商家名單的統一編號已由另一筆未封存（進行中或已入駐）的業者持有：建立搭配 already_exists，修改與還原搭配
  /// failed_precondition。ErrorInfo.metadata 的 prospectIds 是持有者的 ID（十進位，只有一筆，格式同
  /// PROSPECT_MATCH_FOUND）。修改時持有者是進行中的，client 可以讓管理員與它合併；還原時是同一家就不需要還原
  /// （ADR 2026-10-04-prospect-contact-points-and-matching）。
  static const ErrorReason PROSPECT_TAX_ID_HELD =
      ErrorReason._(10, _omitEnumNames ? '' : 'PROSPECT_TAX_ID_HELD');

  /// 賣家同意的入駐文件版本不是現行版本（搭配 failed_precondition）。client 重讀
  /// seller.v1alpha1.SellerPublicService.GetSellerAgreements，顯示新版本，請賣家重新勾選後再送出。
  static const ErrorReason AGREEMENT_OUTDATED =
      ErrorReason._(6, _omitEnumNames ? '' : 'AGREEMENT_OUTDATED');

  /// 賣家做這個操作需要的前置條件沒有滿足（搭配 failed_precondition）：身分認證未核准，或店家正在隨帳號刪除。停權不用它，
  /// 停權是 SELLER_SUSPENDED。ErrorInfo.metadata 的 missingRequirements 是缺的條件，值是
  /// seller.v1alpha1.SellerRequirement 的值名（不加 SELLER_REQUIREMENT_ 前綴，例如 KYC_APPROVED），依列舉的數字由小到大以逗號連接。client 先看 GetMyStore 的能力狀態，這個 reason 只是
  /// 伺服器逐方法檢查的後備：重讀 GetMyStore，依缺的條件顯示。
  static const ErrorReason SELLER_REQUIREMENT_MISSING =
      ErrorReason._(7, _omitEnumNames ? '' : 'SELLER_REQUIREMENT_MISSING');

  /// 帳號停權中，不能申請開店（搭配 failed_precondition）；解除停權後可以再申請。client 顯示 LocalizedMessage，並導向措施與申覆。帳號同時有生效中的終止合作時是
  /// STORE_OPENING_TERMINATED。
  static const ErrorReason STORE_OPENING_SUSPENDED =
      ErrorReason._(8, _omitEnumNames ? '' : 'STORE_OPENING_SUSPENDED');

  /// 帳號有生效中的終止合作（條款第 21.3 條；終止撤銷之前同一個帳號不能再開店），不能申請開店（搭配 failed_precondition）。client 顯示 LocalizedMessage，
  /// 並導向措施與申覆。換身分認證被另一個帳號的終止擋下的情形不用它（隱私：不透露命中的帳號或欄位），那條路徑維持通用的 failed_precondition 與文字。
  static const ErrorReason STORE_OPENING_TERMINATED =
      ErrorReason._(9, _omitEnumNames ? '' : 'STORE_OPENING_TERMINATED');

  /// 帳號擁有商店，不能被停權（搭配 failed_precondition）：停權對店家、未結訂單與撥款的效果由賣家條款決定，條款定出之前有店的帳號不停權。
  /// 只出現在後台與內部的處分路徑；client 顯示 LocalizedMessage，請營運先處理店家（措施或關店），再停權帳號。
  static const ErrorReason ACCOUNT_HAS_STORE =
      ErrorReason._(11, _omitEnumNames ? '' : 'ACCOUNT_HAS_STORE');

  static const $core.List<ErrorReason> values = <ErrorReason>[
    ERROR_REASON_UNSPECIFIED,
    VERSION_CONFLICT,
    SELLER_SUSPENDED,
    PROSPECT_MATCH_FOUND,
    PROSPECT_TAX_ID_HELD,
    AGREEMENT_OUTDATED,
    SELLER_REQUIREMENT_MISSING,
    STORE_OPENING_SUSPENDED,
    STORE_OPENING_TERMINATED,
    ACCOUNT_HAS_STORE,
  ];

  static final $core.List<ErrorReason?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 11);
  static ErrorReason? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ErrorReason._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
