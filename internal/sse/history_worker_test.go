package sse

import (
	"testing"
	"time"

	"NodePassDash/internal/models"
)

func TestHistoryRecordTimeUsesUTC(t *testing.T) {
	worker := &HistoryWorker{historyWriteChan: make(chan *models.ServiceHistory, 1)}
	eventTime := time.Date(2026, 9, 7, 13, 20, 35, 0, time.FixedZone("UTC+8", 8*60*60))
	worker.aggregateAndEnqueueWrite([]MonitoringData{
		{EndpointID: 1, InstanceID: "a", Timestamp: eventTime.Add(-5 * time.Second), TCPIn: 100},
		{EndpointID: 1, InstanceID: "a", Timestamp: eventTime, TCPIn: 200},
	})
	record := <-worker.historyWriteChan
	if record.RecordTime.Location() != time.UTC || !record.RecordTime.Equal(eventTime.UTC().Truncate(time.Minute)) {
		t.Fatalf("record time = %s, want UTC minute", record.RecordTime)
	}
}
