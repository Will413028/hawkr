import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:video_player/video_player.dart';

/// Lightweight thumbnail for video posts.
/// - Initializes the controller, does NOT auto-play.
/// - Displays the first frame with correct aspect ratio.
class PostVideoThumbnail extends StatefulWidget {
  const PostVideoThumbnail({super.key, required this.url});
  final String url;

  @override
  State<PostVideoThumbnail> createState() => _PostVideoThumbnailState();
}

class _PostVideoThumbnailState extends State<PostVideoThumbnail> {
  VideoPlayerController? _controller;
  bool _initialized = false;
  bool _error = false;

  @override
  void initState() {
    super.initState();
    _init();
  }

  Future<void> _init() async {
    try {
      final c = VideoPlayerController.networkUrl(Uri.parse(widget.url));
      await c.initialize();
      await c.setVolume(0);
      await c.pause();
      if (!mounted) return;
      setState(() {
        _controller = c;
        _initialized = true;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() => _error = true);
    }
  }

  @override
  void dispose() {
    _controller?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (_error) {
      return Container(color: AppColors.gray300);
    }
    if (!_initialized || _controller == null) {
      return Container(color: AppColors.borderSubtle);
    }
    final c = _controller!;
    final aspect = c.value.aspectRatio == 0 ? 1.0 : c.value.aspectRatio;
    return AspectRatio(aspectRatio: aspect, child: VideoPlayer(c));
  }
}
