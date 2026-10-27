import 'package:flutter/material.dart';

/// Mixin for pages that can expose their top bar to a shared container.
mixin TopBarProvider {
  PreferredSizeWidget buildTopBar(BuildContext context);
}
