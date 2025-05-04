package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/DigitalIndependence/models"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/blob"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/db"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/utils"
	"github.com/zmb3/spotify/v2"
	"go.uber.org/zap"
)

type Service interface {
	StartProcessing(ctx context.Context) error
}

type service struct {
	database      db.Database
	s3            blob.BlobStorage
	log           *zap.Logger
	spotifyClient *spotify.Client

	destination    string
	s3Enabled      bool
	sleepInMinutes int
	libraryPath    string
}

func NewService(database db.Database, log *zap.Logger, s3 blob.BlobStorage, spotifyClient *spotify.Client, destination, libraryPath string, s3Enabled bool, sleepInMinutes int) Service {
	return &service{
		database:       database,
		log:            log,
		spotifyClient:  spotifyClient,
		destination:    destination,
		s3:             s3,
		s3Enabled:      s3Enabled,
		sleepInMinutes: sleepInMinutes,
		libraryPath:    libraryPath,
	}
}

// StartProcessing starts the processing of the requests
func (s *service) StartProcessing(ctx context.Context) error {
	downloadError := s.ProcessDownloadRequest(ctx)

	playlistError := s.ProcessPlaylistRequest(ctx)

	return errors.Join(downloadError, playlistError)
}

func (s *service) ProcessDownloadRequest(ctx context.Context) error {
	active, err := s.database.GetActiveRequests(ctx)
	if err != nil {
		s.log.Error("failed to get active requests", zap.Error(err))
		return err
	}

	s.log.Info("processing active requests", zap.Any("requests", len(active)))

	// sort active by errored so errored requests are processed last
	sort.Slice(active, func(i, j int) bool {
		if active[i].Errored && !active[j].Errored {
			return false
		}
		if !active[i].Errored && active[j].Errored {
			return true
		}
		return active[i].CreatedAt < active[j].CreatedAt
	})

	s.log.Info("sorted active requests", zap.Any("requests", active))

	for _, request := range active {
		time.Sleep(time.Duration(s.sleepInMinutes) * time.Minute)

		errored := false
		if err := s.ProcessRequest(ctx, request); err != nil {
			s.log.Error("failed to process request", zap.Error(err), zap.Any("request", request))
			errored = true
		}

		if !errored {
			if request.SyncCount >= 3 {
				request.Active = false
			} else {
				request.SyncCount++
			}
		} else {
			request.Errored = true
			request.RetryCount++
			s.log.Warn("request processing encountered an error", zap.Any("request", request))
		}

		s.log.Info("updated request status", zap.Any("request", request))

		if err := s.database.UpdateActiveRequest(ctx, request); err != nil {
			s.log.Error("failed to update request", zap.Error(err), zap.Any("request", request))
		}
	}

	s.log.Info("completed processing of active requests")
	return nil
}

// ProcessRequest processes the request
func (s *service) ProcessRequest(ctx context.Context, request models.DownloadQueueRequest) error {
	defer func() error {
		if r := recover(); r != nil {
			s.log.Error("recovered from panic", zap.Any("panic", r))
			return errors.New("recovered from panic")
		}
		return nil
	}()

	// Run the "spotdl --sync {url}" command
	cmd := exec.Command("spotdl", request.SpotifyURL, "--sync-without-deleting", "--cookie-file", "/home/maks/music.youtube.com_cookies.txt", "--bitrate", "320k", "--format", "m4a", "--output", s.destination)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Execute the command
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func (s *service) ProcessPlaylistRequest(ctx context.Context) error {
	playlists, err := s.database.GetActivePlaylists(ctx)
	if err != nil {
		s.log.Error("failed to get active playlists", zap.Error(err))
		return err
	}

	s.log.Info("processing active playlists", zap.Any("playlists", len(playlists)))

	for _, playlist := range playlists {
		if err := s.ProcessPlaylist(ctx, playlist); err != nil {
			s.log.Error("failed to process playlist", zap.Error(err), zap.Any("playlist", playlist))
		} else {
			playlist.Active = false
		}

		if err := s.database.UpdatePlaylistRequest(ctx, playlist); err != nil {
			s.log.Error("failed to update playlist", zap.Error(err), zap.Any("playlist", playlist))
		}
	}

	s.log.Info("completed processing of active playlists")
	return nil
}

func (s *service) ProcessPlaylist(ctx context.Context, playlist models.PlaylistRequest) error {
	s.log.Info("processing playlist", zap.Any("playlist", playlist))

	playlistName, songList, err := s.GetPlaylistData(playlist.SpotifyURL)
	if err != nil {
		s.log.Error("failed to get playlist data", zap.Error(err))
		return err
	}

	artistSong := make(map[string]string)
	for _, item := range songList {
		artist := []string{}
		for _, artistItem := range item.Track.Track.Artists {
			artist = append(artist, strings.ToLower(artistItem.Name))
		}

		artistSong[strings.Join(artist, ", ")] = strings.ToLower(item.Track.Track.Name)
	}

	indexedPaths, err := s.database.FindMusicFilePaths(ctx, artistSong)
	if err != nil {
		s.log.Error("failed to find music file paths", zap.Error(err))
		return err
	}

	if len(indexedPaths) == 0 {
		s.log.Error("no indexed paths found for playlist", zap.Any("playlistName", playlistName))
		return errors.New("no indexed paths found for playlist")
	}

	for i, path := range indexedPaths {
		indexedPaths[i] = strings.ReplaceAll(path, "/srv/remotemount/nascore_media/Music", "/music/")
	}

	s.log.Info("got playlist data", zap.Any("playlistName", playlistName), zap.Any("songList", songList))

	// sort by spotifyItems
	// sort.Slice(songList, func(i, j int) bool {
	// 	return songList[i].Track.Track.Name < songList[j].Track.Track.Name
	// })

	outputPath := s.destination + "/Playlists/" + playlistName + ".m3u"

	if err := utils.CreateM3UPlaylist(indexedPaths, s.libraryPath, outputPath); err != nil {
		s.log.Error("failed to create m3u playlist", zap.Error(err))
		return err
	}

	s.log.Info("created m3u playlist", zap.Any("outputPath", outputPath))

	return nil
}

func (s *service) GetPlaylistData(playlistURL string) (string, []spotify.PlaylistItem, error) {
	// Extract the list name from the playlist URL

	playlistID := strings.Split(strings.Split(playlistURL, "/")[4], "?")[0]
	playlist, err := s.spotifyClient.GetPlaylist(context.Background(), spotify.ID(playlistID))
	if err != nil {
		fmt.Println("Error getting playlist:", err)
		return "", nil, err
	}

	playlistName := playlist.Name

	var playlistItems []spotify.PlaylistItem
	itemsPage, err := s.spotifyClient.GetPlaylistItems(context.Background(), spotify.ID(playlistID))
	if err != nil {
		fmt.Println("Error getting playlist items:", err)
		return "", nil, err
	}

	if itemsPage.Total > spotify.Numeric(len(itemsPage.Items)) {
		total := int(itemsPage.Total)
		for i := 0; i < total; i += int(itemsPage.Limit) {
			items, err := s.spotifyClient.GetPlaylistItems(context.Background(), spotify.ID(playlistID), spotify.Limit(int(itemsPage.Limit)), spotify.Offset(i))
			if err != nil {
				fmt.Println("Error getting playlist items:", err)
				return "", nil, err
			}
			playlistItems = append(playlistItems, items.Items...)
		}
	} else {
		playlistItems = itemsPage.Items
	}

	// Create a list of "Artist - Song" strings
	// var songList []string
	// for _, song := range playlistItems {
	// 	artistNames := make([]string, len(song.Track.Track.Artists))
	// 	for i, artist := range song.Track.Track.Artists {
	// 		artistNames[i] = artist.Name
	// 	}
	// 	songList = append(songList, fmt.Sprintf("%s - %s", strings.Join(artistNames, ", "), song.Track.Track.Name))
	// }

	return playlistName, playlistItems, nil
}
