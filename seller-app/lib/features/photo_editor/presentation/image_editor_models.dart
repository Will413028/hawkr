import 'dart:typed_data';

import 'package:flutter/material.dart';

import 'image_editor_sticker_badge.dart';

/// 編輯歷史記錄
class EditHistory {
  final String action;
  final double brightness;
  final double contrast;
  final double saturation;
  final double temperature;
  final double blur;
  final List<TextLayer> textLayers;
  final List<StickerLayer> stickerLayers;
  final Uint8List? imageBytes; // 圖片緩存（用於 AI 編輯後的 undo）
  final int imageVersion; // 圖片版本號
  final Matrix4? transformMatrix; // 整張圖片的縮放/平移變換矩陣

  EditHistory({
    required this.action,
    required this.brightness,
    required this.contrast,
    required this.saturation,
    required this.temperature,
    required this.blur,
    required this.textLayers,
    required this.stickerLayers,
    this.imageBytes,
    this.imageVersion = 0,
    this.transformMatrix,
  });
}

/// 文字圖層
class TextLayer {
  final String text;
  final double x;
  final double y;
  final double fontSize;
  final Color color;
  final bool isBold;
  final bool isItalic;
  final bool isVisible;
  final TextAlign textAlign;
  final String fontFamily;
  final double rotation; // 旋轉角度（弧度）
  final double scale; // 縮放比例

  TextLayer({
    required this.text,
    this.x = 50,
    this.y = 50,
    this.fontSize = 24,
    this.color = Colors.black,
    this.isBold = false,
    this.isItalic = false,
    this.isVisible = true,
    this.textAlign = TextAlign.left,
    this.fontFamily = 'notoSansTC',
    this.rotation = 0,
    this.scale = 1.0,
  });

  TextLayer copyWith({
    String? text,
    double? x,
    double? y,
    double? fontSize,
    Color? color,
    bool? isBold,
    bool? isItalic,
    bool? isVisible,
    TextAlign? textAlign,
    String? fontFamily,
    double? rotation,
    double? scale,
  }) {
    return TextLayer(
      text: text ?? this.text,
      x: x ?? this.x,
      y: y ?? this.y,
      fontSize: fontSize ?? this.fontSize,
      color: color ?? this.color,
      isBold: isBold ?? this.isBold,
      isItalic: isItalic ?? this.isItalic,
      isVisible: isVisible ?? this.isVisible,
      textAlign: textAlign ?? this.textAlign,
      fontFamily: fontFamily ?? this.fontFamily,
      rotation: rotation ?? this.rotation,
      scale: scale ?? this.scale,
    );
  }
}

/// 貼圖圖層
class StickerLayer {
  final StickerBadge? badge;
  final String? emoji;
  final double x;
  final double y;
  final double size;
  final double rotation;
  final double scale; // 縮放比例
  final bool isVisible;

  StickerLayer({
    this.badge,
    this.emoji,
    this.x = 50,
    this.y = 50,
    this.size = 60,
    this.rotation = 0,
    this.scale = 1.0,
    this.isVisible = true,
  });

  bool get isBadgeSticker => badge != null;

  StickerLayer copyWith({
    StickerBadge? badge,
    String? emoji,
    double? x,
    double? y,
    double? size,
    double? rotation,
    double? scale,
    bool? isVisible,
  }) {
    return StickerLayer(
      badge: badge ?? this.badge,
      emoji: emoji ?? this.emoji,
      x: x ?? this.x,
      y: y ?? this.y,
      size: size ?? this.size,
      rotation: rotation ?? this.rotation,
      scale: scale ?? this.scale,
      isVisible: isVisible ?? this.isVisible,
    );
  }
}
