import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:video_player/video_player.dart';

class PostVideoPlayer extends StatefulWidget {
  final String url;
  final bool autoPlay;
  final bool loop;
  final bool muted;

  const PostVideoPlayer({
    super.key,
    required this.url,
    this.autoPlay = true,
    this.loop = true,
    this.muted = true,
  });

  @override
  State<PostVideoPlayer> createState() => _PostVideoPlayerState();
}

class _PostVideoPlayerState extends State<PostVideoPlayer> {
  VideoPlayerController? _controller;
  VoidCallback? _onControllerChanged;
  bool _initialized = false;
  bool _error = false;
  bool _showOverlay = true;

  @override
  void initState() {
    super.initState();
    _init();
  }

  Future<void> _init() async {
    try {
      final c = VideoPlayerController.networkUrl(Uri.parse(widget.url));
      await c.initialize();
      c.setLooping(widget.loop);
      if (widget.muted) await c.setVolume(0);
      if (widget.autoPlay) await c.play();
      setState(() {
        _controller = c;
        _initialized = true;
        _error = false;
        _showOverlay = !widget.autoPlay;
      });
      _onControllerChanged = () {
        if (!mounted) return;
        final isPlaying = c.value.isPlaying;
        if (_showOverlay == isPlaying) {
          setState(() => _showOverlay = !isPlaying);
        }
      };
      c.addListener(_onControllerChanged!);
    } catch (_) {
      setState(() {
        _error = true;
      });
    }
  }

  @override
  void dispose() {
    final c = _controller;
    if (c != null && _onControllerChanged != null) {
      c.removeListener(_onControllerChanged!);
    }
    _controller?.dispose();
    super.dispose();
  }

  void _togglePlay() async {
    final c = _controller;
    if (c == null) return;
    if (c.value.isPlaying) {
      await c.pause();
    } else {
      await c.play();
    }
    if (mounted) setState(() => _showOverlay = !c.value.isPlaying);
  }

  @override
  Widget build(BuildContext context) {
    if (_error) {
      return Container(
        color: Colors.black12,
        child: const Center(
          child: Icon(AppIcons.errorOutline, color: AppIconColors.danger),
        ),
      );
    }
    if (!_initialized || _controller == null) {
      return Container(
        color: Colors.black12,
        child: const Center(child: CircularProgressIndicator()),
      );
    }

    final c = _controller!;
    final aspect = c.value.aspectRatio == 0 ? 16 / 9 : c.value.aspectRatio;
    return GestureDetector(
      onTap: _togglePlay,
      child: Stack(
        alignment: Alignment.center,
        children: [
          Center(
            child: AspectRatio(aspectRatio: aspect, child: VideoPlayer(c)),
          ),
          if (_showOverlay)
            Container(
              color: Colors.black26,
              child: Center(
                child: Icon(
                  AppIcons.playCircleFill,
                  color: AppIconColors.onBrand,
                  size: 64.sp,
                ),
              ),
            ),
        ],
      ),
    );
  }
}
