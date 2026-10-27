import 'dart:io';

import 'package:hawkr_buyer/features/seller/presentation/widgets/merchant_layout.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('merchant page uses same-size semantic icon controls', () {
    final header = File(
      'lib/features/seller/presentation/widgets/merchant_header.dart',
    ).readAsStringSync();

    expect(header, contains('icon: HkIconName.back'));
    for (final icon in <String>['cart', 'chat']) {
      expect(header, contains('icon: HkIconName.$icon'));
    }
    expect(header, contains('iconSize: HkControl.icon'));
    expect(header, contains('dimension: 48'));
    expect(header, isNot(contains('replayId')));
    expect(header, isNot(contains('assets/')));
    expect(header, isNot(contains('Image.asset')));
  });

  test('merchant post empty state and award use Material glyphs', () {
    final home = File(
      'lib/features/seller/presentation/widgets/merchant_home_tab.dart',
    ).readAsStringSync();
    final header = File(
      'lib/features/seller/presentation/widgets/merchant_header.dart',
    ).readAsStringSync();
    final registry = File('lib/shared/widgets/hk_icon.dart').readAsStringSync();

    expect(home, contains('HkIconName.post'));
    expect(home, contains('size: 36'));
    expect(home, isNot(contains('AppIcons.gridViewOutlined')));
    expect(registry, contains('HkIconName.post => AppIcons.'));

    expect(header, contains('semanticIcon: HkIconName.award'));
    expect(header, contains('iconColor: AppIconColors.gold'));
    expect(registry, contains('HkIconName.award => AppIcons.'));
    expect(header, isNot(contains('AppIcons.emojiEventsOutlined')));
  });

  test(
    'merchant storefront has a distinct website-derived surface hierarchy',
    () {
      final colors = File('lib/core/theme/app_colors.dart').readAsStringSync();
      final memberCard = File(
        'lib/features/home/presentation/home_widget/home_member_card_section.dart',
      ).readAsStringSync();
      final merchantCommon = File(
        'lib/features/seller/presentation/widgets/merchant_common.dart',
      ).readAsStringSync();
      final merchantHeader = File(
        'lib/features/seller/presentation/widgets/merchant_header.dart',
      ).readAsStringSync();
      final merchantTabs = File(
        'lib/features/seller/presentation/widgets/merchant_tab_bar.dart',
      ).readAsStringSync();
      final merchantAbout = File(
        'lib/features/seller/presentation/widgets/merchant_about_section.dart',
      ).readAsStringSync();

      expect(colors, contains('memberCardSurface = Color(0xFFE2E2DC)'));
      expect(memberCard, contains('color: AppColors.memberCardSurface'));
      expect(colors, contains('storefrontHeroStart = Color(0xFFF0A38C)'));
      expect(colors, contains('storefrontHeroEnd = Color(0xFFF6C9BC)'));
      expect(merchantCommon, contains('AppColors.storefrontHeroStart'));
      expect(merchantCommon, isNot(contains('AppColors.memberCardSurface')));
      expect(merchantHeader, contains('AppColors.storefrontProfileSurface'));
      expect(merchantTabs, contains('AppColors.storefrontPageSurface'));
      expect(merchantAbout, contains("'商家介紹'"));
      expect(merchantAbout, contains('該商家沒有填寫'));
      expect(merchantAbout, isNot(contains('歡迎到 Hawkr 看更多商品')));
      expect(merchantHeader, contains('if (hasCustomBanner)'));
    },
  );

  test('merchant responsive rules cover compact phones and tablets', () {
    final compact = MerchantLayoutSpec.forSize(const Size(320, 568));
    final phone = MerchantLayoutSpec.forSize(const Size(390, 844));
    final tablet = MerchantLayoutSpec.forSize(const Size(768, 1024));

    expect(compact.postColumnCount(296), 2);
    expect(phone.postColumnCount(358), 3);
    expect(tablet.postColumnCount(720), 4);
    expect(compact.pageHorizontalPadding, 12);
    expect(tablet.pageHorizontalPadding, 24);
    expect(compact.stacksProfileDetails, isTrue);
    expect(phone.stacksProfileDetails, isFalse);
    expect(
      MerchantLayoutSpec.forSize(
        const Size(390, 844),
        textScale: 1.4,
      ).stacksProfileDetails,
      isTrue,
    );
    expect(phone.expandedAppBarHeight, greaterThan(270));
    expect(tablet.expandedHeroHeight, lessThanOrEqualTo(160));
  });

  test('seller about appears once in the persistent header, not every tab', () {
    final header = File(
      'lib/features/seller/presentation/widgets/merchant_header.dart',
    ).readAsStringSync();
    final screen = File(
      'lib/features/seller/presentation/merchant_screen.dart',
    ).readAsStringSync();

    expect(header, contains('MerchantAboutPanel('));
    expect(screen, contains('merchantDescription: _merchantDescription'));

    for (final path in <String>[
      'lib/features/seller/presentation/widgets/merchant_home_tab.dart',
      'lib/features/seller/presentation/widgets/merchant_shop_tab.dart',
      'lib/features/seller/presentation/widgets/merchant_chat_tab.dart',
    ]) {
      final source = File(path).readAsStringSync();
      expect(source, isNot(contains('MerchantAboutSliver(')), reason: path);
    }
  });

  test('guest storefront skips private follow lookup and duplicate stats', () {
    final screen = File(
      'lib/features/seller/presentation/merchant_screen.dart',
    ).readAsStringSync();

    expect(screen, contains('await ref.read(sessionProvider.future)'));
    expect(screen, contains('if (!hasUsableSession(session)) return;'));
    expect(screen, isNot(contains('.getFansCount(')));
    expect(
      RegExp(r"'/public/sellers/\$currentId/stats'").allMatches(screen).length,
      1,
    );
  });
}
