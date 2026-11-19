import 'package:flutter/material.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../data/post_publish.dart';

/// 店家沒有 MARKETING 能力時，貼文列表上方的說明。[includeStatusBarInset] 為 true 時
/// 背景延伸到狀態列下（notice 貼在螢幕頂端時用）。
class PostRestrictedNotice extends StatelessWidget {
  const PostRestrictedNotice({super.key, this.includeStatusBarInset = false});

  final bool includeStatusBarInset;

  @override
  Widget build(BuildContext context) => Container(
    key: const ValueKey('seller-posts-restricted-notice'),
    width: double.infinity,
    color: AppColors.warningSoft,
    padding: EdgeInsets.fromLTRB(
      HkSpace.lg,
      (includeStatusBarInset ? MediaQuery.paddingOf(context).top : 0) +
          HkSpace.sm,
      HkSpace.lg,
      HkSpace.sm,
    ),
    child: const Text(
      kPostRestrictedNotice,
      style: TextStyle(color: AppColors.warningText),
    ),
  );
}
