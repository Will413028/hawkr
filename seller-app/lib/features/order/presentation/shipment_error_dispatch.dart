import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../shared/utils/floating_toast.dart';

/// 從 ApiException 取得「業務碼」。
///
/// 後端通常 throw `ApiException(code: <HTTP status>, message: "[5xxx] xxx")`，
/// 真正的業務碼藏在 message 開頭的 [5xxx] 前綴；先試解析它，
/// 沒有的話才退回 [ApiException.code]。
int shipmentBusinessCode(ApiException e) {
  final m = RegExp(r'^\[(\d+)\]').firstMatch(e.message);
  if (m != null) {
    final parsed = int.tryParse(m.group(1)!);
    if (parsed != null) return parsed;
  }
  return e.code;
}

/// 處理物流相關 API 業務碼（backend-payuni#666 — 5001 / 5046 / 5047 / 5048）。
///
/// 命中後已顯示對應 toast 並回 true；未命中回 false 讓 caller fallback。
/// 5046 / 5047 是暫時性錯誤，**不能紅字** — 系統會自動補拉，提示用戶稍後再試或刷新即可。
///
/// 收到 5046 時會額外呼叫 [markShipmentCreated]，
/// 把該 orderNo 加進前端 cooldown，避免用戶在 30 秒內反覆點按再吃到 5046
/// （常見場景：後端已建立物流單但 list API 還沒同步，UI 仍給用戶「預約出貨」按鈕）。
bool handleShipmentApiError(BuildContext context, Object e, {String? orderNo}) {
  if (e is! ApiException) return false;
  final bizCode = shipmentBusinessCode(e);
  switch (bizCode) {
    case 5001:
      FloatingToast.error(context, '此筆訂單異常（找不到物流資料），請聯絡客服處理');
      return true;
    case 5046:
      if (orderNo != null && orderNo.isNotEmpty) {
        markShipmentCreated(orderNo);
        incrementShipmentRetryCount(orderNo);
      }
      if (orderNo != null && isShipmentBackendStuck(orderNo)) {
        FloatingToast.show(
          context,
          '物流商通訊異常處理中，請稍後（數分鐘）再試或聯絡客服',
          icon: AppIcons.warningAmberOutlined,
        );
        return true;
      }
      final remaining5046 = orderNo != null
          ? shipmentCooldownRemainingSeconds(orderNo)
          : 30;
      FloatingToast.show(
        context,
        remaining5046 > 0 ? '物流單剛建立，請等 $remaining5046 秒後再操作' : '物流單剛建立，請稍候再操作',
        icon: AppIcons.accessTime,
      );
      return true;
    case 5047:
      if (orderNo != null && orderNo.isNotEmpty) {
        markShipmentCreated(orderNo);
        incrementShipmentRetryCount(orderNo);
      }
      if (orderNo != null && isShipmentBackendStuck(orderNo)) {
        FloatingToast.show(
          context,
          '物流商通訊異常處理中，請稍後（數分鐘）再試或聯絡客服',
          icon: AppIcons.warningAmberOutlined,
        );
        return true;
      }
      final remaining5047 = orderNo != null
          ? shipmentCooldownRemainingSeconds(orderNo)
          : 30;
      FloatingToast.show(
        context,
        remaining5047 > 0
            ? '系統處理中（PayUni 通訊中），請等 $remaining5047 秒後再試'
            : '系統處理中，請稍候再試',
        icon: AppIcons.sync,
      );
      return true;
    case 5048:
      FloatingToast.error(context, e.message);
      return true;
    default:
      return false;
  }
}

/// 把 ApiException 轉成適合 toast 的口語化文字（剝掉 [code] 前綴與括號內技術備註）。
/// 用於 [handleShipmentApiError] 沒命中時的 fallback toast，
/// 避免直接把 `ApiException(5xxx): xxx` 整串給用戶看。
String friendlyShipmentMessage(Object e) {
  if (e is ApiException) {
    var msg = e.message.trim();
    msg = msg.replaceFirst(RegExp(r'^\[\d+\]\s*'), '');
    msg = msg.replaceAll(RegExp(r'（[^）]*）'), '').trim();
    return msg.isEmpty ? '操作失敗，請稍後再試' : msg;
  }
  return '操作失敗，請稍後再試';
}

const _shipmentCooldownDuration = Duration(seconds: 30);
const _shipmentBackendStuckThreshold = 3;
final Map<String, DateTime> _shipmentCreatedAt = {};
final Map<String, int> _shipment5046Count = {};

/// 檢查訂單是否還在物流單建立 cooldown 期間。
bool isUnderShipmentCooldown(String orderNo) {
  final t = _shipmentCreatedAt[orderNo];
  if (t == null) return false;
  return DateTime.now().difference(t) < _shipmentCooldownDuration;
}

/// 取得剩餘冷卻秒數（給 toast 顯示用）。
int shipmentCooldownRemainingSeconds(String orderNo) {
  final t = _shipmentCreatedAt[orderNo];
  if (t == null) return 0;
  final remaining =
      _shipmentCooldownDuration.inSeconds -
      DateTime.now().difference(t).inSeconds;
  return remaining > 0 ? remaining : 0;
}

/// 物流單成功建立後呼叫，啟動 30 秒冷卻計時。
void markShipmentCreated(String orderNo) {
  _shipmentCreatedAt[orderNo] = DateTime.now();
}

final Map<String, ({String odno, String validationNo})> _localOdnoOverrides =
    {};

/// 取號成功後本地暫存（list endpoint 同步前的 placeholder）。
void setLocalOdnoOverride(
  String orderNo, {
  required String odno,
  required String validationNo,
}) {
  _localOdnoOverrides[orderNo] = (odno: odno, validationNo: validationNo);
}

/// 取得 order 的有效 odno — 優先用後端傳的，沒有就 fallback 本地暫存。
String effectiveOdno(String orderNo, String backendOdno) {
  if (backendOdno.isNotEmpty) return backendOdno;
  return _localOdnoOverrides[orderNo]?.odno ?? '';
}

/// 取得 order 的有效 validationNo — 優先後端，沒有就 fallback 本地。
String effectiveValidationNo(String orderNo, String backendValidationNo) {
  if (backendValidationNo.isNotEmpty) return backendValidationNo;
  return _localOdnoOverrides[orderNo]?.validationNo ?? '';
}

/// 收到 5046 / 5047 時呼叫，計數連續失敗次數。
/// 達門檻表示 backend 真的卡住（非單純初始 sync 等待）。
void incrementShipmentRetryCount(String orderNo) {
  _shipment5046Count[orderNo] = (_shipment5046Count[orderNo] ?? 0) + 1;
}

/// 訂單成功操作後呼叫，重置計數。
void resetShipmentRetryCount(String orderNo) {
  _shipment5046Count.remove(orderNo);
}

/// 是否已連續多次 5046 / 5047（疑似 backend reconcile 卡住）。
bool isShipmentBackendStuck(String orderNo) =>
    (_shipment5046Count[orderNo] ?? 0) >= _shipmentBackendStuckThreshold;

/// 在發 API 前檢查冷卻；命中則顯示 toast 並回 true（caller 應 return）。
bool guardShipmentCooldown(BuildContext context, String orderNo) {
  if (!isUnderShipmentCooldown(orderNo)) return false;
  final remaining = shipmentCooldownRemainingSeconds(orderNo);
  if (isShipmentBackendStuck(orderNo)) {
    FloatingToast.show(
      context,
      '物流商通訊異常處理中，請稍後（數分鐘）再試或聯絡客服',
      icon: AppIcons.warningAmberOutlined,
    );
  } else {
    FloatingToast.show(
      context,
      '物流單剛建立，請等 $remaining 秒後再試',
      icon: AppIcons.accessTime,
    );
  }
  return true;
}
