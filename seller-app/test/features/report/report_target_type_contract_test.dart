import 'dart:io';

import 'package:hawkr_seller/features/report/data/report_model.dart';
import 'package:flutter_test/flutter_test.dart';

/// 檢舉 `target_type` 是 social-service 的路由鍵：送錯值 API 不會拒絕，
/// 但 `target_owner_id` 會落成 NULL。本檔守住 enum 的字面值、與後端白名單
/// 一一對應，以及只有聊天室開檢舉。
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
  });

  group('檢舉開啟點', () {
    // 留言頁不開檢舉流程；店主在自己店的內容上用刪除與封鎖。
    for (final file in [
      'lib/features/post/presentation/page_home_feed_screen.dart',
      'lib/features/post/presentation/widgets/page_echo_tab.dart',
    ]) {
      test('${file.split('/').last} 不開檢舉', () {
        final source = File(file).readAsStringSync();
        expect(source.contains('showReportSheet'), isFalse);
        expect(source.contains('ReportTargetType.'), isFalse);
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
    // 只掃 `targetType:` 具名參數直接接字串字面值的形狀；
    // 負向前瞻排除 `'targetType': ...` 的 map key。
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
