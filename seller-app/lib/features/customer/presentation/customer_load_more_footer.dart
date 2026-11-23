import 'package:hawkr_seller/l10n/l10n.dart';
import 'package:flutter/material.dart';

import '../../../core/theme/hk_tokens.dart';
import '../../../shared/widgets/hk_button_label.dart';

/// 列表底部的「載入下一頁」：出現時自動載入；失敗時顯示錯誤與重試（不自動重試）。
class CustomerLoadMoreFooter extends StatefulWidget {
  const CustomerLoadMoreFooter({
    super.key,
    required this.onLoadMore,
    this.errorMessage,
    this.errorKey,
  });

  final Future<void> Function() onLoadMore;
  final String? errorMessage;
  final Key? errorKey;

  @override
  State<CustomerLoadMoreFooter> createState() => _CustomerLoadMoreFooterState();
}

class _CustomerLoadMoreFooterState extends State<CustomerLoadMoreFooter> {
  @override
  void initState() {
    super.initState();
    if (widget.errorMessage == null) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) widget.onLoadMore();
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final error = widget.errorMessage;
    if (error != null) {
      return Padding(
        key: widget.errorKey,
        padding: const EdgeInsets.symmetric(vertical: HkSpace.md),
        child: Column(
          children: [
            Text(error, textAlign: TextAlign.center),
            TextButton(
              onPressed: widget.onLoadMore,
              child: HkButtonLabel(context.l10n.commonRetry),
            ),
          ],
        ),
      );
    }
    return const Padding(
      padding: EdgeInsets.symmetric(vertical: HkSpace.md),
      child: Center(child: CircularProgressIndicator()),
    );
  }
}
