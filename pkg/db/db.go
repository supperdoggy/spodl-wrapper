package db

import (
	"context"

	"github.com/DigitalIndependence/models"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"gopkg.in/mgo.v2/bson"
)

type Database interface {
	GetActiveRequests(ctx context.Context) ([]models.DownloadQueueRequest, error)
}

type db struct {
	conn *mongo.Client
	log  *zap.Logger

	// Collections
	downloadQueueRequestCollection *mongo.Collection
}

func NewDatabase(ctx context.Context, log *zap.Logger, url, dbname string) (Database, error) {
	conn, err := mongo.Connect(context.Background(), options.Client().ApplyURI(url))
	if err != nil {
		return nil, err
	}

	return &db{
		conn: conn,
		log:  log,

		downloadQueueRequestCollection: conn.Database(dbname).Collection("download-queue-requests"),
	}, nil
}

func (d *db) GetActiveRequests(ctx context.Context) ([]models.DownloadQueueRequest, error) {
	var requests []models.DownloadQueueRequest

	cursor, err := d.downloadQueueRequestCollection.Find(ctx, bson.M{"active": true})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var request models.DownloadQueueRequest
		if err := cursor.Decode(&request); err != nil {
			return nil, err
		}

		requests = append(requests, request)
	}

	return requests, nil
}
