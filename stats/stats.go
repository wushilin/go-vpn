package stats

import (
	"sync/atomic"
)

type GlobalStats struct {
	uploaded    uint64
	downloaded  uint64
	reconnected uint64
}

func New() *GlobalStats {
	return &GlobalStats{
		uploaded:    0,
		downloaded:  0,
		reconnected: 0,
	}
}

func (v *GlobalStats) IncreaseUploadedBytes(new uint64) uint64 {
	return atomic.AddUint64(&v.uploaded, new)
}

func (v *GlobalStats) IncreaseReconnectCount() uint64 {
	return atomic.AddUint64(&v.reconnected, 1)
}

func (v *GlobalStats) ReconnectedCount() uint64 {
	return v.reconnected
}

func (v *GlobalStats) IncreaseDownloadedBytes(new uint64) uint64 {
	return atomic.AddUint64(&v.downloaded, new)
}

func (v *GlobalStats) DownloadedBytes() uint64 {
	return v.downloaded
}

func (v *GlobalStats) UploadedBytes() uint64 {
	return v.uploaded
}
