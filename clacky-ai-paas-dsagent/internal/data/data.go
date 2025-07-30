package data

import (
	"dsagent/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
)

// Note: Data providers are now registered in the Dig container
// See internal/container/container.go for provider registration

// Data .
type Data struct {
	// TODO wrapped database client
}

// NewData .
func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	cleanup := func() {
		log.NewHelper(logger).Info("closing the data resources")
	}
	return &Data{}, cleanup, nil
}
