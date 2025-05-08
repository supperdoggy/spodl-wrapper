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
	GetActiveRequest(ctx context.Context, url string) (models.DownloadQueueRequest, error)
	CheckIfRequestAlreadySynced(ctx context.Context, url string) (bool, error)
	NewDownloadRequest(ctx context.Context, url, name string, creatorID int64) error
	UpdateActiveRequest(ctx context.Context, request models.DownloadQueueRequest) error

	GetActivePlaylists(ctx context.Context) ([]models.PlaylistRequest, error)
	UpdatePlaylistRequest(ctx context.Context, request models.PlaylistRequest) error

	FindMusicFiles(ctx context.Context, artists, titles []string) ([]models.MusicFile, error)
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

func (d *db) NewDownloadRequest(ctx context.Context, url, name string, creatorID int64) error {
	id, err := uuid.NewV4()
	if err != nil {
		return err
	}

	request := models.DownloadQueueRequest{
		SpotifyURL: url,
		Name:       name,
		Active:     true,
		ID:         id.String(),
		CreatedAt:  time.Now().Unix(),
		CreatorID:  creatorID,
	}

	_, err = d.downloadQueueRequestCollection().InsertOne(ctx, request)
	if err != nil {
		return err
	}

	return nil
}

func (d *db) CheckIfRequestAlreadySynced(ctx context.Context, url string) (bool, error) {
	var count int64
	count, err := d.downloadQueueRequestCollection().CountDocuments(ctx, bson.M{"spotify_url": url, "active": false})
	if err != nil && err != mongo.ErrNoDocuments {
		return false, err
	}

	return count > 0, nil
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

func (d *db) GetActivePlaylists(ctx context.Context) ([]models.PlaylistRequest, error) {
	var requests []models.PlaylistRequest
	cursor, err := d.playlistsCollection().Find(ctx, bson.M{"active": true})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var request models.PlaylistRequest
		if err := cursor.Decode(&request); err != nil {
			return nil, err
		}

		requests = append(requests, request)
	}
	return requests, nil
}

func (d *db) UpdatePlaylistRequest(ctx context.Context, request models.PlaylistRequest) error {
	info, err := d.playlistsCollection().UpdateOne(ctx, bson.M{"_id": request.ID}, bson.M{"$set": bson.M{
		"active":      request.Active,
		"errored":     request.Errored,
		"retry_count": request.RetryCount,
	}})

	if info.MatchedCount == 0 {
		return errors.New("not found")
	}

	return err
}

func (d *db) UpdateActiveRequest(ctx context.Context, request models.DownloadQueueRequest) error {
	info, err := d.downloadQueueRequestCollection().UpdateOne(ctx, bson.M{"_id": request.ID}, bson.M{"$set": bson.M{
		"active":      request.Active,
		"sync_count":  request.SyncCount,
		"errored":     request.Errored,
		"retry_count": request.RetryCount,
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

func (d *db) playlistsCollection() *mongo.Collection {
	if err := d.conn.Ping(context.Background(), nil); err != nil {
		d.log.Error("failed to ping database. reconnecting.", zap.Error(err))
		d.reconnectToDB()
	}

	return d.conn.Database(d.dbname).Collection("playlist-requests")
}

// musicFilesCollection returns the music files collection
func (d *db) musicFilesCollection() *mongo.Collection {
	if err := d.conn.Ping(context.Background(), nil); err != nil {
		d.log.Error("failed to ping database. reconnecting.", zap.Error(err))
		d.reconnectToDB()
	}

	return d.conn.Database(d.dbname).Collection("music-files")
}
func (d *db) FindMusicFiles(ctx context.Context, artists, titles []string) ([]models.MusicFile, error) {
	orPairs := make([]bson.M, 0, len(artists))
	for i := range artists {
		orPairs = append(orPairs, bson.M{
			"artist": artists[i],
			"title":  titles[i],
		})
	}

	d.log.Info("Finding music files", zap.Any("orPairs", orPairs))

	cur, err := d.musicFilesCollection().Find(ctx, bson.M{
		"$or": orPairs,
	}, options.Find().SetProjection(bson.M{"meta_data": 0}))
	if err != nil {
		return nil, err
	}

	defer cur.Close(ctx)

	files := make([]models.MusicFile, 0)
	for cur.Next(ctx) {
		var file models.MusicFile
		if err := cur.Decode(&file); err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}

	return files, nil
}

func (d *db) GetActiveRequest(ctx context.Context, url string) (models.DownloadQueueRequest, error) {
	cur := d.downloadQueueRequestCollection().FindOne(ctx, bson.M{"spotify_url": url, "active": true})
	var req models.DownloadQueueRequest
	if err := cur.Decode(&req); err != nil {
		return models.DownloadQueueRequest{}, err
	}

	return req, nil
}
