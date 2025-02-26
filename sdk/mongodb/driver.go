package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"code.finan.one/finan-one-be/fo-utils/l"
)

var ll = l.New()

// MustConnectDB ...

func MustConnectDB(cfg *Config) (*mongo.Client, context.CancelFunc) {
	ll.Info("Start connect to mongodb")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Second)
	ll.Debug("connection string", l.String("conn", cfg.GenConnectString()))
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.GenConnectString()))
	if err != nil {
		panic("error connect mongodb")
	}
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		ll.Error("Ping", l.Error(err))
		panic("error connect mongodb ping")
	}
	ll.Info("connected to mongodb")
	return client, cancel
}
