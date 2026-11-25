package legacyetl

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type LegacyDownloadedPhoto struct {
	ID           uint64
	URL          string
	SourcePage   string
	Title        string
	Artist       string
	UploadDate   *string
	FilePath     string
	FileName     string
	DownloadedAt string
	FileSize     int64
	Status       string
	ErrorMessage string
}

type LegacyExtractionRun struct {
	ID               uint64
	StartTime        string
	EndTime          *string
	Status           string
	PagesProcessed   int
	PhotosFound      int
	PhotosDownloaded int
	PhotosSkipped    int
	PhotosFailed     int
	ErrorMessage     string
}

type DumpRows struct {
	DownloadedPhotos []LegacyDownloadedPhoto
	ExtractionRuns   []LegacyExtractionRun
}

type sqlValue struct {
	text string
	null bool
}

var defaultPhotoColumns = []string{
	"id", "url", "source_page", "title", "artist", "upload_date", "file_path",
	"file_name", "downloaded_at", "file_size", "status", "error_message",
}

var defaultRunColumns = []string{
	"id", "start_time", "end_time", "status", "pages_processed", "photos_found",
	"photos_downloaded", "photos_skipped", "photos_failed", "error_message",
}

func ParseDump(r io.Reader) (DumpRows, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return DumpRows{}, err
	}
	statements, err := splitSQLStatements(string(data))
	if err != nil {
		return DumpRows{}, err
	}
	var out DumpRows
	for _, stmt := range statements {
		table, columns, tuples, ok, err := parseInsertStatement(stmt)
		if err != nil {
			return DumpRows{}, err
		}
		if !ok {
			continue
		}
		switch table {
		case DownloadedPhotosTable:
			if len(columns) == 0 {
				columns = defaultPhotoColumns
			}
			for _, tuple := range tuples {
				row, err := mapPhoto(columns, tuple)
				if err != nil {
					return DumpRows{}, err
				}
				out.DownloadedPhotos = append(out.DownloadedPhotos, row)
			}
		case ExtractionRunsTable:
			if len(columns) == 0 {
				columns = defaultRunColumns
			}
			for _, tuple := range tuples {
				row, err := mapRun(columns, tuple)
				if err != nil {
					return DumpRows{}, err
				}
				out.ExtractionRuns = append(out.ExtractionRuns, row)
			}
		default:
			return DumpRows{}, fmt.Errorf("dump contains non-owned table %q", table)
		}
	}
	return out, nil
}

func splitSQLStatements(input string) ([]string, error) {
	var statements []string
	var b strings.Builder
	inString := false
	escaped := false
	scanner := bufio.NewReader(strings.NewReader(input))
	for {
		r, _, err := scanner.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		b.WriteRune(r)
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch r {
			case '\\':
				escaped = true
			case '\'':
				next, _, readErr := scanner.ReadRune()
				if readErr == nil {
					if next == '\'' {
						b.WriteRune(next)
						continue
					}
					_ = scanner.UnreadRune()
				}
				inString = false
			}
			continue
		}
		if r == '\'' {
			inString = true
			continue
		}
		if r == ';' {
			stmt := strings.TrimSpace(b.String())
			if stmt != "" {
				statements = append(statements, strings.TrimSuffix(stmt, ";"))
			}
			b.Reset()
		}
	}
	trailing := strings.TrimSpace(b.String())
	if trailing != "" && !strings.HasPrefix(trailing, "--") {
		return nil, fmt.Errorf("unterminated SQL statement")
	}
	return statements, nil
}

func parseInsertStatement(stmt string) (string, []string, [][]sqlValue, bool, error) {
	trimmed := strings.TrimSpace(stmt)
	if !strings.HasPrefix(strings.ToUpper(trimmed), "INSERT INTO ") {
		return "", nil, nil, false, nil
	}
	rest := strings.TrimSpace(trimmed[len("INSERT INTO "):])
	table, rest, err := readIdentifier(rest)
	if err != nil {
		return "", nil, nil, true, err
	}
	rest = strings.TrimSpace(rest)
	var columns []string
	if strings.HasPrefix(rest, "(") {
		var raw string
		raw, rest, err = readParenthesized(rest)
		if err != nil {
			return "", nil, nil, true, err
		}
		for _, col := range strings.Split(raw, ",") {
			columns = append(columns, strings.Trim(strings.TrimSpace(col), "`"))
		}
		rest = strings.TrimSpace(rest)
	}
	if !strings.HasPrefix(strings.ToUpper(rest), "VALUES") {
		return "", nil, nil, true, fmt.Errorf("INSERT for %q is not a VALUES dump", table)
	}
	tuples, err := parseTuples(strings.TrimSpace(rest[len("VALUES"):]))
	return table, columns, tuples, true, err
}

func readIdentifier(input string) (string, string, error) {
	if input == "" {
		return "", "", fmt.Errorf("missing table name")
	}
	if input[0] == '`' {
		end := strings.IndexByte(input[1:], '`')
		if end < 0 {
			return "", "", fmt.Errorf("unterminated quoted table name")
		}
		return input[1 : end+1], input[end+2:], nil
	}
	for i, r := range input {
		if r == ' ' || r == '\t' || r == '\n' || r == '(' {
			return input[:i], input[i:], nil
		}
	}
	return input, "", nil
}

func readParenthesized(input string) (string, string, error) {
	depth := 0
	for i, r := range input {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return input[1:i], input[i+1:], nil
			}
		}
	}
	return "", "", fmt.Errorf("unterminated parenthesized list")
}

func parseTuples(input string) ([][]sqlValue, error) {
	var tuples [][]sqlValue
	i := 0
	for i < len(input) {
		for i < len(input) && (input[i] == ',' || input[i] == ' ' || input[i] == '\n' || input[i] == '\t') {
			i++
		}
		if i >= len(input) {
			break
		}
		if input[i] != '(' {
			return nil, fmt.Errorf("expected tuple start near %q", input[i:])
		}
		tuple, next, err := parseTuple(input, i+1)
		if err != nil {
			return nil, err
		}
		tuples = append(tuples, tuple)
		i = next
	}
	return tuples, nil
}

func parseTuple(input string, start int) ([]sqlValue, int, error) {
	var values []sqlValue
	var b strings.Builder
	inString := false
	escaped := false
	valueQuoted := false
	for i := start; i < len(input); i++ {
		ch := input[i]
		if inString {
			if escaped {
				b.WriteByte(unescapeMariaDBByte(ch))
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '\'':
				if i+1 < len(input) && input[i+1] == '\'' {
					b.WriteByte('\'')
					i++
					continue
				}
				inString = false
			default:
				b.WriteByte(ch)
			}
			continue
		}
		switch ch {
		case '\'':
			inString = true
			valueQuoted = true
		case ',':
			values = append(values, finishValue(b.String(), valueQuoted))
			b.Reset()
			valueQuoted = false
		case ')':
			values = append(values, finishValue(b.String(), valueQuoted))
			return values, i + 1, nil
		default:
			b.WriteByte(ch)
		}
	}
	return nil, 0, fmt.Errorf("unterminated tuple")
}

func finishValue(raw string, quoted bool) sqlValue {
	if !quoted && strings.EqualFold(strings.TrimSpace(raw), "NULL") {
		return sqlValue{null: true}
	}
	if quoted {
		return sqlValue{text: raw}
	}
	return sqlValue{text: strings.TrimSpace(raw)}
}

func unescapeMariaDBByte(ch byte) byte {
	switch ch {
	case '0':
		return 0
	case 'n':
		return '\n'
	case 'r':
		return '\r'
	case 't':
		return '\t'
	case 'b':
		return '\b'
	case 'Z':
		return 26
	default:
		return ch
	}
}

func mapPhoto(columns []string, values []sqlValue) (LegacyDownloadedPhoto, error) {
	if len(columns) != len(values) {
		return LegacyDownloadedPhoto{}, fmt.Errorf("downloaded_photos column/value mismatch: %d columns, %d values", len(columns), len(values))
	}
	byName := valuesByColumn(columns, values)
	id, err := parseUint(byName["id"])
	if err != nil {
		return LegacyDownloadedPhoto{}, fmt.Errorf("downloaded_photos.id: %w", err)
	}
	fileSize, err := parseInt(byName["file_size"])
	if err != nil {
		return LegacyDownloadedPhoto{}, fmt.Errorf("downloaded_photos.file_size: %w", err)
	}
	return LegacyDownloadedPhoto{
		ID:           id,
		URL:          valueText(byName["url"]),
		SourcePage:   valueText(byName["source_page"]),
		Title:        valueText(byName["title"]),
		Artist:       valueText(byName["artist"]),
		UploadDate:   valuePtr(byName["upload_date"]),
		FilePath:     valueText(byName["file_path"]),
		FileName:     valueText(byName["file_name"]),
		DownloadedAt: valueText(byName["downloaded_at"]),
		FileSize:     fileSize,
		Status:       valueText(byName["status"]),
		ErrorMessage: valueText(byName["error_message"]),
	}, nil
}

func mapRun(columns []string, values []sqlValue) (LegacyExtractionRun, error) {
	if len(columns) != len(values) {
		return LegacyExtractionRun{}, fmt.Errorf("extraction_runs column/value mismatch: %d columns, %d values", len(columns), len(values))
	}
	byName := valuesByColumn(columns, values)
	id, err := parseUint(byName["id"])
	if err != nil {
		return LegacyExtractionRun{}, fmt.Errorf("extraction_runs.id: %w", err)
	}
	pagesProcessed, err := parseIntAsInt(byName["pages_processed"])
	if err != nil {
		return LegacyExtractionRun{}, err
	}
	photosFound, err := parseIntAsInt(byName["photos_found"])
	if err != nil {
		return LegacyExtractionRun{}, err
	}
	photosDownloaded, err := parseIntAsInt(byName["photos_downloaded"])
	if err != nil {
		return LegacyExtractionRun{}, err
	}
	photosSkipped, err := parseIntAsInt(byName["photos_skipped"])
	if err != nil {
		return LegacyExtractionRun{}, err
	}
	photosFailed, err := parseIntAsInt(byName["photos_failed"])
	if err != nil {
		return LegacyExtractionRun{}, err
	}
	return LegacyExtractionRun{
		ID:               id,
		StartTime:        valueText(byName["start_time"]),
		EndTime:          valuePtr(byName["end_time"]),
		Status:           valueText(byName["status"]),
		PagesProcessed:   pagesProcessed,
		PhotosFound:      photosFound,
		PhotosDownloaded: photosDownloaded,
		PhotosSkipped:    photosSkipped,
		PhotosFailed:     photosFailed,
		ErrorMessage:     valueText(byName["error_message"]),
	}, nil
}

func valuesByColumn(columns []string, values []sqlValue) map[string]sqlValue {
	out := make(map[string]sqlValue, len(columns))
	for i, col := range columns {
		out[strings.ToLower(strings.Trim(col, "`"))] = values[i]
	}
	return out
}

func parseUint(value sqlValue) (uint64, error) {
	if value.null {
		return 0, fmt.Errorf("is NULL")
	}
	return strconv.ParseUint(value.text, 10, 64)
}

func parseInt(value sqlValue) (int64, error) {
	if value.null || value.text == "" {
		return 0, nil
	}
	return strconv.ParseInt(value.text, 10, 64)
}

func parseIntAsInt(value sqlValue) (int, error) {
	parsed, err := parseInt(value)
	return int(parsed), err
}

func valueText(value sqlValue) string {
	if value.null {
		return ""
	}
	return value.text
}

func valuePtr(value sqlValue) *string {
	if value.null || value.text == "" {
		return nil
	}
	text := value.text
	return &text
}
