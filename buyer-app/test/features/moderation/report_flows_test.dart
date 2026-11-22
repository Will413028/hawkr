import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart'
    show LocalizedMessage;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_api/transport.dart'
    show Code, ConnectException, ErrorDetail;
import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.pbenum.dart';
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/features/moderation/data/report_target.dart';
import 'package:hawkr_buyer/features/moderation/data/report_target_type.dart';
import 'package:hawkr_buyer/features/moderation/presentation/report_sheet.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'moderation_fakes.dart';

final _uuidV4 = RegExp(
  r'^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$',
);

class _LegacyAdapter implements HttpClientAdapter {
  final List<RequestOptions> requests = [];
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    return ResponseBody.fromString(
      jsonEncode({
        'code': 0,
        'data': {'reportId': 'legacy-1'},
      }),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

Future<_LegacyAdapter> _pump(
  WidgetTester tester,
  FakeModeration moderation,
  ReportTarget target,
) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 844);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  final legacy = _LegacyAdapter();
  final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
    ..httpClientAdapter = legacy;
  addTearDown(dio.close);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        dioProvider.overrideWithValue(dio),
        moderationAppClientProvider.overrideWithValue(moderation.client()),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp(
          theme: AppTheme.light,
          home: Scaffold(
            body: Builder(
              builder: (context) => TextButton(
                onPressed: () => showReportSheet(context, target: target),
                child: const Text('開啟檢舉'),
              ),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.tap(find.text('開啟檢舉'));
  await tester.pumpAndSettle();
  return legacy;
}

Future<void> _choose(WidgetTester tester, String label) async {
  await tester.tap(find.text(label));
  await tester.pumpAndSettle();
}

Future<void> _submit(WidgetTester tester) async {
  await tester.tap(find.byKey(const ValueKey('report-submit')));
  await tester.pumpAndSettle();
}

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  group('社群對象走 SubmitReport', () {
    for (final (target, kind) in [
      (
        const SocialReportTarget.post('p1'),
        ReportTargetKind.REPORT_TARGET_KIND_POST,
      ),
      (
        const SocialReportTarget.comment('c1'),
        ReportTargetKind.REPORT_TARGET_KIND_COMMENT,
      ),
      (
        const SocialReportTarget.forumEntry('e1'),
        ReportTargetKind.REPORT_TARGET_KIND_FORUM_ENTRY,
      ),
      (
        const SocialReportTarget.review('r1'),
        ReportTargetKind.REPORT_TARGET_KIND_REVIEW,
      ),
    ]) {
      testWidgets('${kind.name}：送出對象種類、原因與 UUIDv4 的 request_id', (
        tester,
      ) async {
        final moderation = FakeModeration();
        final legacy = await _pump(tester, moderation, target);

        await _choose(tester, '垃圾訊息');
        await _submit(tester);

        final request = moderation.submitted.single;
        expect(request.target.kind, kind);
        expect(request.target.id, target.id);
        expect(request.category, PolicyCategory.POLICY_CATEGORY_SPAM);
        expect(request.description, '');
        expect(_uuidV4.hasMatch(request.requestId), isTrue);
        expect(legacy.requests, isEmpty);
        expect(find.byType(ReportSheetWidget), findsNothing);
        await tester.pump(const Duration(seconds: 3));
      });
    }

    testWidgets('只列社群檢舉人可選的八個原因', (tester) async {
      await _pump(
        tester,
        FakeModeration(),
        const SocialReportTarget.post('p1'),
      );

      for (final label in [
        '垃圾訊息',
        '騷擾或霸凌',
        '仇恨或歧視言論',
        '色情或性相關內容',
        '不實或誤導資訊',
        '食品安全',
        '詐欺或操縱交易',
        '其他（請說明）',
      ]) {
        expect(
          find.text(label, skipOffstage: false),
          findsOneWidget,
          reason: label,
        );
      }
      for (final label in ['仿冒商品', '干擾或濫用平台', '假認證']) {
        expect(
          find.text(label, skipOffstage: false),
          findsNothing,
          reason: label,
        );
      }
    });

    testWidgets('同一次嘗試失敗後重試，帶同一個 request_id', (tester) async {
      final moderation = FakeModeration(failFirst: 1);
      await _pump(tester, moderation, const SocialReportTarget.post('p1'));

      await _choose(tester, '垃圾訊息');
      await _submit(tester);
      expect(find.byType(ReportSheetWidget), findsOneWidget);
      expect(find.text('服務暫時不可用'), findsOneWidget);
      await tester.pump(const Duration(seconds: 3));
      await _submit(tester);

      expect(moderation.submitted, hasLength(2));
      expect(
        moderation.submitted[1].requestId,
        moderation.submitted[0].requestId,
      );
      expect(find.byType(ReportSheetWidget), findsNothing);
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('失敗後改了原因就是新的嘗試，換新的 request_id', (tester) async {
      final moderation = FakeModeration(failFirst: 1);
      await _pump(tester, moderation, const SocialReportTarget.post('p1'));

      await _choose(tester, '垃圾訊息');
      await _submit(tester);
      await tester.pump(const Duration(seconds: 3));
      await _choose(tester, '騷擾或霸凌');
      await _submit(tester);

      expect(moderation.submitted, hasLength(2));
      expect(
        moderation.submitted[1].requestId,
        isNot(moderation.submitted[0].requestId),
      );
      expect(
        moderation.submitted[1].category,
        PolicyCategory.POLICY_CATEGORY_HARASSMENT,
      );
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('重新開啟檢舉面板是新的嘗試', (tester) async {
      final moderation = FakeModeration();
      await _pump(tester, moderation, const SocialReportTarget.post('p1'));
      await _choose(tester, '垃圾訊息');
      await _submit(tester);
      await tester.pump(const Duration(seconds: 3));

      await tester.tap(find.text('開啟檢舉'));
      await tester.pumpAndSettle();
      await _choose(tester, '垃圾訊息');
      await _submit(tester);

      expect(moderation.submitted, hasLength(2));
      expect(
        moderation.submitted[1].requestId,
        isNot(moderation.submitted[0].requestId),
      );
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('「其他」要填說明才能送出，說明去除首尾空白', (tester) async {
      final moderation = FakeModeration();
      await _pump(tester, moderation, const SocialReportTarget.post('p1'));

      await _choose(tester, '其他（請說明）');
      final submit = find.byKey(const ValueKey('report-submit'));
      expect(tester.widget<ElevatedButton>(submit).onPressed, isNull);
      await tester.enterText(find.byType(TextField), '  亂發廣告  ');
      await tester.pump();
      await _submit(tester);

      final request = moderation.submitted.single;
      expect(request.category, PolicyCategory.POLICY_CATEGORY_OTHER);
      expect(request.description, '亂發廣告');
      await tester.pump(const Duration(seconds: 3));
    });
  });

  group('社群檢舉的送出細節', () {
    testWidgets('切離「其他」之後不送說明', (tester) async {
      final moderation = FakeModeration();
      await _pump(tester, moderation, const SocialReportTarget.post('p1'));

      await _choose(tester, '其他（請說明）');
      await tester.enterText(find.byType(TextField), '亂發廣告');
      await tester.pump();
      await _choose(tester, '垃圾訊息');
      await _submit(tester);

      final request = moderation.submitted.single;
      expect(request.category, PolicyCategory.POLICY_CATEGORY_SPAM);
      expect(request.description, '');
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('說明可寫到 500 字（契約上限），不是 200', (tester) async {
      final moderation = FakeModeration();
      await _pump(tester, moderation, const SocialReportTarget.post('p1'));

      await _choose(tester, '其他（請說明）');
      await tester.enterText(find.byType(TextField), '字' * 600);
      await tester.pump();
      await _submit(tester);

      expect(moderation.submitted.single.description, '字' * 500);
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('說明的計數與上限以 rune 計，emoji ZWJ 序列不被拆開', (tester) async {
      final moderation = FakeModeration();
      await _pump(tester, moderation, const SocialReportTarget.post('p1'));
      const family = '\u{1F468}\u200D\u{1F469}\u200D\u{1F467}';

      await _choose(tester, '其他（請說明）');
      await tester.enterText(find.byType(TextField), '字' * 3 + family);
      await tester.pump();
      expect(find.text('8/500'), findsOneWidget);

      await tester.enterText(find.byType(TextField), '字' * 497 + family);
      await tester.pump();
      expect(find.text('497/500'), findsOneWidget);
      await _submit(tester);
      expect(moderation.submitted.single.description, '字' * 497);
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('伺服器帶 LocalizedMessage 的失敗：顯示伺服器的文字', (tester) async {
      final moderation = FakeModeration(
        submitError: ConnectException(
          Code.resourceExhausted,
          'quota',
          details: [
            ErrorDetail(
              'google.rpc.LocalizedMessage',
              (LocalizedMessage()
                    ..locale = 'zh-TW'
                    ..message = '今天的檢舉次數已達上限')
                  .writeToBuffer(),
            ),
          ],
        ),
      );
      await _pump(tester, moderation, const SocialReportTarget.post('p1'));

      await _choose(tester, '垃圾訊息');
      await _submit(tester);

      expect(find.text('今天的檢舉次數已達上限'), findsOneWidget);
      expect(find.text('服務暫時不可用'), findsNothing);
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('沒有伺服器文字的失敗（連線問題）：用通用句', (tester) async {
      final moderation = FakeModeration(
        submitError: ConnectException(Code.unavailable, 'offline'),
      );
      await _pump(tester, moderation, const SocialReportTarget.post('p1'));

      await _choose(tester, '垃圾訊息');
      await _submit(tester);

      expect(find.text('服務暫時不可用'), findsOneWidget);
      await tester.pump(const Duration(seconds: 3));
    });
  });

  group('聊天仍走舊 REST', () {
    testWidgets('聊天訊息：POST /app/reports，不呼叫 ModerationAppService', (
      tester,
    ) async {
      final moderation = FakeModeration();
      final legacy = await _pump(
        tester,
        moderation,
        const LegacyReportTarget(ReportTargetType.chatMessage, 'm1'),
      );

      await _choose(tester, ReportReasonCode.harassment.label);
      await _submit(tester);

      final request = legacy.requests.single;
      expect(request.method, 'POST');
      expect(request.path, '/app/reports');
      expect(request.data, {
        'targetType': 'CHAT_MESSAGE',
        'targetId': 'm1',
        'reasonCode': 'HARASSMENT',
      });
      expect(moderation.submitted, isEmpty);
      await tester.pump(const Duration(seconds: 3));
    });
  });
}
