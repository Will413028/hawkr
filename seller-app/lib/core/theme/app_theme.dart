import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../shared/icons/app_icon_colors.dart';
import '../motion/hk_motion_transitions.dart';
import 'app_colors.dart';
import 'app_typography.dart';
import 'hk_tokens.dart';

/// The production theme shared by the seller app and its component catalog.
/// Keep component defaults here so catalog previews reflect shipped styling.
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
          surface: AppColors.surfaceCard,
          onSurface: AppColors.textDark,
          surfaceContainerLowest: AppColors.neutralWhite,
          surfaceContainerLow: AppColors.neutralCream,
          surfaceContainer: AppColors.neutralMuted,
          surfaceContainerHigh: AppColors.neutralPanel,
          surfaceContainerHighest: AppColors.brandSecondarySoft,
          error: AppColors.error,
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
    materialTapTargetSize: MaterialTapTargetSize.padded,
    visualDensity: VisualDensity.standard,
    splashFactory: NoSplash.splashFactory,
    highlightColor: AppColors.primaryHover,
    hoverColor: AppColors.primaryHover,
    focusColor: AppColors.primarySoft,
    dividerColor: AppColors.dividerLight,
    textTheme: AppTypography.textTheme,
    scaffoldBackgroundColor: AppColors.bgApp,
    progressIndicatorTheme: const ProgressIndicatorThemeData(
      color: AppColors.primary,
    ),
    iconTheme: const IconThemeData(color: AppIconColors.defaultColor, size: 24),
    snackBarTheme: const SnackBarThemeData(
      backgroundColor: AppColors.textDark,
      contentTextStyle: TextStyle(color: AppColors.white100),
      behavior: SnackBarBehavior.floating,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.all(Radius.circular(HkRadius.md)),
      ),
    ),
    navigationBarTheme: const NavigationBarThemeData(
      backgroundColor: AppColors.navBarBackground,
      indicatorColor: AppColors.primarySoft,
      height: HkControl.navHeight,
    ),
    iconButtonTheme: IconButtonThemeData(
      style: IconButton.styleFrom(
        foregroundColor: AppColors.textPrimary,
        minimumSize: const Size.square(HkControl.minTap),
        iconSize: HkControl.icon,
      ),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        minimumSize: const Size.square(HkControl.minTap),
        backgroundColor: AppColors.actionPrimary,
        foregroundColor: AppColors.white100,
        disabledBackgroundColor: AppColors.neutralWhisper,
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
        foregroundColor: AppColors.white100,
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
        side: const BorderSide(color: AppColors.cardBorder),
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
        textStyle: AppTypography.buttonMedium,
        padding: const EdgeInsets.symmetric(horizontal: HkSpace.lg),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(HkRadius.md),
        ),
      ),
    ),
    inputDecorationTheme: InputDecorationThemeData(
      filled: true,
      fillColor: AppColors.surfaceGlass,
      hintStyle: AppTypography.body.copyWith(color: AppColors.textSecondary),
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
      backgroundColor: AppColors.topBarGlass,
      foregroundColor: AppColors.chromeForeground,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      systemOverlayStyle: SystemUiOverlayStyle(
        statusBarColor: Colors.transparent,
        statusBarIconBrightness: Brightness.dark,
        statusBarBrightness: Brightness.light,
      ),
    ),
    cardTheme: const CardThemeData(
      color: AppColors.surfaceCard,
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
      dragHandleColor: AppColors.cardBorder,
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
      checkColor: const WidgetStatePropertyAll(AppColors.white100),
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
            ? AppColors.white100
            : AppColors.surfaceCard,
      ),
      trackColor: WidgetStateProperty.resolveWith(
        (states) => states.contains(WidgetState.selected)
            ? AppColors.actionPrimary
            : AppColors.gray300,
      ),
    ),
    floatingActionButtonTheme: const FloatingActionButtonThemeData(
      backgroundColor: AppColors.actionPrimary,
      foregroundColor: AppColors.white100,
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
  );
}
