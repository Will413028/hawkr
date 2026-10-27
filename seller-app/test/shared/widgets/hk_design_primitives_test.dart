import 'package:hawkr_seller/shared/widgets/hk_adaptive_page.dart';
import 'package:hawkr_seller/shared/widgets/hk_content_card.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_chrome_surface.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:hawkr_seller/shared/widgets/hk_page_section.dart';
import 'package:hawkr_seller/shared/widgets/hk_status_badge.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('seller design primitives fit a 320px screen', (tester) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(320, 720);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: HkAdaptivePage(
            child: ListView(
              children: [
                HkPageSection(
                  title: '營運狀態',
                  description: '長文字也必須自然換行，不可以壓縮字級。',
                  child: HkContentCard(
                    child: Row(
                      children: [
                        const HkIcon(HkIconName.warning),
                        const SizedBox(width: 12),
                        const Expanded(child: Text('有一筆訂單需要在今天完成出貨')),
                        const SizedBox(width: 8),
                        HkStatusBadge(
                          label: '待處理',
                          tone: HkStatusTone.warning,
                          icon: HkIconName.clock,
                        ),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );

    expect(find.text('營運狀態'), findsOneWidget);
    expect(find.text('待處理'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('interactive card exposes a labeled accessible target', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    var tapped = false;

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Center(
            child: SizedBox(
              width: 240,
              height: 64,
              child: HkContentCard(
                semanticLabel: '查看訂單',
                onTap: () => tapped = true,
                child: const Text('訂單內容'),
              ),
            ),
          ),
        ),
      ),
    );

    expect(find.bySemanticsLabel(RegExp('查看訂單')), findsOneWidget);
    expect(
      tester.getSemantics(find.bySemanticsLabel(RegExp('查看訂單'))),
      isSemantics(label: '查看訂單\n訂單內容', isButton: true, hasTapAction: true),
    );
    await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
    await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));

    await tester.tap(find.text('訂單內容'));
    expect(tapped, isTrue);
    semantics.dispose();
  });

  testWidgets('compact glass chrome shrink-wraps in a positioned overlay', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Stack(
            children: [
              Positioned(
                right: 16,
                bottom: 16,
                child: HkGlassChromeSurface(
                  surfaceKey: const ValueKey('compact-glass-surface'),
                  height: 48,
                  borderRadius: BorderRadius.circular(24),
                  showBottomBorder: false,
                  child: TextButton.icon(
                    onPressed: () {},
                    icon: const HkIcon(HkIconName.photoAdd),
                    label: const Text('更換封面'),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );

    final size = tester.getSize(
      find.byKey(const ValueKey('compact-glass-surface')),
    );
    expect(size.width, greaterThan(0));
    expect(size.width, lessThan(400));
    expect(size.height, 48);
    expect(tester.takeException(), isNull);

    semantics.dispose();
  });
}
