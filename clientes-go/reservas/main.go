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

var collInventario *mongo.Collection
var collReservas *mongo.Collection
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
	collInventario = mongoClient.Database("boletia").Collection("inventario")
	_, err = collInventario.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "nombre", Value: 1}}, Options: options.Index().SetUnique(true)})
	if err != nil {
		os.Exit(1)
	}
	collReservas = mongoClient.Database("boletia").Collection("reservas")
	_, err = collReservas.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "evento", Value: 1}, {Key: "email", Value: 1}}, Options: options.Index().SetUnique(false)})
	if err != nil {
		os.Exit(1)
	}

	r := mux.NewRouter()
	r.HandleFunc("/reservas/inventario", getEventos).Methods("GET")
	r.HandleFunc("/reservas/inventario/{nombre}", getEvento).Methods("GET")
	r.HandleFunc("/reservas/{evento}/{email}", getReservasCliente).Methods("GET")
	r.HandleFunc("/reservas/{id}", getReservaId).Methods("GET")
	r.HandleFunc("/reservas", postReserva).Methods("POST")
	r.HandleFunc("/reservas/{id}", deleteReservaId).Methods("DELETE")
	log.Fatal(http.ListenAndServe(":8071", r))
}

func getEvento(w http.ResponseWriter, r *http.Request) {
	nombre := mux.Vars(r)["nombre"]
	inventario := contratos.ListaInventario{}
	err := collInventario.FindOne(ctx, bson.M{"nombre": nombre}).Decode(&inventario)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json;charset=utf8")
	json.NewEncoder(w).Encode(&inventario)
}

func getEventos(w http.ResponseWriter, r *http.Request) {
	cur, err := collInventario.Find(ctx, bson.D{})
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	inventarios := []contratos.ListaInventario{}
	if err = cur.All(ctx, &inventarios); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json;charset=utf8")
	json.NewEncoder(w).Encode(&inventarios)
}

func postReserva(w http.ResponseWriter, r *http.Request) {
	reserva := contratos.Reserva{}
	err := json.NewDecoder(r.Body).Decode(&reserva)
	if err != nil {
		http.Error(w, "JSON no válido", http.StatusBadRequest)
		return
	}
	if reserva.Cantidad <= 0 {
		http.Error(w, "Cantidad incorrecta", http.StatusBadRequest)
		return
	}
	reserva.Id = primitive.NewObjectID()
	reserva.Estado = "A"
	sInventario, _ := json.Marshal(&reserva)
	var resp bytes.Buffer
	_ = json.Indent(&resp, sInventario, "", "  ")
	update := bson.D{
		{Key: "$inc", Value: bson.D{{Key: "disponible", Value: -reserva.Cantidad}}},
		{Key: "$set", Value: bson.D{
			{Key: "idreserva", Value: reserva.Id},
			{Key: "email", Value: reserva.Email},
			{Key: "cantidad", Value: reserva.Cantidad},
		}},
	}
	filter := bson.D{
		{Key: "nombre", Value: reserva.Evento},
		{Key: "estado", Value: "A"},
		{Key: "disponible", Value: bson.D{{Key: "$gt", Value: reserva.Cantidad - 1}}}}
	err = collInventario.FindOneAndUpdate(ctx, filter, update).Err()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "evento "+reserva.Evento+" no encontrado o sin capacidad en este momento", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(resp.Bytes())
}

func deleteReservaId(w http.ResponseWriter, r *http.Request) {
	ids := mux.Vars(r)["id"]
	id, err := primitive.ObjectIDFromHex(ids)
	if err != nil || len(id) != 12 {
		http.Error(w, "id incorrecta, el formato es /id/(12 bytes hex)", http.StatusBadRequest)
		return
	}
	reserva := contratos.Reserva{}
	filter := bson.D{
		{Key: "_id", Value: id},
		{Key: "estado", Value: "A"}}
	update := bson.D{
		{Key: "$set", Value: bson.D{{Key: "estado", Value: "X"}}},
	}
	err = collReservas.FindOneAndUpdate(ctx, filter, update).Decode(&reserva)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "reserva Id "+ids+" no encontrada o ya cancelada", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write([]byte("reserva Id: " + ids + " Cliente: " + reserva.Email + " Evento: " + reserva.Evento + " cancelada"))
}

func getReservasCliente(w http.ResponseWriter, r *http.Request) {
	evento := mux.Vars(r)["evento"]
	email := mux.Vars(r)["email"]
	reservas := []contratos.Reserva{}
	cur, err := collReservas.Find(ctx, bson.D{{Key: "evento", Value: evento}, {Key: "email", Value: email}})
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if err = cur.All(ctx, &reservas); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json;charset=utf8")
	json.NewEncoder(w).Encode(&reservas)
}

func getReservaId(w http.ResponseWriter, r *http.Request) {
	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "id incorrecta, el formato es /id/(12 bytes hex)", http.StatusBadRequest)
		return
	}
	reserva := contratos.Reserva{}
	err = collReservas.FindOne(ctx, bson.M{"_id": id}).Decode(&reserva)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json;charset=utf8")
	json.NewEncoder(w).Encode(&reserva)
}
