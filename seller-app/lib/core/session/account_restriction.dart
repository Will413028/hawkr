import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart'
    show PolicyCategory;
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'account_restriction.g.dart';

/// 帳號被停權時的受限狀態（`GetSession.suspension`）：登入保留，App 只顯示受限頁。
class AccountRestriction {
  const AccountRestriction({
    required this.category,
    required this.reason,
    this.suspendedAt,
  });

  /// 停權的政策分類；UNSPECIFIED 是讀不到停權紀錄。
  final PolicyCategory category;

  /// 伺服器給帳號擁有者看的理由原文；沒有時是空字串。
  final String reason;

  final DateTime? suspendedAt;
}

/// 目前登入帳號的受限狀態；null 是沒有受限。進入的來源：GetSession 回報停權，或前景請求收到 `ACCOUNT_SUSPENDED`。
@Riverpod(keepAlive: true)
class AccountRestrictionState extends _$AccountRestrictionState {
  @override
  AccountRestriction? build() => null;

  void enter(AccountRestriction restriction) => state = restriction;

  void clear() => state = null;
}
