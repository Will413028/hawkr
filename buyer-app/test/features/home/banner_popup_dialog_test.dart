import 'package:hawkr_api/images.dart' as api_image;
import 'package:hawkr_buyer/features/home/presentation/home_widget/banner_popup_dialog.dart';
import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const longCta = '查看本週產地直送限時優惠活動完整內容';

  for (final size in <Size>[const Size(320, 568), const Size(568, 320)]) {
    testWidgets(
      'popup stays reachable at ${size.width.toInt()}x${size.height.toInt()} '
      'with large text',
      (tester) async {
        final semantics = tester.ensureSemantics();
        tester.view.devicePixelRatio = 1;
        tester.view.physicalSize = size;
        addTearDown(tester.view.resetDevicePixelRatio);
        addTearDown(tester.view.resetPhysicalSize);

        await tester.pumpWidget(
          MaterialApp(
            builder: (context, child) => MediaQuery(
              data: MediaQuery.of(context).copyWith(
                padding: const EdgeInsets.only(top: 24, bottom: 16),
                textScaler: const TextScaler.linear(1.4),
              ),
              child: child!,
            ),
            home: Builder(
              builder: (context) => Scaffold(
                body: Center(
                  child: FilledButton(
                    onPressed: () => showBannerPopupDialog(
                      context,
                      popupImage: api_image.Image(),
                      action: const BannerAction(
                        type: 'url',
                        url: 'https://example.test/activity',
                        buttonText: longCta,
                      ),
                    ),
                    child: const Text('開啟活動'),
                  ),
                ),
              ),
            ),
          ),
        );

        await tester.tap(find.text('開啟活動'));
        await tester.pumpAndSettle();

        expect(tester.takeException(), isNull);
        expect(
          find.byKey(const ValueKey('banner-popup-scroll')),
          findsOneWidget,
        );
        final scrollRect = tester.getRect(
          find.byKey(const ValueKey('banner-popup-scroll')),
        );
        expect(scrollRect.top, greaterThanOrEqualTo(24));
        expect(scrollRect.bottom, lessThanOrEqualTo(size.height - 16));
        final imageRect = tester.getRect(
          find.byKey(const ValueKey('banner-popup-image')),
        );
        expect(imageRect.left, greaterThanOrEqualTo(0));
        expect(imageRect.right, lessThanOrEqualTo(size.width));
        expect(imageRect.height / imageRect.width, closeTo(404 / 360, 0.01));

        final cta = find.byKey(const ValueKey('banner-popup-cta'));
        expect(tester.getSize(cta).height, 48);
        final label = tester.widget<Text>(
          find.descendant(of: cta, matching: find.text(longCta)),
        );
        expect(label.maxLines, 1);
        expect(label.softWrap, isFalse);
        expect(label.overflow, TextOverflow.ellipsis);
        expect(
          find.descendant(of: cta, matching: find.byType(HkButtonLabel)),
          findsOneWidget,
        );
        expect(find.bySemanticsLabel(longCta), findsOneWidget);

        final close = find.byKey(const ValueKey('banner-popup-close'));
        expect(tester.getSize(close), const Size.square(48));

        final scrollable = tester.state<ScrollableState>(
          find.descendant(
            of: find.byKey(const ValueKey('banner-popup-scroll')),
            matching: find.byType(Scrollable),
          ),
        );
        if (size.width > size.height) {
          expect(scrollable.position.maxScrollExtent, greaterThan(0));
          await tester.ensureVisible(cta);
          await tester.pumpAndSettle();
          expect(cta, findsOneWidget);
        } else {
          await tester.tap(close);
          await tester.pumpAndSettle();
          expect(cta, findsNothing);
        }

        expect(tester.takeException(), isNull);
        semantics.dispose();
      },
    );
  }
}
