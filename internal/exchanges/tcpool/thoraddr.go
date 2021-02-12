package tcpool

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	types "github.com/binance-chain/go-sdk/common/types"
	"gitlab.com/zlyzol/skadi/internal/common"
	"gitlab.com/zlyzol/skadi/internal/c"

	//	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)
var THOR_API_RATE_LIMIT_ERROR = fmt.Errorf("THORNode Skadi API rate limit exceeded")

// ThorchainPool defines model for thorchain/pools pool detail
type ThorchainPool struct {
	BalanceRune		string `json:"balance_rune,omitempty"`
	BalanceAsset	string `json:"balance_asset,omitempty"`
	Asset			string `json:"asset,omitempty"`
	PoolUnits		string `json:"pool_units,omitempty"`
	Status			string `json:"status,omitempty"`
}
func (p *ThorchainPool) String() string {
	return fmt.Sprintf("+%v", *p)
}
// ThorchainPools defines model for thorchain/pools
type ThorchainPools []ThorchainPool

// ThorchainPoolAddress defines model for ThorchainPoolAddress
type ThorchainPoolAddress struct {
	Address string `json:"address,omitempty"`
	Chain   string `json:"chain,omitempty"`
	PubKey  string `json:"pub_key,omitempty"`
	Halted  bool   `json:"halted,omitempty"`
}
// ThorchainPoolAddresses defines model for ThorchainPoolAddresses.
type ThorchainPoolAddresses struct {
	Current *[]ThorchainPoolAddress `json:"current,omitempty"`
}

// ThorchainLastblock defines model for thorchain/lastblock  detail
type ThorchainLastblock struct {
	Chain 			string `json:"chain,omitempty"`
	Lastobservedin	string `json:"lastobservedin,omitempty"`
	Lastsignedout	string `json:"lastsignedout,omitempty"`
	Thorchain		string `json:"thorchain,omitempty"`
}

type ThorAddr struct {
	logger			zerolog.Logger
	seed			string
	mux 			sync.Mutex
	bnb				string
	acc				types.AccAddress
	thornode		string
	rateLimitTime	int64	
}

func NewThorAddr(seed string) *ThorAddr {
	t := ThorAddr{
		logger:	log.With().Str("module", "ThorAddr").Logger(),
		seed: seed,
	}
	err := t.getThornodeAddresses()
	if err != nil {
		t.logger.Panic().Err(err).Msg("getThornodeAddresses failed")
	} else {
		t.logger.Info().Str("Thornode API url", t.getThornode()).Str("address", t.bnb).Msg("Thornode choosed")
	}
	go t.scan()
	return &t
}
func (t *ThorAddr) getAddr() string {
	t.mux.Lock()
	defer t.mux.Unlock()
	return t.bnb
}
func (t *ThorAddr) getAcc() types.AccAddress {
	t.mux.Lock()
	defer t.mux.Unlock()
	return t.acc
}
func (t *ThorAddr) getThornode() string {
	t.mux.Lock()
	defer t.mux.Unlock()
	return "http://" + t.thornode + ":1317/thorchain"
}
func (t *ThorAddr) getThornodeIP() string {
	t.mux.Lock()
	defer t.mux.Unlock()
	return t.thornode
}
func (t *ThorAddr) scan() {
	for {
		time.Sleep(5 * time.Minute)
		err := t.getThornodeAddresses()
		if err != nil {
			t.logger.Error().Err(err).Msg("getThornodeAddresses failed")
		} else {
			//t.logger.Info().Str("Thornode API scan url", t.GetThornode())
		}
	}
}
func (t *ThorAddr) getThornodeAddresses() error {
	type mapAddr struct {
		n   int
		ips []string
	}
	type arrAddr struct {
		a   string
		n   int
		ips []string
	}
	for try := 1; try < 10; try++ {
		var nodes []string
		var err error

		url := "https://" + t.seed
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}
		bytes, err := common.DoHttpRequest(req)
		if err != nil {
			return err
		}
		err = json.Unmarshal(bytes, &nodes)
		if err != nil {
			return err
		}
		if len(nodes) < 1 {
			return fmt.Errorf("Seed returned 0 nodes %s", url)
		}

		m := make(map[string]mapAddr)
		bad := 0
		minCnt := int(math.Ceil(float64(len(nodes))/3 + 1))
		if len(nodes) == 1 {
			minCnt = 1
		} else if len(nodes) == 2 {
			minCnt = 1
		}
		// filter out nodes which have lastBlock max or max -1
		var maxLastblock int64 = -1
		nodesLastblock := make(map[string]int64)
		for _, ip := range nodes {
			lbs, err := t.getThornodeLastblock(ip)
			if err != nil {
				continue
			}
			lb, err := strconv.ParseInt(lbs, 10, 64)
			if err != nil {
				t.logger.Error().Str("Lastblock",ip).Msg("failed to parse Lastblock for IP")
				continue
			}
			nodesLastblock[ip] = lb
			if maxLastblock < lb {
				maxLastblock = lb
			}
		}
		var okNodes []string
		for _, ip := range nodes {
			if nodesLastblock[ip] < maxLastblock - 6 {
				t.logger.Error().Str("node", ip).Int64("Lastblock", nodesLastblock[ip]).Int64("maxLastblock", maxLastblock).Msg("kicked out for low Lastblock")
				continue
			}
			okNodes = append(okNodes, ip)
		}
		nodes = okNodes
		for _, ip := range nodes {
			teps, err := t.getThornodePoolAddress(ip)
			if err != nil {
				bad++
				continue
			}
			pa := ""
			for _, tep := range *teps.Current {
				if tep.Chain == "BNB" && tep.Halted == false {
					pa = tep.Address
					break
				}
			}
			if pa == "" {
				t.logger.Error().Str("node", ip).Msg("failed to find BNB chain pools on server")
				continue
			}
			if ma, ok := m[pa]; ok {
				m[pa] = mapAddr{n: ma.n + 1, ips: append(ma.ips, ip)} // use the first found IP
			} else {
				m[pa] = mapAddr{n: 1, ips: []string{string(ip)}}
			}
		}
		var p []arrAddr
		p = make([]arrAddr, 0, len(m))
		for k, v := range m {
			i := sort.Search(len(p), func(i int) bool { return p[i].n < v.n })
			p = append(p, arrAddr{})
			copy(p[i+1:], p[i:])
			p[i] = arrAddr{a: k, n: v.n, ips: v.ips}
		}
		if len(p) < 1 {
			err := fmt.Errorf("NOT EVEN ONE Thorchain BNB chain pool found")
			t.logger.Error().Err(err)
			return err
		}
		best := p[0]
		if best.n < minCnt {
			t.logger.Error().Int("try", try).Msg("less than 1/3 + 1 nodes agreed on BNB chain pool address - try again")
			time.Sleep(10 * time.Second)
		} else {
			// found enough nodes which agree on BNB chain pool address
			rand.Seed(time.Now().UnixNano())
			cntFailed := 0
			cntTested := 0
			fastestIP := ""
			var fastestTime time.Duration = 100 * time.Hour
			for {
				ipndx := rand.Intn(len(best.ips))
				ip := best.ips[ipndx]
				//t.logger.Info().Int("ipndx", ipndx).Int("ip", ip).Msg("....test Thornode")
				if fastestIP == "" {
					fastestIP = ip
				}
				if err != nil {
					return err
				}
				testStart := time.Now()
				err := t.testThornodeSpeed(ip)
				if err != nil && cntFailed < 10 {
					cntFailed++
					t.logger.Error().Err(err).Int("failed cnt", cntFailed).Str("ip", ip).Msg("....test Thornode")
				continue
				}
				cntTested++
				timeNow := time.Now()
				elapsed := timeNow.Sub(testStart)
				//t.logger.Debug().Str("elapsed", elapsed.String()).Str("ip", ip).Msg("....test Thornode")
				if fastestTime > elapsed {
					fastestTime = elapsed
					fastestIP = ip
				}
				if cntTested > 3 {
					t.mux.Lock()
					t.thornode = fastestIP // ths is the first returned server from seed that also agreed on valid Thorchain BNB pool wallet address
					t.bnb = best.a                              // Thorchain BNB pool wallet address
					t.acc, err = types.AccAddressFromBech32(t.bnb)
					t.mux.Unlock()
					t.logger.Info().Str("ip", t.thornode).Str("addr", t.bnb).Msg("Thornode found")
					break
				}
			}
			return nil
		}
	}
	err := fmt.Errorf("Error: less than 1/3 + 1 nodes agreed on BNB chain pool address (after 10x try)")
	t.logger.Error().Err(err)
	return err
}

func (t *ThorAddr) getThornodePoolAddress(ip string) (*ThorchainPoolAddresses, error) {
	var url string
	// url = "http://" + ip + ":8080/v1/thorchain/pool_addresses"
	url = "http://" + ip + ":1317/thorchain/pool_addresses"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	bytes, err := common.DoHttpRequest(req)
	if err != nil {
		return nil, err
	}
	var data ThorchainPoolAddresses
	err = json.Unmarshal(bytes, &data)
	if err != nil {
		return nil, err
	}
	return &data, nil
}
func (t *ThorAddr) getThornodeLastblock(ip string) (string, error) {
	url := "http://" + ip + ":1317/thorchain/lastblock"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.logger.Error().Err(err).Str("url", url).Msg("Cannot get /thorchain/lastblock (NewRequest)")
		return "", err
	}
	bytes, err := common.DoHttpRequest(req)
	if err != nil {
		t.logger.Error().Err(err).Str("url", url).Msg("Cannot get /thorchain/lastblock (DoHttpRequest)")
		return "", err
	}
	var data ThorchainLastblock
	err = json.Unmarshal(bytes, &data)
	if err != nil {
		t.logger.Error().Err(err).Str("url", url).Msg("Cannot get /thorchain/lastblock (Unmarshal)")
		return "", err
	}
	return data.Thorchain, nil
}
func (t *ThorAddr) testThornodeSpeed(ip string) error {
	var url string
	// url = "http://" + ip + ":8080/v1/pools/detail?asset=BNB.BNB"
	url = "http://" + ip + ":1317/thorchain/pools"
	for i := 0; i < 5; i++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			t.logger.Error().Err(err).Msg("Cannot get /thorchain/pools (NewRequest)")
			return err
		}
		bytes, err := common.DoHttpRequest(req)
		if err != nil {
			t.logger.Error().Err(err).Msg("Cannot get /thorchain/pools (DoHttpRequest)")
			return err
		}
		var data ThorchainPools
		err = json.Unmarshal(bytes, &data)
		if err != nil {
			t.logger.Error().Err(err).Msg("Cannot get /thorchain/pools (Unmarshal)")
			return err
		}
	}
	return nil
}
func (t *ThorAddr) getPools() (ThorchainPools, error) {
	if !t.rateLimitOK() {
		time.Sleep(c.THOR_API_RATE_LIMIT_MS * time.Millisecond)
	}
	var pools ThorchainPools
	url := t.getThornode() + "/pools"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.logger.Error().Err(err).Msg("Cannot get /pools (NewRequest)")
		return nil, err
	}
	bytes, err := common.DoHttpRequest(req)
	if err != nil {
		t.logger.Error().Err(err).Msg("Cannot get /pools (DoHttpRequest)")
		return nil, err
	}
	err = json.Unmarshal(bytes, &pools)
	if err != nil {
		t.logger.Error().Err(err).Msg("Cannot get /pools (Unmarshal)")
		return nil, err
	}
	return pools, nil
}
func (t *ThorAddr) getPools_WithHeight() (ThorchainPools, error) {
	if !t.rateLimitOK() {
		time.Sleep(c.THOR_API_RATE_LIMIT_MS * time.Millisecond)
	}
	lb, err := t.getThornodeLastblock(t.getThornodeIP())
	if err != nil {
		return nil, err
	}
	var pools ThorchainPools
	url := t.getThornode() + "/pools?height=" + lb
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.logger.Error().Err(err).Msg("Cannot get /pools (NewRequest)")
		return nil, err
	}
	bytes, err := common.DoHttpRequest(req)
	if err != nil {
		t.logger.Error().Err(err).Msg("Cannot get /pools (DoHttpRequest)")
		return nil, err
	}
	err = json.Unmarshal(bytes, &pools)
	if err != nil {
		t.logger.Error().Err(err).Msg("Cannot get /pools (Unmarshal)")
		return nil, err
	}
	return pools, nil
}
func (t *ThorAddr) getPool(asset string) (*ThorchainPool, error) {
	t.logger.Debug().Msgf("getPool(%s)", asset)
	if !t.rateLimitOK() {
		time.Sleep(200 * time.Millisecond)
		if !t.rateLimitOK() {
			return nil, THOR_API_RATE_LIMIT_ERROR
		}
	}
	var url string
	// url = t.GetMidgard() + "/pools/detail?asset=" + asset
	url = t.getThornode() + "/pool/" + asset
	//t.logger.Debug().Str("url", url).Msg("Trying")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.logger.Error().Err(err).Str("url", url).Msg("Cannot get /pool/asset (NewRequest)")
		return nil, err
	}
	bytes, err := common.DoHttpRequest(req)
	if err != nil {
		t.logger.Error().Err(err).Str("url", url).Msg("Cannot get /pool/asset (DoHttpRequest)")
		return nil, err
	}
	var pool ThorchainPool
	err = json.Unmarshal(bytes, &pool)
	if err != nil {
		t.logger.Error().Err(err).Str("url", url).Msg("Cannot get /pool/asset (Unmarshal)")
		return nil, err
	}
	//t.logger.Debug().Str("asset", asset).Str("pool", pool.String()).Str("url", url).Msg("GetPool result")
	return &pool, nil
}
func (t *ThorAddr) getPool_WithHeight(asset string) (*ThorchainPool, error) {
	if !t.rateLimitOK() {
		return nil, THOR_API_RATE_LIMIT_ERROR
	}
	lb, err := t.getThornodeLastblock(t.getThornodeIP())
	if err != nil {
		return nil, err
	}
	var url string
	// url = t.GetMidgard() + "/pools/detail?asset=" + asset
	url = t.getThornode() + "/pool/" + asset + "?height=" + lb
	//t.logger.Debug().Str("url", url).Msg("Trying")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.logger.Error().Err(err).Str("url", url).Msg("Cannot get /pool/asset (NewRequest)")
		return nil, err
	}
	bytes, err := common.DoHttpRequest(req)
	if err != nil {
		t.logger.Error().Err(err).Str("url", url).Msg("Cannot get /pool/asset (DoHttpRequest)")
		return nil, err
	}
	var pool ThorchainPool
	err = json.Unmarshal(bytes, &pool)
	if err != nil {
		t.logger.Error().Err(err).Str("url", url).Msg("Cannot get /pool/asset (Unmarshal)")
		return nil, err
	}
	//t.logger.Debug().Str("asset", asset).Str("pool", pool.String()).Str("url", url).Msg("GetPool result")
	return &pool, nil
}
func (t *ThorAddr) rateLimitOK() bool {
	last := atomic.LoadInt64(&t.rateLimitTime)
	now := time.Now().UnixNano()
	if now - last < c.THOR_API_RATE_LIMIT_MS * 1000000 {
		return false
	}
	atomic.StoreInt64(&t.rateLimitTime, now)
	return true
}