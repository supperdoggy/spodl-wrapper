package service

import (
	"context"
	"errors"
	"strings"

	"github.com/supperdoggy/spot-models"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/utils"
	"github.com/zmb3/spotify/v2"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

var (
	ErrMissingFiles = errors.New("missing files")
)

func (s *service) ProcessPlaylistRequest(ctx context.Context) error {

	indexStatus, err := s.database.GetIndexStatus(ctx)
	if err != nil {
		s.log.Error("failed to get index status", zap.Error(err))
		return err
	}

	if indexStatus.LastUpdated > indexStatus.LastIndexed {
		s.log.Info("indexing in progress, skipping playlist processing")
		return nil
	}

	playlists, err := s.database.GetActivePlaylists(ctx)
	if err != nil {
		s.log.Error("failed to get active playlists", zap.Error(err))
		return err
	}

	s.log.Info("processing active playlists", zap.Any("playlists", len(playlists)))

	for _, playlist := range playlists {
		if err := s.ProcessPlaylist(ctx, playlist); err != nil {
			s.log.Error("failed to process playlist", zap.Error(err), zap.Any("playlist", playlist))
			playlist.Errored = true
			playlist.RetryCount++
		} else {
			playlist.Active = false
		}

		if playlist.RetryCount >= 5 {
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

	// checking if playlist is ready to be processed
	// by checking if we have active request for playlist download
	downloadRequest, err := s.database.GetActiveRequest(ctx, playlist.SpotifyURL)
	if err != nil && err != mongo.ErrNoDocuments {
		s.log.Error("failed to get active request", zap.Error(err))
		return err
	}

	if downloadRequest.Active {
		s.log.Info("download request is still active, will continue to process playlist once done", zap.Any("playlist", playlist))
		return ErrMissingFiles
	}

	playlistName, err := s.spotifyService.GetObjectName(ctx, playlist.SpotifyURL)
	if err != nil {
		s.log.Error("failed to get playlist name", zap.Error(err))
		return err
	}

	songList, err := s.spotifyService.GetPlaylistTracks(ctx, playlist.SpotifyURL)
	if err != nil {
		s.log.Error("failed to get playlist data", zap.Error(err))
		return err
	}

	artists := []string{}
	titles := []string{}
	for _, item := range songList {
		if item.Track.Track == nil {
			s.log.Error("skipping empty track", zap.Any("item", item))
			continue
		}
		artist := []string{}
		for _, artistItem := range item.Track.Track.Artists {
			artist = append(artist, strings.ToLower(artistItem.Name))
		}

		artists = append(artists, strings.Join(artist, ", "))
		titles = append(titles, strings.ToLower(item.Track.Track.Name))
	}

	foundMusic, err := s.database.FindMusicFiles(ctx, artists, titles)
	if err != nil {
		s.log.Error("failed to find music file paths", zap.Error(err))
		return err
	}

	if len(foundMusic) == 0 {
		s.log.Error("no indexed paths found for playlist", zap.Any("playlistName", playlistName))
		return errors.New("no indexed paths found for playlist")
	}

	foundMusicMap := make(map[string]models.MusicFile)
	for _, music := range foundMusic {
		key := music.Artist + " " + music.Title
		foundMusicMap[key] = music
	}

	missingMusicFiles := []spotify.PlaylistItem{}
	indexedPaths := make([]string, 0)
	for _, song := range songList {
		if song.Track.Track == nil {
			s.log.Error("skipping empty track", zap.Any("item", song))
			continue
		}
		artists := []string{}
		for _, artistItem := range song.Track.Track.Artists {
			artists = append(artists, strings.ToLower(artistItem.Name))
		}

		artist := strings.Join(artists, ", ")

		songName := strings.ToLower(song.Track.Track.Name)

		key := artist + " " + songName

		if _, ok := foundMusicMap[key]; !ok {
			s.log.Error("song not found in indexed paths", zap.Any("artist", artist), zap.Any("songName", songName))
			missingMusicFiles = append(missingMusicFiles, song)
			// return errors.New("song not found in indexed paths")
			continue
		}

		indexedPaths = append(indexedPaths, foundMusicMap[key].Path)
	}

	// if we tried to download the playlist but it failed then whatever
	if len(missingMusicFiles) > 0 && !playlist.NoPull {
		s.log.Error("missing music files", zap.Any("missingMusicFiles", missingMusicFiles))
		alreadySynced, err := s.database.CheckIfRequestAlreadySynced(ctx, playlist.SpotifyURL)
		if err != nil {
			s.log.Error("failed to check if request already synced", zap.Error(err))
		}

		if !alreadySynced {
			if err := s.database.NewDownloadRequest(ctx, playlist.SpotifyURL, playlistName, 0); err != nil {
				s.log.Error("failed to add download request", zap.Error(err))
			}
			return ErrMissingFiles
		} else {
			s.log.Info("skipping already synced song", zap.Any("song", missingMusicFiles))
		}

	}

	for i, path := range indexedPaths {
		indexedPaths[i] = strings.ReplaceAll(path, "/mnt/music", "/music")
	}

	playlistPathName := strings.ReplaceAll(playlistName, "/", `-`)
	// playlistPathName = strings.ReplaceAll(playlistPathName, " ", `\ `)

	outputPath := s.destination + "/Playlists/" + playlistPathName + ".m3u"

	if err := utils.CreateM3UPlaylist(indexedPaths, s.libraryPath, outputPath); err != nil {
		s.log.Error("failed to create m3u playlist", zap.Error(err))
		return err
	}

	s.log.Info("created m3u playlist", zap.Any("outputPath", outputPath))

	return nil
}
