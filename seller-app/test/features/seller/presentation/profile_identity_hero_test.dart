import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/features/post/presentation/widgets/page_header.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  testWidgets('settings and store use the same expanded identity geometry', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(402, 874);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      ProviderScope(
        child: ScreenUtilInit(
          designSize: const Size(393, 852),
          minTextAdapt: true,
          builder: (_, _) => const MaterialApp(
            home: Scaffold(body: _HeaderFixture(isSettings: false)),
          ),
        ),
      ),
    );
    await tester.pump();

    final storeAvatarRect = tester.getRect(
      find.byKey(const ValueKey('store-identity-avatar-frame')),
    );
    final storeIdentityRect = tester.getRect(
      find.byKey(const Key('seller_store_centered_identity')),
    );

    await tester.pumpWidget(
      ProviderScope(
        child: ScreenUtilInit(
          designSize: const Size(393, 852),
          minTextAdapt: true,
          builder: (_, _) => const MaterialApp(
            home: Scaffold(body: _HeaderFixture(isSettings: true)),
          ),
        ),
      ),
    );
    await tester.pump();

    expect(
      tester.getRect(find.byKey(const ValueKey('store-identity-avatar-frame'))),
      storeAvatarRect,
    );
    expect(
      tester.getRect(find.byKey(const Key('seller_store_centered_identity'))),
      storeIdentityRect,
    );
    expect(find.text('設定'), findsOneWidget);
    expect(find.textContaining('ID '), findsNothing);
    expect(find.text('範例測試店'), findsOneWidget);
    expect(find.text('商家介紹'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('settings keeps only its editing actions on the shared header', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(402, 874);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);
    final semantics = tester.ensureSemantics();
    var coverTaps = 0;
    var avatarTaps = 0;

    await tester.pumpWidget(
      ProviderScope(
        child: ScreenUtilInit(
          designSize: const Size(393, 852),
          minTextAdapt: true,
          builder: (_, _) => MaterialApp(
            home: Scaffold(
              body: _HeaderFixture(
                isSettings: true,
                onPickCover: () => coverTaps++,
                onPickAvatar: () => avatarTaps++,
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pump();

    expect(find.bySemanticsLabel('通知中心'), findsOneWidget);
    expect(find.bySemanticsLabel('聊天'), findsOneWidget);
    expect(find.bySemanticsLabel('更換封面'), findsOneWidget);
    expect(find.bySemanticsLabel('更換店舖頭像'), findsOneWidget);
    for (final label in ['通知中心', '聊天', '更換封面']) {
      expect(
        tester.getSize(find.bySemanticsLabel(label)),
        const Size.square(HkControl.minTap),
      );
    }

    await tester.tap(find.bySemanticsLabel('更換封面'));
    await tester.tap(find.bySemanticsLabel('更換店舖頭像'));
    expect(coverTaps, 1);
    expect(avatarTaps, 1);
    semantics.dispose();
  });

  testWidgets(
    'shared centered identity scales safely with accessibility text',
    (tester) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = const Size(320, 720);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);

      await tester.pumpWidget(
        ProviderScope(
          child: ScreenUtilInit(
            designSize: const Size(393, 852),
            minTextAdapt: true,
            builder: (_, _) => MaterialApp(
              builder: (context, child) => MediaQuery(
                data: MediaQuery.of(
                  context,
                ).copyWith(textScaler: const TextScaler.linear(2)),
                child: child!,
              ),
              home: const Scaffold(body: _HeaderFixture(isSettings: true)),
            ),
          ),
        ),
      );
      await tester.pump();

      expect(
        find.byKey(const Key('seller_store_centered_identity')),
        findsOneWidget,
      );
      expect(tester.takeException(), isNull);
    },
  );
}

class _HeaderFixture extends StatelessWidget {
  const _HeaderFixture({
    required this.isSettings,
    this.onPickCover,
    this.onPickAvatar,
  });

  final bool isSettings;
  final VoidCallback? onPickCover;
  final VoidCallback? onPickAvatar;

  @override
  Widget build(BuildContext context) {
    const location = '基隆市信義區';
    const description = '每天提供產地直送的新鮮蔬果。';
    final spec = PageHeaderLayoutSpec.of(context);
    final height = spec.expandedProfileHeightFor(
      hasAward: false,
      displayedLocation: location,
      description: description,
    );
    return SizedBox(
      height: height,
      child: PageHeader(
        sellerName: '範例測試店',
        location: location,
        description: description,
        title: isSettings ? '設定' : null,
        showNotifications: isSettings,
        onNotifications: isSettings ? _noop : null,
        onChat: _noop,
        onPickCover: isSettings ? (onPickCover ?? _noop) : null,
        onPickAvatar: isSettings ? (onPickAvatar ?? _noop) : null,
      ),
    );
  }
}

void _noop() {}
