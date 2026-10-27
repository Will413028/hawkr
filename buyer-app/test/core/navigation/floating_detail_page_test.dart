import 'dart:io';

import 'package:hawkr_buyer/core/routing/floating_detail_page.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

void main() {
  // App 只發 iOS：每個平台（含測試主機預設的 android）都走 Cupertino drill-down。
  for (final platform in [
    TargetPlatform.android,
    TargetPlatform.iOS,
    TargetPlatform.macOS,
  ]) {
    testWidgets(
      '$platform floating detail uses Cupertino drill-down and edge swipe',
      (tester) async {
        tester.view.devicePixelRatio = 1;
        tester.view.physicalSize = const Size(390, 844);
        addTearDown(tester.view.resetDevicePixelRatio);
        addTearDown(tester.view.resetPhysicalSize);

        var originalDisposed = false;
        late PageRoute<dynamic> detailRoute;
        final router = _detailRouter(
          onOriginalDispose: () => originalDisposed = true,
          onDetailBuild: (context) {
            detailRoute = ModalRoute.of(context)! as PageRoute<dynamic>;
          },
        );
        addTearDown(router.dispose);

        await tester.pumpWidget(
          MaterialApp.router(
            theme: ThemeData(platform: platform),
            routerConfig: router,
          ),
        );
        await tester.pumpAndSettle();

        final result = router.push<void>('/detail');
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 1));

        expect(find.byType(CupertinoPageTransition), findsWidgets);
        expect(find.byType(CupertinoFullscreenDialogTransition), findsNothing);

        await tester.pumpAndSettle();
        expect(detailRoute.fullscreenDialog, isFalse);
        expect(detailRoute.opaque, isFalse);
        expect(detailRoute.maintainState, isTrue);
        expect(detailRoute.barrierColor, AppColors.bgOverlay);
        expect(
          detailRoute.transitionDuration,
          CupertinoRouteTransitionMixin.kTransitionDuration,
        );
        expect(detailRoute.popGestureEnabled, isTrue);
        expect(find.byKey(const ValueKey('original-page')), findsOneWidget);
        expect(find.byKey(const ValueKey('floating-detail')), findsOneWidget);
        expect(originalDisposed, isFalse);

        final gesture = await tester.startGesture(const Offset(5, 300));
        await gesture.moveBy(const Offset(390, 0));
        await tester.pump();
        expect(find.byKey(const ValueKey('original-page')), findsOneWidget);
        expect(find.byKey(const ValueKey('floating-detail')), findsOneWidget);
        expect(originalDisposed, isFalse);
        await gesture.up();
        await tester.pumpAndSettle();
        await result;

        expect(find.byKey(const ValueKey('floating-detail')), findsNothing);
        expect(find.byKey(const ValueKey('original-page')), findsOneWidget);
        expect(originalDisposed, isFalse);
      },
    );
  }

  for (final platform in [TargetPlatform.android, TargetPlatform.iOS]) {
    testWidgets('$platform Reduce Motion makes detail push/pop immediate', (
      tester,
    ) async {
      late PageRoute<dynamic> detailRoute;
      final router = _detailRouter(
        onDetailBuild: (context) {
          detailRoute = ModalRoute.of(context)! as PageRoute<dynamic>;
        },
      );
      addTearDown(router.dispose);

      await tester.pumpWidget(
        MaterialApp.router(
          theme: ThemeData(platform: platform),
          routerConfig: router,
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(context).copyWith(disableAnimations: true),
            child: child!,
          ),
        ),
      );
      await tester.pumpAndSettle();

      final result = router.push<void>('/detail');
      await tester.pump();
      expect(detailRoute.transitionDuration, Duration.zero);
      expect(detailRoute.reverseTransitionDuration, Duration.zero);
      expect(find.byKey(const ValueKey('floating-detail')), findsOneWidget);
      expect(find.byKey(const ValueKey('original-page')), findsOneWidget);

      router.pop();
      await tester.pump();
      await result;
      expect(find.byKey(const ValueKey('floating-detail')), findsNothing);
      expect(find.byKey(const ValueKey('original-page')), findsOneWidget);
    });
  }

  test('post, product and order detail routes share one overlay page', () {
    final source = File('lib/core/routing/app_router.dart').readAsStringSync();
    expect('floatingDetailPage<'.allMatches(source), hasLength(3));
    for (final path in [
      '/merchant-product',
      '/orders/detail',
      '/social/post/:postId',
    ]) {
      expect(
        source,
        matches(
          RegExp(
            "path: '$path',[\\s\\S]*?pageBuilder:[\\s\\S]*?"
            'floatingDetailPage<',
          ),
        ),
        reason: path,
      );
    }
  });
}

GoRouter _detailRouter({
  VoidCallback? onOriginalDispose,
  ValueChanged<BuildContext>? onDetailBuild,
}) {
  return GoRouter(
    routes: [
      GoRoute(
        path: '/',
        builder: (_, __) => _MountedOriginalPage(onDispose: onOriginalDispose),
      ),
      GoRoute(
        path: '/detail',
        pageBuilder: (context, state) => floatingDetailPage<void>(
          context: context,
          key: state.pageKey,
          child: Builder(
            builder: (context) {
              onDetailBuild?.call(context);
              return const Scaffold(
                key: ValueKey('floating-detail'),
                backgroundColor: Colors.transparent,
                body: Center(child: Text('浮層詳情')),
              );
            },
          ),
        ),
      ),
    ],
  );
}

class _MountedOriginalPage extends StatefulWidget {
  const _MountedOriginalPage({this.onDispose});

  final VoidCallback? onDispose;

  @override
  State<_MountedOriginalPage> createState() => _MountedOriginalPageState();
}

class _MountedOriginalPageState extends State<_MountedOriginalPage> {
  @override
  void dispose() {
    widget.onDispose?.call();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return const Scaffold(
      key: ValueKey('original-page'),
      body: Center(child: Text('原本頁面')),
    );
  }
}
