import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../data/after_sale_model.dart';

class ConfirmReturnReceiptData {
  const ConfirmReturnReceiptData({
    required this.actualRefundAmount,
    this.deductionReason,
    this.inspectionNote,
    this.inspectionPhotoTokens = const [],
  });

  final String actualRefundAmount;
  final String? deductionReason;
  final String? inspectionNote;
  final List<String> inspectionPhotoTokens;
}

Future<void> showAfterSaleConfirmReceiptDialog({
  required BuildContext context,
  required AfterSaleCase afterSaleCase,
  required Future<void> Function(ConfirmReturnReceiptData data) onSubmit,
}) {
  return showDialog<void>(
    context: context,
    barrierColor: AppColors.gray1.withValues(alpha: 0.72),
    builder: (_) =>
        _ConfirmReceiptDialog(afterSaleCase: afterSaleCase, onSubmit: onSubmit),
  );
}

class _ConfirmReceiptDialog extends StatefulWidget {
  const _ConfirmReceiptDialog({
    required this.afterSaleCase,
    required this.onSubmit,
  });

  final AfterSaleCase afterSaleCase;
  final Future<void> Function(ConfirmReturnReceiptData data) onSubmit;

  @override
  State<_ConfirmReceiptDialog> createState() => _ConfirmReceiptDialogState();
}

class _ConfirmReceiptDialogState extends State<_ConfirmReceiptDialog> {
  final _deductionCtrl = TextEditingController(text: '0');
  final _deductionReasonCtrl = TextEditingController();
  bool _appearanceOk = true;
  bool _matchesDescription = true;
  bool _packageOk = false;
  bool _submitting = false;

  int get _maxRefund =>
      int.tryParse(widget.afterSaleCase.refundableAmount.trim()) ?? 0;

  int get _deduction {
    final parsed = int.tryParse(_deductionCtrl.text.trim()) ?? 0;
    return parsed.clamp(0, _maxRefund).toInt();
  }

  int get _actualRefund => (_maxRefund - _deduction).clamp(0, _maxRefund);

  bool get _needsReason => _deduction > 0;

  bool get _canSubmit =>
      !_submitting &&
      _maxRefund > 0 &&
      _actualRefund > 0 &&
      (!_needsReason || _deductionReasonCtrl.text.trim().isNotEmpty);

  @override
  void initState() {
    super.initState();
    _deductionCtrl.addListener(_refresh);
    _deductionReasonCtrl.addListener(_refresh);
  }

  @override
  void dispose() {
    _deductionCtrl.removeListener(_refresh);
    _deductionReasonCtrl.removeListener(_refresh);
    _deductionCtrl.dispose();
    _deductionReasonCtrl.dispose();
    super.dispose();
  }

  void _refresh() {
    if (mounted) setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
      insetPadding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 32.h),
      backgroundColor: AppColors.bgCard,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16.r)),
      child: SingleChildScrollView(
        padding: EdgeInsets.fromLTRB(24.w, 24.h, 24.w, 20.h),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              '驗貨確認・退款執行',
              textAlign: TextAlign.center,
              style: AppTypography.sectionTitle.copyWith(
                fontSize: 20.sp,
                color: AppColors.textEmphasis,
              ),
            ),
            SizedBox(height: 8.h),
            Text(
              '確認退回商品狀況後，系統將依金額執行退款作業',
              textAlign: TextAlign.center,
              style: AppTypography.body.copyWith(
                color: AppColors.gray1_80,
                height: 1.45,
              ),
            ),
            SizedBox(height: 18.h),
            Text(
              '驗貨清單',
              style: AppTypography.productName.copyWith(
                color: AppTypography.colorEmphasis,
              ),
            ),
            SizedBox(height: 10.h),
            _CheckRow(
              checked: _appearanceOk,
              label: '商品外觀完整，無額外損壞',
              onTap: () => setState(() => _appearanceOk = !_appearanceOk),
            ),
            SizedBox(height: 10.h),
            _CheckRow(
              checked: _matchesDescription,
              label: '商品與買家描述瑕疵一致',
              onTap: () =>
                  setState(() => _matchesDescription = !_matchesDescription),
            ),
            SizedBox(height: 10.h),
            _CheckRow(
              checked: _packageOk,
              label: '包裝完好，配件齊全',
              muted: true,
              onTap: () => setState(() => _packageOk = !_packageOk),
            ),
            SizedBox(height: 18.h),
            _RefundPanel(
              maxRefund: _maxRefund,
              deductionCtrl: _deductionCtrl,
              actualRefund: _actualRefund,
            ),
            if (_needsReason) ...[
              SizedBox(height: 12.h),
              Text(
                '扣款原因',
                style: AppTypography.meta.copyWith(
                  color: AppColors.textSecondary,
                  fontWeight: FontWeight.w700,
                ),
              ),
              SizedBox(height: 6.h),
              TextField(
                controller: _deductionReasonCtrl,
                maxLines: 2,
                decoration: InputDecoration(
                  hintText: '例如：退回時包裝破損，需扣除處理費',
                  hintStyle: AppTypography.meta.copyWith(
                    color: AppColors.textHint,
                  ),
                  filled: true,
                  fillColor: AppColors.bgApp,
                  contentPadding: EdgeInsets.symmetric(
                    horizontal: 12.w,
                    vertical: 12.h,
                  ),
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(8.r),
                    borderSide: BorderSide.none,
                  ),
                ),
              ),
            ],
            SizedBox(height: 14.h),
            Row(
              children: const [
                Expanded(
                  child: _RefundInfoTile(title: '退款方式', value: '原路退回'),
                ),
                SizedBox(width: 8),
                Expanded(
                  child: _RefundInfoTile(title: '預計入帳', value: '1-3 個工作天'),
                ),
                SizedBox(width: 8),
                Expanded(
                  child: _RefundInfoTile(title: '退回至', value: '信用卡'),
                ),
              ],
            ),
            SizedBox(height: 18.h),
            Row(
              children: [
                Expanded(
                  child: _DialogButton(
                    label: '取消',
                    outlined: true,
                    onTap: _submitting
                        ? null
                        : () => Navigator.of(context).pop(),
                  ),
                ),
                SizedBox(width: 12.w),
                Expanded(
                  child: _DialogButton(
                    label: '確認退款 NT\$ $_actualRefund',
                    loading: _submitting,
                    onTap: _canSubmit ? _submit : null,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _submit() async {
    setState(() => _submitting = true);
    final checklist = <String>[
      if (_appearanceOk) '商品外觀完整',
      if (_matchesDescription) '與買家描述一致',
      if (_packageOk) '包裝配件齊全',
    ];
    final reason = _deductionReasonCtrl.text.trim();
    try {
      await widget.onSubmit(
        ConfirmReturnReceiptData(
          actualRefundAmount: _actualRefund.toString(),
          deductionReason: _needsReason ? reason : null,
          inspectionNote: checklist.isEmpty ? null : checklist.join('、'),
        ),
      );
      if (mounted) Navigator.of(context).pop();
    } catch (_) {
      // Parent callback is responsible for the toast; keep the dialog open.
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }
}

class _CheckRow extends StatelessWidget {
  const _CheckRow({
    required this.checked,
    required this.label,
    required this.onTap,
    this.muted = false,
  });

  final bool checked;
  final String label;
  final VoidCallback onTap;
  final bool muted;

  @override
  Widget build(BuildContext context) {
    final color = checked ? AppColors.primary : AppColors.gray1_20;
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(8.r),
      child: Row(
        children: [
          Container(
            width: 28.w,
            height: 28.w,
            decoration: BoxDecoration(
              color: color,
              borderRadius: BorderRadius.circular(8.r),
            ),
            alignment: Alignment.center,
            child: checked
                ? Icon(
                    AppIcons.check,
                    color: AppIconColors.onBrand,
                    size: 18.sp,
                  )
                : null,
          ),
          SizedBox(width: 12.w),
          Expanded(
            child: Text(
              label,
              style: AppTypography.bodyBold.copyWith(
                color: muted && !checked
                    ? AppColors.textDisabled
                    : AppColors.gray1,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _RefundPanel extends StatelessWidget {
  const _RefundPanel({
    required this.maxRefund,
    required this.deductionCtrl,
    required this.actualRefund,
  });

  final int maxRefund;
  final TextEditingController deductionCtrl;
  final int actualRefund;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: AppColors.neutralWhisper,
        borderRadius: BorderRadius.circular(12.r),
      ),
      child: Column(
        children: [
          _AmountRow(label: '原訂單金額', value: 'NT\$ $maxRefund', strong: true),
          SizedBox(height: 10.h),
          Row(
            children: [
              Expanded(
                child: Text(
                  '扣除金額',
                  style: AppTypography.body.copyWith(color: AppColors.gray1_80),
                ),
              ),
              Text(
                'NT\$',
                style: AppTypography.bodyBold.copyWith(
                  color: AppColors.gray1_50,
                ),
              ),
              SizedBox(width: 8.w),
              SizedBox(
                width: 74.w,
                height: 38.h,
                child: TextField(
                  controller: deductionCtrl,
                  keyboardType: TextInputType.number,
                  inputFormatters: [FilteringTextInputFormatter.digitsOnly],
                  textAlign: TextAlign.center,
                  style: AppTypography.bodyBold,
                  decoration: InputDecoration(
                    filled: true,
                    fillColor: AppColors.bgCard,
                    contentPadding: EdgeInsets.zero,
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(8.r),
                      borderSide: BorderSide.none,
                    ),
                  ),
                ),
              ),
            ],
          ),
          Padding(
            padding: EdgeInsets.symmetric(vertical: 12.h),
            child: Divider(color: AppColors.primary.withValues(alpha: 0.18)),
          ),
          _AmountRow(
            label: '實際退款金額',
            value: 'NT\$ $actualRefund',
            color: AppColors.primary,
            strong: true,
          ),
        ],
      ),
    );
  }
}

class _AmountRow extends StatelessWidget {
  const _AmountRow({
    required this.label,
    required this.value,
    this.color = AppColors.gray1,
    this.strong = false,
  });

  final String label;
  final String value;
  final Color color;
  final bool strong;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(
          label,
          style: AppTypography.body.copyWith(color: AppColors.gray1_80),
        ),
        Text(
          value,
          style: AppTypography.productName.copyWith(
            color: color,
            fontSize: strong ? 18.sp : 14.sp,
          ),
        ),
      ],
    );
  }
}

class _RefundInfoTile extends StatelessWidget {
  const _RefundInfoTile({required this.title, required this.value});

  final String title;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: BoxConstraints(minHeight: 66.h),
      padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
      decoration: BoxDecoration(
        color: AppColors.bgApp,
        borderRadius: BorderRadius.circular(8.r),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Text(
            title,
            style: AppTypography.meta.copyWith(color: AppColors.textHint),
          ),
          SizedBox(height: 4.h),
          Text(
            value,
            style: AppTypography.bodyBold,
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
          ),
        ],
      ),
    );
  }
}

class _DialogButton extends StatelessWidget {
  const _DialogButton({
    required this.label,
    this.outlined = false,
    this.loading = false,
    this.onTap,
  });

  final String label;
  final bool outlined;
  final bool loading;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final enabled = onTap != null;
    return SizedBox(
      height: 44.h,
      child: Material(
        color: outlined
            ? AppColors.bgCard
            : enabled
            ? AppColors.primary
            : AppColors.primary.withValues(alpha: 0.45),
        borderRadius: BorderRadius.circular(12.r),
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(12.r),
          child: Container(
            alignment: Alignment.center,
            decoration: outlined
                ? BoxDecoration(
                    borderRadius: BorderRadius.circular(12.r),
                    border: Border.all(color: AppColors.gray1_20),
                  )
                : null,
            child: loading
                ? SizedBox(
                    width: 18.sp,
                    height: 18.sp,
                    child: const CircularProgressIndicator(
                      strokeWidth: 2,
                      color: AppColors.bgCard,
                    ),
                  )
                : Text(
                    label,
                    textAlign: TextAlign.center,
                    style: AppTypography.bodyBold.copyWith(
                      color: outlined ? AppColors.gray1_50 : AppColors.bgCard,
                    ),
                  ),
          ),
        ),
      ),
    );
  }
}
