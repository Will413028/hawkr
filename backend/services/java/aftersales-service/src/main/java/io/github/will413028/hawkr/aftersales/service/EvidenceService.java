package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.entity.AfterSaleEvidenceEntity;
import io.github.will413028.hawkr.aftersales.grpc.client.MediaGrpcClient;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleEvidenceMapper;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.media.grpc.FinalizeImagesResponse;
import io.github.will413028.hawkr.media.grpc.FinalizedAsset;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

import java.util.List;
import java.util.stream.Collectors;

/**
 * 售後舉證 / 驗貨照片共用服務。
 * 供 AfterSaleCaseService（買家申請/補件）與 ReturnReceiptService（賣家驗貨）共用。
 */
@Service
@RequiredArgsConstructor
public class EvidenceService {

    private final AfterSaleEvidenceMapper evidenceMapper;
    private final MediaGrpcClient mediaGrpcClient;

    /**
     * 圖片轉正 + 連結 + 插入舉證記錄。
     *
     * @param caseNo       售後單編號
     * @param uploadedBy   上傳者 account_id
     * @param photoTokens  Media stagedToken 清單（空或 null 時 no-op）
     * @param evidenceType AfterSaleEvidenceType value（8601=PHOTO / 8602=INSPECTION_PHOTO）
     */
    public void finalizeAndInsert(String caseNo, Long uploadedBy,
                                   List<String> photoTokens, String evidenceType) {
        if (photoTokens == null || photoTokens.isEmpty()) {
            return;
        }

        FinalizeImagesResponse finalizeResp =
                mediaGrpcClient.finalizeImages(photoTokens, AssetReferenceType.AFTERSALE.value());
        List<FinalizedAsset> assets = finalizeResp.getAssetsList();
        if (assets.isEmpty()) {
            return;
        }

        List<Long> assetIds = assets.stream()
                .map(a -> ProtoConversionUtils.parseLongRequired(a.getAssetId(), "assetId"))
                .collect(Collectors.toList());
        mediaGrpcClient.linkImages(AssetReferenceType.AFTERSALE.value(), caseNo, assetIds);

        for (FinalizedAsset asset : assets) {
            AfterSaleEvidenceEntity evidence = new AfterSaleEvidenceEntity();
            evidence.setId(TsidUtil.generate());
            evidence.setCaseNo(caseNo);
            evidence.setAssetId(ProtoConversionUtils.parseLongRequired(asset.getAssetId(), "assetId"));
            evidence.setObjectKey(asset.getObjectKey());
            evidence.setUploadedBy(uploadedBy);
            evidence.setEvidenceType(evidenceType);
            evidenceMapper.insert(evidence);
        }
    }
}
