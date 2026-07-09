package services

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	"task1/backend/models"
	"task1/backend/utils"
)

func parseCSVContent(content []byte) (models.FileData, error) {
	text, encoding, err := utils.DecodeCSVContent(content)
	if err != nil {
		return models.FileData{}, err
	}
	if strings.TrimSpace(text) == "" {
		return models.FileData{}, ErrEmptyFile
	}

	reader := csv.NewReader(strings.NewReader(text))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	reader.Comma = utils.DetectCSVDelimiter(text)

	records := make([]parsedRecord, 0)
	recordNumber := 0
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		recordNumber++
		if err != nil {
			return models.FileData{}, fmt.Errorf("%w %v", ErrInvalidCSV, err)
		}

		records = append(records, parsedRecord{
			Number: recordNumber,
			Values: record,
		})
	}

	data, err := recordsToFileData(records)
	if err != nil {
		return models.FileData{}, err
	}
	data.Encoding = encoding
	if encoding != "UTF-8" {
		data.Warnings = append(data.Warnings, models.ProcessingWarning{
			Message: fmt.Sprintf("CSV декодирован из %s.", encoding),
		})
		refreshStats(&data)
	}

	return data, nil
}
