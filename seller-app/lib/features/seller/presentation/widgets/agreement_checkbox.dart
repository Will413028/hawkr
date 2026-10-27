import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

/// 「［前文］〈條款名稱〉［後文］」加勾選方塊：條款名稱開啟條款網址（伺服器回的絕對網址）。條款還沒讀到
/// （[url] 是 null）時不能勾選，讀取失敗時顯示重試。
class AgreementCheckbox extends StatefulWidget {
  const AgreementCheckbox({
    super.key,
    required this.checkboxKey,
    required this.before,
    required this.linkText,
    required this.after,
    required this.url,
    required this.loadFailed,
    required this.checked,
    required this.enabled,
    required this.onChanged,
    required this.onRetry,
  });

  /// 勾選方塊的 key（測試與無障礙用）。
  final Key checkboxKey;
  final String before;
  final String linkText;
  final String after;
  final String? url;
  final bool loadFailed;
  final bool checked;
  final bool enabled;
  final ValueChanged<bool> onChanged;
  final VoidCallback onRetry;

  @override
  State<AgreementCheckbox> createState() => _AgreementCheckboxState();
}

class _AgreementCheckboxState extends State<AgreementCheckbox> {
  late final TapGestureRecognizer _open = TapGestureRecognizer()
    ..onTap = _openAgreement;

  @override
  void dispose() {
    _open.dispose();
    super.dispose();
  }

  Future<void> _openAgreement() async {
    final url = widget.url;
    if (url == null) return;
    final uri = Uri.tryParse(url);
    if (uri == null) return;
    await launchUrl(uri, mode: LaunchMode.externalApplication);
  }

  @override
  Widget build(BuildContext context) {
    final ready = widget.url != null;
    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        Checkbox(
          key: widget.checkboxKey,
          value: widget.checked,
          onChanged: ready && widget.enabled
              ? (value) => widget.onChanged(value ?? false)
              : null,
        ),
        Expanded(
          child: Text.rich(
            TextSpan(
              style: const TextStyle(
                color: AppColors.textPrimary,
                fontSize: 14,
                fontWeight: FontWeight.w700,
                height: 1.45,
              ),
              children: [
                TextSpan(text: widget.before),
                TextSpan(
                  text: widget.linkText,
                  recognizer: _open,
                  style: const TextStyle(
                    color: AppColors.actionPrimary,
                    decoration: TextDecoration.underline,
                  ),
                ),
                TextSpan(text: widget.after),
              ],
            ),
          ),
        ),
        if (widget.loadFailed)
          TextButton(
            onPressed: widget.onRetry,
            child: const HkButtonLabel('重試'),
          ),
      ],
    );
  }
}
