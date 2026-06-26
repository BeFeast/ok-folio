package legacyetl

import (
	"strings"
	"testing"
)

func TestParseDumpAcceptsOnlyOwnedTableRows(t *testing.T) {
	dump := `
INSERT INTO ` + "`downloaded_photos`" + ` VALUES
(7,'https://example.test/p.jpg','https://example.test/page','Title','Artist\'s Name','2026-06-01 12:00:00.000','/originals/p.jpg','p.jpg','2026-06-02 03:04:05.000',123,'pending','waiting');
INSERT INTO ` + "`extraction_runs`" + ` VALUES
(3,'2026-06-02 00:00:00.000',NULL,'running',1,2,3,4,5,'');
`
	rows, err := ParseDump(strings.NewReader(dump))
	if err != nil {
		t.Fatalf("ParseDump failed: %v", err)
	}
	if len(rows.DownloadedPhotos) != 1 || len(rows.ExtractionRuns) != 1 {
		t.Fatalf("unexpected row counts: %#v", rows)
	}
	photo := rows.DownloadedPhotos[0]
	if photo.ID != 7 || photo.Artist != "Artist's Name" || photo.Status != "pending" || photo.FilePath != "/originals/p.jpg" {
		t.Fatalf("unexpected photo row: %#v", photo)
	}
	run := rows.ExtractionRuns[0]
	if run.ID != 3 || run.EndTime != nil || run.PhotosFailed != 5 {
		t.Fatalf("unexpected run row: %#v", run)
	}
}

func TestParseDumpRejectsNonOwnedTables(t *testing.T) {
	dump := "INSERT INTO `photos` VALUES (1,'legacy');"
	if _, err := ParseDump(strings.NewReader(dump)); err == nil {
		t.Fatal("expected non-owned legacy table dump to be rejected")
	}
}

func TestParseDumpSupportsExplicitColumnOrder(t *testing.T) {
	dump := "INSERT INTO `extraction_runs` (`status`,`id`,`start_time`,`error_message`,`photos_failed`,`photos_skipped`,`photos_downloaded`,`photos_found`,`pages_processed`,`end_time`) VALUES ('completed',9,'2026-06-02 00:00:00.000','ok',0,1,2,3,4,'2026-06-02 00:01:00.000');"
	rows, err := ParseDump(strings.NewReader(dump))
	if err != nil {
		t.Fatalf("ParseDump failed: %v", err)
	}
	run := rows.ExtractionRuns[0]
	if run.ID != 9 || run.Status != "completed" || run.PagesProcessed != 4 || run.EndTime == nil {
		t.Fatalf("unexpected explicit-column row: %#v", run)
	}
}
