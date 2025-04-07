package db

import (
	"context"
	"errors"
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
	UpdateActiveRequest(ctx context.Context, request models.DownloadQueueRequest) error
	IndexMusicFile(ctx context.Context, file models.MusicFile) error
}

type db struct {
	conn *mongo.Client
	log  *zap.Logger

	url    string
	dbname string
}

func NewDatabase(ctx context.Context, log *zap.Logger, url, dbname string) (Database, error) {
	conn, err := mongo.Connect(context.Background(), options.Client().ApplyURI(url))
	if err != nil {
		return nil, err
	}

	return &db{
		conn: conn,
		log:  log,

		url:    url,
		dbname: dbname,
	}, nil
}

func (d *db) GetActiveRequests(ctx context.Context) ([]models.DownloadQueueRequest, error) {
	var requests []models.DownloadQueueRequest

	cursor, err := d.downloadQueueRequestCollection().Find(ctx, bson.M{"active": true})
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

func (d *db) UpdateActiveRequest(ctx context.Context, request models.DownloadQueueRequest) error {
	info, err := d.downloadQueueRequestCollection().UpdateOne(ctx, bson.M{"_id": request.ID}, bson.M{"$set": bson.M{
		"active":     request.Active,
		"sync_count": request.SyncCount,
	}})
	if err != nil {
		return err
	}

	if info.MatchedCount == 0 {
		return errors.New("not found")
	}
	return nil
}

// IndexMusicFile indexes a music file in the database
func (d *db) IndexMusicFile(ctx context.Context, file models.MusicFile) error {
	file.ID = uuid.Must(uuid.NewV4()).String()
	file.CreatedAt = time.Now().Unix()
	_, err := d.musicFilesCollection().InsertOne(ctx, file)
	return err
}

// MusicFileExist checks if a music file exists in the database
func (d *db) MusicFileExist(ctx context.Context, title string) (bool, error) {
	var count int64
	count, err := d.musicFilesCollection().CountDocuments(ctx, bson.M{"title": title})
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (d *db) reconnectToDB() error {
	d.conn.Disconnect(context.Background())

	conn, err := mongo.Connect(context.Background(), options.Client().ApplyURI(d.url))
	if err != nil {
		return err
	}

	d.conn = conn
	return nil
}

// Collections

// downloadQueueRequestCollection returns the download queue request collection
func (d *db) downloadQueueRequestCollection() *mongo.Collection {
	if err := d.conn.Ping(context.Background(), nil); err != nil {
		d.log.Error("failed to ping database. reconnecting.", zap.Error(err))
		d.reconnectToDB()
	}
	return d.conn.Database(d.dbname).Collection("download-queue-requests")
}

// musicFilesCollection returns the music files collection
func (d *db) musicFilesCollection() *mongo.Collection {
	if err := d.conn.Ping(context.Background(), nil); err != nil {
		d.log.Error("failed to ping database. reconnecting.", zap.Error(err))
		d.reconnectToDB()
	}

	return d.conn.Database(d.dbname).Collection("music-files")
}
