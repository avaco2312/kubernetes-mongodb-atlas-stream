package main

import (
	"context"
	"encoding/json"
	"os"

	"stream/clientes-go/contratos"
	"stream/clientes-go/streams"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var collInventario *mongo.Collection
var ctx context.Context

func main() {
	mongoURL := os.Getenv("MONGO_URL")
	ctx = context.TODO()
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURL))
	if err != nil {
		os.Exit(1)
	}
	defer func() {
		if err = mongoClient.Disconnect(ctx); err != nil {
			os.Exit(1)
		}
	}()
	// Ping the primary
	if err = mongoClient.Ping(ctx, readpref.Primary()); err != nil {
		os.Exit(1)
	}
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match",
			Value: bson.D{{Key: "$and",
				Value: bson.A{
					bson.D{{Key: "operationType", Value: "update"}},
					bson.D{{Key: "fullDocument.estado", Value: "X"}},
				},
			}},
		}},
	}
	collInventario = mongoClient.Database("boletia").Collection("inventario")
	_, err = collInventario.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "nombre", Value: 1}}, Options: options.Index().SetUnique(true)})
	if err != nil {
		os.Exit(1)
	}
	streams.ConnectStream(ctx, mongoClient, "reserva", "reservas", &pipeline, procesa)
}

func procesa(document contratos.StreamDoc, token string) (err error) {
	jsonbody, err := json.Marshal(document.FullDocument)
	if err != nil {
		return
	}
	reserva := contratos.Reserva{}
	err = json.Unmarshal(jsonbody, &reserva)
	if err != nil {
		return
	}
	filter := bson.D{
		{Key: "nombre", Value: reserva.Evento},
		{Key: "estado", Value: "A"},
		{Key: "token", Value: bson.D{{Key: "$ne", Value: token}}},
	}
	update := bson.D{
		{Key: "$inc",
			Value: bson.D{
				{Key: "disponible", Value: reserva.Cantidad},
			},
		},
		{Key: "$set",
			Value: bson.D{
				{Key: "token", Value: token},
			},
		},
	}
	res := collInventario.FindOneAndUpdate(ctx, filter, update)
	err = res.Err()
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}
	return nil
}
