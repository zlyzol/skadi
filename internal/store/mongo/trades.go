package mongo

import (
	"context"

	"github.com/pkg/errors"
	"gitlab.com/zlyzol/skadi/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (m *Mongo) GetTrades() (models.Trades, error) {
	var limit int64 = 200
	collection := m.db.Database(m.cfg.Database).Collection("trades")
	findOptions := options.Find()
	findOptions.SetLimit(limit)
	findOptions.SetSort(bson.D{{"time", -1}})
	results := make(models.Trades, 0, 100)
	cur, err := collection.Find(context.TODO(), bson.D{{}}, findOptions)
	if err != nil {
		return results, errors.Wrap(err, "failed to read trades from mongo")
	}
	for cur.Next(context.TODO()) {
		var elem models.Trade
		err := cur.Decode(&elem)
		if err != nil {
			return results, errors.Wrap(err, "failed to read trades from mongo")
		}
		results = append(results, elem)
	}
	if err := cur.Err(); err != nil {
		return results, errors.Wrap(err, "failed to read trades from mongo")
	}
	cur.Close(context.TODO())
	return results, nil
}

func (m *Mongo) InsertTrade(record models.Trade) error {
	collection := m.db.Database(m.cfg.Database).Collection("trades")
	_, err := collection.InsertOne(context.TODO(), record)
	if err != nil {
		return errors.Wrap(err, "failed to read trades from mongo")
	}
	return nil
}
