import '../../../shared/utils/platform_io.dart';
import 'universal_ocr_service.dart';
import '../../../shared/utils/hk_log.dart';

/// 台灣身分證 OCR 辨識結果
class IdCardOcrResult {
  final String? name; // 姓名
  final String? idNumber; // 身分證字號
  final String? birthDate; // 出生日期 (民國年/月/日)
  final String? issueDate; // 發證日期 (民國年/月/日)
  final String? issueType; // 發證類型 (初發/補發/換發)
  final String? issueLocation; // 發證地點
  final String? rawText; // 原始辨識文字
  final OcrEngine? engine; // 使用的 OCR 引擎

  IdCardOcrResult({
    this.name,
    this.idNumber,
    this.birthDate,
    this.issueDate,
    this.issueType,
    this.issueLocation,
    this.rawText,
    this.engine,
  });

  bool get hasData => name != null || idNumber != null || birthDate != null;

  @override
  String toString() {
    // 個資合規（issue #608 §2）：姓名／身分證字號／出生日期屬敏感個資，
    // 一律遮蔽，避免 toString 被任何 log 字串插值時洩漏 PII。
    String mask(String? v) => v == null ? 'null' : '***';
    return 'IdCardOcrResult(name: ${mask(name)}, idNumber: ${mask(idNumber)}, '
        'birthDate: ${mask(birthDate)}, issueDate: $issueDate, issueType: $issueType, '
        'issueLocation: $issueLocation, engine: $engine)';
  }
}

/// 台灣身分證 OCR 服務
///
/// 整合 UniversalOcrService：
/// - iOS 平台：使用原生 Vision 框架
class IdCardOcrService {
  /// 辨識身分證正面
  /// 正面包含：姓名、身分證字號、出生日期
  static Future<IdCardOcrResult> recognizeFront(File imageFile) async {
    try {
      HKLog.logD('[IdCardOCR] Recognizing front...');
      final result = await UniversalOcrService.recognizeText(imageFile);

      if (!result.success) {
        HKLog.logD('[IdCardOCR] Front OCR error: ${result.errorMessage}');
        return IdCardOcrResult(
          rawText: 'Error: ${result.errorMessage}',
          engine: result.engine,
        );
      }

      // 個資合規（issue #608 §2）：OCR 原始文字含姓名／身分證字號／出生日期，
      // 嚴禁落 log；僅記錄長度供除錯判斷是否有辨識到文字。
      HKLog.logD(
        '[IdCardOCR] Front OCR raw text redacted (len=${result.text.length})',
      );
      final parsed = _parseFrontCard(result.text);
      return IdCardOcrResult(
        name: parsed.name,
        idNumber: parsed.idNumber,
        birthDate: parsed.birthDate,
        issueDate: parsed.issueDate,
        issueType: parsed.issueType,
        issueLocation: parsed.issueLocation,
        rawText: result.text,
        engine: result.engine,
      );
    } catch (e) {
      HKLog.logD('[IdCardOCR] Error recognizing front: $e');
      return IdCardOcrResult(rawText: 'Error: $e');
    }
  }

  /// 辨識身分證反面
  /// 反面包含：發證日期、發證類型、發證地點
  static Future<IdCardOcrResult> recognizeBack(File imageFile) async {
    try {
      HKLog.logD('[IdCardOCR] Recognizing back...');
      final result = await UniversalOcrService.recognizeText(imageFile);

      if (!result.success) {
        HKLog.logD('[IdCardOCR] Back OCR error: ${result.errorMessage}');
        return IdCardOcrResult(
          rawText: 'Error: ${result.errorMessage}',
          engine: result.engine,
        );
      }

      // 個資合規（issue #608 §2）：OCR 原始文字禁止落 log，僅記錄長度。
      HKLog.logD(
        '[IdCardOCR] Back OCR raw text redacted (len=${result.text.length})',
      );
      final parsed = _parseBackCard(result.text);
      return IdCardOcrResult(
        issueDate: parsed.issueDate,
        issueType: parsed.issueType,
        issueLocation: parsed.issueLocation,
        rawText: result.text,
        engine: result.engine,
      );
    } catch (e) {
      HKLog.logD('[IdCardOCR] Error recognizing back: $e');
      return IdCardOcrResult(rawText: 'Error: $e');
    }
  }

  // ============================================================
  // Parsing Logic (Shared)
  // ============================================================

  /// 合併正反面辨識結果
  /// 優先使用正面的資料，若正面沒有則使用反面的
  static IdCardOcrResult mergeResults(
    IdCardOcrResult front,
    IdCardOcrResult back,
  ) {
    return IdCardOcrResult(
      name: front.name ?? back.name,
      idNumber: front.idNumber ?? back.idNumber,
      birthDate: front.birthDate ?? back.birthDate,
      // 發證資料：正面有就用正面的，否則用反面的
      issueDate: front.issueDate ?? back.issueDate,
      issueType: front.issueType ?? back.issueType,
      issueLocation: front.issueLocation ?? back.issueLocation,
      rawText: '${front.rawText ?? ''}\n---\n${back.rawText ?? ''}',
      engine: front.engine,
    );
  }

  /// 解析身分證正面文字
  static IdCardOcrResult _parseFrontCard(String text) {
    String? name;
    String? idNumber;
    String? birthDate;
    String? issueDate;
    String? issueType;
    String? issueLocation;

    final lines = text.split('\n');

    // 提取身分證字號 (1個英文字母 + 9個數字)
    final idRegex = RegExp(r'[A-Z][12]\d{8}');
    final idMatch = idRegex.firstMatch(text);
    if (idMatch != null) {
      idNumber = idMatch.group(0);
      // 個資合規（issue #608 §2）：身分證字號禁止落 log。
      HKLog.logD('[IdCardOCR] Found ID number (redacted)');
    }

    // 提取出生日期 - 支援多種格式
    // 格式: 民國XX年X月X日, XX年X月X日, XX/XX/XX
    final birthPatterns = [
      RegExp(r'民[國国]?\s*(\d{2,3})\s*年\s*(\d{1,2})\s*月\s*(\d{1,2})\s*日'),
      RegExp(r'(\d{2,3})\s*年\s*(\d{1,2})\s*月\s*(\d{1,2})\s*日'),
      RegExp(r'(\d{2,3})[/\.\-](\d{1,2})[/\.\-](\d{1,2})'),
    ];

    for (final line in lines) {
      // 尋找包含「出生」或「出线」(OCR誤識) 的行
      if (line.contains('出生') ||
          line.contains('出线') ||
          line.contains('生日') ||
          line.contains('BIRTH') ||
          line.contains('年月')) {
        for (final pattern in birthPatterns) {
          final match = pattern.firstMatch(line);
          if (match != null) {
            final year = match.group(1)!.padLeft(3, '0');
            final month = match.group(2)!.padLeft(2, '0');
            final day = match.group(3)!.padLeft(2, '0');
            birthDate = '$year/$month/$day';
            // 個資合規（issue #608 §2）：出生日期禁止落 log。
            HKLog.logD('[IdCardOCR] Found birth date (redacted)');
            break;
          }
        }
        if (birthDate != null) break;
      }
    }

    // 如果還沒找到出生日期，用第一個日期格式（排除發證日期）
    if (birthDate == null) {
      for (final line in lines) {
        // 跳過發證相關的行
        if (line.contains('發') ||
            line.contains('核發') ||
            line.contains('換發') ||
            line.contains('補發') ||
            line.contains('初發')) {
          continue;
        }
        for (final pattern in birthPatterns) {
          final match = pattern.firstMatch(line);
          if (match != null) {
            final year = match.group(1)!.padLeft(3, '0');
            final month = match.group(2)!.padLeft(2, '0');
            final day = match.group(3)!.padLeft(2, '0');
            birthDate = '$year/$month/$day';
            // 個資合規（issue #608 §2）：出生日期禁止落 log。
            HKLog.logD('[IdCardOCR] Found birth date (fallback, redacted)');
            break;
          }
        }
        if (birthDate != null) break;
      }
    }

    // 提取發證日期和類型 (新版身分證正面可能有)
    // 格式: "發控日 民M 94年7月1日（北市）換發" 或 "民國94年7月1日 換發"
    for (final line in lines) {
      // 檢查是否是發證相關的行
      if (line.contains('發控') ||
          line.contains('發證') ||
          line.contains('核發') ||
          line.contains('換發') ||
          line.contains('補發') ||
          line.contains('初發')) {
        // 個資合規（issue #608 §2）：原始行文字可能含姓名／出生日期，禁止落 log。
        HKLog.logD('[IdCardOCR] Processing issue line (redacted)');

        // 提取發證類型
        if (line.contains('初發') || line.contains('初 發')) {
          issueType = '初發';
          HKLog.logD('[IdCardOCR] Found issue type from front: $issueType');
        } else if (line.contains('補發') || line.contains('補 發')) {
          issueType = '補發';
          HKLog.logD('[IdCardOCR] Found issue type from front: $issueType');
        } else if (line.contains('換發') || line.contains('換 發')) {
          issueType = '換發';
          HKLog.logD('[IdCardOCR] Found issue type from front: $issueType');
        }

        // 提取發證日期
        for (final pattern in birthPatterns) {
          final match = pattern.firstMatch(line);
          if (match != null) {
            final year = match.group(1)!.padLeft(3, '0');
            final month = match.group(2)!.padLeft(2, '0');
            final day = match.group(3)!.padLeft(2, '0');
            issueDate = '$year/$month/$day';
            HKLog.logD('[IdCardOCR] Found issue date from front: $issueDate');
            break;
          }
        }

        // 提取發證地點 (括號內的縣市)
        // 支援：（北市）、(北市)、（臺北市）
        final locationInParenRegex = RegExp(r'[（\(]([^）\)]+)[）\)]');
        final locMatch = locationInParenRegex.firstMatch(line);
        if (locMatch != null) {
          var loc = locMatch.group(1);
          // 擴展縮寫
          issueLocation = _expandLocationAbbrev(loc);
          HKLog.logD(
            '[IdCardOCR] Found issue location from front: $issueLocation',
          );
        }
      }
    }

    // 提取姓名 - 改進策略
    // 1. 先找「姓名」或「妹名」(OCR誤識) 後面的文字
    // 2. 找被拆分的姓名片段並合併
    name = _extractName(text, lines);

    return IdCardOcrResult(
      name: name,
      idNumber: idNumber,
      birthDate: birthDate,
      issueDate: issueDate,
      issueType: issueType,
      issueLocation: issueLocation,
      rawText: text,
    );
  }

  /// 從 OCR 文字中提取姓名
  static String? _extractName(String text, List<String> lines) {
    String? name;

    // 方法1: 先嘗試找連續的單字行並合併 (處理被拆開的姓名如「陳」「筱」「玲」)
    // 這是最可靠的方法，因為身分證姓名通常會被 OCR 拆成單字
    List<String> singleCharLines = [];
    bool foundNameLabel = false;

    for (int i = 0; i < lines.length; i++) {
      final line = lines[i].trim();

      // 檢查是否是姓名標籤行（包含「妹名」或「姓名」）
      if (line.contains('姓名') || line.contains('妹名')) {
        foundNameLabel = true;
        // 嘗試從這行提取姓氏（如「妹名陳」→「陳」）
        final labelNameMatch = RegExp(
          r'(?:姓|妹)\s*名\s*([\u4e00-\u9fa5])',
        ).firstMatch(line);
        if (labelNameMatch != null) {
          singleCharLines.add(labelNameMatch.group(1)!);
        }
        continue;
      }

      // 如果已經找到姓名標籤，開始收集後續的單字
      if (foundNameLabel) {
        // 單個中文字
        if (RegExp(r'^[\u4e00-\u9fa5]$').hasMatch(line)) {
          singleCharLines.add(line);
        } else if (singleCharLines.isNotEmpty) {
          // 遇到非單字行，停止收集
          break;
        }
      }
    }

    // 檢查收集到的姓名
    if (singleCharLines.length >= 2 && singleCharLines.length <= 4) {
      final candidateName = singleCharLines.join();
      if (!_isExcludedWord(candidateName)) {
        name = candidateName;
        // 個資合規（issue #608 §2）：姓名禁止落 log。
        HKLog.logD(
          '[IdCardOCR] Found name from label + single chars (redacted)',
        );
        return name;
      }
    }

    // 方法2: 如果方法1失敗，找所有連續的單字行
    singleCharLines.clear();
    for (int i = 0; i < lines.length; i++) {
      final line = lines[i].trim();
      // 單個中文字（排除常見單字如「父」「母」等）
      if (RegExp(r'^[\u4e00-\u9fa5]$').hasMatch(line)) {
        final char = line;
        // 排除常見的非姓名單字
        if (!['父', '母', '女', '男', '樣', '本'].contains(char)) {
          singleCharLines.add(char);
        }
      } else if (singleCharLines.isNotEmpty) {
        // 遇到非單字行，檢查收集的單字
        if (singleCharLines.length >= 2 && singleCharLines.length <= 4) {
          final candidateName = singleCharLines.join();
          if (!_isExcludedWord(candidateName)) {
            name = candidateName;
            // 個資合規（issue #608 §2）：姓名禁止落 log。
            HKLog.logD(
              '[IdCardOCR] Found name from consecutive single chars (redacted)',
            );
            return name;
          }
        }
        singleCharLines.clear();
      }
    }
    // 檢查最後收集的單字
    if (singleCharLines.length >= 2 && singleCharLines.length <= 4) {
      final candidateName = singleCharLines.join();
      if (!_isExcludedWord(candidateName)) {
        name = candidateName;
        // 個資合規（issue #608 §2）：姓名禁止落 log。
        HKLog.logD(
          '[IdCardOCR] Found name from trailing single chars (redacted)',
        );
        return name;
      }
    }

    // 方法3: 找「姓名」或類似標籤後的文字（完整的2-4字）
    final namePatterns = [
      RegExp(r'(?:姓\s*名|妹\s*名)[：:\s]*([\u4e00-\u9fa5]{2,4})'),
    ];

    for (final pattern in namePatterns) {
      final match = pattern.firstMatch(text);
      if (match != null) {
        final candidate = match.group(1)?.trim();
        if (candidate != null &&
            candidate.isNotEmpty &&
            !_isExcludedWord(candidate)) {
          name = candidate;
          // 個資合規（issue #608 §2）：姓名禁止落 log。
          HKLog.logD('[IdCardOCR] Found name after label (redacted)');
          return name;
        }
      }
    }

    // 方法4: 備用 - 找2-4字的中文字串（排除常見詞彙）
    final chineseNameRegex = RegExp(r'[\u4e00-\u9fa5]{2,4}');
    final chineseMatches = chineseNameRegex.allMatches(text).toList();

    for (final match in chineseMatches) {
      final candidate = match.group(0)!;
      if (!_isExcludedWord(candidate)) {
        name = candidate;
        // 個資合規（issue #608 §2）：姓名禁止落 log。
        HKLog.logD('[IdCardOCR] Found name (fallback, redacted)');
        return name;
      }
    }

    return name;
  }

  /// 檢查是否為應排除的詞彙
  static bool _isExcludedWord(String word) {
    const excludeWords = [
      '身分證',
      '中華民國',
      '國民身分',
      '統一編號',
      '出生日期',
      '身份證',
      '中華民困',
      '閻民',
      '樣本',
      '出线',
      '注別',
      '發控日',
      '民國',
      '年月',
      '配偶',
      '役別',
      '出生',
      '住址',
      '父母',
      '監護人',
    ];
    return excludeWords.any((w) => word.contains(w) || w.contains(word));
  }

  /// 擴展縣市縮寫
  static String? _expandLocationAbbrev(String? abbrev) {
    if (abbrev == null) return null;
    const abbrevMap = {
      '北市': '臺北市',
      '新北': '新北市',
      '桃市': '桃園市',
      '中市': '臺中市',
      '南市': '臺南市',
      '高市': '高雄市',
      '基市': '基隆市',
      '竹市': '新竹市',
      '嘉市': '嘉義市',
      '竹縣': '新竹縣',
      '苗縣': '苗栗縣',
      '彰縣': '彰化縣',
      '投縣': '南投縣',
      '雲縣': '雲林縣',
      '嘉縣': '嘉義縣',
      '屏縣': '屏東縣',
      '宜縣': '宜蘭縣',
      '花縣': '花蓮縣',
      '東縣': '臺東縣',
      '澎縣': '澎湖縣',
      '金縣': '金門縣',
      '連江': '連江縣',
    };
    for (final entry in abbrevMap.entries) {
      if (abbrev.contains(entry.key)) {
        return entry.value;
      }
    }
    // 統一用「臺」
    return abbrev.replaceAll('台', '臺');
  }

  /// 解析身分證反面文字
  static IdCardOcrResult _parseBackCard(String text) {
    String? issueDate;
    String? issueType;
    String? issueLocation;

    final lines = text.split('\n');

    // 提取發證日期 - 支援多種格式
    final datePatterns = [
      RegExp(r'民[國国]?\s*(\d{2,3})\s*年\s*(\d{1,2})\s*月\s*(\d{1,2})\s*日'),
      RegExp(r'(\d{2,3})\s*年\s*(\d{1,2})\s*月\s*(\d{1,2})\s*日'),
      RegExp(r'(\d{2,3})[/\.\-](\d{1,2})[/\.\-](\d{1,2})'),
    ];

    for (final line in lines) {
      if (line.contains('發證') ||
          line.contains('發控') ||
          line.contains('核發') ||
          line.contains('ISSUE')) {
        for (final pattern in datePatterns) {
          final match = pattern.firstMatch(line);
          if (match != null) {
            final year = match.group(1)!.padLeft(3, '0');
            final month = match.group(2)!.padLeft(2, '0');
            final day = match.group(3)!.padLeft(2, '0');
            issueDate = '$year/$month/$day';
            HKLog.logD('[IdCardOCR] Found issue date: $issueDate');
            break;
          }
        }
        if (issueDate != null) break;
      }
    }

    // 備用：找最後一個日期格式（通常是發證日期）
    if (issueDate == null) {
      for (final pattern in datePatterns) {
        final allDates = pattern.allMatches(text).toList();
        if (allDates.isNotEmpty) {
          final match = allDates.last;
          final year = match.group(1)!.padLeft(3, '0');
          final month = match.group(2)!.padLeft(2, '0');
          final day = match.group(3)!.padLeft(2, '0');
          issueDate = '$year/$month/$day';
          HKLog.logD('[IdCardOCR] Found issue date (fallback): $issueDate');
          break;
        }
      }
    }

    // 提取發證類型
    if (text.contains('初發') || text.contains('初 發')) {
      issueType = '初發';
    } else if (text.contains('補發') || text.contains('補 發')) {
      issueType = '補發';
    } else if (text.contains('換發') || text.contains('換 發')) {
      issueType = '換發';
    }
    HKLog.logD('[IdCardOCR] Found issue type: $issueType');

    // 提取發證地點（台灣縣市）- 完整名稱
    final locationRegex = RegExp(
      r'(臺北市|新北市|桃園市|臺中市|臺南市|高雄市|'
      r'基隆市|新竹市|嘉義市|新竹縣|苗栗縣|彰化縣|'
      r'南投縣|雲林縣|嘉義縣|屏東縣|宜蘭縣|花蓮縣|'
      r'臺東縣|澎湖縣|金門縣|連江縣|'
      r'台北市|台中市|台南市|台東縣)',
    );
    final locationMatch = locationRegex.firstMatch(text);
    if (locationMatch != null) {
      issueLocation = locationMatch.group(0);
      // 統一用「臺」
      issueLocation = issueLocation?.replaceAll('台', '臺');
      HKLog.logD('[IdCardOCR] Found issue location: $issueLocation');
    }

    // 備用：從「出生X」或括號內找縣市縮寫
    if (issueLocation == null) {
      // 處理「出生北」→「臺北市」這種情況
      for (final line in lines) {
        if (line.contains('出生')) {
          final abbrevMatch = RegExp(
            r'出生\s*(北|中|南|東|高|基|竹|嘉|苗|彰|投|雲|屏|宜|花|澎|金|連)',
          ).firstMatch(line);
          if (abbrevMatch != null) {
            final abbrev = abbrevMatch.group(1);
            issueLocation = _expandSingleCharLocation(abbrev);
            if (issueLocation != null) {
              HKLog.logD(
                '[IdCardOCR] Found issue location from birth place: $issueLocation',
              );
            }
          }
        }
      }
    }

    return IdCardOcrResult(
      issueDate: issueDate,
      issueType: issueType,
      issueLocation: issueLocation,
      rawText: text,
    );
  }

  /// 擴展單字縣市縮寫
  static String? _expandSingleCharLocation(String? abbrev) {
    if (abbrev == null) return null;
    const singleCharMap = {
      '北': '臺北市',
      '中': '臺中市',
      '南': '臺南市',
      '東': '臺東縣',
      '高': '高雄市',
      '基': '基隆市',
      '竹': '新竹市', // 可能是新竹市或新竹縣
      '嘉': '嘉義市', // 可能是嘉義市或嘉義縣
      '苗': '苗栗縣',
      '彰': '彰化縣',
      '投': '南投縣',
      '雲': '雲林縣',
      '屏': '屏東縣',
      '宜': '宜蘭縣',
      '花': '花蓮縣',
      '澎': '澎湖縣',
      '金': '金門縣',
      '連': '連江縣',
    };
    return singleCharMap[abbrev];
  }

  /// 釋放資源
  static void dispose() {
    // 資源已由原生端自動釋放
  }
}
