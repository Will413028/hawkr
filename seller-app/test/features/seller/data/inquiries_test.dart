import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/seller/data/inquiries.dart';

import '../../../support/connect_errors.dart';
import '../../../support/inquiry_fixtures.dart';

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

String _defaults(Code code) => 'default-${code.name}';

class _Server {
  final pages = <String, ListMyInquiriesResponse>{};
  final listRequests = <ListMyInquiriesRequest>[];
  final gets = <String>[];
  final replies = <ReplyToInquiryRequest>[];
  Object? replyFailure;
  Object? getFailure;
  Inquiry? getAnswer;

  SellerAppServiceClient get client => SellerAppServiceClient(
    FakeTransportBuilder()
        .unary(SellerAppService.listMyInquiries, (request, _) {
          listRequests.add(request);
          return pages[request.pageToken] ?? ListMyInquiriesResponse();
        })
        .unary(SellerAppService.getMyInquiry, (request, _) {
          gets.add(request.inquiryId);
          final error = getFailure;
          if (error != null) throw error;
          return GetMyInquiryResponse(
            inquiry: getAnswer ?? inquiryOf(id: request.inquiryId),
          );
        })
        .unary(SellerAppService.replyToInquiry, (request, _) {
          replies.add(request);
          final error = replyFailure;
          if (error != null) throw error;
          return ReplyToInquiryResponse(
            inquiry: inquiryOf(
              id: request.inquiryId,
              state: InquiryState.INQUIRY_STATE_REPLIED,
            ),
          );
        })
        .build(),
  );
}

ProviderContainer _container(_Server server) {
  final container = ProviderContainer(
    overrides: [
      sessionProvider.overrideWith(_LoggedIn.new),
      sellerClientProvider.overrideWithValue(server.client),
    ],
  );
  addTearDown(container.dispose);
  return container;
}

void main() {
  group('fetchMyInquiries', () {
    test('讀完所有分頁', () async {
      final server = _Server()
        ..pages[''] = ListMyInquiriesResponse(
          inquiries: [inquiryOf(id: 'a')],
          nextPageToken: 't1',
        )
        ..pages['t1'] = ListMyInquiriesResponse(
          inquiries: [inquiryOf(id: 'b')],
        );
      final result = await fetchMyInquiries(server.client);
      expect([for (final i in result) i.id], ['a', 'b']);
      expect(server.listRequests.map((r) => r.pageSize), [
        inquiriesPageSize,
        inquiriesPageSize,
      ]);
    });

    test('next_page_token 重複出現時丟 StateError', () async {
      final server = _Server()
        ..pages[''] = ListMyInquiriesResponse(nextPageToken: 't1')
        ..pages['t1'] = ListMyInquiriesResponse(nextPageToken: 't1');
      await expectLater(fetchMyInquiries(server.client), throwsStateError);
    });
  });

  group('providers', () {
    test('待回覆數取伺服器的 pending_reply_count，只問一筆', () async {
      final server = _Server()
        ..pages[''] = ListMyInquiriesResponse(pendingReplyCount: 3);
      final container = _container(server);
      expect(
        await container.read(pendingInquiryCountProvider('acc-1').future),
        3,
      );
      expect(server.listRequests.single.pageSize, 1);
    });

    test('currentPendingInquiryCount：讀取中是 0，讀到後是伺服器的數字', () async {
      final server = _Server()
        ..pages[''] = ListMyInquiriesResponse(pendingReplyCount: 2);
      final container = _container(server);
      container.listen(currentPendingInquiryCountProvider, (_, _) {});
      expect(container.read(currentPendingInquiryCountProvider), 0);
      await container.read(pendingInquiryCountProvider('acc-1').future);
      expect(container.read(currentPendingInquiryCountProvider), 2);
    });

    test('inquiryById：列表已讀過就用列表的，不再 GetMyInquiry', () async {
      final server = _Server()
        ..pages[''] = ListMyInquiriesResponse(
          inquiries: [inquiryOf(id: 'a', message: '來自列表')],
        );
      final container = _container(server);
      await container.read(myInquiriesProvider('acc-1').future);
      final inquiry = await container.read(
        inquiryByIdProvider('acc-1', 'a').future,
      );
      expect(inquiry.message, '來自列表');
      expect(server.gets, isEmpty);
    });

    test('inquiryById：列表裡換成最新一筆後跟著更新（canReply 不過期）', () async {
      final server = _Server()
        ..pages[''] = ListMyInquiriesResponse(
          inquiries: [inquiryOf(id: 'a', canReply: true)],
        );
      final container = _container(server);
      await container.read(myInquiriesProvider('acc-1').future);
      final sub = container.listen(
        inquiryByIdProvider('acc-1', 'a'),
        (_, _) {},
      );
      addTearDown(sub.close);
      expect(
        (await container.read(
          inquiryByIdProvider('acc-1', 'a').future,
        )).canReply,
        isTrue,
      );
      container
          .read(myInquiriesProvider('acc-1').notifier)
          .adopt(
            inquiryOf(
              id: 'a',
              state: InquiryState.INQUIRY_STATE_REPLIED,
              canReply: false,
            ),
          );
      final latest = await container.read(
        inquiryByIdProvider('acc-1', 'a').future,
      );
      expect(latest.canReply, isFalse);
      expect(latest.state, InquiryState.INQUIRY_STATE_REPLIED);
    });

    test('inquiryById：沒讀過列表就 GetMyInquiry', () async {
      final server = _Server();
      final container = _container(server);
      await container.read(inquiryByIdProvider('acc-1', 'z').future);
      expect(server.gets, ['z']);
    });

    test('adopt：以 id 取代那一列，沒有那一筆時不動', () async {
      final server = _Server()
        ..pages[''] = ListMyInquiriesResponse(
          inquiries: [
            inquiryOf(id: 'a'),
            inquiryOf(id: 'b'),
          ],
        );
      final container = _container(server);
      await container.read(myInquiriesProvider('acc-1').future);
      final notifier = container.read(myInquiriesProvider('acc-1').notifier);
      notifier.adopt(
        inquiryOf(id: 'a', state: InquiryState.INQUIRY_STATE_REPLIED),
      );
      notifier.adopt(inquiryOf(id: 'zzz'));
      final list = container.read(myInquiriesProvider('acc-1')).value!;
      expect([for (final i in list) i.id], ['a', 'b']);
      expect(list.first.state, InquiryState.INQUIRY_STATE_REPLIED);
    });
  });

  group('isInquiryOverdue', () {
    final due = DateTime.utc(2026, 11, 27, 2);
    final inquiry = inquiryOf(due: due);
    test('過了期限的待回覆是逾期，期限當下與之前不是', () {
      expect(
        isInquiryOverdue(inquiry, due.add(const Duration(seconds: 1))),
        isTrue,
      );
      expect(isInquiryOverdue(inquiry, due), isFalse);
      expect(
        isInquiryOverdue(inquiry, due.subtract(const Duration(days: 1))),
        isFalse,
      );
    });

    test('已回覆與已撤回不算逾期', () {
      final replied = inquiryOf(state: InquiryState.INQUIRY_STATE_REPLIED);
      expect(
        isInquiryOverdue(replied, due.add(const Duration(days: 9))),
        isFalse,
      );
    });
  });

  group('submitInquiryReply', () {
    final inquiry = inquiryOf();

    Future<InquiryReplyResult> submit(
      _Server server, {
      String body = '  內容  ',
    }) => submitInquiryReply(
      server.client,
      inquiry: inquiry,
      body: body,
      evidenceTokens: const ['tok-1'],
      defaults: _defaults,
    );

    test('送出 trim 後的內容與 token', () async {
      final server = _Server();
      final result = await submit(server);
      expect(result, isA<InquiryReplySubmitted>());
      expect(server.replies.single.body, '內容');
      expect(server.replies.single.inquiryId, 'inq-1');
      expect(server.replies.single.evidenceStagedTokens, ['tok-1']);
    });

    test('failed_precondition：重讀最新一筆並帶伺服器的句子', () async {
      final server = _Server()
        ..replyFailure = reasonError(
          Code.failedPrecondition,
          'INQUIRY_NOT_OPEN',
          message: '這則詢問已回覆',
        )
        ..getAnswer = inquiryOf(state: InquiryState.INQUIRY_STATE_REPLIED);
      final result = await submit(server);
      expect(result, isA<InquiryReplyClosed>());
      final closed = result as InquiryReplyClosed;
      expect(closed.message, '這則詢問已回覆');
      expect(closed.latest?.state, InquiryState.INQUIRY_STATE_REPLIED);
    });

    test('failed_precondition 之後重讀失敗：latest 是 null', () async {
      final server = _Server()
        ..replyFailure = reasonError(Code.failedPrecondition, 'X', message: 'm')
        ..getFailure = ConnectException(Code.unavailable, 'down');
      final closed = await submit(server) as InquiryReplyClosed;
      expect(closed.latest, isNull);
    });

    test('not_found', () async {
      final server = _Server()
        ..replyFailure = reasonError(Code.notFound, 'X', message: '找不到');
      expect(await submit(server), isA<InquiryReplyNotFound>());
    });

    test('body 欄位錯誤', () async {
      final server = _Server()..replyFailure = fieldError('body', '回覆不能是空的');
      final result = await submit(server);
      expect((result as InquiryReplyBodyInvalid).message, '回覆不能是空的');
    });

    test('附件欄位錯誤：索引與整個欄位', () async {
      final server = _Server()
        ..replyFailure = fieldErrors({
          'evidence_staged_tokens[1]': '檔案已過期',
          'evidence_staged_tokens': '附件太多',
        });
      final result = await submit(server) as InquiryReplyEvidenceInvalid;
      expect(result.byIndex, {1: '檔案已過期'});
      expect(result.evidenceMessage, '附件太多');
    });

    test('其他錯誤：顯示伺服器的句子', () async {
      final server = _Server()
        ..replyFailure = reasonError(Code.unavailable, 'X', message: '稍後再試');
      final result = await submit(server);
      expect((result as InquiryReplyFailed).message, '稍後再試');
    });
  });
}
