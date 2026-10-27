import 'dart:io';

import 'package:hawkr_seller/features/chat/data/chat_model.dart';
import 'package:hawkr_seller/features/order/data/after_sale_model.dart';
import 'package:hawkr_seller/features/order/data/order_model.dart';
import 'package:hawkr_seller/features/post/data/post_model.dart';
import 'package:flutter_test/flutter_test.dart';

/// 買家頭像欄位收斂契約測試（backend `697c989bd`：九個名字 → `avatarKey`/`avatarUrl`）。
///
/// ## fixture 出處
///
/// 每個 fixture 都是**實打回應的逐字複製**（含欄位順序），不是照型別宣告推的形狀——
/// 契約測試用捏造的 wire shape，只會把錯誤假設一起背書。
///
/// 擷取環境：本機全棧（OrbStack，`./dev.sh`），gateway 跑 backend 分支 `dev-alexis`
/// 的 `697c989bd`（Air 熱載）。擷取時間 2026-08-05，seller `9`（翠綠有機農場）、
/// buyer `2001`（王小明）、post `7490620040132062876`。
/// 原始 capture 存於 backend repo `.claude/tmp/media-seo-0805/evidence/
/// P4-seller-wire-capture.json`。
///
/// | fixture | 來源 |
/// |---|---|
/// | [_wsChatFrame] | WS `/app/chat/ws` 的 `type=chat` frame |
/// | [_buyersMe] | `GET /app/buyers/me` 的 `data` |
/// | [_conversationItem] | `GET /app/chat/conversations` 的 `conversations[]` |
/// | [_sellerOrderRecord] | `GET /app/orders?view=seller` 的 `records[]` |
/// | [_publicProductReview] | `GET /public/products/{id}/reviews` 的 `records[]` |
/// | [_postCommentRead] | `GET /public/posts/{id}/comments` 的 `records[]` |
/// | [_postCommentWrite] | `POST /app/posts/{id}/comments` 的 `data` |
/// | [_forumRecord] | `GET /public/sellers/{id}/forum` 的 `records[]` |
///
/// 一個例外（**白盒**，本機無法實打，已標注）：
/// - [_afterSaleCase]：本機無售後案件（`aftersale_cases` 0 列）；欄名取自
///   `gateway/internal/handler/aftersales.go` `AfterSaleCaseSummaryResponse`
///   的 struct tag，由該 repo 的 `TestAvatarDTOs_OutwardJSONKeyIsFlatAvatarKey`
///   逐 struct marshal 釘住。
///
/// ## 為什麼每組都配一個「只給舊名」的反例
///
/// 「新名解析得出來」證明不了「舊名已經不再被讀」——兩者同時成立時，回退任何一個
/// 讀取點都不會轉紅。所以每組都補一個「fixture 只有舊欄位名 → 結果必須為空」的
/// 斷言，那才是這份測試的鑑別力來源。
const String _avatarKey =
    'public/user_avatar/2685914682/20260729/2c7c6d22-28b9-4ad0-b802-f5818bbb632b.png';
const String _avatarUrl =
    'https://test-media.example.com/i/original/$_avatarKey';

/// WS `/app/chat/ws` 的 `type=chat` frame（逐字）。
/// 注意：**沒有** `senderAvatar`，也沒有 `avatarUrl`——WS 不經 Gateway 組 URL
/// （49 §6.7 S4），只帶裸 objectKey。
const Map<String, dynamic> _wsChatFrame = {
  'roomId': 'b2001_s9',
  'type': 'chat',
  'content': 'P4-seller WS 契約驗證訊息',
  'from': '2001',
  'to': '9',
  'messageId': '7490625959463587764',
  'seqId': '4',
  'timestamp': 1785904397836,
  'msgType': 1,
  'senderName': '王小明',
  'avatarKey': _avatarKey,
};

/// `GET /app/buyers/me` 的 `data`（逐字）。整支端點於本批改成 camelCase DTO。
const Map<String, dynamic> _buyersMe = {
  'id': '2001',
  'accountId': '2001',
  'nickname': '王小明',
  'avatarKey': _avatarKey,
  'avatarUrl': _avatarUrl,
  'level': 'bronze',
  'birthDate': '1990-05-21',
  'city': '台北市',
  'job': '工程師',
  'email': 'buyer1@hawkr.test',
  'phone': '0933111222',
  'gender': 'male',
  'createdAt': '2026-07-13T17:33:45Z',
  'updatedAt': '2026-08-05T04:10:43Z',
};

/// `GET /app/chat/conversations` 的單筆 `conversations[]`（逐字）。
const Map<String, dynamic> _conversationItem = {
  'avatarKey': _avatarKey,
  'avatarUrl': _avatarUrl,
  'lastMessage': 'P4-seller WS 契約驗證訊息',
  'lastMessageTime': '2026-08-05T04:33:17Z',
  'lastMessageType': 'text',
  'logoKey': '',
  'logoUrl': '',
  'storeCode': '',
  'storeName': '',
  'targetNickname': '王小明',
  'targetUserId': '2001',
  'unreadCount': '1',
};

/// `GET /app/orders?view=seller` 的單筆 `records[]`（逐字，`items` 略）。
const Map<String, dynamic> _sellerOrderRecord = {
  'buyerNickname': '王小明',
  'status': '0',
  'totalActualAmount': '249',
  'itemCount': '1',
  'storeId': '9',
  'sellerName': '翠綠有機農場',
  'sellerCity': '台南市',
  'sellerDistrict': '新化區',
  'shippingMethod': '3001',
  'deliveryTemp': '3202',
  'shippingStatus': '91',
  'buyerId': '2001',
  'recipientName': 'Smoke Test',
  'createTime': '2026-08-05T04:20:28.375875Z',
  'shippingAddress': '台北市內湖區瑞光路1號',
  'avatarKey': _avatarKey,
  'avatarUrl': _avatarUrl,
  'orderNo': 'O509225915',
};

/// `GET /public/products/99001/reviews` 的單筆 `records[]`（逐字）。
const Map<String, dynamic> _publicProductReview = {
  'reviewId': '7466164685380591689',
  'buyerId': '2001',
  'productId': '99001',
  'username': '王**',
  'avatarKey': _avatarKey,
  'avatarUrl': _avatarUrl,
  'rating': '4',
  'content': '不錯，會再回購',
  'createdAt': '2026-05-29T16:32:55Z',
  'imageKeys': <String>[],
  'imageUrls': <String>[],
};

/// `GET /public/posts/{id}/comments` 的單筆 `records[]`（逐字）。
const Map<String, dynamic> _postCommentRead = {
  'userName': '王小明',
  'avatarKey': _avatarKey,
  'avatarUrl': _avatarUrl,
  'content': 'P4 契約驗證留言（買家 2001）',
  'id': '7490620122015620422',
  'postId': '7490620040132062876',
  'isMine': false,
  'parentId': '0',
  'replyToId': '0',
  'createdAt': 1785903006,
  'likeCount': 0,
  'role': 1,
  'isLiked': false,
};

/// `POST /app/posts/{id}/comments` 的 `data`（逐字）。
/// 本批之前這裡是 `user_avatar_key`（snake，proto 直吐）——同一筆留言在建立時
/// 與列表時欄名不同，樂觀更新必定讀不到頭像。現已與讀取端同形狀。
const Map<String, dynamic> _postCommentWrite = {
  'userName': '',
  'avatarKey': '',
  'avatarUrl': '',
  'content': 'P4-seller 寫入回應形狀驗證',
  'id': '7490625711635290314',
  'postId': '7490620040132062876',
  'isMine': true,
  'parentId': '0',
  'replyToId': '0',
  'createdAt': 1785904338,
  'likeCount': 0,
  'role': 1,
  'isLiked': false,
};

/// `GET /public/sellers/9/forum` 的單筆 `records[]`（逐字）。
const Map<String, dynamic> _forumRecord = {
  'userName': '王小明',
  'avatarKey': _avatarKey,
  'avatarUrl': _avatarUrl,
  'content': 'P4 討論區驗證留言（買家 2001）',
  'id': '7490620245815251003',
  'sellerId': '9',
  'isMine': false,
  'parentId': '0',
  'replyToId': '0',
  'createdAt': 1785903035,
  'replyCount': 0,
};

/// 售後案件 summary（**白盒**，見檔頭）。
const Map<String, dynamic> _afterSaleCase = {
  'caseNo': 'AS202608050001',
  'orderNo': 'O509225915',
  'buyerId': '2001',
  'storeId': '9',
  'buyerName': '王小明',
  'avatarKey': _avatarKey,
  'avatarUrl': _avatarUrl,
  'logoKey': '',
  'logoUrl': '',
  'status': '10',
};

void main() {
  group('WS 聊天訊息（唯一頭像來源）', () {
    test('讀 avatarKey', () {
      final m = ChatMessage.fromJson(Map<String, dynamic>.from(_wsChatFrame));
      expect(m.senderAvatar, _avatarKey);
      // WS 只帶 key，不帶顯示 URL（49 §6.7 S4）。
      expect(_wsChatFrame.containsKey('avatarUrl'), isFalse);
    });

    test('舊名 senderAvatar 已不再被讀（只給舊名 → 解析為空）', () {
      final m = ChatMessage.fromJson({
        'id': '1',
        'from': '2001',
        'to': '9',
        'content': 'hi',
        'type': 'text',
        'timestamp': 1785904397836,
        'senderAvatar': _avatarKey,
        'sender_avatar': _avatarKey,
        'senderAvatarUrl': _avatarUrl,
        'avatarUrl': _avatarUrl,
      });
      expect(m.senderAvatar, isNull);
    });
  });

  group('賣家訂單卡片的買家頭像', () {
    test('讀 avatarUrl（顯示 URL，非裸 key）', () {
      final o = OrderSummary.fromJson(
        Map<String, dynamic>.from(_sellerOrderRecord),
      );
      expect(o.buyerAvatar, _avatarUrl);
      expect(o.buyerName, '王小明');
    });

    test('刻意不吃 avatarKey：下游 BuyerAvatarWidget 以 startsWith(http) 判斷有無圖，'
        '餵裸 objectKey 會靜默退成預設頭像', () {
      final o = OrderSummary.fromJson({
        'orderNo': 'O1',
        'buyerNickname': '王小明',
        'avatarKey': _avatarKey,
      });
      expect(o.buyerAvatar, isEmpty);
    });

    test('舊名 buyerAvatarUrl 已不再被讀（只給舊名 → 解析為空）', () {
      final o = OrderSummary.fromJson({
        'orderNo': 'O1',
        'buyerAvatarKey': _avatarKey,
        'buyerAvatarUrl': _avatarUrl,
      });
      expect(o.buyerAvatar, isEmpty);
    });
  });

  group('售後案件的買家頭像', () {
    test('讀 avatarUrl', () {
      final c = AfterSaleCase.fromJson(
        Map<String, dynamic>.from(_afterSaleCase),
      );
      expect(c.avatarUrl, _avatarUrl);
    });

    test('舊名 buyerAvatarUrl 已不再被讀（只給舊名 → 解析為空）', () {
      final c = AfterSaleCase.fromJson({
        'caseNo': 'AS1',
        'buyerAvatarUrl': _avatarUrl,
      });
      expect(c.avatarUrl, isEmpty);
    });
  });

  group('貼文留言（讀取與寫入同形狀）', () {
    test('讀取端點：avatarKey 優先', () {
      final c = Comment.fromJson(Map<String, dynamic>.from(_postCommentRead));
      expect(c.avatarKey, _avatarKey);
      expect(c.author, '王小明');
    });

    test('寫入端點回應形狀與讀取端一致（含頭像欄成對）', () {
      expect(_postCommentWrite.containsKey('avatarKey'), isTrue);
      expect(_postCommentWrite.containsKey('avatarUrl'), isTrue);
      expect(_postCommentWrite.containsKey('user_avatar_key'), isFalse);
      final c = Comment.fromJson(Map<String, dynamic>.from(_postCommentWrite));
      expect(c.id, 7490625711635290314);
    });

    test('avatarKey 缺席時退回 avatarUrl', () {
      final c = Comment.fromJson({'id': '1', 'avatarUrl': _avatarUrl});
      expect(c.avatarKey, _avatarUrl);
    });

    test('舊名已不再被讀（只給舊名 → 解析為空）', () {
      final c = Comment.fromJson({
        'id': '1',
        'userAvatarKey': _avatarKey,
        'userAvatarUrl': _avatarUrl,
        'user_avatar_key': _avatarKey,
        'userAvatar': _avatarUrl,
      });
      expect(c.avatarKey, isEmpty);
    });
  });

  group('契約 fixture 的舊名確實已從 wire 消失', () {
    const retired = [
      'avatar',
      'senderAvatar',
      'sender_avatar',
      'targetAvatarKey',
      'targetAvatarUrl',
      'buyerAvatarKey',
      'buyerAvatarUrl',
      'userAvatarKey',
      'userAvatarUrl',
      'user_avatar_key',
      'userAvatar',
      'object_key',
    ];
    const fixtures = <String, Map<String, dynamic>>{
      'ws chat frame': _wsChatFrame,
      'GET /app/buyers/me': _buyersMe,
      'GET /app/chat/conversations': _conversationItem,
      'GET /app/orders?view=seller': _sellerOrderRecord,
      'GET /public/products/{id}/reviews': _publicProductReview,
      'GET /public/posts/{id}/comments': _postCommentRead,
      'POST /app/posts/{id}/comments': _postCommentWrite,
      'GET /public/sellers/{id}/forum': _forumRecord,
    };

    for (final entry in fixtures.entries) {
      test(entry.key, () {
        for (final key in retired) {
          expect(
            entry.value.containsKey(key),
            isFalse,
            reason: '${entry.key} 仍帶已退役欄位 `$key`',
          );
        }
      });
    }
  });

  group('lib/ 內不得殘留已退役的頭像 wire key', () {
    // 讀取點散落在 widget 私有函式裡（chat_list_screen `_resolveItem`、
    // page_home_feed `_CommentVM.fromRaw`、page_echo_tab、comments_bottom_sheet、
    // product_reviews_screen），無法從測試直接呼叫。這裡改用來源掃描把它們一起
    // 釘住：把任何一處改回舊名，這條就轉紅。
    //
    // 白名單只有 FCM／通知中心那條路——它們讀的是 notification-service 的
    // MQ 模板輸出（`sender_avatar` / `buyerAvatar` …），**不是** Gateway HTTP
    // 契約，本批沒動、也不該動。
    const allowedFiles = {
      'lib/features/notification/data/push/manager.dart',
      'lib/features/notification/presentation/notifications_screen.dart',
    };
    const retiredLiterals = [
      "'senderAvatar'",
      "'sender_avatar'",
      "'senderAvatarUrl'",
      "'sender_avatar_url'",
      "'targetAvatarKey'",
      "'targetAvatarUrl'",
      "'buyerAvatarKey'",
      "'buyerAvatarUrl'",
      "'userAvatarKey'",
      "'userAvatarUrl'",
      "'user_avatar_key'",
      "'userAvatar'",
    ];

    test('current inventory is empty', () {
      final violations = <String>[];
      final files = Directory('lib')
          .listSync(recursive: true)
          .whereType<File>()
          .where((f) => f.path.endsWith('.dart'))
          .where((f) => !f.path.endsWith('.g.dart'))
          .where((f) => !f.path.endsWith('.freezed.dart'));

      for (final file in files) {
        final rel = file.path.replaceAll(r'\', '/');
        if (allowedFiles.contains(rel)) continue;
        final lines = file.readAsLinesSync();
        for (var i = 0; i < lines.length; i++) {
          for (final literal in retiredLiterals) {
            if (lines[i].contains(literal)) {
              violations.add('$rel:${i + 1} → $literal');
            }
          }
        }
      }

      expect(violations, isEmpty);
    });
  });
}
