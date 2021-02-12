package bot

import (
	"time"

	"github.com/pkg/errors"
//	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gitlab.com/zlyzol/skadi/internal/config"
	"gitlab.com/zlyzol/skadi/internal/common"
	"gitlab.com/zlyzol/skadi/internal/store"
)

type Bot struct {
	cfg			*config.Configuration
	logger		zerolog.Logger
	store		store.Store
	startTime	time.Time
	exchanges	map[string]common.Exchange
	accounts	*common.Accounts
}

// NewUsecase initiate a new Usecase.
func NewBot(store store.Store, cfg *config.Configuration) (*Bot, error) {
	if cfg == nil {
		return nil, errors.New("conf can't be nil")
	}
	bot := Bot{
		cfg:  cfg,
		logger:	log.With().Str("module", "bot").Logger(),
		store: store,
	}
	return &bot, nil
}

func tetik_swap() common.Uint {
	b, _ := common.NewAsset("BNB.COTI")
	q, _ := common.NewAsset("BNB.BNB")
	r, _ := common.NewAsset("BNB.RUNE")
	pd1 := common.NewSinglePool(nil, common.NewMarket(b, r))
	pd1.AtomicSetInnerData(0, common.Amounts{ BaseAmount: common.NewUintFromFx8String("76242597255640"), QuoteAmount: common.NewUintFromFx8String("4112297753864") })
	pd2 := common.NewSinglePool(nil, common.NewMarket(q, r))
	pd2.AtomicSetInnerData(1, common.Amounts{ BaseAmount: common.NewUintFromFx8String("16086409606268"), QuoteAmount: common.NewUintFromFx8String("583231139499105") })
	p := common.NewPool(pd1, pd2) //COTI/BNB
	x := common.NewUintFromString("22.20183072")
	ret := p.GetSwapReturn(x, common.SwapToAsset)
	_ = ret	
	return ret
}

// Start bot
func (bot *Bot) Start() error {
	bot.startTime = time.Now()
	err := bot.connectExchanges()
	if err != nil {
		return errors.Wrap(err, "failed to connect exchanges")
	}
	err = bot.startHunters()
	if err != nil {
		return errors.Wrap(err, "failed to start hunters")
	}
	return err
}

// Stop bot
func (bot *Bot) Stop() error {
	err := bot.stopHunters()
	return err
}

// StopScanner stops the hunters.
func (bot *Bot) stopHunters() error {
	return nil // hunters are stopped by common.Quit channel close
}

