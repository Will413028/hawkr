import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../exceptions/app_exception.dart';
import '../../shared/widgets/common_snackbar.dart';

extension AsyncValueUiX<T> on AsyncValue<T> {
  void showErrorSnackbar(BuildContext context) {
    if (this case AsyncError(:final error)) {
      final msg = error is AppException ? error.message : error.toString();
      showFloatingToast(context, msg, isSuccess: false);
    }
  }
}
