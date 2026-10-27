import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:flutter/material.dart';

/// Hawkr 唯一的緊湊型選擇控制。
///
/// 適用於排序、篩選、縣市等同層級的單選／複選項目。視覺規格取自網頁設計系統
/// 的 sm 控制項階梯（見 [HkControl.compactChoiceHeight]）。
///
/// [InkWell] 刻意包在 48dp 的外框上、而不是包在 32dp 的膠囊裡：膠囊只負責上色，
/// 整個 48dp 都要能點。先前相反，外框只是版面佔位，上下各 8dp 是不會反應的死區。
class HkChoiceChip extends StatelessWidget {
  const HkChoiceChip({
    super.key,
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      selected: selected,
      child: SizedBox(
        height: HkControl.minTap,
        child: Material(
          color: Colors.transparent,
          child: InkWell(
            onTap: onTap,
            customBorder: const StadiumBorder(),
            child: Center(
              // widthFactor 讓整顆收斂成膠囊寬度；少了它，chip 會撐滿整行，
              // Wrap 會變成一行一顆。
              widthFactor: 1,
              child: AnimatedContainer(
                duration: HkMotion.duration,
                curve: HkMotion.emphasized,
                height: HkControl.compactChoiceHeight,
                padding: const EdgeInsets.symmetric(
                  horizontal: HkControl.compactChoiceHorizontalPadding,
                ),
                // 這裡不能用 Container 的 alignment：一旦設了 alignment，
                // Container 就會撐滿可用寬度，Wrap 會變成一行一顆。垂直置中
                // 改由下面的 Align 負責（見該處註解）。
                decoration: ShapeDecoration(
                  // 未選中吃淡色實底而不是描邊：頁面本身就是米白，14% 透明度的
                  // 細框疊上去只會讀成一圈沒畫完的灰線。對應網頁 Chip 的 default
                  // solid（`border-transparent bg-[var(--hkui-default-100)]`，該
                  // token 即 neutral-whisper，與這裡的 gray100 是同一個色票）。
                  color: selected ? AppColors.actionPrimary : AppColors.gray100,
                  shape: const StadiumBorder(),
                ),
                // 固定高度會把 tight constraint 一路傳到 Text，文字就會被釘在
                // 盒子頂端。Align 的 heightFactor 留 null 以撐滿這 32dp 並把
                // 文字垂直置中，widthFactor: 1 則讓寬度仍然收斂成文字寬度。
                child: Align(
                  alignment: Alignment.center,
                  widthFactor: 1,
                  child: AnimatedDefaultTextStyle(
                    duration: HkMotion.duration,
                    curve: HkMotion.emphasized,
                    style: TextStyle(
                      fontSize: HkControl.compactChoiceFontSize,
                      height: 1,
                      // 字重固定。選取時若從 w500 跳到 w700，膠囊寬度會跟著變，
                      // 而篩選面板是 Wrap 排版，每點一下整排就重新換行。選取狀態
                      // 交給實底與反白文字表達，已經夠明確。
                      fontWeight: FontWeight.w500,
                      color: selected
                          ? AppColors.textInverse
                          : AppColors.textSecondary,
                    ),
                    child: Text(
                      label,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
