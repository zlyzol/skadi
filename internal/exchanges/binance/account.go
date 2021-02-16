package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"gitlab.com/zlyzol/skadi/internal/c"
	"gitlab.com/zlyzol/skadi/internal/common"
	"gitlab.com/zlyzol/skadi/internal/config"
	binapi "gitlab.com/zlyzol/skadi/internal/exchanges/binance/api"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var accc *Account

type Account struct {
	logger         zerolog.Logger
	cfg            *config.AccountConfiguration
	bin            *binapi.Client
	atomicBalances atomic.Value // common.Balances
	ex             *binance
	listeners      common.ThreadSafeSlice
}

func (b *binance) NewAccount(cfg *config.AccountConfiguration, accs *common.Accounts) (*Account, *binapi.Client, error) {
	_, found := accs.Get(cfg.Name)
	if found {
		b.logger.Panic().Msg("it is not allowed to use the same API keys for more than one binance connection: %v" + cfg.Name)
	}
	var err error
	ca, found := cfg.Chains["ALL"]
	if !found {
		b.logger.Panic().Msg("failed to find account configuration for for Binance for ALL chains")
	}
	bin := binapi.NewClient(ca.KeystoreOrApiKey, ca.PasswordOrSecretKey)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to open API connection to Binance (check whitelisted IP)")
	}
	acc := Account{
		logger: log.With().Str("module", "binance.account").Logger(),
		cfg:    cfg,
		bin:    bin,
		ex:     b,
	}
	accc = &acc
	acc.readBalances()
	accs.Add(&acc)
	go acc.accountWatcher()
	return &acc, acc.bin, nil
}
func (acc *Account) GetName() string {
	return acc.cfg.Name
}
func (acc *Account) GetBalance(asset common.Asset) common.Uint {
	balances := acc.GetBalances()
	ticker := asset.Ticker.String()
	if v, ok := balances[ticker]; ok {
		return v
	}
	return common.ZeroUint()
}
func (acc *Account) GetBalances() common.Balances {
	bali := acc.atomicBalances.Load()
	if bali == nil {
		return common.Balances{}
	}
	return bali.(common.Balances)
}
func (acc *Account) GetDepositAddresses(asset common.Asset) common.DepositAddresses {
	if asset.Chain.Contains(common.BNBChain) {
		res := acc.tryChain(common.BNBChain.String(), asset.Ticker.String())
		if res != nil {
			return res
		}
	}
	for _, chain := range asset.Chain {
		res := acc.tryChain(chain, asset.Ticker.String())
		if res != nil {
			return res
		}
	}
	return nil
}
func (acc *Account) tryChain(chain, ticker string) common.DepositAddresses {
	ticker = common.Ticker(ticker).String() // covert B-tickers
	for i := 0; ; i++ {
		resAddr, err := acc.bin.NewGetDepositAddressWithNetworkService().Coin(ticker).Network(chain).Do(context.Background())
		if err != nil {
			acc.logger.Panic().Msg("if unsupported chain - we should not repeat this request")
			// if unsupported chain - we should not repeat this request
			if i >= c.BINANCE_GETSERVICE_ERR_SLEEP_TRY_CNT {
				acc.logger.Error().Err(err).Msg("NewGetDepositAddressWithNetworkService failed")
				return nil
			}
			acc.logger.Error().Err(err).Int("i", i).Msg("NewGetDepositAddressWithNetworkService failed - try once again")
			time.Sleep(c.BINANCE_GETSERVICE_ERR_SLEEP_MS * time.Millisecond)
			continue
		}
		res := make(common.DepositAddresses)
		res[chain] = common.DepositAddress{
			Address: resAddr.Address,
			Memo:    resAddr.Tag,
		}
		return res
	}
}
func (acc *Account) Send(amount common.Uint, srcAsset common.Asset, target common.Account, wait bool) (txId *string, sent common.Uint, err error) {
	if amount.IsZero() {
		return nil, common.ZeroUint(), errors.New("cannot send zero amount")
	}
	if acc.ex.chains == nil {
		return nil, common.ZeroUint(), errors.New(fmt.Sprintf("failed to find asset for ticker chain for ticker %s in wallet %s", srcAsset.Ticker, acc.GetName()))
	}

	// add al possible chains
	chi, found := acc.ex.chains[srcAsset.Ticker]
	if !found {
		return nil, common.ZeroUint(), errors.New(fmt.Sprintf("failed to find asset for ticker chain for ticker %s in wallet %s", srcAsset.Ticker, acc.GetName()))
	}
	srcAsset, _ = common.NewMultiChainAsset(chi.chain, srcAsset.Symbol.String())
	chain, depositAddress, err := acc.getTargetDepositAddress(srcAsset, target)
	if err != nil {
		return nil, common.ZeroUint(), err
	}
	// fee and min withdraw
	chd, found := chi.chainDetails[chain]
	fee := common.ZeroUint()
	mul := common.OneFx8Uint()
	if found {
		fee = chd.withdrawFee
		mul = chd.withdrawMul
	}
	startTime := time.Now().Unix() * 1000

	// do withdraw
	sent = amount.RoundTo(mul)
	amountStr := sent.String()
	var res *binapi.CreateWithdrawResponse
	for i := 0; ; i++ {
		res, err = acc.bin.NewCreateWithdrawService().
			Address(depositAddress.Address).
			AddressTag(depositAddress.Memo).
			Amount(amountStr).
			Asset(srcAsset.Ticker.String()).
			Network(chain).
			Do(context.Background())
		if err != nil || (i >= c.BINANCE_GETSERVICE_ERR_SLEEP_TRY_CNT && !res.Success) || (!res.Success && strings.Contains(res.Msg, "The user has insufficient balance available")) {
			if err == nil {
				err = errors.New(res.Msg)
			}
			acc.logger.Error().Err(err).Msg("bin.NewCreateWithdrawService failed")
			return nil, common.ZeroUint(), errors.Wrap(err, "bin.NewCreateWithdrawService failed")
		}
		if err != nil || !res.Success {
			if err == nil {
				err = errors.New("bin.NewCreateWithdrawService resturned Success == false")
			}
			acc.logger.Error().Err(err).Int("i", i).Msg("bin.NewCreateWithdrawService failed - trying again")
			time.Sleep(c.BINANCE_GETSERVICE_ERR_SLEEP_MS * time.Millisecond)
			continue
		}
		break
	}
	acc.logger.Info().Str("deposit address", depositAddress.Address).Str("asset", srcAsset.Ticker.String()).Str("from", acc.GetName()).Str("to", target.GetName()).Str("chain", chain).Msg("funds sent (async)")

	// if wait then wait
	if wait {
		withdraw, err := acc.waitForWithdraw(res.ID, srcAsset.Ticker.String(), startTime)
		if err != nil {
			return nil, common.ZeroUint(), errors.Wrap(err, "waitForWithdraw failed")
		}
		time.Sleep(400 * time.Millisecond)
		return &withdraw.ID, common.NewUintFromFloat(withdraw.TransactionFee), nil
	}
	return nil, fee, nil
}
func (acc *Account) waitForWithdraw(withdrawId, ticker string, startTime int64) (withdraw *binapi.Withdraw, err error) {
	worker := common.ThreadSafeSliceWorker{ListenerC: make(chan interface{}, 10)}
	acc.listeners.Push(&worker)
	defer acc.listeners.Remove(&worker)
	timeoutC := time.After(24 * time.Hour)
	tickerC := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-common.Quit:
			return
		case <-worker.ListenerC:
			withdraw, err = acc.checkWithdraw(withdrawId, ticker, startTime)
			if withdraw != nil {
				return withdraw, err
			}
		case <-tickerC.C:
			withdraw, err = acc.checkWithdraw(withdrawId, ticker, startTime)
			if withdraw != nil {
				return withdraw, err
			}
		case <-timeoutC:
			err = errors.New(fmt.Sprintf("waitForWithdraw timeout, withdrawId = %s, ticker = %s, startTime = %v", withdrawId, ticker, startTime))
			acc.logger.Error().Err(err)
			return
		}
	}
}
func (acc *Account) checkWithdraw(withdrawId, ticker string, startTime int64) (withdraw *binapi.Withdraw, err error) {
	var withdraws []*binapi.Withdraw
	err = common.Try(3, c.BINANCE_GETSERVICE_ERR_SLEEP_MS, func() error {
		withdraws, err = acc.bin.NewListWithdrawsService().
			Asset(ticker).
			Status(6). // Completed
			StartTime(startTime).
			Do(context.Background())
		return err
	})
	if err != nil {
		return nil, errors.Wrap(err, "bin.NewListWithdrawsService failed")
	}
	for _, withdraw := range withdraws {
		if withdraw.ID == withdrawId {
			return withdraw, nil
		}
	}
	return nil, nil // withdraw not completed yet
}
func (acc *Account) WaitForDeposit(txID string) error {
	startTime := (time.Now().Unix() - 60) * 1000 // minus one minute
	_, err := acc.waitForDeposit(txID, startTime)
	return err
}
func (acc *Account) waitForDeposit(txID string, startTime int64) (deposit *binapi.Deposit, err error) {
	worker := common.ThreadSafeSliceWorker{ListenerC: make(chan interface{}, 10)}
	acc.listeners.Push(&worker)
	defer acc.listeners.Remove(&worker)
	timeoutC := time.After(24 * time.Hour)
	for {
		select {
		case <-common.Quit:
			return
		case <-worker.ListenerC:
			deposit, err = acc.checkDeposit(txID, startTime)
			if deposit != nil {
				return deposit, err
			}
		case <-timeoutC:
			err = errors.New(fmt.Sprintf("waitForDeposit timeout, txID = %s, startTime = %v", txID, startTime))
			acc.logger.Error().Err(err)
			return
		}
	}
}
func (acc *Account) checkDeposit(txID string, startTime int64) (deposit *binapi.Deposit, err error) {
	var deposits []*binapi.Deposit
	err = common.Try(3, c.BINANCE_GETSERVICE_ERR_SLEEP_MS, func() error {
		deposits, err = acc.bin.NewListDepositsService().
			Do(context.Background())
		return err
	})
	if err != nil {
		return nil, errors.Wrap(err, "bin.NewListDepositsService failed")
	}
	for _, deposit := range deposits {
		if deposit.TxID == txID && (deposit.Status == 1 || deposit.Status == 6) {
			return deposit, nil
		}
	}
	return nil, nil // deposit not completed yet
}
func (acc *Account) Refresh() common.Balances {
	return acc.readBalances()
}
func (acc *Account) readBalances() common.Balances {
	ip, err := common.GetPublicIP()
	if err != nil {
		ip, err = common.GetPublicIP()
		if err != nil {
			ip = "cannot get IP address: " + err.Error()
		}
	}
	acc.logger.Info().Msgf("PUBLIC IP: %s)", ip)
	res, err := acc.bin.NewGetAccountService().Do(context.Background())
	if err != nil {
		ip, err2 := common.GetPublicIP()
		if err2 != nil {
			ip = "cannot get IP address: " + err2.Error()
		}
		acc.logger.Err(err).Msgf("readBalances failed (ip: %s): %s", ip, err)
		if strings.Contains(err.Error(), "msg=Invalid API-key, IP, or permissions for action") {
			acc.logger.Panic().Msgf("INVALID API KEY - check whitelisted IP address on Exchanges, our IP: %s", ip)
		}
		return common.Balances{}
	}
	balances := make(common.Balances, len(res.Balances))
	for _, a := range res.Balances {
		am := common.NewUintFromString(a.Free)
		as, err := common.NewAsset(a.Asset)
		if err != nil {
			acc.logger.Err(err).Str("asset", a.Asset).Msg("Unknown asset")
			continue
		}
		balances[as.Ticker.String()] = am
	}
	acc.atomicBalances.Store(balances)
	acc.logger.Debug().Msgf("Account balances read: %v", balances)
	return balances
}

func (acc *Account) getTargetDepositAddress(srcAsset common.Asset, target common.Account) (chain string, depositAddressPtr *common.DepositAddress, err error) {
	depositAddresses := target.GetDepositAddresses(srcAsset)
	if depositAddresses == nil {
		err := errors.New(fmt.Sprintf("cannot send asset, deposit asddress not found [asset = %s, account = %s]", srcAsset, target.GetName()))
		acc.logger.Error().Err(err).Msg("problem")
		return "", nil, err
	}
	var depositAddress common.DepositAddress
	found := false
	if len(srcAsset.Chain) > 1 { // we have more common chains, try to find one of the preffered chains ("BNB", "BSC", "THOR", "ETH")
		for _, chain = range c.CHAIN_SENDING_PREFERENCE {
			ch, _ := common.NewChain(chain)
			if !srcAsset.Chain.Contains(ch) {
				continue
			}
			depositAddress, found = depositAddresses[chain]
			if !found {
				continue
			}
			break
		}
		if !found { // if there is not common preffered chain, so we choose the first
			for _, chain = range srcAsset.Chain {
				depositAddress, found = depositAddresses[chain]
				if found {
					break
				}
			}
		}
		if !found {
			err := errors.New(fmt.Sprintf("chain not found in destination account [asset = %s, account = %s]", srcAsset, target.GetName()))
			acc.logger.Error().Err(err).Msg("problem")
			return "", nil, err
		}
	} else if len(srcAsset.Chain) == 1 {
		chain = srcAsset.Chain[0]
		depositAddress, found = depositAddresses[chain]
		if !found {
			err := errors.New(fmt.Sprintf("chain not found in destination account [asset = %s, account = %s]", srcAsset, target.GetName()))
			acc.logger.Error().Err(err).Msg("problem")
			return "", nil, err
		}
	} else {
		err := errors.New(fmt.Sprintf("no chain defined on asset, cannot withdraw from account [asset = %s, account = %s]", srcAsset, target.GetName()))
		acc.logger.Error().Err(err).Msg("problem")
		return "", nil, err
	}
	return chain, &depositAddress, nil
}
func (acc *Account) accountWatcher() {
	var stopC chan struct{}
	eventC := make(chan struct{})
	errorC := make(chan struct{})
	var listenKey string
	var err error
	err = common.Try(3, 100, func() error {
		listenKey, err = acc.bin.NewStartUserStreamService().Do(context.Background())
		return err
	})
	if err != nil {
		acc.logger.Err(err).Msg("NewStartUserStreamService failed")
	}
	wsHandler := func(msg []byte) {
		acc.logger.Debug().Str("message", string(msg)).Msg("WsUserDataServe message")
		var result map[string]interface{}
		json.Unmarshal([]byte(msg), &result)
		if result["e"] == "balanceUpdate" {
			acc.logger.Info().Msg("binance balance Update event")
			eventC <- struct{}{}
		}
	}
	errHandler := func(err error) { acc.logger.Debug().Err(err).Msg("WsUserDataServe failed (but we continue)") }
	err = common.Try(3, 100, func() error { _, stopC, err = binapi.WsUserDataServe(listenKey, wsHandler, errHandler); return err })
	if err != nil {
		acc.logger.Err(err).Msg("binance.WsUserDataServe failed")
	}
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-common.Quit:
			close(stopC)
			return
		case <-eventC:
			acc.Refresh()
			acc.listeners.Iter(func(worker *common.ThreadSafeSliceWorker) {
				worker.ListenerC <- struct{}{}
			})
		case <-ticker.C:
			acc.Refresh()
			acc.listeners.Iter(func(worker *common.ThreadSafeSliceWorker) {
				worker.ListenerC <- struct{}{}
			})
		case <-errorC:
			err = common.Try(3, 100, func() error { _, stopC, err = binapi.WsUserDataServe(listenKey, wsHandler, errHandler); return err })
			if err != nil {
				acc.logger.Err(err).Msg("binance.WsUserDataServe failed")
			}
		}
	}
}
func account_interface_test() {
	var _ common.Account = &Account{}
}
