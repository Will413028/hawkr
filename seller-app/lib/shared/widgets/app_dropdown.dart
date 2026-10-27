import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

/// A reusable dropdown widget that opens an overlay below the trigger.
///
/// Supports:
/// - Default trigger button style (white background, rounded border)
/// - Custom trigger via [child] parameter
/// - Optional search bar ([showSearch])
/// - Long press drag to highlight and select items
/// - Selected item highlighted with [Color(0x338F3315)]
/// - Hovered item highlighted with [Color(0xB38F3315)] + white text
class AppDropdown extends StatefulWidget {
  const AppDropdown({
    super.key,
    required this.items,
    required this.onSelected,
    this.selectedItem,
    this.placeholder = '選擇',
    this.width,
    this.height,
    this.showSearch = false,
    this.child,
  });

  final List<String> items;
  final ValueChanged<String> onSelected;
  final String? selectedItem;
  final String placeholder;
  final double? width;
  final double? height;
  final bool showSearch;

  /// Custom trigger. If provided, this widget is wrapped with a GestureDetector
  /// that opens/closes the dropdown. If null, the default trigger style is used.
  final Widget? child;

  @override
  State<AppDropdown> createState() => _AppDropdownState();
}

class _AppDropdownState extends State<AppDropdown> {
  final _triggerKey = GlobalKey();
  OverlayEntry? _overlayEntry;

  bool get _isOpen => _overlayEntry != null;

  void _show() {
    if (_isOpen) {
      _dismiss();
      return;
    }
    final box = _triggerKey.currentContext?.findRenderObject() as RenderBox?;
    if (box == null) return;
    final pos = box.localToGlobal(Offset.zero);
    final size = box.size;
    _overlayEntry = OverlayEntry(
      builder: (_) => _AppDropdownOverlay(
        position: pos,
        fieldWidth: size.width,
        fieldHeight: size.height,
        items: widget.items,
        selectedItem: widget.selectedItem,
        showSearch: widget.showSearch,
        onSelect: (item) {
          widget.onSelected(item);
          _dismiss();
        },
        onDismiss: _dismiss,
      ),
    );
    Overlay.of(context).insert(_overlayEntry!);
    setState(() {});
  }

  void _dismiss() {
    _overlayEntry?.remove();
    _overlayEntry = null;
    if (mounted) setState(() {});
  }

  @override
  void dispose() {
    _overlayEntry?.remove();
    _overlayEntry = null;
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (widget.child != null) {
      return GestureDetector(
        key: _triggerKey,
        onTap: _show,
        child: widget.child,
      );
    }
    final hasValue = widget.selectedItem != null;
    return GestureDetector(
      key: _triggerKey,
      onTap: _show,
      child: Container(
        width: widget.width,
        height: widget.height,
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(12.r),
          border: Border.all(
            color: AppColors.textPrimary.withValues(alpha: 0.2),
          ),
        ),
        padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 8.h),
        child: Row(
          children: [
            Expanded(
              child: Text(
                widget.selectedItem ?? widget.placeholder,
                style: TextStyle(
                  fontSize: 16.sp,
                  color: hasValue
                      ? AppColors.textPrimary
                      : AppColors.textPrimary.withValues(alpha: 0.5),
                  height: 24 / 16,
                ),
                overflow: TextOverflow.ellipsis,
              ),
            ),
            SizedBox(width: 8.w),
            Icon(
              AppIcons.keyboardArrowDown,
              color: AppIconColors.defaultColor,
              size: 24.sp,
            ),
          ],
        ),
      ),
    );
  }
}

class _AppDropdownOverlay extends StatefulWidget {
  const _AppDropdownOverlay({
    required this.position,
    required this.fieldWidth,
    required this.fieldHeight,
    required this.items,
    required this.selectedItem,
    required this.showSearch,
    required this.onSelect,
    required this.onDismiss,
  });

  final Offset position;
  final double fieldWidth;
  final double fieldHeight;
  final List<String> items;
  final String? selectedItem;
  final bool showSearch;
  final ValueChanged<String> onSelect;
  final VoidCallback onDismiss;

  @override
  State<_AppDropdownOverlay> createState() => _AppDropdownOverlayState();
}

class _AppDropdownOverlayState extends State<_AppDropdownOverlay> {
  String? _hoveredItem;
  bool _isLongPressActive = false;
  final TextEditingController _searchController = TextEditingController();
  List<String> _filteredItems = [];
  final GlobalKey _listKey = GlobalKey();
  final Map<int, GlobalKey> _itemKeys = {};
  final ScrollController _scrollController = ScrollController();

  @override
  void initState() {
    super.initState();
    _filteredItems = List.from(widget.items);
    _searchController.addListener(_onSearchChanged);
  }

  @override
  void dispose() {
    _searchController.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  void _onSearchChanged() {
    final query = _searchController.text.toLowerCase();
    setState(() {
      if (query.isEmpty) {
        _filteredItems = List.from(widget.items);
      } else {
        _filteredItems = widget.items
            .where((c) => c.toLowerCase().contains(query))
            .toList();
      }
      _itemKeys.clear();
    });
  }

  /// Find the item at the given global position.
  String? _findItemAtPosition(Offset globalPosition) {
    for (int i = 0; i < _filteredItems.length; i++) {
      final key = _itemKeys[i];
      if (key?.currentContext != null) {
        final renderBox = key!.currentContext!.findRenderObject() as RenderBox?;
        if (renderBox != null) {
          final itemPosition = renderBox.localToGlobal(Offset.zero);
          final itemSize = renderBox.size;
          final itemRect = Rect.fromLTWH(
            itemPosition.dx,
            itemPosition.dy,
            itemSize.width,
            itemSize.height,
          );
          if (itemRect.contains(globalPosition)) {
            return _filteredItems[i];
          }
        }
      }
    }
    return null;
  }

  void _onLongPressStart(LongPressStartDetails details) {
    setState(() {
      _isLongPressActive = true;
      _hoveredItem = _findItemAtPosition(details.globalPosition);
    });
  }

  void _onLongPressMoveUpdate(LongPressMoveUpdateDetails details) {
    if (_isLongPressActive) {
      setState(() {
        _hoveredItem = _findItemAtPosition(details.globalPosition);
      });
    }
  }

  void _onLongPressEnd(LongPressEndDetails details) {
    if (_isLongPressActive && _hoveredItem != null) {
      widget.onSelect(_hoveredItem!);
    }
    setState(() {
      _isLongPressActive = false;
      _hoveredItem = null;
    });
  }

  @override
  Widget build(BuildContext context) {
    final screenHeight = MediaQuery.of(context).size.height;
    final maxHeight =
        (screenHeight - widget.position.dy - widget.fieldHeight - 20).clamp(
          150.0,
          350.0,
        );

    return GestureDetector(
      onTap: widget.onDismiss,
      onLongPressStart: _onLongPressStart,
      onLongPressMoveUpdate: _onLongPressMoveUpdate,
      onLongPressEnd: _onLongPressEnd,
      behavior: HitTestBehavior.opaque,
      child: Stack(
        children: [
          // Transparent background to catch taps outside
          Positioned.fill(child: Container(color: Colors.transparent)),
          // Dropdown panel
          Positioned(
            left: widget.position.dx,
            top: widget.position.dy + widget.fieldHeight + 4,
            child: Material(
              elevation: 0,
              color: Colors.transparent,
              child: Container(
                width: widget.fieldWidth,
                constraints: BoxConstraints(maxHeight: maxHeight),
                decoration: BoxDecoration(
                  color: AppColors.bgPrimary,
                  borderRadius: BorderRadius.circular(12.r),
                  border: Border.all(color: AppColors.cardBorder),
                  boxShadow: const [
                    BoxShadow(
                      color: AppColors.cardShadow,
                      blurRadius: 4,
                      offset: Offset(0, 4),
                    ),
                  ],
                ),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    // Optional search bar
                    if (widget.showSearch)
                      Padding(
                        padding: EdgeInsets.all(8.w),
                        child: Container(
                          height: 30.h,
                          decoration: BoxDecoration(
                            color: AppColors.bgCard,
                            borderRadius: BorderRadius.circular(999.r),
                            border: Border.all(color: AppColors.cardBorder),
                          ),
                          padding: EdgeInsets.symmetric(horizontal: 12.w),
                          child: Row(
                            children: [
                              Icon(
                                AppIcons.search,
                                color: AppIconColors.defaultColor,
                                size: 16.sp,
                              ),
                              SizedBox(width: 8.w),
                              Expanded(
                                child: TextField(
                                  controller: _searchController,
                                  style: TextStyle(
                                    fontSize: 14.sp,
                                    color: AppColors.textPrimary,
                                  ),
                                  decoration: InputDecoration(
                                    hintText: '搜尋',
                                    hintStyle: TextStyle(
                                      fontSize: 14.sp,
                                      color: AppColors.textSecondary,
                                    ),
                                    border: InputBorder.none,
                                    isDense: true,
                                    contentPadding: EdgeInsets.zero,
                                  ),
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    // Item list
                    Flexible(
                      child: ListView.builder(
                        key: _listKey,
                        controller: _scrollController,
                        shrinkWrap: true,
                        padding: EdgeInsets.symmetric(
                          horizontal: 8.w,
                          vertical: 4.h,
                        ),
                        itemCount: _filteredItems.length,
                        itemBuilder: (context, index) {
                          _itemKeys[index] ??= GlobalKey();
                          final item = _filteredItems[index];
                          final isHovered = _hoveredItem == item;
                          final isSelected = widget.selectedItem == item;

                          return GestureDetector(
                            onTap: () => widget.onSelect(item),
                            child: Container(
                              key: _itemKeys[index],
                              padding: EdgeInsets.symmetric(
                                vertical: 8.h,
                                horizontal: 8.w,
                              ),
                              decoration: BoxDecoration(
                                color: isHovered
                                    ? const Color(0xB38F3315)
                                    : (isSelected
                                          ? const Color(0x338F3315)
                                          : Colors.transparent),
                                borderRadius: BorderRadius.circular(999.r),
                              ),
                              child: Text(
                                item,
                                style: TextStyle(
                                  fontSize: 16.sp,
                                  color: isHovered
                                      ? AppColors.bgCard
                                      : AppColors.textPrimary,
                                  letterSpacing: 2,
                                ),
                              ),
                            ),
                          );
                        },
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
