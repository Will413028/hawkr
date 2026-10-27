import 'dart:async';
import 'dart:ui' show CheckedState, Tristate;

import 'package:hawkr_buyer/features/notification/data/notification_model.dart';
import 'package:hawkr_buyer/features/notification/presentation/notification_controller.dart';
import 'package:hawkr_buyer/features/notification/presentation/notifications_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

class _FixedNotificationListController extends NotificationListController {
  @override
  Future<NotificationListState> build() async {
    return NotificationListState(
      unreadCount: 1,
      hasMore: false,
      notifications: [
        NotificationItem(
          id: 101,
          type: 'SYSTEM',
          title: '訂單提醒',
          content: '商品已準備出貨',
          createdAt: DateTime(2020, 7, 26, 8),
        ),
      ],
    );
  }

  @override
  Future<void> silentRefresh() async {}
}

void main() {
  testWidgets(
    'notification actions and delete-mode row meet mobile a11y guidelines',
    (tester) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = const Size(390, 844);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);
      final semantics = tester.ensureSemantics();

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            notificationInboxStreamProvider.overrideWithValue(
              const Stream<Object?>.empty(),
            ),
            notificationListControllerProvider.overrideWith(
              _FixedNotificationListController.new,
            ),
          ],
          child: ScreenUtilInit(
            designSize: const Size(390, 844),
            enableScaleWH: () => false,
            enableScaleText: () => false,
            builder: (context, child) =>
                const MaterialApp(home: NotificationsScreen()),
          ),
        ),
      );
      await tester.pump();
      await tester.pump();

      final markAll = find.byKey(
        const ValueKey('notifications-mark-all-read-action'),
      );
      final deleteMode = find.byKey(
        const ValueKey('notifications-delete-mode-action'),
      );
      expect(markAll, findsOneWidget);
      expect(deleteMode, findsOneWidget);
      for (final target in <Finder>[markAll, deleteMode]) {
        expect(tester.getSize(target).width, greaterThanOrEqualTo(48));
        expect(tester.getSize(target).height, greaterThanOrEqualTo(48));
      }
      expect(
        tester
            .widget<Icon>(
              find.descendant(of: markAll, matching: find.byType(Icon)),
            )
            .size,
        24,
      );
      expect(find.bySemanticsLabel('全部標示為已讀'), findsOneWidget);
      expect(find.bySemanticsLabel('進入刪除模式'), findsOneWidget);

      final normalRow = find.byKey(const ValueKey('notification-row-101'));
      final normalRowSemantics = tester
          .getSemantics(normalRow)
          .getSemanticsData();
      expect(normalRowSemantics.label, '通知：訂單提醒，商品已準備出貨，7/26');
      expect(normalRowSemantics.flagsCollection.isButton, isTrue);
      expect(normalRowSemantics.flagsCollection.isChecked, CheckedState.none);

      final initialDeleteSemantics = tester
          .getSemantics(deleteMode)
          .getSemanticsData();
      expect(
        initialDeleteSemantics.flagsCollection.isToggled,
        Tristate.isFalse,
      );
      await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
      await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
      await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));

      await tester.tap(deleteMode);
      await tester.pump();

      expect(markAll, findsNothing);
      expect(find.bySemanticsLabel('結束刪除模式'), findsOneWidget);
      final row = find.byKey(const ValueKey('notification-row-101'));
      final indicator = find.byKey(
        const ValueKey('notification-selection-indicator-101'),
      );
      expect(row, findsOneWidget);
      expect(tester.getSize(row).height, greaterThanOrEqualTo(48));
      expect(tester.getSize(indicator), const Size.square(22));

      var rowSemantics = tester.getSemantics(row).getSemanticsData();
      expect(rowSemantics.label, '選取通知：訂單提醒，商品已準備出貨');
      expect(rowSemantics.flagsCollection.isChecked, CheckedState.isFalse);
      expect(rowSemantics.flagsCollection.isButton, isFalse);

      await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
      await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
      await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));

      await tester.tap(row);
      await tester.pump();
      rowSemantics = tester.getSemantics(row).getSemanticsData();
      expect(rowSemantics.flagsCollection.isChecked, CheckedState.isTrue);
      await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
      await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
      await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
      semantics.dispose();
    },
  );
}
