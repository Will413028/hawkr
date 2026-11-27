import 'dart:async';

import 'package:flutter/material.dart';
import 'package:hawkr_buyer/l10n/l10n.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/features/seller/presentation/widgets/merchant_chat_tab.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../social/social_fakes.dart';

late ProviderContainer _container;

SwitchableSession get _session =>
    _container.read(sessionProvider.notifier) as SwitchableSession;

Future<void> _pump(WidgetTester tester, FakeSocialPublic public) async {
  final container = ProviderContainer(
    overrides: [
      socialPublicClientProvider.overrideWithValue(public.client()),
      socialAppClientProvider.overrideWithValue(FakeSocialApp().client()),
      sessionProvider.overrideWith(SwitchableSession.new),
    ],
  );
  addTearDown(container.dispose);
  _container = container;
  await container.read(sessionProvider.future);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp.router(
          locale: const Locale('zh'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          theme: AppTheme.light,
          routerConfig: GoRouter(
            routes: [
              GoRoute(
                path: '/',
                builder: (_, _) => Scaffold(
                  body: Consumer(
                    builder: (context, ref, _) =>
                        MerchantChatTab(merchantId: '7', merchantName: '南崁王生技'),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    ),
  );
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 100));
  await tester.pump();
}

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  testWidgets('R2-4 換觀看者後重讀失敗：清掉前一位的留言與權限，顯示錯誤與重試', (tester) async {
    var calls = 0;
    var fail = true;
    final public = FakeSocialPublic(
      forumResponder: (_) {
        if (++calls > 1 && fail) {
          throw ConnectException(Code.unavailable, 'down');
        }
        return pb.ListForumEntriesResponse(
          entries: [
            fakeForumEntry(
              'c-1',
              body: '前一位看得到',
              canEdit: true,
              canDelete: true,
            ),
          ],
          viewerCanComment: true,
        );
      },
    );
    await _pump(tester, public);
    expect(find.text('前一位看得到'), findsOneWidget);
    expect(find.byKey(const Key('merchant-comment-input')), findsOneWidget);

    _session.signIn();
    await tester.pumpAndSettle();

    expect(
      find.byKey(const Key('merchant-comments-load-error')),
      findsOneWidget,
    );
    expect(find.text('載入失敗，請稍後再試'), findsOneWidget);
    expect(find.text('前一位看得到'), findsNothing);
    expect(find.byKey(const Key('merchant-comment-input')), findsNothing);

    fail = false;
    await tester.tap(find.byKey(const Key('merchant-comments-load-retry')));
    await tester.pumpAndSettle();

    expect(find.text('前一位看得到'), findsOneWidget);
    expect(find.byKey(const Key('merchant-comment-input')), findsOneWidget);
  });

  testWidgets('換觀看者前發出的讀取晚回來：丟棄，不蓋掉新觀看者的討論區', (tester) async {
    final oldLoad = Completer<pb.ListForumEntriesResponse>();
    var calls = 0;
    final public = FakeSocialPublic(
      forumResponder: (_) {
        if (++calls == 2) return oldLoad.future;
        return pb.ListForumEntriesResponse(
          entries: [fakeForumEntry('n-1', body: '新觀看者的留言')],
          viewerCanComment: true,
        );
      },
    );
    await _pump(tester, public);
    _session.signIn();
    await tester.pump();
    _session.switchTo('account-3002');
    await tester.pumpAndSettle();
    oldLoad.complete(
      pb.ListForumEntriesResponse(
        entries: [fakeForumEntry('o-1', body: '舊觀看者的留言')],
        viewerCanComment: false,
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('舊觀看者的留言'), findsNothing);
    expect(find.text('新觀看者的留言'), findsOneWidget);
    expect(find.byKey(const Key('merchant-comment-input')), findsOneWidget);
  });

  testWidgets('換觀看者前發出的回覆晚回來：丟棄', (tester) async {
    final oldReplies = Completer<pb.ListForumEntriesResponse>();
    var replyCalls = 0;
    final public = FakeSocialPublic(
      forumResponder: (req) {
        if (!req.hasParentId()) {
          return pb.ListForumEntriesResponse(
            entries: [fakeForumEntry('c-1', replyCount: 1)],
            viewerCanComment: true,
          );
        }
        if (++replyCalls == 1) return oldReplies.future;
        return pb.ListForumEntriesResponse();
      },
    );
    await _pump(tester, public);
    expect(replyCalls, 1);

    _session.signIn();
    await tester.pumpAndSettle();
    oldReplies.complete(
      pb.ListForumEntriesResponse(
        entries: [fakeForumEntry('r-1', body: '舊的回覆')],
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('舊的回覆'), findsNothing);
  });
}
