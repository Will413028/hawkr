import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:hawkr_api/images.dart' show ImagePreset, ImageUri;
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:image_picker/image_picker.dart';

import 'package:hawkr_buyer/core/l10n/ui_locale.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';
import 'package:hawkr_buyer/shared/widgets/hk_placeholder.dart';

import '../../../config/image_config.dart';
import '../../../core/exceptions/app_exception.dart';
import '../../../core/motion/hk_motion_transitions.dart';
import '../../../core/networking/suspension_signal.dart';
import '../../../shared/widgets/app_cached_image.dart';
import '../../../shared/widgets/chat_conversation_chrome.dart';
import '../../../shared/widgets/chat_input_bar.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/notification_count_badge.dart';
import '../../../shared/widgets/oss_image.dart';
import '../../media/data/media_repository.dart';
import '../../moderation/data/block_target.dart';
import '../../moderation/data/report_target.dart';
import '../../moderation/presentation/block_confirm_dialog.dart';
import '../../moderation/presentation/report_sheet.dart';
import '../../seller/data/seller_repository.dart';
import '../../seller/data/store_card_display.dart';
import '../data/active_chat_gate.dart';
import '../data/chat_display.dart';
import '../data/chat_thread_notifier.dart';
import 'chat_focus_policy.dart';

/// 對方在線狀態的輪詢間隔（`GetPresence` 是 unary，沒有推送）。
const _presenceInterval = Duration(seconds: 30);

/// 往上捲到離頂端這麼近就載入更舊的一頁。
const _loadOlderThreshold = 300.0;

/// 一列：伺服器的訊息，或還沒有伺服器訊息的送出。
class _Row {
  const _Row.message(pb.Message this.message) : pending = null;
  const _Row.pending(PendingSend this.pending) : message = null;

  final pb.Message? message;
  final PendingSend? pending;

  DateTime get time => message != null && message!.hasCreateTime()
      ? message!.createTime.toDateTime().toLocal()
      : DateTime.now();

  bool get isSelf =>
      pending != null || message!.sender == pb.ChatParty.CHAT_PARTY_BUYER;

  bool get isImage => pending != null
      ? pending!.content.imageStagedToken != null
      : message!.kind == pb.MessageKind.MESSAGE_KIND_IMAGE &&
            !message!.hidden &&
            !message!.erased;

  String get key =>
      message != null ? 'm${message!.seq}' : 'p$pending.requestId';
}

Widget _buildInlineImage(String url) {
  if (url.isEmpty) {
    return Container(
      width: 200.w,
      height: 100.h,
      color: AppColors.gray200,
      child: const Icon(AppIcons.brokenImage, color: AppIconColors.muted),
    );
  }
  return ClipRRect(
    borderRadius: BorderRadius.circular(8.r),
    child: AppCachedImage(
      imageUrl: url,
      scene: ImageScene.chatImage,
      width: 200.w,
      height: 200.h,
      fit: BoxFit.cover,
    ),
  );
}

void _openFullScreenImage(BuildContext context, String url) {
  if (url.isEmpty || !url.startsWith('http')) return;
  Navigator.of(context).push(
    PageRouteBuilder<void>(
      opaque: false,
      barrierColor: AppColors.textEmphasis,
      barrierDismissible: true,
      transitionDuration: HkMotionTransitions.durationOf(
        context,
        presentation: HkModalPresentation.photoZoom,
      ),
      reverseTransitionDuration: HkMotionTransitions.durationOf(
        context,
        presentation: HkModalPresentation.photoZoom,
      ),
      pageBuilder: (_, __, ___) => _ImageViewerPage(url: url),
      transitionsBuilder: (transitionContext, animation, __, child) =>
          HkMotionTransitions.modal(
            context: transitionContext,
            animation: animation,
            presentation: HkModalPresentation.photoZoom,
            child: child,
          ),
    ),
  );
}

String _messageImageUrl(pb.Message m, ImagePreset preset) =>
    m.hasImage() ? m.image.uri(preset) : '';

/// 買家與一家店的聊天室。[conversationId]（從列表或推播進來）或 [storeId]（從店面或訂單進來，
/// 對話可能還不存在）至少給一個。
class ChatThreadScreen extends HookConsumerWidget {
  const ChatThreadScreen({
    super.key,
    this.storeId,
    this.storeName,
    this.conversationId,
  });

  final String? storeId;
  final String? storeName;
  final String? conversationId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final arg = ChatThreadArg(
      storeId: (storeId?.isNotEmpty ?? false) ? storeId : null,
      conversationId: (conversationId?.isNotEmpty ?? false)
          ? conversationId
          : null,
    );
    final provider = chatThreadProvider(arg);
    final thread = ref.watch(provider);
    final notifier = ref.read(provider.notifier);
    final locale = ref.watch(uiLocaleProvider);

    final textCtrl = useTextEditingController();
    final inputFocusNode = useFocusNode();
    final scrollCtrl = useScrollController();
    final showFab = useState(false);
    final unreadAway = useState(0);
    final animatedKeys = useRef(<String>{});
    final fetchedCard = useState<StoreCard?>(null);

    useEffect(() {
      ChatFocusPolicy.keepClosedOnEntry(inputFocusNode);
      return null;
    }, [inputFocusNode]);

    bool isNearBottom() {
      if (!scrollCtrl.hasClients) return true;
      return (scrollCtrl.position.pixels - scrollCtrl.position.minScrollExtent)
              .abs() <
          40;
    }

    void scrollToBottom() {
      if (!scrollCtrl.hasClients) return;
      scrollCtrl.animateTo(
        scrollCtrl.position.minScrollExtent,
        duration: const Duration(milliseconds: 180),
        curve: Curves.easeOut,
      );
    }

    void showError(Object e) {
      if (!context.mounted || isAccountSuspendedError(e)) return;
      final message = e is AppException
          ? e.message
          : errorText(e, defaults: ref.read(defaultErrorTextProvider));
      showFloatingToast(context, message, isSuccess: false);
    }

    // 進入時標記目前的聊天對象，避免橫幅與前景推播重複通知。
    useEffect(() {
      final gate = ref.read(activeChatGateProvider.notifier);
      gate.enter(storeId: arg.storeId, conversationId: thread.conversationId);
      return gate.clear;
    }, [thread.conversationId]);

    // 沒有對話可讀店家卡片時（對話還不存在），改用店家資料讀頭像與名稱。
    useEffect(() {
      final id = arg.storeId;
      if (id == null) return null;
      var cancelled = false;
      ref
          .read(sellerRepositoryProvider)
          .batchGetStores([id])
          .then((cards) {
            if (!cancelled) fetchedCard.value = cards[id];
          })
          .catchError((Object _) {});
      return () => cancelled = true;
    }, const []);

    // 對方在線狀態：載入完成後輪詢。
    useEffect(() {
      if (thread.conversationId == null) return null;
      void poll() =>
          unawaited(notifier.pollPresence().catchError((Object _) {}));
      poll();
      final timer = Timer.periodic(_presenceInterval, (_) => poll());
      return timer.cancel;
    }, [thread.conversationId]);

    // 新進來的訊息：入場動畫、未讀計數、在底部時自動捲到最新。
    ref.listen<ChatThreadState>(provider, (prev, next) {
      if (prev == null || prev.loadStatus != ChatLoadStatus.loaded) return;
      final before = prev.maxSeq;
      for (final m in next.messages) {
        if (m.seq <= before) continue;
        animatedKeys.value.add('m${m.seq}');
        if (m.sender == pb.ChatParty.CHAT_PARTY_STORE && !isNearBottom()) {
          unreadAway.value += 1;
        }
      }
      if (next.messages.length != prev.messages.length ||
          next.pending.length > prev.pending.length) {
        if (isNearBottom() || next.pending.length > prev.pending.length) {
          WidgetsBinding.instance.addPostFrameCallback((_) {
            if (!context.mounted) return;
            scrollToBottom();
          });
        }
      }
    });

    useEffect(() {
      void onScroll() {
        if (scrollCtrl.hasClients) {
          final nearTop =
              scrollCtrl.position.maxScrollExtent - scrollCtrl.position.pixels <
              _loadOlderThreshold;
          if (nearTop) {
            unawaited(notifier.loadOlder().then((_) {}, onError: showError));
          }
        }
        final nearBottom = isNearBottom();
        void apply() {
          if (!context.mounted) return;
          if (showFab.value == nearBottom) showFab.value = !nearBottom;
          if (nearBottom && unreadAway.value > 0) unreadAway.value = 0;
        }

        final phase = SchedulerBinding.instance.schedulerPhase;
        if (phase == SchedulerPhase.idle ||
            phase == SchedulerPhase.postFrameCallbacks) {
          apply();
        } else {
          WidgetsBinding.instance.addPostFrameCallback((_) => apply());
        }
      }

      scrollCtrl.addListener(onScroll);
      return () => scrollCtrl.removeListener(onScroll);
    }, const []);

    final StoreCard? card = thread.conversation?.hasStore() == true
        ? thread.conversation!.store
        : fetchedCard.value;
    final title = card?.displayName(locale) ?? storeName;
    final avatarUrl = card?.logoUri;

    Future<void> onSend() async {
      final t = textCtrl.text.trim();
      if (t.isEmpty) return;
      textCtrl.clear();
      try {
        await notifier.sendText(t);
      } catch (e) {
        showError(e);
      }
    }

    Future<void> onImagePick() async {
      try {
        final image = await ImagePicker().pickImage(
          source: ImageSource.gallery,
        );
        if (image == null) return;
        final upload = await ref
            .read(mediaRepositoryProvider)
            .uploadImage(file: File(image.path));
        await notifier.sendImage(upload.stagedToken, upload.previewUrl);
      } catch (e) {
        showError(e);
      }
    }

    Future<void> onRefresh() async {
      try {
        await notifier.refresh();
      } catch (e) {
        showError(e);
      }
    }

    void showMessageMenu(pb.Message m) {
      final fromStore = m.sender == pb.ChatParty.CHAT_PARTY_STORE;
      final copyText = m.hidden || m.erased
          ? null
          : (m.hasText() ? m.text : null);
      showModalBottomSheet<void>(
        context: context,
        showDragHandle: false,
        backgroundColor: AppColors.bgCard,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.vertical(top: Radius.circular(16.r)),
        ),
        builder: (ctx) => SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              SizedBox(height: 8.h),
              Container(
                width: 36.w,
                height: 4.h,
                decoration: BoxDecoration(
                  color: AppColors.gray300,
                  borderRadius: BorderRadius.circular(2.r),
                ),
              ),
              SizedBox(height: 12.h),
              if (copyText != null)
                ListTile(
                  leading: Icon(
                    AppIcons.copy,
                    color: AppIconColors.defaultColor,
                    size: 22.sp,
                  ),
                  title: const Text('複製', style: TextStyle(fontSize: 16)),
                  onTap: () {
                    Clipboard.setData(ClipboardData(text: copyText));
                    Navigator.pop(ctx);
                    HapticFeedback.lightImpact();
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(
                        content: Text('已複製'),
                        duration: Duration(seconds: 1),
                      ),
                    );
                  },
                ),
              if (fromStore) ...[
                ListTile(
                  leading: Icon(
                    AppIcons.reportOutlined,
                    color: AppIconColors.warning,
                    size: 22.sp,
                  ),
                  title: const Text('檢舉此訊息', style: TextStyle(fontSize: 16)),
                  onTap: () {
                    Navigator.pop(ctx);
                    showReportSheet(
                      context,
                      target: SocialReportTarget.chatMessage(m.id),
                    );
                  },
                ),
                ListTile(
                  leading: Icon(
                    AppIcons.blockOutlined,
                    color: AppIconColors.danger,
                    size: 22.sp,
                  ),
                  title: const Text('封鎖此使用者', style: TextStyle(fontSize: 16)),
                  onTap: () {
                    Navigator.pop(ctx);
                    showBlockConfirmDialog(
                      context,
                      target: ChatMessageBlockTarget(
                        m.id,
                        storeId: card?.id ?? arg.storeId,
                      ),
                      userLabel: title ?? '',
                    );
                  },
                ),
              ],
              SizedBox(height: 8.h),
            ],
          ),
        ),
      );
    }

    void showPendingMenu(PendingSend p) {
      HapticFeedback.heavyImpact();
      notifier.discard(p.requestId);
    }

    Widget buildMessageList() {
      if (thread.loadStatus == ChatLoadStatus.loading) {
        return const Center(child: HkBrandLoader(size: HkBrandLoader.page));
      }
      final rows = <_Row>[
        for (final m in thread.messages) _Row.message(m),
        for (final p in thread.pending) _Row.pending(p),
      ];
      final total = rows.length;
      return Stack(
        children: [
          const Positioned.fill(child: ChatConversationDotPattern()),
          ListView.builder(
            controller: scrollCtrl,
            physics: const AlwaysScrollableScrollPhysics(),
            padding: EdgeInsets.fromLTRB(12.w, 8.h, 12.w, 20.h),
            reverse: true,
            itemCount: total + 1,
            itemBuilder: (_, index) {
              if (index == 0) return const SizedBox(height: 1);
              final i = total - index;
              final row = rows[i];
              final prev = i > 0 ? rows[i - 1] : null;
              final next = i < total - 1 ? rows[i + 1] : null;
              final isSelf = row.isSelf;
              const gap = Duration(minutes: 5);
              final showTimeSeparator =
                  prev != null && row.time.difference(prev.time).abs() > gap;
              final sameSenderAsPrev =
                  prev != null && prev.isSelf == isSelf && !showTimeSeparator;
              final sameSenderAsNext =
                  next != null &&
                  next.isSelf == isSelf &&
                  next.time.difference(row.time).abs() <= gap;
              final isFirstInGroup = !sameSenderAsPrev;
              final isLastInGroup = !sameSenderAsNext;
              final shouldAnimate = animatedKeys.value.contains(row.key);

              Widget? timeSeparator;
              if (showTimeSeparator) {
                final t = row.time;
                final timeStr =
                    '${t.hour.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}';
                final today = DateTime.now();
                final isToday =
                    t.year == today.year &&
                    t.month == today.month &&
                    t.day == today.day;
                timeSeparator = Padding(
                  padding: EdgeInsets.symmetric(vertical: 12.h),
                  child: Center(
                    child: Container(
                      padding: EdgeInsets.symmetric(
                        horizontal: 12.w,
                        vertical: 4.h,
                      ),
                      decoration: BoxDecoration(
                        color: AppColors.textPrimary.withValues(alpha: 0.08),
                        borderRadius: BorderRadius.circular(12.r),
                      ),
                      child: Text(
                        isToday ? timeStr : '${t.month}/${t.day} $timeStr',
                        style: TextStyle(
                          fontSize: 12,
                          color: AppColors.textSecondary,
                        ),
                      ),
                    ),
                  ),
                );
              }

              Widget avatarSlot = const SizedBox.shrink();
              if (!isSelf) {
                avatarSlot = isLastInGroup
                    ? Container(
                        width: 32.w,
                        height: 32.w,
                        margin: EdgeInsets.only(bottom: 4.h, right: 8.w),
                        child: ClipOval(
                          child: (avatarUrl != null && avatarUrl.isNotEmpty)
                              ? OssImage(
                                  imageSource: avatarUrl,
                                  scene: ImageScene.chatAvatar,
                                  width: 32.w,
                                  height: 32.w,
                                  fit: BoxFit.cover,
                                  errorWidget: _AvatarInitial(
                                    name: title ?? '?',
                                  ),
                                )
                              : _AvatarInitial(name: title ?? '?'),
                        ),
                      )
                    : SizedBox(width: 32.w + 6.w);
              }

              final isImage = row.isImage;
              final String imageUrl = row.pending != null
                  ? (row.pending!.previewUrl ?? '')
                  : _messageImageUrl(row.message!, ImagePreset.chat);
              final String fullImageUrl = row.pending != null
                  ? (row.pending!.previewUrl ?? '')
                  : _messageImageUrl(row.message!, ImagePreset.original);
              final Widget content = isImage
                  ? _buildInlineImage(imageUrl)
                  : Text(
                      row.pending != null
                          ? (row.pending!.content.text ?? '')
                          : (chatMessageText(row.message!, locale) ?? ''),
                      softWrap: true,
                      style: TextStyle(
                        fontSize: 16,
                        color: isSelf
                            ? AppColors.bgCard
                            : AppColors.textEmphasis,
                        height: 1.5,
                      ),
                    );

              final t = row.time;
              final h12 = t.hour == 0
                  ? 12
                  : (t.hour > 12 ? t.hour - 12 : t.hour);
              final timeStr =
                  '${t.hour < 12 ? "上午" : "下午"}${h12.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}';
              final read =
                  row.message != null && row.message!.seq <= thread.peerReadSeq;
              final failed = row.pending?.failed ?? false;
              final timeStatusRow = Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (row.pending != null)
                    failed
                        ? Icon(
                            AppIcons.refresh,
                            size: 16.sp,
                            color: AppIconColors.danger,
                          )
                        : SizedBox(
                            width: 12.w,
                            height: 12.w,
                            child: const CircularProgressIndicator(
                              strokeWidth: 1.5,
                            ),
                          )
                  else ...[
                    Text(
                      timeStr,
                      style: TextStyle(
                        fontSize: 12,
                        color: AppColors.textSecondary,
                      ),
                    ),
                    if (isSelf) ...[
                      SizedBox(width: 4.w),
                      Text(
                        read ? '已讀' : '未讀',
                        style: TextStyle(
                          fontSize: 12,
                          color: AppColors.textSecondary,
                        ),
                      ),
                    ],
                  ],
                ],
              );

              Widget bubble = Align(
                alignment: isSelf
                    ? Alignment.centerRight
                    : Alignment.centerLeft,
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.end,
                  children: [
                    if (!isSelf) avatarSlot,
                    Flexible(
                      child: ConstrainedBox(
                        constraints: BoxConstraints(
                          maxWidth: MediaQuery.of(context).size.width * 0.72,
                        ),
                        child: GestureDetector(
                          behavior: HitTestBehavior.opaque,
                          onTap: () {
                            if (failed) {
                              unawaited(
                                notifier
                                    .retry(row.pending!.requestId)
                                    .then((_) {}, onError: showError),
                              );
                            } else if (isImage) {
                              _openFullScreenImage(context, fullImageUrl);
                            }
                          },
                          onLongPress: () {
                            if (row.pending != null) {
                              if (failed) showPendingMenu(row.pending!);
                              return;
                            }
                            HapticFeedback.heavyImpact();
                            showMessageMenu(row.message!);
                          },
                          child: Container(
                            margin: EdgeInsets.only(
                              top: isFirstInGroup ? 8.h : 4.h,
                            ),
                            child: Column(
                              crossAxisAlignment: isSelf
                                  ? CrossAxisAlignment.end
                                  : CrossAxisAlignment.start,
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                Container(
                                  padding: isImage
                                      ? EdgeInsets.zero
                                      : EdgeInsets.all(12.w),
                                  decoration: BoxDecoration(
                                    color: isSelf
                                        ? AppColors.primary
                                        : AppColors.bgCard,
                                    borderRadius: _smartBorderRadius(
                                      isSelf,
                                      isFirstInGroup,
                                      sameSenderAsNext,
                                    ),
                                    boxShadow: const [
                                      BoxShadow(
                                        color: AppColors.cardShadow,
                                        blurRadius: 3,
                                        offset: Offset(0, 1),
                                      ),
                                    ],
                                  ),
                                  child: content,
                                ),
                                Padding(
                                  padding: EdgeInsets.all(4.w),
                                  child: timeStatusRow,
                                ),
                              ],
                            ),
                          ),
                        ),
                      ),
                    ),
                    if (isSelf) SizedBox(width: 4.w),
                  ],
                ),
              );

              if (shouldAnimate) {
                Future.delayed(const Duration(milliseconds: 300), () {
                  animatedKeys.value.remove(row.key);
                });
                bubble = _MessageEntrance(isSelf: isSelf, child: bubble);
              }
              if (timeSeparator != null) {
                return Column(children: [timeSeparator, bubble]);
              }
              return bubble;
            },
          ),
        ],
      );
    }

    final messageArea = Stack(
      children: [
        FruitRefreshIndicator(onRefresh: onRefresh, child: buildMessageList()),
        if (showFab.value)
          Positioned(
            right: 16.w,
            bottom: 12.h,
            child: GestureDetector(
              onTap: () {
                scrollToBottom();
                unreadAway.value = 0;
              },
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
                    if (unreadAway.value > 0)
                      Positioned(
                        top: -8.h,
                        right: -8.w,
                        child: NotificationCountBadge(count: unreadAway.value),
                      ),
                  ],
                ),
              ),
            ),
          ),
      ],
    );

    final errorToShow = thread.loadStatus == ChatLoadStatus.failed
        ? thread.loadError
        : thread.syncError;
    final content = Column(
      children: [
        commonTopBar(
          context: context,
          horizontalPadding: EdgeInsets.zero,
          child: _TopBar(
            title: title,
            avatarUrl: avatarUrl,
            statusLabel: card?.statusLabel(locale),
            online: thread.peerOnline,
          ),
        ),
        Expanded(child: messageArea),
        if (errorToShow != null)
          Material(
            color: Colors.transparent,
            child: InkWell(
              onTap: () {
                if (thread.loadStatus == ChatLoadStatus.failed) {
                  unawaited(notifier.reload());
                } else {
                  unawaited(onRefresh());
                }
              },
              child: Container(
                width: double.infinity,
                padding: EdgeInsets.symmetric(vertical: 12.h, horizontal: 16.w),
                color: AppColors.dangerSoft,
                child: Text(
                  errorText(
                    errorToShow,
                    defaults: ref.read(defaultErrorTextProvider),
                  ),
                  textAlign: TextAlign.center,
                  style: TextStyle(fontSize: 14, color: AppColors.warningText),
                ),
              ),
            ),
          ),
        if (thread.readOnlyMessage != null)
          Container(
            width: double.infinity,
            padding: EdgeInsets.symmetric(vertical: 16.h, horizontal: 16.w),
            color: AppColors.bgCard,
            child: Text(
              thread.readOnlyMessage!,
              textAlign: TextAlign.center,
              style: TextStyle(fontSize: 14, color: AppColors.textSecondary),
            ),
          )
        else
          ChatInputBar(
            controller: textCtrl,
            focusNode: inputFocusNode,
            onSend: () => unawaited(onSend()),
            onPlusPressed: () => unawaited(onImagePick()),
          ),
      ],
    );

    return Scaffold(backgroundColor: AppColors.bgPrimary, body: content);
  }
}

class _TopBar extends StatelessWidget {
  const _TopBar({
    required this.title,
    required this.avatarUrl,
    required this.statusLabel,
    required this.online,
  });

  final String? title;
  final String? avatarUrl;
  final String? statusLabel;
  final bool online;

  Widget _buildAvatar() {
    const double size = 40;
    final url = (avatarUrl ?? '').trim();
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        border: Border.all(color: AppColors.borderStrong, width: 1.8),
        color: AppColors.surfaceMuted,
      ),
      clipBehavior: Clip.antiAlias,
      child: ClipOval(
        child: url.isEmpty
            ? const SizedBox(
                width: size,
                height: size,
                child: HkPlaceholder(HkPlaceholderKind.merchantAvatar),
              )
            : AppCachedImage(
                imageUrl: url,
                scene: ImageScene.chatAvatar,
                fit: BoxFit.cover,
                width: size,
                height: size,
              ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return ChatConversationHeaderContent(
      avatar: _buildAvatar(),
      title: title ?? '',
      statusLabel: statusLabel ?? (online ? '線上' : '離線'),
      statusColor: statusLabel == null && online
          ? AppColors.success.withValues(alpha: 0.85)
          : AppColors.textDisabled,
      onBack: () => context.pop(),
    );
  }
}

/// Smart border radius — Telegram-style grouped bubble corners
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

/// Typing indicator — three animated dots, staggered 200ms apart, 600ms period
/// Telegram 風格的 typing indicator：

/// Message entrance animation — slide up + fade in (200ms)
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

/// Full-screen image viewer — InteractiveViewer, min 0.5x max 4.0x, close button
class _ImageViewerPage extends StatelessWidget {
  const _ImageViewerPage({required this.url});
  final String url;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => Navigator.pop(context),
      onVerticalDragEnd: (d) {
        if (d.primaryVelocity != null && d.primaryVelocity!.abs() > 300) {
          Navigator.pop(context);
        }
      },
      child: Scaffold(
        backgroundColor: Colors.transparent,
        body: Stack(
          children: [
            Center(
              child: InteractiveViewer(
                minScale: 0.5,
                maxScale: 4.0,
                child: OssImage(
                  imageSource: url,
                  scene: ImageScene.fullscreen,
                  fit: BoxFit.contain,
                  errorWidget: Icon(
                    AppIcons.brokenImage,
                    size: 48.sp,
                    color: AppIconColors.onBrand,
                  ),
                ),
              ),
            ),
            Positioned(
              top: MediaQuery.of(context).padding.top + 8,
              right: 12,
              child: GestureDetector(
                onTap: () => Navigator.pop(context),
                child: Container(
                  padding: const EdgeInsets.all(8),
                  decoration: BoxDecoration(
                    color: AppColors.textSecondary,
                    shape: BoxShape.circle,
                  ),
                  child: const Icon(
                    AppIcons.close,
                    color: AppIconColors.onBrand,
                    size: 20,
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Avatar initial fallback
class _AvatarInitial extends StatelessWidget {
  const _AvatarInitial({required this.name});
  final String name;

  @override
  Widget build(BuildContext context) {
    final initial = name.isNotEmpty ? name[0].toUpperCase() : '?';
    return Container(
      color: AppColors.primary,
      child: Center(
        child: Text(
          initial,
          style: const TextStyle(
            color: AppColors.bgCard,
            fontSize: 14,
            fontWeight: FontWeight.bold,
          ),
        ),
      ),
    );
  }
}
