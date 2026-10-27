package io.github.will413028.hawkr.media.converter;

import io.github.will413028.hawkr.media.dto.FinalizedAssetDTO;
import io.github.will413028.hawkr.media.entity.MediaAssetEntity;
import io.github.will413028.hawkr.media.grpc.FinalizedAsset;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.stream.Collectors;

/**
 * MediaAsset 轉換器
 * 負責 Entity ↔ DTO ↔ Proto 轉換
 */
@Component
public class MediaAssetConverter {

    /**
     * Entity → FinalizedAssetDTO
     */
    public FinalizedAssetDTO toFinalizedDTO(MediaAssetEntity entity, String stagedToken) {
        return FinalizedAssetDTO.builder()
            .stagedToken(stagedToken)
            .assetId(entity.getId().toString())
            .objectKey(entity.getObjectKey())
            .isSecure(entity.getIsSecure())
            .build();
    }

    /**
     * FinalizedAssetDTO → Proto FinalizedAsset
     */
    public FinalizedAsset toProto(FinalizedAssetDTO dto) {
        return FinalizedAsset.newBuilder()
            .setStagedToken(dto.getStagedToken())
            .setAssetId(dto.getAssetId())
            .setObjectKey(dto.getObjectKey())
            .setIsSecure(dto.getIsSecure())
            .build();
    }

    /**
     * FinalizedAssetDTO List → Proto FinalizedAsset List
     */
    public List<FinalizedAsset> toProtoList(List<FinalizedAssetDTO> dtos) {
        return dtos.stream()
            .map(this::toProto)
            .collect(Collectors.toList());
    }
}
