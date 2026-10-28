import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pbenum.dart';
import 'package:hawkr_api/dates.dart';
import 'package:hawkr_twaddr/hawkr_twaddr.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:image_picker/image_picker.dart';

import '../../../config/image_config.dart';
import '../../../core/exceptions/app_exception.dart';
import '../../../core/extensions/url_extensions.dart';
import '../../../core/l10n/ui_locale.dart';
import '../../../core/session/clerk_session_provider.dart';
import '../../../core/session/session_provider.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/app_date_picker.dart';
import '../../../shared/widgets/common_dialog.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_tap_target.dart';
import '../../../shared/widgets/oss_image.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../auth/data/account_deletion.dart';
import '../../auth/data/auth_repository.dart';
import '../../media/data/media_repository.dart';
import 'widgets/profile_edit_widgets.dart';

/// Profile edit screen
/// - Responsive account/contact and optional-profile sections
/// - Editable: username, mobile, gender, birth, city, job, avatar
/// - Readonly: login email from Clerk, hidden when unavailable
class ProfileEditScreen extends ConsumerStatefulWidget {
  const ProfileEditScreen({super.key});

  @override
  ConsumerState<ProfileEditScreen> createState() => _ProfileEditScreenState();
}

const double _kAvatarRadius = 50.0;

class _ProfileEditScreenState extends ConsumerState<ProfileEditScreen> {
  final _formKey = GlobalKey<FormState>();

  // Display fields
  String header = '';
  String userName = '';
  String email = '';
  String mobile = '';
  String city = '';
  String job = '';

  /// null 是沒有填。
  Gender? gender;
  DateTime? birth;

  // Inputs
  final TextEditingController _userNameCtrl = TextEditingController();
  final TextEditingController _emailCtrl = TextEditingController();
  final TextEditingController _mobileCtrl = TextEditingController();
  final TextEditingController _cityCtrl = TextEditingController();
  final TextEditingController _jobCtrl = TextEditingController();

  final FocusNode _userNameFocus = FocusNode();
  final FocusNode _mobileFocus = FocusNode();
  final FocusNode _jobFocus = FocusNode();

  bool _busy = false;
  String? _deleteMessage;
  BuyerProfile? _profile;
  bool _loaded = false;
  void _logSave(String message, {Object? error, StackTrace? stackTrace}) {
    if (error != null) {
      HKLog.logE('[ProfileEditSave] $message', error, stackTrace);
    } else {
      HKLog.logD('[ProfileEditSave] $message');
    }
  }

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _userNameCtrl.dispose();
    _emailCtrl.dispose();
    _mobileCtrl.dispose();
    _cityCtrl.dispose();
    _jobCtrl.dispose();

    _userNameFocus.dispose();
    _mobileFocus.dispose();
    _jobFocus.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final session = await ref.read(sessionProvider.future);
    final usable = hasUsableSession(session);
    final sessionNickname = usable ? (session!.nickname ?? '') : '';

    BuyerProfile? profile;
    try {
      if (usable && session!.userId.isNotEmpty) {
        profile = await ref.read(authRepositoryProvider).tryGetProfile();
      }
    } catch (_) {}

    String? loginEmail;
    try {
      loginEmail = (await ref.read(clerkSessionProvider.future)).email;
    } catch (e, st) {
      HKLog.logE('[ProfileEdit] 讀取登入信箱失敗', e, st);
    }
    if (!mounted) return;
    setState(() {
      _profile = profile;
      _loaded = true;
      header = profile?.avatarUrl ?? '';
      userName = profile?.displayName ?? sessionNickname;
      email = loginEmail ?? '';
      mobile = profile?.phone ?? '';
      city = profile?.city ?? '';
      job = profile?.job ?? '';
      gender = profile?.gender;
      birth = dateToDateTime(profile?.birthDate);
    });
    _syncControllers();
  }

  // 這裡原有 _resolveEmail（5 個候選鍵）與 _resolveUserName（8 個候選鍵）。
  // 兩者都只餵 `GET /app/buyers/me`，而它只回 `email` 與 `nickname`。
  //
  // _resolveUserName 的候選鏈還會在 nickname 為空時退回 email 或 mobile，
  // 把它填進「用戶名稱」輸入框——使用者按下儲存，那個 email 就會被當成暱稱
  // 寫進 buyers.display_name。空的欄位顯示為空才是對的。
  void _syncControllers() {
    _userNameCtrl.text = userName;
    _emailCtrl.text = email;
    _mobileCtrl.text = mobile;
    _cityCtrl.text = city;
    _jobCtrl.text = job;
  }

  // 這裡原有 _firstNonEmpty：給一組候選鍵，在整個 JSON 樹裡**遞迴**尋找，
  // 任何一層的 Map 或 List 命中就回傳。它是上面那兩段猜測的引擎，
  // 唯一的兩個呼叫點（provider 判斷、Google 名字覆蓋）都已移除。
  // 這種「在任意深度找任意鍵」的取值方式沒有可預期的行為——同名欄位出現在
  // 巢狀結構的任何位置都會被當成答案，而且永遠不會報錯。

  Future<void> _pickAvatar() async {
    final picker = ImagePicker();
    final x = await picker.pickImage(
      source: ImageSource.gallery,
      imageQuality: 80,
    );
    if (x == null) return;
    final bytes = await x.readAsBytes();
    final previewImage = MemoryImage(bytes);

    final fileName = x.path.split('/').last;
    HKLog.logD(
      '[AvatarUpload] Picked image file=$fileName bytes=${bytes.length}',
    );

    if (!mounted) return;
    final use = await showDialog<bool>(
      context: context,
      builder: (_) => AlertDialog(
        title: const Text('使用此照片作為頭像？'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            CircleAvatar(
              radius: _kAvatarRadius,
              backgroundImage: previewImage,
              backgroundColor: AppColors.surfacePanel,
            ),
            const SizedBox(height: HkSpace.md),
            const Text('預覽為實際頭像顯示比例'),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const HkButtonLabel('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const HkButtonLabel('使用'),
          ),
        ],
      ),
    );
    if (use != true) {
      HKLog.logD('[AvatarUpload] User cancelled avatar upload after preview');
      return;
    }

    if (!mounted) return;
    HKLog.logD('[AvatarUpload] Confirmed, start avatar session upload');

    setState(() => _busy = true);
    try {
      final mediaRepo = ref.read(mediaRepositoryProvider);
      final result = await mediaRepo.uploadImage(file: File(x.path));
      final stagedToken = result.stagedToken.trim();
      if (stagedToken.isEmpty) {
        HKLog.logW('[AvatarUpload] Upload finished but no stagedToken');
        if (mounted) {
          showCommonSnackBar(context, '上傳失敗，請稍後再試');
        }
        return;
      }

      // 用 stagedToken 走 UpdateProfile（avatar_staged_token 進 update mask），取回永久 object_key。
      // 不要存 result.previewUrl（tmp/ 暫時簽章 URL，過一陣子就失效）。
      final objectKey = await ref
          .read(authRepositoryProvider)
          .updateAvatar(
            id: _profile?.id ?? '',
            stagedToken: stagedToken,
            etag: _profile?.etag ?? '',
          );
      HKLog.logD('[AvatarUpload] Finalized object_key=$objectKey');
      setState(() {
        header = objectKey;
      });
      await _load();
      if (mounted) {
        showCommonSnackBar(context, '頭像已更新');
      }
    } catch (e, st) {
      HKLog.logE('[AvatarUpload] Upload error', e, st);
      if (mounted) {
        showCommonSnackBar(
          context,
          e is AppException ? e.message : '上傳失敗，請稍後再試',
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _save() async {
    final form = _formKey.currentState;
    _logSave('Save tapped (form ready=${form != null})');
    if (form == null) {
      _logSave('FormState is null, aborting save');
      return;
    }
    final valid = form.validate();
    _logSave('Form validation result=$valid');
    if (!valid) {
      final missing = _missingRequiredFields();
      if (missing.isNotEmpty) {
        unawaited(_showMissingFieldsDialog(missing));
      }
      if (mounted) {
        showCommonSnackBar(context, '請檢查必填欄位（用戶名稱／手機）');
      }
      return;
    }
    _logSave('Validation passed: start merging form data');
    FocusScope.of(context).unfocus();
    setState(() => _busy = true);

    try {
      final nameValue = _userNameCtrl.text.trim();
      final mobileValue = _mobileCtrl.text.trim();
      final cityValue = _cityCtrl.text.trim();
      final jobValue = _jobCtrl.text.trim();
      final birthValue = birth;
      final profile = _profile;
      if (profile == null) {
        throw const ServerException('個人資料版本資訊不存在，請重新載入');
      }
      // 這個表單的文字欄位不含頭像：頭像走 staged token 流程（_pickAvatar），
      // 同一個 UpdateProfile、另一組 update mask。
      final edit = (
        id: profile.id,
        etag: profile.etag,
        displayName: nameValue,
        phone: mobileValue,
        city: cityValue,
        job: jobValue,
        gender: gender,
        birthDate: birthValue == null ? null : dateFromDateTime(birthValue),
      );

      // Update remote via auth repository
      final session = await ref.read(sessionProvider.future);
      final id = session?.userId;
      _logSave('Remote update started (id=$id)');
      await ref.read(authRepositoryProvider).updateProfile(edit);
      _logSave('Remote update success (id=$id)');
      // Update session nickname so settings screen reflects the change
      await ref.read(sessionProvider.notifier).updateNickname(nameValue);

      if (!mounted) return;
      showCommonSnackBar(context, '已儲存個人資料');
      final popped = await Navigator.maybePop(context, true);
      _logSave('Navigator maybePop returned $popped');
      if (!popped && mounted) {
        _logSave('maybePop failed, popping via context.pop');
        context.pop();
      }
    } catch (e, st) {
      _logSave('Save failed', error: e, stackTrace: st);
      if (!mounted) return;
      showCommonSnackBar(context, e is AppException ? e.message : '儲存失敗，請稍後再試');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  List<String> _missingRequiredFields() {
    final List<String> missing = [];
    if (_userNameCtrl.text.trim().isEmpty) missing.add('用戶名稱');
    if (_mobileCtrl.text.trim().isEmpty) missing.add('手機');
    return missing;
  }

  Future<void> _showMissingFieldsDialog(List<String> fields) async {
    if (!mounted || fields.isEmpty) return;
    final String content = fields.map((e) => '• $e').join('\n');
    await showCommonDialog(
      context: context,
      title: '請完善必填欄位',
      content: '$content\n\n請完整填寫後再儲存。',
      confirmText: '我知道了',
    );
  }

  ImageProvider? _avatarProvider(String s) {
    final v = s.trim();
    if (v.isEmpty) return null;
    if (v.startsWith('http')) {
      return getOssSceneImageProvider(v, ImageScene.avatarLarge) ??
          NetworkImage(v);
    }
    if (v.startsWith('/') ||
        v.startsWith('member/avatar/view') ||
        v.startsWith('member-avatars') ||
        v.startsWith('avatars/') ||
        v.startsWith('tmp/') ||
        v.startsWith('public/')) {
      final full = v.normalizeAvatarUrl();
      if (!full.startsWith('http')) {
        return null; // 無法解析（如 /public/defaults/...）→ 用預設頭像
      }
      return getOssSceneImageProvider(full, ImageScene.avatarLarge) ??
          NetworkImage(full);
    }
    final String pure = v.startsWith('data:image') && v.contains(',')
        ? v.split(',').last
        : v;
    try {
      return MemoryImage(base64Decode(pure));
    } catch (_) {
      return null;
    }
  }

  @override
  Widget build(BuildContext context) {
    final canSave = !_busy && _loaded;
    final l = ref.watch(uiLocaleProvider);
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: buildCommonAppBar(
        context,
        child: Row(
          children: [
            SizedBox(
              width: 64,
              child: Align(
                alignment: Alignment.centerLeft,
                child: HkTapTarget(
                  semanticLabel: '返回',
                  onTap: _busy ? null : () => Navigator.maybePop(context),
                  child: const HkIcon(
                    HkIconName.back,
                    size: HkControl.icon,
                    color: AppIconColors.strong,
                  ),
                ),
              ),
            ),
            Expanded(
              child: Text(
                '個人資料設定',
                textAlign: TextAlign.center,
                style: AppTypography.h1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
            const SizedBox(width: 64),
          ],
        ),
      ),
      body: SafeArea(
        top: false,
        child: !_loaded
            ? const TruckLoadingOverlay()
            : LayoutBuilder(
                builder: (context, constraints) {
                  final width = constraints.maxWidth;
                  final pageGutter = HkBreakpoints.pageGutter(width);
                  final centeredGutter = (width - HkControl.chromeMaxWidth) / 2;
                  final horizontalPadding = centeredGutter > pageGutter
                      ? centeredGutter
                      : pageGutter;
                  return Form(
                    key: _formKey,
                    child: AutofillGroup(
                      child: FocusTraversalGroup(
                        child: ListView(
                          key: const PageStorageKey<String>(
                            'profile-edit-form',
                          ),
                          keyboardDismissBehavior:
                              ScrollViewKeyboardDismissBehavior.onDrag,
                          padding: EdgeInsets.fromLTRB(
                            horizontalPadding,
                            HkSpace.lg,
                            horizontalPadding,
                            MediaQuery.viewInsetsOf(context).bottom +
                                HkSpace.xxl,
                          ),
                          children: [
                            ProfileAvatarEditor(
                              avatar: _avatarProvider(header),
                              busy: _busy,
                              onPressed: _busy ? null : _pickAvatar,
                              displayName: _userNameCtrl.text,
                            ),
                            const SizedBox(height: HkSpace.lg),
                            ProfileSectionCard(
                              title: '帳號與聯絡方式',
                              description: '必填資料會用於帳號辨識與重要通知。',
                              icon: HkIconName.user,
                              badgeLabel: '必填',
                              children: [
                                ProfileTextField(
                                  label: '用戶名稱',
                                  controller: _userNameCtrl,
                                  focusNode: _userNameFocus,
                                  hintText: '請輸入用戶名稱',
                                  keyboardType: TextInputType.name,
                                  textInputAction: TextInputAction.next,
                                  autofillHints: const [AutofillHints.name],
                                  icon: HkIconName.user,
                                  maxLength: 30,
                                  required: true,
                                  enabled: !_busy,
                                  onChanged: (_) => setState(() {}),
                                  validator: (value) =>
                                      value == null || value.trim().isEmpty
                                      ? '請輸入用戶名稱'
                                      : null,
                                ),
                                if (email.isNotEmpty)
                                  ProfileTextField(
                                    label: '登入信箱',
                                    controller: _emailCtrl,
                                    icon: HkIconName.document,
                                    helperText: '登入信箱目前無法在此變更',
                                    readOnly: true,
                                    enabled: !_busy,
                                  ),
                                ProfileTextField(
                                  label: '手機號碼',
                                  controller: _mobileCtrl,
                                  focusNode: _mobileFocus,
                                  hintText: '例如：0912 345 678',
                                  keyboardType: TextInputType.phone,
                                  textInputAction: TextInputAction.next,
                                  autofillHints: const [
                                    AutofillHints.telephoneNumber,
                                  ],
                                  icon: HkIconName.phone,
                                  inputFormatters: [
                                    FilteringTextInputFormatter.digitsOnly,
                                    LengthLimitingTextInputFormatter(10),
                                  ],
                                  required: true,
                                  enabled: !_busy,
                                  validator: (value) {
                                    if (value == null || value.trim().isEmpty) {
                                      return '請輸入手機號碼';
                                    }
                                    final normalized = value.trim();
                                    if (!RegExp(
                                      r'^0?9\d{8}$',
                                    ).hasMatch(normalized)) {
                                      return '請輸入 09 或 9 開頭的手機號碼';
                                    }
                                    return null;
                                  },
                                ),
                              ],
                            ),
                            const SizedBox(height: HkSpace.lg),
                            ProfileSectionCard(
                              title: '其他個人資料',
                              description: '選填資料可協助提供更符合所在地的內容。',
                              icon: HkIconName.edit,
                              badgeLabel: '選填',
                              children: [
                                ProfileResponsivePair(
                                  first: ProfileDropdownField<Gender?>(
                                    label: '性別',
                                    value: gender,
                                    placeholder: buyerProfileGenderUnset(l),
                                    items: _genderChoices,
                                    itemLabel: (g) => _genderText(g, l),
                                    onSelected: _selectGender,
                                    enabled: !_busy,
                                  ),
                                  second: ProfileSelectionField(
                                    label: '生日',
                                    value: birth == null ? '' : _birthLabel(),
                                    placeholder: '選擇日期',
                                    icon: HkIconName.calendar,
                                    onTap: _busy ? null : _pickBirthDate,
                                    enabled: !_busy,
                                  ),
                                ),
                                ProfileDropdownField<String>(
                                  label: '縣市',
                                  value: _cityCtrl.text.trim().isEmpty
                                      ? null
                                      : _cityCtrl.text.trim(),
                                  placeholder: '選擇縣市',
                                  items: [for (final c in counties) c.name],
                                  itemLabel: (name) => name,
                                  icon: HkIconName.location,
                                  showSearch: true,
                                  onSelected: _selectCity,
                                  enabled: !_busy,
                                ),
                                ProfileTextField(
                                  label: '職業',
                                  controller: _jobCtrl,
                                  focusNode: _jobFocus,
                                  hintText: '選填，例如：設計師',
                                  icon: HkIconName.document,
                                  maxLength: 30,
                                  textInputAction: TextInputAction.done,
                                  enabled: !_busy,
                                ),
                              ],
                            ),
                            const SizedBox(height: HkSpace.lg),
                            ProfileDeleteAccountSection(
                              busy: _busy,
                              onPressed: _confirmAndDeleteAccount,
                              message: _deleteMessage,
                            ),
                            const SizedBox(height: HkSpace.sm),
                          ],
                        ),
                      ),
                    ),
                  );
                },
              ),
      ),
      bottomNavigationBar: !_loaded
          ? null
          : ProfileSaveBar(enabled: canSave, busy: _busy, onPressed: _save),
    );
  }

  Future<void> _confirmAndDeleteAccount() async {
    if (_busy) return;
    final confirmed = await showCommonDialog(
      context: context,
      title: '刪除帳號',
      content:
          '刪除後帳號資料會被移除，無法復原。還有未完成的訂單時，帳號會在最後一筆訂單完成後刪除，期間無法新增收藏與追蹤。\n\n若你有開設商店，商店也會一起關閉，商店與商品不再公開，賣家身分會移除。有未結訂單時，帳號與商店會在訂單結清後才刪除，期間商店不能自行關閉或換店。',
      confirmText: '確認刪除',
      cancelText: '取消',
      confirmColor: AppColors.danger,
      barrierDismissible: true,
    );
    if (confirmed != true || !mounted) return;

    setState(() {
      _busy = true;
      _deleteMessage = null;
    });
    final AccountDeletionOutcome outcome;
    try {
      outcome = await ref.read(accountDeletionProvider).run();
    } finally {
      if (mounted) setState(() => _busy = false);
    }
    if (!mounted) return;
    switch (outcome) {
      case AccountDeleted():
        await ref.read(sessionProvider.notifier).logout();
        if (!mounted) return;
        context.go('/');
      case AccountDeletionPending():
        setState(() => _deleteMessage = accountPendingOrdersMessage);
      case AccountDeletionFailed(:final message):
        setState(() => _deleteMessage = message);
    }
  }

  void _selectCity(String selected) {
    if (_busy || selected == _cityCtrl.text.trim()) return;
    setState(() {
      _cityCtrl.text = selected;
      city = selected;
    });
  }

  /// 第一項 null 是「沒有填」，選了就清除性別；其餘由生成的 enum 推導，
  /// 目錄要求每個非零值都有文字。
  static final List<Gender?> _genderChoices = [
    null,
    ...Gender.values.where((g) => g != Gender.GENDER_UNSPECIFIED),
  ];

  /// 沒有文字的值（不認得的值）呈現同沒有設值（API 規範，optional enum）。
  static String _genderText(Gender? g, UiLocale l) =>
      g?.uiText(l) ?? buyerProfileGenderUnset(l);

  void _selectGender(Gender? selected) {
    if (_busy) return;
    if (selected != gender) setState(() => gender = selected);
  }

  String _birthLabel() {
    if (birth == null) return '請填寫年月日';
    final d = birth!;
    String two(int v) => v.toString().padLeft(2, '0');
    return '${d.year}-${two(d.month)}-${two(d.day)}';
  }

  Future<void> _pickBirthDate() async {
    final now = DateTime.now();
    final initial = birth ?? DateTime(now.year - 18, now.month, now.day);
    final picked = await AppDatePicker.show(
      context,
      initial: initial,
      firstDate: DateTime(1900, 1, 1),
      lastDate: now,
    );
    if (picked != null && mounted) {
      setState(() => birth = picked);
    }
  }
}
