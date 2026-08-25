package importer

import (
	"fmt"
	"sync"
)

// BrokerRegistry maintains the catalog of extensible broker parsers.
type BrokerRegistry struct {
	mu      sync.RWMutex
	parsers []BrokerParser
}

func NewBrokerRegistry() *BrokerRegistry {
	r := &BrokerRegistry{}
	// Register default standard institutional parsers in priority order
	r.Register(NewCamsParser())
	r.Register(NewKFintechParser())
	r.Register(NewZerodhaMFParser())
	r.Register(NewZerodhaEquityParser())
	r.Register(NewGenericParser())
	return r
}

// Register adds a new parser to the catalog (OCP: enables plugging new broker formats without modifying core code).
func (r *BrokerRegistry) Register(parser BrokerParser) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.parsers = append(r.parsers, parser)
}

// DetectAndParse finds the first matching parser for a raw table and executes extraction.
func (r *BrokerRegistry) DetectAndParse(rows [][]string) (BrokerParser, []RawTradeDraft, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(rows) < 2 {
		return nil, nil, fmt.Errorf("file does not contain enough data rows to parse")
	}

	for _, p := range r.parsers {
		if p.Match(rows) {
			drafts, err := p.Parse(rows)
			if err != nil {
				return nil, nil, fmt.Errorf("error in parser '%s': %w", p.DisplayName(), err)
			}
			return p, drafts, nil
		}
	}

	return nil, nil, fmt.Errorf("unable to automatically detect broker format from headers")
}
