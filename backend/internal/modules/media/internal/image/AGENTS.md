# media/internal/image

公開圖片的辨認、解碼與重新編碼。

## 不變量

- `public/` 的圖片一律經 libvips 重新編碼（JPEG、PNG、WebP、GIF；sRGB、已套用方向、無 metadata）；辨認不出、無法解碼、超過上限永久拒絕（動態 WebP 單邊超過 16383 在標頭階段永久拒收，`TestAnimatedWebPSideOverEncoderLimitIsRejectedFromTheHeader`）；`image.Limits` 非正回 `ErrInvalidLimits`。守門：`TestFakeImageIsRejected`、`TestHeightOverDimensionLimitIsRejectedFromTheHeader`、`TestNonPositiveLimitsAreRejected`。
