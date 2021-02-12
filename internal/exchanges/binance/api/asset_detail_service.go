package api

import (
	"context"
	"encoding/json"
	"log"
)

// ListDepositsService fetches deposit history.
//
// See https://binance-docs.github.io/apidocs/spot/en/#deposit-history-user_data
type AssetDetailService struct {
	c         *Client
	asset     *string
}

// Asset sets the asset parameter.
func (s *AssetDetailService) Asset(asset string) *AssetDetailService {
	s.asset = &asset
	return s
}

// Do sends the request.
func (s *AssetDetailService) Do(ctx context.Context) (assetDetails AssetDetailResponse, err error) {
	r := &request{
		method:   "GET",
		endpoint: "/wapi/v3/assetDetail.html",
		secType:  secTypeSigned,
	}
	if s.asset != nil {
		r.setParam("asset", *s.asset)
	}
	data, err := s.c.callAPI(ctx, r)
	if err != nil {
		return
	}
	hres := new(helperAssetDetailResponse)
	err = json.Unmarshal(data, hres)
	if err != nil {
		return
	}
	res := make(AssetDetailResponse, len(hres.AssetDetails))
	for a, d := range hres.AssetDetails {
		log.Printf("Get %v, %+v", a,d)
		res[a] = d.Get()
	}

	return res, nil
}

// AssetDetailResponse represents a response from AssetDetailService.
type helperAssetDetailResponse struct {
	Success  		bool							`json:"success"`
	AssetDetails	map[string]helperAssetDetail	`json:"assetDetail"`
}
type helperAssetDetail map[string]interface{}

// AssetDetailResponse represents a response from AssetDetailService.
type AssetDetailResponse map[string]AssetDetail

// Deposit represents a single deposit entry.
type AssetDetail struct {
	DepositStatus		bool	`json:"depositStatus"`
	MinWithdrawAmount	float64	`json:"minWithdrawAmount"`
	WithdrawFee			float64	`json:"withdrawFee"`
	WithdrawStatus		bool	`json:"withdrawStatus"`
	DepositTip			string  `json:"depositTip"`
}

// Get return the AssetDetail
func (m helperAssetDetail) Get() AssetDetail {
	d := AssetDetail{}
	if i, ok := m["depositStatus"]; ok {
		d.DepositStatus = i.(bool)
	}
	if i, ok := m["minWithdrawAmount"]; ok {
		d.MinWithdrawAmount = i.(float64)
	}
	if i, ok := m["withdrawFee"]; ok {
		d.WithdrawFee = i.(float64)
	}
	if i, ok := m["withdrawStatus"]; ok {
		d.WithdrawStatus = i.(bool)
	}
	if i, ok := m["depositTip"]; ok {
		d.DepositTip = i.(string)
	}
	return d
}
