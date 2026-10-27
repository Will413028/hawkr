import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/app_typography.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../data/chat_repository.dart';
import '../../dashboard/data/dashboard_chat_unread_provider.dart';
import '../../../config/image_config.dart';
import '../../../config/media_image.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/hk_async_state_panel.dart';
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
    final error = useState<String?>(null);
    final conversations = useState<List<Map<String, dynamic>>>(const []);
    final unreadTotal = useState(0);
    // Note: participant names and avatars are now returned directly by
    // GET /app/chat/conversations (targetNickname, avatarKey/avatarUrl,
    // storeName, logoKey/logoUrl). No per-user profile fetch (N+1) is needed.
    final isUpdating = useState(false);
    final isInitialLoading = useState(true);
    final searchCtrl = useTextEditingController();
    final query = useState('');

    Future<void> refreshUnreadTotal() async {
      try {
        final unread = await ref.read(chatRepositoryProvider).getUnreadCount();
        if (context.mounted) unreadTotal.value = unread;
      } catch (_) {
        if (context.mounted) unreadTotal.value = 0;
      }
    }

    Future<void> loadConversations() async {
      error.value = null;
      try {
        final svc = ref.read(chatRepositoryProvider);
        // 平行抓對話列表 + 未讀數，減少首屏等待
        final results = await Future.wait([
          svc.getConversations(pageSize: 50),
          svc.getUnreadCount(),
        ]);
        if (context.mounted) {
          conversations.value = results[0] as List<Map<String, dynamic>>;
          unreadTotal.value = results[1] as int;
        }
      } catch (e) {
        if (context.mounted) error.value = e.toString();
      } finally {
        if (context.mounted) isInitialLoading.value = false;
      }
    }

    Future<void> backgroundUpdate() async {
      if (isUpdating.value || !context.mounted) return;
      isUpdating.value = true;
      try {
        final svc = ref.read(chatRepositoryProvider);
        final convsList = await svc.getConversations(pageSize: 50);
        final unreadCount = await svc.getUnreadCount();
        if (context.mounted) {
          final hasChanges =
              conversations.value.length != convsList.length ||
              unreadTotal.value != unreadCount ||
              !conversations.value.every(
                (old) => convsList.any(
                  (n) =>
                      old['conversationId'] == n['conversationId'] &&
                      old['lastContent'] == n['lastContent'],
                ),
              );
          if (hasChanges) {
            conversations.value = convsList;
            unreadTotal.value = unreadCount;
          }
        }
      } catch (e) {
        if (kDebugMode) {
          HKLog.logD('[ChatList] Background update failed: $e');
        }
      } finally {
        isUpdating.value = false;
      }
    }

    useEffect(() {
      // 聊天列表不建立 WS 連線，靠 FCM 推播更新。WS 只在聊天室內使用。
      loadConversations();

      // 仍監聽 WS 訊息流（如果使用者從聊天室返回列表，WS 可能還未斷線）
      final svc = ref.read(chatRepositoryProvider);
      final msgSub = svc.onMessage.listen((msg) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (context.mounted) {
            HKLog.logD('[ChatList] New message received, refreshing...');
            backgroundUpdate();
          }
        });
      });
      final unreadSub = svc.onUnreadCountChange.listen((count) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (context.mounted && count != unreadTotal.value) {
            HKLog.logD('[ChatList] Unread count changed: $count');
            unreadTotal.value = count;
            backgroundUpdate();
          }
        });
      });

      void onSearchChanged() {
        if (context.mounted) query.value = searchCtrl.text.trim();
      }

      searchCtrl.addListener(onSearchChanged);

      return () {
        msgSub.cancel();
        unreadSub.cancel();
        searchCtrl.removeListener(onSearchChanged);
      };
    }, const []);

    String? extractLastMessageIdFor(String? conversationId) {
      if (conversationId == null || conversationId.isEmpty) return null;
      try {
        final raw = conversations.value.firstWhere(
          (c) => (c['targetUserId'] ?? '').toString() == conversationId,
          orElse: () => const {},
        );
        if (raw.isEmpty) return null;
        final v = raw['lastMessageId'];
        return v?.toString();
      } catch (_) {
        return null;
      }
    }

    Widget buildBody() {
      if (isInitialLoading.value) {
        return const _ChatListLoadingSkeleton();
      }

      if (error.value != null) {
        return ListView(
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
              onAction: loadConversations,
            ),
          ],
        );
      }

      final q = query.value.toLowerCase();
      final filteredConvs = q.isEmpty
          ? conversations.value
          : conversations.value.where((raw) {
              final rawTitle = (raw['targetNickname'] ?? '').toString();
              final lastMsg = (raw['lastMessage'] ?? '').toString();
              final nameCandidate = rawTitle.toLowerCase();
              return nameCandidate.contains(q) ||
                  lastMsg.toLowerCase().contains(q);
            }).toList();

      final allItems = <_ChatItem>[];
      for (final it in filteredConvs) {
        allItems.add(_resolveItem(it));
      }

      if (allItems.isEmpty) {
        return ChatEmptyState(
          isSearching: q.isNotEmpty,
          onClearSearch: () {
            searchCtrl.clear();
            query.value = '';
            FocusScope.of(context).unfocus();
          },
        );
      }

      return ListView.separated(
        key: const ValueKey('seller-chat-conversation-list'),
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(
          HkSpace.lg,
          HkSpace.sm,
          HkSpace.lg,
          HkSpace.section,
        ),
        itemCount: allItems.length,
        separatorBuilder: (_, __) => const SizedBox(height: HkSpace.sm),
        itemBuilder: (context, index) {
          final item = allItems[index];
          final isService = item.isService;
          final avatarUrl = item.avatarUrl;

          Future<void> openConversation() async {
            final pid0 = item.participantId ?? '';
            if (pid0.isNotEmpty) {
              unawaited(
                ref
                    .read(chatRepositoryProvider)
                    .markAllMessagesRead(participantId: pid0)
                    .catchError((_) => false)
                    .whenComplete(() {
                      if (context.mounted) {
                        ref.invalidate(dashboardChatUnreadProvider);
                      }
                    }),
              );
            }

            if (!isService && item.unreadCount > 0) {
              final convId = item.conversationId;
              extractLastMessageIdFor(convId);

              conversations.value = conversations.value.map((c) {
                final cid = (c['targetUserId'] ?? '').toString();
                if (cid == (convId ?? '')) {
                  final updated = Map<String, dynamic>.from(c);
                  updated['unreadCount'] = 0;
                  return updated;
                }
                return c;
              }).toList();

              final targetId = item.participantId ?? convId;
              if (targetId != null && targetId.isNotEmpty) {
                unawaited(
                  ref
                      .read(chatRepositoryProvider)
                      .markConversationAsRead(targetId)
                      .catchError((_) => false)
                      .whenComplete(() {
                        refreshUnreadTotal();
                        if (context.mounted) {
                          ref.invalidate(dashboardChatUnreadProvider);
                        }
                      }),
                );
              } else {
                unawaited(refreshUnreadTotal());
              }
            }

            if (!context.mounted) return;
            await context.push(
              '/chat/thread',
              extra: {
                'title': item.title,
                'conversationId': item.conversationId,
                'participantId': item.participantId,
              },
            );
            if (context.mounted) await loadConversations();
          }

          return _ConversationCard(
            title: item.title,
            subtitle: item.subtitle,
            date: item.date,
            unreadCount: item.unreadCount,
            avatarUrl: avatarUrl,
            onTap: openConversation,
          );
        },
      );
    }

    return Scaffold(
      backgroundColor: AppColors.surfacePage,
      appBar: HkGlassAppBar(
        title: const Text('訊息中心'),
        subtitle: unreadTotal.value > 0
            ? '${unreadTotal.value} 則未讀訊息'
            : '與顧客保持聯繫',
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
                            count: conversations.value.length,
                            unreadCount: unreadTotal.value,
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(height: HkSpace.xs),
                    Expanded(
                      child: FruitRefreshIndicator(
                        onRefresh: loadConversations,
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
      separatorBuilder: (_, __) => const SizedBox(height: HkSpace.sm),
      itemBuilder: (_, __) => const HkContentCard(
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

_ChatItem _resolveItem(Map<String, dynamic> raw) {
  final convId = (raw['targetUserId'] ?? '').toString();

  final rawTitle = (raw['targetNickname'] ?? '').toString();
  final title = rawTitle.isNotEmpty ? rawTitle : '顧客';

  // 契約成對回 `avatarKey` + `avatarUrl`（gateway chat.go `GetConversations`），
  // SSOT 是 key。舊欄位 `targetAvatarKey` / `targetAvatarUrl` / `targetAvatar`
  // 皆已不存在——這裡沒有任何其他欄位能兜底（賣家端會話的 `logoKey`/`logoUrl`
  // 恆為空），讀錯即全空。
  final rawAvatar = (raw['avatarKey'] ?? raw['avatarUrl'] ?? '')
      .toString()
      .trim();
  final resolvedAvatar = rawAvatar.isEmpty
      ? ''
      : buildImageUrl(rawAvatar, ImagePreset.avatar);
  final avatarUrl = resolvedAvatar.isEmpty ? null : resolvedAvatar;

  final lastMsg = (raw['lastMessage'] ?? '').toString();

  final lastTypeRaw = (raw['lastMessageType'] ?? '').toString().toLowerCase();
  final subtitle = derivePreview(lastMsg, lastTypeRaw);

  // API spec: lastMessageTime 是 int64 epoch 毫秒
  final ts = raw['lastMessageTime'];

  String date = '';
  try {
    DateTime? d;
    if (ts is int && ts > 0) {
      // API 可能回傳秒或毫秒，用數值大小判斷
      d = (ts < 10000000000)
          ? DateTime.fromMillisecondsSinceEpoch(ts * 1000)
          : DateTime.fromMillisecondsSinceEpoch(ts);
    } else if (ts != null) {
      // 嘗試 ISO 字串
      d = DateTime.tryParse('$ts');
      if (d == null) {
        final tsInt = int.tryParse('$ts') ?? 0;
        if (tsInt > 0) {
          d = (tsInt < 10000000000)
              ? DateTime.fromMillisecondsSinceEpoch(tsInt * 1000)
              : DateTime.fromMillisecondsSinceEpoch(tsInt);
        }
      }
    }
    if (d != null && d.millisecondsSinceEpoch > 0) {
      if (d.isUtc) d = d.toLocal();
      date = formatTimeAgo(d);
    }
  } catch (_) {}

  final unread = (raw['unreadCount'] is int)
      ? raw['unreadCount'] as int
      : int.tryParse('${raw['unreadCount']}') ?? 0;

  final participantId = (raw['targetUserId'] ?? '').toString();

  if (kDebugMode) {
    HKLog.logD(
      '[ChatList] convId=$convId title="$title" lastMsg="$lastMsg" date=$date ts=$ts unread=$unread',
    );
  }

  return _ChatItem(
    title: title,
    subtitle: subtitle,
    date: date,
    unreadCount: unread,
    isService: false,
    avatarUrl: avatarUrl,
    conversationId: convId.isNotEmpty ? convId : null,
    participantId: participantId.isNotEmpty ? participantId : null,
  );
}

class _ChatItem {
  final String title;
  final String subtitle;
  final String date;
  final int unreadCount;
  final bool isService;
  final String? avatarUrl;
  final String? conversationId;
  final String? participantId;

  const _ChatItem({
    required this.title,
    required this.subtitle,
    required this.date,
    this.unreadCount = 0,
    this.isService = false,
    this.avatarUrl,
    this.conversationId,
    this.participantId,
  });

  _ChatItem copyWith({
    String? title,
    String? subtitle,
    String? date,
    int? unreadCount,
    bool? isService,
    String? avatarUrl,
    String? conversationId,
    String? participantId,
  }) => _ChatItem(
    title: title ?? this.title,
    subtitle: subtitle ?? this.subtitle,
    date: date ?? this.date,
    unreadCount: unreadCount ?? this.unreadCount,
    isService: isService ?? this.isService,
    avatarUrl: avatarUrl ?? this.avatarUrl,
    conversationId: conversationId ?? this.conversationId,
    participantId: participantId ?? this.participantId,
  );
}
