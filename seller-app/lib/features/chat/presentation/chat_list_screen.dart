import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/app_typography.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import '../../../core/l10n/ui_locale.dart';
import '../../../shared/models/buyer_card_info.dart';
import '../data/chat_conversations_notifier.dart';
import '../data/chat_model.dart';
import '../../dashboard/data/dashboard_chat_unread_provider.dart';
import '../../../config/image_config.dart';
import '../../../shared/widgets/hk_async_state_panel.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_content_card.dart';
import '../../../shared/widgets/hk_frosted_card_surface.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/optimized_image.dart';
import 'chat_empty_state.dart';
import 'chat_preview.dart';

class ChatListScreen extends HookConsumerWidget {
  const ChatListScreen({super.key});

  static Route<T> route<T extends Object?>({RouteSettings? settings}) {
    return MaterialPageRoute<T>(
      builder: (_) => const ChatListScreen(),
      settings: settings,
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final conversations = ref.watch(chatConversationsProvider);
    final notifier = ref.read(chatConversationsProvider.notifier);
    final unreadTotal = ref.watch(dashboardChatUnreadProvider).value ?? 0;
    final locale = ref.watch(uiLocaleProvider);
    final searchCtrl = useTextEditingController();
    final query = useState('');

    useEffect(() {
      void onSearchChanged() {
        if (context.mounted) query.value = searchCtrl.text.trim();
      }

      searchCtrl.addListener(onSearchChanged);
      return () => searchCtrl.removeListener(onSearchChanged);
    }, const []);

    Future<void> reload() async {
      ref.invalidate(dashboardChatUnreadProvider);
      await notifier.reload();
    }

    Widget buildBody() {
      return conversations.when(
        loading: () => const _ChatListLoadingSkeleton(),
        error: (e, _) => ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: const EdgeInsets.fromLTRB(
            HkSpace.lg,
            HkSpace.sm,
            HkSpace.lg,
            HkSpace.section,
          ),
          children: [
            HkAsyncStatePanel(
              kind: HkAsyncStateKind.error,
              title: '目前無法載入對話',
              description: '請確認網路連線後再試一次；既有訊息不會因此遺失。',
              actionLabel: '重新載入',
              onAction: reload,
            ),
          ],
        ),
        data: (data) {
          final q = query.value.toLowerCase();
          final items =
              [
                for (final c in data.items)
                  _ChatItem.fromConversation(c, locale),
              ].where((it) {
                if (q.isEmpty) return true;
                return it.title.toLowerCase().contains(q) ||
                    it.subtitle.toLowerCase().contains(q);
              }).toList();

          if (items.isEmpty) {
            return ChatEmptyState(
              isSearching: q.isNotEmpty,
              onClearSearch: () {
                searchCtrl.clear();
                query.value = '';
                FocusScope.of(context).unfocus();
              },
            );
          }

          return NotificationListener<ScrollNotification>(
            onNotification: (n) {
              if (n.metrics.extentAfter < 200) unawaited(notifier.loadMore());
              return false;
            },
            child: ListView.separated(
              key: const ValueKey('seller-chat-conversation-list'),
              physics: const AlwaysScrollableScrollPhysics(),
              padding: const EdgeInsets.fromLTRB(
                HkSpace.lg,
                HkSpace.sm,
                HkSpace.lg,
                HkSpace.section,
              ),
              itemCount: items.length + (data.hasMore ? 1 : 0),
              separatorBuilder: (_, _) => const SizedBox(height: HkSpace.sm),
              itemBuilder: (context, index) {
                if (index == items.length) {
                  return data.loadMoreFailed
                      ? TextButton(
                          onPressed: notifier.loadMore,
                          child: const HkButtonLabel('載入更多失敗，點此重試'),
                        )
                      : const Padding(
                          padding: EdgeInsets.all(HkSpace.md),
                          child: Center(child: CircularProgressIndicator()),
                        );
                }
                final item = items[index];
                return _ConversationCard(
                  title: item.title,
                  subtitle: item.subtitle,
                  date: item.date,
                  unreadCount: item.unreadCount,
                  avatarUrl: item.avatarUrl,
                  onTap: () async {
                    await context.push(
                      '/chat/thread',
                      extra: {
                        'title': item.title,
                        'conversationId': item.conversationId,
                        'participantId': item.participantId,
                      },
                    );
                    if (context.mounted) {
                      ref.invalidate(dashboardChatUnreadProvider);
                      unawaited(notifier.refresh());
                    }
                  },
                );
              },
            ),
          );
        },
      );
    }

    return Scaffold(
      backgroundColor: AppColors.surfacePage,
      appBar: HkGlassAppBar(
        title: const Text('訊息中心'),
        subtitle: unreadTotal > 0 ? '$unreadTotal 則未讀訊息' : '與顧客保持聯繫',
      ),
      body: DecoratedBox(
        decoration: BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
            colors: [
              AppColors.brandPrimaryStrong.withValues(alpha: 0.08),
              AppColors.surfacePage,
              AppColors.brandSecondary.withValues(alpha: 0.05),
            ],
          ),
        ),
        child: LayoutBuilder(
          builder: (context, constraints) {
            final horizontalGutter = HkBreakpoints.pageGutter(
              constraints.maxWidth,
            );
            return Align(
              alignment: Alignment.topCenter,
              child: ConstrainedBox(
                constraints: const BoxConstraints(
                  maxWidth: HkControl.chromeMaxWidth,
                ),
                child: Column(
                  children: [
                    Padding(
                      padding: EdgeInsets.fromLTRB(
                        horizontalGutter,
                        HkSpace.lg,
                        horizontalGutter,
                        0,
                      ),
                      child: Column(
                        children: [
                          _ChatSearchField(
                            controller: searchCtrl,
                            query: query.value,
                            onSubmitted: (value) => query.value = value.trim(),
                            onClear: () {
                              searchCtrl.clear();
                              query.value = '';
                              FocusScope.of(context).unfocus();
                            },
                          ),
                          if (kAiAssistantChatEntryEnabled) ...[
                            const SizedBox(height: HkSpace.md),
                            _AiAssistantEntry(
                              onTap: () => context.push('/chat/ai-assistant'),
                            ),
                          ],
                          const SizedBox(height: HkSpace.xl),
                          _ConversationSectionHeader(
                            count: conversations.value?.items.length ?? 0,
                            unreadCount: unreadTotal,
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(height: HkSpace.xs),
                    Expanded(
                      child: FruitRefreshIndicator(
                        onRefresh: reload,
                        child: buildBody(),
                      ),
                    ),
                  ],
                ),
              ),
            );
          },
        ),
      ),
    );
  }
}

class _ChatSearchField extends StatelessWidget {
  const _ChatSearchField({
    required this.controller,
    required this.query,
    required this.onSubmitted,
    required this.onClear,
  });

  final TextEditingController controller;
  final String query;
  final ValueChanged<String> onSubmitted;
  final VoidCallback onClear;

  @override
  Widget build(BuildContext context) {
    return HkFrostedCardSurface(
      radius: HkRadius.xl,
      shadows: HkShadows.sm,
      blurSigma: HkGlass.topBarBlurSigma,
      child: SizedBox(
        height: HkControl.prominentSearchHeight,
        child: TextField(
          controller: controller,
          textInputAction: TextInputAction.search,
          textAlignVertical: TextAlignVertical.center,
          style: AppTypography.body,
          decoration: InputDecoration(
            hintText: '搜尋顧客或訊息',
            hintStyle: AppTypography.body.copyWith(color: AppColors.textMuted),
            border: InputBorder.none,
            enabledBorder: InputBorder.none,
            focusedBorder: InputBorder.none,
            contentPadding: const EdgeInsets.symmetric(vertical: HkSpace.lg),
            prefixIcon: const SizedBox(
              width: HkControl.minTap,
              height: HkControl.minTap,
              child: Center(
                child: HkIcon(
                  HkIconName.search,
                  size: HkControl.icon,
                  color: AppIconColors.muted,
                  semanticLabel: '搜尋',
                ),
              ),
            ),
            suffixIcon: query.isNotEmpty
                ? IconButton(
                    onPressed: onClear,
                    tooltip: '清除搜尋',
                    icon: const HkIcon(
                      HkIconName.close,
                      size: 20,
                      color: AppIconColors.muted,
                    ),
                  )
                : null,
          ),
          onSubmitted: onSubmitted,
        ),
      ),
    );
  }
}

/// AI 小幫手完成度不足，暫時對 user 隱藏入口；功能補完後改回 true。
const bool kAiAssistantChatEntryEnabled = false;

class _AiAssistantEntry extends StatelessWidget {
  const _AiAssistantEntry({required this.onTap});

  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return HkContentCard(
      onTap: onTap,
      semanticLabel: '開啟 AI 營運助手，查詢訂單、出貨與金流',
      elevated: true,
      padding: const EdgeInsets.symmetric(
        horizontal: HkSpace.lg,
        vertical: HkSpace.md,
      ),
      child: Row(
        children: [
          const DecoratedBox(
            decoration: BoxDecoration(
              color: AppColors.primarySoft,
              shape: BoxShape.circle,
            ),
            child: SizedBox.square(
              dimension: HkControl.minTap,
              child: Center(
                child: Icon(
                  AppIcons.smartToyOutlined,
                  color: AppIconColors.active,
                  size: HkControl.icon,
                ),
              ),
            ),
          ),
          const SizedBox(width: HkSpace.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('AI 營運助手', style: AppTypography.sectionTitle),
                const SizedBox(height: HkSpace.xs),
                Text(
                  '快速查詢訂單、出貨與金流',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.metaMuted,
                ),
              ],
            ),
          ),
          const SizedBox(width: HkSpace.sm),
          const HkIcon(
            HkIconName.chevronRight,
            size: 20,
            color: AppIconColors.muted,
          ),
        ],
      ),
    );
  }
}

class _ConversationSectionHeader extends StatelessWidget {
  const _ConversationSectionHeader({
    required this.count,
    required this.unreadCount,
  });

  final int count;
  final int unreadCount;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      container: true,
      label: '顧客對話，共 $count 則，$unreadCount 則未讀',
      child: ExcludeSemantics(
        child: Row(
          children: [
            Expanded(
              child: Text(
                '顧客對話',
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: AppTypography.sectionTitle,
              ),
            ),
            const SizedBox(width: HkSpace.sm),
            if (unreadCount > 0)
              Flexible(
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: HkSpace.sm,
                    vertical: HkSpace.xs,
                  ),
                  decoration: BoxDecoration(
                    color: AppColors.primarySoft,
                    borderRadius: BorderRadius.circular(HkRadius.pill),
                  ),
                  child: Text(
                    '$unreadCount 則未讀',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.caption.copyWith(
                      color: AppColors.textAccentOnTint,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
              )
            else
              Flexible(
                child: Text(
                  '$count 則對話',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.metaMuted,
                ),
              ),
          ],
        ),
      ),
    );
  }
}

class _ConversationCard extends StatelessWidget {
  const _ConversationCard({
    required this.title,
    required this.subtitle,
    required this.date,
    required this.unreadCount,
    required this.avatarUrl,
    required this.onTap,
  });

  final String title;
  final String subtitle;
  final String date;
  final int unreadCount;
  final String? avatarUrl;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final hasUnread = unreadCount > 0;
    return HkContentCard(
      onTap: onTap,
      semanticLabel:
          '$title，${subtitle.isEmpty ? '尚無訊息' : subtitle}'
          '${hasUnread ? '，$unreadCount 則未讀' : ''}',
      elevated: hasUnread,
      borderColor: hasUnread
          ? AppColors.primary.withValues(alpha: 0.22)
          : AppColors.cardBorder,
      padding: const EdgeInsets.all(HkSpace.md),
      child: ConstrainedBox(
        constraints: const BoxConstraints(minHeight: 56),
        child: Row(
          children: [
            _ConversationAvatar(avatarUrl: avatarUrl),
            const SizedBox(width: HkSpace.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          title,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.sectionTitle,
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
                          subtitle.isNotEmpty ? subtitle : '尚無訊息',
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.meta.copyWith(
                            color: hasUnread
                                ? AppColors.textPrimary
                                : AppColors.textMuted,
                            fontWeight: hasUnread
                                ? FontWeight.w500
                                : FontWeight.w400,
                          ),
                        ),
                      ),
                      if (hasUnread) ...[
                        const SizedBox(width: HkSpace.sm),
                        _UnreadBadge(count: unreadCount),
                      ],
                    ],
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _ConversationAvatar extends StatelessWidget {
  const _ConversationAvatar({required this.avatarUrl});

  final String? avatarUrl;

  @override
  Widget build(BuildContext context) {
    const fallback = DecoratedBox(
      decoration: BoxDecoration(
        color: AppColors.primarySoft,
        shape: BoxShape.circle,
      ),
      child: SizedBox.square(
        dimension: HkControl.minTap,
        child: Center(
          child: HkIcon(
            HkIconName.user,
            size: HkControl.icon,
            color: AppIconColors.active,
          ),
        ),
      ),
    );
    if (avatarUrl == null || avatarUrl!.isEmpty) return fallback;

    return Container(
      width: HkControl.minTap,
      height: HkControl.minTap,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        border: Border.all(color: AppColors.neutralWhite, width: 1.5),
        boxShadow: HkShadows.xs,
      ),
      child: ClipOval(
        child: OptimizedNetworkImage(
          imageUrl: avatarUrl!,
          scene: ImageScene.chatAvatar,
          fit: BoxFit.cover,
          width: HkControl.minTap,
          height: HkControl.minTap,
          errorWidget: fallback,
        ),
      ),
    );
  }
}

class _UnreadBadge extends StatelessWidget {
  const _UnreadBadge({required this.count});

  final int count;

  @override
  Widget build(BuildContext context) {
    final label = count > 99 ? '99+' : '$count';
    return Container(
      constraints: const BoxConstraints(minWidth: 24, minHeight: 24),
      padding: const EdgeInsets.symmetric(horizontal: HkSpace.xs),
      decoration: const BoxDecoration(
        color: AppColors.danger,
        shape: BoxShape.circle,
      ),
      alignment: Alignment.center,
      child: Text(
        label,
        style: const TextStyle(
          color: AppColors.textInverse,
          fontSize: 12,
          fontWeight: FontWeight.w700,
          height: 1,
        ),
      ),
    );
  }
}

class _ChatListLoadingSkeleton extends StatelessWidget {
  const _ChatListLoadingSkeleton();

  @override
  Widget build(BuildContext context) {
    return ListView.separated(
      physics: const NeverScrollableScrollPhysics(),
      padding: const EdgeInsets.fromLTRB(
        HkSpace.lg,
        HkSpace.sm,
        HkSpace.lg,
        HkSpace.section,
      ),
      itemCount: 4,
      separatorBuilder: (_, _) => const SizedBox(height: HkSpace.sm),
      itemBuilder: (_, _) => const HkContentCard(
        padding: EdgeInsets.all(HkSpace.md),
        child: SizedBox(
          height: 56,
          child: Row(
            children: [
              _SkeletonBlock.circle(size: HkControl.minTap),
              SizedBox(width: HkSpace.md),
              Expanded(
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _SkeletonBlock(width: 120, height: 14),
                    SizedBox(height: HkSpace.sm),
                    _SkeletonBlock(width: 200, height: 12),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _SkeletonBlock extends StatelessWidget {
  const _SkeletonBlock({required this.width, required this.height})
    : shape = BoxShape.rectangle;

  const _SkeletonBlock.circle({required double size})
    : width = size,
      height = size,
      shape = BoxShape.circle;

  final double width;
  final double height;
  final BoxShape shape;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: BoxDecoration(
        color: AppColors.surfaceMuted,
        shape: shape,
        borderRadius: shape == BoxShape.rectangle
            ? BorderRadius.circular(HkRadius.sm)
            : null,
      ),
      child: SizedBox(width: width, height: height),
    );
  }
}

class _ChatItem {
  const _ChatItem({
    required this.title,
    required this.subtitle,
    required this.date,
    required this.unreadCount,
    required this.conversationId,
    required this.participantId,
    this.avatarUrl,
  });

  factory _ChatItem.fromConversation(ChatConversation c, UiLocale locale) {
    final name = c.buyer.name;
    return _ChatItem(
      title: c.buyer.deleted ? deletedBuyerLabel : (name ?? '顧客'),
      subtitle: previewText(c.preview, locale),
      date: c.lastMessageTime == null ? '' : formatTimeAgo(c.lastMessageTime!),
      unreadCount: c.unreadCount,
      conversationId: c.id,
      participantId: c.buyer.id,
      avatarUrl: c.buyer.avatarUri,
    );
  }

  final String title;
  final String subtitle;
  final String date;
  final int unreadCount;
  final String conversationId;
  final String participantId;
  final String? avatarUrl;
}
