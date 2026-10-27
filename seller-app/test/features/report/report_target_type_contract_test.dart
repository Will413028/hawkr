import 'dart:io';

import 'package:hawkr_seller/features/report/data/report_model.dart';
import 'package:flutter_test/flutter_test.dart';

/// 檢舉 `target_type` 契約。
///
/// 這個值不是顯示用的分類，它是 social-service 的**路由鍵**：admin 裁決時
/// `FanoutHideDispatcher.HideTarget` / `ResolveOwner`
/// （`services/social-service/internal/usecase/dispatcher.go:41,63`）
/// 用它決定要打哪張表。送錯值 API 不會拒絕（六個值都在
/// `report.go:19-26` 的白名單裡），而是安靜地查錯表 →
/// `target_owner_id` 落庫成 NULL → 之後要 hide/ban 找不到對象。
///
/// 因此正確的值取決於「這則內容是從哪個端點讀來的」，
/// 不是 UI 上叫不叫「留言」——賣家 App 兩個頁面的選單文案都是「檢舉留言」，
/// 而它們背後是不同的表。這正是兩者曾經被寫反的原因。
void main() {
  group('ReportTargetType wire values', () {
    test('每個值都對齊後端白名單字面值', () {
      expect(ReportTargetType.review.value, 'REVIEW');
      expect(ReportTargetType.post.value, 'POST');
      expect(ReportTargetType.postComment.value, 'POST_COMMENT');
      expect(ReportTargetType.sellerForum.value, 'SELLER_FORUM');
      expect(ReportTargetType.chatMessage.value, 'CHAT_MESSAGE');
      expect(ReportTargetType.user.value, 'USER');
    });

    test('後端白名單六個值全數有對應 enum，沒有多也沒有少', () {
      // 對齊 services/social-service/internal/usecase/report.go:19-26
      const backendWhitelist = {
        'REVIEW',
        'POST',
        'POST_COMMENT',
        'SELLER_FORUM',
        'CHAT_MESSAGE',
        'USER',
      };
      expect(
        ReportTargetType.values.map((t) => t.value).toSet(),
        backendWhitelist,
      );
    });

    test('reportTargetTypeLabel 以 enum 為唯一來源，未知值原樣回傳', () {
      for (final t in ReportTargetType.values) {
        expect(reportTargetTypeLabel(t.value), t.label);
      }
      expect(reportTargetTypeLabel('SOMETHING_NEW'), 'SOMETHING_NEW');
    });
  });

  group('檢舉開啟點送出的 target_type 對齊其資料來源端點', () {
    // (畫面, 該畫面載入留言用的 repository 方法, 該方法打的端點與落表, 應送值, 不得出現的值)
    final entryPoints =
        <
          ({
            String file,
            String sourceMethod,
            String table,
            ReportTargetType expected,
            ReportTargetType forbidden,
          })
        >[
          (
            file: 'lib/features/post/presentation/page_home_feed_screen.dart',
            sourceMethod: 'listCommentsRaw',
            table: 'cms_comment (GET /public/posts/{id}/comments)',
            expected: ReportTargetType.postComment,
            forbidden: ReportTargetType.sellerForum,
          ),
          (
            file: 'lib/features/post/presentation/widgets/page_echo_tab.dart',
            sourceMethod: 'listSellerMessages',
            table: 'cms_seller_forum (GET /public/sellers/{id}/forum)',
            expected: ReportTargetType.sellerForum,
            forbidden: ReportTargetType.postComment,
          ),
        ];

    for (final e in entryPoints) {
      test('${e.file.split('/').last} → ${e.expected.value}', () {
        final source = File(e.file).readAsStringSync();

        // 前提：這個畫面確實是從宣稱的那個端點載入留言的。
        // 若哪天資料來源換了，這條先炸，提醒重新判定 target_type。
        expect(
          source.contains(e.sourceMethod),
          isTrue,
          reason:
              '${e.file} 不再呼叫 ${e.sourceMethod}；資料來源變了就要重新判定 '
              'target_type（本表宣稱它來自 ${e.table}）',
        );

        expect(
          source.contains('ReportTargetType.${e.expected.name}'),
          isTrue,
          reason:
              '${e.file} 的檢舉必須送 ${e.expected.value}，'
              '因為它的內容存在 ${e.table}',
        );

        expect(
          source.contains('ReportTargetType.${e.forbidden.name}'),
          isFalse,
          reason:
              '${e.file} 送了 ${e.forbidden.value}，那會讓 admin 裁決打到別張表、'
              'target_owner_id 落庫成 NULL',
        );
      });
    }

    test('聊天室檢舉送 CHAT_MESSAGE', () {
      final source = File(
        'lib/features/chat/presentation/chat_thread_screen.dart',
      ).readAsStringSync();
      expect(source.contains('ReportTargetType.chatMessage'), isTrue);
    });
  });

  test('lib/ 內不得再有 targetType: 傳裸字串的呼叫點（enum 是唯一出口）', () {
    // 裸字串正是兩個畫面寫反卻沒人發現的成因：型別上看不出 POST_COMMENT 與
    // SELLER_FORUM 有什麼不同，兩邊的選單文案又都是「檢舉留言」。
    //
    // 只掃「`targetType:` 具名參數直接接字串字面值」這個形狀，不掃字面值本身：
    // `CHAT_MESSAGE` / `POST_COMMENT` 在推播通知那邊是另一個命名空間
    // （`notification.type`，同名不同義，還有 `POST_LIKE` / `POST_COMMENT_REPLY`
    // 之類本 enum 沒有的值），把它們一起掃進來只會逼人加豁免清單。
    // 負向前瞻排除 `'targetType': ...`（map key，coupon 受眾用的是另一組代碼）。
    final rawTargetTypeArg = RegExp(r"(?<!')targetType:\s*'");
    final offenders = <String>[];

    for (final entity in Directory('lib').listSync(recursive: true)) {
      if (entity is! File || !entity.path.endsWith('.dart')) continue;
      final lines = entity.readAsLinesSync();
      for (var i = 0; i < lines.length; i++) {
        if (rawTargetTypeArg.hasMatch(lines[i])) {
          offenders.add('${entity.path}:${i + 1} → ${lines[i].trim()}');
        }
      }
    }

    expect(
      offenders,
      isEmpty,
      reason: '請改用 ReportTargetType；裸字串無型別保護，寫反不會有人發現',
    );
  });
}
