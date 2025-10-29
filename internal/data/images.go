package data

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type ImageModel interface {
	Insert(userID int64, input *ImageData) (*ImageData, error)
	Delete(userID int64, noteID int64, s3Key string) error
	GetForNote(noteID int64) ([]*ImageData, error)
}

type ImageData struct {
	ID               int64     `json:"id"`
	NoteID           int64     `json:"note_id"`
	S3Key            string    `json:"s3_key"`
	PresignedURL     string    `json:"image_url,omitempty"`
	Width            int       `json:"width"`
	Height           int       `json:"height"`
	OriginalFileName string    `json:"original_filename"`
	MimeType         string    `json:"mime_type"`
	FileSize         int       `json:"file_size"`
	CreatedAt        time.Time `json:"created_at"`
}

type imageModel struct {
	DB *sql.DB
}

func NewImageModel(db *sql.DB) imageModel {
	return imageModel{DB: db}
}

// insert into the images table
func (m imageModel) Insert(userID int64, input *ImageData) (*ImageData, error) {
	query := `
		INSERT INTO images	
			(note_id, s3_key, width, height, file_size, mime_type, original_filename, created_at)
		SELECT 
			n.id as note_id,
			$3, $4, $5, $6, $7, $8, $9
		FROM 
			notes n
		WHERE 
			n.user_id = $1
			AND n.id = $2
		RETURNING *
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var response ImageData

	args := []any{userID, input.NoteID, input.S3Key, input.Width,
		input.Height, input.FileSize, input.MimeType, input.OriginalFileName, input.CreatedAt}

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&response.ID, &response.NoteID, &response.S3Key, &response.Width, &response.Height,
		&response.OriginalFileName, &response.MimeType, &response.FileSize, &response.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return &response, nil
}

func (m imageModel) Delete(userID int64, noteID int64, s3Key string) error {
	query := `
		DELETE FROM 
			images i
		USING 
			notes n
		WHERE 
			i.note_id = n.id
			AND n.user_id = $1
			AND n.id = $2
			AND i.s3_key = $3
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, userID, noteID, s3Key)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m imageModel) GetForNote(noteID int64) ([]*ImageData, error) {
	query := `
		SELECT 
			* 
		FROM
			images 
		WHERE 
			note_id = $1 
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, noteID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	var response []*ImageData

	for rows.Next() {
		var data ImageData
		err := rows.Scan(
			&data.ID,
			&data.NoteID,
			&data.S3Key,
			&data.Width,
			&data.Height,
			&data.OriginalFileName,
			&data.MimeType,
			&data.FileSize,
			&data.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		response = append(response, &data)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return response, nil
}
