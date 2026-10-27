import Foundation
import Flutter
import Vision
import UIKit

/// Apple Vision Framework OCR Plugin
/// 使用 iOS 原生 Vision 框架進行文字識別，支援 Simulator 和真機
class AppleVisionOcrPlugin: NSObject, FlutterPlugin {

    // MARK: - Plugin Registration

    static func register(with registrar: FlutterPluginRegistrar) {
        let channel = FlutterMethodChannel(
            name: "io.github.will413028.hawkr.seller/apple_vision_ocr",
            binaryMessenger: registrar.messenger()
        )
        let instance = AppleVisionOcrPlugin()
        registrar.addMethodCallDelegate(instance, channel: channel)
        HKLog("[AppleVisionOCR] Plugin registered")
    }

    // MARK: - Method Call Handler

    func handle(_ call: FlutterMethodCall, result: @escaping FlutterResult) {
        switch call.method {
        case "recognizeText":
            guard let args = call.arguments as? [String: Any],
                  let imagePath = args["imagePath"] as? String else {
                result(FlutterError(code: "INVALID_ARGUMENTS",
                                    message: "Missing imagePath parameter",
                                    details: nil))
                return
            }

            let language = args["language"] as? String ?? "zh-Hant"
            recognizeText(imagePath: imagePath, language: language, result: result)

        case "isAvailable":
            // Vision framework is always available on iOS 13+
            result(true)

        case "getSupportedLanguages":
            getSupportedLanguages(result: result)

        default:
            result(FlutterMethodNotImplemented)
        }
    }

    // MARK: - Text Recognition

    private func recognizeText(imagePath: String, language: String, result: @escaping FlutterResult) {
        HKLog("[AppleVisionOCR] Starting text recognition for: \(imagePath)")

        // Load image from path
        guard let image = UIImage(contentsOfFile: imagePath) else {
            HKLog("[AppleVisionOCR] Failed to load image")
            result(FlutterError(code: "IMAGE_LOAD_ERROR",
                                message: "Failed to load image from path: \(imagePath)",
                                details: nil))
            return
        }

        guard let cgImage = image.cgImage else {
            HKLog("[AppleVisionOCR] Failed to get CGImage")
            result(FlutterError(code: "IMAGE_CONVERSION_ERROR",
                                message: "Failed to convert UIImage to CGImage",
                                details: nil))
            return
        }

        // Create Vision request
        let request = VNRecognizeTextRequest { [weak self] request, error in
            self?.handleRecognitionResult(request: request, error: error, result: result)
        }

        // Configure recognition
        request.recognitionLevel = .accurate
        request.usesLanguageCorrection = true

        // Set recognition languages
        // 支援繁體中文、簡體中文、英文
        if #available(iOS 16.0, *) {
            request.recognitionLanguages = ["zh-Hant", "zh-Hans", "en-US"]
            request.automaticallyDetectsLanguage = true
        } else {
            // iOS 13-15 使用舊的語言設定
            request.recognitionLanguages = ["zh-Hant", "zh-Hans", "en-US"]
        }

        HKLog("[AppleVisionOCR] Recognition languages: \(request.recognitionLanguages)")

        // Perform recognition
        let handler = VNImageRequestHandler(cgImage: cgImage, options: [:])

        DispatchQueue.global(qos: .userInitiated).async {
            do {
                try handler.perform([request])
            } catch {
                HKLog("[AppleVisionOCR] Recognition failed: \(error.localizedDescription)")
                DispatchQueue.main.async {
                    result(FlutterError(code: "RECOGNITION_ERROR",
                                        message: "Text recognition failed: \(error.localizedDescription)",
                                        details: nil))
                }
            }
        }
    }

    private func handleRecognitionResult(request: VNRequest, error: Error?, result: @escaping FlutterResult) {
        DispatchQueue.main.async {
            if let error = error {
                HKLog("[AppleVisionOCR] Recognition error: \(error.localizedDescription)")
                result(FlutterError(code: "RECOGNITION_ERROR",
                                    message: error.localizedDescription,
                                    details: nil))
                return
            }

            guard let observations = request.results as? [VNRecognizedTextObservation] else {
                HKLog("[AppleVisionOCR] No text observations found")
                result(["text": "", "blocks": []])
                return
            }

            var fullText = ""
            var blocks: [[String: Any]] = []

            for observation in observations {
                guard let topCandidate = observation.topCandidates(1).first else { continue }

                let text = topCandidate.string
                let confidence = topCandidate.confidence

                // Get bounding box (normalized coordinates)
                let boundingBox = observation.boundingBox

                fullText += text + "\n"

                blocks.append([
                    "text": text,
                    "confidence": confidence,
                    "boundingBox": [
                        "x": boundingBox.origin.x,
                        "y": boundingBox.origin.y,
                        "width": boundingBox.width,
                        "height": boundingBox.height
                    ]
                ])
            }

            HKLog("[AppleVisionOCR] Recognized \(blocks.count) text blocks")
            HKLog("[AppleVisionOCR] Full text preview: \(fullText.prefix(200))...")

            result([
                "text": fullText.trimmingCharacters(in: .whitespacesAndNewlines),
                "blocks": blocks
            ])
        }
    }

    // MARK: - Supported Languages

    private func getSupportedLanguages(result: @escaping FlutterResult) {
        if #available(iOS 16.0, *) {
            let request = VNRecognizeTextRequest()
            request.recognitionLevel = .accurate
            do {
                let languages = try request.supportedRecognitionLanguages()
                HKLog("[AppleVisionOCR] Supported languages: \(languages)")
                result(languages)
            } catch {
                HKLog("[AppleVisionOCR] Failed to get supported languages: \(error)")
                result(["zh-Hant", "zh-Hans", "en-US"]) // Fallback
            }
        } else {
            // Fallback for older iOS versions
            result(["zh-Hant", "zh-Hans", "en-US"])
        }
    }
}

// MARK: - HKLog Helper (if not already defined)

#if !DEBUG
private func HKLog(_ message: String, file: String = #file, function: String = #function, line: Int = #line) {
    // No-op in release builds
}
#endif
