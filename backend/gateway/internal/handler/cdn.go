package handler

// 媒體 URL 組裝由各微服務自行負責（媒體 base URL + objectKey）
// Gateway 僅透傳微服務回傳的完整 URL，不做 URL 組裝
//
// 共用配置（49 號設計書 §8.5）：
// - Java：common-core MediaUrlProperties（hawkr.media.base-url）
// - Go：pkg/mediaurl.BuildURL
// - 環境變數：MEDIA_BASE_URL（**無預設值，缺值即啟動失敗**；媒體域名不做新舊相容，
//   dev 一律 test-media.example.com、prod 一律 media.example.com。理由見
//   gateway/internal/config.Config.MediaBaseURL 與 backend#2283）
// - 微服務透過 MediaUrlProperties.buildUrl(objectKey) / mediaurl.BuildURL 組裝完整 URL
