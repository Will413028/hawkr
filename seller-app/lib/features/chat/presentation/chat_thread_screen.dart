import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:image_picker/image_picker.dart';

import '../../../config/image_config.dart';
import '../../../core/l10n/ui_locale.dart';
import '../../../core/motion/hk_motion_transitions.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/utils/image_format_detector.dart';
import '../../../shared/widgets/chat_conversation_chrome.dart';
import '../../../shared/widgets/chat_input_bar.dart';
import '../../../shared/widgets/hk_async_state_panel.dart';
import '../../../shared/widgets/optimized_image.dart';
import '../../dashboard/data/dashboard_chat_unread_provider.dart';
import '../../report/presentation/widgets/block_actions.dart';
import '../../report/presentation/widgets/report_sheet.dart';
import '../../seller/data/current_store.dart';
import '../../seller/data/seller_repository.dart';
import '../data/active_chat_gate.dart';
import '../data/chat_model.dart';
import '../data/chat_repository.dart';
import '../data/chat_thread_notifier.dart';
import 'chat_preview.dart';
import 'chat_send_error_dispatch.dart';

/// 對方在線狀態的輪詢間隔（`GetStorePresence`；沒有推送）。
const _presenceInterval = Duration(seconds: 30);

class ChatThreadScreen extends HookConsumerWidget {
  const ChatThreadScreen({
    super.key,
    required this.title,
    this.conversationId,
    this.participantId,
  });

  final String title;
  final String? conversationId;
  final String? participantId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final arg = ChatThreadArg(
      peerId: participantId ?? '',
      conversationId: conversationId,
    );
    final thread = ref.watch(chatThreadProvider(arg));
    final notifier = ref.read(chatThreadProvider(arg).notifier);
    final locale = ref.watch(uiLocaleProvider);
    final scrollCtrl = useScrollController();
    final inputCtrl = useTextEditingController();
    final inputFocus = useFocusNode();
    final sending = useState(false);
    final readOnlyMessage = useState<String?>(null);
    final isPeerOnline = useState(false);
    final data = thread.value;
    final convId = data?.conversationId;

    // 標記使用者正在這個聊天室裡，讓前景通知跳過同一個對話。
    useEffect(() {
      final peerVal = (participantId ?? '').isNotEmpty ? participantId : null;
      final convVal = (conversationId ?? '').isNotEmpty ? conversationId : null;
      final gate = ref.read(activeChatGateProvider.notifier);
      if (peerVal != null || convVal != null) {
        Future.microtask(
          () => gate.enter(peerId: peerVal, conversationId: convVal),
        );
      }
      return () => Future.microtask(gate.clear);
    }, [participantId, conversationId]);

    useEffect(() {
      if (convId == null) return null;
      Future.microtask(
        () => ref
            .read(activeChatGateProvider.notifier)
            .updateConversationId(convId),
      );
      return null;
    }, [convId]);

    // 對方是否在線：進頁後與每 30 秒讀一次。
    useEffect(() {
      if (convId == null) return null;
      var disposed = false;
      Future<void> poll() async {
        try {
          final res = await ref.read(chatRepositoryProvider).presence([convId]);
          if (!disposed) isPeerOnline.value = res[convId] ?? false;
        } on Object {
          if (!disposed) isPeerOnline.value = false;
        }
      }

      unawaited(poll());
      final timer = Timer.periodic(_presenceInterval, (_) => poll());
      return () {
        disposed = true;
        timer.cancel();
      };
    }, [convId]);

    // 捲到最舊端時載入更早的訊息（列表是反向的，最舊的在 maxScrollExtent）。
    useEffect(() {
      void onScroll() {
        if (!scrollCtrl.hasClients) return;
        final pos = scrollCtrl.position;
        if (pos.maxScrollExtent - pos.pixels < 200) {
          unawaited(notifier.loadOlder());
        }
      }

      scrollCtrl.addListener(onScroll);
      return () => scrollCtrl.removeListener(onScroll);
    }, const []);

    // 新訊息到尾端時，使用者在底部附近就跟著捲下去。
    final lastCount = data?.messages.length ?? 0;
    useEffect(() {
      if (scrollCtrl.hasClients && scrollCtrl.position.pixels < 120) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (scrollCtrl.hasClients) scrollCtrl.jumpTo(0);
        });
      }
      return null;
    }, [lastCount]);

    final peerName = (data?.buyer?.deleted ?? false)
        ? buyerProfileDeletedName(locale)
        : (data?.buyer?.name ?? (title.isNotEmpty ? title : '顧客'));
    final peerAvatar = data?.buyer?.avatarUri;

    void showSendError(Object e) {
      final failure = classifyChatSendError(
        e,
        defaults: ref.read(defaultErrorTextProvider),
      );
      if (failure.readOnly) readOnlyMessage.value = failure.message;
      if (context.mounted) FloatingToast.error(context, failure.message);
    }

    Future<void> onSendText() async {
      final text = inputCtrl.text.trim();
      if (text.isEmpty || sending.value) return;
      inputCtrl.clear();
      sending.value = true;
      try {
        await notifier.sendText(text);
        ref.invalidate(dashboardChatUnreadProvider);
      } on Object catch (e) {
        showSendError(e);
      } finally {
        if (context.mounted) sending.value = false;
      }
    }

    Future<void> onPickPhoto() async {
      try {
        final picked = await ImagePicker().pickImage(
          source: ImageSource.gallery,
          maxWidth: 2048.w,
          imageQuality: 85,
        );
        if (picked == null) return;
        final bytes = await picked.readAsBytes();
        if (bytes.isEmpty) return;
        final mid = ref.read(sellerStoreIdProvider);
        if (mid == null) {
          if (context.mounted) {
            FloatingToast.error(context, '無法取得商家ID，請重新登入後再試');
          }
          return;
        }
        sending.value = true;
        final (ext, mime) = ImageFormatDetector.detect(bytes);
        final upload = await ref
            .read(sellerRepositoryProvider)
            .uploadMedia(
              sellerId: mid,
              bytes: bytes,
              filename: 'chat_${DateTime.now().millisecondsSinceEpoch}.$ext',
              mimeType: mime,
            );
        final stagedToken = upload['data']?['stagedToken']?.toString();
        if (stagedToken == null || stagedToken.isEmpty) {
          throw StateError('upload returned no staged token');
        }
        await notifier.sendImage(stagedToken);
      } on Object catch (e) {
        HKLog.logD('[ChatThread] image send error: $e');
        showSendError(e);
      } finally {
        if (context.mounted) sending.value = false;
      }
    }

    Future<void> retry(ChatMessage m) async {
      final rid = m.requestId;
      if (rid == null) return;
      try {
        await notifier.retry(rid);
      } on Object catch (e) {
        showSendError(e);
      }
    }

    Widget bubble(ChatMessage m, {required bool first, required bool hasNext}) {
      final mine = m.fromStore;
      final tomb = tombstoneText(m, locale);
      Widget content;
      if (tomb != null) {
        content = Text(
          tomb,
          style: TextStyle(
            fontSize: 16.sp,
            fontStyle: FontStyle.italic,
            color: mine ? AppIconColors.onBrand : AppColors.textSecondary,
          ),
        );
      } else if (m.kind == pb.MessageKind.MESSAGE_KIND_IMAGE) {
        final uri = m.imageUri ?? '';
        content = GestureDetector(
          onTap: () => _openImageViewer(context, uri),
          child: ClipRRect(
            borderRadius: BorderRadius.circular(12.r),
            child: uri.isEmpty
                ? const SizedBox.shrink()
                : OptimizedNetworkImage(
                    imageUrl: uri,
                    scene: ImageScene.chatImage,
                    width: 200.w,
                    fit: BoxFit.cover,
                    errorWidget: Icon(AppIcons.brokenImage, size: 32.sp),
                  ),
          ),
        );
      } else if (m.kind == pb.MessageKind.MESSAGE_KIND_TEXT) {
        content = Text(
          m.text ?? '',
          style: TextStyle(
            fontSize: 16.sp,
            height: 1.4,
            color: mine ? AppIconColors.onBrand : AppColors.textPrimary,
          ),
        );
      } else {
        content = Text(
          chatMessageUnsupportedKind(locale),
          style: TextStyle(
            fontSize: 16.sp,
            fontStyle: FontStyle.italic,
            color: mine ? AppIconColors.onBrand : AppColors.textSecondary,
          ),
        );
      }
      final read = mine && m.seq > 0 && m.seq <= (data?.peerReadSeq ?? 0);
      return Padding(
        padding: EdgeInsets.only(top: first ? 8.h : 3.h),
        child: Column(
          crossAxisAlignment: mine
              ? CrossAxisAlignment.end
              : CrossAxisAlignment.start,
          children: [
            GestureDetector(
              onLongPress: m.isPending
                  ? null
                  : () => _showMessageMenu(context, ref, m, peerName),
              onTap: m.sendState == ChatSendState.failed
                  ? () => retry(m)
                  : null,
              child: Container(
                constraints: BoxConstraints(maxWidth: 280.w),
                padding: EdgeInsets.all(
                  m.kind == pb.MessageKind.MESSAGE_KIND_IMAGE && tomb == null
                      ? 4.w
                      : 12.w,
                ),
                decoration: BoxDecoration(
                  color: mine ? AppColors.primary : AppColors.bgCard,
                  borderRadius: _smartBorderRadius(mine, first, hasNext),
                ),
                child: content,
              ),
            ),
            if (m.sendState == ChatSendState.failed)
              Padding(
                padding: EdgeInsets.only(top: 2.h),
                child: Text(
                  '送出失敗，點一下重送',
                  key: const ValueKey('chat-send-failed'),
                  style: TextStyle(fontSize: 12.sp, color: AppColors.danger),
                ),
              )
            else if (m.sendState == ChatSendState.sending)
              Padding(
                padding: EdgeInsets.only(top: 2.h),
                child: Text(
                  '傳送中…',
                  style: TextStyle(
                    fontSize: 12.sp,
                    color: AppColors.textSecondary,
                  ),
                ),
              )
            else if (!hasNext)
              Padding(
                padding: EdgeInsets.only(top: 2.h),
                child: Text(
                  '${read ? '已讀 ' : ''}${_hhmm(m.createTime)}',
                  style: TextStyle(
                    fontSize: 12.sp,
                    color: AppColors.textSecondary,
                  ),
                ),
              ),
          ],
        ),
      );
    }

    Widget body() {
      return thread.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => Center(
          child: HkAsyncStatePanel(
            kind: HkAsyncStateKind.error,
            title: '目前無法載入訊息',
            description: '請確認網路連線後再試一次。',
            actionLabel: '重新載入',
            onAction: notifier.reload,
          ),
        ),
        data: (d) {
          if (d.messages.isEmpty) {
            return const Center(child: Text('尚無訊息'));
          }
          final list = d.messages.reversed.toList();
          return Stack(
            children: [
              const Positioned.fill(child: ChatConversationDotPattern()),
              ListView.builder(
                key: const ValueKey('chat-message-list'),
                controller: scrollCtrl,
                reverse: true,
                padding: EdgeInsets.fromLTRB(16.w, 12.h, 16.w, 20.h),
                itemCount: list.length,
                itemBuilder: (context, i) {
                  final m = list[i];
                  // reverse：i+1 是時間上更早的一則。
                  final older = i + 1 < list.length ? list[i + 1] : null;
                  final newer = i > 0 ? list[i - 1] : null;
                  final first = older == null || older.fromStore != m.fromStore;
                  final hasNext =
                      newer != null && newer.fromStore == m.fromStore;
                  return Align(
                    alignment: m.fromStore
                        ? Alignment.centerRight
                        : Alignment.centerLeft,
                    child: bubble(m, first: first, hasNext: hasNext),
                  );
                },
              ),
            ],
          );
        },
      );
    }

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      resizeToAvoidBottomInset: true,
      body: Column(
        children: [
          ChatConversationTopBar(
            avatar: ClipOval(
              child: (peerAvatar != null && peerAvatar.isNotEmpty)
                  ? OptimizedNetworkImage(
                      imageUrl: peerAvatar,
                      scene: ImageScene.chatAvatar,
                      fit: BoxFit.cover,
                      width: 40,
                      height: 40,
                      errorWidget: _PeerAvatarInitial(name: peerName),
                    )
                  : _PeerAvatarInitial(name: peerName),
            ),
            title: peerName,
            statusLabel: isPeerOnline.value ? '線上' : '離線',
            statusColor: isPeerOnline.value
                ? AppColors.success
                : AppColors.textHint,
            onBack: () => context.pop(),
          ),
          Expanded(child: body()),
          ChatInputBar(
            controller: inputCtrl,
            focusNode: inputFocus,
            sending: sending.value,
            onSend: onSendText,
            onPlusPressed: onPickPhoto,
            readOnlyMessage: readOnlyMessage.value,
          ),
        ],
      ),
    );
  }
}

String _hhmm(DateTime t) {
  final l = t.toLocal();
  return '${l.hour.toString().padLeft(2, '0')}:${l.minute.toString().padLeft(2, '0')}';
}

/// 智能圓角 — 連續同一人訊息合併角落（Telegram 風格）
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

/// 長按訊息選單：複製（有文字時），買家的訊息可檢舉訊息與封鎖買家（以訊息 ID 當參照）。
void _showMessageMenu(
  BuildContext context,
  WidgetRef ref,
  ChatMessage msg,
  String peerName,
) {
  showModalBottomSheet<void>(
    context: context,
    backgroundColor: AppColors.bgCard,
    shape: RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(16.r)),
    ),
    builder: (ctx) => SafeArea(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(height: 12.h),
          if (msg.text != null && !msg.isTombstone)
            ListTile(
              leading: Icon(
                AppIcons.copy,
                color: AppIconColors.defaultColor,
                size: 22.sp,
              ),
              title: Text('複製', style: TextStyle(fontSize: 16.sp)),
              onTap: () {
                Clipboard.setData(ClipboardData(text: msg.text!));
                Navigator.pop(ctx);
                HapticFeedback.lightImpact();
              },
            ),
          if (!msg.fromStore) ...[
            ListTile(
              leading: Icon(
                AppIcons.reportOutlined,
                color: AppIconColors.defaultColor,
                size: 22.sp,
              ),
              title: Text('檢舉訊息', style: TextStyle(fontSize: 16.sp)),
              onTap: () {
                Navigator.pop(ctx);
                showReportSheet(context, ref, messageId: msg.id);
              },
            ),
            ListTile(
              leading: Icon(
                AppIcons.blockOutlined,
                color: AppIconColors.danger,
                size: 22.sp,
              ),
              title: Text(
                '封鎖此人',
                style: TextStyle(fontSize: 16.sp, color: AppColors.danger),
              ),
              onTap: () {
                Navigator.pop(ctx);
                blockBuyerByMessageWithConfirm(
                  context,
                  ref,
                  messageId: msg.id,
                  buyerName: peerName,
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

/// 圖片全螢幕檢視
void _openImageViewer(BuildContext context, String url) {
  if (url.isEmpty) return;

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
      pageBuilder: (_, _, _) => _ImageViewerPage(url: url),
      transitionsBuilder: (transitionContext, animation, _, child) =>
          HkMotionTransitions.modal(
            context: transitionContext,
            animation: animation,
            presentation: HkModalPresentation.photoZoom,
            child: child,
          ),
    ),
  );
}

/// 全螢幕圖片檢視頁面 — 捏合縮放 + 點擊/下滑關閉
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
        body: Center(
          child: InteractiveViewer(
            minScale: 0.5,
            maxScale: 4.0,
            child: OptimizedNetworkImage(
              imageUrl: url,
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
      ),
    );
  }
}

class _PeerAvatarInitial extends StatelessWidget {
  const _PeerAvatarInitial({required this.name});
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
