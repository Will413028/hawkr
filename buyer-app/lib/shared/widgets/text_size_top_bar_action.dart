import 'dart:async';

import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../core/layout/app_text_size.dart';
import '../../core/theme/hk_tokens.dart';
import '../icons/app_icon_colors.dart';
import '../widgets/hk_icon.dart';
import 'hk_tap_target.dart';
import 'text_size_settings_sheet.dart';

/// Compact header shortcut for the shared Small / Standard / Large preference.
class TextSizeTopBarAction extends ConsumerWidget {
  const TextSizeTopBarAction({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final selected = ref.watch(appTextSizeProvider);
    return SizedBox.square(
      key: const ValueKey('topbar-text-size-action'),
      dimension: HkControl.minTap,
      child: HkTapTarget(
        semanticLabel: '字體大小，目前${selected.label}',
        hint: '可切換小、中、大字體',
        tooltip: '字體大小：${selected.label}',
        onTap: () => unawaited(showTextSizeSettingsSheet(context)),
        child: HkIcon(
          HkIconName.textSize,
          key: const ValueKey('topbar-text-size-icon'),
          size: HkControl.icon,
          color: AppIconColors.strong,
        ),
      ),
    );
  }
}
