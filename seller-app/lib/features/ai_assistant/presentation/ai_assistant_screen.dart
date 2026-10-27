import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../data/ai_assistant_repository.dart';
import '../data/ai_message_model.dart';
import '../../../shared/widgets/chat_conversation_chrome.dart';
import '../../../shared/widgets/chat_input_bar.dart';
import 'ai_assistant_controller.dart';
import 'ai_function_menu.dart';
import 'widgets/ai_assistant_avatar.dart';

/// AI 小幫手全螢幕聊天頁面 (Figma: 7178:45950)
class AiAssistantScreen extends HookConsumerWidget {
  const AiAssistantScreen({super.key});

  static const _green = AppColors.primary;
  static const _textDark = AppColors.textEmphasis;

  static const _welcomeMessage =
      '您好，我是 Hawkr 賣家助手。\n'
      '我可以協助您查詢訂單、出貨狀況與金流資訊。\n\n'
      '提醒：涉及退款、賠償或食安責任問題，請直接要求轉接真人。';

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

    Future<void> triggerQuickAction(
      SellerAssistantEndpoint endpoint,
      String label,
    ) async {
      if (sending.value) return;
      sending.value = true;
      try {
        await controller.sendQuickAction(
          endpoint: endpoint,
          defaultMessage: label,
        );
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
            label: '查詢未出貨訂單',
            onTap: () =>
                triggerQuickAction(SellerAssistantEndpoint.orders, '查詢未出貨訂單'),
          ),
          AiFunctionMenuItem(
            icon: AppIcons.archiveOutlined,
            label: '目前有庫存不足嗎?',
            onTap: () => triggerQuickAction(
              SellerAssistantEndpoint.inventory,
              '目前有庫存不足嗎?',
            ),
          ),
          AiFunctionMenuItem(
            icon: AppIcons.attachMoney,
            label: '收款與對帳',
            onTap: () =>
                triggerQuickAction(SellerAssistantEndpoint.payment, '收款與對帳'),
          ),
          AiFunctionMenuItem(
            icon: AppIcons.insightsOutlined,
            label: '商品分析與建議',
            onTap: () =>
                triggerQuickAction(SellerAssistantEndpoint.price, '商品分析與建議'),
          ),
        ],
      );
    }

    return Scaffold(
      key: const ValueKey('seller-ai-assistant-chat-screen'),
      backgroundColor: AppColors.bgPrimary,
      resizeToAvoidBottomInset: true,
      body: Column(
        children: [
          ChatConversationTopBar(
            avatar: AiAssistantAvatar(
              key: const ValueKey('seller-ai-assistant-header-avatar'),
              size: 40.w,
              isReplying: sending.value,
            ),
            title: 'AI 營運助手',
            statusLabel: 'AI 服務中・訂單、出貨與金流',
            statusColor: AppColors.success.withValues(alpha: 0.85),
            onBack: () => context.pop(),
          ),

          // ── Message area + FAB ──
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

                    final hasOrders =
                        !isUser &&
                        msg.orderList != null &&
                        msg.orderList!.isNotEmpty;

                    Widget bubble = Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        if (msg.content.isNotEmpty || !hasOrders)
                          _buildBubble(
                            context,
                            msg,
                            isUser,
                            isFirst,
                            hasNext || hasOrders,
                            isReplying:
                                sending.value &&
                                !isUser &&
                                i == messages.length - 1,
                          ),
                        if (hasOrders)
                          Padding(
                            padding: EdgeInsets.only(
                              top: msg.content.isNotEmpty ? 8.h : 0,
                            ),
                            child: _OrderCardsBlock(orders: msg.orderList!),
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
                                  _formatSeparatorTime(msg.timestamp),
                                  style: const TextStyle(
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
                              _formatTime(msg.timestamp),
                              style: const TextStyle(
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
                          color: _green,
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
                                top: -4.h,
                                right: -4.w,
                                child: Container(
                                  padding: EdgeInsets.symmetric(
                                    horizontal: 8.w,
                                    vertical: 4.h,
                                  ),
                                  decoration: const BoxDecoration(
                                    color: AppColors.danger,
                                    shape: BoxShape.circle,
                                  ),
                                  child: Text(
                                    '${unreadWhileAway.value}',
                                    style: TextStyle(
                                      fontSize: 12.sp,
                                      color: AppColors.bgCard,
                                      fontWeight: FontWeight.w700,
                                    ),
                                  ),
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
                  color: isUser ? _green : AppColors.bgCard,
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
                          fontSize: 16.sp,
                          color: isUser ? AppColors.bgCard : _textDark,
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

  String _formatTime(DateTime dt) {
    final t = dt.toLocal();
    final h = t.hour;
    final m = t.minute.toString().padLeft(2, '0');
    final period = h < 12 ? '上午' : '下午';
    final h12 = h == 0 ? 12 : (h > 12 ? h - 12 : h);
    return '$period${h12.toString().padLeft(2, '0')}:$m';
  }

  String _formatSeparatorTime(DateTime dt) {
    final t = dt.toLocal();
    final time =
        '${t.hour.toString().padLeft(2, '0')}:'
        '${t.minute.toString().padLeft(2, '0')}';
    final today = DateTime.now();
    if (t.year == today.year && t.month == today.month && t.day == today.day) {
      return time;
    }
    return '${t.month}/${t.day} $time';
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

// ── 助理回覆中的三點動畫 ──
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
                  AppColors.gray200,
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

// ── 訂單卡片區塊（AI 訊息中的訂單清單，Figma: 9636:99531）──
class _OrderCardsBlock extends StatelessWidget {
  const _OrderCardsBlock({required this.orders});
  final List<AiOrderItem> orders;

  @override
  Widget build(BuildContext context) {
    // 與 _buildBubble 中 AI 頭像槽一致的左側縮排（32 + 6 = 38）
    return Padding(
      padding: EdgeInsets.only(left: 32.w + 8.w),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          for (int i = 0; i < orders.length; i++)
            Padding(
              padding: EdgeInsets.only(top: i == 0 ? 0 : 12.h),
              child: _OrderCard(order: orders[i]),
            ),
        ],
      ),
    );
  }
}

class _OrderCard extends StatelessWidget {
  const _OrderCard({required this.order});
  final AiOrderItem order;

  static const _gray = AppColors.textPrimary;

  ({Color bg, Color fg, String label}) _statusStyle() {
    final s = order.status.toLowerCase();
    final isAfterSales =
        order.status.contains('售後') ||
        s.contains('after') ||
        s.contains('refund');
    if (isAfterSales) {
      return (
        bg: AppColors.error,
        fg: AppColors.bgCard,
        label: order.status.isEmpty ? '售後服務' : order.status,
      );
    }
    return (
      bg: const Color(0x80E0C44C),
      fg: _gray,
      label: order.status.isEmpty ? '待出貨' : order.status,
    );
  }

  String _formatDate() {
    if (order.createTimeMs <= 0) return '';
    final d = DateTime.fromMillisecondsSinceEpoch(order.createTimeMs).toLocal();
    return '${d.year}/${d.month.toString().padLeft(2, '0')}/${d.day.toString().padLeft(2, '0')}';
  }

  @override
  Widget build(BuildContext context) {
    final status = _statusStyle();
    final date = _formatDate();
    return Material(
      color: Colors.transparent,
      child: InkWell(
        borderRadius: BorderRadius.circular(16),
        onTap: () {
          HapticFeedback.selectionClick();
          // 引導到訂單列表頁讓賣家處理(訂單詳情頁需要 OrderSummary extra,
          // AI tool_result 目前只有 order_no,故先導到列表頁)
          GoRouter.of(context).push('/orders/view');
        },
        child: Ink(
          width: 300.w,
          padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
          decoration: BoxDecoration(
            color: AppColors.bgCard,
            borderRadius: BorderRadius.circular(16),
            border: Border.all(color: AppColors.cardBorder),
            boxShadow: const [
              BoxShadow(
                color: AppColors.cardShadow,
                blurRadius: 3,
                offset: Offset(0, 1),
              ),
            ],
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    // Row 1: 訂單編號 + 狀態 badge + 金額
                    Row(
                      children: [
                        Flexible(
                          child: Text(
                            '#${order.orderId}',
                            style: TextStyle(
                              fontSize: 14.sp,
                              fontWeight: FontWeight.w700,
                              color: _gray,
                              height: 20 / 14,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                        SizedBox(width: 8.w),
                        Container(
                          padding: EdgeInsets.symmetric(
                            horizontal: 8.w,
                            vertical: 4.h,
                          ),
                          decoration: BoxDecoration(
                            color: status.bg,
                            borderRadius: BorderRadius.circular(999),
                          ),
                          child: Text(
                            status.label,
                            style: TextStyle(
                              fontSize: 12.sp,
                              fontWeight: FontWeight.w700,
                              color: status.fg,
                              letterSpacing: 1,
                              height: 1.5,
                            ),
                          ),
                        ),
                        const Spacer(),
                        Text(
                          '\$${order.amountTwd}',
                          style: TextStyle(
                            fontSize: 14.sp,
                            fontWeight: FontWeight.w700,
                            color: _gray,
                            height: 20 / 14,
                          ),
                        ),
                      ],
                    ),
                    if (date.isNotEmpty) ...[
                      SizedBox(height: 8.h),
                      // Row: 日期（買家/品項資訊不在 model 中，省略以避免假資料）
                      Align(
                        alignment: Alignment.centerRight,
                        child: Text(
                          date,
                          style: TextStyle(
                            fontSize: 14.sp,
                            color: _gray,
                            height: 1.44,
                          ),
                        ),
                      ),
                    ],
                  ],
                ),
              ),
              SizedBox(width: 4.w),
              Icon(AppIcons.chevronRight, size: 24.sp, color: _gray),
            ],
          ),
        ),
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
