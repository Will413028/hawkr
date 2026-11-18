import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/core/networking/suspension_signal.dart';

/// 社群讀寫改走 Connect（A7-4c-2）之後不得回頭的結構：留言不即時推送、留言與回覆不用 Map、
/// 擁有權只讀伺服器的 `viewer_can_*`、討論區沒有按讚。以字串讀原始碼，不 import 被守的檔。
String _read(String path) {
  final file = File(path);
  expect(file.existsSync(), isTrue, reason: '$path 不存在，cwd 可能不對');
  return file.readAsStringSync();
}

String _code(String source) => source
    .split('\n')
    .map((line) => line.replaceFirst(RegExp(r'(?<!:)//.*$'), ''))
    .join('\n');

void main() {
  test('留言不再有即時推送（SSE）', () {
    expect(
      File('lib/features/social/data/comment_stream_service.dart').existsSync(),
      isFalse,
    );
    final offenders = <String>[];
    for (final entity in Directory('lib').listSync(recursive: true)) {
      if (entity is! File || !entity.path.endsWith('.dart')) continue;
      if (entity.path.endsWith('agui_sse.dart')) continue;
      final source = _code(entity.readAsStringSync());
      if (source.contains('CommentStreamService') ||
          source.contains('comment_stream_service')) {
        offenders.add(entity.path);
      }
    }
    expect(offenders, isEmpty);
  });

  test('留言面板用生成的訊息，不用 Map，也不比對擁有權', () {
    final source = _code(
      _read('lib/shared/widgets/comments_bottom_sheet.dart'),
    );
    expect(source, isNot(contains('Map<String, dynamic>')));
    expect(source, isNot(contains('isMine')));
    expect(source, isNot(contains('is_mine')));
    expect(source, contains('viewerCanDelete'));
    expect(source, contains('viewerCanReply'));
    expect(source, contains('viewerCanComment'));
  });

  test('討論區沒有按讚，也不用 isMine', () {
    final source = _code(
      _read('lib/features/seller/presentation/widgets/merchant_chat_tab.dart'),
    );
    expect(source, isNot(contains('onLike')));
    expect(source, isNot(contains('likeCount')));
    expect(source, isNot(contains('/like')));
    expect(source, isNot(contains('isMine')));
    expect(source, contains('viewerCanEdit'));
    expect(source, contains('viewerCanDelete'));
    expect(source, contains('viewerCanReply'));
  });

  test('RecordPostView 走沒有停權 interceptor 的可選登入傳輸，不列為背景寫入', () {
    final source = _code(_read('lib/core/networking/connect_provider.dart'));
    final publicClient = RegExp(
      r'SocialPublicServiceClient socialPublicClient\(Ref ref\) =>\s*SocialPublicServiceClient\(([^;]*)\);',
    ).firstMatch(source);
    expect(publicClient, isNotNull);
    expect(
      publicClient!.group(1),
      contains('connectOptionalAuthTransportProvider'),
    );
    final optionalTransport = RegExp(
      r'Transport connectOptionalAuthTransport\(Ref ref\) =>([^;]*);',
    ).firstMatch(source);
    expect(optionalTransport, isNotNull);
    expect(
      optionalTransport!.group(1),
      isNot(contains('suspensionInterceptor')),
    );
    expect(
      isBackgroundConnectProcedure(
        '/hawkr.social.v1alpha1.SocialPublicService/RecordPostView',
      ),
      isFalse,
    );
  });

  test('social_repository 只剩檢舉（4c-3 處理）', () {
    final source = _code(
      _read('lib/features/social/data/social_repository.dart'),
    );
    final methods = RegExp(
      r'Future<[\w<>?, ]+>\s+(\w+)\(',
    ).allMatches(source).map((m) => m.group(1)).toSet();
    expect(methods, {'submitReport', 'getMyReports'});
  });
}
