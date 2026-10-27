import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:flutter/material.dart';

import '../../../../core/l10n/ui_locale.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../data/payuni.dart';

/// PayUni 審核意見是第三方原文：不翻譯，容器標 zh-TW（i18n ADR）。
const payuniReasonLocale = Locale.fromSubtags(
  languageCode: 'zh',
  scriptCode: 'Hant',
  countryCode: 'TW',
);

/// 賣家看到的 PayUni 狀態的內文（錢包面板、認證頁鎖住畫面與問題框共用）：伺服器的狀態說明、PayUni 的審核意見（有的話）、
/// 要修正的欄位清單（有的話）。標題由各畫面自己放（[payuniStateTitle]）。
class PayuniStatusBody extends StatelessWidget {
  const PayuniStatusBody({
    required this.payuni,
    this.textAlign = TextAlign.start,
    super.key,
  });

  final SellerPayUni payuni;
  final TextAlign textAlign;

  @override
  Widget build(BuildContext context) {
    final UiLocale l = context.uiLocale;
    final secondary = AppTypography.body.copyWith(
      color: AppColors.textSecondary,
    );
    final reason = payuni.reason;
    final fields = payuniFixFieldLabels(payuni, l);
    final cross = textAlign == TextAlign.center
        ? CrossAxisAlignment.center
        : CrossAxisAlignment.start;
    return Column(
      crossAxisAlignment: cross,
      children: [
        if (payuni.hasExplanation() && payuni.explanation.message.isNotEmpty)
          Text(
            payuni.explanation.message,
            key: const ValueKey('payuni-explanation'),
            textAlign: textAlign,
            style: secondary,
          ),
        if (reason.isNotEmpty) ...[
          const SizedBox(height: HkSpace.sm),
          Text(
            sellerSellerPayUniReasonHeading(l),
            textAlign: textAlign,
            style: AppTypography.bodyBold,
          ),
          Text(
            reason,
            key: const ValueKey('payuni-reason'),
            locale: payuniReasonLocale,
            textAlign: textAlign,
            style: secondary,
          ),
        ],
        if (fields.isNotEmpty) ...[
          const SizedBox(height: HkSpace.sm),
          Text(
            sellerSellerPayUniFixFieldsHeading(l),
            textAlign: textAlign,
            style: AppTypography.bodyBold,
          ),
          for (final (i, label) in fields.indexed)
            Text(
              '・$label',
              key: ValueKey('payuni-fix-field-$i'),
              textAlign: textAlign,
              style: secondary,
            ),
        ],
      ],
    );
  }
}
