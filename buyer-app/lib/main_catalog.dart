import 'package:flutter/material.dart';

import 'catalog/component_catalog.dart';

/// Isolated UI entrypoint: no Firebase, account, API or analytics bootstrap.
/// `?component=buttons` includes the shared circular frosted composer/menu actions.
/// `?component=cart&textSize=large` previews the existing Small/Standard/Large
/// text policy with real cart controls, two-store selection, unavailable rows,
/// a right-aligned seller-store action, quantity beside the product thumbnail,
/// and a store-specific checkout summary. Cart title/body/support/total roles
/// use 22/16/14/24 logical px; quantity uses the 16px body role. No transactions.
void main() {
  WidgetsFlutterBinding.ensureInitialized();
  runApp(BuyerComponentCatalogApp(initialUri: Uri.base));
}
