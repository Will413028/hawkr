import 'dart:async';

import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/theme/hk_tokens.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/category_top_bar.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/hk_tap_target.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/chat_conversation_fields.dart';
import '../data/chat_model.dart';
import '../data/chat_repository.dart';
import 'chat_focus_policy.dart';
import 'chat_preview.dart';
import 'widgets/chat_list_widgets.dart';

class ChatListScreen extends ConsumerStatefulWidget {
  const ChatListScreen({super.key});
  @override
  ConsumerState<ChatListScreen> createState() => _ChatListScreenState();
}

class _ChatListScreenState extends ConsumerState<ChatListScreen> {
  /// 將時間轉為相對時間文字
  static String _formatTimeAgo(DateTime dt) {
    final now = DateTime.now();
    final diff = now.difference(dt);
    if (diff.inSeconds < 60) return '剛剛';
    if (diff.inMinutes < 60) return '${diff.inMinutes}分鐘前';
    if (diff.inHours < 24) return '${diff.inHours}小時前';
    if (diff.inDays < 7) return '${diff.inDays}天前';
    if (dt.year == now.year) return '${dt.month}/${dt.day}';
    return '${dt.year}/${dt.month}/${dt.day}';
  }

  bool _loading = true;
  String? _error;
  List<Map<String, dynamic>> _conversations = const [];
  Timer? _updateTimer;
  StreamSubscription<ChatMessage>? _wsMessageSub;
  Timer? _wsRefreshDebounce;
  // keepAlive provider，於 initState（widget 存活期）capture 一次；dispose() 改用
  // 這個實例呼叫，不在 dispose 內再碰 ref——widget 拆除路徑上 ref.read 本身不安全。
  late final ChatRepository _chatRepo;
  // Note: participant names and avatars are returned directly by
  // GET /app/chat/conversations（欄位定義見 data/chat_conversation_fields.dart）。
  // No per-user profile fetch (N+1) is needed.

  bool _isUpdating = false;

  final TextEditingController _searchController = TextEditingController();
  String _searchQuery = '';

  @override
  void initState() {
    super.initState();
    // 進場零動畫
    WidgetsBinding.instance.addPostFrameCallback((_) => _skipRouteTransition());
    _chatRepo = ref.read(chatRepositoryProvider);
    _refresh(showLoading: true);
    // 列表駐留期間保持 WebSocket 連線（refCount +1），訂閱 onMessage 讓對方
    // 新訊息即時刷新列表（lastMessage / unreadCount），不再只靠 FCM + pull-to-refresh
    unawaited(_chatRepo.connectForChat());
    _wsMessageSub = _chatRepo.onMessage.listen(_handleIncomingWsMessage);
  }

  void _handleIncomingWsMessage(ChatMessage m) {
    // 忽略 typing / presence 等非訊息事件（這些 id 為空）
    if (m.id.isEmpty) return;
    // 防抖：短時間多條訊息只觸發一次 refetch，避免打爆 API
    _wsRefreshDebounce?.cancel();
    _wsRefreshDebounce = Timer(const Duration(milliseconds: 400), () {
      if (!mounted) return;
      unawaited(_updateConversationsInBackground());
    });
  }

  void _skipRouteTransition() {
    final route = ModalRoute.of(context);
    if (route == null) return;

    void jumpToEnd(Animation<double>? anim) {
      if (anim == null) return;
      final parent = anim is ProxyAnimation ? anim.parent : anim;
      if (parent is AnimationController) {
        parent.value = 1;
        parent.stop();
      }
    }

    jumpToEnd(route.animation);
    jumpToEnd(route.secondaryAnimation);
  }

  Future<void> _updateConversationsInBackground() async {
    if (_isUpdating) return; // 防重入
    _isUpdating = true;

    try {
      HKLog.logD('ChatListScreen: Background updating conversations...');
      // 非同步獲取最新的對話列表
      final conversations = await ref
          .read(chatRepositoryProvider)
          .getConversations(pageSize: 50);

      // 只有在組件還存在時才更新UI
      if (mounted) {
        // 檢查是否有變化，避免不必要的UI更新
        final hasChanges =
            _conversations.length != conversations.length ||
            !_areConversationsEqual(_conversations, conversations);

        if (hasChanges) {
          HKLog.logD(
            'ChatListScreen: Conversations updated (${conversations.length} items)',
          );
          setState(() {
            _conversations = conversations;
            _error = null;
          });
        }
      }
    } catch (e) {
      // 靜默處理錯誤，避免干擾用戶操作
      HKLog.logE('ChatListScreen: Background update failed: $e');
    } finally {
      _isUpdating = false;
    }
  }

  // 比較兩個對話列表是否相等（簡單比較）
  bool _areConversationsEqual(
    List<Map<String, dynamic>> list1,
    List<Map<String, dynamic>> list2,
  ) {
    if (list1.length != list2.length) return false;
    for (int i = 0; i < list1.length; i++) {
      final conv1 = list1[i];
      final conv2 = list2[i];
      // 比較關鍵欄位（im.proto Conversation 的實際欄位；原本比的
      // conversationId / lastContent / lastTimestamp 後端從未回過，三個恆為
      // null，使長度相同的兩份列表恆判定為「沒變」，背景更新永遠不重繪）
      if (conv1[conversationTargetUserIdKey] !=
              conv2[conversationTargetUserIdKey] ||
          conv1[conversationLastMessageKey] !=
              conv2[conversationLastMessageKey] ||
          conv1[conversationUnreadCountKey] !=
              conv2[conversationUnreadCountKey] ||
          conv1[conversationLastMessageTimeKey] !=
              conv2[conversationLastMessageTimeKey]) {
        return false;
      }
    }
    return true;
  }

  @override
  void dispose() {
    _updateTimer?.cancel();
    _wsRefreshDebounce?.cancel();
    _wsMessageSub?.cancel();
    unawaited(_chatRepo.disconnectFromChat());
    _searchController.dispose();
    super.dispose();
  }

  Future<void> _refresh({bool showLoading = false}) async {
    // 暫停自動更新，避免衝突
    _isUpdating = true;

    setState(() {
      if (showLoading) _loading = true;
      _error = null;
    });
    try {
      final convs = await ref
          .read(chatRepositoryProvider)
          .getConversations(pageSize: 50);
      if (!mounted) return;
      setState(() {
        _conversations = convs;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = '載入失敗，請稍後再試';
        _loading = false;
      });
    } finally {
      // 恢復自動更新
      _isUpdating = false;
    }
  }

  void _clearSearch() {
    _searchController.clear();
    FocusScope.of(context).unfocus();
    setState(() => _searchQuery = '');
  }

  void _openAssistant() => context.push('/chat/ai-assistant');

  Widget _buildFillState(String storageKey, Widget child) {
    return CustomScrollView(
      key: PageStorageKey<String>(storageKey),
      physics: const AlwaysScrollableScrollPhysics(),
      slivers: [SliverFillRemaining(hasScrollBody: false, child: child)],
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      floatingActionButton: !_loading && _conversations.isNotEmpty
          ? ChatAssistantFab(onPressed: _openAssistant)
          : null,
      floatingActionButtonLocation: FloatingActionButtonLocation.endFloat,
      body: Column(
        children: [
          // 與全站一致的 Header，搜尋欄維持 48dp 觸控高度。
          CategoryTopBar(
            backgroundColor: AppColors.topBarBackground,
            systemOverlayStyle: AppColors.topBarOverlay,
            middle: Align(
              alignment: Alignment.centerLeft,
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 680),
                child: SizedBox(
                  width: double.infinity,
                  child: CategorySearchField(
                    controller: _searchController,
                    hintText: '搜尋對話或商家',
                    height: HkControl.minTap,
                    onChanged: (value) => setState(() => _searchQuery = value),
                    onSubmitted: (value) {
                      FocusScope.of(context).unfocus();
                      setState(() => _searchQuery = value.trim());
                    },
                    suffixIcon: _searchQuery.isNotEmpty
                        ? Semantics(
                            label: '清除搜尋',
                            button: true,
                            onTap: _clearSearch,
                            child: ExcludeSemantics(
                              child: GestureDetector(
                                behavior: HitTestBehavior.opaque,
                                onTap: _clearSearch,
                                child: const SizedBox.square(
                                  dimension: HkControl.minTap,
                                  child: Center(
                                    child: HkIcon(
                                      HkIconName.close,
                                      size: 20,
                                      color: AppIconColors.muted,
                                    ),
                                  ),
                                ),
                              ),
                            ),
                          )
                        : null,
                  ),
                ),
              ),
            ),
            leading: HkTapTarget(
              semanticLabel: '返回',
              onTap: () => context.pop(),
              child: const HkIcon(
                HkIconName.back,
                color: AppIconColors.strong,
                size: HkControl.icon,
              ),
            ),
          ),
          if (_error != null && _conversations.isNotEmpty)
            Center(
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 720),
                child: ChatErrorBanner(message: _error!, onRetry: _refresh),
              ),
            ),
          Expanded(
            child: _loading && _conversations.isEmpty
                ? const TruckLoadingOverlay()
                : LayoutBuilder(
                    builder: (context, constraints) => Align(
                      alignment: Alignment.topCenter,
                      child: SizedBox(
                        width: constraints.maxWidth > 720
                            ? 720
                            : constraints.maxWidth,
                        child: FruitRefreshIndicator(
                          onRefresh: _refresh,
                          child: Builder(
                            builder: (context) {
                              final base = _conversations;
                              final rawQuery = _searchQuery.trim();
                              final q = rawQuery.toLowerCase();
                              final list = q.isEmpty
                                  ? base
                                  : base.where((c) {
                                      final last =
                                          (c[conversationLastMessageKey] ?? '')
                                              .toString();
                                      final name =
                                          resolveConversationDisplayName(c);
                                      final hay = ('$name $last').toLowerCase();
                                      return hay.contains(q);
                                    }).toList();

                              if (_error != null && base.isEmpty) {
                                return _buildFillState(
                                  'chat-load-error',
                                  ChatLoadErrorState(
                                    onRetry: () => _refresh(showLoading: true),
                                  ),
                                );
                              }
                              if (list.isEmpty) {
                                if (q.isEmpty) {
                                  return _buildFillState(
                                    'chat-empty-state',
                                    ChatEmptyState(
                                      onStartAssistant: _openAssistant,
                                    ),
                                  );
                                }
                                return _buildFillState(
                                  'chat-search-empty-$q',
                                  ChatSearchEmptyState(
                                    query: rawQuery,
                                    onClear: _clearSearch,
                                  ),
                                );
                              }
                              return ListView.separated(
                                key: const PageStorageKey<String>(
                                  'chat-conversation-list',
                                ),
                                padding: const EdgeInsets.fromLTRB(
                                  HkSpace.lg,
                                  HkSpace.lg,
                                  HkSpace.lg,
                                  HkSpace.sectionLarge,
                                ),
                                itemCount: list.length,
                                separatorBuilder: (_, __) =>
                                    const SizedBox(height: HkSpace.md),
                                itemBuilder: (context, index) {
                                  final c = list[index];
                                  final preview = chatConversationPreview(c);
                                  final localTime =
                                      resolveConversationLastMessageTime(c);
                                  final String date = localTime == null
                                      ? ''
                                      : _formatTimeAgo(localTime);
                                  final unread = resolveConversationUnreadCount(
                                    c,
                                  );

                                  final String merchantId =
                                      resolveConversationTargetId(c);
                                  // 需求：列表顯示商家名稱（不是暱稱）
                                  final String displayName =
                                      resolveConversationDisplayName(c);
                                  final String avatarUrl =
                                      resolveConversationAvatarUrl(c);

                                  HKLog.logD(
                                    'ChatList item merchantId=$merchantId name=$displayName avatar=${avatarUrl.isNotEmpty ? avatarUrl : '(none)'}',
                                  );

                                  return ChatConversationTile(
                                    key: ValueKey(merchantId),
                                    name: displayName,
                                    avatarUrl: avatarUrl,
                                    lastMessage: preview,
                                    date: date,
                                    unread: unread,
                                    onTap: () async {
                                      HKLog.logD(
                                        '=== ChatListScreen: Item Tap Event ===',
                                      );
                                      HKLog.logD(
                                        'About to navigate with merchantId: "$merchantId" merchantName: "$displayName"',
                                      );

                                      if (merchantId.isEmpty) {
                                        HKLog.logE(
                                          'ERROR: merchantId is empty! Original data was: $c',
                                        );
                                        showCommonSnackBar(
                                          context,
                                          '無法提取商家ID，原始數據：${c.keys.toList()}',
                                        );
                                        return;
                                      }

                                      // 如果有未讀訊息，先清除未讀數量
                                      // 後端 PATCH /app/chat/conversations/{targetId} 的 targetId 是
                                      // 「對方使用者 ID」（int64），也就是 targetUserId。
                                      final readTargetId = merchantId;
                                      if (unread > 0) {
                                        HKLog.logD(
                                          'Clearing unread count with targetId=$readTargetId',
                                        );
                                        try {
                                          // 更新本地狀態，立即清除紅點
                                          setState(() {
                                            c[conversationUnreadCountKey] = 0;
                                          });

                                          // 調用後端 API 清除未讀（唯一正確路徑：用對方 user id）
                                          if (readTargetId.isNotEmpty) {
                                            await ref
                                                .read(chatRepositoryProvider)
                                                .markConversationAsRead(
                                                  readTargetId,
                                                );
                                          }
                                        } catch (e) {
                                          HKLog.logE(
                                            'Failed to clear unread count: $e',
                                          );
                                        }
                                      }

                                      HKLog.logD(
                                        'SUCCESS: Navigating to ChatThreadScreen...',
                                      );
                                      if (context.mounted) {
                                        ChatFocusPolicy.dismissKeyboard();
                                        await context.push(
                                          '/chat/room',
                                          // conversationId 不在這裡傳：對話列表 API
                                          // 不回這個欄位，ChatThreadScreen 進場時
                                          // 本來就是 null，收到第一則 WS 訊息才回填。
                                          extra: {
                                            'merchantId': merchantId,
                                            'merchantName': displayName,
                                          },
                                        );
                                        // 從 chat room 返回後先確保 server 端未讀已清（防止
                                        // 離開前最後一批訊息未被 markMessageRead）
                                        if (readTargetId.isNotEmpty) {
                                          try {
                                            await ref
                                                .read(chatRepositoryProvider)
                                                .markConversationAsRead(
                                                  readTargetId,
                                                );
                                          } catch (_) {}
                                        }
                                        // 再重抓對話列表
                                        if (mounted) {
                                          await _updateConversationsInBackground();
                                          // 最終保險：refetch 回來若 server 仍回 unread>0（cache
                                          // 延遲 / 對方於回列表瞬間又送訊息），強制歸 0 僅限此對話
                                          if (mounted) {
                                            setState(() {
                                              for (final item
                                                  in _conversations) {
                                                final itemTarget =
                                                    resolveConversationTargetId(
                                                      item,
                                                    );
                                                if (itemTarget ==
                                                        readTargetId &&
                                                    itemTarget.isNotEmpty) {
                                                  item[conversationUnreadCountKey] =
                                                      0;
                                                }
                                              }
                                            });
                                          }
                                        }
                                      }
                                    },
                                  );
                                },
                              );
                            },
                          ),
                        ),
                      ),
                    ),
                  ),
          ),
        ],
      ),
    );
  }
}
