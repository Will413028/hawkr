import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../shared/utils/floating_toast.dart';
import '../exceptions/app_exception.dart';

/// AsyncValue UI 擴展
///
/// 提供便捷方法在 AsyncError 時顯示 FloatingToast。
/// 用法：
/// ```dart
/// ref.listen(someProvider, (_, next) {
///   next.showSnackBarOnError(context);
/// });
/// ```
extension AsyncValueUI on AsyncValue<dynamic> {
  /// 當狀態為 AsyncError 時，自動顯示錯誤 FloatingToast
  void showSnackBarOnError(BuildContext context) {
    if (!isLoading && hasError) {
      final errorMessage = _extractErrorMessage(error);
      FloatingToast.error(context, errorMessage);
    }
  }

  /// 從 error 物件提取可讀訊息
  String _extractErrorMessage(Object? error) {
    if (error is AppException) {
      return error.message;
    }
    if (error is Exception) {
      return error.toString().replaceFirst('Exception: ', '');
    }
    return '發生未知錯誤，請稍後再試';
  }
}
