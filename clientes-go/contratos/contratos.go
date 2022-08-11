package contratos

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Evento struct {
	Id        primitive.ObjectID `bson:"_id"`
	Nombre    string
	Capacidad int
	Categoria string
	Estado    string
}

type Inventario struct {
	Id         primitive.ObjectID `bson:"_id"`
	Nombre     string
	Disponible int
	Categoria  string
	Estado     string
	IdReserva  primitive.ObjectID
	Email      string
	Cantidad   int
}

type ListaInventario struct {
	Id         primitive.ObjectID `bson:"_id"`
	Nombre     string
	Disponible int
	Categoria  string
	Estado     string
}

type Reserva struct {
	Id       primitive.ObjectID `bson:"_id" json:"_id"`
	Evento   string
	Estado   string
	Email    string
	Cantidad int
}

type StreamDoc struct {
	OperationType string
	FullDocument  primitive.M
}
