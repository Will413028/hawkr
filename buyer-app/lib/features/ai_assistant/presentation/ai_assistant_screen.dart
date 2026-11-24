import 'dart:async';

import 'package:hawkr_api/gen/hawkr/assistant/v1alpha1/assistant.pbenum.dart'
    show AssistantTopic;
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../shared/widgets/chat_conversation_chrome.dart';
import '../../../shared/widgets/chat_input_bar.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/notification_count_badge.dart';
import '../data/ai_message_model.dart';
import 'ai_assistant_controller.dart';
import 'ai_function_menu.dart';
import 'widgets/ai_assistant_avatar.dart';
import '../../../shared/time/taipei_display.dart';

/// AI 小幫手全螢幕聊天頁面 (Figma: 7178:45950)
class AiAssistantScreen extends HookConsumerWidget {
  const AiAssistantScreen({super.key});

  static const _welcomeMessage =
      '您好！我是您的生鮮買家小幫手 🛍️\n'
      '我可以協助您查詢訂單、包裹追蹤、退貨申請等服務。';

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final messages = ref.watch(aiAssistantControllerProvider);
    final controller = ref.read(aiAssistantControllerProvider.notifier);
    final inputCtrl = useTextEditingController();
    final scrollCtrl = useScrollController();
    final sending = useState(false);
    final showScrollFab = useState(false);
    final unreadWhileAway = useState(0);
    final lastAnimatedCount = useState(0);

    // 初次進入：載入歷史對話，若無對話則顯示歡迎訊息
    useEffect(() {
      WidgetsBinding.instance.addPostFrameCallback((_) async {
        await controller.loadLatestSession();
        if (!context.mounted) return;
        if (ref.read(aiAssistantControllerProvider).isEmpty) {
          controller.addSystemMessage(_welcomeMessage);
        }
      });
      return null;
    }, const []);

    // 新訊息：捲到最底
    useEffect(() {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!scrollCtrl.hasClients) return;
        final atBottom =
            scrollCtrl.position.maxScrollExtent - scrollCtrl.offset < 120;
        if (atBottom) {
          scrollCtrl.animateTo(
            scrollCtrl.position.maxScrollExtent,
            duration: const Duration(milliseconds: 220),
            curve: Curves.easeOut,
          );
        } else {
          unreadWhileAway.value++;
        }
        lastAnimatedCount.value = messages.length;
      });
      return null;
    }, [messages.length]);

    // 捲動 FAB 顯示邏輯
    useEffect(() {
      void listener() {
        if (!scrollCtrl.hasClients) return;
        final atBottom =
            scrollCtrl.position.maxScrollExtent - scrollCtrl.offset < 120;
        if (showScrollFab.value == atBottom) showScrollFab.value = !atBottom;
        if (atBottom) unreadWhileAway.value = 0;
      }

      scrollCtrl.addListener(listener);
      return () => scrollCtrl.removeListener(listener);
    }, [scrollCtrl]);

    void scrollToBottom() {
      if (!scrollCtrl.hasClients) return;
      scrollCtrl.animateTo(
        scrollCtrl.position.maxScrollExtent,
        duration: const Duration(milliseconds: 300),
        curve: Curves.easeOut,
      );
      unreadWhileAway.value = 0;
    }

    Future<void> onSend([String? text]) async {
      final msg = text ?? inputCtrl.text.trim();
      if (msg.isEmpty || sending.value) return;
      inputCtrl.clear();
      HapticFeedback.lightImpact();
      sending.value = true;
      try {
        await controller.sendMessage(msg);
      } finally {
        if (context.mounted) sending.value = false;
      }
    }

    Future<void> triggerQuickAction(AssistantTopic topic, String label) async {
      if (sending.value) return;
      sending.value = true;
      try {
        await controller.sendQuickAction(topic: topic, defaultMessage: label);
      } finally {
        if (context.mounted) sending.value = false;
      }
    }

    Future<void> openFunctionMenu() async {
      if (sending.value) return;
      await showAiFunctionMenu(
        context: context,
        items: [
          AiFunctionMenuItem(
            icon: AppIcons.assignmentOutlined,
            label: '查詢訂單',
            onTap: () => triggerQuickAction(
              AssistantTopic.ASSISTANT_TOPIC_ORDERS,
              '查詢訂單',
            ),
          ),
          AiFunctionMenuItem(
            icon: AppIcons.localShippingOutlined,
            label: '我的包裹到了嗎？',
            onTap: () => triggerQuickAction(
              AssistantTopic.ASSISTANT_TOPIC_TRACKING,
              '我的包裹到了嗎？',
            ),
          ),
          AiFunctionMenuItem(
            icon: AppIcons.assignmentReturnOutlined,
            label: '如何申請退貨',
            onTap: () => triggerQuickAction(
              AssistantTopic.ASSISTANT_TOPIC_RETURN_POLICY,
              '如何申請退貨',
            ),
          ),
        ],
      );
    }

    return Scaffold(
      key: const ValueKey('ai-assistant-chat-screen'),
      backgroundColor: AppColors.bgPrimary,
      resizeToAvoidBottomInset: true,
      body: Column(
        children: [
          commonTopBar(
            context: context,
            horizontalPadding: EdgeInsets.zero,
            child: ChatConversationHeaderContent(
              avatar: AiAssistantAvatar(
                key: const ValueKey('ai-assistant-header-avatar'),
                size: 40.w,
                isReplying: sending.value,
              ),
              title: '買家小幫手 ✨',
              statusLabel: 'AI 服務中',
              statusColor: AppColors.success.withValues(alpha: 0.85),
              onBack: () => context.pop(),
            ),
          ),

          // ── Message area + FAB + typing indicator ──
          Expanded(
            child: Stack(
              children: [
                const Positioned.fill(child: ChatConversationDotPattern()),

                // 訊息列表
                ListView.builder(
                  controller: scrollCtrl,
                  physics: const AlwaysScrollableScrollPhysics(),
                  padding: EdgeInsets.fromLTRB(12.w, 8.h, 12.w, 20.h),
                  itemCount: messages.length,
                  itemBuilder: (ctx, i) {
                    final msg = messages[i];
                    final isUser = msg.role == 'user';
                    final prevRole = i > 0 ? messages[i - 1].role : null;
                    final nextRole = i < messages.length - 1
                        ? messages[i + 1].role
                        : null;
                    final showTimeSeparator =
                        i > 0 &&
                        msg.timestamp
                                .difference(messages[i - 1].timestamp)
                                .inMinutes
                                .abs() >
                            5;
                    final sameSenderAsPrev =
                        prevRole == msg.role && !showTimeSeparator;
                    final sameSenderAsNext =
                        nextRole == msg.role &&
                        messages[i + 1].timestamp
                                .difference(msg.timestamp)
                                .inMinutes
                                .abs() <=
                            5;
                    final isFirst = !sameSenderAsPrev;
                    final hasNext = sameSenderAsNext;
                    final isNew = i >= lastAnimatedCount.value - 1;

                    Widget bubble = Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        _buildBubble(
                          context,
                          msg,
                          isUser,
                          isFirst,
                          hasNext,
                          isReplying:
                              sending.value &&
                              !isUser &&
                              i == messages.length - 1,
                        ),
                      ],
                    );
                    if (isNew) {
                      bubble = _MessageEntrance(isSelf: isUser, child: bubble);
                    }

                    return Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        if (showTimeSeparator)
                          Center(
                            child: Padding(
                              padding: EdgeInsets.symmetric(vertical: 12.h),
                              child: Container(
                                padding: EdgeInsets.symmetric(
                                  horizontal: 12.w,
                                  vertical: 4.h,
                                ),
                                decoration: BoxDecoration(
                                  color: AppColors.textPrimary.withValues(
                                    alpha: 0.08,
                                  ),
                                  borderRadius: BorderRadius.circular(12.r),
                                ),
                                child: Text(
                                  _formatSeparatorTime(
                                    msg.timestamp,
                                    context.taipeiDisplay,
                                  ),
                                  style: TextStyle(
                                    fontSize: 12,
                                    color: AppColors.textSecondary,
                                  ),
                                ),
                              ),
                            ),
                          ),
                        bubble,
                        Align(
                          alignment: isUser
                              ? Alignment.centerRight
                              : Alignment.centerLeft,
                          child: Padding(
                            padding: EdgeInsets.only(
                              top: 4.h,
                              bottom: 4.h,
                              left: isUser ? 4.w : 40.w,
                              right: isUser ? 4.w : 0,
                            ),
                            child: Text(
                              _formatTime(msg.timestamp, context.taipeiDisplay),
                              style: TextStyle(
                                fontSize: 12,
                                color: AppColors.textSecondary,
                              ),
                            ),
                          ),
                        ),
                      ],
                    );
                  },
                ),

                // 捲到最底 FAB（實心綠色，跟普通聊天室一樣）
                if (showScrollFab.value)
                  Positioned(
                    right: 16.w,
                    bottom: 12.h,
                    child: GestureDetector(
                      onTap: scrollToBottom,
                      child: Container(
                        width: 40.w,
                        height: 40.w,
                        decoration: BoxDecoration(
                          color: AppColors.primary,
                          shape: BoxShape.circle,
                          boxShadow: [
                            BoxShadow(
                              color: AppColors.cardShadow,
                              blurRadius: 6,
                              offset: Offset(0, 2.h),
                            ),
                          ],
                        ),
                        child: Stack(
                          clipBehavior: Clip.none,
                          children: [
                            Center(
                              child: Icon(
                                AppIcons.keyboardArrowDown,
                                color: AppIconColors.onBrand,
                                size: 24.sp,
                              ),
                            ),
                            if (unreadWhileAway.value > 0)
                              Positioned(
                                top: -8.h,
                                right: -8.w,
                                child: NotificationCountBadge(
                                  count: unreadWhileAway.value,
                                ),
                              ),
                          ],
                        ),
                      ),
                    ),
                  ),
              ],
            ),
          ),

          ChatInputBar(
            controller: inputCtrl,
            sending: sending.value,
            onSend: () => unawaited(onSend()),
            onPlusPressed: () => unawaited(openFunctionMenu()),
          ),
        ],
      ),
    );
  }

  Widget _buildBubble(
    BuildContext context,
    AiMessage msg,
    bool isUser,
    bool isFirst,
    bool hasNext, {
    required bool isReplying,
  }) {
    final maxW = MediaQuery.of(context).size.width * 0.72;

    // AI 頭像槽（跟普通聊天室的 avatarSlot 邏輯一樣）
    Widget avatarSlot;
    if (!isUser) {
      if (!hasNext) {
        avatarSlot = Padding(
          padding: EdgeInsets.only(right: 8.w, bottom: 4.h),
          child: AiAssistantAvatar(
            size: 32.w,
            isReplying: isReplying,
            showBorder: false,
          ),
        );
      } else {
        avatarSlot = SizedBox(width: 32.w + 6.w);
      }
    } else {
      avatarSlot = const SizedBox.shrink();
    }

    return Align(
      alignment: isUser ? Alignment.centerRight : Alignment.centerLeft,
      child: Row(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          if (!isUser) avatarSlot,
          Flexible(
            child: ConstrainedBox(
              constraints: BoxConstraints(maxWidth: maxW),
              child: Container(
                margin: EdgeInsets.only(top: isFirst ? 8.h : 4.h),
                padding: EdgeInsets.fromLTRB(12.w, 12.h, 12.w, 12.h),
                decoration: BoxDecoration(
                  color: isUser ? AppColors.primary : AppColors.bgCard,
                  borderRadius: _smartBorderRadius(isUser, isFirst, hasNext),
                  boxShadow: const [
                    BoxShadow(
                      color: AppColors.cardShadow,
                      blurRadius: 3,
                      offset: Offset(0, 1),
                    ),
                  ],
                ),
                child: msg.content.isEmpty
                    ? const _AssistantReplyLoadingDots()
                    : Text(
                        msg.content,
                        style: TextStyle(
                          fontSize: 16,
                          color: isUser
                              ? AppColors.bgCard
                              : AppColors.textEmphasis,
                          height: 1.5,
                        ),
                      ),
              ),
            ),
          ),
          if (isUser) SizedBox(width: 4.w),
        ],
      ),
    );
  }

  String _formatTime(DateTime dt, TaipeiDisplay td) {
    return td.format(dt, TaipeiFormat.time);
  }

  String _formatSeparatorTime(DateTime dt, TaipeiDisplay td) {
    return td.separator(dt);
  }
}

BorderRadius _smartBorderRadius(bool isSelf, bool isFirst, bool hasNext) {
  const full = Radius.circular(16);
  const small = Radius.circular(6);
  if (isSelf) {
    return BorderRadius.only(
      topLeft: full,
      topRight: isFirst ? full : small,
      bottomLeft: full,
      bottomRight: hasNext ? small : full,
    );
  } else {
    return BorderRadius.only(
      topLeft: isFirst ? full : small,
      topRight: full,
      bottomLeft: hasNext ? small : full,
      bottomRight: full,
    );
  }
}

// ── 助理回覆 loading 三點動畫（跟賣家版完全相同，內嵌在助理 bubble 中）──
class _AssistantReplyLoadingDots extends StatefulWidget {
  const _AssistantReplyLoadingDots();
  @override
  State<_AssistantReplyLoadingDots> createState() =>
      _AssistantReplyLoadingDotsState();
}

class _AssistantReplyLoadingDotsState extends State<_AssistantReplyLoadingDots>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 900),
    )..repeat();
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  double _dotProgress(int index) {
    final phase = (_ctrl.value - index * 0.18) % 1.0;
    final normalized = phase < 0 ? phase + 1 : phase;
    final mirrored = normalized < 0.5 ? normalized * 2 : (1 - normalized) * 2;
    return Curves.easeInOut.transform(mirrored);
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _ctrl,
      builder: (_, __) => Row(
        mainAxisSize: MainAxisSize.min,
        children: List.generate(3, (i) {
          final progress = _dotProgress(i);
          return Transform.translate(
            offset: Offset(0, -1.5.h * progress),
            child: Container(
              margin: EdgeInsets.symmetric(horizontal: 4.w, vertical: 4.h),
              width: 7.w,
              height: 7.w,
              decoration: BoxDecoration(
                color: Color.lerp(
                  AppColors.gray300,
                  AppColors.primary,
                  progress,
                ),
                shape: BoxShape.circle,
              ),
            ),
          );
        }),
      ),
    );
  }
}

// ── 訊息入場動畫（滑入 + 淡入，跟 chat_thread_screen 完全相同）──
class _MessageEntrance extends StatefulWidget {
  const _MessageEntrance({required this.isSelf, required this.child});
  final bool isSelf;
  final Widget child;

  @override
  State<_MessageEntrance> createState() => _MessageEntranceState();
}

class _MessageEntranceState extends State<_MessageEntrance>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;
  late final Animation<Offset> _slide;
  late final Animation<double> _fade;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 200),
    );
    _slide = Tween<Offset>(
      begin: Offset(widget.isSelf ? 0.3 : -0.3, 0.1),
      end: Offset.zero,
    ).animate(CurvedAnimation(parent: _ctrl, curve: Curves.easeOut));
    _fade = Tween<double>(
      begin: 0,
      end: 1,
    ).animate(CurvedAnimation(parent: _ctrl, curve: Curves.easeIn));
    _ctrl.forward();
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return SlideTransition(
      position: _slide,
      child: FadeTransition(opacity: _fade, child: widget.child),
    );
  }
}
