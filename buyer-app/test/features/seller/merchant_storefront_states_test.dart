import 'dart:convert';
import 'dart:typed_data';
import 'dart:ui' show Tristate;

import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/features/seller/presentation/widgets/merchant_chat_tab.dart';
import 'package:hawkr_buyer/features/seller/presentation/widgets/merchant_empty_state.dart';
import 'package:hawkr_buyer/features/seller/presentation/widgets/merchant_home_tab.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/widgets/hk_empty_state.dart';
import 'package:hawkr_buyer/shared/widgets/hk_frosted_icon_button.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

class _EmptyListAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    return ResponseBody.fromString(
      jsonEncode(const <String, dynamic>{
        'code': 0,
        'data': <String, dynamic>{'records': <dynamic>[]},
      }),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  Future<void> pumpStorefront(
    WidgetTester tester,
    Widget Function(WidgetRef ref) builder,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    final dio = Dio(BaseOptions(baseUrl: 'https://test.local'))
      ..httpClientAdapter = _EmptyListAdapter();
    addTearDown(dio.close);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [dioProvider.overrideWithValue(dio)],
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          child: Consumer(
            builder: (context, ref, _) =>
                MaterialApp(home: Scaffold(body: builder(ref))),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('comments use the shared compact semantic empty state', (
    tester,
  ) async {
    await pumpStorefront(
      tester,
      (ref) => MerchantHomeTab(ref: ref, merchantId: '2683382637'),
    );

    expect(find.byType(MerchantEmptyState), findsOneWidget);
    expect(find.text('暫無貼文'), findsOneWidget);

    await pumpStorefront(
      tester,
      (ref) => MerchantChatTab(
        ref: ref,
        merchantId: '2683382637',
        merchantName: '金湖水產',
      ),
    );

    expect(find.byType(HkEmptyState), findsOneWidget);
    expect(find.text('暫無留言'), findsOneWidget);
    final emptyIcon = tester.widget<HkIcon>(
      find.byKey(const Key('merchant-comments-empty-icon')),
    );
    expect(emptyIcon.name, HkIconName.chat);
    expect(emptyIcon.size, HkEmptyState.compactIconSize);
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'comment composer preserves input treatment and frosted send semantics',
    (tester) async {
      final semantics = tester.ensureSemantics();
      try {
        await pumpStorefront(
          tester,
          (ref) => MerchantChatTab(
            ref: ref,
            merchantId: '2683382637',
            merchantName: '金湖水產',
          ),
        );

        final composer = find.byKey(const Key('merchant-comment-composer'));
        expect(tester.widget(composer), isA<TextFieldTapRegion>());
        expect(
          tester.widget<TextFieldTapRegion>(composer).child,
          isA<SafeArea>(),
        );
        expect(
          tester
              .getSize(find.byKey(const Key('merchant-comment-composer')))
              .height,
          lessThanOrEqualTo(76),
        );

        final input = tester.widget<TextField>(
          find.byKey(const Key('merchant-comment-input')),
        );
        expect(input.minLines, 1);
        expect(input.maxLines, 4);
        expect(input.maxLength, 500);
        expect(input.decoration!.hintText, '留言給這個賣家...');
        expect(input.decoration!.filled, isTrue);
        expect(input.decoration!.fillColor, AppColors.bgCard);
        final enabledBorder =
            input.decoration!.enabledBorder! as OutlineInputBorder;
        final focusedBorder =
            input.decoration!.focusedBorder! as OutlineInputBorder;
        expect(enabledBorder.borderRadius, BorderRadius.circular(12));
        expect(enabledBorder.borderSide.color, AppColors.borderInput);
        expect(focusedBorder.borderRadius, BorderRadius.circular(12));
        expect(focusedBorder.borderSide.color, AppColors.actionPrimary);
        expect(focusedBorder.borderSide.width, 1.5);

        final submitSurface = find.byKey(
          const Key('merchant-comment-submit-surface'),
        );
        final submit = find.byKey(const Key('merchant-comment-submit'));
        expect(
          find.descendant(
            of: composer,
            matching: find.byType(HkFrostedIconButton),
          ),
          findsOneWidget,
        );
        expect(tester.getSize(submitSurface), const Size.square(48));
        expect(tester.widget<HkFrostedIconButton>(submit).onPressed, isNull);
        final submitButton = find.descendant(
          of: submit,
          matching: find.byType(IconButton),
        );
        expect(tester.widget<IconButton>(submitButton).onPressed, isNull);
        final disabledSemantics = tester.getSemantics(
          find.bySemanticsLabel('送出留言'),
        );
        expect(disabledSemantics.flagsCollection.isButton, isTrue);
        expect(disabledSemantics.flagsCollection.isEnabled, Tristate.isFalse);

        await tester.enterText(
          find.byKey(const Key('merchant-comment-input')),
          '商品很新鮮',
        );
        await tester.pump();
        expect(tester.widget<HkFrostedIconButton>(submit).onPressed, isNotNull);
        expect(tester.widget<HkFrostedIconButton>(submit).loading, isFalse);
        expect(tester.widget<IconButton>(submitButton).onPressed, isNotNull);
        expect(tester.getSize(submitSurface), const Size.square(48));
        final enabledSemantics = tester.getSemantics(
          find.bySemanticsLabel('送出留言'),
        );
        expect(enabledSemantics.flagsCollection.isButton, isTrue);
        expect(enabledSemantics.flagsCollection.isEnabled, Tristate.isTrue);
        expect(find.byIcon(AppIcons.send), findsOneWidget);
        expect(tester.takeException(), isNull);
      } finally {
        semantics.dispose();
      }
    },
  );
}
