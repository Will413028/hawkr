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
      // 物流單不存在 — 後端 DB 沒對應物流記錄，是永久狀態（重試沒用）。
      // 通常發生在：訂單從未建立物流單、或訂單過老物流記錄被 GC 清掉。
      FloatingToast.error(context, '此筆訂單異常（找不到物流資料），請聯絡客服處理');
      return true;
    case 5046:
      // 物流單建立未滿 30 秒，PayUni 後端尚未同步完
      // 把 cooldown 也記在前端，下次按按鈕會被 guard 擋住並顯示剩餘秒數
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
      // 物流商呼叫進行中（後端跟 PayUni 通訊中），背景會自動補上代碼
      // 跟 5046 一樣加進 cooldown，避免賣家在 PayUni 回應前反覆點按
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
      // 5048 帶業務情境訊息，後端已寫成可顯示文字
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
    // 後端常見格式 "[5xxx] 訊息" → 去掉前綴
    msg = msg.replaceFirst(RegExp(r'^\[\d+\]\s*'), '');
    // 工程備註括號（如「（PayUni 後端需時間同步資料）」）
    msg = msg.replaceAll(RegExp(r'（[^）]*）'), '').trim();
    return msg.isEmpty ? '操作失敗，請稍後再試' : msg;
  }
  return '操作失敗，請稍後再試';
}

// ─────────────────────────────────────────────────────────────────
// 5046 / 5047 防護 — client-side cooldown + 連續失敗偵測
//
// 正常流程：物流單建立後 30 秒內後端會回 5046，client 先擋一層避免無謂 API
// 呼叫。但若連續多次 5046（>=3 次）代表 backend reconcile 真的卡住（如
// B-143 的 LgsType bug），單純等 30 秒沒用，要提示用戶這是 backend 異常。
// ─────────────────────────────────────────────────────────────────

const _shipmentCooldownDuration = Duration(seconds: 30);
const _shipmentBackendStuckThreshold = 3; // 連 3 次 5046 視為 backend 卡住
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

// ─────────────────────────────────────────────────────────────────
// 本地 odno 暫存（B-147 workaround）
//
// print_cvs_label API 成功會直接回 odno + validationNo，但緊接著的
// GET /app/orders list endpoint 沒同步那兩個欄位（後端 read replica 或
// cache lag），導致卡片按鈕仍判斷 odno 為空 → 顯示「預約取號」。
//
// workaround：把 print_cvs_label 回的 odno+validationNo 暫存在 client，
// 按鈕邏輯優先看這份 override 再 fallback 到 order.odno。
// 等 B-147 後端修好後可以拿掉這段。
// ─────────────────────────────────────────────────────────────────

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
  // 多次 5046 後改提示 backend 異常，避免用戶以為等 30 秒就行
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
