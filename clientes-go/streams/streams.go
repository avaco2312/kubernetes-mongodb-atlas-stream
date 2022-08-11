package streams

import (
	"context"
	"os"
	"stream/clientes-go/contratos"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type procFunction func(documento contratos.StreamDoc, token string) (err error)

func ConnectStream(ctx context.Context, client *mongo.Client, stream, collection string, pipeline *mongo.Pipeline, procesa procFunction) {
	tokenColl := client.Database("boletia").Collection("streamToken")
	optFind := options.FindOne().SetProjection(bson.D{{Key: "token", Value: 1}})
	mToken := primitive.M{}
	err := tokenColl.FindOne(ctx, bson.D{{Key: "stream", Value: stream}}, optFind).Decode(&mToken)
	if err != nil && err != mongo.ErrNoDocuments {
		waitAndTerminate()
	}
	optStream := options.ChangeStream().SetFullDocument(options.UpdateLookup)
	if _, ok := mToken["token"]; ok {
		token := mToken["token"].(string)
		optStream = optStream.SetStartAfter(bson.M{"_data": token})
	}
	coll := client.Database("boletia").Collection(collection)
	collStream, err := coll.Watch(ctx, *pipeline, optStream)
	if err != nil {
		waitAndTerminate()
	}
	for collStream.Next(ctx) {
		var data bson.M
		if err := collStream.Decode(&data); err != nil {
			waitAndTerminate()
		}
		newToken := data["_id"].(primitive.M)["_data"].(string)
		document := contratos.StreamDoc{}
		document.OperationType = data["operationType"].(string)
		document.FullDocument = data["fullDocument"].(primitive.M)
		err := procesa(document, newToken)
		if err != nil {
			waitAndTerminate()
		}
		update := bson.M{"$set": bson.M{"token": newToken}}
		optToken := options.Update().SetUpsert(true)
		_, err = tokenColl.UpdateOne(ctx, bson.D{{Key: "stream", Value: stream}}, update, optToken)
		if err != nil {
			waitAndTerminate()
		}
	}
}

func waitAndTerminate() {
	time.Sleep(2 * time.Second)
	os.Exit(1)
}
