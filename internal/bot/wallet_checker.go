package bot

import (
	//	"hash/adler32"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"sort"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/zlyzol/skadi/internal/c"
	"gitlab.com/zlyzol/skadi/internal/common"
	"gitlab.com/zlyzol/skadi/internal/models"
)

type WalletChecker struct {
	logger		zerolog.Logger
	accounts	[]common.Account
	syncer		*HunterSyncer
	startBal	common.Balances
	curBal		common.Balances
	mux			sync.Mutex
}

func NewWalletChecker(syncer *HunterSyncer) *WalletChecker {
	wch := WalletChecker{
		logger:		log.With().Str("module", "WalletChecker").Logger(),
		accounts:	make([]common.Account, 0, 10),
		syncer:		syncer,
		startBal:	common.Balances{},
		curBal:		common.Balances{},
	}
	return &wch
}
func (wch *WalletChecker) GetWallet() models.Wallet{
	wallet := models.Wallet{
		Time: time.Now(),
	}
	wch.read()
	wch.mux.Lock()
	defer wch.mux.Unlock()
	comp := wch.merge2am(wch.startBal, wch.curBal)
	wallet.Assets = make([]models.WalletAssetData, 0, len(comp))
	for a, b2 := range comp {
		dif := b2.am2.Sub(b2.am1)
		asset, _ := common.NewAsset(a)
		difr := common.Oracle.GetRuneValueOf(dif, asset)
		wallet.PlusRune += difr.ToFloat()
		wad := models.WalletAssetData{
			Asset:	a,
			Start:	b2.am1.ToFloat(),
			Cur:	b2.am2.ToFloat(),
			Plus:	dif.ToFloat(),
			PlusRune: difr.ToFloat(),
		}
		wallet.Assets = append(wallet.Assets, wad)
	}
	sort.SliceStable(wallet.Assets, func(i, j int) bool {
		return wallet.Assets[i].PlusRune < wallet.Assets[j].PlusRune
	})
	return wallet
}
func (wch *WalletChecker) addAccount(acc common.Account) {
	for _, a := range wch.accounts {
		if a == acc {
			return
		}
	}
	wch.accounts = append(wch.accounts, acc)
}
func (wch *WalletChecker) start() {
	wch.read()
	wch.startBal = wch.balances()
	go wch.loop()
}
func (wch *WalletChecker) loop() {
	ticker := time.NewTicker(c.WALLET_CHECK_SEC * time.Second)
	for {
		select {
		case <-common.Stop:
			wch.logger.Info().Msg("WalletChecker loop stopped on common.Stop signal")
			return
		case <-ticker.C:
			wch.read()
		}
	}
}
func (wch *WalletChecker) read() {
	wch.syncer.MuxHUNTERS.Lock()
	defer wch.syncer.MuxHUNTERS.Unlock()
	if !atomic.CompareAndSwapInt32(&wch.syncer.AtomicHUNTER_BLOCKED, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&wch.syncer.AtomicHUNTER_BLOCKED, 0)
	wch.mux.Lock()
	defer wch.mux.Unlock()
	var sum common.Balances
	for _, acc := range wch.accounts {
		bals := acc.GetBalances()
		sum = wch.merge(sum, bals)
	}
	wch.curBal = sum
}
func (wch *WalletChecker) merge(b1, b2 common.Balances) (bal common.Balances) {
	cnt := len(b1)
	if cnt < len(b2) {
		cnt = len(b2)
	}
	bal = make(common.Balances, cnt)
	for ts, am := range b1 {
		if am.IsZero() { continue }
		t, _ := common.NewTicker(ts)
		ts = t.String()
		bal[ts] = am
	}
	for ts, am := range b2 {
		if am.IsZero() { continue }
		t, _ := common.NewTicker(ts)
		ts = t.String()
		am1, found := bal[ts]
		if found {
			bal[ts] = am1.Add(am)
		} else {
			bal[ts] = am
		}
	}
	return bal
}
type balances2am map[string]struct{am1, am2 common.Uint}
func (wch *WalletChecker) merge2am(b1, b2 common.Balances) (bal balances2am) {
	cnt := len(b1)
	if cnt < len(b2) {
		cnt = len(b2)
	}
	bal = make(balances2am, cnt)
	for ts, am := range b1 {
		if am.IsZero() { continue }
		t, _ := common.NewTicker(ts)
		ts = t.String()
		bal[ts] = struct{am1, am2 common.Uint}{am, common.ZeroUint()}
	}
	for ts, am := range b2 {
		if am.IsZero() { continue }
		t, _ := common.NewTicker(ts)
		ts = t.String()
		am1, found := bal[ts]
		if found {
			bal[ts] = struct{am1, am2 common.Uint}{am1.am1, am}
		} else {
			bal[ts] = struct{am1, am2 common.Uint}{am, common.ZeroUint()}
		}
	}
	return bal
}
func (wch *WalletChecker) balances() common.Balances {
	return wch.curBal
}
func (wch *WalletChecker) String() (s string) {
	wallet := wch.GetWallet()
	for _, wa := range wallet.Assets {
		s = fmt.Sprintf("%s\r\n|%-10s|%8.6f|%8.6f|%8.6f|%8.6f|", s, wa.Asset, wa.Start, wa.Cur, wa.Plus, wa.PlusRune)
	}
	s = fmt.Sprintf("%s\r\n|%-10s......%12.6f", s, "PLUS:", wallet.PlusRune)
	return s
}
