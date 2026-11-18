import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../core/session/session_model.dart';
import '../../core/session/session_provider.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_typography.dart';
import '../../core/theme/hk_tokens.dart';
import '../utils/require_login.dart';

/// 訪客（session 已解析且沒有可用登入）才為 true。session 還在載入時不是訪客，避免已登入者閃一下登入提示；
/// 登入但不能留言（例如被封鎖）也不是訪客。
bool isGuestSession(AsyncValue<SessionModel?> session) =>
    session.hasValue && !hasUsableSession(session.value);

/// 訪客的留言框替代品：點了走既有的登入閘門 [requireLogin]。
class LoginToCommentPrompt extends ConsumerWidget {
  const LoginToCommentPrompt({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return SafeArea(
      top: false,
      child: Semantics(
        button: true,
        label: '登入後留言',
        excludeSemantics: true,
        child: InkWell(
          key: const ValueKey('login-to-comment-prompt'),
          onTap: () => requireLogin(context, ref),
          child: Container(
            constraints: const BoxConstraints(minHeight: 48),
            alignment: Alignment.center,
            padding: const EdgeInsets.symmetric(
              horizontal: HkSpace.md,
              vertical: HkSpace.sm,
            ),
            child: Text(
              '登入後留言',
              style: AppTypography.body.copyWith(color: AppColors.primary),
            ),
          ),
        ),
      ),
    );
  }
}
