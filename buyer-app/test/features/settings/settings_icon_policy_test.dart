import 'dart:io';
import 'dart:ui' show CheckedState, Tristate;

import 'package:hawkr_buyer/core/layout/app_media_query_policy.dart';
import 'package:hawkr_buyer/core/layout/app_text_size.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/features/settings/presentation/settings_screen.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:flutter/foundation.dart'
    show debugDefaultTargetPlatformOverride;
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  test(
    'settings screen uses semantic Material icons instead of legacy PNGs',
    () {
      final screen = File(
        'lib/features/settings/presentation/settings_screen.dart',
      ).readAsStringSync();
      final registry = File(
        'lib/shared/widgets/hk_icon.dart',
      ).readAsStringSync();

      for (final icon in <String>[
        'edit',
        'user',
        'location',
        'textSize',
        'afterSales',
        'notification',
        'help',
        'chevronRight',
      ]) {
        expect(screen, contains('HkIconName.$icon'));
      }

      for (final icon in <String>[
        'settingsFollowing',
        'settingsFavorite',
        'settingsViewed',
      ]) {
        expect(screen, contains('HkIconName.$icon'));
        expect(registry, contains('HkIconName.$icon =>'));
      }

      expect(screen, isNot(contains('assets/setting_icon')));
      expect(screen, isNot(contains('Image.asset(')));

      expect(registry, contains('HkIconName.afterSales => AppIcons.'));
      expect(registry, contains('HkIconName.help => AppIcons.'));
    },
  );

  for (final scenario in <({double width, double textScale})>[
    (width: 320, textScale: 1),
    (width: 320, textScale: 1.15),
    (width: 320, textScale: 1.4),
    (width: 390, textScale: 1.4),
    (width: 390, textScale: 2),
    (width: 768, textScale: 1.4),
  ]) {
    testWidgets(
      'Material icon settings layout fits ${scenario.width.toInt()}px at '
      '${scenario.textScale}x text',
      (tester) async {
        tester.view.devicePixelRatio = 1;
        tester.view.physicalSize = Size(scenario.width, 844);
        addTearDown(tester.view.resetDevicePixelRatio);
        addTearDown(tester.view.resetPhysicalSize);

        await tester.pumpWidget(
          ProviderScope(
            overrides: [sessionProvider.overrideWith(_GuestSession.new)],
            child: ScreenUtilInit(
              designSize: const Size(393, 852),
              enableScaleWH: () => false,
              enableScaleText: () => false,
              builder: (context, child) => MaterialApp(
                builder: (context, child) => MediaQuery(
                  data: AppMediaQueryPolicy.apply(
                    MediaQuery.of(context).copyWith(
                      textScaler: TextScaler.linear(scenario.textScale),
                    ),
                  ),
                  child: child!,
                ),
                home: const SettingsScreen(showTopBar: false),
              ),
            ),
          ),
        );
        await tester.pump();

        final settingsContext = tester.element(find.byType(SettingsScreen));
        expect(
          MediaQuery.textScalerOf(settingsContext).scale(10),
          closeTo(10 * scenario.textScale.clamp(0, 1.4), 0.0001),
        );
        expect(tester.takeException(), isNull);
        for (final label in <String>[
          '追蹤中',
          '收藏',
          '瀏覽過',
          '個人資料設定',
          '收貨地址管理',
          '字體大小',
          '售後服務',
          '通知設定',
          '幫助中心',
        ]) {
          expect(find.text(label), findsOneWidget);
        }
        expect(find.byType(HkIcon), findsWidgets);
        const statsNames = <HkIconName>[
          HkIconName.settingsFollowing,
          HkIconName.settingsFavorite,
          HkIconName.settingsViewed,
        ];
        final statsIcons = tester
            .widgetList<HkIcon>(find.byType(HkIcon))
            .where((icon) => statsNames.contains(icon.name))
            .toList();
        expect(statsIcons.map((icon) => icon.name), statsNames);
        for (final icon in statsIcons) {
          expect(icon.size, 30);
          expect(icon.color, AppIconColors.secondary);
        }

        final stage = tester.getRect(
          find.byKey(const ValueKey('settings-avatar-stage')),
        );
        final avatar = tester.getRect(
          find.byKey(const ValueKey('settings-avatar-image')),
        );
        final editTarget = tester.getRect(
          find.byKey(const ValueKey('settings-avatar-edit-target')),
        );
        final editBadge = tester.getRect(
          find.byKey(const ValueKey('settings-avatar-edit-badge')),
        );

        expect(stage.size, const Size.square(100 + HkSpace.sm));
        expect(avatar.size, const Size.square(100));
        expect(editTarget.size, const Size.square(HkControl.minTap));
        expect(editBadge.size, const Size.square(HkSpace.section));
        expect(avatar.center.dx, stage.center.dx);
        expect(editBadge.center - avatar.center, const Offset(30, 34));
        expect(editTarget.left, greaterThanOrEqualTo(stage.left));
        expect(editTarget.top, greaterThanOrEqualTo(stage.top));
        expect(editTarget.right, lessThanOrEqualTo(stage.right));
        expect(editTarget.bottom, lessThanOrEqualTo(stage.bottom));

        final editIcon = tester.widget<HkIcon>(
          find.descendant(
            of: find.byKey(const ValueKey('settings-avatar-edit-badge')),
            matching: find.byType(HkIcon),
          ),
        );
        expect(editIcon.size, HkSpace.lg);
      },
    );
  }

  testWidgets('font size sheet applies and persists the large preference', (
    tester,
  ) async {
    SharedPreferences.setMockInitialValues({});
    final semantics = tester.ensureSemantics();
    debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(320, 568);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    try {
      await tester.pumpWidget(
        ProviderScope(
          overrides: [sessionProvider.overrideWith(_GuestSession.new)],
          child: Consumer(
            builder: (context, ref, _) {
              final preference = ref.watch(appTextSizeProvider);
              return ScreenUtilInit(
                designSize: const Size(393, 852),
                enableScaleWH: () => false,
                enableScaleText: () => false,
                builder: (context, child) => MaterialApp(
                  builder: (context, child) => MediaQuery(
                    data: AppMediaQueryPolicy.apply(
                      MediaQuery.of(context),
                      textSize: preference,
                    ),
                    child: child!,
                  ),
                  home: const SettingsScreen(showTopBar: false),
                ),
              );
            },
          ),
        ),
      );
      await tester.pump();

      await tester.ensureVisible(find.text('字體大小'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('字體大小'));
      await tester.pumpAndSettle();
      expect(
        find.byKey(const ValueKey('settings-text-size-2')),
        findsOneWidget,
      );
      for (final (index, label) in ['小', '中', '大'].indexed) {
        expect(find.text(label), findsWidgets);
        final data = tester
            .getSemantics(find.bySemanticsLabel(label))
            .getSemanticsData();
        expect(
          data.flagsCollection.isSelected,
          index == 1 ? Tristate.isTrue : Tristate.isFalse,
        );
        expect(data.flagsCollection.isChecked, CheckedState.none);
        expect(data.flagsCollection.isInMutuallyExclusiveGroup, isTrue);
        expect(
          tester
              .getSize(find.byKey(ValueKey('settings-text-size-$index')))
              .height,
          48,
        );
      }

      await tester.tap(find.byKey(const ValueKey('settings-text-size-2')));
      await tester.pumpAndSettle();

      final largeData = tester
          .getSemantics(find.bySemanticsLabel('大'))
          .getSemanticsData();
      final standardData = tester
          .getSemantics(find.bySemanticsLabel('中'))
          .getSemanticsData();
      expect(largeData.flagsCollection.isSelected, Tristate.isTrue);
      expect(standardData.flagsCollection.isSelected, Tristate.isFalse);

      final settingsContext = tester.element(find.byType(SettingsScreen));
      expect(MediaQuery.textScalerOf(settingsContext).scale(20), 23);
      final preferences = await SharedPreferences.getInstance();
      expect(
        preferences.getString(AppTextSizeController.storageKey),
        AppTextSize.large.name,
      );
      expect(tester.takeException(), isNull);
    } finally {
      debugDefaultTargetPlatformOverride = null;
      semantics.dispose();
    }
  });
}

class _GuestSession extends Session {
  @override
  Future<SessionModel?> build() async => null;
}
