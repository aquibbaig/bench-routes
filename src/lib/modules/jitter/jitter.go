package jitter

import (
	"strconv"
	"sync"
	"time"

	"github.com/zairza-cetb/bench-routes/src/lib/filters"
	scrap "github.com/zairza-cetb/bench-routes/src/lib/filters/scraps"
	"github.com/zairza-cetb/bench-routes/src/lib/logger"
	"github.com/zairza-cetb/bench-routes/src/lib/parser"
	"github.com/zairza-cetb/bench-routes/src/lib/utils"
	"github.com/zairza-cetb/bench-routes/tsdb"
)

// Jitter is the structure that implements the Jitter service.
type Jitter struct {
	localConfig    *parser.YAMLBenchRoutesType
	scrapeInterval TestInterval
	chain          []*tsdb.Chain
}

//TestInterval stores the value of the duration and the type of test
type TestInterval struct {
	OfType   string
	Duration int64
}

// New returns a Jitter type.
func New(configuration *parser.YAMLBenchRoutesType, scrapeInterval TestInterval, chain []*tsdb.Chain) *Jitter {
	return &Jitter{
		localConfig:    configuration,
		scrapeInterval: scrapeInterval,
		chain:          chain,
	}
}

// Iterate iterates over the local-configuration file to keep state
// of the jitter service in sync with the local configuration.
// It is responbile for stopping the service without damaging the currently
// calculated samples.
func (ps *Jitter) Iterate(signal string) bool {
	// Get latest service state settings

	conf := ps.localConfig
	conf.Refresh()
	pingServiceState := conf.Config.UtilsConf.ServicesSignal.Jitter

	switch signal {
	case "start":
		if pingServiceState == "passive" {

			conf.Config.UtilsConf.ServicesSignal.Jitter = "active"
			_, e := conf.Write()
			if e != nil {
				panic(e)
			}
			go func() {
				ps.setConfigurations()
			}()
			return true
		}
		return true
	case "stop":
		conf.Config.UtilsConf.ServicesSignal.Jitter = "passive"
		_, e := conf.Write()
		if e != nil {
			panic(e)
		}
		return true
	default:
		logger.Terminal("invalid signal", "f")
	}
	return false
}

func (ps *Jitter) setConfigurations() {
	pingConfig := ps.localConfig.Config.Routes
	pingInterval := ps.scrapeInterval

	urlStack := make(map[string]string)
	for _, route := range pingConfig {
		url := route.URL
		urlHash := utils.GetHash(url)
		// maintain urls uniquely
		_, ok := urlStack[urlHash]
		if !ok {
			urlStack[urlHash] = *filters.HTTPPingFilter(&url)
		}
	}

	ps.perform(urlStack, pingInterval)
}

func (ps *Jitter) perform(urlStack map[string]string, pingInterval TestInterval) {
	i := 0
	config := ps.localConfig

	for {
		i++
		config.Refresh()

		switch config.Config.UtilsConf.ServicesSignal.Jitter {
		case "active":
			err, _ := utils.VerifyConnection()
			if !err {
				logger.Terminal("Not able to connect to externel network please check you internet connection", "p")
			} else {
				var wg sync.WaitGroup
				wg.Add(len(urlStack))
				for _, u := range urlStack {
					go ps.jitter(u, 10, u, &wg, false)
				}
				wg.Wait()
			}
		case "passive":
			// terminate the goroutine
			logger.Terminal("terminating jitter goroutine", "p")
			return
		default:
			logger.Terminal("invalid service-state value of jitter", "f")
			return
		}

		intrv := time.Duration(pingInterval.Duration)
		switch pingInterval.OfType {
		case "hr":
			time.Sleep(intrv * time.Hour)
		case "min":
			time.Sleep(intrv * time.Minute)
		case "sec":
			time.Sleep(intrv * time.Second)
		default:
			logger.Terminal("invalid interval-type for jitter", "f")
			return
		}
	}
}

func (ps *Jitter) jitter(urlRaw string, packets int, tsdbNameHash string, wg *sync.WaitGroup, isTest bool) {
	chain := ps.chain
	tsdbNameHash = utils.PathPing + "/" + "chunk_ping_" + tsdbNameHash + ".json"

	resp, err := utils.CLIPing(urlRaw, packets)
	if err != nil {
		msg := "Failure occured for " + urlRaw
		logger.Terminal(msg, "p")
		wg.Done()
		return
	}

	result := *scrap.CLIPingScrap(resp)
	newBlock := *tsdb.GetNewBlock("jitter", getNormalizedBlockString(result))
	urlExists := false

	for index := range chain {
		if chain[index].Path == tsdbNameHash {
			urlExists = true
			chain[index] = chain[index].Append(newBlock)
			chain[index].Commit()
			break
		}
	}

	if !urlExists && !isTest {
		panic("faulty hashing! impossible to look for a hash match.")
	}

	wg.Done()
}

func getNormalizedBlockString(v utils.TypePingScrap) string {
	return fToS(v.Min) + tsdb.BlockDataSeparator + fToS(v.Avg) +
		tsdb.BlockDataSeparator + fToS(v.Max) + tsdb.BlockDataSeparator + fToS(v.Mdev)
}

func fToS(v float64) string {
	return strconv.FormatFloat(v, 'f', 6, 64)
}
