package bot

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/zlyzol/skadi/internal/common"
)
// MM structure
type MM struct {
	logger       zerolog.Logger
	side		 common.OrderSide
	amount       common.Uint
	limBuyPrice  common.Uint
	limSellPrice common.Uint
	h 			 *Hunter
}

func NewMM(side common.OrderSide, amount, limBuyPrice, limSellPrice common.Uint, h *Hunter) *MM {
	mm := MM{
		logger:		  log.With().Str("module", "MM").Str("exchange", h.ob.GetExchange().GetName()).Str("market", h.ob.GetMarket().String()).Logger(),
		side:         side,
		amount:       amount,
		limBuyPrice:  limBuyPrice,
		limSellPrice: limSellPrice,
		h:       	  h,
	}
	return &mm
}

func (mm *MM) process() {

}
