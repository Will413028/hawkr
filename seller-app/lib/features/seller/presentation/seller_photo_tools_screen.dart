import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'dart:math' as math;
import 'dart:typed_data';
import '../../../shared/utils/platform_io.dart';
import 'dart:ui' as ui;
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:video_player/video_player.dart';
import '../../photo_editor/data/ai_image_service.dart';
import '../../photo_editor/presentation/photo_editor_args.dart';
import '../../../shared/utils/floating_toast.dart';
import 'package:vector_math/vector_math_64.dart' as vm;
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

class SellerPhotoToolsScreen extends ConsumerStatefulWidget {
  final Uint8List? imagePreview;
  final Uint8List? mediaBytes;
  final String contentType; // 'image' | 'video'
  const SellerPhotoToolsScreen({
    super.key,
    this.imagePreview,
    this.mediaBytes,
    this.contentType = 'image',
  });

  @override
  ConsumerState<SellerPhotoToolsScreen> createState() =>
      _SellerPhotoToolsScreenState();
}

class _SellerPhotoToolsScreenState
    extends ConsumerState<SellerPhotoToolsScreen> {
  VideoPlayerController? _vc;
  bool _videoReady = false;
  File? _tmpVideo;

  final TransformationController _transformController =
      TransformationController();
  Size? _imageSize;
  double? _viewportSize; // square viewport dimension in logical pixels
  double _minScale = 1.0;
  static const double _maxScale = 5.0;
  bool _transformInitialized = false;
  bool _userInteracted = false;

  // Local mutable state for image editing/preview
  Uint8List? _previewBytes;
  Uint8List? _mediaBytes;
  String _contentType = 'image';
  // UI state for AI options under the preview
  bool _showAiOptions = false;

  @override
  void initState() {
    super.initState();
    _previewBytes = widget.imagePreview;
    _mediaBytes = widget.mediaBytes;
    _contentType = widget.contentType;
    if (_contentType == 'video' &&
        _mediaBytes != null &&
        _mediaBytes!.isNotEmpty) {
      _initVideo();
    } else {
      _loadImageDimensions();
    }
  }

  Future<void> _initVideo() async {
    try {
      final f = File(
        '${Directory.systemTemp.path}/seller_tools_${DateTime.now().millisecondsSinceEpoch}.mp4',
      );
      await f.writeAsBytes(widget.mediaBytes!, flush: true);
      final c = VideoPlayerController.file(f)..setLooping(true);
      await c.initialize();
      if (!mounted) return;
      setState(() {
        _vc = c;
        _tmpVideo = f;
        _videoReady = true;
      });
    } catch (_) {}
  }

  @override
  void dispose() {
    try {
      _vc?.dispose();
    } catch (_) {}
    try {
      _tmpVideo?.delete();
    } catch (_) {}
    _transformController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: HkGlassAppBar(
        title: Text(_contentType == 'video' ? '影片工具' : '圖片工具'),
        subtitle: '裁切、調整與 AI 優化',
        actions: [
          TextButton(
            onPressed: _handleDone,
            child: const HkButtonLabel(
              '完成',
              style: TextStyle(fontWeight: FontWeight.w700),
            ),
          ),
          const SizedBox(width: 8),
        ],
      ),
      body: Column(
        children: [
          Expanded(
            child: Column(
              children: [
                Padding(
                  padding: EdgeInsets.fromLTRB(16.w, 8.h, 16.w, 8.h),
                  child: AspectRatio(
                    aspectRatio: 1,
                    child: _DashedPreview(child: _previewWidget()),
                  ),
                ),
                if (_showAiOptions)
                  Padding(
                    padding: EdgeInsets.fromLTRB(16.w, 0, 16.w, 8.h),
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        OutlinedButton(
                          onPressed: () async {
                            await _runAiEdit();
                            if (mounted) {
                              setState(() => _showAiOptions = false);
                            }
                          },
                          child: const HkButtonLabel('智慧優化'),
                        ),
                        SizedBox(width: 12.w),
                        OutlinedButton(
                          onPressed: () async {
                            final p = await _askCustomPrompt();
                            if (p != null && p.trim().isNotEmpty) {
                              await _runAiEdit(prompt: p.trim());
                            }
                            if (mounted) {
                              setState(() => _showAiOptions = false);
                            }
                          },
                          child: const HkButtonLabel('自訂指令'),
                        ),
                      ],
                    ),
                  ),
                const Spacer(),
                _bottomTools(),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _previewWidget() {
    if (widget.contentType == 'video') {
      if (!_videoReady || _vc == null) {
        return const TruckLoadingOverlay();
      }
      return Stack(
        alignment: Alignment.center,
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(12.r),
            child: AspectRatio(
              aspectRatio: _vc!.value.aspectRatio == 0
                  ? 1
                  : _vc!.value.aspectRatio,
              child: VideoPlayer(_vc!),
            ),
          ),
          GestureDetector(
            onTap: () {
              setState(() {
                if (_vc!.value.isPlaying) {
                  _vc!.pause();
                } else {
                  _vc!.play();
                }
              });
            },
            child: Container(
              decoration: const BoxDecoration(
                color: AppColors.textDisabled,
                shape: BoxShape.circle,
              ),
              padding: EdgeInsets.all(8.w),
              child: Icon(
                _vc!.value.isPlaying
                    ? AppIcons.pauseCircleFilled
                    : AppIcons.playCircleFill,
                size: 56.sp,
                color: AppIconColors.onBrand,
              ),
            ),
          ),
        ],
      );
    }

    if (_previewBytes != null) {
      return ClipRRect(
        borderRadius: BorderRadius.circular(12.r),
        child: _interactiveImage(),
      );
    }
    return Center(
      child: Icon(
        AppIcons.imageOutlined,
        size: 36.sp,
        color: AppIconColors.strong.withValues(alpha: 0.5),
      ),
    );
  }

  Widget _bottomTools() {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 16.h),
      decoration: const BoxDecoration(color: AppColors.bgPrimary),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          // AI Editor with tap handler
          Expanded(
            child: InkWell(
              onTap: _onAiEditorTapped,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  ExcludeSemantics(
                    child: Icon(
                      AppIcons.autoAwesomeOutlined,
                      size: 28.w,
                      color: AppIconColors.active,
                    ),
                  ),
                  SizedBox(height: 6.h),
                  Text('AI Editor', style: TextStyle(fontSize: 12.sp)),
                  Text(
                    '',
                    style: TextStyle(
                      fontSize: 12.sp,
                      color: AppColors.textSecondary,
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _runAiEdit({String? prompt}) async {
    if (_contentType != 'image') {
      // ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('目前只支援圖片 AI 編輯')));
      FloatingToast.error(context, '目前只支援圖片 AI 編輯');
      return;
    }
    final src = _mediaBytes ?? _previewBytes;
    if (src == null || src.isEmpty) {
      // ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('沒有可處理的圖片')));
      FloatingToast.error(context, '沒有可處理的圖片');
      return;
    }

    // Show loading dialog
    TruckLoadingOverlay.show(context);

    try {
      final edited = await ref
          .read(aiImageServiceProvider)
          .transformWithGemini(imageBytes: src, prompt: prompt);
      if (!mounted) return;
      Navigator.of(context, rootNavigator: true).pop();

      setState(() {
        _previewBytes = edited;
        _mediaBytes = edited; // bubble back as the media to upload
        _contentType = 'image';
        _transformInitialized = false;
      });
      await _loadImageDimensions(edited);
    } on AiQuotaExceededException catch (e) {
      if (mounted) {
        Navigator.of(context, rootNavigator: true).pop();
        FloatingToast.error(context, e.message);
      }
    } on AiTransformException catch (e) {
      // 後端已依業務碼分好類（沒產圖 / 未過安全審查 / 版權相似 / 上游不可用），
      // 直接顯示它的文案，不要蓋成一句泛用的「AI 編輯失敗」。
      if (mounted) {
        Navigator.of(context, rootNavigator: true).pop();
        FloatingToast.error(context, e.message);
      }
    } catch (e) {
      if (mounted) {
        Navigator.of(context, rootNavigator: true).pop();
        // ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('AI 編輯失敗，請稍後再試')));
        FloatingToast.error(context, 'AI 編輯失敗，請稍後再試');
      }
    }
  }

  Future<void> _onAiEditorTapped() async {
    if (_contentType != 'image') {
      // ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('目前只支援圖片 AI 編輯')));
      FloatingToast.error(context, '目前只支援圖片 AI 編輯');
      return;
    }

    // 跳轉到圖片編輯器頁面，帶上當前圖片
    final imageBytes = _mediaBytes ?? _previewBytes;
    if (imageBytes == null || imageBytes.isEmpty) {
      // ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('沒有可編輯的圖片')));
      FloatingToast.error(context, '沒有可編輯的圖片');
      return;
    }

    // 將 bytes 寫入臨時檔案
    final tempFile = File(
      '${Directory.systemTemp.path}/edit_image_${DateTime.now().millisecondsSinceEpoch}.jpg',
    );
    await tempFile.writeAsBytes(imageBytes);

    if (!mounted) return;

    final result = await context.push<File?>(
      '/photo-editor',
      extra: PhotoEditorArgs(file: tempFile, category: 'post'),
    );

    // 如果編輯完成並返回了檔案，更新預覽
    if (result != null && mounted) {
      final editedBytes = await result.readAsBytes();
      setState(() {
        _previewBytes = editedBytes;
        _mediaBytes = editedBytes;
        _transformInitialized = false;
      });
      await _loadImageDimensions(editedBytes);
    }
  }

  Future<String?> _askCustomPrompt() async {
    final controller = TextEditingController();
    final res = await showDialog<String>(
      context: context,
      barrierDismissible: false,
      builder: (ctx) => Dialog(
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
                padding: EdgeInsets.symmetric(vertical: 20.h, horizontal: 16.w),
                decoration: BoxDecoration(
                  color: AppColors.bgCard,
                  borderRadius: BorderRadius.vertical(
                    top: Radius.circular(24.r),
                  ),
                ),
                child: Column(
                  children: [
                    Text(
                      '自訂提示詞',
                      style: TextStyle(
                        fontSize: 18.sp,
                        fontWeight: FontWeight.w700,
                        color: AppColors.textEmphasis,
                      ),
                    ),
                    SizedBox(height: 16.h),
                    TextField(
                      controller: controller,
                      decoration: InputDecoration(
                        hintText: 'Enter your prompt (English)',
                        hintStyle: TextStyle(color: AppColors.textSecondary),
                        filled: true,
                        fillColor: AppColors.gray100,
                        border: OutlineInputBorder(
                          borderRadius: BorderRadius.circular(12.r),
                          borderSide: BorderSide.none,
                        ),
                        contentPadding: EdgeInsets.all(12.w),
                      ),
                      maxLines: 3,
                    ),
                  ],
                ),
              ),
              const Divider(height: 0, thickness: 1, color: AppColors.gray300),
              Row(
                children: [
                  Expanded(
                    child: Material(
                      color: AppColors.bgCard,
                      borderRadius: BorderRadius.only(
                        bottomLeft: Radius.circular(24.r),
                      ),
                      child: InkWell(
                        borderRadius: BorderRadius.only(
                          bottomLeft: Radius.circular(24.r),
                        ),
                        onTap: () => Navigator.of(ctx).pop(null),
                        child: SizedBox(
                          height: 54.h,
                          child: Center(
                            child: Text(
                              '取消',
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
                  ),
                  Container(width: 1, height: 54.h, color: AppColors.gray300),
                  Expanded(
                    child: Material(
                      color: AppColors.bgCard,
                      borderRadius: BorderRadius.only(
                        bottomRight: Radius.circular(24.r),
                      ),
                      child: InkWell(
                        borderRadius: BorderRadius.only(
                          bottomRight: Radius.circular(24.r),
                        ),
                        onTap: () => Navigator.of(ctx).pop(controller.text),
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
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
    return res;
  }

  Future<void> _loadImageDimensions([Uint8List? bytes]) async {
    final data = bytes ?? _mediaBytes ?? _previewBytes;
    if (data == null || data.isEmpty) {
      setState(() {
        _imageSize = null;
        _transformInitialized = false;
        _userInteracted = false;
      });
      return;
    }
    try {
      final ui.Codec codec = await ui.instantiateImageCodec(data);
      final ui.FrameInfo frame = await codec.getNextFrame();
      final ui.Image img = frame.image;
      if (!mounted) {
        try {
          codec.dispose();
        } catch (_) {}
        img.dispose();
        return;
      }
      setState(() {
        _imageSize = Size(img.width.toDouble(), img.height.toDouble());
        _transformInitialized = false;
        _userInteracted = false;
      });
      try {
        codec.dispose();
      } catch (_) {}
      img.dispose();
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        _ensureInitialTransform(force: true);
      });
    } catch (_) {}
  }

  Widget _interactiveImage() {
    if (_previewBytes == null) return const SizedBox();
    return LayoutBuilder(
      builder: (context, constraints) {
        final double side = math.min(
          constraints.maxWidth,
          constraints.maxHeight,
        );
        if (side <= 0) return const SizedBox();
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (!mounted) return;
          final needsUpdate =
              _viewportSize == null || (_viewportSize! - side).abs() > 0.5;
          if (needsUpdate) {
            setState(() {
              _viewportSize = side;
            });
            _ensureInitialTransform(force: !_transformInitialized);
          } else {
            _ensureInitialTransform();
          }
        });

        return Center(
          child: SizedBox(
            width: side,
            height: side,
            child: ClipRect(
              child: InteractiveViewer(
                transformationController: _transformController,
                boundaryMargin: EdgeInsets.zero,
                minScale: _minScale,
                maxScale: _maxScale,
                panEnabled: true,
                scaleEnabled: true,
                constrained: false,
                onInteractionStart: (_) => _userInteracted = true,
                onInteractionUpdate: (_) {
                  WidgetsBinding.instance.addPostFrameCallback((_) {
                    if (!mounted) return;
                    _clampTransformation();
                  });
                },
                onInteractionEnd: (_) => _clampTransformation(),
                child: _buildInteractiveChild(),
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _buildInteractiveChild() {
    if (_previewBytes == null) return const SizedBox();
    if (_imageSize == null) {
      return Image.memory(_previewBytes!, fit: BoxFit.contain);
    }
    return SizedBox(
      width: _imageSize!.width,
      height: _imageSize!.height,
      child: Image.memory(
        _previewBytes!,
        fit: BoxFit.cover,
        gaplessPlayback: true,
      ),
    );
  }

  void _ensureInitialTransform({bool force = false}) {
    if (_contentType != 'image') return;
    if (_imageSize == null || _viewportSize == null) return;
    final double viewport = _viewportSize!;
    if (viewport <= 0) return;
    final double coverScale = math.max(
      viewport / _imageSize!.width,
      viewport / _imageSize!.height,
    );

    if ((_minScale - coverScale).abs() > 0.001) {
      setState(() {
        _minScale = coverScale;
      });
    } else {
      _minScale = coverScale;
    }

    if (!_transformInitialized || force) {
      final double scaledWidth = _imageSize!.width * coverScale;
      final double scaledHeight = _imageSize!.height * coverScale;
      final double dx = (viewport - scaledWidth) / 2;
      final double dy = (viewport - scaledHeight) / 2;
      final Matrix4 matrix = Matrix4.identity()
        ..scaleByDouble(coverScale, coverScale, 1.0, 1.0);
      matrix.setTranslation(vm.Vector3(dx, dy, 0));
      _transformController.value = matrix;
      _transformInitialized = true;
    } else if (_userInteracted) {
      _clampTransformation();
    }
  }

  void _clampTransformation() {
    if (_imageSize == null || _viewportSize == null) return;
    final Matrix4 matrix = Matrix4.copy(_transformController.value);
    double scale = matrix.getMaxScaleOnAxis();
    if (scale.isNaN || scale <= 0) scale = _minScale;
    scale = scale.clamp(_minScale, _maxScale);

    final vm.Vector3 translation = matrix.getTranslation();
    double dx = translation.x;
    double dy = translation.y;

    final double viewport = _viewportSize!;
    final double scaledWidth = _imageSize!.width * scale;
    final double scaledHeight = _imageSize!.height * scale;

    double minDx, maxDx;
    if (scaledWidth >= viewport) {
      minDx = viewport - scaledWidth;
      maxDx = 0;
    } else {
      minDx = maxDx = (viewport - scaledWidth) / 2;
    }

    double minDy, maxDy;
    if (scaledHeight >= viewport) {
      minDy = viewport - scaledHeight;
      maxDy = 0;
    } else {
      minDy = maxDy = (viewport - scaledHeight) / 2;
    }

    dx = dx.clamp(minDx, maxDx);
    dy = dy.clamp(minDy, maxDy);

    final Matrix4 next = Matrix4.identity()
      ..scaleByDouble(scale, scale, 1.0, 1.0);
    next.setTranslation(vm.Vector3(dx, dy, 0));
    _transformController.value = next;
  }

  Future<void> _handleDone() async {
    Uint8List? preview = _previewBytes;
    Uint8List? media = _mediaBytes;

    if (_contentType == 'image') {
      await Future<void>.delayed(Duration.zero);
      _clampTransformation();
      final Uint8List? transformed = await _exportTransformedImage();
      if (transformed != null && transformed.isNotEmpty) {
        preview = transformed;
        media = transformed;
      }
    }

    if (!mounted) return;
    context.pop({
      'preview': preview,
      'media': media,
      'contentType': _contentType,
    });
  }

  Future<Uint8List?> _exportTransformedImage() async {
    if (_imageSize == null || _viewportSize == null) return _previewBytes;
    final Uint8List? source = _mediaBytes ?? _previewBytes;
    if (source == null || source.isEmpty) return _previewBytes;

    final Matrix4 matrix = Matrix4.copy(_transformController.value);
    Matrix4? inverse;
    try {
      inverse = Matrix4.inverted(matrix);
    } catch (_) {
      return source;
    }

    Offset mapPoint(double x, double y) {
      final vm.Vector3 result = inverse!.transform3(vm.Vector3(x, y, 0));
      return Offset(result.x, result.y);
    }

    final double viewport = _viewportSize!;
    final List<Offset> points = [
      mapPoint(0, 0),
      mapPoint(viewport, 0),
      mapPoint(0, viewport),
      mapPoint(viewport, viewport),
    ];

    double minX = points.map((p) => p.dx).reduce(math.min);
    double maxX = points.map((p) => p.dx).reduce(math.max);
    double minY = points.map((p) => p.dy).reduce(math.min);
    double maxY = points.map((p) => p.dy).reduce(math.max);

    minX = minX.clamp(0.0, _imageSize!.width);
    maxX = maxX.clamp(0.0, _imageSize!.width);
    minY = minY.clamp(0.0, _imageSize!.height);
    maxY = maxY.clamp(0.0, _imageSize!.height);

    final double width = maxX - minX;
    final double height = maxY - minY;
    if (width <= 1 || height <= 1) return source;

    final ui.Codec codec = await ui.instantiateImageCodec(source);
    final ui.FrameInfo frame = await codec.getNextFrame();
    final ui.Image image = frame.image;

    final ui.PictureRecorder recorder = ui.PictureRecorder();
    final Canvas canvas = Canvas(recorder);
    final Paint paint = Paint();
    final Rect src = Rect.fromLTRB(minX, minY, maxX, maxY);
    final int outWidth = math.max(1, src.width.round());
    final int outHeight = math.max(1, src.height.round());
    final Rect dst = Rect.fromLTWH(
      0,
      0,
      outWidth.toDouble(),
      outHeight.toDouble(),
    );
    canvas.drawImageRect(image, src, dst, paint);
    final ui.Picture picture = recorder.endRecording();
    final ui.Image cropped = await picture.toImage(outWidth, outHeight);
    final ByteData? byteData = await cropped.toByteData(
      format: ui.ImageByteFormat.png,
    );
    final Uint8List? bytes = byteData?.buffer.asUint8List();

    try {
      codec.dispose();
    } catch (_) {}
    image.dispose();
    cropped.dispose();

    return bytes ?? source;
  }
}

class _DashedPreview extends StatelessWidget {
  final Widget child;
  const _DashedPreview({required this.child});

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: AppColors.primarySoft,
        borderRadius: BorderRadius.circular(12.r),
      ),
      child: CustomPaint(painter: _DashGridPainter(), child: child),
    );
  }
}

class _DashGridPainter extends CustomPainter {
  final Paint _p = Paint()
    ..color = AppColors.dividerLight
    ..strokeWidth = 1;

  @override
  void paint(Canvas canvas, Size size) {
    const double dash = 6;
    const double gap = 6;

    void drawDashedLine(Offset a, Offset b) {
      final double totalLength = (b - a).distance;
      final int count = (totalLength / (dash + gap)).floor();
      final Offset dir = (b - a) / totalLength;
      double current = 0;
      for (int i = 0; i < count; i++) {
        final Offset start = a + dir * current;
        final Offset end = a + dir * (current + dash);
        canvas.drawLine(start, end, _p);
        current += dash + gap;
      }
    }

    // Draw 3x3 grid dashed lines
    final double w = size.width;
    final double h = size.height;
    final List<double> verticals = [w / 4, w / 2, 3 * w / 4];
    final List<double> horizontals = [h / 4, h / 2, 3 * h / 4];

    for (final x in verticals) {
      drawDashedLine(Offset(x, 16), Offset(x, h - 16));
    }
    for (final y in horizontals) {
      drawDashedLine(Offset(16, y), Offset(w - 16, y));
    }
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}
