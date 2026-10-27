import 'dart:convert';

// ⚠️ 跨 app 共用邏輯：本檔的 derivePreview / cardLabelForType / _isImageType /
// _looksLikeImageUrl / _replyBody / _looksLikeCardJson / _looksLikeCardMap 與
// hawkr_seller 的 chat_preview.dart 為「同一份」，修改時務必兩邊同步，
// 讓同一則對話在 buyer / seller 顯示一致的預覽文案。
// （兩者為獨立 repo 無法互相 import；長期解為抽 shared package。）

/// 由 conversation map 取出最後訊息內容/型別，衍生聊天列表預覽文案（buyer 專屬入口）。
///
/// key 直接取 `GET /app/chat/conversations` 回的 `lastMessage` / `lastMessageType`
/// （im.proto `Conversation.last_message` / `last_message_type`）。不要再補
/// lastContent / lastMessageContent / content / messageType / lastType / type
/// 這類候選——後端一個都沒回過，掛著只會讓人以為它們可能存在。
String chatConversationPreview(Map<String, dynamic> c) {
  final content = (c['lastMessage'] ?? '').toString();
  final type = (c['lastMessageType'] ?? '').toString();
  return derivePreview(content, type);
}

/// 由「最後一則訊息」內容 + 型別衍生聊天列表預覽文案。
///
/// 核心不變式：任何以 `{` / `[` 開頭的結構化 payload 都不得洩漏成原文
/// —— 即使後端把預覽截斷成不完整的 JSON（`json.decode` 會失敗）也一樣。
String derivePreview(String lastMsg, String lastType) {
  final t = lastType.trim().toLowerCase();
  if (t.isNotEmpty) {
    if (_isImageType(t)) return '傳送了一張圖片';
    final card = cardLabelForType(t);
    if (card != null) return card;
  }

  final trimmed = lastMsg.trim();
  if (trimmed.isEmpty) return '';

  // 回覆格式：↩︎ 預覽\n內容（buyer）或「預覽」\n內容（seller）→ 只顯示真正內容
  final reply = _replyBody(trimmed);
  if (reply != null) return reply;

  final low = trimmed.toLowerCase();
  if (low.startsWith('http') && _looksLikeImageUrl(low)) return '傳送了一張圖片';
  if (low.startsWith('data:image/')) return '傳送了一張圖片';

  if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
    try {
      final decoded = json.decode(trimmed);
      if (decoded is Map) {
        final txt = (decoded['text'] ?? decoded['content'] ?? '')
            .toString()
            .trim();
        if (txt.isNotEmpty) return txt;
        final mt = (decoded['messageType'] ?? decoded['type'] ?? '')
            .toString()
            .toLowerCase();
        if (_isImageType(mt)) return '傳送了一張圖片';
        final card = cardLabelForType(mt);
        if (card != null) return card;
        final hasOrders = decoded['orders'] is List;
        final hasTitle = (decoded['title'] ?? '').toString().isNotEmpty;
        if (hasOrders && hasTitle) return '傳送了一張卡片';
        for (final k in ['url', 'image', 'imageUrl', 'img', 'src', 'path']) {
          final v = decoded[k];
          if (v != null && v.toString().trim().isNotEmpty) return '傳送了一張圖片';
        }
      }
      if (_looksLikeCardJson(trimmed)) return '傳送了一張卡片';
    } catch (_) {
      // 解析失敗（例如後端截斷過的預覽 JSON），落到下方 fallback
    }
    // 結構化 payload 辨識不出細節時仍以卡片字樣呈現，絕不顯示原始 JSON。
    return '傳送了一張卡片';
  }

  return trimmed;
}

String? cardLabelForType(String t) {
  if (t.contains('product')) return '傳送了一張商品卡片';
  if (t.contains('order')) return '傳送了一張訂單卡片';
  if (t == 'card' || t.contains('card')) return '傳送了一張卡片';
  return null;
}

bool _isImageType(String t) =>
    t.contains('image') ||
    t.contains('photo') ||
    t.contains('video') ||
    t.contains('media') ||
    t.contains('picture');

bool _looksLikeImageUrl(String low) =>
    low.endsWith('.jpg') ||
    low.endsWith('.jpeg') ||
    low.endsWith('.png') ||
    low.endsWith('.gif') ||
    low.endsWith('.webp') ||
    low.contains('/image/') ||
    low.contains('/images/') ||
    low.contains('/files/tmp/') ||
    low.contains('=image');

String? _replyBody(String trimmed) {
  if (trimmed.startsWith('↩︎')) {
    final nl = trimmed.indexOf('\n');
    if (nl > 0 && nl + 1 < trimmed.length) {
      final body = trimmed.substring(nl + 1).trim();
      if (body.isNotEmpty) return body;
    }
  }
  final quoted = RegExp(r'^「(.+?)」\s*\n([\s\S]*)').firstMatch(trimmed);
  if (quoted != null) {
    final rest = (quoted.group(2) ?? '').trim();
    if (rest.isNotEmpty) return rest;
  }
  return null;
}

bool _looksLikeCardJson(String text) {
  if (!(text.startsWith('{') || text.startsWith('['))) return false;
  try {
    final parsed = json.decode(text);
    if (parsed is List) {
      if (parsed.isEmpty) return false;
      final first = parsed.first;
      if (first is Map) {
        return _looksLikeCardMap(Map<String, dynamic>.from(first));
      }
      return false;
    }
    if (parsed is Map) {
      var map = Map<String, dynamic>.from(parsed);
      if (map['card'] is Map) map = Map<String, dynamic>.from(map['card']);
      return _looksLikeCardMap(map);
    }
  } catch (_) {}
  return false;
}

bool _looksLikeCardMap(Map<String, dynamic> map) {
  final type = (map['messageType'] ?? map['type'] ?? '')
      .toString()
      .toLowerCase();
  if (type == 'card' || type.contains('order') || type == 'product') {
    return true;
  }
  if (map.containsKey('orders')) return true;
  if (map.containsKey('orderSn') ||
      map.containsKey('orderNo') ||
      map.containsKey('order_id') ||
      map.containsKey('orderId')) {
    return true;
  }
  if (map.containsKey('title') &&
      (map.containsKey('subtitle') ||
          map.containsKey('description') ||
          map.containsKey('desc'))) {
    return true;
  }
  return false;
}
