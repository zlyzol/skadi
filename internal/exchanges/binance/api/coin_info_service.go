package api

import (
	"context"
	"encoding/json"
)

//sapi/v1/capital/config/getall

// GetCoinInfoService get all coins' info
type CoinInfoService struct {
	c *Client
}

// Do send request
func (s *CoinInfoService) Do(ctx context.Context, opts ...RequestOption) (res *CoinsInfo, err error) {
	r := &request{
		method:   "GET",
		endpoint: "/sapi/v1/capital/config/getall",
		secType:  secTypeSigned,
	}
	data, err := s.c.callAPI(ctx, r, opts...)
	if err != nil {
		return nil, err
	}
	res = new(CoinsInfo)
	err = json.Unmarshal(data, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

type CoinsInfo []CoinInfo

// CoinInfo define coin info
type CoinInfo struct {
	Coin				string 			`json:"coin"`
	DepositAllEnable	bool 			`json:"depositAllEnable"`
	Free				string 			`json:"free"`
	Freeze				string 			`json:"freeze"`
	Ipoable				string 			`json:"ipoable"`
	Ipoing				string 			`json:"ipoing"`
	IsLegalMoney		bool 			`json:"isLegalMoney"`
	Locked				string 			`json:"locked"`
	Name				string 			`json:"name"`
	NetworkList			[]map[string]interface{}	`json:"networkList"`
	Storage				string 			`json:"storage"`
	Trading				bool 			`json:"trading"`
	WithdrawAllEnable	bool 			`json:"withdrawAllEnable"`
	Withdrawing			string 			`json:"withdrawing"`
}

func (s *CoinInfo) CoinNetworkList() *[]CoinNetwork {
	cnl := make([]CoinNetwork, 0, len(s.NetworkList))
	for _, imap := range s.NetworkList {
		cn := CoinNetwork{}
		if v, found := imap["coin"]; found { cn.Coin = v.(string) }
		if v, found := imap["name"]; found { cn.Name = v.(string) }
		if v, found := imap["network"]; found { cn.Network = v.(string) }
		if v, found := imap["depositEnable"]; found { cn.DepositEnable = v.(bool) }
		if v, found := imap["depositDesc"]; found { cn.DepositDesc = v.(string) }
		if v, found := imap["withdrawEnable"]; found { cn.WithdrawEnable = v.(bool) }
		if v, found := imap["withdrawDesc"]; found { cn.WithdrawDesc = v.(string) }
		if v, found := imap["isDefault"]; found { cn.IsDefault = v.(bool) }
		if v, found := imap["withdrawFee"]; found { cn.WithdrawFee = v.(string) }
		if v, found := imap["withdrawMin"]; found { cn.WithdrawMin = v.(string) }
		if v, found := imap["withdrawIntegerMultiple"]; found { cn.WithdrawIntegerMultiple = v.(string) }
		cnl = append(cnl, cn)
	}
	return &cnl
}

// CoinNetwork define coin network/chain info
type CoinNetwork struct {
	Coin			string 			`json:"coin"`
	Name			string 			`json:"name"`
	Network			string 			`json:"network"`
	DepositEnable	bool 			`json:"depositEnable"`
	DepositDesc		string 			`json:"depositDesc"`
	WithdrawEnable	bool 			`json:"withdrawEnable"`
	WithdrawDesc	string 			`json:"withdrawDesc"`
	IsDefault		bool 			`json:"isDefault"`
	WithdrawFee		string 			`json:"withdrawFee"`
	WithdrawMin		string 			`json:"withdrawMin"`
	WithdrawIntegerMultiple	string		`json:"withdrawIntegerMultiple"`
}
/*
            {
                "addressRegex": "^(bnb1)[0-9a-z]{38}$",
                "coin": "BTC",
                "depositDesc": "Wallet Maintenance, Deposit Suspended", // shown only when "depositEnable" is false.
                "depositEnable": false,
                "isDefault": false,        
                "memoRegex": "^[0-9A-Za-z\\-_]{1,120}$",
                "minConfirm": 1,  // min number for balance confirmation
                "name": "BEP2",
                "network": "BNB",            
                "resetAddressStatus": false,
                "specialTips": "Both a MEMO and an Address are required to successfully deposit your BEP2-BTCB tokens to Binance.",
                "unLockConfirm": 0,  // confirmation number for balance unlcok 
                "withdrawDesc": "Wallet Maintenance, Withdrawal Suspended", // shown only when "withdrawEnable" is false.
                "withdrawEnable": false,
                "withdrawFee": "0.00000220",
                "withdrawMin": "0.00000440"
            },
            {
                "addressRegex": "^[13][a-km-zA-HJ-NP-Z1-9]{25,34}$|^(bc1)[0-9A-Za-z]{39,59}$",
                "coin": "BTC",
                "depositEnable": true,
                "insertTime": 1563532929000,
                "isDefault": true,
                "memoRegex": "",
                "minConfirm": 1, 
                "name": "BTC",
                "network": "BTC",
                "resetAddressStatus": false,
                "specialTips": "",
                "unLockConfirm": 2,
                "updateTime": 1571014804000, 
                "withdrawEnable": true,
                "withdrawFee": "0.00050000",
                "withdrawIntegerMultiple": "0.00000001",
                "withdrawMin": "0.00100000"
            }
        ],
*/