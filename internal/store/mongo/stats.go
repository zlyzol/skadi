package mongo

import (
	"context"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	//	mongodb "go.mongodb.org/mongo-driver/mongo"
	//	"go.mongodb.org/mongo-driver/mongo/options"
	"gitlab.com/zlyzol/skadi/internal/models"
)

func (m *Mongo) GetStats() (models.Stats, error) {
	c := m.db.Database(m.cfg.Database).Collection("trades")
	pipe := []bson.M{
		{"$group": bson.M{
			"_id":         "",
			"totalVolume": bson.M{"$sum": "$amountin"},
			"AvgPNL":      bson.M{"$avg": "$pnl"},
			"avgTrade":    bson.M{"$avg": "$amountin"},
			"tradeCount":  bson.M{"$sum": 1},
		}},
	}
	cur, err := c.Aggregate(context.TODO(), pipe)
	result := models.Stats{}
	if err != nil {
		return result, errors.Wrap(err, "failed to read trades from mongo")
	}
	if cur.Next(context.TODO()) {
		var elem models.Stats
		err := cur.Decode(&elem)
		if err != nil {
			return result, errors.Wrap(err, "failed to decode trades from mongo")
		}
		result = elem
	}
	if err := cur.Err(); err != nil {
		return result, errors.Wrap(err, "failed to read trades from mongo")
	}
	cur.Close(context.TODO())

	return result, nil
}
