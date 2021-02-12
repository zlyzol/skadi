package mongo

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/zlyzol/skadi/internal/config"
	mongodb "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Mongo struct {
	logger zerolog.Logger
	cfg    config.MongoConfiguration
	db     *mongodb.Client
}

func NewClient(cfg config.MongoConfiguration) (*Mongo, error) {
	if cfg.Host == "skip" {
		return nil, nil
	}
	time.Sleep(3 * time.Second)
	logger := log.With().Str("module", "mongo").Logger()
	connStr := fmt.Sprintf("mongodb://%s:%v", cfg.Host, cfg.Port)

	// connect to DB
	clientOptions := options.Client().ApplyURI(connStr)
	db, err := mongodb.Connect(context.TODO(), clientOptions)
	if err != nil {
		logger.Err(err).Msg("Open")
		return nil, errors.Wrap(err, "failed to connect to mongodb")
	}
	err = db.Ping(context.TODO(), nil)
	if err != nil {
		logger.Err(err).Msg("Ping")
		return nil, errors.Wrap(err, "failed to ping mongodb")
	}
	return &Mongo{
		cfg:    cfg,
		db:     db,
		logger: logger,
	}, nil
}

func (m *Mongo) Ping() error {
	return m.db.Ping(context.TODO(), nil)
}
