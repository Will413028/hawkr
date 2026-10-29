import 'package:hawkr_buyer/core/l10n/ui_locale.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math' as math;
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';

import '../../media/data/media_repository.dart';
import '../../order/data/order_repository.dart';
import '../../seller/data/seller_repository.dart';
import '../../seller/data/store_card_display.dart';
import '../../product/data/product_repository.dart';
import '../../product/presentation/merchant_item_detail_screen.dart';
import '../../../config/flavor.dart';
import '../../../core/extensions/url_extensions.dart';
import '../../../core/motion/hk_motion_transitions.dart';
import '../data/chat_model.dart';
import '../data/chat_repository.dart';
import '../data/chat_thread_notifier.dart'; // chatThreadProvider + ChatThreadArg
import '../data/active_chat_gate.dart'; // activeChatGateProvider
import 'chat_preview.dart';
import '../../moderation/data/report_target_type.dart';
import '../../moderation/presentation/report_sheet.dart';
import '../../moderation/presentation/block_confirm_dialog.dart';
import 'chat_focus_policy.dart';
import '_draggable_reply_wrapper.dart';
import '../../../shared/widgets/chat_input_bar.dart';
import '../../../shared/widgets/chat_conversation_chrome.dart';
import '../../../shared/widgets/chat_order_bubble.dart';
import '../../../shared/widgets/chat_product_bubble.dart';
import '../../../shared/widgets/chat_order_picker_sheet.dart';
import '../../../shared/widgets/chat_product_picker_sheet.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/app_cached_image.dart';
import '../../../config/image_config.dart';
import '../../../config/media_image.dart';
import '../../../shared/widgets/oss_image.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/notification_count_badge.dart';
import '../../ai_assistant/presentation/ai_function_menu.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';
import 'package:hawkr_buyer/shared/widgets/hk_placeholder.dart';

// 買家聊天對象一定是賣家，peerRole 固定為 2
const int _peerRoleSeller = 2;

/// 顯示聊天「+」按鈕的功能選單
void _showChatPlusMenu({
  required BuildContext context,
  required VoidCallback onPickPhoto,
  required VoidCallback onPickOrder,
  VoidCallback? onPickProduct,
}) {
  showAiFunctionMenu(
    context: context,
    items: [
      AiFunctionMenuItem(
        icon: AppIcons.assignmentOutlined,
        label: '選擇訂單',
        onTap: onPickOrder,
      ),
      AiFunctionMenuItem(
        icon: AppIcons.localFloristOutlined,
        label: '選擇上架商品',
        onTap: onPickProduct ?? () {},
      ),
      AiFunctionMenuItem(
        icon: AppIcons.imageOutlined,
        label: '上傳照片',
        onTap: onPickPhoto,
      ),
    ],
  );
}

// ─────────────────────────────────────────────────────────────────────────────
// Pure helpers (no widget state) — extracted from the old State class.
// ─────────────────────────────────────────────────────────────────────────────

Future<void> _waitForLayout() async {
  await Future<void>.delayed(Duration.zero);
  await WidgetsBinding.instance.endOfFrame;
}

/// 依「訊息 id」取出可顯示的預覽文字（被回覆訊息的縮略）。
String _previewForMessageId(List<ChatMessage> msgs, String id) {
  if (id.isEmpty) return '';
  try {
    final idx = msgs.indexWhere((e) => e.id == id);
    if (idx == -1) return '';
    final m = msgs[idx];
    final raw = m.messageContent.trim();
    if (raw.isEmpty) return '';
    // Try parse JSON text payloads
    if (raw.startsWith('{') && raw.endsWith('}')) {
      try {
        final d = json.decode(raw);
        if (d is Map) {
          final Map<String, dynamic> mm = Map<String, dynamic>.from(d);
          for (final k in [
            'text',
            'title',
            'name',
            'productName',
            'messageContent',
          ]) {
            final v = mm[k];
            if (v != null && v.toString().trim().isNotEmpty) {
              return v.toString().trim();
            }
          }
          // Image-like
          for (final k in ['url', 'image', 'imageUrl', 'img', 'src', 'path']) {
            final v = mm[k];
            if (v != null && v.toString().trim().isNotEmpty) return '[圖片]';
          }
        }
      } catch (_) {}
    }
    // Image message
    if (m.messageType == 'image') return '[圖片]';
    return raw;
  } catch (_) {
    return '';
  }
}

/// 依預覽文字嘗試找出第一個匹配的訊息 ID（純文字或 JSON text 欄位）。
String _findMessageIdByPreview(List<ChatMessage> msgs, String preview) {
  if (preview.trim().isEmpty) return '';
  for (final m in msgs) {
    final txt = m.messageContent.trim();
    if (txt.isEmpty) continue;
    // JSON 文字
    if (txt.startsWith('{') && txt.endsWith('}')) {
      try {
        final d = json.decode(txt);
        if (d is Map) {
          final Map<String, dynamic> mm = Map<String, dynamic>.from(d);
          final t = (mm['text'] ?? '').toString().trim();
          if (t.isNotEmpty && t == preview) return m.id;
        }
      } catch (_) {}
    }
    // 純文字
    if (txt == preview) return m.id;
  }
  return '';
}

String _computeReplyPreview(ChatMessage m) {
  if (m.messageType == 'image') return '回覆照片';
  if (m.messageType == 'product' ||
      m.messageType == 'order_prev' ||
      m.messageType == 'order_check') {
    return '回覆卡片';
  }
  String raw = m.messageContent.replaceAll(RegExp(r'\s+'), ' ').trim();
  if (raw.isEmpty) return '回覆訊息';
  if (raw.length > 30) raw = '${raw.substring(0, 30)}…';
  return raw;
}

// --- Helpers for building order cards in chat ---
String _pickStr(Map<String, dynamic>? m, List<String> keys) {
  if (m == null) return '';
  for (final k in keys) {
    final v = m[k];
    if (v == null) continue;
    final s = v.toString().trim();
    if (s.isNotEmpty) return s;
  }
  return '';
}

int _tsFromOrder2(Map<String, dynamic>? m) {
  String raw = _pickStr(m, const [
    'finishTime',
    'receiveTime',
    'deliveryTime',
    'paymentTime',
    'updateTime',
    'gmtModified',
    'orderTime',
    'createTime',
    'gmtCreate',
  ]);
  raw = raw.trim();
  if (raw.isEmpty) return 0;
  try {
    if (RegExp(r'^\d+$').hasMatch(raw)) {
      if (raw.length >= 13) {
        final v = int.tryParse(raw.substring(0, 13));
        if (v != null) return v;
      }
      if (raw.length >= 10) {
        final v = int.tryParse(raw.substring(0, 10));
        if (v != null) return v * 1000;
      }
    }
    final norm = raw.contains('T') ? raw : raw.replaceFirst(' ', 'T');
    final norm2 = norm.replaceAll('/', '-');
    final dt = DateTime.tryParse(norm2);
    if (dt != null) return dt.millisecondsSinceEpoch;
  } catch (_) {}
  return 0;
}

Widget _buildInlineImage(String raw) {
  // Resolve source from raw text (same as seller approach)
  String s = raw.trim();
  if (s.startsWith('{') || s.startsWith('[')) {
    try {
      final j = json.decode(s);
      if (j is Map) {
        for (final k in [
          'imageKey',
          'key',
          'url',
          'imageUrl',
          'image',
          'src',
          'path',
        ]) {
          final v = j[k];
          if (v is String && v.isNotEmpty) {
            s = v.trim();
            break;
          }
        }
      }
    } catch (_) {}
  }
  if (s.isEmpty) {
    return Container(
      width: 200.w,
      height: 100.h,
      color: AppColors.gray200,
      child: const Icon(AppIcons.brokenImage, color: AppIconColors.muted),
    );
  }
  // 49 號成對契約（S4）：WS 圖片訊息 content = 裸 objectKey、REST 歷史為顯示 URL。
  // AppCachedImage(scene) 內部走 ImageConfig.urlFor → buildImageUrl dual-mode
  // （chat preset），key 或 URL 皆可，且統一套用聊天圖片 preset。
  return ClipRRect(
    borderRadius: BorderRadius.circular(8.r),
    child: AppCachedImage(
      imageUrl: s,
      scene: ImageScene.chatImage,
      width: 200.w,
      height: 200.h,
      fit: BoxFit.cover,
    ),
  );
}

void _openFullScreenImage(BuildContext context, String raw) {
  String s = raw.trim();
  try {
    if (s.startsWith('{')) {
      final j = json.decode(s);
      if (j is Map) {
        for (final k in [
          'imageKey',
          'key',
          'url',
          'imageUrl',
          'image',
          'src',
          'path',
        ]) {
          final v = j[k];
          if (v is String && v.isNotEmpty) {
            s = v.trim();
            break;
          }
        }
      }
    }
  } catch (_) {}
  // 49 號成對契約（S4）：content 可能為裸 objectKey，過 buildImageUrl dual-mode
  // 組全螢幕原圖（fullscreen → original preset）。
  final url = ImageConfig.urlFor(s, ImageScene.fullscreen);
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

// ─────────────────────────────────────────────────────────────────────────────
// ChatThreadScreen — Phase 2: HookConsumerWidget driven by chatThreadProvider.
// 訊息列表 / dedup / 已讀 / reconnect refetch 全部由 ChatThread Notifier 擁有；
// 本 widget 只負責 ephemeral UI（打字、捲動 FAB、回覆、商家 profile、推薦氣泡）。
// ─────────────────────────────────────────────────────────────────────────────
class ChatThreadScreen extends HookConsumerWidget {
  const ChatThreadScreen({
    super.key,
    required this.merchantId,
    this.merchantName,
    this.conversationId,
  });
  final String merchantId; // receiverId / peerId
  final String? merchantName;
  final String? conversationId; // 如果有會話 ID，優先用它拉歷史

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final normalizedConvId = (conversationId?.isNotEmpty ?? false)
        ? conversationId
        : null;
    final arg = ChatThreadArg(
      peerId: merchantId,
      conversationId: normalizedConvId,
    );
    final chatState = ref.watch(chatThreadProvider(arg));
    final msgs = chatState.value ?? const <ChatMessage>[];

    // ── Hooks (ephemeral UI state) ──────────────────────────────────────────
    final textCtrl = useTextEditingController();
    final inputFocusNode = useFocusNode();
    final scrollCtrl = useScrollController();
    final highlightId = useState<String?>(null);
    final highlightTimer = useRef<Timer?>(null);
    final animatedIds = useRef(<String>{});
    final showFab = useState(false);
    final unreadAway = useState(0);
    final isPeerTyping = useState(false);
    final typingTimer = useRef<Timer?>(null);
    final typingDebounce = useRef<Timer?>(null);
    final sentTyping = useRef(false);
    final replyTarget = useState<ChatMessage?>(null);
    final replyPreview = useState<String?>(null);
    final avatarUrl = useState<String?>(null);
    final merchantNameState = useState<String?>(merchantName);
    final storeStatusLabel = useState<String?>(null);

    useEffect(() {
      ChatFocusPolicy.keepClosedOnEntry(inputFocusNode);
      return null;
    }, [inputFocusNode]);
    final isMerchantOnline = useState(false);
    final msgRowKeys = useRef(<String, GlobalKey>{});
    final messageHeights = useRef(<String, double>{});
    final avgHeight = useRef(80.0);
    final suggestionShown = useRef(false);
    final pinnedSuggestionIds = useRef(<String>{});
    final suggestionTimers = useRef(<String, Timer>{});
    // 已對「進來的商家訊息」做過 ephemeral 反應（動畫 / 未讀 / 已讀 / 捲動）的 id，
    // 避免重複 frame 造成 double 反應（dedup 由 notifier 負責資料層）。
    final seenIncomingIds = useRef(<String>{});

    // ── leaf closures ───────────────────────────────────────────────────────
    List<ChatMessage> readMsgs() =>
        ref.read(chatThreadProvider(arg)).value ?? const <ChatMessage>[];

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

    void recordMessageHeight(String? id, double height) {
      if (id == null || id.isEmpty) return;
      if (height <= 0) return;
      final prev = messageHeights.value[id];
      if (prev != null && (prev - height).abs() < 0.5) return;
      messageHeights.value[id] = height;
      if (messageHeights.value.isNotEmpty) {
        final total = messageHeights.value.values.fold<double>(
          0,
          (sum, h) => sum + h,
        );
        avgHeight.value = total / messageHeights.value.length;
      }
    }

    double estimateOffsetForIndex(int targetIndex) {
      if (!scrollCtrl.hasClients) return 0;
      final list = readMsgs();
      if (targetIndex < 0 || targetIndex >= list.length) return 0;
      double offset = 0;
      // reverse list: index 0 is oldest，所以從最新往回累加
      for (int i = list.length - 1; i > targetIndex; i--) {
        final id = list[i].id;
        offset += messageHeights.value[id] ?? avgHeight.value;
      }
      return offset.clamp(0.0, scrollCtrl.position.maxScrollExtent);
    }

    Future<bool> ensureMessageLoaded(String messageId) async {
      if (messageId.isEmpty) return false;
      if (readMsgs().any((m) => m.id == messageId)) return true;
      final notifier = ref.read(chatThreadProvider(arg).notifier);
      var guard = 0;
      while (guard++ < 60) {
        final more = await notifier.loadOlder();
        if (readMsgs().any((m) => m.id == messageId)) return true;
        if (!more) break;
      }
      return readMsgs().any((m) => m.id == messageId);
    }

    void triggerHighlight(String id) {
      if (!context.mounted) return;
      highlightTimer.value?.cancel();
      highlightId.value = id;
      highlightTimer.value = Timer(const Duration(seconds: 3), () {
        if (!context.mounted) return;
        if (highlightId.value == id) highlightId.value = null;
      });
    }

    Future<void> loadMerchantProfile() async {
      try {
        final cards = await ref.read(sellerRepositoryProvider).batchGetStores([
          merchantId,
        ]);
        final card = cards[merchantId];
        if (card == null || !context.mounted) return;
        final logo = card.logoUri;
        avatarUrl.value = logo.isNotEmpty ? logo : null;
        final locale = ref.read(uiLocaleProvider);
        merchantNameState.value = card.displayName(locale);
        storeStatusLabel.value = card.statusLabel(locale);
      } catch (_) {
        if (!context.mounted) return;
        if ((merchantNameState.value ?? '').isEmpty &&
            (merchantName ?? '').isNotEmpty) {
          merchantNameState.value = merchantName;
        }
      }
    }

    Future<void> markAllReadSafe() async {
      // 標記該對象所有訊息為已讀（後端 read_all 端點，一次清空整個對話）。
      try {
        await ref
            .read(chatRepositoryProvider)
            .markAllMessagesRead(participantId: merchantId);
      } catch (_) {}
    }

    // One-time product suggestion（從瀏覽紀錄挑一張圖，15 秒自動消失、點擊保留）。
    Future<void> maybeShowProductSuggestion() async {
      if (suggestionShown.value) return;
      try {
        String imgUrl = '';
        try {
          imgUrl = await ref
              .read(productRepositoryProvider)
              .getMerchantFirstProductImage(merchantId);
        } catch (_) {}
        // 沒有可用資料則跳過
        if (imgUrl.isEmpty) return;

        final payload = <String, dynamic>{
          'title': '推薦商品',
          'productName': '推薦商品',
          'skuId': '',
          'merchantId': merchantId,
          'merchantName': (merchantName ?? '').toString(),
          if (imgUrl.isNotEmpty) 'imageUrl': imgUrl,
        };

        final id = 'suggest_${DateTime.now().millisecondsSinceEpoch}';
        if (!context.mounted) return;
        // 假的「商家」訊息（左側）：senderId == merchantId → 渲染在左邊。
        ref
            .read(chatThreadProvider(arg).notifier)
            .addOptimistic(
              ChatMessage(
                id: id,
                senderId: merchantId,
                receiverId: '',
                messageContent: json.encode(payload),
                messageType: 'product',
                timestamp: DateTime.now(),
              ),
            );
        suggestionShown.value = true;
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (scrollCtrl.hasClients) {
            scrollCtrl.animateTo(
              0,
              duration: const Duration(milliseconds: 300),
              curve: Curves.easeOut,
            );
          }
        });
        suggestionTimers.value[id] = Timer(const Duration(seconds: 15), () {
          if (!context.mounted) return;
          if (pinnedSuggestionIds.value.contains(id)) return;
          ref.read(chatThreadProvider(arg).notifier).removeOptimistic(id);
        });
      } catch (_) {}
    }

    // ── Seed ephemeral-dedup set from history / refetch（非 live applyIncoming）──
    // live applyIncoming：AsyncData→AsyncData（prev is AsyncData）；
    // 首次載入 / refetch：…→AsyncLoading→AsyncData（refetchAndReplace 設
    // const AsyncLoading()，prev is AsyncLoading）。以 `prev is AsyncLoading`
    // 為閘，只從歷史 / 重抓結果 seed，不與 live 到貨競爭（live 訊息仍須入場動畫）。
    ref.listen<AsyncValue<List<ChatMessage>>>(chatThreadProvider(arg), (
      prev,
      next,
    ) {
      if (next is AsyncData<List<ChatMessage>> && prev is AsyncLoading) {
        for (final m in next.value) {
          if (m.id.isNotEmpty) seenIncomingIds.value.add(m.id);
        }
      }
    });

    // ── Effect A: activeChatGate enter/clear + WS 連線生命週期 ─────────────────
    useEffect(() {
      // repo/gateNotifier 兩者皆為 keepAlive provider；在 effect 設置時（widget 存活
      // 期）capture 一次，cleanup 只用存好的實例，不在 cleanup 內再呼叫 ref.read——
      // widget 拆除路徑上 ref 已不安全，即使目標 provider 本身未被回收也一樣。
      final repo = ref.read(chatRepositoryProvider);
      final gateNotifier = ref.read(activeChatGateProvider.notifier);
      // 標記正在聊天的對象，避免 main.dart 重複彈通知 + FCM 前景 banner 重彈。
      gateNotifier.enter(peerId: merchantId, conversationId: normalizedConvId);
      repo.connectForChat();
      return () {
        gateNotifier.clear();
        final merchantIdInt = int.tryParse(merchantId);
        if (merchantIdInt != null && merchantIdInt != 0) {
          repo.unsubscribePresence(merchantIdInt, _peerRoleSeller);
        }
        highlightTimer.value?.cancel();
        typingTimer.value?.cancel();
        typingDebounce.value?.cancel();
        if (sentTyping.value) {
          repo.sendTypingIndicator(receiverId: merchantId, isTyping: false);
        }
        for (final t in suggestionTimers.value.values) {
          t.cancel();
        }
        repo.disconnectFromChat();
      };
    }, const []);

    // ── Effect B: presence 訂閱（含重連後重新 subscribe） ──────────────────────
    useEffect(() {
      final merchantIdInt = int.tryParse(merchantId);
      if (merchantIdInt == null || merchantIdInt == 0) return null;
      final repo = ref.read(chatRepositoryProvider);
      final presenceSub = repo.onPresence.listen((e) {
        if (e.peerId != merchantIdInt || e.peerRole != _peerRoleSeller) return;
        if (e.online != isMerchantOnline.value) {
          isMerchantOnline.value = e.online;
        }
      });
      if (repo.connectionState == ChatConnectionState.connected) {
        repo.subscribePresence(merchantIdInt, _peerRoleSeller);
      }
      // WS 連線後（含重連）重新送一次 subscribe。
      final connSub = repo.connectionStateStream.skip(1).listen((s) {
        if (s == ChatConnectionState.connected) {
          repo.subscribePresence(merchantIdInt, _peerRoleSeller);
        }
      });
      return () {
        presenceSub.cancel();
        connSub.cancel();
      };
    }, const []);

    // ── Effect C: WS frames 的 ephemeral 反應 ────────────────────────────────
    // 訊息「入列 / dedup / 已讀狀態」由 notifier 處理；這裡只處理 UI 副作用：
    // 打字指示器、入場動畫標記、未讀 badge、自動捲到底、conversationId 回填、標記已讀。
    useEffect(() {
      final sub = ref.read(chatRepositoryProvider).onMessage.listen((m) {
        // 打字指示器
        if (m.messageType == 'typing' || m.messageType == 'start_typing') {
          if (m.senderId == merchantId) {
            isPeerTyping.value = true;
            typingTimer.value?.cancel();
            typingTimer.value = Timer(const Duration(seconds: 4), () {
              isPeerTyping.value = false;
            });
          }
          return;
        }
        if (m.messageType == 'stop_typing') {
          if (m.senderId == merchantId) isPeerTyping.value = false;
          return;
        }
        // 已讀回執由 notifier.markMyMessagesRead 處理，screen 無 ephemeral 反應。
        if (m.messageType == 'read_receipt') return;

        final inConversation =
            m.senderId == merchantId || m.receiverId == merchantId;
        if (!inConversation) return;

        // conversationId 回填（進場時可能為 null）讓 FCM banner gate 也能比對到。
        if (ref.read(activeChatGateProvider).conversationId == null &&
            (m.conversationId ?? '').isNotEmpty) {
          ref
              .read(activeChatGateProvider.notifier)
              .enter(peerId: merchantId, conversationId: m.conversationId);
        }
        // 跳過控制類訊息（空內容），避免誤觸 UI 副作用。
        if (m.messageContent.isEmpty && m.messageType != 'text') return;

        // 只對「收到的商家訊息」做 ephemeral 反應；自己的訊息由送出流程處理。
        if (m.senderId != merchantId) return;
        if (m.id.isNotEmpty) {
          if (seenIncomingIds.value.contains(m.id)) return;
          seenIncomingIds.value.add(m.id);
        }
        // 收到對方訊息 → 關閉打字指示器
        typingTimer.value?.cancel();
        isPeerTyping.value = false;
        // 標記為需要入場動畫的新訊息
        if (m.id.isNotEmpty) animatedIds.value.add(m.id);
        // 如果用戶往上捲了，累計未讀數
        if (!isNearBottom()) unreadAway.value += 1;
        // 標記已讀
        unawaited(
          ref.read(chatRepositoryProvider).markConversationAsRead(m.id),
        );
        // 在底部則自動捲到最新
        if (isNearBottom()) {
          WidgetsBinding.instance.addPostFrameCallback((_) {
            if (!context.mounted) return;
            if (scrollCtrl.hasClients) {
              scrollCtrl.animateTo(
                0.0,
                duration: const Duration(milliseconds: 300),
                curve: Curves.easeOut,
              );
            }
          });
        }
      });
      return sub.cancel;
    }, const []);

    // ── Effect D: TextField 輸入 → 發送 WebSocket typing 事件 ──────────────────
    useEffect(() {
      // repo capture 一次：onChanged 由 textCtrl 的 listener 觸發，使用者快速切走
      // 畫面時可能與 unmount 競態，屆時 ref 已不安全，改用存好的實例呼叫。
      final repo = ref.read(chatRepositoryProvider);
      void onChanged() {
        final hasText = textCtrl.text.trim().isNotEmpty;
        if (hasText && !sentTyping.value) {
          sentTyping.value = true;
          repo.sendTypingIndicator(receiverId: merchantId);
        }
        typingDebounce.value?.cancel();
        typingDebounce.value = Timer(const Duration(seconds: 3), () {
          if (sentTyping.value) {
            sentTyping.value = false;
            repo.sendTypingIndicator(receiverId: merchantId, isTyping: false);
          }
        });
      }

      textCtrl.addListener(onChanged);
      return () => textCtrl.removeListener(onChanged);
    }, const []);

    // ── Effect E: scroll-to-bottom FAB listener ──────────────────────────────
    useEffect(() {
      void applyUpdate({
        required bool shouldShowFab,
        required bool shouldClearUnread,
      }) {
        if (!context.mounted) return;
        if (showFab.value != shouldShowFab) showFab.value = shouldShowFab;
        if (shouldClearUnread) unreadAway.value = 0;
      }

      void onScroll() {
        final nearBottom = isNearBottom();
        final shouldShowFab = !nearBottom;
        final shouldClearUnread = nearBottom && unreadAway.value > 0;
        if (showFab.value == nearBottom || shouldClearUnread) {
          // 若在 layout/paint/semantics 期間觸發，延遲到 idle / post-frame 再更新，
          // 避免 parentDataDirty assertion。
          final phase = SchedulerBinding.instance.schedulerPhase;
          if (phase == SchedulerPhase.idle ||
              phase == SchedulerPhase.postFrameCallbacks) {
            applyUpdate(
              shouldShowFab: shouldShowFab,
              shouldClearUnread: shouldClearUnread,
            );
          } else {
            WidgetsBinding.instance.addPostFrameCallback((_) {
              applyUpdate(
                shouldShowFab: shouldShowFab,
                shouldClearUnread: shouldClearUnread,
              );
            });
          }
        }
      }

      scrollCtrl.addListener(onScroll);
      return () => scrollCtrl.removeListener(onScroll);
    }, const []);

    // ── Effect F: 商家 profile（avatar + store name） ─────────────────────────
    useEffect(() {
      unawaited(loadMerchantProfile());
      return null;
    }, const []);

    // ── Effect G: 首次載入完成後 → 標記全部已讀 + 一次性商品推薦 ─────────────────
    useEffect(() {
      if (!chatState.hasValue) return null;
      unawaited(markAllReadSafe());
      if (!suggestionShown.value) {
        unawaited(maybeShowProductSuggestion());
      }
      return null;
    }, [chatState.hasValue]);

    // ── Reply-jump closures ───────────────────────────────────────────────────
    Future<void> scrollToMessageById(String id) async {
      if (id.isEmpty) return;

      Future<bool> ensureForKey(GlobalKey key) async {
        try {
          await Scrollable.ensureVisible(
            key.currentContext!,
            alignment: 0.2,
            duration: const Duration(milliseconds: 250),
            curve: Curves.easeInOut,
          );
          return true;
        } catch (_) {
          return false;
        }
      }

      Future<bool> ensureCurrentVisible() async {
        final key = msgRowKeys.value[id];
        if (key != null && key.currentContext != null) {
          return ensureForKey(key);
        }
        return false;
      }

      if (await ensureCurrentVisible()) {
        triggerHighlight(id);
        return;
      }

      final loaded = await ensureMessageLoaded(id);
      if (!loaded) {
        if (context.mounted) {
          showFloatingToast(context, '找不到被回覆的訊息', isSuccess: false);
        }
        return;
      }

      if (!context.mounted) return;
      await _waitForLayout();

      final targetIndex = readMsgs().indexWhere((m) => m.id == id);
      if (targetIndex == -1) {
        if (context.mounted) {
          showFloatingToast(context, '找不到被回覆的訊息', isSuccess: false);
        }
        return;
      }

      if (scrollCtrl.hasClients) {
        final estimatedOffset = estimateOffsetForIndex(targetIndex);
        try {
          await scrollCtrl.animateTo(
            estimatedOffset,
            duration: const Duration(milliseconds: 250),
            curve: Curves.easeOutQuad,
          );
        } catch (_) {}
      }

      await _waitForLayout();

      var success = await ensureCurrentVisible();
      var attempt = 0;
      while (!success && attempt < 3) {
        await _waitForLayout();
        success = await ensureCurrentVisible();
        attempt++;
      }

      if (success) {
        triggerHighlight(id);
        return;
      }

      if (context.mounted) {
        showFloatingToast(context, '找不到被回覆的訊息', isSuccess: false);
      }
    }

    // 依「預覽文字」遍歷整個對話，直到找到對應訊息並跳轉。
    Future<void> jumpToByPreview(String preview) async {
      final target = preview.trim();
      if (target.isEmpty) return;
      var id = _findMessageIdByPreview(readMsgs(), target);
      if (id.isNotEmpty) {
        await scrollToMessageById(id);
        return;
      }
      final notifier = ref.read(chatThreadProvider(arg).notifier);
      var guard = 0;
      while (guard++ < 60) {
        final more = await notifier.loadOlder();
        id = _findMessageIdByPreview(readMsgs(), target);
        if (id.isNotEmpty) {
          await scrollToMessageById(id);
          return;
        }
        if (!more) break;
      }
      if (!context.mounted) return;
      showFloatingToast(context, '找不到被回覆的訊息', isSuccess: false);
    }

    void setReplyTo(ChatMessage m) {
      replyTarget.value = m;
      replyPreview.value = _computeReplyPreview(m);
    }

    void showBuyerMessageContextMenu(BuildContext context, ChatMessage m) {
      final isSelf = m.senderId != merchantId;
      showModalBottomSheet(
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
              ListTile(
                leading: Icon(
                  AppIcons.copy,
                  color: AppIconColors.defaultColor,
                  size: 22.sp,
                ),
                title: Text('複製', style: TextStyle(fontSize: 16)),
                onTap: () {
                  Clipboard.setData(ClipboardData(text: m.messageContent));
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
              ListTile(
                leading: Icon(
                  AppIcons.reply,
                  color: AppIconColors.defaultColor,
                  size: 22.sp,
                ),
                title: Text('回覆', style: TextStyle(fontSize: 16)),
                onTap: () {
                  Navigator.pop(ctx);
                  setReplyTo(m);
                },
              ),
              if (!isSelf) ...[
                ListTile(
                  leading: Icon(
                    AppIcons.reportOutlined,
                    color: AppIconColors.warning,
                    size: 22.sp,
                  ),
                  title: Text('檢舉此訊息', style: TextStyle(fontSize: 16)),
                  onTap: () {
                    Navigator.pop(ctx);
                    showReportSheet(
                      context,
                      targetType: ReportTargetType.chatMessage,
                      targetId: m.id,
                    );
                  },
                ),
                ListTile(
                  leading: Icon(
                    AppIcons.blockOutlined,
                    color: AppIconColors.danger,
                    size: 22.sp,
                  ),
                  title: Text('封鎖此使用者', style: TextStyle(fontSize: 16)),
                  onTap: () {
                    Navigator.pop(ctx);
                    showBlockConfirmDialog(
                      context,
                      userId: merchantId,
                      userLabel: merchantNameState.value ?? '此使用者',
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

    // 顯示商品選擇底部彈窗（列出該賣家的商品）。
    Future<void> openProductPicker() async {
      try {
        final merchantIdInt = int.tryParse(merchantId);
        if (merchantIdInt == null) return;
        final rawList = await ref
            .read(productRepositoryProvider)
            .getMerchantProducts(merchantId);
        if (!context.mounted) return;
        final products = <Map<String, dynamic>>[];
        for (final item in rawList) {
          if (item is Map<String, dynamic>) {
            products.add(item);
          } else if (item is Map) {
            products.add(Map<String, dynamic>.from(item));
          }
        }
        if (products.length == 1 && products.first.containsKey('records')) {
          final records = products.first['records'];
          if (records is List) {
            products
              ..clear()
              ..addAll(records.whereType<Map<String, dynamic>>());
          }
        }
        if (products.isEmpty) {
          if (context.mounted) showCommonSnackBar(context, '此賣家暫無上架商品');
          return;
        }
        final selected = await showProductPickerSheet(
          context: context,
          products: products,
        );
        if (selected == null || !context.mounted) return;

        final payload = <String, dynamic>{
          'messageType': 'product',
          'productName': selected['productName'] ?? selected['name'] ?? '',
          'price':
              selected['price'] ??
              selected['unitPrice'] ??
              selected['priceMin'] ??
              '',
          // gateway `ProductItem`：imageKey ＋ imageUrl 成對（49 §6.1）。
          'imageUrl': selected['imageUrl'] ?? selected['imageKey'] ?? '',
          'skuId': selected['skuId'] ?? selected['id'] ?? '',
          'productId': selected['productId'] ?? selected['id'] ?? '',
        };
        final content = json.encode(payload);
        final tempId = 'temp_${DateTime.now().millisecondsSinceEpoch}';
        final notifier = ref.read(chatThreadProvider(arg).notifier);
        animatedIds.value.add(tempId);
        notifier.addOptimistic(
          ChatMessage(
            id: tempId,
            senderId: '',
            receiverId: merchantId,
            messageContent: content,
            messageType: 'product',
            timestamp: DateTime.now(),
          ),
        );
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (scrollCtrl.hasClients) {
            scrollCtrl.animateTo(
              0,
              duration: const Duration(milliseconds: 300),
              curve: Curves.easeOut,
            );
          }
        });
        try {
          final resp = await ref
              .read(chatRepositoryProvider)
              .sendMessage(
                receiverId: merchantId,
                messageContent: content,
                messageType: 'product',
                extra: {'preview': cardLabelForType('product')!},
              );
          final realId = (resp?['data']?['messageId'] ?? '').toString();
          if (realId.isNotEmpty && realId != 'null') {
            notifier.confirmOptimistic(tempId, realId);
          }
        } catch (_) {
          notifier.removeOptimistic(tempId);
          animatedIds.value.remove(tempId);
        }
      } catch (e) {
        if (context.mounted) showCommonSnackBar(context, '載入商品失敗');
      }
    }

    // 顯示過往訂單底部彈窗（跟賣家端共用 ChatOrderPickerSheet）。
    Future<void> openPastOrders() async {
      try {
        final list = await ref
            .read(orderRepositoryProvider)
            .getOrdersRaw(page: 1);
        final cands = <Map<String, dynamic>>[];
        for (final m in list) {
          final mid = _pickStr(m, const [
            'matchedMerchantId',
            'merchantId',
            'merchantUserId',
            'sellerId',
            'shopId',
            'storeId',
          ]);
          if (mid != merchantId) continue;
          cands.add(m);
        }

        if (cands.isEmpty) {
          if (context.mounted) {
            showFloatingToast(context, '沒有過往訂單紀錄', isSuccess: false);
          }
          return;
        }

        cands.sort((a, b) => _tsFromOrder2(b).compareTo(_tsFromOrder2(a)));

        if (!context.mounted) return;
        final selected = await showOrderPickerSheet(
          context: context,
          orders: cands,
          isSeller: false,
        );
        if (selected == null || !context.mounted) return;

        final payload = <String, dynamic>{
          'messageType': 'order_prev',
          'orderNo': _pickStr(selected, const ['orderSn', 'orderNo', 'sn']),
          ...selected,
        };
        final content = json.encode(payload);
        final tempId = 'temp_${DateTime.now().millisecondsSinceEpoch}';
        final notifier = ref.read(chatThreadProvider(arg).notifier);
        animatedIds.value.add(tempId);
        notifier.addOptimistic(
          ChatMessage(
            id: tempId,
            senderId: '',
            receiverId: merchantId,
            messageContent: content,
            messageType: 'order_prev',
            timestamp: DateTime.now(),
          ),
        );
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (scrollCtrl.hasClients) {
            scrollCtrl.animateTo(
              0,
              duration: const Duration(milliseconds: 300),
              curve: Curves.easeOut,
            );
          }
        });
        try {
          final resp = await ref
              .read(chatRepositoryProvider)
              .sendMessage(
                receiverId: merchantId,
                messageContent: content,
                messageType: 'order_prev',
                extra: {'preview': cardLabelForType('order_prev')!},
              );
          final realId = (resp?['data']?['messageId'] ?? '').toString();
          if (realId.isNotEmpty && realId != 'null') {
            notifier.confirmOptimistic(tempId, realId);
          }
        } catch (_) {
          notifier.removeOptimistic(tempId);
          animatedIds.value.remove(tempId);
        }
      } catch (e) {
        if (context.mounted) {
          showFloatingToast(context, '載入訂單失敗: $e', isSuccess: false);
        }
      }
    }

    Future<void> onRefresh() async {
      await ref.read(chatThreadProvider(arg).notifier).refetchAndReplace();
      await markAllReadSafe();
      await loadMerchantProfile();
    }

    Future<void> onSend() async {
      final t = textCtrl.text.trim();
      if (t.isEmpty) return;
      String messageContent;
      final reply = replyTarget.value;
      if (reply != null && reply.id.isNotEmpty) {
        messageContent = json.encode({
          'messageType': 'text',
          'text': t,
          'replyToId': reply.id,
        });
      } else {
        messageContent = t;
      }
      textCtrl.clear();
      replyTarget.value = null;
      replyPreview.value = null;
      final tempId = 'temp_${DateTime.now().millisecondsSinceEpoch}';
      animatedIds.value.add(tempId);
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (scrollCtrl.hasClients) {
          scrollCtrl.animateTo(
            0,
            duration: const Duration(milliseconds: 300),
            curve: Curves.easeOut,
          );
        }
      });
      try {
        await ref
            .read(chatThreadProvider(arg).notifier)
            .send(content: messageContent, optimisticTempId: tempId);
      } catch (e) {
        // notifier.send 失敗時已移除樂觀氣泡；這裡清掉動畫標記並提示。
        animatedIds.value.remove(tempId);
        if (context.mounted) {
          showFloatingToast(context, '傳送失敗: $e', isSuccess: false);
        }
      }
    }

    Future<void> onImagePick() async {
      try {
        final picker = ImagePicker();
        final image = await picker.pickImage(source: ImageSource.gallery);
        if (image == null) return;
        final file = File(image.path);
        final uploadResult = await ref
            .read(mediaRepositoryProvider)
            .uploadImage(file: file);
        final tempId = 'temp_${DateTime.now().millisecondsSinceEpoch}';
        final notifier = ref.read(chatThreadProvider(arg).notifier);
        animatedIds.value.add(tempId);
        // 樂觀氣泡先用 previewUrl 顯示。
        notifier.addOptimistic(
          ChatMessage(
            id: tempId,
            senderId: '',
            receiverId: merchantId,
            messageContent: uploadResult.previewUrl,
            messageType: 'image',
            timestamp: DateTime.now(),
          ),
        );
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (scrollCtrl.hasClients) {
            scrollCtrl.animateTo(
              0,
              duration: const Duration(milliseconds: 300),
              curve: Curves.easeOut,
            );
          }
        });
        try {
          final response = await ref
              .read(chatRepositoryProvider)
              .sendMessage(
                receiverId: merchantId,
                messageContent: uploadResult.stagedToken,
                messageType: 'image',
              );
          final realId = (response?['data']?['messageId'] ?? '').toString();
          final cdnUrl = (response?['data']?['content'] ?? '').toString();
          final hasReal = realId.isNotEmpty && realId != 'null';
          if (hasReal) {
            notifier.confirmOptimistic(tempId, realId);
          }
          // 用 server 回傳的 CDN URL 替換 previewUrl（tmp 已被刪除）。
          if (cdnUrl.isNotEmpty && cdnUrl != uploadResult.previewUrl) {
            final swapId = hasReal ? realId : tempId;
            notifier.removeOptimistic(swapId);
            notifier.addOptimistic(
              ChatMessage(
                id: swapId,
                senderId: '',
                receiverId: merchantId,
                messageContent: cdnUrl,
                messageType: 'image',
                timestamp: DateTime.now(),
              ),
            );
          }
        } catch (_) {
          notifier.removeOptimistic(tempId);
          animatedIds.value.remove(tempId);
          if (context.mounted) {
            showFloatingToast(context, '圖片傳送失敗', isSuccess: false);
          }
        }
      } catch (e) {
        if (context.mounted) {
          showFloatingToast(context, '圖片傳送失敗: $e', isSuccess: false);
        }
      }
    }

    // ── Message list builder ──────────────────────────────────────────────────
    Widget buildMessageList() {
      // 只在「首次載入、尚無任何訊息」時顯示整頁 spinner。reconnect / 下拉重抓時
      // isLoading 同樣為 true，但 riverpod 會保留前一份資料，故不可清空既有列表。
      if (chatState.isLoading && msgs.isEmpty) {
        return const Center(child: HkBrandLoader(size: HkBrandLoader.page));
      }
      final total = msgs.length;

      return Stack(
        children: [
          // Background dot pattern
          const Positioned.fill(child: ChatConversationDotPattern()),
          ListView.builder(
            controller: scrollCtrl,
            physics: const AlwaysScrollableScrollPhysics(),
            padding: EdgeInsets.fromLTRB(12.w, 8.h, 12.w, 20.h),
            reverse: true,
            itemCount: total + 1,
            itemBuilder: (_, index) {
              // Bottom anchor
              if (index == 0) return const SizedBox(height: 1);

              final msgIndex = total - index;
              final m = msgs[msgIndex];
              final String mid = m.id;
              final GlobalKey rowKey = mid.isNotEmpty
                  ? msgRowKeys.value.putIfAbsent(mid, () => GlobalKey())
                  : GlobalKey();

              // Grouping logic
              final prevMsg = msgIndex > 0 ? msgs[msgIndex - 1] : null;
              final nextMsg = msgIndex < total - 1 ? msgs[msgIndex + 1] : null;
              final curTsVal = m.timestamp.millisecondsSinceEpoch;
              final prevTsVal = prevMsg?.timestamp.millisecondsSinceEpoch;
              final nextTsVal = nextMsg?.timestamp.millisecondsSinceEpoch;
              final isSelf = m.senderId != merchantId;

              final showTimeSeparator =
                  prevMsg != null &&
                  prevTsVal != null &&
                  (curTsVal - prevTsVal).abs() > 5 * 60 * 1000;
              final sameSenderAsPrev =
                  prevMsg != null &&
                  (prevMsg.senderId == merchantId) ==
                      (m.senderId == merchantId) &&
                  !(prevMsg.messageType == 'product' ||
                      prevMsg.messageType == 'order_prev' ||
                      prevMsg.messageType == 'order_check') &&
                  !(m.messageType == 'product' ||
                      m.messageType == 'order_prev' ||
                      m.messageType == 'order_check') &&
                  !showTimeSeparator;
              final sameSenderAsNext =
                  nextMsg != null &&
                  (nextMsg.senderId == merchantId) ==
                      (m.senderId == merchantId) &&
                  !(nextMsg.messageType == 'product' ||
                      nextMsg.messageType == 'order_prev' ||
                      nextMsg.messageType == 'order_check') &&
                  !(m.messageType == 'product' ||
                      m.messageType == 'order_prev' ||
                      m.messageType == 'order_check') &&
                  (nextTsVal == null ||
                      (nextTsVal - curTsVal).abs() <= 5 * 60 * 1000);
              final isFirstInGroup = !sameSenderAsPrev;
              final isLastInGroup = !sameSenderAsNext;
              final shouldAnimate =
                  mid.isNotEmpty && animatedIds.value.contains(mid);

              // Time separator pill
              Widget? timeSeparator;
              if (showTimeSeparator) {
                final t = DateTime.fromMillisecondsSinceEpoch(
                  curTsVal,
                ).toLocal();
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
                          fontSize: 12,
                          color: AppColors.textSecondary,
                        ),
                      ),
                    ),
                  ),
                );
              }

              // 訂單 / 商品卡片 → 用 Figma 對齊的新 bubble widget
              final isOrderCard =
                  m.messageType == 'order_prev' ||
                  m.messageType == 'order_check' ||
                  m.messageType == 'order_check_note' ||
                  ChatOrderBubble.tryParseOrder(m.messageContent) != null;
              final isProductCard =
                  !isOrderCard &&
                  (m.messageType == 'product' ||
                      ChatProductBubble.tryParseProduct(m.messageContent) !=
                          null);
              if (isOrderCard || isProductCard) {
                Widget cardContent;
                if (isOrderCard) {
                  cardContent = ChatOrderBubble(
                    messageContent: m.messageContent,
                    isSelf: isSelf,
                    onTap: () {
                      final order = ChatOrderBubble.tryParseOrder(
                        m.messageContent,
                      );
                      if (order != null) {
                        final orderNo =
                            (order['orderNo'] ?? order['orderSn'] ?? '')
                                .toString();
                        if (orderNo.isNotEmpty) {
                          context.push(
                            '/orders/detail',
                            extra: {'orderSn': orderNo},
                          );
                        }
                      }
                    },
                  );
                } else {
                  cardContent = ChatProductBubble(
                    messageContent: m.messageContent,
                    isSelf: isSelf,
                    onTap: () {
                      final product = ChatProductBubble.tryParseProduct(
                        m.messageContent,
                      );
                      if (product != null) {
                        context.push(
                          '/merchant-product',
                          extra: MerchantItemDetailScreen(
                            title:
                                (product['productName'] ??
                                        product['name'] ??
                                        '')
                                    .toString(),
                            imageUrl:
                                (product['imageUrl'] ?? product['image'] ?? '')
                                    .toString(),
                            price: num.tryParse(
                              (product['price'] ?? '0').toString(),
                            ),
                            skuId:
                                (product['skuId'] ?? product['productId'] ?? '')
                                    .toString(),
                            merchantId: merchantId,
                            merchantName:
                                merchantNameState.value ?? merchantName,
                          ),
                        );
                      }
                    },
                  );
                }

                Widget bubble = Align(
                  alignment: isSelf
                      ? Alignment.centerRight
                      : Alignment.centerLeft,
                  child: Padding(
                    padding: EdgeInsets.only(
                      left: isSelf ? 8.w : (32.w + 8.w),
                      right: isSelf ? 4.w : 8.w,
                      top: isFirstInGroup ? 8.h : 2.h,
                      bottom: 2.h,
                    ),
                    child: cardContent,
                  ),
                );
                if (shouldAnimate) {
                  Future.delayed(const Duration(milliseconds: 300), () {
                    animatedIds.value.remove(mid);
                  });
                  bubble = _MessageEntrance(isSelf: isSelf, child: bubble);
                }
                if (timeSeparator != null) {
                  return Column(children: [timeSeparator, bubble]);
                }
                return bubble;
              }

              // Inline bubble rendering for text/image (seller-style)
              final isImage = m.messageType == 'image';

              // Avatar slot (show at bottom of group, left side only)
              Widget avatarSlot = const SizedBox.shrink();
              if (!isSelf) {
                if (isLastInGroup) {
                  avatarSlot = Container(
                    width: 32.w,
                    height: 32.w,
                    margin: EdgeInsets.only(bottom: 4.h, right: 8.w),
                    child: ClipOval(
                      child:
                          (avatarUrl.value != null &&
                              avatarUrl.value!.isNotEmpty)
                          ? OssImage(
                              imageSource: avatarUrl.value!,
                              scene: ImageScene.chatAvatar,
                              width: 32.w,
                              height: 32.w,
                              fit: BoxFit.cover,
                              errorWidget: _AvatarInitial(
                                name: merchantNameState.value ?? '?',
                              ),
                            )
                          : _AvatarInitial(
                              name: merchantNameState.value ?? '?',
                            ),
                    ),
                  );
                } else {
                  avatarSlot = SizedBox(width: 32.w + 6.w);
                }
              }

              // Message content
              Widget content;
              // Check if it's a reply message (JSON with replyToId)
              String? replyPreviewText;
              String? replyTargetId;
              String displayText = m.messageContent;
              if (!isImage &&
                  displayText.isNotEmpty &&
                  displayText.startsWith('{') &&
                  displayText.endsWith('}')) {
                try {
                  final d = json.decode(displayText);
                  if (d is Map) {
                    final mm = Map<String, dynamic>.from(d);
                    for (final k in [
                      'replyToId',
                      'replyId',
                      'inReplyTo',
                      'parentId',
                      'quotedId',
                    ]) {
                      final v = mm[k];
                      if (v != null && v.toString().trim().isNotEmpty) {
                        replyTargetId = v.toString().trim();
                        break;
                      }
                    }
                    final tx = (mm['text'] ?? '').toString();
                    if (tx.isNotEmpty) displayText = tx;
                    if (replyTargetId != null && replyTargetId.isNotEmpty) {
                      replyPreviewText = _previewForMessageId(
                        msgs,
                        replyTargetId,
                      );
                      if (replyPreviewText.isEmpty) {
                        replyPreviewText =
                            (mm['replyText'] ?? mm['replyPreview'] ?? '')
                                .toString();
                      }
                    }
                  }
                } catch (_) {}
              }
              // Fallback: 「預覽」\n正文 format
              if (replyTargetId == null && displayText.contains('\n')) {
                final re = RegExp(r'^「(.+?)」\s*\n([\s\S]*)');
                final mm = re.firstMatch(displayText);
                if (mm != null) {
                  final quoted = mm.group(1) ?? '';
                  final rest = mm.group(2) ?? '';
                  replyPreviewText = quoted;
                  displayText = rest.trim();
                }
              }

              if (isImage) {
                content = GestureDetector(
                  onLongPress: () =>
                      _openFullScreenImage(context, m.messageContent),
                  child: ClipRRect(
                    borderRadius: BorderRadius.circular(8.r),
                    child: _buildInlineImage(m.messageContent),
                  ),
                );
              } else {
                content = Text(
                  displayText,
                  softWrap: true,
                  overflow: TextOverflow.visible,
                  style: TextStyle(
                    fontSize: 16,
                    color: isSelf ? AppColors.bgCard : AppColors.textEmphasis,
                    height: 1.5,
                  ),
                );
              }

              // Wrap with reply quote if needed
              if (replyPreviewText != null && replyPreviewText.isNotEmpty) {
                final accentColor = isSelf
                    ? AppColors.bgCard.withValues(alpha: 0.8)
                    : AppColors.primary;
                final bgColor = isSelf
                    ? AppColors.bgCard.withValues(alpha: 0.1)
                    : AppColors.gray100;
                final replyTextColor = isSelf
                    ? AppColors.bgCard.withValues(alpha: 0.9)
                    : AppColors.textPrimary;
                final replyBlock = GestureDetector(
                  onTap: () {
                    if (replyTargetId != null && replyTargetId.isNotEmpty) {
                      scrollToMessageById(replyTargetId);
                    } else {
                      unawaited(jumpToByPreview(replyPreviewText!));
                    }
                  },
                  child: Container(
                    margin: EdgeInsets.only(bottom: 8.h),
                    decoration: BoxDecoration(
                      color: bgColor,
                      borderRadius: BorderRadius.circular(6.r),
                      border: Border(
                        left: BorderSide(color: accentColor, width: 3.5.w),
                      ),
                    ),
                    padding: EdgeInsets.fromLTRB(12.w, 4.h, 12.w, 4.h),
                    child: Text(
                      replyPreviewText,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 14,
                        color: replyTextColor,
                        height: 1.4,
                      ),
                    ),
                  ),
                );
                content = Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [replyBlock, content],
                );
              }

              // Time + read status
              final t = DateTime.fromMillisecondsSinceEpoch(curTsVal).toLocal();
              final h12 = t.hour == 0
                  ? 12
                  : (t.hour > 12 ? t.hour - 12 : t.hour);
              final timeStr =
                  '${t.hour < 12 ? "上午" : "下午"}${h12.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}';
              final timeStatusRow = Row(
                mainAxisSize: MainAxisSize.min,
                children: [
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
                      m.read ? '已讀' : '未讀',
                      style: TextStyle(
                        fontSize: 12,
                        color: AppColors.textSecondary,
                      ),
                    ),
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
                        child: DraggableReplyWrapper(
                          enable: !isSelf,
                          onSwipeRightReply: () => setReplyTo(m),
                          child: GestureDetector(
                            behavior: HitTestBehavior.opaque,
                            onTap: isImage
                                ? () => _openFullScreenImage(
                                    context,
                                    m.messageContent,
                                  )
                                : null,
                            onLongPress: () {
                              HapticFeedback.heavyImpact();
                              showBuyerMessageContextMenu(context, m);
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
                                    key: rowKey,
                                    onChanged: (size) =>
                                        recordMessageHeight(mid, size.height),
                                    child: Container(
                                      padding: isImage
                                          ? EdgeInsets.zero
                                          : EdgeInsets.fromLTRB(
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
                                          sameSenderAsNext,
                                        ),
                                        border:
                                            (mid.isNotEmpty &&
                                                mid == highlightId.value)
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
                                        ],
                                      ),
                                      child: content,
                                    ),
                                  ),
                                  // Time + read status
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

              if (shouldAnimate) {
                Future.delayed(const Duration(milliseconds: 300), () {
                  animatedIds.value.remove(mid);
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
        // Scroll-to-bottom FAB
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
        // Typing indicator — 仿 Telegram：左下出現對方頭像 + 跳動 dots 氣泡
        if (isPeerTyping.value)
          Positioned(
            left: 12.w,
            bottom: 8.h,
            child: _TypingIndicator(
              avatarUrl: avatarUrl.value,
              fallbackName: merchantNameState.value ?? merchantName,
            ),
          ),
      ],
    );

    final content = Column(
      children: [
        commonTopBar(
          context: context,
          horizontalPadding: EdgeInsets.zero,
          child: _TopBar(
            merchantName: merchantNameState.value ?? merchantName,
            merchantId: merchantId,
            avatarUrl: avatarUrl.value,
            isMerchantOnline: isMerchantOnline.value,
            storeStatusLabel: storeStatusLabel.value,
          ),
        ),
        Expanded(child: messageArea),
        // 連線中斷錯誤 banner（AsyncError）→ 點擊重抓。
        if (chatState.hasError)
          Material(
            color: Colors.transparent,
            child: InkWell(
              onTap: () => ref
                  .read(chatThreadProvider(arg).notifier)
                  .refetchAndReplace(),
              child: Container(
                width: double.infinity,
                padding: EdgeInsets.symmetric(vertical: 12.h, horizontal: 16.w),
                color: AppColors.dangerSoft,
                child: Text(
                  '連線中斷，點擊重試',
                  textAlign: TextAlign.center,
                  style: TextStyle(fontSize: 14, color: AppColors.warningText),
                ),
              ),
            ),
          ),
        _PremiumInputBar(
          controller: textCtrl,
          focusNode: inputFocusNode,
          replyPreview: replyPreview.value,
          onCancelReply: () {
            replyTarget.value = null;
            replyPreview.value = null;
          },
          onPickOrder: () async {
            await openPastOrders();
          },
          onPickProduct: () async {
            await openProductPicker();
          },
          onSend: () {
            unawaited(onSend());
          },
          onImagePick: () {
            unawaited(onImagePick());
          },
        ),
      ],
    );

    return Scaffold(backgroundColor: AppColors.bgPrimary, body: content);
  }
}

class _TopBar extends StatelessWidget {
  const _TopBar({
    this.merchantName,
    this.merchantId,
    this.avatarUrl,
    this.isMerchantOnline = false,
    this.storeStatusLabel,
  });
  final String? storeStatusLabel;
  final String? merchantName;
  final String? merchantId;
  final String? avatarUrl;
  final bool isMerchantOnline;

  Widget _buildAvatar() {
    final s = (avatarUrl ?? '').trim();
    const double size = 40;
    const Widget defaultAvatar = SizedBox(
      width: size,
      height: size,
      child: HkPlaceholder(HkPlaceholderKind.merchantAvatar),
    );
    Widget img;
    if (s.isEmpty) {
      img = defaultAvatar;
    } else {
      // inline 判準只看 `data:` scheme（media_image.dart isInlineImageData）。
      // 舊寫法「不是 http 且長度 > 60」會把 78~80 字元的 objectKey 誤判成 base64。
      final bool looksBase64 = isInlineImageData(s);
      if (looksBase64) {
        try {
          final b64 = s.contains(',') ? s.split(',').last : s;
          final bytes = base64Decode(b64);
          img = Image.memory(
            bytes,
            fit: BoxFit.cover,
            width: size,
            height: size,
            gaplessPlayback: true,
          );
        } catch (_) {
          img = defaultAvatar;
        }
      } else {
        String finalUrl = s;
        final v = s.trim();
        if (v.startsWith('http://') || v.startsWith('https://')) {
          finalUrl = v;
        } else if (v.startsWith('/member/avatar/view') ||
            v.startsWith('member/avatar/view') ||
            v.startsWith('/member-avatars') ||
            v.startsWith('member-avatars') ||
            v.startsWith('avatars/')) {
          finalUrl = v.normalizeAvatarUrl();
        } else if (v.startsWith('/')) {
          final b = Uri.parse(F.appFlavor.apiBaseUrl);
          final origin =
              '${b.scheme}://${b.host}${b.hasPort ? ':${b.port}' : ''}';
          finalUrl = '$origin$v';
        } else {
          finalUrl = v.normalizeAvatarUrl();
        }
        img = AppCachedImage(
          imageUrl: finalUrl,
          scene: ImageScene.chatAvatar,
          fit: BoxFit.cover,
          width: size,
          height: size,
        );
      }
    }
    // Avatar with online dot
    return SizedBox(
      width: size,
      height: size,
      child: Stack(
        children: [
          Container(
            width: size,
            height: size,
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              border: Border.all(color: AppColors.borderStrong, width: 1.8),
              color: AppColors.surfaceMuted,
            ),
            clipBehavior: Clip.antiAlias,
            child: ClipOval(child: img),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return ChatConversationHeaderContent(
      avatar: _buildAvatar(),
      title: merchantName ?? 'Farm1',
      statusLabel: storeStatusLabel ?? (isMerchantOnline ? '線上' : '離線'),
      statusColor: storeStatusLabel == null && isMerchantOnline
          ? AppColors.success.withValues(alpha: 0.85)
          : AppColors.textDisabled,
      onBack: () => context.pop(),
    );
  }
}

/// Premium input bar — 對齊 Figma 設計稿，外層全交給 ChatInputBar 處理
/// （白底 / top border / shadow / bottom safe area / padding）
class _PremiumInputBar extends StatelessWidget {
  const _PremiumInputBar({
    required this.controller,
    required this.focusNode,
    required this.onSend,
    required this.onImagePick,
    this.onPickOrder,
    this.onPickProduct,
    this.replyPreview,
    this.onCancelReply,
  });
  final TextEditingController controller;
  final FocusNode focusNode;
  final VoidCallback onSend;
  final VoidCallback onImagePick;
  final VoidCallback? onPickOrder;
  final VoidCallback? onPickProduct;
  final String? replyPreview;
  final VoidCallback? onCancelReply;
  @override
  Widget build(BuildContext context) {
    return ChatInputBar(
      controller: controller,
      focusNode: focusNode,
      sending: false,
      onSend: onSend,
      onPlusPressed: () => _showChatPlusMenu(
        context: context,
        onPickPhoto: onImagePick,
        onPickOrder: onPickOrder ?? () {},
        onPickProduct: onPickProduct,
      ),
      topSlot: AnimatedSize(
        duration: const Duration(milliseconds: 200),
        curve: Curves.easeOutCubic,
        child: (replyPreview ?? '').isNotEmpty
            ? Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  // 用左 border 代替 IntrinsicHeight，避免 AnimatedSize 重算
                  // intrinsic 尺寸時觸發 parentDataDirty。
                  Container(
                    margin: EdgeInsets.only(bottom: 8.h),
                    decoration: BoxDecoration(
                      color: AppColors.gray100,
                      borderRadius: BorderRadius.circular(12.r),
                      border: Border(
                        left: BorderSide(
                          color: AppColors.primary,
                          width: 3.5.w,
                        ),
                      ),
                    ),
                    padding: EdgeInsets.symmetric(vertical: 8.h),
                    child: Row(
                      children: [
                        SizedBox(width: 10.w),
                        Expanded(
                          child: Text(
                            replyPreview!,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(
                              fontSize: 14,
                              color: AppColors.textPrimary,
                              height: 1.3,
                            ),
                          ),
                        ),
                        GestureDetector(
                          onTap: onCancelReply,
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
}

class _MeasuredSize extends StatefulWidget {
  const _MeasuredSize({
    super.key,
    required this.child,
    required this.onChanged,
  });
  final Widget child;
  final ValueChanged<Size> onChanged;

  @override
  State<_MeasuredSize> createState() => _MeasuredSizeState();
}

class _MeasuredSizeState extends State<_MeasuredSize> {
  Size? _oldSize;
  bool _measureScheduled = false;

  @override
  Widget build(BuildContext context) {
    // 每次 build 只排一次 post-frame 測量，避免大量重複排程
    // 在 semantics 更新期間觸發 parentDataDirty。
    if (!_measureScheduled) {
      _measureScheduled = true;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        _measureScheduled = false;
        if (!mounted) return;
        final size = context.size;
        if (size != null && (_oldSize == null || size != _oldSize)) {
          _oldSize = size;
          widget.onChanged(size);
        }
      });
    }
    return widget.child;
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// Premium UI widgets (ported from seller chat_thread_screen)
// ─────────────────────────────────────────────────────────────────────────────

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
/// 左側對方頭像 + 白色氣泡內 3 個彈跳 dots（wave 動畫）。
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
    // 1.4s 一個完整 wave 循環，跟 Telegram 接近
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

  /// 每個 dot 在 wave 中的「跳起」階段（0..1 的 t 值區間）。
  /// dot0: 0.00–0.40, dot1: 0.15–0.55, dot2: 0.30–0.70
  /// 之後到 1.0 都靜止，模擬 Telegram「跳完一輪→停一下→再跳」感。
  double _dotOffset(int i, double t) {
    final start = i * 0.15;
    const span = 0.40;
    if (t < start || t > start + span) return 0;
    final local = (t - start) / span; // 0..1
    // 用 sin 做平滑上下：sin(pi * x) → 0 → 1 → 0
    final h = math.sin(math.pi * local);
    return -h * 5.0; // 跳 5px
  }

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        // 對方頭像（與一般 incoming 訊息頭像同尺寸 32x32）
        Container(
          width: 32.w,
          height: 32.w,
          margin: EdgeInsets.only(right: 8.w, bottom: 4.h),
          child: ClipOval(
            child: (widget.avatarUrl != null && widget.avatarUrl!.isNotEmpty)
                ? OssImage(
                    imageSource: widget.avatarUrl!,
                    scene: ImageScene.chatAvatar,
                    width: 32.w,
                    height: 32.w,
                    fit: BoxFit.cover,
                    errorWidget: _AvatarInitial(
                      name: widget.fallbackName ?? '?',
                    ),
                  )
                : _AvatarInitial(name: widget.fallbackName ?? '?'),
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
                crossAxisAlignment: CrossAxisAlignment.center,
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
