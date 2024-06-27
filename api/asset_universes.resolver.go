package api

import (
	"github.com/gin-gonic/gin"
)

type getAssetUniversesResponse struct {
	DisplayName string `json:"displayName"`
	Code        string `json:"code"`
	NumAssets   int    `json:"numAssets"`
}

func (h ApiHandler) getAssetUniverses(c *gin.Context) {

	universeDetails, err := h.AssetUniverseRepository.GetAssetUniverses()
	if err != nil {
		returnErrorJson(err, c)
		return
	}
	totalSize := 0
	out := []getAssetUniversesResponse{}
	for _, universe := range universeDetails {
		totalSize += int(*universe.NumAssets)
		out = append(out, getAssetUniversesResponse{
			DisplayName: *universe.DisplayName,
			Code:        *universe.AssetUniverseName,
			NumAssets:   int(*universe.NumAssets),
		})
	}
	out = append(out, getAssetUniversesResponse{
		DisplayName: "All",
		Code:        "ALL",
		NumAssets:   totalSize,
	})

	c.JSON(200, out)
}
