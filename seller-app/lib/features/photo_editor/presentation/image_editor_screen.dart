import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'dart:ui' as ui;

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import 'package:palette_generator_plus/palette_generator_plus.dart';

import '../../../shared/utils/floating_toast.dart';
import '../../../shared/utils/platform_io.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/ai_image_service.dart';
import '../data/ai_style_model.dart';
import '../data/ai_edit_history_model.dart';
import 'ai_edit_history_controller.dart';
import 'image_editor_adjust_tab.dart';
import 'image_editor_ai_tab.dart';
import 'image_editor_models.dart';
import 'image_editor_sticker_badge.dart';
import 'image_editor_sticker_tab.dart';
import 'image_editor_text_tab.dart';

/// 圖片編輯器主頁面
class ImageEditorScreen extends ConsumerStatefulWidget {
  const ImageEditorScreen({
    super.key,
    this.initialImage,
    this.category = 'product',
  });

  final File? initialImage;

  /// 'product' | 'post' — 決定 AI 分頁載入哪組預設風格清單
  final String category;

  @override
  ConsumerState<ImageEditorScreen> createState() => _ImageEditorScreenState();
}

class _ImageEditorScreenState extends ConsumerState<ImageEditorScreen> {
  File? _currentImage;
  Uint8List? _imageBytes;
  int _imageVersion = 0;
  Color _dominantColor = AppColors.bgPrimary;

  Future<void> _updateDominantColor() async {
    if (_imageBytes == null) return;
    await Future.delayed(const Duration(milliseconds: 100));
    if (!mounted || _imageBytes == null) return;
    try {
      final provider = MemoryImage(_imageBytes!);
      final palette = await PaletteGenerator.fromImageProvider(
        provider,
        size: const Size(50, 50),
        maximumColorCount: 4,
        quantizer: const LegacyQuantizer(),
      );
      if (mounted) {
        setState(
          () => _dominantColor =
              palette.dominantColor?.color ??
              palette.colors.firstOrNull ??
              AppColors.bgPrimary,
        );
      }
    } catch (_) {}
  }

  final List<EditHistory> _history = [];
  int _historyIndex = -1;

  final GlobalKey _repaintBoundaryKey = GlobalKey();

  double _brightness = 0;
  double _contrast = 0;
  double _saturation = 0;
  double _temperature = 0;
  double _blur = 0;

  final List<TextLayer> _textLayers = [];

  final List<StickerLayer> _stickerLayers = [];

  String? _selectedAiStyle;
  String? _selectedAiMode;

  int _aiUsageCount = 0;
  int _aiMaxUsage = 10;

  List<AiStyle>? _apiStyles;
  String _defaultPrompt = '';

  int? _selectedTextIndex;
  int? _selectedStickerIndex;

  int _selectedTabIndex = -1;

  double _baseScale = 1.0;
  double _baseRotation = 0.0;

  final TransformationController _transformationController =
      TransformationController();

  bool _isAiProcessing = false;

  double _previewRenderWidth = 0;
  double _previewRenderHeight = 0;

  @override
  void initState() {
    super.initState();
    _currentImage = widget.initialImage;
    if (_currentImage != null) {
      WidgetsBinding.instance.addPostFrameCallback((_) => _loadImageBytes());
    }
    _fetchAiStyles();
  }

  /// 從後端取得 AI 風格列表 + 使用量（依 widget.category 過濾）
  Future<void> _fetchAiStyles() async {
    final resp = await ref
        .read(aiImageServiceProvider)
        .fetchStyles(category: widget.category);
    if (!mounted || resp == null) return;
    setState(() {
      _apiStyles = resp.styles;
      _defaultPrompt = resp.defaultPrompt;
      _aiUsageCount = resp.weeklyUsage;
      _aiMaxUsage = resp.weeklyLimit;
    });
  }

  /// 添加 AI 操作紀錄（計數以後端回傳的 _aiUsageCount 為準，不本地累加）
  void _addAiHistoryRecord(String action) {
    ref
        .read(aiEditHistoryControllerProvider.notifier)
        .addEntry(action: action, count: _aiUsageCount);
  }

  static Uint8List _readFileSync(String path) => File(path).readAsBytesSync();

  Future<void> _loadImageBytes() async {
    if (_currentImage == null) return;
    final bytes = await compute(_readFileSync, _currentImage!.path);
    if (!mounted) return;
    setState(() => _imageBytes = bytes);
    if (_history.isEmpty) {
      _addHistory('載入圖片', saveImage: true);
    }
    _updateDominantColor();
  }

  @override
  void dispose() {
    _transformationController.dispose();
    super.dispose();
  }

  /// 根據字型 ID 取得對應的 TextStyle（使用系統內建字型，不需下載）
  TextStyle _getFontStyle(
    String fontId, {
    required double fontSize,
    required Color color,
    FontWeight fontWeight = FontWeight.normal,
    FontStyle fontStyle = FontStyle.normal,
  }) {
    String fontFamily;
    switch (fontId) {
      case 'notoSerifTC':
        fontFamily = 'Songti TC';
        break;
      case 'maShanZheng':
        fontFamily = 'Kaiti TC';
        break;
      case 'zhiMangXing':
        fontFamily = 'STXingkai';
        break;
      case 'longCang':
        fontFamily = 'Baoli TC';
        break;
      case 'notoSansTC':
      default:
        fontFamily = 'PingFang TC';
        break;
    }
    return TextStyle(
      fontFamily: fontFamily,
      fontFamilyFallback: const [
        'PingFang TC',
        '.AppleSystemUIFont',
        'Noto Sans CJK TC',
        'Roboto',
      ],
      fontSize: fontSize,
      color: color,
      fontWeight: fontWeight,
      fontStyle: fontStyle,
    );
  }

  Future<void> _pickImage() async {
    final picker = ImagePicker();
    final picked = await picker.pickImage(source: ImageSource.gallery);
    if (picked != null) {
      _history.clear();
      _historyIndex = -1;
      setState(() {
        _currentImage = File(picked.path);
      });
      _loadImageBytes();
    }
  }

  void _addHistory(
    String action, {
    bool saveImage = false,
    bool saveTransform = false,
  }) {
    setState(() {
      if (_historyIndex < _history.length - 1) {
        _history.removeRange(_historyIndex + 1, _history.length);
      }
      _history.add(
        EditHistory(
          action: action,
          brightness: _brightness,
          contrast: _contrast,
          saturation: _saturation,
          temperature: _temperature,
          blur: _blur,
          textLayers: List.from(_textLayers.map((l) => l.copyWith())),
          stickerLayers: List.from(_stickerLayers.map((l) => l.copyWith())),
          imageBytes: saveImage
              ? (_imageBytes != null ? Uint8List.fromList(_imageBytes!) : null)
              : null,
          imageVersion: _imageVersion,
          transformMatrix: saveTransform
              ? _transformationController.value.clone()
              : null,
        ),
      );
      _historyIndex = _history.length - 1;

      if (_history.length > 20) {
        _history.removeAt(0);
        _historyIndex--;
      }
    });
  }

  void _undo() {
    if (_historyIndex > 0) {
      setState(() {
        _historyIndex--;
      });
      _applyHistory(_history[_historyIndex]);
    }
  }

  void _redo() {
    if (_historyIndex < _history.length - 1) {
      setState(() {
        _historyIndex++;
      });
      _applyHistory(_history[_historyIndex]);
    }
  }

  void _applyHistory(EditHistory h) {
    setState(() {
      _brightness = h.brightness;
      _contrast = h.contrast;
      _saturation = h.saturation;
      _temperature = h.temperature;
      _blur = h.blur;
      _textLayers.clear();
      _textLayers.addAll(h.textLayers);
      _stickerLayers.clear();
      _stickerLayers.addAll(h.stickerLayers);

      if (h.imageBytes != null) {
        _imageBytes = Uint8List.fromList(h.imageBytes!);
        _imageVersion = h.imageVersion;
      }

      if (h.transformMatrix != null) {
        _transformationController.value = h.transformMatrix!.clone();
      }
    });
  }

  Future<void> _saveImage() async {
    if (_currentImage == null || _imageBytes == null) {
      FloatingToast.error(context, '沒有可儲存的圖片');
      return;
    }

    setState(() {
      _selectedTextIndex = null;
      _selectedStickerIndex = null;
    });

    await Future.delayed(const Duration(milliseconds: 50));
    if (!mounted) return;

    TruckLoadingOverlay.show(context);

    try {
      final boundary =
          _repaintBoundaryKey.currentContext?.findRenderObject()
              as RenderRepaintBoundary?;
      if (boundary == null) {
        throw Exception('無法取得圖片渲染區域');
      }

      final ui.Image image = await boundary.toImage(pixelRatio: 3.0);
      final byteData = await image.toByteData(format: ui.ImageByteFormat.png);
      if (byteData == null) {
        throw Exception('無法轉換圖片');
      }
      final Uint8List pngBytes = byteData.buffer.asUint8List();

      await _currentImage!.writeAsBytes(pngBytes);

      if (!mounted) return;
      Navigator.of(context, rootNavigator: true).pop();
      context.pop(_currentImage);
    } catch (e) {
      if (!mounted) return;
      Navigator.of(context, rootNavigator: true).pop();
      FloatingToast.error(context, '儲存失敗，請稍後再試');
    }
  }

  /// 執行 AI 圖片編輯（使用 Gemini API）
  Future<void> _runAiEdit({String? prompt, String? styleName}) async {
    if (_imageBytes == null || _imageBytes!.isEmpty) {
      FloatingToast.error(context, '沒有可處理的圖片');
      return;
    }

    _addHistory('AI 編輯前', saveImage: true);

    setState(() => _isAiProcessing = true);

    try {
      final edited = await ref
          .read(aiImageServiceProvider)
          .transformWithGemini(imageBytes: _imageBytes!, prompt: prompt);
      if (!mounted) return;

      setState(() {
        _imageBytes = edited;
        _imageVersion++;
        _isAiProcessing = false;
      });
      if (_currentImage != null) {
        await _currentImage!.writeAsBytes(edited);
      }
      _addHistory('AI 編輯完成', saveImage: true);
      await _fetchAiStyles();
      if (!mounted) return;
      _addAiHistoryRecord(styleName ?? 'AI 編輯');
      FloatingToast.success(context, 'AI 編輯完成');
    } on AiQuotaExceededException catch (e) {
      if (mounted) {
        setState(() => _isAiProcessing = false);
        if (_history.isNotEmpty && _history.last.action == 'AI 編輯前') {
          _history.removeLast();
          _historyIndex--;
        }
        FloatingToast.error(context, e.message);
      }
      await _fetchAiStyles();
    } on AiTransformException catch (e) {
      if (mounted) {
        setState(() => _isAiProcessing = false);
        if (_history.isNotEmpty && _history.last.action == 'AI 編輯前') {
          _history.removeLast();
          _historyIndex--;
        }
        FloatingToast.error(context, e.message);
      }
    } catch (e) {
      if (mounted) {
        setState(() => _isAiProcessing = false);
        if (_history.isNotEmpty && _history.last.action == 'AI 編輯前') {
          _history.removeLast();
          _historyIndex--;
        }
        FloatingToast.error(context, 'AI 編輯失敗，請稍後再試');
      }
    }
  }

  /// 根據風格 ID 取得風格名稱
  /// 從 API 風格列表中查找風格名稱
  String _getStyleName(String? styleName) {
    if (styleName == null) return 'AI 編輯';
    final style = _apiStyles?.where((s) => s.name == styleName).firstOrNull;
    return style?.label ?? 'AI 編輯';
  }

  /// 從 API 風格列表中查找風格 prompt
  String _getPromptForStyle(String? styleName) {
    if (styleName == null) return _defaultPrompt;
    final style = _apiStyles?.where((s) => s.name == styleName).firstOrNull;
    return style?.prompt ?? _defaultPrompt;
  }

  /// 顯示 AI 風格確認 dialog
  Future<bool> _showAiStyleConfirmDialog(String styleName) async {
    if (!mounted) return false;
    final result = await showDialog<bool>(
      context: context,
      barrierDismissible: false,
      builder: (context) => Dialog(
        backgroundColor: Colors.transparent,
        insetPadding: EdgeInsets.symmetric(horizontal: 40.w),
        child: Container(
          decoration: BoxDecoration(
            color: AppColors.dividerLight,
            borderRadius: BorderRadius.circular(24.r),
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: double.infinity,
                padding: EdgeInsets.symmetric(vertical: 24.h, horizontal: 16.w),
                decoration: BoxDecoration(
                  color: Colors.white,
                  borderRadius: BorderRadius.vertical(
                    top: Radius.circular(24.r),
                  ),
                ),
                child: Text(
                  '是否要消耗點數\n將相片更換為$styleName風格',
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 18.sp,
                    color: AppColors.textEmphasis,
                    height: 1.6,
                  ),
                ),
              ),
              const Divider(height: 0, thickness: 1, color: AppColors.gray300),
              Material(
                color: Colors.white,
                borderRadius: BorderRadius.vertical(
                  bottom: Radius.circular(24.r),
                ),
                child: Row(
                  children: [
                    Expanded(
                      child: InkWell(
                        borderRadius: BorderRadius.only(
                          bottomLeft: Radius.circular(24.r),
                        ),
                        onTap: () => Navigator.of(context).pop(false),
                        child: SizedBox(
                          height: 54.h,
                          child: Center(
                            child: Text(
                              '取消',
                              style: TextStyle(
                                fontSize: 20.sp,
                                fontWeight: FontWeight.w700,
                                color: AppColors.textSecondary,
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                    Container(width: 1, height: 54.h, color: AppColors.gray300),
                    Expanded(
                      child: InkWell(
                        borderRadius: BorderRadius.only(
                          bottomRight: Radius.circular(24.r),
                        ),
                        onTap: () => Navigator.of(context).pop(true),
                        child: SizedBox(
                          height: 54.h,
                          child: Center(
                            child: Text(
                              '確定',
                              style: TextStyle(
                                fontSize: 20.sp,
                                fontWeight: FontWeight.w700,
                                color: AppColors.textEmphasis,
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
    return result ?? false;
  }

  @override
  Widget build(BuildContext context) {
    final aiHistory =
        ref.watch(aiEditHistoryControllerProvider).value ?? const [];
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: HkGlassAppBar(
        title: const Text('圖片編輯器'),
        onBackPressed: context.pop,
        actions: [
          TextButton(
            onPressed: _saveImage,
            child: const HkButtonLabel(
              '儲存',
              style: TextStyle(fontWeight: FontWeight.w700),
            ),
          ),
          const SizedBox(width: 8),
        ],
      ),
      body: Column(
        children: [
          _buildTabBar(),
          AspectRatio(aspectRatio: 4 / 3, child: _buildImagePreview()),
          Expanded(
            child: _selectedTabIndex >= 0
                ? _buildInlineTabContent(_selectedTabIndex, aiHistory)
                : AnimatedContainer(
                    duration: const Duration(milliseconds: 300),
                    color: _imageBytes != null
                        ? _dominantColor.withValues(alpha: 0.45)
                        : AppColors.bgPrimary,
                  ),
          ),
        ],
      ),
    );
  }

  /// 底部彈跳視窗內容（調整功能使用 StatefulBuilder 讓滑桿可以即時更新）
  /// 直接顯示在圖片下方的 tab 內容（取代底部彈出抽屜）
  Widget _buildInlineTabContent(
    int tabIndex,
    List<AiEditHistoryEntry> aiHistory,
  ) {
    return Container(
      decoration: BoxDecoration(
        color: Colors.white,
        border: Border(top: BorderSide(color: AppColors.gray100, width: 0.5)),
      ),
      child: _buildInlineTabBody(tabIndex, aiHistory),
    );
  }

  Widget _buildInlineTabBody(int tabIndex, List<AiEditHistoryEntry> aiHistory) {
    switch (tabIndex) {
      case 0:
        return SingleChildScrollView(
          padding: EdgeInsets.only(bottom: 24.h),
          child: ImageEditorAiTab(
            selectedMode: _selectedAiMode,
            selectedStyle: _selectedAiStyle,
            aiHistory: aiHistory
                .map((entry) => entry.toViewMap())
                .toList(growable: false),
            aiUsageCount: _aiUsageCount,
            aiMaxUsage: _aiMaxUsage,
            aiStyles: _apiStyles,
            category: widget.category,
            onModeChanged: (mode) => setState(() => _selectedAiMode = mode),
            onStyleChanged: (style) async {
              final styleName = _getStyleName(style);
              final confirmed = await _showAiStyleConfirmDialog(styleName);
              if (confirmed && mounted) {
                setState(() {
                  _selectedAiStyle = style;
                  _selectedTabIndex = -1;
                });
                final prompt = _getPromptForStyle(style);
                _runAiEdit(prompt: prompt, styleName: styleName);
              }
            },
            onApplyPrompt: (prompt) {
              setState(() => _selectedTabIndex = -1);
              _runAiEdit(prompt: prompt, styleName: '自訂提示詞');
            },
          ),
        );

      case 1:
        return SingleChildScrollView(
          padding: EdgeInsets.only(bottom: 24.h),
          child: ImageEditorAdjustTab(
            brightness: _brightness,
            contrast: _contrast,
            saturation: _saturation,
            temperature: _temperature,
            blur: _blur,
            onBrightnessChanged: (v) => setState(() => _brightness = v),
            onContrastChanged: (v) => setState(() => _contrast = v),
            onSaturationChanged: (v) => setState(() => _saturation = v),
            onTemperatureChanged: (v) => setState(() => _temperature = v),
            onBlurChanged: (v) => setState(() => _blur = v),
            onReset: () {
              setState(() {
                _brightness = 0;
                _contrast = 0;
                _saturation = 0;
                _temperature = 0;
                _blur = 0;
              });
              _addHistory('重置調整');
            },
            onApply: () {
              _addHistory('套用調整');
              setState(() => _selectedTabIndex = -1);
            },
          ),
        );

      case 2:
        return SingleChildScrollView(
          padding: EdgeInsets.only(bottom: 24.h),
          child: ImageEditorTextTab(
            textLayers: _textLayers,
            onAddText: (layer) {
              setState(() => _textLayers.add(layer));
              _addHistory('新增文字');
            },
            onUpdateLayer: (index, layer) =>
                setState(() => _textLayers[index] = layer),
            onDeleteLayer: (index) {
              setState(() => _textLayers.removeAt(index));
              _addHistory('刪除文字');
            },
            onToggleVisibility: (index) {
              setState(() {
                _textLayers[index] = _textLayers[index].copyWith(
                  isVisible: !_textLayers[index].isVisible,
                );
              });
            },
          ),
        );

      case 3:
        return SingleChildScrollView(
          padding: EdgeInsets.only(bottom: 24.h),
          child: ImageEditorStickerTab(
            stickerLayers: _stickerLayers,
            onAddSticker: (layer) {
              setState(() => _stickerLayers.add(layer));
              _addHistory('新增貼圖');
            },
            onUpdateLayer: (index, layer) =>
                setState(() => _stickerLayers[index] = layer),
            onDeleteLayer: (index) {
              setState(() => _stickerLayers.removeAt(index));
              _addHistory('刪除貼圖');
            },
            onToggleVisibility: (index) {
              setState(() {
                _stickerLayers[index] = _stickerLayers[index].copyWith(
                  isVisible: !_stickerLayers[index].isVisible,
                );
              });
            },
          ),
        );

      default:
        return const SizedBox.shrink();
    }
  }

  Widget _buildImagePreview() {
    if (_imageBytes == null) {
      return Container(
        margin: EdgeInsets.all(16.w),
        child: Center(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(
                AppIcons.imageOutlined,
                size: 64.sp,
                color: AppIconColors.muted,
              ),
              SizedBox(height: 16.h),
              ElevatedButton.icon(
                onPressed: _pickImage,
                icon: const Icon(AppIcons.upload),
                label: const HkButtonLabel('上傳圖片'),
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.primary,
                  foregroundColor: Colors.white,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(24.r),
                  ),
                ),
              ),
            ],
          ),
        ),
      );
    }

    return LayoutBuilder(
      builder: (context, constraints) {
        final double areaWidth = constraints.maxWidth;
        final double areaHeight = constraints.maxHeight;
        final double renderWidth = areaWidth;
        final double renderHeight = renderWidth * 3 / 4;

        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (_previewRenderWidth != renderWidth ||
              _previewRenderHeight != renderHeight) {
            _previewRenderWidth = renderWidth;
            _previewRenderHeight = renderHeight;
          }
        });

        return AnimatedContainer(
          duration: const Duration(milliseconds: 300),
          width: areaWidth,
          height: areaHeight,
          color: _dominantColor.withValues(alpha: 0.45),
          child: Center(
            child: GestureDetector(
              onTap: () {
                setState(() {
                  _selectedTextIndex = null;
                  _selectedStickerIndex = null;
                });
              },
              child: RepaintBoundary(
                key: _repaintBoundaryKey,
                child: Container(
                  width: renderWidth,
                  height: renderHeight,
                  decoration: BoxDecoration(color: AppColors.bgPrimary),
                  child: Container(
                    color: _dominantColor.withValues(alpha: 0.45),
                    child: Stack(
                      children: [
                        Positioned.fill(
                          child: ColorFiltered(
                            colorFilter: ColorFilter.matrix(
                              _buildColorMatrix(),
                            ),
                            child: Image.memory(
                              _imageBytes!,
                              key: ValueKey(_imageVersion),
                              fit: BoxFit.contain,
                              gaplessPlayback: true,
                            ),
                          ),
                        ),
                        ..._textLayers
                            .asMap()
                            .entries
                            .where((e) => e.value.isVisible)
                            .map((entry) {
                              final index = entry.key;
                              final layer = entry.value;
                              final isSelected = _selectedTextIndex == index;

                              return Positioned(
                                left: layer.x,
                                top: layer.y,
                                child: GestureDetector(
                                  onTap: () {
                                    setState(() {
                                      _selectedTextIndex = index;
                                      _selectedStickerIndex = null;
                                    });
                                  },
                                  onScaleStart: (details) {
                                    setState(() {
                                      _selectedTextIndex = index;
                                      _selectedStickerIndex = null;
                                    });
                                    _baseScale = layer.scale;
                                    _baseRotation = layer.rotation;
                                  },
                                  onScaleUpdate: (details) {
                                    setState(() {
                                      if (details.pointerCount == 1) {
                                        _textLayers[index] = layer.copyWith(
                                          x:
                                              layer.x +
                                              details.focalPointDelta.dx,
                                          y:
                                              layer.y +
                                              details.focalPointDelta.dy,
                                        );
                                      } else {
                                        final newScale =
                                            (_baseScale * details.scale).clamp(
                                              0.3,
                                              5.0,
                                            );
                                        final newRotation =
                                            _baseRotation + details.rotation;
                                        _textLayers[index] = layer.copyWith(
                                          scale: newScale,
                                          rotation: newRotation,
                                          x:
                                              layer.x +
                                              details.focalPointDelta.dx,
                                          y:
                                              layer.y +
                                              details.focalPointDelta.dy,
                                        );
                                      }
                                    });
                                  },
                                  onScaleEnd: (_) => _addHistory('編輯文字'),
                                  child: Stack(
                                    clipBehavior: Clip.none,
                                    children: [
                                      if (isSelected)
                                        Positioned.fill(
                                          child: Container(
                                            decoration: BoxDecoration(
                                              border: Border.all(
                                                color: AppColors.primary,
                                                width: 2.w,
                                              ),
                                              borderRadius:
                                                  BorderRadius.circular(6.r),
                                            ),
                                          ),
                                        ),
                                      Transform.rotate(
                                        angle: layer.rotation,
                                        child: Transform.scale(
                                          scale: layer.scale,
                                          child: Padding(
                                            padding: EdgeInsets.all(8.w),
                                            child: Text(
                                              layer.text,
                                              style: _getFontStyle(
                                                layer.fontFamily,
                                                fontSize: layer.fontSize,
                                                color: layer.color,
                                                fontWeight: layer.isBold
                                                    ? FontWeight.bold
                                                    : FontWeight.normal,
                                                fontStyle: layer.isItalic
                                                    ? FontStyle.italic
                                                    : FontStyle.normal,
                                              ),
                                            ),
                                          ),
                                        ),
                                      ),
                                    ],
                                  ),
                                ),
                              );
                            }),
                        ..._stickerLayers
                            .asMap()
                            .entries
                            .where((e) => e.value.isVisible)
                            .map((entry) {
                              final index = entry.key;
                              final layer = entry.value;
                              final isSelected = _selectedStickerIndex == index;

                              return Positioned(
                                left: layer.x,
                                top: layer.y,
                                child: GestureDetector(
                                  onTap: () {
                                    setState(() {
                                      _selectedStickerIndex = index;
                                      _selectedTextIndex = null;
                                    });
                                  },
                                  onScaleStart: (details) {
                                    setState(() {
                                      _selectedStickerIndex = index;
                                      _selectedTextIndex = null;
                                    });
                                    _baseScale = layer.scale;
                                    _baseRotation = layer.rotation;
                                  },
                                  onScaleUpdate: (details) {
                                    setState(() {
                                      if (details.pointerCount == 1) {
                                        _stickerLayers[index] = layer.copyWith(
                                          x:
                                              layer.x +
                                              details.focalPointDelta.dx,
                                          y:
                                              layer.y +
                                              details.focalPointDelta.dy,
                                        );
                                      } else {
                                        final newScale =
                                            (_baseScale * details.scale).clamp(
                                              0.3,
                                              5.0,
                                            );
                                        final newRotation =
                                            _baseRotation + details.rotation;
                                        _stickerLayers[index] = layer.copyWith(
                                          scale: newScale,
                                          rotation: newRotation,
                                          x:
                                              layer.x +
                                              details.focalPointDelta.dx,
                                          y:
                                              layer.y +
                                              details.focalPointDelta.dy,
                                        );
                                      }
                                    });
                                  },
                                  onScaleEnd: (_) => _addHistory('編輯貼圖'),
                                  child: Stack(
                                    clipBehavior: Clip.none,
                                    children: [
                                      if (isSelected)
                                        Positioned.fill(
                                          child: Container(
                                            decoration: BoxDecoration(
                                              border: Border.all(
                                                color: AppColors.primary,
                                                width: 2.w,
                                              ),
                                              borderRadius:
                                                  BorderRadius.circular(6.r),
                                            ),
                                          ),
                                        ),
                                      Transform.rotate(
                                        angle: layer.rotation,
                                        child: Transform.scale(
                                          scale: layer.scale,
                                          child: Padding(
                                            padding: EdgeInsets.all(8.w),
                                            child: layer.isBadgeSticker
                                                ? StickerBadgeView(
                                                    layer.badge!,
                                                    size: layer.size,
                                                  )
                                                : Text(
                                                    layer.emoji ?? '',
                                                    style: TextStyle(
                                                      fontSize:
                                                          layer.size * 0.8,
                                                    ),
                                                  ),
                                          ),
                                        ),
                                      ),
                                    ],
                                  ),
                                ),
                              );
                            }),
                        if (_isAiProcessing)
                          Positioned.fill(child: _AiProcessingOverlay()),
                      ],
                    ),
                  ),
                ),
              ),
            ),
          ),
        );
      },
    );
  }

  List<double> _buildColorMatrix() {
    final double b = _brightness / 100;
    final double c = 1 + (_contrast / 100);
    final double s = 1 + (_saturation / 100);
    final double t = _temperature / 100;

    double aiB = 0;
    double aiC = 1;
    double aiS = 1;
    double aiWarm = 0;

    switch (_selectedAiStyle) {
      case 'natural':
        aiB = 0.05;
        aiS = 1.15;
        aiWarm = 0.05;
        break;
      case 'warm':
        aiWarm = 0.25;
        aiB = 0.03;
        aiS = 1.0;
        aiC = 0.95;
        break;
      case 'crisp':
        aiS = 1.3;
        aiC = 1.15;
        aiB = 0.05;
        break;
      case 'giftbox':
        aiC = 1.1;
        aiS = 1.1;
        aiB = 0.03;
        aiWarm = 0.08;
        break;
      case 'documentary':
        aiS = 0.9;
        aiC = 1.05;
        aiWarm = 0.05;
        break;
      case 'kitchen':
        aiWarm = 0.12;
        aiS = 1.05;
        aiB = 0.02;
        break;
      case 'clean':
        aiB = 0.1;
        aiC = 1.2;
        aiS = 1.1;
        break;
    }

    final double finalB = b + aiB;
    final double finalC = c * aiC;
    final double finalS = s * aiS;
    final double finalWarm = t + aiWarm;

    final double rAdj = finalWarm * 30;
    final double bAdj = -finalWarm * 30;

    return [
      finalC * finalS,
      0,
      0,
      0,
      finalB * 255 + rAdj,
      0,
      finalC * finalS,
      0,
      0,
      finalB * 255,
      0,
      0,
      finalC * finalS,
      0,
      finalB * 255 + bAdj,
      0,
      0,
      0,
      1,
      0,
    ];
  }

  Widget _buildTabBar() {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 4.h),
      decoration: const BoxDecoration(
        color: Colors.white,
        border: Border(
          bottom: BorderSide(color: Color(0xFF000000), width: 0.5),
        ),
      ),
      child: Row(
        children: [
          _buildTabButton(0, AppIcons.autoAwesomeOutlined, 'AI'),
          _buildTabButton(1, AppIcons.tune, '調整'),
          _buildTabButton(2, AppIcons.title, '文字'),
          _buildTabButton(3, AppIcons.tagFacesOutlined, '標籤'),
          const Spacer(),
          Material(
            color: Colors.transparent,
            child: InkWell(
              borderRadius: BorderRadius.circular(16.r),
              onTap: _historyIndex > 0
                  ? () {
                      _undo();
                    }
                  : null,
              child: Padding(
                padding: EdgeInsets.all(8.w),
                child: Icon(
                  AppIcons.undo,
                  size: 20.sp,
                  color: _historyIndex > 0
                      ? AppIconColors.muted
                      : AppIconColors.disabled,
                ),
              ),
            ),
          ),
          Material(
            color: Colors.transparent,
            child: InkWell(
              borderRadius: BorderRadius.circular(16.r),
              onTap: _historyIndex < _history.length - 1
                  ? () {
                      _redo();
                    }
                  : null,
              child: Padding(
                padding: EdgeInsets.all(8.w),
                child: Icon(
                  AppIcons.redo,
                  size: 20.sp,
                  color: _historyIndex < _history.length - 1
                      ? AppIconColors.muted
                      : AppIconColors.disabled,
                ),
              ),
            ),
          ),
          Material(
            color: Colors.transparent,
            child: InkWell(
              borderRadius: BorderRadius.circular(16.r),
              onTap: () {
                setState(() {
                  _brightness = 0;
                  _contrast = 0;
                  _saturation = 0;
                  _temperature = 0;
                  _blur = 0;
                  _selectedAiStyle = null;
                  _textLayers.clear();
                  _stickerLayers.clear();
                });
                _addHistory('重置');
              },
              child: Padding(
                padding: EdgeInsets.all(8.w),
                child: Icon(
                  AppIcons.refresh,
                  size: 20.sp,
                  color: AppIconColors.muted,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildTabButton(int index, IconData icon, String label) {
    final isSelected = _selectedTabIndex == index;
    return GestureDetector(
      onTap: () {
        setState(() {
          _selectedTabIndex = isSelected ? -1 : index;
        });
      },
      child: Container(
        width: 52.w,
        margin: EdgeInsets.only(right: 8.w),
        padding: EdgeInsets.symmetric(vertical: 8.h),
        decoration: BoxDecoration(
          color: isSelected ? AppColors.primary : Colors.transparent,
          borderRadius: BorderRadius.circular(8.r),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              icon,
              size: 20.sp,
              color: isSelected ? AppIconColors.onBrand : AppIconColors.muted,
            ),
            SizedBox(height: 2.h),
            Text(
              label,
              style: TextStyle(
                fontSize: 12.sp,
                color: isSelected ? Colors.white : AppColors.textSecondary,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// AI 處理中的蒙版動畫（從右到左單向移動）
class _AiProcessingOverlay extends StatefulWidget {
  const _AiProcessingOverlay();

  @override
  State<_AiProcessingOverlay> createState() => _AiProcessingOverlayState();
}

class _AiProcessingOverlayState extends State<_AiProcessingOverlay>
    with TickerProviderStateMixin {
  late AnimationController _spinCtrl;
  late AnimationController _pulseCtrl;
  late IconData _fruitIcon;
  late Color _fruitColor;

  static const _fruits = [
    (icon: AppIcons.ecoOutlined, color: Color(0x4DFF3B30)),
    (icon: AppIcons.localFloristOutlined, color: Color(0x4DFF9500)),
    (icon: AppIcons.spaOutlined, color: Color(0x4DFFCC00)),
    (icon: AppIcons.grassOutlined, color: Color(0x4DFF4F64)),
    (icon: AppIcons.yardOutlined, color: Color(0x4DFF2D55)),
    (icon: AppIcons.ecoOutlined, color: Color(0x4DA2D729)),
    (icon: AppIcons.localFloristOutlined, color: Color(0x4D5856D6)),
    (icon: AppIcons.spaOutlined, color: Color(0x4D66D464)),
  ];

  @override
  void initState() {
    super.initState();
    final pick = _fruits[DateTime.now().millisecond % _fruits.length];
    _fruitIcon = pick.icon;
    _fruitColor = pick.color;
    _spinCtrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1200),
    )..repeat();
    _pulseCtrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1500),
    )..repeat(reverse: true);
  }

  @override
  void dispose() {
    _spinCtrl.dispose();
    _pulseCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      color: AppColors.textEmphasis.withValues(alpha: 0.35),
      child: Center(
        child: AnimatedBuilder(
          animation: Listenable.merge([_spinCtrl, _pulseCtrl]),
          builder: (_, _) {
            final pulse = 0.9 + _pulseCtrl.value * 0.15;
            return Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Transform.scale(
                  scale: pulse,
                  child: Container(
                    width: 72.w,
                    height: 72.w,
                    decoration: BoxDecoration(
                      color: _fruitColor,
                      shape: BoxShape.circle,
                      boxShadow: [
                        BoxShadow(
                          color: _fruitColor.withValues(alpha: 0.5),
                          blurRadius: 20,
                          spreadRadius: 4,
                        ),
                      ],
                    ),
                    child: Center(
                      child: RotationTransition(
                        turns: _spinCtrl,
                        child: ExcludeSemantics(
                          child: Icon(
                            _fruitIcon,
                            size: 48.w,
                            color: _fruitColor.withValues(alpha: 1),
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
                SizedBox(height: 16.h),
                Text(
                  'AI 處理中...',
                  style: TextStyle(
                    fontSize: 14.sp,
                    fontWeight: FontWeight.w700,
                    color: Colors.white,
                    shadows: [
                      Shadow(
                        color: AppColors.textEmphasis.withValues(alpha: 0.3),
                        blurRadius: 4,
                      ),
                    ],
                  ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }
}
