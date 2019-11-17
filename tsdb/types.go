// contains the required struct types for the project in order to marshal and unmarshal
// the general tsdb block

package tsdb

// Ping type for storing Ping values in TSDB
type Ping struct {
	Min  float64
	Mean float64
	Max  float64
	MDev float64
}

// FloodPing type for storing Ping values in TSDB
type FloodPing struct {
	Min        float64
	Mean       float64
	Max        float64
	MDev       float64
	PacketLoss float64
}

// BlockPing block for ping case
type BlockPing struct {
	PrevBlock      *BlockPing
	NextBlock      *BlockPing
	Datapoint      Ping
	NormalizedTime int64
}

// BlockFloodPing block for ping case
type BlockFloodPing struct {
	PrevBlock      *BlockFloodPing
	NextBlock      *BlockFloodPing
	Datapoint      FloodPing
	NormalizedTime int64
}
