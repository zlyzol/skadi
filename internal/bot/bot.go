package bot

import (
	"time"

	"github.com/pkg/errors"
//	"google.golang.org/genproto/googleapis/rpc/status"
	//	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gitlab.com/zlyzol/skadi/internal/common"
	"gitlab.com/zlyzol/skadi/internal/config"
	"gitlab.com/zlyzol/skadi/internal/store"
)

type Bot struct {
	cfg			*config.Configuration
	logger		zerolog.Logger
	store		store.Store
	startTime	time.Time
	exchanges	map[string]common.Exchange
	accounts	*common.Accounts
	hunters		[]*Hunter
	status		int8
	statusError	error
	WalletChecker	*WalletChecker
	syncer			*HunterSyncer
}
var BotStatus = struct {
	Starting	int8
	Running		int8
	Stopped		int8
	Error		int8
} {0,1,2,3}

// NewUsecase initiate a new Usecase.
func NewBot(store store.Store, cfg *config.Configuration) (*Bot, error) {
	if cfg == nil {
		return nil, errors.New("conf can't be nil")
	}
	syncer := &HunterSyncer{
		HUNT_SYNC_ONE: true,
		AtomicHUNT_SYNC_ONE: 1, // hunting disabled for now
	}
	bot := Bot{
		cfg:  cfg,
		logger:	log.With().Str("module", "bot").Logger(),
		store: store,
		syncer: syncer,
		WalletChecker: NewWalletChecker(syncer),
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
// Stop bot
func (bot *Bot) Stop() error {
	err := bot.disconnectExchanges()
	return err
}
// Start bot
func (bot *Bot) Start() error {
	ip := common.GetPublicIP()
	bot.logger.Info().Msgf("PUBLIC IP: %s)", ip)
	bot.startTime = time.Now()
	err := bot.connectExchanges()
	if err != nil {
		bot.status = BotStatus.Error
		bot.statusError = errors.Wrap(err, "failed to connect exchanges")
		return bot.statusError
	}
	bot.WalletChecker.start()
	return bot.StartHunters()
}
// StartHunters bot
func (bot *Bot) StartHunters() error {
	if 	bot.status == BotStatus.Running {
		return nil
	}
	bot.status = BotStatus.Starting
	err := bot.startHunters()
	if err != nil {
		bot.status = BotStatus.Error
		bot.statusError = errors.Wrap(err, "failed to start hunters")
		return bot.statusError
	}
	bot.status = BotStatus.Running
	bot.statusError = nil
	return bot.statusError
}
// Stop bot
func (bot *Bot) StopHunters() error {
	if 	bot.status == BotStatus.Stopped {
		return nil
	}
	err := bot.stopHunters()
	if err != nil {
		bot.status = BotStatus.Error
		bot.statusError = errors.Wrap(err, "failed to start hunters")
		return bot.statusError
	}
	bot.status = BotStatus.Stopped
	bot.statusError = nil
	return bot.statusError
}
// Get Bot Status
func (bot *Bot) GetStatus() string {
	if bot.status == BotStatus.Running {
		return "running"
	} else if bot.status == BotStatus.Stopped {
		return "stopped"
	} else if bot.status == BotStatus.Starting {
		return "starting"
	} else if bot.status == BotStatus.Error {
		return "error: " + bot.statusError.Error()
	}
	return "unknown"
}
