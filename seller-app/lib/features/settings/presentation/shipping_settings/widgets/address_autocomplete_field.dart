import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../../core/theme/app_colors.dart';
import '../../../../../core/theme/app_typography.dart';
import '../../../../../core/theme/hk_tokens.dart';
import '../../../data/tgos_address_service.dart';
import '../shipping_constants.dart';

/// 地址自動完成輸入欄位
///
/// 使用者在「路名+門牌」欄位輸入時，透過 TGOS API 做 debounce 搜尋，
/// 顯示候選地址下拉清單（與縣市/區域下拉選單同風格）。
/// 選取後自動填入並標記為「已驗證」。
class AddressAutocompleteField extends ConsumerStatefulWidget {
  final TextEditingController controller;
  final String? county;
  final String? district;
  final bool enabled;
  final bool verified;
  final ValueChanged<AddressSuggestion>? onSuggestionSelected;
  final ValueChanged<String>? onManualEdit;

  const AddressAutocompleteField({
    super.key,
    required this.controller,
    this.county,
    this.district,
    this.enabled = true,
    this.verified = false,
    this.onSuggestionSelected,
    this.onManualEdit,
  });

  @override
  ConsumerState<AddressAutocompleteField> createState() =>
      _AddressAutocompleteFieldState();
}

class _AddressAutocompleteFieldState
    extends ConsumerState<AddressAutocompleteField> {
  OverlayEntry? _overlayEntry;
  Timer? _debounce;
  List<AddressSuggestion> _suggestions = [];
  bool _isLoading = false;
  bool _suppressSearch = false;
  int _requestGeneration = 0;

  @override
  void initState() {
    super.initState();
    widget.controller.addListener(_onTextChanged);
  }

  @override
  void didUpdateWidget(covariant AddressAutocompleteField oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.controller != widget.controller) {
      oldWidget.controller.removeListener(_onTextChanged);
      widget.controller.addListener(_onTextChanged);
    }
    if (oldWidget.controller != widget.controller ||
        oldWidget.county != widget.county ||
        oldWidget.district != widget.district ||
        oldWidget.enabled != widget.enabled ||
        oldWidget.verified != widget.verified) {
      _requestGeneration++;
      _debounce?.cancel();
      _removeOverlay();
      _suggestions = const [];
      _isLoading = false;
    }
  }

  @override
  void dispose() {
    widget.controller.removeListener(_onTextChanged);
    _requestGeneration++;
    _debounce?.cancel();
    _removeOverlay();
    super.dispose();
  }

  void _onTextChanged() {
    final generation = ++_requestGeneration;
    if (_suppressSearch || widget.verified) {
      _debounce?.cancel();
      _removeOverlay();
      return;
    }
    widget.onManualEdit?.call(widget.controller.text);

    _debounce?.cancel();

    // iOS 注音輸入法組字中（composing）不觸發搜尋
    if (widget.controller.value.composing.isValid) return;

    final query = widget.controller.text.trim();
    final county = widget.county;
    final district = widget.district;

    if (query.length < 2 || county == null || district == null) {
      if (_isLoading || _suggestions.isNotEmpty) {
        setState(() {
          _isLoading = false;
          _suggestions = const [];
        });
      }
      _removeOverlay();
      return;
    }

    _debounce = Timer(const Duration(milliseconds: 400), () {
      _fetchSuggestions(
        query: query,
        county: county,
        district: district,
        generation: generation,
      );
    });
  }

  Future<void> _fetchSuggestions({
    required String query,
    required String county,
    required String district,
    required int generation,
  }) async {
    if (!mounted || generation != _requestGeneration) return;
    setState(() => _isLoading = true);

    List<AddressSuggestion> results;
    try {
      results = await ref
          .read(tgosAddressServiceProvider)
          .getSuggestions(county: county, district: district, query: query);
    } catch (_) {
      results = const [];
    }

    if (!mounted ||
        generation != _requestGeneration ||
        widget.controller.text.trim() != query ||
        widget.county != county ||
        widget.district != district) {
      return;
    }
    setState(() {
      _suggestions = results;
      _isLoading = false;
    });

    if (_suggestions.isNotEmpty) {
      _showOverlay();
    } else {
      _removeOverlay();
    }
  }

  void _showOverlay() {
    _removeOverlay();

    final RenderBox box = context.findRenderObject() as RenderBox;
    final offset = box.localToGlobal(Offset.zero);
    final fieldWidth = box.size.width;

    _overlayEntry = OverlayEntry(
      builder: (ctx) {
        return Stack(
          children: [
            // 點外面關閉
            GestureDetector(
              onTap: _removeOverlay,
              child: Container(
                color: ShippingColors.transparent,
                width: double.infinity,
                height: double.infinity,
              ),
            ),
            // 下拉面板（與縣市/區域下拉選單同風格）
            Positioned(
              left: offset.dx,
              top: offset.dy + box.size.height + 4,
              width: fieldWidth,
              child: Material(
                elevation: 4,
                borderRadius: BorderRadius.circular(12.r),
                color: ShippingColors.white,
                child: ConstrainedBox(
                  constraints: BoxConstraints(maxHeight: 265.h),
                  child: _AddressSuggestionList(
                    suggestions: _suggestions,
                    onSelect: _selectSuggestion,
                  ),
                ),
              ),
            ),
          ],
        );
      },
    );
    Overlay.of(context).insert(_overlayEntry!);
  }

  void _selectSuggestion(AddressSuggestion suggestion) {
    _requestGeneration++;
    _debounce?.cancel();
    _suppressSearch = true;
    widget.controller.text = suggestion.detail;
    widget.controller.selection = TextSelection.collapsed(
      offset: suggestion.detail.length,
    );
    _suppressSearch = false;

    _removeOverlay();
    widget.onSuggestionSelected?.call(suggestion);
  }

  void _removeOverlay() {
    _overlayEntry?.remove();
    _overlayEntry = null;
  }

  @override
  Widget build(BuildContext context) {
    return ConstrainedBox(
      constraints: const BoxConstraints(minHeight: HkControl.minTap),
      child: DecoratedBox(
        decoration: BoxDecoration(
          color: AppColors.neutralWhite.withValues(alpha: 0.72),
          borderRadius: BorderRadius.circular(HkRadius.md),
          border: Border.all(
            color: widget.verified
                ? AppColors.actionPrimary
                : AppColors.cardBorder,
          ),
        ),
        child: Row(
          children: [
            Expanded(
              child: TextField(
                controller: widget.controller,
                enabled: widget.enabled,
                decoration: InputDecoration(
                  hintText: '路名 + 門牌號碼',
                  hintStyle: AppTypography.body.copyWith(
                    color: AppColors.textSecondary,
                  ),
                  contentPadding: const EdgeInsets.symmetric(
                    horizontal: HkSpace.md,
                    vertical: HkSpace.md,
                  ),
                  border: InputBorder.none,
                  isDense: true,
                ),
                style: AppTypography.body,
              ),
            ),
            if (_isLoading)
              Padding(
                padding: EdgeInsets.only(right: 8.w),
                child: SizedBox(
                  width: 16.w,
                  height: 16.w,
                  child: const CircularProgressIndicator(
                    strokeWidth: 2,
                    color: AppColors.actionPrimary,
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// 地址候選清單（與 SearchDropdown 同風格）
/// 支援輕觸選取、長按滑動選取、橘色 highlight、自動捲動
class _AddressSuggestionList extends StatefulWidget {
  final List<AddressSuggestion> suggestions;
  final ValueChanged<AddressSuggestion> onSelect;

  const _AddressSuggestionList({
    required this.suggestions,
    required this.onSelect,
  });

  @override
  State<_AddressSuggestionList> createState() => _AddressSuggestionListState();
}

class _AddressSuggestionListState extends State<_AddressSuggestionList> {
  int _highlightIndex = -1;
  bool _isDragging = false;
  final ScrollController _scrollCtrl = ScrollController();
  final double _itemHeight = 40;

  @override
  void dispose() {
    _scrollCtrl.dispose();
    super.dispose();
  }

  void _updateHighlightFromGlobal(Offset globalPos) {
    final RenderBox? box = context.findRenderObject() as RenderBox?;
    if (box == null) return;
    final localY = box.globalToLocal(globalPos).dy + _scrollCtrl.offset;
    final idx = (localY / _itemHeight.h).floor();
    final clamped = idx.clamp(0, widget.suggestions.length - 1);
    if (clamped != _highlightIndex) {
      setState(() => _highlightIndex = clamped);
    }

    // 自動捲動：手指在頂部或底部邊緣時
    final localInView = box.globalToLocal(globalPos).dy;
    final viewHeight = box.size.height;
    const edgeZone = 40.0;
    if (localInView < edgeZone && _scrollCtrl.offset > 0) {
      _scrollCtrl.jumpTo(
        (_scrollCtrl.offset - 8).clamp(0, _scrollCtrl.position.maxScrollExtent),
      );
    } else if (localInView > viewHeight - edgeZone &&
        _scrollCtrl.offset < _scrollCtrl.position.maxScrollExtent) {
      _scrollCtrl.jumpTo(
        (_scrollCtrl.offset + 8).clamp(0, _scrollCtrl.position.maxScrollExtent),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onLongPressStart: (d) {
        _isDragging = true;
        _updateHighlightFromGlobal(d.globalPosition);
      },
      onLongPressMoveUpdate: (d) {
        if (_isDragging) _updateHighlightFromGlobal(d.globalPosition);
      },
      onLongPressEnd: (_) {
        _isDragging = false;
        if (_highlightIndex >= 0 &&
            _highlightIndex < widget.suggestions.length) {
          widget.onSelect(widget.suggestions[_highlightIndex]);
        }
        setState(() => _highlightIndex = -1);
      },
      child: ListView.builder(
        controller: _scrollCtrl,
        padding: EdgeInsets.symmetric(vertical: 4.h),
        shrinkWrap: true,
        itemCount: widget.suggestions.length,
        itemExtent: _itemHeight.h,
        itemBuilder: (_, i) {
          final item = widget.suggestions[i];
          final isHighlight = _highlightIndex == i;

          return GestureDetector(
            onTap: () => widget.onSelect(item),
            child: Container(
              height: _itemHeight.h,
              padding: EdgeInsets.symmetric(horizontal: 12.w),
              color: isHighlight
                  ? ShippingColors.dropdownHighlight.withValues(alpha: 0.15)
                  : ShippingColors.transparent,
              child: Align(
                alignment: Alignment.centerLeft,
                child: Text(
                  item.detail.isNotEmpty ? item.detail : item.fullAddress,
                  style: TextStyle(
                    fontSize: 14.sp,
                    color: isHighlight
                        ? ShippingColors.dropdownHighlight
                        : ShippingColors.gray100,
                    fontWeight: FontWeight.w400,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
            ),
          );
        },
      ),
    );
  }
}
