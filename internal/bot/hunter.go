package bot

import (
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/zlyzol/skadi/internal/common"
	"gitlab.com/zlyzol/skadi/internal/store"

)

var Debug_do = false // true
var Debug_ex = "Binance" // "Binance DEX"
var Debug_bt = "COTI"
var Debug_qt = "USDT"
var Debug_prey *Prey
var Debug_side = common.OrderSideBUY
var Debug_am = 4000.1
var Debug_pr = 0.06739


// these two variables are used oly for debugging - theay enable only one hunting (prey processing) at a time
var HUNT_SYNC_ONE = true
var atomicHUNT_SYNC_ONE int32 = 0

/*
type hunterMarket struct {
	exchange	common.Exchange
	market 		common.Market
	atomicOffer	atomic.Value // PoolData or OrderbookData
}
*/

// Hunter structure
type Hunter struct {
	logger 			zerolog.Logger
	atomicChanged	int32
	atomicHunting	int32
	ob 				*common.Orderbook
	pool			*common.Pool
	market			common.Market
	lastCheck		time.Time
	lastYieldInfo	time.Time
	balancer		*AccBalancer
	store			store.Store 
}

func (bot *Bot) debug_fun() bool {
	tcex, found1 := bot.exchanges["thorchain"]
	//bdex, found2 := bot.exchanges["binance-dex"]
	biex, found3 := bot.exchanges["binance"]
	_, _ = found1, found3
	dexacc := tcex.GetAccount()
	binacc := biex.GetAccount()
	var a common.Asset
	var err error
	var sent common.Uint

//	a, _ = common.NewAsset("BNB.RUNE")
//	_, sent, err = binacc.Send(common.NewUintFromFloat(1), a, dexacc, true)

//	a, _ = common.NewAsset("BNB.BTC")
//	_, sent, err = binacc.Send(common.NewUintFromFloat(0.0001), a, dexacc, true)

	a, _ = common.NewAsset("BNB.BNB")
	_, sent, err = dexacc.Send(common.NewUintFromFloat(0.1), a, binacc, true) 
	if err != nil {
		bot.logger.Error().Err(err).Msg("send failed")
	}

	_, _ = sent, err
	
	return true
}
// StartHunters starts the bot's arb hunters.
func (bot *Bot) startHunters() error {

	if false && bot.debug_fun() {
		return nil
	}

	balancer := NewAccBalancer()
	atomic.StoreInt32(&atomicHUNT_SYNC_ONE, 1) // hunting disabled
	tcex, found := bot.exchanges["thorchain"]
	if !found {
		bot.logger.Panic().Msg("cannot find thorchain exchange")
	}
	hunterCnt := 0
	for _, ex := range bot.exchanges {
		if ex == tcex { continue }
		if ex == nil { continue }
		exHunterCnt := 0
		for _, exm := range ex.GetMarkets() {

			if Debug_do {
				// DEBUG ******************************
				// dex - rune / busd
				if
				ex.GetName() == Debug_ex &&
				exm.BaseAsset.Ticker.String() == Debug_bt &&
				exm.QuoteAsset.Ticker.String() == Debug_qt &&
				true {
					// we do this asset
				} else { continue }
			}

			/*
			// dex - coti / bnb
			if exm.BaseAsset.Ticker.String() == "COTI" {
				bot.logger.Debug().Msg("COTI")
			}
			if
			ex.GetName() == "Binance DEX" && 
			exm.BaseAsset.Ticker.String() == "COTI" &&
			exm.QuoteAsset.Ticker.String() == "BNB" &&
			true {
			} else { continue }
			// DEBUG ******************************
*/
			rune := false
			a := [2]common.Asset{exm.BaseAsset, exm.QuoteAsset} // twt/bnb | twtp, bnbp -> twt/bnb | twtp, bnbp
			var markets [2]*common.Market						// bnb/eth | bnbp, ethp -> bnb/eth | bnbp, ethp
			for i := 0; i < 2; i++ {					// rune/bnb | bnbp -> nofxlip, rune/bnb | bnbp
				if a[i].Intersects(common.RuneAsset()) {	// xrp/rune | xrpp -> fxlip, xrp/rune | xrpp 
					rune = true
					if i == 1 { markets[1] = markets[0] }
					continue
				}
				for _, tcm := range tcex.GetMarkets() {
					if a[i].Intersects(tcm.BaseAsset) {
						markets[i] = &tcm
						if rune { markets[0] = markets[1] }
						break
					}
				}
			}
			if markets[0] == nil || markets[1] == nil {
				bot.logger.Debug().Str("exchange", ex.GetName()).Str("market", exm.String()).Msg("cannot find TC pool for both asset ... skipping")
				continue
			}

			// found - we can start hunting
			ob := common.NewOrderbook(ex, exm)
			var pool *common.Pool
			if markets[0] == markets[1] { // single pool
				pool1 := common.NewSinglePoolFlip(tcex, *markets[0], exm.BaseAsset.Contains(common.RuneAsset()))
				pool = pool1
			} else { // dual pool
				pool1 := common.NewSinglePool(tcex, *markets[0])
				pool2 := common.NewSinglePool(tcex, *markets[1])
				pool = common.NewPool(pool1, pool2)
			}

			// DEBUG ***************************
			/*
			// just RUNE/BNB on bdex
			if 	//hms[0].exchange.GetName() == "Binance" && 
				f[0] == f[1] && f[0].BaseAsset.Ticker.String() == "BNB" ||
				//hms[0].exchange.GetName() == "Binance DEX" && 
				f[0] != f[1] && f[0].BaseAsset.Ticker.String() == "COTI" && f[1].BaseAsset.Ticker.String() == "BNB" ||
				false {
				//OK
			} else {
				continue
			}*/

			balancer.addMarket(exm)
			h, err := bot.NewHunter(ob, pool, balancer, bot.store)
			if err != nil {
				return err
			}
			if Debug_do {
				Debug_prey = NewPrey(Debug_side, common.NewUintFromFloat(Debug_am), common.NewUintFromFloat(Debug_pr), common.NewUintFromFloat(Debug_pr), 1, h)
			}
			bot.logger.Debug().Str("exchange", ex.GetName()).Str("market", exm.String()).Msg("hunters wanted to start")
			hunterCnt++
			exHunterCnt++
			h.start()
			time.Sleep(400 * time.Millisecond)
		}
		bot.logger.Info().Str("exchange", ex.GetName()).Int("hunter count", exHunterCnt).Msg("exchange hunters started")
		balancer.balanceAll(ex.GetAccount(), tcex.GetAccount())
	}
	bot.logger.Info().Int("hunter count", hunterCnt).Msg("all hunters started - hunting enabled")

	atomic.StoreInt32(&atomicHUNT_SYNC_ONE, 0) // hunting enabled
	
	return nil
}

// NewHunter creates new hunter
func (bot *Bot) NewHunter(ob *common.Orderbook, pool *common.Pool, balancer *AccBalancer, store store.Store) (*Hunter, error) {
	h := Hunter {
		logger:		log.With().Str("module", "hunter").Str("exchange", ob.GetExchange().GetName()).Str("market", ob.GetMarket().String()).Logger(),
		atomicHunting:	0,
		market:		ob.GetMarket(),
		ob:			ob,
		pool:		pool,
		balancer:	balancer,
		store:		store,
	}
	return &h, nil
}
func (h *Hunter) start() {
	h.logger.Info().Msg("Hunter started")
	h.lastCheck = time.Now()
	h.lastYieldInfo = time.Now()
	h.subscribeOB(h.ob)
	time.Sleep(50 * time.Millisecond)
	h.subscribePool(h.pool)
	time.Sleep(50 * time.Millisecond)
}
func (h *Hunter) subscribeOB(ob *common.Orderbook) {
	ob.GetExchange().Subscribe(ob.GetMarket(), func(data interface{}) {
		atomic.CompareAndSwapInt32(&h.atomicChanged, 0, 1)
		ob.AtomicSetData(data.(common.BidsAsks))
		//h.logger.Debug().Msg("go h.hunt() calling")
		go h.hunt()
	})
}
func (h *Hunter) subscribePool(pool *common.Pool) {
	pool.GetExchange().Subscribe(pool.GetInnerMarket(0), func(data interface{}) {
		atomic.CompareAndSwapInt32(&h.atomicChanged, 0, 1)
		pool.AtomicSetInnerData(0, data.(common.Amounts))
		//h.logger.Debug().Msg("go h.hunt() calling")
		go h.hunt()
	})
	if pool.IsDual() {
		pool.GetExchange().Subscribe(pool.GetInnerMarket(1), func(data interface{}) {
			atomic.CompareAndSwapInt32(&h.atomicChanged, 0, 1)
			pool.AtomicSetInnerData(1, data.(common.Amounts))
			//h.logger.Debug().Msg("go h.hunt() calling")
			go h.hunt()
		})
	}
}
/*
// onMarketChange - called when orderbook change occurs, can be called from another thread
func (h *Hunter) onMarketChange(hm *hunterMarket, offer common.Offer) {
	atomic.CompareAndSwapInt32(&h.atomicChanged, 0, 1)
	hm.atomicOffer.Store(offer)
	//h.logger.Debug().Msg("go h.hunt() calling")
	go h.hunt()
}
*/
func (h *Hunter) hasMarketChanged() bool {
	return atomic.LoadInt32(&h.atomicChanged) == 1
}
func (h *Hunter) RefreshOfferCaches() {
	h.ob.UpdateCache()
	h.pool.UpdateCache()
}
func (h *Hunter) RefreshOffers() {
	bas := h.ob.GetExchange().GetCurrentOfferData(h.ob.GetMarket()).(common.BidsAsks)
	if bas.Equal(common.EmptyBidsAsks) {
		time.Sleep(400 * time.Millisecond)
		bas = h.ob.GetExchange().GetCurrentOfferData(h.ob.GetMarket()).(common.BidsAsks)
		if bas.Equal(common.EmptyBidsAsks) {
			h.logger.Info().Msg("CANNOT Refresh Orderbook Data")
		}
	}
	depths := h.pool.Refresh()
	if depths.IsEmpty() {
		time.Sleep(400 * time.Millisecond)
		depths := h.pool.Refresh()
		if depths.IsEmpty() {
			h.logger.Info().Msg("CANNOT Refresh Pool Data")
		}
	}
	h.RefreshOfferCaches()
}
func (h *Hunter) hunt() {
	if atomic.LoadInt32(&atomicHUNT_SYNC_ONE) == 1 {
		return
	}
	if HUNT_SYNC_ONE {
		if !atomic.CompareAndSwapInt32(&atomicHUNT_SYNC_ONE, 0, 1) {
			//h.logger.Debug().Msg("already hunting")
			return // already hunting
		}
		defer atomic.StoreInt32(&atomicHUNT_SYNC_ONE, 0)
	} else {
		if !atomic.CompareAndSwapInt32(&h.atomicHunting, 0, 1) {
			//h.logger.Debug().Msg("already hunting")
			return // already hunting
		}
		defer atomic.StoreInt32(&h.atomicHunting, 0)
	}

	//h.logger.Info().Msg("hunting started - refreshinh caches")
	h.RefreshOfferCaches()
	
	// preCheck() ???

	// Prey
	prey := h.findPrey()
	if prey == nil {
		//h.logger.Info().Msg("no prey found")
	} else {
		h.logger.Debug().Str("prey", prey.amount.String()).Msg("prey found")
		h.releaseTrap()
		prey.process()
		h.RefreshOffers()
	}
	// MM
	mm := h.findTrap()
	if mm == nil {
		h.logger.Debug().Msg("no MM option found")
	} else {
		h.logger.Debug().Msg("MM found")
		mm.process()
		h.RefreshOffers()
	}
}
/*
func (h *Hunter) loadOffers() ([]hunterOffer, int) {
	l := len(h.hmarkets)
	hofs := make([]hunterOffer, l)
	for i := 0; i < l; i++ {
		offeri := h.hmarkets[i].atomicOffer.Load()
		if offeri == nil { // not all offers are loaded -> no hunting yet
			return []hunterOffer{}, 0
		}
		offer := offeri.(common.Offer)
		hofs[i] = hunterOffer{
			exchange:	h.hmarkets[i].exchange,
			market:		h.hmarkets[i].market,
			offer:		offer,
		}
	}
	return hofs, l
}
func (h *Hunter) getNormalizedOffers() DualOffer {
	lm := len(h.hmarkets)
	ofs, l := h.loadOffers()
	if lm != l {
		h.logger.Debug().Int("market count", lm).Int("offer count", l).Msg("failed to load all offers")
		return nil
	}
	for i := 0; i < l; i++ {
		if ofs[i].IsEmpty() {
			return nil
		}
	}
	if l < 2 || l > 3 {
		h.logger.Error().Int("offer count", l).Msg("unsupported market combination")
		return nil
	}
	if l == 2 && ofs[0].GetType() == OB && ofs[1].GetType() == POOL {
		return ofs
	} else if l == 3 && ofs[0].GetType() == OB && ofs[1].GetType() == POOL && ofs[2].GetType() == POOL {
		newofs := []hunterOffer { ofs[0], {
				exchange:	ofs[1].exchange,
				market:		common.NewMarket(ofs[1].market.BaseAsset, ofs[2].market.BaseAsset),
				offer:		common.NewDualPool(ofs[1].offer.(common.Pool), ofs[2].offer.(common.Pool)),
				dual:		true,
			},
		}
		return newofs
	} else {
		h.logger.Debug().Msg("unsupported market combination")
		return nil
	}
}
*/