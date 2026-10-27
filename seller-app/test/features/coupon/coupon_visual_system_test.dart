import 'package:hawkr_seller/features/coupon/presentation/widgets/date_picker_card.dart';
import 'package:hawkr_seller/features/coupon/presentation/widgets/form_card.dart';
import 'package:hawkr_seller/features/coupon/presentation/widgets/preview_card.dart';
import 'package:hawkr_seller/features/coupon/presentation/widgets/section_card.dart';
import 'package:hawkr_seller/features/coupon/presentation/widgets/submit_button.dart';
import 'package:hawkr_seller/features/coupon/presentation/widgets/target_user_card.dart';
import 'package:hawkr_seller/shared/widgets/hk_content_card.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('coupon cards and action use shared accessible controls', (
    tester,
  ) async {
    var submitted = false;
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Column(
            children: [
              const CouponSectionCard(child: Text('優惠內容')),
              CouponSubmitButton(
                isComplete: true,
                saving: false,
                onTap: () => submitted = true,
              ),
            ],
          ),
        ),
      ),
    );

    expect(find.byType(HkContentCard), findsOneWidget);
    expect(find.byType(FilledButton), findsOneWidget);
    expect(
      tester.getSize(find.byType(FilledButton)).height,
      greaterThanOrEqualTo(48),
    );
    await tester.tap(find.text('儲存並發放優惠券'));
    expect(submitted, isTrue);
  });

  testWidgets('coupon creation flow stays readable on a 320pt phone', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(320, 1800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final nameCtrl = TextEditingController(text: '週末鮮果優惠');
    final discountCtrl = TextEditingController(text: '100');
    final thresholdCtrl = TextEditingController(text: '500');
    final maxDiscountCtrl = TextEditingController();
    addTearDown(nameCtrl.dispose);
    addTearDown(discountCtrl.dispose);
    addTearDown(thresholdCtrl.dispose);
    addTearDown(maxDiscountCtrl.dispose);

    var selectedTarget = '全部會員';
    await tester.pumpWidget(
      MaterialApp(
        builder: (context, child) {
          final media = MediaQuery.of(context);
          return MediaQuery(
            data: media.copyWith(textScaler: const TextScaler.linear(1.3)),
            child: child!,
          );
        },
        home: Scaffold(
          body: StatefulBuilder(
            builder: (context, setState) {
              return ListView(
                padding: const EdgeInsets.all(16),
                children: [
                  CouponPreviewCard(
                    nameCtrl: nameCtrl,
                    discountCtrl: discountCtrl,
                    minPurchaseCtrl: thresholdCtrl,
                    discountRule: '現金折抵',
                    noExpiry: false,
                    hasExpiry: true,
                    validUntil: DateTime(2026, 12, 31),
                  ),
                  const SizedBox(height: 16),
                  CouponFormCard(
                    nameCtrl: nameCtrl,
                    discountCtrl: discountCtrl,
                    minPurchaseCtrl: thresholdCtrl,
                    discountRule: '現金折抵',
                    onDiscountRuleChanged: (_) {},
                    maxDiscountAmountCtrl: maxDiscountCtrl,
                  ),
                  const SizedBox(height: 16),
                  CouponDatePickerCard(
                    noExpiry: false,
                    selectedDays: null,
                    useCustomDate: true,
                    validUntil: DateTime(2026, 12, 31),
                    hasExpiry: true,
                    calendarMonth: DateTime(2026, 12),
                    onNoExpiryChanged: (_) {},
                    onCustomDateChanged: (_) {},
                    onDaysSelected: (_) {},
                    onDateSelected: (_) {},
                    onCalendarMonthChanged: (_) {},
                  ),
                  const SizedBox(height: 16),
                  CouponTargetUserCard(
                    targetUserType: selectedTarget,
                    onTypeSelected: (value) =>
                        setState(() => selectedTarget = value),
                  ),
                ],
              );
            },
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(find.text('即時票券預覽'), findsOneWidget);
    expect(find.text('設定優惠內容'), findsOneWidget);
    expect(find.text('設定使用期限'), findsOneWidget);

    await tester.scrollUntilVisible(
      find.text('選擇發放對象'),
      600,
      scrollable: find.byType(Scrollable).first,
      maxScrolls: 20,
    );
    expect(find.text('選擇發放對象'), findsOneWidget);
    await tester.scrollUntilVisible(
      find.text('指定會員'),
      300,
      scrollable: find.byType(Scrollable).first,
      maxScrolls: 10,
    );
    await tester.ensureVisible(find.text('指定會員'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('指定會員'));
    await tester.pumpAndSettle();
    expect(selectedTarget, '指定會員');
    expect(tester.takeException(), isNull);

    final targetControl = find
        .ancestor(of: find.text('指定會員'), matching: find.byType(InkWell))
        .first;
    expect(tester.getSize(targetControl).height, greaterThanOrEqualTo(48));
  });

  testWidgets('coupon flow adapts to landscape with reduced motion', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(667, 375);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final nameCtrl = TextEditingController(text: '會員感謝祭');
    final discountCtrl = TextEditingController(text: '15');
    final thresholdCtrl = TextEditingController(text: '800');
    final maxDiscountCtrl = TextEditingController(text: '300');
    addTearDown(nameCtrl.dispose);
    addTearDown(discountCtrl.dispose);
    addTearDown(thresholdCtrl.dispose);
    addTearDown(maxDiscountCtrl.dispose);

    await tester.pumpWidget(
      MaterialApp(
        builder: (context, child) {
          final media = MediaQuery.of(context);
          return MediaQuery(
            data: media.copyWith(
              disableAnimations: true,
              textScaler: const TextScaler.linear(1.3),
            ),
            child: child!,
          );
        },
        home: Scaffold(
          body: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: Column(
              children: [
                CouponPreviewCard(
                  nameCtrl: nameCtrl,
                  discountCtrl: discountCtrl,
                  minPurchaseCtrl: thresholdCtrl,
                  discountRule: '折數折抵',
                  noExpiry: false,
                  hasExpiry: true,
                  validUntil: DateTime(2026, 12, 31),
                ),
                const SizedBox(height: 16),
                CouponFormCard(
                  nameCtrl: nameCtrl,
                  discountCtrl: discountCtrl,
                  minPurchaseCtrl: thresholdCtrl,
                  discountRule: '折數折抵',
                  onDiscountRuleChanged: (_) {},
                  maxDiscountAmountCtrl: maxDiscountCtrl,
                ),
                const SizedBox(height: 16),
                CouponTargetUserCard(
                  targetUserType: 'TAG 標籤',
                  onTypeSelected: (_) {},
                ),
              ],
            ),
          ),
        ),
      ),
    );
    await tester.pump();

    expect(tester.takeException(), isNull);
    expect(find.text('15% OFF'), findsOneWidget);
    expect(find.text('TAG 標籤'), findsOneWidget);
  });
}
