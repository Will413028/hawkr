import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/media/data/private_document_uploader.dart';
import 'package:hawkr_seller/features/seller/presentation/seller_inquiries_screen.dart';
import 'package:hawkr_seller/features/seller/presentation/seller_inquiry_detail_screen.dart';
import 'package:hawkr_seller/features/seller/presentation/seller_inquiry_reply_screen.dart';

import '../../../support/connect_errors.dart';
import '../../../support/inquiry_fixtures.dart';

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

final _now = DateTime.utc(2026, 11, 22);

class _Server {
  List<Inquiry> inquiries = [];
  Object? listFailure;
  Object? replyFailure;
  Inquiry? replyAnswer;
  final replies = <ReplyToInquiryRequest>[];

  SellerAppServiceClient get client => SellerAppServiceClient(
    FakeTransportBuilder()
        .unary(SellerAppService.listMyInquiries, (_, _) {
          final error = listFailure;
          if (error != null) throw error;
          return ListMyInquiriesResponse(
            inquiries: inquiries,
            pendingReplyCount: inquiries
                .where((i) => i.state == InquiryState.INQUIRY_STATE_OPEN)
                .length,
          );
        })
        .unary(SellerAppService.getMyInquiry, (request, _) {
          final found = inquiries.where((i) => i.id == request.inquiryId);
          if (found.isEmpty) {
            throw reasonError(Code.notFound, 'NOT_FOUND', message: '不存在');
          }
          return GetMyInquiryResponse(inquiry: found.first);
        })
        .unary(SellerAppService.replyToInquiry, (request, _) {
          replies.add(request);
          final error = replyFailure;
          if (error != null) throw error;
          final answer =
              replyAnswer ??
              inquiryOf(
                id: request.inquiryId,
                state: InquiryState.INQUIRY_STATE_REPLIED,
                reply: InquiryReply(
                  body: request.body,
                  channel:
                      InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_SELLER_CENTER,
                  attachmentCount: request.evidenceStagedTokens.length,
                ),
              );
          inquiries = [
            for (final i in inquiries) i.id == answer.id ? answer : i,
          ];
          return ReplyToInquiryResponse(inquiry: answer);
        })
        .build(),
  );
}

Future<void> _pump(
  WidgetTester tester,
  _Server server, {
  String initial = SellerRoutes.inquiries,
  List<String>? uploaded,
}) async {
  tester.view.physicalSize = const Size(800, 2400);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final router = GoRouter(
    initialLocation: initial,
    routes: [
      GoRoute(
        path: '/',
        builder: (_, _) => const Scaffold(body: Text('ROOT')),
      ),
      GoRoute(
        path: MainRoutes.settings,
        builder: (_, _) => const Scaffold(body: Text('SETTINGS')),
      ),
      GoRoute(
        path: SellerRoutes.inquiries,
        builder: (_, _) => SellerInquiriesScreen(clock: () => _now),
      ),
      GoRoute(
        path: SellerRoutes.inquiryDetail,
        builder: (_, state) => SellerInquiryDetailScreen(
          inquiryId: state.pathParameters['inquiryId']!,
          clock: () => _now,
        ),
      ),
      GoRoute(
        path: SellerRoutes.inquiryReply,
        builder: (_, state) => SellerInquiryReplyScreen(
          inquiryId: state.pathParameters['inquiryId']!,
          pickEvidence: (_) async =>
              (bytes: Uint8List.fromList(_png), filename: 'a.jpg'),
        ),
      ),
    ],
  );
  addTearDown(router.dispose);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        sessionProvider.overrideWith(_LoggedIn.new),
        sellerClientProvider.overrideWithValue(server.client),
        privateDocumentUploaderProvider.overrideWithValue((
          bytes,
          docType,
          filename,
        ) async {
          uploaded?.add('$docType:$filename');
          return 'tok-${uploaded?.length ?? 0}';
        }),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, _) => MaterialApp.router(routerConfig: router),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

const _png = <int>[
  0x89,
  0x50,
  0x4E,
  0x47,
  0x0D,
  0x0A,
  0x1A,
  0x0A,
  0x00,
  0x00,
  0x00,
  0x0D,
  0x49,
  0x48,
  0x44,
  0x52,
  0x00,
  0x00,
  0x00,
  0x01,
  0x00,
  0x00,
  0x00,
  0x01,
  0x08,
  0x06,
  0x00,
  0x00,
  0x00,
  0x1F,
  0x15,
  0xC4,
  0x89,
  0x00,
  0x00,
  0x00,
  0x0D,
  0x49,
  0x44,
  0x41,
  0x54,
  0x78,
  0x9C,
  0x63,
  0xF8,
  0xFF,
  0xFF,
  0x3F,
  0x00,
  0x05,
  0xFE,
  0x02,
  0xFE,
  0xA7,
  0x35,
  0x81,
  0x84,
  0x00,
  0x00,
  0x00,
  0x00,
  0x49,
  0x45,
  0x4E,
  0x44,
  0xAE,
  0x42,
  0x60,
  0x82,
];

Future<void> _drainToast(WidgetTester tester) async {
  await tester.pump(const Duration(seconds: 3));
  await tester.pumpAndSettle();
}

void main() {
  group('列表', () {
    testWidgets('待回覆的排在前面；逾期標示「已逾期」並顯示期限', (tester) async {
      final server = _Server()
        ..inquiries = [
          inquiryOf(
            id: 'done',
            message: '已回覆的說明',
            state: InquiryState.INQUIRY_STATE_REPLIED,
          ),
          inquiryOf(
            id: 'late',
            message: '逾期的說明',
            due: DateTime.utc(2026, 11, 21),
          ),
          inquiryOf(id: 'soon', message: '還沒到期的說明'),
        ];
      await _pump(tester, server);

      expect(find.text('已逾期'), findsOneWidget);
      expect(find.text('待回覆'), findsOneWidget);
      expect(find.text('已回覆'), findsOneWidget);
      expect(find.byKey(const ValueKey('inquiry-deadline')), findsNWidgets(2));
      final late = tester.getTopLeft(find.text('逾期的說明')).dy;
      final done = tester.getTopLeft(find.text('已回覆的說明')).dy;
      expect(late, lessThan(done));
    });

    testWidgets('沒有詢問：顯示空狀態', (tester) async {
      await _pump(tester, _Server());
      expect(find.text('目前沒有平台詢問'), findsOneWidget);
    });

    testWidgets('讀取失敗：顯示錯誤面板，不當成沒有詢問', (tester) async {
      final server = _Server()
        ..listFailure = ConnectException(Code.unavailable, 'down');
      await _pump(tester, server);
      expect(find.text('無法載入平台詢問'), findsOneWidget);
      expect(find.text('目前沒有平台詢問'), findsNothing);
    });

    testWidgets('點一則進入詳情', (tester) async {
      final server = _Server()..inquiries = [inquiryOf(message: '詳情用的說明')];
      await _pump(tester, server);
      await tester.tap(find.text('詳情用的說明'));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('inquiry-message')), findsOneWidget);
    });
  });

  group('詳情', () {
    testWidgets('can_reply 為真：有回覆入口', (tester) async {
      final server = _Server()..inquiries = [inquiryOf()];
      await _pump(
        tester,
        server,
        initial: SellerRoutes.inquiryDetailFor('inq-1'),
      );
      expect(
        find.byKey(const ValueKey('inquiry-reply-button')),
        findsOneWidget,
      );
    });

    testWidgets('已回覆：顯示管道、內容與附件數，沒有回覆入口', (tester) async {
      final server = _Server()
        ..inquiries = [
          inquiryOf(
            state: InquiryState.INQUIRY_STATE_REPLIED,
            reply: InquiryReply(
              body: '管理員代記的內容',
              channel: InquiryReplyChannel.INQUIRY_REPLY_CHANNEL_PHONE,
              attachmentCount: 2,
            ),
          ),
        ];
      await _pump(
        tester,
        server,
        initial: SellerRoutes.inquiryDetailFor('inq-1'),
      );
      expect(find.text('管理員代記的內容'), findsOneWidget);
      expect(find.text('附件 2 份'), findsOneWidget);
      expect(find.byKey(const ValueKey('inquiry-reply-meta')), findsOneWidget);
      expect(find.byKey(const ValueKey('inquiry-reply-button')), findsNothing);
    });

    testWidgets('已撤回：唯讀，沒有回覆入口', (tester) async {
      final server = _Server()
        ..inquiries = [inquiryOf(state: InquiryState.INQUIRY_STATE_WITHDRAWN)];
      await _pump(
        tester,
        server,
        initial: SellerRoutes.inquiryDetailFor('inq-1'),
      );
      expect(find.text('已撤回'), findsOneWidget);
      expect(find.byKey(const ValueKey('inquiry-reply-button')), findsNothing);
    });

    testWidgets('not_found：顯示找不到，回到列表', (tester) async {
      await _pump(
        tester,
        _Server(),
        initial: SellerRoutes.inquiryDetailFor('nope'),
      );
      expect(find.text('找不到這則詢問'), findsOneWidget);
      await tester.tap(find.text('回到平台詢問'));
      await tester.pumpAndSettle();
      expect(find.text('目前沒有平台詢問'), findsOneWidget);
    });
  });

  group('回覆表單', () {
    Finder submit() => find.byKey(const ValueKey('inquiry-reply-submit'));
    bool canSubmit(WidgetTester tester) =>
        tester.widget<FilledButton>(submit()).onPressed != null;

    testWidgets('沒有內容不能送出；有內容才能', (tester) async {
      final server = _Server()..inquiries = [inquiryOf()];
      await _pump(
        tester,
        server,
        initial: SellerRoutes.inquiryReplyFor('inq-1'),
      );
      expect(canSubmit(tester), isFalse);
      await tester.enterText(
        find.byKey(const ValueKey('inquiry-reply-input')),
        '  ',
      );
      await tester.pump();
      expect(canSubmit(tester), isFalse);
      await tester.enterText(
        find.byKey(const ValueKey('inquiry-reply-input')),
        '好',
      );
      await tester.pump();
      expect(canSubmit(tester), isTrue);
    });

    testWidgets('超過 4000 字不能送出', (tester) async {
      final server = _Server()..inquiries = [inquiryOf()];
      await _pump(
        tester,
        server,
        initial: SellerRoutes.inquiryReplyFor('inq-1'),
      );
      await tester.enterText(
        find.byKey(const ValueKey('inquiry-reply-input')),
        '字' * 4001,
      );
      await tester.pump();
      expect(canSubmit(tester), isFalse);
      expect(find.text('回覆內容最多 4000 字'), findsOneWidget);
    });

    testWidgets('can_reply 為假：不顯示表單', (tester) async {
      final server = _Server()
        ..inquiries = [inquiryOf(state: InquiryState.INQUIRY_STATE_WITHDRAWN)];
      await _pump(
        tester,
        server,
        initial: SellerRoutes.inquiryReplyFor('inq-1'),
      );
      expect(find.byKey(const ValueKey('inquiry-reply-input')), findsNothing);
      expect(find.text('這則詢問目前不能回覆'), findsOneWidget);
    });

    testWidgets('附件上傳後與內容一起送出；成功後回到詳情並顯示已回覆', (tester) async {
      final server = _Server()..inquiries = [inquiryOf()];
      final uploaded = <String>[];
      await _pump(
        tester,
        server,
        initial: SellerRoutes.inquiryDetailFor('inq-1'),
        uploaded: uploaded,
      );
      await tester.tap(find.byKey(const ValueKey('inquiry-reply-button')));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(const ValueKey('inquiry-reply-input')),
        '  這是回覆  ',
      );
      await tester.tap(find.bySemanticsLabel('新增照片'));
      await tester.pumpAndSettle();
      expect(uploaded, ['inquiry_evidence:a.jpg']);
      await tester.tap(submit());
      await tester.pumpAndSettle();

      expect(server.replies.single.body, '這是回覆');
      expect(server.replies.single.evidenceStagedTokens, ['tok-1']);
      expect(find.text('這是回覆'), findsOneWidget);
      expect(find.text('附件 1 份'), findsOneWidget);
      expect(find.byKey(const ValueKey('inquiry-reply-button')), findsNothing);
      await _drainToast(tester);
    });

    testWidgets('reply_attachment_max 為 0：沒有附件區', (tester) async {
      final server = _Server()..inquiries = [inquiryOf(attachmentMax: 0)];
      await _pump(
        tester,
        server,
        initial: SellerRoutes.inquiryReplyFor('inq-1'),
      );
      expect(find.bySemanticsLabel('新增照片'), findsNothing);
    });

    testWidgets('已被回覆（failed_precondition）：顯示伺服器的句子並離開表單', (tester) async {
      final server = _Server()
        ..inquiries = [inquiryOf()]
        ..replyFailure = reasonError(
          Code.failedPrecondition,
          'INQUIRY_NOT_OPEN',
          message: '這則詢問已經回覆',
        );
      await _pump(
        tester,
        server,
        initial: SellerRoutes.inquiryDetailFor('inq-1'),
      );
      await tester.tap(find.byKey(const ValueKey('inquiry-reply-button')));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(const ValueKey('inquiry-reply-input')),
        '好',
      );
      await tester.pump();
      await tester.tap(submit());
      await tester.pumpAndSettle();
      expect(find.text('這則詢問已經回覆'), findsOneWidget);
      expect(find.byKey(const ValueKey('inquiry-reply-input')), findsNothing);
      await _drainToast(tester);
    });

    testWidgets('欄位錯誤顯示在內容欄位並保留輸入', (tester) async {
      final server = _Server()
        ..inquiries = [inquiryOf()]
        ..replyFailure = fieldError('body', '內容不符規定');
      await _pump(
        tester,
        server,
        initial: SellerRoutes.inquiryReplyFor('inq-1'),
      );
      await tester.enterText(
        find.byKey(const ValueKey('inquiry-reply-input')),
        '好',
      );
      await tester.pump();
      await tester.tap(submit());
      await tester.pumpAndSettle();
      expect(find.text('內容不符規定'), findsOneWidget);
      expect(find.text('好'), findsOneWidget);
    });
  });
}
