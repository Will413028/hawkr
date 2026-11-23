import 'package:hawkr_seller/l10n/l10n.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../core/l10n/ui_locale.dart';
import '../../../../shared/widgets/hk_button_label.dart';
import '../../../customer/data/customer_model.dart';
import '../../../customer/data/customer_repository.dart';

/// 從店家的客戶名單（`CrmAppService.ListCustomers`）挑發券對象。已在名單裡的客戶（[excludedIds]）不再出現。
/// 回傳這次勾選的客戶；關閉不選回 null。
Future<List<Customer>?> showCouponCustomerPicker(
  BuildContext context, {
  Set<String> excludedIds = const {},
}) {
  return showModalBottomSheet<List<Customer>>(
    context: context,
    isScrollControlled: true,
    useRootNavigator: true,
    backgroundColor: AppColors.bgCard,
    builder: (_) => FractionallySizedBox(
      heightFactor: 0.85,
      child: _CustomerPickerSheet(excludedIds: excludedIds),
    ),
  );
}

class _CustomerPickerSheet extends ConsumerStatefulWidget {
  const _CustomerPickerSheet({required this.excludedIds});

  final Set<String> excludedIds;

  @override
  ConsumerState<_CustomerPickerSheet> createState() =>
      _CustomerPickerSheetState();
}

class _CustomerPickerSheetState extends ConsumerState<_CustomerPickerSheet> {
  final _customers = <Customer>[];
  final _selected = <String, Customer>{};
  String _nextToken = '';
  bool _loading = false;
  bool _loadedOnce = false;
  Object? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    if (_loading) return;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final page = await ref
          .read(customerRepositoryProvider)
          .listCustomers(pageToken: _nextToken);
      if (!mounted) return;
      setState(() {
        _customers.addAll(
          page.customers.where((c) => !widget.excludedIds.contains(c.id)),
        );
        _nextToken = page.nextPageToken;
        _loadedOnce = true;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e;
        _loading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final error = _error;
    return SafeArea(
      child: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(HkSpace.lg),
            child: Text(
              context.l10n.customerPickerTitle,
              style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w700),
            ),
          ),
          Expanded(
            child: ListView(
              children: [
                if (_loadedOnce && _customers.isEmpty && error == null)
                  Padding(
                    key: const ValueKey('coupon-picker-empty'),
                    padding: const EdgeInsets.all(HkSpace.xl),
                    child: Center(
                      child: Text(context.l10n.customerPickerEmpty),
                    ),
                  ),
                for (final c in _customers)
                  CheckboxListTile(
                    key: ValueKey('coupon-picker-${c.id}'),
                    value: _selected.containsKey(c.id),
                    title: Text(c.displayName(context.uiLocale, context.l10n)),
                    subtitle: Text('ID ${c.id}'),
                    onChanged: (v) => setState(() {
                      if (v == true) {
                        _selected[c.id] = c;
                      } else {
                        _selected.remove(c.id);
                      }
                    }),
                  ),
                if (error != null)
                  Padding(
                    key: const ValueKey('coupon-picker-error'),
                    padding: const EdgeInsets.all(HkSpace.lg),
                    child: Column(
                      children: [
                        Text(
                          errorText(
                            error,
                            defaults: ref.read(defaultErrorTextProvider),
                          ),
                          textAlign: TextAlign.center,
                        ),
                        TextButton(
                          onPressed: _load,
                          child: HkButtonLabel(context.l10n.commonRetry),
                        ),
                      ],
                    ),
                  )
                else if (_loading)
                  const Padding(
                    padding: EdgeInsets.all(HkSpace.lg),
                    child: Center(child: CircularProgressIndicator()),
                  )
                else if (_nextToken.isNotEmpty)
                  TextButton(
                    key: const ValueKey('coupon-picker-more'),
                    onPressed: _load,
                    child: HkButtonLabel(context.l10n.commonLoadMore),
                  ),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(HkSpace.lg),
            child: SizedBox(
              width: double.infinity,
              child: FilledButton(
                onPressed: _selected.isEmpty
                    ? null
                    : () => Navigator.of(
                        context,
                      ).pop(_selected.values.toList(growable: false)),
                child: HkButtonLabel(
                  context.l10n.customerPickerAdd(_selected.length),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
