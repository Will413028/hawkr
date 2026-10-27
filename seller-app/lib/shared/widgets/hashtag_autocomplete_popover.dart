import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../features/product/data/hashtag_repository.dart';
import 'fruit_spinner.dart';

class HashtagAutocompleteItem {
  const HashtagAutocompleteItem({
    required this.name,
    this.displayName = '',
    this.count = 0,
  });

  final String name;
  final String displayName;
  final int count;

  String get presented => displayName.isNotEmpty ? displayName : name;
}

typedef HashtagAutocompleteSearch =
    Future<List<HashtagAutocompleteItem>> Function(String prefix);
typedef HashtagCountLabelBuilder = String Function(int count);

/// rich text editor 內 hashtag autocomplete 浮動建議列表。
///
/// 設計決定：inline 出現在 TextField 下方（不用 Overlay positioning），
/// 因為當前 block 永遠是被聚焦的，suggestions 直接接在下面最直覺。
///
/// - prefix 變更時自動 debounce 200ms 後打 [HashtagRepository.search]
/// - 若使用者輸入的 prefix 不在後端結果中且 prefix 不為空 → 提供「新增 #xxx」
///   選項（issue #759 約定：未存在的 hashtag 也允許上架）
/// - 商品數 >= 50 標「熱門」icon
class HashtagAutocompletePopover extends ConsumerStatefulWidget {
  const HashtagAutocompletePopover({
    super.key,
    required this.prefix,
    required this.onSelect,
    this.search,
    this.countLabelBuilder,
    this.popularThreshold = 50,
  });

  /// `#` 後使用者打到目前的字串（不含 `#` 本身）
  final String prefix;

  /// 點選某個 hashtag 時的 callback；傳入的是 hashtag name（不含 #）
  final ValueChanged<String> onSelect;

  /// 自訂搜尋來源。未提供時沿用商品 hashtag API，保持商品描述既有行為。
  final HashtagAutocompleteSearch? search;

  /// 建議列副標，例如「12 個商品」或「12 篇貼文」。
  final HashtagCountLabelBuilder? countLabelBuilder;

  /// 超過此數量時顯示「熱門」。
  final int popularThreshold;

  @override
  ConsumerState<HashtagAutocompletePopover> createState() =>
      _HashtagAutocompletePopoverState();
}

class _HashtagAutocompletePopoverState
    extends ConsumerState<HashtagAutocompletePopover> {
  Timer? _debounce;
  List<HashtagAutocompleteItem> _items = const [];
  bool _loading = false;
  String _lastQuery = '';

  static const _debounceMs = 180;

  @override
  void initState() {
    super.initState();
    _query(immediate: true);
  }

  @override
  void didUpdateWidget(covariant HashtagAutocompletePopover old) {
    super.didUpdateWidget(old);
    if (old.prefix != widget.prefix) {
      _query();
    }
  }

  @override
  void dispose() {
    _debounce?.cancel();
    super.dispose();
  }

  void _query({bool immediate = false}) {
    _debounce?.cancel();
    final p = widget.prefix;
    if (p.isEmpty) {
      setState(() {
        _items = const [];
        _loading = false;
      });
      return;
    }
    Future<void> run() async {
      _lastQuery = p;
      if (mounted) setState(() => _loading = true);
      try {
        final items = await _search(p);
        if (!mounted || _lastQuery != p) return;
        setState(() {
          _items = items;
          _loading = false;
        });
      } catch (_) {
        if (mounted) {
          setState(() {
            _items = const [];
            _loading = false;
          });
        }
      }
    }

    if (immediate) {
      run();
    } else {
      _debounce = Timer(const Duration(milliseconds: _debounceMs), run);
    }
  }

  Future<List<HashtagAutocompleteItem>> _search(String prefix) async {
    final search = widget.search;
    if (search != null) return search(prefix);

    final repo = ref.read(hashtagRepositoryProvider);
    final items = await repo.search(prefix: prefix);
    return items
        .map(
          (item) => HashtagAutocompleteItem(
            name: item.name,
            displayName: item.displayName,
            count: item.productCount,
          ),
        )
        .toList(growable: false);
  }

  @override
  Widget build(BuildContext context) {
    final prefix = widget.prefix;
    final exactMatch = _items.any(
      (i) => i.name.toLowerCase() == prefix.toLowerCase(),
    );
    final showCreate = prefix.isNotEmpty && !exactMatch && !_loading;
    final countLabelBuilder =
        widget.countLabelBuilder ?? (count) => '$count 個商品';

    return Container(
      margin: EdgeInsets.symmetric(horizontal: 8.w, vertical: 4.h),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.dividerLight),
        boxShadow: const [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 12,
            offset: Offset(0, 4),
          ),
        ],
      ),
      constraints: BoxConstraints(maxHeight: 220.h),
      child: Material(
        color: Colors.transparent,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (_loading)
                Padding(
                  padding: EdgeInsets.all(12.w),
                  child: Center(child: FruitSpinner(size: 28.w)),
                ),
              for (final item in _items)
                _Row(
                  text: '#${item.presented}',
                  subtitle: countLabelBuilder(item.count),
                  popular: item.count >= widget.popularThreshold,
                  onTap: () => widget.onSelect(item.name),
                ),
              if (showCreate)
                _Row(
                  text: '#$prefix',
                  subtitle: '建立新 hashtag',
                  isCreate: true,
                  onTap: () => widget.onSelect(prefix),
                ),
              if (!_loading && _items.isEmpty && !showCreate)
                Padding(
                  padding: EdgeInsets.symmetric(
                    horizontal: 12.w,
                    vertical: 16.h,
                  ),
                  child: Text(
                    '輸入 # 加文字搜尋現有 hashtag',
                    style: TextStyle(
                      color: AppColors.textSecondary,
                      fontSize: 14.sp,
                    ),
                    textAlign: TextAlign.center,
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}

class _Row extends StatelessWidget {
  const _Row({
    required this.text,
    required this.onTap,
    this.subtitle,
    this.popular = false,
    this.isCreate = false,
  });

  final String text;
  final String? subtitle;
  final bool popular;
  final bool isCreate;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    // 注意：用 Listener.onPointerDown 而非 GestureDetector/InkWell。
    // popover 是 inline 在 TextField 同一個 Column 內，外層 block 包了一個
    // GestureDetector(onTap: requestFocus)，會跟 popover 的 tap 在 gesture
    // arena 競爭，常常輸給外層 → popover row 的 onTap 永遠沒被呼叫到。
    // Listener 不參與 arena，pointer 一觸下即觸發，最直接、最可靠。
    return Listener(
      behavior: HitTestBehavior.opaque,
      onPointerDown: (_) => onTap(),
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
        child: Row(
          children: [
            Icon(
              isCreate ? AppIcons.addCircleOutline : AppIcons.tag,
              size: 16.sp,
              color: AppIconColors.active,
            ),
            SizedBox(width: 8.w),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    text,
                    style: TextStyle(
                      fontSize: 14.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textEmphasis,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  if (subtitle != null)
                    Text(
                      subtitle!,
                      style: TextStyle(
                        fontSize: 12.sp,
                        color: AppColors.textSecondary,
                      ),
                    ),
                ],
              ),
            ),
            if (popular)
              Container(
                margin: EdgeInsets.only(left: 8.w),
                padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 4.h),
                decoration: BoxDecoration(
                  color: AppColors.warningSoft,
                  borderRadius: BorderRadius.circular(6.r),
                ),
                child: Text(
                  '熱門',
                  style: TextStyle(
                    fontSize: 12.sp,
                    color: AppColors.warning,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
            Icon(
              AppIcons.keyboardReturn,
              size: 14.sp,
              color: AppIconColors.disabled,
            ),
          ],
        ),
      ),
    );
  }
}
