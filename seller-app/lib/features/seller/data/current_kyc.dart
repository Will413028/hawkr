import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';
import '../../../core/session/session_provider.dart';
import '../../../shared/utils/hk_log.dart';
import 'account_scope.dart';
import 'kyc_flow.dart';

part 'current_kyc.g.dart';

/// 一次 `GetMyKYC` 讀取的結果：整份申請（[application]）與它的流程狀態（`flow`）。沒有申請（或申請已取消）時是 [MyKyc.notStarted]。
final class MyKyc {
  const MyKyc({
    required this.status,
    this.identityLocked = false,
    this.application,
  });

  /// 沒有申請：伺服器回 not_found。
  const MyKyc.notStarted()
    : status = KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED,
      identityLocked = false,
      application = null;

  /// 伺服器回的申請；沒有流程狀態、或狀態不是有效值（`UNSPECIFIED`、不認得）時丟 [FormatException]。
  factory MyKyc.fromApplication(KYCApplication application) {
    final flow = application.hasFlow() ? application.flow : null;
    if (flow == null || !isValidKycFlowStatus(flow.status)) {
      throw FormatException('GetMyKYC 回了無效的流程狀態：${flow?.status}');
    }
    return MyKyc(
      status: flow.status,
      identityLocked: flow.identityLocked,
      application: application,
    );
  }

  final KYCFlowStatus status;

  /// 店名與商店介紹是否已鎖定（`flow.identity_locked`，PayUni 已建檔）；只是提示，最終防線在 `UpdateMyStore`。
  final bool identityLocked;

  /// 伺服器回的整份申請（etag、欄位、缺項、退回、證件）；沒有申請時是 null。
  final KYCApplication? application;

  /// 賣家看到的 PayUni 狀態（`flow.payuni`）；申請不在 PayUni 階段時是 null。
  SellerPayUni? get payuni =>
      application != null &&
          application!.hasFlow() &&
          application!.flow.hasPayuni()
      ? application!.flow.payuni
      : null;

  /// 賣家能不能自己修正後重新送出（`flow.can_resubmit`）；沒有申請時是 false。
  bool get canResubmit =>
      application != null &&
      application!.hasFlow() &&
      application!.flow.canResubmit;

  @override
  bool operator ==(Object other) =>
      other is MyKyc &&
      other.status == status &&
      other.identityLocked == identityLocked &&
      other.application == application;

  @override
  int get hashCode => Object.hash(status, identityLocked, application);
}

/// [accountId] 這個帳號的認證流程狀態（`GetMyKYC`）。not_found 是 [MyKyc.notStarted]；其他失敗是 error 狀態，
/// 不當成沒開始認證。以帳號為鍵，換帳號不會看到上一個帳號的結果。申請的唯一來源：寫入的回應用 [adopt] 直接寫進來。
@Riverpod(keepAlive: true, retry: noRetry)
class AccountKyc extends _$AccountKyc {
  /// 這一次讀取進行期間 [adopt] 的結果：讀取回來的是寫入之前的狀態，不能蓋掉它。Notifier 在重建之間保留（riverpod 3.1.0
  /// CHANGELOG 3.0.0-dev.16：「Revert Notifier life-cycle change. They are once again preserved across rebuilds」），所以每次
  /// 讀取開始時要清掉。
  MyKyc? _adoptedDuringBuild;

  @override
  Future<MyKyc> build(String accountId) async {
    _adoptedDuringBuild = null;
    MyKyc newest(MyKyc read) => _adoptedDuringBuild ?? read;
    try {
      final response = await ref
          .watch(sellerClientProvider)
          .getMyKYC(GetMyKYCRequest());
      return newest(MyKyc.fromApplication(response.application));
    } on FormatException catch (error) {
      HKLog.logE('[CurrentKyc] $error');
      final adopted = _adoptedDuringBuild;
      if (adopted != null) return adopted;
      rethrow;
    } catch (error) {
      if (ConnectException.from(error).code == Code.notFound) {
        return newest(const MyKyc.notStarted());
      }
      HKLog.logW('[CurrentKyc] GetMyKYC failed: $error');
      final adopted = _adoptedDuringBuild;
      if (adopted != null) return adopted;
      rethrow;
    }
  }

  /// 寫入（SaveKYCDraft、SubmitKYC、CancelKYC）回的申請直接成為目前的狀態，不必再讀一次；[application] 是 null 表示申請已取消。
  /// 回應沒有有效的流程狀態時退回重讀。
  void adopt(KYCApplication? application) {
    if (application == null) {
      _adopt(const MyKyc.notStarted());
      return;
    }
    try {
      _adopt(MyKyc.fromApplication(application));
    } on FormatException catch (error) {
      HKLog.logW('[CurrentKyc] adopt rejected, rereading: $error');
      ref.invalidateSelf();
    }
  }

  void _adopt(MyKyc kyc) {
    _adoptedDuringBuild = kyc;
    state = AsyncData(kyc);
  }
}

/// 目前登入帳號的認證流程狀態；沒有登入時是 null。
@Riverpod(keepAlive: true)
AsyncValue<MyKyc>? currentKyc(Ref ref) {
  final accountId = signedInAccountId(ref.watch(sessionProvider));
  if (accountId == null) return null;
  return ref.watch(accountKycProvider(accountId));
}

/// 重讀目前帳號的認證流程狀態（`GetMyKYC`）。
@Riverpod(keepAlive: true)
class KycReader extends _$KycReader {
  @override
  void build() {}

  /// 寫入回的申請直接成為目前帳號的申請狀態（見 [AccountKyc.adopt]）。沒有登入時不動。
  void adopt(KYCApplication? application) {
    final accountId = signedInAccountId(ref.read(sessionProvider));
    if (accountId == null) return;
    ref.read(accountKycProvider(accountId).notifier).adopt(application);
  }

  /// 重讀並回傳結果；讀取失敗時丟出該錯誤（目前的值保持 error 狀態）。沒有登入時回 null。
  Future<MyKyc?> reload() async {
    final accountId = signedInAccountId(ref.read(sessionProvider));
    if (accountId == null) return null;
    ref.invalidate(accountKycProvider(accountId));
    return ref.read(accountKycProvider(accountId).future);
  }

  /// 重讀，完成後回傳有沒有成功。
  Future<bool> refresh() async {
    try {
      await reload();
      return true;
    } catch (_) {
      return false;
    }
  }
}
