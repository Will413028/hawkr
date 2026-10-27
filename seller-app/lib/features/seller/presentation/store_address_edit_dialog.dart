import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_twaddr/hawkr_twaddr.dart' show countyCodeOfTownCode;
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../data/store_form.dart';
import 'town_picker.dart';

/// 儲存地址：[townCode] 與 [addressDetail] 都是空字串表示清掉地址。成功回傳 null（對話框關閉），失敗回傳要顯示在對話框裡的文字。
typedef StoreAddressSave =
    Future<String?> Function(String townCode, String addressDetail);

/// 地址編輯對話框：縣市、鄉鎮市區（內政部代碼）加地址明細。按「保存」時呼叫 [onSave]，失敗時對話框留著、輸入保留。
///
/// 返回 `true` 表示已儲存；取消則返回 `null`。
class StoreAddressEditDialog extends StatefulWidget {
  const StoreAddressEditDialog({
    super.key,
    required this.initialTownCode,
    required this.initialDetail,
    required this.onSave,
    this.title = '編輯店家地址',
  });

  final String title;
  final String initialTownCode;
  final String initialDetail;
  final StoreAddressSave onSave;

  @override
  State<StoreAddressEditDialog> createState() => _StoreAddressEditDialogState();
}

class _StoreAddressEditDialogState extends State<StoreAddressEditDialog> {
  late String _countyCode = widget.initialTownCode.isEmpty
      ? ''
      : countyCodeOfTownCode(widget.initialTownCode);
  late String _townCode = widget.initialTownCode;
  late final TextEditingController _detailController = TextEditingController(
    text: widget.initialDetail,
  );
  String? _error;
  bool _saving = false;

  @override
  void dispose() {
    _detailController.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    if (_saving) return;
    final detail = _detailController.text.trim();
    final invalid = validateAddressInput(
      townCode: _townCode,
      addressDetail: detail,
    );
    if (invalid != null) {
      setState(() => _error = invalid);
      return;
    }
    setState(() {
      _saving = true;
      _error = null;
    });
    final failure = await widget.onSave(_townCode, detail);
    if (!mounted) return;
    if (failure == null) {
      Navigator.of(context).pop(true);
      return;
    }
    setState(() {
      _saving = false;
      _error = failure;
    });
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
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
                borderRadius: BorderRadius.vertical(top: Radius.circular(24.r)),
              ),
              child: Column(
                children: [
                  Text(
                    widget.title,
                    style: TextStyle(
                      fontSize: 18.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textEmphasis,
                    ),
                  ),
                  SizedBox(height: 16.h),
                  TownPicker(
                    countyCode: _countyCode,
                    townCode: _townCode,
                    gap: 8.w,
                    onChanged: (county, town) => setState(() {
                      _countyCode = county;
                      _townCode = town;
                      _error = null;
                    }),
                  ),
                  SizedBox(height: 10.h),
                  TextField(
                    controller: _detailController,
                    autofocus: true,
                    maxLength: 120,
                    decoration: InputDecoration(
                      hintText: '請輸入地址',
                      hintStyle: TextStyle(color: AppColors.textSecondary),
                      counterText: '',
                      filled: true,
                      fillColor: AppColors.gray100,
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(12.r),
                        borderSide: BorderSide.none,
                      ),
                      contentPadding: EdgeInsets.all(12.w),
                    ),
                  ),
                  if (_error != null) ...[
                    SizedBox(height: 8.h),
                    Align(
                      alignment: Alignment.centerLeft,
                      child: Text(
                        _error!,
                        key: const Key('profile_edit_error'),
                        style: TextStyle(
                          fontSize: 14.sp,
                          color: AppColors.danger,
                        ),
                      ),
                    ),
                  ],
                ],
              ),
            ),
            const Divider(height: 0, thickness: 1, color: AppColors.gray300),
            // ── 按鈕列 ──────────────────────────────────
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
                      onTap: _saving ? null : () => Navigator.of(context).pop(),
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
                      onTap: _saving ? null : _save,
                      child: SizedBox(
                        height: 54.h,
                        child: Center(
                          child: Text(
                            '保存',
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
    );
  }
}
