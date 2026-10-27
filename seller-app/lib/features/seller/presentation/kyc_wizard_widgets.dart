import 'package:hawkr_twaddr/hawkr_twaddr.dart' as twaddr;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/theme/app_colors.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/icons/app_icons.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/app_date_picker.dart';
import '../../../shared/widgets/location_icon.dart';
import '../../media/presentation/protected_media_controller.dart';
import '../data/kyc_form.dart';

TextStyle _labelStyle() => TextStyle(
  fontSize: 14.sp,
  fontWeight: FontWeight.w500,
  color: AppColors.textEmphasis,
);

OutlineInputBorder _border(Color color) => OutlineInputBorder(
  borderRadius: BorderRadius.circular(12.r),
  borderSide: BorderSide(color: color),
);

/// 欄位的標籤、欄位本體與下方的說明或錯誤。
class KycFieldFrame extends StatelessWidget {
  const KycFieldFrame({
    super.key,
    required this.label,
    required this.child,
    this.error,
    this.helper,
  });

  final String label;
  final Widget child;
  final String? error;
  final String? helper;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Align(
          alignment: Alignment.centerLeft,
          child: Text(label, style: _labelStyle()),
        ),
        SizedBox(height: 6.h),
        child,
        if (error != null)
          Padding(
            padding: EdgeInsets.only(top: 6.h),
            child: Text(
              error!,
              style: TextStyle(fontSize: 12.sp, color: AppColors.dangerStrong),
            ),
          )
        else if (helper != null)
          Padding(
            padding: EdgeInsets.only(top: 6.h),
            child: Text(
              helper!,
              style: TextStyle(fontSize: 12.sp, color: AppColors.textSecondary),
            ),
          ),
      ],
    );
  }
}

/// 以外部的 [value] 為準的文字欄位：值被外部改掉（OCR、統編查詢、版本衝突）時畫面跟著換，使用者輸入時游標不跳。
class KycTextField extends StatefulWidget {
  const KycTextField({
    super.key,
    required this.label,
    required this.value,
    required this.onChanged,
    this.hint,
    this.helper,
    this.error,
    this.keyboardType,
    this.inputFormatters,
    this.readOnly = false,
    this.suffix,
  });

  final String label;
  final String value;
  final ValueChanged<String> onChanged;
  final String? hint;
  final String? helper;
  final String? error;
  final TextInputType? keyboardType;
  final List<TextInputFormatter>? inputFormatters;
  final bool readOnly;
  final Widget? suffix;

  @override
  State<KycTextField> createState() => _KycTextFieldState();
}

class _KycTextFieldState extends State<KycTextField> {
  late final TextEditingController _controller = TextEditingController(
    text: widget.value,
  );

  @override
  void didUpdateWidget(KycTextField oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.value != _controller.text) {
      _controller.value = TextEditingValue(
        text: widget.value,
        selection: TextSelection.collapsed(offset: widget.value.length),
      );
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final hasError = widget.error != null;
    final borderColor = hasError
        ? AppColors.dangerStrong
        : AppColors.dividerLight;
    return KycFieldFrame(
      label: widget.label,
      error: widget.error,
      helper: widget.helper,
      child: ConstrainedBox(
        constraints: const BoxConstraints(minHeight: HkSpace.control),
        child: TextField(
          controller: _controller,
          readOnly: widget.readOnly,
          keyboardType: widget.keyboardType,
          inputFormatters: widget.inputFormatters,
          style: TextStyle(
            fontSize: 16.sp,
            color: widget.readOnly
                ? AppColors.textSecondary
                : AppColors.textEmphasis,
          ),
          decoration: InputDecoration(
            hintText: widget.hint,
            hintStyle: TextStyle(
              color: AppColors.textSecondary,
              fontSize: 14.sp,
            ),
            filled: true,
            fillColor: widget.readOnly
                ? AppColors.gray100
                : AppColors.bgCard.withValues(alpha: 0.8),
            contentPadding: EdgeInsets.symmetric(
              horizontal: 12.w,
              vertical: 12.h,
            ),
            suffixIcon: widget.suffix,
            suffixIconConstraints: BoxConstraints(
              minWidth: 32.w,
              minHeight: 24.h,
            ),
            border: _border(borderColor),
            enabledBorder: _border(borderColor),
            focusedBorder: _border(
              hasError ? AppColors.dangerStrong : AppColors.primary,
            ),
          ),
          onChanged: widget.onChanged,
        ),
      ),
    );
  }
}

/// 點一下開底部選單的欄位。
class KycPickerField extends StatelessWidget {
  const KycPickerField({
    super.key,
    required this.label,
    required this.value,
    required this.hint,
    required this.options,
    required this.onChanged,
    this.error,
    this.readOnly = false,
  });

  final String label;
  final String value;
  final String hint;
  final List<String> options;
  final ValueChanged<String> onChanged;
  final String? error;
  final bool readOnly;

  /// 目前的值不在選項裡（伺服器上的舊值）時也要列出來，不讓它在畫面上消失。
  List<String> get _shown =>
      value.isEmpty || options.contains(value) ? options : [value, ...options];

  Future<void> _open(BuildContext context) async {
    final picked = await showModalBottomSheet<String>(
      context: context,
      backgroundColor: AppColors.bgCard,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(24.r)),
      ),
      builder: (context) => SafeArea(
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Padding(
                padding: EdgeInsets.all(16.w),
                child: Text(
                  label,
                  style: TextStyle(
                    fontSize: 18.sp,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
              for (final option in _shown)
                ListTile(
                  title: Text(option, textAlign: TextAlign.center),
                  trailing: value == option
                      ? const Icon(
                          AppIcons.check,
                          color: AppIconColors.secondary,
                        )
                      : null,
                  onTap: () => Navigator.pop(context, option),
                ),
              SizedBox(height: 16.h),
            ],
          ),
        ),
      ),
    );
    if (picked != null) onChanged(picked);
  }

  @override
  Widget build(BuildContext context) {
    return KycFieldFrame(
      label: label,
      error: error,
      child: GestureDetector(
        onTap: readOnly ? null : () => _open(context),
        child: _FieldBox(
          hasError: error != null,
          readOnly: readOnly,
          child: Row(
            children: [
              Expanded(
                child: Text(
                  value.isEmpty ? hint : value,
                  style: TextStyle(
                    fontSize: 16.sp,
                    color: value.isEmpty
                        ? AppColors.textSecondary
                        : AppColors.textEmphasis,
                  ),
                ),
              ),
              const Icon(AppIcons.chevronRight, color: AppIconColors.muted),
            ],
          ),
        ),
      ),
    );
  }
}

class _FieldBox extends StatelessWidget {
  const _FieldBox({
    required this.child,
    this.hasError = false,
    this.readOnly = false,
  });

  final Widget child;
  final bool hasError;
  final bool readOnly;

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: const BoxConstraints(minHeight: HkSpace.control),
      padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 8.h),
      decoration: BoxDecoration(
        color: readOnly
            ? AppColors.gray100
            : AppColors.bgCard.withValues(alpha: 0.8),
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(
          color: hasError ? AppColors.dangerStrong : AppColors.dividerLight,
        ),
      ),
      child: child,
    );
  }
}

/// 日期欄位：表單存 `YYYY-MM-DD`，畫面顯示民國年；點一下開日期選擇器。
class KycDateField extends StatelessWidget {
  const KycDateField({
    super.key,
    required this.label,
    required this.value,
    required this.onChanged,
    this.hint = '請選擇日期',
    this.error,
    this.readOnly = false,
  });

  final String label;
  final String value;
  final ValueChanged<String> onChanged;
  final String hint;
  final String? error;
  final bool readOnly;

  Future<void> _pick(BuildContext context) async {
    final current = DateTime.tryParse(value);
    final picked = await AppDatePicker.show(
      context,
      initial: current ?? DateTime.now(),
      firstDate: DateTime(1911),
      lastDate: DateTime.now(),
    );
    if (picked != null) onChanged(isoOfDateTime(picked));
  }

  @override
  Widget build(BuildContext context) {
    final shown = rocDateOfIso(value);
    return KycFieldFrame(
      label: label,
      error: error,
      child: GestureDetector(
        onTap: readOnly ? null : () => _pick(context),
        child: _FieldBox(
          hasError: error != null,
          readOnly: readOnly,
          child: Row(
            children: [
              Expanded(
                child: Text(
                  shown.isEmpty ? hint : shown,
                  style: TextStyle(
                    fontSize: 16.sp,
                    color: shown.isEmpty
                        ? AppColors.textSecondary
                        : AppColors.textEmphasis,
                  ),
                ),
              ),
              Icon(
                AppIcons.calendarToday,
                size: 18.sp,
                color: AppIconColors.muted,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// 顯示「縣市 鄉鎮區 地址明細」的地址欄位，點一下由呼叫端開地址對話框。
class KycAddressField extends StatelessWidget {
  const KycAddressField({
    super.key,
    required this.label,
    required this.townCode,
    required this.detail,
    required this.onTap,
    this.error,
    this.helper,
  });

  final String label;
  final String townCode;
  final String detail;
  final VoidCallback onTap;
  final String? error;
  final String? helper;

  @override
  Widget build(BuildContext context) {
    final text = twaddr.formatAddress(townCode, detail.trim());
    return KycFieldFrame(
      label: label,
      error: error,
      helper: helper,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12.r),
        child: _FieldBox(
          hasError: error != null,
          child: Row(
            children: [
              HawkrLocationIcon(size: 16.sp, color: AppIconColors.muted),
              SizedBox(width: 8.w),
              Expanded(
                child: Text(
                  text.isEmpty ? '請選擇縣市 / 鄉鎮區 / 詳細地址' : text,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 14.sp,
                    color: text.isEmpty
                        ? AppColors.textSecondary
                        : AppColors.textEmphasis,
                  ),
                ),
              ),
              Icon(
                AppIcons.chevronRight,
                size: 20.sp,
                color: AppIconColors.muted,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// 步驟底部的主按鈕。
class KycPrimaryButton extends StatelessWidget {
  const KycPrimaryButton({
    super.key,
    required this.label,
    required this.onTap,
    this.busy = false,
  });

  final String label;
  final VoidCallback? onTap;
  final bool busy;

  @override
  Widget build(BuildContext context) {
    final enabled = onTap != null && !busy;
    final reduceMotion =
        MediaQuery.maybeOf(context)?.disableAnimations ?? false;
    return Semantics(
      button: true,
      enabled: enabled,
      label: label,
      child: AnimatedContainer(
        duration: reduceMotion ? Duration.zero : HkMotion.duration,
        curve: HkMotion.ease,
        width: double.infinity,
        height: HkSpace.control,
        decoration: BoxDecoration(
          color: enabled ? AppColors.primary : AppColors.gray300,
          borderRadius: BorderRadius.circular(HkRadius.pill),
          boxShadow: enabled ? HkShadows.sm : null,
        ),
        child: Material(
          type: MaterialType.transparency,
          child: InkWell(
            onTap: enabled ? onTap : null,
            borderRadius: BorderRadius.circular(HkRadius.pill),
            child: Center(
              child: busy
                  ? SizedBox(
                      width: 20.w,
                      height: 20.w,
                      child: const CircularProgressIndicator(
                        strokeWidth: 2,
                        color: AppColors.bgCard,
                      ),
                    )
                  : Text(
                      label,
                      style: TextStyle(
                        fontSize: 16.sp,
                        fontWeight: FontWeight.w700,
                        color: AppColors.bgCard,
                      ),
                    ),
            ),
          ),
        ),
      ),
    );
  }
}

/// 摘要的一列：標籤與值，沒有值時顯示「未填寫」。
class KycInfoRow extends StatelessWidget {
  const KycInfoRow(this.label, this.value, {super.key});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.symmetric(vertical: 8.h),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(
            label,
            style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
          ),
          SizedBox(width: 12.w),
          Flexible(
            child: Text(
              value.isNotEmpty ? value : '未填寫',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: 14.sp,
                fontWeight: FontWeight.w500,
                color: AppColors.textEmphasis,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// 證件圖片：剛選的用本機位元組，申請上已有的經有授權的下載路徑讀取。
class KycDocImage extends ConsumerStatefulWidget {
  const KycDocImage({super.key, required this.doc, required this.label});

  final KycDoc doc;
  final String label;

  @override
  ConsumerState<KycDocImage> createState() => _KycDocImageState();
}

class _KycDocImageState extends ConsumerState<KycDocImage> {
  Future<Uint8List?>? _download;
  String _downloadFor = '';

  Future<Uint8List?> _load(String uri) async {
    try {
      return await ref
          .read(protectedMediaControllerProvider.notifier)
          .download(uri);
    } catch (error) {
      HKLog.logW('[KycDocImage] download failed: $error');
      return null;
    }
  }

  @override
  Widget build(BuildContext context) {
    final doc = widget.doc;
    final preview = doc.preview;
    if (preview != null) return _memory(preview);
    if (doc.downloadUri.isEmpty) return _broken();
    if (_downloadFor != doc.downloadUri) {
      _downloadFor = doc.downloadUri;
      _download = _load(doc.downloadUri);
    }
    return FutureBuilder<Uint8List?>(
      future: _download,
      builder: (context, snapshot) {
        if (snapshot.connectionState != ConnectionState.done) {
          return const Center(child: CircularProgressIndicator(strokeWidth: 2));
        }
        final bytes = snapshot.data;
        return bytes == null ? _broken() : _memory(bytes);
      },
    );
  }

  Widget _memory(Uint8List bytes) => Image.memory(
    bytes,
    fit: BoxFit.contain,
    width: double.infinity,
    height: double.infinity,
    errorBuilder: (_, _, _) => _broken(),
  );

  Widget _broken() => Container(
    width: double.infinity,
    height: double.infinity,
    decoration: BoxDecoration(
      color: AppColors.dangerSoft,
      borderRadius: BorderRadius.circular(16.r),
    ),
    child: Column(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        Icon(
          AppIcons.brokenImageOutlined,
          size: 32.sp,
          color: AppIconColors.warningText,
        ),
        SizedBox(height: 8.h),
        Text(
          '請重新上傳${widget.label}',
          style: TextStyle(fontSize: 12.sp, color: AppColors.warningText),
        ),
      ],
    ),
  );
}

/// 點一下選圖上傳的證件框。
class KycUploadBox extends StatelessWidget {
  const KycUploadBox({
    super.key,
    required this.label,
    required this.doc,
    required this.onTap,
    this.uploading = false,
    this.height,
    this.onRemove,
    this.error,
  });

  final String label;
  final KycDoc? doc;
  final VoidCallback onTap;
  final bool uploading;
  final double? height;
  final VoidCallback? onRemove;

  /// 這張文件的待處理退回問題；有值時邊框標紅並在下方顯示說明。
  final String? error;

  @override
  Widget build(BuildContext context) {
    final shown = doc;
    final box = GestureDetector(
      onTap: uploading ? null : onTap,
      child: Container(
        width: double.infinity,
        height: height ?? 120.h,
        decoration: BoxDecoration(
          color: AppColors.bgCard.withValues(alpha: 0.6),
          borderRadius: BorderRadius.circular(16.r),
          border: Border.all(
            color: error != null
                ? AppColors.dangerStrong
                : AppColors.dividerLight,
          ),
        ),
        child: Stack(
          children: [
            if (shown != null)
              ClipRRect(
                borderRadius: BorderRadius.circular(16.r),
                child: KycDocImage(doc: shown, label: label),
              )
            else
              Center(
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Icon(AppIcons.add, size: 32.sp, color: AppIconColors.muted),
                    SizedBox(height: 8.h),
                    Text(
                      label,
                      style: TextStyle(
                        fontSize: 14.sp,
                        color: AppColors.textSecondary,
                      ),
                    ),
                  ],
                ),
              ),
            if (uploading)
              Positioned.fill(
                child: Container(
                  decoration: BoxDecoration(
                    color: AppColors.textEmphasis.withValues(alpha: 0.3),
                    borderRadius: BorderRadius.circular(16.r),
                  ),
                  child: const Center(
                    child: CircularProgressIndicator(
                      valueColor: AlwaysStoppedAnimation<Color>(
                        AppColors.bgCard,
                      ),
                      strokeWidth: 2,
                    ),
                  ),
                ),
              )
            else if (shown != null && onRemove != null)
              Positioned(
                top: 4.h,
                right: 4.w,
                child: GestureDetector(
                  onTap: onRemove,
                  child: Container(
                    padding: EdgeInsets.all(4.w),
                    decoration: BoxDecoration(
                      color: AppColors.textSecondary,
                      borderRadius: BorderRadius.circular(12.r),
                    ),
                    child: Icon(
                      AppIcons.close,
                      size: 14.sp,
                      color: AppIconColors.onBrand,
                    ),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
    final message = error;
    if (message == null) return box;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        box,
        Padding(
          padding: EdgeInsets.only(top: 6.h),
          child: Text(
            message,
            style: TextStyle(fontSize: 12.sp, color: AppColors.dangerStrong),
          ),
        ),
      ],
    );
  }
}

/// 英文字母轉大寫的輸入格式。
class UpperCaseTextFormatter extends TextInputFormatter {
  @override
  TextEditingValue formatEditUpdate(
    TextEditingValue oldValue,
    TextEditingValue newValue,
  ) => newValue.copyWith(text: newValue.text.toUpperCase());
}
