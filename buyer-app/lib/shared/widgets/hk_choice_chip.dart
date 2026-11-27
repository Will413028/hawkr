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
              widthFactor: 1,
              child: AnimatedContainer(
                duration: HkMotion.duration,
                curve: HkMotion.emphasized,
                height: HkControl.compactChoiceHeight,
                padding: const EdgeInsets.symmetric(
                  horizontal: HkControl.compactChoiceHorizontalPadding,
                ),
                decoration: ShapeDecoration(
                  color: selected ? AppColors.actionPrimary : AppColors.gray100,
                  shape: const StadiumBorder(),
                ),
                child: Align(
                  alignment: Alignment.center,
                  widthFactor: 1,
                  child: AnimatedDefaultTextStyle(
                    duration: HkMotion.duration,
                    curve: HkMotion.emphasized,
                    style: TextStyle(
                      fontSize: HkControl.compactChoiceFontSize,
                      height: 1,
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
