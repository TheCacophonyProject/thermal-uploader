package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseDateTimeFromFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected time.Time
	}{
		{"dashed layout", "/var/spool/cptv/2023-05-04--21-03-09.cptv", time.Date(2023, 5, 4, 21, 3, 9, 0, time.Local)},
		{"dashed layout with suffix", "2023-05-04--21-03-09-track.cptv", time.Date(2023, 5, 4, 21, 3, 9, 0, time.Local)},
		{"compact layout", "20230504-210309.cptv", time.Date(2023, 5, 4, 21, 3, 9, 0, time.Local)},
		{"compact layout with microseconds", "20230504-210309.123456.cptv", time.Date(2023, 5, 4, 21, 3, 9, 123456000, time.Local)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := newUploadJob(tt.filename)
			require.NoError(t, u.parseDateTime())
			require.True(t, u.dateParsed)
			require.True(t, tt.expected.Equal(u.recDate), "expected %v got %v", tt.expected, u.recDate)
		})
	}
}

func TestParseDateTimeFromFilenameFails(t *testing.T) {
	for _, filename := range []string{
		"recording.cptv",
		"2023-05-04.cptv",
		"2023-13-45--99-99-99.cptv",
		"abcdefgh-ijklmn.cptv",
		"",
	} {
		t.Run(filename, func(t *testing.T) {
			u := newUploadJob(filename)
			err := u.parseDateTime()
			require.Error(t, err)
			require.Contains(t, err.Error(), filename)
			require.False(t, u.dateParsed)
			require.True(t, u.recDate.IsZero())
		})
	}
}

func TestParseDateTimeUTC(t *testing.T) {
	dt, err := parseDateTime("20230504-210309.cptv", "20060102-150405", true)
	require.NoError(t, err)
	require.True(t, time.Date(2023, 5, 4, 21, 3, 9, 0, time.UTC).Equal(dt))

	_, err = parseDateTime("short.cptv", "20060102-150405", true)
	require.Error(t, err)
}
