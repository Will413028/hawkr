import 'package:flutter/widgets.dart';

/// 把全域懸浮 Navbar 的避讓高度放進 [CustomScrollView] 的可捲動內容。
///
/// `BuyerFloatingNavigationLayout` 透過 [MediaQueryData.padding] 提供固定的
/// Navbar clearance；`CustomScrollView` 不會自動消耗該值，因此頁面需在最後
/// 加入這個 sliver。Navbar 顯示時最後內容可完整捲到其上方，隱藏時 viewport
/// 仍保持全高，不會留下固定空白帶或造成內容跳動。
class BuyerNavigationClearanceSliver extends StatelessWidget {
  const BuyerNavigationClearanceSliver({super.key});

  @override
  Widget build(BuildContext context) {
    return SliverToBoxAdapter(
      child: SizedBox(height: MediaQuery.paddingOf(context).bottom),
    );
  }
}
