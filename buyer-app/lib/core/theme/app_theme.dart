import 'package:flutter/material.dart';

import '../../shared/icons/app_icon_colors.dart';
import '../motion/hk_motion_transitions.dart';
import 'app_colors.dart';
import 'app_typography.dart';
import 'hk_tokens.dart';

/// Production buyer theme shared by the app and its component catalog.
/// Changes here are visible in both surfaces; catalog stories do not restyle it.
abstract final class AppTheme {
  static ThemeData get light => ThemeData(
    colorScheme: ColorScheme.fromSeed(seedColor: AppColors.brandPrimaryStrong)
        .copyWith(
          primary: AppColors.actionPrimary,
          onPrimary: AppColors.textInverse,
          primaryContainer: AppColors.primarySoft,
          onPrimaryContainer: AppColors.brandPrimaryStrong,
          secondary: AppColors.secondary,
          onSecondary: AppColors.textInverse,
          secondaryContainer: AppColors.surfaceMuted,
          onSecondaryContainer: AppColors.brandPrimaryStrong,
          tertiary: AppColors.brandSecondary,
          onTertiary: AppColors.brandInk,
          tertiaryContainer: AppColors.warningSoft,
          onTertiaryContainer: AppColors.brandInk,
          surface: AppColors.bgCard,
          onSurface: AppColors.gray900,
          surfaceContainerLowest: AppColors.neutralWhite,
          surfaceContainerLow: AppColors.neutralCream,
          surfaceContainer: AppColors.neutralMuted,
          surfaceContainerHigh: AppColors.neutralPanel,
          surfaceContainerHighest: AppColors.brandSecondarySoft,
          error: AppColors.danger,
          onError: AppColors.textInverse,
          outline: AppColors.borderStrong,
          outlineVariant: AppColors.cardBorder,
          shadow: AppColors.neutralInk,
          scrim: AppColors.bgOverlay,
          inverseSurface: AppColors.neutralInk,
          onInverseSurface: AppColors.neutralCream,
          inversePrimary: AppColors.brandSecondary,
          surfaceTint: AppColors.brandPrimaryStrong,
        ),
    useMaterial3: true,
    pageTransitionsTheme: HkPageTransitions.theme,
    // Keep button semantics and custom motion, but remove Flutter's
    // built-in ripple / pressed overlay from every Material control.
    splashFactory: NoSplash.splashFactory,
    splashColor: Colors.transparent,
    highlightColor: Colors.transparent,
    hoverColor: Colors.transparent,
    focusColor: Colors.transparent,
    materialTapTargetSize: MaterialTapTargetSize.padded,
    visualDensity: VisualDensity.standard,
    scaffoldBackgroundColor: AppColors.bgPrimary,
    textTheme: AppTypography.textTheme,
    progressIndicatorTheme: const ProgressIndicatorThemeData(
      color: AppColors.primary,
    ),
    iconTheme: const IconThemeData(color: AppIconColors.defaultColor, size: 24),
    navigationBarTheme: const NavigationBarThemeData(
      backgroundColor: AppColors.navBarBackground,
      indicatorColor: AppColors.primarySoft,
      height: HkControl.navHeight,
    ),
    iconButtonTheme: IconButtonThemeData(
      style: IconButton.styleFrom(
        foregroundColor: AppColors.textPrimary,
        overlayColor: Colors.transparent,
        minimumSize: const Size.square(HkControl.minTap),
        iconSize: HkControl.icon,
      ),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        minimumSize: const Size.square(HkControl.minTap),
        backgroundColor: AppColors.actionPrimary,
        foregroundColor: AppColors.textInverse,
        overlayColor: Colors.transparent,
        disabledBackgroundColor: AppColors.gray100,
        disabledForegroundColor: AppColors.chromeMuted,
        textStyle: AppTypography.buttonMedium,
        padding: const EdgeInsets.symmetric(horizontal: HkSpace.xl),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(HkRadius.md),
        ),
      ),
    ),
    elevatedButtonTheme: ElevatedButtonThemeData(
      style: ElevatedButton.styleFrom(
        minimumSize: const Size.square(HkControl.minTap),
        backgroundColor: AppColors.actionPrimary,
        foregroundColor: AppColors.textInverse,
        overlayColor: Colors.transparent,
        elevation: 0,
        textStyle: AppTypography.buttonMedium,
        padding: const EdgeInsets.symmetric(horizontal: HkSpace.xl),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(HkRadius.md),
        ),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        minimumSize: const Size.square(HkControl.minTap),
        foregroundColor: AppColors.actionPrimary,
        overlayColor: Colors.transparent,
        side: const BorderSide(color: AppColors.borderStrong),
        textStyle: AppTypography.buttonMedium,
        padding: const EdgeInsets.symmetric(horizontal: HkSpace.xl),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(HkRadius.md),
        ),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: TextButton.styleFrom(
        minimumSize: const Size.square(HkControl.minTap),
        foregroundColor: AppColors.actionPrimary,
        overlayColor: Colors.transparent,
        textStyle: AppTypography.buttonMedium,
        padding: const EdgeInsets.symmetric(horizontal: HkSpace.lg),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(HkRadius.md),
        ),
      ),
    ),
    inputDecorationTheme: InputDecorationThemeData(
      filled: true,
      fillColor: AppColors.bgCard,
      hintStyle: AppTypography.body.copyWith(color: AppColors.textHint),
      labelStyle: AppTypography.body.copyWith(color: AppColors.textPrimary),
      contentPadding: const EdgeInsets.symmetric(
        horizontal: HkSpace.lg,
        vertical: HkSpace.md,
      ),
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(HkRadius.md),
        borderSide: const BorderSide(color: AppColors.borderInput),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(HkRadius.md),
        borderSide: const BorderSide(color: AppColors.borderInput),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(HkRadius.md),
        borderSide: const BorderSide(
          color: AppColors.actionPrimary,
          width: 1.5,
        ),
      ),
    ),
    bottomNavigationBarTheme: const BottomNavigationBarThemeData(
      backgroundColor: AppColors.navBarBackground,
      showSelectedLabels: false,
      showUnselectedLabels: false,
    ),
    appBarTheme: const AppBarThemeData(
      backgroundColor: AppColors.topBarBackground,
      foregroundColor: AppColors.chromeForeground,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      systemOverlayStyle: AppColors.topBarOverlay,
    ),
    cardTheme: const CardThemeData(
      color: AppColors.bgCard,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(
        side: BorderSide(color: AppColors.cardBorder),
        borderRadius: BorderRadius.all(Radius.circular(HkRadius.lg)),
      ),
    ),
    dialogTheme: const DialogThemeData(
      backgroundColor: AppColors.bgElevated,
      surfaceTintColor: Colors.transparent,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.all(Radius.circular(HkRadius.dialog)),
      ),
    ),
    bottomSheetTheme: const BottomSheetThemeData(
      backgroundColor: AppColors.bgElevated,
      surfaceTintColor: Colors.transparent,
      showDragHandle: true,
      dragHandleColor: AppColors.dragHandle,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(HkRadius.xl)),
      ),
    ),
    chipTheme: const ChipThemeData(
      backgroundColor: AppColors.surfacePanel,
      selectedColor: AppColors.primarySoft,
      disabledColor: AppColors.gray100,
      side: BorderSide(color: AppColors.cardBorder),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.all(Radius.circular(HkRadius.sm)),
      ),
      padding: EdgeInsets.symmetric(horizontal: HkSpace.sm),
      labelStyle: TextStyle(
        color: AppColors.textPrimary,
        fontSize: 14,
        fontWeight: FontWeight.w500,
      ),
    ),
    tabBarTheme: const TabBarThemeData(
      labelColor: AppColors.actionPrimary,
      unselectedLabelColor: AppColors.textSecondary,
      labelStyle: TextStyle(fontSize: 14, fontWeight: FontWeight.w700),
      unselectedLabelStyle: TextStyle(
        fontSize: 14,
        fontWeight: FontWeight.w500,
      ),
      indicatorColor: AppColors.actionPrimary,
      dividerColor: Colors.transparent,
    ),
    listTileTheme: const ListTileThemeData(
      textColor: AppColors.textPrimary,
      iconColor: AppColors.primary,
      minVerticalPadding: HkSpace.sm,
      contentPadding: EdgeInsets.symmetric(horizontal: HkSpace.lg),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.all(Radius.circular(HkRadius.md)),
      ),
    ),
    checkboxTheme: CheckboxThemeData(
      fillColor: WidgetStateProperty.resolveWith(
        (states) => states.contains(WidgetState.selected)
            ? AppColors.actionPrimary
            : Colors.transparent,
      ),
      checkColor: const WidgetStatePropertyAll(AppColors.textInverse),
      side: const BorderSide(color: AppColors.borderStrong),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(HkRadius.xs),
      ),
    ),
    radioTheme: RadioThemeData(
      fillColor: WidgetStateProperty.resolveWith(
        (states) => states.contains(WidgetState.selected)
            ? AppColors.actionPrimary
            : AppColors.textSecondary,
      ),
    ),
    switchTheme: SwitchThemeData(
      thumbColor: WidgetStateProperty.resolveWith(
        (states) => states.contains(WidgetState.selected)
            ? AppColors.textInverse
            : AppColors.bgCard,
      ),
      trackColor: WidgetStateProperty.resolveWith(
        (states) => states.contains(WidgetState.selected)
            ? AppColors.actionPrimary
            : AppColors.gray300,
      ),
    ),
    floatingActionButtonTheme: const FloatingActionButtonThemeData(
      backgroundColor: AppColors.actionPrimary,
      foregroundColor: AppColors.textInverse,
      elevation: 2,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.all(Radius.circular(HkRadius.lg)),
      ),
    ),
    bottomAppBarTheme: const BottomAppBarThemeData(
      color: AppColors.navBarBackground,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      padding: EdgeInsets.symmetric(horizontal: HkSpace.sm),
    ),
    dividerTheme: const DividerThemeData(
      color: AppColors.dividerLight,
      thickness: 1,
    ),
    snackBarTheme: const SnackBarThemeData(
      backgroundColor: AppColors.gray900,
      contentTextStyle: TextStyle(color: AppColors.textInverse),
      behavior: SnackBarBehavior.floating,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.all(Radius.circular(HkRadius.md)),
      ),
    ),
  );
}
