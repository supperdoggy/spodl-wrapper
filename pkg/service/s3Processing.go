package service

import (
	"context"
	"os"
	"strings"

	"github.com/DigitalIndependence/models"
	"github.com/dhowden/tag"
	"go.uber.org/zap"
)

// S3Processing processes the downloaded files
func (s *service) S3Processing(ctx context.Context) {
	s.log.Info("processed all active requests")

	// get all downloaded files
	files, err := s.GetDownloadedFilesPath(ctx)
	if err != nil {
		s.log.Error("failed to get downloaded files", zap.Error(err))
		return
	}

	for _, file := range files {
		data, metadata, err := s.GetFileData(ctx, file)
		if err != nil {
			s.log.Error("failed to get file data", zap.Error(err))
			continue
		}

		// upload the file to the blob storage
		path, err := s.s3.UploadFile(data, file)
		if err != nil {
			s.log.Error("failed to upload file to blob storage", zap.Error(err))
			continue
		}

		// create a new music file
		musicFile := models.MusicFile{
			Title: file,
			Path:  path,

			MetaData: metadata.Raw(),
		}

		// index the music file in the database
		if err := s.database.IndexMusicFile(ctx, musicFile); err != nil {
			s.log.Error("failed to index music file", zap.Error(err))
			continue
		}

		// delete the file
		if err := s.DeleteDownloadedFiles(ctx, []string{file}); err != nil {
			s.log.Error("failed to delete downloaded file", zap.Error(err))
			continue
		}
	}
}

// GetDownloadedFilesPath returns the path of all the downloaded files
func (s *service) GetDownloadedFilesPath(ctx context.Context) ([]string, error) {
	// get all files in the destination folder
	files, err := os.ReadDir(s.destination)
	if err != nil {
		return nil, err
	}

	// get all files that are mp3
	var mp3Files []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if strings.Contains(file.Name(), ".mp3") {
			mp3Files = append(mp3Files, s.destination+"/"+file.Name())
		}
	}

	return mp3Files, nil
}

// DeleteDownloadedFiles deletes the files that are passed in the files array
func (s *service) DeleteDownloadedFiles(ctx context.Context, files []string) error {
	for _, file := range files {
		if err := os.Remove(file); err != nil {
			return err
		}
	}

	return nil
}

// GetFileData returns the data of the file at the path
func (s *service) GetFileData(ctx context.Context, path string) ([]byte, tag.Metadata, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return nil, nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	// get file meta data
	metadata, err := tag.ReadFrom(file)
	if err != nil {
		return nil, nil, err
	}

	return data, metadata, nil
}
