import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/widgets/magnetic_tab_bar.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('wallet tab icons are filled only for the selected tab', (
    tester,
  ) async {
    await tester.pumpWidget(const MaterialApp(home: _WalletTabHarness()));
    await tester.pumpAndSettle();

    final memberIcon = find.descendant(
      of: find.byKey(const ValueKey('test-wallet-member-icon')),
      matching: find.byType(Icon),
    );
    final couponIcon = find.descendant(
      of: find.byKey(const ValueKey('test-wallet-coupon-icon')),
      matching: find.byType(Icon),
    );
    expect(tester.widget<Icon>(memberIcon).icon, AppIcons.cardMembership);
    expect(
      tester.widget<Icon>(couponIcon).icon,
      AppIcons.confirmationNumberOutlined,
    );

    await tester.tap(find.text('優惠券'));
    await tester.pumpAndSettle();

    expect(
      tester.widget<Icon>(memberIcon).icon,
      AppIcons.cardMembershipOutlined,
    );
    expect(tester.widget<Icon>(couponIcon).icon, AppIcons.confirmationNumber);
  });
}

class _WalletTabHarness extends StatefulWidget {
  const _WalletTabHarness();

  @override
  State<_WalletTabHarness> createState() => _WalletTabHarnessState();
}

class _WalletTabHarnessState extends State<_WalletTabHarness>
    with SingleTickerProviderStateMixin {
  late final TabController _controller = TabController(length: 2, vsync: this);

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Align(
        alignment: Alignment.topCenter,
        child: SizedBox(
          width: 390,
          child: MagneticTabBar(
            controller: _controller,
            labels: const ['會員卡', '優惠券'],
            iconBuilders: [
              (selected) => HkIcon(
                HkIconName.walletMember,
                key: const ValueKey('test-wallet-member-icon'),
                selected: selected,
                size: 24,
              ),
              (selected) => HkIcon(
                HkIconName.walletCoupon,
                key: const ValueKey('test-wallet-coupon-icon'),
                selected: selected,
                size: 24,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
