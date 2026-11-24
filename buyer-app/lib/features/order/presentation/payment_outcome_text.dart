import '../../../l10n/l10n.dart';
import '../data/payment_flow.dart';

/// 付款流程結果的使用者面文字。
String paymentOutcomeMessage(AppLocalizations l10n, PaymentOutcome outcome) =>
    switch (outcome) {
      PaymentOutcome.succeeded => l10n.paymentSucceededMessage,
      PaymentOutcome.pending => l10n.paymentPendingMessage,
      PaymentOutcome.canceled => l10n.paymentCanceledMessage,
      PaymentOutcome.failed => l10n.paymentFailedMessage,
      PaymentOutcome.anomaly => l10n.paymentAnomalyMessage,
    };
