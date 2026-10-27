import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('all active hearts use the shared bright-red semantic token', () {
    expect(AppColors.favoriteActive, const Color(0xFFFF3B4F));
    expect(AppIconColors.favorite, AppColors.favoriteActive);
  });
}
