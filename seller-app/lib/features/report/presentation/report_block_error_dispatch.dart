import 'package:flutter/material.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/exceptions/report_block_codes.dart';
import '../../../shared/utils/floating_toast.dart';

/// 處理檢舉／封鎖相關 API 業務碼（backend-payuni#645）。
///
/// 命中已顯示 toast 並回 true；未命中回 false 讓 caller fallback 顯示自己的
/// 錯誤文案。走 call-site 層處理（非全域 interceptor），比照
/// shipment_error_dispatch.dart 的模式。
bool handleReportBlockApiError(BuildContext context, Object e) {
  if (e is! ApiException) return false;
  switch (e.code) {
    case ReportBlockCode.blockedMessageNotAllowed:
      FloatingToast.error(context, '對方已將你封鎖，無法傳送訊息');
      return true;
    case ReportBlockCode.moderationRejected:
      final msg = e.message.trim();
      FloatingToast.error(context, msg.isNotEmpty ? msg : '內容未通過審核，請調整後再試');
      return true;
    default:
      return false;
  }
}
