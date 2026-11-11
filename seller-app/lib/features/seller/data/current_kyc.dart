import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';
import '../../../core/session/session_provider.dart';
import '../../../shared/utils/hk_log.dart';
import 'account_scope.dart';

part 'current_kyc.g.dart';

/// 一次 `GetMyKYC` 讀取的結果：整份申請（[application]）。沒有申請（或申請已取消）時是 [MyKyc.notStarted]。
final class MyKyc {
  const MyKyc({this.application});

  /// 沒有申請：伺服器回 not_found。
  const MyKyc.notStarted() : application = null;

  /// 伺服器回的申請；狀態不是有效值（`UNSPECIFIED`、不認得）時丟 [FormatException]。
  factory MyKyc.fromApplication(KYCApplication application) {
    if (application.status == KYCStatus.KYC_STATUS_UNSPECIFIED) {
      throw FormatException('GetMyKYC 回了無效的申請狀態：${application.status}');
    }
    return MyKyc(application: application);
  }

  /// 伺服器回的整份申請（etag、欄位、缺項、退回、證件）；沒有申請時是 null。
  final KYCApplication? application;

  /// 平台審核的狀態（`KYCApplication.status`）；沒有申請是 null。
  KYCStatus? get reviewStatus => application?.status;

  @override
  bool operator ==(Object other) =>
      other is MyKyc && other.application == application;

  @override
  int get hashCode => application.hashCode;
}

/// [accountId] 這個帳號的認證申請（`GetMyKYC`）。not_found 是 [MyKyc.notStarted]；其他失敗是 error 狀態，
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
  /// 回應沒有有效的申請狀態時退回重讀。
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

/// 目前登入帳號的認證申請；沒有登入時是 null。
@Riverpod(keepAlive: true)
AsyncValue<MyKyc>? currentKyc(Ref ref) {
  final accountId = signedInAccountId(ref.watch(sessionProvider));
  if (accountId == null) return null;
  return ref.watch(accountKycProvider(accountId));
}

/// 重讀目前帳號的認證申請（`GetMyKYC`）。
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
