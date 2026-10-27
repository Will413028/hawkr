import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/config/image_config.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/widgets/app_cached_image.dart';
import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_buyer/shared/widgets/hk_placeholder.dart';

/// 沒有任何歷史對話時的主要入口。
class ChatEmptyState extends StatelessWidget {
  const ChatEmptyState({super.key, required this.onStartAssistant});

  final VoidCallback onStartAssistant;

  @override
  Widget build(BuildContext context) {
    final viewport = MediaQuery.sizeOf(context);
    final compact = viewport.width < 340 || viewport.height < 620;
    return Center(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 380),
        child: Padding(
          padding: const EdgeInsets.all(HkSpace.xxl),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const ExcludeSemantics(
                child: HkIcon(HkIconName.emptyChat, size: 64),
              ),
              SizedBox(height: compact ? HkSpace.lg : HkSpace.xxl),
              Text(
                '開始第一段對話',
                textAlign: TextAlign.center,
                style: AppTypography.h1,
              ),
              const SizedBox(height: HkSpace.sm),
              Text(
                '你可以從商品或商家頁聯絡商家，\n也可以先讓智慧客服協助你。',
                textAlign: TextAlign.center,
                style: AppTypography.body.copyWith(
                  color: AppColors.textSecondary,
                ),
              ),
              const SizedBox(height: HkSpace.xxl),
              Semantics(
                button: true,
                label: '詢問智慧客服',
                hint: '開啟智慧客服對話',
                onTap: onStartAssistant,
                child: ExcludeSemantics(
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(
                      minHeight: HkControl.minTap,
                    ),
                    child: FilledButton.icon(
                      key: const ValueKey('chat-empty-assistant-action'),
                      onPressed: onStartAssistant,
                      icon: const _AssistantIcon(size: 22),
                      label: const HkButtonLabel('詢問智慧客服'),
                    ),
                  ),
                ),
              ),
              const SizedBox(height: HkSpace.md),
              Text(
                '之後的商家對話都會集中顯示在這裡',
                textAlign: TextAlign.center,
                style: AppTypography.caption,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// 搜尋沒有命中任何商家或訊息時的回復入口。
class ChatSearchEmptyState extends StatelessWidget {
  const ChatSearchEmptyState({
    super.key,
    required this.query,
    required this.onClear,
  });

  final String query;
  final VoidCallback onClear;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 380),
        child: Padding(
          padding: const EdgeInsets.all(HkSpace.xxl),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const ExcludeSemantics(
                child: HkIcon(HkIconName.emptySearch, size: 64),
              ),
              const SizedBox(height: HkSpace.xl),
              Text(
                '找不到「$query」',
                textAlign: TextAlign.center,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: AppTypography.h1,
              ),
              const SizedBox(height: HkSpace.sm),
              Text(
                '試試商家名稱，或訊息中的其他關鍵字。',
                textAlign: TextAlign.center,
                style: AppTypography.body.copyWith(
                  color: AppColors.textSecondary,
                ),
              ),
              const SizedBox(height: HkSpace.xl),
              Semantics(
                button: true,
                label: '清除搜尋',
                onTap: onClear,
                child: ExcludeSemantics(
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(
                      minHeight: HkControl.minTap,
                    ),
                    child: OutlinedButton(
                      key: const ValueKey('chat-search-clear-action'),
                      onPressed: onClear,
                      child: const HkButtonLabel('清除搜尋'),
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// 初次載入失敗且沒有可保留對話時的完整錯誤狀態。
class ChatLoadErrorState extends StatelessWidget {
  const ChatLoadErrorState({super.key, required this.onRetry});

  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 380),
        child: Padding(
          padding: const EdgeInsets.all(HkSpace.xxl),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const ExcludeSemantics(
                child: DecoratedBox(
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    color: AppColors.dangerSoft,
                  ),
                  child: SizedBox.square(
                    dimension: 72,
                    child: Center(
                      child: HkIcon(
                        HkIconName.warning,
                        size: 32,
                        color: AppIconColors.danger,
                      ),
                    ),
                  ),
                ),
              ),
              const SizedBox(height: HkSpace.xl),
              Text('對話載入失敗', style: AppTypography.h1),
              const SizedBox(height: HkSpace.sm),
              Text(
                '請檢查網路連線後再試一次。',
                textAlign: TextAlign.center,
                style: AppTypography.body.copyWith(
                  color: AppColors.textSecondary,
                ),
              ),
              const SizedBox(height: HkSpace.xl),
              Semantics(
                button: true,
                label: '重新載入對話',
                onTap: onRetry,
                child: ExcludeSemantics(
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(
                      minHeight: HkControl.minTap,
                    ),
                    child: FilledButton.icon(
                      onPressed: onRetry,
                      icon: const HkIcon(
                        HkIconName.refresh,
                        size: 20,
                        color: AppIconColors.onBrand,
                      ),
                      label: const HkButtonLabel('重新載入'),
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// 已有資料時的非阻斷式錯誤提示。
class ChatErrorBanner extends StatelessWidget {
  const ChatErrorBanner({
    super.key,
    required this.message,
    required this.onRetry,
  });

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: AppColors.dangerSoft,
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: HkSpace.lg,
          vertical: HkSpace.xs,
        ),
        child: Row(
          children: [
            const HkIcon(
              HkIconName.warning,
              size: 20,
              color: AppIconColors.danger,
            ),
            const SizedBox(width: HkSpace.sm),
            Expanded(
              child: Text(
                message,
                style: AppTypography.meta.copyWith(color: AppColors.danger),
              ),
            ),
            TextButton(onPressed: onRetry, child: const HkButtonLabel('重試')),
          ],
        ),
      ),
    );
  }
}

/// 有歷史對話時保留的智慧客服快捷入口。
class ChatAssistantFab extends StatelessWidget {
  const ChatAssistantFab({super.key, required this.onPressed});

  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: '開啟智慧客服',
      onTap: onPressed,
      child: ExcludeSemantics(
        child: FloatingActionButton.extended(
          heroTag: 'chat-assistant-fab',
          onPressed: onPressed,
          tooltip: '智慧客服',
          backgroundColor: AppColors.actionPrimary,
          foregroundColor: AppColors.textInverse,
          elevation: 2,
          icon: const _AssistantIcon(size: 22),
          label: Text(
            '智慧客服',
            style: AppTypography.buttonLarge.copyWith(height: 1),
          ),
        ),
      ),
    );
  }
}

/// 單筆對話摘要卡片。
class ChatConversationTile extends StatelessWidget {
  const ChatConversationTile({
    super.key,
    required this.name,
    this.avatarUrl,
    required this.lastMessage,
    required this.date,
    this.unread = 0,
    this.onTap,
  });

  final String name;
  final String? avatarUrl;
  final String lastMessage;
  final String date;
  final int unread;
  final VoidCallback? onTap;

  String get _unreadText => unread > 99 ? '99+' : '$unread';

  Widget _buildAvatar() {
    final source = (avatarUrl ?? '').trim();
    const size = 52.0;
    final image = source.startsWith('http://') || source.startsWith('https://')
        ? AppCachedImage(
            imageUrl: source,
            scene: ImageScene.chatAvatar,
            fit: BoxFit.cover,
            width: size,
            height: size,
          )
        : const SizedBox(
            width: size,
            height: size,
            child: HkPlaceholder(HkPlaceholderKind.merchantAvatar),
          );

    return DecoratedBox(
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        border: Border.all(color: AppColors.cardBorder),
      ),
      child: ClipOval(
        child: SizedBox.square(dimension: size, child: image),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final preview = lastMessage.trim().isEmpty ? '尚無訊息' : lastMessage.trim();
    final semanticsLabel = [
      '與$name的對話',
      if (unread > 0) '$unread 則未讀訊息',
      preview,
      if (date.isNotEmpty) date,
    ].join('，');

    return Semantics(
      button: true,
      enabled: onTap != null,
      label: semanticsLabel,
      onTap: onTap,
      child: ExcludeSemantics(
        child: Material(
          color: AppColors.bgCard,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(HkRadius.lg),
            side: const BorderSide(color: AppColors.cardBorder),
          ),
          clipBehavior: Clip.antiAlias,
          child: InkWell(
            onTap: onTap,
            child: ConstrainedBox(
              constraints: const BoxConstraints(minHeight: 84),
              child: Padding(
                padding: const EdgeInsets.all(HkSpace.md),
                child: Row(
                  children: [
                    _buildAvatar(),
                    const SizedBox(width: HkSpace.md),
                    Expanded(
                      child: Column(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          Row(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Expanded(
                                child: Text(
                                  name,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: AppTypography.productName.copyWith(
                                    color: AppColors.textEmphasis,
                                  ),
                                ),
                              ),
                              if (date.isNotEmpty) ...[
                                const SizedBox(width: HkSpace.sm),
                                Text(date, style: AppTypography.caption),
                              ],
                            ],
                          ),
                          const SizedBox(height: HkSpace.xs),
                          Row(
                            children: [
                              Expanded(
                                child: Text(
                                  preview,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: AppTypography.meta.copyWith(
                                    color: unread > 0
                                        ? AppColors.textPrimary
                                        : AppColors.textSecondary,
                                    fontWeight: unread > 0
                                        ? FontWeight.w700
                                        : FontWeight.w400,
                                  ),
                                ),
                              ),
                              if (unread > 0) ...[
                                const SizedBox(width: HkSpace.sm),
                                Container(
                                  constraints: const BoxConstraints(
                                    minWidth: 24,
                                    minHeight: 24,
                                  ),
                                  padding: const EdgeInsets.symmetric(
                                    horizontal: HkSpace.xs,
                                  ),
                                  decoration: BoxDecoration(
                                    color: AppColors.danger,
                                    borderRadius: BorderRadius.circular(
                                      HkRadius.pill,
                                    ),
                                  ),
                                  alignment: Alignment.center,
                                  child: Text(
                                    _unreadText,
                                    style: AppTypography.caption.copyWith(
                                      color: AppColors.textInverse,
                                      fontWeight: FontWeight.w700,
                                      height: 1,
                                    ),
                                  ),
                                ),
                              ],
                            ],
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _AssistantIcon extends StatelessWidget {
  const _AssistantIcon({required this.size});

  final double size;

  @override
  Widget build(BuildContext context) {
    return ExcludeSemantics(
      child: Icon(
        AppIcons.smartToyOutlined,
        size: size,
        color: AppIconColors.active,
      ),
    );
  }
}
