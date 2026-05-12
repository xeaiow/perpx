package bybit

import (
	"github.com/yourname/poscli/internal/config"
	"github.com/yourname/poscli/internal/exchange"
)

func init() {
	exchange.Register(config.Bybit, func(c *config.Credentials, rt config.Runtime) (exchange.Exchange, error) {
		return New(c, rt), nil
	})
}
