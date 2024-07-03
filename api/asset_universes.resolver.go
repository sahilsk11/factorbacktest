package api

import (
	"sort"

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
	sortUniverses(out)

	// handle sorting of ALL differently - add after to ensure it's
	// always at the end
	out = append(out, getAssetUniversesResponse{
		DisplayName: "All",
		Code:        "ALL",
		NumAssets:   totalSize,
	})

	c.JSON(200, out)
}

func sortUniverses(universes []getAssetUniversesResponse) {
	idealCodeOrder := []string{
		"SPY_TOP_80",
		"SPY_TOP_100",
		"SPY_TOP_300",
		"F-PRIME_FINTECH_INDEX",
	}
	sort.Slice(universes, func(i, j int) bool {
		// if not found, stick it at the end
		ithIndex := len(universes) + 1
		jthIndex := len(universes) + 1
		for x, s := range idealCodeOrder {
			if s == universes[i].Code {
				ithIndex = x
			}
			if s == universes[j].Code {
				jthIndex = x
			}
		}
		return ithIndex < jthIndex
	})
}
