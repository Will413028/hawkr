import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';

/// A reusable dropdown widget that opens an overlay below the trigger.
///
/// Supports:
/// - Default trigger button style (white background, rounded border)
/// - Custom trigger via [child] parameter
/// - Typed items: [items] are values of [T], displayed through [itemLabel];
///   [onSelected] receives the value, never its label. [T] may be nullable, so
///   a "none" entry is an ordinary item whose value is null
/// - Optional search bar ([showSearch])
/// - Long press drag to highlight and select items
/// - Selected and hovered states derive from the active Material color scheme
class AppDropdown<T> extends StatefulWidget {
  const AppDropdown({
    super.key,
    required this.items,
    required this.itemLabel,
    required this.onSelected,
    this.selectedItem,
    this.placeholder = '選擇',
    this.width,
    this.height,
    this.showSearch = false,
    this.enabled = true,
    this.semanticLabel,
    this.child,
  });

  final List<T> items;
  final String Function(T item) itemLabel;
  final ValueChanged<T> onSelected;

  /// The current value. The trigger shows [placeholder] when it is null and
  /// its label otherwise; the menu marks the entry equal to it as selected, so
  /// a value that is not in [items] is still shown but marks nothing.
  final T? selectedItem;
  final String placeholder;
  final double? width;
  final double? height;
  final bool showSearch;
  final bool enabled;
  final String? semanticLabel;

  /// Custom trigger. If provided, this widget is wrapped with a GestureDetector
  /// that opens/closes the dropdown. If null, the default trigger style is used.
  final Widget? child;

  @override
  State<AppDropdown<T>> createState() => _AppDropdownState<T>();
}

class _AppDropdownState<T> extends State<AppDropdown<T>> {
  final _triggerKey = GlobalKey();
  OverlayEntry? _overlayEntry;

  bool get _isOpen => _overlayEntry != null;

  int? get _selectedIndex {
    final selected = widget.selectedItem;
    final index = widget.items.indexWhere((e) => e == selected);
    return index < 0 ? null : index;
  }

  String get _triggerText {
    final selected = widget.selectedItem;
    return selected == null ? widget.placeholder : widget.itemLabel(selected);
  }

  void _show() {
    if (!widget.enabled) return;
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
        labels: [for (final item in widget.items) widget.itemLabel(item)],
        selectedIndex: _selectedIndex,
        showSearch: widget.showSearch,
        onSelect: (index) {
          widget.onSelected(widget.items[index]);
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
    final scheme = Theme.of(context).colorScheme;
    Widget trigger(Widget child, String fallbackLabel) {
      return Semantics(
        button: true,
        enabled: widget.enabled,
        label: widget.semanticLabel ?? fallbackLabel,
        onTap: widget.enabled ? _show : null,
        child: ExcludeSemantics(
          child: GestureDetector(
            key: _triggerKey,
            behavior: HitTestBehavior.opaque,
            onTap: widget.enabled ? _show : null,
            child: child,
          ),
        ),
      );
    }

    if (widget.child != null) {
      return trigger(widget.child!, '$_triggerText，下拉選單');
    }
    final hasValue = widget.selectedItem != null;
    return trigger(
      Container(
        width: widget.width,
        height: widget.height,
        decoration: BoxDecoration(
          color: scheme.surface,
          borderRadius: BorderRadius.circular(HkRadius.md),
          border: Border.all(color: scheme.outlineVariant),
        ),
        padding: const EdgeInsets.symmetric(
          horizontal: HkSpace.md,
          vertical: HkSpace.sm,
        ),
        child: Row(
          children: [
            Expanded(
              child: Text(
                _triggerText,
                style: AppTypography.body.copyWith(
                  color: hasValue ? scheme.onSurface : scheme.onSurfaceVariant,
                ),
                overflow: TextOverflow.ellipsis,
              ),
            ),
            const SizedBox(width: HkSpace.sm),
            Icon(
              AppIcons.keyboardArrowDown,
              color: AppIconColors.defaultColor,
              size: HkControl.icon,
            ),
          ],
        ),
      ),
      '$_triggerText，下拉選單',
    );
  }
}

class _AppDropdownOverlay extends StatefulWidget {
  const _AppDropdownOverlay({
    required this.position,
    required this.fieldWidth,
    required this.fieldHeight,
    required this.labels,
    required this.selectedIndex,
    required this.showSearch,
    required this.onSelect,
    required this.onDismiss,
  });

  final Offset position;
  final double fieldWidth;
  final double fieldHeight;
  final List<String> labels;
  final int? selectedIndex;
  final bool showSearch;
  final ValueChanged<int> onSelect;
  final VoidCallback onDismiss;

  @override
  State<_AppDropdownOverlay> createState() => _AppDropdownOverlayState();
}

class _AppDropdownOverlayState extends State<_AppDropdownOverlay> {
  int? _hoveredIndex;
  bool _isLongPressActive = false;
  final TextEditingController _searchController = TextEditingController();
  List<int> _filtered = [];
  final GlobalKey _listKey = GlobalKey();
  final Map<int, GlobalKey> _itemKeys = {};
  final ScrollController _scrollController = ScrollController();

  @override
  void initState() {
    super.initState();
    _filtered = List.generate(widget.labels.length, (i) => i);
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
      _filtered = [
        for (var i = 0; i < widget.labels.length; i++)
          if (query.isEmpty || widget.labels[i].toLowerCase().contains(query))
            i,
      ];
      _itemKeys.clear();
    });
  }

  /// Find the index (into the unfiltered items) at the given global position.
  int? _findItemAtPosition(Offset globalPosition) {
    for (int i = 0; i < _filtered.length; i++) {
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
            return _filtered[i];
          }
        }
      }
    }
    return null;
  }

  void _onLongPressStart(LongPressStartDetails details) {
    setState(() {
      _isLongPressActive = true;
      _hoveredIndex = _findItemAtPosition(details.globalPosition);
    });
  }

  void _onLongPressMoveUpdate(LongPressMoveUpdateDetails details) {
    if (_isLongPressActive) {
      setState(() {
        _hoveredIndex = _findItemAtPosition(details.globalPosition);
      });
    }
  }

  void _onLongPressEnd(LongPressEndDetails details) {
    final hovered = _hoveredIndex;
    if (_isLongPressActive && hovered != null) {
      widget.onSelect(hovered);
    }
    setState(() {
      _isLongPressActive = false;
      _hoveredIndex = null;
    });
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
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
                  color: scheme.surface,
                  borderRadius: BorderRadius.circular(HkRadius.md),
                  border: Border.all(color: scheme.outlineVariant),
                  boxShadow: HkShadows.sm,
                ),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    // Optional search bar
                    if (widget.showSearch)
                      Padding(
                        padding: const EdgeInsets.all(HkSpace.sm),
                        child: _DropdownSearchField(
                          controller: _searchController,
                        ),
                      ),
                    // Item list
                    Flexible(
                      child: ListView.builder(
                        key: _listKey,
                        controller: _scrollController,
                        shrinkWrap: true,
                        padding: const EdgeInsets.symmetric(
                          horizontal: HkSpace.sm,
                          vertical: HkSpace.xs,
                        ),
                        itemCount: _filtered.length,
                        itemBuilder: (context, index) {
                          _itemKeys[index] ??= GlobalKey();
                          final itemIndex = _filtered[index];
                          final label = widget.labels[itemIndex];
                          final isHovered = _hoveredIndex == itemIndex;
                          final isSelected = widget.selectedIndex == itemIndex;

                          return Semantics(
                            key: ValueKey('app-dropdown-item-$itemIndex'),
                            selected: isSelected,
                            button: true,
                            child: GestureDetector(
                              onTap: () => widget.onSelect(itemIndex),
                              child: Container(
                                key: _itemKeys[index],
                                padding: const EdgeInsets.symmetric(
                                  vertical: HkSpace.sm,
                                  horizontal: HkSpace.sm,
                                ),
                                decoration: BoxDecoration(
                                  color: isHovered
                                      ? scheme.primary
                                      : (isSelected
                                            ? scheme.primaryContainer
                                            : Colors.transparent),
                                  borderRadius: BorderRadius.circular(
                                    HkRadius.pill,
                                  ),
                                ),
                                child: Text(
                                  label,
                                  style: AppTypography.body.copyWith(
                                    color: isHovered
                                        ? scheme.onPrimary
                                        : scheme.onSurface,
                                  ),
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

/// 下拉選單內的搜尋控制項自行繪製單一表面與焦點狀態。
///
/// 這裡不繼承全域 [InputDecorationTheme] 的邊框，避免外層表面與
/// [TextField] 的 focused border 疊成雙層輪廓。
class _DropdownSearchField extends StatefulWidget {
  const _DropdownSearchField({required this.controller});

  final TextEditingController controller;

  @override
  State<_DropdownSearchField> createState() => _DropdownSearchFieldState();
}

class _DropdownSearchFieldState extends State<_DropdownSearchField> {
  final FocusNode _focusNode = FocusNode();

  @override
  void dispose() {
    _focusNode.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return AnimatedBuilder(
      animation: _focusNode,
      builder: (context, child) {
        final focused = _focusNode.hasFocus;
        return AnimatedContainer(
          key: const ValueKey('app-dropdown-search-surface'),
          duration: const Duration(milliseconds: 160),
          curve: Curves.easeOutCubic,
          height: HkControl.searchFieldHeight,
          clipBehavior: Clip.antiAlias,
          decoration: BoxDecoration(
            color: scheme.surfaceContainerLowest,
            borderRadius: BorderRadius.circular(HkRadius.md),
            border: Border.all(
              color: focused ? scheme.primary : scheme.outlineVariant,
              width: focused ? 1.5 : 1,
            ),
            boxShadow: focused
                ? [
                    BoxShadow(
                      color: scheme.primary.withValues(alpha: 0.12),
                      spreadRadius: 3,
                    ),
                  ]
                : null,
          ),
          child: child,
        );
      },
      child: TextField(
        key: const ValueKey('app-dropdown-search-field'),
        controller: widget.controller,
        focusNode: _focusNode,
        textInputAction: TextInputAction.search,
        autocorrect: false,
        cursorColor: scheme.primary,
        style: AppTypography.body.copyWith(color: scheme.onSurface),
        textAlignVertical: TextAlignVertical.center,
        decoration: InputDecoration(
          filled: false,
          border: InputBorder.none,
          enabledBorder: InputBorder.none,
          focusedBorder: InputBorder.none,
          disabledBorder: InputBorder.none,
          errorBorder: InputBorder.none,
          focusedErrorBorder: InputBorder.none,
          hintText: '搜尋',
          hintStyle: AppTypography.body.copyWith(
            color: scheme.onSurfaceVariant,
          ),
          prefixIcon: Icon(
            AppIcons.search,
            color: scheme.onSurfaceVariant,
            size: 20,
          ),
          prefixIconConstraints: const BoxConstraints.tightFor(
            width: HkControl.searchFieldHeight,
            height: HkControl.searchFieldHeight,
          ),
          isDense: true,
          contentPadding: const EdgeInsets.only(right: HkSpace.md),
        ),
      ),
    );
  }
}
