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

var collReservas *mongo.Collection
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
			Value: bson.D{{Key: "$and",
				Value: bson.A{
					bson.M{"operationType": "update"},
					bson.D{{Key: "$or",
						Value: bson.A{
							bson.D{{Key: "fullDocument.estado", Value: "C"}},
							bson.D{{Key: "$and",
								Value: bson.A{
									bson.D{{Key: "fullDocument.estado", Value: "A"}},
									bson.D{{Key: "fullDocument.cantidad", Value: bson.D{{Key: "$gt", Value: 0}}}},
								},
							}},
						},
					}},
				},
			}},
		}},
	}
	collReservas = mongoClient.Database("boletia").Collection("reservas")
	_, err = collReservas.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "evento", Value: 1}, {Key: "email", Value: 1}}, Options: options.Index().SetUnique(false)})
	if err != nil {
		os.Exit(1)
	}
	streams.ConnectStream(ctx, mongoClient, "inventario", "inventario", &pipeline, procesa)
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
	switch inventario.Estado {
	case "A":
		reserva := contratos.Reserva{}
		reserva.Id = inventario.IdReserva
		reserva.Evento = inventario.Nombre
		reserva.Estado = inventario.Estado
		reserva.Email = inventario.Email
		reserva.Cantidad = inventario.Cantidad
		_, err = collReservas.InsertOne(ctx, &reserva)
		if err != nil && !mongo.IsDuplicateKeyError(err) {
			return err
		}
	case "C":
		filter := bson.D{
			{Key: "evento", Value: inventario.Nombre},
			{Key: "estado", Value: "A"},
		}
		update := bson.D{{Key: "$set", Value: bson.D{{Key: "estado", Value: "C"}}}}
		_, err = collReservas.UpdateMany(ctx, filter, update)
		return err
	}
	return nil
}
