import 'dart:async';

import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart' show UiLocale;

import '../../../core/l10n/ui_locale.dart';
import 'package:hawkr_buyer/l10n/l10n.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/category_top_bar.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/hk_tap_target.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../seller/data/store_card_display.dart';
import '../data/chat_display.dart';
import '../data/chat_realtime.dart';
import '../data/chat_repository.dart';
import '../data/latest_request.dart';
import 'chat_focus_policy.dart';
import 'widgets/chat_list_widgets.dart';
import '../../../shared/time/taipei_display.dart';

class ChatListScreen extends ConsumerStatefulWidget {
  const ChatListScreen({super.key});
  @override
  ConsumerState<ChatListScreen> createState() => _ChatListScreenState();
}

class _ChatListScreenState extends ConsumerState<ChatListScreen> {
  static String _nameOf(pb.Conversation c, UiLocale locale, String fallback) =>
      c.hasStore() ? c.store.displayName(locale) : fallback;

  static String _previewOf(pb.Conversation c, UiLocale locale) =>
      c.hasPreview() ? chatPreviewText(c.preview, locale) : '';

  bool _loading = true;
  Object? _error;
  List<pb.Conversation> _conversations = const [];
  StreamSubscription<pb.ChatEvent>? _eventSub;
  Timer? _refreshDebounce;
  final _latestRefresh = LatestRequest();
  late final ChatRealtime _realtime;

  final TextEditingController _searchController = TextEditingController();
  String _searchQuery = '';

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _skipRouteTransition());
    _realtime = ref.read(chatRealtimeProvider);
    _refresh(showLoading: true);
    _realtime.acquire();
    _eventSub = _realtime.events.listen(_onEvent);
  }

  void _onEvent(pb.ChatEvent event) {
    switch (event.whichEvent()) {
      case pb.ChatEvent_Event.message:
        _refreshDebounce?.cancel();
        _refreshDebounce = Timer(const Duration(milliseconds: 400), () {
          if (mounted) unawaited(_refresh());
        });
      case pb.ChatEvent_Event.ready:
      case pb.ChatEvent_Event.resync:
        _refreshDebounce?.cancel();
        unawaited(_refresh());
      case pb.ChatEvent_Event.unread:
      case pb.ChatEvent_Event.peerRead:
      case pb.ChatEvent_Event.heartbeat:
      case pb.ChatEvent_Event.notSet:
        break;
    }
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

  @override
  void dispose() {
    _refreshDebounce?.cancel();
    unawaited(_eventSub?.cancel());
    _realtime.release();
    _searchController.dispose();
    super.dispose();
  }

  /// 重取全部對話。重取失敗時保留已顯示的列表，只顯示錯誤。
  Future<void> _refresh({bool showLoading = false}) async {
    if (showLoading && mounted) {
      setState(() {
        _loading = true;
        _error = null;
      });
    }
    try {
      final result = await _latestRefresh.run(
        ref.read(chatRepositoryProvider).listAllConversations,
      );
      if (!mounted || result.superseded) return;
      final convs = result.value!;
      setState(() {
        _conversations = convs;
        _loading = false;
        _error = null;
      });
    } catch (e) {
      HKLog.logE('ChatListScreen: refresh failed: $e');
      if (!mounted) return;
      setState(() {
        _error = e;
        _loading = false;
      });
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
                child: ChatErrorBanner(
                  message: errorText(
                    _error!,
                    defaults: ref.read(defaultErrorTextProvider),
                  ),
                  onRetry: _refresh,
                ),
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
                              final locale = ref.watch(uiLocaleProvider);
                              final base = _conversations;
                              final rawQuery = _searchQuery.trim();
                              final q = rawQuery.toLowerCase();
                              final list = q.isEmpty
                                  ? base
                                  : base.where((c) {
                                      final hay =
                                          '${_nameOf(c, locale, context.l10n.chatStoreFallbackName)} ${_previewOf(c, locale)}'
                                              .toLowerCase();
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
                                  final name = _nameOf(
                                    c,
                                    locale,
                                    context.l10n.chatStoreFallbackName,
                                  );
                                  final time = c.hasLastMessageTime()
                                      ? c.lastMessageTime.toDateTime()
                                      : null;
                                  return ChatConversationTile(
                                    key: ValueKey(c.id),
                                    name: name,
                                    avatarUrl: c.hasStore()
                                        ? c.store.logoUri
                                        : '',
                                    lastMessage: _previewOf(c, locale),
                                    date: time == null
                                        ? ''
                                        : context.taipeiDisplay.timeAgoIn(
                                            context.l10n,
                                            time,
                                          ),
                                    unread: c.unreadCount.toInt(),
                                    onTap: () async {
                                      ChatFocusPolicy.dismissKeyboard();
                                      await context.push(
                                        '/chat/room',
                                        extra: {
                                          'conversationId': c.id,
                                          'merchantName': name,
                                        },
                                      );
                                      if (mounted) await _refresh();
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
