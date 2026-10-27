import '../../../features/seller/data/current_store.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'dart:convert';
import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';

import '../data/active_chat_gate.dart';
import '../data/chat_model.dart';
import '../data/chat_repository.dart';
import '../data/chat_thread_notifier.dart';
import '../../../core/analytics/analytics_providers.dart';
import '../../../core/motion/hk_motion_transitions.dart';
import '../../../config/image_config.dart';
import '../../../features/seller/data/seller_repository.dart';
import '../../customer/data/customer_repository.dart';
import '../../media/data/media_repository.dart';
import '../../../shared/utils/keyboard_unfocus_controller.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/chat_conversation_chrome.dart';
import '../../../shared/widgets/chat_input_bar.dart';
import '../../../shared/widgets/chat_order_bubble.dart';
import '../../../shared/widgets/chat_product_bubble.dart';
import '../../../shared/widgets/draggable_reply_wrapper.dart';
import '../../../shared/utils/image_format_detector.dart';
import '../../../shared/widgets/optimized_image.dart';
import '../../report/data/report_model.dart';
import '../../report/presentation/widgets/block_actions.dart';
import '../../report/presentation/widgets/report_sheet.dart';
import 'chat_attachment_picker.dart';
import 'chat_card.dart';
import 'chat_send_error_dispatch.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

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
    // 訊息清單 / dedup / WS 路由 / reconnect-refetch / send 已全由 notifier 擁有；
    // widget 只 watch 結果並保留 presence / typing / scroll / reply 等 UI 狀態。
    final thread = ref.watch(chatThreadProvider(arg));
    List<ChatMessage> readMessages() =>
        ref.read(chatThreadProvider(arg)).value ?? const <ChatMessage>[];
    final scrollCtrl = useScrollController();
    final inputCtrl = useTextEditingController();
    final inputFocus = useFocusNode();
    final sending = useState(false);
    final openSettleTimer = useState<Timer?>(null);
    final convSellerId = useState<String?>(null);
    final autoScrollPending = useState(false);
    final bottomKey = useMemoized(() => GlobalKey(), const []);
    final scrollAnimating = useState(false);
    final lastViewInset = useState(0.0);

    final peerAvatarUrl = useState<String?>(null);
    final peerAvatarFetching = useState(false);
    final peerAvatarFailed = useState(false);
    // Pre-fill from route title (targetNickname from conversation list) so
    // the name shows immediately even if the customer API fails.
    final peerName = useState<String?>(title.isNotEmpty ? title : null);
    final peerNameFetching = useState(false);

    final replyTarget = useState<ChatMessage?>(null);
    final replyPreview = useState<String?>(null);

    // 打字指示器：偵測 TextField 輸入 → 發送 WebSocket typing 事件
    useEffect(() {
      Timer? debounce;
      bool sentTyping = false;
      void listener() {
        final hasText = inputCtrl.text.trim().isNotEmpty;
        if (hasText && !sentTyping) {
          sentTyping = true;
          ref
              .read(chatRepositoryProvider)
              .sendTypingIndicator(
                receiverId: participantId ?? '',
                isTyping: true,
              );
        }
        debounce?.cancel();
        debounce = Timer(const Duration(seconds: 3), () {
          if (sentTyping) {
            sentTyping = false;
            ref
                .read(chatRepositoryProvider)
                .sendTypingIndicator(
                  receiverId: participantId ?? '',
                  isTyping: false,
                );
          }
        });
      }

      inputCtrl.addListener(listener);
      return () {
        inputCtrl.removeListener(listener);
        debounce?.cancel();
        if (sentTyping) {
          ref
              .read(chatRepositoryProvider)
              .sendTypingIndicator(
                receiverId: participantId ?? '',
                isTyping: false,
              );
        }
      };
    }, const []);

    // 標記使用者正在這個聊天室裡，讓 FCM 前景 banner 跳過同一聊天的通知，
    // 避免「人就在裡面還跳 toast」的雙重通知體驗。
    // participantId 進場時就有；conversationId 可能 null（從顧客頁直接開），
    // 等 WS 第一則訊息回來才補（messageHandler 內補一次）。
    useEffect(() {
      final peerVal = (participantId ?? '').isNotEmpty ? participantId! : null;
      final convVal = (conversationId ?? '').isNotEmpty ? conversationId : null;
      // useEffect 在 build 期間同步執行，直接寫 provider 會拋
      // "Tried to modify a provider while the widget tree was building"。
      // 先取 notifier（keepAlive）再延後到 microtask 寫入。
      final gate = ref.read(activeChatGateProvider.notifier);
      if (peerVal != null || convVal != null) {
        Future.microtask(
          () => gate.enter(peerId: peerVal, conversationId: convVal),
        );
      }
      return () => Future.microtask(gate.clear);
    }, [participantId, conversationId]);

    // 對方在線狀態（WebSocket presence push，買家 peerRole=1）
    final isPeerOnline = useState(false);

    // 新訊息動畫追蹤
    final animatedMsgIds = useState<Set<String>>({});
    // 捲動到底部 FAB
    final showScrollFab = useState(false);
    final unreadWhileAway = useState(0);
    // Fix #3：一次性進場捲動 guard（reconnect refetch 不得重觸發捲到底）。
    // Fix #2：追蹤最後一則 tail 訊息 id，用來辨別「新增在尾端」的訊息。
    final didInitialScroll = useRef(false);
    final lastTailId = useRef<String?>(null);
    // 打字指示器
    final isPeerTyping = useState(false);

    // hook 仍需呼叫以保持 hooks call order 穩定；變數未實際使用所以不命名。
    useState<Map<String, double>>({});
    // Use a plain mutable map (not useState) so that adding new keys inside build
    // does NOT trigger setState/markNeedsBuild.
    final msgKeys = useMemoized(() => <String, GlobalKey>{}, const []);
    final highlightIds = useState<Set<String>>({});
    final messageHeights = useState<Map<String, double>>({});
    final averageMessageHeight = useState(84.0);

    GlobalKey messageKeyFor(String id) {
      return msgKeys.putIfAbsent(id, GlobalKey.new);
    }

    void recordMessageHeight(String id, Size? size) {
      if (size == null) return;
      final height = math.max(20.0, size.height + 8.0);
      final prev = messageHeights.value[id];
      if (prev != null && (prev - height).abs() < 0.5) return;
      messageHeights.value = {...messageHeights.value, id: height};
      double total = 0;
      for (final value in messageHeights.value.values) {
        total += value;
      }
      if (messageHeights.value.isNotEmpty) {
        averageMessageHeight.value = total / messageHeights.value.length;
      }
    }

    double estimateOffsetForIndex(int index) {
      final msgs = readMessages();
      if (msgs.isEmpty) return 0.0;
      if (index >= msgs.length - 1) return 0.0;
      double offset = 0.0;
      final double fallback = math.max(
        48.0,
        math.min(280.0, averageMessageHeight.value),
      );
      for (var i = msgs.length - 1; i > index; i--) {
        final id = msgs[i].id;
        offset += messageHeights.value[id] ?? fallback;
      }
      return offset;
    }

    void scrollToBottom() {
      if (scrollAnimating.value) return;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!scrollCtrl.hasClients) return;
        final target = scrollCtrl.position.minScrollExtent;
        final delta = scrollCtrl.position.pixels - target;
        if (delta.abs() < 48) {
          scrollCtrl.jumpTo(target);
          return;
        }
        scrollAnimating.value = true;
        if (bottomKey.currentContext != null) {
          Scrollable.ensureVisible(
            bottomKey.currentContext!,
            duration: const Duration(milliseconds: 180),
            alignment: 1.0,
            curve: Curves.easeOut,
          ).whenComplete(() => scrollAnimating.value = false);
          return;
        }
        scrollCtrl
            .animateTo(
              target,
              duration: const Duration(milliseconds: 180),
              curve: Curves.easeOut,
            )
            .whenComplete(() => scrollAnimating.value = false);
      });
    }

    void scrollToBottomOnOpen() {
      if (!autoScrollPending.value) return;
      if (readMessages().isEmpty) {
        autoScrollPending.value = false;
        return;
      }
      autoScrollPending.value = false;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!scrollCtrl.hasClients) return;
        final target = scrollCtrl.position.minScrollExtent;
        scrollCtrl.jumpTo(target);
        openSettleTimer.value?.cancel();
        openSettleTimer.value = Timer(const Duration(milliseconds: 180), () {
          if (!scrollCtrl.hasClients) return;
          final settleTarget = scrollCtrl.position.minScrollExtent;
          scrollCtrl.jumpTo(settleTarget);
        });
      });
    }

    bool isNearBottom() {
      if (!scrollCtrl.hasClients) return true;
      final pos = scrollCtrl.position;
      final distance = (pos.pixels - pos.minScrollExtent).abs();
      return distance < 40;
    }

    // 監聽捲動位置顯示 FAB
    useEffect(() {
      void onScroll() {
        final near = isNearBottom();
        if (showScrollFab.value == near) showScrollFab.value = !near;
        if (near) unreadWhileAway.value = 0;
      }

      scrollCtrl.addListener(onScroll);
      return () => scrollCtrl.removeListener(onScroll);
    }, const []);

    String resolveReceiverId() {
      final meId = (convSellerId.value ?? ref.read(sellerStoreIdProvider) ?? '')
          .trim();
      final pid = (participantId ?? '').trim();
      if (pid.isNotEmpty && pid != meId) return pid;
      final msgs = readMessages();
      if (msgs.isNotEmpty) {
        final last = msgs.last;
        final peer = (last.senderId == meId || last.senderId.startsWith('m'))
            ? last.receiverId
            : last.senderId;
        if (peer.isNotEmpty && peer != meId) return peer;
      }
      final conv = (conversationId ?? '').trim();
      if (conv.contains('_')) {
        final parts = conv.split('_').where((p) => p.isNotEmpty).toList();
        if (parts.length >= 2) {
          final candidate = parts.firstWhere(
            (p) => p != meId,
            orElse: () => '',
          );
          if (candidate.isNotEmpty && candidate != meId) return candidate;
        }
      }
      HKLog.logD(
        '[ChatThread] resolveReceiverId failed: meId="$meId" pid="$pid" conv="$conversationId"',
      );
      return '';
    }

    bool isSelfMessage(ChatMessage m) {
      final meConv = (convSellerId.value ?? '').trim();
      final meNum = (ref.read(sellerStoreIdProvider) ?? '').trim();
      final s = (m.senderId).trim();
      // 空 senderId = 自己剛送出的樂觀訊息（notifier.send 尚未填 senderId）；
      // 對方/歷史訊息一定帶真實 senderId，故空值必為本人。
      if (s.isEmpty) return true;
      if (meConv.isNotEmpty && s == meConv) return true;
      if (meNum.isNotEmpty) {
        if (s == meNum) return true;
        final coreS = s.replaceAll(RegExp(r'[^0-9]'), '');
        final coreMe = meNum.replaceAll(RegExp(r'[^0-9]'), '');
        if (coreS.isNotEmpty && coreS == coreMe) return true;
        final low = s.toLowerCase();
        if (low == 'm$meNum'.toLowerCase() ||
            low == 'merchant_$meNum'.toLowerCase() ||
            low == 'merchant$meNum'.toLowerCase()) {
          return true;
        }
      }
      if (s.toLowerCase().startsWith('m')) return true;
      return false;
    }

    Future<void> ensurePeerAvatar({String? idHint}) async {
      if (peerAvatarUrl.value != null ||
          peerAvatarFetching.value ||
          peerAvatarFailed.value) {
        return;
      }
      String pid = (idHint ?? participantId ?? '').trim();
      if (pid.isEmpty) pid = resolveReceiverId();
      if (pid.isEmpty) return;

      peerAvatarFetching.value = true;
      try {
        // 買家頭像從顧客 API 取得，非賣家 API
        final customer = await ref
            .read(customerRepositoryProvider)
            .getCustomerByAccountId(pid);
        if (customer != null) {
          final rawUrl = customer.avatarUrl?.toString() ?? '';
          final url = rawUrl.isNotEmpty
              ? (MediaRepository.resolveUrl(rawUrl) ?? rawUrl)
              : '';
          if (context.mounted) {
            if (url.isNotEmpty) peerAvatarUrl.value = url;
            if (peerName.value == null && customer.name.isNotEmpty) {
              peerName.value = customer.name;
            }
          }
        }
      } catch (_) {
        if (context.mounted) peerAvatarFailed.value = true;
      } finally {
        if (context.mounted) peerAvatarFetching.value = false;
      }
    }

    Future<void> ensurePeerName({String? idHint}) async {
      // Already pre-filled from title or previous fetch; skip API call.
      if (peerName.value != null || peerNameFetching.value) return;
      String pid = (idHint ?? participantId ?? '').trim();
      if (pid.isEmpty) pid = resolveReceiverId();
      if (pid.isEmpty) return;

      peerNameFetching.value = true;
      try {
        final customer = await ref
            .read(customerRepositoryProvider)
            .getCustomerByAccountId(pid);
        if (customer != null && customer.name.isNotEmpty && context.mounted) {
          peerName.value = customer.name;
        }
      } catch (_) {
      } finally {
        if (context.mounted) peerNameFetching.value = false;
      }
    }

    Future<bool> ensureMessageLoaded(String id) async {
      final notifier = ref.read(chatThreadProvider(arg).notifier);
      if (readMessages().any((m) => m.id == id)) return true;
      if (readMessages().isEmpty || !notifier.hasMoreOlderHistory) return false;
      var iterations = 0;
      while (context.mounted &&
          notifier.hasMoreOlderHistory &&
          iterations < 20) {
        final earliest = readMessages().first.id;
        if (earliest.isEmpty) break;
        final loaded = await notifier.loadOlder();
        if (readMessages().any((m) => m.id == id)) return true;
        if (!loaded) break;
        iterations++;
      }
      return readMessages().any((m) => m.id == id);
    }

    Future<void> waitForScrollMetrics() async {
      if (!scrollCtrl.hasClients) return;
      var attempts = 0;
      while (!scrollCtrl.position.hasContentDimensions && attempts < 8) {
        await Future.delayed(const Duration(milliseconds: 16));
        if (!scrollCtrl.hasClients) return;
        attempts++;
      }
    }

    Future<void> jumpToMessageId(String id) async {
      if (id.isEmpty) return;
      if (!readMessages().any((m) => m.id == id)) {
        final found = await ensureMessageLoaded(id);
        if (!found) {
          if (!context.mounted) return;
          FloatingToast.error(context, '找不到原始訊息');
          return;
        }
        if (!context.mounted) return;
        await Future.delayed(const Duration(milliseconds: 16));
      }

      final key = messageKeyFor(id);
      BuildContext? ctx = key.currentContext;

      if (ctx == null) {
        await waitForScrollMetrics();
        if (context.mounted && scrollCtrl.hasClients) {
          final targetIndex = readMessages().indexWhere((m) => m.id == id);
          if (targetIndex >= 0) {
            for (var attempt = 0; attempt < 6 && context.mounted; attempt++) {
              final position = scrollCtrl.position;
              final estimated = estimateOffsetForIndex(targetIndex);
              final minScroll = position.minScrollExtent;
              final maxScroll = position.maxScrollExtent;
              final desired = (minScroll + estimated).clamp(
                minScroll,
                maxScroll,
              );
              if ((position.pixels - desired).abs() <= 3.0 && attempt > 0) {
                break;
              }
              await scrollCtrl.animateTo(
                desired,
                duration: const Duration(milliseconds: 260),
                curve: Curves.easeInOut,
              );
              await Future.delayed(const Duration(milliseconds: 40));
              ctx = key.currentContext;
              if (ctx != null) break;
            }
          }
        }
        if (ctx == null && context.mounted && scrollCtrl.hasClients) {
          await scrollCtrl.animateTo(
            scrollCtrl.position.maxScrollExtent,
            duration: const Duration(milliseconds: 260),
            curve: Curves.easeInOut,
          );
          await Future.delayed(const Duration(milliseconds: 40));
          ctx = key.currentContext;
        }
      }

      if (ctx == null || !context.mounted) {
        if (context.mounted) FloatingToast.error(context, '找不到原始訊息');
        return;
      }

      Scrollable.ensureVisible(
        ctx,
        duration: const Duration(milliseconds: 350),
        alignment: 0.2,
        curve: Curves.easeInOut,
      );
      if (!context.mounted) return;
      highlightIds.value = {...highlightIds.value, id};
      Future.delayed(const Duration(milliseconds: 1200), () {
        if (!context.mounted) return;
        final next = {...highlightIds.value};
        next.remove(id);
        highlightIds.value = next;
      });
    }

    String oneLinePreview(String raw) {
      var s = raw.replaceAll(RegExp(r'\s+'), ' ').trim();
      if (s.length > 50) s = '${s.substring(0, 50)}…';
      return s;
    }

    String? lookupMessageTextById(String id) {
      try {
        final m = readMessages().firstWhere((e) => e.id.toString() == id);
        return oneLinePreview(m.messageContent);
      } catch (_) {
        return null;
      }
    }

    void jumpToMessageByPreview(String preview) {
      if (preview.isEmpty) return;
      String norm(String s) => s.replaceAll(RegExp(r'\s+'), ' ').trim();
      final p = norm(preview).replaceAll('…', '');
      for (final m in readMessages()) {
        final one = oneLinePreview(m.messageContent);
        if (norm(one).startsWith(p)) {
          jumpToMessageId(m.id);
          return;
        }
        final raw = norm(m.messageContent);
        if (raw.startsWith(p)) {
          jumpToMessageId(m.id);
          return;
        }
      }
    }

    String computeReplyPreview(ChatMessage m) {
      final type = m.messageType.toLowerCase();
      if (type == 'image') return '回覆照片';
      if (looksLikeCard(m.messageContent)) return '回覆卡片';
      return oneLinePreview(m.messageContent);
    }

    void ensureInputFocus({bool force = false}) {
      if (force || !inputFocus.hasPrimaryFocus) {
        inputFocus.requestFocus();
      }
    }

    Future<void> onSendText() async {
      final text = inputCtrl.text.trim();
      if (text.isEmpty || sending.value) return;
      HapticFeedback.lightImpact();
      ensureInputFocus();
      String msgContent = text;
      final preview = replyPreview.value;
      if (preview != null && preview.isNotEmpty) {
        msgContent = '「$preview」\n$text';
      }
      sending.value = true;
      try {
        // notifier.send 負責 optimistic insert + 後端確認 + dedup。
        await ref
            .read(chatThreadProvider(arg).notifier)
            .send(content: msgContent);
        if (!context.mounted) return;
        inputCtrl.clear();
        if (context.mounted) {
          replyTarget.value = null;
          replyPreview.value = null;
        }
        unawaited(ref.read(analyticsEventsProvider).sendMessage());
      } catch (e) {
        HKLog.logD('[ChatThread] send text error: $e');
        if (context.mounted) {
          final handled =
              e is ChatSendException &&
              handleChatSendReportBlockError(context, e);
          if (!handled) {
            FloatingToast.error(
              context,
              e is ChatSendException && e.message.isNotEmpty
                  ? e.message
                  : '發送失敗，請稍後再試',
            );
          }
        }
      } finally {
        if (context.mounted) sending.value = false;
      }
    }

    Future<void> onPickFromDevice() async {
      try {
        final picker = ImagePicker();
        final picked = await picker.pickImage(
          source: ImageSource.gallery,
          maxWidth: 2048.w,
          imageQuality: 85,
        );
        if (picked == null) return;
        final bytes = await picked.readAsBytes();
        if (bytes.isEmpty) return;

        final mid = ref.read(sellerStoreIdProvider);
        if (mid == null) {
          if (!context.mounted) return;
          FloatingToast.error(context, '無法取得商家ID，請重新登入後再試');
          return;
        }

        sending.value = true;
        final (ext, mime) = ImageFormatDetector.detect(bytes);
        final filename = 'chat_${DateTime.now().millisecondsSinceEpoch}.$ext';

        final upload = await ref
            .read(sellerRepositoryProvider)
            .uploadMedia(
              sellerId: mid,
              bytes: bytes,
              filename: filename,
              mimeType: mime,
            );
        final stagedToken = upload['data']?['stagedToken']?.toString();
        final previewUrl =
            upload['data']?['previewUrl']?.toString() ??
            upload['data']?['url']?.toString() ??
            upload['url']?.toString();
        if (stagedToken == null || stagedToken.isEmpty) {
          throw Exception('上傳失敗，未取得 stagedToken');
        }

        HKLog.logD(
          '[ChatThread] 🖼️ 送出圖片: stagedToken=${stagedToken.substring(0, math.min(50, stagedToken.length))}..., previewUrl=$previewUrl',
        );
        // notifier.send 以 stagedToken 為 optimistic + wire 內容；圖片的顯示 URL
        // 須由後端產生，故送出後 refetchAndReplace 重抓，把 staged token bubble
        // 換成帶 CDN URL 的正式訊息（等同舊 refreshHistory 的角色）。
        await ref
            .read(chatThreadProvider(arg).notifier)
            .send(content: stagedToken, messageType: 'image');
        if (context.mounted) {
          unawaited(
            ref.read(chatThreadProvider(arg).notifier).refetchAndReplace(),
          );
        }
      } catch (e) {
        HKLog.logD('[ChatThread] device pick error: $e');
        if (context.mounted) FloatingToast.error(context, '發送圖片失敗，請稍後再試');
      } finally {
        if (context.mounted) sending.value = false;
      }
    }

    // WidgetsBindingObserver for keyboard metrics
    useEffect(() {
      void onMetricsChanged() {
        final dispatcher = WidgetsBinding.instance.platformDispatcher;
        final views = dispatcher.views;
        final view = views.isNotEmpty ? views.first : null;
        final pixels = view?.viewInsets.bottom ?? 0.0;
        final ratio =
            view?.devicePixelRatio ??
            (views.isNotEmpty ? views.first.devicePixelRatio : 1.0);
        final inset = ratio == 0 ? 0.0 : pixels / ratio;
        if ((inset - lastViewInset.value).abs() < 1.0) {
          lastViewInset.value = inset;
          return;
        }
        final keyboardOpened = inset > lastViewInset.value;
        lastViewInset.value = inset;
        if (keyboardOpened) {
          WidgetsBinding.instance.addPostFrameCallback((_) {
            if (!context.mounted) return;
            scrollToBottom();
          });
        }
      }

      final observer = _MetricsObserver(onMetricsChanged);
      WidgetsBinding.instance.addObserver(observer);
      return () => WidgetsBinding.instance.removeObserver(observer);
    }, const []);

    // notifier 載入 / 更新時的 side-effects（Fix #2 + Fix #3 共用一個 listener）。
    // Fix #3：一次性進場（捲到底 + 推導 convSellerId + 補 peer 顯示）只跑首次 data；
    //   reconnect 的 refetchAndReplace 走 AsyncLoading→AsyncData，didInitialScroll 已
    //   為 true 故跳過，避免使用者往上捲讀歷史時被硬拉回底部。
    // Fix #2：每次有新「尾端」訊息（即時抵達 / 自己送出）就補未讀徽章 + 進場動畫；
    //   loadOlder 是 prepend 到頭部、不在尾端，故不會誤觸。
    void onThreadUpdate(
      AsyncValue<List<ChatMessage>>? prev,
      AsyncValue<List<ChatMessage>> next,
    ) {
      if (next is! AsyncData) return;
      final msgs = next.value ?? const <ChatMessage>[];

      if (!didInitialScroll.value) {
        didInitialScroll.value = true;
        autoScrollPending.value = true;
        // convSellerId 先於 list render 設定，避免 isSelfMessage 用到舊值渲染一幀。
        if (conversationId != null && conversationId!.contains('_')) {
          convSellerId.value = conversationId!
              .split('_')
              .firstWhere((p) => p.startsWith('m'), orElse: () => '');
        }
        scrollToBottomOnOpen();
        final notifier = ref.read(chatThreadProvider(arg).notifier);
        for (final p in notifier.lastParticipants) {
          if (p.userId == (participantId ?? '')) {
            if (peerAvatarUrl.value == null && p.avatarUrl.isNotEmpty) {
              peerAvatarUrl.value =
                  MediaRepository.resolveUrl(p.avatarUrl) ?? p.avatarUrl;
            }
            if (peerName.value == null && p.displayName.isNotEmpty) {
              peerName.value = p.displayName;
            }
          }
        }
        // Fallback：participants 沒帶到 peer → 走原有 customer API。
        if (peerAvatarUrl.value == null) {
          unawaited(ensurePeerAvatar(idHint: participantId));
        }
        if (peerName.value == null) {
          unawaited(ensurePeerName(idHint: participantId));
        }
        // 記錄初始 tail，後續才能辨別「新尾端」；首載整串歷史不放煙火。
        lastTailId.value = msgs.isNotEmpty ? msgs.last.id : null;
        return;
      }

      // ── Fix #2：偵測真正新增在尾端的訊息 ──
      if (msgs.isEmpty) {
        lastTailId.value = null;
        return;
      }
      final prevTailId = lastTailId.value;
      final tailIndex = prevTailId == null
          ? -1
          : msgs.indexWhere((m) => m.id == prevTailId);
      // 舊 tail 找得到且其後有新訊息 → 真正新增在尾端（即時 / 自送）。找不到（整串被
      // refetchAndReplace 換掉，或 confirmOptimistic 換掉 temp id）時只重新對齊、不放煙火。
      if (prevTailId != null && tailIndex >= 0 && tailIndex + 1 < msgs.length) {
        final nearBottom = isNearBottom();
        for (var i = tailIndex + 1; i < msgs.length; i++) {
          animatedMsgIds.value.add(msgs[i].id); // 進場動畫（shouldAnimate 機制）
          if (!nearBottom) unreadWhileAway.value++;
        }
        if (nearBottom) scrollToBottom();
      }
      lastTailId.value = msgs.last.id;
    }

    ref.listen<AsyncValue<List<ChatMessage>>>(
      chatThreadProvider(arg),
      onThreadUpdate,
    );
    // 暖快取補跑：riverpod 3 的 ref.listen 不送當前值（已移除 fireImmediately），
    //   若 provider 掛載時已被全域 pre-warm listener 養成 AsyncData，上面的首載分支
    //   永不跑（卡在頂部、convSellerId null）。掛載後讀一次當前值補呼叫；冷掛載讀到
    //   AsyncLoading 由 onThreadUpdate 早退無害，didInitialScroll guard 確保只跑一次。
    useEffect(() {
      onThreadUpdate(null, ref.read(chatThreadProvider(arg)));
      return null;
    }, const []);

    // Effect A：WS 連線生命週期 + presence（訊息清單 / reconnect-refetch 已移到
    // notifier；這裡只負責 presence 訂閱與重連後重新訂閱）。
    useEffect(() {
      final repo = ref.read(chatRepositoryProvider);
      repo.connectForChat();
      autoScrollPending.value = true;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        KeyboardUnfocusController.pushOverride();
      });

      // 對方是買家（peerRole=1）
      const peerRoleBuyer = 1;
      final peerId = int.tryParse(participantId ?? '') ?? 0;

      if (peerId > 0) repo.subscribePresence(peerId, peerRoleBuyer);

      final presenceSub = repo.onPresence.listen((e) {
        if (!context.mounted) return;
        if (e.peerId != peerId || e.peerRole != peerRoleBuyer) return;
        isPeerOnline.value = e.online;
      });

      // WS 重連時重新訂閱 presence（訊息歷史補抓由 notifier 的 connSub 負責）。
      // skip(1)：connectionStateStream 為 emit-on-subscribe，只對狀態轉換反應。
      final connResubSub = repo.connectionStateStream.skip(1).listen((s) {
        if (s == ChatConnectionState.connected &&
            context.mounted &&
            peerId > 0) {
          repo.subscribePresence(peerId, peerRoleBuyer);
        }
      });

      return () {
        // popOverride modifies a ValueNotifier which can trigger listeners
        // during the frame lock phase (widget unmount). Defer to next frame.
        WidgetsBinding.instance.addPostFrameCallback((_) {
          KeyboardUnfocusController.popOverride();
        });
        connResubSub.cancel();
        presenceSub.cancel();
        if (peerId > 0) repo.unsubscribePresence(peerId, peerRoleBuyer);
        openSettleTimer.value?.cancel();
        repo.disconnectFromChat();
      };
    }, const []);

    // Effect B：打字指示器顯示（widget-local）+ active gate conversationId 補 patch。
    // 訊息 / 已讀回執由 notifier 處理；這裡只看 typing 與 gate。
    useEffect(() {
      final repo = ref.read(chatRepositoryProvider);
      Timer? typingTimer;
      final typingSub = repo.onMessage.listen((msg) {
        if (!context.mounted) return;
        // 進場時 conversationId 可能 null（從顧客頁直接開）；WS 第一則訊息帶 roomId
        // 後補進 active gate，讓 FCM 前景 banner 也能用 conversationId 比對抑制。
        if (ref.read(activeChatGateProvider).conversationId == null &&
            (msg.conversationId ?? '').isNotEmpty) {
          ref
              .read(activeChatGateProvider.notifier)
              .updateConversationId(msg.conversationId!);
        }
        if (msg.senderId != (participantId ?? '')) return;
        if (msg.messageType == 'typing' || msg.messageType == 'start_typing') {
          isPeerTyping.value = true;
          typingTimer?.cancel();
          typingTimer = Timer(const Duration(seconds: 4), () {
            if (context.mounted) isPeerTyping.value = false;
          });
        } else if (msg.messageType == 'stop_typing') {
          isPeerTyping.value = false;
        } else if (msg.messageContent.isNotEmpty || msg.messageType == 'text') {
          // 對方傳來真訊息 → 關閉打字指示器（與原行為一致）。
          isPeerTyping.value = false;
        }
      });
      return () {
        typingSub.cancel();
        typingTimer?.cancel();
      };
    }, const []);

    Widget buildImage(String content) => buildImageWidget(content);

    Widget buildReplyQuoteBlock({
      required String previewText,
      required bool isSelf,
      String? senderName,
    }) {
      final accentColor = isSelf
          ? AppColors.bgCard.withValues(alpha: 0.8)
          : AppColors.primary;
      final bgColor = isSelf
          ? AppColors.bgCard.withValues(alpha: 0.1)
          : AppColors.gray100;
      final nameColor = isSelf ? AppColors.bgCard : AppColors.primary;
      final textColor = isSelf
          ? AppColors.bgCard.withValues(alpha: 0.9)
          : AppColors.textPrimary;

      return Container(
        margin: EdgeInsets.only(bottom: 8.h),
        decoration: BoxDecoration(
          color: bgColor,
          borderRadius: BorderRadius.circular(6.r),
        ),
        clipBehavior: Clip.antiAlias,
        child: IntrinsicHeight(
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Container(width: 3.w, color: accentColor),
              Expanded(
                child: Padding(
                  padding: EdgeInsets.fromLTRB(12.w, 4.h, 12.w, 4.h),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      if (senderName != null && senderName.isNotEmpty)
                        Text(
                          senderName,
                          style: TextStyle(
                            fontSize: 14.sp,
                            fontWeight: FontWeight.w700,
                            color: nameColor,
                          ),
                        ),
                      Text(
                        previewText.isEmpty ? '(原訊息)' : previewText,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(
                          fontSize: 14.sp,
                          color: textColor,
                          height: 1.4,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
      );
    }

    Widget buildReplyCardWidget({
      required String replyPreviewText,
      required String currentType,
      required String currentText,
      required bool isSelf,
    }) {
      Widget lower;
      if (currentType.toLowerCase() == 'image') {
        lower = buildImage(currentText);
      } else {
        lower = Text(
          currentText,
          softWrap: true,
          overflow: TextOverflow.visible,
        );
      }
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          buildReplyQuoteBlock(
            previewText: replyPreviewText,
            isSelf: isSelf,
            senderName: isSelf ? (peerName.value ?? title) : null,
          ),
          lower,
        ],
      );
    }

    Widget buildListBody(List<ChatMessage> list) {
      final media = MediaQuery.of(context);
      final keyboardInset = media.viewInsets.bottom;
      final bottomPadding = (keyboardInset > 0 ? keyboardInset + 20.0 : 20.0);
      if (list.isEmpty) {
        return const Center(child: Text('尚無訊息'));
      }

      final total = list.length;

      return Stack(
        children: [
          const Positioned.fill(child: ChatConversationDotPattern()),
          ListView.builder(
            controller: scrollCtrl,
            reverse: true,
            padding: EdgeInsets.fromLTRB(12.w, bottomPadding, 12.w, 8.h),
            itemCount: total + 1,
            itemBuilder: (context, index) {
              if (index == 0) {
                return SizedBox(key: bottomKey, height: 1);
              }
              final msgIndex = total - index;
              final m = list[msgIndex];
              final isSelf = isSelfMessage(m);
              final isImage =
                  (m.messageType.toLowerCase() == 'image') ||
                  _isImageUrl(m.messageContent);

              // 分組判斷：前後訊息是否同一人 + 時間間隔
              final prevMsg = msgIndex > 0 ? list[msgIndex - 1] : null;
              final nextMsg = msgIndex < total - 1 ? list[msgIndex + 1] : null;
              final sameSenderAsPrev =
                  prevMsg != null && prevMsg.senderId == m.senderId;
              final sameSenderAsNext =
                  nextMsg != null && nextMsg.senderId == m.senderId;
              final showTimeSeparator =
                  prevMsg != null &&
                  m.timestamp.difference(prevMsg.timestamp).inMinutes.abs() > 5;
              final isFirstInGroup = !sameSenderAsPrev || showTimeSeparator;

              VoidCallback? tapAction;
              Widget content = const SizedBox.shrink();
              // 訂單卡片（order_prev / order_check）
              final msgType = m.messageType.toLowerCase();
              final isOrderCard =
                  msgType == 'order_prev' ||
                  msgType == 'order_check' ||
                  ChatOrderBubble.tryParseOrder(m.messageContent) != null;
              // 商品卡片（product）
              final isProductCard =
                  msgType == 'product' ||
                  ChatProductBubble.tryParseProduct(m.messageContent) != null;
              if (isOrderCard) {
                content = ChatOrderBubble(
                  messageContent: m.messageContent,
                  isSelf: isSelf,
                  onTap: () {
                    final order = ChatOrderBubble.tryParseOrder(
                      m.messageContent,
                    );
                    if (order != null) {
                      context.push('/orders/matched/detail', extra: order);
                    }
                  },
                );
              } else if (isProductCard) {
                content = ChatProductBubble(
                  messageContent: m.messageContent,
                  isSelf: isSelf,
                  onTap: () {
                    final product = ChatProductBubble.tryParseProduct(
                      m.messageContent,
                    );
                    if (product != null) {
                      context.push(
                        '/product/preview',
                        extra: {'product': product},
                      );
                    }
                  },
                );
              } else if (isImage) {
                content = buildImage(m.messageContent);
              } else {
                Widget? card;
                if (!isSelf) {
                  card = maybeBuildCard(m.messageContent);
                }
                if (card != null) {
                  content = card;
                } else {
                  bool built = false;
                  try {
                    final parsed = json.decode(m.messageContent);
                    if (parsed is Map) {
                      final map = Map<String, dynamic>.from(parsed);
                      final replyId = _extractReplyToId(map);
                      final currentType =
                          (map['messageType'] ?? map['type'] ?? '')
                              .toString()
                              .toLowerCase();
                      final currentText = (map['text'] ?? map['content'] ?? '')
                          .toString();
                      if (replyId != null && replyId.isNotEmpty) {
                        final targetId = replyId;
                        tapAction = () => jumpToMessageId(targetId);
                        final rPreview = oneLinePreview(
                          lookupMessageTextById(replyId) ??
                              (map['replyText']?.toString() ?? ''),
                        );
                        content = buildReplyCardWidget(
                          replyPreviewText: rPreview,
                          currentType: currentType.isEmpty
                              ? 'text'
                              : currentType,
                          currentText: currentText.isEmpty
                              ? m.messageContent
                              : currentText,
                          isSelf: isSelf,
                        );
                        built = true;
                      }
                    }
                  } catch (_) {}
                  if (!built) {
                    final text = m.messageContent;
                    final re = RegExp(r'^「(.+?)」\s*\n([\s\S]*)');
                    final mm = re.firstMatch(text);
                    if (mm != null) {
                      final quoted = mm.group(1) ?? '';
                      final rest = mm.group(2) ?? '';
                      final previewText = quoted;
                      tapAction = () => jumpToMessageByPreview(previewText);
                      content = Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          buildReplyQuoteBlock(
                            previewText: quoted,
                            isSelf: isSelf,
                          ),
                          if (rest.trim().isNotEmpty)
                            Text(
                              rest.trim(),
                              softWrap: true,
                              overflow: TextOverflow.visible,
                            ),
                        ],
                      );
                    } else {
                      content = Text(
                        m.messageContent,
                        softWrap: true,
                        overflow: TextOverflow.visible,
                      );
                    }
                  }
                }
              }

              final key = messageKeyFor(m.id);
              final shouldAnimate = animatedMsgIds.value.contains(m.id);

              // 時間分隔線（間隔>5分鐘）
              Widget? timeSeparator;
              if (showTimeSeparator) {
                final t = m.timestamp.toLocal();
                final timeStr =
                    '${t.hour.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}';
                final today = DateTime.now();
                final isToday =
                    t.year == today.year &&
                    t.month == today.month &&
                    t.day == today.day;
                final dateStr = isToday
                    ? timeStr
                    : '${t.month}/${t.day} $timeStr';
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
                        dateStr,
                        style: TextStyle(
                          fontSize: 12.sp,
                          color: AppColors.textSecondary,
                        ),
                      ),
                    ),
                  ),
                );
              }

              // ── 氣泡時間 + 已讀狀態（放在氣泡內部右下角）──
              final msgTime = m.timestamp.toLocal();
              final msgH12 = msgTime.hour == 0
                  ? 12
                  : (msgTime.hour > 12 ? msgTime.hour - 12 : msgTime.hour);
              final timeStr =
                  '${msgTime.hour < 12 ? "上午" : "下午"}${msgH12.toString().padLeft(2, '0')}:${msgTime.minute.toString().padLeft(2, '0')}';

              // 時間 + 已讀狀態（氣泡外，右側對齊）
              final timeStatusRow = Row(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.center,
                children: [
                  Text(
                    timeStr,
                    style: TextStyle(
                      fontSize: 12.sp,
                      color: AppColors.textSecondary,
                    ),
                  ),
                  if (isSelf) ...[
                    SizedBox(width: 4.w),
                    Text(
                      m.read ? '已讀' : '未讀',
                      style: TextStyle(
                        fontSize: 12.sp,
                        color: AppColors.textSecondary,
                      ),
                    ),
                  ],
                ],
              );

              // ── 對方頭像（每組第一則顯示，其餘用空白對齊）──
              Widget avatarSlot = const SizedBox.shrink();
              if (!isSelf) {
                if (isFirstInGroup) {
                  avatarSlot = Container(
                    width: 32.w,
                    height: 32.w,
                    margin: EdgeInsets.only(top: 4.h, right: 8.w),
                    child: ClipOval(
                      child:
                          (peerAvatarUrl.value != null &&
                              peerAvatarUrl.value!.isNotEmpty)
                          ? OptimizedNetworkImage(
                              imageUrl: peerAvatarUrl.value!,
                              scene: ImageScene.chatAvatar,
                              fit: BoxFit.cover,
                              width: 32.w,
                              height: 32.w,
                              errorWidget: _PeerAvatarInitial(
                                name: peerName.value ?? '?',
                              ),
                            )
                          : _PeerAvatarInitial(name: peerName.value ?? '?'),
                    ),
                  );
                } else {
                  avatarSlot = SizedBox(width: 32.w + 6.w);
                }
              }

              // 卡片類型（order/product）自帶裝飾，不需要外層氣泡 Container
              final isCardBubble = isOrderCard || isProductCard;

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
                        child: DraggableReplyWrapper(
                          enable: !isSelf,
                          onSwipeRightReply: () {
                            replyTarget.value = m;
                            replyPreview.value = computeReplyPreview(m);
                          },
                          child: GestureDetector(
                            behavior: HitTestBehavior.opaque,
                            onTap:
                                tapAction ??
                                (isImage
                                    ? () => _openImageViewer(
                                        context,
                                        m.messageContent,
                                      )
                                    : null),
                            onLongPress: () {
                              HapticFeedback.heavyImpact();
                              _showMessageContextMenu(
                                context,
                                m,
                                isSelf,
                                ref,
                                replyTarget,
                                replyPreview,
                                computeReplyPreview,
                              );
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
                                  _MeasuredSize(
                                    key: key,
                                    onChanged: (size) =>
                                        recordMessageHeight(m.id, size),
                                    // 卡片自帶裝飾 → 直接用 content；一般訊息 → 包在氣泡 Container
                                    child: isCardBubble
                                        ? content
                                        : Container(
                                            padding: EdgeInsets.fromLTRB(
                                              12.w,
                                              12.h,
                                              12.w,
                                              12.h,
                                            ),
                                            decoration: BoxDecoration(
                                              color: isSelf
                                                  ? AppColors.primary
                                                  : AppColors.bgCard,
                                              borderRadius: _smartBorderRadius(
                                                isSelf,
                                                isFirstInGroup,
                                                sameSenderAsNext &&
                                                    !showTimeSeparator,
                                              ),
                                              border:
                                                  (!isSelf &&
                                                      highlightIds.value
                                                          .contains(m.id))
                                                  ? Border.all(
                                                      color: AppColors.warning,
                                                      width: 2,
                                                    )
                                                  : null,
                                              boxShadow: const [
                                                BoxShadow(
                                                  color: AppColors.cardShadow,
                                                  blurRadius: 3,
                                                  offset: Offset(0, 1),
                                                ),
                                                BoxShadow(
                                                  color: AppColors.cardShadow,
                                                  blurRadius: 2,
                                                  offset: Offset(0, 1),
                                                ),
                                              ],
                                            ),
                                            child: DefaultTextStyle(
                                              style: TextStyle(
                                                fontSize: 16.sp,
                                                color: isSelf
                                                    ? AppColors.bgCard
                                                    : AppColors.textEmphasis,
                                                height: 1.5,
                                              ),
                                              child: content,
                                            ),
                                          ),
                                  ),
                                  // 時間 + 已讀狀態（氣泡外下方）
                                  Padding(
                                    padding: EdgeInsets.only(
                                      top: 4.h,
                                      bottom: 4.h,
                                      left: 4.w,
                                      right: 4.w,
                                    ),
                                    child: timeStatusRow,
                                  ),
                                ],
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                    if (isSelf) SizedBox(width: 4.w),
                  ],
                ),
              );

              // 新訊息入場動畫
              if (shouldAnimate) {
                WidgetsBinding.instance.addPostFrameCallback((_) {
                  animatedMsgIds.value = {...animatedMsgIds.value}
                    ..remove(m.id);
                });
                bubble = _MessageEntrance(isSelf: isSelf, child: bubble);
              }

              // 組合：時間分隔線 + 氣泡
              if (timeSeparator != null) {
                return Column(children: [timeSeparator, bubble]);
              }
              return bubble;
            },
          ),
        ],
      );
    }

    Widget buildBody() {
      return thread.when(
        loading: () {
          // refetchAndReplace 設 AsyncLoading 時 riverpod 3.x 會 copyWithPrevious
          // 保留既有 value；reconnect / refetch 期間若 value 非空就續顯示列表，
          // 避免每次重連都閃一下 TruckLoadingOverlay 把訊息清空（同 Task 5 做法）。
          final current = thread.value;
          if (current != null && current.isNotEmpty) {
            return buildListBody(current);
          }
          return const TruckLoadingOverlay();
        },
        error: (e, _) => Center(
          child: Text(
            e.toString(),
            style: const TextStyle(color: AppColors.danger),
          ),
        ),
        data: buildListBody,
      );
    }

    // ── 輸入列（買家版樣式）──
    Widget inputBar() {
      // ChatInputBar 使用透明外層並自帶底部安全區，避免重複 SafeArea padding。
      return ChatInputBar(
        controller: inputCtrl,
        focusNode: inputFocus,
        sending: sending.value,
        onSend: onSendText,
        onPlusPressed: () => showChatAttachmentMenu(
          context: context,
          onPickPhoto: onPickFromDevice,
          onPickOrder: () => openChatOrderPicker(
            context: context,
            ref: ref,
            participantId: participantId,
            arg: arg,
            sending: sending,
          ),
          onPickProduct: () => openChatProductPicker(
            context: context,
            ref: ref,
            participantId: participantId,
            arg: arg,
            sending: sending,
          ),
        ),
        topSlot: AnimatedSize(
          duration: const Duration(milliseconds: 200),
          curve: Curves.easeOutCubic,
          child: replyPreview.value != null
              ? Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Container(
                      margin: EdgeInsets.only(bottom: 8.h),
                      decoration: BoxDecoration(
                        color: AppColors.gray100,
                        borderRadius: BorderRadius.circular(12.r),
                      ),
                      clipBehavior: Clip.antiAlias,
                      child: IntrinsicHeight(
                        child: Row(
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            // 左側綠色強調線
                            Container(width: 3.5.w, color: AppColors.primary),
                            SizedBox(width: 10.w),
                            // 中間內容
                            Expanded(
                              child: Padding(
                                padding: EdgeInsets.symmetric(vertical: 8.h),
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  mainAxisSize: MainAxisSize.min,
                                  children: [
                                    Text(
                                      '回覆 ${peerName.value ?? title}',
                                      style: TextStyle(
                                        fontSize: 14.sp,
                                        fontWeight: FontWeight.w700,
                                        color: AppColors.primary,
                                      ),
                                    ),
                                    SizedBox(height: 2.h),
                                    Text(
                                      replyPreview.value!,
                                      maxLines: 1,
                                      overflow: TextOverflow.ellipsis,
                                      style: TextStyle(
                                        fontSize: 14.sp,
                                        color: AppColors.textPrimary,
                                        height: 1.3,
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                            ),
                            // 關閉按鈕
                            GestureDetector(
                              onTap: () {
                                replyTarget.value = null;
                                replyPreview.value = null;
                              },
                              child: Padding(
                                padding: EdgeInsets.symmetric(horizontal: 12.w),
                                child: Icon(
                                  AppIcons.closeRounded,
                                  size: 18.sp,
                                  color: AppIconColors.muted,
                                ),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                    Divider(
                      height: 1,
                      thickness: 0.5,
                      color: AppColors.dividerLight,
                    ),
                    SizedBox(height: 8.h),
                  ],
                )
              : const SizedBox.shrink(),
        ),
      );
    }

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      resizeToAvoidBottomInset: true,
      body: Column(
        children: [
          ChatConversationTopBar(
            avatar: ClipOval(
              child:
                  (peerAvatarUrl.value != null &&
                      peerAvatarUrl.value!.isNotEmpty)
                  ? OptimizedNetworkImage(
                      imageUrl: peerAvatarUrl.value!,
                      scene: ImageScene.chatAvatar,
                      fit: BoxFit.cover,
                      width: 40,
                      height: 40,
                      errorWidget: _PeerAvatarInitial(
                        name: peerName.value ?? '?',
                      ),
                    )
                  : _PeerAvatarInitial(name: peerName.value ?? '?'),
            ),
            title: peerName.value ?? title,
            statusLabel: isPeerOnline.value ? '線上' : '離線',
            statusColor: isPeerOnline.value
                ? AppColors.success
                : AppColors.textHint,
            onBack: () => context.pop(),
          ),
          Expanded(
            child: Stack(
              children: [
                buildBody(),
                // 捲動到底部 FAB
                if (showScrollFab.value)
                  Positioned(
                    right: 16.w,
                    bottom: 12.h,
                    child: GestureDetector(
                      onTap: () {
                        scrollToBottom();
                        unreadWhileAway.value = 0;
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
                // 打字指示器 — Telegram 風格：對方頭像 + 跳動 dots 氣泡
                if (isPeerTyping.value)
                  Positioned(
                    left: 12.w,
                    bottom: 8.h,
                    child: _TypingIndicator(
                      avatarUrl: peerAvatarUrl.value,
                      fallbackName: peerName.value,
                    ),
                  ),
              ],
            ),
          ),
          // 輸入列（ChatInputBar 自帶白底 + bottom safe area）
          inputBar(),
        ],
      ),
    );
  }
}

// Helper observer for WidgetsBindingObserver.didChangeMetrics
class _MetricsObserver extends WidgetsBindingObserver {
  _MetricsObserver(this.onChanged);
  final VoidCallback onChanged;

  @override
  void didChangeMetrics() => onChanged();
}

String? _extractReplyToId(Map<String, dynamic> map) {
  for (final k in const [
    'replyToId',
    'replyId',
    'inReplyTo',
    'parentId',
    'quotedId',
    'replyTold',
  ]) {
    final v = map[k];
    if (v == null) continue;
    final s = v.toString().trim();
    if (s.isNotEmpty) return s;
  }
  return null;
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

/// 打字指示器 — 三個跳動圓點
/// Telegram 風格 typing indicator：對方頭像 + 白色氣泡內 3 顆 dots wave 動畫。
class _TypingIndicator extends StatefulWidget {
  const _TypingIndicator({this.avatarUrl, this.fallbackName});
  final String? avatarUrl;
  final String? fallbackName;

  @override
  State<_TypingIndicator> createState() => _TypingIndicatorState();
}

class _TypingIndicatorState extends State<_TypingIndicator>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1400),
    )..repeat();
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  /// 每個 dot 在 wave 中的 -y 偏移（jump up 5px，sin 平滑）。
  /// dot0: 0.00–0.40, dot1: 0.15–0.55, dot2: 0.30–0.70
  double _dotOffset(int i, double t) {
    final start = i * 0.15;
    const span = 0.40;
    if (t < start || t > start + span) return 0;
    final local = (t - start) / span;
    return -math.sin(math.pi * local) * 5.0;
  }

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        // 對方頭像（與 incoming 訊息頭像同尺寸）
        Container(
          width: 32.w,
          height: 32.w,
          margin: EdgeInsets.only(right: 8.w, bottom: 4.h),
          child: ClipOval(
            child: (widget.avatarUrl != null && widget.avatarUrl!.isNotEmpty)
                ? OptimizedNetworkImage(
                    imageUrl: widget.avatarUrl!,
                    scene: ImageScene.chatAvatar,
                    fit: BoxFit.cover,
                    width: 32.w,
                    height: 32.w,
                    errorWidget: _PeerAvatarInitial(
                      name: widget.fallbackName ?? '?',
                    ),
                  )
                : _PeerAvatarInitial(name: widget.fallbackName ?? '?'),
          ),
        ),
        // 氣泡 + 跳動 dots
        Container(
          padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
          decoration: BoxDecoration(
            color: AppColors.bgCard,
            borderRadius: BorderRadius.only(
              topLeft: Radius.circular(6.r),
              topRight: Radius.circular(16.r),
              bottomLeft: Radius.circular(16.r),
              bottomRight: Radius.circular(16.r),
            ),
            boxShadow: [
              BoxShadow(
                color: AppColors.cardShadow,
                blurRadius: 4,
                offset: const Offset(0, 1),
              ),
            ],
          ),
          child: AnimatedBuilder(
            animation: _ctrl,
            builder: (_, __) {
              return Row(
                mainAxisSize: MainAxisSize.min,
                children: List.generate(3, (i) {
                  final dy = _dotOffset(i, _ctrl.value);
                  return Padding(
                    padding: EdgeInsets.symmetric(horizontal: 4.w),
                    child: Transform.translate(
                      offset: Offset(0, dy),
                      child: Container(
                        width: 7.w,
                        height: 7.w,
                        decoration: const BoxDecoration(
                          color: AppColors.textSecondary,
                          shape: BoxShape.circle,
                        ),
                      ),
                    ),
                  );
                }),
              );
            },
          ),
        ),
      ],
    );
  }
}

/// 長按訊息選單（Telegram 風格）
void _showMessageContextMenu(
  BuildContext context,
  ChatMessage msg,
  bool isSelf,
  WidgetRef ref,
  ValueNotifier<ChatMessage?> replyTarget,
  ValueNotifier<String?> replyPreview,
  String Function(ChatMessage) computeReplyPreview,
) {
  showModalBottomSheet(
    context: context,
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
              color: AppColors.gray200,
              borderRadius: BorderRadius.circular(2.r),
            ),
          ),
          SizedBox(height: 12.h),
          // 複製
          ListTile(
            leading: Icon(
              AppIcons.copy,
              color: AppIconColors.defaultColor,
              size: 22.sp,
            ),
            title: Text('複製', style: TextStyle(fontSize: 16.sp)),
            onTap: () {
              Clipboard.setData(ClipboardData(text: msg.messageContent));
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
          // 回覆
          ListTile(
            leading: Icon(
              AppIcons.reply,
              color: AppIconColors.defaultColor,
              size: 22.sp,
            ),
            title: Text('回覆', style: TextStyle(fontSize: 16.sp)),
            onTap: () {
              Navigator.pop(ctx);
              replyTarget.value = msg;
              replyPreview.value = computeReplyPreview(msg);
            },
          ),
          // 刪除（僅自己的訊息）
          if (isSelf)
            ListTile(
              leading: Icon(
                AppIcons.deleteOutline,
                color: AppIconColors.danger,
                size: 22.sp,
              ),
              title: Text(
                '刪除',
                style: TextStyle(fontSize: 16.sp, color: AppColors.danger),
              ),
              onTap: () {
                Navigator.pop(ctx);
                // TODO: 呼叫 delete API
              },
            ),
          // 檢舉／封鎖（僅對方的訊息，backend-payuni#645）
          if (!isSelf) ...[
            ListTile(
              leading: Icon(
                AppIcons.reportOutlined,
                color: AppIconColors.defaultColor,
                size: 22.sp,
              ),
              title: Text('檢舉訊息', style: TextStyle(fontSize: 16.sp)),
              onTap: () {
                Navigator.pop(ctx);
                showReportSheet(
                  context,
                  ref,
                  targetType: ReportTargetType.chatMessage,
                  targetId: msg.id,
                );
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
                blockUserWithConfirm(
                  context,
                  ref,
                  userId: msg.senderId,
                  userName: msg.senderName?.trim().isNotEmpty == true
                      ? msg.senderName!.trim()
                      : '此使用者',
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
void _openImageViewer(BuildContext context, String content) {
  String? url;
  try {
    if (content.startsWith('http')) {
      url = content;
    } else {
      final parsed = json.decode(content);
      if (parsed is Map) {
        url = (parsed['url'] ?? parsed['imageUrl'] ?? parsed['src'] ?? '')
            .toString();
      }
    }
  } catch (_) {
    url = content.startsWith('http') ? content : null;
  }
  if (url == null || url.isEmpty) return;

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
      pageBuilder: (_, __, ___) => _ImageViewerPage(url: url!),
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

/// 訊息入場動畫 — 滑入 + 淡入（200ms）
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

class _MeasuredSize extends StatefulWidget {
  const _MeasuredSize({super.key, required this.child, this.onChanged});

  final Widget child;
  final ValueChanged<Size?>? onChanged;

  @override
  State<_MeasuredSize> createState() => _MeasuredSizeState();
}

class _MeasuredSizeState extends State<_MeasuredSize> {
  Size? _oldSize;

  @override
  Widget build(BuildContext context) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final newSize = context.size;
      if (newSize == null) return;
      if (_oldSize == newSize) return;
      _oldSize = newSize;
      widget.onChanged?.call(newSize);
    });
    return widget.child;
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

bool _isImageUrl(String content) {
  final s = content.trim().toLowerCase();
  if (!s.startsWith('http')) return false;
  return s.endsWith('.jpg') ||
      s.endsWith('.jpeg') ||
      s.endsWith('.png') ||
      s.endsWith('.gif') ||
      s.endsWith('.webp') ||
      s.endsWith('.heic');
}
