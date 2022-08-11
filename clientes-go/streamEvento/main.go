package main

import (
	"context"
	"encoding/json"
	"os"

	"stream/clientes-go/contratos"
	"stream/clientes-go/streams"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var collInventario *mongo.Collection
var ctx context.Context

func main() {
	mongoURL := os.Getenv("MONGO_URL")
	ctx = context.TODO()
	var err error
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
			Value: bson.D{{Key: "operationType",
				Value: bson.D{{Key: "$in",
					Value: bson.A{"insert", "update"},
				}},
			}},
		}},
		bson.D{{Key: "$addFields",
			Value: bson.M{"fullDocument.disponible": "$fullDocument.capacidad"},
		}},
	}
	collInventario = mongoClient.Database("boletia").Collection("inventario")
	_, err = collInventario.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "nombre", Value: 1}}, Options: options.Index().SetUnique(true)})
	if err != nil {
		os.Exit(1)
	}
	streams.ConnectStream(ctx, mongoClient, "evento", "eventos", &pipeline, procesa)
}

func procesa(document contratos.StreamDoc, token string) (err error) {
	jsonbody, err := json.Marshal(document.FullDocument)
	if err != nil {
		return
	}
	inventario := contratos.Inventario{}
	err = json.Unmarshal(jsonbody, &inventario)
	if err != nil {
		return
	}
	switch document.OperationType {
	case "insert":
		inventario.Id = primitive.NewObjectID()
		_, err = collInventario.InsertOne(ctx, &inventario)
		if err != nil && !mongo.IsDuplicateKeyError(err) {
			return err
		}
	case "update":
		filter := bson.D{
			{Key: "nombre", Value: inventario.Nombre},
			{Key: "estado", Value: "A"},
		}
		update := bson.M{"$set": bson.M{"estado": "C"}}
		res := collInventario.FindOneAndUpdate(ctx, filter, update)
		err = res.Err()
		if err != nil && err != mongo.ErrNoDocuments {
			return err
		}
	}
	return nil
}
