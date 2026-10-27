import 'package:flutter/material.dart';

/// Shared-element wrapper for product media.
///
/// The flight follows a direct rect interpolation instead of Material's arced
/// path. Keeping the Hero around media only lets the native Cupertino route own
/// the page movement while the product image maintains spatial continuity.
class ProductMediaHero extends StatelessWidget {
  const ProductMediaHero({super.key, required this.tag, required this.child});

  final Object? tag;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    if (tag == null) return child;
    return Hero(
      tag: tag!,
      transitionOnUserGestures: true,
      createRectTween: (begin, end) => RectTween(begin: begin, end: end),
      flightShuttleBuilder: productMediaHeroFlightShuttleBuilder,
      child: child,
    );
  }
}

/// Keeps the already-rendered list image during both push and interactive pop.
///
/// This avoids decoding or swapping image presets while the user's finger is
/// driving the iOS edge-swipe gesture.
Widget productMediaHeroFlightShuttleBuilder(
  BuildContext flightContext,
  Animation<double> animation,
  HeroFlightDirection flightDirection,
  BuildContext fromHeroContext,
  BuildContext toHeroContext,
) {
  final cardHeroContext = flightDirection == HeroFlightDirection.push
      ? fromHeroContext
      : toHeroContext;
  final cardHero = cardHeroContext.widget as Hero;
  return Material(type: MaterialType.transparency, child: cardHero.child);
}

/// 商品卡 Hero 飛行時固定使用列表卡片的原始比例。
///
/// 詳情頁與列表卡片長寬差異很大，直接讓 Hero child 接收飛行中的
/// constraints 會把商品圖片拉長。用來源卡片尺寸搭配 [BoxFit.contain]，
/// 讓整段放大與縮回動畫都維持原比例。
Widget productCardHeroFlightShuttleBuilder(
  BuildContext flightContext,
  Animation<double> animation,
  HeroFlightDirection flightDirection,
  BuildContext fromHeroContext,
  BuildContext toHeroContext,
) {
  final cardHeroContext = flightDirection == HeroFlightDirection.push
      ? fromHeroContext
      : toHeroContext;
  final cardHero = cardHeroContext.widget as Hero;
  final renderObject = cardHeroContext.findRenderObject();
  // Hero 所在的 ListView 可能把垂直 constraints 設為 unbounded；飛行途中若
  // 直接沿用 constraints.biggest，來源卡片會被目的頁的 tight constraints
  // 重排，造成 RenderFlex overflow。實際 render size 才是使用者看到的卡片
  // 尺寸，也能讓商品圖片與圓角在整段動畫維持原比例。
  final cardSize = renderObject is RenderBox && renderObject.hasSize
      ? renderObject.size
      : null;

  if (cardSize == null || cardSize.isEmpty) {
    return cardHero.child;
  }

  return Material(
    color: Colors.transparent,
    child: FittedBox(
      fit: BoxFit.contain,
      child: SizedBox.fromSize(size: cardSize, child: cardHero.child),
    ),
  );
}
