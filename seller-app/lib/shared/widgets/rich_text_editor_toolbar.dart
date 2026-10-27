import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'hk_tap_target.dart';
import 'rich_text_editor_text_block.dart';

/// 編輯器工具列
class EditorToolbar extends StatelessWidget {
  const EditorToolbar({
    super.key,
    required this.onAddBlock,
    this.onBold,
    this.onItalic,
    this.onUnderline,
    this.onLink,
    this.onColorPicker,
    this.onAlignLeft,
    this.onAlignCenter,
    this.onAlignRight,
    this.onTextStyle,
    this.onInsertImage,
    this.onInsertTable,
    this.onInsertBulletList,
    this.onInsertNumberedList,
    this.onUndo,
    this.onRedo,
    this.isBoldActive = false,
    this.isItalicActive = false,
    this.isUnderlineActive = false,
    this.isLinkActive = false,
    this.isColorActive = false,
    required this.currentLength,
    required this.maxLength,
    this.selectedStyle,
    this.bottomPadding = 0,
  });

  final VoidCallback onAddBlock;
  final VoidCallback? onBold;
  final VoidCallback? onItalic;
  final VoidCallback? onUnderline;
  final VoidCallback? onLink;
  final VoidCallback? onColorPicker;
  final VoidCallback? onAlignLeft;
  final VoidCallback? onAlignCenter;
  final VoidCallback? onAlignRight;
  final VoidCallback? onTextStyle;
  final VoidCallback? onInsertImage;
  final VoidCallback? onInsertTable;
  final VoidCallback? onInsertBulletList;
  final VoidCallback? onInsertNumberedList;
  final VoidCallback? onUndo;
  final VoidCallback? onRedo;
  final bool isBoldActive;
  final bool isItalicActive;
  final bool isUnderlineActive;
  final bool isLinkActive;
  final bool isColorActive;
  final int currentLength;
  final int maxLength;
  final TextBlockStyle? selectedStyle;
  final double bottomPadding;

  @override
  Widget build(BuildContext context) {
    final hasSelection = selectedStyle != null;

    return Container(
      decoration: BoxDecoration(
        color: Colors.white,
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 8.r,
            offset: Offset(0, -2.h),
          ),
        ],
      ),
      child: SafeArea(
        top: false,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // 第一行工具列
            Container(
              height: 48,
              padding: EdgeInsets.symmetric(horizontal: 12.w),
              child: SingleChildScrollView(
                key: const PageStorageKey('editor-toolbar-primary'),
                scrollDirection: Axis.horizontal,
                child: Row(
                  children: [
                    // + 區塊 按鈕
                    _AddBlockButton(onTap: onAddBlock),
                    SizedBox(width: 12.w),
                    // 分隔線
                    Container(
                      width: 1,
                      height: 24.h,
                      color: AppColors.dividerLight,
                    ),
                    SizedBox(width: 12.w),
                    // 粗體
                    _ToolbarIconButton(
                      semanticLabel: '粗體',
                      icon: AppIcons.formatBold,
                      isActive: isBoldActive,
                      toggled: isBoldActive,
                      isEnabled: onBold != null,
                      onTap: onBold,
                    ),
                    // 斜體
                    _ToolbarIconButton(
                      semanticLabel: '斜體',
                      icon: AppIcons.formatItalic,
                      isActive: isItalicActive,
                      toggled: isItalicActive,
                      isEnabled: onItalic != null,
                      onTap: onItalic,
                    ),
                    // 底線
                    _ToolbarIconButton(
                      semanticLabel: '底線',
                      icon: AppIcons.formatUnderlined,
                      isActive: isUnderlineActive,
                      toggled: isUnderlineActive,
                      isEnabled: onUnderline != null,
                      onTap: onUnderline,
                    ),
                    // 連結
                    _ToolbarIconButton(
                      semanticLabel: '插入連結',
                      icon: AppIcons.link,
                      isActive: isLinkActive,
                      toggled: isLinkActive,
                      isEnabled: onLink != null,
                      onTap: onLink,
                    ),
                    // 調色盤
                    _ToolbarIconButton(
                      semanticLabel: '文字顏色',
                      icon: AppIcons.paletteOutlined,
                      isActive: isColorActive,
                      selected: isColorActive,
                      isEnabled: onColorPicker != null,
                      onTap: onColorPicker,
                    ),
                    SizedBox(width: 8.w),
                    // 分隔線
                    Container(
                      width: 1,
                      height: 24.h,
                      color: AppColors.dividerLight,
                    ),
                    SizedBox(width: 8.w),
                    // 對齊方式
                    _AlignmentButton(
                      alignment: TextAlign.left,
                      currentAlignment: selectedStyle?.alignment,
                      isEnabled: hasSelection,
                      onTap: onAlignLeft,
                    ),
                    _AlignmentButton(
                      alignment: TextAlign.center,
                      currentAlignment: selectedStyle?.alignment,
                      isEnabled: hasSelection,
                      onTap: onAlignCenter,
                    ),
                    _AlignmentButton(
                      alignment: TextAlign.right,
                      currentAlignment: selectedStyle?.alignment,
                      isEnabled: hasSelection,
                      onTap: onAlignRight,
                    ),
                  ],
                ),
              ),
            ),
            // 分隔線
            Container(height: 1, color: AppColors.dividerLight),
            // 第二行工具列
            Container(
              height: 48,
              padding: EdgeInsets.symmetric(horizontal: 12.w),
              child: SingleChildScrollView(
                key: const PageStorageKey('editor-toolbar-secondary'),
                scrollDirection: Axis.horizontal,
                child: Row(
                  children: [
                    // T 文字樣式
                    _ToolbarIconButton(
                      semanticLabel: '文字樣式',
                      icon: AppIcons.textFields,
                      isActive: false,
                      isEnabled: hasSelection,
                      onTap: onTextStyle,
                      size: 22,
                    ),
                    // 圖片插入
                    _ToolbarIconButton(
                      semanticLabel: '插入圖片',
                      icon: AppIcons.imageOutlined,
                      isActive: false,
                      isEnabled: onInsertImage != null,
                      onTap: onInsertImage,
                      size: 22,
                    ),
                    // 表格插入
                    _ToolbarIconButton(
                      semanticLabel: '插入表格',
                      icon: AppIcons.tableChartOutlined,
                      isActive: false,
                      isEnabled: onInsertTable != null,
                      onTap: onInsertTable,
                      size: 22,
                    ),
                    // 項目符號清單插入
                    _ToolbarIconButton(
                      semanticLabel: '插入項目符號清單',
                      icon: AppIcons.formatListBulleted,
                      isActive: false,
                      isEnabled: onInsertBulletList != null,
                      onTap: onInsertBulletList,
                      size: 22,
                    ),
                    // 編號清單插入
                    _ToolbarIconButton(
                      semanticLabel: '插入編號清單',
                      icon: AppIcons.formatListNumbered,
                      isActive: false,
                      isEnabled: onInsertNumberedList != null,
                      onTap: onInsertNumberedList,
                      size: 22,
                    ),
                    // 注意：之前這裡有一個 # hashtag 按鈕，但用戶反饋輸入框內直接打 #
                    // 就會自動觸發 autocomplete，按鈕多餘 → 移除。
                    // hashtag 計數器仍在頂部 bar (# n/10)。
                    SizedBox(width: 12.w),
                    Container(
                      width: 1,
                      height: 24.h,
                      color: AppColors.dividerLight,
                    ),
                    SizedBox(width: 12.w),
                    // 撤銷
                    _ToolbarIconButton(
                      semanticLabel: '撤銷',
                      icon: AppIcons.undo,
                      isActive: false,
                      isEnabled: onUndo != null,
                      onTap: onUndo,
                      size: 20,
                    ),
                    // 重做
                    _ToolbarIconButton(
                      semanticLabel: '重做',
                      icon: AppIcons.redo,
                      isActive: false,
                      isEnabled: onRedo != null,
                      onTap: onRedo,
                      size: 20,
                    ),
                    SizedBox(width: 8.w),
                    // 字數統計
                    Semantics(
                      label: '字數 $currentLength，共 $maxLength',
                      child: ExcludeSemantics(
                        child: Text(
                          '$currentLength/$maxLength',
                          style: TextStyle(
                            fontSize: 12.sp,
                            color: currentLength > maxLength
                                ? AppColors.dangerStrong
                                : AppColors.textSecondary,
                          ),
                        ),
                      ),
                    ),
                    SizedBox(width: 8.w),
                    // 鍵盤按鈕
                    HkTapTarget(
                      semanticLabel: '隱藏鍵盤',
                      hint: '收起螢幕鍵盤',
                      tooltip: '隱藏鍵盤',
                      onTap: () => FocusScope.of(context).unfocus(),
                      child: Container(
                        padding: EdgeInsets.all(4.w),
                        child: Icon(
                          AppIcons.keyboardHide,
                          color: AppIconColors.muted,
                          size: 22.sp,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
            // 底部安全區域
            if (bottomPadding > 0) SizedBox(height: bottomPadding),
          ],
        ),
      ),
    );
  }
}

/// + 區塊 按鈕
class _AddBlockButton extends StatelessWidget {
  const _AddBlockButton({required this.onTap});

  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return HkTapTarget(
      semanticLabel: '新增內容區塊',
      hint: '在編輯器中加入文字區塊',
      tooltip: '新增區塊',
      onTap: onTap,
      child: Container(
        padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 8.h),
        decoration: BoxDecoration(
          color: AppColors.primary,
          borderRadius: BorderRadius.circular(24.r),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(AppIcons.add, color: AppIconColors.onBrand, size: 18.sp),
            SizedBox(width: 4.w),
            Text(
              '區塊',
              style: TextStyle(
                color: Colors.white,
                fontSize: 14.sp,
                fontWeight: FontWeight.w700,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// 工具列圖標按鈕
class _ToolbarIconButton extends StatelessWidget {
  const _ToolbarIconButton({
    required this.semanticLabel,
    required this.icon,
    required this.isActive,
    required this.isEnabled,
    required this.onTap,
    this.toggled,
    this.selected,
    this.size = 24,
  });

  final String semanticLabel;
  final IconData icon;
  final bool isActive;
  final bool isEnabled;
  final VoidCallback? onTap;
  final bool? toggled;
  final bool? selected;
  final double size;

  @override
  Widget build(BuildContext context) {
    final color = !isEnabled
        ? AppColors.gray300
        : isActive
        ? AppColors.primary
        : AppColors.textPrimary;

    return HkTapTarget(
      semanticLabel: semanticLabel,
      hint: isEnabled ? '啟用$semanticLabel' : '$semanticLabel目前無法使用',
      tooltip: semanticLabel,
      toggled: toggled,
      selected: selected,
      onTap: isEnabled ? onTap : null,
      child: Container(
        padding: EdgeInsets.all(8.w),
        child: Icon(icon, color: color, size: size.sp),
      ),
    );
  }
}

/// 對齊方式按鈕
class _AlignmentButton extends StatelessWidget {
  const _AlignmentButton({
    required this.alignment,
    required this.currentAlignment,
    required this.isEnabled,
    required this.onTap,
  });

  final TextAlign alignment;
  final TextAlign? currentAlignment;
  final bool isEnabled;
  final VoidCallback? onTap;

  IconData get _icon {
    switch (alignment) {
      case TextAlign.left:
        return AppIcons.formatAlignLeft;
      case TextAlign.center:
        return AppIcons.formatAlignCenter;
      case TextAlign.right:
        return AppIcons.formatAlignRight;
      default:
        return AppIcons.formatAlignLeft;
    }
  }

  @override
  Widget build(BuildContext context) {
    final isActive = currentAlignment == alignment;
    final semanticLabel = switch (alignment) {
      TextAlign.center => '置中對齊',
      TextAlign.right => '靠右對齊',
      _ => '靠左對齊',
    };
    final color = !isEnabled
        ? AppColors.gray300
        : isActive
        ? AppColors.primary
        : AppColors.textPrimary;

    return HkTapTarget(
      semanticLabel: semanticLabel,
      hint: isEnabled ? '設定文字$semanticLabel' : '請先選取文字區塊',
      tooltip: semanticLabel,
      selected: isActive,
      onTap: isEnabled ? onTap : null,
      child: Container(
        padding: EdgeInsets.all(8.w),
        decoration: isActive
            ? BoxDecoration(
                color: const Color(0x1AB8441F),
                borderRadius: BorderRadius.circular(6.r),
              )
            : null,
        child: Icon(_icon, color: color, size: 22.sp),
      ),
    );
  }
}
