package db

import (
	"context"
	"time"

	"github.com/DigitalIndependence/models"
	"github.com/gofrs/uuid"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"gopkg.in/mgo.v2/bson"
)

type Database interface {
	GetActiveRequests(ctx context.Context) ([]models.DownloadQueueRequest, error)
	IndexMusicFile(ctx context.Context, file models.MusicFile) error
}

type db struct {
	conn *mongo.Client
	log  *zap.Logger

	// Collections
	downloadQueueRequestCollection *mongo.Collection
	musicFilesCollection           *mongo.Collection
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
		musicFilesCollection:           conn.Database(dbname).Collection("music-files"),
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

// IndexMusicFile indexes a music file in the database
func (d *db) IndexMusicFile(ctx context.Context, file models.MusicFile) error {
	file.ID = uuid.Must(uuid.NewV4()).String()
	file.CreatedAt = time.Now().Unix()
	_, err := d.musicFilesCollection.InsertOne(ctx, file)
	return err
}

// MusicFileExist checks if a music file exists in the database
func (d *db) MusicFileExist(ctx context.Context, title string) (bool, error) {
	var count int64
	count, err := d.musicFilesCollection.CountDocuments(ctx, bson.M{"title": title})
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
