import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/report/data/store_block_repository.dart';
import 'package:hawkr_seller/features/report/presentation/block_list_screen.dart';

class _Repo implements StoreBlockRepository {
  int tokenCalls = 0;
  bool failToken = true;
  Completer<BlockedBuyersPage>? hang;

  @override
  Future<BlockedBuyersPage> list({String pageToken = '', int pageSize = 20}) {
    if (pageToken.isEmpty) {
      return Future.value(
        const BlockedBuyersPage(items: [], nextPageToken: 'next'),
      );
    }
    tokenCalls++;
    if (hang != null) return hang!.future;
    if (failToken) return Future.error(Exception('boom'));
    return Future.value(
      BlockedBuyersPage(
        items: [BlockedBuyerItem(buyerId: 'a', deleted: false, name: '小明')],
        nextPageToken: '',
      ),
    );
  }

  @override
  Future<void> unblock(String buyerId) async {}

  @override
  Future<void> blockByComment(String commentId) => throw UnimplementedError();

  @override
  Future<void> blockByForumEntry(String forumEntryId) =>
      throw UnimplementedError();
}

Future<void> _pump(
  WidgetTester tester,
  _Repo repo, {
  bool settle = true,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 844);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [storeBlockRepositoryProvider.overrideWithValue(repo)],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) =>
            MaterialApp(theme: AppTheme.light, home: const BlockListScreen()),
      ),
    ),
  );
  if (settle) {
    await tester.pumpAndSettle();
  } else {
    await tester.pump();
    await tester.pump();
  }
}

void main() {
  testWidgets('failed auto-load on an empty list shows retry, not a spinner', (
    tester,
  ) async {
    final repo = _Repo();
    await _pump(tester, repo);
    expect(repo.tokenCalls, 1);
    expect(find.byType(CircularProgressIndicator), findsNothing);
    expect(find.text('重試'), findsOneWidget);

    repo.failToken = false;
    await tester.tap(find.text('重試'));
    await tester.pumpAndSettle();
    expect(find.text('小明'), findsOneWidget);
    expect(find.text('重試'), findsNothing);
  });

  testWidgets('spinner shows while the auto-load is actually in flight', (
    tester,
  ) async {
    final repo = _Repo()..hang = Completer();
    await _pump(tester, repo, settle: false);
    expect(find.byType(CircularProgressIndicator), findsOneWidget);
    expect(find.text('重試'), findsNothing);
  });
}
