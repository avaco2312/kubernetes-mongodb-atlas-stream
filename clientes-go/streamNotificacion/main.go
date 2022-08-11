package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"stream/clientes-go/contratos"
	"stream/clientes-go/streams"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var ctx context.Context
var svc *ses.SES
var sender string

const CharSet = "UTF-8"

type mensaje struct {
	subject  string
	textbody string
}

var mensajes = [...]mensaje{
	{
		subject:  "Confirmación de reserva",
		textbody: "Su reserva %s de %d boletos para el evento %s está confirmada",
	},
	{
		subject:  "Cancelación de reserva",
		textbody: "Su reserva %s de %d boletos para el evento %s fue cancelada, el evento fue suspendido por los organizadores",
	},
	{
		subject:  "Cancelación de reserva",
		textbody: "Su reserva %s de %d boletos para el evento %s fue cancelada a petición suya",
	},
}

func main() {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-west-2")},
	)
	if err != nil {
		os.Exit(1)
	}
	svc = ses.New(sess)
	sender = os.Getenv("SENDER_EMAIL")

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
			Value: bson.D{{Key: "operationType",
				Value: bson.D{{Key: "$in",
					Value: bson.A{"insert", "update"},
				}},
			}},
		}},
	}
	streams.ConnectStream(ctx, mongoClient, "notificacion", "reservas", &pipeline, procesa)
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
	err = EnviaEmail(reserva.Id.Hex(), reserva.Evento, reserva.Estado, reserva.Email, reserva.Cantidad)
	return err
}

func EnviaEmail(id, evento, estado, email string, cantidad int) error {
	tipo := strings.Index("ACX", estado)
	if tipo == -1 {
		return errors.New("estado de la reserva no valido")
	}
	msg := fmt.Sprintf(mensajes[tipo].textbody, id, cantidad, evento)
	input := &ses.SendEmailInput{
		Destination: &ses.Destination{
			CcAddresses: []*string{},
			ToAddresses: []*string{
				aws.String(email),
			},
		},
		Message: &ses.Message{
			Body: &ses.Body{
				Text: &ses.Content{
					Charset: aws.String(CharSet),
					Data:    aws.String(msg),
				},
			},
			Subject: &ses.Content{
				Charset: aws.String(CharSet),
				Data:    aws.String(mensajes[tipo].subject),
			},
		},
		Source: aws.String(sender),
	}
	_, err := svc.SendEmail(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ses.ErrCodeMessageRejected:
				log.Println(ses.ErrCodeMessageRejected, aerr.Error())
			case ses.ErrCodeMailFromDomainNotVerifiedException:
				log.Println(ses.ErrCodeMailFromDomainNotVerifiedException, aerr.Error())
			case ses.ErrCodeConfigurationSetDoesNotExistException:
				log.Println(ses.ErrCodeConfigurationSetDoesNotExistException, aerr.Error())
			default:
				log.Println(aerr.Error())
			}
			return nil
		} else {
			return err
		}
	}
	return nil
}
