import 'package:hawkr_seller/features/customer/data/subscriber_model.dart';
import 'package:hawkr_seller/features/customer/presentation/widgets/subscriber_list_content.dart';
import 'package:hawkr_seller/shared/widgets/hk_empty_state.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  testWidgets('subscriber section preserves empty-state copy and semantics', (
    tester,
  ) async {
    await tester.pumpWidget(
      _Harness(
        child: SubscriberListContent(
          state: AsyncData(SubscriberListViewData(const [])),
          onSubscriberTap: (_) {},
        ),
      ),
    );
    await tester.pump();

    expect(find.byType(HkEmptyState), findsOneWidget);
    expect(find.text('暫無訂閱者'), findsOneWidget);
    expect(find.text('買家訂閱商店後會自動出現在這裡。'), findsOneWidget);
  });

  testWidgets('subscriber section renders and dispatches the selected item', (
    tester,
  ) async {
    const subscriber = Subscriber(
      buyerId: '42',
      subscribedAt: '2026-07-27',
      nickname: '陳小農',
    );
    Subscriber? selected;

    await tester.pumpWidget(
      _Harness(
        child: SubscriberListContent(
          state: AsyncData(SubscriberListViewData(const [subscriber])),
          onSubscriberTap: (value) => selected = value,
        ),
      ),
    );

    expect(find.text('陳小農'), findsOneWidget);
    expect(find.text('訂閱者'), findsOneWidget);

    await tester.tap(find.text('陳小農'));
    await tester.pump();
    expect(identical(selected, subscriber), isTrue);
  });
}

class _Harness extends StatelessWidget {
  const _Harness({required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    return ScreenUtilInit(
      designSize: const Size(390, 844),
      builder: (_, __) => MaterialApp(home: Scaffold(body: child)),
    );
  }
}
