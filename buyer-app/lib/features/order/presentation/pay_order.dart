import 'package:flutter/widgets.dart';
import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../l10n/l10n.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../data/payment_flow.dart';
import 'payment_outcome_text.dart';

/// 對一張待付款訂單跑一次付款（PaymentSheet），結果以 toast 告知。回傳結果；呼叫（網路或伺服器）失敗回 null。
Future<PaymentOutcome?> payOrder(
  BuildContext context,
  WidgetRef ref,
  String orderNo,
) async {
  try {
    final outcome = await ref.read(paymentFlowProvider).pay(orderNo);
    if (context.mounted) {
      showFloatingToast(
        context,
        paymentOutcomeMessage(context.l10n, outcome),
        isSuccess: outcome == PaymentOutcome.succeeded,
      );
    }
    return outcome;
  } catch (e) {
    if (context.mounted) {
      showFloatingToast(
        context,
        errorText(e, defaults: ref.read(defaultErrorTextProvider)),
        isSuccess: false,
      );
    }
    return null;
  }
}
