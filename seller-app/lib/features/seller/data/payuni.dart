import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

import '../../../config/flavor.dart';
import '../../../core/session/my_store.dart';
import 'current_kyc.dart';
import 'kyc_fields.dart';

/// 有 PayUni 狀態：`flow.payuni` 有值而且不是 APPROVED（APPROVED 沒有面板）。何時有值由伺服器決定（申請已核准，或賣家正在修正
/// PayUni 拒絕的那份草稿），client 不另判流程狀態。
bool payuniHasState(SellerPayUni? payuni) =>
    payuni != null && payuni.state != PayUniState.PAY_UNI_STATE_APPROVED;

/// [state] 的標題（共用文案目錄的 `PayUniState` 名詞）；`UNSPECIFIED` 與不認得的值是目錄的「未知」。
String payuniStateTitle(PayUniState state, UiLocale l) =>
    state.uiText(l) ?? commonUnknown(l);

/// 這個狀態是不是警示（版面）：審核中不是，其他（含不認得的值）是。
bool payuniStateAlert(PayUniState state) =>
    state != PayUniState.PAY_UNI_STATE_REVIEWING;

/// 要賣家修正的欄位名稱（伺服器已列好的申請欄位與店家欄位，不認得的略過）。申請欄位依表單控制項去重（與缺項清單同一條規則，
/// [fieldsByControl]），店家欄位同名只列一次。
List<String> payuniFixFieldLabels(SellerPayUni? payuni, UiLocale l) {
  if (payuni == null) return const [];
  final labels = <String>[];
  for (final field in fieldsByControl(payuni.fixFields)) {
    final label = kycFieldLabel(field, l);
    if (label != null) labels.add(label);
  }
  for (final field in payuni.fixStoreFields) {
    final label = storeFieldLabel(field, l);
    if (label != null && !labels.contains(label)) labels.add(label);
  }
  return labels;
}

/// 要在店家設定修正的欄位（聯絡資料、地址）有認得的值：畫面要給店家設定的入口。
bool payuniFixPointsAtStore(SellerPayUni? payuni) =>
    payuni != null && payuni.fixStoreFields.any(isKnownStoreField);

/// 錢包顯示前往提領的條件：店家已認證，而且 PayUni 可以收款（`flow.payuni.state == APPROVED`）。App 沒有交易明細，這個判斷
/// 對應 website 的 `walletShowsTransactions`。
bool walletCanWithdraw(MyStore? store, MyKyc? kyc) =>
    (store?.isCertified ?? false) &&
    kyc?.payuni?.state == PayUniState.PAY_UNI_STATE_APPROVED;

/// PayUni 賣家登入頁（前往提領）。PayUni 沒有產生登入連結的 API，網址由 App 依環境決定：正式環境用正式站，其餘用 sandbox。
Uri payuniLoginUri(Flavor flavor) => Uri.parse(
  flavor == Flavor.prod
      ? 'https://www.payuni.com.tw/login'
      : 'https://sandbox.payuni.com.tw/login',
);
