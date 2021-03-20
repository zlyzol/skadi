package bdex

import (
	"fmt"
	"strings"
	"time"
	"sync/atomic"
	"sync"

	"gitlab.com/zlyzol/skadi/internal/common"
	"gitlab.com/zlyzol/skadi/internal/c"
	"gitlab.com/zlyzol/skadi/internal/config"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/binance-chain/go-sdk/client/websocket"

	"github.com/binance-chain/go-sdk/client"
	"github.com/binance-chain/go-sdk/client/transaction"
	types "github.com/binance-chain/go-sdk/common/types"
	"github.com/binance-chain/go-sdk/keys"
	"github.com/binance-chain/go-sdk/types/msg"
)

type Account struct {
	logger   zerolog.Logger
	cfg      *config.AccountConfiguration
	bdex     *bdex
	atomicBalances atomic.Value // common.Balances
	number   int64
	sequence int64
	wallet   string
	bech32   types.AccAddress
	key      keys.KeyManager // Binance DEX wallet
	listeners	common.ThreadSafeSlice
	muxSequence	sync.Mutex

}
func (b *bdex) NewAccount(cfg *config.AccountConfiguration, accs *common.Accounts) (*Account, error) {
	ea, ok := accs.Get(cfg.Name)
	if ok {
		return ea.(*Account), nil
	}
	a := Account{
		logger: log.With().Str("module", "bdex.account").Logger(),
		cfg:    cfg,
		bdex:   b,
	}
	var err error
	ca, ok := cfg.Chains[common.BNBChainStr]
	if !ok {
		return &a, errors.Wrap(err, "failed to find keystore for BNB chain")
	}
	a.key, err = keys.NewKeyStoreKeyManager(ca.KeystoreOrApiKey, ca.PasswordOrSecretKey)
	if err != nil {
		return &a, errors.Wrap(err, "failed to open keystore")
	}
	a.bech32 = a.key.GetAddr()
	a.wallet = a.bech32.String()
	// create Binance DEX Client
	b.dex, err = client.NewDexClient("dex.binance.org", types.ProdNetwork, a.key)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to dex client")
	}
	a.readBalances()
	accs.Add(&a)
	go a.accountWatcher()
	return &a, nil
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
	//acc.logger.Info().Msgf("asset ticker %s not found in balances, returning zero amount", asset.Ticker)
	return common.ZeroUint()
}
func (acc *Account) GetBalances() common.Balances {
	bali := acc.atomicBalances.Load()
	if bali == nil {
		return common.Balances{}
	}
	return bali.(common.Balances)
}
func (acc *Account) Refresh() common.Balances {
	return acc.readBalances()
}
func (acc *Account) GetDepositAddresses(asset common.Asset) common.DepositAddresses {
	return common.DepositAddresses{ BASECHAIN.String(): common.DepositAddress{ Address: acc.wallet, Memo: "" }} 
}
func (acc *Account) GetDex() client.DexClient {
	return acc.bdex.dex
}
func (acc *Account) Send(amount common.Uint, srcAsset common.Asset, target common.Account, wait bool) (txId *string, sent common.Uint, err error) {
	if amount.IsZero() {
		return nil, common.ZeroUint(), errors.New("cannot send zero amount")
	}
	srcAsset, err = common.Symbols.GetSymbolAsset(srcAsset)
	if err != nil {
		return nil, common.ZeroUint(), err
	}
	if srcAsset.Equal(common.EmptyAsset) {
		return nil, common.ZeroUint(), errors.New(fmt.Sprintf("failed to find asset for ticker %s in wallet %s", srcAsset.Ticker, acc.GetName()))
	}
	if srcAsset.Chain.IsEmpty() || srcAsset.Chain.IsUnknown() {
		srcAsset, _ = common.NewMultiChainAsset(BASECHAIN, srcAsset.Symbol.String())
	}
	depositAddress, err := acc.getTargetDepositAddress(srcAsset, target)
	if err != nil { return nil, common.ZeroUint(), err }
	//func (acc *BinAccount) fromDexToCex(asset string, toMove float64) bool {
	toAddr, err := types.AccAddressFromBech32(depositAddress.Address)
	if err != nil { return nil, common.ZeroUint(), errors.Wrapf(err, "failed to convert address to Bech32 %s", depositAddress.Address) }
	msgs := []msg.Transfer{{
		ToAddr: toAddr,
		Coins:  types.Coins{types.Coin{Denom: srcAsset.Symbol.String(), Amount: amount.Fx8Int64()}},
	}}
	//resSend, err := acc.bdex.dex.SendToken(msgs, true, transaction.WithMemo(depositAddress.Memo))
	acc.sequenceLock()
	acc.logger.Info().Msgf("Current sequence is %v", acc.sequence)
	resSend, err := acc.bdex.dex.SendToken(msgs, true, transaction.WithMemo(depositAddress.Memo), transaction.WithAcNumAndSequence(acc.number, acc.sequence))
	if err != nil {
		for i := 0; i < c.BNB_SEND_TOKEN_INV_SEQ_TRY_CNT && strings.Contains(strings.ToLower(err.Error()), "invalid sequence."); i++ {
			acc.sequence++
			acc.logger.Info().Msgf("Incrementing after Invalid sequence, new sequence is %v", acc.sequence)
			resSend, err = acc.bdex.dex.SendToken(msgs, true, transaction.WithMemo(depositAddress.Memo), transaction.WithAcNumAndSequence(acc.number, acc.sequence))
			if err == nil {
				acc.logger.Info().Msg("Incrementing sequence was successfull")
				break
			}
			acc.logger.Info().Msgf("Still error (%s-th / %s) after Incrementing after Invalid sequence, new sequence %s", i + 1, c.BNB_SEND_TOKEN_INV_SEQ_TRY_CNT, err)
		}
	} else {
		acc.logger.Info().Msg("Current sequence was ok")
	}
	acc.sequenceUnlock()
	if err != nil || !resSend.Ok {
		if err == nil {
			err = errors.New("bdex accoun SendToken result Ok is false")
		}
		return nil, common.ZeroUint(), errors.Wrapf(err, "failed to send funds: %s %s from %s to %s ", amount, srcAsset, acc.GetName(), target.GetName())
	}
	acc.logger.Info().Str("deposit address", depositAddress.Address).Str("asset", srcAsset.Ticker.String()).
		Str("from", acc.GetName()).
		Str("to", target.GetName()).
		Str("chain", BASECHAIN.String()).
		Str("amount", amount.String()).
		Str("memo", depositAddress.Memo).
		Str("hash", resSend.Hash).
		Str("commitHash", resSend.TxCommitResult.Hash).
		Msg("funds sent (async)")

	if wait {
		err := target.WaitForDeposit(resSend.TxCommitResult.Hash)
		if err != nil {
			acc.logger.Error().Err(err).Str("hash", resSend.TxCommitResult.Hash).Msg("wait for deposit failed")
			return &resSend.Hash, amount, err
		}
	}
	return &resSend.Hash, amount, nil
}
func (acc *Account) WaitForDeposit(hash string) error {
	return nil // Binance chain tx is immediate
}
func (acc *Account) readBalances() common.Balances {
	balance, err := acc.bdex.dex.GetAccount(acc.wallet)
	for i := 0; err != nil && i < 10; i++ {
		acc.logger.Err(err).Msg("dex.GetAccount failed")
		time.Sleep(100)
		balance, err = acc.bdex.dex.GetAccount(acc.wallet)
	}
	if err != nil {
		return acc.GetBalances()
	}
	balances := make(common.Balances, len(balance.Balances))
	for _, coin := range balance.Balances {
		as, err := common.NewAsset(coin.Symbol)
		if err != nil {
			acc.logger.Err(err).Str("asset", coin.Symbol).Msg("Unknown asset")
			continue
		}
		balances[as.Ticker.String()] = common.NewUintFromFx8(coin.Free)
	}
	acc.muxSequence.Lock()
	acc.number = balance.Number
	acc.sequence = balance.Sequence
	acc.muxSequence.Unlock()
	acc.atomicBalances.Store(balances)

	acc.logger.Debug().Msgf("Account balances read: %+v", balances)
	return balances
}
func (acc *Account) getTargetDepositAddress(srcAsset common.Asset, target common.Account) (depositAddress *common.DepositAddress, err error) {
	depositAddresses := target.GetDepositAddresses(srcAsset)
	if depositAddresses == nil {
		return nil, errors.New(fmt.Sprintf("cannot send asset, deposit asddress not found [asset = %s, account = %s]", srcAsset, target.GetName()))
	}
	addr, found := depositAddresses[BASECHAIN.String()]
	if !found {
		return nil, errors.New(fmt.Sprintf("chain [%s] not found in destination account [asset = %s, account = %s]", BASECHAIN, srcAsset, target.GetName()))
	}
	return &addr, nil
}
func (acc *Account) sequenceLock() (sequence int64, number int64) {
	acc.muxSequence.Lock()
	return acc.sequence, acc.number
}
func (acc *Account) sequenceUnlock() {
	acc.sequence += 1
	acc.muxSequence.Unlock()
}
func (acc *Account) accountWatcher() {
	eventC := make(chan struct{}, 10) // account event receiver channel
	ticker := time.NewTicker(5 * time.Second)
	err := acc.GetDex().SubscribeAccountEvent(acc.wallet, common.Stop, func(event *websocket.AccountEvent) { eventC <- struct{}{} }, nil, nil)
	if err != nil {
		acc.logger.Error().Err(err).Msg("bdex SubscribeAccountEvent failed")
	}
	for {
		select {
		case <-common.Stop:
			return
		case <-eventC:
			acc.logger.Info().Msg("bdex balance Update event")
			acc.Refresh()
			acc.listeners.Iter(func(worker *common.ThreadSafeSliceWorker) {
				worker.ListenerC <- struct{}{}
			})
		case <-ticker.C:
			acc.Refresh()
			acc.listeners.Iter(func(worker *common.ThreadSafeSliceWorker) {
				worker.ListenerC <- struct{}{}
			})
		}
	}
}
func account_interface_test() {
	var _ common.Account = &Account{}
}
