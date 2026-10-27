import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';

import 'hk_button_label.dart';
import 'hk_glass_chrome_surface.dart';

Future<bool> showAppConfirmDialog(
  BuildContext context, {
  required String title,
  String? message,
  String cancelText = '取消',
  String confirmText = '確認',
  Color confirmTextColor = AppColors.textEmphasis,
  Color barrierColor = AppColors.bgOverlay,
  bool barrierDismissible = false,
}) async {
  final confirmed = await showDialog<bool>(
    context: context,
    barrierDismissible: barrierDismissible,
    barrierColor: barrierColor,
    builder: (ctx) => Dialog(
      backgroundColor: Colors.transparent,
      insetPadding: const EdgeInsets.symmetric(horizontal: 24),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 360),
        child: HkGlassChromeSurface(
          borderRadius: BorderRadius.circular(24),
          showBottomBorder: false,
          child: Material(
            color: Colors.transparent,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Padding(
                  padding: const EdgeInsets.all(24),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        title,
                        textAlign: TextAlign.center,
                        style: const TextStyle(
                          fontSize: 20,
                          color: AppColors.textEmphasis,
                          height: 1.35,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      if (message?.trim().isNotEmpty ?? false) ...[
                        const SizedBox(height: 10),
                        Text(
                          message!,
                          textAlign: TextAlign.center,
                          style: const TextStyle(
                            fontSize: 16,
                            color: AppColors.textPrimary,
                            height: 1.5,
                          ),
                        ),
                      ],
                    ],
                  ),
                ),
                const Divider(
                  height: 1,
                  thickness: 1,
                  color: AppColors.borderSoft,
                ),
                Row(
                  children: [
                    Expanded(
                      child: Semantics(
                        button: true,
                        label: cancelText,
                        excludeSemantics: true,
                        child: InkWell(
                          onTap: () => Navigator.of(ctx).pop(false),
                          child: SizedBox(
                            height: 54,
                            child: Center(
                              child: HkButtonLabel(
                                cancelText,
                                style: const TextStyle(
                                  fontSize: 16,
                                  fontWeight: FontWeight.w700,
                                  color: AppColors.textPrimary,
                                ),
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                    const SizedBox(
                      width: 1,
                      height: 54,
                      child: ColoredBox(color: AppColors.borderSoft),
                    ),
                    Expanded(
                      child: Semantics(
                        button: true,
                        label: confirmText,
                        excludeSemantics: true,
                        child: InkWell(
                          onTap: () => Navigator.of(ctx).pop(true),
                          child: SizedBox(
                            height: 54,
                            child: Center(
                              child: HkButtonLabel(
                                confirmText,
                                style: TextStyle(
                                  fontSize: 16,
                                  fontWeight: FontWeight.w700,
                                  color: confirmTextColor,
                                ),
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    ),
  );

  return confirmed ?? false;
}
