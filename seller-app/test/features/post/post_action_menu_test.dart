import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/presentation/widgets/post_card.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_frosted_icon_button.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets(
    'bottom-edge menu stays visible, supports back/cancel and selects once',
    (tester) async {
      tester.view.physicalSize = const Size(320, 720);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      var edits = 0;
      var deletes = 0;
      await tester.pumpWidget(
        ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, _) => MaterialApp(
            theme: AppTheme.light,
            builder: (context, child) => MediaQuery(
              data: MediaQuery.of(
                context,
              ).copyWith(textScaler: const TextScaler.linear(1.4)),
              child: child!,
            ),
            home: Scaffold(
              body: Align(
                alignment: Alignment.bottomRight,
                child: Builder(
                  builder: (context) => HkFrostedIconButton(
                    label: '更多操作',
                    icon: AppIcons.moreHoriz,
                    onPressed: () => showPostActionMenu(
                      context,
                      onEdit: () => edits++,
                      onDelete: () => deletes++,
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      );
      Future<void> open() async {
        await tester.tap(find.byTooltip('更多操作'));
        await tester.pumpAndSettle();
        for (final label in ['編輯貼文', '刪除貼文']) {
          final item = find.widgetWithText(PopupMenuItem<String>, label);
          final rect = tester.getRect(item);
          expect(rect.top, greaterThanOrEqualTo(0));
          expect(rect.bottom, lessThanOrEqualTo(720));
          expect(rect.left, greaterThanOrEqualTo(0));
          expect(rect.right, lessThanOrEqualTo(320));
          expect(rect.height, greaterThanOrEqualTo(48));
        }
        expect(tester.takeException(), isNull);
      }

      await open();
      await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();
      expect(find.byType(PopupMenuItem<String>), findsNothing);
      await open();
      await tester.tapAt(const Offset(10, 50));
      await tester.pumpAndSettle();
      expect(edits + deletes, 0);
      await open();
      await tester.tap(find.text('編輯貼文'));
      await tester.pumpAndSettle();
      expect(edits, 1);
      expect(deletes, 0);
      await open();
      await tester.tap(find.text('刪除貼文'));
      await tester.pumpAndSettle();
      expect(edits, 1);
      expect(deletes, 1);
      await open();
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pumpAndSettle();
      expect(find.byType(PopupMenuItem<String>), findsNothing);
      expect(edits + deletes, 2);
      expect(tester.takeException(), isNull);
    },
  );
}
