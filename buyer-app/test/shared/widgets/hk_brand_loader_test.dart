import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';
import 'package:hawkr_buyer/shared/widgets/hk_skeleton_shimmer.dart';
import 'package:flutter/material.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:flutter_test/flutter_test.dart';

Widget _app({required Widget child, bool disableAnimations = false}) {
  return MediaQuery(
    data: MediaQueryData(disableAnimations: disableAnimations),
    child: Directionality(
      textDirection: TextDirection.ltr,
      child: Center(child: child),
    ),
  );
}

/// 遮雨棚與檯面都已綻放、尚未開始收回的時間點（網頁 keyframes 的 46%～86% 之間）。
const _allBladesUp = Duration(milliseconds: 1800);

void main() {
  test('時間軸與網頁 ilBladeAwning／ilBladeCounter keyframes 對齊', () {
    // 網頁 .il-blade 的 `2.6s` 迴圈；改這個值等於讓 App 與網頁的載入動態分岔。
    expect(HkBrandMark.loopDuration, const Duration(milliseconds: 2600));
    // 網頁 keyframes 在 86% 開始收回，減少動態時要停在收回之前的完成態。
    expect(HkBrandMark.staticFrame, lessThan(0.86));
  });

  testWidgets('依指定尺寸畫出線稿加遮雨棚與檯面', (tester) async {
    await tester.pumpWidget(
      _app(child: const HkBrandLoader(size: HkBrandLoader.page)),
    );

    expect(
      tester.getSize(find.byType(HkBrandLoader)),
      const Size.square(HkBrandLoader.page),
    );
    // 起始幀只有兩條線稿；遮雨棚（底色加條紋）與檯面要等綻放序列跑完才到齊。
    expect(find.byType(SvgPicture), findsNWidgets(2));
    await tester.pump(_allBladesUp);
    expect(find.byType(SvgPicture), findsNWidgets(5));
  });

  testWidgets('不給尺寸時填滿父層', (tester) async {
    await tester.pumpWidget(
      _app(
        child: const SizedBox(width: 18, height: 18, child: HkBrandLoader()),
      ),
    );

    expect(tester.getSize(find.byType(HkBrandLoader)), const Size(18, 18));
  });

  testWidgets('mono 把整枚商標壓成呼叫端給的前景色', (tester) async {
    await tester.pumpWidget(
      _app(
        child: const HkBrandLoader(
          size: HkBrandLoader.buttonLg,
          tone: HkBrandLoaderTone.mono,
          color: AppColors.bgCard,
        ),
      ),
    );

    await tester.pump(_allBladesUp);

    // 實心品牌色底上，只要有一個部分仍是品牌色就會整片隱形。
    const expected = ColorFilter.mode(AppColors.bgCard, BlendMode.srcIn);
    final pictures = tester.widgetList<SvgPicture>(find.byType(SvgPicture));
    expect(pictures, hasLength(5));
    for (final picture in pictures) {
      expect(picture.colorFilter, expected);
    }
  });

  testWidgets('brand 用遮雨棚、條紋、檯面三色，不是單一顏色', (tester) async {
    await tester.pumpWidget(
      _app(child: const HkBrandLoader(size: HkBrandLoader.page)),
    );

    await tester.pump(_allBladesUp);

    final filters = tester
        .widgetList<SvgPicture>(find.byType(SvgPicture))
        .map((picture) => picture.colorFilter)
        .toSet();
    expect(filters, hasLength(3));
  });

  testWidgets('減少動態時停在完成態，不再排下一幀', (tester) async {
    await tester.pumpWidget(
      _app(
        disableAnimations: true,
        child: const HkBrandLoader(size: HkBrandLoader.page),
      ),
    );
    await tester.pump();

    expect(tester.binding.hasScheduledFrame, isFalse);
  });

  testWidgets('一般情況持續循環播放', (tester) async {
    await tester.pumpWidget(
      _app(child: const HkBrandLoader(size: HkBrandLoader.page)),
    );
    await tester.pump(const Duration(milliseconds: 600));

    expect(tester.binding.hasScheduledFrame, isTrue);
  });

  testWidgets('骨架屏掃光在減少動態時整層移除', (tester) async {
    const child = SizedBox(
      width: 120,
      height: 20,
      child: ColoredBox(color: AppColors.dividerLight),
    );

    await tester.pumpWidget(_app(child: const HkSkeletonShimmer(child: child)));
    expect(find.byType(DecoratedBox), findsWidgets);

    await tester.pumpWidget(
      _app(
        disableAnimations: true,
        child: const HkSkeletonShimmer(child: child),
      ),
    );
    await tester.pump();

    expect(tester.binding.hasScheduledFrame, isFalse);
    expect(find.byType(DecoratedBox), findsNothing);
  });
}
