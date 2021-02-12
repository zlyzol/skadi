package bot

import (
//	"gitlab.com/zlyzol/skadi/internal/common"
)

func (h *Hunter) releaseTrap() {
/*
ARB:
- ak ideme robit arb, uvolnime MM ordery
    - jednak, aby neboli zablokovane pocas arb/move/swap
    - druhak, aby sa nam uvolnilo quote asset
*/
}

func (h *Hunter) preCheck() *MM {
	/*
	pred MM / ARB check:
	- checkneme, ci mame nejake MM ordery
	- ak nemame, mozeme pokracovat checkPrey, potom checkMM
	- ak mame checkneme ich stav, ci sa nieco nevykonalo
		- ak sa MM order vykonal full, tak dokoncime MM arb -> potom zopakujeme check
		- ak sa nevykonal ziadny, tak
			- z tabulky sucasnych bid/ask-ov vymazeme MM
			- vypocitame nove MM ordery
			- ak su rovnake, tak koncime tento predMMcheck
			- ak su rozne, tak:
				- ak je zmena len v amountoch, tak ak je zmena vacsia ako 20% (x), tak pridame/uberieme
				- ak je zmena v cenach/poziciach, tak mazeme stare a vkladame nove
	*/
    return nil
}

func (h *Hunter) findTrap() *MM {
	/*
	MM:
	- ak nie je prey (ci uz je poolpr v strede alebo je maly yield) -> spustame checkMM()
	- najdeme cenu o jeden tick nizsie ako ask a vyssie ako bid, ktorych suma je vacsia ako 50 (x) run, ak je vyssia / nizsia ako poolpr
	- skusime pool.GetTradeSize, potom vypocitame yield, najprv bez accLimitu, potom s accLimitom -> rozdiel hlasime
	- logneme rozdiel ak je velky
	- ak nemame dostatocny yield, tak koncime
	- vlozime 1 alebo 2 GTC LIMIT order(y)
	- koncime - ak nieco nastane, onChange nas spusti
	*/

	return nil
}