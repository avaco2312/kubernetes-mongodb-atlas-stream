package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"stream/clientes-go/contratos"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var collEventos *mongo.Collection
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
	if err := mongoClient.Ping(ctx, readpref.Primary()); err != nil {
		os.Exit(1)
	}
	collEventos = mongoClient.Database("boletia").Collection("eventos")
	_, err = collEventos.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "nombre", Value: 1}}, Options: options.Index().SetUnique(true)})
	if err != nil {
		os.Exit(1)
	}

	r := mux.NewRouter()
	r.HandleFunc("/eventos", getEventos).Methods("GET")
	r.HandleFunc("/eventos/{nombre}", getEvento).Methods("GET")
	r.HandleFunc("/eventos", postEvento).Methods("POST")
	r.HandleFunc("/eventos/{nombre}", deleteEvento).Methods("DELETE")
	log.Fatal(http.ListenAndServe(":8070", r))
}

func deleteEvento(w http.ResponseWriter, r *http.Request) {
	nombre := mux.Vars(r)["nombre"]
	res, err := collEventos.UpdateOne(ctx, bson.M{"nombre": nombre}, bson.M{"$set": bson.M{"estado": "C"}})
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if res.MatchedCount == 0 {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	w.Write([]byte("Evento: " + nombre + " cancelado"))
}

func postEvento(w http.ResponseWriter, r *http.Request) {
	evento := contratos.Evento{}
	err := json.NewDecoder(r.Body).Decode(&evento)
	if err != nil {
		http.Error(w, "JSON no válido", http.StatusBadRequest)
		return
	}
	if evento.Capacidad <= 0 {
		http.Error(w, "cantidad debe ser mayor que 0", http.StatusBadRequest)
		return
	}
	evento.Estado = "A"
	evento.Id = primitive.NewObjectID()
	res, err := collEventos.InsertOne(ctx, &evento)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			http.Error(w, "Evento "+evento.Nombre+" ya existente", http.StatusBadRequest)
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	pId, ok := res.InsertedID.(primitive.ObjectID)
	if !ok {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	evento.Id = pId
	sEvento, _ := json.Marshal(&evento)
	var resp bytes.Buffer
	_ = json.Indent(&resp, sEvento, "", "  ")
	w.Write(resp.Bytes())
}

func getEvento(w http.ResponseWriter, r *http.Request) {
	nombre := mux.Vars(r)["nombre"]
	evento := contratos.Evento{}
	err := collEventos.FindOne(ctx, bson.M{"nombre": nombre}).Decode(&evento)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json;charset=utf8")
	json.NewEncoder(w).Encode(&evento)
}

func getEventos(w http.ResponseWriter, r *http.Request) {
	cur, err := collEventos.Find(ctx, bson.D{})
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	eventos := []contratos.Evento{}
	if err = cur.All(ctx, &eventos); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json;charset=utf8")
	json.NewEncoder(w).Encode(&eventos)
}
